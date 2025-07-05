package compose

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/protocol"
)

// mockRuntime implements container.Runtime interface for testing
type mockRuntime struct {
	containers     map[string]container.ContainerInfo
	networks       map[string]bool
	startErrors    map[string]error
	stopErrors     map[string]error
	buildErrors    map[string]error
	logs           map[string]string
	runtimeName    string
}

func newMockRuntime() *mockRuntime {
	return &mockRuntime{
		containers:  make(map[string]container.ContainerInfo),
		networks:    make(map[string]bool),
		startErrors: make(map[string]error),
		stopErrors:  make(map[string]error),
		buildErrors: make(map[string]error),
		logs:        make(map[string]string),
		runtimeName: "mock",
	}
}

func (m *mockRuntime) GetRuntimeName() string {
	return m.runtimeName
}

func (m *mockRuntime) StartContainer(opts *container.ContainerOptions) (*container.ContainerInfo, error) {
	if err, exists := m.startErrors[opts.Name]; exists {
		return nil, err
	}

	info := container.ContainerInfo{
		ID:     "mock-" + opts.Name,
		Name:   opts.Name,
		Status: "running",
		Image:  opts.Image,
		Ports:  []container.PortBinding{},
	}
	m.containers[opts.Name] = info
	return &info, nil
}

func (m *mockRuntime) StopContainer(name string) error {
	if err, exists := m.stopErrors[name]; exists {
		return err
	}

	if info, exists := m.containers[name]; exists {
		info.Status = "stopped"
		m.containers[name] = info
	}
	return nil
}

func (m *mockRuntime) GetContainerStatus(name string) (string, error) {
	if info, exists := m.containers[name]; exists {
		return info.Status, nil
	}
	return "", fmt.Errorf("container not found")
}

func (m *mockRuntime) ListContainers() ([]container.ContainerInfo, error) {
	var infos []container.ContainerInfo
	for _, info := range m.containers {
		infos = append(infos, info)
	}
	return infos, nil
}

func (m *mockRuntime) ShowContainerLogs(name string, follow bool) error {
	if logs, exists := m.logs[name]; exists {
		fmt.Print(logs)
		return nil
	}
	return fmt.Errorf("no logs for container %s", name)
}

func (m *mockRuntime) BuildImage(opts *container.BuildOptions) error {
	if len(opts.Tags) > 0 {
		if err, exists := m.buildErrors[opts.Tags[0]]; exists {
			return err
		}
	}
	return nil
}

func (m *mockRuntime) NetworkExists(name string) (bool, error) {
	return m.networks[name], nil
}

func (m *mockRuntime) CreateNetwork(name string) error {
	m.networks[name] = true
	return nil
}

func (m *mockRuntime) DeleteNetwork(name string) error {
	delete(m.networks, name)
	return nil
}

func (m *mockRuntime) InspectContainer(name string) (*container.ContainerInfo, error) {
	if info, exists := m.containers[name]; exists {
		return &info, nil
	}
	return nil, fmt.Errorf("container not found")
}

func (m *mockRuntime) ExecInContainer(ctx context.Context, name string, cmd []string) error {
	return nil
}

func (m *mockRuntime) CopyToContainer(ctx context.Context, name, srcPath, destPath string) error {
	return nil
}

func (m *mockRuntime) CopyFromContainer(ctx context.Context, name, srcPath, destPath string) error {
	return nil
}

func (m *mockRuntime) PullImage(ctx context.Context, image string) error {
	return nil
}

// Test helpers
func createTestConfig(t *testing.T, content string) string {
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test-config.yaml")
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}
	return configFile
}

