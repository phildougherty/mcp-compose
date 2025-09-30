// internal/cmd/ls.go
package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/compose"

	"github.com/spf13/cobra"
)

func NewLsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ls",
		Short:  "List all defined MCP servers and their status (alias for ps)",
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			fmt.Println("Note: 'ls' is deprecated. Use 'mcp-compose ps' instead.")
			fmt.Println()

			return compose.List(file)
		},
	}

	return cmd
}
