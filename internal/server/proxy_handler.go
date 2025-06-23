// internal/server/proxy_handler.go
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/logging"
	"mcpcompose/internal/openapi"
)

// MCPHTTPConnection represents a persistent HTTP connection to an MCP server
type MCPHTTPConnection struct {
	ServerName   string
	BaseURL      string // This is THE MCP Endpoint for the server, e.g., http://mcp-compose-filesystem:8001/
	LastUsed     time.Time
	Initialized  bool
	Healthy      bool
	Capabilities map[string]interface{} // Capabilities reported by the server after its initialize response
	ServerInfo   map[string]interface{} // ServerInfo reported by the server after its initialize response
	SessionID    string                 // To store Mcp-Session-Id received from the server
	mu           sync.Mutex             // To protect access to this connection's state (SessionID, Healthy, Initialized, LastUsed etc.)
}

type MCPSTDIOConnection struct {
	ServerName  string
	Host        string
	Port        int
	Connection  net.Conn
	Reader      *bufio.Reader
	Writer      *bufio.Writer
	LastUsed    time.Time
	Initialized bool
	Healthy     bool
	mu          sync.Mutex
}

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
	sseResponse *http.Response
	sseBody     io.ReadCloser
	sseCancel   context.CancelFunc
	sseReader   *bufio.Scanner

	pendingRequests map[interface{}]chan map[string]interface{}
	reqMutex        sync.Mutex
	mu              sync.Mutex
}

type ProxyHandler struct {
	Manager           *Manager
	ConfigFile        string
	APIKey            string
	EnableAPI         bool
	ProxyStarted      time.Time
	ServerConnections map[string]*MCPHTTPConnection // For HTTP transport
	SSEConnections    map[string]*MCPSSEConnection  // For SSE transport
	StdioConnections  map[string]*MCPSTDIOConnection
	ConnectionMutex   sync.RWMutex
	StdioMutex        sync.RWMutex
	SSEMutex          sync.RWMutex // New mutex for SSE connections
	logger            *logging.Logger
	httpClient        *http.Client // For regular HTTP requests
	sseClient         *http.Client // For SSE connections (no timeout)
	GlobalRequestID   int
	GlobalIDMutex     sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	toolCache         map[string]string
	toolCacheMu       sync.RWMutex
	cacheExpiry       time.Time
	connectionStats   map[string]*ConnectionStats
}

// MCPRequest, MCPResponse, MCPError structs (standard JSON-RPC definitions)
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ConnectionStats struct {
	TotalRequests  int64
	FailedRequests int64
	TimeoutErrors  int64
	LastError      time.Time
	LastSuccess    time.Time
	mu             sync.RWMutex
}

func (h *ProxyHandler) refreshToolCache() {
	h.toolCacheMu.Lock()
	defer h.toolCacheMu.Unlock()

	// Only refresh if cache is expired
	if time.Now().Before(h.cacheExpiry) {
		return
	}

	h.logger.Info("Refreshing tool cache...")
	newCache := make(map[string]string)

	for serverName := range h.Manager.config.Servers {
		tools, err := h.discoverServerTools(serverName)
		if err != nil {
			h.logger.Warning("Failed to discover tools for %s during cache refresh: %v", serverName, err)
			continue
		}

		for _, tool := range tools {
			newCache[tool.Name] = serverName
			h.logger.Debug("Cached tool %s -> %s", tool.Name, serverName)
		}
	}

	h.toolCache = newCache
	h.cacheExpiry = time.Now().Add(5 * time.Minute) // Cache for 5 minutes
	h.logger.Info("Tool cache refreshed with %d tools", len(newCache))
}

func NewProxyHandler(mgr *Manager, configFile, apiKey string) *ProxyHandler {
	ctx, cancel := context.WithCancel(context.Background())

	// Regular HTTP client for short requests
	customTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		MaxConnsPerHost:       20,
		WriteBufferSize:       4096,
		ReadBufferSize:        4096,
	}

	// SSE client with no timeout for persistent connections
	sseTransport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       5 * time.Minute, // Longer for SSE
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		MaxConnsPerHost:       5,
	}

	logLvl := "info"
	if mgr.config != nil && mgr.config.Logging.Level != "" {
		logLvl = mgr.config.Logging.Level
	}

	handler := &ProxyHandler{
		Manager:           mgr,
		ConfigFile:        configFile,
		APIKey:            apiKey,
		EnableAPI:         true,
		ProxyStarted:      time.Now(),
		ServerConnections: make(map[string]*MCPHTTPConnection),
		SSEConnections:    make(map[string]*MCPSSEConnection),
		StdioConnections:  make(map[string]*MCPSTDIOConnection),
		httpClient: &http.Client{
			Transport: customTransport,
			Timeout:   60 * time.Second,
		},
		sseClient: &http.Client{
			Transport: sseTransport,
			// No timeout for SSE connections!
		},
		logger:          logging.NewLogger(logLvl),
		ctx:             ctx,
		cancel:          cancel,
		toolCache:       make(map[string]string),
		cacheExpiry:     time.Now(),
		connectionStats: make(map[string]*ConnectionStats),
	}

	handler.startConnectionMaintenance()
	return handler
}

func (h *ProxyHandler) getNextRequestID() int {
	h.GlobalIDMutex.Lock()
	defer h.GlobalIDMutex.Unlock()
	h.GlobalRequestID++
	return h.GlobalRequestID
}

func (h *ProxyHandler) getServerConnection(serverName string) (*MCPHTTPConnection, error) {
	h.ConnectionMutex.RLock()
	conn, exists := h.ServerConnections[serverName]
	h.ConnectionMutex.RUnlock()

	if exists {
		if h.isConnectionHealthy(conn) {
			conn.mu.Lock()
			conn.LastUsed = time.Now()
			conn.mu.Unlock()
			h.logger.Debug("Reusing healthy connection for %s", serverName)
			return conn, nil
		}
		h.logger.Info("Existing connection for %s found unhealthy or uninitialized. Attempting to recreate.", serverName)
		h.ConnectionMutex.Lock()
		delete(h.ServerConnections, serverName) // Remove bad/stale connection
		h.ConnectionMutex.Unlock()
	}

	h.logger.Info("Creating new HTTP connection for server: %s", serverName)

	serverConfig, cfgExists := h.Manager.config.Servers[serverName]
	if !cfgExists {
		return nil, fmt.Errorf("configuration for server '%s' not found", serverName)
	}
	// Ensure server is configured for HTTP
	if serverConfig.Protocol != "http" && serverConfig.HttpPort == 0 {
		isHTTPInArgs := false
		for _, arg := range serverConfig.Args {
			if strings.Contains(strings.ToLower(arg), "http") || strings.Contains(arg, "--port") {
				isHTTPInArgs = true
				break
			}
		}
		if !isHTTPInArgs {
			return nil, fmt.Errorf("server '%s' is not configured for HTTP transport ('protocol: http' or 'http_port' missing)", serverName)
		}
		h.logger.Warning("Server %s: 'protocol: http' or 'http_port' not explicit in YAML. Relying on command args for HTTP mode configuration.", serverName)
	}

	baseURL := h.getServerHTTPURL(serverName, serverConfig)
	if strings.Contains(baseURL, "INVALID_PORT_CONFIG_ERROR") {
		return nil, fmt.Errorf("cannot create connection for '%s' due to invalid port configuration", serverName)
	}

	newConn := &MCPHTTPConnection{
		ServerName:   serverName,
		BaseURL:      baseURL,
		LastUsed:     time.Now(),
		Healthy:      true,
		Capabilities: make(map[string]interface{}),
		ServerInfo:   make(map[string]interface{}),
	}

	maxRetries := 3
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := h.initializeHTTPConnection(newConn) // This will try to make the 'initialize' call
		if err == nil {
			h.ConnectionMutex.Lock()
			h.ServerConnections[serverName] = newConn
			h.ConnectionMutex.Unlock()
			h.logger.Info("Successfully created and initialized HTTP connection for %s.", serverName)
			return newConn, nil
		}
		lastErr = err
		h.logger.Warning("Initialization attempt %d/%d for %s failed: %v.", attempt, maxRetries, newConn.ServerName, err)
		if attempt < maxRetries {
			var waitDuration time.Duration
			if strings.Contains(strings.ToLower(err.Error()), "connection refused") || strings.Contains(err.Error(), "no such host") {
				waitDuration = time.Duration(attempt*3+2) * time.Second
				h.logger.Info("Server %s might be starting (connection refused), waiting %v before retry %d...", newConn.ServerName, waitDuration, attempt+1)
			} else if strings.Contains(strings.ToLower(err.Error()), "timeout") {
				waitDuration = time.Duration(attempt*2+1) * time.Second
				h.logger.Info("Server %s initialization timed out, waiting %v before retry %d...", newConn.ServerName, waitDuration, attempt+1)
			} else {
				waitDuration = time.Duration(attempt) * time.Second
				h.logger.Info("General initialization error for %s, waiting %v before retry %d...", newConn.ServerName, waitDuration, attempt+1)
			}
			select {
			case <-time.After(waitDuration):
			case <-h.ctx.Done(): // If proxy is shutting down, stop retrying
				return nil, fmt.Errorf("proxy shutting down during connection retry for %s: %w", serverName, h.ctx.Err())
			}
		}
	}
	return nil, fmt.Errorf("failed to establish and initialize HTTP connection for %s after %d attempts: %w", serverName, maxRetries, lastErr)
}

func (h *ProxyHandler) initializeHTTPConnection(conn *MCPHTTPConnection) error {
	conn.mu.Lock()
	conn.Initialized = false
	conn.SessionID = ""
	conn.mu.Unlock()

	h.logger.Info("Attempting to initialize HTTP MCP session for %s at %s", conn.ServerName, conn.BaseURL)

	requestID := h.getNextRequestID()
	initRequestPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      requestID,
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"clientInfo": map[string]interface{}{
				"name":    "mcp-compose-proxy",
				"version": "1.1.0", // TODO: Use actual mcp-compose version
			},
			"capabilities": map[string]interface{}{ /* Proxy's client capabilities */ },
		},
	}

	// doHTTPRequest performs the HTTP POST and returns the *http.Response
	resp, err := h.doHTTPRequest(conn, initRequestPayload, 90*time.Second) // 90s for init
	if err != nil {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("HTTP initialize POST to %s failed: %w", conn.BaseURL, err)
	}
	defer resp.Body.Close() // Ensure body is closed after processing

	rawResponseData, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("failed to read initialize response body from %s: %w", conn.BaseURL, readErr)
	}
	h.logger.Debug("Raw Initialize HTTP response from %s (Status: %d, Content-Type: %s): %s", conn.ServerName, resp.StatusCode, resp.Header.Get("Content-Type"), string(rawResponseData))

	conn.mu.Lock()
	serverSessionID := resp.Header.Get("Mcp-Session-Id")
	if serverSessionID != "" {
		conn.SessionID = serverSessionID
		h.logger.Info("Server %s provided Mcp-Session-Id: %s", conn.ServerName, conn.SessionID)
	}
	conn.mu.Unlock()

	if resp.StatusCode != http.StatusOK {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("HTTP initialize request to %s failed with status %d: %s", conn.BaseURL, resp.StatusCode, string(rawResponseData))
	}

	contentType := resp.Header.Get("Content-Type")
	var responseJSONData []byte

	if strings.HasPrefix(contentType, "text/event-stream") {
		h.logger.Info("Server %s responded with text/event-stream for initialize. Reading first 'data:' event.", conn.ServerName)
		scanner := bufio.NewScanner(bytes.NewReader(rawResponseData))
		eventDataFound := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data:") {
				responseJSONData = []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
				eventDataFound = true
				break
			}
		}
		if !eventDataFound {
			conn.mu.Lock()
			conn.Healthy = false
			conn.mu.Unlock()
			return fmt.Errorf("SSE stream from %s for initialize, but no 'data:' event parsed. Body: %s", conn.ServerName, string(rawResponseData))
		}
	} else if strings.HasPrefix(contentType, "application/json") {
		responseJSONData = rawResponseData
	} else {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("unexpected Content-Type '%s' from %s for initialize. Body: %s", contentType, conn.ServerName, string(rawResponseData))
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(responseJSONData, &responseMap); err != nil {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("failed to parse initialize JSON from %s (Content-Type: %s): %w. Data: %s", conn.ServerName, contentType, err, string(responseJSONData))
	}
	h.logger.Debug("Parsed Initialize JSON response from %s: %+v", conn.ServerName, responseMap)

	if errResp, ok := responseMap["error"].(map[string]interface{}); ok {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("initialize error from %s: code=%v, message=%v, data=%v",
			conn.ServerName, errResp["code"], errResp["message"], errResp["data"])
	}

	result, ok := responseMap["result"].(map[string]interface{})
	if !ok {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return fmt.Errorf("initialize response from %s missing 'result' or not an object. Parsed: %+v", conn.ServerName, responseMap)
	}

	conn.mu.Lock()
	if caps, ok := result["capabilities"].(map[string]interface{}); ok {
		conn.Capabilities = caps
	}
	if sInfo, ok := result["serverInfo"].(map[string]interface{}); ok {
		conn.ServerInfo = sInfo
	}
	conn.Initialized = true
	conn.Healthy = true
	conn.mu.Unlock()

	initializedNotificationPayload := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialized",
		"params":  map[string]interface{}{},
	}
	if err := h.sendHTTPNotification(conn, initializedNotificationPayload); err != nil {
		h.logger.Warning("Failed to send 'initialized' notification to %s: %v. Session continues.", conn.ServerName, err)
	}

	h.logger.Info("HTTP MCP session initialized successfully for %s.", conn.ServerName)
	return nil
}

