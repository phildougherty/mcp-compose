// internal/container/docker.go
package container

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DockerRuntime implements container runtime using Docker
type DockerRuntime struct {
	execPath string
}

// NewDockerRuntime creates a Docker runtime
func NewDockerRuntime(path string) (Runtime, error) {
	if path == "" {
		return nil, fmt.Errorf("docker executable path cannot be empty")
	}
	return &DockerRuntime{execPath: path}, nil
}

func (d *DockerRuntime) RemoveNetwork(name string) error {
	cmd := exec.Command("docker", "network", "rm", name)
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

func (d *DockerRuntime) GetRuntimeName() string {
	return "docker"
}

func (d *DockerRuntime) StartContainer(opts *ContainerOptions) (string, error) {
	// Check if container with this name already exists and remove it
	inspectCmd := exec.Command(d.execPath, "inspect", "--type=container", opts.Name)
	if err := inspectCmd.Run(); err == nil {
		fmt.Printf("Container '%s' already exists. Stopping and removing it first...\n", opts.Name)
		stopCmd := exec.Command(d.execPath, "stop", opts.Name)
		_ = stopCmd.Run()
		rmCmd := exec.Command(d.execPath, "rm", "-f", opts.Name)
		if err := rmCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to remove existing container '%s': %w", opts.Name, err)
		}
		fmt.Printf("Existing container '%s' removed.\n", opts.Name)
	}

	imageToRun := opts.Image // Use the image name from config if provided

	// Handle building the image if build context is provided
	if opts.Build.Context != "" {
		// If opts.Image is also set, it acts as the tag for the built image.
		// If opts.Image is empty, generate a dynamic one.
		if imageToRun == "" {
			imageToRun = fmt.Sprintf("mcp-compose-built-%s:latest", strings.ToLower(opts.Name))
		}

		buildCmdArgs := []string{"build", "-t", imageToRun}
		if opts.Build.Dockerfile != "" {
			// Dockerfile path should be relative to the build context
			dockerfilePath := opts.Build.Dockerfile
			if !filepath.IsAbs(dockerfilePath) { // Make it relative to context if not absolute
				// This logic for joining might need adjustment if Dockerfile is outside context
				// For now, assume Dockerfile is inside or at root of context.
				dockerfilePath = filepath.Join(opts.Build.Context, opts.Build.Dockerfile)
				// Check if file exists, sometimes context is '.' and Dockerfile is just 'sub/Dockerfile'
				// More robust Dockerfile pathing might be needed for complex setups.
				// The standard `docker build -f <path_to_dockerfile> <context_path>`
				// expects Dockerfile path to be interpretable by the daemon or client.
				// If Dockerfile is always AT THE ROOT OF THE CONTEXT, then `opts.Build.Dockerfile` is fine.
				// If it can be nested, `filepath.Join(opts.Build.Context, opts.Build.Dockerfile)` is safer,
				// but `docker build -f` might interpret this differently.
				// Simplest: assume Dockerfile name is relative *within* the context path for `docker build -f`
				if !strings.HasPrefix(opts.Build.Dockerfile, opts.Build.Context) && opts.Build.Dockerfile != "" {
					buildCmdArgs = append(buildCmdArgs, "-f", filepath.Join(opts.Build.Context, opts.Build.Dockerfile))
				} else if opts.Build.Dockerfile != "" {
					buildCmdArgs = append(buildCmdArgs, "-f", opts.Build.Dockerfile) // If already a full path or just name in context
				}
			} else { // Absolute path for Dockerfile
				buildCmdArgs = append(buildCmdArgs, "-f", opts.Build.Dockerfile)
			}
		}
		for k, v := range opts.Build.Args {
			buildCmdArgs = append(buildCmdArgs, "--build-arg", fmt.Sprintf("%s=%s", k, v))
		}
		buildCmdArgs = append(buildCmdArgs, opts.Build.Context)

		fmt.Printf("Building image '%s' with command: %s %s\n", imageToRun, d.execPath, strings.Join(buildCmdArgs, " "))
		buildCmd := exec.Command(d.execPath, buildCmdArgs...)
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to build image for server '%s' from context '%s': %w", opts.Name, opts.Build.Context, err)
		}
		fmt.Printf("Successfully built image '%s' for server '%s'\n", imageToRun, opts.Name)
	}

	if imageToRun == "" {
		return "", fmt.Errorf("no image specified or could be built for server '%s'", opts.Name)
	}

	// Pull image if requested AND no build was performed (build implies local image)
	if opts.Pull && opts.Build.Context == "" {
		fmt.Printf("Pulling image '%s'...\n", imageToRun)
		pullCmd := exec.Command(d.execPath, "pull", imageToRun)
		pullCmd.Stdout = os.Stdout
		pullCmd.Stderr = os.Stderr
		if err := pullCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to pull image '%s': %w", imageToRun, err)
		}
	}

	networkName := "mcp-net" // Default network
	// ... (your network creation logic here - looks OK) ...
	if d.GetRuntimeName() != "none" { // Ensure runtime is not none before network ops
		networkExists, _ := d.NetworkExists(networkName)
		if !networkExists {
			if err := d.CreateNetwork(networkName); err != nil {
				fmt.Printf("Warning: Failed to create default network %s: %v.\n", networkName, err)
			} else {
				fmt.Printf("Created Docker network: %s\n", networkName)
			}
		}
	}

	runArgs := []string{"run", "-d", "--name", opts.Name}
	runArgs = append(runArgs, "-i") // Keep interactive for potential STDIO piping
	runArgs = append(runArgs, "--restart=unless-stopped")

	for k, v := range opts.Env {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	for _, p := range opts.Ports {
		runArgs = append(runArgs, "-p", p)
	}
	for _, v := range opts.Volumes {
		runArgs = append(runArgs, "-v", v)
	}
	if opts.WorkDir != "" {
		runArgs = append(runArgs, "-w", opts.WorkDir)
	}

	var primaryNetworkConnected string
	if opts.NetworkMode != "" {
		runArgs = append(runArgs, "--network", opts.NetworkMode)
		primaryNetworkConnected = opts.NetworkMode
	} else {
		runArgs = append(runArgs, "--network", networkName) // Default to mcp-net
		primaryNetworkConnected = networkName
	}

	runArgs = append(runArgs, imageToRun) // ***** USE imageToRun HERE *****

	// The opts.Command and opts.Args are for the command INSIDE the container.
	// If the image has an ENTRYPOINT, these become arguments to the ENTRYPOINT.
	// If the image has only a CMD, these override the CMD.
	if opts.Command != "" {
		runArgs = append(runArgs, opts.Command)
		if len(opts.Args) > 0 {
			runArgs = append(runArgs, opts.Args...)
		}
	}

	// Corrected logging message to distinguish from your compose.go log
	fmt.Printf("DockerRuntime: Executing: %s %s\n", d.execPath, strings.Join(runArgs, " "))

	startCmd := exec.Command(d.execPath, runArgs...)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "DockerRuntime: Failed to start container '%s' with image '%s': %v. Output: %s\n", opts.Name, imageToRun, err, string(output))
		// ... (your existing log retrieval logic for failed container) ...
		inspectCmdForID := exec.Command(d.execPath, "inspect", "--format", "{{.Id}}", opts.Name) // Attempt to inspect by name
		idOutput, idErr := inspectCmdForID.Output()                                              // CombinedOutput might be better if inspect errors
		if idErr == nil {
			tempContainerID := strings.TrimSpace(string(idOutput))
			if tempContainerID != "" {
				logsCmd := exec.Command(d.execPath, "logs", "--tail", "50", tempContainerID)
				logsOutput, _ := logsCmd.CombinedOutput()
				fmt.Fprintf(os.Stderr, "DockerRuntime: Last 50 log lines for potentially created container %s (ID: %s):\n%s\n", opts.Name, tempContainerID, string(logsOutput))
				_ = exec.Command(d.execPath, "rm", "-f", opts.Name).Run() // Clean up the failed container by name
			} else {
				fmt.Fprintf(os.Stderr, "DockerRuntime: Could not get ID for container %s after start failure, it might not have been created.\n", opts.Name)
			}
		} else {
			fmt.Fprintf(os.Stderr, "DockerRuntime: Failed to inspect container %s after start failure: %v\n", opts.Name, idErr)
		}
		return "", fmt.Errorf("failed to start container '%s' with image '%s': %w. Output: %s", opts.Name, imageToRun, err, string(output))
	}
	containerID := strings.TrimSpace(string(output))

	// ... (Connect to additional networks logic - looks OK) ...
	for _, net := range opts.Networks {
		if net != primaryNetworkConnected && net != "" {
			exists, _ := d.NetworkExists(net)
			if !exists {
				if errNetCreate := d.CreateNetwork(net); errNetCreate != nil {
					fmt.Printf("Warning: Failed to create additional network %s for container %s: %v\n", net, opts.Name, errNetCreate)
					continue
				}
			}
			fmt.Printf("Connecting container %s (ID: %s) to additional network %s...\n", opts.Name, containerID, net)
			connectCmd := exec.Command(d.execPath, "network", "connect", net, containerID) // Use ID for connect
			if errNetConnect := connectCmd.Run(); errNetConnect != nil {
				fmt.Printf("Warning: Failed to connect container %s to additional network %s: %v\n", opts.Name, net, errNetConnect)
			}
		}
	}

	// ... (Status check logic - looks OK, ensure it uses containerID) ...
	time.Sleep(1 * time.Second) // Brief pause
	statusCmd := exec.Command(d.execPath, "inspect", "--format={{.State.Status}}", containerID)
	statusOutput, statusErr := statusCmd.CombinedOutput()
	if statusErr != nil {
		fmt.Printf("Warning: Could not verify status of container '%s' (ID: %s): %v. Output: %s\n", opts.Name, containerID, statusErr, string(statusOutput))
	} else {
		currentStatus := strings.TrimSpace(string(statusOutput))
		fmt.Printf("Container '%s' (ID: %s...) current status: %s\n", opts.Name, containerID[:12], currentStatus)
		if currentStatus != "running" {
			fmt.Printf("Warning: Container '%s' is %s, not 'running'.\n", opts.Name, currentStatus)
			logsCmd := exec.Command(d.execPath, "logs", "--tail", "20", containerID)
			logsOutput, _ := logsCmd.CombinedOutput()
			fmt.Printf("Last 20 log lines for %s (ID: %s):\n%s\n", opts.Name, containerID, string(logsOutput))
		}
	}

	return containerID, nil
}

