package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"mcpcompose/internal/auth"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/protocol"
)

// MockRuntime for testing server manager
type MockRuntime struct {
	containers      map[string]*container.ContainerInfo
	networks        map[string]bool
	startErrors     map[string]error
	stopErrors      map[string]error
	shouldFailBuild bool
	buildCommands   [][]string
	runtimeName     string
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		containers:    make(map[string]*container.ContainerInfo),
		networks:      make(map[string]bool),
		startErrors:   make(map[string]error),
		stopErrors:    make(map[string]error),
		buildCommands: make([][]string, 0),
		runtimeName:   "mock",
	}
}

func (m *MockRuntime) GetRuntimeName() string {
	return m.runtimeName
}

func (m *MockRuntime) StartContainer(opts *container.ContainerOptions) (string, error) {
	if err, exists := m.startErrors[opts.Name]; exists {
		return "", err
	}

	containerID := "mock-" + opts.Name
	info := &container.ContainerInfo{
		ID:     containerID,
		Name:   opts.Name,
		Status: "running",
		State:  "running",
		Image:  opts.Image,
		Ports:  []container.PortBinding{},
		Env:    mapToSlice(opts.Env),
		Labels: opts.Labels,
	}

	m.containers[opts.Name] = info
	return containerID, nil
}

func (m *MockRuntime) StopContainer(name string) error {
	if err, exists := m.stopErrors[name]; exists {
		return err
	}

	if info, exists := m.containers[name]; exists {
		info.Status = "stopped"
		info.State = "exited"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerStatus(name string) (string, error) {
	if info, exists := m.containers[name]; exists {
		return info.Status, nil
	}
	return "", fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerInfo(name string) (*container.ContainerInfo, error) {
	if info, exists := m.containers[name]; exists {
		return info, nil
	}
	return nil, fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) ListContainers(filters map[string]string) ([]container.ContainerInfo, error) {
	var containers []container.ContainerInfo
	for _, info := range m.containers {
		containers = append(containers, *info)
	}
	return containers, nil
}

func (m *MockRuntime) ShowContainerLogs(name string, follow bool) error {
	if _, exists := m.containers[name]; exists {
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) NetworkExists(name string) (bool, error) {
	return m.networks[name], nil
}

func (m *MockRuntime) CreateNetwork(name string) error {
	m.networks[name] = true
	return nil
}

// Implement remaining interface methods minimally
func (m *MockRuntime) RestartContainer(name string) error { return nil }
func (m *MockRuntime) PauseContainer(name string) error   { return nil }
func (m *MockRuntime) UnpauseContainer(name string) error { return nil }
func (m *MockRuntime) GetContainerStats(name string) (*container.ContainerStats, error) {
	return &container.ContainerStats{}, nil
}
func (m *MockRuntime) WaitForContainer(name string, condition string) error { return nil }
func (m *MockRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	return nil, nil, nil, nil
}
func (m *MockRuntime) PullImage(image string, auth *container.ImageAuth) error { return nil }
func (m *MockRuntime) BuildImage(opts *container.BuildOptions) error           { return nil }
func (m *MockRuntime) RemoveImage(image string, force bool) error              { return nil }
func (m *MockRuntime) ListImages() ([]container.ImageInfo, error)              { return []container.ImageInfo{}, nil }
func (m *MockRuntime) CreateVolume(name string, opts *container.VolumeOptions) error { return nil }
func (m *MockRuntime) RemoveVolume(name string, force bool) error                    { return nil }
func (m *MockRuntime) ListVolumes() ([]container.VolumeInfo, error)                 { return []container.VolumeInfo{}, nil }
func (m *MockRuntime) DeleteNetwork(name string) error                              { delete(m.networks, name); return nil }
func (m *MockRuntime) RemoveNetwork(name string) error                              { delete(m.networks, name); return nil }
func (m *MockRuntime) ListNetworks() ([]container.NetworkInfo, error)              { return []container.NetworkInfo{}, nil }
func (m *MockRuntime) GetNetworkInfo(name string) (*container.NetworkInfo, error)  { return nil, nil }
func (m *MockRuntime) ConnectToNetwork(containerName, networkName string) error    { return nil }
func (m *MockRuntime) DisconnectFromNetwork(containerName, networkName string) error { return nil }
func (m *MockRuntime) UpdateContainerResources(name string, resources *container.ResourceLimits) error {
	return nil
}
func (m *MockRuntime) ValidateSecurityContext(opts *container.ContainerOptions) error { return nil }

// Helper function
func mapToSlice(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func TestNewManager(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.config != cfg {
		t.Error("Expected config to be set")
	}

	if manager.runtime != runtime {
		t.Error("Expected runtime to be set")
	}

	if manager.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if len(manager.servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(manager.servers))
	}

	if manager.servers["test-server"] == nil {
		t.Error("Expected test-server to be initialized")
	}
}

func TestServerInstance(t *testing.T) {
	cfg := config.ServerConfig{
		Command: "echo",
		Args:    []string{"hello"},
		Protocol: "stdio",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	instance := &ServerInstance{
		Name:         "test-server",
		Config:       cfg,
		IsContainer:  false,
		Status:       "stopped",
		StartTime:    time.Now(),
		Capabilities: make(map[string]bool),
		ConnectionInfo: map[string]string{
			"protocol": "stdio",
		},
		HealthStatus: "healthy",
		ctx:          ctx,
		cancel:       cancel,
	}

	if instance.Name != "test-server" {
		t.Errorf("Expected name 'test-server', got %s", instance.Name)
	}

	if instance.IsContainer {
		t.Error("Expected IsContainer to be false")
	}

	if instance.Status != "stopped" {
		t.Errorf("Expected status 'stopped', got %s", instance.Status)
	}

	if instance.ConnectionInfo["protocol"] != "stdio" {
		t.Errorf("Expected protocol 'stdio', got %s", instance.ConnectionInfo["protocol"])
	}
}

func TestNewProxyHandler(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	if proxyHandler == nil {
		t.Fatal("Expected proxy handler to be created")
	}

	if proxyHandler.Manager != manager {
		t.Error("Expected manager to be set")
	}

	if proxyHandler.ConfigFile != "config.yaml" {
		t.Errorf("Expected config file 'config.yaml', got %s", proxyHandler.ConfigFile)
	}

	if proxyHandler.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got %s", proxyHandler.APIKey)
	}

	if !proxyHandler.EnableAPI {
		t.Error("Expected EnableAPI to be true")
	}

	if proxyHandler.ServerConnections == nil {
		t.Error("Expected ServerConnections to be initialized")
	}

	if proxyHandler.SSEConnections == nil {
		t.Error("Expected SSEConnections to be initialized")
	}

	if proxyHandler.StdioConnections == nil {
		t.Error("Expected StdioConnections to be initialized")
	}
}

func TestMCPRequest(t *testing.T) {
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-id",
		Method:  "tools/list",
		Params:  map[string]interface{}{"include_context": true},
	}

	// Test JSON marshaling
	data, err := json.Marshal(request)
	if err != nil {
		t.Errorf("Failed to marshal MCP request: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MCPRequest
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal MCP request: %v", err)
	}

	if unmarshaled.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got %s", unmarshaled.JSONRPC)
	}

	if unmarshaled.Method != "tools/list" {
		t.Errorf("Expected method 'tools/list', got %s", unmarshaled.Method)
	}
}

func TestMCPResponse(t *testing.T) {
	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      "test-id",
		Result: map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "test-tool",
					"description": "A test tool",
				},
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Failed to marshal MCP response: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MCPResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal MCP response: %v", err)
	}

	if unmarshaled.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC '2.0', got %s", unmarshaled.JSONRPC)
	}

	if unmarshaled.Result == nil {
		t.Error("Expected result to be set")
	}
}

