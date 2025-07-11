package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"mcpcompose/internal/constants"
	"mcpcompose/internal/dashboard"
	"mcpcompose/internal/protocol"
)

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

func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	dashboard.BroadcastActivity("INFO", "request", getServerNameFromPath(r.URL.Path), getClientIP(r),
		fmt.Sprintf("Request: %s to %s", r.Method, r.URL.Path),
		map[string]interface{}{
			"method":   r.Method,
			"endpoint": r.URL.Path,
		})

	h.logger.Info("Request: %s %s from %s (User-Agent: %s)", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("User-Agent"))

	// CORS Headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID, Mcp-Session-Id, X-Client-ID, X-MCP-Capabilities, X-Supports-Notifications")
	w.Header().Set("Access-Control-Expose-Headers", "Mcp-Session-Id, Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)

		return
	}

	path := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", constants.URLPathParts)

	// Handle OAuth endpoints FIRST - these should NOT require API key authentication
	if h.oauthEnabled && h.authServer != nil {
		if h.handleOAuthEndpoints(w, r, path) {

			return
		}
	}

	// NOW do authentication check for other endpoints
	if !h.authenticateAPIRequest(w, r) {

		return
	}

	// Handle server-specific OpenAPI specs
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
		handled = h.handleAPIEndpoints(w, r, path)
	}

	if handled {
		h.logger.Debug("Processed API request %s %s in %v", r.Method, r.URL.Path, time.Since(start))

		return
	}

	// CRITICAL FIX: Handle direct tool calls BEFORE server routing
	if len(parts) == 1 && parts[0] != "" && r.Method == http.MethodPost {
		toolName := parts[0]
		// First check if it's a known tool
		if h.isKnownTool(toolName) {
			h.logger.Info("Handling direct tool call for: %s", toolName)
			h.handleDirectToolCall(w, r, toolName)
			h.logger.Debug("Processed direct tool call %s %s in %v", r.Method, r.URL.Path, time.Since(start))

			return
		}

		// If not a known tool, check if it's a server name
		if _, exists := h.Manager.GetServerInstance(toolName); exists {
			h.logger.Info("Routing to server: %s", toolName)
			// This is a server, handle as server request
			goto handleServer
		}

		// Neither a tool nor a server
		h.logger.Warning("Unknown tool or server: %s", toolName)
		h.corsError(w, "Tool or server not found", http.StatusNotFound)

		return
	}

	if path == "/" {
		h.handleIndex(w, r)

		return
	}

