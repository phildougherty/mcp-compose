// internal/container/docker.go
package container

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DockerRuntime implements container runtime using Docker
type DockerRuntime struct {
	execPath string
}

// NewDockerRuntime creates a Docker runtime
func NewDockerRuntime(path string) (Runtime, error) {
	return &DockerRuntime{execPath: path}, nil
}

func (d *DockerRuntime) GetRuntimeName() string {
	return "docker"
}

func (d *DockerRuntime) StartContainer(opts *ContainerOptions) (string, error) {
	// Check if container with this name already exists
	cmd := exec.Command(d.execPath, "inspect", "--type=container", opts.Name)
	if err := cmd.Run(); err == nil {
		// Container exists, remove it
		rmCmd := exec.Command(d.execPath, "rm", "-f", opts.Name)
		if err := rmCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to remove existing container: %w", err)
		}
	}

	// Pull image if requested
	if opts.Pull {
		fmt.Printf("Pulling image '%s'...\n", opts.Image)
		pullCmd := exec.Command(d.execPath, "pull", opts.Image)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to pull image: %w", err)
		}
	}

	// Prepare docker run command
	args := []string{"run", "-d", "--name", opts.Name}

	// Keep stdin open for interactive use - critical for MCP servers
	args = append(args, "-i")

	// Add restart policy to ensure containers restart on failure
	args = append(args, "--restart=unless-stopped")

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

	// Ensure mcp-net network exists
	networkName := "mcp-net"
	networkExists, _ := d.NetworkExists(networkName)
	if !networkExists {
		// Create the network
		createNetCmd := exec.Command(d.execPath, "network", "create", networkName)
		if err := createNetCmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to create network %s: %v\n", networkName, err)
		} else {
			fmt.Printf("Created Docker network: %s\n", networkName)
		}
	}

	// Add network to the container if not already specified
	if opts.NetworkMode == "" {
		// Check if mcp-net is already in the networks list
		inNetworks := false
		for _, net := range opts.Networks {
			if net == networkName {
				inNetworks = true
				break
			}
		}

		if !inNetworks {
			args = append(args, "--network", networkName)
		}
	}

	// Add networks
	for _, network := range opts.Networks {
		if network == networkName {
			continue // Already added above
		}

		// Ensure network exists
		networkExists, _ := d.NetworkExists(network)
		if !networkExists {
			// Create the network
			if err := d.CreateNetwork(network); err != nil {
				return "", err
			}
		}

		if opts.NetworkMode == "" { // Only add --network if not using special network mode
			args = append(args, "--network", network)
		}
	}

	// Save the original command and args
	originalCommand := opts.Command
	var originalArgs []string
	if len(opts.Args) > 0 {
		originalArgs = make([]string, len(opts.Args))
		copy(originalArgs, opts.Args)
	}

	// Special handling for MCP servers to ensure proper execution
	if originalCommand == "npx" || originalCommand == "node" {
		// For MCP servers using npx or node, wrap in bash -c to ensure proper stdin/stdout handling
		fullCommand := originalCommand
		if len(originalArgs) > 0 {
			fullCommand = fmt.Sprintf("%s %s", fullCommand, strings.Join(originalArgs, " "))
		}

		// Set the proper bash wrapper command
		opts.Command = "bash"
		opts.Args = []string{"-c", fullCommand}

		// Debug output
		fmt.Printf("Wrapping command: '%s' with arguments: '%s'\n", originalCommand, strings.Join(originalArgs, " "))
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

	// Prepare and log the full docker command for debugging
	fullCmd := fmt.Sprintf("%s %s", d.execPath, strings.Join(args, " "))
	fmt.Printf("Executing: %s\n", fullCmd)

	// Execute docker run
	cmd = exec.Command(d.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to start container: %w, %s", err, string(output))
	}

	// Get the container ID from output
	containerID := strings.TrimSpace(string(output))

	// Wait briefly to ensure container starts properly
	time.Sleep(500 * time.Millisecond)

	// Check container status to ensure it's actually running
	statusCmd := exec.Command(d.execPath, "container", "inspect", "--format", "{{.State.Status}}", containerID)
	statusOutput, statusErr := statusCmd.CombinedOutput()
	if statusErr == nil {
		status := strings.TrimSpace(string(statusOutput))
		fmt.Printf("DEBUG: Raw container status for '%s': '%s'\n", opts.Name, status)
	}

	// Log container status
	fmt.Printf("Container '%s' started with ID %s\n", opts.Name, containerID[:12])

	// If the original command was wrapped, restore it for proper references later
	if originalCommand == "npx" || originalCommand == "node" {
		opts.Command = originalCommand
		if len(originalArgs) > 0 {
			opts.Args = originalArgs
		}
	}

	return containerID, nil
}

func (d *DockerRuntime) StopContainer(name string) error {
	// Check if container exists
	cmd := exec.Command(d.execPath, "inspect", "--type=container", name)
	if err := cmd.Run(); err != nil {
		// Container doesn't exist, nothing to do
		return nil
	}
	// Stop the container
	cmd = exec.Command(d.execPath, "stop", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	// Remove the container
	cmd = exec.Command(d.execPath, "rm", "-f", name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

func (d *DockerRuntime) GetContainerStatus(name string) (string, error) {
	// Check if container exists at all
	inspectCmd := exec.Command(d.execPath, "container", "inspect", name)
	if err := inspectCmd.Run(); err != nil {
		// Container doesn't exist
		return "stopped", nil
	}

	// Get detailed status information
	statusCmd := exec.Command(d.execPath, "container", "inspect", "--format", "{{.State.Status}}", name)
	output, err := statusCmd.CombinedOutput()
	if err != nil {
		return "unknown", err
	}

	status := strings.TrimSpace(string(output))
	fmt.Printf("DEBUG: Raw container status for '%s': '%s'\n", name, status)

	// Map Docker-specific statuses to running/stopped
	switch status {
	case "running":
		return "running", nil
	case "created", "restarting":
		return "starting", nil
	case "exited", "dead", "removing", "paused":
		// Get exit code for exited containers
		if status == "exited" {
			exitCmd := exec.Command(d.execPath, "container", "inspect", "--format", "{{.State.ExitCode}}", name)
			exitOutput, exitErr := exitCmd.CombinedOutput()
			if exitErr == nil {
				exitCode := strings.TrimSpace(string(exitOutput))
				return fmt.Sprintf("exited(%s)", exitCode), nil
			}
		}
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

func (d *DockerRuntime) GetExecPath() string {
	return d.execPath
}

func (d *DockerRuntime) ShowContainerLogs(name string, follow bool) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, name)
	cmd := exec.Command(d.execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *DockerRuntime) NetworkExists(name string) (bool, error) {
	cmd := exec.Command(d.execPath, "network", "inspect", name)
	err := cmd.Run()
	return err == nil, nil
}

func (d *DockerRuntime) CreateNetwork(name string) error {
	cmd := exec.Command(d.execPath, "network", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create network '%s': %w, %s", name, err, string(output))
	}
	return nil
}
