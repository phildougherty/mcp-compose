package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/logging"
	"mcpcompose/internal/runtime"

	"github.com/fsnotify/fsnotify"
)

// ServerInstance represents a running server instance
type ServerInstance struct {
	Name             string
	Config           config.ServerConfig
	ContainerID      string
	Process          *runtime.Process
	IsContainer      bool
	Status           string
	StartTime        time.Time
	Capabilities     map[string]bool
	ConnectionInfo   map[string]string
	HealthStatus     string
	ResourcesWatcher *ResourcesWatcher
}

// Manager handles server lifecycle operations
type Manager struct {
	config           *config.ComposeConfig
	containerRuntime container.Runtime
	projectName      string
	projectDir       string
	servers          map[string]*ServerInstance
	networks         map[string]bool
	logger           *logging.Logger
	mu               sync.Mutex
}

// NewManager creates a new server manager
func NewManager(cfg *config.ComposeConfig, rt container.Runtime) (*Manager, error) {
	// Use absolute path for project directory
	wd, _ := os.Getwd()

	// Create logger
	logLevel := "info"
	if cfg.Logging.Level != "" {
		logLevel = cfg.Logging.Level
	}
	logger := logging.NewLogger(logLevel)

	manager := &Manager{
		config:           cfg,
		containerRuntime: rt,
		projectName:      filepath.Base(wd),
		projectDir:       wd,
		servers:          make(map[string]*ServerInstance),
		networks:         make(map[string]bool),
		logger:           logger,
	}

	// Initialize server instances
	for name, serverCfg := range cfg.Servers {
		manager.servers[name] = &ServerInstance{
			Name:           name,
			Config:         serverCfg,
			IsContainer:    serverCfg.Image != "",
			Status:         "stopped",
			Capabilities:   make(map[string]bool),
			ConnectionInfo: make(map[string]string),
			HealthStatus:   "unknown",
		}
	}

	return manager, nil
}

// StartServer starts a server with the given name
func (m *Manager) StartServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("server '%s' not found", name)
	}

	srvCfg := instance.Config

	// Create server ID for containers
	serverID := fmt.Sprintf("%s-%s", m.projectName, name)

	// Check if the server is already running
	status, err := m.GetServerStatus(name)
	if err == nil && status == "running" {
		m.logger.Info("Server '%s' is already running", name)
		return nil
	}

	// Run pre-start hooks if defined
	if srvCfg.Lifecycle.PreStart != "" {
		m.logger.Info("Running pre-start hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PreStart); err != nil {
			return fmt.Errorf("pre-start hook failed: %w", err)
		}
	}

	// Create networks if needed
	if len(srvCfg.Networks) > 0 {
		for _, network := range srvCfg.Networks {
			if err := m.ensureNetworkExists(network); err != nil {
				return err
			}
		}
	}

	// Start the server
	var startErr error
	if srvCfg.Image != "" || srvCfg.Runtime != "" {
		startErr = m.startContainerServer(name, serverID, &srvCfg)
	} else if srvCfg.Command != "" {
		startErr = m.startProcessServer(name, serverID, &srvCfg)
	} else {
		startErr = fmt.Errorf("server '%s' has no command or image specified", name)
	}

	if startErr != nil {
		return startErr
	}

	// Update server instance
	instance.Status = "running"
	instance.StartTime = time.Now()

	// Set up resource watchers if configured
	if config.IsCapabilityEnabled(srvCfg, "resources") && len(srvCfg.Resources.Paths) > 0 {
		watcher, err := NewResourcesWatcher(&srvCfg)
		if err != nil {
			m.logger.Warning("Failed to initialize resource watcher for server '%s': %v", name, err)
		} else {
			instance.ResourcesWatcher = watcher
			go watcher.Start()
		}
	}

	// Run post-start hooks if defined
	if srvCfg.Lifecycle.PostStart != "" {
		m.logger.Info("Running post-start hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PostStart); err != nil {
			m.logger.Warning("Post-start hook failed: %v", err)
		}
	}

	// Start health check if configured
	if srvCfg.Lifecycle.HealthCheck.Endpoint != "" {
		go m.startHealthCheck(name)
	}

	// Initialize MCP capabilities
	if err := m.initializeServerCapabilities(name); err != nil {
		m.logger.Warning("Failed to initialize capabilities for server '%s': %v", name, err)
	}

	return nil
}

