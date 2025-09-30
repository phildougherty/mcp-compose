package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

type SemanticSearcher struct {
	db                    *sql.DB
	embeddingProvider     string
	embeddingAPIKey       string
	embeddingModel        string
	similarityThreshold   float64
	maxResults            int
	httpClient            *http.Client
	embeddingCache        map[string]pgvector.Vector
	cacheTTL              time.Duration
	cacheLastCleanup      time.Time
	embeddingDimension    int
	hybridTextWeight      float64
	hybridVectorWeight    float64
}

type SemanticSearchConfig struct {
	EmbeddingProvider   string
	EmbeddingAPIKey     string
	EmbeddingModel      string
	SimilarityThreshold float64
	MaxResults          int
	EmbeddingDimension  int
	HybridTextWeight    float64
	HybridVectorWeight  float64
}

type SearchResult struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Description string                 `json:"description"`
	Score       float64                `json:"score"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type EmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func NewSemanticSearcher(db *sql.DB, config SemanticSearchConfig) *SemanticSearcher {
	if config.SimilarityThreshold == 0 {
		config.SimilarityThreshold = 0.7
	}
	if config.MaxResults == 0 {
		config.MaxResults = 10
	}
	if config.EmbeddingDimension == 0 {
		config.EmbeddingDimension = 1536
	}
	if config.HybridTextWeight == 0 {
		config.HybridTextWeight = 0.4
	}
	if config.HybridVectorWeight == 0 {
		config.HybridVectorWeight = 0.6
	}
	if config.EmbeddingModel == "" {
		config.EmbeddingModel = "text-embedding-ada-002"
	}

	return &SemanticSearcher{
		db:                   db,
		embeddingProvider:    config.EmbeddingProvider,
		embeddingAPIKey:      config.EmbeddingAPIKey,
		embeddingModel:       config.EmbeddingModel,
		similarityThreshold:  config.SimilarityThreshold,
		maxResults:           config.MaxResults,
		embeddingDimension:   config.EmbeddingDimension,
		hybridTextWeight:     config.HybridTextWeight,
		hybridVectorWeight:   config.HybridVectorWeight,
		httpClient:           &http.Client{Timeout: 30 * time.Second},
		embeddingCache:       make(map[string]pgvector.Vector),
		cacheTTL:             1 * time.Hour,
		cacheLastCleanup:     time.Now(),
	}
}

func (s *SemanticSearcher) GenerateEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	if text == "" {
		return pgvector.Vector{}, fmt.Errorf("text cannot be empty")
	}

	cacheKey := fmt.Sprintf("%s:%s", s.embeddingProvider, text)
	if cached, ok := s.embeddingCache[cacheKey]; ok {
		return cached, nil
	}

	s.cleanupCache()

	var embedding []float32
	var err error

	switch s.embeddingProvider {
	case "openai":
		embedding, err = s.generateOpenAIEmbedding(ctx, text)
	case "ollama":
		embedding, err = s.generateOllamaEmbedding(ctx, text)
	case "openrouter":
		embedding, err = s.generateOpenRouterEmbedding(ctx, text)
	default:
		return pgvector.Vector{}, fmt.Errorf("unsupported embedding provider: %s", s.embeddingProvider)
	}

	if err != nil {
		return pgvector.Vector{}, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if len(embedding) != s.embeddingDimension {
		return pgvector.Vector{}, fmt.Errorf("embedding dimension mismatch: expected %d, got %d", s.embeddingDimension, len(embedding))
	}

	vec := pgvector.NewVector(embedding)

	s.embeddingCache[cacheKey] = vec

	return vec, nil
}

func (s *SemanticSearcher) generateOpenAIEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": s.embeddingModel,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.embeddingAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("OpenAI API error: %d - %s", resp.StatusCode, string(body))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embResp.Data[0].Embedding, nil
}

func (s *SemanticSearcher) generateOllamaEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"model":  s.embeddingModel,
		"prompt": text,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	ollamaURL := "http://localhost:11434/api/embeddings"

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaURL, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("Ollama API error: %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Embedding, nil
}

func (s *SemanticSearcher) generateOpenRouterEmbedding(ctx context.Context, text string) ([]float32, error) {
	reqBody := map[string]interface{}{
		"input": text,
		"model": s.embeddingModel,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/embeddings", strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.embeddingAPIKey)
	req.Header.Set("HTTP-Referer", "https://github.com/phildougherty/mcp-compose")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("OpenRouter API error: %d - %s", resp.StatusCode, string(body))
	}

	var embResp EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return embResp.Data[0].Embedding, nil
}

func (s *SemanticSearcher) SearchSimilarEntities(ctx context.Context, queryText string) ([]SearchResult, error) {
	embedding, err := s.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	query := `
		SELECT
			id,
			name,
			type,
			description,
			1 - (embedding <=> $1) AS similarity,
			metadata
		FROM entities
		WHERE
			archived = FALSE
			AND embedding IS NOT NULL
			AND 1 - (embedding <=> $1) >= $2
		ORDER BY embedding <=> $1
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, embedding, s.similarityThreshold, s.maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}

	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var metadataJSON []byte

		if err := rows.Scan(&result.ID, &result.Name, &result.Type, &result.Description, &result.Score, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *SemanticSearcher) HybridSearch(ctx context.Context, queryText string) ([]SearchResult, error) {
	embedding, err := s.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	query := `
		WITH text_scores AS (
			SELECT
				e.id,
				ts_rank(to_tsvector('english', e.description), plainto_tsquery('english', $1)) AS text_score
			FROM entities e
			WHERE e.archived = FALSE
				AND to_tsvector('english', e.description) @@ plainto_tsquery('english', $1)
		),
		vector_scores AS (
			SELECT
				e.id,
				1 - (e.embedding <=> $2) AS vector_score
			FROM entities e
			WHERE e.archived = FALSE
				AND e.embedding IS NOT NULL
		)
		SELECT
			e.id,
			e.name,
			e.type,
			e.description,
			(COALESCE(ts.text_score, 0) * $3 + COALESCE(vs.vector_score, 0) * $4) AS combined_score,
			e.metadata
		FROM entities e
		LEFT JOIN text_scores ts ON e.id = ts.id
		LEFT JOIN vector_scores vs ON e.id = vs.id
		WHERE COALESCE(ts.text_score, 0) > 0 OR COALESCE(vs.vector_score, 0) > 0
		ORDER BY combined_score DESC
		LIMIT $5
	`

	rows, err := s.db.QueryContext(ctx, query, queryText, embedding, s.hybridTextWeight, s.hybridVectorWeight, s.maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to execute hybrid search: %w", err)
	}

	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var metadataJSON []byte

		if err := rows.Scan(&result.ID, &result.Name, &result.Type, &result.Description, &result.Score, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *SemanticSearcher) StoreEntityWithEmbedding(ctx context.Context, name, entityType, description string, metadata map[string]interface{}, importanceScore float64) (string, error) {
	embedding, err := s.GenerateEmbedding(ctx, description)
	if err != nil {
		return "", fmt.Errorf("failed to generate embedding: %w", err)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	var id string
	query := `
		INSERT INTO entities (name, type, description, metadata, importance_score, embedding, created_at, updated_at, last_accessed_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW(), NOW())
		RETURNING id
	`

	err = s.db.QueryRowContext(ctx, query, name, entityType, description, metadataJSON, importanceScore, embedding).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("failed to insert entity: %w", err)
	}

	return id, nil
}

func (s *SemanticSearcher) UpdateEntityEmbedding(ctx context.Context, entityID string) error {
	var description string
	query := `SELECT description FROM entities WHERE id = $1 AND archived = FALSE`

	err := s.db.QueryRowContext(ctx, query, entityID).Scan(&description)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("entity not found: %s", entityID)
		}

		return fmt.Errorf("failed to fetch entity: %w", err)
	}

	if description == "" {
		return fmt.Errorf("entity has no description to embed")
	}

	embedding, err := s.GenerateEmbedding(ctx, description)
	if err != nil {
		return fmt.Errorf("failed to generate embedding: %w", err)
	}

	updateQuery := `
		UPDATE entities
		SET embedding = $1, updated_at = NOW()
		WHERE id = $2
	`

	_, err = s.db.ExecContext(ctx, updateQuery, embedding, entityID)
	if err != nil {
		return fmt.Errorf("failed to update entity embedding: %w", err)
	}

	return nil
}

