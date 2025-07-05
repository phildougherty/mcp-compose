package dashboard

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

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"

	"github.com/gorilla/websocket"
)

// MockRuntime for testing dashboard manager
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
		fmt.Printf("Mock logs for %s\n", name)
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

func (m *MockRuntime) DeleteNetwork(name string) error {
	delete(m.networks, name)
	return nil
}

func (m *MockRuntime) RemoveNetwork(name string) error {
	delete(m.networks, name)
	return nil
}

func (m *MockRuntime) ListNetworks() ([]container.NetworkInfo, error) {
	var networks []container.NetworkInfo
	for name := range m.networks {
		networks = append(networks, container.NetworkInfo{
			Name:   name,
			Driver: "bridge",
		})
	}
	return networks, nil
}

func (m *MockRuntime) GetNetworkInfo(name string) (*container.NetworkInfo, error) {
	if m.networks[name] {
		return &container.NetworkInfo{
			Name:   name,
			Driver: "bridge",
		}, nil
	}
	return nil, fmt.Errorf("network not found: %s", name)
}

// Implement remaining interface methods
func (m *MockRuntime) RestartContainer(name string) error {
	if info, exists := m.containers[name]; exists {
		info.Status = "running"
		info.State = "running"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) PauseContainer(name string) error {
	if info, exists := m.containers[name]; exists {
		info.Status = "paused"
		info.State = "paused"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) UnpauseContainer(name string) error {
	if info, exists := m.containers[name]; exists {
		info.Status = "running"
		info.State = "running"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerStats(name string) (*container.ContainerStats, error) {
	return &container.ContainerStats{}, nil
}

func (m *MockRuntime) WaitForContainer(name string, condition string) error {
	return nil
}

func (m *MockRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	return nil, nil, nil, nil
}

func (m *MockRuntime) PullImage(image string, auth *container.ImageAuth) error {
	return nil
}

func (m *MockRuntime) BuildImage(opts *container.BuildOptions) error {
	if m.shouldFailBuild {
		return fmt.Errorf("mock build failure")
	}
	return nil
}

func (m *MockRuntime) RemoveImage(image string, force bool) error {
	return nil
}

func (m *MockRuntime) ListImages() ([]container.ImageInfo, error) {
	return []container.ImageInfo{}, nil
}

func (m *MockRuntime) CreateVolume(name string, opts *container.VolumeOptions) error {
	return nil
}

func (m *MockRuntime) RemoveVolume(name string, force bool) error {
	return nil
}

func (m *MockRuntime) ListVolumes() ([]container.VolumeInfo, error) {
	return []container.VolumeInfo{}, nil
}

func (m *MockRuntime) ConnectToNetwork(containerName, networkName string) error {
	return nil
}

func (m *MockRuntime) DisconnectFromNetwork(containerName, networkName string) error {
	return nil
}

func (m *MockRuntime) UpdateContainerResources(name string, resources *container.ResourceLimits) error {
	return nil
}

func (m *MockRuntime) ValidateSecurityContext(opts *container.ContainerOptions) error {
	return nil
}

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
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    3001,
			Host:    "0.0.0.0",
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
}

func TestNewManagerWithActivityStorage(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled:     true,
			Port:        3001,
			Host:        "0.0.0.0",
			PostgresURL: "postgresql://test:test@localhost:5432/test",
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

	// Activity storage creation may fail in test environment, which is expected
	// The manager should still be created successfully
}

func TestManagerSetConfigFile(t *testing.T) {
	cfg := &config.ComposeConfig{Version: "1"}
	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	configFile := "/path/to/config.yaml"
	manager.SetConfigFile(configFile)

	if manager.configFile != configFile {
		t.Errorf("Expected config file %s, got %s", configFile, manager.configFile)
	}
}

func TestManagerStart(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    3001,
			Host:    "0.0.0.0",
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test successful start
	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start, got error: %v", err)
	}

	// Verify container was started
	containerInfo, err := runtime.GetContainerInfo("mcp-compose-dashboard")
	if err != nil {
		t.Errorf("Expected dashboard container to be started: %v", err)
	} else {
		if containerInfo.Status != "running" {
			t.Errorf("Expected dashboard container to be running, got %s", containerInfo.Status)
		}
	}

	// Verify network was created
	exists, err := runtime.NetworkExists("mcp-net")
	if err != nil {
		t.Errorf("Failed to check network existence: %v", err)
	}
	if !exists {
		t.Error("Expected mcp-net network to be created")
	}
}

func TestManagerStartDisabled(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: false,
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err == nil {
		t.Error("Expected error when starting disabled dashboard")
	}

	if !strings.Contains(err.Error(), "disabled in configuration") {
		t.Errorf("Expected error message about disabled dashboard, got: %s", err.Error())
	}
}

func TestManagerStop(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start container first
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-dashboard",
		Image: "mcp-compose-dashboard:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test stop
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Expected successful stop, got error: %v", err)
	}

	// Verify container was stopped
	status, _ := runtime.GetContainerStatus("mcp-compose-dashboard")
	if status != "stopped" {
		t.Errorf("Expected dashboard container to be stopped, got %s", status)
	}
}

func TestManagerRestart(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start container first
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-dashboard",
		Image: "mcp-compose-dashboard:latest",
	}
	runtime.StartContainer(containerOpts)

	err := manager.Restart()
	if err != nil {
		t.Fatalf("Expected successful restart, got error: %v", err)
	}

	// Verify container is running after restart
	status, err := runtime.GetContainerStatus("mcp-compose-dashboard")
	if err != nil {
		t.Errorf("Failed to get dashboard container status: %v", err)
	}
	if status != "running" {
		t.Errorf("Expected dashboard container to be running after restart, got %s", status)
	}
}

