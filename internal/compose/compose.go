// internal/compose/compose.go
package compose

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"

	// server import might not be needed if compose commands directly use container runtime
	// "mcpcompose/internal/server"

	"github.com/fatih/color"
)

func Up(configFile string, serverNames []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	serversToStart := getServersToStart(cfg, serverNames)
	if len(serversToStart) == 0 {
		fmt.Println("No servers selected or defined to start.")
		return nil
	}

	fmt.Printf("Starting %d MCP server(s) with mixed transports...\n", len(serversToStart))
	var composeErrors []string // Renamed to avoid conflict with the 'errors' package
	successCount := 0

	// Ensure mcp-net network exists before starting any server
	if cRuntime.GetRuntimeName() != "none" {
		networkExists, _ := cRuntime.NetworkExists("mcp-net")
		if !networkExists {
			fmt.Println("Default network 'mcp-net' does not exist, attempting to create it...")
			if err := cRuntime.CreateNetwork("mcp-net"); err != nil {
				// This is a significant issue if containers need to communicate.
				// Depending on strictness, you might want to return an error here.
				fmt.Fprintf(os.Stderr, "Warning: Failed to create 'mcp-net' network: %v. Inter-server communication might fail.\n", err)
			}
		}
	}

	for _, serverName := range serversToStart {
		fmt.Printf("Processing server '%s'...\n", serverName)
		startTime := time.Now()
		serverCfg, exists := cfg.Servers[serverName]
		if !exists {
			composeErrors = append(composeErrors, fmt.Sprintf("Server '%s' not found in config", serverName))
			fmt.Printf("[✖] Server %-30s Error: not found in config\n", serverName)
			continue
		}

		// For HTTP transport, ensure the server configuration is appropriate
		if serverCfg.Image != "" {
			isHTTPIntended := serverCfg.Protocol == "http" || serverCfg.HttpPort > 0
			hasHTTPArgs := false
			for _, arg := range serverCfg.Args {
				if strings.Contains(strings.ToLower(arg), "http") || strings.Contains(arg, "--port") {
					hasHTTPArgs = true
					break
				}
			}

			if !isHTTPIntended && !hasHTTPArgs {
				fmt.Printf("[i] Server %-30s will start in STDIO mode (no HTTP config detected).\n", serverName)
			} else if isHTTPIntended || hasHTTPArgs {
				fmt.Printf("[i] Server %-30s will start in HTTP mode.\n", serverName)
			}
		}

		err := startServerContainer(serverName, serverCfg, cRuntime) // This function now handles both container and process servers
		duration := time.Since(startTime)
		if err != nil {
			errMsg := fmt.Sprintf("Server '%s' failed to start: %v", serverName, err)
			composeErrors = append(composeErrors, errMsg)
			fmt.Printf("[✖] Server %-30s Error: %v (%s)\n", serverName, err, ShortDuration(duration))
		} else {
			successCount++
			// Brief pause to allow server to initialize its HTTP listener
			time.Sleep(2 * time.Second)
			fmt.Printf("[✔] Server %-30s Started (%s). Proxy will attempt HTTP connection.\n", serverName, ShortDuration(duration))
		}
	}

	fmt.Printf("\n=== HTTP STARTUP SUMMARY ===\n")
	fmt.Printf("Servers processed: %d\n", len(serversToStart))
	fmt.Printf("Successfully started: %d\n", successCount)
	fmt.Printf("Failed: %d\n", len(composeErrors))

	if len(composeErrors) > 0 {
		fmt.Printf("\nErrors encountered:\n")
		for _, e := range composeErrors {
			fmt.Printf("- %s\n", e)
		}
		return fmt.Errorf("failed to start some servers. Check server configurations for HTTP mode and port exposure, and ensure commands are correct for HTTP startup")
	}

	if successCount > 0 {
		fmt.Printf("\n✅ Startup completed. %d/%d servers are attempting to run in HTTP mode.\n", successCount, len(serversToStart))
		fmt.Printf("The proxy will connect to them over HTTP via the 'mcp-net' Docker network (for containers) or localhost (for processes).\n")
		fmt.Printf("Use 'mcp-compose down' to stop them.\n")
	}
	return nil
}