// doHTTPRequest performs HTTP POST, returns *http.Response. Caller must close body.
func (h *ProxyHandler) doHTTPRequest(conn *MCPHTTPConnection, requestPayload map[string]interface{}, timeout time.Duration) (*http.Response, error) {
	requestData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal request for %s: %w", conn.ServerName, err)
	}

	targetURL := conn.BaseURL
	h.logger.Debug("Request to %s (%s): %s", conn.ServerName, targetURL, string(requestData))

	reqCtx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()
	// N.B.: defer cancel() here would cancel *before* http.Client.Do could finish if timeout is shorter
	// than Do's internal processing. http.Client handles context cancellation.

	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", targetURL, bytes.NewBuffer(requestData))
	if err != nil {
		cancel() // important to cancel if NewRequestWithContext fails before Do
		return nil, fmt.Errorf("create HTTP request for %s: %w", conn.ServerName, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	conn.mu.Lock()
	sessionIDForRequest := conn.SessionID
	conn.mu.Unlock()

	if sessionIDForRequest != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionIDForRequest)
	}

	resp, err := h.httpClient.Do(httpReq) // httpClient's own timeout might also apply
	if err != nil {
		cancel() // Ensure context is cancelled if Do fails
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return nil, fmt.Errorf("HTTP POST to %s failed: %w", targetURL, err)
	}
	// If Do is successful, don't cancel context here, caller needs resp.Body

	conn.mu.Lock()
	conn.LastUsed = time.Now()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		// Only mark healthy on unequivocally good status. Other statuses might be protocol errors.
		conn.Healthy = true
	}
	conn.mu.Unlock()
	return resp, nil
}

func (h *ProxyHandler) sendHTTPJsonRequest(conn *MCPHTTPConnection, requestPayload map[string]interface{}, timeout time.Duration) (map[string]interface{}, error) {
	requestData, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, fmt.Errorf("marshal request for %s: %w", conn.ServerName, err)
	}

	targetURL := conn.BaseURL
	h.logger.Debug("Request to %s (%s): %s", conn.ServerName, targetURL, string(requestData))

	reqCtx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()

	// For socat STDIO servers, we send raw JSON to the TCP port, not HTTP POST
	// But your current socat setup expects HTTP POST, so we'll use HTTP
	httpReq, err := http.NewRequestWithContext(reqCtx, "POST", targetURL, bytes.NewBuffer(requestData))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request for %s: %w", conn.ServerName, err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	conn.mu.Lock()
	sessionIDForRequest := conn.SessionID
	conn.mu.Unlock()

	if sessionIDForRequest != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionIDForRequest)
	}

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return nil, fmt.Errorf("HTTP POST to %s failed: %w", targetURL, err)
	}
	defer resp.Body.Close()

	conn.mu.Lock()
	conn.LastUsed = time.Now()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		conn.Healthy = true
	}
	conn.mu.Unlock()

	// Handle session ID updates
	newSessionID := resp.Header.Get("Mcp-Session-Id")
	if newSessionID != "" {
		conn.mu.Lock()
		if newSessionID != conn.SessionID {
			h.logger.Info("Server %s updated Mcp-Session-Id from '%s' to '%s'", conn.ServerName, conn.SessionID, newSessionID)
			conn.SessionID = newSessionID
		}
		conn.mu.Unlock()
	}

	// Check for non-OK status
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("HTTP request to %s failed with status %d: %s", targetURL, resp.StatusCode, string(bodyBytes))
	}

	// Read and parse response
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response from %s: %w", targetURL, err)
	}

	h.logger.Debug("Raw response from %s: %s", conn.ServerName, string(responseData))

	var responseMap map[string]interface{}
	if err := json.Unmarshal(responseData, &responseMap); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response from %s: %w. Data: %s", targetURL, err, string(responseData))
	}

	return responseMap, nil
}

func (h *ProxyHandler) sendHTTPNotification(conn *MCPHTTPConnection, notificationPayload map[string]interface{}) error {
	resp, err := h.doHTTPRequest(conn, notificationPayload, 20*time.Second) // 20s for notifications
	if err != nil {
		return fmt.Errorf("sending notification to %s failed: %w", conn.ServerName, err)
	}
	defer resp.Body.Close()

	// Per Streamable HTTP spec, notifications should get HTTP 202 Accepted. 200 OK is also common.
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		h.logger.Warning("HTTP notification to %s received non-202/200 status %d: %s", conn.ServerName, resp.StatusCode, string(bodyBytes))
	} else {
		h.logger.Debug("HTTP notification to %s sent, status %d.", conn.ServerName, resp.StatusCode)
	}
	return nil
}

func (h *ProxyHandler) isConnectionHealthy(conn *MCPHTTPConnection) bool {
	conn.mu.Lock()
	if !conn.Healthy {
		conn.mu.Unlock()
		h.logger.Debug("Skipping health check for %s; already marked unhealthy.", conn.ServerName)
		return false
	}
	conn.mu.Unlock()

	pingRequestMap := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "ping",
	}

	h.logger.Debug("Performing health check ping to %s at %s", conn.ServerName, conn.BaseURL)
	_, err := h.sendHTTPJsonRequest(conn, pingRequestMap, 30*time.Second) // Uses sendHTTPJsonRequest
	if err != nil {
		h.logger.Warning("Health check ping to %s FAILED: %v", conn.ServerName, err)
		conn.mu.Lock()
		conn.Healthy = false
		conn.mu.Unlock()
		return false
	}

	h.logger.Debug("Health check ping to %s SUCCEEDED.", conn.ServerName)
	// Note: sendHTTPJsonRequest already marks healthy on success.
	return true
}

func (h *ProxyHandler) Shutdown() error {
	h.logger.Info("Shutting down proxy handler...")

	if h.cancel != nil {
		h.cancel()
	}

	// Close HTTP client connections
	h.httpClient.CloseIdleConnections()

	// Close HTTP connections
	h.ConnectionMutex.Lock()
	for name := range h.ServerConnections {
		h.logger.Debug("Cleaning up HTTP connection to server %s", name)
	}
	h.ServerConnections = make(map[string]*MCPHTTPConnection)
	h.ConnectionMutex.Unlock()

	// Close SSE connections
	h.SSEMutex.Lock()
	for name := range h.SSEConnections {
		h.logger.Debug("Cleaning up SSE connection to server %s", name)
	}
	h.SSEConnections = make(map[string]*MCPSSEConnection)
	h.SSEMutex.Unlock()

	// Close STDIO connections
	h.StdioMutex.Lock()
	for name, conn := range h.StdioConnections {
		if conn != nil && conn.Connection != nil {
			h.logger.Debug("Closing STDIO connection to server %s", name)
			conn.Connection.Close()
		}
	}
	h.StdioConnections = make(map[string]*MCPSTDIOConnection)
	h.StdioMutex.Unlock()

	// Clear tool cache
	h.toolCacheMu.Lock()
	h.toolCache = make(map[string]string)
	h.cacheExpiry = time.Now()
	h.toolCacheMu.Unlock()

	// Wait for goroutines
	h.wg.Wait()

	h.logger.Info("Proxy handler shutdown complete.")
	return nil
}

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	h.logger.Info("Request: %s %s from %s (User-Agent: %s)", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("User-Agent"))

	// CORS Headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Authentication check
	var apiKeyToCheck string
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.ProxyAuth.Enabled {
		apiKeyToCheck = h.Manager.config.ProxyAuth.APIKey
	}
	if h.APIKey != "" {
		apiKeyToCheck = h.APIKey
	}

	if apiKeyToCheck != "" {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiKeyToCheck {
			h.logger.Warning("Unauthorized access attempt to %s from %s (API key mismatch)", r.URL.Path, r.RemoteAddr)
			h.corsError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)

	// Handle server-specific OpenAPI specs (like MCPO does)
	if len(parts) >= 2 && parts[1] == "openapi.json" {
		serverName := parts[0]
		if _, exists := h.Manager.config.Servers[serverName]; exists {
			h.handleServerOpenAPISpec(w, r, serverName)
			h.logger.Debug("Processed server OpenAPI spec %s %s in %v", r.Method, r.URL.Path, time.Since(start))
			return
		}
	}

	// Handle server-specific docs
	if len(parts) >= 2 && parts[1] == "docs" {
		serverName := parts[0]
		if _, exists := h.Manager.config.Servers[serverName]; exists {
			h.handleServerDocs(w, r, serverName)
			h.logger.Debug("Processed server docs %s %s in %v", r.Method, r.URL.Path, time.Since(start))
			return
		}
	}

	var handled bool
	if h.EnableAPI {
		switch path {
		case "/api/reload":
			h.handleAPIReload(w, r)
			handled = true
		case "/api/servers":
			h.handleAPIServers(w, r)
			handled = true
		case "/api/status":
			h.handleAPIStatus(w, r)
			handled = true
		case "/api/discovery":
			h.handleDiscoveryEndpoint(w, r)
			handled = true
		case "/api/connections":
			h.handleConnectionsAPI(w, r)
			handled = true
		case "/openapi.json":
			h.handleOpenAPISpec(w, r)
			handled = true
		}
	}

	if handled {
		h.logger.Debug("Processed API request %s %s in %v", r.Method, r.URL.Path, time.Since(start))
		return
	}

	// Handle direct tool calls
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodPost {
		toolName := parts[0]
		if h.isKnownTool(toolName) {
			h.handleDirectToolCall(w, r, toolName)
			h.logger.Debug("Processed direct tool call %s %s in %v", r.Method, r.URL.Path, time.Since(start))
			return
		}
	}

	if path == "/" {
		h.handleIndex(w, r)
	} else if len(parts) > 0 && parts[0] != "api" {
		serverName := parts[0]
		if instance, exists := h.Manager.GetServerInstance(serverName); exists {
			if r.Method == http.MethodPost {
				h.handleServerForward(w, r, serverName, instance)
			} else if r.Method == http.MethodGet && (len(parts) == 1 || (len(parts) > 1 && strings.HasSuffix(parts[1], ".json"))) {
				h.handleServerDetails(w, r, serverName, instance)
			} else if r.Method == http.MethodDelete && len(parts) == 1 && r.Header.Get("Mcp-Session-Id") != "" {
				h.handleSessionTermination(w, r, serverName)
			} else {
				h.logger.Warning("Method %s not allowed for /%s", r.Method, serverName)
				h.corsError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			}
		} else {
			h.logger.Warning("Requested server '%s' not found in config.", serverName)
			h.corsError(w, "Server Not Found", http.StatusNotFound)
		}
	} else {
		h.logger.Warning("Path not found: %s", r.URL.Path)
		h.corsError(w, "Not Found", http.StatusNotFound)
	}
	h.logger.Info("Processed request %s %s (%s) in %v", r.Method, r.URL.Path, path, time.Since(start))
}