// startContainerServer starts a server in a container
func (m *Manager) startContainerServer(name, serverID string, srvCfg *config.ServerConfig) error {
	runtime := srvCfg.Runtime
	if runtime == "" {
		runtime = "docker" // Default to docker if not specified
	}

	if m.containerRuntime.GetRuntimeName() == "none" {
		return fmt.Errorf("server '%s' requires container runtime '%s' which is not available", name, runtime)
	}

	imageName := srvCfg.Image
	if imageName == "" {
		return fmt.Errorf("server '%s' requires an image to be specified", name)
	}

	m.logger.Info("Starting container-based server '%s' using image '%s'", name, imageName)

	// Set up resource volume mappings if configured
	volumes := srvCfg.Volumes
	for _, resourcePath := range srvCfg.Resources.Paths {
		// Only map local paths for containers
		absPath, err := filepath.Abs(resourcePath.Source)
		if err == nil {
			volumeMapping := fmt.Sprintf("%s:%s", absPath, resourcePath.Target)
			if resourcePath.ReadOnly {
				volumeMapping += ":ro"
			}
			volumes = append(volumes, volumeMapping)
		}
	}

	// Prepare container options
	opts := &container.ContainerOptions{
		Name:        serverID,
		Image:       imageName,
		Command:     srvCfg.Command,
		Args:        srvCfg.Args,
		Env:         srvCfg.Env,
		Pull:        srvCfg.Pull,
		Volumes:     volumes,
		Ports:       srvCfg.Ports,
		NetworkMode: srvCfg.NetworkMode,
		Networks:    srvCfg.Networks,
	}

	// Set working directory if specified
	if srvCfg.WorkDir != "" {
		opts.WorkDir = srvCfg.WorkDir
	}

	// Set up connection-related ports
	for _, conn := range m.config.Connections {
		if conn.Expose && conn.Port > 0 {
			portMapping := fmt.Sprintf("%d:%d", conn.Port, conn.Port)
			if !contains(opts.Ports, portMapping) {
				opts.Ports = append(opts.Ports, portMapping)
			}
		}
	}

	// Start the container
	containerID, err := m.containerRuntime.StartContainer(opts)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Store container ID
	m.servers[name].ContainerID = containerID
	return nil
}

// startProcessServer starts a server as a local process
func (m *Manager) startProcessServer(name, serverID string, srvCfg *config.ServerConfig) error {
	m.logger.Info("Starting process-based server '%s' using command '%s'", name, srvCfg.Command)

	// Create environment with added connection info
	env := make(map[string]string)
	if srvCfg.Env != nil {
		for k, v := range srvCfg.Env {
			env[k] = v
		}
	}

	// Add standard environment variables
	env["MCP_SERVER_NAME"] = name
	env["MCP_PROJECT_NAME"] = m.projectName

	// Add connection-related environment
	for connName, conn := range m.config.Connections {
		env[fmt.Sprintf("MCP_CONN_%s_TRANSPORT", strings.ToUpper(connName))] = conn.Transport
		if conn.Port > 0 {
			env[fmt.Sprintf("MCP_CONN_%s_PORT", strings.ToUpper(connName))] = fmt.Sprintf("%d", conn.Port)
		}
		if conn.Host != "" {
			env[fmt.Sprintf("MCP_CONN_%s_HOST", strings.ToUpper(connName))] = conn.Host
		}
		if conn.Path != "" {
			env[fmt.Sprintf("MCP_CONN_%s_PATH", strings.ToUpper(connName))] = conn.Path
		}
	}

	// Create a process wrapper
	proc, err := runtime.NewProcess(srvCfg.Command, srvCfg.Args, runtime.ProcessOptions{
		Env:     env,
		WorkDir: srvCfg.WorkDir,
		Name:    serverID,
	})

	if err != nil {
		return fmt.Errorf("failed to create process: %w", err)
	}

	// Start the process
	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Store process reference
	m.servers[name].Process = proc
	return nil
}

