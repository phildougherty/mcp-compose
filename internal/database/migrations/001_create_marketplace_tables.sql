-- Migration: Create marketplace tables
-- Version: 001
-- Description: Initial marketplace infrastructure for MCP server catalog

-- Marketplace servers catalog
CREATE TABLE IF NOT EXISTS marketplace_servers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    docker_image VARCHAR(500),
    npm_package VARCHAR(255),
    category VARCHAR(100) NOT NULL,
    tags TEXT[] DEFAULT '{}',
    protocol VARCHAR(50) DEFAULT 'stdio',
    capabilities TEXT[] DEFAULT '{}',
    config_template JSONB NOT NULL,
    featured BOOLEAN DEFAULT false,
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0.0,
    author VARCHAR(255),
    repository_url VARCHAR(500),
    documentation_url VARCHAR(500),
    icon_url VARCHAR(500),
    version VARCHAR(50) DEFAULT '1.0.0',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- User installed servers tracking
CREATE TABLE IF NOT EXISTS user_installed_servers (
    id SERIAL PRIMARY KEY,
    server_id INTEGER NOT NULL REFERENCES marketplace_servers(id) ON DELETE CASCADE,
    user_id VARCHAR(255) DEFAULT 'default',
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    config JSONB,
    status VARCHAR(50) DEFAULT 'active',
    UNIQUE(server_id, user_id)
);

-- Marketplace categories
CREATE TABLE IF NOT EXISTS marketplace_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    icon VARCHAR(100),
    sort_order INTEGER DEFAULT 0
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_category ON marketplace_servers(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_featured ON marketplace_servers(featured);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_user ON user_installed_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_status ON user_installed_servers(status);

-- Insert default categories
INSERT INTO marketplace_categories (name, display_name, description, icon, sort_order) VALUES
    ('filesystem', 'File System', 'File and directory operations', 'folder', 1),
    ('database', 'Databases', 'Database connectivity and tools', 'database', 2),
    ('search', 'Search & Web', 'Web search and data fetching', 'search', 3),
    ('productivity', 'Productivity', 'Note-taking and task management', 'clipboard', 4),
    ('development', 'Development', 'Developer tools and utilities', 'code', 5),
    ('ai', 'AI & ML', 'AI and machine learning tools', 'brain', 6),
    ('communication', 'Communication', 'Messaging and collaboration', 'message-square', 7),
    ('storage', 'Storage & Memory', 'Persistent storage and memory', 'hard-drive', 8)
ON CONFLICT (name) DO NOTHING;

-- Trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_marketplace_server_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER marketplace_servers_updated_at
    BEFORE UPDATE ON marketplace_servers
    FOR EACH ROW
    EXECUTE FUNCTION update_marketplace_server_updated_at();
