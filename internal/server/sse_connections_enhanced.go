package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
)

// EnhancedMCPSSEConnection represents a high-performance Server-Sent Events connection to an MCP server
type EnhancedMCPSSEConnection struct {
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

	// Enhanced connection management
	sseResponse *http.Response
	sseBody     io.ReadCloser
	sseCancel   context.CancelFunc
	sseReader   *bufio.Scanner

	// PERFORMANCE: Use string-keyed map for direct lookups, no type conversion needed
	pendingRequests map[string]chan map[string]interface{}
	reqMutex        sync.RWMutex

	// Stream management
	streamChan   chan map[string]interface{}
	streamMutex  sync.RWMutex
	streamActive bool

	// Connection state
	mu            sync.RWMutex
	nextRequestID int64

	// Metrics
	requestCount  int64
	responseCount int64
	errorCount    int64
}

// getNextRequestID generates a unique string ID for requests
func (conn *EnhancedMCPSSEConnection) getNextRequestID() string {
	conn.mu.Lock()
	defer conn.mu.Unlock()
	conn.nextRequestID++

	return strconv.FormatInt(conn.nextRequestID, 10)
}

func (h *ProxyHandler) getEnhancedSSEConnection(serverName string) (*EnhancedMCPSSEConnection, error) {
	h.SSEMutex.RLock()
	if conn, exists := h.EnhancedSSEConnections[serverName]; exists && h.isEnhancedSSEConnectionHealthy(conn) {
		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
		h.SSEMutex.RUnlock()
		h.logger.Debug("Reusing healthy enhanced SSE connection for %s", serverName)

		return conn, nil
	}
	h.SSEMutex.RUnlock()

	h.logger.Info("Creating new enhanced SSE connection for server: %s", serverName)
	serverConfig, cfgExists := h.Manager.config.Servers[serverName]
	if !cfgExists {

		return nil, fmt.Errorf("configuration for server '%s' not found", serverName)
	}

	newConn, err := h.createEnhancedSSEConnection(serverName, serverConfig)
	if err != nil {

		return nil, fmt.Errorf("failed to create enhanced SSE connection: %w", err)
	}

	h.SSEMutex.Lock()
	if h.EnhancedSSEConnections == nil {
		h.EnhancedSSEConnections = make(map[string]*EnhancedMCPSSEConnection)
	}
	h.EnhancedSSEConnections[serverName] = newConn
	h.SSEMutex.Unlock()

	return newConn, nil
}

func (h *ProxyHandler) createEnhancedSSEConnection(serverName string, serverConfig config.ServerConfig) (*EnhancedMCPSSEConnection, error) {
	baseURL, sseEndpoint := h.getServerSSEURL(serverName, serverConfig)

	conn := &EnhancedMCPSSEConnection{
		ServerName:      serverName,
		BaseURL:         baseURL,
		SSEEndpoint:     sseEndpoint,
		LastUsed:        time.Now(),
		Healthy:         true,
		Capabilities:    make(map[string]interface{}),
		ServerInfo:      make(map[string]interface{}),
		pendingRequests: make(map[string]chan map[string]interface{}),
		streamChan:      make(chan map[string]interface{}, constants.SSEStreamBuffer), // Buffered channel for streaming
		nextRequestID:   0,
	}

	// Initialize SSE connection
	if err := h.initializeEnhancedSSEConnection(conn); err != nil {

		return nil, fmt.Errorf("failed to initialize enhanced SSE connection: %w", err)
	}

	h.logger.Info("Successfully created and initialized enhanced SSE connection for %s", serverName)

	return conn, nil
}

func (h *ProxyHandler) initializeEnhancedSSEConnection(conn *EnhancedMCPSSEConnection) error {
	h.logger.Info("Initializing enhanced SSE connection to %s at %s", conn.ServerName, conn.SSEEndpoint)

	// Phase 1: Get session endpoint via GET to /sse
	sessionEndpoint, err := h.getEnhancedSSESessionEndpoint(conn)
	if err != nil {

		return fmt.Errorf("failed to get SSE session endpoint: %w", err)
	}

	conn.mu.Lock()
	conn.SessionEndpoint = sessionEndpoint
	conn.mu.Unlock()

	h.logger.Info("Got enhanced SSE session endpoint for %s: %s", conn.ServerName, sessionEndpoint)

	// Phase 2: Send initialize request with typed ID
	initRequestID := conn.getNextRequestID()
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      initRequestID,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcp-compose-proxy-enhanced",
				"version": "1.0.0",
			},
		},
	}

	// Send initialize but don't wait for response
	_, err = h.sendEnhancedSSERequestNoResponse(conn, initRequest)
	if err != nil {

		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Phase 3: Send initialized notification
	initializedNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	_, err = h.sendEnhancedSSERequestNoResponse(conn, initializedNotification)
	if err != nil {
		h.logger.Warning("Failed to send initialized notification to %s: %v (continuing anyway)", conn.ServerName, err)
	} else {
		h.logger.Debug("Sent initialized notification to %s", conn.ServerName)
	}

	// Give the server a moment to process
	time.Sleep(constants.PerformanceShortSleep) // Reduced from 500ms for better performance

	conn.mu.Lock()
	conn.Initialized = true
	conn.Healthy = true
	conn.Capabilities = map[string]interface{}{
		"tools": map[string]interface{}{},
	}
	conn.ServerInfo = map[string]interface{}{
		"name":    "mcp-enhanced",
		"version": "1.0.0",
	}
	conn.mu.Unlock()

	h.logger.Info("Enhanced SSE connection to %s initialized successfully", conn.ServerName)

	return nil
}

