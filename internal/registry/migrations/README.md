# Registry Database Migrations

This directory contains SQL migrations for the MCP Registry database schema.

## Running Migrations

### Option 1: Manual Migration

Execute the migration SQL file against your PostgreSQL database:

```bash
psql -U postgres -d mcp_dashboard -f internal/registry/migrations/001_initial_schema.sql
```

### Option 2: Using Docker

If you're running PostgreSQL in Docker (as with mcp-compose):

```bash
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_dashboard -f /tmp/001_initial_schema.sql
```

First copy the file into the container:

```bash
docker cp internal/registry/migrations/001_initial_schema.sql mcp-compose-postgres-memory:/tmp/
```

### Option 3: Automatic Migration (Future Enhancement)

Future versions may include automatic migration on application startup.

## Migrations

### 001_initial_schema.sql

**Purpose:** Creates the initial MCP Registry database schema

**Tables Created:**
- `marketplace_servers` - Stores available MCP servers in the registry
- `marketplace_categories` - Stores server categories (filesystem, database, etc.)
- `user_installed_servers` - Tracks which servers users have installed

**Seed Data:**
- 8 categories (filesystem, database, search, productivity, development, ai, communication, storage)
- 10 initial MCP servers from the official ModelContextProtocol catalog

**Idempotent:** Yes - Can be run multiple times safely using `ON CONFLICT` clauses

## Schema Overview

### marketplace_servers

Stores the registry of available MCP servers.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL | Primary key |
| name | VARCHAR(255) | Unique server identifier |
| display_name | VARCHAR(255) | Human-readable name |
| description | TEXT | Server description |
| docker_image | VARCHAR(512) | Docker image name |
| npm_package | VARCHAR(512) | NPM package name |
| category | VARCHAR(100) | Server category |
| tags | TEXT[] | Search tags |
| protocol | VARCHAR(50) | MCP protocol (stdio, sse, http) |
| capabilities | TEXT[] | Server capabilities |
| config_template | JSONB | Configuration template |
| featured | BOOLEAN | Whether server is featured |
| downloads | INTEGER | Download count |
| rating | DECIMAL(3,2) | Average rating (0-5) |
| author | VARCHAR(255) | Server author |
| repository_url | VARCHAR(512) | Source repository URL |
| documentation_url | VARCHAR(512) | Documentation URL |
| icon_url | VARCHAR(512) | Server icon URL |
| version | VARCHAR(50) | Server version |
| created_at | TIMESTAMP | Creation timestamp |
| updated_at | TIMESTAMP | Last update timestamp |

### marketplace_categories

Stores server categories for organization and filtering.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL | Primary key |
| name | VARCHAR(100) | Unique category identifier |
| display_name | VARCHAR(255) | Human-readable name |
| description | TEXT | Category description |
| icon | VARCHAR(50) | Category icon (emoji or URL) |
| sort_order | INTEGER | Display order |
| created_at | TIMESTAMP | Creation timestamp |

### user_installed_servers

Tracks which MCP servers users have installed.

| Column | Type | Description |
|--------|------|-------------|
| id | SERIAL | Primary key |
| user_id | VARCHAR(255) | User identifier (default: 'default') |
| server_id | INTEGER | Foreign key to marketplace_servers |
| server_name | VARCHAR(255) | Server name (for quick lookup) |
| installed_at | TIMESTAMP | Installation timestamp |
| config | JSONB | User-specific configuration |

## Troubleshooting

### Error: relation already exists

This is normal if you've run the migration before. The migration is idempotent and safe to run multiple times.

### Error: column does not exist

Make sure you're running migrations in order (001, 002, etc.).

### Verification

To verify the schema was created correctly:

```sql
-- List all tables
\dt marketplace*

-- Check marketplace_servers structure
\d marketplace_servers

-- Count seeded servers
SELECT COUNT(*) FROM marketplace_servers;

-- List categories
SELECT * FROM marketplace_categories ORDER BY sort_order;
```

Expected output:
- 3 tables (marketplace_servers, marketplace_categories, user_installed_servers)
- 10 servers in marketplace_servers
- 8 categories in marketplace_categories
