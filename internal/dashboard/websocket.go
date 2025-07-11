package dashboard

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"mcpcompose/internal/constants"
	"github.com/gorilla/websocket"
)

// Message types for different WebSocket streams
type LogMessage struct {
	Timestamp string `json:"timestamp"`
	Server    string `json:"server"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

type MetricsMessage struct {
	Timestamp   string                 `json:"timestamp"`
	Status      map[string]interface{} `json:"status"`
	Connections map[string]interface{} `json:"connections"`
}

type ActivityMessage struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Type      string                 `json:"type"` // request, connection, tool, error
	Server    string                 `json:"server,omitempty"`
	Client    string                 `json:"client,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// SafeWebSocketConn - WebSocket connection wrapper with mutex for safe concurrent writes
type SafeWebSocketConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func (s *SafeWebSocketConn) WriteJSON(v interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.WriteJSON(v)
}

func (s *SafeWebSocketConn) WriteMessage(messageType int, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.WriteMessage(messageType, data)
}

func (s *SafeWebSocketConn) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.SetWriteDeadline(t)
}

func (s *SafeWebSocketConn) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.conn.Close()
}

// ActivityBroadcaster handles activity stream WebSocket connections
type ActivityBroadcaster struct {
	clients       map[*SafeWebSocketConn]bool
	mu            sync.RWMutex
	register      chan *SafeWebSocketConn
	unregister    chan *SafeWebSocketConn
	broadcast     chan ActivityMessage
	shutdown      chan struct{}
	running       bool
	runMutex      sync.Mutex
	clientCounter int64
	storage       *ActivityStorage
}

// Global activity broadcaster instance
var activityBroadcaster = &ActivityBroadcaster{
	clients:    make(map[*SafeWebSocketConn]bool),
	register:   make(chan *SafeWebSocketConn, constants.WebSocketChannelSize),
	unregister: make(chan *SafeWebSocketConn, constants.WebSocketChannelSize),
	broadcast:  make(chan ActivityMessage, constants.ActivityChannelSize),
	shutdown:   make(chan struct{}),
}

func init() {
	// Initialize storage if database URL is available
	dbURL := os.Getenv("POSTGRES_URL")
	if dbURL != "" {
		storage, err := NewActivityStorage(dbURL)
		if err != nil {
			log.Printf("[ACTIVITY] Failed to initialize activity storage: %v", err)
		} else {
			activityBroadcaster.storage = storage
			log.Printf("[ACTIVITY] Activity storage initialized")

			// Start cleanup routine
			go startActivityCleanup(storage, context.Background())
		}
	}

	activityBroadcaster.start()
}

func (ab *ActivityBroadcaster) start() {
	ab.runMutex.Lock()
	if ab.running {
		ab.runMutex.Unlock()

		return
	}
	ab.running = true
	ab.runMutex.Unlock()

	go ab.run() // Start the goroutine instead of calling start recursively
}

func startActivityCleanup(storage *ActivityStorage, ctx context.Context) {
	ticker := time.NewTicker(constants.DailyCleanupInterval) // Run daily
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Clean up activities older than 30 days
			if err := storage.CleanupOldActivities(30 * 24 * time.Hour); err != nil {
				log.Printf("[ACTIVITY] Cleanup error: %v", err)
			}
		case <-ctx.Done():
			log.Printf("[ACTIVITY] Cleanup goroutine shutting down")

			return
		}
	}
}

