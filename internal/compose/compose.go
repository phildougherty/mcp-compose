// internal/compose/compose.go
package compose

import (
	"fmt"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/server"
	"os"
	"os/exec"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	// Import time for delays if needed for debugging
	"github.com/fatih/color"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
)

// Up starts all servers defined in the compose file or only the specified servers
func Up(configFile string, serverNames []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}

	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime) // Pass cfg and cRuntime
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	serversToStart := getServersToStart(cfg, serverNames)
	if len(serversToStart) == 0 {
		fmt.Println("No servers selected or defined to start.")
		return nil
	}

	fmt.Printf("Starting %d MCP server(s)...\n", len(serversToStart))

	levels := getDependencyLevels(cfg, serversToStart)

	totalServers := len(serversToStart)
	startedCount := 0
	var overallErrors []string

	// For Docker Compose like output
	serverStatuses := make(map[string]string)
	var serverOrder []string // To print in a somewhat consistent order

	for i, level := range levels {
		fmt.Printf("INFO: Starting dependency level %d: %v\n", i+1, level)
		var wg sync.WaitGroup

		// Use a channel to get status updates from goroutines
		statusCh := make(chan struct {
			name     string
			err      error
			duration time.Duration
		}, len(level))

		for _, name := range level {
			serverOrder = append(serverOrder, name) // Keep track of order
			serverStatuses[name] = "Starting"       // Initial status

			wg.Add(1)
			serverNameToStart := name
			go func() {
				defer wg.Done()
				startTime := time.Now()
				startErr := mgr.StartServer(serverNameToStart)
				duration := time.Since(startTime)
				statusCh <- struct {
					name     string
					err      error
					duration time.Duration
				}{serverNameToStart, startErr, duration}
			}()
		}

		// Wait for all goroutines in this level to send their status
		// but also periodically update the display (more advanced, for now just wait then print)

		// Simple wait and collect
		for j := 0; j < len(level); j++ {
			update := <-statusCh
			if update.err != nil {
				serverStatuses[update.name] = fmt.Sprintf("Error (%s)", ShortDuration(update.duration))
				errMsg := fmt.Sprintf("Error starting '%s': %v", update.name, update.err)
				fmt.Printf("%s[✖] Server %-30s %s (%v)%s\n", colorRed, update.name, "Error", update.err, colorReset)
				overallErrors = append(overallErrors, errMsg)
			} else {
				serverStatuses[update.name] = fmt.Sprintf("Started (%s)", ShortDuration(update.duration))
				fmt.Printf("%s[✔] Server %-30s %s%s\n", colorGreen, update.name, "Started", colorReset)
				startedCount++
			}
		}
		close(statusCh)
		wg.Wait() // Ensure all goroutines truly finished, though channel sync mostly covers this

		if len(overallErrors) > 0 && i < len(levels)-1 {
			// If errors in a level, perhaps don't continue to next levels?
			// Docker Compose often tries to start all independent services.
			fmt.Fprintf(os.Stderr, "%sWARNING: Errors encountered in level %d, subsequent levels may fail or be skipped.%s\n", colorYellow, i+1, colorReset)
			// To stop: return fmt.Errorf("failed to start all servers in level %d: %s", i+1, strings.Join(overallErrors, "; "))
		}
	}

	fmt.Println("\n--- Summary ---")
	for _, name := range serverOrder {
		status := serverStatuses[name]
		if strings.HasPrefix(status, "Error") {
			fmt.Printf("%s[✖] Server %-30s %s%s\n", colorRed, name, status, colorReset)
		} else {
			fmt.Printf("%s[✔] Server %-30s %s%s\n", colorGreen, name, status, colorReset)
		}
	}

	if len(overallErrors) > 0 {
		fmt.Fprintf(os.Stderr, "\n%sFinished with errors (%d/%d started):%s\n", colorRed, startedCount, totalServers, colorReset)
		for _, e := range overallErrors {
			fmt.Fprintf(os.Stderr, "- %s\n", e)
		}
		return fmt.Errorf("one or more servers failed to start")
	}

	fmt.Printf("\n%sAll %d/%d servers started successfully.%s\n", colorGreen, startedCount, totalServers, colorReset)
	return nil
}

