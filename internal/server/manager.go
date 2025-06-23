package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs" // Keep for filepath.Walk, os.Stat etc.
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

	"github.com/fsnotify/fsnotify" // Keep if ResourcesWatcher uses it
)

// ServerInstance represents a running server instance
type ServerInstance struct {
	Name             string
	Config           config.ServerConfig
	ContainerID      string // Actual ID from the container runtime
	Process          *runtime.Process
	IsContainer      bool
	Status           string
	StartTime        time.Time
	Capabilities     map[string]bool
	ConnectionInfo   map[string]string
	HealthStatus     string
	ResourcesWatcher *ResourcesWatcher
	mu               sync.RWMutex // Protects access to instance fields
	ctx              context.Context
	cancel           context.CancelFunc
}

// Manager handles server lifecycle operations
type Manager struct {
	config           *config.ComposeConfig
	containerRuntime container.Runtime
	projectDir       string // For running lifecycle hooks and resolving relative paths
	servers          map[string]*ServerInstance
	networks         map[string]bool // Tracks networks known/created by this manager instance
	logger           *logging.Logger
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	wg               sync.WaitGroup
	shutdownCh       chan struct{}
	healthCheckers   map[string]context.CancelFunc
	healthCheckMu    sync.Mutex
}

// Update the NewManager function in internal/server/manager.go
func NewManager(cfg *config.ComposeConfig, rt container.Runtime) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	logLevel := "info"
	if cfg.Logging.Level != "" {
		logLevel = cfg.Logging.Level
	}

	logger := logging.NewLogger(logLevel)

	// CREATE CONTEXT AND CANCEL FUNCTION
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		config:           cfg,
		containerRuntime: rt,
		projectDir:       wd,
		servers:          make(map[string]*ServerInstance),
		networks:         make(map[string]bool),
		logger:           logger,
		// ADD THESE FIELDS:
		ctx:            ctx,
		cancel:         cancel,
		shutdownCh:     make(chan struct{}),
		healthCheckers: make(map[string]context.CancelFunc),
	}

	// Initialize server instances
	for name, serverCfg := range cfg.Servers {
		instanceCtx, instanceCancel := context.WithCancel(ctx)
		manager.servers[name] = &ServerInstance{
			Name:           name,
			Config:         serverCfg,
			IsContainer:    serverCfg.Image != "" || serverCfg.Runtime != "",
			Status:         "stopped",
			Capabilities:   make(map[string]bool),
			ConnectionInfo: make(map[string]string),
			HealthStatus:   "unknown",
			ctx:            instanceCtx,
			cancel:         instanceCancel,
		}
	}

	return manager, nil
}

