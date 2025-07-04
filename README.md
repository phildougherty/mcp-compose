# MCP-Compose

A comprehensive orchestration tool for managing Model Context Protocol (MCP) servers with container and proxy capabilities. MCP-Compose provides a Docker Compose-style interface for deploying, configuring, and managing multiple MCP servers through a unified HTTP proxy.

## Overview

MCP-Compose bridges the gap between traditional MCP STDIO servers and modern HTTP-based architectures. It combines the simplicity of Docker Compose configuration with robust MCP protocol support, enabling seamless integration with clients like Claude Desktop, OpenWebUI, and custom applications.

## Features and Benefits

### Core Features
- **Docker Compose-Style Configuration**: Familiar YAML syntax for defining MCP server infrastructure
- **Multi-Transport Protocol Support**: Native support for STDIO, HTTP, and SSE (Server-Sent Events) transports
- **Unified HTTP Proxy**: Single endpoint exposing all MCP servers via RESTful HTTP interface
- **Container & Process Management**: Run servers as Docker containers, Podman containers, or native processes
- **Network Orchestration**: Automatic Docker network management for inter-server communication
- **Session Management**: Persistent MCP sessions with automatic connection pooling and health monitoring

### Developer Experience
- **Built-in MCP Inspector**: Interactive debugging tool for MCP protocol communication
- **Real-time Monitoring**: Health checks, connection status, and performance metrics
- **Auto-generated Documentation**: OpenAPI specifications and interactive docs for each server
- **Hot Configuration Reload**: Update server configurations without full restart
- **Comprehensive Logging**: Structured logging with configurable levels and formats

### Client Integration Benefits
- **OpenWebUI Compatibility**: Direct OpenAPI spec integration with authentication
- **Claude Desktop Support**: Streamlined configuration generation and HTTP endpoint support
- **Custom Client Ready**: RESTful API with proper CORS, authentication, and error handling
- **Tool Discovery**: Automatic tool enumeration and caching across all servers
- **Direct Tool Calls**: FastAPI-style direct tool invocation endpoints

## Compatibility

### MCP Client Compatibility
- **Claude Desktop**: Full HTTP transport support with session management
- **OpenWebUI**: Native OpenAPI specification compatibility with individual server endpoints
- **Custom MCP Clients**: Complete MCP protocol compliance over HTTP with JSON-RPC 2.0
- **Development Tools**: Built-in inspector compatible with standard MCP debugging workflows

### Container Runtime Support
- **Docker**: Full support with automatic network creation and management
- **Podman**: Complete compatibility with rootless containers
- **Native Processes**: Process-based servers for lightweight deployments
- **Hybrid Deployments**: Mix containers and processes in the same configuration

### Transport Protocol Support
- **STDIO**: Direct process communication with automatic socat TCP bridging
- **HTTP**: Native HTTP MCP servers with connection pooling
- **SSE (Server-Sent Events)**: Real-time bidirectional communication
- **TCP**: Raw TCP connections for specialized MCP implementations

### OpenAPI Specification Compliance
- **OpenAPI 3.1.0**: Full specification compliance for tool definitions
- **FastAPI Compatibility**: Direct tool invocation with automatic request/response validation
- **Swagger UI Integration**: Interactive API documentation for each server
- **Schema Generation**: Automatic parameter and response schema inference

## Security Notice

⚠️ **CRITICAL SECURITY REQUIREMENTS** ⚠️

Before using MCP-Compose, please review these security requirements:

1. **Never commit secrets to version control**: All API keys, passwords, and tokens must be stored in environment variables
2. **Use strong credentials**: Generate secure random strings for all API keys and passwords
3. **Follow the principle of least privilege**: Run containers as non-root users and drop unnecessary capabilities
4. **Use the provided security templates**: See `mcp-compose_example.yaml` for secure configuration patterns

### Required Environment Variables

Copy `.env.example` to `.env` and configure:

```bash
# Core authentication
MCP_API_KEY=your-secure-random-api-key-here
POSTGRES_PASSWORD=your-secure-database-password

# Optional external services
GITHUB_TOKEN=ghp_your-github-token-here
OPENROUTER_API_KEY=sk-or-v1-your-openrouter-api-key-here
OAUTH_CLIENT_SECRET=your-oauth-client-secret-here
```

**Generate secure keys:**
```bash
# Generate a secure API key
openssl rand -hex 32

# Or using /dev/urandom
head -c 32 /dev/urandom | base64
```

## Documentation

