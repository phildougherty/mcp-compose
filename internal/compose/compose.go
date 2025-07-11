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

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"
	"mcpcompose/internal/logging"
	"mcpcompose/internal/protocol"
	"mcpcompose/internal/runtime"
	"mcpcompose/internal/server"

	"github.com/fatih/color"
)

// Composer orchestrates the entire MCP compose environment
type Composer struct {
	config           *config.ComposeConfig
	manager          *server.Manager
	lifecycleManager *LifecycleManager
	protocolManagers map[string]*ProtocolManagerSet
	logger           *logging.Logger
	mu               sync.RWMutex
}

// ProtocolManagerSet contains all protocol managers for a server
type ProtocolManagerSet struct {
	Progress     *protocol.ProgressManager
	Resource     *protocol.ResourceManager
	Sampling     *protocol.SamplingManager
	Subscription *protocol.SubscriptionManager
	Change       *protocol.ChangeNotificationManager
}

// NewComposer creates a new composer instance
func NewComposer(configPath string) (*Composer, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Use DetectRuntime instead of NewRuntime
	containerRuntime, err := container.DetectRuntime()
	if err != nil {
		return nil, fmt.Errorf("failed to detect container runtime: %w", err)
	}

	mgr, err := server.NewManager(cfg, containerRuntime)
	if err != nil {
		return nil, fmt.Errorf("failed to create server manager: %w", err)
	}

	logger := logging.NewLogger(cfg.Logging.Level)
	lifecycleManager := NewLifecycleManager(cfg, logger, ".")

	composer := &Composer{
		config:           cfg,
		manager:          mgr,
		lifecycleManager: lifecycleManager,
		protocolManagers: make(map[string]*ProtocolManagerSet),
		logger:           logger,
	}

	// Initialize protocol managers for each server
	for serverName := range cfg.Servers {
		composer.protocolManagers[serverName] = &ProtocolManagerSet{
			Progress:     protocol.NewProgressManager(),
			Resource:     protocol.NewResourceManager(),
			Sampling:     protocol.NewSamplingManager(),
			Subscription: protocol.NewSubscriptionManager(),
			Change:       protocol.NewChangeNotificationManager(),
		}

		// Register default transformer
		composer.protocolManagers[serverName].Resource.RegisterTransformer(
			"default",
			&protocol.DefaultTextTransformer{},
		)
	}

	return composer, nil
}

// GetProtocolManagers returns protocol managers for a server
func (c *Composer) GetProtocolManagers(serverName string) *ProtocolManagerSet {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.protocolManagers[serverName]
}

// StartServer starts a specific server with protocol integration
func (c *Composer) StartServer(serverName string) error {
	return c.manager.StartServer(serverName)
}

// StopServer stops a specific server
func (c *Composer) StopServer(serverName string) error {
	return c.manager.StopServer(serverName)
}

// StartAll starts all configured servers
func (c *Composer) StartAll() error {
	for serverName := range c.config.Servers {
		if err := c.StartServer(serverName); err != nil {
			c.logger.Error("Failed to start server %s: %v", serverName, err)
			return err
		}
	}
	return nil
}

// StopAll stops all running servers
func (c *Composer) StopAll() error {
	for serverName := range c.config.Servers {
		if err := c.StopServer(serverName); err != nil {
			c.logger.Warning("Failed to stop server %s: %v", serverName, err)
		}
	}
	return nil
}

// Shutdown gracefully shuts down the composer
func (c *Composer) Shutdown() error {
	c.logger.Info("Shutting down composer...")

	// Stop all servers
	if err := c.StopAll(); err != nil {
		c.logger.Warning("Error stopping servers during shutdown: %v", err)
	}

	// Shutdown protocol managers
	c.mu.Lock()
	for serverName, managers := range c.protocolManagers {
		c.logger.Debug("Cleaning up protocol managers for %s", serverName)
		managers.Subscription.CleanupExpiredSubscriptions(0)
		managers.Change.CleanupInactiveSubscribers(0)
	}
	c.mu.Unlock()

	// Shutdown server manager
	if err := c.manager.Shutdown(); err != nil {
		c.logger.Warning("Error shutting down server manager: %v", err)
	}

	return nil
}

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

			var err error
			if isContainerServer(serverCfg) {
				err = startServerContainer(name, serverCfg, cRuntime)
			} else {
				err = startServerProcess(name, serverCfg)
			}
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
			return fmt.Errorf("failed to start any servers. Check server configurations and ensure commands/images are correct")
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

