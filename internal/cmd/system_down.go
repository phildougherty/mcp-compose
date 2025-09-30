package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/memory"

	"github.com/spf13/cobra"
)

func NewSystemDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [SERVICE...]",
		Short: "Stop and remove system services",
		Long: `Stop and remove one or more system services.

Available system services:
  proxy           HTTP proxy server
  dashboard       Web dashboard
  task-scheduler  Task scheduler
  memory          Memory server

Examples:
  mcp-compose system down                    # Stop all system services
  mcp-compose system down proxy dashboard    # Stop specific services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			if len(args) == 0 {
				return stopAllSystemServices(file)
			}

			for _, service := range args {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service. Use 'mcp-compose down %s' for user services", service, service)
				}
			}

			return stopSystemServices(file, args)
		},
	}

	return cmd
}

func stopAllSystemServices(configFile string) error {
	allServices := []string{"proxy", "dashboard", "task-scheduler", "memory"}

	return stopSystemServices(configFile, allServices)
}

func stopSystemServices(configFile string, services []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Printf("Warning: Could not load config: %v\n", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	for _, service := range services {
		switch service {
		case "proxy":
			if err := downProxy(); err != nil {
				fmt.Printf("Warning: Failed to stop proxy: %v\n", err)
			}
		case "dashboard":
			if err := downDashboard(configFile); err != nil {
				fmt.Printf("Warning: Failed to stop dashboard: %v\n", err)
			}
		case "task-scheduler":
			if err := downTaskScheduler(configFile); err != nil {
				fmt.Printf("Warning: Failed to stop task scheduler: %v\n", err)
			}
		case "memory":
			memoryManager := memory.NewManager(cfg, runtime)
			memoryManager.SetConfigFile(configFile)
			if err := memoryManager.Stop(); err != nil {
				fmt.Printf("Warning: Failed to stop memory server: %v\n", err)
			} else {
				fmt.Println("Memory server stopped successfully")
			}
		default:
			return fmt.Errorf("unknown system service: %s", service)
		}
	}

	return nil
}