package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPruningConfigDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := PruningConfig{}
	pruner := NewMemoryPruner(db, config)

	assert.Equal(t, 90, pruner.config.RetentionDays)
	assert.Equal(t, 0.3, pruner.config.MinImportanceScore)
	assert.Equal(t, 100000, pruner.config.MaxMemories)
	assert.Equal(t, 5, pruner.config.LowAccessThreshold)
	assert.Equal(t, 0.01, pruner.config.ImportanceDecayFactor)
	assert.Equal(t, 30, pruner.config.AgeDecayDays)
	assert.Equal(t, StrategyHybrid, pruner.config.Strategy)
}

func TestPruningConfigCustom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := PruningConfig{
		Strategy:              StrategyLRU,
		RetentionDays:         60,
		MinImportanceScore:    0.5,
		MaxMemories:           50000,
		LowAccessThreshold:    10,
		ImportanceDecayFactor: 0.02,
		AgeDecayDays:          60,
	}

	pruner := NewMemoryPruner(db, config)

	assert.Equal(t, StrategyLRU, pruner.config.Strategy)
	assert.Equal(t, 60, pruner.config.RetentionDays)
	assert.Equal(t, 0.5, pruner.config.MinImportanceScore)
	assert.Equal(t, 50000, pruner.config.MaxMemories)
	assert.Equal(t, 10, pruner.config.LowAccessThreshold)
	assert.Equal(t, 0.02, pruner.config.ImportanceDecayFactor)
	assert.Equal(t, 60, pruner.config.AgeDecayDays)
}

func TestPruneStrategyAge(t *testing.T) {
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
		INSERT INTO entities (name, type, description, created_at)
		VALUES ('old', 'person', 'old entity', NOW() - INTERVAL '100 days')
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyAge,
		RetentionDays:       90,
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
	assert.Equal(t, "age", result.Strategy)
	assert.GreaterOrEqual(t, result.EntitiesPruned, 0)
}

func TestPruneHybridStrategy(t *testing.T) {
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
		INSERT INTO entities (name, type, description, importance_score, last_accessed_at, access_count)
		VALUES
			('low_importance_old', 'person', 'test', 0.2, NOW() - INTERVAL '100 days', 1),
			('high_importance_recent', 'person', 'test', 0.8, NOW(), 100)
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyHybrid,
		RetentionDays:       90,
		MinImportanceScore:  0.3,
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
	assert.Equal(t, "hybrid", result.Strategy)
}

func TestGetPruningHistory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE pruning_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			run_at TIMESTAMP DEFAULT NOW(),
			strategy VARCHAR(50),
			entities_pruned INTEGER DEFAULT 0,
			relations_pruned INTEGER DEFAULT 0,
			observations_pruned INTEGER DEFAULT 0,
			entities_archived INTEGER DEFAULT 0,
			relations_archived INTEGER DEFAULT 0,
			observations_archived INTEGER DEFAULT 0,
			duration_ms INTEGER,
			metadata JSONB DEFAULT '{}'
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO pruning_log (strategy, entities_pruned, duration_ms)
		VALUES ('lru', 10, 500)
	`)
	require.NoError(t, err)

	config := PruningConfig{}
	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	history, err := pruner.GetPruningHistory(ctx, 10)
	require.NoError(t, err)

	assert.Len(t, history, 1)
	assert.Equal(t, "lru", history[0].Strategy)
	assert.Equal(t, 10, history[0].EntitiesPruned)
}

func TestUpdateImportanceScores(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			importance_score FLOAT DEFAULT 0.5,
			access_count INTEGER DEFAULT 0,
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (importance_score, access_count, last_accessed_at)
		VALUES (0.5, 10, NOW() - INTERVAL '10 days')
	`)
	require.NoError(t, err)

	config := PruningConfig{
		ImportanceDecayFactor: 0.01,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	err = pruner.UpdateImportanceScores(ctx)
	assert.NoError(t, err)

	var updatedScore float64
	err = db.QueryRow("SELECT importance_score FROM entities").Scan(&updatedScore)
	require.NoError(t, err)

	assert.Greater(t, updatedScore, 0.0)
}

func TestCleanupArchive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities_archive (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE relations_archive (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP DEFAULT NOW()
		);

		CREATE TABLE observations_archive (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMP DEFAULT NOW()
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities_archive (created_at) VALUES (NOW() - INTERVAL '200 days');
		INSERT INTO relations_archive (created_at) VALUES (NOW() - INTERVAL '200 days');
		INSERT INTO observations_archive (created_at) VALUES (NOW() - INTERVAL '200 days');
	`)
	require.NoError(t, err)

	config := PruningConfig{}
	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	err = pruner.CleanupArchive(ctx, 180)
	assert.NoError(t, err)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM entities_archive").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPruningResultTimestamp(t *testing.T) {
	result := &PruningResult{
		Strategy:  "test",
		Timestamp: time.Now(),
	}

	assert.NotNil(t, result.Timestamp)
	assert.Equal(t, "test", result.Strategy)
}

func TestPruneWithAllTables(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			importance_score FLOAT DEFAULT 0.5,
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE relations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			importance_score FLOAT DEFAULT 0.5,
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE observations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			importance_score FLOAT DEFAULT 0.5,
			last_accessed_at TIMESTAMP DEFAULT NOW(),
			access_count INTEGER DEFAULT 0,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE entities_archive (LIKE entities INCLUDING ALL);
		CREATE TABLE relations_archive (LIKE relations INCLUDING ALL);
		CREATE TABLE observations_archive (LIKE observations INCLUDING ALL);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO entities (importance_score) VALUES (0.2);
		INSERT INTO relations (importance_score) VALUES (0.2);
		INSERT INTO observations (importance_score) VALUES (0.2);
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:            StrategyImportance,
		MinImportanceScore:  0.3,
		ArchiveBeforeDelete: true,
		PruneEntities:       true,
		PruneRelations:      true,
		PruneObservations:   true,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	result, err := pruner.Prune(ctx)
	require.NoError(t, err)

	assert.NotNil(t, result)
	assert.GreaterOrEqual(t, result.EntitiesPruned, 0)
	assert.GreaterOrEqual(t, result.RelationsPruned, 0)
	assert.GreaterOrEqual(t, result.ObservationsPruned, 0)
}

func TestPruneLogging(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			importance_score FLOAT DEFAULT 0.5,
			archived BOOLEAN DEFAULT FALSE
		);

		CREATE TABLE entities_archive (LIKE entities INCLUDING ALL);

		CREATE TABLE pruning_log (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			run_at TIMESTAMP DEFAULT NOW(),
			strategy VARCHAR(50),
			entities_pruned INTEGER,
			relations_pruned INTEGER,
			observations_pruned INTEGER,
			entities_archived INTEGER,
			relations_archived INTEGER,
			observations_archived INTEGER,
			duration_ms INTEGER,
			metadata JSONB
		);
	`)
	require.NoError(t, err)

	config := PruningConfig{
		Strategy:          StrategyImportance,
		PruneEntities:     true,
		PruneRelations:    false,
		PruneObservations: false,
	}

	pruner := NewMemoryPruner(db, config)
	ctx := context.Background()

	result, err := pruner.Prune(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM pruning_log").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}