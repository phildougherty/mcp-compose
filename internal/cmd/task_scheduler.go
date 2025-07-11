// internal/cmd/task_scheduler.go
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"

	"github.com/spf13/cobra"
)

func NewTaskSchedulerCommand() *cobra.Command {
	var (
		port             int
		host             string
		enable           bool
		disable          bool
		native           bool
		containerMode    bool
		dbPath           string
		workspace        string
		logLevel         string
		mcpProxyURL      string
		mcpProxyAPIKey   string
		ollamaURL        string
		ollamaModel      string
		openrouterAPIKey string
		openrouterModel  string
		cpus             string
		memory           string
		healthCheck      bool
		debug            bool
	)

	cmd := &cobra.Command{
		Use:   "task-scheduler",
		Short: "Manage the task scheduler service",
		Long: `Start, stop, enable, or disable the MCP task scheduler service.
        
The task scheduler provides intelligent task automation with:
- Cron-like scheduling
- AI-powered task execution  
- Persistent task storage
- Integration with MCP proxy

Examples:
  mcp-compose task-scheduler                    # Start task scheduler
  mcp-compose task-scheduler --native           # Run natively
  mcp-compose task-scheduler --enable           # Enable in config
  mcp-compose task-scheduler --disable          # Disable service`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("file")
			cfg, err := config.LoadConfig(configFile)
			if err != nil {

				return fmt.Errorf("failed to load config: %w", err)
			}

			if enable {

				return enableTaskScheduler(configFile, cfg)
			}

			if disable {

				return disableTaskScheduler(configFile, cfg)
			}

			// Use task_scheduler config values as defaults if not set via flags
			if cfg.TaskScheduler.Enabled {
				if port == 0 && cfg.TaskScheduler.Port != 0 {
					port = cfg.TaskScheduler.Port
				}
				if host == "" && cfg.TaskScheduler.Host != "" {
					host = cfg.TaskScheduler.Host
				}
				if dbPath == "" && cfg.TaskScheduler.DatabasePath != "" {
					dbPath = cfg.TaskScheduler.DatabasePath
				}
				if workspace == "" && cfg.TaskScheduler.Workspace != "" {
					workspace = cfg.TaskScheduler.Workspace
				}
				if logLevel == "" && cfg.TaskScheduler.LogLevel != "" {
					logLevel = cfg.TaskScheduler.LogLevel
				}
				if mcpProxyURL == "" && cfg.TaskScheduler.MCPProxyURL != "" {
					mcpProxyURL = cfg.TaskScheduler.MCPProxyURL
				}
				if mcpProxyAPIKey == "" && cfg.TaskScheduler.MCPProxyAPIKey != "" {
					mcpProxyAPIKey = cfg.TaskScheduler.MCPProxyAPIKey
				}
				if ollamaURL == "" && cfg.TaskScheduler.OllamaURL != "" {
					ollamaURL = cfg.TaskScheduler.OllamaURL
				}
				if ollamaModel == "" && cfg.TaskScheduler.OllamaModel != "" {
					ollamaModel = cfg.TaskScheduler.OllamaModel
				}
				if openrouterAPIKey == "" && cfg.TaskScheduler.OpenRouterAPIKey != "" {
					openrouterAPIKey = cfg.TaskScheduler.OpenRouterAPIKey
				}
				if openrouterModel == "" && cfg.TaskScheduler.OpenRouterModel != "" {
					openrouterModel = cfg.TaskScheduler.OpenRouterModel
				}
				if cpus == constants.ResourceLimitCPUs && cfg.TaskScheduler.CPUs != "" {
					cpus = cfg.TaskScheduler.CPUs
				}
				if memory == constants.ResourceLimitMemory && cfg.TaskScheduler.Memory != "" {
					memory = cfg.TaskScheduler.Memory
				}
			}

			// Use config values from individual server config if available (as fallback)
			if serverConfig, exists := cfg.Servers["task-scheduler"]; exists {
				if port == 0 && serverConfig.HttpPort != 0 {
					port = serverConfig.HttpPort
				}
				// Use environment variables from config
				if mcpProxyURL == "" {
					if url, ok := serverConfig.Env["MCP_PROXY_URL"]; ok {
						mcpProxyURL = url
					}
				}
				if mcpProxyAPIKey == "" {
					if key, ok := serverConfig.Env["MCP_PROXY_API_KEY"]; ok {
						mcpProxyAPIKey = key
					}
				}
				if ollamaURL == "" {
					if url, ok := serverConfig.Env["MCP_CRON_OLLAMA_BASE_URL"]; ok {
						ollamaURL = url
					}
				}
				if ollamaModel == "" {
					if model, ok := serverConfig.Env["MCP_CRON_OLLAMA_DEFAULT_MODEL"]; ok {
						ollamaModel = model
					}
				}
				if openrouterAPIKey == "" {
					if key, ok := serverConfig.Env["OPENROUTER_API_KEY"]; ok {
						openrouterAPIKey = key
					}
				}
				if openrouterModel == "" {
					if model, ok := serverConfig.Env["OPENROUTER_MODEL"]; ok {
						openrouterModel = model
					}
				}
			}

			// Set final defaults
			if port == 0 {
				port = constants.TaskSchedulerDefaultPort
			}
			if host == "" {
				host = constants.DefaultHostInterface
			}
			if workspace == "" {
				workspace = constants.DefaultWorkspacePath // Default workspace
			}
			if dbPath == "" {
				dbPath = constants.DefaultDatabasePath
			}

			fmt.Printf("Starting task scheduler with port: %d\n", port)

			// Choose mode: native or containerized
			if native {

				return runNativeTaskScheduler(cfg, port, host, dbPath, workspace, logLevel, debug)
			} else {

				return runContainerizedTaskScheduler(cfg, configFile, port, host, dbPath, workspace, logLevel, mcpProxyURL, mcpProxyAPIKey, ollamaURL, ollamaModel, openrouterAPIKey, openrouterModel, cpus, memory, healthCheck, debug)
			}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Task scheduler port (default: from config or 8080)")
	cmd.Flags().StringVar(&host, "host", "", "Task scheduler host interface (default: 0.0.0.0)")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the task scheduler in config")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the task scheduler")
	cmd.Flags().BoolVar(&native, "native", false, "Run task scheduler natively")
	cmd.Flags().BoolVar(&containerMode, "container", true, "Run task scheduler as container")
	cmd.Flags().StringVar(&dbPath, "db-path", "", "Database path (default: /data/task-scheduler.db)")
	cmd.Flags().StringVar(&workspace, "workspace", "", "Workspace directory (default: /home/phil)")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&mcpProxyURL, "mcp-proxy-url", "", "MCP Proxy URL")
	cmd.Flags().StringVar(&mcpProxyAPIKey, "mcp-proxy-api-key", "", "MCP Proxy API key")
	cmd.Flags().StringVar(&ollamaURL, "ollama-url", "", "Ollama base URL")
	cmd.Flags().StringVar(&ollamaModel, "ollama-model", "", "Ollama model")
	cmd.Flags().StringVar(&openrouterAPIKey, "openrouter-api-key", "", "OpenRouter API key")
	cmd.Flags().StringVar(&openrouterModel, "openrouter-model", "", "OpenRouter model")
	cmd.Flags().StringVar(&cpus, "cpus", constants.ResourceLimitCPUs, "CPU limit")
	cmd.Flags().StringVar(&memory, "memory", constants.ResourceLimitMemory, "Memory limit")
	cmd.Flags().BoolVar(&healthCheck, "health-check", true, "Enable health checks")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")


	return cmd
}

