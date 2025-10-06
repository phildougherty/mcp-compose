// internal/memory/manager.go
package memory

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	_ "github.com/lib/pq"
)

type Manager struct {
	cfg             *config.ComposeConfig
	runtime         container.Runtime
	configFile      string
	db              *sql.DB
	semanticSearch  *SemanticSearcher
	pruner          *MemoryPruner
	pruningSchedule *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewManager(cfg *config.ComposeConfig, runtime container.Runtime) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		cfg:     cfg,
		runtime: runtime,
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *Manager) SetConfigFile(configFile string) {
	m.configFile = configFile
}

func (m *Manager) Start() error {

	// Get PostgreSQL password from config or environment first
	pgPassword := "password"
	if m.cfg.Memory.PostgresPassword != "" {
		pgPassword = m.cfg.Memory.PostgresPassword
	}
	// Also check environment variable directly
	if envPassword := os.Getenv("POSTGRES_PASSWORD"); envPassword != "" {
		pgPassword = envPassword
	}

	// Check if postgres-memory is running first
	postgresStatus, err := m.runtime.GetContainerStatus("mcp-compose-postgres-memory")
	if err != nil || postgresStatus != "running" {
		if err := m.startPostgres(pgPassword); err != nil {

			return fmt.Errorf("failed to start postgres-memory: %w", err)
		}
	}

	// Build memory server image
	if err := m.buildMemoryImage(); err != nil {

		return fmt.Errorf("failed to build memory image: %w", err)
	}

	// Stop existing container
	_ = m.runtime.StopContainer("mcp-compose-memory")

	// Ensure network exists
	networkExists, _ := m.runtime.NetworkExists("mcp-net")
	if !networkExists {
		if err := m.runtime.CreateNetwork("mcp-net"); err != nil {

			return fmt.Errorf("failed to create mcp-net network: %w", err)
		}
	}

	// Get configuration values with defaults
	dbURL := fmt.Sprintf("postgresql://postgres:%s@mcp-compose-postgres-memory:5432/memory_graph?sslmode=disable", pgPassword)
	if m.cfg.Memory.DatabaseURL != "" {
		dbURL = m.cfg.Memory.DatabaseURL
		// Ensure sslmode=disable is included if not present
		if !strings.Contains(dbURL, "sslmode=") {
			if strings.Contains(dbURL, "?") {
				dbURL += "&sslmode=disable"
			} else {
				dbURL += "?sslmode=disable"
			}
		}
	}

	// Get CPU and memory limits with defaults
	cpus := "1.0"
	if m.cfg.Memory.CPUs != "" {
		cpus = m.cfg.Memory.CPUs
	}

	memory := "1g"
	if m.cfg.Memory.Memory != "" {
		memory = m.cfg.Memory.Memory
	}

	// Start memory server
	opts := &container.ContainerOptions{
		Name:     "mcp-compose-memory",
		Image:    "mcp-compose-memory:latest",
		Ports:    []string{"3001:3001"},
		Networks: []string{"mcp-net"},
		Env: map[string]string{
			"NODE_ENV":          "production",
			"DATABASE_URL":      dbURL,
			"POSTGRES_PASSWORD": pgPassword,
		},
		User:        "root",
		CPUs:        cpus,
		Memory:      memory,
		DNSSearch:   []string{"."},
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "memory",
		},
		RestartPolicy: "unless-stopped",
	}

	_, err = m.runtime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start memory container: %w", err)
	}

	return nil
}

