// internal/compose/compose.go
package compose

import (
	"fmt"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/server"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/fatih/color"
)

// Up starts all servers defined in the compose file or only the specified servers
func Up(configFile string, serverNames []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	// Determine which runtime to use
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	// Create server manager
	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	// Start servers based on dependencies
	serversToStart := getServersToStart(cfg, serverNames)
	fmt.Println("Starting MCP servers...")

	// Group servers by their dependency level
	levels := getDependencyLevels(cfg, serversToStart)

	// Start servers level by level
	for i, level := range levels {
		fmt.Printf("Starting dependency level %d servers...\n", i+1)
		var wg sync.WaitGroup
		errCh := make(chan error, len(level))

		for _, name := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				fmt.Printf("Starting server '%s'...\n", name)
				if err := mgr.StartServer(name); err != nil {
					errCh <- fmt.Errorf("failed to start server '%s': %w", name, err)
				} else {
					fmt.Printf("Server '%s' started successfully\n", name)
				}
			}(name)
		}

		wg.Wait()
		close(errCh)

		// Check for errors
		if len(errCh) > 0 {
			var errs []string
			for err := range errCh {
				errs = append(errs, err.Error())
			}
			return fmt.Errorf("failed to start servers: %s", strings.Join(errs, "; "))
		}
	}

	fmt.Println("All MCP servers started successfully")
	return nil
}

// Down stops and removes all servers
func Down(configFile string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	fmt.Println("Stopping MCP servers...")
	// Get all servers
	serverNames := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		serverNames = append(serverNames, name)
	}
	// Stop servers in reverse dependency order
	levels := getDependencyLevels(cfg, serverNames)
	// Reverse the levels to stop in opposite order
	for i := len(levels) - 1; i >= 0; i-- {
		level := levels[i]
		fmt.Printf("Stopping dependency level %d servers...\n", i+1)
		var wg sync.WaitGroup
		errCh := make(chan error, len(level))
		for _, name := range level {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				fmt.Printf("Stopping server '%s'...\n", name)
				if err := mgr.StopServer(name); err != nil {
					errCh <- fmt.Errorf("failed to stop server '%s': %w", name, err)
				}
			}(name)
		}
		wg.Wait()
		close(errCh)
		// Check for errors but continue stopping others
		if len(errCh) > 0 {
			for err := range errCh {
				fmt.Fprintf(os.Stderr, "Warning: %s\n", err.Error())
			}
		}
	}
	fmt.Println("All MCP servers stopped")
	return nil
}

// Start starts specific servers
func Start(configFile string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified")
	}
	return Up(configFile, serverNames)
}

// Stop stops specific servers
func Stop(configFile string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified")
	}
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	// Validate server names
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			return fmt.Errorf("server '%s' not found in configuration", name)
		}
	}
	// Stop specified servers
	for _, name := range serverNames {
		fmt.Printf("Stopping server '%s'...\n", name)
		if err := mgr.StopServer(name); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to stop server '%s': %v\n", name, err)
		}
	}
	fmt.Println("Requested MCP servers stopped")
	return nil
}

// List lists all defined servers and their status
func List(configFile string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}

	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	// Get detailed information about Docker containers
	containerInfo := make(map[string]string)
	if dockerRuntime, ok := cRuntime.(*container.DockerRuntime); ok {
		// If using Docker, get detailed container info
		info, err := getDetailedContainerInfo(dockerRuntime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to get detailed container info: %v\n", err)
		} else {
			containerInfo = info
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "SERVER\tSTATUS\tTYPE\tCONTAINER ID\tCAPABILITIES")

	// Define colored status strings
	running := color.New(color.FgGreen).Sprint("Running")
	stopped := color.New(color.FgRed).Sprint("Stopped")
	starting := color.New(color.FgYellow).Sprint("Starting")
	exited := color.New(color.FgRed).Sprint("Exited")
	unknown := color.New(color.FgCyan).Sprint("Unknown")

	for name, svcConfig := range cfg.Servers {
		status, err := mgr.GetServerStatus(name)
		if err != nil {
			status = "Unknown"
		}

		// Get detailed container status if available
		detailedStatus := ""
		projectName := filepath.Base(configFile)
		containerName := fmt.Sprintf("%s-%s", projectName, name)
		if info, exists := containerInfo[containerName]; exists {
			detailedStatus = info
		}

		// Determine status display
		var statusStr string
		if status == "running" {
			statusStr = running
		} else if status == "starting" {
			statusStr = starting
		} else if strings.HasPrefix(status, "exited") {
			exitCode := strings.TrimPrefix(status, "exited")
			if exitCode != "" && exitCode != "(0)" {
				statusStr = color.New(color.FgRed).Sprintf("Exited%s", exitCode)
			} else {
				statusStr = exited
			}
		} else if status == "unknown" {
			statusStr = unknown
		} else {
			statusStr = stopped
		}

		// Add detailed status as a suffix if available
		if detailedStatus != "" && status != "running" {
			statusStr = fmt.Sprintf("%s (%s)", statusStr, detailedStatus)
		}

		serverType := "process"
		containerId := "-"
		if svcConfig.Image != "" {
			serverType = "container"

			// Get container ID from manager if available
			if serverInstance, exists := mgr.GetServerInstance(name); exists && serverInstance.ContainerID != "" {
				containerId = serverInstance.ContainerID[:12] // Short container ID
			}
		}

		capabilities := strings.Join(svcConfig.Capabilities, ", ")
		if capabilities == "" {
			capabilities = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, statusStr, serverType, containerId, capabilities)
	}

	w.Flush()
	return nil
}

