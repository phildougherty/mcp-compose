// internal/server/stdio_bridge.go
package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"mcpcompose/internal/constants"
	"mcpcompose/internal/logging"
)

// STDIOBridge handles stdio communication for MCP hosts like Claude Desktop
type STDIOBridge struct {
	ProxyURL   string
	ServerName string
	APIKey     string
	logger     *logging.Logger
	stdin      io.Reader
	stdout     io.Writer
	httpClient *http.Client
	sessionID  string
}

// NewSTDIOBridge creates a new stdio bridge
func NewSTDIOBridge(proxyURL, serverName, apiKey string, stdin io.Reader, stdout io.Writer) *STDIOBridge {

	return &STDIOBridge{
		ProxyURL:   proxyURL,
		ServerName: serverName,
		APIKey:     apiKey,
		logger:     logging.NewLogger("info"),
		stdin:      stdin,
		stdout:     stdout,
		httpClient: &http.Client{
			Timeout: constants.STDIOBridgeTimeout,
		},
	}
}

// Start begins the stdio bridge loop
func (b *STDIOBridge) Start() error {
	b.logger.Info("Starting MCP STDIO bridge for server '%s' via proxy '%s'", b.ServerName, b.ProxyURL)

	scanner := bufio.NewScanner(b.stdin)

	for scanner.Scan() {
		line := scanner.Text()

		var request map[string]interface{}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			b.logger.Error("Invalid JSON from stdio: %v", err)
			b.sendErrorResponse(request, -32700, "Parse error")

			continue
		}

		// Forward to HTTP proxy and get response
		response, err := b.forwardToProxy(request)
		if err != nil {
			b.logger.Error("Error forwarding to proxy: %v", err)
			b.sendErrorResponse(request, -32603, err.Error())

			continue
		}

		// Send response back via stdout
		responseBytes, _ := json.Marshal(response)
		if _, err := fmt.Fprintln(b.stdout, string(responseBytes)); err != nil {
			b.logger.Error("Failed to write response to stdout: %v", err)
		}
	}


	return scanner.Err()
}

func (b *STDIOBridge) forwardToProxy(request map[string]interface{}) (map[string]interface{}, error) {
	targetURL := fmt.Sprintf("%s/%s", b.ProxyURL, b.ServerName)

	requestBytes, _ := json.Marshal(request)

	ctx, cancel := context.WithTimeout(context.Background(), constants.STDIOBridgeConnectTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", targetURL, bytes.NewBuffer(requestBytes))
	if err != nil {

		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if b.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+b.APIKey)
	}

	// Forward session ID if we have one
	if b.sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", b.sessionID)
	}

	resp, err := b.httpClient.Do(httpReq)
	if err != nil {

		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Store session ID from response
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		b.sessionID = sessionID
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, constants.HTTPResponseBufferSize))

		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {

		return nil, fmt.Errorf("failed to decode response: %w", err)
	}


	return response, nil
}

func (b *STDIOBridge) sendErrorResponse(request map[string]interface{}, code int, message string) {
	var id interface{}
	if request != nil {
		id = request["id"]
	}

	errorResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}

	responseBytes, _ := json.Marshal(errorResponse)
	_, _ = fmt.Fprintln(b.stdout, string(responseBytes))
}
