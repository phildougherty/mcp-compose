package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/memory"
	"github.com/phildougherty/mcp-compose/internal/output"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"

	_ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

func NewSystemUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [SERVICE...]",
		Short: "Start system services",
		Long: `Start one or more system services.

Available system services:
  proxy           HTTP proxy server
  dashboard       Web dashboard
  task-scheduler  Task scheduler
  memory          Memory server
  migrations      Database migrations

Examples:
  mcp-compose system up                    # Start all system services
  mcp-compose system up proxy dashboard    # Start specific services`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			verbose, _ := cmd.Flags().GetBool("verbose")
			output.SetVerbose(verbose)

			if len(args) == 0 {
				return startAllSystemServices(file, verbose)
			}

			for _, service := range args {
				if !IsSystemService(service) {
					return fmt.Errorf("'%s' is not a system service. Use 'mcp-compose up %s' for user services", service, service)
				}
			}

			return startSystemServices(file, args, verbose)
		},
	}

	cmd.Flags().Bool("verbose", false, "Enable verbose output")

	return cmd
}

func startAllSystemServices(configFile string, verbose bool) error {
	allServices := []string{"proxy", "memory", "dashboard", "migrations", "task-scheduler"}

	return startSystemServices(configFile, allServices, verbose)
}

func startSystemServices(configFile string, services []string, verbose bool) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	successCount := 0
	totalCount := len(services)

	for _, service := range services {
		startTime := time.Now()
		serviceOutput := output.NewServiceOutput(service, verbose)

		switch service {
		case "proxy":
			output.Info("Note: Use 'mcp-compose proxy' to start the proxy with custom options")
			serviceOutput.CompleteWithMessage(true, "skipped (use 'mcp-compose proxy')")
		case "memory":
			serviceOutput.Start("memory server")
			memoryManager := memory.NewManager(cfg, runtime)
			memoryManager.SetConfigFile(configFile)
			err := memoryManager.Start()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "migrations":
			serviceOutput.Start("database migrations")
			err := runDatabaseMigrations(cfg, verbose)
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "task-scheduler":
			serviceOutput.Start("task scheduler")
			taskManager := task_scheduler.NewManager(cfg, runtime)
			taskManager.SetConfigFile(configFile)
			err := taskManager.Start()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		case "dashboard":
			serviceOutput.Start("web dashboard")
			dashManager := dashboard.NewManager(cfg, runtime)
			dashManager.SetConfigFile(configFile)
			err := dashManager.Start()
			serviceOutput.Complete(err)
			if err == nil {
				successCount++
			}
		default:
			return fmt.Errorf("unknown system service: %s", service)
		}

		if verbose && service != "proxy" {
			output.Verbose(fmt.Sprintf("Duration: %s", output.ShortDuration(time.Since(startTime))))
		}
	}

	output.Summary(successCount, totalCount)

	return nil
}

func runDatabaseMigrations(cfg *config.ComposeConfig, verbose bool) error {
	var postgresURL string

	if cfg.Dashboard.PostgresURL != "" {
		postgresURL = cfg.Dashboard.PostgresURL
	} else if cfg.TaskScheduler.PostgresURL != "" {
		postgresURL = cfg.TaskScheduler.PostgresURL
	} else if cfg.Memory.PostgresEnabled && cfg.Memory.DatabaseURL != "" {
		postgresURL = cfg.Memory.DatabaseURL
	} else if cfg.Memory.PostgresEnabled {
		postgresURL = fmt.Sprintf(
			"postgresql://%s:%s@mcp-compose-postgres-memory:%d/%s?sslmode=disable",
			cfg.Memory.PostgresUser,
			cfg.Memory.PostgresPassword,
			cfg.Memory.PostgresPort,
			cfg.Memory.PostgresDB,
		)
	}

	if postgresURL == "" {
		if verbose {
			output.Verbose("No PostgreSQL database configured, skipping migrations")
		}

		return nil
	}

	dbName := "mcp_compose"
	if !strings.Contains(postgresURL, "/mcp_compose") {
		parts := strings.Split(postgresURL, "/")
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			dbNameQuery := strings.Split(lastPart, "?")
			if len(dbNameQuery) > 0 && dbNameQuery[0] != "" {
				dbName = dbNameQuery[0]
			}
		}
	}

	postgresURL = strings.Replace(postgresURL, "/"+dbName, "/mcp_compose", 1)

	db, err := sql.Open("postgres", postgresURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrationFiles := []string{
		"001_create_marketplace_tables.sql",
		"002_seed_marketplace_servers.sql",
		"002.5_create_chat_tables.sql",
		"003_create_scheduler_schema.sql",
		"004_enhance_chat_sessions.sql",
	}

	for _, filename := range migrationFiles {
		var exists bool
		err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check migration status for %s: %w", filename, err)
		}

		if exists {
			if verbose {
				output.Verbose(fmt.Sprintf("Migration %s already applied, skipping", filename))
			}

			continue
		}

		migrationPath := filepath.Join("internal", "database", "migrations", filename)
		sqlBytes, err := os.ReadFile(migrationPath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filename, err)
		}

		if _, err := db.Exec(string(sqlBytes)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		if _, err := db.Exec("INSERT INTO schema_migrations (version) VALUES ($1)", filename); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		if verbose {
			output.Verbose(fmt.Sprintf("Applied migration: %s", filename))
		}
	}

	return nil
}