func (ab *ActivityBroadcaster) sendRecentActivities(client *SafeWebSocketConn) {
	if ab.storage == nil {

		return
	}

	// Send last 50 activities to new client
	activities, err := ab.storage.GetRecentActivities(constants.RecentActivitiesCount, nil)
	if err != nil {
		log.Printf("[ACTIVITY] Failed to get recent activities: %v", err)

		return
	}

	for _, activity := range activities {
		// Convert StoredActivity back to ActivityMessage
		activityMsg := ActivityMessage{
			ID:        activity.ActivityID,
			Timestamp: activity.Timestamp.Format(time.RFC3339Nano),
			Level:     activity.Level,
			Type:      activity.Type,
			Server:    activity.Server,
			Client:    activity.Client,
			Message:   activity.Message,
			Details:   activity.Details,
		}

		// Send directly to the client using WriteJSON
		if err := client.SetWriteDeadline(time.Now().Add(constants.DefaultWebSocketTimeout)); err != nil {
			log.Printf("[ACTIVITY] Failed to set write deadline for client: %v", err)
		}
		if err := client.WriteJSON(activityMsg); err != nil {
			log.Printf("[ACTIVITY] Failed to send historical activity to client: %v", err)

			return // Client disconnected
		}
	}

	log.Printf("[ACTIVITY] Sent %d historical activities to new client", len(activities))
}

func (ab *ActivityBroadcaster) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTIVITY] Broadcaster panic recovered: %v", r)
			time.Sleep(time.Second)
			ab.runMutex.Lock()
			ab.running = false
			ab.runMutex.Unlock()
			ab.start() // Restart
		}
	}()

	log.Println("[ACTIVITY] Activity broadcaster running")

	for {
		select {
		case client := <-ab.register:
			ab.handleClientRegistration(client)

		case client := <-ab.unregister:
			ab.handleClientUnregistration(client)

		case message := <-ab.broadcast:
			// Store the activity in database
			if ab.storage != nil {
				if err := ab.storage.StoreActivity(message); err != nil {
					log.Printf("[ACTIVITY] Failed to store activity: %v", err)
				}
			}

			// Broadcast to connected clients
			ab.handleBroadcast(message)

		case <-ab.shutdown:
			ab.handleShutdown()

			return
		}
	}
}

