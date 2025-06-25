// internal/container/null.go
package container

import (
	"fmt"
	"io"
	"os/exec"
)

// NullRuntime is a fallback when no container runtime is available
type NullRuntime struct{}

// NewNullRuntime creates a null runtime that can't run containers
func NewNullRuntime() Runtime {
	return &NullRuntime{}
}

func (n *NullRuntime) RemoveNetwork(name string) error {
	return fmt.Errorf("no runtime available for network operations")
}

func (n *NullRuntime) GetRuntimeName() string {
	return "none"
}

func (n *NullRuntime) StartContainer(opts *ContainerOptions) (string, error) {
	return "", fmt.Errorf("no container runtime available, cannot start container with image '%s'", opts.Image)
}

func (n *NullRuntime) StopContainer(name string) error {
	return fmt.Errorf("no container runtime available, cannot stop container '%s'", name)
}

func (n *NullRuntime) GetContainerStatus(name string) (string, error) {
	return "unknown", fmt.Errorf("no container runtime available")
}

func (n *NullRuntime) ShowContainerLogs(name string, follow bool) error {
	return fmt.Errorf("no container runtime available, cannot show logs for container '%s'", name)
}

func (n *NullRuntime) NetworkExists(name string) (bool, error) {
	return false, fmt.Errorf("no container runtime available, cannot check network '%s'", name)
}

func (n *NullRuntime) CreateNetwork(name string) error {
	return fmt.Errorf("no container runtime available, cannot create network '%s'", name)
}

// ExecContainer executes a command in a running container
func (n *NullRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	return nil, nil, nil, fmt.Errorf("no container runtime available, cannot execute command in container '%s'", containerName)
}
