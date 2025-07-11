package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// MCPSTDIOConnection represents a STDIO connection to an MCP server
type MCPSTDIOConnection struct {
	ServerName  string
	Host        string
	Port        int
	Connection  net.Conn
	Reader      *bufio.Reader
	Writer      *bufio.Writer
	LastUsed    time.Time
	Initialized bool
	Healthy     bool
	mu          sync.Mutex
}

func (h *ProxyHandler) getStdioConnection(serverName string) (*MCPSTDIOConnection, error) {
	h.StdioMutex.RLock()
	conn, exists := h.StdioConnections[serverName]
	h.StdioMutex.RUnlock()

	if exists && h.isStdioConnectionReallyHealthy(conn) {
		conn.mu.Lock()
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
		h.logger.Debug("Reusing healthy STDIO connection for %s", serverName)
		return conn, nil
	}

	// If we have an unhealthy connection, clean it up
	if exists && !h.isStdioConnectionReallyHealthy(conn) {
		h.logger.Info("Cleaning up unhealthy STDIO connection for %s", serverName)
		h.StdioMutex.Lock()
		if conn.Connection != nil {
			if err := conn.Connection.Close(); err != nil {
				h.logger.Warning("Failed to close unhealthy STDIO connection to %s: %v", serverName, err)
			}
		}
		delete(h.StdioConnections, serverName)
		h.StdioMutex.Unlock()
	}

	h.logger.Info("Creating new STDIO connection for server: %s", serverName)

	// Retry connection creation up to 3 times
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		conn, err := h.createStdioConnection(serverName)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		h.logger.Warning("STDIO connection attempt %d/3 failed for %s: %v", attempt, serverName, err)
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed to create STDIO connection after 3 attempts: %w", lastErr)
}

func (h *ProxyHandler) createStdioConnection(serverName string) (*MCPSTDIOConnection, error) {
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	port := serverConfig.StdioHosterPort
	address := fmt.Sprintf("%s:%d", containerName, port)

	// Use shorter connection timeout
	var d net.Dialer
	ctx, cancel := context.WithTimeout(h.ctx, 15*time.Second)
	defer cancel()

	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Enable TCP keepalive with aggressive settings
	if tcpConn, ok := netConn.(*net.TCPConn); ok {
		if err := tcpConn.SetKeepAlive(true); err != nil {
			h.logger.Warning("Failed to enable TCP keepalive for %s: %v", serverName, err)
		}
		if err := tcpConn.SetKeepAlivePeriod(15 * time.Second); err != nil {
			h.logger.Warning("Failed to set TCP keepalive period for %s: %v", serverName, err)
		}
		if err := tcpConn.SetNoDelay(true); err != nil {
			h.logger.Warning("Failed to set TCP no delay for %s: %v", serverName, err)
		}
		h.logger.Debug("Enabled TCP keepalive for connection to %s", serverName)
	}

	conn := &MCPSTDIOConnection{
		ServerName:  serverName,
		Host:        containerName,
		Port:        port,
		Connection:  netConn,
		Reader:      bufio.NewReaderSize(netConn, 8192),
		Writer:      bufio.NewWriterSize(netConn, 8192),
		LastUsed:    time.Now(),
		Healthy:     true,
		Initialized: false,
	}

	// Initialize the connection with shorter timeout
	if err := h.initializeStdioConnection(conn); err != nil {
		if closeErr := conn.Connection.Close(); closeErr != nil {
			h.logger.Warning("Failed to close connection after init failure for %s: %v", serverName, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize STDIO connection to %s: %w", serverName, err)
	}

	h.StdioMutex.Lock()
	if h.StdioConnections == nil {
		h.StdioConnections = make(map[string]*MCPSTDIOConnection)
	}
	h.StdioConnections[serverName] = conn
	h.StdioMutex.Unlock()

	h.logger.Info("Successfully created and initialized STDIO connection for %s", serverName)
	return conn, nil
}

func (h *ProxyHandler) initializeStdioConnection(conn *MCPSTDIOConnection) error {
	h.logger.Info("Initializing STDIO connection to %s", conn.ServerName)

	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcp-compose-proxy",
				"version": "1.0.0",
			},
		},
	}

	// Set initial deadline for initialization
	if err := conn.Connection.SetWriteDeadline(time.Now().Add(30 * time.Second)); err != nil {
		h.logger.Warning("Failed to set write deadline for %s: %v", conn.ServerName, err)
	}
	if err := conn.Connection.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		h.logger.Warning("Failed to set read deadline for %s: %v", conn.ServerName, err)
	}

	if err := h.sendStdioRequestWithoutLock(conn, initRequest); err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	// Read initialize response
	response, err := h.readStdioResponseWithoutLock(conn)
	if err != nil {
		return fmt.Errorf("failed to read initialize response: %w", err)
	}

	if mcpError, hasError := response["error"]; hasError {
		return fmt.Errorf("initialize failed: %v", mcpError)
	}

	// Send initialized notification - this is critical and was missing proper handling
	initNotification := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params":  map[string]interface{}{},
	}

	if err := h.sendStdioRequestWithoutLock(conn, initNotification); err != nil {
		// Don't fail if notification fails, some servers don't support it
		h.logger.Warning("Failed to send initialized notification to %s: %v (continuing anyway)", conn.ServerName, err)
	}

	// Reset deadlines after successful initialization
	if err := conn.Connection.SetWriteDeadline(time.Time{}); err != nil {
		h.logger.Warning("Failed to reset write deadline for %s: %v", conn.ServerName, err)
	}
	if err := conn.Connection.SetReadDeadline(time.Time{}); err != nil {
		h.logger.Warning("Failed to reset read deadline for %s: %v", conn.ServerName, err)
	}

	conn.mu.Lock()
	conn.Initialized = true
	conn.Healthy = true
	conn.mu.Unlock()

	h.logger.Info("STDIO connection to %s initialized successfully", conn.ServerName)
	return nil
}