func (ab *ActivityBroadcaster) handleClientRegistration(client *SafeWebSocketConn) {
	ab.mu.Lock()
	ab.clients[client] = true
	ab.clientCounter++
	clientCount := len(ab.clients)
	clientID := ab.clientCounter
	ab.mu.Unlock()

	log.Printf("[ACTIVITY] ✅ Client #%d registered (total: %d)", clientID, clientCount)

	// Send recent activities to newly connected client
	if ab.storage != nil {
		go ab.sendRecentActivities(client)
	}

	welcomeMsg := ActivityMessage{
		ID:        generateID(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     "INFO",
		Type:      "connection",
		Message:   fmt.Sprintf("Client #%d successfully registered to activity stream", clientID),
		Details: map[string]interface{}{
			"client_id":     clientID,
			"total_clients": clientCount,
		},
	}

	go func() {
		if err := client.SetWriteDeadline(time.Now().Add(constants.DefaultWebSocketTimeout)); err != nil {
			log.Printf("[ACTIVITY] Failed to set write deadline for client #%d: %v", clientID, err)
		}
		if err := client.WriteJSON(welcomeMsg); err != nil {
			log.Printf("[ACTIVITY] ❌ Failed to send welcome message to client #%d: %v", clientID, err)
		} else {
			log.Printf("[ACTIVITY] ✅ Welcome message sent to client #%d", clientID)
		}
	}()
}

func (ab *ActivityBroadcaster) handleClientUnregistration(client *SafeWebSocketConn) {
	ab.mu.Lock()
	if _, exists := ab.clients[client]; exists {
		delete(ab.clients, client)
		if err := client.Close(); err != nil {
			log.Printf("[ACTIVITY] Warning: Failed to close client connection: %v", err)
		}
	}
	clientCount := len(ab.clients)
	ab.mu.Unlock()
	log.Printf("[ACTIVITY] ❌ Client unregistered (remaining: %d)", clientCount)
}

func (ab *ActivityBroadcaster) handleBroadcast(message ActivityMessage) {
	ab.mu.RLock()
	clientCount := len(ab.clients)
	ab.mu.RUnlock()

	if clientCount == 0 {
		log.Printf("[ACTIVITY] 📭 No clients to broadcast to: %s", message.Message)

		return
	}

	log.Printf("[ACTIVITY] 📢 Broadcasting to %d clients: %s", clientCount, message.Message)

	ab.mu.Lock()
	defer ab.mu.Unlock()

	sentCount := 0
	failedCount := 0
	for client := range ab.clients {
		if ab.sendToClient(client, message) {
			sentCount++
		} else {
			failedCount++
			delete(ab.clients, client)
		}
	}

	log.Printf("[ACTIVITY] 📊 Message delivered to %d/%d clients (%d failed)", sentCount, sentCount+failedCount, failedCount)
}

func (ab *ActivityBroadcaster) sendToClient(client *SafeWebSocketConn, message ActivityMessage) bool {
	done := make(chan bool, 1)
	go func() {
		if err := client.SetWriteDeadline(time.Now().Add(constants.DefaultWebSocketTimeout)); err != nil {
			log.Printf("[ACTIVITY] Failed to set write deadline for client: %v", err)
		}
		err := client.WriteJSON(message)
		done <- (err == nil)
		if err != nil {
			log.Printf("[ACTIVITY] ❌ Failed to send to client: %v", err)
			if closeErr := client.Close(); closeErr != nil {
				log.Printf("[ACTIVITY] Warning: Failed to close client connection: %v", closeErr)
			}
		}
	}()

	select {
	case success := <-done:

		return success
	case <-time.After(constants.DefaultConnectionTimeout):
		log.Printf("[ACTIVITY] ⏰ Client send timeout, disconnecting slow client")
		if err := client.Close(); err != nil {
			log.Printf("[ACTIVITY] Warning: Failed to close slow client connection: %v", err)
		}

		return false
	}
}

func (ab *ActivityBroadcaster) handleShutdown() {
	log.Println("[ACTIVITY] Shutting down broadcaster...")
	ab.mu.Lock()
	for client := range ab.clients {
		if err := client.Close(); err != nil {
			log.Printf("[ACTIVITY] Warning: Failed to close client connection during shutdown: %v", err)
		}
	}
	ab.clients = make(map[*SafeWebSocketConn]bool)
	ab.mu.Unlock()
	log.Println("[ACTIVITY] All clients disconnected")
}

// Dashboard WebSocket handlers
func (d *DashboardServer) handleLogWebSocket(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)

		return
	}

	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		d.logger.Error("Failed to upgrade websocket connection: %v", err)

		return
	}

	safeConn := &SafeWebSocketConn{conn: conn}
	defer func() {
		if err := safeConn.Close(); err != nil {
			d.logger.Debug("Warning: Failed to close WebSocket connection for server %s: %v", serverName, err)
		} else {
			d.logger.Debug("WebSocket connection closed for server: %s", serverName)
		}
	}()

	d.logger.Info("Starting log stream WebSocket for server: %s", serverName)

	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from client for server: %s", serverName)

		return nil
	})

	containerName := "mcp-compose-" + serverName
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Try to use proxy endpoint for streaming logs first
	endpoint := fmt.Sprintf("/api/containers/%s/logs?follow=true&tail=50", containerName)
	if d.streamLogsViaProxyEndpoint(safeConn, endpoint, serverName, ctx) {
		return
	}

	// Fallback to direct docker command if proxy fails
	d.logger.Info("Proxy streaming failed, falling back to direct docker command for %s", containerName)
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", "50", containerName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.logger.Error("Failed to create stdout pipe for %s: %v", containerName, err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to create log stream: %v", err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}

		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.logger.Error("Failed to create stderr pipe for %s: %v", containerName, err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to create log stream: %v", err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}

		return
	}

	if err := cmd.Start(); err != nil {
		d.logger.Error("Failed to start docker logs command for %s: %v", containerName, err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to start log stream: %v", err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}

		return
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	go d.streamLogs(safeConn, stdout, serverName, "stdout", cancel)
	go d.streamLogs(safeConn, stderr, serverName, "stderr", cancel)

	pingTicker := time.NewTicker(constants.WebSocketPingInterval)
	defer pingTicker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil && err.Error() != "signal: killed" && !strings.Contains(err.Error(), "context canceled") {
				d.logger.Error("Docker logs command for %s exited with error: %v", containerName, err)
				if writeErr := safeConn.WriteJSON(map[string]string{
					"error": fmt.Sprintf("Log stream ended: %v", err),
				}); writeErr != nil {
					d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
				}
			}

			return
		case <-pingTicker.C:
			if err := safeConn.SetWriteDeadline(time.Now().Add(constants.WebSocketWriteTimeout)); err != nil {
				d.logger.Debug("Failed to set write deadline for ping to client for %s: %v", serverName, err)
			}
			if err := safeConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				d.logger.Debug("Failed to send ping to client for %s: %v", serverName, err)
				cancel()

				return
			}
		}
	}
}