func TestMCPError(t *testing.T) {
	mcpError := MCPError{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    map[string]interface{}{"details": "Missing required parameter"},
	}

	response := MCPResponse{
		JSONRPC: "2.0",
		ID:      "test-id",
		Error:   &mcpError,
	}

	// Test JSON marshaling
	data, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Failed to marshal MCP error response: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MCPResponse
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal MCP error response: %v", err)
	}

	if unmarshaled.Error == nil {
		t.Fatal("Expected error to be set")
	}

	if unmarshaled.Error.Code != -32600 {
		t.Errorf("Expected error code -32600, got %d", unmarshaled.Error.Code)
	}

	if unmarshaled.Error.Message != "Invalid Request" {
		t.Errorf("Expected error message 'Invalid Request', got %s", unmarshaled.Error.Message)
	}
}

func TestProxyHandlerHTTPRouting(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Command:  "echo",
				Args:     []string{"hello"},
				Protocol: "http",
				HttpPort: 8080,
			},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 for health endpoint, got %d", w.Code)
	}

	var healthResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &healthResp); err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	}

	if healthResp["status"] != "ok" {
		t.Errorf("Expected health status 'ok', got %v", healthResp["status"])
	}
}

func TestProxyHandlerCORS(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test CORS preflight
	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "POST")
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("Expected CORS header '*', got %s", w.Header().Get("Access-Control-Allow-Origin"))
	}

	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Expected Access-Control-Allow-Methods header to be set")
	}

	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("Expected Access-Control-Allow-Headers header to be set")
	}
}