func (m *Manager) StartServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("MANAGER: StartServer called for '%s'", name)

	instance, ok := m.servers[name]
	if !ok {
		m.logger.Error("MANAGER: Server '%s' not found in configuration during StartServer", name)
		return fmt.Errorf("server '%s' not found in configuration", name)
	}

	srvCfg := instance.Config
	fixedIdentifier := fmt.Sprintf("mcp-compose-%s", name)
	m.logger.Info("MANAGER: Determined fixedIdentifier for '%s' as '%s'", name, fixedIdentifier)

	// Check current status
	m.logger.Info("MANAGER: Checking current status for '%s' (identifier: %s)...", name, fixedIdentifier)
	currentStatus, statusErr := m.getServerStatusUnsafe(name, fixedIdentifier)
	if statusErr != nil {
		m.logger.Warning("MANAGER: Error getting status for '%s': %v. Proceeding with start attempt.", name, statusErr)
	}
	m.logger.Info("MANAGER: Current status for '%s' is '%s'", name, currentStatus)

	if currentStatus == "running" {
		m.logger.Info("MANAGER: Server '%s' (identifier: %s) reported as already running by status check.", name, fixedIdentifier)
		return nil
	}

	// Pre-start hooks
	if srvCfg.Lifecycle.PreStart != "" {
		m.logger.Info("MANAGER: Running pre-start hook for server '%s'...", name)
		if hookErr := m.runLifecycleHook(srvCfg.Lifecycle.PreStart); hookErr != nil {
			m.logger.Error("MANAGER: Pre-start hook for server '%s' failed: %v", name, hookErr)
			return fmt.Errorf("pre-start hook for server '%s' failed: %w", name, hookErr)
		}
		m.logger.Info("MANAGER: Pre-start hook for server '%s' completed.", name)
	}

	// Ensure networks
	if len(srvCfg.Networks) > 0 {
		m.logger.Info("MANAGER: Ensuring networks for server '%s': %v", name, srvCfg.Networks)
		for _, networkName := range srvCfg.Networks {
			if networkErr := m.ensureNetworkExists(networkName, true); networkErr != nil {
				m.logger.Error("MANAGER: Failed to ensure network '%s' for server '%s': %v", networkName, name, networkErr)
				return fmt.Errorf("failed to ensure network '%s' for server '%s': %w", networkName, name, networkErr)
			}
		}
		m.logger.Info("MANAGER: Networks ensured for server '%s'.", name)
	}

	var startErr error
	if instance.IsContainer {
		m.logger.Info("MANAGER: Server '%s' is container. Calling startContainerServer with identifier '%s'.", name, fixedIdentifier)
		startErr = m.startContainerServer(name, fixedIdentifier, &srvCfg)
	} else if srvCfg.Command != "" {
		m.logger.Info("MANAGER: Server '%s' is process. Calling startProcessServer with identifier '%s'.", name, fixedIdentifier)
		startErr = m.startProcessServer(name, fixedIdentifier, &srvCfg)
	} else {
		m.logger.Error("MANAGER: Server '%s' has no command or image specified.", name)
		startErr = fmt.Errorf("server '%s' has no command or image specified in config", name)
	}

	if startErr != nil {
		m.logger.Error("MANAGER: Error starting server '%s' (identifier: %s): %v", name, fixedIdentifier, startErr)
		return fmt.Errorf("failed to start server '%s' (identifier: %s): %w", name, fixedIdentifier, startErr)
	}

	instance.Status = "running"
	instance.StartTime = time.Now()
	m.logger.Info("MANAGER: Server '%s' (identifier: %s) marked as started successfully. ContainerID (if any): %s", name, fixedIdentifier, instance.ContainerID)

	// REMOVE ALL THE BLOCKING POST-START ACTIVITIES
	// Just start them in background goroutines without waiting

	// Post-start hooks (non-blocking)
	if srvCfg.Lifecycle.PostStart != "" {
		go func() {
			m.logger.Info("MANAGER: Running post-start hook for server '%s' (background)...", name)
			if hookErr := m.runLifecycleHook(srvCfg.Lifecycle.PostStart); hookErr != nil {
				m.logger.Warning("MANAGER: Post-start hook for server '%s' failed: %v", name, hookErr)
			} else {
				m.logger.Info("MANAGER: Post-start hook for server '%s' completed.", name)
			}
		}()
	}

	// Resource watcher (non-blocking)
	if config.IsCapabilityEnabled(srvCfg, "resources") && len(srvCfg.Resources.Paths) > 0 {
		go func() {
			m.logger.Info("MANAGER: Initializing resource watcher for server '%s' (background)...", name)
			watcher, watchErr := NewResourcesWatcher(&srvCfg, m.logger)
			if watchErr != nil {
				m.logger.Warning("MANAGER: Failed to initialize resource watcher for server '%s': %v", name, watchErr)
				return
			}
			instance.ResourcesWatcher = watcher
			watcher.Start()
		}()
	}

	// Health check (non-blocking)
	if srvCfg.Lifecycle.HealthCheck.Endpoint != "" {
		go func() {
			m.logger.Info("MANAGER: Starting health check for server '%s' (background)...", name)
			m.startHealthCheck(name, fixedIdentifier)
		}()
	}

	// Capabilities (non-blocking)
	go func() {
		if capErr := m.initializeServerCapabilities(name); capErr != nil {
			m.logger.Warning("MANAGER: Failed to initialize capabilities for server '%s': %v", name, capErr)
		} else {
			m.logger.Info("MANAGER: Capabilities initialized for server '%s'", name)
		}
	}()

	m.logger.Info("MANAGER: StartServer for '%s' completed.", name)
	return nil
}