// StopServer stops a server with the given name
func (m *Manager) StopServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("server '%s' not found", name)
	}

	srvCfg := instance.Config

	// Create server ID for containers
	serverID := fmt.Sprintf("%s-%s", m.projectName, name)

	// Check if the server is running
	status, _ := m.GetServerStatus(name)
	if status != "running" {
		m.logger.Info("Server '%s' is not running", name)
		return nil
	}

	// Run pre-stop hooks if defined
	if srvCfg.Lifecycle.PreStop != "" {
		m.logger.Info("Running pre-stop hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PreStop); err != nil {
			m.logger.Warning("Pre-stop hook failed: %v", err)
		}
	}

	// Stop resource watcher if running
	if instance.ResourcesWatcher != nil {
		instance.ResourcesWatcher.Stop()
		instance.ResourcesWatcher = nil
	}

	// Stop the server
	var err error
	if instance.IsContainer {
		// Container-based server
		m.logger.Info("Stopping container-based server '%s'", name)
		err = m.containerRuntime.StopContainer(serverID)
		if err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
		// Clear container ID
		instance.ContainerID = ""
	} else {
		// Process-based server
		m.logger.Info("Stopping process-based server '%s'", name)
		if instance.Process != nil {
			err = instance.Process.Stop()
			if err != nil {
				return fmt.Errorf("failed to stop process: %w", err)
			}
			// Clear process reference
			instance.Process = nil
		}
	}

	// Update server instance
	instance.Status = "stopped"
	instance.HealthStatus = "unknown"

	// Run post-stop hooks if defined
	if srvCfg.Lifecycle.PostStop != "" {
		m.logger.Info("Running post-stop hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PostStop); err != nil {
			m.logger.Warning("Post-stop hook failed: %v", err)
		}
	}

	return nil
}

// GetServerStatus returns the status of a server
func (m *Manager) GetServerStatus(name string) (string, error) {
	instance, ok := m.servers[name]
	if !ok {
		return "", fmt.Errorf("server '%s' not found", name)
	}

	// If we already know it's running from our cache, return quickly
	if instance.Status == "running" {
		// Double-check with the runtime
		if instance.IsContainer && instance.ContainerID != "" {
			status, _ := m.containerRuntime.GetContainerStatus(instance.ContainerID)
			if status != "running" {
				// Update our cache
				instance.Status = status
			}
		} else if instance.Process != nil {
			isRunning, _ := instance.Process.IsRunning()
			if !isRunning {
				instance.Status = "stopped"
			}
		}
	}

	// Create server ID for containers
	serverID := fmt.Sprintf("%s-%s", m.projectName, name)

	// Check with the runtime
	if instance.IsContainer {
		// Container-based server
		status, _ := m.containerRuntime.GetContainerStatus(serverID)
		instance.Status = status
	} else {
		// Process-based server
		proc, err := runtime.FindProcess(serverID)
		if err != nil {
			instance.Status = "stopped"
		} else {
			isRunning, err := proc.IsRunning()
			if err != nil || !isRunning {
				instance.Status = "stopped"
			} else {
				instance.Status = "running"
			}
		}
	}

	return instance.Status, nil
}

// ShowLogs displays logs for a server
func (m *Manager) ShowLogs(name string, follow bool) error {
	instance, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("server '%s' not found", name)
	}

	// Create server ID for containers
	serverID := fmt.Sprintf("%s-%s", m.projectName, name)

	// Either show container logs or process logs
	if instance.IsContainer {
		// Container-based server
		return m.containerRuntime.ShowContainerLogs(serverID, follow)
	} else {
		// Process-based server
		proc, err := runtime.FindProcess(serverID)
		if err != nil {
			return fmt.Errorf("server process not found: %w", err)
		}
		return proc.ShowLogs(follow)
	}
}

// ResourcesWatcher watches for changes in resource paths
type ResourcesWatcher struct {
	config       *config.ServerConfig
	active       bool
	stopCh       chan struct{}
	watchersMu   sync.Mutex
	fsWatchers   []*fsnotify.Watcher
	changedFiles map[string]time.Time
	ticker       *time.Ticker
}

