package dashboard

import (
	"fmt"
	"log"
	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"
	"mcpcompose/internal/logging"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

type Manager struct {
	config          *config.ComposeConfig
	runtime         container.Runtime
	logger          *logging.Logger
	configFile      string
	activityStorage *ActivityStorage
}

func NewManager(cfg *config.ComposeConfig, runtime container.Runtime) *Manager {
	m := &Manager{
		config:  cfg,
		runtime: runtime,
		logger:  logging.NewLogger(cfg.Logging.Level),
	}

	// Initialize activity storage if PostgreSQL URL is provided
	if cfg.Dashboard.PostgresURL != "" {
		activityStorage, err := NewActivityStorage(cfg.Dashboard.PostgresURL)
		if err != nil {
			// Use Info instead of Warn if Warn doesn't exist
			m.logger.Info("Failed to initialize activity storage: %v", err)
			// Continue without activity storage
		} else {
			m.activityStorage = activityStorage
		}
	}


	return m
}

func (m *Manager) SetConfigFile(configFile string) {
	m.configFile = configFile
}

func (m *Manager) Start() error {
	if !m.config.Dashboard.Enabled {

		return fmt.Errorf("dashboard is disabled in configuration")
	}

	// Check if dashboard container is already running
	status, err := m.runtime.GetContainerStatus("mcp-compose-dashboard")
	if err == nil && status == "running" {
		m.logger.Info("Dashboard container is already running")

		return nil
	}

	// Build dashboard image
	if err := m.buildDashboardImage(); err != nil {

		return fmt.Errorf("failed to build dashboard image: %w", err)
	}

	// Start dashboard container

	return m.startDashboardContainer()
}

func (m *Manager) Stop() error {
	// Close activity storage if initialized
	if m.activityStorage != nil {
		if err := m.activityStorage.Close(); err != nil {
			// Use Info instead of Warn if Warn doesn't exist
			m.logger.Info("Error closing activity storage: %v", err)
		}
	}

	err := m.runtime.StopContainer("mcp-compose-dashboard")
	if err != nil {

		return fmt.Errorf("failed to stop dashboard container: %w", err)
	}
	m.logger.Info("Dashboard container stopped")

	return nil
}

func (m *Manager) buildDashboardImage() error {
	m.logger.Info("Building dashboard Docker image...")

	// Enhanced Dockerfile with PostgreSQL dependencies
	dockerfileContent := `FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build the main mcp-compose binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o mcp-compose cmd/mcp-compose/main.go

FROM alpine:latest
# Install required packages including PostgreSQL client
RUN apk --no-cache add ca-certificates docker-cli curl postgresql-client

# Create non-root user
RUN adduser -D -u 1000 dashboard
WORKDIR /app
COPY --from=builder /build/mcp-compose .

# Change ownership to dashboard user
RUN chown dashboard:dashboard /app/mcp-compose && chmod +x /app/mcp-compose

EXPOSE 3001

# Create a startup script with health checks
RUN echo '#!/bin/sh' > /app/start.sh && \
    echo 'echo "Dashboard container starting..."' >> /app/start.sh && \
    echo 'echo "Environment variables:"' >> /app/start.sh && \
    echo 'echo "  MCP_PROXY_URL: $MCP_PROXY_URL"' >> /app/start.sh && \
    echo 'echo "  MCP_API_KEY: $MCP_API_KEY"' >> /app/start.sh && \
    echo 'echo "  MCP_DASHBOARD_HOST: $MCP_DASHBOARD_HOST"' >> /app/start.sh && \
    echo 'echo "  POSTGRES_URL: ${POSTGRES_URL:+configured}"' >> /app/start.sh && \
    echo 'echo "Starting dashboard server on 0.0.0.0:3001..."' >> /app/start.sh && \
    echo 'exec ./mcp-compose dashboard --native --file /app/mcp-compose.yaml --port 3001 --host 0.0.0.0' >> /app/start.sh && \
    chmod +x /app/start.sh && \
    chown dashboard:dashboard /app/start.sh

# Switch to non-root user
USER dashboard
CMD ["/app/start.sh"]
`

	dockerfilePath := "Dockerfile.dashboard"
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), constants.DefaultFileMode); err != nil {

		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}
	defer func() {
		if err := os.Remove(dockerfilePath); err != nil {
			log.Printf("Warning: failed to remove dockerfile: %v", err)
		}
	}()

	// Build the image
	cmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", "mcp-compose-dashboard:latest", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {

		return fmt.Errorf("docker build failed: %w", err)
	}

	m.logger.Info("Dashboard image built successfully")

	return nil
}

