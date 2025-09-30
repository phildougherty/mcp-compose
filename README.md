# MCP-Compose

[![codecov](https://codecov.io/gh/phildougherty/mcp-compose/branch/main/graph/badge.svg)](https://codecov.io/gh/phildougherty/mcp-compose)
[![Go Report Card](https://goreportcard.com/badge/github.com/phildougherty/mcp-compose)](https://goreportcard.com/report/github.com/phildougherty/mcp-compose)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Release](https://img.shields.io/github/v/release/phildougherty/mcp-compose)](https://github.com/phildougherty/mcp-compose/releases)

Docker Compose for Model Context Protocol servers. Run multiple MCP servers with unified configuration, HTTP proxy, and container orchestration.

![MCP-Compose Demo](demo.gif)

## Features

- Docker Compose-style YAML configuration
- HTTP proxy with automatic protocol translation (STDIO → HTTP)
- Native support for Docker and Podman
- Session management and connection pooling
- Built-in dashboard and monitoring
- OpenAPI spec generation

## Installation

```bash
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
make build

# Add to PATH
sudo cp build/mcp-compose /usr/local/bin/
# OR
export PATH="$PWD/build:$PATH"
```

## Quick Start

### Interactive Setup

```bash
mcp-compose init
```

Follow the prompts to create your configuration.

### Manual Setup

Create `mcp-compose.yaml`:

```yaml
version: '1'
servers:
  filesystem:
    image: mcp/filesystem
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "/tmp"
    capabilities: [resources, tools]

  memory:
    image: mcp/memory
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-memory"
    capabilities: [resources, tools]
```

Start servers:

```bash
mcp-compose up
```

Start the HTTP proxy:

```bash
mcp-compose proxy --port 9876
```

Test it:

```bash
curl http://localhost:9876/api/servers
```

## Working Configuration Example

This configuration has been tested and works:

```yaml
version: '1'
servers:
  filesystem:
    image: mcp/filesystem
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "${HOME}"
    capabilities: [resources, tools]
    environment:
      NODE_ENV: production
    volumes:
      - "${HOME}:${HOME}:ro"

  memory:
    image: mcp/memory
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-memory"
    capabilities: [resources, tools]

  brave-search:
    image: mcp/brave-search
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-brave-search"
    capabilities: [tools]
    environment:
      BRAVE_API_KEY: "${BRAVE_API_KEY}"
```

Set required environment variables:

```bash
export BRAVE_API_KEY="your-api-key-here"
```

## Commands

### Service Management

```bash
mcp-compose up                 # Start all services
mcp-compose up filesystem      # Start specific service
mcp-compose down               # Stop all services
mcp-compose down filesystem    # Stop specific service
mcp-compose restart            # Restart all services
mcp-compose ps                 # List service status
mcp-compose logs filesystem    # View logs
```

### System Services

System services are infrastructure components (proxy, dashboard, task-scheduler, memory).

```bash
mcp-compose system up          # Start system services
mcp-compose system down        # Stop system services
mcp-compose system ps          # List system services
mcp-compose system status      # Health overview
mcp-compose system logs proxy  # View system logs
```

### Proxy

```bash
mcp-compose proxy --port 9876              # Start proxy
mcp-compose proxy --api-key $(openssl rand -hex 32)  # With authentication
```

### Dashboard

```bash
mcp-compose dashboard          # Start web dashboard
# Access at http://localhost:3111
```

### Configuration

```bash
mcp-compose init               # Interactive setup wizard
mcp-compose validate           # Validate config file
mcp-compose create-config --type claude  # Generate client config
```

## Configuration Reference

### Basic Structure

```yaml
version: '1'

# Optional: proxy authentication
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  my-server:
    image: string              # Container image name
    protocol: string           # stdio, http, sse, tcp
    command: string            # Executable to run
    args: [string]             # Command arguments
    capabilities: [string]     # tools, resources, prompts, sampling
    environment:               # Environment variables
      KEY: value
    volumes:                   # Volume mounts
      - "host:container:mode"
```

### Protocol Types

**STDIO** (most common for NPM packages):
```yaml
servers:
  my-server:
    protocol: stdio
    command: npx
    args: ["-y", "@modelcontextprotocol/server-name"]
```

**HTTP**:
```yaml
servers:
  my-server:
    protocol: http
    http_port: 8080
```

**SSE** (Server-Sent Events):
```yaml
servers:
  my-server:
    protocol: sse
    sse_path: /events
```

### Volume Mounts

```yaml
volumes:
  - "/host/path:/container/path:ro"   # Read-only
  - "/host/path:/container/path:rw"   # Read-write
  - "${HOME}/code:/workspace:ro"      # Environment variables
```

### Environment Variables

In your config:
```yaml
environment:
  API_KEY: "${MY_API_KEY}"
```

In your shell:
```bash
export MY_API_KEY="secret"
mcp-compose up
```

## Security

Never commit secrets to your config file. Use environment variables:

```bash
# Generate secure keys
export MCP_API_KEY=$(openssl rand -hex 32)
export OAUTH_CLIENT_SECRET=$(openssl rand -hex 32)
```

Reference in config:
```yaml
proxy_auth:
  api_key: "${MCP_API_KEY}"
```

## Client Integration

### Claude Desktop

Generate config:
```bash
mcp-compose create-config --type claude --output ./claude-config
```

Copy contents to Claude Desktop settings.

### Direct HTTP

```bash
# List tools
curl -X POST http://localhost:9876/filesystem \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call tool
curl -X POST http://localhost:9876/filesystem \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"tools/call",
    "params":{
      "name":"read_file",
      "arguments":{"path":"/tmp/test.txt"}
    }
  }'
```

## Troubleshooting

### Check Service Status

```bash
mcp-compose ps
mcp-compose system status
```

### View Logs

```bash
mcp-compose logs filesystem
mcp-compose system logs proxy
```

### Validate Configuration

```bash
mcp-compose validate
```

### Common Issues

**Port already in use:**
```bash
lsof -i :9876
mcp-compose proxy --port 9877  # Use different port
```

**Container not starting:**
```bash
mcp-compose logs my-server     # Check logs
docker ps -a                   # Check container status
```

**Authentication errors:**
```bash
echo $MCP_API_KEY              # Verify key is set
```

## Requirements

- Docker 20.10+ or Podman 3.0+
- Linux, macOS, or Windows with WSL2
- Go 1.19+ (for building from source)

## Architecture

```
┌──────────────────────────────┐
│     MCP-Compose Proxy        │
│  (HTTP API + Authentication) │
└──────────────┬───────────────┘
               │
        ┌──────┼──────┐
        │      │      │
    ┌───▼──┐ ┌─▼──┐ ┌▼───┐
    │ FS   │ │ Mem│ │Search│
    │STDIO │ │HTTP│ │ SSE  │
    └──────┘ └────┘ └──────┘
```

## License

GNU Affero General Public License v3.0 - see [LICENSE](LICENSE).

## Links

- [GitHub Issues](https://github.com/phildougherty/mcp-compose/issues)
- [Model Context Protocol](https://modelcontextprotocol.io/)