func getDetailedContainerInfo(docker *container.DockerRuntime) (map[string]string, error) {
	// Get all containers (including stopped ones)
	cmd := exec.Command(docker.GetExecPath(), "ps", "-a", "--format", "{{.Names}}|{{.Status}}|{{.ID}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}

	info := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		name := parts[0]
		status := parts[1]
		info[name] = status
	}

	return info, nil
}

// Logs shows logs for one or more servers
func Logs(configFile string, serverNames []string, follow bool) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	// If no server names provided, show logs for all servers
	if len(serverNames) == 0 {
		serverNames = make([]string, 0, len(cfg.Servers))
		for name := range cfg.Servers {
			serverNames = append(serverNames, name)
		}
	}
	// Validate server names
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			return fmt.Errorf("server '%s' not found in configuration", name)
		}
	}
	// Show logs for each server
	for _, name := range serverNames {
		if len(serverNames) > 1 {
			fmt.Printf("=== Logs for server '%s' ===\n", name)
		}
		if err := mgr.ShowLogs(name, follow); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to show logs for server '%s': %v\n", name, err)
		}
		if len(serverNames) > 1 && !follow {
			fmt.Println() // Add newline between different server logs
		}
	}
	return nil
}

// Validate validates the compose file
func Validate(configFile string) error {
	_, err := config.LoadConfig(configFile)
	if err != nil {
		return err
	}
	fmt.Println("Configuration file is valid")
	return nil
}

// getServersToStart returns the list of servers to start
// If serverNames is empty, returns all servers. Otherwise, returns only the specified servers.
func getServersToStart(cfg *config.ComposeConfig, serverNames []string) []string {
	if len(serverNames) == 0 {
		// Start all servers
		allServers := make([]string, 0, len(cfg.Servers))
		for name := range cfg.Servers {
			allServers = append(allServers, name)
		}
		return allServers
	}
	// Start only specified servers and their dependencies
	result := make(map[string]bool)
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: server '%s' not found in configuration\n", name)
			continue
		}
		result[name] = true
		// Add dependencies
		addDependencies(cfg, name, result)
	}
	// Convert map to slice
	servers := make([]string, 0, len(result))
	for name := range result {
		servers = append(servers, name)
	}
	return servers
}

// addDependencies recursively adds dependencies of a server to the result map
func addDependencies(cfg *config.ComposeConfig, serverName string, result map[string]bool) {
	server := cfg.Servers[serverName]
	for _, dep := range server.DependsOn {
		if _, exists := cfg.Servers[dep]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: dependency '%s' not found for server '%s'\n", dep, serverName)
			continue
		}
		if !result[dep] {
			result[dep] = true
			addDependencies(cfg, dep, result)
		}
	}
}

// getDependencyLevels returns a list of dependency levels, where each level contains
// servers that can be started in parallel.
func getDependencyLevels(cfg *config.ComposeConfig, servers []string) [][]string {
	// Create a map to track which servers to include
	include := make(map[string]bool)
	for _, name := range servers {
		include[name] = true
	}
	// Create a dependency graph
	graph := make(map[string][]string)
	inDegree := make(map[string]int)
	// Initialize graphs
	for name := range cfg.Servers {
		if include[name] {
			graph[name] = []string{}
			inDegree[name] = 0
		}
	}
	// Build the dependency graph
	for name, server := range cfg.Servers {
		if !include[name] {
			continue
		}
		for _, dep := range server.DependsOn {
			if !include[dep] {
				continue
			}
			graph[dep] = append(graph[dep], name)
			inDegree[name]++
		}
	}
	// Topological sort using BFS
	var levels [][]string
	for {
		currentLevel := []string{}
		for name, degree := range inDegree {
			if degree == 0 {
				currentLevel = append(currentLevel, name)
			}
		}
		if len(currentLevel) == 0 {
			break
		}
		levels = append(levels, currentLevel)
		// Remove current level nodes from graph
		for _, name := range currentLevel {
			delete(inDegree, name)
			for _, next := range graph[name] {
				inDegree[next]--
			}
		}
	}
	// Check if there's a cycle
	if len(inDegree) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Dependency cycle detected among servers\n")
		// Add remaining servers to a final level
		remainingServers := make([]string, 0, len(inDegree))
		for name := range inDegree {
			remainingServers = append(remainingServers, name)
		}
		levels = append(levels, remainingServers)
	}
	return levels
}
