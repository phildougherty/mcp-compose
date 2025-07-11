// internal/cmd/completion.go
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

func NewCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate the autocompletion script for the specified shell",
		Long: `Generate the autocompletion script for mcp-compose for the specified shell.
To load completions:

Bash:
  $ source <(mcp-compose completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ mcp-compose completion bash > /etc/bash_completion.d/mcp-compose
  # macOS:
  $ mcp-compose completion bash > $(brew --prefix)/etc/bash_completion.d/mcp-compose

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ mcp-compose completion zsh > "${fpath[1]}/_mcp-compose"

Fish:
  $ mcp-compose completion fish > ~/.config/fish/completions/mcp-compose.fish

PowerShell:
  PS> mcp-compose completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> mcp-compose completion powershell > mcp-compose.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":

				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":

				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":

				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":

				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}

			return nil
		},
	}

	return cmd
}
