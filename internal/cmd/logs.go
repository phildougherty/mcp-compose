// internal/cmd/logs.go
package cmd

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/phildougherty/mcp-compose/internal/compose"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/spf13/cobra"
)

func NewLogsCommand() *cobra.Command {
	var follow bool
	var since string
	var until string
	var tail int
	var level string
	var grep string
	var noColor bool
	var showAll bool
	var showSystem bool

	cmd := &cobra.Command{
		Use:   "logs [SERVER...]",
		Short: "View logs from MCP services with filtering",
		Long: `View logs from MCP user services.

For system services, use:
  mcp-compose system logs [SERVICE...]

Or use flags:
  --system   Show only system service logs
  --all      Show both user and system service logs

Examples:
  mcp-compose logs                              # Show logs from user services
  mcp-compose logs --system                     # Show logs from system services
  mcp-compose logs --all                        # Show logs from all services
  mcp-compose logs server1 -f                   # Follow specific server logs
  mcp-compose logs --level error                # Show only error logs
  mcp-compose logs --grep "authentication"      # Filter logs by pattern
  mcp-compose logs --since "2h"                 # Logs from last 2 hours
  mcp-compose logs --tail 50                    # Last 50 lines`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			opts := LogOptions{
				Follow:  follow,
				Since:   since,
				Until:   until,
				Tail:    tail,
				Level:   level,
				Grep:    grep,
				NoColor: noColor,
			}

			if showSystem {
				systemSvcs := []string{"proxy", "dashboard", "task-scheduler", "memory"}
				return runLogsCommand(file, systemSvcs, opts)
			}

			if showAll {
				if err := runLogsCommand(file, args, opts); err != nil {
					return err
				}
				fmt.Println()
				systemSvcs := []string{"proxy", "dashboard", "task-scheduler", "memory"}
				return runLogsCommand(file, systemSvcs, opts)
			}

			return runLogsCommand(file, args, opts)
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since timestamp (e.g., 2h, 30m, 2023-01-01T00:00:00)")
	cmd.Flags().StringVar(&until, "until", "", "Show logs until timestamp")
	cmd.Flags().IntVarP(&tail, "tail", "n", 0, "Number of lines to show from the end")
	cmd.Flags().StringVar(&level, "level", "", "Filter by log level (debug, info, warn, error)")
	cmd.Flags().StringVar(&grep, "grep", "", "Filter logs by pattern (regex supported)")
	cmd.Flags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show logs from both user and system services")
	cmd.Flags().BoolVar(&showSystem, "system", false, "Show logs from system services only")

	return cmd
}

type LogOptions struct {
	Follow  bool
	Since   string
	Until   string
	Tail    int
	Level   string
	Grep    string
	NoColor bool
}

type LogFormatter struct {
	debugColor   *color.Color
	infoColor    *color.Color
	warnColor    *color.Color
	errorColor   *color.Color
	timestampColor *color.Color
	enabled      bool
}

func NewLogFormatter(enabled bool) *LogFormatter {
	return &LogFormatter{
		debugColor:     color.New(color.FgCyan),
		infoColor:      color.New(color.FgBlue),
		warnColor:      color.New(color.FgYellow),
		errorColor:     color.New(color.FgRed, color.Bold),
		timestampColor: color.New(color.Faint),
		enabled:        enabled,
	}
}

func (f *LogFormatter) Format(line string) string {
	if !f.enabled {
		return line
	}

	lowerLine := strings.ToLower(line)

	if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "fatal") {
		return f.errorColor.Sprint(line)
	}

	if strings.Contains(lowerLine, "warn") || strings.Contains(lowerLine, "warning") {
		return f.warnColor.Sprint(line)
	}

	if strings.Contains(lowerLine, "debug") {
		return f.debugColor.Sprint(line)
	}

	if strings.Contains(lowerLine, "info") {
		return f.infoColor.Sprint(line)
	}

	return line
}

