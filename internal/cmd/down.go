// internal/cmd/down.go
package cmd

import (
	"fmt"
	"mcpcompose/internal/compose"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/dashboard"

	"github.com/spf13/cobra"
)

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [SERVER|proxy|dashboard]...",
		Short: "Stop and remove MCP servers, proxy, or dashboard",
		Long: `Stop and remove MCP servers, the proxy server, or the dashboard.

Examples:
  mcp-compose down                    # Stop and remove all servers
  mcp-compose down server1 server2   # Stop and remove specific servers
  mcp-compose down proxy             # Stop and remove the HTTP proxy
  mcp-compose down dashboard         # Stop and remove the dashboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			// If no args provided, stop all servers
			if len(args) == 0 {
				return compose.Down(file, []string{})
			}

			// Process each argument
			regularServers := []string{}
			for _, target := range args {
				switch target {
				case "proxy":
					if err := downProxy(); err != nil {
						return fmt.Errorf("failed to stop/remove proxy: %w", err)
					}
				case "dashboard":
					if err := downDashboard(file); err != nil {
						return fmt.Errorf("failed to stop/remove dashboard: %w", err)
					}
				default:
					// Collect regular servers
					regularServers = append(regularServers, target)
				}
			}

			// Handle regular servers if any
			if len(regularServers) > 0 {
				return compose.Down(file, regularServers)
			}

			return nil
		},
	}

	return cmd
}

func downProxy() error {
	fmt.Println("Stopping and removing MCP proxy...")

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	proxyContainerName := "mcp-compose-http-proxy"

	if err := runtime.StopContainer(proxyContainerName); err != nil {
		return fmt.Errorf("failed to stop/remove proxy container: %w", err)
	}

	fmt.Println("✅ Proxy stopped and removed successfully.")
	return nil
}

func downDashboard(configFile string) error {
	fmt.Println("Stopping and removing MCP dashboard...")

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	dashManager := dashboard.NewManager(cfg, runtime)
	dashManager.SetConfigFile(configFile)

	if err := dashManager.Stop(); err != nil {
		return fmt.Errorf("failed to stop/remove dashboard: %w", err)
	}

	fmt.Println("✅ Dashboard stopped and removed successfully.")
	return nil
}
