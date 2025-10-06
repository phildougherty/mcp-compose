package cmd

import (
	"fmt"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/output"
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
			verbose, _ := cmd.Flags().GetBool("verbose")
			output.SetVerbose(verbose)

			if len(args) == 0 {
				return restartAllSystemServices(file, verbose)
			}

			for _, service := range args {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service. Use 'mcp-compose restart %s' for user services", service, service)
				}
			}

			return restartSystemServices(file, args, verbose)
		},
	}

	cmd.Flags().Bool("verbose", false, "Enable verbose output")

	return cmd
}

func restartAllSystemServices(configFile string, verbose bool) error {
	allServices := []string{"proxy", "dashboard", "task-scheduler", "memory"}

	return restartSystemServices(configFile, allServices, verbose)
}

func restartSystemServices(configFile string, services []string, verbose bool) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	successCount := 0
	totalCount := len(services)

	for _, service := range services {
		startTime := time.Now()
		serviceOutput := output.NewServiceOutput(service, verbose)

		switch service {
		case "proxy":
			serviceOutput.Start("restarting proxy")
			err := restartProxy()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "dashboard":
			serviceOutput.Start("restarting dashboard")
			dashManager := dashboard.NewManager(cfg, runtime)
			dashManager.SetConfigFile(configFile)
			if err := dashManager.Stop(); err != nil {
				output.Verbose(fmt.Sprintf("Warning during stop: %v", err))
			}
			err := dashManager.Start()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "task-scheduler":
			serviceOutput.Start("restarting task scheduler")
			taskManager := task_scheduler.NewManager(cfg, runtime)
			taskManager.SetConfigFile(configFile)
			err := taskManager.Restart()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "memory":
			serviceOutput.Start("restarting memory server")
			if err := stopSystemServices(configFile, []string{"memory"}); err != nil {
				output.Verbose(fmt.Sprintf("Warning during stop: %v", err))
			}
			err := startSystemServices(configFile, []string{"memory"}, false)
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		default:
			return fmt.Errorf("unknown system service: %s", service)
		}

		if verbose {
			output.Verbose(fmt.Sprintf("Duration: %s", output.ShortDuration(time.Since(startTime))))
		}
	}

	output.Summary(successCount, totalCount)

	return nil
}