func (m *Manager) startDashboardContainer() error {
	// Ensure network exists
	networkExists, _ := m.runtime.NetworkExists("mcp-net")
	if !networkExists {
		if err := m.runtime.CreateNetwork("mcp-net"); err != nil {

			return fmt.Errorf("failed to create network: %w", err)
		}
	}

	// Get absolute path to config file
	var configPath string
	if m.configFile != "" {
		absPath, err := filepath.Abs(m.configFile)
		if err != nil {

			return fmt.Errorf("failed to get absolute path for config file: %w", err)
		}
		configPath = absPath
	} else {
		// Default config file
		cwd, err := os.Getwd()
		if err != nil {

			return fmt.Errorf("failed to get current directory: %w", err)
		}
		configPath = filepath.Join(cwd, "mcp-compose.yaml")
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {

		return fmt.Errorf("config file not found: %s", configPath)
	}

	// Determine host port (what the user will access)
	hostPort := m.config.Dashboard.Port
	if hostPort == 0 {
		hostPort = 3001
	}

	// Container always listens on port 3001 internally
	containerPort := 3001

	// Prepare environment variables for container
	env := map[string]string{
		"MCP_DASHBOARD_HOST":          "0.0.0.0",                            // Must bind to all interfaces in container
		"MCP_PROXY_URL":               "http://mcp-compose-http-proxy:9876", // Container network URL
		"MCP_API_KEY":                 m.config.ProxyAuth.APIKey,
		"MCP_DASHBOARD_THEME":         m.config.Dashboard.Theme,
		"MCP_DASHBOARD_LOG_STREAMING": strconv.FormatBool(m.config.Dashboard.LogStreaming),
		"MCP_DASHBOARD_CONFIG_EDITOR": strconv.FormatBool(m.config.Dashboard.ConfigEditor),
		"MCP_DASHBOARD_METRICS":       strconv.FormatBool(m.config.Dashboard.Metrics),
		"POSTGRES_URL":                m.config.Dashboard.PostgresURL,
	}

	// Prepare volumes - mount config file and docker socket
	volumes := []string{
		"/var/run/docker.sock:/var/run/docker.sock:ro",         // For Docker API access
		fmt.Sprintf("%s:/app/mcp-compose.yaml:ro", configPath), // Mount config file
	}

	opts := &container.ContainerOptions{
		Name:     "mcp-compose-dashboard",
		Image:    "mcp-compose-dashboard:latest",
		Env:      env,
		Ports:    []string{fmt.Sprintf("%d:%d", hostPort, containerPort)}, // hostPort:3001
		Networks: []string{"mcp-net"},
		Volumes:  volumes,
		// Security configuration for dashboard:
		User: "1000:1000", // Run as non-root user
		Security: container.SecurityConfig{
			AllowDockerSocket:  true,  // Dashboard needs Docker socket access for monitoring
			TrustedImage:       true,  // Mark as trusted system container
			AllowPrivilegedOps: false, // No privileged operations needed
		},
		// Resource limits
		CPUs:   "0.5",
		Memory: "512m",
		// Security hardening
		CapDrop:     []string{"ALL"},
		CapAdd:      []string{"SETUID", "SETGID"}, // Minimal capabilities for container operations
		SecurityOpt: []string{"no-new-privileges:true"},
		ReadOnly:    false, // Dashboard may need to write temp files
		// System container labels
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "dashboard",
		},
		// Restart policy
		RestartPolicy: "unless-stopped",
	}

	containerID, err := m.runtime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start dashboard container: %w", err)
	}

	m.logger.Info("Dashboard container started with ID: %s", containerID[:12])
	m.logger.Info("Dashboard available at http://localhost:%d", hostPort)
	m.logger.Info("Config file mounted from: %s", configPath)
	m.logger.Info("Container listening on port %d, mapped to host port %d", containerPort, hostPort)

	// Log activity storage status
	if m.config.Dashboard.PostgresURL != "" {
		m.logger.Info("Activity storage enabled with PostgreSQL")
	} else {
		m.logger.Info("Activity storage disabled - no PostgreSQL URL configured")
	}


	return nil
}

// GetActivityStorage returns the activity storage instance (for use by other components)
func (m *Manager) GetActivityStorage() *ActivityStorage {

	return m.activityStorage
}