handleServer:
	// Handle server routing
	if len(parts) > 0 && parts[0] != "api" {
		serverName := parts[0]
		if instance, exists := h.Manager.GetServerInstance(serverName); exists {
			if r.Method == http.MethodPost {
				// Use the new notification-aware method handler
				h.handleMCPMethodForwarding(w, r, serverName, instance)
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

func (h *ProxyHandler) handleOAuthEndpoints(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/.well-known/oauth-authorization-server":
		h.authServer.HandleDiscovery(w, r)

		return true
	case "/.well-known/oauth-protected-resource":
		if h.resourceMeta != nil {
			h.resourceMeta.HandleProtectedResourceMetadata(w, r)
		}

		return true
	case "/oauth/authorize":
		h.authServer.HandleAuthorize(w, r)

		return true
	case "/oauth/token":
		h.authServer.HandleToken(w, r)

		return true
	case "/oauth/userinfo": // Add this
		h.authServer.HandleUserInfo(w, r)

		return true
	case "/oauth/revoke": // Add this
		h.authServer.HandleRevoke(w, r)

		return true
	case "/oauth/register":
		h.authServer.HandleRegister(w, r)

		return true
	case "/oauth/callback":
		h.handleOAuthCallback(w, r)

		return true
	case "/api/oauth/status":
		h.handleOAuthStatus(w, r)

		return true
	case "/api/oauth/clients":
		h.handleOAuthClientsList(w, r)

		return true
	case "/api/oauth/scopes":
		h.handleOAuthScopesList(w, r)

		return true
	}

	// Handle OAuth client deletion (path starts with /api/oauth/clients/)
	if strings.HasPrefix(path, "/api/oauth/clients/") && r.Method == http.MethodDelete {
		h.handleOAuthClientDelete(w, r)

		return true
	}


	return false
}

func (h *ProxyHandler) handleAPIEndpoints(w http.ResponseWriter, r *http.Request, path string) bool {
	switch path {
	case "/api/reload":
		h.handleAPIReload(w, r)

		return true
	case "/api/servers":
		h.handleAPIServers(w, r)

		return true
	case "/api/status":
		h.handleAPIStatus(w, r)

		return true
	case "/api/discovery":
		h.handleDiscoveryEndpoint(w, r)

		return true
	case "/api/connections":
		h.handleConnectionsAPI(w, r)

		return true
	case "/api/subscriptions":
		h.handleSubscriptionsAPI(w, r)

		return true
	case "/api/notifications":
		h.handleNotificationsAPI(w, r)

		return true
	case "/openapi.json":
		h.handleOpenAPISpec(w, r)

		return true
	}

	// ADD CONTAINER ENDPOINTS HERE
	if strings.HasPrefix(path, "/api/containers/") {
		h.handleContainerAPI(w, r)

		return true
	}

	// Handle server-specific OAuth endpoints
	if strings.HasPrefix(path, "/api/servers/") {
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= constants.URLPathPartsExtended {
			switch pathParts[3] {
			case "oauth":
				h.handleServerOAuthConfig(w, r)

				return true
			case "test-oauth":
				h.handleServerOAuthTest(w, r)

				return true
			case "tokens":
				h.handleServerTokens(w, r)

				return true
			}
		}
	}


	return false
}

func (h *ProxyHandler) authenticateAPIRequest(w http.ResponseWriter, r *http.Request) bool {
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

			return false
		}
	}

	return true
}

func (h *ProxyHandler) handleMCPMethodForwarding(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance) {
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

	dashboard.BroadcastActivity("INFO", "request", serverName, getClientIP(r),
		fmt.Sprintf("MCP Request: %s", reqMethodVal),
		map[string]interface{}{
			"method":   reqMethodVal,
			"id":       reqIDVal,
			"endpoint": r.URL.Path,
		})

	// Handle notification-related methods first
	switch reqMethodVal {
	case "resources/subscribe":
		h.handleResourceSubscribe(w, r, serverName, requestPayload)

		return
	case "resources/unsubscribe":
		h.handleResourceUnsubscribe(w, r, serverName, requestPayload)

		return
	case "tools/list":
		// Check if client wants change notifications
		if h.supportsNotifications(r) {
			clientID := h.getClientID(r)
			sessionID := r.Header.Get("Mcp-Session-Id")
			notifyFunc := func(notification *protocol.ChangeNotification) error {

				return h.sendChangeNotificationToClient(clientID, notification)
			}
			h.changeNotificationManager.SubscribeToToolChanges(clientID, sessionID, notifyFunc)
			h.logger.Debug("Client %s subscribed to tool changes for server %s", clientID, serverName)
		}
		h.forwardToServerWithBody(w, r, serverName, instance, body, reqIDVal, reqMethodVal)

		return
	case "prompts/list":
		// Check if client wants change notifications
		if h.supportsNotifications(r) {
			clientID := h.getClientID(r)
			sessionID := r.Header.Get("Mcp-Session-Id")
			notifyFunc := func(notification *protocol.ChangeNotification) error {

				return h.sendChangeNotificationToClient(clientID, notification)
			}
			h.changeNotificationManager.SubscribeToPromptChanges(clientID, sessionID, notifyFunc)
			h.logger.Debug("Client %s subscribed to prompt changes for server %s", clientID, serverName)
		}
		h.forwardToServerWithBody(w, r, serverName, instance, body, reqIDVal, reqMethodVal)

		return
	default:
		h.forwardToServerWithBody(w, r, serverName, instance, body, reqIDVal, reqMethodVal)
	}
}

