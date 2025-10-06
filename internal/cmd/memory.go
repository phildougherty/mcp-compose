// internal/cmd/memory.go
package cmd

import (
	"fmt"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/constants"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/memory"
	"github.com/phildougherty/mcp-compose/internal/output"

	"github.com/spf13/cobra"
)

func NewMemoryCommand() *cobra.Command {
	var enable bool
	var disable bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "memory",
		Short: "Manage the postgres-backed memory MCP server",
		Long: `Start, stop, enable, or disable the postgres-backed memory MCP server.
The memory server provides persistent knowledge graph storage with:
- PostgreSQL backend for reliability
- Graph-based knowledge storage
- Entity and relationship management
- Observation tracking

Examples:
  mcp-compose memory                    # Start memory server
  mcp-compose memory --enable           # Enable in config
  mcp-compose memory --disable          # Disable service
  mcp-compose memory --verbose          # Start with verbose output`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("file")
			verbose, _ := cmd.Flags().GetBool("verbose")
			output.SetVerbose(verbose)

			cfg, err := config.LoadConfig(configFile)
			if err != nil {

				return fmt.Errorf("failed to load config: %w", err)
			}

			runtime, err := container.DetectRuntime()
			if err != nil {

				return fmt.Errorf("failed to detect container runtime: %w", err)
			}

			memoryManager := memory.NewManager(cfg, runtime)
			memoryManager.SetConfigFile(configFile)

			if enable {

				return enableMemoryServer(configFile, cfg)
			}

			if disable {

				return disableMemoryServer(configFile, cfg, memoryManager)
			}

			if !cfg.Memory.Enabled {
				output.Info("Memory server is not enabled in configuration.")
				output.Info("Use --enable flag to enable it first.")

				return nil
			}

			serviceOutput := output.NewServiceOutput("memory", verbose)
			serviceOutput.Start("PostgreSQL-backed knowledge graph")

			err = memoryManager.Start()
			serviceOutput.Complete(err)

			if err == nil && verbose {
				info := map[string]string{
					"Port":     fmt.Sprintf("%d", cfg.Memory.Port),
					"Database": cfg.Memory.DatabaseURL,
					"Host":     cfg.Memory.Host,
				}
				output.PrintServiceInfo("Memory Server", info)

				endpoints := map[string]string{
					"Memory Server": fmt.Sprintf("http://%s:%d", cfg.Memory.Host, cfg.Memory.Port),
					"Health Check":  fmt.Sprintf("http://%s:%d/health", cfg.Memory.Host, cfg.Memory.Port),
				}
				output.PrintEndpoints("Available Endpoints", endpoints)
			}

			return err
		},
	}

	cmd.Flags().BoolVar(&enable, "enable", false, "Enable the memory server in config")
	cmd.Flags().BoolVar(&disable, "disable", false, "Disable the memory server")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	return cmd
}

