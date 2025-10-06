-- Add task integration to chat sessions (public schema)
-- No database switch needed - already in mcp_compose database
ALTER TABLE chat_sessions
    ADD COLUMN IF NOT EXISTS associated_task_ids TEXT[] DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS unread_message_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_read_at TIMESTAMPTZ DEFAULT NOW(),
    ADD COLUMN IF NOT EXISTS has_active_agents BOOLEAN DEFAULT false;

-- Index for unread notifications
CREATE INDEX IF NOT EXISTS idx_chat_sessions_unread
    ON chat_sessions(user_id, unread_message_count DESC)
    WHERE unread_message_count > 0;

-- Index for active agents
CREATE INDEX IF NOT EXISTS idx_chat_sessions_agents
    ON chat_sessions(user_id, has_active_agents)
    WHERE has_active_agents = true;

-- Add task execution tracking to messages
ALTER TABLE chat_messages
    ADD COLUMN IF NOT EXISTS from_task_run_id TEXT,
    ADD COLUMN IF NOT EXISTS is_automated BOOLEAN DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_chat_messages_task_run
    ON chat_messages(from_task_run_id)
    WHERE from_task_run_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_chat_messages_automated
    ON chat_messages(session_id, is_automated, created_at DESC)
    WHERE is_automated = true;

-- Add foreign key (cross-schema reference)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'fk_chat_messages_task_run'
    ) THEN
        ALTER TABLE public.chat_messages
        ADD CONSTRAINT fk_chat_messages_task_run
        FOREIGN KEY (from_task_run_id)
        REFERENCES task_scheduler.scheduler_task_runs(id) ON DELETE SET NULL;
    END IF;
END $$;
