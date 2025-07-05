package task_scheduler

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
)

// MockRuntime for testing task scheduler manager
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

// Implement remaining interface methods with minimal implementations
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
func (m *MockRuntime) BuildImage(opts *container.BuildOptions) error { return nil }
func (m *MockRuntime) RemoveImage(image string, force bool) error     { return nil }
func (m *MockRuntime) ListImages() ([]container.ImageInfo, error)     { return []container.ImageInfo{}, nil }
func (m *MockRuntime) CreateVolume(name string, opts *container.VolumeOptions) error { return nil }
func (m *MockRuntime) RemoveVolume(name string, force bool) error                    { return nil }
func (m *MockRuntime) ListVolumes() ([]container.VolumeInfo, error)                 { return []container.VolumeInfo{}, nil }
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
		TaskScheduler: &config.TaskScheduler{
			Port:              8080,
			Host:              "0.0.0.0",
			LogLevel:          "info",
			CPUs:              "2.0",
			Memory:            "1g",
			OpenRouterAPIKey:  "test-key",
			OpenRouterModel:   "anthropic/claude-3-sonnet:beta",
			OllamaURL:         "http://localhost:11434",
			OllamaModel:       "llama2",
			MCPProxyURL:       "http://localhost:3001",
			MCPProxyAPIKey:    "proxy-key",
			OpenWebUIEnabled:  true,
			DatabasePath:      "/data/scheduler.db",
			Workspace:         "/workspace",
			Volumes:           []string{"/host/path:/container/path"},
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
			},
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
}

func TestSetConfigFile(t *testing.T) {
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
		TaskScheduler: &config.TaskScheduler{
			Port:              8080,
			Host:              "0.0.0.0",
			LogLevel:          "info",
			CPUs:              "2.0",
			Memory:            "1g",
			OpenRouterAPIKey:  "test-key",
			OpenRouterModel:   "anthropic/claude-3-sonnet:beta",
			OllamaURL:         "http://localhost:11434",
			OllamaModel:       "llama2",
			MCPProxyURL:       "http://localhost:3001",
			MCPProxyAPIKey:    "proxy-key",
			OpenWebUIEnabled:  true,
			DatabasePath:      "/data/scheduler.db",
			Workspace:         "/workspace",
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
			},
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
	containerInfo, err := runtime.GetContainerInfo("mcp-compose-task-scheduler")
	if err != nil {
		t.Errorf("Expected task scheduler container to be started: %v", err)
	} else {
		if containerInfo.Status != "running" {
			t.Errorf("Expected task scheduler container to be running, got %s", containerInfo.Status)
		}

		// Check environment variables
		envMap := make(map[string]string)
		for _, env := range containerInfo.Env {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				envMap[parts[0]] = parts[1]
			}
		}

		expectedEnvVars := map[string]string{
			"MCP_CRON_SERVER_PORT":    "8080",
			"MCP_CRON_SERVER_ADDRESS": "0.0.0.0",
			"MCP_CRON_LOGGING_LEVEL":  "info",
			"OPENROUTER_API_KEY":      "test-key",
			"OPENROUTER_MODEL":        "anthropic/claude-3-sonnet:beta",
			"MCP_CRON_OLLAMA_BASE_URL": "http://localhost:11434",
			"MCP_CRON_OLLAMA_DEFAULT_MODEL": "llama2",
			"MCP_PROXY_URL":           "http://localhost:3001",
			"MCP_PROXY_API_KEY":       "proxy-key",
			"MCP_CRON_OPENWEBUI_ENABLED": "true",
			"CUSTOM_VAR":              "custom_value",
		}

		for key, expectedValue := range expectedEnvVars {
			if actualValue, exists := envMap[key]; !exists {
				t.Errorf("Expected environment variable %s to be set", key)
			} else if actualValue != expectedValue {
				t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
			}
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

func TestManagerStartWithDefaults(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{}, // Empty config to test defaults
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start with defaults, got error: %v", err)
	}

	// Verify defaults were applied
	if cfg.TaskScheduler.Port != 8080 {
		t.Errorf("Expected default port 8080, got %d", cfg.TaskScheduler.Port)
	}

	if cfg.TaskScheduler.Host != "0.0.0.0" {
		t.Errorf("Expected default host '0.0.0.0', got %s", cfg.TaskScheduler.Host)
	}

	if cfg.TaskScheduler.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", cfg.TaskScheduler.LogLevel)
	}

	if cfg.TaskScheduler.CPUs != "2.0" {
		t.Errorf("Expected default CPUs '2.0', got %s", cfg.TaskScheduler.CPUs)
	}

	if cfg.TaskScheduler.Memory != "1g" {
		t.Errorf("Expected default memory '1g', got %s", cfg.TaskScheduler.Memory)
	}

	// Verify container was started with defaults
	containerInfo, err := runtime.GetContainerInfo("mcp-compose-task-scheduler")
	if err != nil {
		t.Errorf("Expected task scheduler container to be started: %v", err)
	}

	if containerInfo.Status != "running" {
		t.Errorf("Expected task scheduler container to be running, got %s", containerInfo.Status)
	}
}

