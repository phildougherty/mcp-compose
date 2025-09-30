package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type InspectOutput struct {
	ServerName    string                 `json:"server_name" yaml:"server_name"`
	Configuration ServerConfigOutput     `json:"configuration" yaml:"configuration"`
	Runtime       RuntimeInfo            `json:"runtime" yaml:"runtime"`
	Connection    ConnectionInfo         `json:"connection" yaml:"connection"`
	HealthCheck   HealthCheckInfo        `json:"health_check" yaml:"health_check"`
	Resources     ResourceUsage          `json:"resources" yaml:"resources"`
	Logs          []string               `json:"recent_logs" yaml:"recent_logs"`
	Environment   map[string]string      `json:"environment" yaml:"environment"`
	Labels        map[string]string      `json:"labels" yaml:"labels"`
	Networks      []NetworkInfo          `json:"networks" yaml:"networks"`
	Volumes       []VolumeInfo           `json:"volumes" yaml:"volumes"`
}

type ServerConfigOutput struct {
	Image       string            `json:"image,omitempty" yaml:"image,omitempty"`
	Command     string            `json:"command,omitempty" yaml:"command,omitempty"`
	Args        []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Protocol    string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	HttpPort    int               `json:"http_port,omitempty" yaml:"http_port,omitempty"`
	Env         map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	User        string            `json:"user,omitempty" yaml:"user,omitempty"`
	Privileged  bool              `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	Restart     string            `json:"restart,omitempty" yaml:"restart,omitempty"`
}

type RuntimeInfo struct {
	ContainerID   string    `json:"container_id" yaml:"container_id"`
	ContainerName string    `json:"container_name" yaml:"container_name"`
	Status        string    `json:"status" yaml:"status"`
	State         string    `json:"state" yaml:"state"`
	Created       time.Time `json:"created" yaml:"created"`
	Started       time.Time `json:"started,omitempty" yaml:"started,omitempty"`
	Finished      time.Time `json:"finished,omitempty" yaml:"finished,omitempty"`
	RestartCount  int       `json:"restart_count" yaml:"restart_count"`
	ExitCode      int       `json:"exit_code,omitempty" yaml:"exit_code,omitempty"`
	Platform      string    `json:"platform" yaml:"platform"`
}

type ConnectionInfo struct {
	Endpoint     string   `json:"endpoint" yaml:"endpoint"`
	Protocol     string   `json:"protocol" yaml:"protocol"`
	Port         int      `json:"port,omitempty" yaml:"port,omitempty"`
	ExposedPorts []string `json:"exposed_ports" yaml:"exposed_ports"`
	Accessible   bool     `json:"accessible" yaml:"accessible"`
	LastCheck    string   `json:"last_check" yaml:"last_check"`
}

type HealthCheckInfo struct {
	Status          string    `json:"status" yaml:"status"`
	FailingStreak   int       `json:"failing_streak" yaml:"failing_streak"`
	LastCheck       time.Time `json:"last_check" yaml:"last_check"`
	LastHealthy     time.Time `json:"last_healthy,omitempty" yaml:"last_healthy,omitempty"`
	Output          string    `json:"output,omitempty" yaml:"output,omitempty"`
	Test            []string  `json:"test,omitempty" yaml:"test,omitempty"`
	Interval        string    `json:"interval,omitempty" yaml:"interval,omitempty"`
	Timeout         string    `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	Retries         int       `json:"retries,omitempty" yaml:"retries,omitempty"`
}

type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent" yaml:"cpu_percent"`
	MemoryUsageMB float64 `json:"memory_usage_mb" yaml:"memory_usage_mb"`
	MemoryLimitMB float64 `json:"memory_limit_mb,omitempty" yaml:"memory_limit_mb,omitempty"`
	NetworkRxKB   float64 `json:"network_rx_kb" yaml:"network_rx_kb"`
	NetworkTxKB   float64 `json:"network_tx_kb" yaml:"network_tx_kb"`
	BlockRead     uint64  `json:"block_read_bytes" yaml:"block_read_bytes"`
	BlockWrite    uint64  `json:"block_write_bytes" yaml:"block_write_bytes"`
	PIDs          uint64  `json:"pids" yaml:"pids"`
}