// NewResourcesWatcher creates a new resources watcher
func NewResourcesWatcher(cfg *config.ServerConfig) (*ResourcesWatcher, error) {
	return &ResourcesWatcher{
		config:       cfg,
		active:       false,
		stopCh:       make(chan struct{}),
		fsWatchers:   make([]*fsnotify.Watcher, 0),
		changedFiles: make(map[string]time.Time),
	}, nil
}

// Start starts the resource watcher
func (w *ResourcesWatcher) Start() error {
	w.watchersMu.Lock()
	defer w.watchersMu.Unlock()

	if w.active {
		return nil
	}

	w.active = true
	w.changedFiles = make(map[string]time.Time)

	// Set up watchers for each resource path
	for _, resourcePath := range w.config.Resources.Paths {
		if resourcePath.Watch {
			if err := w.watchPath(resourcePath.Source); err != nil {
				w.Stop() // Clean up any watchers already created
				return fmt.Errorf("failed to watch path %s: %w", resourcePath.Source, err)
			}
		}
	}

	// Determine sync interval
	syncInterval := 5 * time.Second // Default
	if w.config.Resources.SyncInterval != "" {
		if interval, err := time.ParseDuration(w.config.Resources.SyncInterval); err == nil {
			syncInterval = interval
		}
	}

	// Start ticker for periodic scanning
	w.ticker = time.NewTicker(syncInterval)
	go w.runWatcher()

	return nil
}

// watchPath sets up a watcher for a directory
func (w *ResourcesWatcher) watchPath(dirPath string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}

	// Add the directory itself
	if err := watcher.Add(dirPath); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add directory to watcher: %w", err)
	}

	// Add all subdirectories recursively
	err = filepath.Walk(dirPath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := watcher.Add(path); err != nil {
				return fmt.Errorf("failed to add subdirectory to watcher: %w", err)
			}
		}
		return nil
	})

	if err != nil {
		watcher.Close()
		return fmt.Errorf("failed to walk directory tree: %w", err)
	}

	w.fsWatchers = append(w.fsWatchers, watcher)
	return nil
}

// runWatcher runs the main watcher loop
func (w *ResourcesWatcher) runWatcher() {
	// Process events from all watchers
	for _, watcher := range w.fsWatchers {
		go func(fw *fsnotify.Watcher) {
			for {
				select {
				case event, ok := <-fw.Events:
					if !ok {
						return
					}
					// Filter out events we don't care about
					if !w.shouldProcessEvent(event) {
						continue
					}
					// Record the change
					w.recordChange(event.Name)
				case err, ok := <-fw.Errors:
					if !ok {
						return
					}
					fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
				}
			}
		}(watcher)
	}

	// Process ticker events for batched notifications
	for {
		select {
		case <-w.stopCh:
			return
		case <-w.ticker.C:
			w.processChanges()
		}
	}
}

// shouldProcessEvent determines if an event should be processed
func (w *ResourcesWatcher) shouldProcessEvent(event fsnotify.Event) bool {
	// Skip temporary files and hidden files
	baseName := filepath.Base(event.Name)
	if strings.HasPrefix(baseName, ".") || strings.HasSuffix(baseName, "~") {
		return false
	}

	// We care about create, write, rename, remove operations
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove) == 0 {
		return false
	}

	return true
}

// recordChange records a file change
func (w *ResourcesWatcher) recordChange(path string) {
	w.watchersMu.Lock()
	defer w.watchersMu.Unlock()

	// Store current time directly without unused variable
	w.changedFiles[path] = time.Now()
}

// processChanges processes accumulated changes and notifies subscribers
func (w *ResourcesWatcher) processChanges() {
	w.watchersMu.Lock()
	defer w.watchersMu.Unlock()

	if len(w.changedFiles) == 0 {
		return
	}

	// Build a list of changed files
	changes := make(map[string]string, len(w.changedFiles))
	for path := range w.changedFiles {
		// Check if file still exists
		info, err := os.Stat(path)
		if err == nil {
			// File exists
			if info.IsDir() {
				changes[path] = "directory"
			} else {
				changes[path] = "file"
			}
		} else if os.IsNotExist(err) {
			// File was deleted
			changes[path] = "deleted"
		} else {
			// Error checking file, skip
			continue
		}
	}

	// Clear the changed files map
	w.changedFiles = make(map[string]time.Time)

	// Send notifications
	if len(changes) > 0 {
		w.notifyChanges(changes)
	}
}

