package container

import (
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"mcpcompose/internal/config"
)

// MockRuntime implements the Runtime interface for testing
type MockRuntime struct {
	containers          map[string]*ContainerInfo
	images              []ImageInfo
	volumes             []VolumeInfo
	networks            map[string]*NetworkInfo
	containerErrors     map[string]error
	commandOutputs      map[string]string
	runtimeName         string
	shouldFailStart     bool
	shouldFailStop      bool
	shouldFailNetwork   bool
	validateSecError    error
}

func NewMockRuntime() *MockRuntime {
	return &MockRuntime{
		containers:      make(map[string]*ContainerInfo),
		networks:        make(map[string]*NetworkInfo),
		containerErrors: make(map[string]error),
		commandOutputs:  make(map[string]string),
		runtimeName:     "mock",
	}
}

func (m *MockRuntime) GetRuntimeName() string {
	return m.runtimeName
}

func (m *MockRuntime) StartContainer(opts *ContainerOptions) (string, error) {
	if m.shouldFailStart {
		return "", fmt.Errorf("mock start failure")
	}
	
	if err := ValidateContainerOptions(opts); err != nil {
		return "", err
	}

	containerID := fmt.Sprintf("mock-id-%s", opts.Name)
	container := &ContainerInfo{
		ID:      containerID,
		Name:    opts.Name,
		Image:   opts.Image,
		Status:  "running",
		State:   "running",
		Command: append([]string{opts.Command}, opts.Args...),
		Env:     mapToSlice(opts.Env),
		Labels:  opts.Labels,
		Ports:   []PortBinding{},
	}

	// Convert port strings to PortBinding
	for _, portStr := range opts.Ports {
		// Simple parsing for test purposes
		if strings.Contains(portStr, ":") {
			container.Ports = append(container.Ports, PortBinding{
				PrivatePort: 8080,
				PublicPort:  8080,
				Type:        "tcp",
			})
		}
	}

	m.containers[opts.Name] = container
	return containerID, nil
}

