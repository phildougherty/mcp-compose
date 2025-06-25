package dashboard

import (
	"encoding/json"
	"fmt"
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

func (d *DashboardServer) handleCatchAll(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	// Handle dashboard root
	if path == "" || path == "dashboard" {
		d.handleIndex(w, r)
		return
	}

	// Skip API and static routes (they should be handled by specific handlers)
	if strings.HasPrefix(path, "api/") || strings.HasPrefix(path, "static/") || strings.HasPrefix(path, "ws/") {
		http.NotFound(w, r)
		return
	}

	// Check if this looks like a server route
	parts := strings.SplitN(path, "/", 2)
	serverName := parts[0]

	// Verify this is a valid server
	servers, err := d.proxyRequest("/api/servers")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var serversMap map[string]interface{}
	if err := json.Unmarshal(servers, &serversMap); err != nil {
		http.NotFound(w, r)
		return
	}

	if _, exists := serversMap[serverName]; !exists {
		http.NotFound(w, r)
		return
	}

	// Proxy to the MCP proxy directly
	resp, err := d.proxyRequest("/" + path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to access %s", path), http.StatusInternalServerError)
		return
	}

	// Set content type
	if strings.HasSuffix(path, ".json") || strings.HasSuffix(path, "openapi.json") {
		w.Header().Set("Content-Type", "application/json")
	} else {
		w.Header().Set("Content-Type", "text/html")
	}

	w.Write(resp)
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
