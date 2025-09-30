-- Enhanced Memory Service Schema
-- Entity-Relation-Observation (ERO) model with vector embeddings
-- Requires PostgreSQL 14+ with pgvector extension

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgvector";

-- Entities table: stores unique entities (people, places, things, concepts)
CREATE TABLE IF NOT EXISTS entities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    description TEXT,
    metadata JSONB DEFAULT '{}',
    importance_score FLOAT DEFAULT 0.5 CHECK (importance_score >= 0 AND importance_score <= 1),
    embedding vector(1536),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    access_count INTEGER DEFAULT 0,
    archived BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_entities_name ON entities(name);
CREATE INDEX idx_entities_type ON entities(type);
CREATE INDEX idx_entities_importance ON entities(importance_score DESC);
CREATE INDEX idx_entities_last_accessed ON entities(last_accessed_at DESC);
CREATE INDEX idx_entities_archived ON entities(archived) WHERE archived = FALSE;
CREATE INDEX idx_entities_name_trgm ON entities USING gin(name gin_trgm_ops);
CREATE INDEX idx_entities_description_fts ON entities USING gin(to_tsvector('english', description));
CREATE INDEX idx_entities_embedding_hnsw ON entities USING hnsw (embedding vector_cosine_ops);

-- Relations table: stores relationships between entities
CREATE TABLE IF NOT EXISTS relations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    target_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    relation_type VARCHAR(100) NOT NULL,
    description TEXT,
    metadata JSONB DEFAULT '{}',
    importance_score FLOAT DEFAULT 0.5 CHECK (importance_score >= 0 AND importance_score <= 1),
    embedding vector(1536),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    access_count INTEGER DEFAULT 0,
    archived BOOLEAN DEFAULT FALSE,
    CONSTRAINT relation_self_check CHECK (source_id != target_id)
);

CREATE INDEX idx_relations_source ON relations(source_id);
CREATE INDEX idx_relations_target ON relations(target_id);
CREATE INDEX idx_relations_type ON relations(relation_type);
CREATE INDEX idx_relations_importance ON relations(importance_score DESC);
CREATE INDEX idx_relations_last_accessed ON relations(last_accessed_at DESC);
CREATE INDEX idx_relations_archived ON relations(archived) WHERE archived = FALSE;
CREATE INDEX idx_relations_source_target ON relations(source_id, target_id);
CREATE INDEX idx_relations_description_fts ON relations USING gin(to_tsvector('english', description));
CREATE INDEX idx_relations_embedding_hnsw ON relations USING hnsw (embedding vector_cosine_ops);

-- Observations table: stores temporal observations about entities
CREATE TABLE IF NOT EXISTS observations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    entity_id UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    observation_type VARCHAR(100) NOT NULL,
    metadata JSONB DEFAULT '{}',
    importance_score FLOAT DEFAULT 0.5 CHECK (importance_score >= 0 AND importance_score <= 1),
    embedding vector(1536),
    observed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_accessed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    access_count INTEGER DEFAULT 0,
    archived BOOLEAN DEFAULT FALSE
);

CREATE INDEX idx_observations_entity ON observations(entity_id);
CREATE INDEX idx_observations_type ON observations(observation_type);
CREATE INDEX idx_observations_importance ON observations(importance_score DESC);
CREATE INDEX idx_observations_observed_at ON observations(observed_at DESC);
CREATE INDEX idx_observations_last_accessed ON observations(last_accessed_at DESC);
CREATE INDEX idx_observations_archived ON observations(archived) WHERE archived = FALSE;
CREATE INDEX idx_observations_content_fts ON observations USING gin(to_tsvector('english', content));
CREATE INDEX idx_observations_embedding_hnsw ON observations USING hnsw (embedding vector_cosine_ops);

-- Archive tables for cold storage
CREATE TABLE IF NOT EXISTS entities_archive (
    LIKE entities INCLUDING ALL
);