// collectRequiredNetworks gathers all networks used by the container servers being started
func collectRequiredNetworks(cfg *config.ComposeConfig, serverNames []string) map[string][]string {
	networkToServers := make(map[string][]string)

	for _, serverName := range serverNames {
		serverCfg, exists := cfg.Servers[serverName]
		if !exists {
			continue
		}

		// Only process container servers for network requirements
		if !isContainerServer(serverCfg) {
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
	// If it has an image, it's definitely a container
	if serverCfg.Image != "" {
		return true
	}

	// If it has a build context, it's definitely a container
	if serverCfg.Build.Context != "" {
		return true
	}

	// If it has container-specific configuration, it's a container
	if len(serverCfg.Volumes) > 0 {
		return true
	}

	if len(serverCfg.Networks) > 0 {
		return true
	}

	if serverCfg.NetworkMode != "" {
		return true
	}

	// If it has HTTP/SSE protocol settings, likely a container
	if serverCfg.HttpPort > 0 || serverCfg.StdioHosterPort > 0 {
		return true
	}

	// If it has container security settings, it's a container
	if serverCfg.User != "" || serverCfg.Privileged || len(serverCfg.CapAdd) > 0 || len(serverCfg.CapDrop) > 0 {
		return true
	}

	// If it has resource limits (deploy section), it's a container
	if serverCfg.Deploy.Resources.Limits.CPUs != "" ||
		serverCfg.Deploy.Resources.Limits.Memory != "" ||
		serverCfg.Deploy.Resources.Limits.PIDs > 0 {
		return true
	}

	// If command starts with container-style paths, it's a container
	if strings.HasPrefix(serverCfg.Command, "/app/") {
		return true
	}

	// If it has Docker/container specific environment or settings
	if serverCfg.RestartPolicy != "" || len(serverCfg.SecurityOpt) > 0 {
		return true
	}

	// If none of the above, it's a process-based server
	return false
}

// startServerProcess handles process-based server startup
func startServerProcess(serverName string, serverCfg config.ServerConfig) error {
	fmt.Printf("Starting process '%s' for server '%s'.\n", serverCfg.Command, serverName)

	env := make(map[string]string)
	if serverCfg.Env != nil {
		for k, v := range serverCfg.Env {
			env[k] = v
		}
	}
	// Add standard MCP environment variables
	env["MCP_SERVER_NAME"] = serverName

	proc, err := runtime.NewProcess(serverCfg.Command, serverCfg.Args, runtime.ProcessOptions{
		Env:     env,
		WorkDir: serverCfg.WorkDir,
		Name:    fmt.Sprintf("mcp-compose-%s", serverName),
	})
	if err != nil {
		return fmt.Errorf("failed to create process structure for server '%s': %w", serverName, err)
	}
	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start process for server '%s': %w", serverName, err)
	}

	return nil
}

func ShortDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/constants.NanosecondsToMilliseconds)
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, constants.TableColumnSpacing, ' ', 0)
	if _, err := fmt.Fprintln(w, "SERVER NAME\tSTATUS\tTRANSPORT\tCONTAINER/PROCESS NAME\tPORTS\tCAPABILITIES"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	runningColor := color.New(color.FgGreen).SprintFunc()
	stoppedColor := color.New(color.FgRed).SprintFunc()
	unknownColor := color.New(color.FgYellow).SprintFunc()
	processColor := color.New(color.FgCyan).SprintFunc()

	for serverName, srvConfig := range cfg.Servers {
		identifier := fmt.Sprintf("mcp-compose-%s", serverName)
		var statusStr string

		// USE THE SAME DETECTION LOGIC AS STARTUP
		isContainer := isContainerServer(srvConfig)

		if isContainer {
			if cRuntime != nil && cRuntime.GetRuntimeName() != "none" {
				rawStatus, statusErr := cRuntime.GetContainerStatus(identifier)
				if statusErr != nil {
					statusStr = stoppedColor("Stopped")
				} else {
					switch strings.ToLower(rawStatus) {
					case "running":
						statusStr = runningColor("Running")
					case "exited", "dead", "stopped":
						caser := cases.Title(language.English)
						statusStr = stoppedColor(caser.String(strings.ToLower(rawStatus)))
					default:
						statusStr = unknownColor(rawStatus)
					}
				}
			} else {
				statusStr = stoppedColor("No Runtime")
			}
		} else {
			// This is actually a process-based server
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

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			serverName, statusStr, transport, identifier, ports, capabilities)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}
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
				_, _ = fmt.Fprintf(os.Stdout, "Info: Server '%s' is process-based. View its logs directly.\n", name)
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

func convertSecurityConfig(serverName string, serverCfg config.ServerConfig) container.ContainerOptions {
	opts := container.ContainerOptions{
		Name:        fmt.Sprintf("mcp-compose-%s", serverName),
		Image:       serverCfg.Image,
		Build:       serverCfg.Build,
		Command:     serverCfg.Command,
		Args:        serverCfg.Args,
		Env:         config.MergeEnv(serverCfg.Env, map[string]string{"MCP_SERVER_NAME": serverName}),
		Pull:        serverCfg.Pull,
		Volumes:     serverCfg.Volumes,
		Ports:       serverCfg.Ports,
		Networks:    determineServerNetworks(serverCfg),
		WorkDir:     serverCfg.WorkDir,
		NetworkMode: serverCfg.NetworkMode,

		// Security configuration
		Privileged:  serverCfg.Privileged,
		User:        serverCfg.User,
		Groups:      serverCfg.Groups,
		ReadOnly:    serverCfg.ReadOnly,
		Tmpfs:       serverCfg.Tmpfs,
		CapAdd:      serverCfg.CapAdd,
		CapDrop:     serverCfg.CapDrop,
		SecurityOpt: serverCfg.SecurityOpt,

		// Resource limits
		PidsLimit: serverCfg.Deploy.Resources.Limits.PIDs,

		// Lifecycle
		RestartPolicy: serverCfg.RestartPolicy,
		StopSignal:    serverCfg.StopSignal,
		StopTimeout:   serverCfg.StopTimeout,

		// Runtime options (removed Runtime field)
		Platform:   serverCfg.Platform,
		Hostname:   serverCfg.Hostname,
		DomainName: serverCfg.DomainName,
		DNS:        serverCfg.DNS,
		DNSSearch:  serverCfg.DNSSearch,
		ExtraHosts: serverCfg.ExtraHosts,

		// Logging
		LogDriver:  serverCfg.LogDriver,
		LogOptions: serverCfg.LogOptions,

		// Labels and metadata
		Labels:      serverCfg.Labels,
		Annotations: serverCfg.Annotations,

		// Security config for validation
		Security: container.SecurityConfig{
			AllowDockerSocket:  serverCfg.Security.AllowDockerSocket,
			AllowHostMounts:    serverCfg.Security.AllowHostMounts,
			AllowPrivilegedOps: serverCfg.Security.AllowPrivilegedOps,
			TrustedImage:       serverCfg.Security.TrustedImage,
		},
	}

	// Resource limits
	if serverCfg.Deploy.Resources.Limits.CPUs != "" {
		opts.CPUs = serverCfg.Deploy.Resources.Limits.CPUs
	}
	if serverCfg.Deploy.Resources.Limits.Memory != "" {
		opts.Memory = serverCfg.Deploy.Resources.Limits.Memory
	}
	if serverCfg.Deploy.Resources.Limits.MemorySwap != "" {
		opts.MemorySwap = serverCfg.Deploy.Resources.Limits.MemorySwap
	}

	// Restart policy
	if serverCfg.Deploy.RestartPolicy != "" {
		opts.RestartPolicy = serverCfg.Deploy.RestartPolicy
	}

	// Convert health check if present
	if serverCfg.HealthCheck != nil {
		opts.HealthCheck = &container.HealthCheck{
			Test:        serverCfg.HealthCheck.Test,
			Interval:    serverCfg.HealthCheck.Interval,
			Timeout:     serverCfg.HealthCheck.Timeout,
			Retries:     serverCfg.HealthCheck.Retries,
			StartPeriod: serverCfg.HealthCheck.StartPeriod,
		}
	}

	// Security options based on configuration
	if serverCfg.Security.NoNewPrivileges {
		opts.SecurityOpt = append(opts.SecurityOpt, "no-new-privileges:true")
	}

	if serverCfg.Security.AppArmor != "" {
		opts.SecurityOpt = append(opts.SecurityOpt, fmt.Sprintf("apparmor:%s", serverCfg.Security.AppArmor))
	}

	if serverCfg.Security.Seccomp != "" {
		opts.SecurityOpt = append(opts.SecurityOpt, fmt.Sprintf("seccomp:%s", serverCfg.Security.Seccomp))
	}

	return opts
}

// UPDATE the startServerContainer function to use the new converter:
func startServerContainer(serverName string, serverCfg config.ServerConfig, cRuntime container.Runtime) error {
	opts := convertSecurityConfig(serverName, serverCfg)

	// Transport-specific configuration
	isSocatHostedStdio := serverCfg.StdioHosterPort > 0
	isHttp := serverCfg.Protocol == "http" || serverCfg.HttpPort > 0

	if isSocatHostedStdio {
		fmt.Printf("Starting container '%s' for server '%s' (Socat STDIO Hoster mode on internal port %d).\n",
			opts.Name, serverName, serverCfg.StdioHosterPort)
		opts.Env["MCP_SOCAT_INTERNAL_PORT"] = strconv.Itoa(serverCfg.StdioHosterPort)
	} else if isHttp {
		fmt.Printf("Starting container '%s' for server '%s' (HTTP mode on internal port %d).\n",
			opts.Name, serverName, serverCfg.HttpPort)
		if serverCfg.HttpPort > 0 {
			opts.Env["MCP_HTTP_PORT"] = strconv.Itoa(serverCfg.HttpPort)
		}
		opts.Env["MCP_TRANSPORT"] = "http"
	} else {
		fmt.Printf("Starting container '%s' for server '%s' (Direct STDIO mode).\n",
			opts.Name, serverName)
	}

	// Log security configuration
	if len(opts.CapAdd) > 0 {
		fmt.Printf("Container '%s' adding capabilities: %s\n", opts.Name, strings.Join(opts.CapAdd, ", "))
	}
	if len(opts.CapDrop) > 0 {
		fmt.Printf("Container '%s' dropping capabilities: %s\n", opts.Name, strings.Join(opts.CapDrop, ", "))
	}
	if opts.Privileged {
		fmt.Printf("Container '%s' running in privileged mode\n", opts.Name)
	}

	_, err := cRuntime.StartContainer(&opts)
	if err != nil {
		return fmt.Errorf("failed to start container for server '%s': %w", serverName, err)
	}

	return nil
}
