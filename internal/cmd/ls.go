// internal/cmd/ls.go
package cmd

import (
	"github.com/phildougherty/mcp-compose/internal/compose"

	"github.com/spf13/cobra"
)

func NewLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "List all defined MCP servers and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			return compose.List(file)
		},
	}

	return cmd
}