func (d *DashboardServer) streamLogsViaProxyEndpoint(safeConn *SafeWebSocketConn, endpoint, serverName string, ctx context.Context) bool {
	url := d.proxyURL + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		d.logger.Error("Failed to create proxy request: %v", err)
		return false
	}

	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		d.logger.Error("Failed to make proxy request: %v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		d.logger.Error("Proxy request failed: %d %s", resp.StatusCode, resp.Status)
		return false
	}

	d.logger.Info("Successfully connected to proxy logs stream for %s", serverName)

	// Stream logs from proxy response (SSE format)
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return true
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse SSE format: "data: {json}" lines contain the actual log data
		if strings.HasPrefix(line, "data: ") {
			jsonData := strings.TrimPrefix(line, "data: ")
			
			// Try to parse as JSON to extract log content
			var logData map[string]interface{}
			if err := json.Unmarshal([]byte(jsonData), &logData); err == nil {
				// Check if it's a log event (has "content" field)
				if content, ok := logData["content"].(string); ok {
					msg := LogMessage{
						Timestamp: time.Now().Format(time.RFC3339),
						Server:    serverName,
						Level:     d.parseLogLevel(content),
						Message:   content,
					}

					if err := safeConn.SetWriteDeadline(time.Now().Add(constants.WebSocketWriteDeadline)); err != nil {
						d.logger.Debug("Failed to set write deadline: %v", err)
					}
					if err := safeConn.WriteJSON(msg); err != nil {
						d.logger.Debug("Failed to write log message: %v", err)
						return true
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		d.logger.Error("Error reading proxy logs: %v", err)
		return false
	}

	return true
}

func (d *DashboardServer) streamLogs(safeConn *SafeWebSocketConn, reader io.Reader, serverName, source string, cancel context.CancelFunc) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		msg := LogMessage{
			Timestamp: time.Now().Format(time.RFC3339),
			Server:    serverName,
			Level:     d.parseLogLevel(line),
			Message:   line,
		}

		if err := safeConn.SetWriteDeadline(time.Now().Add(constants.WebSocketWriteDeadline)); err != nil {
			d.logger.Debug("Failed to set write deadline for log message to WebSocket for %s: %v", serverName, err)
		}
		if err := safeConn.WriteJSON(msg); err != nil {
			d.logger.Debug("Failed to write log message to WebSocket for %s: %v", serverName, err)
			cancel()

			break
		}
	}

	if err := scanner.Err(); err != nil {
		d.logger.Error("Error reading logs for %s: %v", serverName, err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Error reading %s logs: %v", source, err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}
	}
}

func (d *DashboardServer) handleMetricsWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		d.logger.Error("Failed to upgrade metrics websocket connection: %v", err)

		return
	}

	safeConn := &SafeWebSocketConn{conn: conn}
	defer func() {
		if err := safeConn.Close(); err != nil {
			d.logger.Debug("Warning: Failed to close metrics WebSocket connection: %v", err)
		} else {
			d.logger.Debug("Metrics WebSocket connection closed")
		}
	}()

	d.logger.Info("Starting metrics stream WebSocket")

	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from metrics client")

		return nil
	})

	metricsTicker := time.NewTicker(constants.MetricsUpdateInterval)
	defer metricsTicker.Stop()

	pingTicker := time.NewTicker(constants.WebSocketPingInterval)
	defer pingTicker.Stop()

	d.sendMetricsUpdate(safeConn)

	for {
		select {
		case <-metricsTicker.C:
			d.sendMetricsUpdate(safeConn)
		case <-pingTicker.C:
			if err := safeConn.SetWriteDeadline(time.Now().Add(constants.WebSocketWriteTimeout)); err != nil {
				d.logger.Debug("Failed to set write deadline for ping to metrics client: %v", err)
			}
			if err := safeConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				d.logger.Debug("Failed to send ping to metrics client: %v", err)

				return
			}
		}
	}
}