func (m *Manager) startPostgres(pgPassword string) error {

	// Get postgres configuration with defaults
	pgCpus := "2.0"
	if m.cfg.Memory.PostgresCPUs != "" {
		pgCpus = m.cfg.Memory.PostgresCPUs
	}

	pgMemory := "2g"
	if m.cfg.Memory.PostgresMemory != "" {
		pgMemory = m.cfg.Memory.PostgresMemory
	}

	pgDB := "memory_graph"
	if m.cfg.Memory.PostgresDB != "" {
		pgDB = m.cfg.Memory.PostgresDB
	}

	pgUser := "postgres"
	if m.cfg.Memory.PostgresUser != "" {
		pgUser = m.cfg.Memory.PostgresUser
	}

	// Get volumes with defaults
	volumes := []string{"postgres-memory-data:/var/lib/postgresql/data"}
	if len(m.cfg.Memory.Volumes) > 0 {
		volumes = m.cfg.Memory.Volumes
	}

	opts := &container.ContainerOptions{
		Name:     "mcp-compose-postgres-memory",
		Image:    "postgres:15-alpine",
		Networks: []string{"mcp-net"},
		Env: map[string]string{
			"POSTGRES_DB":       pgDB,
			"POSTGRES_USER":     pgUser,
			"POSTGRES_PASSWORD": pgPassword,
		},
		Volumes:     volumes,
		User:        "postgres",
		CPUs:        pgCpus,
		Memory:      pgMemory,
		DNSSearch:   []string{"."},
		SecurityOpt: []string{"no-new-privileges:true"},
		Labels: map[string]string{
			"mcp-compose.system": "true",
			"mcp-compose.role":   "database",
		},
		RestartPolicy: "unless-stopped",
	}

	_, err := m.runtime.StartContainer(opts)
	if err != nil {

		return fmt.Errorf("failed to start postgres container: %w", err)
	}

	return nil
}

func (m *Manager) buildMemoryImage() error {
	dockerfilePath := "dockerfiles/Dockerfile.memory-go"

	// Check if Dockerfile exists
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {

		return fmt.Errorf("dockerfile not found at %s", dockerfilePath)
	}

	// Build with no cache to force fresh download of git repo
	buildCmd := exec.Command("docker", "build", "--no-cache", "-f", dockerfilePath, "-t", "mcp-compose-memory:latest", ".")
	buildCmd.Stdout = nil
	buildCmd.Stderr = nil

	if err := buildCmd.Run(); err != nil {

		return fmt.Errorf("failed to build Go memory image: %w", err)
	}

	return nil
}

func (m *Manager) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.pruningSchedule != nil {
		m.pruningSchedule.Stop()
	}

	if m.db != nil {
		_ = m.db.Close()
	}

	_ = m.runtime.StopContainer("mcp-compose-memory")
	_ = m.runtime.StopContainer("mcp-compose-postgres-memory")

	return nil
}

func (m *Manager) Restart() error {
	_ = m.Stop()

	return m.Start()
}

func (m *Manager) Status() (string, error) {

	return m.runtime.GetContainerStatus("mcp-compose-memory")
}

func (m *Manager) ConnectDatabase() error {
	pgPassword := "password"
	if m.cfg.Memory.PostgresPassword != "" {
		pgPassword = m.cfg.Memory.PostgresPassword
	}
	if envPassword := os.Getenv("POSTGRES_PASSWORD"); envPassword != "" {
		pgPassword = envPassword
	}

	dbURL := fmt.Sprintf("postgresql://postgres:%s@localhost:5432/memory_graph?sslmode=disable", pgPassword)
	if m.cfg.Memory.DatabaseURL != "" {
		dbURL = m.cfg.Memory.DatabaseURL
		if !strings.Contains(dbURL, "sslmode=") {
			if strings.Contains(dbURL, "?") {
				dbURL += "&sslmode=disable"
			} else {
				dbURL += "?sslmode=disable"
			}
		}
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()

		return fmt.Errorf("failed to ping database: %w", err)
	}

	m.db = db

	return nil
}