func (h *ProxyHandler) getEnhancedSSESessionEndpoint(conn *EnhancedMCPSSEConnection) (string, error) {
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
		_ = resp.Body.Close()
		cancel()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, constants.SSEResponseBuffer))

		return "", fmt.Errorf("SSE session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Store connection resources
	conn.mu.Lock()
	conn.sseResponse = resp
	conn.sseBody = resp.Body
	conn.sseCancel = cancel
	conn.sseReader = bufio.NewScanner(resp.Body)
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
		h.closeEnhancedSSEConnection(conn)

		return "", fmt.Errorf("no session endpoint found in SSE stream")
	}

	// Start background reader to handle async responses
	go h.readEnhancedSSEResponses(conn)

	h.logger.Info("Got enhanced SSE session endpoint for %s: %s", conn.ServerName, sessionEndpoint)

	return sessionEndpoint, nil
}

func (h *ProxyHandler) readEnhancedSSEResponses(conn *EnhancedMCPSSEConnection) {
	defer func() {
		h.logger.Info("Enhanced SSE response reader ending for %s", conn.ServerName)
		h.closeEnhancedSSEConnection(conn)
	}()

	h.logger.Info("Starting enhanced SSE response reader for %s", conn.ServerName)
	lineCount := 0

	for {
		conn.mu.RLock()
		reader := conn.sseReader
		conn.mu.RUnlock()

		if reader == nil {
			h.logger.Warning("Enhanced SSE reader is nil for %s", conn.ServerName)

			break
		}

		if !reader.Scan() {
			if err := reader.Err(); err != nil {
				h.logger.Error("Enhanced SSE reader scan error for %s: %v", conn.ServerName, err)
				conn.mu.Lock()
				conn.errorCount++
				conn.mu.Unlock()
			} else {
				h.logger.Info("Enhanced SSE reader scan returned false (EOF?) for %s", conn.ServerName)
			}

			break
		}

		line := reader.Text()
		lineCount++

		if line == "" {

			continue
		}

		if strings.HasPrefix(line, "event: message") {
			// Next line should have the message data
			if reader.Scan() {
				lineCount++
				dataLine := reader.Text()

				if strings.HasPrefix(dataLine, "data: ") {
					messageData := strings.TrimPrefix(dataLine, "data: ")
					h.processEnhancedSSEMessage(conn, messageData)
				}
			}
		}
	}

	h.logger.Info("Enhanced SSE response reader ended for %s after %d lines", conn.ServerName, lineCount)
}

func (h *ProxyHandler) processEnhancedSSEMessage(conn *EnhancedMCPSSEConnection, messageData string) {
	var response map[string]interface{}
	if err := json.Unmarshal([]byte(messageData), &response); err != nil {
		h.logger.Error("Failed to parse enhanced SSE message JSON for %s: %v", conn.ServerName, err)
		conn.mu.Lock()
		conn.errorCount++
		conn.mu.Unlock()

		return
	}

	conn.mu.Lock()
	conn.responseCount++
	conn.mu.Unlock()

	// PERFORMANCE: Direct string lookup, no type conversion needed
	if responseIDInterface := response["id"]; responseIDInterface != nil {
		// Convert response ID to string for consistent lookup
		responseID := fmt.Sprintf("%v", responseIDInterface)

		conn.reqMutex.RLock()
		responseChan, exists := conn.pendingRequests[responseID]
		conn.reqMutex.RUnlock()

		if exists {
			conn.reqMutex.Lock()
			delete(conn.pendingRequests, responseID)
			conn.reqMutex.Unlock()

			// Send response to waiting goroutine
			select {
			case responseChan <- response:
				h.logger.Debug("Successfully delivered enhanced SSE response for request ID %s to %s", responseID, conn.ServerName)
			case <-time.After(1 * time.Second):
				h.logger.Warning("Timeout delivering response for request ID %s to %s", responseID, conn.ServerName)
			}
		} else {
			h.logger.Warning("No pending request found for response ID %s from %s", responseID, conn.ServerName)
		}
	} else {
		// This is a notification or streaming message
		conn.streamMutex.RLock()
		if conn.streamActive {
			select {
			case conn.streamChan <- response:
				h.logger.Debug("Streamed notification from %s", conn.ServerName)
			default:
				h.logger.Warning("Stream channel full for %s, dropping message", conn.ServerName)
			}
		}
		conn.streamMutex.RUnlock()
	}
}

