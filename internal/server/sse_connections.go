package server

import (
    "bufio"
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
    "time"

    "mcpcompose/internal/config"
)

// MCPSSEConnection represents a Server-Sent Events connection to an MCP server
type MCPSSEConnection struct {
    ServerName      string
    BaseURL         string
    SSEEndpoint     string
    SessionEndpoint string
    LastUsed        time.Time
    Initialized     bool
    Healthy         bool
    Capabilities    map[string]interface{}
    ServerInfo      map[string]interface{}
    SessionID       string
    // Critical: Keep these alive for the session lifetime
    sseResponse     *http.Response
    sseBody         io.ReadCloser
    sseCancel       context.CancelFunc
    sseReader       *bufio.Scanner
    pendingRequests map[interface{}]chan map[string]interface{}
    reqMutex        sync.Mutex
    mu              sync.Mutex
}

func (h *ProxyHandler) getSSEConnection(serverName string) (*MCPSSEConnection, error) {
    h.SSEMutex.RLock()
    conn, exists := h.SSEConnections[serverName]
    h.SSEMutex.RUnlock()

    if exists && h.isSSEConnectionHealthy(conn) {
        conn.mu.Lock()
        conn.LastUsed = time.Now()
        conn.mu.Unlock()
        h.logger.Debug("Reusing healthy SSE connection for %s", serverName)
        return conn, nil
    }

    // Clean up unhealthy connection
    if exists && !h.isSSEConnectionHealthy(conn) {
        h.logger.Info("Cleaning up unhealthy SSE connection for %s", serverName)
        h.SSEMutex.Lock()
        delete(h.SSEConnections, serverName)
        h.SSEMutex.Unlock()
    }

    h.logger.Info("Creating new SSE connection for server: %s", serverName)
    serverConfig, cfgExists := h.Manager.config.Servers[serverName]
    if !cfgExists {
        return nil, fmt.Errorf("configuration for server '%s' not found", serverName)
    }

    newConn, err := h.createSSEConnection(serverName, serverConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to create SSE connection: %w", err)
    }

    h.SSEMutex.Lock()
    if h.SSEConnections == nil {
        h.SSEConnections = make(map[string]*MCPSSEConnection)
    }
    h.SSEConnections[serverName] = newConn
    h.SSEMutex.Unlock()

    return newConn, nil
}

func (h *ProxyHandler) createSSEConnection(serverName string, serverConfig config.ServerConfig) (*MCPSSEConnection, error) {
    baseURL, sseEndpoint := h.getServerSSEURL(serverName, serverConfig)

    conn := &MCPSSEConnection{
        ServerName:   serverName,
        BaseURL:      baseURL,
        SSEEndpoint:  sseEndpoint,
        LastUsed:     time.Now(),
        Healthy:      true,
        Capabilities: make(map[string]interface{}),
        ServerInfo:   make(map[string]interface{}),
    }

    // Initialize SSE connection
    if err := h.initializeSSEConnection(conn); err != nil {
        return nil, fmt.Errorf("failed to initialize SSE connection: %w", err)
    }

    h.logger.Info("Successfully created and initialized SSE connection for %s", serverName)
    return conn, nil
}

func (h *ProxyHandler) getServerSSEURL(serverName string, serverConfig config.ServerConfig) (string, string) {
    targetHost := fmt.Sprintf("mcp-compose-%s", serverName)
    targetPort := serverConfig.HttpPort
    if serverConfig.SSEPort > 0 {
        targetPort = serverConfig.SSEPort
    }

    // Build the base URL
    baseURL := fmt.Sprintf("http://%s:%d", targetHost, targetPort)

    // Determine SSE endpoint - use sse_path if specified
    sseEndpoint := "/sse" // default
    if serverConfig.SSEPath != "" {
        sseEndpoint = serverConfig.SSEPath
    } else if serverConfig.HttpPath != "" {
        sseEndpoint = serverConfig.HttpPath
    }

    if !strings.HasPrefix(sseEndpoint, "/") {
        sseEndpoint = "/" + sseEndpoint
    }

    sseURL := baseURL + sseEndpoint
    h.logger.Debug("Resolved SSE URL for server %s: %s", serverName, sseURL)
    return baseURL, sseURL
}

