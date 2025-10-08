-- Create workflow schema and tables
CREATE SCHEMA IF NOT EXISTS workflows;

CREATE TABLE IF NOT EXISTS workflows.workflows (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by TEXT,
    metadata JSONB
);

CREATE TABLE IF NOT EXISTS workflows.workflow_nodes (
    id TEXT PRIMARY KEY,
    workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    position_x DOUBLE PRECISION,
    position_y DOUBLE PRECISION,
    data JSONB NOT NULL
);

CREATE TABLE IF NOT EXISTS workflows.workflow_edges (
    id TEXT PRIMARY KEY,
    workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    source_handle TEXT,
    target_handle TEXT
);

CREATE TABLE IF NOT EXISTS workflows.workflow_executions (
    id TEXT PRIMARY KEY,
    workflow_id TEXT REFERENCES workflows.workflows(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error TEXT,
    result JSONB
);

CREATE TABLE IF NOT EXISTS workflows.node_execution_states (
    id SERIAL PRIMARY KEY,
    execution_id TEXT REFERENCES workflows.workflow_executions(id) ON DELETE CASCADE,
    node_id TEXT NOT NULL,
    status TEXT NOT NULL,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error TEXT,
    output JSONB
);

CREATE INDEX IF NOT EXISTS idx_workflows_created_at ON workflows.workflows(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflow_nodes_workflow_id ON workflows.workflow_nodes(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_edges_workflow_id ON workflows.workflow_edges(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflows.workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflows.workflow_executions(status);
CREATE INDEX IF NOT EXISTS idx_node_execution_states_execution_id ON workflows.node_execution_states(execution_id);