func (s *SemanticSearcher) BatchGenerateEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}

	embeddings := make([]pgvector.Vector, len(texts))

	for i, text := range texts {
		embedding, err := s.GenerateEmbedding(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	return embeddings, nil
}

func (s *SemanticSearcher) FindSimilarRelations(ctx context.Context, queryText string) ([]SearchResult, error) {
	embedding, err := s.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	query := `
		SELECT
			r.id,
			r.relation_type AS name,
			'relation' AS type,
			r.description,
			1 - (r.embedding <=> $1) AS similarity,
			r.metadata
		FROM relations r
		WHERE
			r.archived = FALSE
			AND r.embedding IS NOT NULL
			AND 1 - (r.embedding <=> $1) >= $2
		ORDER BY r.embedding <=> $1
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, embedding, s.similarityThreshold, s.maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}

	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var metadataJSON []byte

		if err := rows.Scan(&result.ID, &result.Name, &result.Type, &result.Description, &result.Score, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *SemanticSearcher) FindSimilarObservations(ctx context.Context, queryText string) ([]SearchResult, error) {
	embedding, err := s.GenerateEmbedding(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	query := `
		SELECT
			o.id,
			o.observation_type AS name,
			'observation' AS type,
			o.content AS description,
			1 - (o.embedding <=> $1) AS similarity,
			o.metadata
		FROM observations o
		WHERE
			o.archived = FALSE
			AND o.embedding IS NOT NULL
			AND 1 - (o.embedding <=> $1) >= $2
		ORDER BY o.embedding <=> $1
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, embedding, s.similarityThreshold, s.maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to execute similarity search: %w", err)
	}

	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var result SearchResult
		var metadataJSON []byte

		if err := rows.Scan(&result.ID, &result.Name, &result.Type, &result.Description, &result.Score, &metadataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (s *SemanticSearcher) cleanupCache() {
	if time.Since(s.cacheLastCleanup) < s.cacheTTL {
		return
	}

	s.embeddingCache = make(map[string]pgvector.Vector)
	s.cacheLastCleanup = time.Now()
}

func (s *SemanticSearcher) GetCacheStats() map[string]interface{} {
	return map[string]interface{}{
		"cache_size":         len(s.embeddingCache),
		"cache_ttl_seconds":  s.cacheTTL.Seconds(),
		"last_cleanup":       s.cacheLastCleanup,
		"similarity_threshold": s.similarityThreshold,
		"max_results":        s.maxResults,
		"embedding_dimension": s.embeddingDimension,
	}
}

func VectorToFloatSlice(v pgvector.Vector) []float32 {
	slice := v.Slice()

	result := make([]float32, len(slice))

	copy(result, slice)

	return result
}

func FloatSliceToVector(f []float32) pgvector.Vector {
	return pgvector.NewVector(f)
}

func ScanVector(scanner interface{ Scan(...interface{}) error }, dest *pgvector.Vector) error {
	return scanner.Scan(dest)
}

func ArrayToVector(arr pq.Float64Array) pgvector.Vector {
	vec := make([]float32, len(arr))
	for i, v := range arr {
		vec[i] = float32(v)
	}

	return pgvector.NewVector(vec)
}