func (h *ProxyHandler) handleAPIReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.corsError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clear connection cache and reload config
	h.ConnectionMutex.Lock()
	h.ServerConnections = make(map[string]*MCPHTTPConnection)
	h.ConnectionMutex.Unlock()

	// Refresh tool cache
	h.toolCacheMu.Lock()
	h.cacheExpiry = time.Now() // Force cache refresh
	h.toolCacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
}

func (h *ProxyHandler) corsError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Content-Type")
	http.Error(w, message, code)
}

func (h *ProxyHandler) handleServerOpenAPISpec(w http.ResponseWriter, _ *http.Request, serverName string) {
	h.logger.Info("Generating OpenAPI spec for server: %s", serverName)

	// Create server-specific OpenAPI spec
	schema := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       fmt.Sprintf("%s MCP Server", strings.Title(serverName)),
			"description": fmt.Sprintf("%s MCP Server\n\n- [back to tool list](/docs)", serverName),
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{
				"url":         "http://192.168.86.201:9876",
				"description": serverName + " MCP Server\n\n- [back to tool list](/docs)"},
		},
		"paths": map[string]interface{}{},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"HTTPBearer": map[string]interface{}{
					"type":   "http",
					"scheme": "bearer",
				},
			},
			"schemas": map[string]interface{}{
				"ValidationError": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"detail": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
		},
		"security": []map[string][]string{{"HTTPBearer": {}}},
	}

	paths := make(map[string]interface{})

	// Get tools for this specific server only
	tools, err := h.discoverServerTools(serverName)
	if err != nil {
		h.logger.Warning("Failed to discover tools for %s: %v", serverName, err)
		// Return empty spec but still valid
		schema["paths"] = paths
	} else {
		h.logger.Info("Discovered %d tools for server %s", len(tools), serverName)

		// Add tools for this server
		for _, tool := range tools {
			toolPath := fmt.Sprintf("/%s", tool.Name)

			paths[toolPath] = map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     strings.Title(strings.ReplaceAll(tool.Name, "_", " ")),
					"description": tool.Description,
					"operationId": tool.Name,
					"tags":        []string{"default"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": tool.Parameters,
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful Response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
						"422": map[string]interface{}{
							"description": "Validation Error",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ValidationError",
									},
								},
							},
						},
					},
					"security": []map[string][]string{{"HTTPBearer": {}}},
				},
			}
		}
		schema["paths"] = paths
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(schema); err != nil {
		h.logger.Error("Failed to encode server OpenAPI spec for %s: %v", serverName, err)
		h.corsError(w, "Internal server error", http.StatusInternalServerError)
	} else {
		h.logger.Info("Successfully generated OpenAPI spec for server %s with %d paths", serverName, len(paths))
	}
}