CREATE TABLE IF NOT EXISTS relations_archive (
    LIKE relations INCLUDING ALL
);

CREATE TABLE IF NOT EXISTS observations_archive (
    LIKE observations INCLUDING ALL
);

-- Memory statistics table for tracking usage patterns
CREATE TABLE IF NOT EXISTS memory_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    stat_date DATE NOT NULL DEFAULT CURRENT_DATE,
    total_entities INTEGER DEFAULT 0,
    total_relations INTEGER DEFAULT 0,
    total_observations INTEGER DEFAULT 0,
    archived_entities INTEGER DEFAULT 0,
    archived_relations INTEGER DEFAULT 0,
    archived_observations INTEGER DEFAULT 0,
    avg_importance_score FLOAT DEFAULT 0.0,
    most_accessed_entity_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(stat_date)
);

-- Pruning log table for audit trail
CREATE TABLE IF NOT EXISTS pruning_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    strategy VARCHAR(50) NOT NULL,
    entities_pruned INTEGER DEFAULT 0,
    relations_pruned INTEGER DEFAULT 0,
    observations_pruned INTEGER DEFAULT 0,
    entities_archived INTEGER DEFAULT 0,
    relations_archived INTEGER DEFAULT 0,
    observations_archived INTEGER DEFAULT 0,
    duration_ms INTEGER,
    metadata JSONB DEFAULT '{}'
);

CREATE INDEX idx_pruning_log_run_at ON pruning_log(run_at DESC);

-- Function to update last_accessed_at and access_count
CREATE OR REPLACE FUNCTION update_access_stats()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_accessed_at = NOW();
    NEW.access_count = OLD.access_count + 1;
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically update access stats on UPDATE
CREATE TRIGGER entities_access_trigger
BEFORE UPDATE ON entities
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION update_access_stats();

CREATE TRIGGER relations_access_trigger
BEFORE UPDATE ON relations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION update_access_stats();

CREATE TRIGGER observations_access_trigger
BEFORE UPDATE ON observations
FOR EACH ROW
WHEN (OLD.* IS DISTINCT FROM NEW.*)
EXECUTE FUNCTION update_access_stats();

