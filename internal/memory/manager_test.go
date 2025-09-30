package memory

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := "postgres://postgres:password@localhost:5432/memory_graph_test?sslmode=disable"
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Skipf("Skipping test: PostgreSQL not available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		t.Skipf("Skipping test: PostgreSQL not available: %v", err)
	}

	_, err = db.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
	require.NoError(t, err)

	return db
}

func TestNewManager(t *testing.T) {
	cfg := &config.ComposeConfig{
		Memory: config.MemoryConfig{
			Enabled: true,
		},
	}

	manager := NewManager(cfg, nil)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.ctx)
	assert.NotNil(t, manager.cancel)
	assert.Equal(t, cfg, manager.cfg)
}

func TestManagerSetConfigFile(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)

	configFile := "/path/to/config.yaml"
	manager.SetConfigFile(configFile)

	assert.Equal(t, configFile, manager.configFile)
}

func TestManagerConnectDatabase(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.ComposeConfig{
		Memory: config.MemoryConfig{
			DatabaseURL:      "postgres://postgres:password@localhost:5432/memory_graph_test?sslmode=disable",
			PostgresPassword: "password",
		},
	}

	manager := NewManager(cfg, nil)
	defer manager.Stop()

	err := manager.ConnectDatabase()
	require.NoError(t, err)

	assert.NotNil(t, manager.db)

	err = manager.db.Ping()
	assert.NoError(t, err)
}

func TestManagerInitializeSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.ComposeConfig{
		Memory: config.MemoryConfig{
			DatabaseURL: "postgres://postgres:password@localhost:5432/memory_graph_test?sslmode=disable",
		},
	}

	manager := NewManager(cfg, nil)
	defer manager.Stop()

	err := manager.ConnectDatabase()
	require.NoError(t, err)

	err = manager.InitializeSchema()
	assert.Error(t, err)
}

func TestManagerInitializeSemanticSearch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.ComposeConfig{
		Memory: config.MemoryConfig{
			DatabaseURL: "postgres://postgres:password@localhost:5432/memory_graph_test?sslmode=disable",
		},
	}

	manager := NewManager(cfg, nil)
	defer manager.Stop()

	err := manager.ConnectDatabase()
	require.NoError(t, err)

	err = manager.InitializeSemanticSearch("openai", "test-key", "text-embedding-ada-002")
	assert.NoError(t, err)

	assert.NotNil(t, manager.semanticSearch)
}

func TestManagerInitializePruning(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	cfg := &config.ComposeConfig{
		Memory: config.MemoryConfig{
			DatabaseURL: "postgres://postgres:password@localhost:5432/memory_graph_test?sslmode=disable",
		},
	}

	manager := NewManager(cfg, nil)
	defer manager.Stop()

	err := manager.ConnectDatabase()
	require.NoError(t, err)

	err = manager.InitializePruning(StrategyHybrid, 90, 0.3)
	assert.NoError(t, err)

	assert.NotNil(t, manager.pruner)
}

func TestManagerSearchSimilarNotInitialized(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)
	defer manager.Stop()

	ctx := context.Background()
	_, err := manager.SearchSimilar(ctx, "test query")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "semantic search not initialized")
}

func TestManagerHybridSearchNotInitialized(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)
	defer manager.Stop()

	ctx := context.Background()
	_, err := manager.HybridSearch(ctx, "test query")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "semantic search not initialized")
}

func TestManagerRunPruningNotInitialized(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)
	defer manager.Stop()

	ctx := context.Background()
	_, err := manager.RunPruning(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pruner not initialized")
}

func TestManagerGetMemoryStatsNotInitialized(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)
	defer manager.Stop()

	ctx := context.Background()
	_, err := manager.GetMemoryStats(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pruner not initialized")
}

func TestManagerStop(t *testing.T) {
	cfg := &config.ComposeConfig{}
	manager := NewManager(cfg, nil)

	err := manager.Stop()
	assert.NoError(t, err)
}

