// internal/container/runtime.go
package container

import (
	"fmt"
	"io"
	"github.com/phildougherty/mcp-compose/internal/config"
	"os/exec"
)

// ContainerOptions holds container creation options
type ContainerOptions struct {
	Name        string
	Image       string
	Command     string
	Args        []string
	Env         map[string]string
	Ports       []string
	Volumes     []string
	WorkDir     string
	Pull        bool
	NetworkMode string
	Networks    []string
	Build       config.BuildConfig

	// Security context
	Privileged  bool     `yaml:"privileged,omitempty"`
	User        string   `yaml:"user,omitempty"`
	Groups      []string `yaml:"groups,omitempty"`
	CapAdd      []string `yaml:"cap_add,omitempty"`
	CapDrop     []string `yaml:"cap_drop,omitempty"`
	SecurityOpt []string `yaml:"security_opt,omitempty"`
	ReadOnly    bool     `yaml:"read_only,omitempty"`
	Tmpfs       []string `yaml:"tmpfs,omitempty"`

	// Resource limits
	CPUs       string `yaml:"cpus,omitempty"`
	Memory     string `yaml:"memory,omitempty"`
	MemorySwap string `yaml:"memory_swap,omitempty"`
	PidsLimit  int    `yaml:"pids_limit,omitempty"`

	// Lifecycle
	RestartPolicy string       `yaml:"restart,omitempty"`
	StopSignal    string       `yaml:"stop_signal,omitempty"`
	StopTimeout   *int         `yaml:"stop_grace_period,omitempty"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty"`

	// Runtime options
	Runtime    string   `yaml:"runtime,omitempty"`
	Platform   string   `yaml:"platform,omitempty"`
	Hostname   string   `yaml:"hostname,omitempty"`
	DomainName string   `yaml:"domainname,omitempty"`
	DNS        []string `yaml:"dns,omitempty"`
	DNSSearch  []string `yaml:"dns_search,omitempty"`
	ExtraHosts []string `yaml:"extra_hosts,omitempty"`

	// Logging
	LogDriver  string            `yaml:"log_driver,omitempty"`
	LogOptions map[string]string `yaml:"log_options,omitempty"`

	// Labels and metadata
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`

	// Security configuration for validation
	Security SecurityConfig `yaml:"security,omitempty"`
}

// HealthCheck defines health check configuration
type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

// SecurityConfig for container validation
type SecurityConfig struct {
	AllowDockerSocket  bool     `yaml:"allow_docker_socket,omitempty"`
	AllowHostMounts    []string `yaml:"allow_host_mounts,omitempty"`
	AllowPrivilegedOps bool     `yaml:"allow_privileged_ops,omitempty"`
	TrustedImage       bool     `yaml:"trusted_image,omitempty"`
}

// ContainerInfo represents detailed container information
type ContainerInfo struct {
	ID           string                     `json:"id"`
	Name         string                     `json:"name"`
	Image        string                     `json:"image"`
	Status       string                     `json:"status"`
	State        string                     `json:"state"`
	Created      string                     `json:"created"`
	Ports        []PortBinding              `json:"ports"`
	Mounts       []MountInfo                `json:"mounts"`
	Networks     map[string]NetworkEndpoint `json:"networks"`
	Labels       map[string]string          `json:"labels"`
	Env          []string                   `json:"env"`
	Command      []string                   `json:"command"`
	RestartCount int                        `json:"restart_count"`
}

// ImageInfo represents image information
type ImageInfo struct {
	ID      string            `json:"id"`
	Tags    []string          `json:"tags"`
	Size    int64             `json:"size"`
	Created string            `json:"created"`
	Labels  map[string]string `json:"labels"`
}

// VolumeInfo represents volume information
type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	Labels     map[string]string `json:"labels"`
	Options    map[string]string `json:"options"`
	Scope      string            `json:"scope"`
}

// NetworkInfo represents network information
type NetworkInfo struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Driver     string                     `json:"driver"`
	Scope      string                     `json:"scope"`
	Internal   bool                       `json:"internal"`
	Attachable bool                       `json:"attachable"`
	Containers map[string]NetworkEndpoint `json:"containers"`
	Options    map[string]string          `json:"options"`
	Labels     map[string]string          `json:"labels"`
}

// NetworkEndpoint represents a network endpoint
type NetworkEndpoint struct {
	EndpointID  string `json:"endpoint_id"`
	MacAddress  string `json:"mac_address"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
}

