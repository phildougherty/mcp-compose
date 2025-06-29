// internal/container/docker.go
package container

import (
	"encoding/json"
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

// ADD these methods to DockerRuntime:

func (d *DockerRuntime) RestartContainer(name string) error {
	cmd := exec.Command(d.execPath, "restart", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart container '%s': %w. Output: %s", name, err, string(output))
	}
	return nil
}

func (d *DockerRuntime) PauseContainer(name string) error {
	cmd := exec.Command(d.execPath, "pause", name)
	return cmd.Run()
}

func (d *DockerRuntime) UnpauseContainer(name string) error {
	cmd := exec.Command(d.execPath, "unpause", name)
	return cmd.Run()
}

func (d *DockerRuntime) GetContainerInfo(name string) (*ContainerInfo, error) {
	format := `{
        "ID": "{{.Id}}",
        "Name": "{{.Name}}",
        "Image": "{{.Config.Image}}",
        "Status": "{{.State.Status}}",
        "State": "{{.State.Status}}",
        "Created": "{{.Created}}",
        "Command": {{json .Config.Cmd}},
        "Labels": {{json .Config.Labels}},
        "Env": {{json .Config.Env}},
        "RestartCount": {{.RestartCount}}
    }`

	cmd := exec.Command(d.execPath, "inspect", "--format", format, name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container '%s': %w", name, err)
	}

	var info ContainerInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	return &info, nil
}

func (d *DockerRuntime) ListContainers(filters map[string]string) ([]ContainerInfo, error) {
	args := []string{"ps", "-a", "--format", "json"}

	for key, value := range filters {
		args = append(args, "--filter", fmt.Sprintf("%s=%s", key, value))
	}

	cmd := exec.Command(d.execPath, args...)
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
			continue // Skip malformed entries
		}
		containers = append(containers, container)
	}

	return containers, nil
}