func TestNewComposer(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
logging:
  level: "info"
`
	configFile := createTestConfig(t, configContent)

	// Test successful creation
	composer, err := NewComposer(configFile)
	if err != nil {
		t.Fatalf("Expected successful composer creation, got error: %v", err)
	}

	if composer == nil {
		t.Fatal("Expected composer to be created")
	}

	if composer.config == nil {
		t.Error("Expected config to be loaded")
	}

	if composer.manager == nil {
		t.Error("Expected manager to be created")
	}

	if composer.protocolManagers == nil {
		t.Error("Expected protocol managers to be initialized")
	}

	// Test protocol managers for server
	managers := composer.GetProtocolManagers("test-server")
	if managers == nil {
		t.Error("Expected protocol managers for test-server")
	}

	if managers.Progress == nil {
		t.Error("Expected progress manager")
	}

	if managers.Resource == nil {
		t.Error("Expected resource manager")
	}

	// Test shutdown
	err = composer.Shutdown()
	if err != nil {
		t.Errorf("Expected clean shutdown, got error: %v", err)
	}
}

func TestNewComposerInvalidConfig(t *testing.T) {
	// Test with non-existent config file
	_, err := NewComposer("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}

	// Test with invalid config content
	invalidConfig := `invalid yaml content [[[`
	configFile := createTestConfig(t, invalidConfig)
	
	_, err = NewComposer(configFile)
	if err == nil {
		t.Error("Expected error for invalid config content")
	}
}

func TestComposerStartStopServer(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
`
	configFile := createTestConfig(t, configContent)

	composer, err := NewComposer(configFile)
	if err != nil {
		t.Fatalf("Failed to create composer: %v", err)
	}
	defer composer.Shutdown()

	// Test starting server
	err = composer.StartServer("test-server")
	if err != nil {
		t.Errorf("Expected successful server start, got error: %v", err)
	}

	// Test starting non-existent server
	err = composer.StartServer("non-existent")
	if err == nil {
		t.Error("Expected error when starting non-existent server")
	}

	// Test stopping server
	err = composer.StopServer("test-server")
	if err != nil {
		t.Errorf("Expected successful server stop, got error: %v", err)
	}
}

func TestComposerStartStopAll(t *testing.T) {
	configContent := `version: "1"
servers:
  server1:
    protocol: stdio
    command: "echo hello1"
  server2:
    protocol: stdio
    command: "echo hello2"
`
	configFile := createTestConfig(t, configContent)

	composer, err := NewComposer(configFile)
	if err != nil {
		t.Fatalf("Failed to create composer: %v", err)
	}
	defer composer.Shutdown()

	// Test starting all servers
	err = composer.StartAll()
	if err != nil {
		t.Errorf("Expected successful start all, got error: %v", err)
	}

	// Test stopping all servers
	err = composer.StopAll()
	if err != nil {
		t.Errorf("Expected successful stop all, got error: %v", err)
	}
}

func TestUpFunction(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
`
	configFile := createTestConfig(t, configContent)

	// Test up with all servers
	err := Up(configFile, []string{})
	if err != nil {
		t.Errorf("Expected successful up command, got error: %v", err)
	}

	// Test up with specific servers
	err = Up(configFile, []string{"test-server"})
	if err != nil {
		t.Errorf("Expected successful up command with specific server, got error: %v", err)
	}

	// Test up with non-existent server
	err = Up(configFile, []string{"non-existent"})
	if err != nil {
		t.Errorf("Up should handle non-existent servers gracefully, got error: %v", err)
	}
}

func TestDownFunction(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    image: "test-image"
    protocol: http
    http_port: 8080
`
	configFile := createTestConfig(t, configContent)

	// Test down with container runtime
	err := Down(configFile, []string{})
	if err != nil {
		t.Errorf("Expected successful down command, got error: %v", err)
	}

	// Test down with specific servers
	err = Down(configFile, []string{"test-server"})
	if err != nil {
		t.Errorf("Expected successful down command with specific server, got error: %v", err)
	}
}

func TestStartStopFunctions(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
`
	configFile := createTestConfig(t, configContent)

	// Test start function
	err := Start(configFile, []string{"test-server"})
	if err != nil {
		t.Errorf("Expected successful start, got error: %v", err)
	}

	// Test start with no servers
	err = Start(configFile, []string{})
	if err == nil {
		t.Error("Expected error when starting with no servers specified")
	}

	// Test stop function  
	err = Stop(configFile, []string{"test-server"})
	if err != nil {
		t.Errorf("Expected successful stop, got error: %v", err)
	}

	// Test stop with no servers
	err = Stop(configFile, []string{})
	if err == nil {
		t.Error("Expected error when stopping with no servers specified")
	}
}

func TestListFunction(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
  container-server:
    image: "test-image"
    protocol: http
    http_port: 8080
`
	configFile := createTestConfig(t, configContent)

	err := List(configFile)
	if err != nil {
		t.Errorf("Expected successful list command, got error: %v", err)
	}
}

func TestValidateFunction(t *testing.T) {
	configContent := `version: "1"
servers:
  test-server:
    protocol: stdio
    command: "echo hello"
`
	configFile := createTestConfig(t, configContent)

	// Test valid config
	err := Validate(configFile)
	if err != nil {
		t.Errorf("Expected successful validation, got error: %v", err)
	}

	// Test invalid config file
	err = Validate("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLogsFunction(t *testing.T) {
	configContent := `version: "1"
servers:
  container-server:
    image: "test-image"
    protocol: http
    http_port: 8080
  process-server:
    protocol: stdio
    command: "echo hello"
`
	configFile := createTestConfig(t, configContent)

	// Test logs with all servers
	err := Logs(configFile, []string{}, false)
	if err != nil {
		t.Errorf("Expected successful logs command, got error: %v", err)
	}

	// Test logs with specific servers
	err = Logs(configFile, []string{"container-server"}, false)
	if err != nil {
		t.Errorf("Expected successful logs command with specific server, got error: %v", err)
	}

	// Test logs with follow
	err = Logs(configFile, []string{"container-server"}, true)
	if err != nil {
		t.Errorf("Expected successful logs command with follow, got error: %v", err)
	}
}

func TestIsContainerServer(t *testing.T) {
	tests := []struct {
		name     string
		config   config.ServerConfig
		expected bool
	}{
		{
			name: "server with image",
			config: config.ServerConfig{
				Image: "test-image:latest",
			},
			expected: true,
		},
		{
			name: "server with build context",
			config: config.ServerConfig{
				Build: config.BuildConfig{
					Context: "./build",
				},
			},
			expected: true,
		},
		{
			name: "server with volumes",
			config: config.ServerConfig{
				Volumes: []string{"/data:/app/data"},
			},
			expected: true,
		},
		{
			name: "server with networks",
			config: config.ServerConfig{
				Networks: []string{"custom-network"},
			},
			expected: true,
		},
		{
			name: "server with HTTP port",
			config: config.ServerConfig{
				Protocol: "http",
				HttpPort: 8080,
			},
			expected: true,
		},
		{
			name: "server with user",
			config: config.ServerConfig{
				User: "1000:1000",
			},
			expected: true,
		},
		{
			name: "server with capabilities",
			config: config.ServerConfig{
				CapAdd: []string{"NET_ADMIN"},
			},
			expected: true,
		},
		{
			name: "server with resource limits",
			config: config.ServerConfig{
				Deploy: config.DeployConfig{
					Resources: config.ResourcesDeployConfig{
						Limits: config.ResourceLimitsConfig{
							CPUs:   "0.5",
							Memory: "512M",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "simple process server",
			config: config.ServerConfig{
				Protocol: "stdio",
				Command:  "echo hello",
			},
			expected: false,
		},
		{
			name: "server with container-style command",
			config: config.ServerConfig{
				Command: "/app/server",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isContainerServer(tt.config)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for config: %+v", tt.expected, result, tt.config)
			}
		})
	}
}

func TestDetermineServerNetworks(t *testing.T) {
	tests := []struct {
		name     string
		config   config.ServerConfig
		expected []string
	}{
		{
			name: "server with network mode",
			config: config.ServerConfig{
				NetworkMode: "host",
			},
			expected: nil,
		},
		{
			name: "server with custom networks",
			config: config.ServerConfig{
				Networks: []string{"custom1", "custom2"},
			},
			expected: []string{"custom1", "custom2", "mcp-net"},
		},
		{
			name:     "server with no network config",
			config:   config.ServerConfig{},
			expected: []string{"mcp-net"},
		},
		{
			name: "server with mcp-net already specified",
			config: config.ServerConfig{
				Networks: []string{"mcp-net", "custom"},
			},
			expected: []string{"mcp-net", "custom"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineServerNetworks(tt.config)
			
			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d networks, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check contents (order might vary due to deduplication)
			resultMap := make(map[string]bool)
			for _, net := range result {
				resultMap[net] = true
			}

			for _, expected := range tt.expected {
				if !resultMap[expected] {
					t.Errorf("Expected network %q not found in result: %v", expected, result)
				}
			}
		})
	}
}

func TestGetServersToStart(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"server1": {
				Protocol: "stdio",
				Command:  "echo server1",
			},
			"server2": {
				Protocol:  "stdio", 
				Command:   "echo server2",
				DependsOn: []string{"server1"},
			},
			"server3": {
				Protocol:  "stdio",
				Command:   "echo server3", 
				DependsOn: []string{"server2"},
			},
		},
	}

	tests := []struct {
		name         string
		serverNames  []string
		expectedLen  int
		expectedLast string // The last server in dependency order
	}{
		{
			name:         "start all servers",
			serverNames:  []string{},
			expectedLen:  3,
			expectedLast: "server3",
		},
		{
			name:         "start specific server with dependencies",
			serverNames:  []string{"server3"},
			expectedLen:  3,
			expectedLast: "server3",
		},
		{
			name:         "start server without dependencies",
			serverNames:  []string{"server1"},
			expectedLen:  1,
			expectedLast: "server1",
		},
		{
			name:         "start middle server with dependencies",
			serverNames:  []string{"server2"},
			expectedLen:  2,
			expectedLast: "server2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getServersToStart(cfg, tt.serverNames)

			if len(result) != tt.expectedLen {
				t.Errorf("Expected %d servers, got %d: %v", tt.expectedLen, len(result), result)
			}

			if len(result) > 0 && result[len(result)-1] != tt.expectedLast {
				t.Errorf("Expected last server to be %q, got %q", tt.expectedLast, result[len(result)-1])
			}

			// Verify dependency order
			serverOrder := make(map[string]int)
			for i, server := range result {
				serverOrder[server] = i
			}

			for _, serverName := range result {
				serverCfg := cfg.Servers[serverName]
				for _, dep := range serverCfg.DependsOn {
					if depOrder, exists := serverOrder[dep]; exists {
						if serverOrder[serverName] <= depOrder {
							t.Errorf("Server %q should come after its dependency %q", serverName, dep)
						}
					}
				}
			}
		})
	}
}

func TestShortDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{time.Nanosecond * 500, "500ns"},
		{time.Millisecond * 250, "250.00ms"},
		{time.Second * 2, "2.00s"},
		{time.Minute * 1, "60.00s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := ShortDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestConvertSecurityConfig(t *testing.T) {
	serverName := "test-server"
	serverCfg := config.ServerConfig{
		Image:       "test-image:latest",
		Command:     "/app/server",
		Args:        []string{"--config", "/config.yaml"},
		Privileged:  false,
		User:        "1000:1000",
		ReadOnly:    true,
		CapAdd:      []string{"NET_BIND_SERVICE"},
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges:true"},
		Env: map[string]string{
			"ENV_VAR": "value",
		},
		Deploy: config.DeployConfig{
			Resources: config.ResourcesDeployConfig{
				Limits: config.ResourceLimitsConfig{
					CPUs:   "0.5",
					Memory: "512M",
					PIDs:   100,
				},
			},
		},
		Security: config.SecurityConfig{
			NoNewPrivileges: true,
			AppArmor:        "unconfined",
		},
	}

	opts := convertSecurityConfig(serverName, serverCfg)

	// Test basic container options
	expectedName := "mcp-compose-test-server"
	if opts.Name != expectedName {
		t.Errorf("Expected name %q, got %q", expectedName, opts.Name)
	}

	if opts.Image != serverCfg.Image {
		t.Errorf("Expected image %q, got %q", serverCfg.Image, opts.Image)
	}

	if opts.Command != serverCfg.Command {
		t.Errorf("Expected command %q, got %q", serverCfg.Command, opts.Command)
	}

	// Test security settings
	if opts.Privileged != serverCfg.Privileged {
		t.Errorf("Expected privileged %v, got %v", serverCfg.Privileged, opts.Privileged)
	}

	if opts.User != serverCfg.User {
		t.Errorf("Expected user %q, got %q", serverCfg.User, opts.User)
	}

	if opts.ReadOnly != serverCfg.ReadOnly {
		t.Errorf("Expected readOnly %v, got %v", serverCfg.ReadOnly, opts.ReadOnly)
	}

	// Test capabilities
	if len(opts.CapAdd) != len(serverCfg.CapAdd) {
		t.Errorf("Expected %d CapAdd entries, got %d", len(serverCfg.CapAdd), len(opts.CapAdd))
	}

	if len(opts.CapDrop) != len(serverCfg.CapDrop) {
		t.Errorf("Expected %d CapDrop entries, got %d", len(serverCfg.CapDrop), len(opts.CapDrop))
	}

	// Test resource limits
	if opts.CPUs != serverCfg.Deploy.Resources.Limits.CPUs {
		t.Errorf("Expected CPUs %q, got %q", serverCfg.Deploy.Resources.Limits.CPUs, opts.CPUs)
	}

	if opts.Memory != serverCfg.Deploy.Resources.Limits.Memory {
		t.Errorf("Expected Memory %q, got %q", serverCfg.Deploy.Resources.Limits.Memory, opts.Memory)
	}

	if opts.PidsLimit != serverCfg.Deploy.Resources.Limits.PIDs {
		t.Errorf("Expected PidsLimit %d, got %d", serverCfg.Deploy.Resources.Limits.PIDs, opts.PidsLimit)
	}

	// Test environment variables
	if opts.Env["MCP_SERVER_NAME"] != serverName {
		t.Errorf("Expected MCP_SERVER_NAME to be %q, got %q", serverName, opts.Env["MCP_SERVER_NAME"])
	}

	if opts.Env["ENV_VAR"] != "value" {
		t.Errorf("Expected ENV_VAR to be 'value', got %q", opts.Env["ENV_VAR"])
	}

	// Test security options
	expectedSecOpts := 2 // no-new-privileges + apparmor
	if len(opts.SecurityOpt) < expectedSecOpts {
		t.Errorf("Expected at least %d security options, got %d: %v", expectedSecOpts, len(opts.SecurityOpt), opts.SecurityOpt)
	}

	// Check for specific security options
	hasNoNewPrivs := false
	hasAppArmor := false
	for _, opt := range opts.SecurityOpt {
		if opt == "no-new-privileges:true" {
			hasNoNewPrivs = true
		}
		if strings.HasPrefix(opt, "apparmor:") {
			hasAppArmor = true
		}
	}

	if !hasNoNewPrivs {
		t.Error("Expected no-new-privileges security option")
	}

	if !hasAppArmor {
		t.Error("Expected apparmor security option")
	}
}

func TestServerCfgHasHTTPArg(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "has transport http",
			args:     []string{"--transport", "http"},
			expected: true,
		},
		{
			name:     "has port flag",
			args:     []string{"--port", "8080"},
			expected: true,
		},
		{
			name:     "has port-server flag",
			args:     []string{"--port-server", "9090"},
			expected: true,
		},
		{
			name:     "no http args",
			args:     []string{"--verbose", "--config", "file.yaml"},
			expected: false,
		},
		{
			name:     "transport but not http",
			args:     []string{"--transport", "stdio"},
			expected: false,
		},
		{
			name:     "empty args",
			args:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := serverCfgHasHTTPArg(tt.args)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for args: %v", tt.expected, result, tt.args)
			}
		})
	}
}