func startServerContainer(serverName string, serverCfg config.ServerConfig, cRuntime container.Runtime) error {
	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	envVars := config.MergeEnv(serverCfg.Env, map[string]string{"MCP_SERVER_NAME": serverName}) // Pass MCP_SERVER_NAME

	isSocatHostedStdio := serverCfg.StdioHosterPort > 0
	isHttp := serverCfg.Protocol == "http" || serverCfg.HttpPort > 0

	dockerCommandForContainer := serverCfg.Command // This will be passed to entrypoint or become CMD
	dockerArgsForContainer := serverCfg.Args       // These too

	if isSocatHostedStdio {
		fmt.Printf("Starting container '%s' for server '%s' (Socat STDIO Hoster mode on internal port %d).\n",
			containerName, serverName, serverCfg.StdioHosterPort)
		envVars["MCP_SOCAT_INTERNAL_PORT"] = strconv.Itoa(serverCfg.StdioHosterPort)
		// The serverCfg.Command and serverCfg.Args will be passed to the entrypoint.sh inside the socat hoster image
	} else if isHttp {
		fmt.Printf("Starting container '%s' for server '%s' (HTTP mode on internal port %d).\n",
			containerName, serverName, serverCfg.HttpPort)
		if serverCfg.HttpPort > 0 {
			envVars["MCP_HTTP_PORT"] = strconv.Itoa(serverCfg.HttpPort)
		}
		envVars["MCP_TRANSPORT"] = "http"
	} else { // Plain STDIO (direct exec model - may still be needed for servers not using socat hoster)
		fmt.Printf("Starting container '%s' for server '%s' (Direct STDIO mode).\n",
			containerName, serverName)
		// No special env vars for transport needed for direct STDIO
	}

	// Add other env vars from config
	for k, v := range serverCfg.Env {
		if _, exists := envVars[k]; !exists { // Don't override already set specific vars
			envVars[k] = v
		}
	}

	networks := []string{"mcp-net"}
	for _, net := range serverCfg.Networks {
		isMcpNet := false
		for _, existingNet := range networks {
			if net == existingNet {
				isMcpNet = true
				break
			}
		}
		if !isMcpNet {
			networks = append(networks, net)
		}
	}

	opts := &container.ContainerOptions{
		Name:        containerName,
		Image:       serverCfg.Image,           // If using 'build', this will be the built image name/tag
		Build:       serverCfg.Build,           // Pass build context if defined
		Command:     dockerCommandForContainer, // Becomes CMD in container, args to ENTRYPOINT
		Args:        dockerArgsForContainer,
		Env:         envVars,
		Pull:        serverCfg.Pull,
		Volumes:     serverCfg.Volumes,
		Ports:       serverCfg.Ports, //
		Networks:    networks,
		WorkDir:     serverCfg.WorkDir,
		NetworkMode: serverCfg.NetworkMode,
	}

	_, err := cRuntime.StartContainer(opts)
	if err != nil {
		return fmt.Errorf("failed to start container for server '%s': %w", serverName, err)
	}
	return nil
}

func ShortDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func Down(configFile string, serverNames []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}
	if cRuntime.GetRuntimeName() == "none" {
		fmt.Println("No container runtime detected. 'down' command primarily targets containers.")
		// Optionally, add logic here to stop process-based servers if they are managed by mcp-compose
		return nil
	}

	fmt.Println("Stopping MCP servers...")
	var serversToStop []string
	if len(serverNames) > 0 {
		serversToStop = serverNames
	} else {
		for name, srvCfg := range cfg.Servers {
			if srvCfg.Image != "" || srvCfg.Runtime != "" { // Only target containerized servers
				serversToStop = append(serversToStop, name)
			}
		}
	}

	if len(serversToStop) == 0 {
		fmt.Println("No containerized servers specified or defined to stop.")
		return nil
	}

	successCount := 0
	var composeErrors []string
	for _, serverName := range serversToStop {
		// Ensure server to stop is actually defined as a container
		srvCfg, exists := cfg.Servers[serverName]
		if !exists || (srvCfg.Image == "" && srvCfg.Runtime == "") {
			fmt.Printf("Skipping '%s' as it's not defined as a containerized server.\n", serverName)
			continue
		}

		containerName := fmt.Sprintf("mcp-compose-%s", serverName)
		if err := cRuntime.StopContainer(containerName); err != nil {
			// StopContainer should be idempotent (not error if already stopped/gone)
			// but if it does error for other reasons, log it.
			if !strings.Contains(err.Error(), "No such container") { // Example of specific error to ignore
				composeErrors = append(composeErrors, fmt.Sprintf("Failed to stop %s: %v", serverName, err))
				fmt.Printf("[✖] Server %-30s Error stopping: %v\n", serverName, err)
			} else {
				fmt.Printf("[✔] Server %-30s (container %s) already stopped or removed.\n", serverName, containerName)
				successCount++ // Count as success if goal is "stopped"
			}
		} else {
			successCount++
			fmt.Printf("[✔] Server %-30s (container %s) stopped and removed.\n", serverName, containerName)
		}
	}

	fmt.Printf("\n=== SHUTDOWN SUMMARY ===\n")
	fmt.Printf("Containerized servers processed for shutdown: %d\n", len(serversToStop))
	fmt.Printf("Successfully stopped/ensured stopped: %d\n", successCount)
	fmt.Printf("Failed operations: %d\n", len(composeErrors))
	if len(composeErrors) > 0 {
		fmt.Printf("\nErrors encountered during stop operations:\n")
		for _, e := range composeErrors {
			fmt.Printf("- %s\n", e)
		}
	}
	return nil
}

func Start(configFile string, serverNames []string) error {
	// 'Start' should effectively be an alias for 'Up' for specified servers
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to start")
	}
	fmt.Printf("Starting specified MCP servers (and their dependencies): %v\n", serverNames)
	return Up(configFile, serverNames)
}

func Stop(configFile string, serverNames []string) error {
	// 'Stop' should target specific servers for shutdown.
	// It's simpler than `Down` as it doesn't try to stop *all* if no names given.
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to stop")
	}
	return Down(configFile, serverNames) // Reuse Down's logic for specified servers
}

func List(configFile string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", configFile, err)
	}
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		fmt.Printf("Warning: failed to detect container runtime: %v. Container statuses will be 'Unknown'.\n", err)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SERVER NAME\tSTATUS\tTRANSPORT\tCONTAINER/PROCESS NAME\tPORTS\tCAPABILITIES")

	runningColor := color.New(color.FgGreen).SprintFunc()
	stoppedColor := color.New(color.FgRed).SprintFunc()
	unknownColor := color.New(color.FgYellow).SprintFunc()
	processColor := color.New(color.FgCyan).SprintFunc()

	for serverName, srvConfig := range cfg.Servers {
		identifier := fmt.Sprintf("mcp-compose-%s", serverName) // Default for containers
		statusStr := unknownColor("Unknown")
		isContainer := srvConfig.Image != "" || srvConfig.Runtime != ""

		if isContainer {
			if cRuntime != nil && cRuntime.GetRuntimeName() != "none" {
				rawStatus, statusErr := cRuntime.GetContainerStatus(identifier)
				if statusErr != nil {
					statusStr = stoppedColor("Error/Missing")
				} else {
					switch strings.ToLower(rawStatus) {
					case "running":
						statusStr = runningColor("Running")
					case "exited", "dead", "stopped":
						statusStr = stoppedColor(strings.Title(strings.ToLower(rawStatus)))
					default:
						statusStr = unknownColor(rawStatus)
					}
				}
			} else {
				statusStr = stoppedColor("No Runtime")
			}
		} else { // Process-based
			identifier = fmt.Sprintf("process-%s", serverName) // Or actual PID if managed
			statusStr = processColor("Process")                // `mcp-compose ls` doesn't manage process status directly
		}

		transport := "stdio (default)"
		if srvConfig.Protocol == "http" {
			transport = fmt.Sprintf("http (:%d)", srvConfig.HttpPort)
		} else if srvConfig.HttpPort > 0 { // If http_port is set, assume http
			transport = fmt.Sprintf("http (:%d)", srvConfig.HttpPort)
		} else if serverCfgHasHTTPArg(srvConfig.Args) {
			transport = "http (inferred)"
		}

		ports := "-"
		if len(srvConfig.Ports) > 0 {
			ports = strings.Join(srvConfig.Ports, ", ")
		}

		capabilities := strings.Join(srvConfig.Capabilities, ", ")
		if capabilities == "" {
			capabilities = "-"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			serverName, statusStr, transport, identifier, ports, capabilities)
	}
	w.Flush()
	return nil
}