func (m *Manager) InitializeSchema() error {
	if m.db == nil {
		return fmt.Errorf("database connection not established")
	}

	schemaSQL, err := os.ReadFile("internal/memory/schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	if _, err := m.db.ExecContext(ctx, string(schemaSQL)); err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}

	return nil
}

func (m *Manager) InitializeSemanticSearch(embeddingProvider, embeddingAPIKey, embeddingModel string) error {
	if m.db == nil {
		return fmt.Errorf("database connection not established")
	}

	config := SemanticSearchConfig{
		EmbeddingProvider:   embeddingProvider,
		EmbeddingAPIKey:     embeddingAPIKey,
		EmbeddingModel:      embeddingModel,
		SimilarityThreshold: 0.7,
		MaxResults:          10,
		EmbeddingDimension:  1536,
		HybridTextWeight:    0.4,
		HybridVectorWeight:  0.6,
	}

	m.semanticSearch = NewSemanticSearcher(m.db, config)

	return nil
}

func (m *Manager) InitializePruning(strategy PruningStrategy, retentionDays int, minImportance float64) error {
	if m.db == nil {
		return fmt.Errorf("database connection not established")
	}

	config := PruningConfig{
		Enabled:                true,
		Strategy:               strategy,
		RetentionDays:          retentionDays,
		MinImportanceScore:     minImportance,
		MaxMemories:            100000,
		ArchiveBeforeDelete:    true,
		DryRun:                 false,
		PruneEntities:          true,
		PruneRelations:         true,
		PruneObservations:      true,
		LowAccessThreshold:     5,
		ImportanceDecayFactor:  0.01,
		AgeDecayDays:           30,
	}

	m.pruner = NewMemoryPruner(m.db, config)

	return nil
}

func (m *Manager) StartPruningSchedule() error {
	if m.pruner == nil {
		return fmt.Errorf("pruner not initialized")
	}

	m.pruningSchedule = time.NewTicker(24 * time.Hour)

	go func() {
		for {
			select {
			case <-m.pruningSchedule.C:
				_, _ = m.pruner.Prune(m.ctx)
			case <-m.ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (m *Manager) SearchSimilar(ctx context.Context, query string) ([]SearchResult, error) {
	if m.semanticSearch == nil {
		return nil, fmt.Errorf("semantic search not initialized")
	}

	return m.semanticSearch.SearchSimilarEntities(ctx, query)
}

func (m *Manager) HybridSearch(ctx context.Context, query string) ([]SearchResult, error) {
	if m.semanticSearch == nil {
		return nil, fmt.Errorf("semantic search not initialized")
	}

	return m.semanticSearch.HybridSearch(ctx, query)
}

func (m *Manager) RunPruning(ctx context.Context) (*PruningResult, error) {
	if m.pruner == nil {
		return nil, fmt.Errorf("pruner not initialized")
	}

	return m.pruner.Prune(ctx)
}

func (m *Manager) GetMemoryStats(ctx context.Context) (map[string]interface{}, error) {
	if m.pruner == nil {
		return nil, fmt.Errorf("pruner not initialized")
	}

	return m.pruner.GetMemoryStats(ctx)
}

func (m *Manager) GetPruningHistory(ctx context.Context, limit int) ([]PruningResult, error) {
	if m.pruner == nil {
		return nil, fmt.Errorf("pruner not initialized")
	}

	return m.pruner.GetPruningHistory(ctx, limit)
}

func (m *Manager) StoreEntity(ctx context.Context, name, entityType, description string, metadata map[string]interface{}, importance float64) (string, error) {
	if m.semanticSearch == nil {
		return "", fmt.Errorf("semantic search not initialized")
	}

	return m.semanticSearch.StoreEntityWithEmbedding(ctx, name, entityType, description, metadata, importance)
}

func (m *Manager) UpdateImportanceScores(ctx context.Context) error {
	if m.pruner == nil {
		return fmt.Errorf("pruner not initialized")
	}

	return m.pruner.UpdateImportanceScores(ctx)
}

func (m *Manager) CleanupArchive(ctx context.Context, daysOld int) error {
	if m.pruner == nil {
		return fmt.Errorf("pruner not initialized")
	}

	return m.pruner.CleanupArchive(ctx, daysOld)
}

func (m *Manager) GetSemanticSearchStats() map[string]interface{} {
	if m.semanticSearch == nil {
		return map[string]interface{}{"error": "semantic search not initialized"}
	}

	return m.semanticSearch.GetCacheStats()
}
