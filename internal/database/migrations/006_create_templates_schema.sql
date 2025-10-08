-- Create templates schema and tables
CREATE SCHEMA IF NOT EXISTS templates;

CREATE TABLE IF NOT EXISTS templates.workflow_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    category TEXT NOT NULL,
    author TEXT NOT NULL,
    thumbnail TEXT,
    tags TEXT[] DEFAULT '{}',
    version TEXT NOT NULL,
    downloads INTEGER DEFAULT 0,
    rating DOUBLE PRECISION DEFAULT 0.0,
    workflow_definition JSONB NOT NULL,
    required_servers TEXT[] DEFAULT '{}',
    estimated_cost TEXT,
    difficulty TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_templates_category ON templates.workflow_templates(category);
CREATE INDEX IF NOT EXISTS idx_templates_difficulty ON templates.workflow_templates(difficulty);
CREATE INDEX IF NOT EXISTS idx_templates_downloads ON templates.workflow_templates(downloads DESC);
CREATE INDEX IF NOT EXISTS idx_templates_rating ON templates.workflow_templates(rating DESC);
CREATE INDEX IF NOT EXISTS idx_templates_created_at ON templates.workflow_templates(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_templates_tags ON templates.workflow_templates USING GIN(tags);
