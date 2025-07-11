package task_scheduler

import (
	"fmt"
	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"
	"mcpcompose/internal/dashboard" // Add this import for BroadcastActivity
	"time"
)

// Manager manages the task scheduler service
type Manager struct {
	config     *config.ComposeConfig
	runtime    container.Runtime
	configFile string
}

// NewManager creates a new task scheduler manager
func NewManager(cfg *config.ComposeConfig, runtime container.Runtime) *Manager {
	return &Manager{
		config:  cfg,
		runtime: runtime,
	}
}

// SetConfigFile sets the configuration file path
func (m *Manager) SetConfigFile(configFile string) {
	m.configFile = configFile
}

// Start starts the task scheduler service
func (m *Manager) Start() error {
	// Broadcast start attempt
	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Starting task scheduler service...",
		map[string]interface{}{
			"port": m.config.TaskScheduler.Port,
			"host": m.config.TaskScheduler.Host,
		})

	// Check if already running
	status, err := m.runtime.GetContainerStatus("mcp-compose-task-scheduler")
	if err == nil && status == "running" {
		dashboard.BroadcastActivity("WARN", "service", "task-scheduler", "",
			"Task scheduler is already running",
			map[string]interface{}{
				"status": status,
			})
		return fmt.Errorf("task scheduler is already running")
	}

	// Ensure defaults are set
	if m.config.TaskScheduler.Port == 0 {
		m.config.TaskScheduler.Port = 8080
	}
	if m.config.TaskScheduler.Host == "" {
		m.config.TaskScheduler.Host = "0.0.0.0"
	}
	if m.config.TaskScheduler.LogLevel == "" {
		m.config.TaskScheduler.LogLevel = "info"
	}
	if m.config.TaskScheduler.CPUs == "" {
		m.config.TaskScheduler.CPUs = "2.0"
	}
	if m.config.TaskScheduler.Memory == "" {
		m.config.TaskScheduler.Memory = "1g"
	}

	// Build the image
	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Building task scheduler Docker image...",
		nil)

	if err := m.buildImage(); err != nil {
		dashboard.BroadcastActivity("ERROR", "service", "task-scheduler", "",
			"Failed to build task scheduler image",
			map[string]interface{}{
				"error": err.Error(),
			})
		return fmt.Errorf("failed to build task scheduler image: %w", err)
	}

	// Ensure network exists
	networkExists, _ := m.runtime.NetworkExists("mcp-net")
	if !networkExists {
		dashboard.BroadcastActivity("INFO", "network", "task-scheduler", "",
			"Creating mcp-net network...",
			nil)

		if err := m.runtime.CreateNetwork("mcp-net"); err != nil {
			dashboard.BroadcastActivity("ERROR", "network", "task-scheduler", "",
				"Failed to create mcp-net network",
				map[string]interface{}{
					"error": err.Error(),
				})
			return fmt.Errorf("failed to create mcp-net network: %w", err)
		}
	}

	// Set up environment
	env := m.buildEnvironment()

	// Set up volumes
	volumes := []string{
		"task-scheduler-data:/data",
	}
	if m.config.TaskScheduler.Workspace != "" {
		volumes = append(volumes, fmt.Sprintf("%s:/workspace:rw", m.config.TaskScheduler.Workspace))
	}
	volumes = append(volumes, m.config.TaskScheduler.Volumes...)

	// Container options
	opts := &container.ContainerOptions{
		Name:     "mcp-compose-task-scheduler",
		Image:    "mcp-compose-task-scheduler:latest",
		Ports:    []string{fmt.Sprintf("%d:%d", m.config.TaskScheduler.Port, m.config.TaskScheduler.Port)},
		Env:      env,
		Networks: []string{"mcp-net"},
		Volumes:  volumes,
		User:     "root",
		CPUs:     m.config.TaskScheduler.CPUs,
		Memory:   m.config.TaskScheduler.Memory,
		Security: container.SecurityConfig{
			TrustedImage:       true,
			AllowPrivilegedOps: true,
		},
		CapDrop:     []string{"SYS_ADMIN", "NET_ADMIN"},
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "task-scheduler",
		},
	}

	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Starting task scheduler container...",
		map[string]interface{}{
			"image": opts.Image,
			"ports": opts.Ports,
			"resources": map[string]interface{}{
				"cpus":   opts.CPUs,
				"memory": opts.Memory,
			},
		})

	containerID, err := m.runtime.StartContainer(opts)
	if err != nil {
		dashboard.BroadcastActivity("ERROR", "service", "task-scheduler", "",
			"Failed to start task scheduler container",
			map[string]interface{}{
				"error": err.Error(),
			})
		return fmt.Errorf("failed to start task scheduler container: %w", err)
	}

	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Task scheduler container started successfully",
		map[string]interface{}{
			"containerId": containerID[:12],
			"port":        m.config.TaskScheduler.Port,
		})

	// Wait for service to be ready
	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Waiting for task scheduler to become healthy...",
		nil)

	if err := m.waitForHealthy(30 * time.Second); err != nil {
		dashboard.BroadcastActivity("ERROR", "service", "task-scheduler", "",
			"Task scheduler failed health check",
			map[string]interface{}{
				"error":   err.Error(),
				"timeout": "30s",
			})
		return fmt.Errorf("task scheduler failed to start properly: %w", err)
	}

	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Task scheduler is now healthy and ready",
		map[string]interface{}{
			"port":        m.config.TaskScheduler.Port,
			"containerId": containerID[:12],
		})

	return nil
}

