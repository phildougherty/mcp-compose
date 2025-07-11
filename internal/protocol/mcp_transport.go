// internal/protocol/mcp_transport.go
package protocol

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/constants"
)

// MCPTransport implements MCP-compliant transport layer
type MCPTransport interface {
	Transport
	// GetType returns the transport type
	GetType() string
	// IsConnected returns true if transport is connected
	IsConnected() bool
	// GetLastActivity returns the last activity timestamp
	GetLastActivity() time.Time
}

// HTTPTransport implements MCP HTTP transport according to specification
type HTTPTransport struct {
	baseURL     string
	sessionID   string
	client      *http.Client
	lastUsed    time.Time
	mu          sync.RWMutex
	initialized bool
	healthy     bool
}

// NewHTTPTransport creates a new MCP-compliant HTTP transport
func NewHTTPTransport(baseURL string) *HTTPTransport {

	return &HTTPTransport{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: constants.HTTPRequestTimeout,
		},
		lastUsed: time.Now(),
		healthy:  true,
	}
}

func (h *HTTPTransport) GetType() string {

	return "http"
}

func (h *HTTPTransport) IsConnected() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.healthy && h.initialized
}

func (h *HTTPTransport) GetLastActivity() time.Time {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.lastUsed
}

func (h *HTTPTransport) Send(msg MCPMessage) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.lastUsed = time.Now()

	// Marshal message
	data, err := json.Marshal(msg)
	if err != nil {

		return NewTransportError("http", fmt.Sprintf("failed to marshal message: %v", err))
	}

	// Create request
	req, err := http.NewRequest("POST", h.baseURL, bytes.NewBuffer(data))
	if err != nil {

		return NewTransportError("http", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set headers per MCP HTTP transport spec
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if h.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", h.sessionID)
	}

	// Send request
	resp, err := h.client.Do(req)
	if err != nil {
		h.healthy = false

		return NewTransportError("http", fmt.Sprintf("request failed: %v", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	// Update session ID if provided
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		h.sessionID = sessionID
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		h.healthy = false
		body, _ := io.ReadAll(resp.Body)

		return NewTransportError("http", fmt.Sprintf("bad status %d: %s", resp.StatusCode, string(body)))
	}

	h.healthy = true

	return nil
}

func (h *HTTPTransport) Receive() (MCPMessage, error) {
	// HTTP transport is request/response - no async receive

	return MCPMessage{}, NewTransportError("http", "HTTP transport does not support async receive")
}

func (h *HTTPTransport) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.healthy = false
	h.initialized = false

	// Send session termination if we have a session
	if h.sessionID != "" {
		req, err := http.NewRequest("DELETE", h.baseURL, nil)
		if err == nil {
			req.Header.Set("Mcp-Session-Id", h.sessionID)
			if resp, err := h.client.Do(req); err == nil && resp != nil {
				// Close response body to prevent resource leaks
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("Warning: failed to close cleanup response body: %v\n", err)
				}
			}
		}
		h.sessionID = ""
	}

	return nil
}

func (h *HTTPTransport) SupportsProgress() bool {

	return true
}

func (h *HTTPTransport) SendProgress(notification *ProgressNotification) error {
	// Convert to generic message
	msg := MCPMessage{
		JSONRPC: notification.JSONRPC,
		Method:  notification.Method,
	}
	params, err := json.Marshal(notification.Params)
	if err != nil {

		return err
	}
	msg.Params = params

	return h.Send(msg)
}

// SSETransport implements MCP SSE transport according to specification
type SSETransport struct {
	sseURL      string
	postURL     string
	sessionID   string
	client      *http.Client
	sseReader   *bufio.Scanner
	sseResponse *http.Response
	lastUsed    time.Time
	mu          sync.RWMutex
	initialized bool
	healthy     bool
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewSSETransport creates a new MCP-compliant SSE transport
func NewSSETransport(sseURL string) *SSETransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &SSETransport{
		sseURL: sseURL,
		client: &http.Client{
			Timeout: 0, // No timeout for SSE connections
		},
		lastUsed: time.Now(),
		healthy:  true,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *SSETransport) GetType() string {

	return "sse"
}

func (s *SSETransport) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.healthy && s.initialized && s.sseReader != nil
}

