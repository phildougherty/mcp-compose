// internal/cmd/up.go
package cmd

import (
	"mcpcompose/internal/compose"

	"github.com/spf13/cobra"
)

func NewUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [SERVER...]",
		Short: "Create and start MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			return compose.Up(file, args)
		},
	}
	return cmd
}
