// internal/cmd/proxy.go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"mcpcompose/internal/compose"
	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"
	"mcpcompose/internal/server"

	"github.com/spf13/cobra"
)

func NewProxyCommand() *cobra.Command {
	var port int
	var generateConfig bool
	var clientType string
	var outputDir string
	var apiKey string
	var containerized bool // Keep for containerized proxy, though native is now primary

	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "Run an MCP proxy server for all services",
		Long: `Run a proxy server that exposes all your MCP services through a unified HTTP endpoint.
This proxy uses HTTP/SSE for communication with MCP servers, eliminating the need for docker exec.
Servers must be configured to run in HTTP mode and expose their ports.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			// Load the configuration
			cfg, err := config.LoadConfig(file)
			if err != nil {

				return fmt.Errorf("failed to load config: %w", err)
			}
			projectName := getProjectName(file)

			// If only generating config, do that and exit
			if generateConfig {

				return generateProxyClientConfig(cfg, projectName, port, clientType, outputDir)
			}

			// Run containerized Go proxy (if requested)
			if containerized {

				return startContainerizedGoProxy(cfg, projectName, port, outputDir, apiKey, file)
			}

			// Run native Go proxy (primary mode)

			return startNativeGoProxy(cfg, projectName, port, apiKey, file)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", constants.DefaultProxyPort, "Port to run the proxy server on")
	cmd.Flags().BoolVarP(&generateConfig, "generate-config", "g", false, "Generate client configuration file only")
	cmd.Flags().StringVarP(&clientType, "client", "t", "claude", "Client type (claude, openai, anthropic, all)")
	cmd.Flags().StringVarP(&outputDir, "output", "o", "client-config", "Output directory for client configuration")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for securing the proxy server")
	cmd.Flags().BoolVarP(&containerized, "container", "C", false, "Run proxy server as a container (less common now)")

	return cmd
}

func startContainerizedGoProxy(cfg *config.ComposeConfig, projectName string, port int, outputDir string, apiKey string, configFile string) error {
	fmt.Println("Starting containerized Go MCP proxy (HTTP transport)...")

	cRuntime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	if err := buildGoProxyImage(true); err != nil {

		return fmt.Errorf("failed to build Go HTTP proxy image: %w", err)
	}

	_ = cRuntime.StopContainer("mcp-compose-http-proxy")
	networkExists, _ := cRuntime.NetworkExists("mcp-net")
	if !networkExists {
		if err := cRuntime.CreateNetwork("mcp-net"); err != nil {

			return fmt.Errorf("failed to create mcp-net network: %w", err)
		}
		fmt.Println("Created mcp-net network for proxy.")
	}

	absConfigFile, err := filepath.Abs(configFile)
	if err != nil {

		return fmt.Errorf("failed to get absolute path for config file: %w", err)
	}

	env := map[string]string{
		"MCP_PROXY_PORT":    fmt.Sprintf("%d", port),
		"MCP_PROJECT_NAME":  projectName,
		"MCP_CONFIG_FILE":   "/app/mcp-compose.yaml",
		"MCP_PROTOCOL_MODE": "enhanced",
	}

	if apiKey != "" {
		env["MCP_API_KEY"] = apiKey
	}

	opts := &container.ContainerOptions{
		Name:     "mcp-compose-http-proxy",
		Image:    "mcp-compose-go-http-proxy:latest",
		Ports:    []string{fmt.Sprintf("%d:%d", port, port)},
		Env:      env,
		Networks: []string{"mcp-net"},
		Volumes: []string{
			fmt.Sprintf("%s:/app/mcp-compose.yaml:ro", absConfigFile),
			"/var/run/docker.sock:/var/run/docker.sock:ro",
		},

		// ADD SECURITY CONFIGURATION FOR PROXY CONTAINER:
		User: "root", // Proxy needs root access for Docker socket
		Security: container.SecurityConfig{
			AllowDockerSocket:  true,  // ALLOW Docker socket access for proxy
			AllowPrivilegedOps: false, // But don't allow other privileged operations
			TrustedImage:       true,  // Mark as trusted system container
		},

		// Add resource limits
		CPUs:   "1.0",
		Memory: "512m",

		// Security hardening
		CapDrop:     []string{"ALL"},
		CapAdd:      []string{"SETUID", "SETGID"}, // Minimal capabilities for container management
		SecurityOpt: []string{"no-new-privileges:true"},

		// Labels to identify as system container
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "proxy",
		},
	}

	containerID, err := cRuntime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start HTTP proxy container: %w", err)
	}

	fmt.Printf("Go HTTP proxy container started with ID: %s\n", containerID[:12])
	fmt.Printf("MCP Proxy (HTTP mode) is running at http://localhost:%d\n", port)

	if apiKey != "" {
		fmt.Printf("API key authentication is enabled. Use 'Bearer %s' in Authorization header.\n", apiKey)
	}

	// Enhanced endpoint information
	fmt.Println("\nAvailable endpoints:")
	fmt.Printf("  Dashboard:     http://localhost:%d/\n", port)
	fmt.Printf("  OpenAPI Spec:  http://localhost:%d/openapi.json\n", port)
	fmt.Printf("  Server Status: http://localhost:%d/api/servers\n", port)
	fmt.Printf("  Discovery:     http://localhost:%d/api/discovery\n", port)
	fmt.Printf("  Subscriptions: http://localhost:%d/api/subscriptions\n", port)
	fmt.Printf("  Notifications: http://localhost:%d/api/notifications\n", port)

	if err := generateProxyClientConfig(cfg, projectName, port, "claude", outputDir); err != nil {
		fmt.Printf("Warning: Failed to generate client config: %v\n", err)
	} else {
		fmt.Printf("Client configuration generated in %s/\n", outputDir)
	}

	fmt.Println("To stop the proxy: mcp-compose stop proxy")

	return nil
}

func startNativeGoProxy(cfg *config.ComposeConfig, _ string, port int, apiKey string, configFile string) error {
	fmt.Printf("Starting native Go MCP proxy (HTTP transport) on port %d...\n", port)

	// Detect container runtime
	cRuntime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime (for server management): %w", err)
	}

	// Create server manager
	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {

		return fmt.Errorf("failed to create server manager: %w", err)
	}

	// Try to create composer for full protocol integration (optional)
	var composer *compose.Composer
	if composerInstance, err := compose.NewComposer(configFile); err != nil {
		fmt.Printf("Warning: Failed to create composer (advanced features disabled): %v\n", err)
		composer = nil
	} else {
		composer = composerInstance
	}

	// Create the proxy handler
	handler := server.NewProxyHandler(mgr, configFile, apiKey)

	// Set up cleanup on shutdown
	if composer != nil {
		defer func() {
			if err := composer.Shutdown(); err != nil {
				fmt.Printf("Warning: Composer shutdown error: %v\n", err)
			}
		}()
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\nShutting down proxy...")

		// Shutdown in proper order
		if err := handler.Shutdown(); err != nil {
			fmt.Printf("Warning: ProxyHandler shutdown error: %v\n", err)
		}

		if err := mgr.Shutdown(); err != nil {
			fmt.Printf("Warning: Manager shutdown error: %v\n", err)
		}

		cancel()
		os.Exit(0)
	}()

	// Get configurable timeouts or use defaults
	readTimeout := constants.FileOperationTimeout  // Default for large file operations
	writeTimeout := constants.FileOperationTimeout // Default for long-running tools
	idleTimeout := constants.ConnectionKeepAlive  // Default for connection keepalive

	if len(cfg.Connections) > 0 {
		for _, conn := range cfg.Connections {
			readTimeout = conn.Timeouts.GetReadTimeout()
			writeTimeout = conn.Timeouts.GetWriteTimeout()
			idleTimeout = conn.Timeouts.GetIdleTimeout()

			break // Use first connection's timeout config
		}
	}

	// Create HTTP server with configurable timeouts
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	fmt.Printf("MCP Proxy (HTTP mode) is running at http://localhost:%d\n", port)
	if apiKey != "" {
		fmt.Printf("API key authentication is enabled. Use 'Bearer %s' in Authorization header.\n", apiKey)
	}

	// Print enhanced endpoints available
	fmt.Println("\nAvailable endpoints:")
	fmt.Printf("  Dashboard:     http://localhost:%d/\n", port)
	fmt.Printf("  OpenAPI Spec:  http://localhost:%d/openapi.json\n", port)
	fmt.Printf("  Server Status: http://localhost:%d/api/servers\n", port)
	fmt.Printf("  Discovery:     http://localhost:%d/api/discovery\n", port)

	// Print server-specific endpoints
	for serverName := range cfg.Servers {
		caser := cases.Title(language.English)
		fmt.Printf("  %s Server:    http://localhost:%d/%s\n",
			caser.String(serverName), port, serverName)
		fmt.Printf("  %s OpenAPI:   http://localhost:%d/%s/openapi.json\n",
			caser.String(serverName), port, serverName)
	}

	// Start HTTP server in goroutine
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			cancel()
		}
	}()

	// Wait for cancellation
	<-ctx.Done()

	// Graceful shutdown with configurable timeout
	shutdownTimeout := constants.DefaultShutdownTimeout // Default fallback
	if len(cfg.Connections) > 0 {
		for _, conn := range cfg.Connections {
			shutdownTimeout = conn.Timeouts.GetShutdownTimeout()

			break
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()


	return httpServer.Shutdown(shutdownCtx)
}

func getProjectName(configFile string) string {
	projectName := filepath.Base(strings.TrimSuffix(configFile, filepath.Ext(configFile)))
	if projectName == "." || projectName == "" {
		if cwd, err := os.Getwd(); err == nil {
			projectName = filepath.Base(cwd)
		} else {
			projectName = "mcp-compose"
		}
	}

	return projectName
}

func buildGoProxyImage(httpProxy bool) error {
	imageName := "mcp-compose-go-proxy:latest"
	dockerfileName := "Dockerfile.mcp-proxy"

	if httpProxy {
		imageName = "mcp-compose-go-http-proxy:latest"
	}

	fmt.Printf("Building Go proxy image (%s)...\n", imageName)

	// Enhanced Dockerfile with protocol support
	dockerfileContent := `FROM golang:1.21-alpine AS builder
WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build with enhanced protocol support
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -a -installsuffix cgo \
    -o mcp-compose-executable \
    cmd/mcp-compose/main.go

FROM alpine:latest

# Add essential tools for proxy operation
RUN apk --no-cache add \
    ca-certificates \
    docker-cli \
    curl \
    wget \
    jq \
    tzdata

# Set timezone
ENV TZ=UTC

WORKDIR /app

COPY --from=builder /build/mcp-compose-executable .

# Create directories for various protocol features
RUN mkdir -p /app/data /app/logs /app/cache /app/temp

# Set proxy-specific environment variables
ENV MCP_PROXY_PORT=9876
ENV MCP_PROTOCOL_MODE=enhanced
ENV MCP_ENABLE_NOTIFICATIONS=true
ENV MCP_ENABLE_SUBSCRIPTIONS=true
ENV MCP_ENABLE_PROGRESS=true
ENV MCP_ENABLE_SAMPLING=true

EXPOSE 9876

CMD ["./mcp-compose-executable", "proxy", "--file", "/app/mcp-compose.yaml"]
`

	if err := os.WriteFile(dockerfileName, []byte(dockerfileContent), constants.DefaultFileMode); err != nil {

		return fmt.Errorf("failed to write Dockerfile %s: %w", dockerfileName, err)
	}
	defer func() { _ = os.Remove(dockerfileName) }()

	cmd := exec.Command("docker", "build", "-f", dockerfileName, "-t", imageName, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {

		return fmt.Errorf("docker build for %s failed: %w", imageName, err)
	}

	fmt.Printf("Go proxy image %s built successfully.\n", imageName)

	return nil
}

// generateProxyClientConfig remains useful for generating client-side import files
// It should now generate httpEndpoint based on the proxy's address.
func generateProxyClientConfig(cfg *config.ComposeConfig, _ string, proxyPort int, clientType string, outputDir string) error {
	if err := os.MkdirAll(outputDir, constants.DefaultDirMode); err != nil {

		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Proxy URL for all client configs
	proxyBaseURL := fmt.Sprintf("http://localhost:%d", proxyPort)

	if strings.ToLower(clientType) == "all" {
		if err := generateProxyClaudeConfig(cfg, proxyBaseURL, outputDir); err != nil {

			return fmt.Errorf("failed to generate Claude config: %w", err)
		}
		// Add other client config generations here if needed, adapting them for HTTP endpoints
		fmt.Println("Successfully generated all client configurations pointing to the HTTP proxy.")

		return nil
	}

	switch strings.ToLower(clientType) {
	case "claude":

		return generateProxyClaudeConfig(cfg, proxyBaseURL, outputDir)
	// Add cases for 'openai', 'anthropic' if you have specific formats for them
	// that point to an HTTP proxy.
	default:

		return fmt.Errorf("unknown client type: %s", clientType)
	}
}

func generateProxyClaudeConfig(cfg *config.ComposeConfig, proxyBaseURL string, outputDir string) error {
	serversList := make([]map[string]interface{}, 0, len(cfg.Servers))

	for name, serverCfg := range cfg.Servers {
		serverConfig := map[string]interface{}{
			"name":         name,
			"httpEndpoint": fmt.Sprintf("%s/%s", proxyBaseURL, name),
			"capabilities": serverCfg.Capabilities,
			"description":  fmt.Sprintf("MCP %s server (via enhanced proxy)", name),
		}

		// Add enhanced features if available
		enhancedFeatures := make(map[string]interface{})

		// Check for progress support
		if isProgressSupported(serverCfg) {
			enhancedFeatures["progress"] = true
		}

		// Check for subscription support
		if isSubscriptionSupported(serverCfg) {
			enhancedFeatures["subscriptions"] = true
		}

		// Check for sampling support
		if isSamplingSupported(serverCfg) {
			enhancedFeatures["sampling"] = true
		}

		if len(enhancedFeatures) > 0 {
			serverConfig["enhanced"] = enhancedFeatures
		}

		serversList = append(serversList, serverConfig)
	}

	configObject := map[string]interface{}{
		"servers": serversList,
		"proxy": map[string]interface{}{
			"version":  "enhanced",
			"features": []string{"progress", "subscriptions", "notifications", "sampling"},
			"baseUrl":  proxyBaseURL,
		},
	}

	configPath := filepath.Join(outputDir, "claude-desktop-servers.json")
	configData, err := json.MarshalIndent(configObject, "", "  ")
	if err != nil {

		return fmt.Errorf("failed to marshal Claude Desktop config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, constants.DefaultFileMode); err != nil {

		return fmt.Errorf("failed to write Claude Desktop config file: %w", err)
	}

	// Also generate a README with usage instructions
	readmePath := filepath.Join(outputDir, "README.md")
	readmeContent := fmt.Sprintf(`# MCP Proxy Client Configuration

This configuration connects Claude Desktop to your MCP servers via the enhanced HTTP proxy.

## Features Enabled

- **HTTP Transport**: All servers accessible via HTTP endpoints
- **Progress Tracking**: Real-time progress updates for long-running operations
- **Subscriptions**: Automatic notifications when resources change
- **Sampling**: LLM sampling capabilities for compatible servers
- **Direct Tool Access**: FastAPI-style direct tool invocation

## Proxy Endpoints

- Dashboard: %s/
- OpenAPI Spec: %s/openapi.json
- Server Status: %s/api/servers

## Usage

1. Copy the contents of claude-desktop-servers.json to your Claude Desktop configuration
2. Ensure the MCP proxy is running on %s
3. Restart Claude Desktop to apply the configuration

## Authentication

If API key authentication is enabled, you'll need to configure the Authorization header.
`, proxyBaseURL, proxyBaseURL, proxyBaseURL, proxyBaseURL)

	if err := os.WriteFile(readmePath, []byte(readmeContent), constants.DefaultFileMode); err != nil {
		fmt.Printf("Warning: Failed to write README: %v\n", err)
	}


	return nil
}

// Helper functions to check server capabilities
func isProgressSupported(serverCfg config.ServerConfig) bool {
	// Check if server supports long-running operations
	for _, cap := range serverCfg.Capabilities {
		if cap == "tools" || cap == "resources" {

			return true
		}
	}

	return false
}

func isSubscriptionSupported(serverCfg config.ServerConfig) bool {
	// Check if server supports resource subscriptions
	for _, cap := range serverCfg.Capabilities {
		if cap == "resources" {

			return true
		}
	}

	return false
}

func isSamplingSupported(serverCfg config.ServerConfig) bool {
	// Check if server supports sampling
	for _, cap := range serverCfg.Capabilities {
		if cap == "sampling" {

			return true
		}
	}

	return false
}
