-- MCP Registry Database Schema

-- Table: marketplace_servers (renamed from registry_servers for compatibility)
CREATE TABLE IF NOT EXISTS marketplace_servers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    docker_image VARCHAR(512),
    category VARCHAR(100),
    tags TEXT[], -- PostgreSQL array
    config_template JSONB,
    featured BOOLEAN DEFAULT false,
    downloads INTEGER DEFAULT 0,
    rating DECIMAL(3,2) DEFAULT 0.0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Table: marketplace_categories
CREATE TABLE IF NOT EXISTS marketplace_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    icon VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Table: user_installed_servers
CREATE TABLE IF NOT EXISTS user_installed_servers (
    id SERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL DEFAULT 'default',
    server_id INTEGER REFERENCES marketplace_servers(id) ON DELETE CASCADE,
    server_name VARCHAR(255) NOT NULL,
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    config JSONB,
    status VARCHAR(50) DEFAULT 'active',
    UNIQUE(user_id, server_name)
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_category ON marketplace_servers(category);
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_featured ON marketplace_servers(featured);
CREATE INDEX IF NOT EXISTS idx_marketplace_servers_name ON marketplace_servers(name);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_user ON user_installed_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_server ON user_installed_servers(server_id);
CREATE INDEX IF NOT EXISTS idx_user_installed_servers_status ON user_installed_servers(status);

-- Seed categories
INSERT INTO marketplace_categories (name, description, icon) VALUES
    ('filesystem', 'File and directory operations', '📁'),
    ('database', 'Database connectivity and management', '🗄️'),
    ('search', 'Web and data search capabilities', '🔍'),
    ('productivity', 'Productivity and collaboration tools', '✅'),
    ('development', 'Development and CI/CD tools', '🔧'),
    ('ai', 'AI and machine learning services', '🤖'),
    ('communication', 'Communication and messaging', '💬'),
    ('storage', 'Cloud storage and file management', '☁️')
ON CONFLICT (name) DO NOTHING;

-- Seed initial MCP servers
INSERT INTO marketplace_servers (name, description, docker_image, category, tags, config_template, featured, rating) VALUES
    ('filesystem', 'Provides file system operations including read, write, and directory management', 'mcp/server-filesystem:latest', 'filesystem', ARRAY['files', 'directories', 'read', 'write'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/allowed/path"]}', true, 4.8),
    ('memory', 'Persistent key-value storage with MCP integration', 'mcp/server-memory:latest', 'database', ARRAY['storage', 'kv', 'persistence'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-memory"]}', true, 4.7),
    ('brave-search', 'Web search capabilities powered by Brave Search API', 'mcp/server-brave-search:latest', 'search', ARRAY['search', 'web', 'api'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-brave-search"], "env": {"BRAVE_API_KEY": "your-api-key"}}', true, 4.6),
    ('fetch', 'Fetch and process web content from URLs', 'mcp/server-fetch:latest', 'search', ARRAY['web', 'http', 'fetch'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-fetch"]}', true, 4.5),
    ('postgres', 'PostgreSQL database connectivity and query execution', 'mcp/server-postgres:latest', 'database', ARRAY['database', 'sql', 'postgres'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-postgres"], "env": {"POSTGRES_URL": "postgresql://user:pass@host:5432/db"}}', true, 4.7),
    ('github', 'GitHub repository management and operations', 'mcp/server-github:latest', 'development', ARRAY['git', 'github', 'vcs'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"], "env": {"GITHUB_TOKEN": "your-token"}}', true, 4.6),
    ('slack', 'Slack workspace integration and messaging', 'mcp/server-slack:latest', 'communication', ARRAY['chat', 'messaging', 'team'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-slack"], "env": {"SLACK_BOT_TOKEN": "your-token"}}', false, 4.4),
    ('puppeteer', 'Browser automation and web scraping', 'mcp/server-puppeteer:latest', 'development', ARRAY['browser', 'automation', 'scraping'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-puppeteer"]}', false, 4.5),
    ('google-drive', 'Google Drive file management and operations', 'mcp/server-google-drive:latest', 'storage', ARRAY['cloud', 'storage', 'files'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-gdrive"], "env": {"GOOGLE_CLIENT_ID": "your-id", "GOOGLE_CLIENT_SECRET": "your-secret"}}', false, 4.3),
    ('sqlite', 'SQLite database operations and queries', 'mcp/server-sqlite:latest', 'database', ARRAY['database', 'sql', 'sqlite'], '{"transport": "stdio", "command": "npx", "args": ["-y", "@modelcontextprotocol/server-sqlite", "/path/to/database.db"]}', false, 4.4)
ON CONFLICT (name) DO NOTHING;
