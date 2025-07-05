package memory

import (
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
)

// MockRuntime for testing memory manager
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
		Memory: config.MemoryConfig{
			DatabaseURL: "postgresql://test:test@localhost:5432/test",
			CPUs:        "2.0",
			Memory:      "2g",
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	if manager == nil {
		t.Fatal("Expected manager to be created")
	}

	if manager.cfg != cfg {
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
		Memory: config.MemoryConfig{
			DatabaseURL:      "postgresql://test:test@localhost:5432/test",
			CPUs:             "1.0",
			Memory:           "1g",
			PostgresCPUs:     "2.0",
			PostgresMemory:   "2g",
			PostgresDB:       "memory_graph",
			PostgresUser:     "postgres",
			PostgresPassword: "password",
			Volumes:          []string{"postgres-data:/var/lib/postgresql/data"},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test successful start
	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start, got error: %v", err)
	}

	// Verify postgres container was started
	postgresInfo, err := runtime.GetContainerInfo("mcp-compose-postgres-memory")
	if err != nil {
		t.Errorf("Expected postgres container to be started: %v", err)
	} else {
		if postgresInfo.Status != "running" {
			t.Errorf("Expected postgres container to be running, got %s", postgresInfo.Status)
		}
	}

	// Verify memory container was started
	memoryInfo, err := runtime.GetContainerInfo("mcp-compose-memory")
	if err != nil {
		t.Errorf("Expected memory container to be started: %v", err)
	} else {
		if memoryInfo.Status != "running" {
			t.Errorf("Expected memory container to be running, got %s", memoryInfo.Status)
		}

		// Check environment variables
		found := false
		for _, env := range memoryInfo.Env {
			if strings.HasPrefix(env, "DATABASE_URL=") {
				found = true
				if !strings.Contains(env, "sslmode=disable") {
					t.Error("Expected DATABASE_URL to include sslmode=disable")
				}
				break
			}
		}
		if !found {
			t.Error("Expected DATABASE_URL environment variable to be set")
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

func TestManagerStartWithExistingPostgres(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory:  config.MemoryConfig{},
	}

	runtime := NewMockRuntime()
	
	// Start postgres container first
	postgresOpts := &container.ContainerOptions{
		Name:  "mcp-compose-postgres-memory",
		Image: "postgres:15-alpine",
	}
	runtime.StartContainer(postgresOpts)

	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start with existing postgres, got error: %v", err)
	}

	// Should only have started memory container, not postgres again
	memoryInfo, err := runtime.GetContainerInfo("mcp-compose-memory")
	if err != nil {
		t.Errorf("Expected memory container to be started: %v", err)
	}
	if memoryInfo.Status != "running" {
		t.Errorf("Expected memory container to be running, got %s", memoryInfo.Status)
	}
}

func TestManagerStartWithErrors(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*MockRuntime)
		expectErr bool
	}{
		{
			name: "postgres start failure",
			setupFunc: func(r *MockRuntime) {
				r.startErrors["mcp-compose-postgres-memory"] = fmt.Errorf("postgres start failure")
			},
			expectErr: true,
		},
		{
			name: "memory container start failure", 
			setupFunc: func(r *MockRuntime) {
				r.startErrors["mcp-compose-memory"] = fmt.Errorf("memory start failure")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ComposeConfig{
				Version: "1",
				Memory:  config.MemoryConfig{},
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
		Version: "1",
		Memory:  config.MemoryConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start containers first
	memoryOpts := &container.ContainerOptions{
		Name:  "mcp-compose-memory",
		Image: "mcp-compose-memory:latest",
	}
	runtime.StartContainer(memoryOpts)

	postgresOpts := &container.ContainerOptions{
		Name:  "mcp-compose-postgres-memory",
		Image: "postgres:15-alpine",
	}
	runtime.StartContainer(postgresOpts)

	// Test stop
	err := manager.Stop()
	if err != nil {
		t.Fatalf("Expected successful stop, got error: %v", err)
	}

	// Verify containers were stopped
	memoryStatus, _ := runtime.GetContainerStatus("mcp-compose-memory")
	if memoryStatus != "stopped" {
		t.Errorf("Expected memory container to be stopped, got %s", memoryStatus)
	}

	postgresStatus, _ := runtime.GetContainerStatus("mcp-compose-postgres-memory")
	if postgresStatus != "stopped" {
		t.Errorf("Expected postgres container to be stopped, got %s", postgresStatus)
	}
}

func TestManagerStopWithErrors(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory:  config.MemoryConfig{},
	}

	runtime := NewMockRuntime()
	runtime.stopErrors["mcp-compose-memory"] = fmt.Errorf("memory stop failure")
	runtime.stopErrors["mcp-compose-postgres-memory"] = fmt.Errorf("postgres stop failure")

	manager := NewManager(cfg, runtime)

	// Should not return error even if individual containers fail to stop
	err := manager.Stop()
	if err != nil {
		t.Errorf("Stop should handle errors gracefully, got: %v", err)
	}
}

func TestManagerRestart(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory:  config.MemoryConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Start containers first
	memoryOpts := &container.ContainerOptions{
		Name:  "mcp-compose-memory",
		Image: "mcp-compose-memory:latest",
	}
	runtime.StartContainer(memoryOpts)

	err := manager.Restart()
	if err != nil {
		t.Fatalf("Expected successful restart, got error: %v", err)
	}

	// Verify containers are running after restart
	memoryStatus, err := runtime.GetContainerStatus("mcp-compose-memory")
	if err != nil {
		t.Errorf("Failed to get memory container status: %v", err)
	}
	if memoryStatus != "running" {
		t.Errorf("Expected memory container to be running after restart, got %s", memoryStatus)
	}
}

func TestManagerStatus(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory:  config.MemoryConfig{},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	// Test status with non-existent container
	status, err := manager.Status()
	if err == nil {
		t.Error("Expected error for non-existent container")
	}

	// Start memory container
	memoryOpts := &container.ContainerOptions{
		Name:  "mcp-compose-memory",
		Image: "mcp-compose-memory:latest",
	}
	runtime.StartContainer(memoryOpts)

	// Test status with running container
	status, err = manager.Status()
	if err != nil {
		t.Errorf("Expected no error getting status, got: %v", err)
	}
	if status != "running" {
		t.Errorf("Expected status 'running', got %s", status)
	}
}

func TestDefaultConfigurationValues(t *testing.T) {
	// Test with minimal configuration
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory:  config.MemoryConfig{}, // Empty config to test defaults
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start with defaults, got error: %v", err)
	}

	// Verify containers were started with default values
	memoryInfo, err := runtime.GetContainerInfo("mcp-compose-memory")
	if err != nil {
		t.Fatalf("Expected memory container to be started: %v", err)
	}

	// Check default database URL is used
	found := false
	for _, env := range memoryInfo.Env {
		if strings.HasPrefix(env, "DATABASE_URL=") {
			found = true
			if !strings.Contains(env, "mcp-compose-postgres-memory:5432") {
				t.Error("Expected default database URL to reference postgres container")
			}
			break
		}
	}
	if !found {
		t.Error("Expected DATABASE_URL environment variable to be set")
	}

	// Verify postgres container uses defaults
	postgresInfo, err := runtime.GetContainerInfo("mcp-compose-postgres-memory")
	if err != nil {
		t.Fatalf("Expected postgres container to be started: %v", err)
	}

	// Check default postgres environment
	expectedEnvVars := map[string]bool{
		"POSTGRES_DB=memory_graph": false,
		"POSTGRES_USER=postgres":   false,
		"POSTGRES_PASSWORD=password": false,
	}

	for _, env := range postgresInfo.Env {
		for expectedVar := range expectedEnvVars {
			if env == expectedVar {
				expectedEnvVars[expectedVar] = true
			}
		}
	}

	for expectedVar, found := range expectedEnvVars {
		if !found {
			t.Errorf("Expected postgres environment variable %s to be set", expectedVar)
		}
	}
}

func TestCustomConfigurationValues(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Memory: config.MemoryConfig{
			DatabaseURL:      "postgresql://custom:secret@db:5432/custom_db?sslmode=require",
			CPUs:             "4.0",
			Memory:           "4g",
			PostgresCPUs:     "8.0", 
			PostgresMemory:   "8g",
			PostgresDB:       "custom_graph",
			PostgresUser:     "custom_user",
			PostgresPassword: "custom_pass",
			Volumes:          []string{"custom-data:/var/lib/postgresql/data"},
		},
	}

	runtime := NewMockRuntime()
	manager := NewManager(cfg, runtime)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Expected successful start with custom config, got error: %v", err)
	}

	// Verify memory container uses custom values
	memoryInfo, err := runtime.GetContainerInfo("mcp-compose-memory")
	if err != nil {
		t.Fatalf("Expected memory container to be started: %v", err)
	}

	// Check custom database URL is used (with sslmode=disable appended)
	found := false
	for _, env := range memoryInfo.Env {
		if strings.HasPrefix(env, "DATABASE_URL=") {
			found = true
			if !strings.Contains(env, "custom:secret@db:5432/custom_db") {
				t.Error("Expected custom database URL to be preserved")
			}
			if !strings.Contains(env, "sslmode=disable") {
				t.Error("Expected sslmode=disable to be appended")
			}
			break
		}
	}
	if !found {
		t.Error("Expected DATABASE_URL environment variable to be set")
	}

	// Verify postgres container uses custom values
	postgresInfo, err := runtime.GetContainerInfo("mcp-compose-postgres-memory")
	if err != nil {
		t.Fatalf("Expected postgres container to be started: %v", err)
	}

	// Check custom postgres environment
	expectedEnvVars := map[string]bool{
		"POSTGRES_DB=custom_graph":    false,
		"POSTGRES_USER=custom_user":   false,
		"POSTGRES_PASSWORD=custom_pass": false,
	}

	for _, env := range postgresInfo.Env {
		for expectedVar := range expectedEnvVars {
			if env == expectedVar {
				expectedEnvVars[expectedVar] = true
			}
		}
	}

	for expectedVar, found := range expectedEnvVars {
		if !found {
			t.Errorf("Expected postgres environment variable %s to be set", expectedVar)
		}
	}
}

