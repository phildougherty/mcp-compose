-- Create dedicated schema for task scheduler
CREATE SCHEMA IF NOT EXISTS task_scheduler;

-- Grant permissions
GRANT USAGE ON SCHEMA task_scheduler TO postgres;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA task_scheduler TO postgres;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA task_scheduler TO postgres;

-- Set search path for this migration
SET search_path TO task_scheduler, public;

-- Core scheduler tasks table
CREATE TABLE IF NOT EXISTS task_scheduler.scheduler_tasks (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    type TEXT NOT NULL CHECK (type IN ('ai', 'AI', 'shell', 'manual', 'dependency', 'watcher')),
    enabled BOOLEAN NOT NULL DEFAULT true,

    -- Execution configuration
    command TEXT,
    prompt TEXT,
    schedule TEXT NOT NULL,
    timezone TEXT DEFAULT 'America/New_York',

    -- Chat integration (CORE FEATURE)
    chat_session_id TEXT,
    created_from_message_id TEXT,
    output_to_chat BOOLEAN DEFAULT true,
    inherit_session_context BOOLEAN DEFAULT true,
    conversation_id TEXT,
    conversation_name TEXT,

    -- User ownership
    user_id TEXT NOT NULL,

    -- AI configuration (inherited from session or explicit)
    provider TEXT,
    model TEXT,
    mcp_servers JSONB DEFAULT '[]'::jsonb,

    -- Status tracking
    status TEXT NOT NULL DEFAULT 'pending',
    last_run TIMESTAMPTZ,
    next_run TIMESTAMPTZ,

    -- Advanced scheduling
    depends_on JSONB DEFAULT '[]'::jsonb,
    trigger_type TEXT CHECK (trigger_type IN ('schedule', 'dependency', 'watcher', 'manual')),
    watcher_config JSONB,
    run_on_demand_only BOOLEAN DEFAULT false,

    -- Agent support
    is_agent BOOLEAN DEFAULT false,
    agent_personality TEXT,
    memory_summary TEXT,
    last_memory_update TIMESTAMPTZ,

    -- Scheduling constraints
    skip_holidays BOOLEAN DEFAULT false,
    holiday_region TEXT,
    max_execution_time INTERVAL,
    retry_policy JSONB,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT DEFAULT 'system'
);

-- Add foreign key constraints (cross-schema references)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_chat_session'
        AND table_schema = 'task_scheduler'
        AND table_name = 'scheduler_tasks'
    ) THEN
        ALTER TABLE task_scheduler.scheduler_tasks
        ADD CONSTRAINT fk_chat_session
        FOREIGN KEY (chat_session_id)
        REFERENCES public.chat_sessions(id)
        ON DELETE CASCADE;
    END IF;
END $$;

-- Task execution history
CREATE TABLE IF NOT EXISTS task_scheduler.scheduler_task_runs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,

    -- Execution timing
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    duration INTERVAL GENERATED ALWAYS AS (finished_at - started_at) STORED,

    -- Results
    output TEXT,
    error TEXT,
    exit_code INTEGER,
    status TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'timeout')),

    -- Chat integration
    posted_to_chat BOOLEAN DEFAULT false,
    chat_message_id TEXT,

    -- Context
    triggered_by TEXT,

    -- Metrics
    tokens_used INTEGER,
    cost_estimate NUMERIC(10,6),

    -- Foreign key
    CONSTRAINT fk_task FOREIGN KEY (task_id)
        REFERENCES task_scheduler.scheduler_tasks(id) ON DELETE CASCADE
);

-- Task-specific memory storage
CREATE TABLE IF NOT EXISTS task_scheduler.scheduler_task_memory (
    task_id TEXT NOT NULL,
    memory_key TEXT NOT NULL,
    memory_value TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (task_id, memory_key),
    CONSTRAINT fk_task_memory FOREIGN KEY (task_id)
        REFERENCES task_scheduler.scheduler_tasks(id) ON DELETE CASCADE
);

-- Performance indexes
CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_enabled ON task_scheduler.scheduler_tasks(enabled)
    WHERE enabled = true;

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_chat_session ON task_scheduler.scheduler_tasks(chat_session_id)
    WHERE chat_session_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_next_run ON task_scheduler.scheduler_tasks(next_run)
    WHERE enabled = true AND next_run IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_type ON task_scheduler.scheduler_tasks(type);

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_trigger_type ON task_scheduler.scheduler_tasks(trigger_type);

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_user_id ON task_scheduler.scheduler_tasks(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_scheduler_tasks_user_enabled ON task_scheduler.scheduler_tasks(user_id, enabled)
    WHERE enabled = true;

CREATE INDEX IF NOT EXISTS idx_scheduler_task_runs_task_id_started ON task_scheduler.scheduler_task_runs(task_id, started_at DESC);

CREATE INDEX IF NOT EXISTS idx_scheduler_task_runs_status ON task_scheduler.scheduler_task_runs(status);

CREATE INDEX IF NOT EXISTS idx_scheduler_task_runs_started_at ON task_scheduler.scheduler_task_runs(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_scheduler_task_memory_task_id ON task_scheduler.scheduler_task_memory(task_id);

-- Update timestamp trigger
CREATE OR REPLACE FUNCTION task_scheduler.update_scheduler_task_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.triggers
        WHERE trigger_name = 'trigger_scheduler_task_updated_at'
        AND event_object_schema = 'task_scheduler'
        AND event_object_table = 'scheduler_tasks'
    ) THEN
        CREATE TRIGGER trigger_scheduler_task_updated_at
            BEFORE UPDATE ON task_scheduler.scheduler_tasks
            FOR EACH ROW
            EXECUTE FUNCTION task_scheduler.update_scheduler_task_updated_at();
    END IF;
END $$;

-- Update timestamp trigger for task memory
CREATE OR REPLACE FUNCTION task_scheduler.update_scheduler_task_memory_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.triggers
        WHERE trigger_name = 'trigger_scheduler_task_memory_updated_at'
        AND event_object_schema = 'task_scheduler'
        AND event_object_table = 'scheduler_task_memory'
    ) THEN
        CREATE TRIGGER trigger_scheduler_task_memory_updated_at
            BEFORE UPDATE ON task_scheduler.scheduler_task_memory
            FOR EACH ROW
            EXECUTE FUNCTION task_scheduler.update_scheduler_task_memory_updated_at();
    END IF;
END $$;

-- Set default privileges for future tables
ALTER DEFAULT PRIVILEGES IN SCHEMA task_scheduler
    GRANT ALL PRIVILEGES ON TABLES TO postgres;

ALTER DEFAULT PRIVILEGES IN SCHEMA task_scheduler
    GRANT ALL PRIVILEGES ON SEQUENCES TO postgres;