func enableTaskScheduler(configFile string, cfg *config.ComposeConfig) error {
	fmt.Println("Enabling task scheduler and replacing mcp-cron-oi...")

	// Remove old mcp-cron-oi if it exists
	if cfg.Servers != nil {
		if _, exists := cfg.Servers["mcp-cron-oi"]; exists {
			fmt.Println("Removing old mcp-cron-oi configuration...")
			delete(cfg.Servers, "mcp-cron-oi")
		}
	}

	// Enable task scheduler in the TaskScheduler section
	cfg.TaskScheduler.Enabled = true

	// Set defaults if not already configured
	if cfg.TaskScheduler.Port == 0 {
		cfg.TaskScheduler.Port = constants.TaskSchedulerDefaultPort
	}
	if cfg.TaskScheduler.Host == "" {
		cfg.TaskScheduler.Host = constants.DefaultHostInterface
	}
	if cfg.TaskScheduler.DatabasePath == "" {
		cfg.TaskScheduler.DatabasePath = constants.DefaultDatabasePath
	}

	// Add task-scheduler to servers config using values from TaskScheduler section
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]config.ServerConfig)
	}

	allowAPIKey := true
	cfg.Servers["task-scheduler"] = config.ServerConfig{
		Build: config.BuildConfig{
			Context:    "../mcp-cron-persistent",
			Dockerfile: "Dockerfile",
		},
		Command:      "/app/mcp-cron",
		Args:         []string{"--transport", "sse", "--address", "0.0.0.0", "--port", fmt.Sprintf("%d", cfg.TaskScheduler.Port), "--db-path", cfg.TaskScheduler.DatabasePath},
		Protocol:     "sse",
		HttpPort:     cfg.TaskScheduler.Port, // Use the configured port
		SSEPath:      "/sse",
		User:         "root",
		ReadOnly:     false,
		Privileged:   false,
		CapDrop:      []string{"SYS_ADMIN", "NET_ADMIN"},
		SecurityOpt:  []string{"no-new-privileges:true"},
		Capabilities: []string{"tools", "resources"},
		Env: map[string]string{
			"TZ":                                 "America/New_York",
			"MCP_CRON_SERVER_TRANSPORT":          "sse",
			"MCP_CRON_SERVER_ADDRESS":            "0.0.0.0",
			"MCP_CRON_SERVER_PORT":               fmt.Sprintf("%d", cfg.TaskScheduler.Port),
			"MCP_CRON_DATABASE_PATH":             cfg.TaskScheduler.DatabasePath,
			"MCP_CRON_DATABASE_ENABLED":          "true",
			"MCP_CRON_LOGGING_LEVEL":             cfg.TaskScheduler.LogLevel,
			"MCP_CRON_SCHEDULER_DEFAULT_TIMEOUT": "10m",
			"MCP_CRON_OLLAMA_ENABLED":            "true",
			"MCP_CRON_OLLAMA_BASE_URL":           cfg.TaskScheduler.OllamaURL,
			"MCP_CRON_OLLAMA_DEFAULT_MODEL":      cfg.TaskScheduler.OllamaModel,
			"MCP_PROXY_URL":                      cfg.TaskScheduler.MCPProxyURL,
			"MCP_PROXY_API_KEY":                  cfg.TaskScheduler.MCPProxyAPIKey,
			"OPENROUTER_API_KEY":                 cfg.TaskScheduler.OpenRouterAPIKey,
			"OPENROUTER_MODEL":                   cfg.TaskScheduler.OpenRouterModel,
			"USE_OPENROUTER":                     "true",
			"OPENROUTER_ENABLED":                 "true",
			"MCP_CRON_OPENWEBUI_ENABLED":         strconv.FormatBool(cfg.TaskScheduler.OpenWebUIEnabled),
		},
		Volumes:  append(cfg.TaskScheduler.Volumes, "task-scheduler-data:/data"),
		Networks: []string{"mcp-net"},
		Authentication: &config.ServerAuthConfig{
			Enabled:       true,
			RequiredScope: "mcp:tools",
			OptionalAuth:  false,
			AllowAPIKey:   &allowAPIKey,
		},
	}

	// Set workspace from config
	if cfg.TaskScheduler.Workspace != "" {
		// Replace or add workspace volume
		for i, vol := range cfg.Servers["task-scheduler"].Volumes {
			if strings.Contains(vol, ":/workspace") {
				cfg.Servers["task-scheduler"].Volumes[i] = cfg.TaskScheduler.Workspace + ":/workspace:rw"

				break
			}
		}
	}

	fmt.Printf("Task scheduler configuration added to config (port: %d).\n", cfg.TaskScheduler.Port)

	return config.SaveConfig(configFile, cfg)
}

