-- Add status column to user_installed_servers table
ALTER TABLE user_installed_servers
ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'active';

-- Create index on status column for better query performance
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_status ON user_installed_servers(status);
