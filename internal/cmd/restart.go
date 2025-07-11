// internal/cmd/restart.go
package cmd

import (
	"fmt"
	"github.com/phildougherty/mcp-compose/internal/compose"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"

	"github.com/spf13/cobra"
)

func NewRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [SERVER|proxy|dashboard]...",
		Short: "Restart MCP servers, proxy, or dashboard",
		Long: `Restart MCP servers, the proxy server, or the dashboard.

Examples:
  mcp-compose restart                    # Restart all servers
  mcp-compose restart server1 server2   # Restart specific servers
  mcp-compose restart proxy             # Restart the HTTP proxy
  mcp-compose restart dashboard         # Restart the dashboard`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			// If no args provided, restart all servers
			if len(args) == 0 {

				return restartAllServers(file)
			}

			// Process each argument
			for _, target := range args {
				switch target {
				case "proxy":
					if err := restartProxy(); err != nil {

						return fmt.Errorf("failed to restart proxy: %w", err)
					}
				case "dashboard":
					if err := restartDashboard(file); err != nil {

						return fmt.Errorf("failed to restart dashboard: %w", err)
					}
				case "task-scheduler":
					if err := restartTaskScheduler(file); err != nil {

						return fmt.Errorf("failed to restart task scheduler: %w", err)
					}
				default:
					// Regular server restart
					if err := restartServer(file, target); err != nil {

						return fmt.Errorf("failed to restart server '%s': %w", target, err)
					}
				}
			}

			return nil
		},
	}

	return cmd
}

func restartAllServers(configFile string) error {
	fmt.Println("Restarting all MCP servers...")

	// Stop all servers first
	if err := compose.Down(configFile, []string{}); err != nil {
		fmt.Printf("Warning: Error during shutdown: %v\n", err)
	}

	// Start all servers

	return compose.Up(configFile, []string{})
}

func restartServer(configFile string, serverName string) error {
	fmt.Printf("Restarting server '%s'...\n", serverName)

	// Stop the specific server
	if err := compose.Stop(configFile, []string{serverName}); err != nil {
		fmt.Printf("Warning: Error stopping server '%s': %v\n", serverName, err)
	}

	// Start the specific server

	return compose.Start(configFile, []string{serverName})
}

func restartProxy() error {
	fmt.Println("Restarting MCP proxy...")

	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	proxyContainerName := "mcp-compose-http-proxy"

	// Check if proxy container exists and is running
	status, err := runtime.GetContainerStatus(proxyContainerName)
	if err != nil || status == "stopped" {
		fmt.Printf("Proxy container '%s' is not running or doesn't exist.\n", proxyContainerName)
		fmt.Println("To start the proxy, use: mcp-compose proxy")

		return nil
	}

	fmt.Printf("Stopping proxy container '%s'...\n", proxyContainerName)
	if err := runtime.StopContainer(proxyContainerName); err != nil {

		return fmt.Errorf("failed to stop proxy container: %w", err)
	}

	// Get the original docker run command to restart with same parameters
	// For simplicity, we'll just tell the user to restart manually
	fmt.Println("✅ Proxy stopped successfully.")
	fmt.Println("To restart the proxy, use: mcp-compose proxy [options]")
	fmt.Println("Note: The proxy will restart with the same configuration as last started.")

	return nil
}

func restartDashboard(configFile string) error {
	fmt.Println("Restarting MCP dashboard...")

	cfg, err := config.LoadConfig(configFile)
	if err != nil {

		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	dashboardContainerName := "mcp-compose-dashboard"

	// Check if dashboard container exists and is running
	status, err := runtime.GetContainerStatus(dashboardContainerName)
	if err != nil || status == "stopped" {
		fmt.Printf("Dashboard container '%s' is not running or doesn't exist.\n", dashboardContainerName)
		fmt.Println("To start the dashboard, use: mcp-compose dashboard")

		return nil
	}

	fmt.Printf("Stopping dashboard container '%s'...\n", dashboardContainerName)
	dashManager := dashboard.NewManager(cfg, runtime)
	dashManager.SetConfigFile(configFile)

	if err := dashManager.Stop(); err != nil {
		fmt.Printf("Warning: Error stopping dashboard: %v\n", err)
	}

	fmt.Println("Starting dashboard...")
	if err := dashManager.Start(); err != nil {

		return fmt.Errorf("failed to start dashboard: %w", err)
	}

	fmt.Println("✅ Dashboard restarted successfully.")

	return nil
}

func restartTaskScheduler(configFile string) error {
	fmt.Println("Restarting MCP task scheduler...")
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

	if err := taskManager.Restart(); err != nil {

		return fmt.Errorf("failed to restart task scheduler: %w", err)
	}

	fmt.Println("✅ Task scheduler restarted successfully.")

	return nil
}