func disableTaskScheduler(configFile string, cfg *config.ComposeConfig) error {
	fmt.Println("Disabling task scheduler...")

	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	// Stop the container if running
	if err := runtime.StopContainer("mcp-compose-task-scheduler"); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	// Remove from config
	if cfg.Servers != nil {
		delete(cfg.Servers, "task-scheduler")
	}

	fmt.Println("Task scheduler removed from configuration.")

	return config.SaveConfig(configFile, cfg)
}

func runNativeTaskScheduler(cfg *config.ComposeConfig, port int, host, dbPath, workspace, logLevel string, debug bool) error {
	fmt.Printf("Starting native task scheduler on %s:%d...\n", host, port)
	fmt.Printf("Using workspace: %s\n", workspace)
	fmt.Printf("Using database: %s\n", dbPath)

	// Check if we're in the mcp-cron-persistent directory or if it exists as a subdirectory
	mcpCronPaths := []string{
		"./mcp-cron",                                   // If built in current directory
		"../mcp-cron-persistent/mcp-cron",              // If in subdirectory
		"../mcp-cron-persistent/cmd/mcp-cron/mcp-cron", // Alternative path
	}

	var mcpCronPath string
	for _, path := range mcpCronPaths {
		if _, err := os.Stat(path); err == nil {
			mcpCronPath = path

			break
		}
	}

	if mcpCronPath == "" {

		return fmt.Errorf("mcp-cron binary not found. Please build it first:\n" +
			"cd ../mcp-cron-persistent && go build -o mcp-cron ./cmd/mcp-cron")
	}

	// Set up environment from config and parameters
	env := os.Environ()
	env = append(env, fmt.Sprintf("MCP_CRON_SERVER_ADDRESS=%s", host))
	env = append(env, fmt.Sprintf("MCP_CRON_SERVER_PORT=%d", port))
	env = append(env, fmt.Sprintf("MCP_CRON_DATABASE_PATH=%s", dbPath))
	env = append(env, "MCP_CRON_SERVER_TRANSPORT=sse")
	env = append(env, "MCP_CRON_DATABASE_ENABLED=true")

	// Add additional environment from config if available
	if serverConfig, exists := cfg.Servers["task-scheduler"]; exists {
		for key, value := range serverConfig.Env {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	if logLevel != "" {
		env = append(env, fmt.Sprintf("MCP_CRON_LOGGING_LEVEL=%s", logLevel))
	}

	if debug {
		env = append(env, "MCP_CRON_DEBUG=true")
		env = append(env, "MCP_CRON_LOGGING_LEVEL=debug")
	}

	// Create the command
	args := []string{"--transport", "sse", "--address", host, "--port", fmt.Sprintf("%d", port), "--db-path", dbPath}
	if debug {
		args = append(args, "--debug-config")
	}

	cmd := exec.Command(mcpCronPath, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nShutting down task scheduler...")
		cancel()
		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	fmt.Printf("Task scheduler running at http://%s:%d\n", host, port)
	fmt.Printf("Available endpoints:\n")
	fmt.Printf("  Health Check:  http://%s:%d/health\n", host, port)
	fmt.Printf("  SSE Endpoint:  http://%s:%d/sse\n", host, port)

	// Start the command
	if err := cmd.Start(); err != nil {

		return fmt.Errorf("failed to start task scheduler: %w", err)
	}

	// Wait for completion or cancellation
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():

		return nil
	case err := <-done:

		return err
	}
}

func runContainerizedTaskScheduler(_ *config.ComposeConfig, _ string, port int, host, dbPath, workspace, logLevel, mcpProxyURL, mcpProxyAPIKey, ollamaURL, ollamaModel, openrouterAPIKey, openrouterModel, cpus, memory string, healthCheck, debug bool) error {
	fmt.Printf("Starting containerized task scheduler on %s:%d...\n", host, port)

	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	// Build the image with better error handling
	if err := buildTaskSchedulerImageWithRetry(debug); err != nil {

		return fmt.Errorf("failed to build task scheduler image: %w", err)
	}

	// Stop existing container
	_ = runtime.StopContainer("mcp-compose-task-scheduler")

	// Ensure network exists
	networkExists, _ := runtime.NetworkExists("mcp-net")
	if !networkExists {
		if err := runtime.CreateNetwork("mcp-net"); err != nil {

			return fmt.Errorf("failed to create mcp-net network: %w", err)
		}
		fmt.Println("Created mcp-net network for task scheduler.")
	}

	// Prepare environment variables with proper Docker network endpoints
	env := map[string]string{
		"TZ":                                 "America/New_York",
		"MCP_CRON_SERVER_TRANSPORT":          "sse",
		"MCP_CRON_SERVER_ADDRESS":            "0.0.0.0",
		"MCP_CRON_SERVER_PORT":               fmt.Sprintf("%d", port), // Use actual port (8018)
		"MCP_CRON_DATABASE_PATH":             dbPath,
		"MCP_CRON_DATABASE_ENABLED":          "true",
		"MCP_CRON_SCHEDULER_DEFAULT_TIMEOUT": "10m",
		"MCP_CRON_OPENWEBUI_ENABLED":         "false",
		// Default Ollama configuration
		"MCP_CRON_OLLAMA_ENABLED":       "true",
		"MCP_CRON_OLLAMA_BASE_URL":      "http://localhost:11434", // Default, will be overridden
		"MCP_CRON_OLLAMA_DEFAULT_MODEL": "llama3.2",
		// Default OpenRouter configuration (must be enabled)
		"USE_OPENROUTER":     "true",
		"OPENROUTER_ENABLED": "true",
		"OPENROUTER_MODEL":   "anthropic/claude-3.5-sonnet",
		// Docker network endpoints - CRITICAL FIXES
		"MCP_PROXY_URL":           "http://mcp-compose-http-proxy:9876", // Main HTTP proxy
		"MCP_PROXY_TOOLS_ENABLED": "true",
		"MCP_MEMORY_SERVER_URL":   "http://mcp-compose-memory:3001",              // Memory server
		"MCP_TOOLS_BASE_URL":      "http://mcp-compose-memory:3001",              // Alternative env var
		"MCP_TOOLS_ENDPOINT":      "http://mcp-compose-memory:3001/openapi.json", // Direct endpoint
		// Fix hardcoded localhost:3001 in model router
		"MCP_CRON_OPENROUTER_MCP_PROXY_URL": "http://mcp-compose-memory:3001", // Model router gateway
		"MCP_GATEWAY_URL":                   "http://mcp-compose-memory:3001", // Alternative key
		// Additional MCP service endpoints on Docker network
		"MCP_FILESYSTEM_URL":         "http://mcp-compose-filesystem:3000",
		"MCP_OPENROUTER_GATEWAY_URL": "http://mcp-compose-openrouter-gateway:8012",
		"MCP_MEAL_LOG_URL":           "http://mcp-compose-meal-log:8011",
		"MCP_POSTGRES_MCP_URL":       "http://mcp-compose-postgres-mcp:8013",
	}

	// Override with provided values and fix network endpoints
	if logLevel != "" {
		env["MCP_CRON_LOGGING_LEVEL"] = logLevel
	} else {
		env["MCP_CRON_LOGGING_LEVEL"] = "info" // Default
	}

	if debug {
		env["MCP_CRON_DEBUG"] = "true"
		env["MCP_CRON_LOGGING_LEVEL"] = "debug"
		env["MCP_CRON_MODEL_ROUTER_ENABLED"] = "true" // Enable detailed model router logging
	}

	// Fix MCP Proxy URL to use internal Docker network
	if mcpProxyURL != "" {
		// Convert external URLs to Docker network URLs
		if strings.Contains(mcpProxyURL, "192.168.86.201:9876") || strings.Contains(mcpProxyURL, "localhost:9876") {
			env["MCP_PROXY_URL"] = "http://mcp-compose-http-proxy:9876"
			fmt.Printf("Converting proxy URL from %s to Docker network address\n", mcpProxyURL)
		} else {
			env["MCP_PROXY_URL"] = mcpProxyURL
		}
	} else {
		env["MCP_PROXY_URL"] = "http://mcp-compose-http-proxy:9876" // Default to internal network
	}

	if mcpProxyAPIKey != "" {
		env["MCP_PROXY_API_KEY"] = mcpProxyAPIKey
		env["MCP_CRON_OPENROUTER_MCP_PROXY_KEY"] = mcpProxyAPIKey // Ensure both are set
	}

	// Fix Ollama URL format and ensure http:// prefix
	if ollamaURL != "" {
		if !strings.HasPrefix(ollamaURL, "http://") && !strings.HasPrefix(ollamaURL, "https://") {
			env["MCP_CRON_OLLAMA_BASE_URL"] = "http://" + ollamaURL
			fmt.Printf("Added http:// prefix to Ollama URL: http://%s\n", ollamaURL)
		} else {
			env["MCP_CRON_OLLAMA_BASE_URL"] = ollamaURL
		}
	}
	if ollamaModel != "" {
		env["MCP_CRON_OLLAMA_DEFAULT_MODEL"] = ollamaModel
	}

	// OpenRouter configuration with Docker network fixes
	if openrouterAPIKey != "" {
		env["OPENROUTER_API_KEY"] = openrouterAPIKey
		env["MCP_CRON_OPENROUTER_API_KEY"] = openrouterAPIKey // Alternative key
		env["USE_OPENROUTER"] = "true"
		env["OPENROUTER_ENABLED"] = "true"
		// CRITICAL: Override the MCPProxyURL to use Docker network
		env["MCP_CRON_OPENROUTER_MCP_PROXY_URL"] = "http://mcp-compose-memory:3001"
		env["MCP_CRON_OPENROUTER_MCP_PROXY_KEY"] = mcpProxyAPIKey
		fmt.Println("OpenRouter enabled with Docker network proxy address")
	} else {
		fmt.Println("Warning: No OpenRouter API key provided, but OpenRouter is required")
	}
	if openrouterModel != "" {
		env["OPENROUTER_MODEL"] = openrouterModel
		env["MCP_CRON_OPENROUTER_DEFAULT_MODEL"] = openrouterModel // Alternative key
	}

	// Print debug info about network configuration
	if debug {
		fmt.Println("Docker network configuration:")
		fmt.Printf("  MCP Proxy (main): %s\n", env["MCP_PROXY_URL"])
		fmt.Printf("  MCP Memory Server: %s\n", env["MCP_MEMORY_SERVER_URL"])
		fmt.Printf("  Model Router Gateway: %s\n", env["MCP_CRON_OPENROUTER_MCP_PROXY_URL"])
		fmt.Printf("  Ollama: %s\n", env["MCP_CRON_OLLAMA_BASE_URL"])
		fmt.Printf("  OpenRouter Enabled: %s\n", env["OPENROUTER_ENABLED"])
		fmt.Printf("  Filesystem Server: %s\n", env["MCP_FILESYSTEM_URL"])
		fmt.Printf("  OpenRouter Gateway: %s\n", env["MCP_OPENROUTER_GATEWAY_URL"])
	}

	// Container options with correct field names
	opts := &container.ContainerOptions{
		Name:     "mcp-compose-task-scheduler",
		Image:    "mcp-compose-task-scheduler:latest",
		Ports:    []string{fmt.Sprintf("%d:%d", port, port)}, // Map external port to same internal port
		Env:      env,
		Networks: []string{"mcp-net"},
		Volumes: []string{
			"task-scheduler-data:/data",
			fmt.Sprintf("%s:/workspace:rw", workspace),
			"/tmp:/tmp:rw",
		},
		User:        "root",
		CPUs:        cpus,
		Memory:      memory,
		CapDrop:     []string{"ALL"},
		CapAdd:      []string{"SETUID", "SETGID"},
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "task-scheduler",
		},
		RestartPolicy: "unless-stopped",
	}

	// Start container with retry logic
	containerID, err := startContainerWithRetry(runtime, opts, constants.DefaultRetryLimit)
	if err != nil {

		return fmt.Errorf("failed to start task scheduler container: %w", err)
	}

	fmt.Printf("Task scheduler container started with ID: %s\n", containerID[:constants.ContainerIDDisplayLength])
	fmt.Printf("Container using port mapping: %d:%d\n", port, port)

	// Wait for container to be healthy
	if healthCheck {
		fmt.Printf("Waiting for task scheduler to become healthy...\n")
		if err := waitForContainerHealth(runtime, "mcp-compose-task-scheduler", constants.ContainerHealthTimeout); err != nil {
			fmt.Printf("Warning: Health check failed: %v\n", err)
			// Show logs to help debug
			showRecentLogs(runtime, "mcp-compose-task-scheduler")
		} else {
			fmt.Printf("✅ Task scheduler is healthy!\n")
		}
	}

	fmt.Printf("Task scheduler is running at http://%s:%d\n", host, port)
	fmt.Printf("Available endpoints:\n")
	fmt.Printf("  Health Check:  http://%s:%d/health\n", host, port)
	fmt.Printf("  SSE Endpoint:  http://%s:%d/sse\n", host, port)
	fmt.Printf("Network connectivity:\n")
	fmt.Printf("  → MCP Proxy (main): %s\n", env["MCP_PROXY_URL"])
	fmt.Printf("  → Memory Server: %s\n", env["MCP_MEMORY_SERVER_URL"])
	fmt.Printf("  → Model Router: %s\n", env["MCP_CRON_OPENROUTER_MCP_PROXY_URL"])
	fmt.Printf("  → Ollama: %s\n", env["MCP_CRON_OLLAMA_BASE_URL"])
	fmt.Printf("  → Filesystem: %s\n", env["MCP_FILESYSTEM_URL"])
	fmt.Printf("  → OpenRouter Gateway: %s\n", env["MCP_OPENROUTER_GATEWAY_URL"])
	fmt.Printf("\nTo stop the task scheduler: mcp-compose stop task-scheduler\n")


	return nil
}

func buildTaskSchedulerImageWithRetry(debug bool) error {
	for attempt := 1; attempt <= constants.DefaultRetryLimit; attempt++ {
		fmt.Printf("Building task scheduler image (attempt %d/%d)...\n", attempt, constants.DefaultRetryLimit)

		if err := buildTaskSchedulerImage(debug); err == nil {
			fmt.Println("Task scheduler image built successfully.")

			return nil
		} else {
			fmt.Printf("Build attempt %d failed: %v\n", attempt, err)
			if attempt < constants.DefaultRetryLimit {
				fmt.Printf("Retrying in %v...\n", constants.ImageBuildDelay)
				time.Sleep(constants.ImageBuildDelay)
			}
		}
	}


	return fmt.Errorf("failed to build task scheduler image after %d attempts", constants.DefaultRetryLimit)
}

func buildTaskSchedulerImage(debug bool) error {
	dockerfilePath := "dockerfiles/Dockerfile.task-scheduler"
	
	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {

		return fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
	}

	args := []string{"build", "-f", dockerfilePath, "-t", "mcp-compose-task-scheduler:latest", "."}
	if debug {
		args = append(args, "--progress=plain", "--no-cache")
	}

	cmd := exec.Command("docker", args...)
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}


	return cmd.Run()
}