func (h *ProxyHandler) sendStdioRequestWithoutLock(conn *MCPSTDIOConnection, request map[string]interface{}) error {
	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	h.logger.Debug("Sending STDIO request to %s: %s", conn.ServerName, string(requestData))

	// Write with newline
	_, err = conn.Writer.WriteString(string(requestData) + "\n")
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to write request: %w", err)
	}

	err = conn.Writer.Flush()
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to flush request: %w", err)
	}

	return nil
}

func (h *ProxyHandler) readStdioResponseWithoutLock(conn *MCPSTDIOConnection) (map[string]interface{}, error) {
	for {
		line, err := conn.Reader.ReadString('\n')
		if err != nil {
			conn.Healthy = false
			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		h.logger.Debug("Received STDIO line from %s: %s", conn.ServerName, line)

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			h.logger.Debug("Skipping non-JSON line from %s: %s", conn.ServerName, line)
			continue
		}

		_, hasResult := response["result"]
		_, hasError := response["error"]
		_, hasMethod := response["method"]

		if (hasResult || hasError) && !hasMethod {
			h.logger.Debug("Found valid JSON-RPC response from %s", conn.ServerName)
			return response, nil
		} else if hasMethod {
			h.logger.Debug("Skipping echoed request/notification from %s: %s", conn.ServerName, line)
			continue
		} else {
			h.logger.Debug("Skipping unknown JSON structure from %s: %s", conn.ServerName, line)
			continue
		}
	}
}

func (h *ProxyHandler) sendStdioRequest(conn *MCPSTDIOConnection, request map[string]interface{}) error {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	requestData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	h.logger.Debug("Sending STDIO request to %s: %s", conn.ServerName, string(requestData))

	// Set reasonable write deadline - longer than before
	writeDeadline := time.Now().Add(60 * time.Second)
	if err := conn.Connection.SetWriteDeadline(writeDeadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	defer func() {
		if err := conn.Connection.SetWriteDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset write deadline: %v", err)
		}
	}()

	// Write with newline
	_, err = conn.Writer.WriteString(string(requestData) + "\n")
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to write request: %w", err)
	}

	err = conn.Writer.Flush()
	if err != nil {
		conn.Healthy = false
		return fmt.Errorf("failed to flush request: %w", err)
	}

	return nil
}