func (m *Manager) startContainerServer(serverKeyName, containerNameToUse string, srvCfg *config.ServerConfig) error {
	runtimeType := srvCfg.Runtime
	if runtimeType == "" && srvCfg.Image != "" {
		runtimeType = "docker" // Default to docker if image is specified
	}
	if m.containerRuntime.GetRuntimeName() == "none" && srvCfg.Image != "" {
		return fmt.Errorf("server '%s' requires container runtime but none available", serverKeyName)
	}
	if srvCfg.Image == "" {
		return fmt.Errorf("server '%s' (container: %s) has no image specified", serverKeyName, containerNameToUse)
	}
	m.logger.Info("Preparing to start container '%s' for server '%s' with image '%s'", containerNameToUse, serverKeyName, srvCfg.Image)

	// Ensure mcp-net network exists FIRST
	if m.containerRuntime != nil && m.containerRuntime.GetRuntimeName() != "none" {
		networkExists, _ := m.containerRuntime.NetworkExists("mcp-net")
		if !networkExists {
			if err := m.containerRuntime.CreateNetwork("mcp-net"); err != nil {
				m.logger.Warning("Failed to create mcp-net network: %v", err)
			} else {
				m.logger.Info("Created mcp-net network")
			}
		}
	}

	var volumes []string
	if srvCfg.Volumes != nil {
		volumes = append([]string{}, srvCfg.Volumes...) // Copy existing volumes
	}
	for _, resourcePath := range srvCfg.Resources.Paths {
		absPath, err := filepath.Abs(resourcePath.Source)
		if err == nil {
			volumeMapping := fmt.Sprintf("%s:%s", absPath, resourcePath.Target)
			if resourcePath.ReadOnly {
				volumeMapping += ":ro"
			}
			volumes = append(volumes, volumeMapping)
		} else {
			m.logger.Warning("Could not make path absolute for volume mount '%s' for server '%s': %v", resourcePath.Source, serverKeyName, err)
		}
	}

	// Prepare environment variables, including MCP_SERVER_NAME
	envVars := config.MergeEnv(srvCfg.Env, map[string]string{"MCP_SERVER_NAME": serverKeyName})

	// Use existing ports from config (no auto HTTP port exposure)
	ports := make([]string, len(srvCfg.Ports))
	copy(ports, srvCfg.Ports)

	// LOG: Explain why we don't expose HTTP ports for HTTP protocol servers
	if srvCfg.Protocol == "http" {
		m.logger.Info("Server '%s' uses HTTP protocol - accessible via Docker network only (no host port exposure needed)", serverKeyName)
	} else {
		m.logger.Info("Server '%s' uses protocol '%s'", serverKeyName, srvCfg.Protocol)
	}

	// CRITICAL FIX: For HTTP wrapper images, don't override the command
	var command string
	var args []string

	if srvCfg.Protocol == "http" && strings.Contains(srvCfg.Image, "mcp-http-server") {
		// HTTP wrapper images have their own built-in command, don't override it
		m.logger.Info("Using built-in command for HTTP wrapper image '%s'", srvCfg.Image)
		command = "" // Let the image use its default CMD
		args = nil
	} else {
		// Use the configured command for other servers
		command = srvCfg.Command
		args = srvCfg.Args
		m.logger.Info("Using configured command '%s' with args %v", command, args)
	}

	// Ensure networks include mcp-net
	networks := []string{"mcp-net"} // Always include mcp-net
	for _, net := range srvCfg.Networks {
		if net != "mcp-net" { // Don't duplicate
			networks = append(networks, net)
		}
	}

	opts := &container.ContainerOptions{
		Name:        containerNameToUse, // This is the name Docker/Podman will use
		Image:       srvCfg.Image,
		Command:     command, // Don't override for HTTP wrappers
		Args:        args,    // Don't override for HTTP wrappers
		Env:         envVars,
		Pull:        srvCfg.Pull,
		Volumes:     volumes,
		Ports:       ports,    // Only explicitly configured ports, no auto HTTP ports
		NetworkMode: "",       // Don't use NetworkMode, use Networks instead
		Networks:    networks, // Ensure mcp-net is included
		WorkDir:     srvCfg.WorkDir,
	}

	// Add globally defined connection ports if exposed
	for connKey, connCfg := range m.config.Connections {
		if connCfg.Expose && connCfg.Port > 0 {
			portMapping := fmt.Sprintf("%d:%d", connCfg.Port, connCfg.Port) // hostPort:containerPort
			if !contains(opts.Ports, portMapping) {
				opts.Ports = append(opts.Ports, portMapping)
				m.logger.Debug("Adding exposed port %s from connection '%s' for server '%s'", portMapping, connKey, serverKeyName)
			}
		}
	}

	// Log final container options for debugging
	m.logger.Info("Starting container with options: Name=%s, Image=%s, Command=%s, Args=%v, Ports=%v, Networks=%v, Protocol=%s",
		opts.Name, opts.Image, opts.Command, opts.Args, opts.Ports, opts.Networks, srvCfg.Protocol)

	// Start the container
	containerID, err := m.containerRuntime.StartContainer(opts)
	if err != nil {
		return fmt.Errorf("failed to start container '%s' for server '%s': %w", containerNameToUse, serverKeyName, err)
	}

	// Store the actual container ID provided by the runtime
	m.mu.RLock()
	instance := m.servers[serverKeyName]
	m.mu.RUnlock()

	if instance != nil {
		instance.mu.Lock()
		instance.ContainerID = containerID
		instance.mu.Unlock()
	}

	m.logger.Info("Container '%s' (ID: %s) for server '%s' started - accessible via Docker network", containerNameToUse, containerID, serverKeyName)
	return nil
}

// startProcessServer uses processIdentifier for log/pid files
func (m *Manager) startProcessServer(serverKeyName, processIdentifier string, srvCfg *config.ServerConfig) error {
	m.logger.Info("Preparing to start process '%s' for server '%s' with command '%s'", processIdentifier, serverKeyName, srvCfg.Command)

	env := make(map[string]string)
	if srvCfg.Env != nil {
		for k, v := range srvCfg.Env {
			env[k] = v
		}
	}
	// Add standard MCP environment variables
	env["MCP_SERVER_NAME"] = serverKeyName
	// Add connection-related environment variables from global config
	for connKey, connCfg := range m.config.Connections {
		prefix := fmt.Sprintf("MCP_CONN_%s_", strings.ToUpper(connKey))
		env[prefix+"TRANSPORT"] = connCfg.Transport
		if connCfg.Port > 0 {
			env[prefix+"PORT"] = fmt.Sprintf("%d", connCfg.Port)
		}
		if connCfg.Host != "" {
			env[prefix+"HOST"] = connCfg.Host
		}
		if connCfg.Path != "" {
			env[prefix+"PATH"] = connCfg.Path
		}
	}

	proc, err := runtime.NewProcess(srvCfg.Command, srvCfg.Args, runtime.ProcessOptions{
		Env:     env,
		WorkDir: srvCfg.WorkDir,
		Name:    processIdentifier, // runtime.Process uses this for its internal tracking (e.g., PID file name)
	})
	if err != nil {
		return fmt.Errorf("failed to create process structure for '%s' (server '%s'): %w", processIdentifier, serverKeyName, err)
	}
	if err := proc.Start(); err != nil {
		return fmt.Errorf("failed to start process '%s' (server '%s'): %w", processIdentifier, serverKeyName, err)
	}

	m.servers[serverKeyName].Process = proc
	m.logger.Info("Process '%s' for server '%s' started", processIdentifier, serverKeyName)
	return nil
}