func enableMemoryServer(configFile string, cfg *config.ComposeConfig) error {
	serviceOutput := output.NewServiceOutput("memory", false)
	serviceOutput.Start("enabling PostgreSQL-backed memory server")

	// 1. Enable in the built-in memory section
	cfg.Memory.Enabled = true
	if cfg.Memory.Port == 0 {
		cfg.Memory.Port = 3001
	}
	if cfg.Memory.Host == "" {
		cfg.Memory.Host = "0.0.0.0"
	}
	if cfg.Memory.DatabaseURL == "" {
		cfg.Memory.DatabaseURL = "postgresql://postgres:password@mcp-compose-postgres-memory:5432/memory_graph?sslmode=disable"
	}
	if !cfg.Memory.PostgresEnabled {
		cfg.Memory.PostgresEnabled = true
	}
	if cfg.Memory.PostgresPort == 0 {
		cfg.Memory.PostgresPort = 5432
	}
	if cfg.Memory.PostgresDB == "" {
		cfg.Memory.PostgresDB = "memory_graph"
	}
	if cfg.Memory.PostgresUser == "" {
		cfg.Memory.PostgresUser = "postgres"
	}
	if cfg.Memory.PostgresPassword == "" {
		cfg.Memory.PostgresPassword = "password"
	}
	if cfg.Memory.CPUs == "" {
		cfg.Memory.CPUs = "1.0"
	}
	if cfg.Memory.Memory == "" {
		cfg.Memory.Memory = "1g"
	}
	if cfg.Memory.PostgresCPUs == "" {
		cfg.Memory.PostgresCPUs = "2.0"
	}
	if cfg.Memory.PostgresMemory == "" {
		cfg.Memory.PostgresMemory = "2g"
	}
	if len(cfg.Memory.Volumes) == 0 {
		cfg.Memory.Volumes = []string{"postgres-memory-data:/var/lib/postgresql/data"}
	}
	if cfg.Memory.Authentication == nil {
		allowAPIKey := true
		cfg.Memory.Authentication = &config.ServerAuthConfig{
			Enabled:       true,
			RequiredScope: "mcp:tools",
			OptionalAuth:  false,
			AllowAPIKey:   &allowAPIKey,
		}
	}

	// 2. ALSO add to servers section for proxy discovery
	if cfg.Servers == nil {
		cfg.Servers = make(map[string]config.ServerConfig)
	}

	allowAPIKey := true

	// Add memory server to servers config (so proxy can find it)
	cfg.Servers["memory"] = config.ServerConfig{
		Build: config.BuildConfig{
			Context:    "github.com/phildougherty/mcp-compose-memory.git",
			Dockerfile: "Dockerfile",
		},
		Command:      "./mcp-compose-memory",
		Args:         []string{"--host", "0.0.0.0", "--port", "3001"},
		Protocol:     "http",
		HttpPort:     constants.DefaultMemoryHTTPPort,
		User:         "root",
		ReadOnly:     false,
		Privileged:   false,
		SecurityOpt:  []string{"no-new-privileges:true"},
		Capabilities: []string{"tools", "resources"},
		Env: map[string]string{
			"NODE_ENV":     "production",
			"DATABASE_URL": cfg.Memory.DatabaseURL,
		},
		Networks: []string{"mcp-net"},
		Authentication: &config.ServerAuthConfig{
			Enabled:       true,
			RequiredScope: "mcp:tools",
			OptionalAuth:  false,
			AllowAPIKey:   &allowAPIKey,
		},
		DependsOn: []string{"postgres-memory"},
	}

	// Add postgres-memory to servers config too
	cfg.Servers["postgres-memory"] = config.ServerConfig{
		Image:       "postgres:15-alpine",
		User:        "postgres",
		ReadOnly:    false,
		Privileged:  false,
		SecurityOpt: []string{"no-new-privileges:true"},
		Env: map[string]string{
			"POSTGRES_DB":       cfg.Memory.PostgresDB,
			"POSTGRES_USER":     cfg.Memory.PostgresUser,
			"POSTGRES_PASSWORD": cfg.Memory.PostgresPassword,
		},
		Volumes:       cfg.Memory.Volumes,
		Networks:      []string{"mcp-net"},
		RestartPolicy: "unless-stopped",
		HealthCheck: &config.HealthCheck{
			Test:        []string{"CMD-SHELL", "pg_isready -U postgres"},
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     constants.DefaultRetryCount,
			StartPeriod: "30s",
		},
	}

	err := config.SaveConfig(configFile, cfg)
	serviceOutput.Complete(err)

	if err == nil {
		output.SuccessMsg("memory", fmt.Sprintf("enabled (port: %d)", cfg.Memory.Port))
	}

	return err
}

func disableMemoryServer(configFile string, cfg *config.ComposeConfig, memoryManager *memory.Manager) error {
	serviceOutput := output.NewServiceOutput("memory", false)
	serviceOutput.Start("disabling memory server")

	if err := memoryManager.Stop(); err != nil {
		output.Verbose(fmt.Sprintf("Warning during stop: %v", err))
	}

	cfg.Memory.Enabled = false

	err := config.SaveConfig(configFile, cfg)
	serviceOutput.Complete(err)

	if err == nil {
		output.SuccessMsg("memory", "disabled")
	}

	return err
}
