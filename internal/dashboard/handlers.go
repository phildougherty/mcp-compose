package dashboard

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func (d *DashboardServer) handleServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Forward to proxy server
	resp, err := d.proxyRequest("/api/servers")
	if err != nil {
		d.logger.Error("Failed to get servers from proxy: %v", err)
		http.Error(w, "Failed to get servers", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := d.proxyRequest("/api/status")
	if err != nil {
		d.logger.Error("Failed to get status from proxy: %v", err)
		http.Error(w, "Failed to get status", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := d.proxyRequest("/api/connections")
	if err != nil {
		d.logger.Error("Failed to get connections from proxy: %v", err)
		http.Error(w, "Failed to get connections", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleContainers(w http.ResponseWriter, r *http.Request) {
	// Extract container name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/containers/")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	containerName := parts[0]
	action := parts[1]

	switch action {
	case "logs":
		// Try proxying first, fall back to local if proxying fails
		if d.tryProxyContainerLogs(w, r, containerName) {
			return
		}
		// Fallback to local handling
		d.handleContainerLogs(w, r, containerName)
	case "stats":
		// Try proxying first, fall back to local if proxying fails
		if d.tryProxyContainerStats(w, r, containerName) {
			return
		}
		// Fallback to local handling
		d.handleContainerStats(w, r, containerName)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

func (d *DashboardServer) tryProxyContainerLogs(w http.ResponseWriter, r *http.Request, containerName string) bool {
	endpoint := fmt.Sprintf("/api/containers/%s/logs", containerName)
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}

	resp, err := d.proxyRequest(endpoint)
	if err != nil {
		d.logger.Debug("Failed to proxy container logs, will try local: %v", err)
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
	return true
}

func (d *DashboardServer) tryProxyContainerStats(w http.ResponseWriter, r *http.Request, containerName string) bool {
	endpoint := fmt.Sprintf("/api/containers/%s/stats", containerName)
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}

	resp, err := d.proxyRequest(endpoint)
	if err != nil {
		d.logger.Debug("Failed to proxy container stats, will try local: %v", err)
		return false
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
	return true
}

func (d *DashboardServer) handleServerStart(w http.ResponseWriter, r *http.Request) {
	d.handleServerAction(w, r, "start")
}

func (d *DashboardServer) handleServerStop(w http.ResponseWriter, r *http.Request) {
	d.handleServerAction(w, r, "stop")
}

func (d *DashboardServer) handleServerRestart(w http.ResponseWriter, r *http.Request) {
	d.handleServerAction(w, r, "restart")
}

func (d *DashboardServer) handleProxyReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp, err := d.proxyRequest("/api/reload")
	if err != nil {
		d.logger.Error("Failed to reload proxy: %v", err)
		http.Error(w, "Failed to reload proxy", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

// getContainerLogs retrieves logs from a Docker container
func (d *DashboardServer) getContainerLogs(containerName, tail string, follow bool) ([]string, error) {
	args := []string{"logs", "--tail", tail}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, containerName)
	cmd := exec.Command("docker", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker logs command failed: %w", err)
	}
	lines := strings.Split(string(output), "\n")
	// Filter out empty lines
	var filteredLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			filteredLines = append(filteredLines, line)
		}
	}
	return filteredLines, nil
}

func (d *DashboardServer) handleActivityReceive(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var activity ActivityMessage
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		log.Printf("[ACTIVITY] Invalid activity JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	// Forward to local broadcaster
	select {
	case activityBroadcaster.broadcast <- activity:
		// Success
	default:
		log.Printf("[ACTIVITY] Channel full, dropping activity")
	}
	w.WriteHeader(http.StatusAccepted)
}

func (d *DashboardServer) handleActivityWebSocket(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)
	log.Printf("[WEBSOCKET] 🔌 New WebSocket connection from %s", clientIP)
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WEBSOCKET] ❌ Failed to upgrade connection: %v", err)
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			d.logger.Error("Failed to close websocket connection: %v", err)
		}
	}()
	log.Printf("[WEBSOCKET] ✅ WebSocket upgraded successfully")
	safeConn := &SafeWebSocketConn{conn: conn}
	activityBroadcaster.register <- safeConn
	defer func() {
		activityBroadcaster.unregister <- safeConn
		log.Printf("[WEBSOCKET] 🔌 Connection closed")
	}()
	// Keep connection alive
	for {
		if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
			break
		}
		time.Sleep(30 * time.Second)
	}
}

func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}
	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func (d *DashboardServer) handleServerDocs(w http.ResponseWriter, r *http.Request) {
	// Extract server name from path /api/server-docs/{serverName}
	path := strings.TrimPrefix(r.URL.Path, "/api/server-docs/")
	if path == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}
	// Proxy request to the MCP proxy
	resp, err := d.proxyRequest(fmt.Sprintf("/%s/docs", path))
	if err != nil {
		d.logger.Error("Failed to get server docs for %s: %v", path, err)
		http.Error(w, "Failed to get server docs", http.StatusInternalServerError)
		return
	}
	// Rewrite the HTML content to fix internal links
	htmlContent := string(resp)
	// Replace proxy-relative URLs with dashboard API URLs
	htmlContent = strings.ReplaceAll(htmlContent, fmt.Sprintf("/%s/openapi.json", path), fmt.Sprintf("/api/server-openapi/%s", path))
	htmlContent = strings.ReplaceAll(htmlContent, fmt.Sprintf("/%s", path), fmt.Sprintf("/api/server-direct/%s", path))
	// Also fix any absolute proxy URLs if they exist
	htmlContent = strings.ReplaceAll(htmlContent, "http://192.168.86.201:9876/", "/api/server-proxy/")
	// Fix any other common patterns
	htmlContent = strings.ReplaceAll(htmlContent, `href="/`, `href="/api/server-proxy/`)
	htmlContent = strings.ReplaceAll(htmlContent, `href="api/`, `href="/api/`) // Don't double-prefix API routes
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(htmlContent)); err != nil {
		d.logger.Error("Failed to write HTML content: %v", err)
	}
}

