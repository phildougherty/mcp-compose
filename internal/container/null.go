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

func (n *NullRuntime) RestartContainer(name string) error {
	return fmt.Errorf("no container runtime available, cannot restart container '%s'", name)
}

func (n *NullRuntime) PauseContainer(name string) error {
	return fmt.Errorf("no container runtime available, cannot pause container '%s'", name)
}

func (n *NullRuntime) UnpauseContainer(name string) error {
	return fmt.Errorf("no container runtime available, cannot unpause container '%s'", name)
}

func (n *NullRuntime) GetContainerInfo(name string) (*ContainerInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot get info for container '%s'", name)
}

func (n *NullRuntime) ListContainers(filters map[string]string) ([]ContainerInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot list containers")
}

func (n *NullRuntime) PullImage(image string, auth *ImageAuth) error {
	return fmt.Errorf("no container runtime available, cannot pull image '%s'", image)
}

func (n *NullRuntime) BuildImage(opts *BuildOptions) error {
	return fmt.Errorf("no container runtime available, cannot build image")
}

func (n *NullRuntime) RemoveImage(image string, force bool) error {
	return fmt.Errorf("no container runtime available, cannot remove image '%s'", image)
}

func (n *NullRuntime) ListImages() ([]ImageInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot list images")
}

func (n *NullRuntime) CreateVolume(name string, opts *VolumeOptions) error {
	return fmt.Errorf("no container runtime available, cannot create volume '%s'", name)
}

func (n *NullRuntime) RemoveVolume(name string, force bool) error {
	return fmt.Errorf("no container runtime available, cannot remove volume '%s'", name)
}

func (n *NullRuntime) ListVolumes() ([]VolumeInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot list volumes")
}

func (n *NullRuntime) ListNetworks() ([]NetworkInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot list networks")
}

func (n *NullRuntime) GetNetworkInfo(name string) (*NetworkInfo, error) {
	return nil, fmt.Errorf("no container runtime available, cannot get network info for '%s'", name)
}

func (n *NullRuntime) ConnectToNetwork(containerName, networkName string) error {
	return fmt.Errorf("no container runtime available, cannot connect container '%s' to network '%s'", containerName, networkName)
}

func (n *NullRuntime) DisconnectFromNetwork(containerName, networkName string) error {
	return fmt.Errorf("no container runtime available, cannot disconnect container '%s' from network '%s'", containerName, networkName)
}

func (n *NullRuntime) GetContainerStats(name string) (*ContainerStats, error) {
	return nil, fmt.Errorf("no container runtime available, cannot get stats for container '%s'", name)
}

func (n *NullRuntime) WaitForContainer(name string, condition string) error {
	return fmt.Errorf("no container runtime available, cannot wait for container '%s'", name)
}

func (n *NullRuntime) ValidateSecurityContext(opts *ContainerOptions) error {
	return fmt.Errorf("no container runtime available, cannot validate security context")
}

func (n *NullRuntime) UpdateContainerResources(name string, resources *ResourceLimits) error {
	return fmt.Errorf("no container runtime available, cannot update resources for container '%s'", name)
}