// PortBinding represents a port binding
type PortBinding struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port"`
	Type        string `json:"type"`
	IP          string `json:"ip"`
}

// MountInfo represents mount information
type MountInfo struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

// ContainerStats represents container statistics
type ContainerStats struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage int64   `json:"memory_usage"`
	MemoryLimit int64   `json:"memory_limit"`
	NetworkIO   struct {
		RxBytes int64 `json:"rx_bytes"`
		TxBytes int64 `json:"tx_bytes"`
	} `json:"network_io"`
	BlockIO struct {
		ReadBytes  int64 `json:"read_bytes"`
		WriteBytes int64 `json:"write_bytes"`
	} `json:"block_io"`
}

// ImageAuth represents image authentication credentials
type ImageAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Registry string `json:"registry"`
}

// BuildOptions represents build configuration options
type BuildOptions struct {
	Context    string            `json:"context"`
	Dockerfile string            `json:"dockerfile"`
	Tags       []string          `json:"tags"`
	Args       map[string]string `json:"args"`
	Target     string            `json:"target"`
	NoCache    bool              `json:"no_cache"`
	Pull       bool              `json:"pull"`
	Platform   string            `json:"platform"`
}

// VolumeOptions represents volume creation options
type VolumeOptions struct {
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driver_opts"`
	Labels     map[string]string `json:"labels"`
}

// ResourceLimits represents resource limits for container updates
type ResourceLimits struct {
	CPUs        string `json:"cpus"`
	Memory      string `json:"memory"`
	PidsLimit   int    `json:"pids_limit"`
	BlkioWeight int    `json:"blkio_weight"`
}

// Runtime defines the interface for container runtimes
type Runtime interface {
	// Container lifecycle management
	StartContainer(opts *ContainerOptions) (string, error)
	StopContainer(name string) error
	RestartContainer(name string) error
	PauseContainer(name string) error
	UnpauseContainer(name string) error

	// Container inspection and monitoring
	GetContainerStatus(name string) (string, error)
	GetContainerInfo(name string) (*ContainerInfo, error)
	ListContainers(filters map[string]string) ([]ContainerInfo, error)
	GetContainerStats(name string) (*ContainerStats, error)
	WaitForContainer(name string, condition string) error

	// Container logs and execution
	ShowContainerLogs(name string, follow bool) error
	ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error)

	// Image management
	PullImage(image string, auth *ImageAuth) error
	BuildImage(opts *BuildOptions) error
	RemoveImage(image string, force bool) error
	ListImages() ([]ImageInfo, error)

	// Volume management
	CreateVolume(name string, opts *VolumeOptions) error
	RemoveVolume(name string, force bool) error
	ListVolumes() ([]VolumeInfo, error)

	// Network management
	NetworkExists(name string) (bool, error)
	CreateNetwork(name string) error
	RemoveNetwork(name string) error
	ListNetworks() ([]NetworkInfo, error)
	GetNetworkInfo(name string) (*NetworkInfo, error)
	ConnectToNetwork(containerName, networkName string) error
	DisconnectFromNetwork(containerName, networkName string) error

	// Resource management
	UpdateContainerResources(name string, resources *ResourceLimits) error

	// Security and validation
	ValidateSecurityContext(opts *ContainerOptions) error

	// Runtime information
	GetRuntimeName() string
}