func (d *DashboardServer) handleServerOpenAPI(w http.ResponseWriter, r *http.Request) {
	// Extract server name from path /api/server-openapi/{serverName}
	path := strings.TrimPrefix(r.URL.Path, "/api/server-openapi/")
	if path == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}
	// Proxy request to the MCP proxy
	resp, err := d.proxyRequest(fmt.Sprintf("/%s/openapi.json", path))
	if err != nil {
		d.logger.Error("Failed to get server OpenAPI for %s: %v", path, err)
		http.Error(w, "Failed to get server OpenAPI", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleServerDirect(w http.ResponseWriter, r *http.Request) {
	// Extract server name from path /api/server-direct/{serverName}
	path := strings.TrimPrefix(r.URL.Path, "/api/server-direct/")
	if path == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}
	// Check if server exists
	servers, err := d.proxyRequest("/api/servers")
	if err != nil {
		d.logger.Error("Failed to get servers list: %v", err)
		http.Error(w, "Failed to verify server exists", http.StatusInternalServerError)
		return
	}
	var serversMap map[string]interface{}
	if err := json.Unmarshal(servers, &serversMap); err != nil {
		d.logger.Error("Failed to parse servers response: %v", err)
		http.Error(w, "Failed to parse servers list", http.StatusInternalServerError)
		return
	}
	if _, exists := serversMap[path]; !exists {
		http.Error(w, fmt.Sprintf("Server '%s' not found", path), http.StatusNotFound)
		return
	}
	// Proxy GET request to the specific server
	resp, err := d.proxyRequest(fmt.Sprintf("/%s", path))
	if err != nil {
		d.logger.Error("Failed to get server details for %s: %v", path, err)
		http.Error(w, fmt.Sprintf("Failed to access server '%s'", path), http.StatusInternalServerError)
		return
	}
	// Determine content type based on what the proxy returns
	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleServerLogs(w http.ResponseWriter, r *http.Request) {
	// Extract server name from path /api/server-logs/{serverName}
	path := strings.TrimPrefix(r.URL.Path, "/api/server-logs/")
	if path == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}
	// Get logs from container (existing functionality)
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	containerName := "mcp-compose-" + path
	logs, err := d.getContainerLogs(containerName, tail, false)
	if err != nil {
		d.logger.Error("Failed to get logs for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}
	response := map[string]interface{}{
		"container": containerName,
		"logs":      logs,
		"timestamp": time.Now().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleOAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Proxy to main server's OAuth status endpoint
	resp, err := d.proxyRequest("/api/oauth/status")
	if err != nil {
		d.logger.Error("Failed to get OAuth status from proxy: %v", err)
		http.Error(w, "Failed to get OAuth status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleOAuthClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Get clients list - proxy to main server
		resp, err := d.proxyRequest("/api/oauth/clients")
		if err != nil {
			d.logger.Error("Failed to get OAuth clients from proxy: %v", err)
			http.Error(w, "Failed to get OAuth clients", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	case http.MethodDelete:
		// Extract client ID from path
		path := strings.TrimPrefix(r.URL.Path, "/api/oauth/clients/")
		if path == "" {
			http.Error(w, "Client ID required", http.StatusBadRequest)
			return
		}

		// Proxy DELETE request to main server
		resp, err := d.proxyDeleteRequest(fmt.Sprintf("/api/oauth/clients/%s", path))
		if err != nil {
			d.logger.Error("Failed to delete OAuth client: %v", err)
			http.Error(w, "Failed to delete OAuth client", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (d *DashboardServer) handleOAuthRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Proxy POST request to main server's registration endpoint
	resp, err := d.proxyPostRequest("/oauth/register", body)
	if err != nil {
		d.logger.Error("Failed to register OAuth client: %v", err)
		http.Error(w, "Failed to register OAuth client", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

// Helper methods for different HTTP methods
func (d *DashboardServer) proxyPostRequest(endpoint string, body []byte) ([]byte, error) {
	url := d.proxyURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (d *DashboardServer) proxyDeleteRequest(endpoint string) ([]byte, error) {
	url := d.proxyURL + endpoint
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (d *DashboardServer) handleOAuthScopes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Proxy to main server's OAuth scopes endpoint
	resp, err := d.proxyRequest("/api/oauth/scopes")
	if err != nil {
		d.logger.Error("Failed to get OAuth scopes from proxy: %v", err)
		// Return default scopes if proxy doesn't have this endpoint
		defaultScopes := []map[string]string{
			{"name": "mcp:tools", "description": "Access to MCP tools"},
			{"name": "mcp:resources", "description": "Access to MCP resources"},
			{"name": "mcp:prompts", "description": "Access to MCP prompts"},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(defaultScopes); err != nil {
			d.logger.Error("Failed to encode default scopes: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleAuditEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Build query string from request parameters
	queryString := r.URL.RawQuery
	endpoint := "/api/audit/entries"
	if queryString != "" {
		endpoint += "?" + queryString
	}

	// Proxy to main server's audit entries endpoint
	resp, err := d.proxyRequest(endpoint)
	if err != nil {
		d.logger.Error("Failed to get audit entries from proxy: %v", err)
		// Return empty audit entries if proxy doesn't have this endpoint
		response := map[string]interface{}{
			"entries": []interface{}{},
			"total":   0,
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			d.logger.Error("Failed to encode response: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleAuditStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Proxy to main server's audit stats endpoint
	resp, err := d.proxyRequest("/api/audit/stats")
	if err != nil {
		d.logger.Error("Failed to get audit stats from proxy: %v", err)
		// Return empty audit stats if proxy doesn't have this endpoint
		response := map[string]interface{}{
			"total_entries": 0,
			"success_rate":  100.0,
			"event_counts":  map[string]int{},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			d.logger.Error("Failed to encode response: %v", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleOAuthToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	// Create request to main server
	url := d.proxyURL + "/oauth/token"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		d.logger.Error("Failed to create OAuth token request: %v", err)
		http.Error(w, "Failed to request token", http.StatusInternalServerError)
		return
	}

	// Copy headers from original request
	req.Header.Set("Content-Type", r.Header.Get("Content-Type"))
	if r.Header.Get("Authorization") != "" {
		req.Header.Set("Authorization", r.Header.Get("Authorization"))
	}

	// Make request to main server
	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Error("OAuth token request failed: %v", err)
		http.Error(w, "Failed to request token", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

	// Copy response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		d.logger.Error("Failed to read OAuth token response: %v", err)
		http.Error(w, "Failed to request token", http.StatusInternalServerError)
		return
	}

	// Copy status code and headers
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	if _, err := w.Write(responseBody); err != nil {
		d.logger.Error("Failed to write response body: %v", err)
	}
}

func (d *DashboardServer) handleOAuthAuthorize(w http.ResponseWriter, r *http.Request) {
	// Build query string from request parameters
	queryString := r.URL.RawQuery
	endpoint := "/oauth/authorize"
	if queryString != "" {
		endpoint += "?" + queryString
	}

	if r.Method == http.MethodPost {
		// Handle POST requests (form submissions)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		proxyURL := d.proxyURL + endpoint
		req, err := http.NewRequest("POST", proxyURL, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

		// CRITICAL: Set a custom redirect policy - don't follow redirects!
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse // Don't follow redirects automatically
			},
		}

		resp, err := client.Do(req)
		if err != nil {
			d.logger.Error("OAuth authorize request failed: %v", err)
			http.Error(w, "Failed to process authorization", http.StatusInternalServerError)
			return
		}
		defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

		// Handle redirects manually
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location != "" {
				d.logger.Info("OAuth server wants to redirect to: %s", location)

				// Parse the redirect URL
				redirectURL, err := url.Parse(location)
				if err != nil {
					d.logger.Error("Failed to parse redirect URL: %v", err)
					http.Error(w, "Invalid redirect URL", http.StatusInternalServerError)
					return
				}

				// If it's a callback URL, redirect to our local callback endpoint
				if strings.Contains(redirectURL.Path, "/oauth/callback") {
					localCallback := "/oauth/callback?" + redirectURL.RawQuery
					d.logger.Info("Redirecting browser to local callback: %s", localCallback)
					http.Redirect(w, r, localCallback, http.StatusFound)
					return
				} else {
					// For other redirects, redirect as-is
					d.logger.Info("Redirecting browser to: %s", location)
					http.Redirect(w, r, location, resp.StatusCode)
					return
				}
			}
		}

		// For non-redirect responses, pass through
		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read response", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(responseBody)
		return
	}

	// For GET requests, proxy to main server
	resp, err := d.proxyRequest(endpoint)
	if err != nil {
		d.logger.Error("Failed to get OAuth authorize from proxy: %v", err)
		http.Error(w, "Failed to process authorization", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
}

func (d *DashboardServer) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Extract authorization code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")
	errorDescription := r.URL.Query().Get("error_description")

	// Build query string to forward to the proxy
	queryString := r.URL.RawQuery
	endpoint := "/oauth/callback"
	if queryString != "" {
		endpoint += "?" + queryString
	}

	// For GET requests (standard OAuth callback), proxy to main server and enhance the response
	if r.Method == http.MethodGet {
		// Get the callback response from the proxy
		resp, err := d.proxyRequest(endpoint)
		if err != nil {
			d.logger.Error("Failed to get OAuth callback from proxy: %v", err)
			// Create our own callback response - NOW PASSING r AS PARAMETER
			html := d.createCallbackHTML(code, state, errorParam, errorDescription, fmt.Sprintf("Proxy error: %v", err), r)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(html))
			return
		}

		// Enhancement: If we got a successful response from proxy, we can enhance it
		// For now, just pass through the proxy response
		w.Header().Set("Content-Type", "text/html")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}
		return
	}

	// For POST requests (if needed), forward to proxy
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		postResp, err := d.proxyPostRequest(endpoint, body)
		if err != nil {
			d.logger.Error("Failed to post OAuth callback to proxy: %v", err)
			http.Error(w, "Failed to process callback", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write(postResp)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (d *DashboardServer) createCallbackHTML(code, state, errorParam, errorDescription, proxyError string, r *http.Request) string {
	var content string
	var title string

	if errorParam != "" {
		title = "OAuth Authorization Failed"
		content = fmt.Sprintf(`
            <div class="error-box">
                <h3>❌ Authorization Failed</h3>
                <div class="error-details">
                    <p><strong>Error:</strong> %s</p>
                    <p><strong>Description:</strong> %s</p>
                    <p><strong>State:</strong> %s</p>
                </div>
            </div>`, errorParam, errorDescription, state)
	} else if code != "" {
		title = "OAuth Authorization Successful"
		content = fmt.Sprintf(`
            <div class="success-box">
                <h3>✅ Authorization Successful!</h3>
                <p>Authorization code received successfully. You can now exchange this code for an access token.</p>
                <div class="code-section">
                    <strong>Authorization Code:</strong>
                    <div class="code-display">
                        <code>%s</code>
                        <button onclick="copyToClipboard('%s')" class="copy-btn">📋 Copy</button>
                    </div>
                </div>
                <div class="state-section">
                    <strong>State:</strong> <code>%s</code>
                </div>
                <div class="next-steps">
                    <h4>🎯 Automatic Token Exchange:</h4>
                    <button onclick="exchangeCodeForToken()" class="exchange-btn">
                        🔄 Exchange Code for Access Token
                    </button>
                    <div id="token-result" class="token-result"></div>
                    
                    <h4>💻 Manual cURL Example:</h4>
                    <p>You can also exchange this code manually using the token endpoint:</p>
                    <div class="curl-example">
                        <div class="curl-header">
                            <span>Copy and run this command:</span>
                            <button onclick="copyToClipboard(document.getElementById('curl-command').textContent)" class="copy-btn">📋 Copy</button>
                        </div>
                        <pre><code id="curl-command">curl -X POST %s/oauth/token \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=authorization_code&code=%s&client_id=YOUR_CLIENT_ID&redirect_uri=%s"</code></pre>
                    </div>
                </div>
            </div>`, code, code, state, d.proxyURL, code, fmt.Sprintf("http://%s/oauth/callback", r.Host))
	} else {
		title = "OAuth Callback Error"
		content = fmt.Sprintf(`
            <div class="error-box">
                <h3>❓ Unexpected Response</h3>
                <p>No authorization code or error received from OAuth provider.</p>
                <p><strong>Proxy Error:</strong> %s</p>
                <div class="troubleshoot">
                    <h4>🔧 Troubleshooting:</h4>
                    <ul>
                        <li>Check that the OAuth client configuration is correct</li>
                        <li>Verify the redirect URI matches exactly</li>
                        <li>Check proxy server logs for errors</li>
                    </ul>
                </div>
            </div>`, proxyError)
	}

	// Create the JavaScript for token exchange - using proper escaping
	exchangeScript := fmt.Sprintf(`
        async function exchangeCodeForToken() {
            const exchangeBtn = document.querySelector('.exchange-btn');
            const resultDiv = document.getElementById('token-result');
            
            exchangeBtn.disabled = true;
            exchangeBtn.textContent = '🔄 Exchanging...';
            resultDiv.style.display = 'block';
            resultDiv.className = 'token-result';
            resultDiv.innerHTML = '<div>🔄 Exchanging authorization code for access token...</div>';
            
            try {
                const response = await fetch('/oauth/token', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: new URLSearchParams({
                        grant_type: 'authorization_code',
                        code: '%s',
                        client_id: 'HFakeCpMUQnRX_m5HJKamRjU_vufUnNbG4xWpmUyvzo',
                        redirect_uri: 'http://%s/oauth/callback',
                        client_secret: 'test-secret-123'
                    })
                });
                
                if (response.ok) {
                    const token = await response.json();
                    resultDiv.className = 'token-result success';
                    resultDiv.innerHTML = '' +
                        '<div><strong>✅ Success! Access Token Generated:</strong></div>' +
                        '<div style="margin: 10px 0;">' +
                            '<strong>Access Token:</strong>' +
                            '<div class="code-display">' +
                                '<code>' + token.access_token + '</code>' +
                                '<button onclick="copyToClipboard(\'' + token.access_token + '\')" class="copy-btn">📋</button>' +
                            '</div>' +
                        '</div>' +
                        '<div><strong>Type:</strong> ' + token.token_type + '</div>' +
                        '<div><strong>Expires In:</strong> ' + token.expires_in + ' seconds</div>' +
                        '<div><strong>Scope:</strong> ' + (token.scope || 'Not specified') + '</div>';
                } else {
                    const errorText = await response.text();
                    resultDiv.className = 'token-result error';
                    resultDiv.innerHTML = '' +
                        '<div><strong>❌ Token Exchange Failed:</strong></div>' +
                        '<div>Status: ' + response.status + '</div>' +
                        '<div>Error: ' + errorText + '</div>';
                }
            } catch (error) {
                resultDiv.className = 'token-result error';
                resultDiv.innerHTML = '' +
                    '<div><strong>❌ Network Error:</strong></div>' +
                    '<div>' + error.message + '</div>';
            } finally {
                exchangeBtn.disabled = false;
                exchangeBtn.textContent = '🔄 Exchange Code for Access Token';
            }
        }`, code, r.Host)

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>%s - MCP Compose Dashboard</title>
    <style>
        body { 
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; 
            max-width: 800px; margin: 50px auto; padding: 20px; 
            background: #f0f2f5; color: #333;
        }
        .success-box { 
            border: 1px solid #28a745; padding: 30px; border-radius: 8px; 
            background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-left: 4px solid #28a745; 
        }
        .error-box { 
            border: 1px solid #dc3545; padding: 30px; border-radius: 8px; 
            background: white; box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            border-left: 4px solid #dc3545; 
        }
        .code-display {
            display: flex; align-items: center; gap: 10px; 
            background: #f8f9fa; padding: 10px; border-radius: 4px; margin: 10px 0;
            border: 1px solid #dee2e6;
        }
        .code-display code { 
            flex: 1; font-family: 'Monaco', 'Consolas', monospace; font-size: 14px;
            word-break: break-all; color: #495057;
        }
        .copy-btn {
            background: #007bff; color: white; border: none; 
            padding: 5px 10px; border-radius: 3px; cursor: pointer; 
            font-size: 12px; white-space: nowrap;
        }
        .copy-btn:hover { background: #0056b3; }
        .exchange-btn {
            background: #28a745; color: white; border: none;
            padding: 10px 20px; border-radius: 5px; cursor: pointer;
            font-size: 14px; margin: 10px 0;
        }
        .exchange-btn:hover { background: #218838; }
        .exchange-btn:disabled { background: #6c757d; cursor: not-allowed; }
        .curl-example {
            background: #2d3748; color: #e2e8f0; padding: 15px; 
            border-radius: 6px; margin: 15px 0; overflow-x: auto;
        }
        .curl-example pre { margin: 0; white-space: pre-wrap; }
        .curl-header {
            display: flex; justify-content: space-between; align-items: center;
            margin-bottom: 10px; color: #a0aec0; font-size: 13px;
        }
        .token-result {
            margin: 15px 0; padding: 15px; border-radius: 6px;
            background: #f8f9fa; border: 1px solid #dee2e6;
            display: none;
        }
        .token-result.success {
            background: #d4edda; border-color: #c3e6cb; color: #155724;
        }
        .token-result.error {
            background: #f8d7da; border-color: #f5c6cb; color: #721c24;
        }
        .back-links { 
            margin: 30px 0; text-align: center;
        }
        .back-links a { 
            color: #007bff; text-decoration: none; margin: 0 15px;
        }
        .back-links a:hover { 
            text-decoration: underline; 
        }
        .next-steps {
            margin-top: 20px; padding: 15px; background: #f8f9fa;
            border-radius: 6px; border: 1px solid #dee2e6;
        }
        .error-details, .troubleshoot {
            background: #f8f9fa; padding: 15px; border-radius: 6px;
            border: 1px solid #dee2e6; margin: 15px 0;
        }
        .popup-info {
            background: #cce5ff; border: 1px solid #007bff;
            padding: 15px; border-radius: 6px; margin: 15px 0;
            color: #004085;
        }
        .countdown {
            font-weight: bold; color: #007bff;
        }
    </style>
    <script>
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(function() {
                event.target.textContent = '✓ Copied!';
                setTimeout(() => {
                    event.target.innerHTML = '📋 Copy';
                }, 2000);
            }).catch(err => {
                alert('Failed to copy to clipboard');
            });
        }
        
        %s
        
        // Handle popup window communication and auto-close
        let countdownInterval;
        
        if (window.opener) {
            console.log('📨 Sending OAuth callback message to parent window');
            window.opener.postMessage({
                type: 'oauth_callback',
                code: '%s',
                state: '%s',
                error: '%s'
            }, '*');
            
            const popupInfo = document.createElement('div');
            popupInfo.className = 'popup-info';
            popupInfo.innerHTML = '' +
                '<div><strong>🪟 Popup Window Detected</strong></div>' +
                '<div>Results have been sent to the parent window.</div>' +
                '<div>This popup will close automatically in <span class="countdown" id="countdown">10</span> seconds.</div>' +
                '<button onclick="window.close()" style="margin-top: 10px; padding: 5px 10px; background: #007bff; color: white; border: none; border-radius: 3px; cursor: pointer;">' +
                    'Close Now' +
                '</button>';
            document.body.insertBefore(popupInfo, document.body.firstChild);
            
            let countdown = 10;
            countdownInterval = setInterval(() => {
                countdown--;
                const countdownEl = document.getElementById('countdown');
                if (countdownEl) {
                    countdownEl.textContent = countdown;
                }
                if (countdown <= 0) {
                    clearInterval(countdownInterval);
                    window.close();
                }
            }, 1000);
        }
        
        const returnUrl = sessionStorage.getItem('oauth_test_return');
        if (returnUrl && !window.opener) {
            setTimeout(() => {
                sessionStorage.removeItem('oauth_test_return');
                if (confirm('Return to OAuth configuration page?')) {
                    window.location.href = returnUrl;
                }
            }, 3000);
        }
    </script>
</head>
<body>
    <h2>🔐 OAuth Authorization Result</h2>
    %s
    <div class="back-links">
        <a href="javascript:history.back()">← Back</a>
        <a href="/">← Return to Dashboard</a>
        <a href="#" onclick="window.location.reload()">🔄 Refresh</a>
    </div>
</body>
</html>`, title, exchangeScript, code, state, errorParam, content)
}

// Add this method to handle OAuth API proxying
func (d *DashboardServer) handleOAuthAPIProxy(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /api/
	path := strings.TrimPrefix(r.URL.Path, "/api")
	endpoint := "/api" + path
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}

	d.logger.Info("Proxying OAuth API request to proxy server: %s %s", r.Method, endpoint)

	switch r.Method {
	case http.MethodGet:
		resp, err := d.proxyRequest(endpoint)
		if err != nil {
			d.logger.Error("Failed to proxy OAuth GET request: %v", err)
			http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	case http.MethodPost, http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var resp []byte
		if r.Method == http.MethodPost {
			resp, err = d.proxyPostRequest(endpoint, body)
		} else {
			resp, err = d.proxyPutRequest(endpoint, body)
		}

		if err != nil {
			d.logger.Error("Failed to proxy OAuth %s request: %v", r.Method, err)
			http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Add a PUT request helper method
func (d *DashboardServer) proxyPutRequest(endpoint string, body []byte) ([]byte, error) {
	url := d.proxyURL + endpoint
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

	return io.ReadAll(resp.Body)
}

// Add this general API proxy method
func (d *DashboardServer) handleAPIProxy(w http.ResponseWriter, r *http.Request) {
	// Extract the API path
	endpoint := r.URL.Path
	if r.URL.RawQuery != "" {
		endpoint += "?" + r.URL.RawQuery
	}

	d.logger.Info("Proxying general API request: %s %s", r.Method, endpoint)

	switch r.Method {
	case http.MethodGet:
		resp, err := d.proxyRequest(endpoint)
		if err != nil {
			d.logger.Error("Failed to proxy API GET request: %v", err)
			http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
			return
		}

		// Try to detect content type from response
		if endpoint == "/api/servers" || strings.Contains(endpoint, "/api/oauth/") {
			w.Header().Set("Content-Type", "application/json")
		} else {
			w.Header().Set("Content-Type", "text/plain")
		}
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		resp, err := d.proxyPostRequest(endpoint, body)
		if err != nil {
			d.logger.Error("Failed to proxy API POST request: %v", err)
			http.Error(w, "Failed to proxy request", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(resp); err != nil {
		d.logger.Error("Failed to write response: %v", err)
	}

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Update the task scheduler proxy to use the inspector service
func (d *DashboardServer) handleTaskSchedulerProxy(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /api/task-scheduler/
	path := strings.TrimPrefix(r.URL.Path, "/api/task-scheduler")

	d.logger.Info("Task scheduler proxy request: %s %s", r.Method, path)

	// Map REST-like calls to MCP tool calls
	var toolName string
	var toolArgs map[string]interface{}

	switch {
	case path == "/tasks" && r.Method == "GET":
		toolName = "list_tasks"
		toolArgs = map[string]interface{}{}

	case path == "/tasks" && r.Method == "POST":
		toolName = "add_task"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &toolArgs); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

	case strings.HasSuffix(path, "/run") && r.Method == "POST":
		toolName = "run_task"
		// Extract task ID from path like /tasks/123/run
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "tasks" {
			toolArgs = map[string]interface{}{
				"id": pathParts[1],
			}
		} else {
			http.Error(w, "Invalid task ID in path", http.StatusBadRequest)
			return
		}

	case strings.HasSuffix(path, "/enable") && r.Method == "POST":
		toolName = "enable_task"
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "tasks" {
			toolArgs = map[string]interface{}{
				"id": pathParts[1],
			}
		} else {
			http.Error(w, "Invalid task ID in path", http.StatusBadRequest)
			return
		}

	case strings.HasSuffix(path, "/disable") && r.Method == "POST":
		toolName = "disable_task"
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "tasks" {
			toolArgs = map[string]interface{}{
				"id": pathParts[1],
			}
		} else {
			http.Error(w, "Invalid task ID in path", http.StatusBadRequest)
			return
		}

	case strings.Contains(path, "/tasks/") && strings.HasSuffix(path, "/output"):
		toolName = "get_run_output"
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "tasks" {
			toolArgs = map[string]interface{}{
				"task_id": pathParts[1],
			}
		} else {
			http.Error(w, "Invalid task ID in path", http.StatusBadRequest)
			return
		}

	case path == "/runs/status" || path == "/runs/status/":
		toolName = "list_run_status"
		toolArgs = map[string]interface{}{}

	case path == "/metrics" || path == "/metrics/":
		toolName = "get_metrics"
		toolArgs = map[string]interface{}{}

	default:
		http.Error(w, fmt.Sprintf("Unsupported operation: %s %s", r.Method, path), http.StatusBadRequest)
		return
	}

	d.logger.Info("Calling MCP tool: %s with args: %v", toolName, toolArgs)

	// Use the inspector service to make the MCP call
	if d.inspectorService == nil {
		http.Error(w, `{"error": "Inspector service not available"}`, http.StatusServiceUnavailable)
		return
	}

	// Create a session for the task-scheduler server
	session, err := d.inspectorService.CreateSession("task-scheduler")
	if err != nil {
		d.logger.Error("Failed to create task scheduler session: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create session: %v"}`, err), http.StatusServiceUnavailable)
		return
	}

	// Create the inspector request
	inspectorReq := InspectorRequest{
		SessionID: session.ID,
		Method:    "tools/call",
		Params:    json.RawMessage(fmt.Sprintf(`{"name": "%s", "arguments": %s}`, toolName, mustJSON(toolArgs))),
	}

	// Execute the request
	response, err := d.inspectorService.ExecuteRequest(session.ID, inspectorReq)
	if err != nil {
		d.logger.Error("Task scheduler tool call failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Tool call failed: %v"}`, err), http.StatusInternalServerError)
		return
	}

	// Clean up session
	d.inspectorService.DestroySession(session.ID)

	// Return the result
	w.Header().Set("Content-Type", "application/json")

	// Extract the tool call result
	if response.Result != nil {
		// The result should contain the tool output
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			if content, exists := resultMap["content"]; exists {
				// Tool results are usually in content array
				if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
					if contentItem, ok := contentArray[0].(map[string]interface{}); ok {
						if text, exists := contentItem["text"]; exists {
							// Try to parse as JSON, fallback to raw text
							var jsonResult interface{}
							if err := json.Unmarshal([]byte(text.(string)), &jsonResult); err == nil {
								if err := json.NewEncoder(w).Encode(jsonResult); err != nil {
									d.logger.Error("Failed to encode JSON result: %v", err)
								}
								return
							}
						}
					}
				}
			}
			// Fallback to returning the whole result
			if err := json.NewEncoder(w).Encode(resultMap); err != nil {
				d.logger.Error("Failed to encode result map: %v", err)
			}
			return
		}
		if err := json.NewEncoder(w).Encode(response.Result); err != nil {
			d.logger.Error("Failed to encode response result: %v", err)
		}
	} else if response.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": response.Error,
		}); err != nil {
			d.logger.Error("Failed to encode error response: %v", err)
		}
	} else {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "success",
		}); err != nil {
			d.logger.Error("Failed to encode success response: %v", err)
		}
	}
}

// Helper function to marshal JSON safely
func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Update health check to use inspector
func (d *DashboardServer) handleTaskSchedulerHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.inspectorService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"error":     "Inspector service not available",
		}); err != nil {
			d.logger.Error("Failed to encode availability response: %v", err)
		}
		return
	}

	// Try to create a session with the task scheduler
	session, err := d.inspectorService.CreateSession("task-scheduler")
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"available": false,
			"error":     err.Error(),
		}); err != nil {
			d.logger.Error("Failed to encode error response: %v", err)
		}
		return
	}

	// Clean up session
	defer d.inspectorService.DestroySession(session.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"available":    true,
		"sessionId":    session.ID,
		"serverName":   "task-scheduler",
		"capabilities": session.Capabilities,
	}); err != nil {
		d.logger.Error("Failed to encode availability response: %v", err)
	}
}

// Update handleContainerLogs function
func (d *DashboardServer) handleContainerLogs(w http.ResponseWriter, r *http.Request, containerName string) {
	// Get query parameters
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	// Validate tail parameter
	tailInt, err := strconv.Atoi(tail)
	if err != nil || tailInt < 1 || tailInt > 10000 {
		tailInt = 100
		tail = "100"
	}

	follow := r.URL.Query().Get("follow") == "true"
	timestamps := r.URL.Query().Get("timestamps") == "true"
	since := r.URL.Query().Get("since") // Optional: logs since timestamp

	d.logger.Info("Getting logs for container: %s (tail: %s, follow: %t, timestamps: %t)",
		containerName, tail, follow, timestamps)

	// Check if container exists first
	if err := d.verifyContainerExists(containerName); err != nil {
		d.logger.Error("Container %s not found: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Container not found: %s", containerName), http.StatusNotFound)
		return
	}

	if follow {
		// Handle streaming logs
		d.streamContainerLogs(w, r, containerName, tail, timestamps, since)
	} else {
		// Handle static logs
		d.getStaticContainerLogs(w, r, containerName, tailInt, timestamps, since)
	}
}

func (d *DashboardServer) verifyContainerExists(containerName string) error {
	runtime := d.detectContainerRuntime()

	var cmd *exec.Cmd
	switch runtime {
	case "docker":
		// Use docker inspect instead of ps with filters
		cmd = exec.Command("docker", "inspect", containerName)
	case "podman":
		cmd = exec.Command("podman", "inspect", containerName)
	default:
		return fmt.Errorf("unsupported container runtime: %s", runtime)
	}

	d.logger.Info("Verifying container %s exists with: %s %v", containerName, cmd.Path, cmd.Args[1:])

	// Run the command and check exit code
	err := cmd.Run()
	if err != nil {
		d.logger.Debug("Container verification failed: %v", err)
		return fmt.Errorf("container not found")
	}

	d.logger.Debug("Container %s verified successfully", containerName)
	return nil
}

func (d *DashboardServer) detectContainerRuntime() string {
	// Try docker first
	if _, err := exec.LookPath("docker"); err == nil {
		if cmd := exec.Command("docker", "version"); cmd.Run() == nil {
			return "docker"
		}
	}

	// Try podman
	if _, err := exec.LookPath("podman"); err == nil {
		if cmd := exec.Command("podman", "version"); cmd.Run() == nil {
			return "podman"
		}
	}

	return "docker" // fallback
}

func (d *DashboardServer) getStaticContainerLogs(w http.ResponseWriter, r *http.Request, containerName string, tail int, timestamps bool, since string) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	logs, err := d.getLogsFromRuntime(ctx, containerName, tail, timestamps, since, false)
	if err != nil {
		d.logger.Error("Failed to get logs for container %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"container": containerName,
		"logs":      logs,
		"tail":      tail,
		"timestamp": time.Now().Format(time.RFC3339),
		"title":     "Logs",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		d.logger.Error("Failed to encode logs response: %v", err)
	}
}

func (d *DashboardServer) streamContainerLogs(w http.ResponseWriter, r *http.Request, containerName, tail string, timestamps bool, since string) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable proxy buffering

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\n")
	fmt.Fprintf(w, "data: {\"container\":\"%s\",\"message\":\"Log stream connected\"}\n\n", containerName)
	flusher.Flush()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Start the log streaming
	if err := d.streamLogsFromRuntime(ctx, w, flusher, containerName, tail, timestamps, since); err != nil {
		d.logger.Error("Error streaming logs for container %s: %v", containerName, err)
		fmt.Fprintf(w, "event: error\n")
		fmt.Fprintf(w, "data: {\"error\":\"%s\"}\n\n", err.Error()) // FIXED: Added err.Error()
		flusher.Flush()
	}
}

func (d *DashboardServer) getLogsFromRuntime(ctx context.Context, containerName string, tail int, timestamps bool, since string, follow bool) ([]string, error) {
	runtime := d.detectContainerRuntime()

	var cmd *exec.Cmd
	var args []string

	switch runtime {
	case "docker":
		args = []string{"logs"}
		if timestamps {
			args = append(args, "-t")
		}
		if tail > 0 {
			args = append(args, "--tail", strconv.Itoa(tail))
		}
		if since != "" {
			args = append(args, "--since", since)
		}
		if follow {
			args = append(args, "-f")
		}
		args = append(args, containerName)
		cmd = exec.CommandContext(ctx, "docker", args...)

	case "podman":
		args = []string{"logs"}
		if timestamps {
			args = append(args, "-t")
		}
		if tail > 0 {
			args = append(args, "--tail", strconv.Itoa(tail))
		}
		if since != "" {
			args = append(args, "--since", since)
		}
		if follow {
			args = append(args, "-f")
		}
		args = append(args, containerName)
		cmd = exec.CommandContext(ctx, "podman", args...)

	default:
		return nil, fmt.Errorf("unsupported container runtime: %s", runtime)
	}

	d.logger.Debug("Executing command: %s %v", cmd.Path, cmd.Args[1:])

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "no such container") ||
			strings.Contains(stderrStr, "No such container") {
			return nil, fmt.Errorf("container not found: %s", containerName)
		}
		return nil, fmt.Errorf("command failed: %v, stderr: %s", err, stderrStr)
	}

	if stderr.Len() > 0 {
		d.logger.Warning("Command stderr for %s: %s", containerName, stderr.String())
	}

	return d.parseLogOutput(stdout.String()), nil
}

func (d *DashboardServer) streamLogsFromRuntime(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, containerName, tail string, timestamps bool, since string) error {
	runtime := d.detectContainerRuntime()

	var cmd *exec.Cmd
	var args []string

	switch runtime {
	case "docker":
		args = []string{"logs", "-f"}
		if timestamps {
			args = append(args, "-t")
		}
		if tail != "" && tail != "0" {
			args = append(args, "--tail", tail)
		}
		if since != "" {
			args = append(args, "--since", since)
		}
		args = append(args, containerName)
		cmd = exec.CommandContext(ctx, "docker", args...)

	case "podman":
		args = []string{"logs", "-f"}
		if timestamps {
			args = append(args, "-t")
		}
		if tail != "" && tail != "0" {
			args = append(args, "--tail", tail)
		}
		if since != "" {
			args = append(args, "--since", since)
		}
		args = append(args, containerName)
		cmd = exec.CommandContext(ctx, "podman", args...)

	default:
		return fmt.Errorf("unsupported container runtime: %s", runtime)
	}

	d.logger.Debug("Streaming command: %s %v", cmd.Path, cmd.Args[1:])

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %v", err)
	}

	// Handle stderr in a separate goroutine
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			if line != "" {
				d.logger.Warning("Container %s stderr: %s", containerName, line)
			}
		}
	}()

	// Stream stdout line by line
	scanner := bufio.NewScanner(stdout)
	lineCount := 0

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			d.logger.Info("Log streaming cancelled for container %s", containerName)
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		lineCount++

		// Parse and format the log line
		logEntry := d.parseLogLine(line, lineCount)

		// Send as SSE event
		fmt.Fprintf(w, "event: log\n")
		fmt.Fprintf(w, "data: %s\n\n", logEntry)
		flusher.Flush()

		// Rate limiting - prevent overwhelming the client
		if lineCount%50 == 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading logs: %v", err)
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err() // Context was cancelled
		}
		return fmt.Errorf("command failed: %v", err)
	}

	// Send completion event
	fmt.Fprintf(w, "event: completed\n")
	fmt.Fprintf(w, "data: {\"message\":\"Log stream completed\"}\n\n")
	flusher.Flush()

	return nil
}

func (d *DashboardServer) parseLogOutput(output string) []string {
	if output == "" {
		return []string{}
	}

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	var result []string

	for i, line := range lines {
		if line != "" { // Skip empty lines
			result = append(result, d.parseLogLine(line, i+1))
		}
	}

	return result
}

func (d *DashboardServer) parseLogLine(line string, lineNumber int) string {
	logEntry := map[string]interface{}{
		"line":      lineNumber,
		"content":   line,
		"timestamp": time.Now().Format(time.RFC3339Nano),
	}

	// Try to extract timestamp from Docker/Podman log line
	if strings.Contains(line, "T") && strings.Contains(line, "Z") {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			if timestamp, err := time.Parse(time.RFC3339Nano, parts[0]); err == nil {
				logEntry["original_timestamp"] = timestamp.Format(time.RFC3339Nano)
				logEntry["content"] = parts[1]
			}
		}
	}

	// Try to detect log level
	content := strings.ToLower(line)
	if strings.Contains(content, "error") || strings.Contains(content, "err") {
		logEntry["level"] = "error"
	} else if strings.Contains(content, "warn") {
		logEntry["level"] = "warning"
	} else if strings.Contains(content, "info") {
		logEntry["level"] = "info"
	} else if strings.Contains(content, "debug") {
		logEntry["level"] = "debug"
	} else {
		logEntry["level"] = "info"
	}

	jsonBytes, err := json.Marshal(logEntry)
	if err != nil {
		d.logger.Error("Failed to marshal log entry: %v", err)
		return fmt.Sprintf("{\"line\":%d,\"content\":%q,\"timestamp\":%q}",
			lineNumber, line, time.Now().Format(time.RFC3339Nano))
	}

	return string(jsonBytes)
}

func (d *DashboardServer) handleContainerStats(w http.ResponseWriter, _ *http.Request, containerName string) {
	runtime := d.detectContainerRuntime()

	var cmd *exec.Cmd
	switch runtime {
	case "docker":
		cmd = exec.Command("docker", "stats", "--no-stream", "--format",
			"table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}",
			containerName)
	case "podman":
		cmd = exec.Command("podman", "stats", "--no-stream", "--format",
			"table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}",
			containerName)
	default:
		http.Error(w, "Unsupported container runtime", http.StatusInternalServerError)
		return
	}

	output, err := cmd.Output()
	if err != nil {
		d.logger.Error("Failed to get container stats for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse stats output
	lines := strings.Split(string(output), "\n")
	var stats map[string]interface{}
	if len(lines) >= 2 && lines[1] != "" {
		fields := strings.Fields(lines[1])
		if len(fields) >= 6 {
			stats = map[string]interface{}{
				"name":      fields[0],
				"cpu_perc":  fields[1],
				"mem_usage": fields[2],
				"mem_perc":  fields[3],
				"net_io":    fields[4],
				"block_io":  fields[5],
				"timestamp": time.Now().Format(time.RFC3339),
				"runtime":   runtime,
			}
		}
	}

	if stats == nil {
		stats = map[string]interface{}{
			"error":   "No stats available for container " + containerName,
			"runtime": runtime,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		d.logger.Error("Failed to encode stats: %v", err)
	}
}

// Update handleServerAction to support both Docker and Podman
func (d *DashboardServer) handleServerAction(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Server string `json:"server"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Server == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}

	containerName := fmt.Sprintf("mcp-compose-%s", req.Server)
	runtime := d.detectContainerRuntime()

	var cmd *exec.Cmd
	switch action {
	case "start":
		// Starting requires rebuilding the container with proper config
		response := map[string]string{
			"error": fmt.Sprintf("Server start not implemented in dashboard yet. Use CLI: mcp-compose start %s", req.Server),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			d.logger.Error("Failed to encode response: %v", err)
		}
		return
	case "stop":
		if runtime == "docker" {
			cmd = exec.Command("docker", "stop", containerName)
		} else {
			cmd = exec.Command("podman", "stop", containerName)
		}
	case "restart":
		if runtime == "docker" {
			cmd = exec.Command("docker", "restart", containerName)
		} else {
			cmd = exec.Command("podman", "restart", containerName)
		}
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("Failed to %s container %s: %v. Output: %s", action, containerName, err, string(output))
		response := map[string]string{
			"error":   fmt.Sprintf("Failed to %s container: %v", action, err),
			"output":  string(output),
			"runtime": runtime,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			d.logger.Error("Failed to encode response: %v", err)
		}
		return
	}

	response := map[string]string{
		"success": fmt.Sprintf("Container %s %sed successfully", containerName, action),
		"output":  string(output),
		"runtime": runtime,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}
