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
	"os/exec"
	"strings"
	"sync"
	"time"

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
}

// Global activity broadcaster instance
var activityBroadcaster = &ActivityBroadcaster{
	clients:    make(map[*SafeWebSocketConn]bool),
	register:   make(chan *SafeWebSocketConn, 10),
	unregister: make(chan *SafeWebSocketConn, 10),
	broadcast:  make(chan ActivityMessage, 1000),
	shutdown:   make(chan struct{}),
}

func init() {
	activityBroadcaster.start()
}

// Activity broadcaster methods
func (ab *ActivityBroadcaster) start() {
	ab.runMutex.Lock()
	defer ab.runMutex.Unlock()
	if !ab.running {
		ab.running = true
		go ab.run()
		log.Println("[ACTIVITY] Activity broadcaster started")
	}
}

func (ab *ActivityBroadcaster) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTIVITY] Broadcaster panic recovered: %v", r)
			time.Sleep(time.Second)
			ab.runMutex.Lock()
			ab.running = false
			ab.runMutex.Unlock()
			ab.start()
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
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))
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
		client.Close()
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
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := client.WriteJSON(message)
		done <- (err == nil)
		if err != nil {
			log.Printf("[ACTIVITY] ❌ Failed to send to client: %v", err)
			client.Close()
		}
	}()

	select {
	case success := <-done:
		return success
	case <-time.After(3 * time.Second):
		log.Printf("[ACTIVITY] ⏰ Client send timeout, disconnecting slow client")
		client.Close()
		return false
	}
}

func (ab *ActivityBroadcaster) handleShutdown() {
	log.Println("[ACTIVITY] Shutting down broadcaster...")
	ab.mu.Lock()
	for client := range ab.clients {
		client.Close()
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
		safeConn.Close()
		d.logger.Debug("WebSocket connection closed for server: %s", serverName)
	}()

	d.logger.Info("Starting log stream WebSocket for server: %s", serverName)

	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from client for server: %s", serverName)
		return nil
	})

	containerName := "mcp-compose-" + serverName
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", "50", containerName)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		d.logger.Error("Failed to create stdout pipe for %s: %v", containerName, err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to create log stream: %v", err),
		})
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		d.logger.Error("Failed to create stderr pipe for %s: %v", containerName, err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to create log stream: %v", err),
		})
		return
	}

	if err := cmd.Start(); err != nil {
		d.logger.Error("Failed to start docker logs command for %s: %v", containerName, err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to start log stream: %v", err),
		})
		return
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	go d.streamLogs(safeConn, stdout, serverName, "stdout", cancel)
	go d.streamLogs(safeConn, stderr, serverName, "stderr", cancel)

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case err := <-done:
			if err != nil && err.Error() != "signal: killed" && !strings.Contains(err.Error(), "context canceled") {
				d.logger.Error("Docker logs command for %s exited with error: %v", containerName, err)
				safeConn.WriteJSON(map[string]string{
					"error": fmt.Sprintf("Log stream ended: %v", err),
				})
			}
			return
		case <-pingTicker.C:
			safeConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := safeConn.WriteMessage(websocket.PingMessage, nil); err != nil {
				d.logger.Debug("Failed to send ping to client for %s: %v", serverName, err)
				cancel()
				return
			}
		}
	}
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

		safeConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := safeConn.WriteJSON(msg); err != nil {
			d.logger.Debug("Failed to write log message to WebSocket for %s: %v", serverName, err)
			cancel()
			break
		}
	}

	if err := scanner.Err(); err != nil {
		d.logger.Error("Error reading logs for %s: %v", serverName, err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Error reading %s logs: %v", source, err),
		})
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
		safeConn.Close()
		d.logger.Debug("Metrics WebSocket connection closed")
	}()

	d.logger.Info("Starting metrics stream WebSocket")

	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from metrics client")
		return nil
	})

	metricsTicker := time.NewTicker(5 * time.Second)
	defer metricsTicker.Stop()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	d.sendMetricsUpdate(safeConn)

	for {
		select {
		case <-metricsTicker.C:
			d.sendMetricsUpdate(safeConn)
		case <-pingTicker.C:
			safeConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get status: %v", err),
		})
		return
	}

	connectionsData, err := d.proxyRequest("/api/connections")
	if err != nil {
		d.logger.Error("Failed to get connections for metrics: %v", err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get connections: %v", err),
		})
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

	safeConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
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
		resp.Body.Close()
	}()
}

// Utility functions
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(6))
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
