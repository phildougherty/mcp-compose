package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"mcpcompose/internal/compose"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/dashboard"
	"mcpcompose/internal/memory"
	"mcpcompose/internal/server"
	"mcpcompose/internal/task_scheduler"
)

// MockRuntime for integration testing
type MockRuntime struct {
	containers      map[string]*container.ContainerInfo
	networks        map[string]bool
	startErrors     map[string]error
	stopErrors      map[string]error
	shouldFailBuild bool
	buildCommands   [][]string
	runtimeName     string
	mu              sync.RWMutex
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
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.mu.Lock()
	defer m.mu.Unlock()

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
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.containers[name]; exists {
		return info.Status, nil
	}
	return "", fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerInfo(name string) (*container.ContainerInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.containers[name]; exists {
		return info, nil
	}
	return nil, fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) ListContainers(filters map[string]string) ([]container.ContainerInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var containers []container.ContainerInfo
	for _, info := range m.containers {
		containers = append(containers, *info)
	}
	return containers, nil
}

func (m *MockRuntime) ShowContainerLogs(name string, follow bool) error {
	return nil
}

func (m *MockRuntime) NetworkExists(name string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.networks[name], nil
}

func (m *MockRuntime) CreateNetwork(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.networks[name] = true
	return nil
}

func (m *MockRuntime) DeleteNetwork(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networks, name)
	return nil
}

func (m *MockRuntime) RemoveNetwork(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.networks, name)
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

// Integration test helper
type IntegrationTestSuite struct {
	runtime         *MockRuntime
	config          *config.ComposeConfig
	composer        *compose.Composer
	proxyHandler    *server.ProxyHandler
	dashboardServer *dashboard.DashboardServer
	memoryManager   *memory.Manager
	taskManager     *task_scheduler.Manager
	tempDir         string
}

func setupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	tempDir, err := os.MkdirTemp("", "mcp-compose-integration-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create a comprehensive test configuration
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"weather-server": {
				Command:  "weather-tool",
				Args:     []string{"--port", "8080"},
				Protocol: "stdio",
				Env: map[string]string{
					"API_KEY": "test-key",
				},
			},
			"calculator-server": {
				Image:    "calculator:latest",
				Protocol: "http",
				HttpPort: 8081,
				Networks: []string{"mcp-net"},
			},
		},
		Networks: map[string]config.NetworkConfig{
			"mcp-net": {
				Driver: "bridge",
			},
		},
		Dashboard: config.DashboardConfig{
			Enabled: true,
			Port:    3001,
			Host:    "0.0.0.0",
		},
		Memory: config.MemoryConfig{
			DatabaseURL: "postgresql://test:test@localhost:5432/memory_test",
		},
		TaskScheduler: &config.TaskScheduler{
			Enabled: true,
			Port:    8080,
			Host:    "0.0.0.0",
		},
		Logging: config.LoggingConfig{
			Level: "info",
		},
		ProxyAuth: config.ProxyAuthConfig{
			Enabled: true,
			APIKey:  "test-api-key",
		},
	}

	runtime := NewMockRuntime()

	// Initialize all components
	composer, err := compose.NewComposer(cfg, runtime)
	if err != nil {
		t.Fatalf("Failed to create composer: %v", err)
	}

	serverManager := server.NewManager(cfg, runtime)
	proxyHandler := server.NewProxyHandler(serverManager, filepath.Join(tempDir, "config.yaml"), "test-api-key", true)

	dashboardServer := dashboard.NewDashboardServer(cfg, runtime, "http://localhost:3000", "test-api-key")
	memoryManager := memory.NewManager(cfg, runtime)
	taskManager := task_scheduler.NewManager(cfg, runtime)

	return &IntegrationTestSuite{
		runtime:         runtime,
		config:          cfg,
		composer:        composer,
		proxyHandler:    proxyHandler,
		dashboardServer: dashboardServer,
		memoryManager:   memoryManager,
		taskManager:     taskManager,
		tempDir:         tempDir,
	}
}

func (suite *IntegrationTestSuite) cleanup(t *testing.T) {
	if err := os.RemoveAll(suite.tempDir); err != nil {
		t.Logf("Failed to cleanup temp dir: %v", err)
	}

	// Shutdown components
	if suite.composer != nil {
		suite.composer.Down()
	}
}

func TestFullStackIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	t.Run("component_initialization", func(t *testing.T) {
		// Test that all components are properly initialized
		if suite.composer == nil {
			t.Error("Composer should be initialized")
		}

		if suite.proxyHandler == nil {
			t.Error("Proxy handler should be initialized")
		}

		if suite.dashboardServer == nil {
			t.Error("Dashboard server should be initialized")
		}

		if suite.memoryManager == nil {
			t.Error("Memory manager should be initialized")
		}

		if suite.taskManager == nil {
			t.Error("Task manager should be initialized")
		}
	})

	t.Run("network_creation", func(t *testing.T) {
		// Test network creation across components
		err := suite.composer.Up([]string{})
		if err != nil {
			t.Fatalf("Failed to start services: %v", err)
		}

		// Verify mcp-net was created
		exists, err := suite.runtime.NetworkExists("mcp-net")
		if err != nil {
			t.Errorf("Failed to check network: %v", err)
		}
		if !exists {
			t.Error("Expected mcp-net to be created")
		}
	})

	t.Run("service_dependencies", func(t *testing.T) {
		// Test that services start in proper order with dependencies
		containers, err := suite.runtime.ListContainers(nil)
		if err != nil {
			t.Fatalf("Failed to list containers: %v", err)
		}

		// Verify expected containers are running
		expectedContainers := []string{
			"mcp-compose-weather-server",
			"mcp-compose-calculator-server",
		}

		runningContainers := make(map[string]bool)
		for _, container := range containers {
			if container.Status == "running" {
				runningContainers[container.Name] = true
			}
		}

		for _, expected := range expectedContainers {
			if !runningContainers[expected] {
				t.Errorf("Expected container %s to be running", expected)
			}
		}
	})
}

func TestProxyServerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	// Start services
	err := suite.composer.Up([]string{})
	if err != nil {
		t.Fatalf("Failed to start services: %v", err)
	}

	t.Run("proxy_routing", func(t *testing.T) {
		// Test that proxy correctly routes requests to servers
		request := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      "test-id",
			"method":  "tools/list",
		}

		reqBody, _ := json.Marshal(request)
		req := httptest.NewRequest("POST", "/weather-server", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-api-key")

		w := httptest.NewRecorder()
		suite.proxyHandler.ServeHTTP(w, req)

		// Verify request was processed (may not succeed due to mock, but should be routed)
		if w.Code == http.StatusNotFound {
			t.Error("Request should be routed to weather-server")
		}
	})

	t.Run("api_endpoints", func(t *testing.T) {
		// Test API endpoints work correctly
		req := httptest.NewRequest("GET", "/api/servers", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")

		w := httptest.NewRecorder()
		suite.proxyHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for /api/servers, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse servers response: %v", err)
		}

		if response["servers"] == nil {
			t.Error("Expected servers in response")
		}
	})

	t.Run("health_check", func(t *testing.T) {
		// Test health endpoint
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		suite.proxyHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for health check, got %d", w.Code)
		}

		var health map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &health); err != nil {
			t.Errorf("Failed to parse health response: %v", err)
		}

		if health["status"] != "ok" {
			t.Errorf("Expected health status 'ok', got %v", health["status"])
		}
	})
}

func TestDashboardIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	// Start dashboard
	dashboardManager := dashboard.NewManager(suite.config, suite.runtime)
	err := dashboardManager.Start()
	if err != nil {
		t.Fatalf("Failed to start dashboard: %v", err)
	}

	t.Run("dashboard_health", func(t *testing.T) {
		// Test dashboard health endpoint
		req := httptest.NewRequest("GET", "/health", nil)
		w := httptest.NewRecorder()

		suite.dashboardServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for dashboard health, got %d", w.Code)
		}
	})

	t.Run("dashboard_api", func(t *testing.T) {
		// Test dashboard API endpoints
		req := httptest.NewRequest("GET", "/api/servers", nil)
		w := httptest.NewRecorder()

		suite.dashboardServer.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for dashboard servers API, got %d", w.Code)
		}
	})
}

func TestMemoryServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	t.Run("memory_manager_lifecycle", func(t *testing.T) {
		// Test memory manager start/stop
		err := suite.memoryManager.Start()
		if err != nil {
			t.Fatalf("Failed to start memory manager: %v", err)
		}

		// Verify memory containers are running
		memoryStatus, err := suite.memoryManager.Status()
		if err != nil {
			t.Errorf("Failed to get memory status: %v", err)
		}

		if memoryStatus != "running" {
			t.Errorf("Expected memory status 'running', got %s", memoryStatus)
		}

		// Test stop
		err = suite.memoryManager.Stop()
		if err != nil {
			t.Errorf("Failed to stop memory manager: %v", err)
		}
	})

	t.Run("memory_postgres_integration", func(t *testing.T) {
		// Test PostgreSQL container management
		err := suite.memoryManager.Start()
		if err != nil {
			t.Fatalf("Failed to start memory manager: %v", err)
		}

		// Verify PostgreSQL container is created
		containers, err := suite.runtime.ListContainers(nil)
		if err != nil {
			t.Fatalf("Failed to list containers: %v", err)
		}

		postgresFound := false
		memoryFound := false
		for _, container := range containers {
			if strings.Contains(container.Name, "postgres-memory") {
				postgresFound = true
			}
			if strings.Contains(container.Name, "memory") && container.Status == "running" {
				memoryFound = true
			}
		}

		if !postgresFound {
			t.Error("Expected PostgreSQL container to be created")
		}

		if !memoryFound {
			t.Error("Expected memory container to be running")
		}
	})
}

func TestTaskSchedulerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	t.Run("task_scheduler_lifecycle", func(t *testing.T) {
		// Test task scheduler start/stop
		err := suite.taskManager.Start()
		if err != nil {
			t.Fatalf("Failed to start task scheduler: %v", err)
		}

		// Verify task scheduler is running
		if !suite.taskManager.IsRunning() {
			t.Error("Expected task scheduler to be running")
		}

		status, err := suite.taskManager.Status()
		if err != nil {
			t.Errorf("Failed to get task scheduler status: %v", err)
		}

		if status != "running" {
			t.Errorf("Expected task scheduler status 'running', got %s", status)
		}

		// Test restart
		err = suite.taskManager.Restart()
		if err != nil {
			t.Errorf("Failed to restart task scheduler: %v", err)
		}

		// Verify still running after restart
		if !suite.taskManager.IsRunning() {
			t.Error("Expected task scheduler to be running after restart")
		}
	})
}

func TestServiceDiscoveryIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	// Start all services
	err := suite.composer.Up([]string{})
	if err != nil {
		t.Fatalf("Failed to start services: %v", err)
	}

	t.Run("service_discovery", func(t *testing.T) {
		// Test that proxy can discover all running servers
		req := httptest.NewRequest("GET", "/api/servers", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")

		w := httptest.NewRecorder()
		suite.proxyHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for service discovery, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse discovery response: %v", err)
		}

		servers, ok := response["servers"].(map[string]interface{})
		if !ok {
			t.Error("Expected servers map in response")
			return
		}

		// Verify expected servers are discovered
		expectedServers := []string{"weather-server", "calculator-server"}
		for _, expected := range expectedServers {
			if servers[expected] == nil {
				t.Errorf("Expected server %s to be discovered", expected)
			}
		}
	})
}

func TestConcurrentOperationsIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	// Start services
	err := suite.composer.Up([]string{})
	if err != nil {
		t.Fatalf("Failed to start services: %v", err)
	}

	t.Run("concurrent_requests", func(t *testing.T) {
		// Test concurrent proxy requests
		var wg sync.WaitGroup
		numRequests := 20
		errors := make(chan error, numRequests)

		for i := 0; i < numRequests; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				request := map[string]interface{}{
					"jsonrpc": "2.0",
					"id":      fmt.Sprintf("test-id-%d", id),
					"method":  "tools/list",
				}

				reqBody, _ := json.Marshal(request)
				req := httptest.NewRequest("POST", "/weather-server", bytes.NewReader(reqBody))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-api-key")

				w := httptest.NewRecorder()
				suite.proxyHandler.ServeHTTP(w, req)

				// Basic validation
				if w.Code == 0 {
					errors <- fmt.Errorf("request %d got zero status code", id)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent request error: %v", err)
		}
	})

	t.Run("concurrent_service_operations", func(t *testing.T) {
		// Test concurrent service start/stop operations
		var wg sync.WaitGroup
		operations := []func() error{
			func() error { return suite.memoryManager.Start() },
			func() error { return suite.taskManager.Start() },
			func() error { _, err := suite.memoryManager.Status(); return err },
			func() error { _, err := suite.taskManager.Status(); return err },
		}

		errors := make(chan error, len(operations))

		for _, op := range operations {
			wg.Add(1)
			go func(operation func() error) {
				defer wg.Done()
				if err := operation(); err != nil {
					errors <- err
				}
			}(op)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent operation error: %v", err)
		}
	})
}

func TestConfigurationReloadIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	t.Run("config_reload", func(t *testing.T) {
		// Start with initial configuration
		err := suite.composer.Up([]string{})
		if err != nil {
			t.Fatalf("Failed to start with initial config: %v", err)
		}

		// Verify initial state
		containers, err := suite.runtime.ListContainers(nil)
		if err != nil {
			t.Fatalf("Failed to list containers: %v", err)
		}

		initialCount := len(containers)

		// Simulate configuration change by adding a new server
		suite.config.Servers["new-server"] = config.ServerConfig{
			Command:  "new-tool",
			Protocol: "stdio",
		}

		// Create new composer with updated config
		newComposer, err := compose.NewComposer(suite.config, suite.runtime)
		if err != nil {
			t.Fatalf("Failed to create new composer: %v", err)
		}

		// Start new configuration
		err = newComposer.Up([]string{})
		if err != nil {
			t.Fatalf("Failed to start with new config: %v", err)
		}

		// Verify new server was added
		containers, err = suite.runtime.ListContainers(nil)
		if err != nil {
			t.Fatalf("Failed to list containers after reload: %v", err)
		}

		newCount := len(containers)
		if newCount <= initialCount {
			t.Errorf("Expected more containers after config reload, got %d vs %d", newCount, initialCount)
		}

		// Verify the new server exists
		found := false
		for _, container := range containers {
			if strings.Contains(container.Name, "new-server") {
				found = true
				break
			}
		}

		if !found {
			t.Error("Expected new-server to be created after config reload")
		}
	})
}

func TestErrorHandlingIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	suite := setupIntegrationTest(t)
	defer suite.cleanup(t)

	t.Run("container_failure_recovery", func(t *testing.T) {
		// Simulate container start failure
		suite.runtime.startErrors["mcp-compose-weather-server"] = fmt.Errorf("container start failure")

		// Attempt to start services
		err := suite.composer.Up([]string{})
		if err == nil {
			t.Error("Expected error when container fails to start")
		}

		// Remove the failure condition
		delete(suite.runtime.startErrors, "mcp-compose-weather-server")

		// Retry should succeed
		err = suite.composer.Up([]string{})
		if err != nil {
			t.Errorf("Expected success after removing failure condition: %v", err)
		}
	})

	t.Run("network_isolation", func(t *testing.T) {
		// Test network isolation between services
		err := suite.composer.Up([]string{})
		if err != nil {
			t.Fatalf("Failed to start services: %v", err)
		}

		// Verify mcp-net exists
		exists, err := suite.runtime.NetworkExists("mcp-net")
		if err != nil {
			t.Errorf("Failed to check network: %v", err)
		}

		if !exists {
			t.Error("Expected mcp-net to exist for service isolation")
		}

		// Verify containers are connected to the network
		containers, err := suite.runtime.ListContainers(nil)
		if err != nil {
			t.Fatalf("Failed to list containers: %v", err)
		}

		// Count containers that should be on mcp-net
		networkContainers := 0
		for _, container := range containers {
			if strings.Contains(container.Name, "calculator-server") {
				networkContainers++
			}
		}

		if networkContainers == 0 {
			t.Error("Expected containers to be connected to mcp-net")
		}
	})
}