// StopServer stops a server using its fixed identifier
func (m *Manager) StopServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("server '%s' not found in manager", name)
	}
	srvCfg := instance.Config
	fixedIdentifier := fmt.Sprintf("mcp-compose-%s", name)

	currentStatus, _ := m.getServerStatusUnsafe(name, fixedIdentifier)
	if currentStatus != "running" {
		m.logger.Info("Server '%s' (identifier: %s) is not running, nothing to stop", name, fixedIdentifier)
		return nil // Or return an error if it was expected to be running
	}

	m.logger.Info("Stopping server '%s' (identifier: %s)...", name, fixedIdentifier)

	if srvCfg.Lifecycle.PreStop != "" {
		m.logger.Info("Running pre-stop hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PreStop); err != nil {
			m.logger.Warning("Pre-stop hook for server '%s' failed: %v", name, err) // Log but continue stopping
		}
	}

	if instance.ResourcesWatcher != nil {
		instance.ResourcesWatcher.Stop()
		instance.ResourcesWatcher = nil
		m.logger.Debug("Resource watcher stopped for server '%s'", name)
	}

	var stopErr error
	if instance.IsContainer {
		m.logger.Info("Stopping container '%s' for server '%s'", fixedIdentifier, name)
		stopErr = m.containerRuntime.StopContainer(fixedIdentifier) // Stop by fixed name
		if stopErr != nil {
			m.logger.Error("Failed to stop container '%s' for server '%s': %v", fixedIdentifier, name, stopErr)
		}
		instance.ContainerID = "" // Clear the runtime ID
	} else if instance.Process != nil {
		m.logger.Info("Stopping process '%s' for server '%s'", fixedIdentifier, name)
		stopErr = instance.Process.Stop() // Assumes Process.Stop uses the name it was initialized with
		if stopErr != nil {
			m.logger.Error("Failed to stop process '%s' for server '%s': %v", fixedIdentifier, name, stopErr)
		}
		instance.Process = nil
	} else {
		m.logger.Warning("Server '%s' (identifier: %s) was marked to stop but had no active container or process reference", name, fixedIdentifier)
	}

	instance.Status = "stopped"
	instance.HealthStatus = "unknown"
	m.logger.Info("Server '%s' (identifier: %s) has been stopped", name, fixedIdentifier)

	if srvCfg.Lifecycle.PostStop != "" {
		m.logger.Info("Running post-stop hook for server '%s'", name)
		if err := m.runLifecycleHook(srvCfg.Lifecycle.PostStop); err != nil {
			m.logger.Warning("Post-stop hook for server '%s' failed: %v", name, err)
		}
	}
	return stopErr // Return the error from the stop operation, if any
}

// GetServerStatus returns the status of a server, using the fixed identifier.
// This public method ensures locking.
func (m *Manager) GetServerStatus(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fixedIdentifier := fmt.Sprintf("mcp-compose-%s", name)
	return m.getServerStatusUnsafe(name, fixedIdentifier)
}

// getServerStatusUnsafe is the internal implementation without locking, for use by other locked methods.
func (m *Manager) getServerStatusUnsafe(name string, fixedIdentifier string) (string, error) {
	instance, ok := m.servers[name]
	if !ok {
		return "unknown", fmt.Errorf("server '%s' not found in manager's list", name)
	}

	var currentRuntimeStatus string
	var err error

	if instance.IsContainer {
		// Try by known ContainerID first for precision, then by fixedIdentifier as fallback
		if instance.ContainerID != "" {
			currentRuntimeStatus, err = m.containerRuntime.GetContainerStatus(instance.ContainerID)
			if err != nil { // e.g., ID is stale
				m.logger.Debug("Failed to get status by ID for %s (%s), trying by name %s: %v", name, instance.ContainerID, fixedIdentifier, err)
				currentRuntimeStatus, err = m.containerRuntime.GetContainerStatus(fixedIdentifier)
			}
		} else { // No ContainerID known, must use fixed name
			currentRuntimeStatus, err = m.containerRuntime.GetContainerStatus(fixedIdentifier)
		}
		if err != nil {
			m.logger.Warning("Error getting container status for '%s' (identifier: %s): %v", name, fixedIdentifier, err)
			// If the runtime returns "stopped" or "exited" along with an error (e.g. "No such container"),
			// then currentRuntimeStatus might already be set correctly by GetContainerStatus.
			// If currentRuntimeStatus is still empty, set to "unknown".
			if currentRuntimeStatus == "" {
				currentRuntimeStatus = "unknown"
			}
		}
	} else { // Process-based server
		proc, findErr := runtime.FindProcess(fixedIdentifier)
		if findErr != nil {
			currentRuntimeStatus = "stopped" // Process not found (e.g., PID file missing)
		} else {
			isRunning, runErr := proc.IsRunning()
			if runErr != nil || !isRunning {
				currentRuntimeStatus = "stopped"
			} else {
				currentRuntimeStatus = "running"
			}
		}
	}
	instance.Status = currentRuntimeStatus // Update cached status
	return currentRuntimeStatus, err       // Return error from runtime if any
}

