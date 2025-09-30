package cmd

import (
	"github.com/spf13/cobra"
)

func NewSystemCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Manage system services (proxy, dashboard, task-scheduler, memory)",
		Long: `Manage MCP-Compose system services.

System services are infrastructure components:
  - proxy:          HTTP proxy server
  - dashboard:      Web dashboard
  - task-scheduler: Cron-like task scheduler
  - memory:         Persistent memory server

Examples:
  mcp-compose system up                    # Start all system services
  mcp-compose system up proxy dashboard    # Start specific system services
  mcp-compose system down                  # Stop all system services
  mcp-compose system ps                    # List system services status
  mcp-compose system logs proxy -f         # Follow proxy logs`,
	}

	cmd.AddCommand(NewSystemUpCommand())
	cmd.AddCommand(NewSystemDownCommand())
	cmd.AddCommand(NewSystemRestartCommand())
	cmd.AddCommand(NewSystemPsCommand())
	cmd.AddCommand(NewSystemLogsCommand())
	cmd.AddCommand(NewSystemStatusCommand())

	return cmd
}