// ShortDuration formats time.Duration for concise output
func ShortDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// Down stops and removes all servers defined in the config
func Down(configFile string, serverNames []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
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

	var serversToStop []string
	if len(serverNames) > 0 {
		// Stop only specified servers (and their dependents if you implement that logic)
		// For now, let's assume Stop specified means just those.
		// For a full "down" of specified, one might want to find all dependents too.
		// However, the original Down seemed to imply all servers in the config if serverNames is empty.
		fmt.Printf("DEBUG: Specified servers to stop: %v\n", serverNames)
		serversToStop = getServersToStop(cfg, serverNames) // You'd need a getServersToStop similar to getServersToStart if you want dependency handling
	} else {
		// Stop all servers defined in the config
		for name := range cfg.Servers {
			serversToStop = append(serversToStop, name)
		}
		fmt.Println("DEBUG: Stopping all configured servers.")
	}

	if len(serversToStop) == 0 {
		fmt.Println("No servers selected or defined to stop.")
		return nil
	}

	levels := getDependencyLevels(cfg, serversToStop) // Get levels for stopping

	for i := len(levels) - 1; i >= 0; i-- { // Stop in reverse dependency order
		level := levels[i]
		fmt.Printf("--- Stopping dependency level %d (original level %d in start order), servers: %v ---\n", len(levels)-i, i+1, level)
		var wg sync.WaitGroup
		errCh := make(chan error, len(level))

		for _, name := range level {
			wg.Add(1)
			serverNameToStop := name
			go func() {
				defer wg.Done()
				fmt.Printf("GOROUTINE: Attempting to stop server '%s'...\n", serverNameToStop)
				if stopErr := mgr.StopServer(serverNameToStop); stopErr != nil {
					// For "down", we usually want to continue even if one server fails to stop.
					fmt.Printf("GOROUTINE WARNING: Failed to stop server '%s': %v\n", serverNameToStop, stopErr)
					errCh <- fmt.Errorf("failed to stop server '%s': %w", serverNameToStop, stopErr) // Still send to channel for logging potential aggregated error
				} else {
					fmt.Printf("GOROUTINE SUCCESS: Server '%s' reported as stopped.\n", serverNameToStop)
				}
			}()
		}
		wg.Wait()
		close(errCh)

		var errsThisLevel []string
		for e := range errCh {
			errsThisLevel = append(errsThisLevel, e.Error())
			fmt.Fprintf(os.Stderr, "Warning during shutdown of level: %s\n", e.Error()) // Log individually
		}
		if len(errsThisLevel) > 0 {
			// Decide if you want Down to return an error if any server fails to stop
			// For now, just logging warnings.
		}
		fmt.Printf("--- Dependency level %d servers processed for shutdown. ---\n", len(levels)-i)
	}
	fmt.Println("All selected MCP servers processed for shutdown.")
	return nil
}

// Start starts specific servers (and their dependencies)
func Start(configFile string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to start")
	}
	fmt.Printf("Starting specified MCP servers: %v (and their dependencies)\n", serverNames)
	return Up(configFile, serverNames) // Up handles starting specified servers + deps
}

// Stop stops specific servers (does not handle dependency stopping, relies on manager.StopServer)
func Stop(configFile string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to stop")
	}
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}
	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	fmt.Printf("Stopping specified MCP servers: %v\n", serverNames)
	var encounteredErrors []string
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			errMsg := fmt.Sprintf("server '%s' not found in configuration, skipping stop", name)
			fmt.Fprintf(os.Stderr, "Warning: %s\n", errMsg)
			encounteredErrors = append(encounteredErrors, errMsg)
			continue
		}
		fmt.Printf("Attempting to stop server '%s'...\n", name)
		if err := mgr.StopServer(name); err != nil {
			errMsg := fmt.Sprintf("failed to stop server '%s': %v", name, err)
			fmt.Fprintf(os.Stderr, "Warning: %s\n", errMsg)
			encounteredErrors = append(encounteredErrors, errMsg)
		} else {
			fmt.Printf("Server '%s' reported as stopped.\n", name)
		}
	}
	fmt.Println("Requested MCP servers processed for shutdown.")
	if len(encounteredErrors) > 0 {
		return fmt.Errorf("encountered errors while stopping servers: %s", strings.Join(encounteredErrors, "; "))
	}
	return nil
}