func TestSemanticSearcherNewSemanticSearcher(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)

	assert.NotNil(t, searcher)
	assert.Equal(t, 0.7, searcher.similarityThreshold)
	assert.Equal(t, 10, searcher.maxResults)
	assert.Equal(t, 1536, searcher.embeddingDimension)
}

func TestSemanticSearcherGenerateEmbeddingEmptyText(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err := searcher.GenerateEmbedding(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "text cannot be empty")
}

func TestSemanticSearcherGenerateEmbeddingUnsupportedProvider(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "unsupported",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err := searcher.GenerateEmbedding(ctx, "test text")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported embedding provider")
}

func TestSemanticSearcherGetCacheStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)

	stats := searcher.GetCacheStats()

	assert.NotNil(t, stats)
	assert.Contains(t, stats, "cache_size")
	assert.Contains(t, stats, "similarity_threshold")
	assert.Contains(t, stats, "max_results")
}

func TestMemoryPrunerNewMemoryPruner(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := PruningConfig{
		Strategy: StrategyHybrid,
	}

	pruner := NewMemoryPruner(db, config)

	assert.NotNil(t, pruner)
	assert.Equal(t, 90, pruner.config.RetentionDays)
	assert.Equal(t, 0.3, pruner.config.MinImportanceScore)
	assert.Equal(t, StrategyHybrid, pruner.config.Strategy)
}

func TestMemoryPrunerUnsupportedStrategy(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := PruningConfig{
		Strategy: PruningStrategy("unsupported"),
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	_, err := pruner.Prune(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported pruning strategy")
}

func TestPruningStrategyLRU(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255),
			type VARCHAR(100),
			description TEXT,
			importance_score FLOAT DEFAULT 0.5,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE entities_archive (LIKE entities INCLUDING ALL);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (name, type, description, last_accessed_at, access_count)
		VALUES ('test', 'person', 'test description', NOW() - INTERVAL '100 days', 1)
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyLRU,
		RetentionDays:       90,
		LowAccessThreshold:  5,
		ArchiveBeforeDelete: true,
		PruneEntities:       true,
		PruneRelations:      false,
		PruneObservations:   false,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	result, err := pruner.Prune(ctx)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, "lru", result.Strategy)
}

func TestPruningStrategyImportance(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255),
			type VARCHAR(100),
			description TEXT,
			importance_score FLOAT DEFAULT 0.5,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE entities_archive (LIKE entities INCLUDING ALL);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (name, type, description, importance_score)
		VALUES ('test', 'person', 'test description', 0.2)
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyImportance,
		MinImportanceScore:  0.3,
		ArchiveBeforeDelete: true,
		PruneEntities:       true,
		PruneRelations:      false,
		PruneObservations:   false,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	result, err := pruner.Prune(ctx)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.Equal(t, "importance", result.Strategy)
}

func TestPruningDryRun(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255),
			type VARCHAR(100),
			description TEXT,
			importance_score FLOAT DEFAULT 0.5,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE entities_archive (LIKE entities INCLUDING ALL);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (name, type, description, importance_score)
		VALUES ('test', 'person', 'test description', 0.2)
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyImportance,
		MinImportanceScore:  0.3,
		ArchiveBeforeDelete: true,
		DryRun:              true,
		PruneEntities:       true,
		PruneRelations:      false,
		PruneObservations:   false,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	result, err := pruner.Prune(ctx)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.True(t, result.DryRun)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM entities WHERE archived = FALSE").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetMemoryStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE relations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE observations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (archived) VALUES (FALSE), (FALSE), (TRUE);
		INSERT INTO relations (archived) VALUES (FALSE), (TRUE);
		INSERT INTO observations (archived) VALUES (FALSE);
	`)
	require.NoError(t, err)

	config := PruningConfig{}
	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	stats, err := pruner.GetMemoryStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, stats["total_entities"])
	assert.Equal(t, 1, stats["total_relations"])
	assert.Equal(t, 1, stats["total_observations"])
	assert.Equal(t, 1, stats["archived_entities"])
	assert.Equal(t, 1, stats["archived_relations"])
	assert.Equal(t, 0, stats["archived_observations"])
}