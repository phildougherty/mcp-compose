// internal/cmd/down.go
package cmd

import (
	"fmt"
	"github.com/phildougherty/mcp-compose/internal/compose"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/memory"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"

	"github.com/spf13/cobra"
)

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [SERVER...]",
		Short: "Stop and remove MCP user services",
		Long: `Stop and remove MCP user services defined in mcp-compose.yaml.

For system services (proxy, dashboard, task-scheduler, memory), use:
  mcp-compose system down [SERVICE...]

Examples:
  mcp-compose down                    # Stop and remove all user services
  mcp-compose down server1 server2   # Stop and remove specific user services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			if len(args) == 0 {
				return downAll(file)
			}

			systemSvcs, userSvcs := SplitSystemAndUserServices(args)

			if len(systemSvcs) > 0 {
				fmt.Printf("Warning: System services detected: %v\n", systemSvcs)
				fmt.Println("Please use 'mcp-compose system down' for system services")
				fmt.Println("This behavior will be removed in v2.0")
				fmt.Println()

				for _, service := range systemSvcs {
					switch service {
					case "proxy":
						if err := downProxy(); err != nil {
							fmt.Printf("Failed to stop/remove proxy: %v\n", err)
						}
					case "dashboard":
						if err := downDashboard(file); err != nil {
							fmt.Printf("Failed to stop/remove dashboard: %v\n", err)
						}
					case "task-scheduler":
						if err := downTaskScheduler(file); err != nil {
							fmt.Printf("Failed to stop/remove task scheduler: %v\n", err)
						}
					case "memory":
						if err := downMemory(file); err != nil {
							fmt.Printf("Failed to stop/remove memory server: %v\n", err)
						}
					}
				}
			}

			if len(userSvcs) > 0 {
				return compose.Down(file, userSvcs)
			}

			return nil
		},
	}

	return cmd
}

func downAll(configFile string) error {
	fmt.Println("Stopping and removing all MCP Compose services...")

	// Stop built-in services first
	if err := downBuiltInServices(configFile); err != nil {
		fmt.Printf("Warning: Error stopping built-in services: %v\n", err)
	}

	// Then stop all docker compose services

	return compose.Down(configFile, []string{})
}

func downBuiltInServices(configFile string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Warning: Could not load config for built-in services: %v\n", err)

		return nil
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		fmt.Printf("Warning: Could not detect container runtime: %v\n", err)

		return nil
	}

	// Stop proxy
	if err := downProxy(); err != nil {
		fmt.Printf("Warning: Failed to stop proxy: %v\n", err)
	}

	// Stop dashboard
	if err := downDashboard(configFile); err != nil {
		fmt.Printf("Warning: Failed to stop dashboard: %v\n", err)
	}

	// Stop task scheduler
	if err := downTaskScheduler(configFile); err != nil {
		fmt.Printf("Warning: Failed to stop task scheduler: %v\n", err)
	}

	// Stop memory server
	memoryManager := memory.NewManager(cfg, runtime)
	memoryManager.SetConfigFile(configFile)
	if err := memoryManager.Stop(); err != nil {
		fmt.Printf("Warning: Failed to stop memory server: %v\n", err)
	}

	return nil
}

func downProxy() error {
	fmt.Println("Stopping and removing MCP proxy...")
	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	proxyContainerName := "mcp-compose-http-proxy"
	if err := runtime.StopContainer(proxyContainerName); err != nil {
		// Don't return error if container doesn't exist
		fmt.Printf("Note: Proxy container may not be running: %v\n", err)
	}

	fmt.Println("Proxy stopped successfully.")

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
		fmt.Printf("Note: Dashboard may not be running: %v\n", err)
	}

	fmt.Println("Dashboard stopped successfully.")

	return nil
}

func downTaskScheduler(configFile string) error {
	fmt.Println("Stopping and removing MCP task scheduler...")
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
		fmt.Printf("Note: Task scheduler may not be running: %v\n", err)
	}

	fmt.Println("Task scheduler stopped successfully.")

	return nil
}

func downMemory(configFile string) error {
	fmt.Println("Stopping and removing MCP memory server...")
	cfg, err := config.LoadConfig(configFile)
	if err != nil {

		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	memoryManager := memory.NewManager(cfg, runtime)
	memoryManager.SetConfigFile(configFile)

	if err := memoryManager.Stop(); err != nil {
		fmt.Printf("Note: Memory server may not be running: %v\n", err)
	}

	fmt.Println("Memory server stopped successfully.")

	return nil
}
