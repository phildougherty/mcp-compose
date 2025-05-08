// internal/cmd/down.go
package cmd

import (
	"mcpcompose/internal/compose" // Ensure this import path is correct for your project

	"github.com/spf13/cobra"
)

func NewDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [SERVER...]", // Allow specifying servers, though current Down might ignore them
		Short: "Stop and remove MCP servers specified, or all if none are specified.",
		Long: `Stop and remove MCP servers.
If server names are provided, only those specific servers will be targeted for shutdown.
If no server names are provided, all servers defined in the compose file will be stopped and removed.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			// Pass the 'args' (which are the server names from the command line)
			// to the compose.Down function.
			// If 'args' is empty, compose.Down should default to stopping all servers.
			return compose.Down(file, args) // ছিল: return compose.Down(file)
		},
	}
	return cmd
}