func (m *MockRuntime) StopContainer(name string) error {
	if m.shouldFailStop {
		return fmt.Errorf("mock stop failure")
	}

	if container, exists := m.containers[name]; exists {
		container.Status = "stopped"
		container.State = "exited"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) RestartContainer(name string) error {
	if container, exists := m.containers[name]; exists {
		container.Status = "running"
		container.State = "running"
		container.RestartCount++
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) PauseContainer(name string) error {
	if container, exists := m.containers[name]; exists {
		container.Status = "paused"
		container.State = "paused"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) UnpauseContainer(name string) error {
	if container, exists := m.containers[name]; exists {
		container.Status = "running"
		container.State = "running"
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerStatus(name string) (string, error) {
	if err, exists := m.containerErrors[name]; exists {
		return "", err
	}
	
	if container, exists := m.containers[name]; exists {
		return container.Status, nil
	}
	return "", fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) GetContainerInfo(name string) (*ContainerInfo, error) {
	if container, exists := m.containers[name]; exists {
		return container, nil
	}
	return nil, fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) ListContainers(filters map[string]string) ([]ContainerInfo, error) {
	var containers []ContainerInfo
	for _, container := range m.containers {
		containers = append(containers, *container)
	}
	return containers, nil
}

func (m *MockRuntime) GetContainerStats(name string) (*ContainerStats, error) {
	if _, exists := m.containers[name]; exists {
		return &ContainerStats{
			CPUUsage:    50.5,
			MemoryUsage: 1024 * 1024 * 128, // 128MB
			MemoryLimit: 1024 * 1024 * 512, // 512MB
		}, nil
	}
	return nil, fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) WaitForContainer(name string, condition string) error {
	if _, exists := m.containers[name]; exists {
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) ShowContainerLogs(name string, follow bool) error {
	if output, exists := m.commandOutputs[name]; exists {
		fmt.Print(output)
		return nil
	}
	return fmt.Errorf("no logs for container: %s", name)
}

func (m *MockRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	if _, exists := m.containers[containerName]; !exists {
		return nil, nil, nil, fmt.Errorf("container not found: %s", containerName)
	}
	
	// Return mock exec.Cmd for testing
	cmd := exec.Command("echo", "mock-exec")
	return cmd, nil, nil, nil
}

func (m *MockRuntime) PullImage(image string, auth *ImageAuth) error {
	return nil
}

func (m *MockRuntime) BuildImage(opts *BuildOptions) error {
	return nil
}

func (m *MockRuntime) RemoveImage(image string, force bool) error {
	return nil
}

func (m *MockRuntime) ListImages() ([]ImageInfo, error) {
	return m.images, nil
}

func (m *MockRuntime) CreateVolume(name string, opts *VolumeOptions) error {
	volume := VolumeInfo{
		Name:   name,
		Driver: "local",
	}
	if opts != nil {
		volume.Driver = opts.Driver
		volume.Labels = opts.Labels
		volume.Options = opts.DriverOpts
	}
	m.volumes = append(m.volumes, volume)
	return nil
}

func (m *MockRuntime) RemoveVolume(name string, force bool) error {
	for i, volume := range m.volumes {
		if volume.Name == name {
			m.volumes = append(m.volumes[:i], m.volumes[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("volume not found: %s", name)
}

func (m *MockRuntime) ListVolumes() ([]VolumeInfo, error) {
	return m.volumes, nil
}

func (m *MockRuntime) NetworkExists(name string) (bool, error) {
	if m.shouldFailNetwork {
		return false, fmt.Errorf("mock network failure")
	}
	_, exists := m.networks[name]
	return exists, nil
}

func (m *MockRuntime) CreateNetwork(name string) error {
	if m.shouldFailNetwork {
		return fmt.Errorf("mock network creation failure")
	}
	m.networks[name] = &NetworkInfo{
		Name:   name,
		Driver: "bridge",
	}
	return nil
}

func (m *MockRuntime) RemoveNetwork(name string) error {
	if m.shouldFailNetwork {
		return fmt.Errorf("mock network removal failure")
	}
	delete(m.networks, name)
	return nil
}

func (m *MockRuntime) ListNetworks() ([]NetworkInfo, error) {
	var networks []NetworkInfo
	for _, network := range m.networks {
		networks = append(networks, *network)
	}
	return networks, nil
}

func (m *MockRuntime) GetNetworkInfo(name string) (*NetworkInfo, error) {
	if network, exists := m.networks[name]; exists {
		return network, nil
	}
	return nil, fmt.Errorf("network not found: %s", name)
}

func (m *MockRuntime) ConnectToNetwork(containerName, networkName string) error {
	return nil
}

func (m *MockRuntime) DisconnectFromNetwork(containerName, networkName string) error {
	return nil
}

func (m *MockRuntime) UpdateContainerResources(name string, resources *ResourceLimits) error {
	if _, exists := m.containers[name]; exists {
		return nil
	}
	return fmt.Errorf("container not found: %s", name)
}

func (m *MockRuntime) ValidateSecurityContext(opts *ContainerOptions) error {
	if m.validateSecError != nil {
		return m.validateSecError
	}
	return nil
}

// Helper function to convert map to slice for environment variables
func mapToSlice(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

func TestContainerOptions(t *testing.T) {
	opts := &ContainerOptions{
		Name:        "test-container",
		Image:       "test:latest",
		Command:     "/app/server",
		Args:        []string{"--port", "8080"},
		Env:         map[string]string{"ENV_VAR": "value"},
		Ports:       []string{"8080:8080"},
		Volumes:     []string{"/data:/app/data"},
		Networks:    []string{"test-network"},
		WorkDir:     "/app",
		Privileged:  false,
		User:        "1000:1000",
		CapAdd:      []string{"NET_BIND_SERVICE"},
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges:true"},
		CPUs:        "0.5",
		Memory:      "512M",
		PidsLimit:   100,
	}

	if opts.Name != "test-container" {
		t.Errorf("Expected name 'test-container', got %s", opts.Name)
	}

	if opts.Image != "test:latest" {
		t.Errorf("Expected image 'test:latest', got %s", opts.Image)
	}

	if len(opts.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(opts.Args))
	}

	if opts.Env["ENV_VAR"] != "value" {
		t.Errorf("Expected ENV_VAR=value, got %s", opts.Env["ENV_VAR"])
	}

	if opts.Privileged {
		t.Error("Expected privileged to be false")
	}

	if opts.User != "1000:1000" {
		t.Errorf("Expected user '1000:1000', got %s", opts.User)
	}

	if opts.CPUs != "0.5" {
		t.Errorf("Expected CPUs '0.5', got %s", opts.CPUs)
	}

	if opts.Memory != "512M" {
		t.Errorf("Expected Memory '512M', got %s", opts.Memory)
	}

	if opts.PidsLimit != 100 {
		t.Errorf("Expected PidsLimit 100, got %d", opts.PidsLimit)
	}
}

func TestHealthCheck(t *testing.T) {
	hc := &HealthCheck{
		Test:        []string{"CMD", "curl", "-f", "http://localhost:8080/health"},
		Interval:    "30s",
		Timeout:     "5s",
		Retries:     3,
		StartPeriod: "60s",
	}

	if len(hc.Test) != 4 {
		t.Errorf("Expected 4 test elements, got %d", len(hc.Test))
	}

	if hc.Interval != "30s" {
		t.Errorf("Expected interval '30s', got %s", hc.Interval)
	}

	if hc.Retries != 3 {
		t.Errorf("Expected 3 retries, got %d", hc.Retries)
	}
}

func TestSecurityConfig(t *testing.T) {
	sec := SecurityConfig{
		AllowDockerSocket:  false,
		AllowHostMounts:    []string{"/safe/path"},
		AllowPrivilegedOps: false,
		TrustedImage:       true,
	}

	if sec.AllowDockerSocket {
		t.Error("Expected AllowDockerSocket to be false")
	}

	if len(sec.AllowHostMounts) != 1 {
		t.Errorf("Expected 1 allowed host mount, got %d", len(sec.AllowHostMounts))
	}

	if !sec.TrustedImage {
		t.Error("Expected TrustedImage to be true")
	}
}

func TestContainerInfo(t *testing.T) {
	info := &ContainerInfo{
		ID:     "abc123",
		Name:   "test-container",
		Image:  "test:latest",
		Status: "running",
		State:  "running",
		Ports: []PortBinding{
			{
				PrivatePort: 8080,
				PublicPort:  8080,
				Type:        "tcp",
				IP:          "0.0.0.0",
			},
		},
		Env:     []string{"ENV_VAR=value"},
		Command: []string{"/app/server", "--port", "8080"},
		Labels:  map[string]string{"version": "1.0"},
	}

	if info.ID != "abc123" {
		t.Errorf("Expected ID 'abc123', got %s", info.ID)
	}

	if info.Status != "running" {
		t.Errorf("Expected status 'running', got %s", info.Status)
	}

	if len(info.Ports) != 1 {
		t.Errorf("Expected 1 port binding, got %d", len(info.Ports))
	}

	if info.Ports[0].PrivatePort != 8080 {
		t.Errorf("Expected private port 8080, got %d", info.Ports[0].PrivatePort)
	}

	if len(info.Command) != 3 {
		t.Errorf("Expected 3 command elements, got %d", len(info.Command))
	}

	if info.Labels["version"] != "1.0" {
		t.Errorf("Expected version label '1.0', got %s", info.Labels["version"])
	}
}

func TestMockRuntime(t *testing.T) {
	runtime := NewMockRuntime()

	// Test runtime name
	if runtime.GetRuntimeName() != "mock" {
		t.Errorf("Expected runtime name 'mock', got %s", runtime.GetRuntimeName())
	}

	// Test starting container
	opts := &ContainerOptions{
		Name:  "test-container",
		Image: "test:latest",
	}

	containerID, err := runtime.StartContainer(opts)
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	if !strings.HasPrefix(containerID, "mock-id-") {
		t.Errorf("Expected container ID to start with 'mock-id-', got %s", containerID)
	}

	// Test getting container status
	status, err := runtime.GetContainerStatus("test-container")
	if err != nil {
		t.Fatalf("Failed to get container status: %v", err)
	}

	if status != "running" {
		t.Errorf("Expected status 'running', got %s", status)
	}

	// Test getting container info
	info, err := runtime.GetContainerInfo("test-container")
	if err != nil {
		t.Fatalf("Failed to get container info: %v", err)
	}

	if info.Name != "test-container" {
		t.Errorf("Expected container name 'test-container', got %s", info.Name)
	}

	// Test stopping container
	err = runtime.StopContainer("test-container")
	if err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	status, err = runtime.GetContainerStatus("test-container")
	if err != nil {
		t.Fatalf("Failed to get container status after stop: %v", err)
	}

	if status != "stopped" {
		t.Errorf("Expected status 'stopped' after stop, got %s", status)
	}
}

func TestMockRuntimeNetworking(t *testing.T) {
	runtime := NewMockRuntime()

	// Test network creation
	err := runtime.CreateNetwork("test-network")
	if err != nil {
		t.Fatalf("Failed to create network: %v", err)
	}

	// Test network exists
	exists, err := runtime.NetworkExists("test-network")
	if err != nil {
		t.Fatalf("Failed to check network existence: %v", err)
	}

	if !exists {
		t.Error("Expected network to exist after creation")
	}

	// Test network info
	info, err := runtime.GetNetworkInfo("test-network")
	if err != nil {
		t.Fatalf("Failed to get network info: %v", err)
	}

	if info.Name != "test-network" {
		t.Errorf("Expected network name 'test-network', got %s", info.Name)
	}

	// Test network removal
	err = runtime.RemoveNetwork("test-network")
	if err != nil {
		t.Fatalf("Failed to remove network: %v", err)
	}

	exists, err = runtime.NetworkExists("test-network")
	if err != nil {
		t.Fatalf("Failed to check network existence after removal: %v", err)
	}

	if exists {
		t.Error("Expected network to not exist after removal")
	}
}

func TestMockRuntimeVolumes(t *testing.T) {
	runtime := NewMockRuntime()

	// Test volume creation
	opts := &VolumeOptions{
		Driver:     "local",
		Labels:     map[string]string{"type": "test"},
		DriverOpts: map[string]string{"o": "bind"},
	}

	err := runtime.CreateVolume("test-volume", opts)
	if err != nil {
		t.Fatalf("Failed to create volume: %v", err)
	}

	// Test listing volumes
	volumes, err := runtime.ListVolumes()
	if err != nil {
		t.Fatalf("Failed to list volumes: %v", err)
	}

	if len(volumes) != 1 {
		t.Errorf("Expected 1 volume, got %d", len(volumes))
	}

	if volumes[0].Name != "test-volume" {
		t.Errorf("Expected volume name 'test-volume', got %s", volumes[0].Name)
	}

	// Test volume removal
	err = runtime.RemoveVolume("test-volume", false)
	if err != nil {
		t.Fatalf("Failed to remove volume: %v", err)
	}

	volumes, err = runtime.ListVolumes()
	if err != nil {
		t.Fatalf("Failed to list volumes after removal: %v", err)
	}

	if len(volumes) != 0 {
		t.Errorf("Expected 0 volumes after removal, got %d", len(volumes))
	}
}

func TestValidateContainerOptions(t *testing.T) {
	tests := []struct {
		name      string
		opts      *ContainerOptions
		expectErr bool
	}{
		{
			name: "valid options with image",
			opts: &ContainerOptions{
				Name:  "test-container",
				Image: "test:latest",
			},
			expectErr: false,
		},
		{
			name: "valid options with build context",
			opts: &ContainerOptions{
				Name: "test-container",
				Build: config.BuildConfig{
					Context: "./build",
				},
			},
			expectErr: false,
		},
		{
			name: "missing name",
			opts: &ContainerOptions{
				Image: "test:latest",
			},
			expectErr: true,
		},
		{
			name: "missing image and build context",
			opts: &ContainerOptions{
				Name: "test-container",
			},
			expectErr: true,
		},
		{
			name: "valid with ports",
			opts: &ContainerOptions{
				Name:  "test-container",
				Image: "test:latest",
				Ports: []string{"8080:8080", "9090:9090"},
			},
			expectErr: false,
		},
		{
			name: "valid with volumes",
			opts: &ContainerOptions{
				Name:    "test-container",
				Image:   "test:latest",
				Volumes: []string{"/data:/app/data", "config:/etc/config"},
			},
			expectErr: false,
		},
		{
			name: "valid with resource limits",
			opts: &ContainerOptions{
				Name:   "test-container",
				Image:  "test:latest",
				CPUs:   "0.5",
				Memory: "512M",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerOptions(tt.opts)
			if tt.expectErr && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

func TestDetectRuntime(t *testing.T) {
	// This test will always return a null runtime in test environment
	// since docker/podman are not expected to be available
	runtime, err := DetectRuntime()
	if err != nil {
		t.Fatalf("DetectRuntime should not return error: %v", err)
	}

	if runtime == nil {
		t.Fatal("DetectRuntime should return a runtime")
	}

	// Should return null runtime in test environment
	if runtime.GetRuntimeName() != "none" {
		t.Logf("Expected 'none' runtime in test environment, got %s", runtime.GetRuntimeName())
	}
}

func TestConvertConfigToContainerOptions(t *testing.T) {
	serverName := "test-server"
	serverCfg := config.ServerConfig{
		Image:       "test:latest",
		Command:     "/app/server",
		Args:        []string{"--port", "8080"},
		Env:         map[string]string{"CONFIG": "test"},
		Ports:       []string{"8080:8080"},
		Volumes:     []string{"/data:/app/data"},
		Networks:    []string{"test-network"},
		WorkDir:     "/app",
		User:        "1000:1000",
		Privileged:  false,
		CapAdd:      []string{"NET_BIND_SERVICE"},
		CapDrop:     []string{"ALL"},
		SecurityOpt: []string{"no-new-privileges:true"},
		Deploy: config.DeployConfig{
			Resources: config.ResourcesDeployConfig{
				Limits: config.ResourceLimitsConfig{
					CPUs:   "0.5",
					Memory: "512M",
					PIDs:   100,
				},
			},
		},
		HealthCheck: &config.HealthCheck{
			Test:     []string{"CMD", "curl", "-f", "http://localhost:8080/health"},
			Interval: "30s",
			Timeout:  "5s",
			Retries:  3,
		},
		Security: config.SecurityConfig{
			NoNewPrivileges: true,
			AppArmor:        "unconfined",
		},
	}

	opts := ConvertConfigToContainerOptions(serverName, serverCfg)

	// Test basic properties
	expectedName := "mcp-compose-test-server"
	if opts.Name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, opts.Name)
	}

	if opts.Image != serverCfg.Image {
		t.Errorf("Expected image %s, got %s", serverCfg.Image, opts.Image)
	}

	if opts.Command != serverCfg.Command {
		t.Errorf("Expected command %s, got %s", serverCfg.Command, opts.Command)
	}

	if !reflect.DeepEqual(opts.Args, serverCfg.Args) {
		t.Errorf("Expected args %v, got %v", serverCfg.Args, opts.Args)
	}

	// Test environment variables (should include MCP_SERVER_NAME)
	if opts.Env["MCP_SERVER_NAME"] != serverName {
		t.Errorf("Expected MCP_SERVER_NAME=%s, got %s", serverName, opts.Env["MCP_SERVER_NAME"])
	}

	if opts.Env["CONFIG"] != "test" {
		t.Errorf("Expected CONFIG=test, got %s", opts.Env["CONFIG"])
	}

	// Test security settings
	if opts.User != serverCfg.User {
		t.Errorf("Expected user %s, got %s", serverCfg.User, opts.User)
	}

	if opts.Privileged != serverCfg.Privileged {
		t.Errorf("Expected privileged %v, got %v", serverCfg.Privileged, opts.Privileged)
	}

	if !reflect.DeepEqual(opts.CapAdd, serverCfg.CapAdd) {
		t.Errorf("Expected CapAdd %v, got %v", serverCfg.CapAdd, opts.CapAdd)
	}

	if !reflect.DeepEqual(opts.CapDrop, serverCfg.CapDrop) {
		t.Errorf("Expected CapDrop %v, got %v", serverCfg.CapDrop, opts.CapDrop)
	}

	// Test resource limits
	if opts.CPUs != serverCfg.Deploy.Resources.Limits.CPUs {
		t.Errorf("Expected CPUs %s, got %s", serverCfg.Deploy.Resources.Limits.CPUs, opts.CPUs)
	}

	if opts.Memory != serverCfg.Deploy.Resources.Limits.Memory {
		t.Errorf("Expected Memory %s, got %s", serverCfg.Deploy.Resources.Limits.Memory, opts.Memory)
	}

	if opts.PidsLimit != serverCfg.Deploy.Resources.Limits.PIDs {
		t.Errorf("Expected PidsLimit %d, got %d", serverCfg.Deploy.Resources.Limits.PIDs, opts.PidsLimit)
	}

	// Test health check conversion
	if opts.HealthCheck == nil {
		t.Error("Expected health check to be converted")
	} else {
		if !reflect.DeepEqual(opts.HealthCheck.Test, serverCfg.HealthCheck.Test) {
			t.Errorf("Expected health check test %v, got %v", serverCfg.HealthCheck.Test, opts.HealthCheck.Test)
		}

		if opts.HealthCheck.Retries != serverCfg.HealthCheck.Retries {
			t.Errorf("Expected health check retries %d, got %d", serverCfg.HealthCheck.Retries, opts.HealthCheck.Retries)
		}
	}

	// Test security options (should include additional ones from config)
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
		t.Error("Expected no-new-privileges security option to be added")
	}

	if !hasAppArmor {
		t.Error("Expected apparmor security option to be added")
	}
}

func TestGetDefaultContainerOptions(t *testing.T) {
	defaults := GetDefaultContainerOptions()

	if defaults.RestartPolicy != "unless-stopped" {
		t.Errorf("Expected default restart policy 'unless-stopped', got %s", defaults.RestartPolicy)
	}

	if len(defaults.Networks) != 1 || defaults.Networks[0] != "mcp-net" {
		t.Errorf("Expected default networks ['mcp-net'], got %v", defaults.Networks)
	}

	if defaults.Security.AllowDockerSocket {
		t.Error("Expected default AllowDockerSocket to be false")
	}

	if defaults.Security.AllowPrivilegedOps {
		t.Error("Expected default AllowPrivilegedOps to be false")
	}

	if defaults.Security.TrustedImage {
		t.Error("Expected default TrustedImage to be false")
	}
}

func TestMergeContainerOptions(t *testing.T) {
	defaults := &ContainerOptions{
		RestartPolicy: "unless-stopped",
		Networks:      []string{"mcp-net"},
		Memory:        "512M",
	}

	opts := &ContainerOptions{
		Name:    "test-container",
		Image:   "test:latest",
		CPUs:    "0.5",
		Networks: []string{"custom-net"},
	}

	merged := MergeContainerOptions(opts, defaults)

	// Should keep values from opts
	if merged.Name != opts.Name {
		t.Errorf("Expected name %s, got %s", opts.Name, merged.Name)
	}

	if merged.Image != opts.Image {
		t.Errorf("Expected image %s, got %s", opts.Image, merged.Image)
	}

	if merged.CPUs != opts.CPUs {
		t.Errorf("Expected CPUs %s, got %s", opts.CPUs, merged.CPUs)
	}

	// Should use defaults where opts is empty
	if merged.RestartPolicy != defaults.RestartPolicy {
		t.Errorf("Expected restart policy %s, got %s", defaults.RestartPolicy, merged.RestartPolicy)
	}

	// Should use opts networks instead of defaults
	if !reflect.DeepEqual(merged.Networks, opts.Networks) {
		t.Errorf("Expected networks %v, got %v", opts.Networks, merged.Networks)
	}

	// Test with nil opts
	mergedNil := MergeContainerOptions(nil, defaults)
	if mergedNil != defaults {
		t.Error("Expected to return defaults when opts is nil")
	}
}

func TestIsContainerRunning(t *testing.T) {
	runtime := NewMockRuntime()

	// Test with non-existent container
	if IsContainerRunning(runtime, "non-existent") {
		t.Error("Expected false for non-existent container")
	}

	// Start a container
	opts := &ContainerOptions{
		Name:  "test-container",
		Image: "test:latest",
	}
	_, err := runtime.StartContainer(opts)
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test with running container
	if !IsContainerRunning(runtime, "test-container") {
		t.Error("Expected true for running container")
	}

	// Stop the container
	err = runtime.StopContainer("test-container")
	if err != nil {
		t.Fatalf("Failed to stop container: %v", err)
	}

	// Test with stopped container
	if IsContainerRunning(runtime, "test-container") {
		t.Error("Expected false for stopped container")
	}
}

func TestWaitForContainerReady(t *testing.T) {
	runtime := NewMockRuntime()

	// Test with non-existent container
	err := WaitForContainerReady(runtime, "non-existent", 10)
	if err == nil {
		t.Error("Expected error for non-existent container")
	}

	// Start a container
	opts := &ContainerOptions{
		Name:  "test-container",
		Image: "test:latest",
	}
	_, err = runtime.StartContainer(opts)
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test with existing container
	err = WaitForContainerReady(runtime, "test-container", 10)
	if err != nil {
		t.Errorf("Expected no error for existing container, got: %v", err)
	}
}

func TestMockRuntimeFailureScenarios(t *testing.T) {
	runtime := NewMockRuntime()

	// Test start failure
	runtime.shouldFailStart = true
	opts := &ContainerOptions{
		Name:  "test-container",
		Image: "test:latest",
	}
	_, err := runtime.StartContainer(opts)
	if err == nil {
		t.Error("Expected start failure")
	}

	// Reset failure flag
	runtime.shouldFailStart = false

	// Start container first
	_, err = runtime.StartContainer(opts)
	if err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	// Test stop failure
	runtime.shouldFailStop = true
	err = runtime.StopContainer("test-container")
	if err == nil {
		t.Error("Expected stop failure")
	}

	// Test network failure
	runtime.shouldFailNetwork = true
	err = runtime.CreateNetwork("test-network")
	if err == nil {
		t.Error("Expected network creation failure")
	}

	exists, err := runtime.NetworkExists("test-network")
	if err == nil {
		t.Error("Expected network exists check failure")
	}
	if exists {
		t.Error("Expected network to not exist")
	}
}

func TestValidationHelpers(t *testing.T) {
	// Test port mapping validation
	tests := []struct {
		port      string
		expectErr bool
	}{
		{"8080:8080", false},
		{"80:8080", false},
		{"", true},
	}

	for _, tt := range tests {
		err := validatePortMapping(tt.port)
		if tt.expectErr && err == nil {
			t.Errorf("Expected error for port mapping %q", tt.port)
		}
		if !tt.expectErr && err != nil {
			t.Errorf("Expected no error for port mapping %q, got: %v", tt.port, err)
		}
	}

	// Test volume mapping validation
	volumeTests := []struct {
		volume    string
		expectErr bool
	}{
		{"/data:/app/data", false},
		{"volume:/app/data", false},
		{"", true},
	}

	for _, tt := range volumeTests {
		err := validateVolumeMapping(tt.volume)
		if tt.expectErr && err == nil {
			t.Errorf("Expected error for volume mapping %q", tt.volume)
		}
		if !tt.expectErr && err != nil {
			t.Errorf("Expected no error for volume mapping %q, got: %v", tt.volume, err)
		}
	}

	// Test CPU limit validation
	err := validateCPULimit("0.5")
	if err != nil {
		t.Errorf("Expected no error for CPU limit validation, got: %v", err)
	}

	err = validateCPULimit("")
	if err != nil {
		t.Errorf("Expected no error for empty CPU limit, got: %v", err)
	}

	// Test memory limit validation
	err = validateMemoryLimit("512M")
	if err != nil {
		t.Errorf("Expected no error for memory limit validation, got: %v", err)
	}

	err = validateMemoryLimit("")
	if err != nil {
		t.Errorf("Expected no error for empty memory limit, got: %v", err)
	}
}