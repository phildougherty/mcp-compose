package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/memory"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"

	"github.com/spf13/cobra"
)

func NewSystemUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [SERVICE...]",
		Short: "Start system services",
		Long: `Start one or more system services.

Available system services:
  proxy           HTTP proxy server
  dashboard       Web dashboard
  task-scheduler  Task scheduler
  memory          Memory server

Examples:
  mcp-compose system up                    # Start all system services
  mcp-compose system up proxy dashboard    # Start specific services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			if len(args) == 0 {
				return startAllSystemServices(file)
			}

			for _, service := range args {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service. Use 'mcp-compose up %s' for user services", service, service)
				}
			}

			return startSystemServices(file, args)
		},
	}

	return cmd
}

func startAllSystemServices(configFile string) error {
	allServices := []string{"proxy", "dashboard", "task-scheduler", "memory"}

	return startSystemServices(configFile, allServices)
}

func startSystemServices(configFile string, services []string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	for _, service := range services {
		switch service {
		case "proxy":
			fmt.Println("Note: Use 'mcp-compose proxy' to start the proxy with custom options")
		case "dashboard":
			dashManager := dashboard.NewManager(cfg, runtime)
			dashManager.SetConfigFile(configFile)
			if err := dashManager.Start(); err != nil {
				return fmt.Errorf("failed to start dashboard: %w", err)
			}
			fmt.Println("Dashboard started successfully")
		case "task-scheduler":
			taskManager := task_scheduler.NewManager(cfg, runtime)
			taskManager.SetConfigFile(configFile)
			if err := taskManager.Start(); err != nil {
				return fmt.Errorf("failed to start task scheduler: %w", err)
			}
			fmt.Println("Task scheduler started successfully")
		case "memory":
			memoryManager := memory.NewManager(cfg, runtime)
			memoryManager.SetConfigFile(configFile)
			if err := memoryManager.Start(); err != nil {
				return fmt.Errorf("failed to start memory server: %w", err)
			}
			fmt.Println("Memory server started successfully")
		default:
			return fmt.Errorf("unknown system service: %s", service)
		}
	}

	return nil
}