func runLogsCommand(configFile string, serverNames []string, opts LogOptions) error {
	// Check if we have special container requests (proxy, dashboard, etc.)
	specialContainers := make(map[string]string)
	regularServers := make([]string, 0)

	for _, name := range serverNames {
		switch name {
		case "proxy":
			specialContainers["proxy"] = "mcp-compose-http-proxy"
		case "dashboard":
			specialContainers["dashboard"] = "mcp-compose-dashboard"
		case "task-scheduler":
			specialContainers["task-scheduler"] = "mcp-compose-task-scheduler"
		case "memory":
			specialContainers["memory"] = "mcp-compose-memory"
		case "postgres-memory":
			specialContainers["postgres-memory"] = "mcp-compose-postgres-memory"
		default:
			regularServers = append(regularServers, name)
		}
	}

	formatter := NewLogFormatter(!opts.NoColor)
	filter := createLogFilter(opts)

	// If we only have special containers, handle them directly
	if len(specialContainers) > 0 && len(regularServers) == 0 {

		return handleSpecialContainerLogs(specialContainers, opts, formatter, filter)
	}

	// If we have a mix or only regular servers, use the compose logs function
	if len(regularServers) > 0 {
		if err := compose.Logs(configFile, regularServers, opts.Follow); err != nil {

			return err
		}
	}

	// Handle special containers after regular servers
	if len(specialContainers) > 0 {
		if len(regularServers) > 0 {
			fmt.Println()
		}

		return handleSpecialContainerLogs(specialContainers, opts, formatter, filter)
	}

	// If no specific servers requested, default to compose.Logs behavior
	if len(serverNames) == 0 {

		return compose.Logs(configFile, serverNames, opts.Follow)
	}

	return nil
}

type LogFilter struct {
	sinceTime    time.Time
	untilTime    time.Time
	levelPattern *regexp.Regexp
	grepPattern  *regexp.Regexp
}

func createLogFilter(opts LogOptions) *LogFilter {
	filter := &LogFilter{}

	if opts.Since != "" {
		if duration, err := parseDuration(opts.Since); err == nil {
			filter.sinceTime = time.Now().Add(-duration)
		} else if t, err := time.Parse(time.RFC3339, opts.Since); err == nil {
			filter.sinceTime = t
		}
	}

	if opts.Until != "" {
		if duration, err := parseDuration(opts.Until); err == nil {
			filter.untilTime = time.Now().Add(-duration)
		} else if t, err := time.Parse(time.RFC3339, opts.Until); err == nil {
			filter.untilTime = t
		}
	}

	if opts.Level != "" {
		pattern := fmt.Sprintf("(?i)(level|lvl)[=:]\\s*%s|\\b%s\\b", opts.Level, opts.Level)
		filter.levelPattern = regexp.MustCompile(pattern)
	}

	if opts.Grep != "" {
		filter.grepPattern = regexp.MustCompile(opts.Grep)
	}

	return filter
}

func (f *LogFilter) Match(line string) bool {
	if f.levelPattern != nil && !f.levelPattern.MatchString(line) {
		return false
	}

	if f.grepPattern != nil && !f.grepPattern.MatchString(line) {
		return false
	}

	return true
}

func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration")
	}

	validUnits := []string{"h", "m", "s", "ms", "us", "ns"}
	hasUnit := false

	for _, unit := range validUnits {
		if strings.HasSuffix(s, unit) {
			hasUnit = true

			break
		}
	}

	if !hasUnit {
		s += "m"
	}

	return time.ParseDuration(s)
}

func handleSpecialContainerLogs(containers map[string]string, opts LogOptions, formatter *LogFormatter, filter *LogFilter) error {
	runtime, err := container.DetectRuntime()
	if err != nil {

		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	if runtime.GetRuntimeName() == "none" {
		fmt.Println("No container runtime detected. Cannot show logs for built-in service containers.")

		return nil
	}

	containerNames := make([]string, 0, len(containers))
	displayNames := make([]string, 0, len(containers))

	for displayName, containerName := range containers {
		// Check if container exists
		status, err := runtime.GetContainerStatus(containerName)
		if err != nil || status == "stopped" {
			fmt.Printf("Warning: Container '%s' (%s) not found or not running\n", displayName, containerName)

			continue
		}
		containerNames = append(containerNames, containerName)
		displayNames = append(displayNames, displayName)
	}

	if len(containerNames) == 0 {

		return fmt.Errorf("no running containers found for the requested services")
	}

	// Show logs for each container
	for i, containerName := range containerNames {
		if len(containerNames) > 1 {
			if i > 0 && !opts.Follow {
				fmt.Println("\n---")
			}
			fmt.Printf("=== Logs for %s (%s) ===\n", displayNames[i], containerName)
		}

		if err := showFilteredLogs(runtime, containerName, opts, formatter, filter); err != nil {
			fmt.Printf("Warning: failed to show logs for %s (%s): %v\n",
				displayNames[i], containerName, err)
		}

		if opts.Follow && len(containerNames) > 1 {
			fmt.Printf("\nNote: Following logs for %s only. Use separate commands to follow multiple containers.\n", displayNames[0])

			break
		}
	}

	return nil
}

func showFilteredLogs(runtime container.Runtime, containerName string, opts LogOptions, formatter *LogFormatter, filter *LogFilter) error {
	if err := runtime.ShowContainerLogs(containerName, opts.Follow); err != nil {
		return err
	}

	return nil
}
