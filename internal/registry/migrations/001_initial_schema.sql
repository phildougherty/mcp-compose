-- MCP Registry Initial Schema Migration
-- This ensures all required tables and columns exist

-- Create marketplace_servers table
CREATE TABLE IF NOT EXISTS marketplace_servers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    description TEXT,
    docker_image VARCHAR(512),
    npm_package VARCHAR(512),
    category VARCHAR(100),
    tags TEXT[],
    protocol VARCHAR(50) DEFAULT 'stdio',
    capabilities TEXT[],
    config_template JSONB,
    featured BOOLEAN DEFAULT false,
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0.0,
    author VARCHAR(255),
    repository_url VARCHAR(512),
    documentation_url VARCHAR(512),
    icon_url VARCHAR(512),
    version VARCHAR(50) DEFAULT '1.0.0',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create marketplace_categories table
CREATE TABLE IF NOT EXISTS marketplace_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255),
    description TEXT,
    icon VARCHAR(50),
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create user_installed_servers table
CREATE TABLE IF NOT EXISTS user_installed_servers (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL DEFAULT 'default',
    server_id INTEGER REFERENCES marketplace_servers(id) ON DELETE CASCADE,
    server_name VARCHAR(255) NOT NULL,
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    config JSONB,
    UNIQUE(user_id, server_name)
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_category ON marketplace_servers(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_featured ON marketplace_servers(featured);
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_name ON marketplace_servers(name);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_user ON user_installed_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_server ON user_installed_servers(server_id);

-- Ensure display_name is set for existing records
UPDATE marketplace_servers SET display_name = name WHERE display_name IS NULL;
UPDATE marketplace_categories SET display_name = name WHERE display_name IS NULL;

-- Seed categories
INSERT INTO marketplace_categories (name, display_name, description, icon, sort_order) VALUES
    ('filesystem', 'Filesystem', 'File and directory operations', '📁', 1),
    ('database', 'Database', 'Database connectivity and management', '🗄️', 2),
    ('search', 'Search', 'Web and data search capabilities', '🔍', 3),
    ('productivity', 'Productivity', 'Productivity and collaboration tools', '✅', 4),
    ('development', 'Development', 'Development and CI/CD tools', '🔧', 5),
    ('ai', 'AI', 'AI and machine learning services', '🤖', 6),
    ('communication', 'Communication', 'Communication and messaging', '💬', 7),
    ('storage', 'Storage', 'Cloud storage and file management', '☁️', 8)
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    icon = EXCLUDED.icon,
    sort_order = EXCLUDED.sort_order;

-- Seed initial MCP servers
INSERT INTO marketplace_servers (name, display_name, description, docker_image, category, tags, config_template, featured, rating, version) VALUES
    ('filesystem', 'Filesystem', 'Provides file system operations including read, write, and directory management', 'mcp/server-filesystem:latest', 'filesystem', ARRAY['files', 'directories', 'read', 'write'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/allowed/path"]}', true, 4.8, '1.0.0'),
    ('memory', 'Memory', 'Persistent key-value storage with MCP integration', 'mcp/server-memory:latest', 'database', ARRAY['storage', 'kv', 'persistence'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-memory"]}', true, 4.7, '1.0.0'),
    ('brave-search', 'Brave Search', 'Web search capabilities powered by Brave Search API', 'mcp/server-brave-search:latest', 'search', ARRAY['search', 'web', 'api'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-brave-search"], "env": {"BRAVE_API_KEY": "your-api-key"}}', true, 4.6, '1.0.0'),
    ('fetch', 'Fetch', 'Fetch and process web content from URLs', 'mcp/server-fetch:latest', 'search', ARRAY['web', 'http', 'fetch'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-fetch"]}', true, 4.5, '1.0.0'),
    ('postgres', 'PostgreSQL', 'PostgreSQL database connectivity and query execution', 'mcp/server-postgres:latest', 'database', ARRAY['database', 'sql', 'postgres'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-postgres"], "env": {"POSTGRES_URL": "postgresql://user:pass@host:5432/db"}}', true, 4.7, '1.0.0'),
    ('github', 'GitHub', 'GitHub repository management and operations', 'mcp/server-github:latest', 'development', ARRAY['git', 'github', 'vcs'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"], "env": {"GITHUB_TOKEN": "your-token"}}', true, 4.6, '1.0.0'),
    ('slack', 'Slack', 'Slack workspace integration and messaging', 'mcp/server-slack:latest', 'communication', ARRAY['chat', 'messaging', 'team'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-slack"], "env": {"SLACK_BOT_TOKEN": "your-token"}}', false, 4.4, '1.0.0'),
    ('puppeteer', 'Puppeteer', 'Browser automation and web scraping', 'mcp/server-puppeteer:latest', 'development', ARRAY['browser', 'automation', 'scraping'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-puppeteer"]}', false, 4.5, '1.0.0'),
    ('google-drive', 'Google Drive', 'Google Drive file management and operations', 'mcp/server-google-drive:latest', 'storage', ARRAY['cloud', 'storage', 'files'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-gdrive"], "env": {"GOOGLE_CLIENT_ID": "your-id", "GOOGLE_CLIENT_SECRET": "your-secret"}}', false, 4.3, '1.0.0'),
    ('sqlite', 'SQLite', 'SQLite database operations and queries', 'mcp/server-sqlite:latest', 'database', ARRAY['database', 'sql', 'sqlite'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-sqlite", "/path/to/database.db"]}', false, 4.4, '1.0.0')
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    docker_image = EXCLUDED.docker_image,
    category = EXCLUDED.category,
    tags = EXCLUDED.tags,
    config_template = EXCLUDED.config_template,
    featured = EXCLUDED.featured,
    rating = EXCLUDED.rating,
    version = EXCLUDED.version;