func (h *ProxyHandler) forwardToServerWithBody(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance, body []byte, reqIDVal interface{}, reqMethodVal string) {
	// Authentication check - validate before processing the request
	if !h.authenticateRequest(w, r, serverName, instance) {

		return // Authentication failed, response already sent
	}

	// Parse the already-read body back to requestPayload for non-HTTP protocols
	var requestPayload map[string]interface{}
	if err := json.Unmarshal(body, &requestPayload); err != nil {
		h.logger.Error("Failed to re-parse request payload for %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32700, "Invalid JSON in request")

		return
	}

	// ONLY handle proxy-specific standard methods, NOT server methods
	if isProxyStandardMethod(reqMethodVal) {
		h.handleProxyStandardMethod(w, r, requestPayload, reqIDVal, reqMethodVal)

		return
	}

	// FORWARD ALL OTHER METHODS TO THE ACTUAL MCP SERVERS
	// Get server config
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		h.logger.Error("Server config not found for %s", serverName)
		h.sendMCPError(w, reqIDVal, -32602, "Server configuration not found")

		return
	}

	// Determine transport protocol
	protocolType := serverConfig.Protocol
	if protocolType == "" {
		protocolType = "stdio" // default
	}

	h.logger.Info("Forwarding request to server '%s' using '%s' transport: Method=%s, ID=%v",
		serverName, protocolType, reqMethodVal, reqIDVal)

	// Route based on transport protocol - pass the body bytes
	switch protocolType {
	case "http":
		h.handleHTTPServerRequestWithBody(w, r, serverName, instance, body, reqIDVal, reqMethodVal)
	case "sse":
		h.handleSSEServerRequest(w, r, serverName, instance, requestPayload, reqIDVal, reqMethodVal)
	case "stdio":
		if serverConfig.StdioHosterPort > 0 {
			h.handleSocatSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		} else {
			h.handleSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		}
	default:
		h.logger.Error("Unsupported transport protocol '%s' for server %s", protocolType, serverName)
		h.sendMCPError(w, reqIDVal, -32602, fmt.Sprintf("Unsupported transport protocol: %s", protocolType))
	}
}

func (h *ProxyHandler) handleProxyStandardMethod(w http.ResponseWriter, _ *http.Request, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	h.logger.Info("Handling proxy standard MCP method '%s'", reqMethodVal)
	var params json.RawMessage
	if requestPayload["params"] != nil {
		paramsBytes, marshalErr := json.Marshal(requestPayload["params"])
		if marshalErr != nil {
			h.sendMCPError(w, reqIDVal, protocol.InvalidParams, "Failed to marshal parameters")

			return
		}
		params = paramsBytes
	}

	// Handle standard method
	if strings.HasPrefix(reqMethodVal, "notifications/") {
		// Handle notification
		err := h.standardHandler.HandleStandardNotification(reqMethodVal, params)
		if err != nil {
			if mcpErr, ok := err.(*protocol.MCPError); ok {
				h.sendMCPError(w, reqIDVal, mcpErr.Code, mcpErr.Message, mcpErr.Data)
			} else {
				h.sendMCPError(w, reqIDVal, protocol.InternalError, err.Error())
			}

			return
		}
		// Notifications don't have responses
		w.WriteHeader(http.StatusOK)

		return
	} else {
		// Handle request method
		response, err := h.standardHandler.HandleStandardMethod(reqMethodVal, params, reqIDVal)
		if err != nil {
			if mcpErr, ok := err.(*protocol.MCPError); ok {
				h.sendMCPError(w, reqIDVal, mcpErr.Code, mcpErr.Message, mcpErr.Data)
			} else {
				h.sendMCPError(w, reqIDVal, protocol.InternalError, err.Error())
			}

			return
		}
		// Send successful response
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Error("Failed to encode standard method response: %v", err)
		}

		return
	}
}