func (h *ProxyHandler) handleServerDocs(w http.ResponseWriter, _ *http.Request, serverName string) {
	h.logger.Debug("Serving docs for server: %s", serverName)

	docsHTML := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s MCP Server</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 40px; line-height: 1.6; }
        .container { max-width: 800px; margin: 0 auto; }
        h1 { color: #2c3e50; border-bottom: 2px solid #3498db; padding-bottom: 10px; }
        .link-box { background: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0; }
        .link-box a { color: #2980b9; text-decoration: none; font-weight: 500; }
        .link-box a:hover { text-decoration: underline; }
        .back-link { margin-top: 30px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>%s MCP Server</h1>
        <p>This is the documentation page for the <strong>%s</strong> MCP server.</p>
        
        <div class="link-box">
            <h3>OpenAPI Specification</h3>
            <p><a href="/%s/openapi.json">View OpenAPI Spec (JSON)</a></p>
            <p>Use this URL in OpenWebUI tools configuration:</p>
            <code>http://192.168.86.201:9876/%s/openapi.json</code>
        </div>
        
        <div class="back-link">
            <p><a href="/">← Back to main proxy dashboard</a></p>
        </div>
    </div>
</body>
</html>`, serverName, serverName, serverName, serverName, serverName)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(docsHTML))
}

// mcpResponseRecorder captures HTTP responses for MCP tool calls
type mcpResponseRecorder struct {
	statusCode int
	body       []byte
	headers    http.Header
}

func (r *mcpResponseRecorder) Header() http.Header {
	if r.headers == nil {
		r.headers = make(http.Header)
	}
	return r.headers
}

func (r *mcpResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *mcpResponseRecorder) Write(body []byte) (int, error) {
	r.body = append(r.body, body...)
	return len(body), nil
}

func (h *ProxyHandler) discoverServerTools(serverName string) ([]openapi.ToolSpec, error) {
	h.logger.Info("Discovering tools from server %s via internal proxy methods", serverName)

	// Create tools/list request
	toolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "tools/list",
	}

	// Check if server exists
	if _, exists := h.Manager.GetServerInstance(serverName); !exists {
		h.logger.Warning("Server instance %s not found, using generic fallback", serverName)
		return h.getGenericToolForServer(serverName), nil
	}

	serverConfig := h.Manager.config.Servers[serverName]

	// Determine the transport protocol
	protocol := serverConfig.Protocol
	if protocol == "" {
		protocol = "stdio" // default
	}

	// Route based on protocol
	h.logger.Info("Server %s using protocol: %s", serverName, protocol)

	// Retry logic with exponential backoff
	maxRetries := 3
	baseTimeout := 10 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		h.logger.Debug("Tool discovery attempt %d/%d for server %s (protocol: %s)", attempt, maxRetries, serverName, protocol)
		timeout := time.Duration(attempt) * baseTimeout // 10s, 20s, 30s

		var response map[string]interface{}
		var err error

		switch protocol {
		case "sse":
			// Use SSE discovery
			response, err = h.sendSSEToolsRequestWithRetry(serverName, toolsRequest, timeout, attempt)
		case "http":
			// Use HTTP discovery
			response, err = h.sendHTTPToolsRequestWithRetry(serverName, toolsRequest, timeout, attempt)
		case "stdio":
			if serverConfig.StdioHosterPort > 0 {
				// Use socat TCP connection
				containerName := fmt.Sprintf("mcp-compose-%s", serverName)
				socatHost := containerName
				socatPort := serverConfig.StdioHosterPort
				response, err = h.sendRawTCPRequestWithRetry(socatHost, socatPort, toolsRequest, timeout, attempt)
			} else {
				// STDIO server - skip for now and use generic
				h.logger.Warning("Direct STDIO server %s tool discovery not implemented, using generic fallback", serverName)
				return h.getGenericToolForServer(serverName), nil
			}
		default:
			h.logger.Warning("Unknown protocol %s for server %s, using generic fallback", protocol, serverName)
			return h.getGenericToolForServer(serverName), nil
		}

		if err == nil {
			// Success - parse and return tools
			specs, parseErr := h.parseToolsResponse(serverName, response)
			if parseErr == nil && len(specs) > 0 {
				toolNames := make([]string, len(specs))
				for i, spec := range specs {
					toolNames[i] = spec.Name
				}
				h.logger.Info("Successfully discovered %d tools from %s: %v", len(specs), serverName, toolNames)
				return specs, nil
			}
			if parseErr != nil {
				h.logger.Warning("Failed to parse tools response from %s on attempt %d: %v", serverName, attempt, parseErr)
				err = parseErr
			}
		}

		// Log the failure and decide whether to retry
		isTimeout := strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout")
		isConnectionError := strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")

		if attempt < maxRetries && (isTimeout || isConnectionError) {
			waitTime := time.Duration(attempt*2) * time.Second // 2s, 4s wait between retries
			h.logger.Warning("Tool discovery attempt %d/%d failed for %s (%v), retrying in %v", attempt, maxRetries, serverName, err, waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Final attempt failed or non-retryable error
		h.logger.Warning("Tool discovery failed for %s after %d attempts: %v, using generic fallback", serverName, attempt, err)
		break
	}

	// All retries failed, use generic fallback
	return h.getGenericToolForServer(serverName), fmt.Errorf("failed to discover tools after %d attempts", maxRetries)
}

func (h *ProxyHandler) sendSSEToolsRequestWithRetry(serverName string, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	h.logger.Debug("Attempting SSE request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)

	conn, connErr := h.getSSEConnection(serverName)
	if connErr != nil {
		return nil, connErr
	}

	return h.sendSSERequest(conn, requestPayload)
}

func (h *ProxyHandler) parseToolsResponse(serverName string, response map[string]interface{}) ([]openapi.ToolSpec, error) {
	h.logger.Debug("Parsing tools response for %s: %v", serverName, response)

	// Check for JSON-RPC error
	if errResp, ok := response["error"].(map[string]interface{}); ok {
		return nil, fmt.Errorf("server returned error: %v", errResp)
	}

	// Parse the tools from the response
	var specs []openapi.ToolSpec
	if result, ok := response["result"].(map[string]interface{}); ok {
		h.logger.Debug("Found result object for %s: %v", serverName, result)

		if tools, ok := result["tools"].([]interface{}); ok {
			h.logger.Debug("Found tools array for %s with %d tools", serverName, len(tools))

			for i, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					spec := openapi.ToolSpec{Type: "function"}
					if name, ok := toolMap["name"].(string); ok {
						spec.Name = name
					} else {
						h.logger.Warning("Tool %d in %s missing name field: %v", i, serverName, toolMap)
						continue
					}

					if desc, ok := toolMap["description"].(string); ok {
						spec.Description = desc
					} else {
						spec.Description = fmt.Sprintf("Tool from %s server", serverName)
					}

					if inputSchema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
						spec.Parameters = inputSchema
					} else {
						spec.Parameters = map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
							"required":   []string{},
						}
					}
					specs = append(specs, spec)
				} else {
					h.logger.Warning("Tool %d in %s is not a map: %v", i, serverName, tool)
				}
			}
		} else {
			h.logger.Warning("No 'tools' array found in result for %s. Result keys: %v", serverName, getKeys(result))
		}
	} else {
		h.logger.Warning("No 'result' object found in response for %s. Response keys: %v", serverName, getKeys(response))
	}

	h.logger.Debug("Parsed %d tools for %s: %v", len(specs), serverName, getToolNames(specs))

	if len(specs) == 0 {
		return nil, fmt.Errorf("no tools found in response")
	}
	return specs, nil
}

// Helper functions for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getToolNames(specs []openapi.ToolSpec) []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}

func (h *ProxyHandler) createFreshStdioConnection(serverName string, timeout time.Duration) (*MCPSTDIOConnection, error) {
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	port := serverConfig.StdioHosterPort
	address := fmt.Sprintf("%s:%d", containerName, port)

	// Use the specified timeout for connection
	var d net.Dialer
	ctx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()

	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Set up the connection but don't store it in the main connection pool
	conn := &MCPSTDIOConnection{
		ServerName:  serverName,
		Host:        containerName,
		Port:        port,
		Connection:  netConn,
		Reader:      bufio.NewReaderSize(netConn, 8192),
		Writer:      bufio.NewWriterSize(netConn, 8192),
		LastUsed:    time.Now(),
		Healthy:     true,
		Initialized: false,
	}

	// Quick initialization for tool discovery
	if err := h.quickInitializeStdioConnection(conn, timeout); err != nil {
		conn.Connection.Close()
		return nil, fmt.Errorf("failed to initialize connection: %w", err)
	}

	return conn, nil
}

func (h *ProxyHandler) quickInitializeStdioConnection(conn *MCPSTDIOConnection, timeout time.Duration) error {
	// Set deadline for entire initialization
	deadline := time.Now().Add(timeout)
	conn.Connection.SetWriteDeadline(deadline)
	conn.Connection.SetReadDeadline(deadline)
	defer func() {
		conn.Connection.SetWriteDeadline(time.Time{})
		conn.Connection.SetReadDeadline(time.Time{})
	}()

	// Send initialize request
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

	if err := h.sendStdioRequestWithoutLock(conn, initRequest); err != nil {
		return err
	}

	response, err := h.readStdioResponseWithoutLock(conn)
	if err != nil {
		return err
	}

	if mcpError, hasError := response["error"]; hasError {
		return fmt.Errorf("initialize failed: %v", mcpError)
	}

	conn.Initialized = true
	conn.Healthy = true
	return nil
}

func (h *ProxyHandler) sendStdioRequestWithTimeout(conn *MCPSTDIOConnection, request map[string]interface{}, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	conn.Connection.SetWriteDeadline(deadline)
	defer conn.Connection.SetWriteDeadline(time.Time{})

	return h.sendStdioRequestWithoutLock(conn, request)
}

func (h *ProxyHandler) readStdioResponseWithTimeout(conn *MCPSTDIOConnection, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	conn.Connection.SetReadDeadline(deadline)
	defer conn.Connection.SetReadDeadline(time.Time{})

	return h.readStdioResponseWithoutLock(conn)
}

func (h *ProxyHandler) sendRawTCPRequestWithRetry(host string, port int, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	// Find server name for connection tracking
	var serverName string
	for name, config := range h.Manager.config.Servers {
		containerName := fmt.Sprintf("mcp-compose-%s", name)
		if containerName == host && config.StdioHosterPort == port {
			serverName = name
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("could not identify server for host %s:%d", host, port)
	}

	h.logger.Debug("Attempting TCP request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)

	// For tool discovery, create a fresh connection each time to avoid stale connection issues
	conn, err := h.createFreshStdioConnection(serverName, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	defer func() {
		if conn.Connection != nil {
			conn.Connection.Close()
		}
	}()

	// Send request with the specified timeout
	if err := h.sendStdioRequestWithTimeout(conn, requestPayload, timeout); err != nil {
		return nil, err
	}

	// Read response with the specified timeout
	return h.readStdioResponseWithTimeout(conn, timeout)
}

func (h *ProxyHandler) sendHTTPToolsRequestWithRetry(serverName string, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	h.logger.Debug("Attempting HTTP request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)

	conn, connErr := h.getServerConnection(serverName)
	if connErr != nil {
		return nil, connErr
	}

	return h.sendHTTPJsonRequest(conn, requestPayload, timeout)
}

// Generic fallback that works with any MCP server
func (h *ProxyHandler) getGenericToolForServer(serverName string) []openapi.ToolSpec {
	return []openapi.ToolSpec{
		{
			Type:        "function",
			Name:        fmt.Sprintf("%s_execute", serverName),
			Description: fmt.Sprintf("Execute any command on %s MCP server", serverName),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"method": map[string]interface{}{
						"type":        "string",
						"description": "MCP method to call (e.g., tools/call, prompts/list, resources/list)",
					},
					"params": map[string]interface{}{
						"type":                 "object",
						"description":          "Parameters for the MCP method",
						"additionalProperties": true,
					},
				},
				"required": []string{"method"},
			},
		},
	}
}

func (h *ProxyHandler) handleOpenAPISpec(w http.ResponseWriter, r *http.Request) {
	// Authentication code
	apiKeyToCheck := h.APIKey
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.ProxyAuth.Enabled {
		apiKeyToCheck = h.Manager.config.ProxyAuth.APIKey
	}

	if apiKeyToCheck != "" {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiKeyToCheck {
			w.Header().Set("WWW-Authenticate", "Bearer")
			h.corsError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// Create FastAPI-compatible OpenAPI spec
	schema := map[string]interface{}{
		"openapi": "3.1.0",
		"info": map[string]interface{}{
			"title":       "MCP Server Functions",
			"description": "Automatically generated API from MCP Tool Schemas",
			"version":     "1.0.0",
		},
		"servers": []map[string]interface{}{
			{
				"url":         "http://192.168.86.201:9876",
				"description": "MCP Proxy Server",
			},
		},
		"paths": map[string]interface{}{},
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"HTTPBearer": map[string]interface{}{
					"type":   "http",
					"scheme": "bearer",
				},
			},
			"schemas": map[string]interface{}{
				"ValidationError": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"detail": map[string]interface{}{
							"type": "array",
							"items": map[string]interface{}{
								"type": "object",
							},
						},
					},
				},
			},
		},
		"security": []map[string][]string{
			{"HTTPBearer": {}},
		},
	}

	paths := make(map[string]interface{})

	// Discover tools from each server and create endpoints
	for serverName := range h.Manager.config.Servers {
		tools, err := h.discoverServerTools(serverName)
		if err != nil {
			h.logger.Warning("Failed to discover tools for %s: %v", serverName, err)
			continue
		}

		for _, tool := range tools {
			toolPath := fmt.Sprintf("/%s", tool.Name)

			// Create FastAPI-style endpoint
			paths[toolPath] = map[string]interface{}{
				"post": map[string]interface{}{
					"summary":     strings.Title(strings.ReplaceAll(tool.Name, "_", " ")),
					"description": tool.Description,
					"operationId": tool.Name,
					"tags":        []string{"default"},
					"requestBody": map[string]interface{}{
						"required": true,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": tool.Parameters,
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Successful Response",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"type": "object",
									},
								},
							},
						},
						"422": map[string]interface{}{
							"description": "Validation Error",
							"content": map[string]interface{}{
								"application/json": map[string]interface{}{
									"schema": map[string]interface{}{
										"$ref": "#/components/schemas/ValidationError",
									},
								},
							},
						},
					},
					"security": []map[string][]string{
						{"HTTPBearer": {}},
					},
				},
			}
		}
	}

	schema["paths"] = paths

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(schema); err != nil {
		h.logger.Error("Failed to encode OpenAPI spec: %v", err)
	}
}

func (h *ProxyHandler) handleServerForward(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance) {
	w.Header().Set("Content-Type", "application/json")

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body for %s: %v", serverName, err)
		h.sendMCPError(w, nil, -32700, "Error reading request body")
		return
	}

	// Parse JSON payload
	var requestPayload map[string]interface{}
	if err := json.Unmarshal(body, &requestPayload); err != nil {
		h.logger.Error("Invalid JSON in request for %s: %v. Body: %s", serverName, err, string(body))
		h.sendMCPError(w, nil, -32700, "Invalid JSON in request")
		return
	}

	reqIDVal := requestPayload["id"]
	reqMethodVal, _ := requestPayload["method"].(string)

	// Get server config
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		h.logger.Error("Server config not found for %s", serverName)
		h.sendMCPError(w, reqIDVal, -32602, "Server configuration not found")
		return
	}

	// Determine transport protocol
	protocol := serverConfig.Protocol
	if protocol == "" {
		protocol = "stdio" // default
	}

	h.logger.Info("Forwarding request to server '%s' using '%s' transport: Method=%s, ID=%v",
		serverName, protocol, reqMethodVal, reqIDVal)

	// Route based on transport protocol
	switch protocol {
	case "http":
		h.handleHTTPServerRequest(w, r, serverName, instance, requestPayload, reqIDVal, reqMethodVal)
	case "sse":
		h.handleSSEServerRequest(w, r, serverName, instance, requestPayload, reqIDVal, reqMethodVal)
	case "stdio":
		if serverConfig.StdioHosterPort > 0 {
			h.handleSocatSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		} else {
			h.handleSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		}
	default:
		h.logger.Error("Unsupported transport protocol '%s' for server %s", protocol, serverName)
		h.sendMCPError(w, reqIDVal, -32602, fmt.Sprintf("Unsupported transport protocol: %s", protocol))
	}
}

func (h *ProxyHandler) handleSSEServerRequest(w http.ResponseWriter, r *http.Request, serverName string, _ *ServerInstance, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	conn, err := h.getSSEConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get/create SSE connection for %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32002, fmt.Sprintf("Proxy cannot connect to server '%s' via SSE", serverName))
		return
	}

	// Forward client's Mcp-Session-Id to the backend if present
	clientSessionID := r.Header.Get("Mcp-Session-Id")
	conn.mu.Lock()
	if clientSessionID != "" && conn.SessionID == "" {
		h.logger.Info("Using client-provided Mcp-Session-Id '%s' for SSE backend request to %s", clientSessionID, serverName)
		conn.SessionID = clientSessionID
	} else if clientSessionID != "" && clientSessionID != conn.SessionID {
		h.logger.Warning("Client Mcp-Session-Id '%s' differs from proxy's stored session '%s' for %s. Using proxy's.", clientSessionID, conn.SessionID, serverName)
	}
	conn.mu.Unlock()

	// Send request via SSE
	responsePayload, err := h.sendSSERequest(conn, requestPayload)
	if err != nil {
		h.logger.Error("SSE request to %s (method: %s) failed: %v", serverName, reqMethodVal, err)
		errData := map[string]interface{}{"details": err.Error()}
		if conn != nil {
			conn.mu.Lock()
			errData["targetEndpoint"] = conn.SSEEndpoint
			conn.mu.Unlock()
		}
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Error during SSE call to '%s'", serverName), errData)
		return
	}

	// Relay Mcp-Session-Id from backend server's response
	conn.mu.Lock()
	if conn.SessionID != "" {
		w.Header().Set("Mcp-Session-Id", conn.SessionID)
	}
	conn.mu.Unlock()

	if err := json.NewEncoder(w).Encode(responsePayload); err != nil {
		h.logger.Error("Failed to encode/send response for %s: %v", serverName, err)
	}

	h.logger.Info("Successfully forwarded SSE request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
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

// New helper function that sends requests without waiting for responses
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

// Update the connection cleanup in maintainSSEConnections
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
			select {
			case matchingChannel <- response:
				h.logger.Info("Successfully delivered SSE response for request ID %v to %s", foundKey, conn.ServerName)
				delete(conn.pendingRequests, foundKey) // Clean up
			default:
				h.logger.Warning("Response channel full for request ID %v to %s", foundKey, conn.ServerName)
			}
		} else {
			h.logger.Warning("No pending request found for response ID %v (type %T) from %s. Pending requests: %v",
				responseID, responseID, conn.ServerName, getMapKeys(conn.pendingRequests))
		}

		conn.reqMutex.Unlock()
	} else {
		h.logger.Info("SSE message without ID from %s (notification?): %s", conn.ServerName, messageData)
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

		defer func() {
			conn.reqMutex.Lock()
			delete(conn.pendingRequests, requestID)
			conn.reqMutex.Unlock()
			close(respCh)
		}()

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

			// Wait for async response
			select {
			case response := <-respCh:
				h.logger.Info("Received async SSE response for %s %s", method, conn.ServerName)
				conn.mu.Lock()
				conn.Healthy = true
				conn.mu.Unlock()
				return response, nil
			case <-time.After(15 * time.Second):
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

func (h *ProxyHandler) sendSSERequest(conn *MCPSSEConnection, request map[string]interface{}) (map[string]interface{}, error) {
	return h.sendSSERequestToSession(conn, request)
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

func (h *ProxyHandler) isSSEConnectionHealthy(conn *MCPSSEConnection) bool {
	if conn == nil {
		return false
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	return conn.Healthy && conn.Initialized
}

func (h *ProxyHandler) handleSTDIOServerRequest(w http.ResponseWriter, _ *http.Request, serverName string, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	serverCfg, cfgExists := h.Manager.config.Servers[serverName]
	if !cfgExists {
		h.logger.Error("Config not found for STDIO server %s", serverName)
		h.sendMCPError(w, reqIDVal, -32603, "Internal server error: missing server config")
		return
	}

	h.logger.Info("Executing STDIO request for server '%s' via container '%s' using its defined command.", serverName, containerName)

	requestJSON, err := json.Marshal(requestPayload)
	if err != nil {
		h.logger.Error("Failed to marshal request for STDIO server %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32700, "Failed to marshal request")
		return
	}

	jsonInputWithNewline := string(append(requestJSON, '\n'))

	// Prepare the command to be executed inside the container
	execCmdAndArgs := []string{"exec", "-i", containerName}
	if serverCfg.Command == "" {
		h.logger.Error("STDIO Server '%s' has no command defined in config. Cannot execute.", serverName)
		h.sendMCPError(w, reqIDVal, -32603, "Internal server error: STDIO server has no command")
		return
	}

	execCmdAndArgs = append(execCmdAndArgs, serverCfg.Command)
	execCmdAndArgs = append(execCmdAndArgs, serverCfg.Args...)

	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.Command("docker", execCmdAndArgs...)
	cmd.Stdin = strings.NewReader(jsonInputWithNewline)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	h.logger.Debug("Executing for STDIO '%s': docker %s", serverName, strings.Join(execCmdAndArgs, " "))

	err = cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("Docker exec for STDIO server %s timed out. Stderr: %s. Stdout: %s", serverName, stderr.String(), stdout.String())
			h.recordConnectionEvent(serverName, false, true)
			h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Timeout communicating with STDIO server '%s'", serverName))
			return
		}
		h.logger.Error("Docker exec for STDIO server %s failed: %v. Stderr: %s. Stdout: %s", serverName, err, stderr.String(), stdout.String())
		h.recordConnectionEvent(serverName, false, false)
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Failed to execute command in STDIO server '%s'", serverName))
		return
	}

	responseData := stdout.Bytes()
	if len(responseData) == 0 {
		h.logger.Error("No stdout response from STDIO server %s. Stderr: %s", serverName, stderr.String())
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("No stdout from STDIO server '%s'", serverName))
		return
	}

	h.logger.Debug("Raw stdout from STDIO server '%s': %s", serverName, string(responseData))

	// Parse the response
	var response map[string]interface{}
	trimmedResponseData := bytes.TrimSpace(responseData)
	if err := json.Unmarshal(trimmedResponseData, &response); err != nil {
		h.logger.Error("Invalid JSON response from STDIO server %s: %v. Raw: %s", serverName, err, string(trimmedResponseData))
		h.sendMCPError(w, reqIDVal, -32700, fmt.Sprintf("Invalid response from STDIO server '%s'", serverName))
		return
	}

	h.recordConnectionEvent(serverName, true, false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	h.logger.Info("Successfully forwarded STDIO request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
}

func (h *ProxyHandler) isStdioConnectionReallyHealthy(conn *MCPSTDIOConnection) bool {
	if conn == nil || conn.Connection == nil {
		return false
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Simply check the flags, don't do active probing
	return conn.Healthy && conn.Initialized
}

func (h *ProxyHandler) getStdioConnection(serverName string) (*MCPSTDIOConnection, error) {
	h.StdioMutex.RLock()
	conn, exists := h.StdioConnections[serverName]
	h.StdioMutex.RUnlock()

	if exists && h.isStdioConnectionReallyHealthy(conn) {
		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
		h.logger.Debug("Reusing healthy STDIO connection for %s", serverName)
		return conn, nil
	}

	// If we have an unhealthy connection, clean it up
	if exists && !h.isStdioConnectionReallyHealthy(conn) {
		h.logger.Info("Cleaning up unhealthy STDIO connection for %s", serverName)
		h.StdioMutex.Lock()
		if conn.Connection != nil {
			conn.Connection.Close()
		}
		delete(h.StdioConnections, serverName)
		h.StdioMutex.Unlock()
	}

	h.logger.Info("Creating new STDIO connection for server: %s", serverName)

	// Retry connection creation up to 3 times
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		conn, err := h.createStdioConnection(serverName)
		if err == nil {
			return conn, nil
		}

		lastErr = err
		h.logger.Warning("STDIO connection attempt %d/3 failed for %s: %v", attempt, serverName, err)

		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed to create STDIO connection after 3 attempts: %w", lastErr)
}

func (h *ProxyHandler) startConnectionMaintenance() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.maintainStdioConnections()
				h.maintainHttpConnections()
				h.maintainSSEConnections() // Add this line
			case <-h.ctx.Done():
				return
			}
		}
	}()
}

func (h *ProxyHandler) maintainHttpConnections() {
	h.ConnectionMutex.Lock()
	defer h.ConnectionMutex.Unlock()

	for serverName, conn := range h.ServerConnections {
		if conn == nil {
			continue
		}

		// Clean up old HTTP connections
		maxIdleTime := 10 * time.Minute
		if time.Since(conn.LastUsed) > maxIdleTime {
			h.logger.Info("Removing idle HTTP connection to %s (idle for %v)",
				serverName, time.Since(conn.LastUsed))
			delete(h.ServerConnections, serverName)
		}
	}

	// Force HTTP client to close idle connections periodically
	h.httpClient.CloseIdleConnections()
}

func (h *ProxyHandler) recordConnectionEvent(serverName string, success bool, isTimeout bool) {
	if h.connectionStats == nil {
		h.connectionStats = make(map[string]*ConnectionStats)
	}

	stats, exists := h.connectionStats[serverName]
	if !exists {
		stats = &ConnectionStats{}
		h.connectionStats[serverName] = stats
	}

	stats.mu.Lock()
	defer stats.mu.Unlock()

	stats.TotalRequests++

	if success {
		stats.LastSuccess = time.Now()
	} else {
		stats.FailedRequests++
		stats.LastError = time.Now()
		if isTimeout {
			stats.TimeoutErrors++
		}
	}
}

func (h *ProxyHandler) maintainStdioConnections() {
	h.StdioMutex.Lock()
	defer h.StdioMutex.Unlock()

	for serverName, conn := range h.StdioConnections {
		if conn == nil {
			continue
		}

		// Increase idle time back to reasonable levels
		maxIdleTime := 15 * time.Minute // Increased from 5 minutes

		if time.Since(conn.LastUsed) > maxIdleTime {
			h.logger.Info("Closing idle STDIO connection to %s (idle for %v)",
				serverName, time.Since(conn.LastUsed))
			if conn.Connection != nil {
				conn.Connection.Close()
			}
			delete(h.StdioConnections, serverName)
		}
		// Remove the aggressive health checking - it's causing more problems
	}
}

func (h *ProxyHandler) createStdioConnection(serverName string) (*MCPSTDIOConnection, error) {
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	port := serverConfig.StdioHosterPort
	address := fmt.Sprintf("%s:%d", containerName, port)

	// Use shorter connection timeout
	var d net.Dialer
	ctx, cancel := context.WithTimeout(h.ctx, 15*time.Second) // Reduced from 30s
	defer cancel()

	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Enable TCP keepalive with aggressive settings
	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(15 * time.Second) // More frequent keepalive
		tcpConn.SetNoDelay(true)                     // Disable Nagle's algorithm
		h.logger.Debug("Enabled TCP keepalive for connection to %s", serverName)
	}

	conn := &MCPSTDIOConnection{
		ServerName:  serverName,
		Host:        containerName,
		Port:        port,
		Connection:  netConn,
		Reader:      bufio.NewReaderSize(netConn, 8192), // Larger buffer
		Writer:      bufio.NewWriterSize(netConn, 8192), // Larger buffer
		LastUsed:    time.Now(),
		Healthy:     true,
		Initialized: false,
	}

	// Initialize the connection with shorter timeout
	if err := h.initializeStdioConnection(conn); err != nil {
		conn.Connection.Close()
		return nil, fmt.Errorf("failed to initialize STDIO connection to %s: %w", serverName, err)
	}

	h.StdioMutex.Lock()
	if h.StdioConnections == nil {
		h.StdioConnections = make(map[string]*MCPSTDIOConnection)
	}
	h.StdioConnections[serverName] = conn
	h.StdioMutex.Unlock()

	h.logger.Info("Successfully created and initialized STDIO connection for %s", serverName)
	return conn, nil
}

func (h *ProxyHandler) initializeStdioConnection(conn *MCPSTDIOConnection) error {
	h.logger.Info("Initializing STDIO connection to %s", conn.ServerName)

	// Send initialize request
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

	// Set initial deadline for initialization
	conn.Connection.SetWriteDeadline(time.Now().Add(30 * time.Second))
	conn.Connection.SetReadDeadline(time.Now().Add(30 * time.Second))

	if err := h.sendStdioRequestWithoutLock(conn, initRequest); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Read initialize response
	response, err := h.readStdioResponseWithoutLock(conn)
	if err != nil {
		return fmt.Errorf("failed to read initialize response: %w", err)
	}

	if mcpError, hasError := response["error"]; hasError {
		return fmt.Errorf("initialize failed: %v", mcpError)
	}

	// Send initialized notification - this is critical and was missing proper handling
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	}

	if err := h.sendStdioRequestWithoutLock(conn, initNotification); err != nil {
		// Don't fail if notification fails, some servers don't support it
		h.logger.Warning("Failed to send initialized notification to %s: %v (continuing anyway)", conn.ServerName, err)
	}

	// Reset deadlines after successful initialization
	conn.Connection.SetWriteDeadline(time.Time{})
	conn.Connection.SetReadDeadline(time.Time{})

	conn.mu.Lock()
	conn.Initialized = true
	conn.Healthy = true
	conn.mu.Unlock()

	h.logger.Info("STDIO connection to %s initialized successfully", conn.ServerName)
	return nil
}

func (h *ProxyHandler) sendStdioRequestWithoutLock(conn *MCPSTDIOConnection, request map[string]interface{}) error {
	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	h.logger.Debug("Sending STDIO request to %s: %s", conn.ServerName, string(requestData))

	// Write with newline
	_, err = conn.Writer.WriteString(string(requestData) + "\n")
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to write request: %w", err)
	}

	err = conn.Writer.Flush()
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to flush request: %w", err)
	}

	return nil
}

func (h *ProxyHandler) readStdioResponseWithoutLock(conn *MCPSTDIOConnection) (map[string]interface{}, error) {
	for {
		line, err := conn.Reader.ReadString('\n')
		if err != nil {
			conn.Healthy = false
			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		h.logger.Debug("Received STDIO line from %s: %s", conn.ServerName, line)

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			h.logger.Debug("Skipping non-JSON line from %s: %s", conn.ServerName, line)
			continue
		}

		_, hasResult := response["result"]
		_, hasError := response["error"]
		_, hasMethod := response["method"]

		if (hasResult || hasError) && !hasMethod {
			h.logger.Debug("Found valid JSON-RPC response from %s", conn.ServerName)
			return response, nil
		} else if hasMethod {
			h.logger.Debug("Skipping echoed request/notification from %s: %s", conn.ServerName, line)
			continue
		} else {
			h.logger.Debug("Skipping unknown JSON structure from %s: %s", conn.ServerName, line)
			continue
		}
	}
}

func (h *ProxyHandler) sendStdioRequest(conn *MCPSTDIOConnection, request map[string]interface{}) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	h.logger.Debug("Sending STDIO request to %s: %s", conn.ServerName, string(requestData))

	// Set reasonable write deadline - longer than before
	writeDeadline := time.Now().Add(60 * time.Second)
	conn.Connection.SetWriteDeadline(writeDeadline)
	defer conn.Connection.SetWriteDeadline(time.Time{})

	// Write with newline
	_, err = conn.Writer.WriteString(string(requestData) + "\n")
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to write request: %w", err)
	}

	err = conn.Writer.Flush()
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to flush request: %w", err)
	}

	return nil
}

func (h *ProxyHandler) readStdioResponse(conn *MCPSTDIOConnection) (map[string]interface{}, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Set reasonable read deadline - longer for complex operations
	readDeadline := time.Now().Add(60 * time.Second)
	conn.Connection.SetReadDeadline(readDeadline)
	defer conn.Connection.SetReadDeadline(time.Time{})

	for {
		line, err := conn.Reader.ReadString('\n')
		if err != nil {
			conn.Healthy = false
			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		h.logger.Debug("Received STDIO line from %s: %s", conn.ServerName, line)

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			h.logger.Debug("Skipping non-JSON line from %s: %s", conn.ServerName, line)
			continue
		}

		_, hasResult := response["result"]
		_, hasError := response["error"]
		_, hasMethod := response["method"]

		if (hasResult || hasError) && !hasMethod {
			h.logger.Debug("Found valid JSON-RPC response from %s", conn.ServerName)
			return response, nil
		} else if hasMethod {
			h.logger.Debug("Skipping echoed request/notification from %s: %s", conn.ServerName, line)
			continue
		} else {
			h.logger.Debug("Skipping unknown JSON structure from %s: %s", conn.ServerName, line)
			continue
		}
	}
}

func (h *ProxyHandler) handleSocatSTDIOServerRequest(w http.ResponseWriter, r *http.Request, serverName string, requestPayload map[string]interface{}, reqIDVal interface{}, _ string) {
	conn, err := h.getStdioConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get STDIO connection for %s: %v", serverName, err)
		h.recordConnectionEvent(serverName, false, strings.Contains(err.Error(), "timeout"))

		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout") {
			h.sendMCPError(w, reqIDVal, -32001, fmt.Sprintf("Server '%s' timed out - connection may be overloaded", serverName))
		} else {
			h.sendMCPError(w, reqIDVal, -32001, fmt.Sprintf("Cannot connect to server '%s'", serverName))
		}
		return
	}

	// Increase timeout for complex operations
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second) // Back to 2 minutes for complex ops
	defer cancel()

	// Create channels to handle the response
	responseChan := make(chan map[string]interface{}, 1)
	errorChan := make(chan error, 1)

	go func() {
		// Send the request
		if err := h.sendStdioRequest(conn, requestPayload); err != nil {
			errorChan <- err
			return
		}

		// Read the response
		response, err := h.readStdioResponse(conn)
		if err != nil {
			errorChan <- err
			return
		}

		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		h.recordConnectionEvent(serverName, true, false)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	case err := <-errorChan:
		h.logger.Error("Failed to communicate with %s: %v", serverName, err)
		isTimeout := strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout")
		h.recordConnectionEvent(serverName, false, isTimeout)

		if isTimeout {
			h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Server '%s' request timed out", serverName))
		} else {
			h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Error communicating with server '%s'", serverName))
		}
	case <-ctx.Done():
		h.logger.Error("Request to %s timed out", serverName)
		h.recordConnectionEvent(serverName, false, true)
		h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Request to server '%s' timed out", serverName))
	}
}

func (h *ProxyHandler) handleHTTPServerRequest(w http.ResponseWriter, r *http.Request, serverName string, _ *ServerInstance, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	conn, err := h.getServerConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get/create HTTP connection for %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32002, fmt.Sprintf("Proxy cannot connect to server '%s'", serverName))
		return
	}

	mcpCallTimeout := 60 * time.Second
	if reqMethodVal == "initialize" {
		mcpCallTimeout = 90 * time.Second
	}

	// Forward client's Mcp-Session-Id to the backend if present
	clientSessionID := r.Header.Get("Mcp-Session-Id")
	conn.mu.Lock()
	if clientSessionID != "" && conn.SessionID == "" {
		h.logger.Info("Using client-provided Mcp-Session-Id '%s' for backend request to %s", clientSessionID, serverName)
		conn.SessionID = clientSessionID
	} else if clientSessionID != "" && clientSessionID != conn.SessionID {
		h.logger.Warning("Client Mcp-Session-Id '%s' differs from proxy's stored session '%s' for %s. Using proxy's.", clientSessionID, conn.SessionID, serverName)
	}
	conn.mu.Unlock()

	responsePayload, err := h.sendHTTPJsonRequest(conn, requestPayload, mcpCallTimeout)
	if err != nil {
		h.logger.Error("MCP request to %s (method: %s) failed: %v", serverName, reqMethodVal, err)
		errData := map[string]interface{}{"details": err.Error()}
		if conn != nil {
			conn.mu.Lock()
			errData["targetUrl"] = conn.BaseURL
			conn.mu.Unlock()
		}

		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host") {
			h.sendMCPError(w, reqIDVal, -32001, fmt.Sprintf("Server '%s' is unreachable or did not respond in time", serverName), errData)
		} else {
			h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Error during MCP call to '%s'", serverName), errData)
		}
		return
	}

	// Relay Mcp-Session-Id from backend server's response
	conn.mu.Lock()
	if conn.SessionID != "" {
		w.Header().Set("Mcp-Session-Id", conn.SessionID)
	}
	conn.mu.Unlock()

	if err := json.NewEncoder(w).Encode(responsePayload); err != nil {
		h.logger.Error("Failed to encode/send response for %s: %v", serverName, err)
	}
	h.logger.Info("Successfully forwarded HTTP request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
}

func (h *ProxyHandler) handleSessionTermination(w http.ResponseWriter, r *http.Request, serverName string) {
	clientSessionID := r.Header.Get("Mcp-Session-Id")
	if clientSessionID == "" {
		h.corsError(w, "Mcp-Session-Id header required for session termination (DELETE)", http.StatusBadRequest)
		return
	}
	h.logger.Info("Received DELETE request to terminate session '%s' for server '%s'", clientSessionID, serverName)

	// Ask the backend server to terminate its session
	conn, err := h.getServerConnection(serverName) // Get existing or create (though for DELETE, expect existing)
	if err != nil {
		h.logger.Warning("Cannot terminate session: No connection to server '%s' (%v)", serverName, err)
		h.corsError(w, "Server not connected via proxy", http.StatusBadGateway)
		return
	}

	conn.mu.Lock()
	if conn.SessionID != clientSessionID && conn.SessionID != "" {
		h.logger.Warning("Mcp-Session-Id mismatch during DELETE. Client '%s', Proxy's known '%s' for %s. Proceeding with client's ID.",
			clientSessionID, conn.SessionID, serverName)
		// Use client's ID for the DELETE request, as that's what they want to terminate.
	}
	conn.mu.Unlock()

	// The MCP spec isn't explicit on a "session/delete" JSON-RPC method.
	// It says client "SHOULD send an HTTP DELETE to the MCP endpoint with the Mcp-Session-Id header".
	// The proxy will forward this HTTP DELETE.

	reqCtx, cancel := context.WithTimeout(h.ctx, 15*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, conn.BaseURL, nil)
	if err != nil {
		h.logger.Error("Failed to create HTTP DELETE request for %s: %v", serverName, err)
		h.corsError(w, "Internal proxy error", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Mcp-Session-Id", clientSessionID) // Pass the client's session ID to terminate

	backendResp, err := h.httpClient.Do(httpReq)
	if err != nil {
		h.logger.Error("HTTP DELETE request to backend server %s failed: %v", serverName, err)
		h.corsError(w, "Failed to communicate with backend server for session termination", http.StatusBadGateway)
		return
	}
	defer backendResp.Body.Close()

	// Clean up proxy's internal state for this session
	h.ConnectionMutex.Lock()
	if activeConn, exists := h.ServerConnections[serverName]; exists {
		activeConn.mu.Lock()
		if activeConn.SessionID == clientSessionID {
			h.logger.Info("Proxy forgetting session '%s' for server '%s' after DELETE.", clientSessionID, serverName)
			activeConn.Initialized = false
			activeConn.SessionID = "" // Clear stored session ID
			// Optionally, fully delete from h.ServerConnections[serverName] if no longer needed
			// delete(h.ServerConnections, serverName)
		}
		activeConn.mu.Unlock()
	}
	h.ConnectionMutex.Unlock()

	// Relay server's response status
	if backendResp.StatusCode == http.StatusMethodNotAllowed {
		w.WriteHeader(http.StatusMethodNotAllowed)
		fmt.Fprint(w, `{"message": "Server does not allow client-initiated session termination via DELETE"}`)
		return
	}

	w.WriteHeader(backendResp.StatusCode) // Send back what the MCP server responded with (e.g., 200, 204)
	io.Copy(w, backendResp.Body)          // Copy body if any
	h.logger.Info("Session termination request for '%s' on server '%s' processed with status %d.", clientSessionID, serverName, backendResp.StatusCode)
}

func (h *ProxyHandler) sendMCPError(w http.ResponseWriter, id interface{}, code int, message string, data ...interface{}) {
	// (Same as previously provided, ensure data is handled correctly)
	errResponse := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
		},
	}
	if len(data) > 0 && data[0] != nil {
		errResponse.Error.Data = data[0]
	}

	w.Header().Set("Content-Type", "application/json")
	httpStatus := http.StatusOK
	// Basic mapping from JSON-RPC error codes to HTTP status codes
	if code == -32700 {
		httpStatus = http.StatusBadRequest
	} // Parse error
	if code == -32600 {
		httpStatus = http.StatusBadRequest
	} // Invalid Request
	if code == -32601 {
		httpStatus = http.StatusNotFound
	} // Method not found (endpoint for server might be, but method on server not)
	if code == -32602 {
		httpStatus = http.StatusBadRequest
	} // Invalid params
	if code >= -32099 && code <= -32000 {
		httpStatus = http.StatusInternalServerError
	} // Server errors

	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(errResponse); err != nil {
		h.logger.Error("CRITICAL: Failed to encode and send MCP JSON-RPC error response to client: %v", err)
	}
}

// --- API Handler Implementations (ensure these match the structure of previous full file) ---
// (handleAPIServers, handleAPIStatus, handleDiscoveryEndpoint, handleConnectionsAPI, handleServerDetails, handleIndex)
// The definitions provided in the prompt ending "Corrected `handleServerDetails` function:" seem complete and correct.
// Just ensure they are all present and use the refined helper functions like getConnectionHealthStatus.
// Copying them here again for absolute completeness:

func (h *ProxyHandler) handleAPIServers(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	serverList := make(map[string]map[string]interface{})

	for name := range h.Manager.config.Servers {
		instance, exists := h.Manager.GetServerInstance(name)
		if !exists {
			h.logger.Warning("Server %s in config but not in manager instance list for /api/servers.", name)
			serverList[name] = map[string]interface{}{"name": name, "status": "error - not in manager"}
			continue
		}

		containerStatus, _ := h.Manager.GetServerStatus(name)
		serverConfig := h.Manager.config.Servers[name]

		serverInfo := map[string]interface{}{
			"name":               name,
			"containerStatus":    containerStatus,
			"configCapabilities": serverConfig.Capabilities,
			"configProtocol":     serverConfig.Protocol,
			"configHttpPort":     serverConfig.HttpPort,
			"isContainer":        instance.IsContainer,
			"proxyTransportMode": "HTTP",
		}

		h.ConnectionMutex.RLock()
		if conn, connExists := h.ServerConnections[name]; connExists {
			conn.mu.Lock()
			serverInfo["httpConnection"] = map[string]interface{}{
				"proxyConnectionStatus":      h.getConnectionHealthStatus(conn),
				"mcpSessionInitialized":      conn.Initialized,
				"mcpSessionID":               conn.SessionID,
				"lastUsedByProxy":            conn.LastUsed.Format(time.RFC3339Nano),
				"targetBaseURL":              conn.BaseURL,
				"serverReportedCapabilities": conn.Capabilities,
				"serverReportedInfo":         conn.ServerInfo,
			}
			conn.mu.Unlock()
		} else {
			serverInfo["httpConnection"] = "Proxy has no active HTTP connection to this server."
		}
		h.ConnectionMutex.RUnlock()

		serverList[name] = serverInfo
	}

	if err := json.NewEncoder(w).Encode(serverList); err != nil {
		h.logger.Error("Failed to encode /api/servers response: %v", err)
	}
}

func (h *ProxyHandler) getConnectionHealthStatus(conn *MCPHTTPConnection) string {
	// Assumes conn.mu is NOT held by caller if reading multiple atomic fields
	// but it's safer if conn.mu IS held by caller to get a consistent snapshot.
	// This function is now called within locked sections or from places that have just updated health.
	if conn.Healthy && conn.Initialized {
		status := "Active & Initialized"
		if conn.SessionID != "" {
			status += " (Session ID: " + conn.SessionID + ")"
		}
		return status
	} else if conn.Initialized {
		status := "Initialized but Unhealthy"
		if conn.SessionID != "" {
			status += " (Session ID: " + conn.SessionID + ")"
		}
		return status
	} else if conn.Healthy {
		return "Contactable but MCP Session Not Initialized"
	}
	return "Unhealthy / MCP Session Not Initialized"
}

func (h *ProxyHandler) handleAPIStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	runningContainers := 0
	activeHTTPConnections := 0
	initializedHTTPSessions := 0
	totalServersInConfig := len(h.Manager.config.Servers)

	for name := range h.Manager.config.Servers {
		if status, _ := h.Manager.GetServerStatus(name); status == "running" {
			runningContainers++
		}
	}

	h.ConnectionMutex.RLock()
	for _, conn := range h.ServerConnections {
		conn.mu.Lock()
		if conn.Healthy {
			activeHTTPConnections++
			if conn.Initialized {
				initializedHTTPSessions++
			}
		}
		conn.mu.Unlock()
	}
	h.ConnectionMutex.RUnlock()

	apiStatus := map[string]interface{}{
		"proxyStartTime":                 h.ProxyStarted.Format(time.RFC3339),
		"proxyUptime":                    time.Since(h.ProxyStarted).String(),
		"totalConfiguredServers":         totalServersInConfig,
		"runningContainers":              runningContainers,
		"activeHttpConnectionsToServers": activeHTTPConnections,
		"initializedMcpSessions":         initializedHTTPSessions,
		"proxyTransportMode":             "HTTP",
		"mcpComposeVersion":              "dev", // TODO: Inject version at build time
		"mcpSpecVersionUsedByProxy":      "2025-03-26",
	}

	if err := json.NewEncoder(w).Encode(apiStatus); err != nil {
		h.logger.Error("Failed to encode /api/status response: %v", err)
	}
}

func (h *ProxyHandler) handleDiscoveryEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	serversForDiscovery := make([]map[string]interface{}, 0)

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	proxyExternalBaseURL := fmt.Sprintf("%s://%s", scheme, r.Host)

	for serverNameInConfig, serverConfigFromFile := range h.Manager.config.Servers {
		clientReachableEndpoint := fmt.Sprintf("%s/%s", proxyExternalBaseURL, serverNameInConfig)

		var currentCapabilities interface{} = serverConfigFromFile.Capabilities
		// mcpSessionIsInitializedByProxy := false // This was the unused var

		h.ConnectionMutex.RLock()
		if liveConn, exists := h.ServerConnections[serverNameInConfig]; exists {
			liveConn.mu.Lock()
			// mcpSessionIsInitializedByProxy = liveConn.Initialized && liveConn.Healthy // If needed for a custom discovery field
			if liveConn.Initialized && liveConn.Healthy && liveConn.Capabilities != nil && len(liveConn.Capabilities) > 0 {
				currentCapabilities = liveConn.Capabilities
			}
			liveConn.mu.Unlock()
		}
		h.ConnectionMutex.RUnlock()

		description := fmt.Sprintf("MCP %s server (via proxy)", serverNameInConfig)
		if len(serverConfigFromFile.Tools) > 0 && serverConfigFromFile.Tools[0].Description != "" {
			// A more specific description if available from config
			// description = serverConfigFromFile.Tools[0].Description // This is too specific, just an example
		}

		serverEntry := map[string]interface{}{
			"name":         serverNameInConfig,
			"httpEndpoint": clientReachableEndpoint,
			"capabilities": currentCapabilities,
			"description":  description,
		}

		// Add tools if ServerConfig has Tools and client expects it
		if len(serverConfigFromFile.Tools) > 0 {
			var toolsForClient []map[string]interface{}
			for _, toolDef := range serverConfigFromFile.Tools {
				toolEntry := map[string]interface{}{"name": toolDef.Name}
				if toolDef.Description != "" {
					toolEntry["description"] = toolDef.Description
				}
				// TODO: Add inputSchema conversion if clients require it here
				toolsForClient = append(toolsForClient, toolEntry)
			}
			if len(toolsForClient) > 0 { // Only add "tools" key if there are tools
				serverEntry["tools"] = toolsForClient
			}
		}
		serversForDiscovery = append(serversForDiscovery, serverEntry)
	}

	discoveryResponse := map[string]interface{}{
		"servers": serversForDiscovery, // Standard key for Claude Desktop
	}

	if err := json.NewEncoder(w).Encode(discoveryResponse); err != nil {
		h.logger.Error("Failed to encode /api/discovery response: %v", err)
	}
}

func (h *ProxyHandler) handleConnectionsAPI(w http.ResponseWriter, _ *http.Request) {
	h.ConnectionMutex.RLock()
	connectionsSnapshot := make(map[string]interface{})

	for name, conn := range h.ServerConnections {
		conn.mu.Lock()
		connectionsSnapshot[name] = map[string]interface{}{
			"serverName":                 conn.ServerName,
			"targetBaseURL":              conn.BaseURL,
			"status":                     h.getConnectionHealthStatus(conn),
			"initialized":                conn.Initialized,
			"rawHealthyFlag":             conn.Healthy, // For debugging
			"mcpSessionID":               conn.SessionID,
			"lastUsedByProxy":            conn.LastUsed.Format(time.RFC3339Nano),
			"serverReportedCapabilities": conn.Capabilities,
			"serverReportedInfo":         conn.ServerInfo,
		}
		conn.mu.Unlock()
	}
	h.ConnectionMutex.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"activeHttpConnectionsManagedByProxy": connectionsSnapshot,
		"totalActiveManagedConnections":       len(connectionsSnapshot),
		"timestamp":                           time.Now().Format(time.RFC3339Nano),
		"proxyToBackendTransportMode":         "HTTP (Streamable HTTP Spec 2025-03-26)",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode /api/connections response: %v", err)
	}
}

func (h *ProxyHandler) handleServerDetails(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance) {
	w.Header().Set("Content-Type", "text/html")
	containerStatus, _ := h.Manager.GetServerStatus(serverName)

	var connectionStatusDisplay, internalURL, clientEndpointURL string
	var liveCaps, liveSInfo interface{}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	clientEndpointURL = fmt.Sprintf("%s://%s/%s", scheme, r.Host, serverName)

	h.ConnectionMutex.RLock()
	if conn, exists := h.ServerConnections[serverName]; exists {
		conn.mu.Lock()
		internalURL = conn.BaseURL
		connectionStatusDisplay = h.getConnectionHealthStatus(conn)
		liveCaps = conn.Capabilities
		liveSInfo = conn.ServerInfo
		conn.mu.Unlock()
	} else {
		connectionStatusDisplay = "○ No Active HTTP Connection via Proxy"
		if srvCfg, ok := h.Manager.config.Servers[serverName]; ok {
			internalURL = h.getServerHTTPURL(serverName, srvCfg) // Show configured target even if not connected
		}
	}
	h.ConnectionMutex.RUnlock()

	capsStrBytes, _ := json.MarshalIndent(liveCaps, "", "  ")
	sInfoStrBytes, _ := json.MarshalIndent(liveSInfo, "", "  ")
	capsStr := string(capsStrBytes)
	sInfoStr := string(sInfoStrBytes)
	if liveCaps == nil {
		capsStr = "{ (not available or not initialized) }"
	}
	if liveSInfo == nil {
		sInfoStr = "{ (not available or not initialized) }"
	}

	htmlOutput := fmt.Sprintf(`
<!DOCTYPE html>
<html lang="en">
<head><title>MCP Server Details: %s</title>
<style>
    body { font-family: "Segoe UI", Tahoma, Geneva, Verdana, sans-serif; margin: 20px; line-height: 1.6; color: #333; background-color: #f9f9f9;}
    .container { max-width: 960px; margin: auto; background-color: #fff; padding: 20px; border-radius: 8px; box-shadow: 0 4px 8px rgba(0,0,0,0.05); }
    h1, h3 { color: #2c3e50; border-bottom: 1px solid #dfe6e9; padding-bottom: 8px;}
    p { margin-bottom: 0.8em; }
    code { background-color: #e9ecef; padding: 3px 6px; border-radius: 4px; font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace; color: #c7254e;}
    strong { color: #495057; }
    pre { background-color: #f1f3f5; padding: 15px; border: 1px solid #ced4da; border-radius: 5px; overflow-x: auto; white-space: pre-wrap; word-wrap: break-word; font-family: "SFMono-Regular", Consolas, "Liberation Mono", Menlo, Courier, monospace; font-size: 0.85em; color: #212529;}
    a { color: #007bff; text-decoration: none; }
    a:hover { text-decoration: underline; }
</style>
</head>
<body>
    <div class="container">
        <h1>MCP Server Details: %s</h1>
        <p><strong>Container/Process Status (from Runtime):</strong> <code>%s</code></p>
        <p><strong>Proxy's HTTP Connection to Server:</strong> %s</p>
        <p><strong>Internal Target URL (Proxy &rarr; Server):</strong> <code>%s</code></p>
        <p><strong>Client Access Endpoint (Client &rarr; Proxy &rarr; Server):</strong> <code>%s</code></p>
        <p><strong>Configured Protocol (in mcp-compose.yaml):</strong> <code>%s</code></p>
        
        <h3>Server Capabilities (Live from Server's Initialize via Proxy):</h3>
        <pre>%s</pre>
        
        <h3>Server Info (Live from Server's Initialize via Proxy):</h3>
        <pre>%s</pre>
        
        <p><a href="/">&larr; Back to Server List</a></p>
        <p><a href="/api/connections">View All Proxy-Server Connections (JSON)</a></p>
    </div>
</body>
</html>
`, serverName, serverName, containerStatus, connectionStatusDisplay, internalURL, clientEndpointURL, instance.Config.Protocol, capsStr, sInfoStr)

	_, err := w.Write([]byte(htmlOutput))
	if err != nil {
		h.logger.Error("Failed to write server details HTTP response: %v", err)
	}
}

func (h *ProxyHandler) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var bodyBuilder strings.Builder
	bodyBuilder.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MCP Compose Proxy (HTTP/SSE Mode)</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif; margin: 0; background-color: #f0f2f5; color: #333; padding: 20px;}
        .container { max-width: 1200px; margin: 0 auto; }
        header { background-color: #2c3e50; color: white; padding: 20px 25px; border-radius: 8px; margin-bottom: 25px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        header h1 { margin: 0; font-size: 2em; font-weight: 600;}
        header p { margin: 5px 0 0; font-size: 1em; opacity: 0.85; }
        h2 { color: #34495e; border-bottom: 2px solid #dfe6e9; padding-bottom: 10px; margin-top: 35px; font-size: 1.6em;}
        .server-list { display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 20px; }
        .server { background-color: #ffffff; padding: 20px; border: 1px solid #dde1e6; border-radius: 6px; box-shadow: 0 4px 8px rgba(0,0,0,0.07); transition: transform 0.2s ease-in-out, box-shadow 0.2s ease-in-out; }
        .server:hover { transform: translateY(-3px); box-shadow: 0 6px 12px rgba(0,0,0,0.1); }
        .server h3 { margin-top: 0; color: #2c3e50; }
        .server a { text-decoration: none; color: #3498db; font-weight: 500; margin-right: 15px; }
        .server a:hover { color: #2575ae; text-decoration: underline; }
        .status, .connection-status { font-size: 0.95em; margin-top: 5px; line-height: 1.5; }
        .status strong, .connection-status strong { color: #4a5568; }
        .status-dot { display: inline-block; width: 10px; height: 10px; border-radius: 50%; margin-right: 7px; }
        .running .status-dot { background-color: #2ecc71; }
        .stopped .status-dot { background-color: #e74c3c; }
        .unknown .status-dot { background-color: #f39c12; }
        .api-links { margin-top: 40px; padding: 25px; background-color: #ffffff; border-radius: 8px; box-shadow: 0 4px 8px rgba(0,0,0,0.05); }
        .api-links ul { list-style-type: none; padding: 0; }
        .api-links li { margin-bottom: 12px; }
        .api-links a { text-decoration: none; color: #2980b9; font-weight: 500; }
        .api-links a:hover { text-decoration: underline; color: #1c5a7d; }
        .openwebui-config { background: #e8f5e8; padding: 15px; border-radius: 6px; margin-top: 20px; }
        .openwebui-config code { background: #fff; padding: 2px 6px; border-radius: 3px; color: #c7254e; }
    </style>
</head>
<body>
    <div class="container">
    <header>
        <h1>MCP Compose Proxy</h1>
        <p>Orchestrating Model Context Protocol Servers with HTTP/SSE Transport</p>
    </header>
    <h2>Available MCP Servers:</h2>
    <div class="server-list">`)

	serverNames := make([]string, 0, len(h.Manager.config.Servers))
	for name := range h.Manager.config.Servers {
		serverNames = append(serverNames, name)
	}

	for _, name := range serverNames {
		containerStatus, _ := h.Manager.GetServerStatus(name)
		statusClass := "unknown"
		statusDotClass := "unknown"
		if strings.ToLower(containerStatus) == "running" {
			statusClass = "running"
			statusDotClass = "running"
		} else if containerStatus == "stopped" || strings.HasPrefix(containerStatus, "exited") || containerStatus == "No Runtime" {
			statusClass = "stopped"
			statusDotClass = "stopped"
		}

		var displayedConnectionStatus string
		h.ConnectionMutex.RLock()
		if conn, exists := h.ServerConnections[name]; exists {
			conn.mu.Lock()
			displayedConnectionStatus = h.getConnectionHealthStatus(conn)
			conn.mu.Unlock()
		} else {
			displayedConnectionStatus = "○ No Active HTTP Connection via Proxy"
		}
		h.ConnectionMutex.RUnlock()

		bodyBuilder.WriteString(fmt.Sprintf(`
    <div class="server %s">
        <h3>%s</h3>
        <div class="status"><span class="status-dot %s"></span><strong>Container/Process Status:</strong> %s</div>
        <div class="connection-status"><strong>Proxy HTTP Connection:</strong> %s</div>
        <div style="margin-top: 15px;">
            <a href="/%s/docs">📖 Docs</a>
            <a href="/%s/openapi.json">📋 OpenAPI Spec</a>
            <a href="/%s">🔧 Direct Access</a>
        </div>
        <div class="openwebui-config">
            <strong>For OpenWebUI:</strong><br>
            <code>http://192.168.86.201:9876/%s/openapi.json</code>
        </div>
    </div>`, statusClass, name, statusDotClass, containerStatus, displayedConnectionStatus, name, name, name, name))
	}

	bodyBuilder.WriteString(`</div>
    <div class="api-links">
        <h2>Diagnostic API Endpoints:</h2>
        <ul>
            <li><a href="/api/servers">/api/servers</a> &ndash; List servers and their proxy connection status.</li>
            <li><a href="/api/status">/api/status</a> &ndash; Overall proxy health and server summary.</li>
            <li><a href="/api/discovery">/api/discovery</a> &ndash; MCP discovery endpoint.</li>
            <li><a href="/api/connections">/api/connections</a> &ndash; Detailed status of active HTTP connections.</li>
            <li><a href="/openapi.json">/openapi.json</a> &ndash; Combined OpenAPI specification.</li>
        </ul>
    </div>
    <div style="margin-top: 40px; padding: 25px; background-color: #fff3cd; border-radius: 8px;">
        <h3>🎯 OpenWebUI Integration</h3>
        <p>Add each server individually to OpenWebUI as separate tools servers:</p>
        <ul>`)

	for _, name := range serverNames {
		bodyBuilder.WriteString(fmt.Sprintf(`
            <li><strong>%s:</strong> <code>http://192.168.86.201:9876/%s/openapi.json</code></li>`, name, name))
	}

	bodyBuilder.WriteString(`
        </ul>
        <p><strong>API Key:</strong> <code>myapikey</code></p>
    </div>
    </div>
</body>
</html>`)

	_, err := w.Write([]byte(bodyBuilder.String()))
	if err != nil {
		h.logger.Error("Failed to write index HTML response: %v", err)
	}
}

func (h *ProxyHandler) getServerHTTPURL(serverName string, serverConfig config.ServerConfig) string {
	targetHost := fmt.Sprintf("mcp-compose-%s", serverName)
	targetPort := serverConfig.HttpPort

	// If HttpPort is not explicitly set in YAML, try to infer it from the 'ports' mapping
	if targetPort == 0 && serverConfig.Protocol == "http" {
		if len(serverConfig.Ports) > 0 {
			for _, portMapping := range serverConfig.Ports {
				parts := strings.Split(portMapping, ":")
				var containerPortStr string
				if len(parts) == 2 {
					containerPortStr = parts[1]
				} else if len(parts) == 1 {
					containerPortStr = parts[0]
				}
				if p, err := strconv.Atoi(containerPortStr); err == nil && p > 0 {
					targetPort = p
					h.logger.Info("Server %s: Inferred internal http_port %d from 'ports' mapping ('%s'). Consider defining 'http_port' explicitly.", serverName, targetPort, portMapping)
					break
				}
			}
		}
	}

	if targetPort == 0 && serverConfig.Protocol == "http" {
		h.logger.Error("Server %s (HTTP): 'http_port' is 0 and could not be inferred from 'ports'. This is a critical configuration error for HTTP communication within Docker network.", serverName)
		return fmt.Sprintf("http://%s:INVALID_PORT_CONFIG_FOR_HTTP_SERVER", targetHost)
	}

	if targetPort == 0 && serverConfig.Protocol != "http" {
		h.logger.Debug("Server %s is likely STDIO (http_port is 0 and protocol is not http). URL constructed for display purposes only if needed.", serverName)
		return fmt.Sprintf("http://%s:0/ (STDIO server, no HTTP port)", targetHost)
	}

	// Build the URL with the HTTP path
	baseURL := fmt.Sprintf("http://%s:%d", targetHost, targetPort)

	// Add the HTTP path if specified
	if serverConfig.HttpPath != "" {
		// Ensure path starts with /
		path := serverConfig.HttpPath
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		baseURL += path
	} else {
		baseURL += "/"
	}

	h.logger.Debug("Resolved internal HTTP URL (MCP Endpoint for containerized proxy) for server %s: %s", serverName, baseURL)
	return baseURL
}

func (h *ProxyHandler) isKnownTool(toolName string) bool {
	// Refresh cache if needed
	h.refreshToolCache()

	h.toolCacheMu.RLock()
	_, exists := h.toolCache[toolName]
	h.toolCacheMu.RUnlock()

	h.logger.Debug("Tool cache lookup for '%s': %v", toolName, exists)
	return exists
}

func (h *ProxyHandler) handleDirectToolCall(w http.ResponseWriter, r *http.Request, toolName string) {
	// Authenticate
	apiKeyToCheck := h.APIKey
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.ProxyAuth.Enabled {
		apiKeyToCheck = h.Manager.config.ProxyAuth.APIKey
	}

	if apiKeyToCheck != "" {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiKeyToCheck {
			h.corsError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	h.logger.Info("Handling direct tool call: %s", toolName)

	// Parse request body as tool arguments
	var arguments map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&arguments); err != nil {
		h.logger.Error("Failed to decode request body for tool %s: %v", toolName, err)
		h.corsError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Find which server has this tool
	serverName, found := h.findServerForTool(toolName)
	if !found {
		h.logger.Warning("Tool %s not found in any server", toolName)
		h.corsError(w, "Tool not found", http.StatusNotFound)
		return
	}

	h.logger.Info("Routing tool %s to server %s", toolName, serverName)

	// Create MCP tools/call request
	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	// Forward to the appropriate server and get response
	if instance, exists := h.Manager.GetServerInstance(serverName); exists {
		// Convert to request body
		requestBody, err := json.Marshal(mcpRequest)
		if err != nil {
			h.logger.Error("Failed to marshal MCP request for tool %s: %v", toolName, err)
			h.corsError(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Create new request
		newRequest := r.Clone(r.Context())
		newRequest.Body = io.NopCloser(bytes.NewReader(requestBody))
		newRequest.ContentLength = int64(len(requestBody))

		// Create a simple response recorder
		recorder := &mcpResponseRecorder{
			statusCode: 200,
			headers:    make(http.Header),
		}

		h.handleServerForward(recorder, newRequest, serverName, instance)

		// Parse and format the MCP response
		if recorder.statusCode == 200 && len(recorder.body) > 0 {
			var mcpResponse map[string]interface{}
			if err := json.Unmarshal(recorder.body, &mcpResponse); err == nil {
				// Check for MCP error
				if mcpError, hasError := mcpResponse["error"].(map[string]interface{}); hasError {
					errorResponse := map[string]interface{}{
						"error": mcpError["message"],
					}
					if data, hasData := mcpError["data"]; hasData {
						errorResponse["details"] = data
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(errorResponse)
					return
				}

				// Extract and format the successful result
				if result, exists := mcpResponse["result"]; exists {
					if resultMap, ok := result.(map[string]interface{}); ok {
						if content, exists := resultMap["content"]; exists {
							// Process the content like MCPO does
							cleanResult := h.processMCPContent(content)

							w.Header().Set("Content-Type", "application/json")
							json.NewEncoder(w).Encode(cleanResult)
							return
						}
					}
				}
			}
		}

		// Fallback to original response if formatting fails
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.statusCode)
		w.Write(recorder.body)
	} else {
		h.corsError(w, "Server not found", http.StatusNotFound)
	}
}

// processMCPContent processes MCP content like the official MCPO tool does
func (h *ProxyHandler) processMCPContent(content interface{}) interface{} {
	if contentArray, ok := content.([]interface{}); ok {
		var processed []interface{}

		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, ok := itemMap["type"].(string); ok {
					switch itemType {
					case "text":
						if text, ok := itemMap["text"].(string); ok {
							// Try to parse as JSON first
							var jsonData interface{}
							if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
								processed = append(processed, jsonData)
							} else {
								processed = append(processed, text)
							}
						} else {
							processed = append(processed, item)
						}
					case "image":
						if data, ok := itemMap["data"].(string); ok {
							if mimeType, ok := itemMap["mimeType"].(string); ok {
								imageURL := fmt.Sprintf("data:%s;base64,%s", mimeType, data)
								processed = append(processed, imageURL)
							} else {
								processed = append(processed, item)
							}
						} else {
							processed = append(processed, item)
						}
					default:
						processed = append(processed, item)
					}
				} else {
					processed = append(processed, item)
				}
			} else {
				processed = append(processed, item)
			}
		}

		// Return single item if array has only one element
		if len(processed) == 1 {
			return processed[0]
		}
		return processed
	}

	return content
}

func (h *ProxyHandler) findServerForTool(toolName string) (string, bool) {
	// Refresh cache if needed
	h.refreshToolCache()

	h.toolCacheMu.RLock()
	serverName, exists := h.toolCache[toolName]
	h.toolCacheMu.RUnlock()

	if exists {
		h.logger.Debug("Found tool %s in server %s via cache", toolName, serverName)
		return serverName, true
	}

	h.logger.Warning("Tool %s not found in cache", toolName)
	return "", false
}
