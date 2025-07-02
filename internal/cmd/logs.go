// internal/cmd/logs.go
package cmd

import (
	"fmt"
	"mcpcompose/internal/compose"
	"mcpcompose/internal/container"

	"github.com/spf13/cobra"
)

func NewLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [SERVER...]",
		Short: "View logs from MCP servers",
		Long: `View logs from MCP servers, proxy, dashboard, task-scheduler, or memory server.
Special containers:
  proxy          - Shows logs from mcp-compose-http-proxy container
  dashboard      - Shows logs from mcp-compose-dashboard container
  task-scheduler - Shows logs from mcp-compose-task-scheduler container
  memory         - Shows logs from mcp-compose-memory container
  postgres-memory - Shows logs from mcp-compose-postgres-memory container

Examples:
  mcp-compose logs                    # Show logs from all servers
  mcp-compose logs proxy -f           # Follow proxy logs
  mcp-compose logs dashboard -f       # Follow dashboard logs  
  mcp-compose logs task-scheduler -f  # Follow task scheduler logs
  mcp-compose logs memory -f          # Follow memory server logs
  mcp-compose logs filesystem -f      # Follow filesystem server logs
  mcp-compose logs proxy dashboard -f # Follow both proxy and dashboard logs`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			follow, _ := cmd.Flags().GetBool("follow")
			return runLogsCommand(file, args, follow)
		},
	}
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	return cmd
}

func runLogsCommand(configFile string, serverNames []string, follow bool) error {
	// Check if we have special container requests (proxy, dashboard, etc.)
	specialContainers := make(map[string]string)
	regularServers := make([]string, 0)

	for _, name := range serverNames {
		switch name {
		case "proxy":
			specialContainers["proxy"] = "mcp-compose-http-proxy"
		case "dashboard":
			specialContainers["dashboard"] = "mcp-compose-dashboard"
		case "task-scheduler":
			specialContainers["task-scheduler"] = "mcp-compose-task-scheduler"
		case "memory":
			specialContainers["memory"] = "mcp-compose-memory"
		case "postgres-memory":
			specialContainers["postgres-memory"] = "mcp-compose-postgres-memory"
		default:
			regularServers = append(regularServers, name)
		}
	}

	// If we only have special containers, handle them directly
	if len(specialContainers) > 0 && len(regularServers) == 0 {
		return handleSpecialContainerLogs(specialContainers, follow)
	}

	// If we have a mix or only regular servers, use the compose logs function
	if len(regularServers) > 0 {
		if err := compose.Logs(configFile, regularServers, follow); err != nil {
			return err
		}
	}

	// Handle special containers after regular servers
	if len(specialContainers) > 0 {
		if len(regularServers) > 0 {
			fmt.Println() // Add spacing between regular and special logs
		}
		return handleSpecialContainerLogs(specialContainers, follow)
	}

	// If no specific servers requested, default to compose.Logs behavior
	if len(serverNames) == 0 {
		return compose.Logs(configFile, serverNames, follow)
	}

	return nil
}

func handleSpecialContainerLogs(containers map[string]string, follow bool) error {
	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	if runtime.GetRuntimeName() == "none" {
		fmt.Println("No container runtime detected. Cannot show logs for built-in service containers.")
		return nil
	}

	containerNames := make([]string, 0, len(containers))
	displayNames := make([]string, 0, len(containers))

	for displayName, containerName := range containers {
		// Check if container exists
		status, err := runtime.GetContainerStatus(containerName)
		if err != nil || status == "stopped" {
			fmt.Printf("Warning: Container '%s' (%s) not found or not running\n", displayName, containerName)
			continue
		}
		containerNames = append(containerNames, containerName)
		displayNames = append(displayNames, displayName)
	}

	if len(containerNames) == 0 {
		return fmt.Errorf("no running containers found for the requested services")
	}

	// Show logs for each container
	for i, containerName := range containerNames {
		if len(containerNames) > 1 {
			if i > 0 && !follow {
				fmt.Println("\n---")
			}
			fmt.Printf("=== Logs for %s (%s) ===\n", displayNames[i], containerName)
		}

		if err := runtime.ShowContainerLogs(containerName, follow); err != nil {
			fmt.Printf("Warning: failed to show logs for %s (%s): %v\n",
				displayNames[i], containerName, err)
		}

		// For follow mode with multiple containers, we can only follow one at a time
		// Docker/Podman doesn't support multiplexed following easily
		if follow && len(containerNames) > 1 {
			fmt.Printf("\nNote: Following logs for %s only. Use separate commands to follow multiple containers.\n", displayNames[0])
			break
		}
	}

	return nil
}
