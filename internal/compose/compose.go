// internal/compose/compose.go
package compose

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/runtime"

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

	fmt.Printf("Starting %d MCP server(s) in parallel...\n", len(serversToStart))

	// Collect all networks needed by servers
	requiredNetworks := collectRequiredNetworks(cfg, serversToStart)

	// Ensure all required networks exist
	if cRuntime.GetRuntimeName() != "none" {
		for networkName := range requiredNetworks {
			networkExists, _ := cRuntime.NetworkExists(networkName)
			if !networkExists {
				fmt.Printf("Network '%s' does not exist, attempting to create it...\n", networkName)
				if err := cRuntime.CreateNetwork(networkName); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to create network '%s': %v. Some inter-server communication might fail.\n", networkName, err)
				} else {
					fmt.Printf("✅ Created network '%s'\n", networkName)
				}
			}
		}
	}

	// Channel to collect results
	type startResult struct {
		serverName string
		err        error
		duration   time.Duration
	}

	results := make(chan startResult, len(serversToStart))
	var wg sync.WaitGroup

	// Start all servers in parallel
	for _, serverName := range serversToStart {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()

			startTime := time.Now()
			fmt.Printf("Processing server '%s'...\n", name)

			serverCfg, exists := cfg.Servers[name]
			if !exists {
				results <- startResult{name, fmt.Errorf("not found in config"), time.Since(startTime)}
				return
			}

			// Log transport mode
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
					fmt.Printf("[i] Server %-30s will start in STDIO mode (no HTTP config detected).\n", name)
				} else if isHTTPIntended || hasHTTPArgs {
					fmt.Printf("[i] Server %-30s will start in HTTP mode.\n", name)
				}
			}

			err := startServerContainer(name, serverCfg, cRuntime)
			duration := time.Since(startTime)
			results <- startResult{name, err, duration}
		}(serverName)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and display results
	var composeErrors []string
	var successfulServers []string
	successCount := 0

	for result := range results {
		if result.err != nil {
			errMsg := fmt.Sprintf("Server '%s' failed to start: %v", result.serverName, result.err)
			composeErrors = append(composeErrors, errMsg)
			fmt.Printf("[✖] Server %-30s Error: %v (%s)\n", result.serverName, result.err, ShortDuration(result.duration))
		} else {
			successCount++
			successfulServers = append(successfulServers, result.serverName)
			fmt.Printf("[✔] Server %-30s Started (%s). Proxy will attempt HTTP connection.\n", result.serverName, ShortDuration(result.duration))
		}
	}

	// Summary
	fmt.Printf("\n=== PARALLEL STARTUP SUMMARY ===\n")
	fmt.Printf("Servers processed: %d\n", len(serversToStart))
	fmt.Printf("Successfully started: %d\n", successCount)
	fmt.Printf("Failed: %d\n", len(composeErrors))

	if len(composeErrors) > 0 {
		fmt.Printf("\nErrors encountered:\n")
		for _, e := range composeErrors {
			fmt.Printf("- %s\n", e)
		}
		if successCount == 0 {
			return fmt.Errorf("failed to start any servers. Check server configurations for HTTP mode and port exposure, and ensure commands are correct for HTTP startup")
		}
	}

	if successCount > 0 {
		// Generate dynamic network description
		networkDesc := generateNetworkDescription(requiredNetworks)
		fmt.Printf("\n✅ Startup completed. %d/%d servers are running.\n", successCount, len(serversToStart))
		fmt.Printf("Servers are accessible%s\n", networkDesc)

		// Show detailed network topology
		showNetworkTopology(cfg, successfulServers)

		fmt.Printf("Use 'mcp-compose down' to stop them.\n")
	}

	return nil
}

// collectRequiredNetworks gathers all networks used by the servers being started
func collectRequiredNetworks(cfg *config.ComposeConfig, serverNames []string) map[string][]string {
	networkToServers := make(map[string][]string)

	for _, serverName := range serverNames {
		serverCfg, exists := cfg.Servers[serverName]
		if !exists {
			continue
		}

		// Skip if using network mode instead of networks
		if serverCfg.NetworkMode != "" {
			continue
		}

		networks := determineServerNetworks(serverCfg)

		// Track which servers use which networks
		for _, network := range networks {
			if networkToServers[network] == nil {
				networkToServers[network] = make([]string, 0)
			}
			networkToServers[network] = append(networkToServers[network], serverName)
		}
	}

	return networkToServers
}

