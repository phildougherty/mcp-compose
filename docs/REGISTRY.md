# MCP Server Registry

The MCP Server Registry is a built-in app store for discovering, browsing, and installing MCP servers with a single click. It provides a curated catalog of pre-configured servers that can be automatically added to your `mcp-compose.yaml` configuration.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Features](#features)
- [Installation & Setup](#installation--setup)
- [Usage Guide](#usage-guide)
- [API Documentation](#api-documentation)
- [Backend Components](#backend-components)
- [Frontend Components](#frontend-components)
- [Database Schema](#database-schema)
- [Adding New Servers](#adding-new-servers)
- [Configuration](#configuration)
- [Future Enhancements](#future-enhancements)

## Architecture Overview

The Registry feature consists of three main layers:

### 1. Backend (Go)

**Location**: `internal/registry/` and `internal/dashboard/registry_handlers.go`

- **Manager** (`manager.go`): Core business logic for server catalog operations
  - Server listing, filtering, and search
  - Installation tracking
  - Category management
  - PostgreSQL data access layer

- **Installer** (`installer.go`): YAML configuration management
  - Safe server installation to `mcp-compose.yaml`
  - Server uninstallation
  - Configuration template expansion
  - Automatic backup creation

- **Registry** (`registry.go`): Docker registry integration
  - Connection to local Docker registry (localhost:5000)
  - Image verification and health checks
  - Image manifest retrieval

- **Handlers** (`registry_handlers.go`): HTTP API endpoints
  - RESTful API for all registry operations
  - Request validation and error handling
  - Integration with dashboard server

### 2. Database (PostgreSQL)

**Location**: `internal/database/migrations/`

- `marketplace_servers`: Catalog of available servers
- `user_installed_servers`: Tracking of installed servers per user
- `marketplace_categories`: Server categories for organization

### 3. Frontend (React)

**Location**: `internal/dashboard/frontend/src/`

- **Store** (`store/dashboardStore.js`): Integrated Zustand state management
  - Unified state for both running servers and registry catalog
  - View mode toggle (my-servers / browse-registry)
  - Combined search and filtering
- **Components** (`components/Dashboard/`):
  - `Dashboard.jsx`: Unified view with integrated registry browsing
  - `ServerCard.jsx`: Running server display
- **Components** (`components/Registry/`):
  - `ServerCard.jsx`: Registry server display
  - `ServerDetails.jsx`: Detailed server modal
  - `CategoryFilter.jsx`: Category and filter controls

## Features

### Phase 1 (Implemented)

- ✅ Browse catalog of MCP servers
- ✅ Search and filter by category, tags, or name
- ✅ View detailed server information
- ✅ One-click server installation to `mcp-compose.yaml`
- ✅ Server uninstallation
- ✅ Featured servers section
- ✅ Installed servers tracking
- ✅ Category-based organization
- ✅ Docker registry integration
- ✅ Automatic configuration backup
- ✅ Environment variable detection
- ✅ Real-time health monitoring
- ✅ **Integrated Dashboard View**: Registry browsing built directly into the Dashboard tab
- ✅ **Unified State Management**: Single store managing both running servers and registry
- ✅ **Seamless View Toggle**: Switch between "My Servers" and "Browse Registry" without navigation
- ✅ **Install & Start**: Optional endpoint to install and automatically start servers

### Not in Phase 1

- ❌ User ratings and reviews
- ❌ Server publishing by third parties
- ❌ Payment/licensing system
- ❌ Automatic server updates
- ❌ Dependency resolution
- ❌ Server version management

## Installation & Setup

### Prerequisites

1. **PostgreSQL Database**
   ```bash
   # Set PostgreSQL connection string
   export POSTGRES_URL="postgresql://user:password@localhost:5432/mcp_compose"
   ```

2. **Docker Registry (Optional)**
   ```bash
   # For Docker image verification
   export DOCKER_REGISTRY_URL="http://localhost:5000"
   ```

### Database Migrations

Migrations are automatically applied on startup. To manually apply:

```bash
# Run migrations
psql $POSTGRES_URL < internal/database/migrations/001_create_marketplace_tables.sql
psql $POSTGRES_URL < internal/database/migrations/002_seed_marketplace_servers.sql
```

### Configuration

Set environment variables in your shell or `.env` file:

```bash
# Required
POSTGRES_URL=postgresql://user:password@localhost:5432/mcp_compose

# Optional
DOCKER_REGISTRY_URL=http://localhost:5000  # Default: http://localhost:5000
MCP_COMPOSE_CONFIG=mcp-compose.yaml        # Default: mcp-compose.yaml
```

## Usage Guide

### Accessing the Registry

1. Start the MCP Compose dashboard:
   ```bash
   ./mcp-compose up
   ./mcp-compose proxy --port 9876
   ```

2. Open the dashboard in your browser (default: http://localhost:8080)

3. In the Dashboard tab, click the "Browse Registry" button in the view toggle at the top right

### Browsing Servers

- **Search**: Use the search bar to find servers by name, description, or tags
- **Filter by Category**: Click "Filters" and select a category (Filesystem, Database, Search, etc.)
- **Featured Servers**: Toggle "Featured only" to see curated recommendations
- **Installed Servers**: View servers you've already installed at the top

### Installing a Server

1. Click on a server card to open detailed information
2. Review the configuration template and required environment variables
3. Click "Install" to add the server to your `mcp-compose.yaml`
4. The server is automatically added with proper configuration
5. A backup file is created at `mcp-compose.yaml.backup`

### Uninstalling a Server

1. Click on an installed server card
2. Click "Uninstall" in the details modal
3. Confirm the uninstallation
4. The server is removed from `mcp-compose.yaml` (backup created)

## API Documentation

All endpoints are prefixed with `/api/registry`

### List Servers

```http
GET /api/registry/servers?category=database&search=postgres&featured=true
```

**Query Parameters**:
- `category` (optional): Filter by category name
- `search` (optional): Search query for name/description
- `featured` (optional): Filter featured servers (true/false)

**Response**:
```json
{
  "servers": [
    {
      "id": 1,
      "name": "postgres",
      "displayName": "PostgreSQL",
      "description": "Connect to PostgreSQL databases...",
      "category": "database",
      "tags": ["postgresql", "database", "sql"],
      "protocol": "stdio",
      "capabilities": ["resources", "tools"],
      "featured": true,
      "downloads": 700,
      "rating": 4.7,
      "version": "1.0.0",
      "configTemplate": { ... }
    }
  ],
  "count": 1
}
```

### Get Server Details

```http
GET /api/registry/servers/:id
```

**Response**:
```json
{
  "server": { ... },
  "installed": true
}
```

### Get Categories

```http
GET /api/registry/categories
```

**Response**:
```json
{
  "categories": [
    {
      "id": 1,
      "name": "filesystem",
      "displayName": "File System",
      "description": "File and directory operations",
      "icon": "folder",
      "sortOrder": 1
    }
  ],
  "count": 8
}
```

### Get Featured Servers

```http
GET /api/registry/featured
```

Returns servers marked as featured.

### Install Server

```http
POST /api/registry/install
Content-Type: application/json

{
  "serverId": 1,
  "config": {
    "env": {
      "POSTGRES_URL": "postgresql://localhost:5432/mydb"
    }
  }
}
```

**Response**:
```json
{
  "success": true,
  "message": "Server 'postgres' installed successfully",
  "server": { ... }
}
```

### Install and Start Server

```http
POST /api/registry/install-and-start
Content-Type: application/json

{
  "serverId": 1,
  "config": {
    "env": {
      "POSTGRES_URL": "postgresql://localhost:5432/mydb"
    }
  }
}
```

Installs the server to mcp-compose.yaml and automatically starts it.

**Response**:
```json
{
  "success": true,
  "message": "Server 'postgres' installed and will start automatically",
  "server": { ... }
}
```

### Uninstall Server

```http
POST /api/registry/uninstall
Content-Type: application/json

{
  "serverId": 1
}
```

**Response**:
```json
{
  "success": true,
  "message": "Server 'postgres' uninstalled successfully"
}
```

### Get Installed Servers

```http
GET /api/registry/installed
```

**Response**:
```json
{
  "installed": [
    {
      "installation": {
        "id": 1,
        "serverId": 1,
        "userId": "default",
        "installedAt": "2025-01-15T10:30:00Z",
        "config": { ... },
        "status": "active"
      },
      "server": { ... }
    }
  ],
  "count": 1
}
```

### Health Check

```http
GET /api/registry/health
```

**Response**:
```json
{
  "status": "healthy",
  "registry": "healthy",
  "database": "healthy"
}
```

## Backend Components

### Registry Manager

**File**: `internal/registry/manager.go`

Main business logic component that handles:
- Server CRUD operations
- Filtering and search
- Category management
- Installation tracking
- In-memory caching for performance

**Key Methods**:
```go
func (m *Manager) ListServers(ctx context.Context, filter *ServerFilter) ([]Server, error)
func (m *Manager) GetServer(ctx context.Context, id int) (*Server, error)
func (m *Manager) InstallServer(ctx context.Context, req *InstallRequest) error
func (m *Manager) UninstallServer(ctx context.Context, serverID int, userID string) error
func (m *Manager) GetInstalledServers(ctx context.Context, userID string) ([]InstalledServer, error)
func (m *Manager) GetCategories(ctx context.Context) ([]Category, error)
```

### Installer

**File**: `internal/registry/installer.go`

Handles YAML configuration manipulation:
- Reads existing `mcp-compose.yaml`
- Safely appends new server configuration
- Removes servers on uninstall
- Creates automatic backups
- Expands environment variables in templates

**Key Methods**:
```go
func (i *Installer) InstallServerToConfig(server *Server, userConfig map[string]interface{}) error
func (i *Installer) UninstallServerFromConfig(serverName string) error
func (i *Installer) IsServerInstalled(serverName string) (bool, error)
```

### Docker Registry Client

**File**: `internal/registry/registry.go`

Integrates with Docker registry:
- Health check verification
- Image listing and manifest retrieval
- Image existence verification

**Key Methods**:
```go
func (r *Registry) HealthCheck() error
func (r *Registry) ListImages() ([]RegistryImage, error)
func (r *Registry) ImageExists(imageName, tag string) (bool, error)
```

### API Handlers

**File**: `internal/dashboard/registry_handlers.go`

HTTP handlers for all registry endpoints:
- Request validation
- Error handling
- JSON marshaling
- Context timeout management

## Frontend Components

### Dashboard Store (Zustand)

**File**: `store/dashboardStore.js`

Unified global state management for both running servers and registry:

**State**:
- `servers`: Running MCP servers
- `metrics`: Dashboard metrics (running, healthy, connections)
- `viewMode`: Current view ('my-servers' or 'browse-registry')
- `registryServers`: Registry catalog servers
- `categories`: Available categories
- `featuredServers`: Featured registry servers
- `installedServers`: User's installed servers
- `selectedRegistryServer`: Currently selected registry server
- `searchQuery`: Unified search query
- `categoryFilter`: Category filter for registry
- `featuredOnly`: Show only featured servers

**Actions**:
- `setViewMode(mode)`: Toggle between My Servers and Browse Registry
- `fetchRegistryServers()`: Load servers with current filters
- `fetchCategories()`: Load all categories
- `fetchFeatured()`: Load featured servers
- `fetchInstalledServers()`: Load user's installed servers
- `installServer(serverId, config)`: Install a server
- `uninstallServer(serverId)`: Uninstall a server
- `isServerInstalled(serverId)`: Check if server is installed

### Dashboard Component

**File**: `components/Dashboard/Dashboard.jsx`

Unified view with integrated registry browsing:
- View mode toggle (My Servers / Browse Registry)
- Conditional rendering based on viewMode
- My Servers view: Running server cards with metrics
- Browse Registry view: Registry catalog with search and filters
- Server details modal for installation
- Automatic refresh for running servers

### Server Card Component

**File**: `components/Registry/ServerCard.jsx`

Individual server display:
- Server name, description, author
- Category badge and protocol indicator
- Downloads and rating display
- Tags and version
- Installed/Featured badges

### Server Details Modal

**File**: `components/Registry/ServerDetails.jsx`

Detailed server information:
- Full description and metadata
- Configuration template preview
- Environment variable requirements
- Install/Uninstall actions
- Links to repository and documentation

### Category Filter Component

**File**: `components/Registry/CategoryFilter.jsx`

Filter controls:
- Category selection grid
- Featured-only toggle
- Clear filters button

## Database Schema

### marketplace_servers

Stores the catalog of available servers.

```sql
CREATE TABLE marketplace_servers (
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
```

**Indexes**:
- `idx_marketplace_servers_category`: Performance for category filtering
- `idx_marketplace_servers_featured`: Quick featured server lookups

### user_installed_servers

Tracks which servers are installed for each user.

```sql
CREATE TABLE user_installed_servers (
    id SERIAL PRIMARY KEY,
    server_id INTEGER NOT NULL REFERENCES marketplace_servers(id) ON DELETE CASCADE,
    user_id VARCHAR(255) DEFAULT 'default',
    installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    config JSONB,
    status VARCHAR(50) DEFAULT 'active',
    UNIQUE(server_id, user_id)
);
```

**Indexes**:
- `idx_user_installed_servers_user`: Fast lookups by user
- `idx_user_installed_servers_status`: Filter by installation status

### marketplace_categories

Defines available categories for organization.

```sql
CREATE TABLE marketplace_categories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    icon VARCHAR(100),
    sort_order INTEGER DEFAULT 0
);
```

## Adding New Servers

### Method 1: SQL Insert

Add servers directly to the database:

```sql
INSERT INTO marketplace_servers (
    name, display_name, description, npm_package,
    category, tags, protocol, capabilities, config_template, featured, author, repository_url
) VALUES (
    'example-server',
    'Example Server',
    'An example MCP server for demonstration',
    '@example/mcp-server',
    'development',
    ARRAY['example', 'demo', 'testing'],
    'stdio',
    ARRAY['tools', 'resources'],
    '{
        "protocol": "stdio",
        "command": "npx",
        "args": ["-y", "@example/mcp-server"],
        "env": {"EXAMPLE_API_KEY": "${EXAMPLE_API_KEY}"}
    }'::jsonb,
    false,
    'Your Name',
    'https://github.com/your-org/mcp-server'
);
```

### Method 2: Migration File

Create a new migration file in `internal/database/migrations/`:

```sql
-- 003_add_custom_servers.sql
INSERT INTO marketplace_servers (name, display_name, ...) VALUES (...);
```

### Configuration Template Format

The `config_template` field is a JSON object matching the `mcp-compose.yaml` server format:

```json
{
  "protocol": "stdio",
  "command": "npx",
  "args": ["-y", "@modelcontextprotocol/server-example"],
  "env": {
    "API_KEY": "${API_KEY}",
    "OPTION": "value"
  },
  "volumes": ["${HOME}/data:/data:ro"]
}
```

**Variable Expansion**:
- `${HOME}`: User's home directory
- `${ANY_ENV_VAR}`: Any environment variable
- Variables are expanded when installing

## Configuration

### Environment Variables

```bash
# Required
POSTGRES_URL=postgresql://user:password@localhost:5432/mcp_compose

# Optional
DOCKER_REGISTRY_URL=http://localhost:5000
MCP_COMPOSE_CONFIG=mcp-compose.yaml
MCP_API_KEY=your-api-key
MCP_PROXY_URL=http://localhost:9876
```

### Docker Registry Setup

If you want to use the Docker registry integration:

```bash
# Start a local Docker registry
docker run -d -p 5000:5000 --name registry registry:2

# Push images to it
docker tag my-mcp-server localhost:5000/my-mcp-server:latest
docker push localhost:5000/my-mcp-server:latest
```

## Current Server Catalog

The initial seed includes 10 MCP servers from the official Anthropic catalog:

### Featured Servers

1. **Filesystem** - File and directory operations
2. **Memory** - Persistent key-value storage
3. **Brave Search** - Web search capabilities
4. **Fetch** - Web content fetching
5. **PostgreSQL** - Database connectivity
6. **GitHub** - Repository management

### Other Servers

7. **Slack** - Team communication
8. **Puppeteer** - Browser automation
9. **Google Drive** - Cloud storage
10. **SQLite** - Lightweight SQL database

## Future Enhancements

### Phase 2: Community Features

- User ratings and reviews
- Server comments and feedback
- Usage statistics and analytics
- Server recommendations based on usage

### Phase 3: Publishing Platform

- Server submission workflow
- Automated testing and validation
- Version management and updates
- Dependency resolution
- Private server registries

### Phase 4: Advanced Features

- Server bundling (install multiple servers at once)
- Configuration wizards for complex servers
- Server health monitoring integration
- Automated update notifications
- Rollback capabilities

## Troubleshooting

### Registry Not Available

If the Registry tab shows errors:

1. **Check PostgreSQL Connection**
   ```bash
   psql $POSTGRES_URL -c "SELECT 1"
   ```

2. **Verify Migrations**
   ```bash
   psql $POSTGRES_URL -c "SELECT * FROM marketplace_servers LIMIT 1"
   ```

3. **Check Logs**
   ```bash
   ./mcp-compose logs dashboard
   ```

### Installation Failures

If server installation fails:

1. **Check File Permissions**
   ```bash
   ls -la mcp-compose.yaml
   ```

2. **Verify YAML Syntax**
   ```bash
   # Install yq if not available
   yq eval . mcp-compose.yaml
   ```

3. **Check Backup Files**
   ```bash
   ls -la mcp-compose.yaml.backup
   ```

### Docker Registry Issues

If Docker registry health check fails:

1. **Verify Registry is Running**
   ```bash
   curl http://localhost:5000/v2/
   ```

2. **Check Registry Configuration**
   ```bash
   echo $DOCKER_REGISTRY_URL
   ```

## Contributing

To contribute new servers to the catalog:

1. Test the server configuration locally
2. Add the server via SQL or migration
3. Update download counts and ratings as needed
4. Submit a pull request with documentation

## Support

For issues or questions:
- GitHub Issues: [mcp-compose/issues](https://github.com/phildougherty/mcp-compose/issues)
- Documentation: [README.md](README.md)
- Examples: [examples/](examples/)

---

**Last Updated**: January 2025
**Version**: 1.0.0 (Phase 1)