// DetectRuntime tries to detect and initialize a container runtime
func DetectRuntime() (Runtime, error) {
	// Try Docker first
	dockerPath, err := exec.LookPath("docker")
	if err == nil {
		fmt.Println("Detected Docker runtime")

		return NewDockerRuntime(dockerPath)
	}

	// Try Podman next
	podmanPath, err := exec.LookPath("podman")
	if err == nil {
		fmt.Println("Detected Podman runtime")

		return NewPodmanRuntime(podmanPath)
	}

	// Return a null runtime that can only handle process-based servers
	fmt.Println("No container runtime detected, only process-based servers will be supported")

	return NewNullRuntime(), nil
}

// ValidateContainerOptions performs basic validation on container options
func ValidateContainerOptions(opts *ContainerOptions) error {
	if opts.Name == "" {

		return fmt.Errorf("container name cannot be empty")
	}

	if opts.Image == "" && opts.Build.Context == "" {

		return fmt.Errorf("container must specify either image or build context")
	}

	// Validate port mappings
	for _, port := range opts.Ports {
		if err := validatePortMapping(port); err != nil {

			return fmt.Errorf("invalid port mapping '%s': %w", port, err)
		}
	}

	// Validate volume mappings
	for _, volume := range opts.Volumes {
		if err := validateVolumeMapping(volume); err != nil {

			return fmt.Errorf("invalid volume mapping '%s': %w", volume, err)
		}
	}

	// Validate resource limits
	if opts.CPUs != "" {
		if err := validateCPULimit(opts.CPUs); err != nil {

			return fmt.Errorf("invalid CPU limit '%s': %w", opts.CPUs, err)
		}
	}

	if opts.Memory != "" {
		if err := validateMemoryLimit(opts.Memory); err != nil {

			return fmt.Errorf("invalid memory limit '%s': %w", opts.Memory, err)
		}
	}

	return nil
}

// Helper validation functions
func validatePortMapping(portMapping string) error {
	if portMapping == "" {

		return fmt.Errorf("empty port mapping")
	}
	// This can be enhanced with more sophisticated port mapping validation

	return nil
}

func validateVolumeMapping(volumeMapping string) error {
	if volumeMapping == "" {

		return fmt.Errorf("empty volume mapping")
	}
	// This can be enhanced with more sophisticated volume mapping validation

	return nil
}

func validateCPULimit(cpu string) error {
	if cpu == "" {

		return nil
	}
	// This can be enhanced with CPU limit validation

	return nil
}

func validateMemoryLimit(memory string) error {
	if memory == "" {

		return nil
	}
	// This can be enhanced with memory limit validation

	return nil
}