// generateNetworkDescription creates a human-readable description of network configuration
func generateNetworkDescription(networkToServers map[string][]string) string {
	if len(networkToServers) == 0 {
		return " via localhost (for process-based servers) or host networking"
	}

	if len(networkToServers) == 1 {
		for networkName := range networkToServers {
			if networkName == "host" {
				return " via host networking"
			}
			return fmt.Sprintf(" via Docker network '%s'", networkName)
		}
	}

	// Multiple networks
	networks := make([]string, 0, len(networkToServers))
	for networkName := range networkToServers {
		if networkName == "host" {
			networks = append(networks, "host networking")
		} else {
			networks = append(networks, fmt.Sprintf("'%s'", networkName))
		}
	}

	return fmt.Sprintf(" via Docker networks: %s", strings.Join(networks, ", "))
}

// showNetworkTopology displays which servers are on which networks
func showNetworkTopology(cfg *config.ComposeConfig, serversStarted []string) {
	fmt.Printf("\n=== NETWORK TOPOLOGY ===\n")

	networkToServers := make(map[string][]string)

	for _, serverName := range serversStarted {
		serverCfg, exists := cfg.Servers[serverName]
		if !exists {
			continue
		}

		var networks []string
		if serverCfg.NetworkMode != "" {
			networks = []string{fmt.Sprintf("mode:%s", serverCfg.NetworkMode)}
		} else {
			networks = determineServerNetworks(serverCfg)
		}

		for _, network := range networks {
			if networkToServers[network] == nil {
				networkToServers[network] = make([]string, 0)
			}
			networkToServers[network] = append(networkToServers[network], serverName)
		}
	}

	if len(networkToServers) == 0 {
		fmt.Printf("No network information available (process-based servers)\n")
		return
	}

	for networkName, servers := range networkToServers {
		fmt.Printf("Network '%s': %s\n", networkName, strings.Join(servers, ", "))
	}
}

// determineServerNetworks determines which networks a server should join
func determineServerNetworks(serverCfg config.ServerConfig) []string {
	// If NetworkMode is set, don't use Networks (they're mutually exclusive)
	if serverCfg.NetworkMode != "" {
		return nil
	}

	// Start with configured networks
	networks := make([]string, 0)
	if len(serverCfg.Networks) > 0 {
		networks = append(networks, serverCfg.Networks...)
	}

	// Ensure default network is included unless explicitly using custom networks only
	hasDefaultNetwork := false
	for _, net := range networks {
		if net == "mcp-net" {
			hasDefaultNetwork = true
			break
		}
	}

	if !hasDefaultNetwork && len(networks) == 0 {
		// No networks specified, use default
		networks = append(networks, "mcp-net")
	} else if !hasDefaultNetwork && len(serverCfg.Networks) > 0 {
		// Custom networks specified, but ensure connectivity with other MCP services
		// Add mcp-net for proxy connectivity unless user explicitly excluded it
		networks = append(networks, "mcp-net")
	}

	// Remove duplicates
	uniqueNetworks := make([]string, 0, len(networks))
	seen := make(map[string]bool)
	for _, network := range networks {
		if !seen[network] {
			uniqueNetworks = append(uniqueNetworks, network)
			seen[network] = true
		}
	}

	return uniqueNetworks
}

// isContainerServer determines if a server should run as a container
func isContainerServer(serverCfg config.ServerConfig) bool {
	return serverCfg.Image != "" || serverCfg.Runtime != ""
}