// List lists all defined servers and their status
func List(configFile string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to detect container runtime, status for containerized servers may be inaccurate: %v\n", err)
	}
	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	containerInfo := make(map[string]string)
	if cRuntime != nil && cRuntime.GetRuntimeName() == "docker" {
		if dockerRuntime, ok := cRuntime.(*container.DockerRuntime); ok {
			info, err := getDetailedContainerInfo(dockerRuntime)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to get detailed Docker container info: %v\n", err)
			} else {
				containerInfo = info
			}
		}
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.TabIndent)
	fmt.Fprintln(w, "SERVER NAME\tEXPECTED STATUS\tRUNTIME TYPE\tCONTAINER IDENTIFIER\tCAPABILITIES")

	runningColor := color.New(color.FgGreen).SprintFunc()
	stoppedColor := color.New(color.FgRed).SprintFunc()
	startingColor := color.New(color.FgYellow).SprintFunc()
	exitedColor := color.New(color.FgRed).SprintFunc()
	unknownColor := color.New(color.FgCyan).SprintFunc()

	for serverKeyName, svcConfig := range cfg.Servers {
		status, _ := mgr.GetServerStatus(serverKeyName)

		fixedIdentifier := fmt.Sprintf("mcp-compose-%s", serverKeyName)
		detailedDockerStatus := ""
		if info, exists := containerInfo[fixedIdentifier]; exists {
			detailedDockerStatus = info
		}

		var statusStr string
		switch status {
		case "running":
			statusStr = runningColor("Running")
		case "starting":
			statusStr = startingColor("Starting")
		case "stopped":
			statusStr = stoppedColor("Stopped")
		default:
			if strings.HasPrefix(status, "exited") {
				statusStr = exitedColor(status)
			} else {
				statusStr = unknownColor(status)
			}
		}

		if detailedDockerStatus != "" && svcConfig.Image != "" {
			if !strings.Contains(strings.ToLower(detailedDockerStatus), strings.ToLower(status)) && status != "unknown" {
				statusStr += fmt.Sprintf(" (Docker: %s)", detailedDockerStatus)
			} else if status == "unknown" && detailedDockerStatus != "" {
				// If our GetServerStatus is unknown, but docker ps gives something, show that.
				statusStr = fmt.Sprintf("%s (Docker: %s)", unknownColor("Unknown"), detailedDockerStatus)
			}
		}

		serverType := "process"
		displayIdentifier := fixedIdentifier // Default to fixed identifier

		if svcConfig.Image != "" {
			serverType = "container"
			// Optionally display runtime ID if available and different, or in addition
			// if serverInstance, exists := mgr.GetServerInstance(serverKeyName); exists && serverInstance.ContainerID != "" && serverInstance.ContainerID != fixedIdentifier {
			//    displayIdentifier += fmt.Sprintf(" (ID: %s)", serverInstance.ContainerID[:12])
			// }
		}

		capabilities := strings.Join(svcConfig.Capabilities, ", ")
		if capabilities == "" {
			capabilities = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			serverKeyName, statusStr, serverType, displayIdentifier, capabilities)
	}
	w.Flush()
	return nil
}

func getDetailedContainerInfo(dockerRuntime *container.DockerRuntime) (map[string]string, error) {
	cmd := exec.Command(dockerRuntime.GetExecPath(), "ps", "-a", "--format", "{{.Names}}|{{.Status}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("docker ps command failed: %w; output: %s", err, string(output))
	}

	info := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			status := strings.TrimSpace(parts[1])
			info[name] = status
		}
	}
	return info, nil
}

func Logs(configFile string, serverNames []string, follow bool) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}
	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}

	var serversToLog []string
	if len(serverNames) == 0 {
		for name := range cfg.Servers {
			serversToLog = append(serversToLog, name)
		}
		if len(serversToLog) == 0 {
			fmt.Println("No servers defined in configuration to show logs for.")
			return nil
		}
	} else {
		for _, name := range serverNames {
			if _, exists := cfg.Servers[name]; !exists {
				fmt.Fprintf(os.Stderr, "Warning: server '%s' not found in configuration, skipping logs.\n", name)
			} else {
				serversToLog = append(serversToLog, name)
			}
		}
		if len(serversToLog) == 0 {
			fmt.Println("None of the specified servers were found in configuration.")
			return nil
		}
	}

	for i, name := range serversToLog {
		if len(serversToLog) > 1 {
			fmt.Printf("=== Logs for server '%s' ===\n", name)
		}
		if err := mgr.ShowLogs(name, follow); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to show logs for server '%s': %v\n", name, err)
		}
		if len(serversToLog) > 1 && i < len(serversToLog)-1 && !follow {
			fmt.Println("\n---")
		}
	}
	return nil
}

func Validate(configFile string) error {
	_, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("configuration file '%s' is invalid: %w", configFile, err)
	}
	fmt.Printf("Configuration file '%s' is valid.\n", configFile)
	return nil
}