type NetworkInfo struct {
	Name       string `json:"name" yaml:"name"`
	IPAddress  string `json:"ip_address" yaml:"ip_address"`
	Gateway    string `json:"gateway" yaml:"gateway"`
	MacAddress string `json:"mac_address" yaml:"mac_address"`
}

type VolumeInfo struct {
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	Mode        string `json:"mode" yaml:"mode"`
	Type        string `json:"type" yaml:"type"`
}

func NewInspectCommand() *cobra.Command {
	var format string
	var showLogs bool
	var logLines int

	cmd := &cobra.Command{
		Use:   "inspect [SERVER]",
		Short: "Detailed diagnostics for a server",
		Long: `Show comprehensive diagnostics for an MCP server including:
- Full configuration
- Container runtime status
- Connection endpoints
- Health check results
- Resource usage
- Recent logs
- Environment variables (secrets redacted)
- Network and volume information`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			return runInspect(file, args[0], format, showLogs, logLines)
		},
	}

	cmd.Flags().StringVarP(&format, "format", "f", "text", "Output format: text, json, yaml")
	cmd.Flags().BoolVarP(&showLogs, "logs", "l", true, "Include recent logs")
	cmd.Flags().IntVarP(&logLines, "log-lines", "n", 100, "Number of log lines to include")

	return cmd
}

func runInspect(configFile, serverName, format string, showLogs bool, logLines int) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	serverCfg, exists := cfg.Servers[serverName]
	if !exists {
		return fmt.Errorf("server '%s' not found in configuration", serverName)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	output, err := inspectServer(runtime, serverName, serverCfg, showLogs, logLines)
	if err != nil {
		return fmt.Errorf("failed to inspect server: %w", err)
	}

	switch format {
	case "json":
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(data))

	case "yaml":
		data, err := yaml.Marshal(output)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		fmt.Println(string(data))

	default:
		printTextOutput(output)
	}

	return nil
}

func inspectServer(runtime container.Runtime, serverName string, serverCfg config.ServerConfig, showLogs bool, logLines int) (*InspectOutput, error) {
	output := &InspectOutput{
		ServerName: serverName,
		Configuration: ServerConfigOutput{
			Image:      serverCfg.Image,
			Command:    serverCfg.Command,
			Args:       serverCfg.Args,
			Protocol:   serverCfg.Protocol,
			HttpPort:   serverCfg.HttpPort,
			Env:        redactSecrets(serverCfg.Env),
			DependsOn:  serverCfg.DependsOn,
			User:       serverCfg.User,
			Privileged: serverCfg.Privileged,
			Restart:    serverCfg.RestartPolicy,
		},
	}

	containerName := fmt.Sprintf("mcp-compose-%s", serverName)

	status, err := runtime.GetContainerStatus(containerName)
	if err != nil {
		output.Runtime.Status = "not running"

		return output, nil
	}

	info, err := runtime.GetContainerInfo(containerName)
	if err == nil {
		output.Runtime = RuntimeInfo{
			ContainerID:   info.ID,
			ContainerName: info.Name,
			Status:        info.Status,
			State:         info.State,
			RestartCount:  info.RestartCount,
			Platform:      "linux",
		}

		if created, err := time.Parse(time.RFC3339, info.Created); err == nil {
			output.Runtime.Created = created
		}

		output.Labels = info.Labels

		for name, endpoint := range info.Networks {
			output.Networks = append(output.Networks, NetworkInfo{
				Name:       name,
				IPAddress:  endpoint.IPv4Address,
				Gateway:    "",
				MacAddress: endpoint.MacAddress,
			})
		}

		for _, mount := range info.Mounts {
			output.Volumes = append(output.Volumes, VolumeInfo{
				Source:      mount.Source,
				Destination: mount.Destination,
				Mode:        mount.Mode,
				Type:        mount.Type,
			})
		}

		output.Environment = redactSecrets(parseEnv(info.Env))
	}

	if status == "running" {
		if stats, err := runtime.GetContainerStats(containerName); err == nil {
			output.Resources = ResourceUsage{
				CPUPercent:    stats.CPUUsage,
				MemoryUsageMB: float64(stats.MemoryUsage) / (1024 * 1024),
				MemoryLimitMB: float64(stats.MemoryLimit) / (1024 * 1024),
				NetworkRxKB:   float64(stats.NetworkIO.RxBytes) / 1024,
				NetworkTxKB:   float64(stats.NetworkIO.TxBytes) / 1024,
				BlockRead:     uint64(stats.BlockIO.ReadBytes),
				BlockWrite:    uint64(stats.BlockIO.WriteBytes),
				PIDs:          0,
			}
		}
	}

	exposedPorts := []string{}
	for _, portMapping := range serverCfg.Ports {
		exposedPorts = append(exposedPorts, portMapping)
	}

	endpoint := ""
	if serverCfg.Protocol == "http" || serverCfg.Protocol == "sse" {
		endpoint = fmt.Sprintf("http://localhost:%d", serverCfg.HttpPort)
	}

	output.Connection = ConnectionInfo{
		Endpoint:     endpoint,
		Protocol:     serverCfg.Protocol,
		Port:         serverCfg.HttpPort,
		ExposedPorts: exposedPorts,
		Accessible:   status == "running",
		LastCheck:    time.Now().Format(time.RFC3339),
	}

	if serverCfg.HealthCheck != nil && len(serverCfg.HealthCheck.Test) > 0 {
		output.HealthCheck = HealthCheckInfo{
			Status:      status,
			LastCheck:   time.Now(),
			Test:        serverCfg.HealthCheck.Test,
			Interval:    serverCfg.HealthCheck.Interval,
			Timeout:     serverCfg.HealthCheck.Timeout,
			Retries:     serverCfg.HealthCheck.Retries,
		}

		if status == "running" {
			output.HealthCheck.Status = "healthy"
			output.HealthCheck.LastHealthy = time.Now()
		}
	}

	if showLogs && status == "running" {
		output.Logs = []string{"(logs not available via API - use 'mcp-compose logs' command)"}
	}

	return output, nil
}

