// internal/cmd/stop.go
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

func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [SERVER...]",
		Short: "Stop MCP user services",
		Long: `Stop MCP user services defined in mcp-compose.yaml.

For system services (proxy, dashboard, task-scheduler, memory), use:
  mcp-compose system down [SERVICE...]

Examples:
  mcp-compose stop server1 server2   # Stop specific user services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("no services specified to stop")
			}

			file, _ := cmd.Flags().GetString("file")

			systemSvcs, userSvcs := SplitSystemAndUserServices(args)

			if len(systemSvcs) > 0 {
				fmt.Printf("Warning: System services detected: %v\n", systemSvcs)
				fmt.Println("Please use 'mcp-compose system down' for system services")
				fmt.Println("This behavior will be removed in v2.0")
				fmt.Println()

				for _, service := range systemSvcs {
					switch service {
					case "proxy":
						if err := stopProxy(); err != nil {
							fmt.Printf("Failed to stop proxy: %v\n", err)
						}
					case "dashboard":
						if err := stopDashboard(file); err != nil {
							fmt.Printf("Failed to stop dashboard: %v\n", err)
						}
					case "task-scheduler":
						if err := stopTaskScheduler(file); err != nil {
							fmt.Printf("Failed to stop task scheduler: %v\n", err)
						}
					}
				}
			}

			if len(userSvcs) > 0 {
				return compose.Stop(file, userSvcs)
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

	fmt.Println("Proxy stopped successfully.")

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

	fmt.Println("Dashboard stopped successfully.")

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

	fmt.Println("Task scheduler stopped successfully.")

	return nil
}