func TestManagerStartAlreadyRunning(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	
	// Start container first to simulate already running state
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err == nil {
		t.Error("Expected error when starting already running task scheduler")
	}

	if !strings.Contains(err.Error(), "already running") {
		t.Errorf("Expected error message to contain 'already running', got: %s", err.Error())
	}
}

func TestManagerStartWithErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*MockRuntime)
		expectErr bool
	}{
		{
			name: "container start failure",
			setupFunc: func(r *MockRuntime) {
				r.startErrors["mcp-compose-task-scheduler"] = fmt.Errorf("task scheduler start failure")
			},
			expectErr: true,
		},
		{
			name: "network creation failure",
			setupFunc: func(r *MockRuntime) {
				// No specific setup needed, network creation can fail
			},
			expectErr: false, // Network creation failure is handled gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ComposeConfig{
				Version:       "1",
				TaskScheduler: &config.TaskScheduler{},
			}

			runtime := NewMockRuntime()
			tt.setupFunc(runtime)

			manager := NewManager(cfg, runtime)

			err := manager.Start()
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestManagerStop(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start container first
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test stop
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Expected successful stop, got error: %v", err)
	}

	// Verify container was stopped
	status, _ := runtime.GetContainerStatus("mcp-compose-task-scheduler")
	if status != "stopped" {
		t.Errorf("Expected task scheduler container to be stopped, got %s", status)
	}
}

func TestManagerStopWithError(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	runtime.stopErrors["mcp-compose-task-scheduler"] = fmt.Errorf("stop failure")

	manager := NewManager(cfg, runtime)

	err := manager.Stop()
	if err == nil {
		t.Error("Expected error when stopping task scheduler fails")
	}
}

func TestManagerRestart(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start container first
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	err := manager.Restart()
	if err != nil {
		t.Fatalf("Expected successful restart, got error: %v", err)
	}

	// Verify container is running after restart
	status, err := runtime.GetContainerStatus("mcp-compose-task-scheduler")
	if err != nil {
		t.Errorf("Failed to get task scheduler container status: %v", err)
	}
	if status != "running" {
		t.Errorf("Expected task scheduler container to be running after restart, got %s", status)
	}
}