func TestProxyHandlerAPIAuthentication(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{},
		ProxyAuth: config.ProxyAuthConfig{
			Enabled: true,
			APIKey:  "secret-key",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "secret-key", true)

	// Test request without API key
	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated request, got %d", w.Code)
	}

	// Test request with valid API key
	req = httptest.NewRequest("GET", "/api/servers", nil)
	req.Header.Set("Authorization", "Bearer secret-key")
	w = httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	// Should not be 401 (may be 404 or other, but not unauthorized)
	if w.Code == http.StatusUnauthorized {
		t.Errorf("Expected authenticated request to pass auth, got status %d", w.Code)
	}
}

func TestConnectionStats(t *testing.T) {
	stats := &ConnectionStats{
		ConnectedAt:    time.Now(),
		LastActivity:   time.Now(),
		RequestCount:   10,
		ResponseCount:  9,
		ErrorCount:     1,
		BytesSent:      1024,
		BytesReceived:  2048,
	}

	if stats.RequestCount != 10 {
		t.Errorf("Expected request count 10, got %d", stats.RequestCount)
	}

	if stats.ResponseCount != 9 {
		t.Errorf("Expected response count 9, got %d", stats.ResponseCount)
	}

	if stats.ErrorCount != 1 {
		t.Errorf("Expected error count 1, got %d", stats.ErrorCount)
	}

	if stats.BytesSent != 1024 {
		t.Errorf("Expected bytes sent 1024, got %d", stats.BytesSent)
	}

	if stats.BytesReceived != 2048 {
		t.Errorf("Expected bytes received 2048, got %d", stats.BytesReceived)
	}
}

func TestProxyHandlerJSONRPCValidation(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test invalid JSON-RPC request
	invalidJSON := `{"invalid": "request"}`
	req := httptest.NewRequest("POST", "/test-server", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	// Should return JSON-RPC error response
	var response MCPResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
	}

	if response.Error == nil {
		t.Error("Expected error response for invalid JSON-RPC")
	}
}

