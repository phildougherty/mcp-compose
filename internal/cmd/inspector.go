// internal/cmd/inspector.go
package cmd

import (
	"mcpcompose/internal/inspector"

	"github.com/spf13/cobra"
)

func NewInspectorCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspector [SERVER]",
		Short: "Launch the MCP Inspector for a server",
		Long: `Launch the MCP Inspector tool for debugging and testing MCP servers.
If a server name is provided, it connects to that specific server.
Otherwise, it provides a UI to select from available servers.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			port, _ := cmd.Flags().GetInt("port")

			var serverName string
			if len(args) > 0 {
				serverName = args[0]
			}

			return inspector.LaunchInspector(file, serverName, port)
		},
	}

	cmd.Flags().IntP("port", "p", 8090, "Port to run the inspector on")

	return cmd
}

// Add this to root.go
// rootCmd.AddCommand(NewInspectorCommand())
