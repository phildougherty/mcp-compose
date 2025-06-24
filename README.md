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

## Configuration Example

Here's a comprehensive `mcp-compose.yaml` showing all major configuration options:

```yaml
version: '1'

# Optional: Proxy authentication
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY:-myapikey}"

# Global connections configuration
connections:
  default:
    transport: http
    port: 9876
    expose: true
    tls: false

# Global logging configuration
logging:
  level: info
  format: json
  destinations:
    - type: stdout
    - type: file
      path: /var/log/mcp-compose.log

# Monitoring configuration
monitoring:
  metrics:
    enabled: true
    port: 9877

# Development tools
development:
  inspector:
    enabled: true
    port: 9878
  testing:
    scenarios:
      - name: basic_functionality
        tools:
          - name: read_file
            input: {"path": "/test.txt"}
            expected_status: "success"

servers:
  # HTTP-based MCP server with full configuration
  filesystem:
    image: node:22-slim
    build:
      context: ./custom_mcp/filesystem
      dockerfile: Dockerfile
      args:
        NODE_VERSION: "22"
    protocol: http
    http_port: 3000
    http_path: "/"
    command: "node"
    args: ["/app/dist/index.js", "--transport", "http", "--host", "0.0.0.0", "--port", "3000"]
    env:
      NODE_ENV: production
      LOG_LEVEL: debug
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/projects:/workspace:rw"
      - "mcp-data:/data"
    networks: [mcp-net]
    ports:
      - "3000:3000"
    # Advanced configurations
    resources:
      paths:
        - source: "/home/user/documents"
          target: "/workspace/docs"
          watch: true
          read_only: false
    security:
      auth:
        type: api_key
        header: "X-API-Key"
    lifecycle:
      pre_start: "echo 'Starting filesystem server'"
      post_start: "echo 'Filesystem server started'"
      health_check:
        endpoint: "/health"
        interval: "30s"
        timeout: "5s"
        retries: 3
        action: "restart"
    capability_options:
      resources:
        enabled: true
        list_changed: true
        subscribe: true
      tools:
        enabled: true
        list_changed: true

  # SSE-based server with cron capabilities
  cron-server:
    build:
      context: ./custom_mcp/cron
      dockerfile: Dockerfile
    protocol: sse
    http_port: 8080
    sse_path: "/sse"
    sse_heartbeat: 30
    command: "/app/mcp-cron"
    args: ["--transport", "sse", "--address", "0.0.0.0", "--port", "8080"]
    env:
      TZ: "America/New_York"
      DATABASE_PATH: "/data/cron.db"
    capabilities: [tools, resources]
    volumes:
      - "cron-data:/data"
    networks: [mcp-net]

  # STDIO server with socat hosting
  memory-server:
    build:
      context: ./docker_utils/socat_stdio_hoster
      dockerfile: Dockerfile.base_socat_hoster
      args:
        BASE_IMAGE: node:22-slim
    runtime: docker
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-memory"]
    env:
      MCP_SOCAT_INTERNAL_PORT: "12347"
    stdio_hoster_port: 12347
    capabilities: [tools, resources]
    networks: [mcp-net]

  # Database dependency example
  postgres:
    image: postgres:15-alpine
    runtime: docker
    env:
      POSTGRES_DB: mcp_data
      POSTGRES_USER: mcp
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
    volumes:
      - "postgres-data:/var/lib/postgresql/data"
    networks: [mcp-net]
    lifecycle:
      health_check:
        test: ["CMD-SHELL", "pg_isready -U mcp"]
        interval: 10s
        timeout: 5s
        retries: 5

  # Server with database dependency
  database-server:
    image: custom-mcp-db:latest
    runtime: docker
    protocol: http
    http_port: 8001
    env:
      DATABASE_URL: "postgresql://mcp:${POSTGRES_PASSWORD}@postgres:5432/mcp_data"
    capabilities: [tools, resources, prompts]
    networks: [mcp-net]
    depends_on:
      - postgres

# Network definitions
networks:
  mcp-net:
    driver: bridge
    
# Volume definitions
volumes:
  mcp-data:
    driver: local
  cron-data:
    driver: local
  postgres-data:
    driver: local

# Environment-specific configurations
environments:
  development:
    servers:
      filesystem:
        env:
          LOG_LEVEL: debug
        resources:
          sync_interval: "5s"
  production:
    servers:
      filesystem:
        env:
          LOG_LEVEL: warn
        resources:
          sync_interval: "30s"
```

