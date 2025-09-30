package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/compose"
	"github.com/phildougherty/mcp-compose/internal/container"

	"github.com/spf13/cobra"
)

func NewPsCommand() *cobra.Command {
	var showAll bool
	var showSystem bool

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List MCP services and their status",
		Long: `List MCP services and their status.

By default, shows only user services defined in mcp-compose.yaml.

Flags:
  --all      Show both user and system services
  --system   Show only system services

Examples:
  mcp-compose ps             # List user services
  mcp-compose ps --all       # List all services
  mcp-compose ps --system    # List system services only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			if showSystem {
				return listSystemServices()
			}

			if showAll {
				if err := compose.List(file); err != nil {
					return err
				}
				fmt.Println()
				return listSystemServices()
			}

			return compose.List(file)
		},
	}

	cmd.Flags().BoolVar(&showAll, "all", false, "Show both user and system services")
	cmd.Flags().BoolVar(&showSystem, "system", false, "Show only system services")

	return cmd
}

func listSystemServices() error {
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