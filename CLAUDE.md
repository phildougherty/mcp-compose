# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Quick Reference

### New Users
- **[Getting Started Guide](GETTING-STARTED.md)** - 10-minute setup tutorial
- **[Basic Examples](mcp-compose-basic.yaml)** - Simple 3-server configuration
- **[Quickstart](mcp-compose-quickstart.yaml)** - Minimal 1-server configuration

### Migration
- **[Migration Guide](MIGRATION.md)** - Migrate from existing MCP setups
- **[Docker Compose Migration](MIGRATION.md#from-docker-compose--mcp-compose)** - Convert docker-compose.yml
- **[Individual Servers Migration](MIGRATION.md#from-individual-mcp-servers--mcp-compose)** - Replace manual server management

### Advanced Configuration
- **[Enterprise Configuration](mcp-compose-advanced.yaml)** - OAuth, audit logging, monitoring
- **[Security Best Practices](README.md#security-notice)** - Production security
- **[Performance Tuning](README.md#troubleshooting)** - Optimization guides

## Common Commands

### Basic Usage
```bash
# Quick start (see GETTING-STARTED.md for details)
./mcp-compose up                    # Start all servers
./mcp-compose proxy --port 9876     # Start HTTP proxy
./mcp-compose ls                    # List server status
./mcp-compose down                  # Stop all servers
```

### Development
```bash
make build          # Build the application to build/mcp-compose
make test          # Run all tests
make clean         # Remove build artifacts
```

### Advanced
```bash
./mcp-compose up [server-name]      # Start specific server
./mcp-compose logs [server-name]    # View logs
./mcp-compose restart [server-name] # Restart server
./mcp-compose create-config --type claude  # Generate client config
```

## Architecture Overview

MCP-Compose is a comprehensive orchestration tool for managing Model Context Protocol (MCP) servers. It provides Docker Compose-style configuration with support for multiple transport protocols.

### Key Components

#### Core Modules
- **cmd/**: CLI commands using Cobra framework
- **internal/config/**: Configuration parsing and validation
- **internal/compose/**: Server lifecycle management
- **internal/container/**: Docker/Podman runtime abstraction
- **internal/protocol/**: MCP protocol implementation
- **internal/server/**: HTTP proxy and API handlers
- **internal/dashboard/**: Web-based monitoring interface

#### Transport Protocols
- **STDIO**: Process-based servers with socat TCP bridging
- **HTTP**: Native HTTP MCP servers with connection pooling
- **SSE**: Server-Sent Events for real-time communication
- **TCP**: Raw TCP connections

#### Server Management
- **Manager**: Central server lifecycle coordination
- **Runtime**: Process/container execution abstraction
- **Proxy**: HTTP-to-MCP protocol translation
- **Sessions**: Persistent connection management

### Configuration
- Primary config: `mcp-compose.yaml` (Docker Compose-style)
- Example config: `mcp-compose_example.yaml`
- Supports environment variables and templating

### Key Features
- Multi-transport MCP protocol support
- Container and process management
- HTTP proxy with OpenAPI generation
- Real-time dashboard and monitoring
- Built-in MCP inspector for debugging
- OpenWebUI and Claude Desktop integration

## Project Structure

```
cmd/mcp-compose/         # Main CLI entry point
internal/
├── cmd/                 # CLI command implementations
├── config/              # Configuration management
├── compose/             # Server orchestration
├── container/           # Runtime abstraction (Docker/Podman)
├── protocol/            # MCP protocol implementation
├── server/              # HTTP proxy and API handlers
├── dashboard/           # Web UI and monitoring
├── auth/                # Authentication and OAuth
├── memory/              # Persistence management
├── audit/               # Logging and audit trails
└── task_scheduler/      # Cron-like task scheduling
custom_mcp/              # Example MCP server implementations
client-config/           # Client configuration examples
```

## Security Requirements

⚠️ **CRITICAL**: This project handles sensitive credentials and API keys. Follow these security practices:

### Environment Variables
Always use environment variables for sensitive data:
- `MCP_API_KEY`: Main proxy authentication
- `POSTGRES_PASSWORD`: Database credentials
- `GITHUB_TOKEN`: GitHub API access
- `OPENROUTER_API_KEY`: OpenRouter API access
- `OAUTH_CLIENT_SECRET`: OAuth client credentials

### Configuration Security
- Never commit hardcoded secrets in `mcp-compose.yaml`
- Use `.env.example` as template for environment variables
- Run containers as non-root users (e.g., `user: "1000:1000"`)
- Drop unnecessary capabilities with `cap_drop: ["ALL"]`
- Use `security_opt: ["no-new-privileges:true"]`

### Default Security Policies
- Containers run as non-root by default
- Privileged mode is disabled
- Host mounts are restricted to safe directories
- Resource limits are enforced

## Development Notes

### Go Module
- Module name: `mcpcompose`
- Go version: 1.19+
- Key dependencies: Cobra (CLI), Gorilla WebSocket, PostgreSQL driver

### Testing
- No dedicated test files found - tests should be added for new features
- Use `go test ./...` for running tests

### Build Process
- Uses standard Go build toolchain
- Makefile provides common build tasks
- Binary output: `build/mcp-compose`

### Protocol Implementation
- Full MCP JSON-RPC 2.0 compliance
- Supports all MCP capabilities: tools, resources, prompts
- WebSocket and HTTP transport layers
- OpenAPI 3.1.0 specification generation

## Production Readiness Features

### Error Handling
- Consistent error handling patterns throughout codebase
- Proper error propagation and logging
- Graceful fallbacks for missing features
- No panic() calls in production code paths

### Resource Management
- Proper goroutine lifecycle management with WaitGroups
- Channel-based shutdown signaling
- Resource cleanup with configurable timeouts
- Connection pooling and cleanup

### Configurable Timeouts
All timeouts are configurable via the `connections.timeouts` section:
```yaml
connections:
  default:
    timeouts:
      connect: "10s"        # Connection timeout
      read: "30s"           # Read timeout for large operations
      write: "30s"          # Write timeout for long responses
      idle: "60s"           # Keep-alive timeout
      health_check: "5s"    # Health check timeout
      shutdown: "30s"       # Graceful shutdown timeout
      lifecycle_hook: "30s" # Hook execution timeout
```

### Graceful Shutdown
- Context-based cancellation throughout the application
- Proper resource cleanup order: connections → servers → networks
- Configurable shutdown timeouts
- WaitGroup coordination for background goroutines

### Health Monitoring
- Configurable health check intervals and timeouts
- Automatic restart policies for failed containers
- Resource usage monitoring and limits
- Audit logging with retention policies