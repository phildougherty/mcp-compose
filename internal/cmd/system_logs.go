package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewSystemLogsCommand() *cobra.Command {
	var follow bool
	var tail int

	cmd := &cobra.Command{
		Use:   "logs [SERVICE...]",
		Short: "View logs from system services",
		Long: `View logs from system services.

Available system services:
  proxy           HTTP proxy server
  dashboard       Web dashboard
  task-scheduler  Task scheduler
  memory          Memory server

Examples:
  mcp-compose system logs                    # Show logs from all system services
  mcp-compose system logs proxy -f           # Follow proxy logs
  mcp-compose system logs dashboard --tail 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			services := args
			if len(services) == 0 {
				services = []string{"proxy", "dashboard", "task-scheduler", "memory"}
			}

			for _, service := range services {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service", service)
				}
			}

			opts := LogOptions{
				Follow: follow,
				Tail:   tail,
			}

			return runLogsCommand(file, services, opts)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "Number of lines to show from the end")

	return cmd
}