func getServersToStart(cfg *config.ComposeConfig, serverNames []string) []string {
	if len(serverNames) == 0 {
		allServers := make([]string, 0, len(cfg.Servers))
		for name := range cfg.Servers {
			allServers = append(allServers, name)
		}
		return allServers
	}

	toProcess := make(map[string]bool)
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: specified server '%s' not found in configuration, skipping.\n", name)
			continue
		}
		addDependenciesRecursive(cfg, name, toProcess)
	}

	result := make([]string, 0, len(toProcess))
	for name := range toProcess {
		result = append(result, name)
	}
	return result
}

func addDependenciesRecursive(cfg *config.ComposeConfig, serverName string, result map[string]bool) {
	if result[serverName] {
		return
	}
	result[serverName] = true

	serverConf, exists := cfg.Servers[serverName]
	if !exists {
		return // Should have been caught by caller
	}

	for _, depName := range serverConf.DependsOn {
		if _, depExists := cfg.Servers[depName]; !depExists {
			fmt.Fprintf(os.Stderr, "Warning: dependency '%s' for server '%s' not found in configuration, skipping dependency.\n", depName, serverName)
			continue
		}
		addDependenciesRecursive(cfg, depName, result)
	}
}

func getDependencyLevels(cfg *config.ComposeConfig, serversToOrder []string) [][]string {
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	serverSet := make(map[string]bool)
	for _, s := range serversToOrder {
		serverSet[s] = true
		inDegree[s] = 0
		graph[s] = []string{}
	}

	for _, serverName := range serversToOrder {
		serverConf, exists := cfg.Servers[serverName]
		if !exists {
			continue
		}
		for _, depName := range serverConf.DependsOn {
			if _, careAboutDep := serverSet[depName]; careAboutDep {
				graph[depName] = append(graph[depName], serverName)
				inDegree[serverName]++
			}
		}
	}

	var levels [][]string
	queue := make([]string, 0)

	for _, serverName := range serversToOrder {
		if inDegree[serverName] == 0 {
			queue = append(queue, serverName)
		}
	}

	for len(queue) > 0 {
		currentLevelSize := len(queue)
		currentLevel := make([]string, currentLevelSize)
		copy(currentLevel, queue[:currentLevelSize]) // Copy current queue to currentLevel
		levels = append(levels, currentLevel)

		queue = queue[currentLevelSize:] // "Dequeue" processed items

		for _, u := range currentLevel {
			// Note: We don't delete from inDegree map here in Kahn's,
			// we just stop considering nodes once they are in a level.
			// The critical part is decrementing neighbors' in-degrees.
			for _, v := range graph[u] {
				inDegree[v]--
				if inDegree[v] == 0 {
					queue = append(queue, v)
				}
			}
		}
	}

	// After the loop, check if all serversToOrder were processed.
	// If some still have inDegree > 0 (or aren't in levels because they were never 0), there's a cycle.
	processedCount := 0
	for _, level := range levels {
		processedCount += len(level)
	}

	if processedCount < len(serversToOrder) {
		cycleNodes := []string{}
		for serverName, degree := range inDegree {
			// This check is a bit tricky because nodes could be in inDegree but processed.
			// A better cycle check: if processedCount != len(serversToOrder)
			// then iterate serversToOrder and if a server isn't in any level, it's part of a cycle or un reachable.
			// For simplicity, just report the inDegree map if it's not empty.
			isProcessed := false
			for _, level := range levels {
				for _, nodeInLevel := range level {
					if nodeInLevel == serverName {
						isProcessed = true
						break
					}
				}
				if isProcessed {
					break
				}
			}
			if !isProcessed && degree > 0 { // Only consider nodes that are part of the problem
				cycleNodes = append(cycleNodes, fmt.Sprintf("%s(degree:%d)", serverName, degree))
			}
		}
		if len(cycleNodes) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: Dependency cycle detected or unreachable dependencies involving servers: %v. These servers might not start/stop in the intended order.\n", cycleNodes)
		}
	}

	return levels
}

// getServersToStop determines server stop order. For simple stop, it might be the reverse of start.
// For targeted stop, it might just be the list itself, or you might want to stop dependents.
// This is a placeholder if specific stop order logic is needed beyond what `Down` does.
func getServersToStop(cfg *config.ComposeConfig, serverNames []string) []string {
	// For now, if specific servers are named, just return that list.
	// `Down` already handles full reverse dependency order.
	// If `stop serverX` should also stop servers that depend on X, this needs more logic.
	if len(serverNames) > 0 {
		return serverNames
	}
	// If no names, implies all servers (handled by Down directly)
	allServers := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		allServers = append(allServers, name)
	}
	return allServers
}
