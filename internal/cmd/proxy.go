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
	"time"

	"mcpcompose/internal/config"
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

	cmd.Flags().IntVarP(&port, "port", "p", 9876, "Port to run the proxy server on")
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

	if err := buildGoProxyImage(true); err != nil { // Pass true for HTTP proxy variant
		return fmt.Errorf("failed to build Go HTTP proxy image: %w", err)
	}

	_ = cRuntime.StopContainer("mcp-compose-http-proxy") // Stop existing first

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
		"MCP_PROXY_PORT":   fmt.Sprintf("%d", port),
		"MCP_PROJECT_NAME": projectName,
		"MCP_CONFIG_FILE":  "/app/mcp-compose.yaml", // Path inside the container
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

	if err := generateProxyClientConfig(cfg, projectName, port, "claude", outputDir); err != nil {
		fmt.Printf("Warning: Failed to generate client config: %v\n", err)
	} else {
		fmt.Printf("Client configuration generated in %s/\n", outputDir)
	}
	fmt.Println("To stop the proxy: docker stop mcp-compose-http-proxy")
	return nil
}

func startNativeGoProxy(cfg *config.ComposeConfig, _ string, port int, apiKey string, configFile string) error {
	fmt.Printf("Starting native Go MCP proxy (HTTP transport) on port %d...\n", port)

	// Detect container runtime. Although proxy runs natively, it needs to know if servers are in containers.
	cRuntime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime (for server management): %w", err)
	}

	mgr, err := server.NewManager(cfg, cRuntime)
	if err != nil {
		return fmt.Errorf("failed to create server manager: %w", err)
	}
	// The proxy handler is now designed for HTTP transport
	handler := server.NewProxyHandler(mgr, configFile, apiKey)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("\nShutting down proxy...")
		if err := handler.Shutdown(); err != nil { // Assuming ProxyHandler now has Shutdown
			fmt.Printf("Warning: ProxyHandler shutdown error: %v\n", err)
		}
		if err := mgr.Shutdown(); err != nil {
			fmt.Printf("Warning: Manager shutdown error: %v\n", err)
		}
		cancel()
		os.Exit(0)
	}()

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}

	fmt.Printf("MCP Proxy (HTTP mode) is running at http://localhost:%d\n", port)
	if apiKey != "" {
		fmt.Printf("API key authentication is enabled. Use 'Bearer %s' in Authorization header.\n", apiKey)
	}

	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
			cancel() // Ensure main context is canceled on server error
		}
	}()

	<-ctx.Done() // Wait for cancellation (e.g., from signal handler or server error)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return httpServer.Shutdown(shutdownCtx)
}

func buildGoProxyImage(httpProxy bool) error { // httpProxy flag might be redundant if content is same
	imageName := "mcp-compose-go-proxy:latest" // Default base name
	dockerfileName := "Dockerfile.mcp-proxy"   // Use a consistent naming scheme
	if httpProxy {
		imageName = "mcp-compose-go-http-proxy:latest"
		// You can use the same Dockerfile if the binary inside handles both modes,
		// or a different one if build steps are different.
		// For now, let's assume the same Dockerfile content is fine, just different image tag.
	}
	fmt.Printf("Building Go proxy image (%s)...\n", imageName)

	// This Dockerfile is for the PROXY CONTAINER itself.
	// If the proxy needs to interact with the host's Docker daemon to
	// get status of OTHER MCP server containers, it needs docker-cli
	// and the docker.sock mounted.
	dockerfileContent := `FROM golang:1.21-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# This builds the main mcp-compose binary which includes the proxy subcommand
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o mcp-compose-executable cmd/mcp-compose/main.go

FROM alpine:latest
# Add docker-cli so the proxy container can talk to the host's Docker daemon via the mounted socket
RUN apk --no-cache add ca-certificates docker-cli 
WORKDIR /app
COPY --from=builder /build/mcp-compose-executable .
ENV MCP_PROXY_PORT=9876
# MCP_CONFIG_FILE and MCP_API_KEY will be passed via 'docker run -e' by mcp-compose
EXPOSE 9876
CMD ["./mcp-compose-executable", "proxy", "--file", "/app/mcp-compose.yaml"]
`
	// The CMD above implies that when this proxy container runs, the mcp-compose.yaml
	// will be at /app/mcp-compose.yaml (mounted by startContainerizedGoProxy)
	// and the api_key (if any) will be passed via MCP_API_KEY env var.
	// The proxy logic (startNativeGoProxy effectively) must read these.

	if err := os.WriteFile(dockerfileName, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile %s: %w", dockerfileName, err)
	}
	defer os.Remove(dockerfileName) // Clean up the temp Dockerfile

	cmd := exec.Command("docker", "build", "-f", dockerfileName, "-t", imageName, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker build for %s failed: %w", imageName, err)
	}

	fmt.Printf("Go proxy image %s built successfully.\n", imageName)
	return nil
}

// Helper functions (mostly unchanged)
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

// generateProxyClientConfig remains useful for generating client-side import files
// It should now generate httpEndpoint based on the proxy's address.
func generateProxyClientConfig(cfg *config.ComposeConfig, _ string, proxyPort int, clientType string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
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
		// For HTTP proxy, the httpEndpoint points to the proxy, which then routes to the server
		serverConfig := map[string]interface{}{
			"name":         name,
			"httpEndpoint": fmt.Sprintf("%s/%s", proxyBaseURL, name), // e.g., http://localhost:9876/filesystem
			"capabilities": serverCfg.Capabilities,
			"description":  fmt.Sprintf("MCP %s server (via proxy)", name),
		}
		serversList = append(serversList, serverConfig)
	}

	configObject := map[string]interface{}{
		"servers": serversList,
	}

	configPath := filepath.Join(outputDir, "claude-desktop-servers.json")
	configData, err := json.MarshalIndent(configObject, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal Claude Desktop config: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write Claude Desktop config file: %w", err)
	}
	return nil
}
