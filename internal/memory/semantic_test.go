package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVectorToFloatSlice(t *testing.T) {
	vec := FloatSliceToVector([]float32{1.0, 2.0, 3.0})
	result := VectorToFloatSlice(vec)

	assert.Equal(t, []float32{1.0, 2.0, 3.0}, result)
}

func TestFloatSliceToVector(t *testing.T) {
	floats := []float32{1.0, 2.0, 3.0}
	vec := FloatSliceToVector(floats)

	assert.NotNil(t, vec)
	assert.Equal(t, 3, len(vec.Slice()))
}

func TestSemanticSearchConfigDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
	}

	searcher := NewSemanticSearcher(db, config)

	assert.Equal(t, 0.7, searcher.similarityThreshold)
	assert.Equal(t, 10, searcher.maxResults)
	assert.Equal(t, 1536, searcher.embeddingDimension)
	assert.Equal(t, 0.4, searcher.hybridTextWeight)
	assert.Equal(t, 0.6, searcher.hybridVectorWeight)
	assert.Equal(t, "text-embedding-ada-002", searcher.embeddingModel)
}

func TestSemanticSearchConfigCustom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider:   "ollama",
		EmbeddingModel:      "llama2",
		SimilarityThreshold: 0.8,
		MaxResults:          20,
		EmbeddingDimension:  768,
		HybridTextWeight:    0.5,
		HybridVectorWeight:  0.5,
	}

	searcher := NewSemanticSearcher(db, config)

	assert.Equal(t, "ollama", searcher.embeddingProvider)
	assert.Equal(t, "llama2", searcher.embeddingModel)
	assert.Equal(t, 0.8, searcher.similarityThreshold)
	assert.Equal(t, 20, searcher.maxResults)
	assert.Equal(t, 768, searcher.embeddingDimension)
	assert.Equal(t, 0.5, searcher.hybridTextWeight)
	assert.Equal(t, 0.5, searcher.hybridVectorWeight)
}

func TestSemanticSearchCache(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)

	stats := searcher.GetCacheStats()
	assert.Equal(t, 0, stats["cache_size"])

	searcher.embeddingCache["test:key"] = FloatSliceToVector([]float32{1.0, 2.0, 3.0})

	stats = searcher.GetCacheStats()
	assert.Equal(t, 1, stats["cache_size"])
}

func TestUpdateEntityEmbeddingNotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			description TEXT,
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	err = searcher.UpdateEntityEmbedding(ctx, "00000000-0000-0000-0000-000000000000")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "entity not found")
}

func TestBatchGenerateEmbeddingsEmpty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err := searcher.BatchGenerateEmbeddings(ctx, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no texts provided")
}

func TestSearchSimilarEntitiesNoResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "pgvector";

		CREATE TABLE entities (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			name VARCHAR(255),
			type VARCHAR(100),
			description TEXT,
			metadata JSONB DEFAULT '{}',
			embedding vector(1536),
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err = searcher.SearchSimilarEntities(ctx, "test query")

	assert.Error(t, err)
}

func TestFindSimilarRelationsNoResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "pgvector";

		CREATE TABLE relations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			relation_type VARCHAR(100),
			description TEXT,
			metadata JSONB DEFAULT '{}',
			embedding vector(1536),
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err = searcher.FindSimilarRelations(ctx, "test query")

	assert.Error(t, err)
}

func TestFindSimilarObservationsNoResults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "pgvector";

		CREATE TABLE observations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			observation_type VARCHAR(100),
			content TEXT,
			metadata JSONB DEFAULT '{}',
			embedding vector(1536),
			archived BOOLEAN DEFAULT FALSE
		);
	`)
	require.NoError(t, err)

	config := SemanticSearchConfig{
		EmbeddingProvider: "openai",
		EmbeddingAPIKey:   "test-key",
	}

	searcher := NewSemanticSearcher(db, config)
	ctx := context.Background()

	_, err = searcher.FindSimilarObservations(ctx, "test query")

	assert.Error(t, err)
}