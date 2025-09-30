package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"

	"github.com/spf13/cobra"
)

func NewSystemRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [SERVICE...]",
		Short: "Restart system services",
		Long: `Restart one or more system services.

Available system services:
  proxy           HTTP proxy server
  dashboard       Web dashboard
  task-scheduler  Task scheduler
  memory          Memory server

Examples:
  mcp-compose system restart                    # Restart all system services
  mcp-compose system restart proxy dashboard    # Restart specific services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			if len(args) == 0 {
				return restartAllSystemServices(file)
			}

			for _, service := range args {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service. Use 'mcp-compose restart %s' for user services", service, service)
				}
			}

			return restartSystemServices(file, args)
		},
	}

	return cmd
}

func restartAllSystemServices(configFile string) error {
	allServices := []string{"proxy", "dashboard", "task-scheduler", "memory"}

	return restartSystemServices(configFile, allServices)
}

func restartSystemServices(configFile string, services []string) error {
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
			if err := restartProxy(); err != nil {
				return fmt.Errorf("failed to restart proxy: %w", err)
			}
		case "dashboard":
			dashManager := dashboard.NewManager(cfg, runtime)
			dashManager.SetConfigFile(configFile)
			if err := dashManager.Stop(); err != nil {
				fmt.Printf("Warning: Error stopping dashboard: %v\n", err)
			}
			if err := dashManager.Start(); err != nil {
				return fmt.Errorf("failed to start dashboard: %w", err)
			}
			fmt.Println("Dashboard restarted successfully")
		case "task-scheduler":
			taskManager := task_scheduler.NewManager(cfg, runtime)
			taskManager.SetConfigFile(configFile)
			if err := taskManager.Restart(); err != nil {
				return fmt.Errorf("failed to restart task scheduler: %w", err)
			}
			fmt.Println("Task scheduler restarted successfully")
		case "memory":
			if err := stopSystemServices(configFile, []string{"memory"}); err != nil {
				fmt.Printf("Warning: Error stopping memory: %v\n", err)
			}
			if err := startSystemServices(configFile, []string{"memory"}); err != nil {
				return fmt.Errorf("failed to start memory: %w", err)
			}
		default:
			return fmt.Errorf("unknown system service: %s", service)
		}
	}

	return nil
}