// ShowLogs displays logs for a server using the fixed identifier
func (m *Manager) ShowLogs(name string, follow bool) error {
	instance, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("server '%s' not found for showing logs", name)
	}
	fixedIdentifier := fmt.Sprintf("mcp-compose-%s", name)
	m.logger.Debug("Requesting logs for server '%s' (identifier: %s)", name, fixedIdentifier)

	if instance.IsContainer {
		// While instance.ContainerID might be more precise if available and current,
		// using fixedIdentifier aligns with how the proxy would refer to it and how Start/Stop work.
		// If the container was recreated with the same fixed name, this would get logs from the new one.
		return m.containerRuntime.ShowContainerLogs(fixedIdentifier, follow)
	} else { // Process-based server
		proc, err := runtime.FindProcess(fixedIdentifier)
		if err != nil {
			return fmt.Errorf("process for server '%s' (identifier: %s) not found: %w", name, fixedIdentifier, err)
		}
		return proc.ShowLogs(follow)
	}
}

// --- ResourcesWatcher (ensure it's fully defined or use a simplified stub) ---
type ResourcesWatcher struct {
	config       *config.ServerConfig
	fsWatcher    *fsnotify.Watcher // Simplified to one watcher for the example
	stopCh       chan struct{}
	active       bool
	logger       *logging.Logger
	mu           sync.Mutex
	changedFiles map[string]time.Time
	ticker       *time.Ticker
}

func NewResourcesWatcher(cfg *config.ServerConfig, loggerInstance ...*logging.Logger) (*ResourcesWatcher, error) {
	var logger *logging.Logger
	if len(loggerInstance) > 0 && loggerInstance[0] != nil {
		logger = loggerInstance[0]
	} else {
		logger = logging.NewLogger("info") // Default if no logger passed
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}
	return &ResourcesWatcher{
		config:       cfg,
		fsWatcher:    watcher,
		stopCh:       make(chan struct{}),
		logger:       logger,
		changedFiles: make(map[string]time.Time),
	}, nil
}

func (w *ResourcesWatcher) Start() {
	w.mu.Lock()
	if w.active {
		w.mu.Unlock()
		w.logger.Debug("Resource watcher already active.")
		return
	}
	w.active = true
	w.mu.Unlock()

	w.logger.Info("Starting resource watcher for paths: %v", w.config.Resources.Paths)

	for _, rp := range w.config.Resources.Paths {
		if rp.Watch {
			// Walk the path to add all subdirectories
			err := filepath.WalkDir(rp.Source, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					w.logger.Error("Error walking path %s for watcher: %v", path, err)
					return err // Or return nil to continue walking other parts
				}
				if d.IsDir() {
					w.logger.Debug("Adding path to watcher: %s", path)
					if addErr := w.fsWatcher.Add(path); addErr != nil {
						w.logger.Error("Failed to add path %s to watcher: %v", path, addErr)
						// Potentially continue to try and watch other directories
					}
				}
				return nil
			})
			if err != nil {
				w.logger.Error("Error setting up watch for path %s: %v", rp.Source, err)
				// Potentially stop or handle error
			}
		}
	}

	syncInterval := 5 * time.Second // Default sync interval
	if w.config.Resources.SyncInterval != "" {
		parsedInterval, err := time.ParseDuration(w.config.Resources.SyncInterval)
		if err == nil {
			syncInterval = parsedInterval
		} else {
			w.logger.Warning("Invalid resource sync interval '%s', using default %v: %v", w.config.Resources.SyncInterval, syncInterval, err)
		}
	}
	w.ticker = time.NewTicker(syncInterval)

	go func() {
		defer w.cleanupWatcher()
		for {
			select {
			case <-w.stopCh:
				w.logger.Info("Resource watcher stop signal received.")
				return
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					w.logger.Info("Watcher events channel closed.")
					return
				}
				if w.shouldProcessEvent(event) {
					w.recordChange(event.Name)
				}
			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					w.logger.Info("Watcher errors channel closed.")
					return
				}
				w.logger.Error("Watcher error: %v", err)
			case <-w.ticker.C:
				w.processChanges()
			}
		}
	}()
}
func (w *ResourcesWatcher) cleanupWatcher() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ticker != nil {
		w.ticker.Stop()
	}
	if w.fsWatcher != nil {
		w.fsWatcher.Close()
	}
	w.active = false
	w.logger.Info("Resource watcher cleaned up.")
}

