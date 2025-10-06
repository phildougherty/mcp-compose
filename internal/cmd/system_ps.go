package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/phildougherty/mcp-compose/internal/constants"
	"github.com/phildougherty/mcp-compose/internal/container"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

func NewSystemPsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List system services status",
		Long: `List the status of system services.

Examples:
  mcp-compose system ps    # List all system services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listSystemServicesTable()
		},
	}

	return cmd
}

func listSystemServicesTable() error {
	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	runningColor := color.New(color.FgGreen).SprintFunc()
	stoppedColor := color.New(color.FgRed).SprintFunc()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, constants.TableColumnSpacing, ' ', 0)
	if _, err := fmt.Fprintln(w, "SERVICE\tSTATUS\tCONTAINER"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for serviceName, containerName := range systemServices {
		rawStatus, err := runtime.GetContainerStatus(containerName)
		var statusStr string
		if err != nil || rawStatus == "stopped" {
			statusStr = stoppedColor("stopped")
		} else {
			switch strings.ToLower(rawStatus) {
			case "running":
				statusStr = runningColor("Running")
			default:
				statusStr = stoppedColor(rawStatus)
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", serviceName, statusStr, containerName)
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}