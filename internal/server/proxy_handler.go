package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"mcpcompose/internal/auth"
	"mcpcompose/internal/config"
	"mcpcompose/internal/logging"
	"mcpcompose/internal/protocol"
)

// ProxyHandler manages HTTP proxy connections to MCP servers
type ProxyHandler struct {
	Manager                   *Manager
	ConfigFile                string
	APIKey                    string
	EnableAPI                 bool
	ProxyStarted              time.Time
	ServerConnections         map[string]*MCPHTTPConnection
	SSEConnections            map[string]*MCPSSEConnection
	StdioConnections          map[string]*MCPSTDIOConnection
	ConnectionMutex           sync.RWMutex
	StdioMutex                sync.RWMutex
	SSEMutex                  sync.RWMutex
	logger                    *logging.Logger
	httpClient                *http.Client
	sseClient                 *http.Client
	GlobalRequestID           int
	GlobalIDMutex             sync.Mutex
	ctx                       context.Context
	cancel                    context.CancelFunc
	wg                        sync.WaitGroup
	toolCache                 map[string]string
	toolCacheMu               sync.RWMutex
	cacheExpiry               time.Time
	connectionStats           map[string]*ConnectionStats
	subscriptionManager       *protocol.SubscriptionManager
	changeNotificationManager *protocol.ChangeNotificationManager
	standardHandler           *protocol.StandardMethodHandler
	authServer                *auth.AuthorizationServer
	authMiddleware            *auth.AuthenticationMiddleware
	resourceMeta              *auth.ResourceMetadataHandler
	oauthEnabled              bool
}

// ConnectionStats tracks connection performance
type ConnectionStats struct {
	TotalRequests  int64
	FailedRequests int64
	TimeoutErrors  int64
	LastError      time.Time
	LastSuccess    time.Time
	mu             sync.RWMutex
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
		IdleConnTimeout:       5 * time.Minute,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableKeepAlives:     false,
		MaxConnsPerHost:       5,
	}

	logLvl := "info"
	if mgr.config != nil && mgr.config.Logging.Level != "" {
		logLvl = mgr.config.Logging.Level
	}

	logger := logging.NewLogger(logLvl)

	// CREATE STANDARD METHOD HANDLER
	serverInfo := protocol.ServerInfo{
		Name:    "mcp-compose-proxy",
		Version: "1.0.0",
	}
	capabilities := protocol.CapabilitiesOpts{
		Resources: &protocol.ResourcesOpts{ListChanged: true, Subscribe: true},
		Tools:     &protocol.ToolsOpts{ListChanged: true},
		Prompts:   &protocol.PromptsOpts{ListChanged: true},
		Roots:     &protocol.RootsOpts{ListChanged: true},
		Logging:   &protocol.LoggingOpts{},
		Sampling:  &protocol.SamplingOpts{},
	}

	// Initialize OAuth if enabled
	var authServer *auth.AuthorizationServer
	var authMiddleware *auth.AuthenticationMiddleware
	var resourceMeta *auth.ResourceMetadataHandler
	var oauthEnabled bool

	if mgr.config.OAuth != nil && mgr.config.OAuth.Enabled {
		authServer, authMiddleware, resourceMeta = initializeOAuth(mgr.config.OAuth, logger)
		oauthEnabled = true
		logger.Info("OAuth 2.1 authorization server initialized")
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
		},
		logger:                    logger,
		ctx:                       ctx,
		cancel:                    cancel,
		toolCache:                 make(map[string]string),
		cacheExpiry:               time.Now(),
		connectionStats:           make(map[string]*ConnectionStats),
		subscriptionManager:       protocol.NewSubscriptionManager(),
		changeNotificationManager: protocol.NewChangeNotificationManager(),
		standardHandler:           protocol.NewStandardMethodHandler(serverInfo, capabilities, logger),
		authServer:                authServer,
		authMiddleware:            authMiddleware,
		resourceMeta:              resourceMeta,
		oauthEnabled:              oauthEnabled,
	}

	// Start periodic token cleanup if OAuth is enabled
	if oauthEnabled && authServer != nil {
		go handler.startOAuthTokenCleanup()
	}

	handler.startConnectionMaintenance()
	handler.initializeNotificationSupport()

	return handler
}

func (h *ProxyHandler) getNextRequestID() int {
	h.GlobalIDMutex.Lock()
	defer h.GlobalIDMutex.Unlock()
	h.GlobalRequestID++
	return h.GlobalRequestID
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

	// CLEANUP NOTIFICATIONS
	if h.subscriptionManager != nil {
		h.subscriptionManager.CleanupExpiredSubscriptions(0)
	}
	if h.changeNotificationManager != nil {
		h.changeNotificationManager.CleanupInactiveSubscribers(0)
	}

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

func (h *ProxyHandler) corsError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Mcp-Session-Id")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Content-Type")
	http.Error(w, message, code)
}

func (h *ProxyHandler) sendMCPError(w http.ResponseWriter, id interface{}, code int, message string, data ...interface{}) {
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
	}
	if code == -32600 {
		httpStatus = http.StatusBadRequest
	}
	if code == -32601 {
		httpStatus = http.StatusNotFound
	}
	if code == -32602 {
		httpStatus = http.StatusBadRequest
	}
	if code >= -32099 && code <= -32000 {
		httpStatus = http.StatusInternalServerError
	}

	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(errResponse); err != nil {
		h.logger.Error("CRITICAL: Failed to encode and send MCP JSON-RPC error response to client: %v", err)
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

func isProxyStandardMethod(method string) bool {
	proxyMethods := map[string]bool{
		"initialize":                true,
		"notifications/initialized": true,
		"ping":                      true,
		"notifications/cancelled":   true,
	}
	return proxyMethods[method]
}