func startContainerWithRetry(runtime container.Runtime, opts *container.ContainerOptions, maxRetries int) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		containerID, err := runtime.StartContainer(opts)
		if err == nil {

			return containerID, nil
		}

		lastErr = err
		fmt.Printf("Container start attempt %d failed: %v\n", attempt, err)

		if attempt < maxRetries {
			fmt.Printf("Retrying in 2 seconds...\n")
			time.Sleep(constants.DefaultRetryDelay)
		}
	}


	return "", fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

func waitForContainerHealth(runtime container.Runtime, containerName string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		status, err := runtime.GetContainerStatus(containerName)
		if err != nil {

			return fmt.Errorf("failed to get container status: %w", err)
		}

		if status == "running" {
			// Give it a moment to start up before considering it healthy
			time.Sleep(constants.DefaultRetryDelay)

			return nil
		}

		if status == "exited" || status == "stopped" {

			return fmt.Errorf("container exited unexpectedly")
		}

		time.Sleep(constants.DefaultRetryDelay)
	}


	return fmt.Errorf("container did not become healthy within %v", timeout)
}

func showRecentLogs(runtime container.Runtime, containerName string) {
	fmt.Printf("Recent logs for %s:\n", containerName)
	if err := runtime.ShowContainerLogs(containerName, false); err != nil {
		fmt.Printf("Could not show logs: %v\n", err)
	}
}