// ExecContainer is generally not used by the proxy for HTTP transport, but kept for other commands.
func (d *DockerRuntime) ExecContainer(containerName string, command []string, interactive bool) (*exec.Cmd, io.Writer, io.Reader, error) {
	args := []string{"exec"}
	if interactive {
		args = append(args, "-i")
	}
	args = append(args, containerName)
	args = append(args, command...)

	cmd := exec.Command(d.execPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create stdin pipe for exec: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close() // Clean up stdin pipe if stdout fails
		return nil, nil, nil, fmt.Errorf("failed to create stdout pipe for exec: %w", err)
	}
	cmd.Stderr = os.Stderr // Redirect Stderr directly for exec command

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, nil, nil, fmt.Errorf("failed to start exec command: %w", err)
	}
	return cmd, stdin, stdout, nil
}

func (d *DockerRuntime) StopContainer(name string) error {
	// Check if container exists before attempting to stop/remove
	inspectCmd := exec.Command(d.execPath, "inspect", "--type=container", name)
	if err := inspectCmd.Run(); err != nil {
		// If inspect fails, container likely doesn't exist.
		// This is not an error for a "stop" operation if the intent is "ensure stopped".
		fmt.Printf("Container '%s' not found or already removed, skipping stop/remove.\n", name)
		return nil
	}

	// Stop the container
	stopCmd := exec.Command(d.execPath, "stop", name)
	if err := stopCmd.Run(); err != nil {
		// Log warning if stop fails, but proceed to rm as it might be already stopped.
		fmt.Printf("Warning: Failed to stop container '%s' (it might be already stopped): %v\n", name, err)
	} else {
		fmt.Printf("Container '%s' stopped.\n", name)
	}

	// Remove the container
	rmCmd := exec.Command(d.execPath, "rm", "-f", name) // -f to force remove if stopped but not removed
	if err := rmCmd.Run(); err != nil {
		return fmt.Errorf("failed to remove container '%s': %w", name, err)
	}
	fmt.Printf("Container '%s' removed.\n", name)
	return nil
}

