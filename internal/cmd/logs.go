// internal/cmd/logs.go
package cmd

import (
	"mcpcompose/internal/compose"

	"github.com/spf13/cobra"
)

func NewLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs [SERVER...]",
		Short: "View logs from MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			follow, _ := cmd.Flags().GetBool("follow")
			return compose.Logs(file, args, follow)
		},
	}
	cmd.Flags().BoolP("follow", "f", false, "Follow log output")
	return cmd
}
