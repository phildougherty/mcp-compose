-- Migration: Seed marketplace with MCP servers
-- Version: 002
-- Description: Populate marketplace with popular MCP servers from the ecosystem

-- Insert MCP servers from the official ModelContextProtocol catalog
INSERT INTO marketplace_servers (
    name, display_name, description, docker_image, npm_package,
    category, tags, protocol, capabilities, config_template, featured, author, repository_url
) VALUES
    -- Filesystem Server
    (
        'filesystem',
        'Filesystem',
        'Access and manipulate local files and directories with read, write, search, and directory operations.',
        NULL,
        '@modelcontextprotocol/server-filesystem',
        'filesystem',
        ARRAY['files', 'directories', 'read', 'write'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-filesystem", "${HOME}"],
            "volumes": ["${HOME}:${HOME}:ro"],
            "env": {}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem'
    ),

    -- Memory Server
    (
        'memory',
        'Memory',
        'Persistent key-value storage across sessions. Store and retrieve information that persists between conversations.',
        NULL,
        '@modelcontextprotocol/server-memory',
        'storage',
        ARRAY['memory', 'storage', 'persistence', 'key-value'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-memory"],
            "env": {}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/memory'
    ),

    -- Brave Search Server
    (
        'brave-search',
        'Brave Search',
        'Web search capabilities using Brave Search API. Search the web and get up-to-date information.',
        NULL,
        '@modelcontextprotocol/server-brave-search',
        'search',
        ARRAY['search', 'web', 'brave', 'internet'],
        'stdio',
        ARRAY['tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-brave-search"],
            "env": {"BRAVE_API_KEY": "${BRAVE_API_KEY}"}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/brave-search'
    ),

    -- Fetch Server
    (
        'fetch',
        'Web Fetch',
        'Fetch and retrieve content from web URLs. Download web pages, APIs, and remote resources.',
        NULL,
        '@modelcontextprotocol/server-fetch',
        'search',
        ARRAY['http', 'fetch', 'web', 'download'],
        'stdio',
        ARRAY['tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-fetch"],
            "env": {}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/fetch'
    ),

    -- PostgreSQL Server
    (
        'postgres',
        'PostgreSQL',
        'Connect to PostgreSQL databases and execute queries. Full database access and management.',
        NULL,
        '@modelcontextprotocol/server-postgres',
        'database',
        ARRAY['postgresql', 'database', 'sql', 'queries'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-postgres"],
            "env": {"POSTGRES_URL": "${POSTGRES_URL}"}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/postgres'
    ),

    -- GitHub Server
    (
        'github',
        'GitHub',
        'Interact with GitHub repositories. Create issues, pull requests, search code, and manage repositories.',
        NULL,
        '@modelcontextprotocol/server-github',
        'development',
        ARRAY['github', 'git', 'repositories', 'code'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-github"],
            "env": {"GITHUB_TOKEN": "${GITHUB_TOKEN}"}
        }'::jsonb,
        true,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/github'
    ),

    -- Slack Server
    (
        'slack',
        'Slack',
        'Send and receive Slack messages. Interact with Slack channels, users, and workspaces.',
        NULL,
        '@modelcontextprotocol/server-slack',
        'communication',
        ARRAY['slack', 'messaging', 'chat', 'team'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-slack"],
            "env": {"SLACK_BOT_TOKEN": "${SLACK_BOT_TOKEN}"}
        }'::jsonb,
        false,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/slack'
    ),

    -- Puppeteer Server
    (
        'puppeteer',
        'Puppeteer Browser',
        'Automate browser interactions with Puppeteer. Web scraping, screenshots, and browser automation.',
        NULL,
        '@modelcontextprotocol/server-puppeteer',
        'development',
        ARRAY['browser', 'automation', 'puppeteer', 'scraping'],
        'stdio',
        ARRAY['tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-puppeteer"],
            "env": {}
        }'::jsonb,
        false,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/puppeteer'
    ),

    -- Google Drive Server
    (
        'gdrive',
        'Google Drive',
        'Access Google Drive files and folders. Read, write, and manage Google Drive documents.',
        NULL,
        '@modelcontextprotocol/server-gdrive',
        'storage',
        ARRAY['google', 'drive', 'cloud', 'documents'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-gdrive"],
            "env": {"GDRIVE_CREDENTIALS": "${GDRIVE_CREDENTIALS}"}
        }'::jsonb,
        false,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/gdrive'
    ),

    -- SQLite Server
    (
        'sqlite',
        'SQLite',
        'Query and manage SQLite databases. Lightweight SQL database access and operations.',
        NULL,
        '@modelcontextprotocol/server-sqlite',
        'database',
        ARRAY['sqlite', 'database', 'sql', 'local'],
        'stdio',
        ARRAY['resources', 'tools'],
        '{
            "protocol": "stdio",
            "command": "npx",
            "args": ["-y", "@modelcontextprotocol/server-sqlite"],
            "env": {}
        }'::jsonb,
        false,
        'Anthropic',
        'https://github.com/modelcontextprotocol/servers/tree/main/src/sqlite'
    )
ON CONFLICT (name) DO NOTHING;

-- Update download counts and ratings (simulated initial data)
UPDATE marketplace_servers SET downloads = 1500, rating = 4.8 WHERE name = 'filesystem';
UPDATE marketplace_servers SET downloads = 1200, rating = 4.7 WHERE name = 'memory';
UPDATE marketplace_servers SET downloads = 900, rating = 4.6 WHERE name = 'brave-search';
UPDATE marketplace_servers SET downloads = 800, rating = 4.5 WHERE name = 'fetch';
UPDATE marketplace_servers SET downloads = 700, rating = 4.7 WHERE name = 'postgres';
UPDATE marketplace_servers SET downloads = 650, rating = 4.6 WHERE name = 'github';
UPDATE marketplace_servers SET downloads = 400, rating = 4.3 WHERE name = 'slack';
UPDATE marketplace_servers SET downloads = 350, rating = 4.4 WHERE name = 'puppeteer';
UPDATE marketplace_servers SET downloads = 300, rating = 4.2 WHERE name = 'gdrive';
UPDATE marketplace_servers SET downloads = 250, rating = 4.1 WHERE name = 'sqlite';