func (h *ProxyHandler) readStdioResponse(conn *MCPSTDIOConnection) (map[string]interface{}, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Set reasonable read deadline - longer for complex operations
	readDeadline := time.Now().Add(60 * time.Second)
	if err := conn.Connection.SetReadDeadline(readDeadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer func() {
		if err := conn.Connection.SetReadDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset read deadline: %v", err)
		}
	}()

	for {
		line, err := conn.Reader.ReadString('\n')
		if err != nil {
			conn.Healthy = false
			return nil, fmt.Errorf("failed to read line: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		h.logger.Debug("Received STDIO line from %s: %s", conn.ServerName, line)

		var response map[string]interface{}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			h.logger.Debug("Skipping non-JSON line from %s: %s", conn.ServerName, line)
			continue
		}

		_, hasResult := response["result"]
		_, hasError := response["error"]
		_, hasMethod := response["method"]

		if (hasResult || hasError) && !hasMethod {
			h.logger.Debug("Found valid JSON-RPC response from %s", conn.ServerName)
			return response, nil
		} else if hasMethod {
			h.logger.Debug("Skipping echoed request/notification from %s: %s", conn.ServerName, line)
			continue
		} else {
			h.logger.Debug("Skipping unknown JSON structure from %s: %s", conn.ServerName, line)
			continue
		}
	}
}

func (h *ProxyHandler) isStdioConnectionReallyHealthy(conn *MCPSTDIOConnection) bool {
	if conn == nil || conn.Connection == nil {
		return false
	}
	conn.mu.Lock()
	defer conn.mu.Unlock()
	// Simply check the flags, don't do active probing
	return conn.Healthy && conn.Initialized
}

func (h *ProxyHandler) maintainStdioConnections() {
	h.StdioMutex.Lock()
	defer h.StdioMutex.Unlock()

	for serverName, conn := range h.StdioConnections {
		if conn == nil {
			continue
		}

		// Increase idle time back to reasonable levels
		maxIdleTime := 15 * time.Minute
		if time.Since(conn.LastUsed) > maxIdleTime {
			h.logger.Info("Closing idle STDIO connection to %s (idle for %v)",
				serverName, time.Since(conn.LastUsed))
			if conn.Connection != nil {
				if err := conn.Connection.Close(); err != nil {
					h.logger.Warning("Failed to close idle STDIO connection to %s: %v", serverName, err)
				}
			}
			delete(h.StdioConnections, serverName)
		}
	}
}

func (h *ProxyHandler) createFreshStdioConnection(serverName string, timeout time.Duration) (*MCPSTDIOConnection, error) {
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		return nil, fmt.Errorf("server %s not found in config", serverName)
	}

	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	port := serverConfig.StdioHosterPort
	address := fmt.Sprintf("%s:%d", containerName, port)

	// Use the specified timeout for connection
	var d net.Dialer
	ctx, cancel := context.WithTimeout(h.ctx, timeout)
	defer cancel()

	netConn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Set up the connection but don't store it in the main connection pool
	conn := &MCPSTDIOConnection{
		ServerName:  serverName,
		Host:        containerName,
		Port:        port,
		Connection:  netConn,
		Reader:      bufio.NewReaderSize(netConn, 8192),
		Writer:      bufio.NewWriterSize(netConn, 8192),
		LastUsed:    time.Now(),
		Healthy:     true,
		Initialized: false,
	}

	// Quick initialization for tool discovery
	if err := h.quickInitializeStdioConnection(conn, timeout); err != nil {
		if closeErr := conn.Connection.Close(); closeErr != nil {
			h.logger.Warning("Failed to close connection after quick init failure for %s: %v", serverName, closeErr)
		}
		return nil, fmt.Errorf("failed to initialize connection: %w", err)
	}

	return conn, nil
}

