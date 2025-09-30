package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/container"

	"github.com/spf13/cobra"
)

func NewSystemPsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List system services status",
		Long: `List the status of system services.

Examples:
  mcp-compose system ps    # List all system services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSystemServicesTable()
		},
	}

	return cmd
}

func listSystemServicesTable() error {
	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	fmt.Println("SYSTEM SERVICES:")
	fmt.Printf("%-20s %-30s %-10s\n", "SERVICE", "CONTAINER", "STATUS")
	fmt.Println("--------------------------------------------------------------------------------")

	for serviceName, containerName := range systemServices {
		status, err := runtime.GetContainerStatus(containerName)
		if err != nil || status == "stopped" {
			status = "stopped"
		}
		fmt.Printf("%-20s %-30s %-10s\n", serviceName, containerName, status)
	}

	return nil
}