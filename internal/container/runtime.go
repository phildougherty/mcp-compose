// internal/container/runtime.go
package container

import (
	"fmt"
	"io"
	"mcpcompose/internal/config"
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

	// Resource limits
	CPUs       string `yaml:"cpus,omitempty"`
	Memory     string `yaml:"memory,omitempty"`
	MemorySwap string `yaml:"memory_swap,omitempty"`
	PidsLimit  int    `yaml:"pids_limit,omitempty"`

	// Security context
	User        string   `yaml:"user,omitempty"`
	Groups      []string `yaml:"groups,omitempty"`
	Privileged  bool     `yaml:"privileged,omitempty"`
	CapAdd      []string `yaml:"cap_add,omitempty"`
	CapDrop     []string `yaml:"cap_drop,omitempty"`
	SecurityOpt []string `yaml:"security_opt,omitempty"`
	ReadOnly    bool     `yaml:"read_only,omitempty"`
	Tmpfs       []string `yaml:"tmpfs,omitempty"`

	// Lifecycle
	RestartPolicy string       `yaml:"restart,omitempty"`
	StopSignal    string       `yaml:"stop_signal,omitempty"`
	StopTimeout   *int         `yaml:"stop_grace_period,omitempty"`
	HealthCheck   *HealthCheck `yaml:"healthcheck,omitempty"`
	DependsOn     []string     `yaml:"depends_on,omitempty"`

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
}

type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

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

type ImageInfo struct {
	ID      string            `json:"id"`
	Tags    []string          `json:"tags"`
	Size    int64             `json:"size"`
	Created string            `json:"created"`
	Labels  map[string]string `json:"labels"`
}

type VolumeInfo struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	Labels     map[string]string `json:"labels"`
	Options    map[string]string `json:"options"`
	Scope      string            `json:"scope"`
}

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

type NetworkEndpoint struct {
	EndpointID  string `json:"endpoint_id"`
	MacAddress  string `json:"mac_address"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
}

type PortBinding struct {
	PrivatePort int    `json:"private_port"`
	PublicPort  int    `json:"public_port"`
	Type        string `json:"type"`
	IP          string `json:"ip"`
}

type MountInfo struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Mode        string `json:"mode"`
	RW          bool   `json:"rw"`
}

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

type ImageAuth struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Registry string `json:"registry"`
}

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

type VolumeOptions struct {
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driver_opts"`
	Labels     map[string]string `json:"labels"`
}

type ResourceLimits struct {
	CPUs        string `json:"cpus"`
	Memory      string `json:"memory"`
	PidsLimit   int    `json:"pids_limit"`
	BlkioWeight int    `json:"blkio_weight"`
}

// Runtime defines the interface for container runtimes
type Runtime interface {
	StartContainer(opts *ContainerOptions) (string, error)
	StopContainer(name string) error
	GetContainerStatus(name string) (string, error)
	ShowContainerLogs(name string, follow bool) error
	GetRuntimeName() string
	NetworkExists(name string) (bool, error)
	CreateNetwork(name string) error
	ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error)

	// Container lifecycle
	RestartContainer(name string) error
	PauseContainer(name string) error
	UnpauseContainer(name string) error

	// Container inspection
	GetContainerInfo(name string) (*ContainerInfo, error)
	ListContainers(filters map[string]string) ([]ContainerInfo, error)

	// Image management
	PullImage(image string, auth *ImageAuth) error
	BuildImage(opts *BuildOptions) error
	RemoveImage(image string, force bool) error
	ListImages() ([]ImageInfo, error)

	// Volume management
	CreateVolume(name string, opts *VolumeOptions) error
	RemoveVolume(name string, force bool) error
	ListVolumes() ([]VolumeInfo, error)

	// Network management (enhanced)
	RemoveNetwork(name string) error
	ListNetworks() ([]NetworkInfo, error)
	GetNetworkInfo(name string) (*NetworkInfo, error)
	ConnectToNetwork(containerName, networkName string) error
	DisconnectFromNetwork(containerName, networkName string) error

	// Health and monitoring
	GetContainerStats(name string) (*ContainerStats, error)
	WaitForContainer(name string, condition string) error

	// Security
	ValidateSecurityContext(opts *ContainerOptions) error

	// Resource management
	UpdateContainerResources(name string, resources *ResourceLimits) error
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