func (d *DockerRuntime) PullImage(image string, auth *ImageAuth) error {
	args := []string{"pull"}
	if auth != nil {
		// Add authentication if provided
		args = append(args, "--username", auth.Username, "--password", auth.Password)
	}
	args = append(args, image)

	cmd := exec.Command(d.execPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *DockerRuntime) BuildImage(opts *BuildOptions) error {
	args := []string{"build"}

	// Only add -f flag if dockerfile is NOT the default name or is in a different location
	if opts.Dockerfile != "" && opts.Dockerfile != "Dockerfile" {
		// For non-default dockerfile names, we need the full path
		dockerfilePath := filepath.Join(opts.Context, opts.Dockerfile)
		args = append(args, "-f", dockerfilePath)
	}
	// If opts.Dockerfile is empty or "Dockerfile", don't use -f flag at all
	// Docker will automatically look for "Dockerfile" in the build context

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

	if opts.Pull {
		args = append(args, "--pull")
	}

	if opts.Platform != "" {
		args = append(args, "--platform", opts.Platform)
	}

	// Add context path last
	args = append(args, opts.Context)

	fmt.Printf("Building image: docker %s\n", strings.Join(args, " "))

	cmd := exec.Command(d.execPath, args...)

	output, err := cmd.CombinedOutput()

	if len(output) > 0 {
		fmt.Printf("Build output:\n%s\n", string(output))
	}

	if err != nil {
		return fmt.Errorf("docker build failed: %w\nBuild output: %s", err, string(output))
	}

	return nil
}

func (d *DockerRuntime) GetContainerStats(name string) (*ContainerStats, error) {
	cmd := exec.Command(d.execPath, "stats", "--no-stream", "--format", "json", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats for container '%s': %w", name, err)
	}

	var stats ContainerStats
	if err := json.Unmarshal(output, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}

	return &stats, nil
}

func (d *DockerRuntime) ValidateSecurityContext(opts *ContainerOptions) error {
	// Check if this is a system container (proxy, dashboard)
	isSystemContainer := false
	if systemLabel, exists := opts.Labels["mcp-compose.system"]; exists && systemLabel == "true" {
		isSystemContainer = true
	}

	// System containers get relaxed validation
	if isSystemContainer {
		fmt.Printf("Info: System container '%s' granted elevated permissions\n", opts.Name)
		return nil
	}

	// Check if this container has security exceptions configured
	securityConfig := opts.Security

	// Privileged mode check
	if opts.Privileged {
		if !securityConfig.AllowPrivilegedOps {
			return fmt.Errorf("container '%s' requests privileged mode but security.allow_privileged_ops is not enabled", opts.Name)
		}
		fmt.Printf("Info: Container '%s' running in privileged mode (explicitly allowed)\n", opts.Name)
	}

	// Volume mount validation
	for _, volume := range opts.Volumes {
		if err := d.validateVolumeMount(volume, opts.Name, &securityConfig); err != nil {
			return err
		}
	}

	// Capability validation
	for _, cap := range opts.CapAdd {
		if err := d.validateCapability(cap, opts.Name); err != nil {
			return err
		}
	}

	return nil
}

func (d *DockerRuntime) validateVolumeMount(volume, containerName string, security *SecurityConfig) error {
	parts := strings.Split(volume, ":")
	if len(parts) < 2 {
		// This is a named volume (e.g., "mcp-cron-data:/data") - always allow
		return nil
	}

	source := parts[0]

	// Check if this is a named volume (doesn't start with / or .)
	if !strings.HasPrefix(source, "/") && !strings.HasPrefix(source, ".") {
		// This is a named Docker volume - always allow
		fmt.Printf("Info: Container '%s' mounting Docker volume '%s'\n", containerName, source)
		return nil
	}

	// Check Docker socket access
	if source == "/var/run/docker.sock" {
		if !security.AllowDockerSocket {
			return fmt.Errorf("container '%s' requests Docker socket access but security.allow_docker_socket is not enabled", containerName)
		}

		// Ensure it's read-only unless explicitly allowed for privileged ops
		if !strings.HasSuffix(volume, ":ro") && !security.AllowPrivilegedOps {
			return fmt.Errorf("container '%s' requests write access to Docker socket but security.allow_privileged_ops is not enabled", containerName)
		}

		fmt.Printf("Info: Container '%s' granted Docker socket access (explicitly allowed)\n", containerName)
		return nil
	}

	// Check dangerous system mounts
	dangerousPaths := []string{"/", "/etc", "/proc", "/sys", "/boot", "/dev"}
	for _, dangerous := range dangerousPaths {
		if source == dangerous {
			if !security.AllowPrivilegedOps {
				return fmt.Errorf("container '%s' requests dangerous mount '%s' but security.allow_privileged_ops is not enabled", containerName, source)
			}
			fmt.Printf("Warning: Container '%s' mounting dangerous path '%s' (explicitly allowed)\n", containerName, source)
			return nil
		}
	}

	// Check allowed host mounts
	if len(security.AllowHostMounts) > 0 {
		for _, allowed := range security.AllowHostMounts {
			if strings.HasPrefix(source, allowed) {
				fmt.Printf("Info: Container '%s' mounting allowed host path '%s'\n", containerName, source)
				return nil
			}
		}
		// If allow list is specified but path not in it
		return fmt.Errorf("container '%s' requests mount '%s' which is not in security.allow_host_mounts list", containerName, source)
	}

	return nil
}

func (d *DockerRuntime) validateCapability(capability, containerName string) error {
	// List of potentially dangerous capabilities
	dangerousCaps := []string{
		"SYS_ADMIN", "SYS_PTRACE", "SYS_MODULE", "DAC_OVERRIDE",
		"SYS_RAWIO", "SYS_TIME", "NET_ADMIN", "SYS_NICE",
	}

	for _, dangerous := range dangerousCaps {
		if strings.ToUpper(capability) == dangerous {
			fmt.Printf("Warning: Container '%s' adding potentially dangerous capability '%s'\n", containerName, capability)
			break
		}
	}

	return nil
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

	imageToRun := opts.Image

	// Handle building (NO SECURITY VALIDATION FOR BUILD PROCESS)
	if opts.Build.Context != "" {
		if imageToRun == "" {
			imageToRun = fmt.Sprintf("mcp-compose-built-%s:latest", strings.ToLower(opts.Name))
		}

		// Add detailed debugging
		fmt.Printf("=== BUILD DEBUG INFO ===\n")
		fmt.Printf("  Server: %s\n", opts.Name)
		fmt.Printf("  Build Context: %s\n", opts.Build.Context)
		fmt.Printf("  Dockerfile: %s\n", opts.Build.Dockerfile)
		fmt.Printf("  Target Image: %s\n", imageToRun)
		fmt.Printf("  Current Working Directory: %s\n", func() string {
			if cwd, err := os.Getwd(); err == nil {
				return cwd
			}
			return "unknown"
		}())

		// Check if context exists
		if info, err := os.Stat(opts.Build.Context); err != nil {
			fmt.Printf("  Context directory error: %v\n", err)
			return "", fmt.Errorf("build context directory '%s' does not exist: %w", opts.Build.Context, err)
		} else {
			fmt.Printf("  Context directory exists: %v\n", info.IsDir())
		}

		// Check dockerfile
		dockerfilePath := "Dockerfile"
		if opts.Build.Dockerfile != "" {
			dockerfilePath = opts.Build.Dockerfile
		}
		fullPath := filepath.Join(opts.Build.Context, dockerfilePath)
		if _, err := os.Stat(fullPath); err != nil {
			fmt.Printf("  Dockerfile error: %v (looking for: %s)\n", err, fullPath)
			return "", fmt.Errorf("dockerfile '%s' does not exist in context '%s': %w", dockerfilePath, opts.Build.Context, err)
		} else {
			fmt.Printf("  Dockerfile exists: %s\n", fullPath)
		}
		fmt.Printf("=== END BUILD DEBUG ===\n")

		buildOpts := &BuildOptions{
			Context:    opts.Build.Context,
			Dockerfile: dockerfilePath,
			Tags:       []string{imageToRun},
			Args:       opts.Build.Args,
			Target:     opts.Build.Target,
			NoCache:    opts.Build.NoCache,
			Pull:       opts.Build.Pull,
			Platform:   opts.Build.Platform,
		}

		// Build process runs as host user - no container security applied
		fmt.Printf("Starting build process for '%s'...\n", imageToRun)
		if err := d.BuildImage(buildOpts); err != nil {
			return "", fmt.Errorf("failed to build image: %w", err)
		}

		fmt.Printf("Successfully built image '%s'\n", imageToRun)
	}

	if imageToRun == "" {
		return "", fmt.Errorf("no image specified or could be built for server '%s'", opts.Name)
	}

	// Pull image if requested AND no build was performed
	if opts.Pull && opts.Build.Context == "" {
		fmt.Printf("Pulling image '%s'...\n", imageToRun)
		if err := d.PullImage(imageToRun, nil); err != nil {
			return "", fmt.Errorf("failed to pull image '%s': %w", imageToRun, err)
		}
	}

	// NOW apply security validation to the CONTAINER RUNTIME only
	fmt.Printf("Applying security validation for container runtime '%s'...\n", opts.Name)
	if err := d.ValidateSecurityContext(opts); err != nil {
		return "", fmt.Errorf("container runtime security validation failed: %w", err)
	}

	// Ensure networks exist
	networkName := "mcp-net"
	if d.GetRuntimeName() != "none" {
		networkExists, _ := d.NetworkExists(networkName)
		if !networkExists {
			if err := d.CreateNetwork(networkName); err != nil {
				fmt.Printf("Warning: Failed to create default network %s: %v.\n", networkName, err)
			} else {
				fmt.Printf("Created Docker network: %s\n", networkName)
			}
		}
	}

	// Build run command with enhanced options
	runArgs := []string{"run", "-d", "--name", opts.Name}
	runArgs = append(runArgs, "-i") // Keep interactive for potential STDIO piping

	// Restart policy
	if opts.RestartPolicy != "" {
		runArgs = append(runArgs, "--restart", opts.RestartPolicy)
	} else {
		runArgs = append(runArgs, "--restart=unless-stopped")
	}

	// Resource limits
	if opts.CPUs != "" {
		runArgs = append(runArgs, "--cpus", opts.CPUs)
	}
	if opts.Memory != "" {
		runArgs = append(runArgs, "--memory", opts.Memory)
	}
	if opts.MemorySwap != "" {
		runArgs = append(runArgs, "--memory-swap", opts.MemorySwap)
	}
	if opts.PidsLimit > 0 {
		runArgs = append(runArgs, "--pids-limit", fmt.Sprintf("%d", opts.PidsLimit))
	}

	// Security options
	if opts.User != "" {
		runArgs = append(runArgs, "--user", opts.User)
	}
	if opts.Privileged {
		runArgs = append(runArgs, "--privileged")
	}
	for _, cap := range opts.CapAdd {
		runArgs = append(runArgs, "--cap-add", cap)
	}
	for _, cap := range opts.CapDrop {
		runArgs = append(runArgs, "--cap-drop", cap)
	}
	for _, opt := range opts.SecurityOpt {
		runArgs = append(runArgs, "--security-opt", opt)
	}
	if opts.ReadOnly {
		runArgs = append(runArgs, "--read-only")
	}

	// Hostname and networking
	if opts.Hostname != "" {
		runArgs = append(runArgs, "--hostname", opts.Hostname)
	}
	if opts.DomainName != "" {
		runArgs = append(runArgs, "--domainname", opts.DomainName)
	}
	for _, dns := range opts.DNS {
		runArgs = append(runArgs, "--dns", dns)
	}
	for _, host := range opts.ExtraHosts {
		runArgs = append(runArgs, "--add-host", host)
	}

	// Environment variables
	for k, v := range opts.Env {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Ports
	for _, p := range opts.Ports {
		runArgs = append(runArgs, "-p", p)
	}

	// Volumes
	for _, v := range opts.Volumes {
		runArgs = append(runArgs, "-v", v)
	}

	// Tmpfs
	for _, tmpfs := range opts.Tmpfs {
		runArgs = append(runArgs, "--tmpfs", tmpfs)
	}

	// Working directory
	if opts.WorkDir != "" {
		runArgs = append(runArgs, "-w", opts.WorkDir)
	}

	// Labels
	for k, v := range opts.Labels {
		runArgs = append(runArgs, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// Health check
	if opts.HealthCheck != nil {
		if len(opts.HealthCheck.Test) > 0 {
			if len(opts.HealthCheck.Test) > 1 {
				runArgs = append(runArgs, "--health-cmd", strings.Join(opts.HealthCheck.Test[1:], " "))
			}
		}
		if opts.HealthCheck.Interval != "" {
			runArgs = append(runArgs, "--health-interval", opts.HealthCheck.Interval)
		}
		if opts.HealthCheck.Timeout != "" {
			runArgs = append(runArgs, "--health-timeout", opts.HealthCheck.Timeout)
		}
		if opts.HealthCheck.Retries > 0 {
			runArgs = append(runArgs, "--health-retries", fmt.Sprintf("%d", opts.HealthCheck.Retries))
		}
		if opts.HealthCheck.StartPeriod != "" {
			runArgs = append(runArgs, "--health-start-period", opts.HealthCheck.StartPeriod)
		}
	}

	// Logging
	if opts.LogDriver != "" {
		runArgs = append(runArgs, "--log-driver", opts.LogDriver)
	}
	for k, v := range opts.LogOptions {
		runArgs = append(runArgs, "--log-opt", fmt.Sprintf("%s=%s", k, v))
	}

	// Platform
	if opts.Platform != "" {
		runArgs = append(runArgs, "--platform", opts.Platform)
	}

	// Stop signal and timeout
	if opts.StopSignal != "" {
		runArgs = append(runArgs, "--stop-signal", opts.StopSignal)
	}
	if opts.StopTimeout != nil {
		runArgs = append(runArgs, "--stop-timeout", fmt.Sprintf("%d", *opts.StopTimeout))
	}

	// Network configuration
	var primaryNetworkConnected string
	if opts.NetworkMode != "" {
		runArgs = append(runArgs, "--network", opts.NetworkMode)
		primaryNetworkConnected = opts.NetworkMode
	} else {
		runArgs = append(runArgs, "--network", networkName)
		primaryNetworkConnected = networkName
	}

	runArgs = append(runArgs, imageToRun)

	// Command and arguments
	if opts.Command != "" {
		runArgs = append(runArgs, opts.Command)
		if len(opts.Args) > 0 {
			runArgs = append(runArgs, opts.Args...)
		}
	}

	fmt.Printf("DockerRuntime: Executing: %s %s\n", d.execPath, strings.Join(runArgs, " "))
	startCmd := exec.Command(d.execPath, runArgs...)
	output, err := startCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "DockerRuntime: Failed to start container '%s' with image '%s': %v. Output: %s\n", opts.Name, imageToRun, err, string(output))

		// Try to get logs if container was created but failed to start
		inspectCmdForID := exec.Command(d.execPath, "inspect", "--format", "{{.Id}}", opts.Name)
		idOutput, idErr := inspectCmdForID.Output()
		if idErr == nil {
			tempContainerID := strings.TrimSpace(string(idOutput))
			if tempContainerID != "" {
				logsCmd := exec.Command(d.execPath, "logs", "--tail", "50", tempContainerID)
				logsOutput, _ := logsCmd.CombinedOutput()
				fmt.Fprintf(os.Stderr, "DockerRuntime: Last 50 log lines for container %s (ID: %s):\n%s\n", opts.Name, tempContainerID, string(logsOutput))
				_ = exec.Command(d.execPath, "rm", "-f", opts.Name).Run()
			}
		}

		return "", fmt.Errorf("failed to start container '%s' with image '%s': %w. Output: %s", opts.Name, imageToRun, err, string(output))
	}

	containerID := strings.TrimSpace(string(output))

	// Connect to additional networks
	for _, net := range opts.Networks {
		if net != primaryNetworkConnected && net != "" {
			exists, _ := d.NetworkExists(net)
			if !exists {
				if errNetCreate := d.CreateNetwork(net); errNetCreate != nil {
					fmt.Printf("Warning: Failed to create additional network %s for container %s: %v\n", net, opts.Name, errNetCreate)
					continue
				}
			}
			fmt.Printf("Connecting container %s to additional network %s...\n", opts.Name, net)
			if err := d.ConnectToNetwork(containerID, net); err != nil {
				fmt.Printf("Warning: Failed to connect container %s to additional network %s: %v\n", opts.Name, net, err)
			}
		}
	}

	// Status check
	time.Sleep(1 * time.Second)
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

func (d *DockerRuntime) ConnectToNetwork(containerName, networkName string) error {
	cmd := exec.Command(d.execPath, "network", "connect", networkName, containerName)
	return cmd.Run()
}

func (d *DockerRuntime) DisconnectFromNetwork(containerName, networkName string) error {
	cmd := exec.Command(d.execPath, "network", "disconnect", networkName, containerName)
	return cmd.Run()
}

func (d *DockerRuntime) CreateVolume(name string, opts *VolumeOptions) error {
	args := []string{"volume", "create"}

	if opts != nil {
		if opts.Driver != "" {
			args = append(args, "--driver", opts.Driver)
		}

		for key, value := range opts.DriverOpts {
			args = append(args, "--opt", fmt.Sprintf("%s=%s", key, value))
		}

		for key, value := range opts.Labels {
			args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
		}
	}

	args = append(args, name)

	cmd := exec.Command(d.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if volume already exists
		if strings.Contains(string(output), "already exists") {
			fmt.Printf("Volume '%s' already exists.\n", name)
			return nil
		}
		return fmt.Errorf("failed to create volume '%s': %w, output: %s", name, err, string(output))
	}

	fmt.Printf("Volume '%s' created.\n", name)
	return nil
}

func (d *DockerRuntime) RemoveVolume(name string, force bool) error {
	args := []string{"volume", "rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, name)

	cmd := exec.Command(d.execPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "no such volume") {
			return nil // Volume doesn't exist, consider it success
		}
		return fmt.Errorf("failed to remove volume '%s': %w, output: %s", name, err, string(output))
	}

	return nil
}

func (d *DockerRuntime) ListVolumes() ([]VolumeInfo, error) {
	cmd := exec.Command(d.execPath, "volume", "ls", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	var volumes []VolumeInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var volume VolumeInfo
		if err := json.Unmarshal([]byte(line), &volume); err != nil {
			continue // Skip malformed entries
		}
		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (d *DockerRuntime) RemoveImage(image string, force bool) error {
	args := []string{"rmi"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, image)

	cmd := exec.Command(d.execPath, args...)
	return cmd.Run()
}

func (d *DockerRuntime) ListImages() ([]ImageInfo, error) {
	cmd := exec.Command(d.execPath, "images", "--format", "json")
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
			continue // Skip malformed entries
		}
		images = append(images, image)
	}

	return images, nil
}

func (d *DockerRuntime) ListNetworks() ([]NetworkInfo, error) {
	cmd := exec.Command(d.execPath, "network", "ls", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var networks []NetworkInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var network NetworkInfo
		if err := json.Unmarshal([]byte(line), &network); err != nil {
			continue // Skip malformed entries
		}
		networks = append(networks, network)
	}

	return networks, nil
}

func (d *DockerRuntime) GetNetworkInfo(name string) (*NetworkInfo, error) {
	cmd := exec.Command(d.execPath, "network", "inspect", name)
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

func (d *DockerRuntime) WaitForContainer(name string, condition string) error {
	cmd := exec.Command(d.execPath, "wait", name)
	return cmd.Run()
}

func (d *DockerRuntime) UpdateContainerResources(name string, resources *ResourceLimits) error {
	args := []string{"update"}

	if resources.CPUs != "" {
		args = append(args, "--cpus", resources.CPUs)
	}
	if resources.Memory != "" {
		args = append(args, "--memory", resources.Memory)
	}
	if resources.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", resources.PidsLimit))
	}

	args = append(args, name)

	cmd := exec.Command(d.execPath, args...)
	return cmd.Run()
}
