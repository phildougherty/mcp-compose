// internal/container/podman.go
package container

import (
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
		stdin.Close()
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, fmt.Errorf("failed to start command: %w", err)
	}

	return cmd, stdin, stdout, nil
}
