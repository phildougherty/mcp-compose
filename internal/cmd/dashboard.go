package cmd

import (
	"fmt"
	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
	"mcpcompose/internal/dashboard"

	"github.com/spf13/cobra"
)

func NewDashboardCommand() *cobra.Command {
	var port int
	var host string
	var enable bool
	var disable bool
	var native bool

	cmd := &cobra.Command{
		Use:   "dashboard",
		Short: "Manage the web dashboard",
		Long:  "Start, stop, enable, or disable the MCP-Compose web dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("file")
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			runtime, err := container.DetectRuntime()
			if err != nil {
				return fmt.Errorf("failed to detect container runtime: %w", err)
			}

			if enable {
				cfg.Dashboard.Enabled = true
				return config.SaveConfig(configFile, cfg)
			}

			if disable {
				cfg.Dashboard.Enabled = false
				dashManager := dashboard.NewManager(cfg, runtime)
				dashManager.SetConfigFile(configFile) // Set config file path
				if err := dashManager.Stop(); err != nil {
					fmt.Printf("Warning: %v\n", err)
				}
				return config.SaveConfig(configFile, cfg)
			}

			// Override config with CLI flags if provided
			if port > 0 {
				cfg.Dashboard.Port = port
			}
			if host != "" {
				cfg.Dashboard.Host = host
			}

			// Set defaults
			if cfg.Dashboard.Port == 0 {
				cfg.Dashboard.Port = 3001
			}
			if cfg.Dashboard.Host == "" {
				cfg.Dashboard.Host = "0.0.0.0"
			}

			// Choose mode: native or containerized
			if native {
				return runNativeDashboard(cfg, runtime)
			} else {
				return runContainerizedDashboard(cfg, runtime, configFile) // Pass configFile
			}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 0, "Dashboard port (default: 3001)")
	cmd.Flags().StringVar(&host, "host", "", "Dashboard host interface (default: 0.0.0.0)")
	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the dashboard in config")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the dashboard")
	cmd.Flags().BoolVar(&native, "native", false, "Run dashboard natively (requires proxy to be native too)")

	return cmd
}

func runNativeDashboard(cfg *config.ComposeConfig, runtime container.Runtime) error {
	// For native mode, proxy must be reachable at localhost
	proxyURL := "http://localhost:9876"

	fmt.Printf("Starting native dashboard on http://%s:%d\n", cfg.Dashboard.Host, cfg.Dashboard.Port)
	fmt.Printf("Connecting to native proxy at: %s\n", proxyURL)

	server := dashboard.NewDashboardServer(cfg, runtime, proxyURL, cfg.ProxyAuth.APIKey)
	return server.Start(cfg.Dashboard.Port, cfg.Dashboard.Host)
}

func runContainerizedDashboard(cfg *config.ComposeConfig, runtime container.Runtime, configFile string) error {
	// Check if proxy container is running
	if !isProxyContainerRunning(runtime) {
		return fmt.Errorf(`
Dashboard requires the proxy container to be running.
Please start the proxy with: 
    mcp-compose proxy --port 9876 --api-key %s --container
Then try starting the dashboard again`, cfg.ProxyAuth.APIKey)
	}

	dashManager := dashboard.NewManager(cfg, runtime)
	dashManager.SetConfigFile(configFile) // Set the config file path
	return dashManager.Start()
}

func isProxyContainerRunning(runtime container.Runtime) bool {
	status, err := runtime.GetContainerStatus("mcp-compose-http-proxy")
	if err != nil {
		return false
	}
	return status == "running"
}