// ConvertConfigToContainerOptions converts server config to container options
func ConvertConfigToContainerOptions(serverName string, serverCfg config.ServerConfig) *ContainerOptions {
	opts := &ContainerOptions{
		Name:        fmt.Sprintf("mcp-compose-%s", serverName),
		Image:       serverCfg.Image,
		Build:       serverCfg.Build,
		Command:     serverCfg.Command,
		Args:        serverCfg.Args,
		Env:         config.MergeEnv(serverCfg.Env, map[string]string{"MCP_SERVER_NAME": serverName}),
		Pull:        serverCfg.Pull,
		Volumes:     serverCfg.Volumes,
		Ports:       serverCfg.Ports,
		Networks:    serverCfg.Networks,
		WorkDir:     serverCfg.WorkDir,
		NetworkMode: serverCfg.NetworkMode,

		// Security configuration
		Privileged:  serverCfg.Privileged,
		User:        serverCfg.User,
		Groups:      serverCfg.Groups,
		ReadOnly:    serverCfg.ReadOnly,
		Tmpfs:       serverCfg.Tmpfs,
		CapAdd:      serverCfg.CapAdd,
		CapDrop:     serverCfg.CapDrop,
		SecurityOpt: serverCfg.SecurityOpt,

		// Resource limits
		PidsLimit: serverCfg.Deploy.Resources.Limits.PIDs,

		// Lifecycle
		RestartPolicy: serverCfg.RestartPolicy,
		StopSignal:    serverCfg.StopSignal,
		StopTimeout:   serverCfg.StopTimeout,

		// Runtime options
		Runtime:    serverCfg.Runtime,
		Platform:   serverCfg.Platform,
		Hostname:   serverCfg.Hostname,
		DomainName: serverCfg.DomainName,
		DNS:        serverCfg.DNS,
		DNSSearch:  serverCfg.DNSSearch,
		ExtraHosts: serverCfg.ExtraHosts,

		// Logging
		LogDriver:  serverCfg.LogDriver,
		LogOptions: serverCfg.LogOptions,

		// Labels and metadata
		Labels:      serverCfg.Labels,
		Annotations: serverCfg.Annotations,

		// Security config for validation
		Security: SecurityConfig{
			AllowDockerSocket:  serverCfg.Security.AllowDockerSocket,
			AllowHostMounts:    serverCfg.Security.AllowHostMounts,
			AllowPrivilegedOps: serverCfg.Security.AllowPrivilegedOps,
			TrustedImage:       serverCfg.Security.TrustedImage,
		},
	}

	// Convert resource limits
	if serverCfg.Deploy.Resources.Limits.CPUs != "" {
		opts.CPUs = serverCfg.Deploy.Resources.Limits.CPUs
	}
	if serverCfg.Deploy.Resources.Limits.Memory != "" {
		opts.Memory = serverCfg.Deploy.Resources.Limits.Memory
	}
	if serverCfg.Deploy.Resources.Limits.MemorySwap != "" {
		opts.MemorySwap = serverCfg.Deploy.Resources.Limits.MemorySwap
	}

	// Convert health check
	if serverCfg.HealthCheck != nil {
		opts.HealthCheck = &HealthCheck{
			Test:        serverCfg.HealthCheck.Test,
			Interval:    serverCfg.HealthCheck.Interval,
			Timeout:     serverCfg.HealthCheck.Timeout,
			Retries:     serverCfg.HealthCheck.Retries,
			StartPeriod: serverCfg.HealthCheck.StartPeriod,
		}
	}

	// Add security options based on configuration
	if serverCfg.Security.NoNewPrivileges {
		opts.SecurityOpt = append(opts.SecurityOpt, "no-new-privileges:true")
	}

	if serverCfg.Security.AppArmor != "" {
		opts.SecurityOpt = append(opts.SecurityOpt, fmt.Sprintf("apparmor:%s", serverCfg.Security.AppArmor))
	}

	if serverCfg.Security.Seccomp != "" {
		opts.SecurityOpt = append(opts.SecurityOpt, fmt.Sprintf("seccomp:%s", serverCfg.Security.Seccomp))
	}

	return opts
}

// GetDefaultContainerOptions returns default container options
func GetDefaultContainerOptions() *ContainerOptions {

	return &ContainerOptions{
		RestartPolicy: "unless-stopped",
		Networks:      []string{"mcp-net"},
		Security: SecurityConfig{
			AllowDockerSocket:  false,
			AllowPrivilegedOps: false,
			TrustedImage:       false,
		},
	}
}

// MergeContainerOptions merges container options with defaults
func MergeContainerOptions(opts, defaults *ContainerOptions) *ContainerOptions {
	if opts == nil {

		return defaults
	}

	merged := *opts // Copy the struct

	// Apply defaults where values are empty
	if merged.RestartPolicy == "" {
		merged.RestartPolicy = defaults.RestartPolicy
	}

	if len(merged.Networks) == 0 {
		merged.Networks = defaults.Networks
	}

	return &merged
}

// IsContainerRunning checks if a container is currently running
func IsContainerRunning(runtime Runtime, containerName string) bool {
	status, err := runtime.GetContainerStatus(containerName)
	if err != nil {

		return false
	}

	return status == "running"
}

// WaitForContainerReady waits for a container to be ready (running and healthy)
func WaitForContainerReady(runtime Runtime, containerName string, maxWait int) error {
	// This can be enhanced with more sophisticated readiness checking

	return runtime.WaitForContainer(containerName, "running")
}