func (s *SSETransport) GetLastActivity() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastUsed
}

func (s *SSETransport) Send(msg MCPMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastUsed = time.Now()

	if s.postURL == "" {

		return NewTransportError("sse", "no post endpoint available")
	}

	// Marshal message
	data, err := json.Marshal(msg)
	if err != nil {

		return NewTransportError("sse", fmt.Sprintf("failed to marshal message: %v", err))
	}

	// Create request
	req, err := http.NewRequest("POST", s.postURL, bytes.NewBuffer(data))
	if err != nil {

		return NewTransportError("sse", fmt.Sprintf("failed to create request: %v", err))
	}

	// Set headers per MCP SSE transport spec
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if s.sessionID != "" {
		req.Header.Set("Mcp-Session-Id", s.sessionID)
	}

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		s.healthy = false

		return NewTransportError("sse", fmt.Sprintf("request failed: %v", err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
	}()

	// Update session ID if provided
	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		s.sessionID = sessionID
	}

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.healthy = false
		body, _ := io.ReadAll(resp.Body)

		return NewTransportError("sse", fmt.Sprintf("bad status %d: %s", resp.StatusCode, string(body)))
	}

	s.healthy = true

	return nil
}

func (s *SSETransport) Receive() (MCPMessage, error) {
	s.mu.RLock()
	reader := s.sseReader
	s.mu.RUnlock()

	if reader == nil {

		return MCPMessage{}, NewTransportError("sse", "SSE connection not established")
	}

	// Read SSE events
	for reader.Scan() {
		line := reader.Text()

		// Parse SSE format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Parse JSON message
			var msg MCPMessage
			if err := json.Unmarshal([]byte(data), &msg); err != nil {
				continue // Skip invalid messages
			}

			s.mu.Lock()
			s.lastUsed = time.Now()
			s.mu.Unlock()

			return msg, nil
		}
	}

	// Check for scanner error
	if err := reader.Err(); err != nil {
		s.mu.Lock()
		s.healthy = false
		s.mu.Unlock()

		return MCPMessage{}, NewTransportError("sse", fmt.Sprintf("SSE read error: %v", err))
	}

	// Connection closed
	s.mu.Lock()
	s.healthy = false
	s.mu.Unlock()

	return MCPMessage{}, NewTransportError("sse", "SSE connection closed")
}

func (s *SSETransport) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.healthy = false
	s.initialized = false

	if s.cancel != nil {
		s.cancel()
	}

	if s.sseResponse != nil {
		if err := s.sseResponse.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close SSE response body: %v\n", err)
		}
		s.sseResponse = nil
	}

	s.sseReader = nil

	return nil
}

func (s *SSETransport) SupportsProgress() bool {

	return true
}

func (s *SSETransport) SendProgress(notification *ProgressNotification) error {
	// Convert to generic message
	msg := MCPMessage{
		JSONRPC: notification.JSONRPC,
		Method:  notification.Method,
	}
	params, err := json.Marshal(notification.Params)
	if err != nil {

		return err
	}
	msg.Params = params

	return s.Send(msg)
}

// Initialize SSE connection
func (s *SSETransport) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create SSE request
	req, err := http.NewRequestWithContext(s.ctx, "GET", s.sseURL, nil)
	if err != nil {

		return NewTransportError("sse", fmt.Sprintf("failed to create SSE request: %v", err))
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// Connect to SSE endpoint
	resp, err := s.client.Do(req)
	if err != nil {
		s.healthy = false

		return NewTransportError("sse", fmt.Sprintf("SSE connection failed: %v", err))
	}

	if resp.StatusCode != http.StatusOK {
		if err := resp.Body.Close(); err != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", err)
		}
		s.healthy = false

		return NewTransportError("sse", fmt.Sprintf("SSE bad status: %d", resp.StatusCode))
	}

	// Set up SSE reader
	s.sseResponse = resp
	s.sseReader = bufio.NewScanner(resp.Body)
	s.initialized = true
	s.healthy = true

	return nil
}