func TestManagerStatus(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test status with non-existent container
	status, err := manager.Status()
	if err == nil {
		t.Error("Expected error for non-existent container")
	}

	// Start container
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-dashboard",
		Image: "mcp-compose-dashboard:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test status with running container
	status, err = manager.Status()
	if err != nil {
		t.Errorf("Expected no error getting status, got: %v", err)
	}
	if status != "running" {
		t.Errorf("Expected status 'running', got %s", status)
	}
}

func TestNewDashboardServer(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    3001,
			Host:    "0.0.0.0",
			Theme:   "dark",
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
	}

	runtime := NewMockRuntime()
	proxyURL := "http://localhost:3000"
	apiKey := "test-api-key"

	server := NewDashboardServer(cfg, runtime, proxyURL, apiKey)

	if server == nil {
		t.Fatal("Expected dashboard server to be created")
	}

	if server.config != cfg {
		t.Error("Expected config to be set")
	}

	if server.runtime != runtime {
		t.Error("Expected runtime to be set")
	}

	if server.proxyURL != proxyURL {
		t.Errorf("Expected proxy URL %s, got %s", proxyURL, server.proxyURL)
	}

	if server.apiKey != apiKey {
		t.Errorf("Expected API key %s, got %s", apiKey, server.apiKey)
	}

	if server.logger == nil {
		t.Error("Expected logger to be initialized")
	}

	if server.httpClient == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if server.inspectorService == nil {
		t.Error("Expected inspector service to be initialized")
	}
}

func TestDashboardServerHTTPHandlers(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    3001,
			Host:    "0.0.0.0",
		},
	}

	runtime := NewMockRuntime()
	server := NewDashboardServer(cfg, runtime, "http://localhost:3000", "test-key")

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var healthResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &healthResp); err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	}

	if healthResp["status"] != "ok" {
		t.Errorf("Expected health status 'ok', got %v", healthResp["status"])
	}
}

func TestDashboardServerAPIEndpoints(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
		},
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Command: "echo",
				Args:    []string{"hello"},
			},
		},
	}

	runtime := NewMockRuntime()
	server := NewDashboardServer(cfg, runtime, "http://localhost:3000", "test-key")

	// Test servers endpoint
	req := httptest.NewRequest("GET", "/api/servers", nil)
	w := httptest.NewRecorder()

	server.handleServers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var serversResp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &serversResp); err != nil {
		t.Errorf("Failed to parse servers response: %v", err)
	}

	if serversResp["servers"] == nil {
		t.Error("Expected servers field in response")
	}
}

