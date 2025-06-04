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
	Build       config.BuildConfig // Add this

}

// Runtime defines the interface for container runtimes
type Runtime interface {
	// StartContainer starts a container with the given options
	StartContainer(opts *ContainerOptions) (string, error)
	// StopContainer stops a container
	StopContainer(name string) error
	// GetContainerStatus returns the status of a container
	GetContainerStatus(name string) (string, error)
	// ShowContainerLogs shows logs for a container
	ShowContainerLogs(name string, follow bool) error
	// GetRuntimeName returns the name of the runtime
	GetRuntimeName() string
	// NetworkExists checks if a network exists
	NetworkExists(name string) (bool, error)
	// CreateNetwork creates a network
	CreateNetwork(name string) error
	// ExecContainer executes a command in a running container
	ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error)
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
