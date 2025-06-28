package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
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
	w.Write(resp)
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
	w.Write(resp)
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
	w.Write(resp)
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
		d.handleContainerLogs(w, r, containerName)
	case "stats":
		d.handleContainerStats(w, r, containerName)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

func (d *DashboardServer) handleContainerLogs(w http.ResponseWriter, r *http.Request, containerName string) {
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}
	logs, err := d.getContainerLogs(containerName, tail, false)
	if err != nil {
		d.logger.Error("Failed to get container logs for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}
	response := map[string]interface{}{
		"container": containerName,
		"logs":      logs,
		"timestamp": time.Now().Format(time.RFC3339),
		"title":     "Logs", // Changed from "Container Logs"
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (d *DashboardServer) handleContainerStats(w http.ResponseWriter, _ *http.Request, containerName string) {
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.MemPerc}}\t{{.NetIO}}\t{{.BlockIO}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		d.logger.Error("Failed to get container stats for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get stats: %v", err), http.StatusInternalServerError)
		return
	}
	// Parse docker stats output
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
			}
		}
	}
	if stats == nil {
		stats = map[string]interface{}{
			"error": "No stats available for container " + containerName,
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
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
	var cmd *exec.Cmd
	switch action {
	case "start":
		// Starting requires rebuilding the container with proper config
		// This is complex, so we'll return a helpful message
		response := map[string]string{
			"error": fmt.Sprintf("Server start not implemented in dashboard yet. Use CLI: mcp-compose start %s", req.Server),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(response)
		return
	case "stop":
		cmd = exec.Command("docker", "stop", containerName)
	case "restart":
		cmd = exec.Command("docker", "restart", containerName)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
		return
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		d.logger.Error("Failed to %s container %s: %v. Output: %s", action, containerName, err, string(output))
		response := map[string]string{
			"error":  fmt.Sprintf("Failed to %s container: %v", action, err),
			"output": string(output),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	response := map[string]string{
		"success": fmt.Sprintf("Container %s %sed successfully", containerName, action),
		"output":  string(output),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
	w.Write(resp)
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
	defer conn.Close()
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
	w.Write([]byte(htmlContent))
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
	w.Write(resp)
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
	w.Write(resp)
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
	json.NewEncoder(w).Encode(response)
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
	w.Write(resp)
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
		w.Write(resp)

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
		w.Write(resp)

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
	w.Write(resp)
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
	defer resp.Body.Close()

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
	defer resp.Body.Close()

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
		json.NewEncoder(w).Encode(defaultScopes)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
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
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
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
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
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
	defer resp.Body.Close()

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
	w.Write(responseBody)
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

		url := d.proxyURL + endpoint
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		req.Header.Set("Content-Type", r.Header.Get("Content-Type"))

		resp, err := d.httpClient.Do(req)
		if err != nil {
			http.Error(w, "Failed to process authorization", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Handle redirects
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			location := resp.Header.Get("Location")
			if location != "" {
				http.Redirect(w, r, location, resp.StatusCode)
				return
			}
		}

		responseBody, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
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
	w.Write(resp)
}

func (d *DashboardServer) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	// Extract authorization code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	error := r.URL.Query().Get("error")

	// Create a simple HTML page to handle the callback
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>OAuth Callback</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .success { color: green; }
        .error { color: red; }
        pre { background: #f5f5f5; padding: 10px; border-radius: 4px; overflow-x: auto; }
    </style>
</head>
<body>
    <h2>OAuth Authorization Result</h2>
    %s
    <p><a href="/">Return to Dashboard</a></p>
    <script>
        // Auto-close popup if opened in popup window
        if (window.opener) {
            window.opener.postMessage({
                type: 'oauth_callback',
                code: '%s',
                state: '%s',
                error: '%s'
            }, '*');
            setTimeout(() => window.close(), 2000);
        }
    </script>
</body>
</html>`, getCallbackContent(code, state, error), code, state, error)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func getCallbackContent(code, state, errorParam string) string {
	if errorParam != "" {
		return fmt.Sprintf(`
            <div class="error">
                <h3>Authorization Failed</h3>
                <p><strong>Error:</strong> %s</p>
            </div>`, errorParam)
	}

	if code != "" {
		return fmt.Sprintf(`
            <div class="success">
                <h3>Authorization Successful!</h3>
                <p>Authorization code received. You can now exchange this code for an access token.</p>
                <details>
                    <summary>Technical Details</summary>
                    <pre>Authorization Code: %s
State: %s</pre>
                </details>
            </div>`, code, state)
	}

	return `<div class="error"><h3>Invalid callback</h3><p>No authorization code received.</p></div>`
}