func (d *DockerRuntime) GetContainerStatus(name string) (string, error) {
	inspectCmd := exec.Command(d.execPath, "inspect", "--format", "{{.State.Status}}", name)
	output, err := inspectCmd.CombinedOutput()
	if err != nil {
		// Try to parse docker's error output for "No such object"
		if strings.Contains(strings.ToLower(string(output)), "no such object") ||
			strings.Contains(strings.ToLower(err.Error()), "no such container") {
			return "stopped", nil
		}
		return "unknown", fmt.Errorf("failed to inspect container '%s': %w, output: %s", name, err, string(output))
	}
	status := strings.TrimSpace(string(output))
	// Map Docker statuses to a more generic set if desired, or return raw
	switch strings.ToLower(status) {
	case "running":
		return "running", nil
	case "created", "restarting":
		return "starting", nil // Or map 'created' to 'stopped' if it means not yet run
	case "paused":
		return "paused", nil
	case "exited", "dead":
		return "stopped", nil
	default:
		// For any other status (like "removing"), or if status is empty
		if status == "" {
			return "unknown", fmt.Errorf("empty status received for container %s", name)
		}
		return status, nil // Return the raw status from Docker
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
	// If `Run` returns an error, the network likely doesn't exist or cannot be inspected.
	// A nil error means the inspect command succeeded, so the network exists.
	err := cmd.Run()
	return err == nil, nil
}

func (d *DockerRuntime) CreateNetwork(name string) error {
	cmd := exec.Command(d.execPath, "network", "create", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if the error is because the network already exists
		if strings.Contains(string(output), "already exists") {
			fmt.Printf("Network '%s' already exists.\n", name)
			return nil
		}
		return fmt.Errorf("failed to create network '%s': %w, output: %s", name, err, string(output))
	}
	fmt.Printf("Network '%s' created.\n", name)
	return nil
}
