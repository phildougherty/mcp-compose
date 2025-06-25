package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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

// WebSocket connection wrapper with mutex for safe concurrent writes
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

func (d *DashboardServer) handleLogWebSocket(w http.ResponseWriter, r *http.Request) {
	serverName := r.URL.Query().Get("server")
	if serverName == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		d.logger.Error("Failed to upgrade websocket connection: %v", err)
		return
	}

	// Wrap connection for safe concurrent access
	safeConn := &SafeWebSocketConn{conn: conn}

	defer func() {
		safeConn.Close()
		d.logger.Debug("WebSocket connection closed for server: %s", serverName)
	}()

	d.logger.Info("Starting log stream WebSocket for server: %s", serverName)

	// Set up ping/pong to keep connection alive
	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from client for server: %s", serverName)
		return nil
	})

	// Start Docker logs command
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

	// Channel to handle container exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Read from both stdout and stderr with safe connection
	go d.streamLogs(safeConn, stdout, serverName, "stdout", cancel)
	go d.streamLogs(safeConn, stderr, serverName, "stderr", cancel)

	// Set up ping ticker to keep connection alive
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Handle WebSocket lifecycle
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
	// Upgrade HTTP connection to WebSocket
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		d.logger.Error("Failed to upgrade metrics websocket connection: %v", err)
		return
	}

	// Wrap connection for safe concurrent access
	safeConn := &SafeWebSocketConn{conn: conn}

	defer func() {
		safeConn.Close()
		d.logger.Debug("Metrics WebSocket connection closed")
	}()

	d.logger.Info("Starting metrics stream WebSocket")

	// Set up ping/pong to keep connection alive
	conn.SetPongHandler(func(string) error {
		d.logger.Debug("Received pong from metrics client")
		return nil
	})

	// Set up tickers
	metricsTicker := time.NewTicker(5 * time.Second)
	defer metricsTicker.Stop()

	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// Send initial data immediately
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
	// Get current status from proxy
	statusData, err := d.proxyRequest("/api/status")
	if err != nil {
		d.logger.Error("Failed to get status for metrics: %v", err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get status: %v", err),
		})
		return
	}

	// Get connections
	connectionsData, err := d.proxyRequest("/api/connections")
	if err != nil {
		d.logger.Error("Failed to get connections for metrics: %v", err)
		safeConn.WriteJSON(map[string]string{
			"error": fmt.Sprintf("Failed to get connections: %v", err),
		})
		return
	}

	// Parse JSON responses
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
	// Check for common log level indicators
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
	// Default to INFO if we can't determine the level
	return "INFO"
}