func (w *ResourcesWatcher) shouldProcessEvent(event fsnotify.Event) bool {
	// Basic filtering, can be expanded
	if strings.HasPrefix(filepath.Base(event.Name), ".") { // Ignore hidden files/dirs
		return false
	}
	// Only interested in these operations
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename)
}

func (w *ResourcesWatcher) recordChange(path string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.changedFiles[path] = time.Now()
	w.logger.Debug("Resource change detected: %s", path)
}

func (w *ResourcesWatcher) processChanges() {
	w.mu.Lock()
	if len(w.changedFiles) == 0 {
		w.mu.Unlock()
		return
	}
	// Create a copy to process, then clear the map
	changesToProcess := make(map[string]time.Time, len(w.changedFiles))
	for k, v := range w.changedFiles {
		changesToProcess[k] = v
	}
	w.changedFiles = make(map[string]time.Time) // Clear original map
	w.mu.Unlock()

	if len(changesToProcess) == 0 {
		return
	}

	mappedChanges := make(map[string]string) // Path -> "file" | "directory" | "deleted"
	for changedPath := range changesToProcess {
		// Determine type or if deleted
		info, err := os.Stat(changedPath)
		changeType := "unknown"
		if err == nil {
			changeType = "file"
			if info.IsDir() {
				changeType = "directory"
			}
		} else if os.IsNotExist(err) {
			changeType = "deleted"
		} else {
			w.logger.Warning("Error stating changed path %s: %v", changedPath, err)
			continue // Skip if cannot determine state
		}

		// Map this changedPath to the target path in the MCP server's context
		var targetPath string
		foundMapping := false
		for _, rp := range w.config.Resources.Paths {
			if strings.HasPrefix(changedPath, rp.Source) {
				relPath, _ := filepath.Rel(rp.Source, changedPath)
				targetPath = filepath.Join(rp.Target, relPath)
				mappedChanges[targetPath] = changeType
				foundMapping = true
				break
			}
		}
		if !foundMapping {
			w.logger.Debug("No resource mapping found for changed path: %s", changedPath)
		}
	}

	if len(mappedChanges) > 0 {
		w.notifyChanges(mappedChanges)
	}
}

func (w *ResourcesWatcher) notifyChanges(changes map[string]string) {
	// Placeholder for actual notification
	// This would involve constructing an MCP resources/list-changed notification
	// and sending it to the associated MCP server instance.
	changesJSON, _ := json.MarshalIndent(changes, "", "  ")
	w.logger.Info("Server notified of resource changes: %s", string(changesJSON))
}

func (w *ResourcesWatcher) Stop() {
	w.mu.Lock()
	if !w.active {
		w.mu.Unlock()
		return
	}
	// Set active to false first to prevent new operations from starting
	w.active = false
	w.mu.Unlock()

	// Signal the watcher goroutine to stop by closing stopCh
	// Check if stopCh is nil or already closed to prevent panic
	w.mu.Lock()
	if w.stopCh != nil {
		select {
		case <-w.stopCh:
			// Already closed or being closed
		default:
			close(w.stopCh) // Close the channel
			w.stopCh = nil  // Mark as closed
		}
	}
	w.mu.Unlock() // Unlock before logging
	w.logger.Info("Resource watcher stop requested.")
}