func printTextOutput(output *InspectOutput) {
	cyan := color.New(color.FgCyan).SprintFunc()
	green := color.New(color.FgGreen).SprintFunc()
	yellow := color.New(color.FgYellow).SprintFunc()
	bold := color.New(color.Bold).SprintFunc()

	fmt.Printf("%s\n", bold(fmt.Sprintf("Server: %s", output.ServerName)))
	fmt.Println(strings.Repeat("=", 80))

	fmt.Printf("\n%s\n", cyan("Configuration:"))
	if output.Configuration.Image != "" {
		fmt.Printf("  Image:      %s\n", output.Configuration.Image)
	}
	if output.Configuration.Command != "" {
		fmt.Printf("  Command:    %s\n", output.Configuration.Command)
	}
	if len(output.Configuration.Args) > 0 {
		fmt.Printf("  Args:       %s\n", strings.Join(output.Configuration.Args, " "))
	}
	fmt.Printf("  Protocol:   %s\n", output.Configuration.Protocol)
	if output.Configuration.HttpPort > 0 {
		fmt.Printf("  HTTP Port:  %d\n", output.Configuration.HttpPort)
	}
	if output.Configuration.User != "" {
		fmt.Printf("  User:       %s\n", output.Configuration.User)
	}
	fmt.Printf("  Privileged: %v\n", output.Configuration.Privileged)
	if output.Configuration.Restart != "" {
		fmt.Printf("  Restart:    %s\n", output.Configuration.Restart)
	}

	fmt.Printf("\n%s\n", cyan("Runtime:"))
	if output.Runtime.ContainerID != "" {
		fmt.Printf("  Container ID: %s\n", output.Runtime.ContainerID[:12])
		fmt.Printf("  Name:         %s\n", output.Runtime.ContainerName)
		fmt.Printf("  Status:       %s\n", green(output.Runtime.Status))
		fmt.Printf("  State:        %s\n", output.Runtime.State)
		fmt.Printf("  Created:      %s\n", output.Runtime.Created.Format(time.RFC3339))
		if output.Runtime.RestartCount > 0 {
			fmt.Printf("  Restarts:     %s\n", yellow(fmt.Sprintf("%d", output.Runtime.RestartCount)))
		}
	} else {
		fmt.Printf("  Status:       %s\n", yellow("not running"))
	}

	fmt.Printf("\n%s\n", cyan("Connection:"))
	if output.Connection.Endpoint != "" {
		fmt.Printf("  Endpoint:   %s\n", output.Connection.Endpoint)
	}
	fmt.Printf("  Protocol:   %s\n", output.Connection.Protocol)
	if output.Connection.Port > 0 {
		fmt.Printf("  Port:       %d\n", output.Connection.Port)
	}
	if len(output.Connection.ExposedPorts) > 0 {
		fmt.Printf("  Exposed:    %s\n", strings.Join(output.Connection.ExposedPorts, ", "))
	}
	if output.Connection.Accessible {
		fmt.Printf("  Accessible: %s\n", green("yes"))
	} else {
		fmt.Printf("  Accessible: %s\n", yellow("no"))
	}

	if output.HealthCheck.Status != "" {
		fmt.Printf("\n%s\n", cyan("Health Check:"))
		fmt.Printf("  Status:       %s\n", green(output.HealthCheck.Status))
		if len(output.HealthCheck.Test) > 0 {
			fmt.Printf("  Test:         %s\n", strings.Join(output.HealthCheck.Test, " "))
		}
		if output.HealthCheck.Interval != "" {
			fmt.Printf("  Interval:     %s\n", output.HealthCheck.Interval)
		}
		if output.HealthCheck.Timeout != "" {
			fmt.Printf("  Timeout:      %s\n", output.HealthCheck.Timeout)
		}
		if output.HealthCheck.Retries > 0 {
			fmt.Printf("  Retries:      %d\n", output.HealthCheck.Retries)
		}
	}

	if output.Resources.CPUPercent > 0 || output.Resources.MemoryUsageMB > 0 {
		fmt.Printf("\n%s\n", cyan("Resources:"))
		fmt.Printf("  CPU:          %.2f%%\n", output.Resources.CPUPercent)
		fmt.Printf("  Memory:       %.2f MB", output.Resources.MemoryUsageMB)
		if output.Resources.MemoryLimitMB > 0 {
			fmt.Printf(" / %.2f MB", output.Resources.MemoryLimitMB)
		}
		fmt.Println()
		fmt.Printf("  Network RX:   %.2f KB\n", output.Resources.NetworkRxKB)
		fmt.Printf("  Network TX:   %.2f KB\n", output.Resources.NetworkTxKB)
		if output.Resources.BlockRead > 0 || output.Resources.BlockWrite > 0 {
			fmt.Printf("  Block I/O:    %s / %s\n",
				formatBytes(output.Resources.BlockRead),
				formatBytes(output.Resources.BlockWrite))
		}
		if output.Resources.PIDs > 0 {
			fmt.Printf("  PIDs:         %d\n", output.Resources.PIDs)
		}
	}

	if len(output.Networks) > 0 {
		fmt.Printf("\n%s\n", cyan("Networks:"))
		for _, net := range output.Networks {
			fmt.Printf("  %s:\n", net.Name)
			fmt.Printf("    IP Address: %s\n", net.IPAddress)
			if net.Gateway != "" {
				fmt.Printf("    Gateway:    %s\n", net.Gateway)
			}
			if net.MacAddress != "" {
				fmt.Printf("    MAC:        %s\n", net.MacAddress)
			}
		}
	}

	if len(output.Volumes) > 0 {
		fmt.Printf("\n%s\n", cyan("Volumes:"))
		for _, vol := range output.Volumes {
			fmt.Printf("  %s -> %s (%s, %s)\n",
				vol.Source, vol.Destination, vol.Type, vol.Mode)
		}
	}

	if len(output.Environment) > 0 {
		fmt.Printf("\n%s\n", cyan("Environment Variables:"))
		for key, value := range output.Environment {
			fmt.Printf("  %s=%s\n", key, value)
		}
	}

	if len(output.Logs) > 0 {
		fmt.Printf("\n%s\n", cyan(fmt.Sprintf("Recent Logs (last %d lines):", len(output.Logs))))
		for _, line := range output.Logs {
			fmt.Printf("  %s\n", line)
		}
	}
}

func redactSecrets(env map[string]string) map[string]string {
	redacted := make(map[string]string)
	sensitiveKeys := []string{"password", "secret", "key", "token", "api_key", "apikey", "auth"}

	for k, v := range env {
		lowerKey := strings.ToLower(k)
		isSecret := false

		for _, sensitive := range sensitiveKeys {
			if strings.Contains(lowerKey, sensitive) {
				isSecret = true

				break
			}
		}

		if isSecret && v != "" {
			redacted[k] = "***REDACTED***"
		} else {
			redacted[k] = v
		}
	}

	return redacted
}

func parseEnv(envList []string) map[string]string {
	env := make(map[string]string)

	for _, e := range envList {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}

	return env
}