// notifyChanges notifies subscribers about changes
func (w *ResourcesWatcher) notifyChanges(changes map[string]string) {
	// In a real implementation, this would send notifications to the MCP server
	// using protocol.MCPNotification with resources/list-changed method

	// For now, log the changes
	changesJSON, _ := json.MarshalIndent(changes, "", "  ")
	fmt.Printf("Resource changes detected: %s\n", string(changesJSON))

	// For each resource path, map the changes to its target path
	for _, resourcePath := range w.config.Resources.Paths {
		sourcePath := filepath.Clean(resourcePath.Source)
		targetPath := filepath.Clean(resourcePath.Target)

		// Map changes from source to target paths
		targetChanges := make(map[string]string)
		for path, changeType := range changes {
			// Check if this path is within the source directory
			if strings.HasPrefix(path, sourcePath) {
				// Replace source path with target path
				relativePath, err := filepath.Rel(sourcePath, path)
				if err != nil {
					continue
				}
				targetPath := filepath.Join(targetPath, relativePath)
				targetChanges[targetPath] = changeType
			}
		}

		// If there are mapped changes for this resource, notify about them
		if len(targetChanges) > 0 {
			// Here we'd send a protocol notification
			notification := map[string]interface{}{
				"jsonrpc": "2.0",
				"method":  "resources/list-changed",
				"params": map[string]interface{}{
					"changes": targetChanges,
				},
			}

			notificationJSON, _ := json.MarshalIndent(notification, "", "  ")
			fmt.Printf("Would send notification: %s\n", string(notificationJSON))
		}
	}
}

// Stop stops the resource watcher
func (w *ResourcesWatcher) Stop() error {
	w.watchersMu.Lock()
	defer w.watchersMu.Unlock()

	if !w.active {
		return nil
	}

	// Stop the ticker
	if w.ticker != nil {
		w.ticker.Stop()
	}

	// Signal the watcher goroutine to stop
	close(w.stopCh)

	// Close all watchers
	for _, watcher := range w.fsWatchers {
		watcher.Close()
	}
	w.fsWatchers = nil

	w.active = false
	return nil
}

// Full health check implementation
func (m *Manager) startHealthCheck(name string) {
	instance, ok := m.servers[name]
	if !ok {
		return
	}

	healthCfg := instance.Config.Lifecycle.HealthCheck
	if healthCfg.Endpoint == "" {
		return
	}

	// Parse interval and timeout durations
	interval := 5 * time.Second // Default interval
	if healthCfg.Interval != "" {
		if parsed, err := time.ParseDuration(healthCfg.Interval); err == nil {
			interval = parsed
		}
	}

	timeout := 2 * time.Second // Default timeout
	if healthCfg.Timeout != "" {
		if parsed, err := time.ParseDuration(healthCfg.Timeout); err == nil {
			timeout = parsed
		}
	}

	retries := healthCfg.Retries
	if retries <= 0 {
		retries = 3 // Default retries
	}

	m.logger.Info("Starting health checks for server '%s' with interval %s", name, interval)

	// Start health check loop
	go func() {
		failCount := 0

		for {
			// Check if the server is still running
			status, _ := m.GetServerStatus(name)
			if status != "running" {
				m.logger.Info("Server '%s' is no longer running, stopping health checks", name)
				break
			}

			// Perform health check
			healthy, err := m.checkServerHealth(name, healthCfg.Endpoint, timeout)

			m.mu.Lock()
			if healthy {
				instance.HealthStatus = "healthy"
				failCount = 0
				m.mu.Unlock()
			} else {
				// Increment fail count
				failCount++

				if failCount >= retries {
					instance.HealthStatus = "unhealthy"
					m.logger.Warning("Server '%s' is unhealthy after %d failed checks: %v",
						name, failCount, err)

					// Check if we should restart the server based on the Action field
					if healthCfg.Action == "restart" {
						m.logger.Warning("Attempting to restart unhealthy server '%s'", name)
						m.mu.Unlock() // Unlock before restarting

						// Stop the server
						if err := m.StopServer(name); err != nil {
							m.logger.Error("Failed to stop unhealthy server '%s': %v", name, err)
						} else {
							// Wait a moment before restarting
							time.Sleep(2 * time.Second)

							// Start the server again
							if err := m.StartServer(name); err != nil {
								m.logger.Error("Failed to restart unhealthy server '%s': %v", name, err)
							} else {
								m.logger.Info("Successfully restarted server '%s'", name)
							}
						}

						// Exit health check loop - new one will start when server restarts
						return
					} else {
						m.mu.Unlock()
					}
				} else {
					instance.HealthStatus = fmt.Sprintf("failing:%d/%d", failCount, retries)
					m.logger.Warning("Health check failed for server '%s' (%d/%d): %v",
						name, failCount, retries, err)
					m.mu.Unlock()
				}
			}

			// Wait for next check
			time.Sleep(interval)
		}
	}()
}