func (h *ProxyHandler) handleHTTPServerRequestWithBody(w http.ResponseWriter, r *http.Request, serverName string, _ *ServerInstance, body []byte, reqIDVal interface{}, reqMethodVal string) {
	conn, err := h.getServerConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get/create HTTP connection for %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32002, fmt.Sprintf("Proxy cannot connect to server '%s'", serverName))

		return
	}

	mcpCallTimeout := constants.HTTPExtendedTimeout
	if reqMethodVal == "initialize" {
		mcpCallTimeout = constants.HTTPLongTimeout
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

	// Use the pre-read body bytes directly
	responsePayload, err := h.forwardHTTPRequest(conn, body, mcpCallTimeout)
	if err != nil {
		dashboard.BroadcastActivity("ERROR", "request", serverName, getClientIP(r),
			fmt.Sprintf("Error: %s failed: %v", reqMethodVal, err),
			map[string]interface{}{"error": err.Error()})

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
	} else {
		dashboard.BroadcastActivity("INFO", "request", serverName, getClientIP(r),
			fmt.Sprintf("Response: %s completed successfully", reqMethodVal), nil)
	}

	h.logger.Info("Successfully forwarded HTTP request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
}

func (h *ProxyHandler) handleSSEServerRequest(w http.ResponseWriter, r *http.Request, serverName string, _ *ServerInstance, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	// Use optimal SSE connection (enhanced if available)
	conn, err := h.getOptimalSSEConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get/create SSE connection for %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32002, fmt.Sprintf("Proxy cannot connect to server '%s' via SSE", serverName))

		return
	}

	// Forward client's Mcp-Session-Id to the backend if present
	clientSessionID := r.Header.Get("Mcp-Session-Id")

	// Handle session ID based on connection type
	if enhancedConn, ok := conn.(*EnhancedMCPSSEConnection); ok {
		enhancedConn.mu.Lock()
		if clientSessionID != "" && enhancedConn.SessionID == "" {
			h.logger.Info("Using client-provided Mcp-Session-Id '%s' for enhanced SSE backend request to %s", clientSessionID, serverName)
			enhancedConn.SessionID = clientSessionID
		} else if clientSessionID != "" && clientSessionID != enhancedConn.SessionID {
			h.logger.Warning("Client Mcp-Session-Id '%s' differs from proxy's stored session '%s' for %s. Using proxy's.", clientSessionID, enhancedConn.SessionID, serverName)
		}
		enhancedConn.mu.Unlock()
	} else if standardConn, ok := conn.(*MCPSSEConnection); ok {
		standardConn.mu.Lock()
		if clientSessionID != "" && standardConn.SessionID == "" {
			h.logger.Info("Using client-provided Mcp-Session-Id '%s' for SSE backend request to %s", clientSessionID, serverName)
			standardConn.SessionID = clientSessionID
		} else if clientSessionID != "" && clientSessionID != standardConn.SessionID {
			h.logger.Warning("Client Mcp-Session-Id '%s' differs from proxy's stored session '%s' for %s. Using proxy's.", clientSessionID, standardConn.SessionID, serverName)
		}
		standardConn.mu.Unlock()
	}

	// Send request via optimal SSE connection
	responsePayload, err := h.sendOptimalSSERequest(serverName, requestPayload)
	if err != nil {
		dashboard.BroadcastActivity("ERROR", "request", serverName, getClientIP(r),
			fmt.Sprintf("Error: %s failed: %v", reqMethodVal, err),
			map[string]interface{}{"error": err.Error()})

		h.logger.Error("SSE request to %s (method: %s) failed: %v", serverName, reqMethodVal, err)
		errData := map[string]interface{}{"details": err.Error()}
		if enhancedConn, ok := conn.(*EnhancedMCPSSEConnection); ok {
			enhancedConn.mu.Lock()
			errData["targetEndpoint"] = enhancedConn.SSEEndpoint
			enhancedConn.mu.Unlock()
		} else if standardConn, ok := conn.(*MCPSSEConnection); ok {
			standardConn.mu.Lock()
			errData["targetEndpoint"] = standardConn.SSEEndpoint
			standardConn.mu.Unlock()
		}
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Error during SSE call to '%s'", serverName), errData)

		return
	}

	// Relay Mcp-Session-Id from backend server's response
	if enhancedConn, ok := conn.(*EnhancedMCPSSEConnection); ok {
		enhancedConn.mu.Lock()
		if enhancedConn.SessionID != "" {
			w.Header().Set("Mcp-Session-Id", enhancedConn.SessionID)
		}
		enhancedConn.mu.Unlock()
	} else if standardConn, ok := conn.(*MCPSSEConnection); ok {
		standardConn.mu.Lock()
		if standardConn.SessionID != "" {
			w.Header().Set("Mcp-Session-Id", standardConn.SessionID)
		}
		standardConn.mu.Unlock()
	}

	if err := json.NewEncoder(w).Encode(responsePayload); err != nil {
		h.logger.Error("Failed to encode/send response for %s: %v", serverName, err)
	} else {
		dashboard.BroadcastActivity("INFO", "request", serverName, getClientIP(r),
			fmt.Sprintf("Response: %s completed successfully", reqMethodVal), nil)
	}

	h.logger.Info("Successfully forwarded SSE request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
}