func (h *ProxyHandler) quickInitializeStdioConnection(conn *MCPSTDIOConnection, timeout time.Duration) error {
	// Set deadline for entire initialization
	deadline := time.Now().Add(timeout)
	if err := conn.Connection.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	if err := conn.Connection.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer func() {
		if err := conn.Connection.SetWriteDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset write deadline: %v", err)
		}
		if err := conn.Connection.SetReadDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset read deadline: %v", err)
		}
	}()

	// Send initialize request
	initRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "mcp-compose-proxy",
				"version": "1.0.0",
			},
		},
	}

	if err := h.sendStdioRequestWithoutLock(conn, initRequest); err != nil {
		return err
	}

	response, err := h.readStdioResponseWithoutLock(conn)
	if err != nil {
		return err
	}

	if mcpError, hasError := response["error"]; hasError {
		return fmt.Errorf("initialize failed: %v", mcpError)
	}

	conn.Initialized = true
	conn.Healthy = true
	return nil
}

func (h *ProxyHandler) sendStdioRequestWithTimeout(conn *MCPSTDIOConnection, request map[string]interface{}, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	if err := conn.Connection.SetWriteDeadline(deadline); err != nil {
		return fmt.Errorf("failed to set write deadline: %w", err)
	}
	defer func() {
		if err := conn.Connection.SetWriteDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset write deadline: %v", err)
		}
	}()

	return h.sendStdioRequestWithoutLock(conn, request)
}

func (h *ProxyHandler) readStdioResponseWithTimeout(conn *MCPSTDIOConnection, timeout time.Duration) (map[string]interface{}, error) {
	deadline := time.Now().Add(timeout)
	if err := conn.Connection.SetReadDeadline(deadline); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}
	defer func() {
		if err := conn.Connection.SetReadDeadline(time.Time{}); err != nil {
			h.logger.Warning("Failed to reset read deadline: %v", err)
		}
	}()

	return h.readStdioResponseWithoutLock(conn)
}

func (h *ProxyHandler) handleSTDIOServerRequest(w http.ResponseWriter, _ *http.Request, serverName string, requestPayload map[string]interface{}, reqIDVal interface{}, reqMethodVal string) {
	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	serverCfg, cfgExists := h.Manager.config.Servers[serverName]
	if !cfgExists {
		h.logger.Error("Config not found for STDIO server %s", serverName)
		h.sendMCPError(w, reqIDVal, -32603, "Internal server error: missing server config")
		return
	}

	h.logger.Info("Executing STDIO request for server '%s' via container '%s' using its defined command.", serverName, containerName)

	requestJSON, err := json.Marshal(requestPayload)
	if err != nil {
		h.logger.Error("Failed to marshal request for STDIO server %s: %v", serverName, err)
		h.sendMCPError(w, reqIDVal, -32700, "Failed to marshal request")
		return
	}

	jsonInputWithNewline := string(append(requestJSON, '\n'))

	// Prepare the command to be executed inside the container
	execCmdAndArgs := []string{"exec", "-i", containerName}
	if serverCfg.Command == "" {
		h.logger.Error("STDIO Server '%s' has no command defined in config. Cannot execute.", serverName)
		h.sendMCPError(w, reqIDVal, -32603, "Internal server error: STDIO server has no command")
		return
	}

	execCmdAndArgs = append(execCmdAndArgs, serverCfg.Command)
	execCmdAndArgs = append(execCmdAndArgs, serverCfg.Args...)

	ctx, cancel := context.WithTimeout(h.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.Command("docker", execCmdAndArgs...)
	cmd.Stdin = strings.NewReader(jsonInputWithNewline)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	h.logger.Debug("Executing for STDIO '%s': docker %s", serverName, strings.Join(execCmdAndArgs, " "))

	err = cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			h.logger.Error("Docker exec for STDIO server %s timed out. Stderr: %s. Stdout: %s", serverName, stderr.String(), stdout.String())
			h.recordConnectionEvent(serverName, false, true)
			h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Timeout communicating with STDIO server '%s'", serverName))
			return
		}
		h.logger.Error("Docker exec for STDIO server %s failed: %v. Stderr: %s. Stdout: %s", serverName, err, stderr.String(), stdout.String())
		h.recordConnectionEvent(serverName, false, false)
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Failed to execute command in STDIO server '%s'", serverName))
		return
	}

	responseData := stdout.Bytes()
	if len(responseData) == 0 {
		h.logger.Error("No stdout response from STDIO server %s. Stderr: %s", serverName, stderr.String())
		h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("No stdout from STDIO server '%s'", serverName))
		return
	}

	h.logger.Debug("Raw stdout from STDIO server '%s': %s", serverName, string(responseData))

	// Parse the response
	var response map[string]interface{}
	trimmedResponseData := bytes.TrimSpace(responseData)
	if err := json.Unmarshal(trimmedResponseData, &response); err != nil {
		h.logger.Error("Invalid JSON response from STDIO server %s: %v. Raw: %s", serverName, err, string(trimmedResponseData))
		h.sendMCPError(w, reqIDVal, -32700, fmt.Sprintf("Invalid response from STDIO server '%s'", serverName))
		return
	}

	h.recordConnectionEvent(serverName, true, false)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
	h.logger.Info("Successfully forwarded STDIO request to %s (method: %s, ID: %v)", serverName, reqMethodVal, reqIDVal)
}