func TestActivityMessage(t *testing.T) {
	activity := ActivityMessage{
		ID:        "test-id",
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     "info",
		Type:      "request",
		Server:    "test-server",
		Client:    "test-client",
		Message:   "Test activity message",
		Details: map[string]interface{}{
			"method": "GET",
			"path":   "/api/test",
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(activity)
	if err != nil {
		t.Errorf("Failed to marshal activity message: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled ActivityMessage
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal activity message: %v", err)
	}

	if unmarshaled.ID != activity.ID {
		t.Errorf("Expected ID %s, got %s", activity.ID, unmarshaled.ID)
	}

	if unmarshaled.Message != activity.Message {
		t.Errorf("Expected message %s, got %s", activity.Message, unmarshaled.Message)
	}
}

func TestLogMessage(t *testing.T) {
	logMsg := LogMessage{
		Timestamp: time.Now().Format(time.RFC3339),
		Server:    "test-server",
		Level:     "info",
		Message:   "Test log message",
	}

	// Test JSON marshaling
	data, err := json.Marshal(logMsg)
	if err != nil {
		t.Errorf("Failed to marshal log message: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled LogMessage
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal log message: %v", err)
	}

	if unmarshaled.Server != logMsg.Server {
		t.Errorf("Expected server %s, got %s", logMsg.Server, unmarshaled.Server)
	}

	if unmarshaled.Message != logMsg.Message {
		t.Errorf("Expected message %s, got %s", logMsg.Message, unmarshaled.Message)
	}
}

func TestMetricsMessage(t *testing.T) {
	metricsMsg := MetricsMessage{
		Timestamp: time.Now().Format(time.RFC3339),
		Status: map[string]interface{}{
			"proxy":     "running",
			"dashboard": "running",
		},
		Connections: map[string]interface{}{
			"active": 5,
			"total":  100,
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(metricsMsg)
	if err != nil {
		t.Errorf("Failed to marshal metrics message: %v", err)
	}

	// Test JSON unmarshaling
	var unmarshaled MetricsMessage
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Errorf("Failed to unmarshal metrics message: %v", err)
	}

	if unmarshaled.Status["proxy"] != "running" {
		t.Errorf("Expected proxy status 'running', got %v", unmarshaled.Status["proxy"])
	}
}

func TestSafeWebSocketConn(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer conn.Close()

		// Echo messages back
		for {
			messageType, p, err := conn.ReadMessage()
			if err != nil {
				break
			}
			if err := conn.WriteMessage(messageType, p); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Test connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()

	safeConn := &SafeWebSocketConn{conn: conn}

	// Test safe writing
	testMessage := "test message"
	err = safeConn.WriteJSON(map[string]string{"message": testMessage})
	if err != nil {
		t.Errorf("Failed to write JSON message: %v", err)
	}

	// Test reading response
	var response map[string]string
	err = conn.ReadJSON(&response)
	if err != nil {
		t.Errorf("Failed to read JSON response: %v", err)
	}

	if response["message"] != testMessage {
		t.Errorf("Expected message %s, got %s", testMessage, response["message"])
	}
}

func TestSafeWebSocketConcurrentWrites(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read and discard messages
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect to WebSocket server: %v", err)
	}
	defer conn.Close()

	safeConn := &SafeWebSocketConn{conn: conn}

	// Test concurrent writes
	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := safeConn.WriteJSON(map[string]interface{}{
				"id":      id,
				"message": fmt.Sprintf("concurrent message %d", id),
			})
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent write error: %v", err)
	}
}

func TestPageData(t *testing.T) {
	pageData := PageData{
		Title:    "MCP Dashboard",
		ProxyURL: "http://localhost:3000",
		APIKey:   "test-api-key",
		Theme:    "dark",
		Port:     3001,
	}

	if pageData.Title != "MCP Dashboard" {
		t.Errorf("Expected title 'MCP Dashboard', got %s", pageData.Title)
	}

	if pageData.ProxyURL != "http://localhost:3000" {
		t.Errorf("Expected proxy URL 'http://localhost:3000', got %s", pageData.ProxyURL)
	}

	if pageData.APIKey != "test-api-key" {
		t.Errorf("Expected API key 'test-api-key', got %s", pageData.APIKey)
	}

	if pageData.Theme != "dark" {
		t.Errorf("Expected theme 'dark', got %s", pageData.Theme)
	}

	if pageData.Port != 3001 {
		t.Errorf("Expected port 3001, got %d", pageData.Port)
	}
}

func TestManagerBuildEnvironment(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled:           true,
			Port:              9090,
			Host:              "127.0.0.1",
			Theme:             "light",
			PostgresURL:       "postgresql://test:test@localhost:5432/test",
			ActivityRetention: "7d",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	env := manager.buildEnvironment()

	expectedVars := map[string]string{
		"MCP_DASHBOARD_PORT":              "9090",
		"MCP_DASHBOARD_HOST":              "127.0.0.1",
		"MCP_DASHBOARD_THEME":             "light",
		"MCP_DASHBOARD_POSTGRES_URL":      "postgresql://test:test@localhost:5432/test",
		"MCP_DASHBOARD_ACTIVITY_RETENTION": "7d",
		"MCP_DASHBOARD_ENABLED":           "true",
	}

	for key, expectedValue := range expectedVars {
		if actualValue, exists := env[key]; !exists {
			t.Errorf("Expected environment variable %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
		}
	}
}

func TestManagerIsRunning(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Dashboard: config.DashboardConfig{
			Enabled: true,
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test IsRunning with non-existent container
	if manager.IsRunning() {
		t.Error("Expected IsRunning to return false for non-existent container")
	}

	// Start container
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-dashboard",
		Image: "mcp-compose-dashboard:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test IsRunning with running container
	if !manager.IsRunning() {
		t.Error("Expected IsRunning to return true for running container")
	}

	// Stop container
	runtime.StopContainer("mcp-compose-dashboard")

	// Test IsRunning with stopped container
	if manager.IsRunning() {
		t.Error("Expected IsRunning to return false for stopped container")
	}
}