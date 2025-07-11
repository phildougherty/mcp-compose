// internal/dashboard/inspector_service.go
package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/phildougherty/mcp-compose/internal/constants"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

type InspectorService struct {
	logger     *logging.Logger
	proxyURL   string
	apiKey     string
	httpClient *http.Client
	sessions   map[string]*InspectorSession
	sessionsMu sync.RWMutex
}

type InspectorSession struct {
	ID           string                 `json:"id"`
	ServerName   string                 `json:"serverName"`
	CreatedAt    time.Time              `json:"createdAt"`
	LastUsed     time.Time              `json:"lastUsed"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
}

type InspectorRequest struct {
	SessionID string          `json:"sessionId"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type InspectorResponse struct {
	ID      interface{} `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

func NewInspectorService(logger *logging.Logger, proxyURL, apiKey string) *InspectorService {

	return &InspectorService{
		logger:   logger,
		proxyURL: proxyURL,
		apiKey:   apiKey,
		sessions: make(map[string]*InspectorSession),
		httpClient: &http.Client{
			Timeout: constants.DefaultReadTimeout,
		},
	}
}

func (is *InspectorService) CreateSession(serverName string) (*InspectorSession, error) {
	is.logger.Info("Creating inspector session for server: %s", serverName)

	sessionID := fmt.Sprintf("inspector-%s-%d", serverName, time.Now().UnixNano())

	session := &InspectorSession{
		ID:         sessionID,
		ServerName: serverName,
		CreatedAt:  time.Now(),
		LastUsed:   time.Now(),
	}

	// Try to get server capabilities via the proxy
	// This is optional - if it fails, we'll create the session anyway
	capabilities, err := is.getServerCapabilities(serverName)
	if err != nil {
		is.logger.Info("Could not get capabilities for server %s: %v. Session will be created anyway.", serverName, err)
		// Set empty capabilities but don't fail
		session.Capabilities = map[string]interface{}{}
	} else {
		session.Capabilities = capabilities
		is.logger.Info("Got capabilities for server %s: %v", serverName, capabilities)
	}

	is.sessionsMu.Lock()
	is.sessions[sessionID] = session
	is.sessionsMu.Unlock()

	is.logger.Info("Created inspector session %s for server %s", sessionID, serverName)

	return session, nil
}

func (is *InspectorService) ExecuteRequest(sessionID string, req InspectorRequest) (*InspectorResponse, error) {
	is.sessionsMu.RLock()
	session, exists := is.sessions[sessionID]
	is.sessionsMu.RUnlock()

	if !exists {

		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	// Update last used
	is.sessionsMu.Lock()
	session.LastUsed = time.Now()
	is.sessionsMu.Unlock()

	// Parse the raw JSON params into an interface{}
	var params interface{}
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			is.logger.Error("Failed to parse request params: %v. Raw params: %s", err, string(req.Params))

			return nil, fmt.Errorf("invalid request params: %w", err)
		}
		is.logger.Info("Parsed params for %s.%s: %v (type: %T)", session.ServerName, req.Method, params, params)
	} else {
		// Empty params
		params = map[string]interface{}{}
		is.logger.Info("Using empty params for %s.%s", session.ServerName, req.Method)
	}

	// Execute the request via the MCP proxy
	response, err := is.proxyRequest(session.ServerName, req.Method, params)
	if err != nil {
		is.logger.Error("Proxy request failed for %s.%s: %v", session.ServerName, req.Method, err)

		return nil, fmt.Errorf("proxy request failed: %w", err)
	}

	is.logger.Info("Inspector request %s.%s executed successfully", session.ServerName, req.Method)

	return response, nil
}

func (is *InspectorService) DestroySession(sessionID string) error {
	is.sessionsMu.Lock()
	session, exists := is.sessions[sessionID]
	if exists {
		delete(is.sessions, sessionID)
	}
	is.sessionsMu.Unlock()

	if !exists {

		return fmt.Errorf("session %s not found", sessionID)
	}

	is.logger.Info("Destroyed inspector session %s for server %s", sessionID, session.ServerName)

	return nil
}

func (is *InspectorService) GetSession(sessionID string) (*InspectorSession, error) {
	is.sessionsMu.RLock()
	session, exists := is.sessions[sessionID]
	is.sessionsMu.RUnlock()

	if !exists {

		return nil, fmt.Errorf("session %s not found", sessionID)
	}

	return session, nil
}

func (is *InspectorService) ListSessions() []*InspectorSession {
	is.sessionsMu.RLock()
	defer is.sessionsMu.RUnlock()

	sessions := make([]*InspectorSession, 0, len(is.sessions))
	for _, session := range is.sessions {
		sessions = append(sessions, session)
	}

	return sessions
}

func (is *InspectorService) CleanupExpiredSessions(maxAge time.Duration) int {
	is.sessionsMu.Lock()
	defer is.sessionsMu.Unlock()

	count := 0
	now := time.Now()

	for id, session := range is.sessions {
		if now.Sub(session.LastUsed) > maxAge {
			delete(is.sessions, id)
			count++
			is.logger.Info("Cleaned up expired inspector session %s", id)
		}
	}

	return count
}

func (is *InspectorService) getServerCapabilities(serverName string) (map[string]interface{}, error) {
	response, err := is.proxyRequest(serverName, "initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"roots": map[string]interface{}{
				"listChanged": true,
			},
		},
		"clientInfo": map[string]interface{}{
			"name":    "MCP Dashboard Inspector",
			"version": "1.0.0",
		},
	})

	if err != nil {

		return nil, err
	}

	if response.Result != nil {
		if result, ok := response.Result.(map[string]interface{}); ok {
			if caps, ok := result["capabilities"].(map[string]interface{}); ok {

				return caps, nil
			}
		}
	}

	return nil, fmt.Errorf("no capabilities in response")
}

func (is *InspectorService) proxyRequest(serverName, method string, params interface{}) (*InspectorResponse, error) {
	is.logger.Info("Creating MCP request for %s.%s with params: %v (type: %T)", serverName, method, params, params)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      time.Now().UnixNano(),
		"method":  method,
	}

	// Always include params, even if nil/empty - some servers expect this field
	if params == nil {
		mcpRequest["params"] = map[string]interface{}{}
	} else {
		mcpRequest["params"] = params
	}

	requestBytes, err := json.Marshal(mcpRequest)
	if err != nil {
		is.logger.Error("Failed to marshal MCP request: %v", err)

		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	is.logger.Info("Marshaled JSON length: %d, content: %s", len(requestBytes), string(requestBytes))

	// Ensure we're not sending an empty body
	if len(requestBytes) == 0 {
		is.logger.Error("Generated empty request body for %s.%s", serverName, method)

		return nil, fmt.Errorf("empty request body generated")
	}

	proxyURL := fmt.Sprintf("%s/%s", strings.TrimSuffix(is.proxyURL, "/"), serverName)
	is.logger.Info("Proxy URL: %s", proxyURL)

	// Create a proper reader for the request body using bytes.Buffer
	bodyBuffer := bytes.NewBuffer(requestBytes)

	req, err := http.NewRequest("POST", proxyURL, bodyBuffer)
	if err != nil {
		is.logger.Error("Failed to create HTTP request: %v", err)

		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set content length explicitly
	req.ContentLength = int64(len(requestBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(requestBytes)))

	if is.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+is.apiKey)
	}

	is.logger.Info("About to send HTTP request with headers: %v", req.Header)
	is.logger.Info("Request body buffer size: %d", bodyBuffer.Len())
	is.logger.Info("Sending MCP request to %s with method %s (body length: %d): %s", proxyURL, method, len(requestBytes), string(requestBytes))

	resp, err := is.httpClient.Do(req)
	if err != nil {
		is.logger.Error("HTTP request to proxy failed: %v", err)

		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			is.logger.Error("Failed to close response body: %v", err)
		}
	}()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		is.logger.Error("Failed to read response body: %v", err)

		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	is.logger.Info("Received response from %s (status %d): %s", proxyURL, resp.StatusCode, string(responseBody))

	if resp.StatusCode != http.StatusOK {
		is.logger.Error("Proxy returned non-200 status %d: %s", resp.StatusCode, string(responseBody))

		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(responseBody))
	}

	var mcpResponse InspectorResponse
	if err := json.Unmarshal(responseBody, &mcpResponse); err != nil {
		is.logger.Error("Failed to parse MCP response JSON: %v. Response was: %s", err, string(responseBody))

		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	is.logger.Info("Successfully parsed MCP response for %s.%s", serverName, method)

	return &mcpResponse, nil
}