// --- HealthCheck and other utility methods ---
// --- HealthCheck and other utility methods ---
func (m *Manager) startHealthCheck(serverName, fixedIdentifier string) { // Added fixedIdentifier
	instance, ok := m.servers[serverName]
	if !ok {
		m.logger.Error("HealthCheck: Server '%s' not found.", serverName)
		return
	}
	healthCfg := instance.Config.Lifecycle.HealthCheck
	if healthCfg.Endpoint == "" {
		m.logger.Debug("HealthCheck: No endpoint for server '%s'.", serverName)
		return
	}
	interval, err := time.ParseDuration(healthCfg.Interval)
	if err != nil {
		interval = 30 * time.Second // Default
		m.logger.Warning("HealthCheck: Invalid interval '%s' for '%s', using default %v: %v", healthCfg.Interval, serverName, interval, err)
	}
	timeout, err := time.ParseDuration(healthCfg.Timeout)
	if err != nil {
		timeout = 5 * time.Second // Default
		m.logger.Warning("HealthCheck: Invalid timeout '%s' for '%s', using default %v: %v", healthCfg.Timeout, serverName, timeout, err)
	}
	retries := healthCfg.Retries
	if retries <= 0 {
		retries = 3 // Default
	}
	m.logger.Info("HealthCheck: Starting for server '%s' (identifier: %s), endpoint: %s, interval: %v, timeout: %v, retries: %d",
		serverName, fixedIdentifier, healthCfg.Endpoint, interval, timeout, retries)

	go func() {
		// Create a new ticker for this specific health check goroutine
		healthCheckTicker := time.NewTicker(interval)
		defer healthCheckTicker.Stop()

		failCount := 0

		// Use a proper loop that the linter understands
		for {
			select {
			case <-healthCheckTicker.C:
				// Check if manager still has this server and if it's supposed to be running
				m.mu.Lock() // Lock for reading server status
				instance, stillExists := m.servers[serverName]
				targetStatus := ""
				if stillExists {
					targetStatus = instance.Status
				}
				m.mu.Unlock() // Unlock after reading

				if !stillExists || targetStatus != "running" {
					m.logger.Info("HealthCheck: Server '%s' no longer exists or is not running, stopping health checks.", serverName)
					return // Exit this goroutine
				}

				// Pass fixedIdentifier to checkServerHealth
				healthy, checkErr := m.checkServerHealth(serverName, fixedIdentifier, healthCfg.Endpoint, timeout)

				m.mu.Lock()                                   // Lock for updating instance health status
				instance, stillExists = m.servers[serverName] // Re-fetch instance under lock
				if !stillExists {                             // Check again in case server was removed during health check
					m.mu.Unlock()
					m.logger.Info("HealthCheck: Server '%s' removed during health check, stopping checks.", serverName)
					return
				}

				if healthy {
					if instance.HealthStatus != "healthy" { // Log only on change
						m.logger.Info("HealthCheck: Server '%s' (identifier: %s) is now healthy.", serverName, fixedIdentifier)
					}
					instance.HealthStatus = "healthy"
					failCount = 0
				} else {
					failCount++
					instance.HealthStatus = fmt.Sprintf("failing (%d/%d)", failCount, retries)
					m.logger.Warning("HealthCheck: Server '%s' (identifier: %s) failed check %d/%d. Error: %v", serverName, fixedIdentifier, failCount, retries, checkErr)

					if failCount >= retries {
						instance.HealthStatus = "unhealthy"
						m.logger.Error("HealthCheck: Server '%s' (identifier: %s) is now unhealthy after %d retries.", serverName, fixedIdentifier, retries)

						if healthCfg.Action == "restart" {
							m.logger.Info("HealthCheck: Restart action configured for unhealthy server '%s'. Attempting restart...", serverName)
							// Important: Unlock before calling StopServer/StartServer to avoid deadlock if they also lock.
							// The restart itself should be in a new goroutine to not block the health ticker.
							m.mu.Unlock() // Unlock BEFORE starting the restart goroutine

							go func(sName string) { // Pass serverName to the new goroutine
								m.logger.Info("HealthCheck: Restart goroutine initiated for '%s'.", sName)
								if err := m.StopServer(sName); err != nil { // Use sName (which is serverName)
									m.logger.Error("HealthCheck: Failed to stop unhealthy server '%s' for restart: %v", sName, err)
								} else {
									m.logger.Info("HealthCheck: Server '%s' stopped for restart. Waiting briefly...", sName)
									time.Sleep(5 * time.Second)                  // Optional: delay before restart
									if err := m.StartServer(sName); err != nil { // Use sName
										m.logger.Error("HealthCheck: Failed to restart server '%s': %v", sName, err)
									} else {
										m.logger.Info("HealthCheck: Server '%s' restarted successfully due to health check.", sName)
									}
								}
							}(serverName) // Pass serverName to the goroutine
							return // Stop this health check goroutine; a new one will be started if the server restarts successfully.
						}
						// else, server remains unhealthy, health checks continue if no restart action
					}
				}
				m.mu.Unlock() // Unlock after updating status

			case <-m.ctx.Done():
				// Exit when the manager context is cancelled
				m.logger.Info("HealthCheck: Manager shutting down, stopping health checks for '%s'", serverName)
				return
			}
		}
	}()
}

func (m *Manager) checkServerHealth(serverName, fixedIdentifier, endpoint string, timeout time.Duration) (bool, error) {
	instance, ok := m.servers[serverName]
	if !ok {
		return false, fmt.Errorf("server '%s' not found for health check", serverName)
	}

	var url string
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		url = endpoint
	} else {
		// Attempt to construct URL if endpoint is relative (e.g., "/health")
		// This part is tricky without knowing the server's actual listening port.
		// If the server exposes a port in mcp-compose.yaml, we could try to use that.
		hostPort := "80" // Default, likely incorrect
		if instance.IsContainer && len(instance.Config.Ports) > 0 {
			// Example: "8080:80" -> use "8080"
			parts := strings.Split(instance.Config.Ports[0], ":")
			if len(parts) > 0 {
				hostPort = parts[0]
			}
		} else if !instance.IsContainer && len(m.config.Connections) > 0 {
			// For process, check global connections for an HTTP one
			for _, conn := range m.config.Connections {
				// This is a heuristic; might need more specific config for health check port
				if (strings.HasPrefix(conn.Transport, "http")) && conn.Port > 0 {
					hostPort = fmt.Sprintf("%d", conn.Port)
					break
				}
			}
		}
		// This assumes the health endpoint is on localhost. If containers are on a docker network,
		// and the proxy is also on that network, the proxy could health check them by container name,
		// but this health check is run by mcp-compose itself.
		url = fmt.Sprintf("http://localhost:%s%s", hostPort, endpoint)
		if instance.IsContainer && m.containerRuntime.GetRuntimeName() != "none" && instance.Config.NetworkMode == "host" {
			// If host network mode, localhost is fine for container.
		} else if instance.IsContainer && m.containerRuntime.GetRuntimeName() != "none" {
			m.logger.Debug("HealthCheck: For container %s, URL %s might only work if port %s is mapped to host. For internal checks, use container name or IP.", fixedIdentifier, url, hostPort)
			// A more advanced health check might exec into the container to curl localhost:containerPort/endpoint
		}

	}

	client := http.Client{Timeout: timeout}
	m.logger.Debug("HealthCheck: Pinging %s for server '%s'", url, serverName)
	resp, err := client.Get(url)
	if err != nil {
		return false, fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 { // Consider 3xx as healthy too for some cases
		return true, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256)) // Read a bit of body for error
	return false, fmt.Errorf("bad status %d from %s: %s", resp.StatusCode, url, string(body))
}

