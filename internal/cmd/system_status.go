package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/container"

	"github.com/spf13/cobra"
)

func NewSystemStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show system services health overview",
		Long: `Show a health overview of all system services.

Examples:
  mcp-compose system status    # Show system health`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return showSystemStatus()
		},
	}

	return cmd
}

func showSystemStatus() error {
	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("                        MCP-COMPOSE SYSTEM STATUS")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	totalServices := len(systemServices)
	runningServices := 0
	stoppedServices := 0

	for serviceName, containerName := range systemServices {
		status, err := runtime.GetContainerStatus(containerName)
		statusIcon := "✗"
		statusText := "STOPPED"

		if err == nil && status == "running" {
			statusIcon = "✓"
			statusText = "RUNNING"
			runningServices++
		} else {
			stoppedServices++
		}

		fmt.Printf("  %s %-20s: %s\n", statusIcon, serviceName, statusText)
	}

	fmt.Println()
	fmt.Println("───────────────────────────────────────────────────────────────────────────")
	fmt.Printf("Total Services: %d  |  Running: %d  |  Stopped: %d\n",
		totalServices, runningServices, stoppedServices)
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")

	if stoppedServices > 0 {
		fmt.Println()
		fmt.Println("Use 'mcp-compose system up' to start stopped services")
	}

	return nil
}