func (d *DashboardServer) sendMetricsUpdate(safeConn *SafeWebSocketConn) {
	statusData, err := d.proxyRequest("/api/status")
	if err != nil {
		d.logger.Error("Failed to get status for metrics: %v", err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get status: %v", err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}

		return
	}

	connectionsData, err := d.proxyRequest("/api/connections")
	if err != nil {
		d.logger.Error("Failed to get connections for metrics: %v", err)
		if writeErr := safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get connections: %v", err),
		}); writeErr != nil {
			d.logger.Error("Failed to write error message to WebSocket: %v", writeErr)
		}

		return
	}

	var status map[string]interface{}
	var connections map[string]interface{}

	if err := json.Unmarshal(statusData, &status); err != nil {
		d.logger.Error("Failed to parse status JSON: %v", err)
		status = map[string]interface{}{"error": "Failed to parse status"}
	}

	if err := json.Unmarshal(connectionsData, &connections); err != nil {
		d.logger.Error("Failed to parse connections JSON: %v", err)
		connections = map[string]interface{}{"error": "Failed to parse connections"}
	}

	metrics := MetricsMessage{
		Timestamp:   time.Now().Format(time.RFC3339),
		Status:      status,
		Connections: connections,
	}

	if err := safeConn.SetWriteDeadline(time.Now().Add(constants.WebSocketWriteDeadline)); err != nil {
		d.logger.Debug("Failed to set write deadline for metrics WebSocket: %v", err)
	}
	if err := safeConn.WriteJSON(metrics); err != nil {
		d.logger.Debug("Failed to write metrics to WebSocket: %v", err)
	}
}

// parseLogLevel attempts to extract log level from log line
func (d *DashboardServer) parseLogLevel(message string) string {
	msg := strings.ToUpper(message)

	if strings.Contains(msg, "ERROR") || strings.Contains(msg, "FATAL") || strings.Contains(msg, "PANIC") {

		return "ERROR"
	}
	if strings.Contains(msg, "WARN") || strings.Contains(msg, "WARNING") {

		return "WARN"
	}
	if strings.Contains(msg, "INFO") {

		return "INFO"
	}
	if strings.Contains(msg, "DEBUG") || strings.Contains(msg, "TRACE") {

		return "DEBUG"
	}


	return "INFO"
}

// Public API for activity broadcasting
func BroadcastActivity(level, activityType, server, client, message string, details map[string]interface{}) {
	activity := ActivityMessage{
		ID:        generateID(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level,
		Type:      activityType,
		Server:    server,
		Client:    client,
		Message:   message,
		Details:   details,
	}

	// Try to broadcast to local connections first
	select {
	case activityBroadcaster.broadcast <- activity:
		// Successfully queued for broadcast
	default:
		// Broadcast channel is full, log warning
		log.Printf("[ACTIVITY] ⚠️ Broadcast channel full, dropping activity: %s", message)
	}

	// Also send to dashboard service if running in distributed mode
	jsonData, err := json.Marshal(activity)
	if err != nil {

		return
	}

	go func() {
		resp, err := http.Post("http://mcp-compose-dashboard:3001/api/activity", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[ACTIVITY] Failed to send to dashboard service: %v", err)

			return
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				log.Printf("[ACTIVITY] Warning: Failed to close response body: %v", closeErr)
			}
		}()
	}()
}

// Utility functions
func generateID() string {

	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(constants.RandomStringLength))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		for i := range bytes {
			bytes[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
	} else {
		for i, b := range bytes {
			bytes[i] = charset[b%byte(len(charset))]
		}
	}

	return string(bytes)
}
