// internal/container/podman.go
package container

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// PodmanRuntime implements container runtime using Podman
type PodmanRuntime struct {
	execPath string
}

// NewPodmanRuntime creates a Podman runtime
func NewPodmanRuntime(path string) (Runtime, error) {

	return &PodmanRuntime{execPath: path}, nil
}

func (p *PodmanRuntime) RemoveNetwork(name string) error {
	cmd := exec.Command("podman", "network", "rm", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if network doesn't exist (not an error for cleanup)
		if strings.Contains(string(output), "not found") {

			return nil
		}

		return fmt.Errorf("failed to remove network %s: %w. Output: %s", name, err, string(output))
	}

	return nil
}

func (p *PodmanRuntime) GetRuntimeName() string {

	return "podman"
}

func (p *PodmanRuntime) StartContainer(opts *ContainerOptions) (string, error) {
	// Check if container with this name already exists
	cmd := exec.Command(p.execPath, "inspect", "--type=container", opts.Name)
	if err := cmd.Run(); err == nil {
		// Container exists, remove it
		rmCmd := exec.Command(p.execPath, "rm", "-f", opts.Name)
		if err := rmCmd.Run(); err != nil {

			return "", fmt.Errorf("failed to remove existing container: %w", err)
		}
	}
	// Pull image if requested
	if opts.Pull {
		fmt.Printf("Pulling image '%s'...\n", opts.Image)
		pullCmd := exec.Command(p.execPath, "pull", opts.Image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {

			return "", fmt.Errorf("failed to pull image: %w", err)
		}
	}
	// Prepare podman run command
	args := []string{"run", "-d", "--name", opts.Name}
	// Add environment variables
	for k, v := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	// Add ports
	for _, p := range opts.Ports {
		args = append(args, "-p", p)
	}
	// Add volumes
	for _, v := range opts.Volumes {
		args = append(args, "-v", v)
	}
	// Set working directory
	if opts.WorkDir != "" {
		args = append(args, "-w", opts.WorkDir)
	}
	// Add network mode if specified
	if opts.NetworkMode != "" {
		args = append(args, "--network", opts.NetworkMode)
	}
	// Add networks
	for _, network := range opts.Networks {
		// Ensure network exists
		networkExists, _ := p.NetworkExists(network)
		if !networkExists {
			// Create the network
			if err := p.CreateNetwork(network); err != nil {

				return "", err
			}
		}
		if opts.NetworkMode == "" { // Only add --network if not using special network mode
			args = append(args, "--network", network)
		}
	}
	// Add image
	args = append(args, opts.Image)
	// Add command and arguments if specified
	if opts.Command != "" {
		args = append(args, opts.Command)
		if len(opts.Args) > 0 {
			args = append(args, opts.Args...)
		}
	}
	// Execute podman run - fixed := to =
	cmd = exec.Command(p.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return "", fmt.Errorf("failed to start container: %w, %s", err, string(output))
	}
	// Get the container ID from output
	containerID := strings.TrimSpace(string(output))

	return containerID, nil
}

func (p *PodmanRuntime) StopContainer(name string) error {
	// Check if container exists
	cmd := exec.Command(p.execPath, "inspect", "--type=container", name)
	if err := cmd.Run(); err != nil {
		// Container doesn't exist, nothing to do

		return nil
	}
	// Stop the container
	cmd = exec.Command(p.execPath, "stop", name)
	if err := cmd.Run(); err != nil {

		return fmt.Errorf("failed to stop container: %w", err)
	}
	// Remove the container
	cmd = exec.Command(p.execPath, "rm", "-f", name)
	if err := cmd.Run(); err != nil {

		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (p *PodmanRuntime) GetContainerStatus(name string) (string, error) {
	cmd := exec.Command(p.execPath, "inspect", "--format", "{{.State.Status}}", name)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return "stopped", nil
	}
	status := strings.TrimSpace(string(output))
	// Map Podman-specific statuses to running/stopped
	switch status {
	case "running":

		return "running", nil
	default:

		return "stopped", nil
	}
}

func (p *PodmanRuntime) ShowContainerLogs(name string, follow bool) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, name)
	cmd := exec.Command(p.execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (p *PodmanRuntime) NetworkExists(name string) (bool, error) {
	cmd := exec.Command(p.execPath, "network", "inspect", name)
	err := cmd.Run()

	return err == nil, nil
}

func (p *PodmanRuntime) CreateNetwork(name string) error {
	cmd := exec.Command(p.execPath, "network", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return fmt.Errorf("failed to create network '%s': %w, %s", name, err, string(output))
	}

	return nil
}

func (p *PodmanRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	args := []string{"exec"}
	if interactive {
		args = append(args, "-i")
	}
	args = append(args, containerName)
	args = append(args, command...)

	cmd := exec.Command(p.execPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {

		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		if closeErr := stdin.Close(); closeErr != nil {

			return nil, nil, nil, fmt.Errorf("failed to create stdout pipe and close stdin: %v, close error: %w", err, closeErr)
		}

		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		if closeErr := stdin.Close(); closeErr != nil {
			if closeErr2 := stdout.Close(); closeErr2 != nil {

				return nil, nil, nil, fmt.Errorf("failed to start command and close pipes: %v, stdin close error: %v, stdout close error: %w", err, closeErr, closeErr2)
			}

			return nil, nil, nil, fmt.Errorf("failed to start command and close stdin: %v, close error: %w", err, closeErr)
		}
		if closeErr := stdout.Close(); closeErr != nil {

			return nil, nil, nil, fmt.Errorf("failed to start command and close stdout: %v, close error: %w", err, closeErr)
		}

		return nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return cmd, stdin, stdout, nil
}

func (p *PodmanRuntime) RestartContainer(name string) error {
	cmd := exec.Command(p.execPath, "restart", name)

	return cmd.Run()
}

func (p *PodmanRuntime) PauseContainer(name string) error {
	cmd := exec.Command(p.execPath, "pause", name)

	return cmd.Run()
}

func (p *PodmanRuntime) UnpauseContainer(name string) error {
	cmd := exec.Command(p.execPath, "unpause", name)

	return cmd.Run()
}

func (p *PodmanRuntime) GetContainerInfo(name string) (*ContainerInfo, error) {
	cmd := exec.Command(p.execPath, "inspect", name)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to inspect container '%s': %w", name, err)
	}

	var containers []ContainerInfo
	if err := json.Unmarshal(output, &containers); err != nil {

		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	if len(containers) == 0 {

		return nil, fmt.Errorf("container '%s' not found", name)
	}

	return &containers[0], nil
}

func (p *PodmanRuntime) ListContainers(filters map[string]string) ([]ContainerInfo, error) {
	args := []string{"ps", "-a", "--format", "json"}

	for key, value := range filters {
		args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command(p.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var containers []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {

			continue
		}
		var container ContainerInfo
		if err := json.Unmarshal([]byte(line), &container); err != nil {

			continue
		}
		containers = append(containers, container)
	}

	return containers, nil
}

func (p *PodmanRuntime) PullImage(image string, auth *ImageAuth) error {
	args := []string{"pull"}
	if auth != nil {
		args = append(args, "--username", auth.Username, "--password", auth.Password)
	}
	args = append(args, image)

	cmd := exec.Command(p.execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (p *PodmanRuntime) BuildImage(opts *BuildOptions) error {
	args := []string{"build"}

	if opts.Dockerfile != "" {
		args = append(args, "-f", opts.Dockerfile)
	}

	for _, tag := range opts.Tags {
		args = append(args, "-t", tag)
	}

	for key, value := range opts.Args {
		args = append(args, "--build-arg", fmt.Sprintf("%s=%s", key, value))
	}

	if opts.Target != "" {
		args = append(args, "--target", opts.Target)
	}

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	if opts.Platform != "" {
		args = append(args, "--platform", opts.Platform)
	}

	args = append(args, opts.Context)

	cmd := exec.Command(p.execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (p *PodmanRuntime) RemoveImage(image string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, image)

	cmd := exec.Command(p.execPath, args...)

	return cmd.Run()
}

func (p *PodmanRuntime) ListImages() ([]ImageInfo, error) {
	cmd := exec.Command(p.execPath, "images", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var images []ImageInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {

			continue
		}
		var image ImageInfo
		if err := json.Unmarshal([]byte(line), &image); err != nil {

			continue
		}
		images = append(images, image)
	}

	return images, nil
}

func (p *PodmanRuntime) CreateVolume(name string, opts *VolumeOptions) error {
	args := []string{"volume", "create"}

	if opts != nil {
		if opts.Driver != "" {
			args = append(args, "--driver", opts.Driver)
		}

		for key, value := range opts.Labels {
			args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
		}
	}

	args = append(args, name)

	cmd := exec.Command(p.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "already exists") {

			return nil
		}

		return fmt.Errorf("failed to create volume '%s': %w", name, err)
	}

	return nil
}

func (p *PodmanRuntime) RemoveVolume(name string, force bool) error {
	args := []string{"volume", "rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.Command(p.execPath, args...)

	return cmd.Run()
}

func (p *PodmanRuntime) ListVolumes() ([]VolumeInfo, error) {
	cmd := exec.Command(p.execPath, "volume", "ls", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var volumes []VolumeInfo
	if err := json.Unmarshal(output, &volumes); err != nil {

		return nil, fmt.Errorf("failed to parse volumes: %w", err)
	}

	return volumes, nil
}

func (p *PodmanRuntime) ListNetworks() ([]NetworkInfo, error) {
	cmd := exec.Command(p.execPath, "network", "ls", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var networks []NetworkInfo
	if err := json.Unmarshal(output, &networks); err != nil {

		return nil, fmt.Errorf("failed to parse networks: %w", err)
	}

	return networks, nil
}

func (p *PodmanRuntime) GetNetworkInfo(name string) (*NetworkInfo, error) {
	cmd := exec.Command(p.execPath, "network", "inspect", name)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to inspect network '%s': %w", name, err)
	}

	var networks []NetworkInfo
	if err := json.Unmarshal(output, &networks); err != nil {

		return nil, fmt.Errorf("failed to parse network info: %w", err)
	}

	if len(networks) == 0 {

		return nil, fmt.Errorf("network '%s' not found", name)
	}

	return &networks[0], nil
}

func (p *PodmanRuntime) ConnectToNetwork(containerName, networkName string) error {
	cmd := exec.Command(p.execPath, "network", "connect", networkName, containerName)

	return cmd.Run()
}

func (p *PodmanRuntime) DisconnectFromNetwork(containerName, networkName string) error {
	cmd := exec.Command(p.execPath, "network", "disconnect", networkName, containerName)

	return cmd.Run()
}

func (p *PodmanRuntime) GetContainerStats(name string) (*ContainerStats, error) {
	cmd := exec.Command(p.execPath, "stats", "--no-stream", "--format", "json", name)
	output, err := cmd.CombinedOutput()
	if err != nil {

		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	var stats ContainerStats
	if err := json.Unmarshal(output, &stats); err != nil {

		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}

	return &stats, nil
}

func (p *PodmanRuntime) WaitForContainer(name string, condition string) error {
	cmd := exec.Command(p.execPath, "wait", name)

	return cmd.Run()
}

func (p *PodmanRuntime) ValidateSecurityContext(opts *ContainerOptions) error {
	// Basic validation for Podman

	return nil
}

func (p *PodmanRuntime) UpdateContainerResources(name string, resources *ResourceLimits) error {
	// Podman doesn't support runtime resource updates like Docker

	return fmt.Errorf("podman doesn't support runtime resource updates")
}
