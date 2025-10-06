package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type MigrationManager struct {
	db     *sql.DB
	logger *logging.Logger
}

func NewMigrationManager(db *sql.DB, logger *logging.Logger) *MigrationManager {
	return &MigrationManager{
		db:     db,
		logger: logger,
	}
}

func (m *MigrationManager) EnsureSchema(ctx context.Context) error {
	if err := m.createMigrationsTable(ctx); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	migrations, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	for _, migration := range migrations {
		if err := m.runMigration(ctx, migration); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration, err)
		}
	}

	return nil
}

func (m *MigrationManager) createMigrationsTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			id SERIAL PRIMARY KEY,
			filename VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`

	_, err := m.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	return nil
}

func (m *MigrationManager) getMigrationFiles() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrations []string
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[len(entry.Name())-4:] == ".sql" {
			migrations = append(migrations, entry.Name())
		}
	}

	sort.Strings(migrations)

	return migrations, nil
}

func (m *MigrationManager) runMigration(ctx context.Context, filename string) error {
	var count int
	err := m.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = $1", filename).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check migration status: %w", err)
	}

	if count > 0 {
		m.logger.Debug("Migration %s already applied, skipping", filename)

		return nil
	}

	sqlBytes, err := migrationsFS.ReadFile("migrations/" + filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES ($1)", filename); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record migration: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration: %w", err)
	}

	m.logger.Info("Applied migration: %s", filename)

	return nil
}

func InitializeRegistrySchema(db *sql.DB, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := NewMigrationManager(db, logger)

	return manager.EnsureSchema(ctx)
}

func EnsureRegistryTables(db *sql.DB, logger *logging.Logger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS marketplace_servers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			display_name VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			docker_image VARCHAR(500),
			npm_package VARCHAR(255),
			category VARCHAR(100) NOT NULL,
			tags TEXT[] DEFAULT '{}',
			protocol VARCHAR(50) DEFAULT 'stdio',
			capabilities TEXT[] DEFAULT '{}',
			config_template JSONB NOT NULL,
			featured BOOLEAN DEFAULT false,
			downloads INTEGER DEFAULT 0,
			rating DECIMAL(3,2) DEFAULT 0.0,
			author VARCHAR(255),
			repository_url VARCHAR(500),
			documentation_url VARCHAR(500),
			icon_url VARCHAR(500),
			version VARCHAR(50) DEFAULT '1.0.0',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS marketplace_categories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			display_name VARCHAR(255) NOT NULL,
			description TEXT,
			icon VARCHAR(100),
			sort_order INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS user_installed_servers (
			id SERIAL PRIMARY KEY,
			server_id INTEGER REFERENCES marketplace_servers(id) ON DELETE CASCADE,
			user_id VARCHAR(255) DEFAULT 'default',
			server_name VARCHAR(255) NOT NULL,
			installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			config JSONB,
			status VARCHAR(50) DEFAULT 'active',
			UNIQUE(user_id, server_name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_marketplace_servers_category ON marketplace_servers(category)`,
		`CREATE INDEX IF NOT EXISTS idx_marketplace_servers_featured ON marketplace_servers(featured)`,
		`CREATE INDEX IF NOT EXISTS idx_user_installed_servers_user ON user_installed_servers(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_installed_servers_status ON user_installed_servers(status)`,
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to create table/index: %w", err)
		}
	}

	logger.Info("Registry tables and indexes ensured")

	return nil
}
