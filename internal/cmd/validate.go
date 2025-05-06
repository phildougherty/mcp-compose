// internal/cmd/validate.go
package cmd

import (
	"mcpcompose/internal/compose"

	"github.com/spf13/cobra"
)

func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the compose file",
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			return compose.Validate(file)
		},
	}
	return cmd
}
