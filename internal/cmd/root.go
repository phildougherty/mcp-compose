// internal/cmd/root.go
package cmd

import (
	"github.com/spf13/cobra"
)

func NewRootCommand(version string) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "mcp-compose",
		Short:   "Manage MCP servers with compose",
		Long:    `MCP-Compose is a tool for defining and running multi-server Model Context Protocol applications.`,
		Version: version, // ← Add this line to enable --version flag
	}

	rootCmd.PersistentFlags().StringP("file", "c", "mcp-compose.yaml", "Specify compose file")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(NewUpCommand())
	rootCmd.AddCommand(NewDownCommand())
	rootCmd.AddCommand(NewStartCommand())
	rootCmd.AddCommand(NewStopCommand())
	rootCmd.AddCommand(NewRestartCommand())
	rootCmd.AddCommand(NewLsCommand())
	rootCmd.AddCommand(NewLogsCommand())
	rootCmd.AddCommand(NewValidateCommand())
	rootCmd.AddCommand(NewCompletionCommand())
	rootCmd.AddCommand(NewCreateConfigCommand())
	rootCmd.AddCommand(NewProxyCommand())
	rootCmd.AddCommand(NewReloadCommand())
	rootCmd.AddCommand(NewDashboardCommand())
	rootCmd.AddCommand(NewTaskSchedulerCommand())
	rootCmd.AddCommand(NewMemoryCommand())


	return rootCmd
}