func TestProtocolManagerSet(t *testing.T) {
	managers := &ProtocolManagerSet{
		Progress:     protocol.NewProgressManager(),
		Resource:     protocol.NewResourceManager(),
		Sampling:     protocol.NewSamplingManager(),
		Subscription: protocol.NewSubscriptionManager(),
		Change:       protocol.NewChangeNotificationManager(),
	}

	if managers.Progress == nil {
		t.Error("Expected progress manager to be initialized")
	}

	if managers.Resource == nil {
		t.Error("Expected resource manager to be initialized")
	}

	if managers.Sampling == nil {
		t.Error("Expected sampling manager to be initialized")
	}

	if managers.Subscription == nil {
		t.Error("Expected subscription manager to be initialized")
	}

	if managers.Change == nil {
		t.Error("Expected change notification manager to be initialized")
	}
}

func TestStartServerProcess(t *testing.T) {
	serverName := "test-process-server"
	serverCfg := config.ServerConfig{
		Protocol: "stdio",
		Command:  "echo",
		Args:     []string{"hello", "world"},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
		WorkDir: "/tmp",
	}

	// This test mainly verifies the function doesn't panic
	// and handles the process creation properly
	err := startServerProcess(serverName, serverCfg)
	if err != nil {
		// This is expected to fail in test environment without actual process
		// Just verify the error is reasonable
		if !strings.Contains(err.Error(), "failed to start process") &&
			!strings.Contains(err.Error(), "failed to create process") {
			t.Errorf("Unexpected error type: %v", err)
		}
	}
}

