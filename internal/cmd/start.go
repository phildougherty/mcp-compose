// internal/cmd/start.go
package cmd

import (
	"github.com/phildougherty/mcp-compose/internal/compose"

	"github.com/spf13/cobra"
)

func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [SERVER...]",
		Short: "Start specific MCP servers",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			return compose.Start(file, args)
		},
	}

	return cmd
}