func (h *ProxyHandler) handleSessionTermination(w http.ResponseWriter, r *http.Request, serverName string) {
	clientSessionID := r.Header.Get("Mcp-Session-Id")
	if clientSessionID == "" {
		h.corsError(w, "Mcp-Session-Id header required for session termination (DELETE)", http.StatusBadRequest)

		return
	}

	h.logger.Info("Received DELETE request to terminate session '%s' for server '%s'", clientSessionID, serverName)

	// Ask the backend server to terminate its session
	conn, err := h.getServerConnection(serverName)
	if err != nil {
		h.logger.Warning("Cannot terminate session: No connection to server '%s' (%v)", serverName, err)
		h.corsError(w, "Server not connected via proxy", http.StatusBadGateway)

		return
	}

	conn.mu.Lock()
	if conn.SessionID != clientSessionID && conn.SessionID != "" {
		h.logger.Warning("Mcp-Session-Id mismatch during DELETE. Client '%s', Proxy's known '%s' for %s. Proceeding with client's ID.",
			clientSessionID, conn.SessionID, serverName)
	}
	conn.mu.Unlock()

	reqCtx, cancel := context.WithTimeout(h.ctx, constants.HTTPContextTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodDelete, conn.BaseURL, nil)
	if err != nil {
		h.logger.Error("Failed to create HTTP DELETE request for %s: %v", serverName, err)
		h.corsError(w, "Internal proxy error", http.StatusInternalServerError)

		return
	}

	httpReq.Header.Set("Mcp-Session-Id", clientSessionID)

	backendResp, err := h.httpClient.Do(httpReq)
	if err != nil {
		h.logger.Error("HTTP DELETE request to backend server %s failed: %v", serverName, err)
		h.corsError(w, "Failed to communicate with backend server for session termination", http.StatusBadGateway)

		return
	}
	defer func() {
		if err := backendResp.Body.Close(); err != nil {
			h.logger.Warning("Failed to close backend response body: %v", err)
		}
	}()

	// Clean up proxy's internal state for this session
	h.ConnectionMutex.Lock()
	if activeConn, exists := h.ServerConnections[serverName]; exists {
		activeConn.mu.Lock()
		if activeConn.SessionID == clientSessionID {
			h.logger.Info("Proxy forgetting session '%s' for server '%s' after DELETE.", clientSessionID, serverName)
			activeConn.Initialized = false
			activeConn.SessionID = ""
		}
		activeConn.mu.Unlock()
	}
	h.ConnectionMutex.Unlock()

	// Relay server's response status
	if backendResp.StatusCode == http.StatusMethodNotAllowed {
		w.WriteHeader(http.StatusMethodNotAllowed)
		if _, err := fmt.Fprint(w, `{"message": "Server does not allow client-initiated session termination via DELETE"}`); err != nil {
			h.logger.Error("Failed to write method not allowed response: %v", err)
		}

		return
	}

	w.WriteHeader(backendResp.StatusCode)
	if _, err := io.Copy(w, backendResp.Body); err != nil {
		h.logger.Error("Failed to copy backend response body: %v", err)
	}

	h.logger.Info("Session termination request for '%s' on server '%s' processed with status %d.", clientSessionID, serverName, backendResp.StatusCode)
}