// Stop stops the task scheduler service
func (m *Manager) Stop() error {
	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Stopping task scheduler service...",
		nil)

	if err := m.runtime.StopContainer("mcp-compose-task-scheduler"); err != nil {
		dashboard.BroadcastActivity("ERROR", "service", "task-scheduler", "",
			"Failed to stop task scheduler container",
			map[string]interface{}{
				"error": err.Error(),
			})
		return fmt.Errorf("failed to stop task scheduler container: %w", err)
	}

	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Task scheduler stopped successfully",
		nil)

	return nil
}

// Restart restarts the task scheduler service
func (m *Manager) Restart() error {
	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Restarting task scheduler service...",
		nil)

	// Stop first (ignore errors)
	_ = m.Stop()

	// Wait a moment
	time.Sleep(constants.DefaultRetryDelay)

	// Start again
	if err := m.Start(); err != nil {
		dashboard.BroadcastActivity("ERROR", "service", "task-scheduler", "",
			"Failed to restart task scheduler",
			map[string]interface{}{
				"error": err.Error(),
			})
		return err
	}

	dashboard.BroadcastActivity("INFO", "service", "task-scheduler", "",
		"Task scheduler restarted successfully",
		nil)

	return nil
}

// Status returns the current status of the task scheduler
func (m *Manager) Status() (string, error) {
	status, err := m.runtime.GetContainerStatus("mcp-compose-task-scheduler")
	if err != nil {
		return "stopped", nil
	}
	return status, nil
}

// IsRunning checks if the task scheduler is currently running
func (m *Manager) IsRunning() bool {
	status, err := m.runtime.GetContainerStatus("mcp-compose-task-scheduler")
	return err == nil && status == "running"
}

// buildImage builds the task scheduler Docker image
func (m *Manager) buildImage() error {
	fmt.Println("Building task scheduler image...")
	// Implementation would build from Dockerfile.task-scheduler
	// For now, assume it exists or will be built externally
	return nil
}

// buildEnvironment builds the environment variables map
func (m *Manager) buildEnvironment() map[string]string {
	env := map[string]string{
		"TZ":                                 "America/New_York",
		"MCP_CRON_SERVER_TRANSPORT":          "sse",
		"MCP_CRON_SERVER_ADDRESS":            m.config.TaskScheduler.Host,
		"MCP_CRON_SERVER_PORT":               fmt.Sprintf("%d", m.config.TaskScheduler.Port),
		"MCP_CRON_DATABASE_PATH":             m.config.TaskScheduler.DatabasePath,
		"MCP_CRON_DATABASE_ENABLED":          "true",
		"MCP_CRON_LOGGING_LEVEL":             m.config.TaskScheduler.LogLevel,
		"MCP_CRON_SCHEDULER_DEFAULT_TIMEOUT": "10m",
	}

	// Add activity broadcasting configuration
	env["MCP_CRON_ACTIVITY_WEBHOOK"] = "http://mcp-compose-dashboard:3001/api/activity"

	// Add OpenRouter configuration
	if m.config.TaskScheduler.OpenRouterAPIKey != "" {
		env["OPENROUTER_API_KEY"] = m.config.TaskScheduler.OpenRouterAPIKey
		env["USE_OPENROUTER"] = "true"
		env["OPENROUTER_ENABLED"] = "true"
	}
	if m.config.TaskScheduler.OpenRouterModel != "" {
		env["OPENROUTER_MODEL"] = m.config.TaskScheduler.OpenRouterModel
	}

	// Add Ollama configuration
	if m.config.TaskScheduler.OllamaURL != "" {
		env["MCP_CRON_OLLAMA_BASE_URL"] = m.config.TaskScheduler.OllamaURL
		env["MCP_CRON_OLLAMA_ENABLED"] = "true"
	}
	if m.config.TaskScheduler.OllamaModel != "" {
		env["MCP_CRON_OLLAMA_DEFAULT_MODEL"] = m.config.TaskScheduler.OllamaModel
	}

	// Add MCP proxy configuration
	if m.config.TaskScheduler.MCPProxyURL != "" {
		env["MCP_PROXY_URL"] = m.config.TaskScheduler.MCPProxyURL
	}
	if m.config.TaskScheduler.MCPProxyAPIKey != "" {
		env["MCP_PROXY_API_KEY"] = m.config.TaskScheduler.MCPProxyAPIKey
	}

	// Add OpenWebUI configuration
	if m.config.TaskScheduler.OpenWebUIEnabled {
		env["MCP_CRON_OPENWEBUI_ENABLED"] = "true"
	} else {
		env["MCP_CRON_OPENWEBUI_ENABLED"] = "false"
	}

	// Add custom environment variables
	for key, value := range m.config.TaskScheduler.Env {
		env[key] = value
	}

	return env
}

// waitForHealthy waits for the task scheduler to become healthy
func (m *Manager) waitForHealthy(timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		if m.IsRunning() {
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("timeout waiting for task scheduler to become healthy")
}

// GetLogs retrieves logs from the task scheduler container
func (m *Manager) GetLogs(follow bool) error {
	return m.runtime.ShowContainerLogs("mcp-compose-task-scheduler", follow)
}
