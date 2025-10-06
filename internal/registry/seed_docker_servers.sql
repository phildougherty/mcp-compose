-- Seed MCP servers using the Docker wrapper image
-- All NPM-based servers now use localhost:5000/mcpcompose/mcp-server-wrapper:latest

INSERT INTO marketplace_servers (name, display_name, description, docker_image, category, tags, protocol, config_template, featured, rating, version) VALUES
    ('filesystem', 'Filesystem', 'Provides file system operations including read, write, and directory management', 'mcp/server-filesystem:latest', 'filesystem', ARRAY['files', 'directories', 'read', 'write'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-filesystem", "MCP_ARGS": "/allowed/path"}}', true, 4.8, '1.0.0'),
    ('memory', 'Memory', 'Persistent key-value storage with MCP integration', 'mcp/server-memory:latest', 'database', ARRAY['storage', 'kv', 'persistence'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-memory"}}', true, 4.7, '1.0.0'),
    ('brave-search', 'Brave Search', 'Web search capabilities powered by Brave Search API', 'mcp/server-brave-search:latest', 'search', ARRAY['search', 'web', 'api'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-brave-search", "BRAVE_API_KEY": "your-api-key"}}', true, 4.6, '1.0.0'),
    ('fetch', 'Fetch', 'Fetch and process web content from URLs', 'mcp/server-fetch:latest', 'search', ARRAY['web', 'http', 'fetch'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-fetch"}}', true, 4.5, '1.0.0'),
    ('postgres', 'PostgreSQL', 'PostgreSQL database connectivity and query execution', 'mcp/server-postgres:latest', 'database', ARRAY['database', 'sql', 'postgres'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-postgres", "POSTGRES_URL": "postgresql://user:pass@host:5432/db"}}', true, 4.7, '1.0.0'),
    ('github', 'GitHub', 'GitHub repository management and operations', 'mcp/server-github:latest', 'development', ARRAY['git', 'github', 'vcs'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-github", "GITHUB_TOKEN": "your-token"}}', true, 4.6, '1.0.0'),
    ('slack', 'Slack', 'Slack workspace integration and messaging', 'mcp/server-slack:latest', 'communication', ARRAY['chat', 'messaging', 'team'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-slack", "SLACK_BOT_TOKEN": "your-token"}}', false, 4.4, '1.0.0'),
    ('puppeteer', 'Puppeteer', 'Browser automation and web scraping', 'mcp/server-puppeteer:latest', 'development', ARRAY['browser', 'automation', 'scraping'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-puppeteer"}}', false, 4.5, '1.0.0'),
    ('google-drive', 'Google Drive', 'Google Drive file management and operations', 'mcp/server-google-drive:latest', 'storage', ARRAY['cloud', 'storage', 'files'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-gdrive", "GOOGLE_CLIENT_ID": "your-id", "GOOGLE_CLIENT_SECRET": "your-secret"}}', false, 4.3, '1.0.0'),
    ('sqlite', 'SQLite', 'SQLite database operations and queries', 'mcp/server-sqlite:latest', 'database', ARRAY['database', 'sql', 'sqlite'], 'stdio', '{"image": "localhost:5000/mcpcompose/mcp-server-wrapper:latest", "protocol": "stdio", "env": {"MCP_PACKAGE": "@modelcontextprotocol/server-sqlite", "MCP_ARGS": "/path/to/database.db"}}', false, 4.4, '1.0.0')
ON CONFLICT (name) DO UPDATE SET
    display_name = EXCLUDED.display_name,
    description = EXCLUDED.description,
    docker_image = EXCLUDED.docker_image,
    category = EXCLUDED.category,
    tags = EXCLUDED.tags,
    protocol = EXCLUDED.protocol,
    config_template = EXCLUDED.config_template,
    featured = EXCLUDED.featured,
    rating = EXCLUDED.rating,
    version = EXCLUDED.version;
