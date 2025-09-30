-- Migration: 001_create_audit_events
-- Description: Create audit_events table with indexes for efficient querying
-- Created: 2025-09-29

-- Create audit_events table
CREATE TABLE IF NOT EXISTS audit_events (
    id BIGSERIAL PRIMARY KEY,
    audit_id VARCHAR(255) NOT NULL UNIQUE,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    event VARCHAR(255) NOT NULL,
    user_id VARCHAR(255),
    client_id VARCHAR(255),
    ip_address VARCHAR(255),
    user_agent TEXT,
    details JSONB,
    success BOOLEAN NOT NULL,
    error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_event ON audit_events(event);
CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_client_id ON audit_events(client_id);
CREATE INDEX IF NOT EXISTS idx_audit_events_success ON audit_events(success);
CREATE INDEX IF NOT EXISTS idx_audit_events_composite ON audit_events(event, user_id, timestamp DESC);

-- Add comment to table
COMMENT ON TABLE audit_events IS 'Audit log entries for security and compliance tracking';

-- Add comments to columns
COMMENT ON COLUMN audit_events.id IS 'Auto-incrementing primary key';
COMMENT ON COLUMN audit_events.audit_id IS 'Unique audit entry identifier';
COMMENT ON COLUMN audit_events.timestamp IS 'Timestamp when the event occurred';
COMMENT ON COLUMN audit_events.event IS 'Event type (e.g., oauth.token.issued)';
COMMENT ON COLUMN audit_events.user_id IS 'User identifier associated with the event';
COMMENT ON COLUMN audit_events.client_id IS 'OAuth client identifier';
COMMENT ON COLUMN audit_events.ip_address IS 'IP address of the requester';
COMMENT ON COLUMN audit_events.user_agent IS 'User agent string of the requester';
COMMENT ON COLUMN audit_events.details IS 'Additional event details in JSON format';
COMMENT ON COLUMN audit_events.success IS 'Whether the operation was successful';
COMMENT ON COLUMN audit_events.error IS 'Error message if operation failed';
COMMENT ON COLUMN audit_events.created_at IS 'Timestamp when record was created in database';