## Quick Start

### Installation

```bash
# Clone and build
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
go build -o mcp-compose cmd/mcp-compose/main.go
```

### Basic Usage

```bash
# Start all servers
./mcp-compose up

# Start specific servers
./mcp-compose up filesystem memory-server

# Check server status
./mcp-compose ls

# Start the HTTP proxy
./mcp-compose proxy --port 9876 --api-key "your-api-key" --container

# View logs
./mcp-compose logs filesystem

# Stop everything
./mcp-compose down
```

## Current Limitations

### Protocol Support
- **WebSocket Transport**: Not yet implemented (planned for v2.0)
- **gRPC Transport**: Not currently supported
- **Custom Protocol Extensions**: Limited to standard MCP methods

### Container Management
- **Kubernetes Support**: Limited to Docker and Podman runtimes
- **Container Updates**: Requires manual restart for image updates
- **Resource Limits**: Basic memory/CPU limiting not implemented in config

### Proxy Features
- **Load Balancing**: Single proxy instance, no clustering support
- **Request Queuing**: No built-in request queue management for high load
- **Rate Limiting**: Authentication only, no per-client rate limiting

### MCP Protocol Compliance
- **Nested Transactions**: Complex transaction scenarios may have edge cases
- **Large Payloads**: File transfers over 10MB may timeout
- **Concurrent Sessions**: Limited testing with high concurrent session counts

### Development Tools
- **Distributed Tracing**: No integration with OpenTelemetry or Jaeger
- **Performance Profiling**: Basic metrics only, no detailed profiling
- **Test Framework**: Limited automated testing for complex multi-server scenarios

## OpenWebUI Integration

### Server-Specific Configuration
Each server provides its own OpenAPI endpoint:

```bash
# Filesystem server
http://localhost:9876/filesystem/openapi.json

# Memory server  
http://localhost:9876/memory-server/openapi.json

# Cron server
http://localhost:9876/cron-server/openapi.json
```

### Authentication Setup
Use the same API key for all servers:
```json
{
  "api_key": "your-configured-api-key",
  "base_url": "http://localhost:9876"
}
```

## Claude Desktop Integration

### Automatic Configuration Generation
```bash
./mcp-compose create-config --type claude --output claude-config.json
```

### Manual Configuration
Add to Claude Desktop settings:
```json
{
  "mcpServers": {
    "mcp-compose-filesystem": {
      "command": "curl",
      "args": [
        "-X", "POST",
        "-H", "Content-Type: application/json", 
        "-H", "Authorization: Bearer your-api-key",
        "http://localhost:9876/filesystem"
      ],
      "capabilities": ["resources", "tools"]
    }
  }
}
```

## API Reference

### Core Endpoints
- `GET /` - Interactive dashboard with server status
- `POST /{server}` - Forward MCP JSON-RPC requests
- `GET /{server}` - Server details and capabilities
- `DELETE /{server}` - Terminate server session
- `GET /{server}/openapi.json` - Server-specific OpenAPI spec
- `GET /{server}/docs` - Interactive documentation

### Management API
- `GET /api/servers` - Detailed server status and configuration
- `GET /api/status` - Overall proxy health and metrics
- `GET /api/connections` - Active connection details
- `GET /api/discovery` - MCP discovery endpoint for clients
- `POST /api/reload` - Hot reload configuration

### Direct Tool Access
- `POST /{tool_name}` - Direct tool invocation (FastAPI-style)

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