func (h *ProxyHandler) initializeSSEConnection(conn *MCPSSEConnection) error {
    h.logger.Info("Initializing SSE connection to %s at %s", conn.ServerName, conn.SSEEndpoint)

    // Phase 1: Get session endpoint via GET to /sse
    sessionEndpoint, err := h.getSSESessionEndpoint(conn)
    if err != nil {
        return fmt.Errorf("failed to get SSE session endpoint: %w", err)
    }

    conn.mu.Lock()
    conn.SessionEndpoint = sessionEndpoint
    conn.mu.Unlock()

    h.logger.Info("Got SSE session endpoint for %s: %s", conn.ServerName, sessionEndpoint)

    // Phase 2: Send initialize request - but don't wait for response!
    initRequest := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      h.getNextRequestID(),
        "method":  "initialize",
        "params": map[string]interface{}{
            "protocolVersion": "2024-11-05",
            "capabilities":    map[string]interface{}{},
            "clientInfo": map[string]interface{}{
                "name":    "mcp-compose-proxy",
                "version": "1.0.0",
            },
        },
    }

    // Send initialize but don't wait for response
    err = h.sendSSERequestNoResponse(conn, initRequest)
    if err != nil {
        return fmt.Errorf("failed to send initialize request: %w", err)
    }

    // Phase 3: Send initialized notification
    initializedNotification := map[string]interface{}{
        "jsonrpc": "2.0",
        "method":  "notifications/initialized",
    }

    err = h.sendSSERequestNoResponse(conn, initializedNotification)
    if err != nil {
        h.logger.Warning("Failed to send initialized notification to %s: %v (continuing anyway)", conn.ServerName, err)
    } else {
        h.logger.Debug("Sent initialized notification to %s", conn.ServerName)
    }

    // Give the server a moment to process
    time.Sleep(500 * time.Millisecond)

    conn.mu.Lock()
    conn.Initialized = true
    conn.Healthy = true
    // Set some default capabilities
    conn.Capabilities = map[string]interface{}{
        "tools": map[string]interface{}{},
    }
    conn.ServerInfo = map[string]interface{}{
        "name":    "mcp-cron-oi",
        "version": "1.0.0",
    }
    conn.mu.Unlock()

    h.logger.Info("SSE connection to %s initialized successfully", conn.ServerName)
    return nil
}