func (m *Manager) GetServerInstance(name string) (*ServerInstance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	instance, exists := m.servers[name]
	return instance, exists
}

// checkServerHealth performs a health check on a server
func (m *Manager) checkServerHealth(name string, endpoint string, timeout time.Duration) (bool, error) {
	instance := m.servers[name]
	if instance == nil {
		return false, fmt.Errorf("server not found")
	}

	// Determine the URL based on server type and endpoint
	var url string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		// Absolute URL provided
		url = endpoint
	} else {
		// Relative path, construct URL based on server
		if instance.IsContainer {
			// For container, use container name as hostname
			serverID := fmt.Sprintf("%s-%s", m.projectName, name)
			url = fmt.Sprintf("http://%s%s", serverID, endpoint)
		} else {
			// For local process, use localhost
			// Extract port from endpoint if possible
			port := "8080" // Default port
			if strings.HasPrefix(endpoint, ":") {
				parts := strings.SplitN(endpoint, "/", 2)
				port = strings.TrimPrefix(parts[0], ":")
			}
			url = fmt.Sprintf("http://localhost:%s%s", port, strings.TrimPrefix(endpoint, ":"+port))
		}
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: timeout,
	}

	// Create request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return false, fmt.Errorf("health check returned status %d: %s", resp.StatusCode, body)
	}

	return true, nil
}

// runLifecycleHook runs a lifecycle hook script
func (m *Manager) runLifecycleHook(hookScript string) error {
	cmd := exec.Command("sh", "-c", hookScript)
	cmd.Env = os.Environ()
	cmd.Dir = m.projectDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("hook script failed: %w, output: %s", err, string(output))
	}
	return nil
}

// ensureNetworkExists creates a Docker/Podman network if it doesn't exist
func (m *Manager) ensureNetworkExists(networkName string) error {
	// Skip if we already checked this network
	if m.networks[networkName] {
		return nil
	}

	// Skip for null runtime
	if m.containerRuntime.GetRuntimeName() == "none" {
		return nil
	}

	m.logger.Info("Ensuring network '%s' exists", networkName)

	// Check if network exists
	exists, err := m.containerRuntime.NetworkExists(networkName)
	if err != nil {
		return fmt.Errorf("failed to check if network exists: %w", err)
	}

	if !exists {
		// Create the network
		if err := m.containerRuntime.CreateNetwork(networkName); err != nil {
			return fmt.Errorf("failed to create network: %w", err)
		}
	}

	// Mark as checked
	m.networks[networkName] = true
	return nil
}

// initializeServerCapabilities initializes the MCP capabilities for a server
func (m *Manager) initializeServerCapabilities(name string) error {
	instance := m.servers[name]
	if instance == nil {
		return fmt.Errorf("server not found")
	}

	// In a real implementation, we would:
	// 1. Connect to the server using the appropriate transport
	// 2. Send an initialize request to get capabilities
	// 3. Store the capabilities in the server instance

	// For this example, just use the configured capabilities
	for _, cap := range instance.Config.Capabilities {
		instance.Capabilities[cap] = true
	}

	return nil
}

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