func (m *Manager) runLifecycleHook(hookScript string) error {
	m.logger.Info("Running lifecycle hook: %s", hookScript)
	cmd := exec.Command("sh", "-c", hookScript)
	cmd.Env = os.Environ() // Inherit current environment
	// Set working directory for the hook script to the project directory
	cmd.Dir = m.projectDir

	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		m.logger.Debug("Lifecycle hook '%s' output:\n%s", hookScript, string(output))
	}
	if err != nil {
		return fmt.Errorf("lifecycle hook script '%s' failed: %w. Output: %s", hookScript, err, string(output))
	}
	m.logger.Info("Lifecycle hook '%s' completed successfully.", hookScript)
	return nil
}

// ensureNetworkExists needs a lock if it modifies m.networks and is called concurrently.
// If called only from StartServer (which is locked), internal lock might not be needed.
// Let's assume it might be called externally or by multiple StartServer goroutines in future.
func (m *Manager) ensureNetworkExists(networkName string, lockedByCaller bool) error {
	if !lockedByCaller {
		m.mu.Lock()
		defer m.mu.Unlock()
	}

	if m.networks[networkName] { // Check if already marked as handled in this session
		m.logger.Debug("Network '%s' already processed in this session.", networkName)
		return nil
	}

	if m.containerRuntime == nil || m.containerRuntime.GetRuntimeName() == "none" {
		m.logger.Debug("No container runtime, skipping network creation for '%s'", networkName)
		return nil
	}

	m.logger.Info("Ensuring Docker/Podman network '%s' exists...", networkName)
	exists, err := m.containerRuntime.NetworkExists(networkName)
	if err != nil {
		return fmt.Errorf("failed to check if network '%s' exists: %w", networkName, err)
	}

	if !exists {
		m.logger.Info("Network '%s' does not exist, attempting to create it...", networkName)
		if err := m.containerRuntime.CreateNetwork(networkName); err != nil {
			return fmt.Errorf("failed to create network '%s': %w", networkName, err)
		}
		m.logger.Info("Network '%s' created successfully.", networkName)
	} else {
		m.logger.Debug("Network '%s' already exists.", networkName)
	}

	m.networks[networkName] = true // Mark as handled
	return nil
}

// Add this method to internal/server/manager.go if it's missing
func (m *Manager) Shutdown() error {
	m.logger.Info("MANAGER: Starting shutdown process")

	// Cancel all contexts
	if m.cancel != nil {
		m.cancel()
	}

	// Stop all health checkers
	m.healthCheckMu.Lock()
	for name, cancel := range m.healthCheckers {
		m.logger.Debug("MANAGER: Stopping health checker for %s", name)
		cancel()
	}
	m.healthCheckers = make(map[string]context.CancelFunc)
	m.healthCheckMu.Unlock()

	// Stop all servers
	m.mu.RLock()
	serverNames := make([]string, 0, len(m.servers))
	for name := range m.servers {
		serverNames = append(serverNames, name)
	}
	m.mu.RUnlock()

	for _, name := range serverNames {
		if err := m.StopServer(name); err != nil {
			m.logger.Error("MANAGER: Error stopping server %s during shutdown: %v", name, err)
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("MANAGER: All goroutines finished")
	case <-time.After(30 * time.Second):
		m.logger.Warning("MANAGER: Timeout waiting for goroutines to finish")
	}

	close(m.shutdownCh)
	return nil
}

func (m *Manager) initializeServerCapabilities(serverName string) error {
	instance, ok := m.servers[serverName]
	if !ok {
		return fmt.Errorf("server '%s' not found for capability initialization", serverName)
	}
	// This is a simplified initialization. A real one might involve an MCP "initialize" call.
	for _, capName := range instance.Config.Capabilities {
		instance.Capabilities[capName] = true
	}
	m.logger.Debug("Initialized capabilities for server '%s' from config: %v", serverName, instance.Capabilities)
	return nil
}

func (m *Manager) GetServerInstance(serverName string) (*ServerInstance, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	instance, exists := m.servers[serverName]
	return instance, exists
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