func TestSSLModeHandling(t *testing.T) {
	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "URL without query params",
			inputURL:    "postgresql://user:pass@host:5432/db",
			expectedURL: "postgresql://user:pass@host:5432/db?sslmode=disable",
		},
		{
			name:        "URL with existing query params",
			inputURL:    "postgresql://user:pass@host:5432/db?param=value",
			expectedURL: "postgresql://user:pass@host:5432/db?param=value&sslmode=disable",
		},
		{
			name:        "URL with existing sslmode",
			inputURL:    "postgresql://user:pass@host:5432/db?sslmode=require",
			expectedURL: "postgresql://user:pass@host:5432/db?sslmode=require",
		},
		{
			name:        "URL with sslmode and other params",
			inputURL:    "postgresql://user:pass@host:5432/db?param=value&sslmode=prefer&other=test",
			expectedURL: "postgresql://user:pass@host:5432/db?param=value&sslmode=prefer&other=test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ComposeConfig{
				Version: "1",
				Memory: config.MemoryConfig{
					DatabaseURL: tt.inputURL,
				},
			}

			runtime := NewMockRuntime()
			manager := NewManager(cfg, runtime)

			err := manager.Start()
			if err != nil {
				t.Fatalf("Expected successful start, got error: %v", err)
			}

			// Check the resulting DATABASE_URL
			memoryInfo, err := runtime.GetContainerInfo("mcp-compose-memory")
			if err != nil {
				t.Fatalf("Expected memory container to be started: %v", err)
			}

			found := false
			for _, env := range memoryInfo.Env {
				if strings.HasPrefix(env, "DATABASE_URL=") {
					actualURL := strings.TrimPrefix(env, "DATABASE_URL=")
					if actualURL != tt.expectedURL {
						t.Errorf("Expected URL %q, got %q", tt.expectedURL, actualURL)
					}
					found = true
					break
				}
			}

			if !found {
				t.Error("Expected DATABASE_URL environment variable to be set")
			}
		})
	}
}