func (h *ProxyHandler) sendSSERequestNoResponse(conn *MCPSSEConnection, request map[string]interface{}) error {
    requestData, err := json.Marshal(request)
    if err != nil {
        return fmt.Errorf("failed to marshal request: %w", err)
    }

    conn.mu.Lock()
    sessionEndpoint := conn.SessionEndpoint
    conn.mu.Unlock()

    if sessionEndpoint == "" {
        return fmt.Errorf("no session endpoint available")
    }

    method, _ := request["method"].(string)
    h.logger.Info("Sending SSE %s request to %s: %s", method, conn.ServerName, string(requestData))

    ctx, cancel := context.WithTimeout(h.ctx, 10*time.Second)
    defer cancel()

    httpReq, err := http.NewRequestWithContext(ctx, "POST", sessionEndpoint, bytes.NewBuffer(requestData))
    if err != nil {
        return fmt.Errorf("failed to create session request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := h.httpClient.Do(httpReq)
    if err != nil {
        return fmt.Errorf("session request failed: %w", err)
    }
    defer resp.Body.Close()

    bodyBytes, _ := io.ReadAll(resp.Body)
    h.logger.Info("SSE session response for %s %s (status %d): %s", method, conn.ServerName, resp.StatusCode, string(bodyBytes))

    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
        return fmt.Errorf("unexpected response status %d: %s", resp.StatusCode, string(bodyBytes))
    }

    return nil
}

func (h *ProxyHandler) getSSESessionEndpoint(conn *MCPSSEConnection) (string, error) {
    // Use a context that WON'T be cancelled - we need this connection to stay alive
    ctx, cancel := context.WithCancel(h.ctx)

    httpReq, err := http.NewRequestWithContext(ctx, "GET", conn.SSEEndpoint, nil)
    if err != nil {
        cancel()
        return "", fmt.Errorf("failed to create SSE session request: %w", err)
    }

    httpReq.Header.Set("Accept", "text/event-stream")
    httpReq.Header.Set("Cache-Control", "no-cache")
    httpReq.Header.Set("Connection", "keep-alive")

    resp, err := h.sseClient.Do(httpReq)
    if err != nil {
        cancel()
        return "", fmt.Errorf("SSE session request failed: %w", err)
    }

    if resp.StatusCode != http.StatusOK {
        resp.Body.Close()
        cancel()
        bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return "", fmt.Errorf("SSE session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
    }

    // CRITICAL: Store these and keep them alive
    conn.mu.Lock()
    conn.sseResponse = resp
    conn.sseBody = resp.Body
    conn.sseCancel = cancel
    conn.sseReader = bufio.NewScanner(resp.Body)
    conn.pendingRequests = make(map[interface{}]chan map[string]interface{})
    conn.mu.Unlock()

    // Read the session endpoint from the stream
    sessionEndpoint := ""
    for conn.sseReader.Scan() {
        line := conn.sseReader.Text()
        h.logger.Debug("SSE initial read from %s: %s", conn.ServerName, line)

        if strings.HasPrefix(line, "event: endpoint") {
            // Next line should have the endpoint data
            if conn.sseReader.Scan() {
                dataLine := conn.sseReader.Text()
                if strings.HasPrefix(dataLine, "data: ") {
                    sessionPath := strings.TrimPrefix(dataLine, "data: ")
                    baseURL := strings.TrimSuffix(conn.BaseURL, "/")
                    sessionEndpoint = baseURL + sessionPath
                    break
                }
            }
        }
    }

    if sessionEndpoint == "" {
        h.closeSSEConnection(conn)
        return "", fmt.Errorf("no session endpoint found in SSE stream")
    }

    // Start background reader to handle async responses
    go h.readSSEResponses(conn)

    h.logger.Info("Got SSE session endpoint for %s: %s", conn.ServerName, sessionEndpoint)
    return sessionEndpoint, nil
}

func (h *ProxyHandler) readSSEResponses(conn *MCPSSEConnection) {
    defer func() {
        h.logger.Info("SSE response reader ending for %s", conn.ServerName)
        h.closeSSEConnection(conn)
    }()

    h.logger.Info("Starting SSE response reader for %s", conn.ServerName)
    lineCount := 0

    for {
        conn.mu.Lock()
        reader := conn.sseReader
        conn.mu.Unlock()

        if reader == nil {
            h.logger.Warning("SSE reader is nil for %s", conn.ServerName)
            break
        }

        h.logger.Debug("Waiting for next SSE line from %s (line %d)", conn.ServerName, lineCount)

        if !reader.Scan() {
            if err := reader.Err(); err != nil {
                h.logger.Error("SSE reader scan error for %s: %v", conn.ServerName, err)
            } else {
                h.logger.Info("SSE reader scan returned false (EOF?) for %s", conn.ServerName)
            }
            break
        }

        line := reader.Text()
        lineCount++
        h.logger.Info("SSE response line %d from %s: %q", lineCount, conn.ServerName, line)

        if line == "" {
            h.logger.Debug("Empty line from %s", conn.ServerName)
            continue
        }

        if strings.HasPrefix(line, "event: message") {
            h.logger.Info("Found message event from %s, reading next line", conn.ServerName)
            // Next line should have the message data
            if reader.Scan() {
                lineCount++
                dataLine := reader.Text()
                h.logger.Info("Message data line %d from %s: %q", lineCount, conn.ServerName, dataLine)

                if strings.HasPrefix(dataLine, "data: ") {
                    messageData := strings.TrimPrefix(dataLine, "data: ")
                    h.logger.Info("Processing message data from %s: %s", conn.ServerName, messageData)
                    h.processSSEMessage(conn, messageData)
                } else {
                    h.logger.Warning("Expected data line but got: %q", dataLine)
                }
            } else {
                h.logger.Warning("Could not read data line after message event from %s", conn.ServerName)
            }
        } else {
            h.logger.Debug("Non-message event line from %s: %q", conn.ServerName, line)
        }
    }

    h.logger.Warning("SSE response reader ended for %s after %d lines", conn.ServerName, lineCount)
}

func (h *ProxyHandler) processSSEMessage(conn *MCPSSEConnection, messageData string) {
    h.logger.Info("Processing SSE message for %s: %s", conn.ServerName, messageData)

    var response map[string]interface{}
    if err := json.Unmarshal([]byte(messageData), &response); err != nil {
        h.logger.Error("Failed to parse SSE message JSON for %s: %v. Raw data: %q", conn.ServerName, err, messageData)
        return
    }

    h.logger.Info("Parsed SSE response for %s: %+v", conn.ServerName, response)

    // Check if this is a response to a pending request
    if responseID := response["id"]; responseID != nil {
        h.logger.Info("SSE response has ID %v (type: %T) for %s", responseID, responseID, conn.ServerName)

        conn.reqMutex.Lock()
        defer conn.reqMutex.Unlock()

        // CRITICAL FIX: Try multiple ID formats due to JSON unmarshaling differences
        var matchingChannel chan map[string]interface{}
        var foundKey interface{}

        // Try direct lookup first
        if respCh, exists := conn.pendingRequests[responseID]; exists {
            matchingChannel = respCh
            foundKey = responseID
        } else {
            // Try converting types - JSON might unmarshal numbers differently
            for pendingID, pendingCh := range conn.pendingRequests {
                if fmt.Sprintf("%v", pendingID) == fmt.Sprintf("%v", responseID) {
                    matchingChannel = pendingCh
                    foundKey = pendingID
                    h.logger.Info("Found pending request via string conversion: %v -> %v", pendingID, responseID)
                    break
                }
            }
        }

        if matchingChannel != nil {
            h.logger.Info("Found pending request channel for ID %v to %s", foundKey, conn.ServerName)
            // Send response and remove from pending requests immediately
            select {
            case matchingChannel <- response:
                h.logger.Info("Successfully delivered SSE response for request ID %v to %s", foundKey, conn.ServerName)
                delete(conn.pendingRequests, foundKey) // Clean up immediately
            default:
                h.logger.Warning("Response channel full for request ID %v to %s", foundKey, conn.ServerName)
                delete(conn.pendingRequests, foundKey) // Still clean up
            }
        } else {
            h.logger.Warning("No pending request found for response ID %v (type %T) from %s. Pending requests: %v",
                responseID, responseID, conn.ServerName, getMapKeys(conn.pendingRequests))
        }
    } else {
        h.logger.Info("SSE message without ID from %s (notification?): %s", conn.ServerName, messageData)
    }
}

func (h *ProxyHandler) sendSSERequest(conn *MCPSSEConnection, request map[string]interface{}) (map[string]interface{}, error) {
    return h.sendSSERequestToSession(conn, request)
}

func (h *ProxyHandler) sendSSERequestToSession(conn *MCPSSEConnection, request map[string]interface{}) (map[string]interface{}, error) {
    requestData, err := json.Marshal(request)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    conn.mu.Lock()
    sessionEndpoint := conn.SessionEndpoint
    conn.mu.Unlock()

    if sessionEndpoint == "" {
        return nil, fmt.Errorf("no session endpoint available")
    }

    h.logger.Debug("Sending SSE request to %s session endpoint %s: %s", conn.ServerName, sessionEndpoint, string(requestData))

    // CRITICAL FIX: Set up response channel BEFORE sending request
    requestID := request["id"]
    if requestID != nil {
        respCh := make(chan map[string]interface{}, 1)
        conn.reqMutex.Lock()
        conn.pendingRequests[requestID] = respCh
        conn.reqMutex.Unlock()

        // Track if we've cleaned up to prevent double cleanup
        var cleanupDone bool
        var cleanupMutex sync.Mutex
        cleanup := func() {
            cleanupMutex.Lock()
            defer cleanupMutex.Unlock()
            if cleanupDone {
                return // Already cleaned up
            }
            cleanupDone = true

            conn.reqMutex.Lock()
            delete(conn.pendingRequests, requestID)
            conn.reqMutex.Unlock()

            // Safely close channel without panic
            select {
            case <-respCh:
                // Channel already has data or is closed
            default:
                // Channel is empty, safe to close
                close(respCh)
            }
        }
        defer cleanup()

        // Now send the request
        ctx, cancel := context.WithTimeout(h.ctx, 60*time.Second)
        defer cancel()

        httpReq, err := http.NewRequestWithContext(ctx, "POST", sessionEndpoint, bytes.NewBuffer(requestData))
        if err != nil {
            return nil, fmt.Errorf("failed to create session request: %w", err)
        }

        httpReq.Header.Set("Content-Type", "application/json")

        resp, err := h.httpClient.Do(httpReq)
        if err != nil {
            conn.mu.Lock()
            conn.Healthy = false
            conn.mu.Unlock()
            return nil, fmt.Errorf("session request failed: %w", err)
        }
        defer resp.Body.Close()

        conn.mu.Lock()
        conn.LastUsed = time.Now()
        conn.mu.Unlock()

        // For 202 Accepted, wait for async response via SSE
        if resp.StatusCode == http.StatusAccepted {
            method, _ := request["method"].(string)
            h.logger.Info("Got 202 Accepted for %s, waiting for SSE response...", method)

            // Wait for async response with proper timeout handling
            select {
            case response, ok := <-respCh:
                if !ok {
                    return nil, fmt.Errorf("response channel closed while waiting for %s", method)
                }
                h.logger.Info("Received async SSE response for %s %s", method, conn.ServerName)
                conn.mu.Lock()
                conn.Healthy = true
                conn.mu.Unlock()
                return response, nil
            case <-time.After(60 * time.Second):
                return nil, fmt.Errorf("timeout waiting for SSE response to %s", method)
            case <-h.ctx.Done():
                return nil, h.ctx.Err()
            }
        }

        // For 200 OK, response is in body
        if resp.StatusCode == http.StatusOK {
            conn.mu.Lock()
            conn.Healthy = true
            conn.mu.Unlock()

            var response map[string]interface{}
            if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
                return nil, fmt.Errorf("failed to decode response: %w", err)
            }
            return response, nil
        }

        // Other status codes are errors
        conn.mu.Lock()
        conn.Healthy = false
        conn.mu.Unlock()

        bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
        return nil, fmt.Errorf("session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
    }

    // For requests without ID (notifications), just send and return success
    ctx, cancel := context.WithTimeout(h.ctx, 60*time.Second)
    defer cancel()

    httpReq, err := http.NewRequestWithContext(ctx, "POST", sessionEndpoint, bytes.NewBuffer(requestData))
    if err != nil {
        return nil, fmt.Errorf("failed to create session request: %w", err)
    }

    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := h.httpClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("session request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
        return map[string]interface{}{
            "jsonrpc": "2.0",
            "result":  "accepted",
        }, nil
    }

    bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
    return nil, fmt.Errorf("session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
}

func (h *ProxyHandler) closeSSEConnection(conn *MCPSSEConnection) {
    conn.mu.Lock()
    defer conn.mu.Unlock()

    if conn.sseBody != nil {
        conn.sseBody.Close()
        conn.sseBody = nil
    }

    if conn.sseCancel != nil {
        conn.sseCancel()
        conn.sseCancel = nil
    }

    conn.sseReader = nil
    conn.Healthy = false

    // Close all pending request channels
    for _, ch := range conn.pendingRequests {
        close(ch)
    }
    conn.pendingRequests = make(map[interface{}]chan map[string]interface{})

    h.logger.Debug("Closed SSE connection for %s", conn.ServerName)
}

func (h *ProxyHandler) isSSEConnectionHealthy(conn *MCPSSEConnection) bool {
    if conn == nil {
        return false
    }
    conn.mu.Lock()
    defer conn.mu.Unlock()
    return conn.Healthy && conn.Initialized
}

func (h *ProxyHandler) maintainSSEConnections() {
    h.SSEMutex.Lock()
    defer h.SSEMutex.Unlock()

    for serverName, conn := range h.SSEConnections {
        if conn == nil {
            continue
        }

        maxIdleTime := 15 * time.Minute
        if time.Since(conn.LastUsed) > maxIdleTime {
            h.logger.Info("Removing idle SSE connection to %s (idle for %v)",
                serverName, time.Since(conn.LastUsed))
            h.closeSSEConnection(conn)
            delete(h.SSEConnections, serverName)
        }
    }
}

// Helper function
func getMapKeys(m map[interface{}]chan map[string]interface{}) []interface{} {
    keys := make([]interface{}, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    return keys
}