func TestManagerStatus(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test status with non-existent container
	status, err := manager.Status()
	if err != nil {
		t.Errorf("Status should not return error for non-existent container: %v", err)
	}
	if status != "stopped" {
		t.Errorf("Expected status 'stopped' for non-existent container, got %s", status)
	}

	// Start container
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
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

func TestManagerIsRunning(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test IsRunning with non-existent container
	if manager.IsRunning() {
		t.Error("Expected IsRunning to return false for non-existent container")
	}

	// Start container
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test IsRunning with running container
	if !manager.IsRunning() {
		t.Error("Expected IsRunning to return true for running container")
	}

	// Stop container
	runtime.StopContainer("mcp-compose-task-scheduler")

	// Test IsRunning with stopped container
	if manager.IsRunning() {
		t.Error("Expected IsRunning to return false for stopped container")
	}
}

func TestBuildEnvironment(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		TaskScheduler: &config.TaskScheduler{
			Port:              9090,
			Host:              "127.0.0.1",
			LogLevel:          "debug",
			DatabasePath:      "/custom/path/db.sqlite",
			OpenRouterAPIKey:  "test-openrouter-key",
			OpenRouterModel:   "anthropic/claude-3-haiku:beta",
			OllamaURL:         "http://custom-ollama:11434",
			OllamaModel:       "custom-model",
			MCPProxyURL:       "http://custom-proxy:3000",
			MCPProxyAPIKey:    "custom-proxy-key",
			OpenWebUIEnabled:  false,
			Env: map[string]string{
				"CUSTOM_ENV_VAR": "custom_value",
				"ANOTHER_VAR":    "another_value",
			},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	env := manager.buildEnvironment()

	expectedVars := map[string]string{
		"MCP_CRON_SERVER_PORT":               "9090",
		"MCP_CRON_SERVER_ADDRESS":            "127.0.0.1",
		"MCP_CRON_LOGGING_LEVEL":             "debug",
		"MCP_CRON_DATABASE_PATH":             "/custom/path/db.sqlite",
		"OPENROUTER_API_KEY":                 "test-openrouter-key",
		"OPENROUTER_MODEL":                   "anthropic/claude-3-haiku:beta",
		"USE_OPENROUTER":                     "true",
		"OPENROUTER_ENABLED":                 "true",
		"MCP_CRON_OLLAMA_BASE_URL":           "http://custom-ollama:11434",
		"MCP_CRON_OLLAMA_DEFAULT_MODEL":      "custom-model",
		"MCP_CRON_OLLAMA_ENABLED":            "true",
		"MCP_PROXY_URL":                      "http://custom-proxy:3000",
		"MCP_PROXY_API_KEY":                  "custom-proxy-key",
		"MCP_CRON_OPENWEBUI_ENABLED":         "false",
		"MCP_CRON_ACTIVITY_WEBHOOK":          "http://mcp-compose-dashboard:3001/api/activity",
		"MCP_CRON_DATABASE_ENABLED":          "true",
		"MCP_CRON_SCHEDULER_DEFAULT_TIMEOUT": "10m",
		"TZ":                                 "America/New_York",
		"MCP_CRON_SERVER_TRANSPORT":          "sse",
		"CUSTOM_ENV_VAR":                     "custom_value",
		"ANOTHER_VAR":                        "another_value",
	}

	for key, expectedValue := range expectedVars {
		if actualValue, exists := env[key]; !exists {
			t.Errorf("Expected environment variable %s to be set", key)
		} else if actualValue != expectedValue {
			t.Errorf("Expected %s=%s, got %s=%s", key, expectedValue, key, actualValue)
		}
	}
}

func TestWaitForHealthy(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test timeout when container is not running
	err := manager.waitForHealthy(100 * time.Millisecond)
	if err == nil {
		t.Error("Expected timeout error when container is not running")
	}

	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error message, got: %s", err.Error())
	}

	// Start container and test successful health check
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	err = manager.waitForHealthy(100 * time.Millisecond)
	if err != nil {
		t.Errorf("Expected no error when container is running, got: %v", err)
	}
}

func TestGetLogs(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version:       "1",
		TaskScheduler: &config.TaskScheduler{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test logs for non-existent container
	err := manager.GetLogs(false)
	if err == nil {
		t.Error("Expected error when getting logs for non-existent container")
	}

	// Start container
	containerOpts := &container.ContainerOptions{
		Name:  "mcp-compose-task-scheduler",
		Image: "mcp-compose-task-scheduler:latest",
	}
	runtime.StartContainer(containerOpts)

	// Test logs for running container
	err = manager.GetLogs(false)
	if err != nil {
		t.Errorf("Expected no error getting logs for running container, got: %v", err)
	}

	// Test logs with follow
	err = manager.GetLogs(true)
	if err != nil {
		t.Errorf("Expected no error getting logs with follow, got: %v", err)
	}
}