### 🚀 New to MCP-Compose?
- **[Getting Started Guide](GETTING-STARTED.md)** - Complete 10-minute tutorial
- **[Basic Configuration](mcp-compose-basic.yaml)** - Simple 3-server example
- **[Quickstart](mcp-compose-quickstart.yaml)** - Minimal 1-server example

### 🔄 Migrating from Other Solutions?
- **[Migration Guide](MIGRATION.md)** - Step-by-step migration instructions
- **[From Docker Compose](MIGRATION.md#from-docker-compose--mcp-compose)** - Convert existing setups
- **[From Individual Servers](MIGRATION.md#from-individual-mcp-servers--mcp-compose)** - Centralize management

### 🏢 Enterprise & Advanced Features?
- **[Advanced Configuration](mcp-compose-advanced.yaml)** - OAuth, audit logging, monitoring
- **[Security Best Practices](#security-notice)** - Production security guide
- **[Performance Tuning](#troubleshooting)** - Optimization and debugging

## Quick Start

### 1. Minimal Configuration (30 seconds)

Create a `mcp-compose.yaml` file:

```yaml
version: '1'

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
```

Run with:
```bash
export MCP_API_KEY="your-secure-key-here"
./mcp-compose up
./mcp-compose proxy --port 9876
```

Your MCP servers are now available at `http://localhost:9876`!

### 2. Basic Configuration (3 servers)

For a more complete setup with file access, memory, and search:

```yaml
version: '1'

# Simple authentication
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  # File system access
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/Documents:/workspace:ro"
    
  # Persistent memory/notes
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/memory.db"
    volumes:
      - "mcp-memory-data:/data"

  # Web search
  search:
    image: "mcp/search:latest"
    capabilities: [tools]
    env:
      SEARCH_ENGINE: "duckduckgo"

volumes:
  mcp-memory-data:
    driver: local
```

### 3. Advanced Configuration

For enterprise features like OAuth, audit logging, and complex deployments, see [Advanced Configuration](mcp-compose-advanced.yaml).

## Installation

### Pre-built Binaries (Recommended)

```bash
# Download for your platform
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose
```

### Build from Source

```bash
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
make build
sudo cp build/mcp-compose /usr/local/bin/
```

## Common Use Cases

### Development Environment

```yaml
# mcp-compose.yaml for developers
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/code:/workspace:rw"
  
  git:
    image: "mcp/git:latest" 
    capabilities: [tools]
    volumes:
      - "${HOME}/.gitconfig:/root/.gitconfig:ro"
      - "${HOME}/code:/workspace:rw"
```

### Content Creation

```yaml
# mcp-compose.yaml for writers/researchers  
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/Documents:/documents:rw"
  
  search:
    image: "mcp/search:latest"
    capabilities: [tools]
    env:
      SEARCH_ENGINE: "duckduckgo"
  
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/notes.db"
    volumes:
      - "content-memory:/data"

volumes:
  content-memory:
    driver: local
```

### Enterprise Setup

For production deployments with OAuth, audit logging, monitoring, and advanced security, see [mcp-compose-advanced.yaml](mcp-compose-advanced.yaml).

## Step-by-Step Tutorial

### 1. First Time Setup (5 minutes)

```bash
# 1. Install mcp-compose (see Installation section above)

# 2. Create your first configuration
cat > mcp-compose.yaml << 'EOF'
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
EOF

# 3. Set your API key
export MCP_API_KEY=$(openssl rand -hex 32)
echo "Your API key: $MCP_API_KEY"

# 4. Start the servers
./mcp-compose up

# 5. Start the proxy in another terminal
./mcp-compose proxy --port 9876 --api-key "$MCP_API_KEY"
```

Your MCP servers are now running! Test with:
```bash
curl -H "Authorization: Bearer $MCP_API_KEY" http://localhost:9876/api/servers
```

### 2. Adding More Servers (2 minutes)

```bash
# Stop current setup
./mcp-compose down

# Update configuration to add memory and search
cat > mcp-compose.yaml << 'EOF'
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/Documents:/workspace:ro"
  
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/memory.db"
    volumes:
      - "mcp-memory:/data"
  
  search:
    image: "mcp/search:latest"
    capabilities: [tools]

volumes:
  mcp-memory:
    driver: local
EOF

# Restart with new configuration
./mcp-compose up
./mcp-compose proxy --port 9876
```

### 3. Connect to Claude Desktop (3 minutes)

```bash
# Generate Claude Desktop configuration
./mcp-compose create-config --type claude --output ./claude-config

# Copy the generated config to Claude Desktop settings
# Location varies by OS:
# - macOS: ~/Library/Application Support/Claude/claude_desktop_config.json
# - Linux: ~/.config/Claude/claude_desktop_config.json
# - Windows: %APPDATA%\Claude\claude_desktop_config.json
```

## Migration Guides

### From Individual MCP Servers

If you're currently running individual MCP servers, migration is straightforward:

**Before (individual servers):**
```bash
npx @modelcontextprotocol/server-filesystem /path/to/files
npx @modelcontextprotocol/server-memory
```

**After (mcp-compose):**
```yaml
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "/path/to/files:/workspace:ro"
  
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
```

### From Docker Compose

If you're already using Docker Compose for MCP servers:

**Before (docker-compose.yml):**
```yaml
version: '3.8'
services:
  mcp-filesystem:
    image: mcp/filesystem
    ports:
      - "3000:3000"
```

**After (mcp-compose.yaml):**
```yaml
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
```

Benefits of migrating:
- ✅ Built-in MCP protocol support
- ✅ Automatic service discovery
- ✅ Unified HTTP proxy
- ✅ Claude Desktop integration
- ✅ Health monitoring and restarts

### From Manual Configuration

**Before (manual Claude Desktop config):**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-filesystem", "/path"],
      "env": {}
    }
  }
}
```

**After (with mcp-compose):**
1. Create `mcp-compose.yaml` (see examples above)
2. Run `./mcp-compose create-config --type claude`
3. Replace your Claude Desktop config with the generated one

## Authentication Examples

### Basic API Key (Recommended for getting started)

```yaml
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"  # Set via environment variable

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
```

### No Authentication (Development only)

```yaml
version: '1'

# No proxy_auth section = no authentication required

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
```

⚠️ **Never use no authentication in production!**

### Enterprise OAuth (Advanced)

For OAuth 2.1, RBAC, audit logging, and enterprise features, see [mcp-compose-advanced.yaml](mcp-compose-advanced.yaml).

## Client Integration

### Claude Desktop

```bash
# Generate configuration
./mcp-compose create-config --type claude --output ./claude-config

# Your servers will be available as:
# - http://localhost:9876/filesystem
# - http://localhost:9876/memory  
# - http://localhost:9876/search
```

### OpenWebUI

Each server provides its own OpenAPI endpoint:
```bash
# Filesystem server API docs
curl http://localhost:9876/filesystem/openapi.json

# Memory server API docs  
curl http://localhost:9876/memory/openapi.json
```

### Custom Clients

Direct HTTP API access:
```bash
# List available tools
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/filesystem -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call a tool
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/filesystem -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/workspace/README.md"}}}'
```

## Troubleshooting

### Common Issues

**"Server not found" error:**
```bash
# Check if servers are running
./mcp-compose ls

# Check logs for errors
./mcp-compose logs filesystem
```

**"Connection refused" error:**
```bash
# Ensure proxy is running
./mcp-compose proxy --port 9876

# Check if port is already in use
lsof -i :9876
```

**Authentication errors:**
```bash
# Verify API key is set
echo $MCP_API_KEY

# Check proxy authentication config
grep -A5 "proxy_auth:" mcp-compose.yaml
```

### Debug Mode

```bash
# Enable debug logging
export MCP_LOG_LEVEL=debug
./mcp-compose up

# View detailed proxy logs
./mcp-compose proxy --port 9876 --debug
```

### Performance Tuning

```yaml
# Add connection timeouts (optional)
connections:
  default:
    timeouts:
      connect: "10s"
      read: "30s" 
      write: "30s"
      idle: "60s"
```

For complete configuration reference, see [mcp-compose-advanced.yaml](mcp-compose-advanced.yaml)

## Architecture

### Transport Flow
```
Client → HTTP Proxy → [Protocol Translation] → MCP Server
                                ↓
                      [STDIO|HTTP|SSE|TCP]
```

### Container Orchestration
```
Docker Network (mcp-net)
├── mcp-compose-filesystem (HTTP:3000)
├── mcp-compose-memory (STDIO→TCP:12347)  
├── mcp-compose-cron (SSE:8080)
└── mcp-compose-postgres (TCP:5432)
```

### Session Management
- Persistent HTTP connections with connection pooling
- Automatic MCP session initialization and cleanup
- Cross-request session state preservation
- Health monitoring with automatic reconnection

## Contributing

1. Fork the repository
2. Create a feature branch
3. Follow Go coding standards and add tests
4. Update documentation for new features
5. Submit a pull request with detailed description

### Development Setup
```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Build with development flags
go build -tags dev -o mcp-compose-dev cmd/mcp-compose/main.go

# Run linter
golangci-lint run
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
