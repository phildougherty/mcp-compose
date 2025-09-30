# MCP-Compose

[![codecov](https://codecov.io/gh/phildougherty/mcp-compose/branch/main/graph/badge.svg)](https://codecov.io/gh/phildougherty/mcp-compose)
[![Go Report Card](https://goreportcard.com/badge/github.com/phildougherty/mcp-compose)](https://goreportcard.com/report/github.com/phildougherty/mcp-compose)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Release](https://img.shields.io/github/v/release/phildougherty/mcp-compose)](https://github.com/phildougherty/mcp-compose/releases)

**Docker Compose for Model Context Protocol (MCP) servers.** Orchestrate multiple MCP servers with a single YAML file, unified HTTP proxy, and production-grade security.

![MCP-Compose Demo](demo.gif)

## Why MCP-Compose?

**Traditional MCP Setup:**
```bash
npx @modelcontextprotocol/server-filesystem /path/to/files &
npx @modelcontextprotocol/server-memory &
npx @modelcontextprotocol/server-git /path/to/repo &
```
Managing ports, authentication, logs, and restarts for each server... painful.

**With MCP-Compose:**
```yaml
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
  git:
    image: "mcp/git:latest"
    capabilities: [tools]
```
```bash
mcp-compose up
```
Done. All servers running, single HTTP endpoint, authentication, health checks, and logs unified.

## Key Features

- **Docker Compose-Style Configuration** - Familiar YAML syntax, zero learning curve
- **Unified HTTP Proxy** - Single endpoint for all MCP servers with automatic protocol translation
- **Multiple Protocols** - Native support for STDIO, HTTP, SSE, and TCP transports
- **Production Security** - OAuth 2.1, RBAC, audit logging, and container hardening
- **Session Management** - Persistent connections with automatic pooling and health monitoring
- **Client Integration** - Works with Claude Desktop, OpenWebUI, and custom clients
- **Developer Tools** - Built-in inspector, real-time dashboard, OpenAPI docs
- **Container & Process Management** - Docker, Podman, or native processes

## Quick Start

### 30-Second Setup

```bash
# Install
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose

# Create config
cat > mcp-compose.yaml << 'EOF'
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
EOF

# Start
export MCP_API_KEY=$(openssl rand -hex 32)
mcp-compose up

# In another terminal
mcp-compose proxy --port 9876
```

Your MCP servers are now running at `http://localhost:9876`!

### Test It

```bash
# List servers
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/api/servers

# List tools
curl -H "Authorization: Bearer $MCP_API_KEY" \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
  http://localhost:9876/filesystem
```

## Documentation

### Getting Started

- **[5-Minute Quickstart](docs/getting-started/quickstart.md)** - Complete tutorial from zero to working setup
- **[Basic Example](examples/basic.yaml)** - 3 servers (filesystem, memory, search)
- **[Installation Guide](docs/getting-started/quickstart.md#installation)** - Binary, source, and Docker

### Configuration

- **[Configuration Reference](docs/configuration/reference.md)** - Complete reference for all options
- **[Examples Directory](examples/)** - Quickstart, basic, development, advanced, production
- **[Profiles](config/profiles/)** - Pre-configured development, staging, production profiles

### Advanced

- **[Architecture Overview](docs/architecture/overview.md)** - System design, request flow, component descriptions
- **[Troubleshooting Guide](docs/troubleshooting/guide.md)** - Common errors, debug mode, performance tuning
- **[Security Best Practices](docs/configuration/reference.md#security-configuration)** - Production hardening

## Use Cases

### For Developers

```yaml
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
      - "${HOME}/code:/workspace:rw"

  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
```

### For Content Creators

```yaml
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

  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
```

### For Enterprise

See [examples/production.yaml](examples/production.yaml) for:
- OAuth 2.1 authentication
- RBAC with role-based scopes
- PostgreSQL audit logging
- Security hardening
- Resource limits
- Health monitoring
- Network segmentation

## Features In-Depth

### Multiple Transport Protocols

MCP-Compose automatically handles protocol translation:

```yaml
servers:
  stdio-server:
    protocol: "stdio"        # Legacy STDIO servers
    command: "/app/server"

  http-server:
    protocol: "http"         # Modern HTTP servers
    http_port: 8080

  sse-server:
    protocol: "sse"          # Real-time streaming
    sse_path: "/sse"

  tcp-server:
    protocol: "tcp"          # Raw TCP connections
```

All accessible via unified HTTP proxy.

### Production-Grade Security

```yaml
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

oauth:
  enabled: true
  security:
    require_pkce: true

rbac:
  enabled: true
  roles:
    admin:
      scopes: ["mcp:*"]
    user:
      scopes: ["mcp:tools", "mcp:resources"]

audit:
  enabled: true
  storage: "postgres"
  events:
    - "oauth.token.issued"
    - "tool.executed"

servers:
  filesystem:
    user: "1000:1000"
    read_only: true
    cap_drop: ["ALL"]
    security_opt:
      - "no-new-privileges:true"
    authentication:
      enabled: true
      required_scope: "mcp:resources"
```

### Resource Management

```yaml
servers:
  web-server:
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: "512m"
          pids: 100
        reservations:
          cpus: "0.5"
          memory: "256m"
      restart_policy: "on-failure"

    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: "30s"
      timeout: "5s"
      retries: 3

    lifecycle:
      pre_start: "echo 'Starting...'"
      post_start: "curl http://localhost:8080/warmup"
```

### Client Integration

#### Claude Desktop

```bash
mcp-compose create-config --type claude --output ./claude-config
# Copy to Claude Desktop config location
```

#### OpenWebUI

```bash
# Each server provides OpenAPI spec
curl http://localhost:9876/filesystem/openapi.json
```

#### Custom Clients

```bash
# Standard JSON-RPC 2.0 over HTTP
curl -H "Authorization: Bearer $MCP_API_KEY" \
  -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"/workspace/README.md"}}}' \
  http://localhost:9876/filesystem
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    MCP-Compose Proxy                     │
│  ┌──────────┬───────────────┬──────────┬────────────┐  │
│  │ HTTP API │ Authentication│  Router  │  Session   │  │
│  │  Server  │     Layer     │          │  Manager   │  │
│  └──────────┴───────────────┴──────────┴────────────┘  │
└─────────────────────────────────────────────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
    ┌─────▼─────┐      ┌────▼────┐      ┌─────▼─────┐
    │ Filesystem│      │  Memory │      │   Search  │
    │   STDIO   │      │   HTTP  │      │    SSE    │
    └───────────┘      └─────────┘      └───────────┘
          │                  │                  │
    ┌─────▼──────────────────▼──────────────────▼─────┐
    │           Docker / Podman Runtime                │
    └──────────────────────────────────────────────────┘
```

See [Architecture Overview](docs/architecture/overview.md) for detailed diagrams and component descriptions.

## Comparison with Alternatives

| Feature | MCP-Compose | Manual Setup | Docker Compose | Kubernetes |
|---------|-------------|--------------|----------------|------------|
| MCP Protocol Support | Yes (Native) | Yes (Manual) | No | No |
| Single Config File | Yes | No | Yes | No (Complex) |
| Unified HTTP Proxy | Yes (Built-in) | No (Manual) | No | Yes (Ingress) |
| Session Management | Yes (Automatic) | No (Manual) | No | Yes (Complex) |
| Authentication | Yes (OAuth/API Key) | No (DIY) | No | Yes (Complex) |
| Setup Time | 30 seconds | Hours | 15 minutes | Days |
| Learning Curve | Low | Medium | Low | High |

## Common Commands

```bash
# Start all servers
mcp-compose up

# Start specific server
mcp-compose up filesystem

# Stop all servers
mcp-compose down

# View server status
mcp-compose ls

# View logs
mcp-compose logs filesystem

# Restart a server
mcp-compose restart filesystem

# Start proxy
mcp-compose proxy --port 9876

# Validate configuration
mcp-compose validate

# Generate client config
mcp-compose create-config --type claude
```

## Performance

Typical characteristics:

| Metric | Value |
|--------|-------|
| Proxy latency (p50) | 5-10ms |
| Proxy latency (p99) | 15-30ms |
| Concurrent connections | 1000+ |
| Request throughput | 10,000+ req/sec |
| Memory (proxy) | 50-100MB base |
| Cold start | 100-500ms |

## Requirements

- **Docker** (20.10+) or **Podman** (3.0+)
- **Linux, macOS, or Windows with WSL2**
- **Go 1.19+** (if building from source)

## Installation

### Pre-built Binaries

```bash
# Linux (amd64)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose

# macOS (Intel)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-darwin-amd64
chmod +x mcp-compose-darwin-amd64
sudo mv mcp-compose-darwin-amd64 /usr/local/bin/mcp-compose

# macOS (Apple Silicon)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-darwin-arm64
chmod +x mcp-compose-darwin-arm64
sudo mv mcp-compose-darwin-arm64 /usr/local/bin/mcp-compose
```

### Build from Source

```bash
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
make build
sudo cp build/mcp-compose /usr/local/bin/
```

## Security Notice

MCP-Compose handles sensitive credentials. Follow these critical security practices:

### Required Environment Variables

```bash
# Core authentication
export MCP_API_KEY=$(openssl rand -hex 32)

# OAuth (if using)
export OAUTH_CLIENT_SECRET=$(openssl rand -hex 32)

# Database (if using PostgreSQL)
export POSTGRES_PASSWORD=$(openssl rand -hex 32)
```

### Never Commit Secrets

```yaml
# WRONG - hardcoded secret
proxy_auth:
  api_key: "my-secret-key-12345"

# CORRECT - environment variable
proxy_auth:
  api_key: "${MCP_API_KEY}"
```

### Container Security Best Practices

```yaml
servers:
  my-server:
    user: "1000:1000"              # Run as non-root
    read_only: true                # Read-only filesystem
    cap_drop: ["ALL"]              # Drop all capabilities
    security_opt:
      - "no-new-privileges:true"   # Prevent privilege escalation
    deploy:
      resources:
        limits:
          memory: "512m"           # Limit resources
          pids: 100
```

See [Configuration Reference](docs/configuration/reference.md#security-configuration) for complete security guide.

## Migration

### From Docker Compose

```yaml
# docker-compose.yml
version: '3.8'
services:
  my-service:
    image: "app:latest"
    ports: ["8080:8080"]

# mcp-compose.yaml
version: '1'
servers:
  my-service:
    image: "app:latest"
    protocol: "http"
    http_port: 8080
    capabilities: [tools]
```

### From Manual MCP Servers

```bash
# Before
npx @modelcontextprotocol/server-filesystem /path &
npx @modelcontextprotocol/server-memory &

# After
cat > mcp-compose.yaml << 'EOF'
version: '1'
servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "/path:/workspace:ro"
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
EOF
mcp-compose up
```

## Examples

- **[quickstart.yaml](examples/quickstart.yaml)** - Minimal 1-server setup (30 seconds)
- **[basic.yaml](examples/basic.yaml)** - 3 common servers (2 minutes)
- **[development.yaml](examples/development.yaml)** - Development environment (3 minutes)
- **[advanced.yaml](examples/advanced.yaml)** - 12 servers with OAuth and monitoring (10 minutes)
- **[production.yaml](examples/production.yaml)** - Security-hardened production (15 minutes)

## Troubleshooting

### Common Issues

**"Server not found"**
```bash
mcp-compose ls                    # Check server status
mcp-compose logs filesystem       # View logs
```

**"Connection refused"**
```bash
mcp-compose proxy --port 9876     # Start proxy
lsof -i :9876                     # Check port
```

**"Authentication failed"**
```bash
echo $MCP_API_KEY                 # Verify key is set
grep "api_key" mcp-compose.yaml   # Check config
```

See [Troubleshooting Guide](docs/troubleshooting/guide.md) for complete solutions.

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Code style guidelines
- Testing requirements
- Pull request process
- Development setup

## Community

- **GitHub Issues**: [Report bugs or request features](https://github.com/phildougherty/mcp-compose/issues)
- **GitHub Discussions**: [Ask questions and share ideas](https://github.com/phildougherty/mcp-compose/discussions)
- **Documentation**: [Full documentation](docs/)

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Roadmap

### v1.0 (Current)
- Docker/Podman orchestration (Complete)
- Multi-protocol support (STDIO, HTTP, SSE, TCP) (Complete)
- Unified HTTP proxy (Complete)
- API key authentication (Complete)
- OAuth 2.1 completion (In Progress)
- RBAC implementation (In Progress)
- Audit logging (In Progress)

### v1.1 (Next)
- Connection pooling optimization
- Response caching
- Rate limiting
- Enhanced monitoring
- AI-powered context management

### v2.0 (Future)
- Kubernetes support (branch: `kubernetes-native`)
- Plugin system
- Advanced caching
- Multi-tenancy
- Distributed tracing

## Acknowledgments

Built with:
- [Model Context Protocol](https://modelcontextprotocol.io/) by Anthropic
- [Docker](https://www.docker.com/) / [Podman](https://podman.io/)
- [Go](https://golang.org/)
- [Cobra](https://github.com/spf13/cobra) CLI framework

## Star History

If you find MCP-Compose useful, please consider giving it a star ⭐

[![Star History Chart](https://api.star-history.com/svg?repos=phildougherty/mcp-compose&type=Date)](https://star-history.com/#phildougherty/mcp-compose&Date)

---

**Made with love for the MCP community**