func (h *ProxyHandler) sendEnhancedSSERequest(conn *EnhancedMCPSSEConnection, request map[string]interface{}) (map[string]interface{}, error) {
	requestData, err := json.Marshal(request)
	if err != nil {

		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	conn.mu.RLock()
	sessionEndpoint := conn.SessionEndpoint
	conn.mu.RUnlock()

	if sessionEndpoint == "" {

		return nil, fmt.Errorf("no session endpoint available")
	}

	// PERFORMANCE: Ensure request ID is always a string
	requestID := ""
	if id := request["id"]; id != nil {
		requestID = fmt.Sprintf("%v", id)
	}

	if requestID != "" {
		// Create response channel
		respCh := make(chan map[string]interface{}, 1)

		// PERFORMANCE: Direct string-keyed map insertion, no type conversion
		conn.reqMutex.Lock()
		conn.pendingRequests[requestID] = respCh
		conn.reqMutex.Unlock()

		cleanup := func() {
			conn.reqMutex.Lock()
			delete(conn.pendingRequests, requestID)
			conn.reqMutex.Unlock()
		}
		defer cleanup()

		// Send the request
		ctx, cancel := context.WithTimeout(h.ctx, constants.HTTPRequestTimeout)
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
			conn.errorCount++
			conn.mu.Unlock()

			return nil, fmt.Errorf("session request failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.requestCount++
		conn.mu.Unlock()

		// For 202 Accepted, wait for async response via SSE
		if resp.StatusCode == http.StatusAccepted {
			method, _ := request["method"].(string)
			h.logger.Debug("Got 202 Accepted for %s, waiting for enhanced SSE response...", method)

			select {
			case response, ok := <-respCh:
				if !ok {

					return nil, fmt.Errorf("response channel closed while waiting for %s", method)
				}
				conn.mu.Lock()
				conn.Healthy = true
				conn.mu.Unlock()

				return response, nil
			case <-time.After(constants.HTTPRequestTimeout):

				return nil, fmt.Errorf("timeout waiting for enhanced SSE response to %s", method)
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
		conn.errorCount++
		conn.mu.Unlock()

		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, constants.SSEResponseBuffer))

		return nil, fmt.Errorf("session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// For requests without ID (notifications), just send and return success

	return h.sendEnhancedSSERequestNoResponse(conn, request)
}

func (h *ProxyHandler) sendEnhancedSSERequestNoResponse(conn *EnhancedMCPSSEConnection, request map[string]interface{}) (map[string]interface{}, error) {
	requestData, err := json.Marshal(request)
	if err != nil {

		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	conn.mu.RLock()
	sessionEndpoint := conn.SessionEndpoint
	conn.mu.RUnlock()

	if sessionEndpoint == "" {

		return nil, fmt.Errorf("no session endpoint available")
	}

	ctx, cancel := context.WithTimeout(h.ctx, constants.HTTPQuickTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", sessionEndpoint, bytes.NewBuffer(requestData))
	if err != nil {

		return nil, fmt.Errorf("failed to create session request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		conn.mu.Lock()
		conn.errorCount++
		conn.mu.Unlock()

		return nil, fmt.Errorf("session request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	conn.mu.Lock()
	conn.requestCount++
	conn.mu.Unlock()

	if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {

		return map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  "accepted",
		}, nil
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, constants.SSEResponseBuffer))

	return nil, fmt.Errorf("session request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
}

func (h *ProxyHandler) closeEnhancedSSEConnection(conn *EnhancedMCPSSEConnection) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	if conn.sseBody != nil {
		if err := conn.sseBody.Close(); err != nil {
			h.logger.Warning("Failed to close enhanced SSE body for %s: %v", conn.ServerName, err)
		}
		conn.sseBody = nil
	}

	if conn.sseCancel != nil {
		conn.sseCancel()
		conn.sseCancel = nil
	}

	conn.sseReader = nil
	conn.Healthy = false

	// Close all pending request channels
	conn.reqMutex.Lock()
	for _, ch := range conn.pendingRequests {
		close(ch)
	}
	conn.pendingRequests = make(map[string]chan map[string]interface{})
	conn.reqMutex.Unlock()

	// Close stream channel
	conn.streamMutex.Lock()
	if conn.streamActive {
		close(conn.streamChan)
		conn.streamActive = false
	}
	conn.streamMutex.Unlock()

	h.logger.Debug("Closed enhanced SSE connection for %s", conn.ServerName)
}

func (h *ProxyHandler) isEnhancedSSEConnectionHealthy(conn *EnhancedMCPSSEConnection) bool {
	if conn == nil {

		return false
	}
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	return conn.Healthy && conn.Initialized
}

// GetConnectionStats returns performance metrics for the connection
func (conn *EnhancedMCPSSEConnection) GetConnectionStats() map[string]interface{} {
	conn.mu.RLock()
	defer conn.mu.RUnlock()

	conn.reqMutex.RLock()
	pendingCount := len(conn.pendingRequests)
	conn.reqMutex.RUnlock()

	return map[string]interface{}{
		"server_name":      conn.ServerName,
		"healthy":          conn.Healthy,
		"initialized":      conn.Initialized,
		"last_used":        conn.LastUsed,
		"request_count":    conn.requestCount,
		"response_count":   conn.responseCount,
		"error_count":      conn.errorCount,
		"pending_requests": pendingCount,
	}
}
