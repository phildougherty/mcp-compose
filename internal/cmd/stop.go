// internal/cmd/stop.go
package cmd

import (
	"mcpcompose/internal/compose"

	"github.com/spf13/cobra"
)

func NewStopCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop [SERVER...]",
		Short: "Stop specific MCP servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			return compose.Stop(file, args)
		},
	}
	return cmd
}