func TestCollectRequiredNetworks(t *testing.T) {
	cfg := &config.ComposeConfig{
		Servers: map[string]config.ServerConfig{
			"container1": {
				Image:    "test:latest",
				Networks: []string{"net1", "net2"},
			},
			"container2": {
				Image:    "test:latest", 
				Networks: []string{"net2", "net3"},
			},
			"process1": {
				Protocol: "stdio",
				Command:  "echo hello",
			},
		},
	}

	serverNames := []string{"container1", "container2", "process1"}
	networks := collectRequiredNetworks(cfg, serverNames)

	// Should only include networks from container servers
	expectedNetworks := map[string]bool{
		"net1":    true,
		"net2":    true, 
		"net3":    true,
		"mcp-net": true, // Added by determineServerNetworks
	}

	for netName := range expectedNetworks {
		if _, exists := networks[netName]; !exists {
			t.Errorf("Expected network %q to be collected", netName)
		}
	}

	// Check that net2 has both servers
	if len(networks["net2"]) < 2 {
		t.Errorf("Expected net2 to have multiple servers, got: %v", networks["net2"])
	}
}

func TestGenerateNetworkDescription(t *testing.T) {
	tests := []struct {
		name              string
		networkToServers  map[string][]string
		expectedContains  []string
	}{
		{
			name:              "no networks",
			networkToServers:  map[string][]string{},
			expectedContains:  []string{"localhost", "host networking"},
		},
		{
			name: "single custom network",
			networkToServers: map[string][]string{
				"custom-net": {"server1", "server2"},
			},
			expectedContains: []string{"custom-net"},
		},
		{
			name: "host network",
			networkToServers: map[string][]string{
				"host": {"server1"},
			},
			expectedContains: []string{"host networking"},
		},
		{
			name: "multiple networks",
			networkToServers: map[string][]string{
				"net1": {"server1"},
				"net2": {"server2"},
			},
			expectedContains: []string{"net1", "net2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateNetworkDescription(tt.networkToServers)
			
			for _, expected := range tt.expectedContains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}