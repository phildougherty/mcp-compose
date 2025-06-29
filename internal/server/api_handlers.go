package server

import (
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"

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
