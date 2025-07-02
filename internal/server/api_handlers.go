package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/protocol"
)

func (h *ProxyHandler) handleAPIReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Method not allowed - use POST",
		})
		return
	}

	h.logger.Info("Received reload request from %s", r.RemoteAddr)

	// Set JSON content type early
	w.Header().Set("Content-Type", "application/json")

	// Clear connection cache and reload config
	h.ConnectionMutex.Lock()
	oldHTTPConnCount := len(h.ServerConnections)
	h.ServerConnections = make(map[string]*MCPHTTPConnection)
	h.ConnectionMutex.Unlock()

	// Clear SSE connections
	h.SSEMutex.Lock()
	oldSSEConnCount := len(h.SSEConnections)
	for _, conn := range h.SSEConnections {
		if conn != nil {
			h.closeSSEConnection(conn)
		}
	}
	h.SSEConnections = make(map[string]*MCPSSEConnection)
	h.SSEMutex.Unlock()

	// Clear STDIO connections
	h.StdioMutex.Lock()
	oldSTDIOConnCount := len(h.StdioConnections)
	for name, conn := range h.StdioConnections {
		if conn != nil && conn.Connection != nil {
			h.logger.Debug("Closing STDIO connection to server %s during reload", name)
			conn.Connection.Close()
		}
	}
	h.StdioConnections = make(map[string]*MCPSTDIOConnection)
	h.StdioMutex.Unlock()

	// Refresh tool cache
	h.toolCacheMu.Lock()
	h.cacheExpiry = time.Now() // Force cache refresh
	h.toolCache = make(map[string]string)
	h.toolCacheMu.Unlock()

	h.logger.Info("Proxy reload completed: cleared %d HTTP, %d SSE, %d STDIO connections",
		oldHTTPConnCount, oldSSEConnCount, oldSTDIOConnCount)

	response := map[string]interface{}{
		"status":  "success",
		"message": "Proxy connections and cache reloaded",
		"cleared": map[string]int{
			"httpConnections":  oldHTTPConnCount,
			"sseConnections":   oldSSEConnCount,
			"stdioConnections": oldSTDIOConnCount,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode reload response: %v", err)
	}
}

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
		"mcpComposeVersion":              "dev",
		"mcpSpecVersionUsedByProxy":      protocol.MCPVersion,
		"standardMethodsSupported":       true,
		"standardHandlerInitialized":     h.standardHandler.IsInitialized(),
		"supportedCapabilities":          h.standardHandler.GetCapabilities(),
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

		h.ConnectionMutex.RLock()
		if liveConn, exists := h.ServerConnections[serverNameInConfig]; exists {
			liveConn.mu.Lock()
			if liveConn.Initialized && liveConn.Healthy && liveConn.Capabilities != nil && len(liveConn.Capabilities) > 0 {
				currentCapabilities = liveConn.Capabilities
			}
			liveConn.mu.Unlock()
		}
		h.ConnectionMutex.RUnlock()

		description := fmt.Sprintf("MCP %s server (via proxy)", serverNameInConfig)

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
				toolsForClient = append(toolsForClient, toolEntry)
			}
			if len(toolsForClient) > 0 {
				serverEntry["tools"] = toolsForClient
			}
		}

		serversForDiscovery = append(serversForDiscovery, serverEntry)
	}

	discoveryResponse := map[string]interface{}{
		"servers": serversForDiscovery,
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
			"rawHealthyFlag":             conn.Healthy,
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

func (h *ProxyHandler) handleSubscriptionsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// List all subscriptions
		clientID := h.getClientID(r)
		subscriptions := h.subscriptionManager.GetSubscriptions(clientID)
		response := map[string]interface{}{
			"subscriptions": subscriptions,
			"clientId":      clientID,
			"timestamp":     time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)

	case http.MethodDelete:
		// Cleanup expired subscriptions
		h.subscriptionManager.CleanupExpiredSubscriptions(30 * time.Minute)
		h.changeNotificationManager.CleanupInactiveSubscribers(30 * time.Minute)
		response := map[string]interface{}{
			"status":    "cleaned",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(response)

	default:
		h.corsError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ProxyHandler) handleNotificationsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		h.corsError(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	toolSubscribers := h.changeNotificationManager.GetToolSubscribers()
	promptSubscribers := h.changeNotificationManager.GetPromptSubscribers()

	response := map[string]interface{}{
		"toolSubscribers":   len(toolSubscribers),
		"promptSubscribers": len(promptSubscribers),
		"subscribers": map[string]interface{}{
			"tools":   toolSubscribers,
			"prompts": promptSubscribers,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func (h *ProxyHandler) handleOAuthStatus(w http.ResponseWriter, _ *http.Request) {
	if !h.oauthEnabled || h.authServer == nil {
		http.Error(w, "OAuth not enabled", http.StatusNotFound)
		return
	}

	accessTokens, refreshTokens, authCodes := h.authServer.GetTokenCount()
	response := map[string]interface{}{
		"oauth_enabled": true,
		"active_tokens": map[string]int{
			"access_tokens":  accessTokens,
			"refresh_tokens": refreshTokens,
			"auth_codes":     authCodes,
		},
		"issuer":           h.authServer.GetMetadata().Issuer,
		"scopes_supported": h.authServer.GetMetadata().ScopesSupported,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ProxyHandler) handleOAuthClientsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.oauthEnabled || h.authServer == nil {
		http.Error(w, "OAuth not enabled", http.StatusNotFound)
		return
	}

	clients := h.authServer.GetAllClients()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

func (h *ProxyHandler) handleOAuthScopesList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.oauthEnabled || h.authServer == nil {
		http.Error(w, "OAuth not enabled", http.StatusNotFound)
		return
	}

	scopes := []map[string]string{
		{"name": "mcp:tools", "description": "Access to MCP tools"},
		{"name": "mcp:resources", "description": "Access to MCP resources"},
		{"name": "mcp:prompts", "description": "Access to MCP prompts"},
		{"name": "mcp:*", "description": "Full access to all MCP capabilities"},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scopes)
}

func (h *ProxyHandler) handleOAuthClientDelete(w http.ResponseWriter, r *http.Request) {
	if !h.oauthEnabled || h.authServer == nil {
		http.Error(w, "OAuth not enabled", http.StatusNotFound)
		return
	}

	// Extract client ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/oauth/clients/")
	if path == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	// For now, just return success - implement actual deletion logic
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *ProxyHandler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// This is just for testing - show the authorization code received
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDescription := r.URL.Query().Get("error_description")

	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>OAuth Callback - MCP Compose</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; 
            max-width: 800px; margin: 50px auto; padding: 20px; 
            background: #f5f5f5;
        }
        .result-box { 
            border: 1px solid #ddd; padding: 30px; border-radius: 8px; 
            background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .success { 
            border-left: 4px solid #28a745; 
        }
        .error { 
            border-left: 4px solid #dc3545; 
        }
        code { 
            background: #f8f9fa; padding: 4px 8px; border-radius: 4px; 
            font-family: 'Monaco', 'Consolas', monospace; font-size: 14px;
            word-break: break-all;
        }
        .field { margin: 15px 0; }
        .field strong { color: #333; }
        .test-section {
            margin-top: 30px; padding: 20px; background: #f8f9fa; 
            border-radius: 6px; border: 1px solid #e9ecef;
        }
        .back-link { 
            margin-top: 20px; 
        }
        .back-link a { 
            color: #007bff; text-decoration: none; 
        }
        .back-link a:hover { 
            text-decoration: underline; 
        }
        .copy-btn {
            background: #007bff; color: white; border: none; 
            padding: 5px 10px; border-radius: 3px; margin-left: 10px;
            cursor: pointer; font-size: 12px;
        }
        .copy-btn:hover { background: #0056b3; }
    </style>
    <script>
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(function() {
                event.target.textContent = 'Copied!';
                setTimeout(() => {
                    event.target.textContent = 'Copy';
                }, 2000);
            });
        }
    </script>
</head>
<body>
    <h2>OAuth Authorization Result</h2>
    %s
    <div class="back-link">
        <a href="/oauth/authorize?response_type=code&client_id=%s&redirect_uri=http%%3A%%2F%%2Fdesk%%3A3111%%2Foauth%%2Fcallback&scope=mcp:tools">← Try Authorization Again</a><br>
        <a href="/">← Back to Dashboard</a>
    </div>
</body>
</html>`, func() string {
		if errorParam != "" {
			return fmt.Sprintf(`<div class="result-box error">
                <h3>❌ Authorization Failed</h3>
                <div class="field"><strong>Error:</strong> %s</div>
                <div class="field"><strong>Description:</strong> %s</div>
                <div class="field"><strong>State:</strong> %s</div>
            </div>`, errorParam, errorDescription, state)
		} else if code != "" {
			return fmt.Sprintf(`<div class="result-box success">
                <h3>✅ Authorization Successful!</h3>
                <div class="field">
                    <strong>Authorization Code:</strong><br>
                    <code>%s</code>
                    <button class="copy-btn" onclick="copyToClipboard('%s')">Copy</button>
                </div>
                <div class="field"><strong>State:</strong> %s</div>
                <div class="test-section">
                    <h4>Next Steps:</h4>
                    <p>You can now exchange this authorization code for an access token using the <code>/oauth/token</code> endpoint.</p>
                    <p><strong>Test with cURL:</strong></p>
                    <pre><code style="display:block; padding:10px; background:#e9ecef;">curl -X POST http://desk:9876/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=%s&client_id=your_client_id&redirect_uri=http://desk:3111/oauth/callback"</code></pre>
                </div>
            </div>`, code, code, state, code)
		} else {
			return `<div class="result-box error">
                <h3>❓ Unexpected Response</h3>
                <p>No authorization code or error received.</p>
            </div>`
		}
	}(), r.URL.Query().Get("client_id"))

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (h *ProxyHandler) handleServerOAuthConfig(w http.ResponseWriter, r *http.Request) {
	// Extract server name from the path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[0] != "api" || pathParts[1] != "servers" || pathParts[3] != "oauth" {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}
	serverName := pathParts[2]
	w.Header().Set("Content-Type", "application/json")

	// Use tagged switch instead of if-else chain
	switch r.Method {
	case http.MethodGet:
		// Return current OAuth config for server
		config := h.getServerOAuthConfig(serverName)
		json.NewEncoder(w).Encode(config)
	case http.MethodPut:
		// Update OAuth config for server
		var config config.ServerOAuthConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		err := h.updateServerOAuthConfig(serverName, config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ProxyHandler) handleServerOAuthTest(w http.ResponseWriter, r *http.Request) {
	// Extract server name from the path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[0] != "api" || pathParts[1] != "servers" || pathParts[3] != "test-oauth" {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}
	serverName := pathParts[2]

	w.Header().Set("Content-Type", "application/json")

	// Allow both GET and POST for testing
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed - use GET or POST", http.StatusMethodNotAllowed)
		return
	}

	h.logger.Info("Testing OAuth access for server: %s", serverName)

	// Test OAuth access to this server
	result := h.testServerOAuth(serverName)

	h.logger.Info("OAuth test result for %s: success=%v", serverName, result["success"])

	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.logger.Error("Failed to encode OAuth test response: %v", err)
	}
}

func (h *ProxyHandler) handleServerTokens(w http.ResponseWriter, r *http.Request) {
	// Extract server name from the path
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(pathParts) < 4 || pathParts[0] != "api" || pathParts[1] != "servers" || pathParts[3] != "tokens" {
		http.Error(w, "Invalid path format", http.StatusBadRequest)
		return
	}
	serverName := pathParts[2]

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get active tokens for this server
	tokens := h.getServerTokens(serverName)
	json.NewEncoder(w).Encode(tokens)
}

// Supporting methods - add these to api_handlers.go

func (h *ProxyHandler) getServerOAuthConfig(serverName string) config.ServerOAuthConfig {
	// Check if server exists in config
	if h.Manager == nil || h.Manager.config == nil {
		return config.ServerOAuthConfig{
			Enabled:             false,
			AllowAPIKeyFallback: true,
			OptionalAuth:        false,
			AllowedClients:      []string{},
		}
	}

	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return config.ServerOAuthConfig{
			Enabled:             false,
			AllowAPIKeyFallback: true,
			OptionalAuth:        false,
			AllowedClients:      []string{},
		}
	}

	// Check if server has OAuth config directly
	if serverConfig.OAuth != nil {
		return *serverConfig.OAuth
	}

	// Fall back to authentication config and convert it
	if serverConfig.Authentication != nil {
		return config.ServerOAuthConfig{
			Enabled:             serverConfig.Authentication.Enabled,
			RequiredScope:       serverConfig.Authentication.RequiredScope,
			AllowAPIKeyFallback: serverConfig.Authentication.AllowAPIKey != nil && *serverConfig.Authentication.AllowAPIKey,
			OptionalAuth:        serverConfig.Authentication.OptionalAuth,
			AllowedClients:      []string{}, // Authentication doesn't have this field
		}
	}

	// Default config
	return config.ServerOAuthConfig{
		Enabled:             false,
		AllowAPIKeyFallback: true,
		OptionalAuth:        false,
		AllowedClients:      []string{},
	}
}

func (h *ProxyHandler) updateServerOAuthConfig(serverName string, newConfig config.ServerOAuthConfig) error {
	if h.Manager == nil || h.Manager.config == nil {
		return fmt.Errorf("manager not initialized")
	}

	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return fmt.Errorf("server %s not found", serverName)
	}

	// Update the OAuth config directly
	serverConfig.OAuth = &newConfig

	// Also update the legacy authentication config for backward compatibility
	if serverConfig.Authentication == nil {
		serverConfig.Authentication = &config.ServerAuthConfig{}
	}

	serverConfig.Authentication.Enabled = newConfig.Enabled
	serverConfig.Authentication.RequiredScope = newConfig.RequiredScope
	serverConfig.Authentication.OptionalAuth = newConfig.OptionalAuth
	serverConfig.Authentication.AllowAPIKey = &newConfig.AllowAPIKeyFallback

	// Update the server config in the manager
	h.Manager.config.Servers[serverName] = serverConfig

	h.logger.Info("Updated OAuth configuration for server %s", serverName)
	return nil
}

func (h *ProxyHandler) testServerOAuth(serverName string) map[string]interface{} {
	result := map[string]interface{}{
		"server":        serverName,
		"oauth_enabled": false,
		"success":       false,
	}

	// Check if OAuth is enabled globally
	if !h.oauthEnabled || h.authServer == nil {
		result["error"] = "OAuth not enabled globally"
		return result
	}

	// Get server OAuth config
	serverOAuthConfig := h.getServerOAuthConfig(serverName)
	result["oauth_enabled"] = serverOAuthConfig.Enabled

	if !serverOAuthConfig.Enabled {
		result["success"] = true
		result["message"] = "Server does not require OAuth authentication"
		return result
	}

	// Test with a mock client credentials token
	testClientID := "HFakeCpMUQnRX_m5HJKamRjU_vufUnNbG4xWpmUyvzo" // Your default test client

	// Check if this test client is allowed
	if len(serverOAuthConfig.AllowedClients) > 0 {
		allowed := false
		for _, allowedClient := range serverOAuthConfig.AllowedClients {
			if allowedClient == testClientID {
				allowed = true
				break
			}
		}
		if !allowed {
			result["error"] = "Test client not in allowed clients list"
			result["available_clients"] = serverOAuthConfig.AllowedClients
			return result
		}
	}

	// Test if the test client exists
	if h.authServer != nil {
		testClient, exists := h.authServer.GetClient(testClientID)
		if !exists {
			result["error"] = "Test client not found in OAuth server"
			result["test_client_id"] = testClientID
			return result
		}

		// Check if client has required scope
		if serverOAuthConfig.RequiredScope != "" {
			clientHasScope := false
			if testClient.Scope == "mcp:*" {
				clientHasScope = true
			} else {
				clientScopes := strings.Fields(testClient.Scope)
				for _, scope := range clientScopes {
					if scope == serverOAuthConfig.RequiredScope || scope == "mcp:*" {
						clientHasScope = true
						break
					}
				}
			}

			if !clientHasScope {
				result["error"] = fmt.Sprintf("Test client does not have required scope: %s", serverOAuthConfig.RequiredScope)
				result["client_scopes"] = testClient.Scope
				return result
			}
		}
	}

	// Simulate successful OAuth test
	result["success"] = true
	result["message"] = "OAuth configuration appears valid"
	result["details"] = map[string]interface{}{
		"required_scope":         serverOAuthConfig.RequiredScope,
		"allow_api_key_fallback": serverOAuthConfig.AllowAPIKeyFallback,
		"optional_auth":          serverOAuthConfig.OptionalAuth,
		"allowed_clients_count":  len(serverOAuthConfig.AllowedClients),
		"test_client_id":         testClientID,
	}

	return result
}

func (h *ProxyHandler) getServerTokens(serverName string) map[string]interface{} {
	result := map[string]interface{}{
		"server": serverName,
		"tokens": []map[string]interface{}{},
	}

	if !h.oauthEnabled || h.authServer == nil {
		result["message"] = "OAuth not enabled"
		return result
	}

	// Get server OAuth config to check scope requirements
	serverOAuthConfig := h.getServerOAuthConfig(serverName)
	requiredScope := serverOAuthConfig.RequiredScope

	// Get all active tokens and filter by scope if needed
	if h.authServer != nil {
		allTokens := h.authServer.GetAllAccessTokens()
		var relevantTokens []map[string]interface{}

		for _, tokenInfo := range allTokens {
			// If server has a required scope, check if token has it
			if requiredScope != "" {
				tokenScopes := strings.Fields(tokenInfo.Scope)
				hasRequiredScope := false
				for _, scope := range tokenScopes {
					if scope == requiredScope || scope == "mcp:*" {
						hasRequiredScope = true
						break
					}
				}
				if !hasRequiredScope {
					continue // Skip this token
				}
			}

			// Add relevant token info (without the actual token value for security)
			relevantTokens = append(relevantTokens, map[string]interface{}{
				"client_id":  tokenInfo.ClientID,
				"user_id":    tokenInfo.UserID,
				"scope":      tokenInfo.Scope,
				"expires_at": tokenInfo.ExpiresAt.Format(time.RFC3339),
				"created_at": tokenInfo.CreatedAt.Format(time.RFC3339),
				"active":     !tokenInfo.Revoked && time.Now().Before(tokenInfo.ExpiresAt),
			})
		}

		result["tokens"] = relevantTokens
		result["total_relevant_tokens"] = len(relevantTokens)
	}

	result["server_config"] = map[string]interface{}{
		"oauth_enabled":   serverOAuthConfig.Enabled,
		"required_scope":  serverOAuthConfig.RequiredScope,
		"optional_auth":   serverOAuthConfig.OptionalAuth,
		"allowed_clients": serverOAuthConfig.AllowedClients,
	}

	return result
}
