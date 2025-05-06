// internal/cmd/down.go
package cmd

import (
	"mcpcompose/internal/compose"

	"github.com/spf13/cobra"
)

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop and remove MCP servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			return compose.Down(file)
		},
	}
	return cmd
}