func (h *ProxyHandler) handleSocatSTDIOServerRequest(w http.ResponseWriter, r *http.Request, serverName string, requestPayload map[string]interface{}, reqIDVal interface{}, _ string) {
	conn, err := h.getStdioConnection(serverName)
	if err != nil {
		h.logger.Error("Failed to get STDIO connection for %s: %v", serverName, err)
		h.recordConnectionEvent(serverName, false, strings.Contains(err.Error(), "timeout"))
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout") {
			h.sendMCPError(w, reqIDVal, -32001, fmt.Sprintf("Server '%s' timed out - connection may be overloaded", serverName))
		} else {
			h.sendMCPError(w, reqIDVal, -32001, fmt.Sprintf("Cannot connect to server '%s'", serverName))
		}
		return
	}

	// Increase timeout for complex operations
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	// Create channels to handle the response
	responseChan := make(chan map[string]interface{}, 1)
	errorChan := make(chan error, 1)

	go func() {
		// Send the request
		if err := h.sendStdioRequest(conn, requestPayload); err != nil {
			errorChan <- err
			return
		}

		// Read the response
		response, err := h.readStdioResponse(conn)
		if err != nil {
			errorChan <- err
			return
		}

		responseChan <- response
	}()

	select {
	case response := <-responseChan:
		h.recordConnectionEvent(serverName, true, false)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	case err := <-errorChan:
		h.logger.Error("Failed to communicate with %s: %v", serverName, err)
		isTimeout := strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout")
		h.recordConnectionEvent(serverName, false, isTimeout)
		if isTimeout {
			h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Server '%s' request timed out", serverName))
		} else {
			h.sendMCPError(w, reqIDVal, -32003, fmt.Sprintf("Error communicating with server '%s'", serverName))
		}
	case <-ctx.Done():
		h.logger.Error("Request to %s timed out", serverName)
		h.recordConnectionEvent(serverName, false, true)
		h.sendMCPError(w, reqIDVal, -32000, fmt.Sprintf("Request to server '%s' timed out", serverName))
	}
}

func (h *ProxyHandler) sendRawTCPRequestWithRetry(host string, port int, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	// Find server name for connection tracking
	var serverName string
	for name, config := range h.Manager.config.Servers {
		containerName := fmt.Sprintf("mcp-compose-%s", name)
		if containerName == host && config.StdioHosterPort == port {
			serverName = name
			break
		}
	}

	if serverName == "" {
		return nil, fmt.Errorf("could not identify server for host %s:%d", host, port)
	}

	h.logger.Debug("Attempting TCP request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)

	// For tool discovery, create a fresh connection each time to avoid stale connection issues
	conn, err := h.createFreshStdioConnection(serverName, timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}
	defer func() {
		if conn.Connection != nil {
			if err := conn.Connection.Close(); err != nil {
				h.logger.Warning("Failed to close temporary STDIO connection to %s: %v", serverName, err)
			}
		}
	}()

	// Send request with the specified timeout
	if err := h.sendStdioRequestWithTimeout(conn, requestPayload, timeout); err != nil {
		return nil, err
	}

	// Read response with the specified timeout
	return h.readStdioResponseWithTimeout(conn, timeout)
}