func serverCfgHasHTTPArg(args []string) bool {
	for i, arg := range args {
		if arg == "--transport" && i+1 < len(args) && strings.ToLower(args[i+1]) == "http" {
			return true
		}
		if strings.HasPrefix(arg, "--port") { // Catches --port=X and --port X
			return true
		}
	}
	return false
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
	if cRuntime.GetRuntimeName() == "none" {
		fmt.Println("No container runtime detected. 'logs' command is for containerized servers.")
		return nil
	}

	var serversToLog []string
	if len(serverNames) == 0 {
		for name, srvCfg := range cfg.Servers {
			if srvCfg.Image != "" || srvCfg.Runtime != "" { // Only containerized
				serversToLog = append(serversToLog, name)
			}
		}
		if len(serversToLog) == 0 {
			fmt.Println("No containerized servers defined in configuration to show logs for.")
			return nil
		}
	} else {
		for _, name := range serverNames {
			srvCfg, exists := cfg.Servers[name]
			if !exists {
				fmt.Fprintf(os.Stderr, "Warning: server '%s' not found in configuration, skipping logs.\n", name)
			} else if srvCfg.Image == "" && srvCfg.Runtime == "" {
				fmt.Fprintf(os.Stdout, "Info: Server '%s' is process-based. View its logs directly.\n", name)
			} else {
				serversToLog = append(serversToLog, name)
			}
		}
		if len(serversToLog) == 0 {
			fmt.Println("None of the specified servers were found or are containerized.")
			return nil
		}
	}

	for i, name := range serversToLog {
		if len(serversToLog) > 1 && i > 0 && !follow { // Add separator if not following and more than one
			fmt.Println("\n---")
		}
		if len(serversToLog) > 1 || len(serverNames) > 1 { // Print header if multiple specified or all
			fmt.Printf("=== Logs for server '%s' ===\n", name)
		}
		containerName := fmt.Sprintf("mcp-compose-%s", name)
		if err := cRuntime.ShowContainerLogs(containerName, follow); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to show logs for server '%s' (container %s): %v\n", name, containerName, err)
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

// getServersToStart determines the order of server startup including dependencies.
// For N servers depending on each other, this ensures a valid topological sort if possible.
func getServersToStart(cfg *config.ComposeConfig, serverNames []string) []string {
	allServerNames := make([]string, 0, len(cfg.Servers))
	for name := range cfg.Servers {
		allServerNames = append(allServerNames, name)
	}

	targetServers := serverNames
	if len(targetServers) == 0 {
		targetServers = allServerNames
	}

	// Build dependency graph
	adj := make(map[string][]string)
	inDegree := make(map[string]int)
	for _, name := range allServerNames {
		adj[name] = []string{}
		inDegree[name] = 0
	}

	for name, srvConfig := range cfg.Servers {
		for _, dep := range srvConfig.DependsOn {
			if _, exists := cfg.Servers[dep]; !exists {
				fmt.Fprintf(os.Stderr, "Warning: Server '%s' depends on '%s', which is not defined. Skipping dependency.\n", name, dep)
				continue
			}
			adj[dep] = append(adj[dep], name) // dep -> name
			inDegree[name]++
		}
	}

	// Initialize queue with nodes having in-degree 0
	queue := make([]string, 0)
	for _, name := range allServerNames {
		if inDegree[name] == 0 {
			queue = append(queue, name)
		}
	}

	var sortedOrder []string
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		sortedOrder = append(sortedOrder, u)

		for _, v := range adj[u] {
			inDegree[v]--
			if inDegree[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if len(sortedOrder) != len(allServerNames) {
		// This indicates a cycle in dependencies.
		// For simplicity, we'll just return the list of target servers potentially with their known deps.
		// A more robust solution would error out or report the cycle.
		fmt.Fprintf(os.Stderr, "Warning: Cycle detected in server dependencies or some servers are unreachable. Startup order might be incorrect.\n")
		// Fallback to a less strict ordering if topo sort fails
		return buildFallbackOrder(cfg, targetServers)
	}

	// Filter the sorted order to include only target servers and their transitive dependencies
	finalOrderMap := make(map[string]bool)
	for _, name := range targetServers {
		if _, exists := cfg.Servers[name]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: Specified server '%s' not found in configuration, skipping.\n", name)
			continue
		}
		addDependenciesRecursive(cfg, name, finalOrderMap) // This marks all deps
	}

	finalSortedOrder := make([]string, 0, len(finalOrderMap))
	for _, name := range sortedOrder { // Iterate in topologically sorted order
		if finalOrderMap[name] { // If this server is needed for the targets
			finalSortedOrder = append(finalSortedOrder, name)
		}
	}
	return finalSortedOrder
}

// addDependenciesRecursive is used by getServersToStart to collect all necessary servers.
func addDependenciesRecursive(cfg *config.ComposeConfig, serverName string, result map[string]bool) {
	if result[serverName] { // Already visited
		return
	}
	result[serverName] = true
	serverConf, exists := cfg.Servers[serverName]
	if !exists {
		return
	}
	for _, depName := range serverConf.DependsOn {
		if _, depExists := cfg.Servers[depName]; !depExists {
			fmt.Fprintf(os.Stderr, "Warning: Dependency '%s' for server '%s' not found. Skipping this dependency.\n", depName, serverName)
			continue
		}
		addDependenciesRecursive(cfg, depName, result)
	}
}

// buildFallbackOrder provides a basic order if topological sort fails (e.g. due to cycle)
func buildFallbackOrder(cfg *config.ComposeConfig, serverNames []string) []string {
	toProcessSet := make(map[string]bool)
	for _, name := range serverNames {
		if _, exists := cfg.Servers[name]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: specified server '%s' not found in configuration, skipping.\n", name)
			continue
		}
		addDependenciesRecursive(cfg, name, toProcessSet)
	}

	fallbackOrder := make([]string, 0, len(toProcessSet))
	// A simple approach: add defined dependencies first, then the rest
	// This is not a perfect topological sort, but provides some ordering.

	// Create a list of all servers to be processed
	var processingList []string
	for name := range toProcessSet {
		processingList = append(processingList, name)
	}

	// Iteratively add servers whose dependencies are met
	added := make(map[string]bool)
	for len(fallbackOrder) < len(processingList) {
		addedThisIteration := 0
		for _, name := range processingList {
			if added[name] {
				continue
			}
			depsMet := true
			srvCfg := cfg.Servers[name]
			for _, depName := range srvCfg.DependsOn {
				if toProcessSet[depName] && !added[depName] { // dep is also in targets and not yet added
					depsMet = false
					break
				}
			}
			if depsMet {
				fallbackOrder = append(fallbackOrder, name)
				added[name] = true
				addedThisIteration++
			}
		}
		if addedThisIteration == 0 && len(fallbackOrder) < len(processingList) {
			// Cycle or missing dependency not in target set but required
			fmt.Fprintf(os.Stderr, "Error: Unable to resolve full dependency order, possibly due to a cycle or unstartable dependency. Remaining servers:\n")
			for _, name := range processingList {
				if !added[name] {
					fmt.Fprintf(os.Stderr, "- %s\n", name)
				}
			}
			// Add remaining unconventionally
			for _, name := range processingList {
				if !added[name] {
					fallbackOrder = append(fallbackOrder, name)
				}
			}
			break
		}
	}
	return fallbackOrder
}