func startServerContainer(serverName string, serverCfg config.ServerConfig, cRuntime container.Runtime) error {
	containerName := fmt.Sprintf("mcp-compose-%s", serverName)
	envVars := config.MergeEnv(serverCfg.Env, map[string]string{"MCP_SERVER_NAME": serverName})

	isSocatHostedStdio := serverCfg.StdioHosterPort > 0
	isHttp := serverCfg.Protocol == "http" || serverCfg.HttpPort > 0

	dockerCommandForContainer := serverCfg.Command
	dockerArgsForContainer := serverCfg.Args

	if isSocatHostedStdio {
		fmt.Printf("Starting container '%s' for server '%s' (Socat STDIO Hoster mode on internal port %d).\n",
			containerName, serverName, serverCfg.StdioHosterPort)
		envVars["MCP_SOCAT_INTERNAL_PORT"] = strconv.Itoa(serverCfg.StdioHosterPort)
	} else if isHttp {
		fmt.Printf("Starting container '%s' for server '%s' (HTTP mode on internal port %d).\n",
			containerName, serverName, serverCfg.HttpPort)
		if serverCfg.HttpPort > 0 {
			envVars["MCP_HTTP_PORT"] = strconv.Itoa(serverCfg.HttpPort)
		}
		envVars["MCP_TRANSPORT"] = "http"
	} else {
		fmt.Printf("Starting container '%s' for server '%s' (Direct STDIO mode).\n",
			containerName, serverName)
	}

	// Add other env vars from config
	for k, v := range serverCfg.Env {
		if _, exists := envVars[k]; !exists {
			envVars[k] = v
		}
	}

	// Handle networks more intelligently
	networks := determineServerNetworks(serverCfg)

	// Log network configuration
	if serverCfg.NetworkMode != "" {
		fmt.Printf("Container '%s' will use network mode: %s\n", containerName, serverCfg.NetworkMode)
	} else if len(networks) == 1 {
		fmt.Printf("Container '%s' will join network: %s\n", containerName, networks[0])
	} else if len(networks) > 1 {
		fmt.Printf("Container '%s' will join networks: %s\n", containerName, strings.Join(networks, ", "))
	}

	opts := &container.ContainerOptions{
		Name:        containerName,
		Image:       serverCfg.Image,
		Build:       serverCfg.Build,
		Command:     dockerCommandForContainer,
		Args:        dockerArgsForContainer,
		Env:         envVars,
		Pull:        serverCfg.Pull,
		Volumes:     serverCfg.Volumes,
		Ports:       serverCfg.Ports,
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
		return nil
	}

	fmt.Println("Stopping MCP servers...")
	var serversToStop []string
	if len(serverNames) > 0 {
		serversToStop = serverNames
	} else {
		for name, srvCfg := range cfg.Servers {
			if srvCfg.Image != "" || srvCfg.Runtime != "" {
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
		srvCfg, exists := cfg.Servers[serverName]
		if !exists || (srvCfg.Image == "" && srvCfg.Runtime == "") {
			fmt.Printf("Skipping '%s' as it's not defined as a containerized server.\n", serverName)
			continue
		}

		containerName := fmt.Sprintf("mcp-compose-%s", serverName)
		if err := cRuntime.StopContainer(containerName); err != nil {
			if !strings.Contains(err.Error(), "No such container") {
				composeErrors = append(composeErrors, fmt.Sprintf("Failed to stop %s: %v", serverName, err))
				fmt.Printf("[✖] Server %-30s Error stopping: %v\n", serverName, err)
			} else {
				fmt.Printf("[✔] Server %-30s (container %s) already stopped or removed.\n", serverName, containerName)
				successCount++
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
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to start")
	}
	fmt.Printf("Starting specified MCP servers (and their dependencies): %v\n", serverNames)
	return Up(configFile, serverNames)
}

func Stop(configFile string, serverNames []string) error {
	if len(serverNames) == 0 {
		return fmt.Errorf("no server names specified to stop")
	}
	return Down(configFile, serverNames)
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
		identifier := fmt.Sprintf("mcp-compose-%s", serverName)
		var statusStr string // Declare without initial assignment
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
		} else {
			identifier = fmt.Sprintf("process-%s", serverName)
			statusStr = processColor("Process")
		}

		transport := "stdio (default)"
		if srvConfig.Protocol == "http" {
			transport = fmt.Sprintf("http (:%d)", srvConfig.HttpPort)
		} else if srvConfig.HttpPort > 0 {
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
		if strings.HasPrefix(arg, "--port") {
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
			if srvCfg.Image != "" || srvCfg.Runtime != "" {
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
		if len(serversToLog) > 1 && i > 0 && !follow {
			fmt.Println("\n---")
		}
		if len(serversToLog) > 1 || len(serverNames) > 1 {
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
			adj[dep] = append(adj[dep], name)
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
		fmt.Fprintf(os.Stderr, "Warning: Cycle detected in server dependencies or some servers are unreachable. Startup order might be incorrect.\n")
		return buildFallbackOrder(cfg, targetServers)
	}

	// Filter the sorted order to include only target servers and their transitive dependencies
	finalOrderMap := make(map[string]bool)
	for _, name := range targetServers {
		if _, exists := cfg.Servers[name]; !exists {
			fmt.Fprintf(os.Stderr, "Warning: Specified server '%s' not found in configuration, skipping.\n", name)
			continue
		}
		addDependenciesRecursive(cfg, name, finalOrderMap)
	}

	finalSortedOrder := make([]string, 0, len(finalOrderMap))
	for _, name := range sortedOrder {
		if finalOrderMap[name] {
			finalSortedOrder = append(finalSortedOrder, name)
		}
	}
	return finalSortedOrder
}

func addDependenciesRecursive(cfg *config.ComposeConfig, serverName string, result map[string]bool) {
	if result[serverName] {
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

	var processingList []string
	for name := range toProcessSet {
		processingList = append(processingList, name)
	}

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
				if toProcessSet[depName] && !added[depName] {
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
			fmt.Fprintf(os.Stderr, "Error: Unable to resolve full dependency order, possibly due to a cycle or unstartable dependency. Remaining servers:\n")
			for _, name := range processingList {
				if !added[name] {
					fmt.Fprintf(os.Stderr, "- %s\n", name)
				}
			}
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
