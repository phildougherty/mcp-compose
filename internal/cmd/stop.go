// internal/cmd/stop.go
package cmd

import (
	"fmt"
	"mcpcompose/internal/compose"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/dashboard"
	"mcpcompose/internal/task_scheduler"

	"github.com/spf13/cobra"
)

func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [SERVER|proxy|dashboard]...",
		Short: "Stop MCP servers, proxy, or dashboard",
		Long: `Stop MCP servers, the proxy server, or the dashboard.

Examples:
  mcp-compose stop server1 server2   # Stop specific servers
  mcp-compose stop proxy             # Stop the HTTP proxy
  mcp-compose stop dashboard         # Stop the dashboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("no servers, proxy, or dashboard specified to stop")
			}

			file, _ := cmd.Flags().GetString("file")

			// Process each argument
			for _, target := range args {
				switch target {
				case "proxy":
					if err := stopProxy(); err != nil {
						return fmt.Errorf("failed to stop proxy: %w", err)
					}
				case "dashboard":
					if err := stopDashboard(file); err != nil {
						return fmt.Errorf("failed to stop dashboard: %w", err)
					}
				case "task-scheduler":
					if err := stopTaskScheduler(file); err != nil {
						return fmt.Errorf("failed to stop task scheduler: %w", err)
					}
				default:
					// Regular server stop - collect all regular servers and stop them together
					regularServers := []string{}
					for _, arg := range args {
						if arg != "proxy" && arg != "dashboard" {
							regularServers = append(regularServers, arg)
						}
					}
					if len(regularServers) > 0 {
						return compose.Stop(file, regularServers)
					}
				}
			}

			return nil
		},
	}

	return cmd
}

func stopProxy() error {
	fmt.Println("Stopping MCP proxy...")

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	proxyContainerName := "mcp-compose-http-proxy"

	if err := runtime.StopContainer(proxyContainerName); err != nil {
		return fmt.Errorf("failed to stop proxy container: %w", err)
	}

	fmt.Println("✅ Proxy stopped successfully.")
	return nil
}

func stopDashboard(configFile string) error {
	fmt.Println("Stopping MCP dashboard...")

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
		return fmt.Errorf("failed to stop dashboard: %w", err)
	}

	fmt.Println("✅ Dashboard stopped successfully.")
	return nil
}

func stopTaskScheduler(configFile string) error {
	fmt.Println("Stopping MCP task scheduler...")
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	taskManager := task_scheduler.NewManager(cfg, runtime)
	taskManager.SetConfigFile(configFile)

	if err := taskManager.Stop(); err != nil {
		return fmt.Errorf("failed to stop task scheduler: %w", err)
	}

	fmt.Println("✅ Task scheduler stopped successfully.")
	return nil
}