-- Function to calculate composite importance score
CREATE OR REPLACE FUNCTION calculate_importance(
    base_score FLOAT,
    access_count INTEGER,
    days_since_access FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN LEAST(1.0, base_score *
        (1.0 + LOG(1 + access_count) * 0.1) *
        (1.0 / (1.0 + days_since_access * 0.01))
    );
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Function to archive old memories
CREATE OR REPLACE FUNCTION archive_old_memories(
    retention_days INTEGER DEFAULT 90,
    min_importance FLOAT DEFAULT 0.3
) RETURNS TABLE (
    entities_archived INTEGER,
    relations_archived INTEGER,
    observations_archived INTEGER
) AS $$
DECLARE
    e_count INTEGER := 0;
    r_count INTEGER := 0;
    o_count INTEGER := 0;
BEGIN
    WITH archived_entities AS (
        INSERT INTO entities_archive
        SELECT * FROM entities
        WHERE archived = FALSE
            AND last_accessed_at < NOW() - INTERVAL '1 day' * retention_days
            AND importance_score < min_importance
        RETURNING id
    )
    UPDATE entities SET archived = TRUE
    WHERE id IN (SELECT id FROM archived_entities);

    GET DIAGNOSTICS e_count = ROW_COUNT;

    WITH archived_relations AS (
        INSERT INTO relations_archive
        SELECT * FROM relations
        WHERE archived = FALSE
            AND last_accessed_at < NOW() - INTERVAL '1 day' * retention_days
            AND importance_score < min_importance
        RETURNING id
    )
    UPDATE relations SET archived = TRUE
    WHERE id IN (SELECT id FROM archived_relations);

    GET DIAGNOSTICS r_count = ROW_COUNT;

    WITH archived_observations AS (
        INSERT INTO observations_archive
        SELECT * FROM observations
        WHERE archived = FALSE
            AND last_accessed_at < NOW() - INTERVAL '1 day' * retention_days
            AND importance_score < min_importance
        RETURNING id
    )
    UPDATE observations SET archived = TRUE
    WHERE id IN (SELECT id FROM archived_observations);

    GET DIAGNOSTICS o_count = ROW_COUNT;

    RETURN QUERY SELECT e_count, r_count, o_count;
END;
$$ LANGUAGE plpgsql;

-- Function to find similar entities using vector embeddings
CREATE OR REPLACE FUNCTION find_similar_entities(
    query_embedding vector(1536),
    similarity_threshold FLOAT DEFAULT 0.7,
    result_limit INTEGER DEFAULT 10
) RETURNS TABLE (
    id UUID,
    name VARCHAR,
    type VARCHAR,
    description TEXT,
    similarity FLOAT
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        e.id,
        e.name,
        e.type,
        e.description,
        1 - (e.embedding <=> query_embedding) AS similarity
    FROM entities e
    WHERE
        e.archived = FALSE
        AND e.embedding IS NOT NULL
        AND 1 - (e.embedding <=> query_embedding) >= similarity_threshold
    ORDER BY e.embedding <=> query_embedding
    LIMIT result_limit;
END;
$$ LANGUAGE plpgsql;

-- Function for hybrid search (full-text + vector)
CREATE OR REPLACE FUNCTION hybrid_search_entities(
    search_query TEXT,
    query_embedding vector(1536),
    text_weight FLOAT DEFAULT 0.4,
    vector_weight FLOAT DEFAULT 0.6,
    result_limit INTEGER DEFAULT 10
) RETURNS TABLE (
    id UUID,
    name VARCHAR,
    type VARCHAR,
    description TEXT,
    combined_score FLOAT
) AS $$
BEGIN
    RETURN QUERY
    WITH text_scores AS (
        SELECT
            e.id,
            ts_rank(to_tsvector('english', e.description), plainto_tsquery('english', search_query)) AS text_score
        FROM entities e
        WHERE e.archived = FALSE
            AND to_tsvector('english', e.description) @@ plainto_tsquery('english', search_query)
    ),
    vector_scores AS (
        SELECT
            e.id,
            1 - (e.embedding <=> query_embedding) AS vector_score
        FROM entities e
        WHERE e.archived = FALSE
            AND e.embedding IS NOT NULL
    )
    SELECT
        e.id,
        e.name,
        e.type,
        e.description,
        (COALESCE(ts.text_score, 0) * text_weight + COALESCE(vs.vector_score, 0) * vector_weight) AS combined_score
    FROM entities e
    LEFT JOIN text_scores ts ON e.id = ts.id
    LEFT JOIN vector_scores vs ON e.id = vs.id
    WHERE COALESCE(ts.text_score, 0) > 0 OR COALESCE(vs.vector_score, 0) > 0
    ORDER BY combined_score DESC
    LIMIT result_limit;
END;
$$ LANGUAGE plpgsql;

-- View for active memories with calculated importance
CREATE OR REPLACE VIEW active_memories AS
SELECT
    e.id,
    e.name,
    e.type,
    e.description,
    e.importance_score,
    e.access_count,
    e.last_accessed_at,
    calculate_importance(
        e.importance_score,
        e.access_count,
        EXTRACT(EPOCH FROM (NOW() - e.last_accessed_at)) / 86400.0
    ) AS calculated_importance,
    COUNT(r.id) AS relation_count,
    COUNT(o.id) AS observation_count
FROM entities e
LEFT JOIN relations r ON e.id = r.source_id OR e.id = r.target_id
LEFT JOIN observations o ON e.id = o.entity_id
WHERE e.archived = FALSE
GROUP BY e.id, e.name, e.type, e.description, e.importance_score, e.access_count, e.last_accessed_at;

-- Grant necessary permissions
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO postgres;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO postgres;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA public TO postgres;