func TestProxyHandlerServerRouting(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"weather-server": {
				Command:  "weather-tool",
				Protocol: "stdio",
			},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test routing to weather-server
	mcpRequest := MCPRequest{
		JSONRPC: "2.0",
		ID:      "test-id",
		Method:  "tools/list",
	}

	reqBody, _ := json.Marshal(mcpRequest)
	req := httptest.NewRequest("POST", "/weather-server", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	proxyHandler.ServeHTTP(w, req)

	// Verify request was routed (exact response depends on server implementation)
	if w.Code == http.StatusNotFound {
		t.Error("Request should be routed to weather-server, not return 404")
	}
}

func TestProxyHandlerConcurrentRequests(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Command:  "echo",
				Protocol: "stdio",
			},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test concurrent requests
	var wg sync.WaitGroup
	numRequests := 10
	errors := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			mcpRequest := MCPRequest{
				JSONRPC: "2.0",
				ID:      fmt.Sprintf("test-id-%d", id),
				Method:  "tools/list",
			}

			reqBody, _ := json.Marshal(mcpRequest)
			req := httptest.NewRequest("POST", "/test-server", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			proxyHandler.ServeHTTP(w, req)

			// Basic validation that request was processed
			if w.Code == 0 {
				errors <- fmt.Errorf("request %d got zero status code", id)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent request error: %v", err)
	}
}

func TestGetServerNameFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/weather-server", "weather-server"},
		{"/api/servers", ""},
		{"/health", ""},
		{"/weather-server/nested", "weather-server"},
		{"/", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := getServerNameFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("getServerNameFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetClientIP(t *testing.T) {
	// Test with X-Forwarded-For header
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Forwarded-For", "192.168.1.100")
	req.RemoteAddr = "10.0.0.1:12345"

	ip := getClientIP(req)
	if ip != "192.168.1.100" {
		t.Errorf("Expected IP from X-Forwarded-For '192.168.1.100', got %s", ip)
	}

	// Test with X-Real-IP header
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Real-IP", "203.0.113.1")
	req.RemoteAddr = "10.0.0.1:12345"

	ip = getClientIP(req)
	if ip != "203.0.113.1" {
		t.Errorf("Expected IP from X-Real-IP '203.0.113.1', got %s", ip)
	}

	// Test with RemoteAddr fallback
	req = httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "172.16.0.1:54321"

	ip = getClientIP(req)
	if ip != "172.16.0.1" {
		t.Errorf("Expected IP from RemoteAddr '172.16.0.1', got %s", ip)
	}
}

func TestProxyHandlerShutdown(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	proxyHandler.ctx = ctx
	proxyHandler.cancel = cancel

	// Simulate shutdown
	cancel()

	// Verify context cancellation
	select {
	case <-proxyHandler.ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected context to be cancelled")
	}
}

func TestConnectionPooling(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"http-server": {
				Command:  "http-tool",
				Protocol: "http",
				HttpPort: 8080,
			},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Verify connection maps are initialized
	if proxyHandler.ServerConnections == nil {
		t.Error("Expected ServerConnections to be initialized")
	}

	if proxyHandler.SSEConnections == nil {
		t.Error("Expected SSEConnections to be initialized")
	}

	if proxyHandler.StdioConnections == nil {
		t.Error("Expected StdioConnections to be initialized")
	}

	// Test connection creation and pooling
	serverName := "http-server"

	// Simulate connection creation
	proxyHandler.ConnectionMutex.Lock()
	proxyHandler.ServerConnections[serverName] = &MCPHTTPConnection{
		ServerName: serverName,
		BaseURL:    "http://localhost:8080",
		Client:     &http.Client{},
	}
	proxyHandler.ConnectionMutex.Unlock()

	// Verify connection exists
	proxyHandler.ConnectionMutex.RLock()
	conn, exists := proxyHandler.ServerConnections[serverName]
	proxyHandler.ConnectionMutex.RUnlock()

	if !exists {
		t.Error("Expected connection to exist in pool")
	}

	if conn.ServerName != serverName {
		t.Errorf("Expected connection server name %s, got %s", serverName, conn.ServerName)
	}
}

func TestRequestIDGeneration(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)
	proxyHandler := NewProxyHandler(manager, "config.yaml", "test-api-key", true)

	// Test thread-safe request ID generation
	var wg sync.WaitGroup
	ids := make(chan int, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			proxyHandler.GlobalIDMutex.Lock()
			proxyHandler.GlobalRequestID++
			id := proxyHandler.GlobalRequestID
			proxyHandler.GlobalIDMutex.Unlock()
			ids <- id
		}()
	}

	wg.Wait()
	close(ids)

	// Verify all IDs are unique
	seen := make(map[int]bool)
	count := 0
	for id := range ids {
		if seen[id] {
			t.Errorf("Duplicate request ID: %d", id)
		}
		seen[id] = true
		count++
	}

	if count != 100 {
		t.Errorf("Expected 100 unique IDs, got %d", count)
	}
}