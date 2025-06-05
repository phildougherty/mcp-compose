# MCP-Compose

A Docker Compose-style orchestration tool for managing Model Context Protocol (MCP) servers. MCP-Compose simplifies the deployment, configuration, and management of multiple MCP servers with a unified HTTP proxy interface.

## Features

- **Docker Compose-style Configuration**: Define MCP servers using familiar YAML syntax
- **Unified HTTP Proxy**: All MCP servers accessible through a single HTTP endpoint
- **Multiple Transport Modes**: Support for STDIO, HTTP, and Socat-hosted STDIO servers
- **Container & Process Management**: Run MCP servers as Docker containers or native processes
- **Client Integration**: Easy integration with Claude Desktop, OpenWebUI, and other MCP clients
- **Development Tools**: Built-in inspector, health checks, and logging
- **OpenAPI Integration**: Auto-generated OpenAPI specs for each server

## Quick Start

### 1. Installation

```bash
# Clone the repository
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose

# Build and install the binary
make build
make install
```

### 2. Create Configuration

Copy the example configuration:

```bash
cp mcp-compose_example.yaml mcp-compose.yaml
```

Edit `mcp-compose.yaml` to configure your servers:

```yaml
version: '1'
proxy_auth:
  enabled: true
  api_key: "your-secure-api-key"

servers:
  filesystem:
    build:
      context: ./docker_utils/socat_stdio_hoster
      dockerfile: Dockerfile.base_socat_hoster
      args:
        BASE_IMAGE: node:22-slim
    runtime: docker
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/files"]
    env:
      MCP_SOCAT_INTERNAL_PORT: "12345"
    stdio_hoster_port: 12345
    capabilities: [resources, tools]
    volumes:
      - "/path/to/your/files:/files:rw"
    networks: [mcp-net]

  github:
    build:
      context: ./docker_utils/socat_stdio_hoster
      dockerfile: Dockerfile.base_socat_hoster
      args:
        BASE_IMAGE: node:22-slim
    runtime: docker
    command: "npx"
    args: ["-y", "@modelcontextprotocol/server-github"]
    env:
      MCP_SOCAT_INTERNAL_PORT: "12346"
      GITHUB_TOKEN: "${GITHUB_TOKEN}"
    stdio_hoster_port: 12346
    capabilities: [tools, resources]
    networks: [mcp-net]

networks:
  mcp-net:
    driver: bridge
```

### 3. Set Environment Variables

```bash
export GITHUB_TOKEN="your_github_token_here"
```

### 4. Start Services

```bash
# Start all servers
./mcp-compose up

# Start specific servers
./mcp-compose up filesystem github

# Start with proxy
./mcp-compose proxy --port 9876 --api-key "your-api-key"
```

### 5. Access Your Servers

- **Proxy Dashboard**: http://localhost:9876
- **Individual Server**: http://localhost:9876/filesystem
- **OpenAPI Spec**: http://localhost:9876/filesystem/openapi.json
- **Server Documentation**: http://localhost:9876/filesystem/docs

## Configuration

### Server Configuration

Each server in the `servers` section can be configured with:

#### Basic Properties
- `command`: Command to run (for process-based servers)
- `args`: Command arguments
- `image`: Docker image (for container-based servers)
- `runtime`: Container runtime (docker, podman)

#### Transport Configuration
- `protocol`: Transport protocol (`stdio`, `http`)
- `http_port`: HTTP port for HTTP protocol servers
- `stdio_hoster_port`: Port for socat-hosted STDIO servers

#### Container Configuration
- `build`: Build configuration for custom images
- `volumes`: Volume mounts
- `networks`: Docker networks
- `ports`: Port mappings
- `env`: Environment variables

#### Capabilities
- `capabilities`: List of MCP capabilities (`tools`, `resources`, `prompts`, `sampling`, `logging`)

### Example Configurations

#### Official MCP Filesystem Server
```yaml
filesystem:
  build:
    context: ./docker_utils/socat_stdio_hoster
    dockerfile: Dockerfile.base_socat_hoster
    args:
      BASE_IMAGE: node:22-slim
  runtime: docker
  command: "npx"
  args: ["-y", "@modelcontextprotocol/server-filesystem", "/files"]
  env:
    MCP_SOCAT_INTERNAL_PORT: "12345"
  stdio_hoster_port: 12345
  capabilities: [resources, tools]
  volumes:
    - "/home/user/documents:/files:rw"
  networks: [mcp-net]
```

#### HTTP-based MCP Server
```yaml
dexcom:
  image: dexcom-mcp-http:local
  runtime: docker
  build:
    context: ./custom_mcp/dexcom
    dockerfile: Dockerfile
  protocol: http
  http_port: 8007
  env:
    HTTP_PORT: "8007"
    DEXCOM_USERNAME: "your-username"
    DEXCOM_PASSWORD: "your-password"
  capabilities: [tools, resources]
  networks: [mcp-net]
```

## Commands

### Core Commands
- `up [SERVER...]`: Start servers
- `down [SERVER...]`: Stop servers  
- `start [SERVER...]`: Start specific stopped servers
- `stop [SERVER...]`: Stop specific running servers
- `ls`: List all servers and their status
- `logs [SERVER...]`: Show server logs

### Proxy Commands
- `proxy`: Start HTTP proxy server
- `reload`: Reload proxy configuration

### Development Commands
- `inspector [SERVER]`: Launch MCP inspector
- `validate`: Validate configuration file
- `create-config`: Generate client configurations

### Examples

```bash
# Start all servers
./mcp-compose up

# Start specific servers
./mcp-compose up filesystem github

# Check status
./mcp-compose ls

# View logs
./mcp-compose logs filesystem

# Start proxy on port 9876
./mcp-compose proxy --port 9876 --api-key "mykey"

# Validate configuration
./mcp-compose validate

# Generate client config
./mcp-compose create-config --type claude --output client-configs
```

## Client Integration

### Claude Desktop

Generate Claude Desktop configuration:

```bash
./mcp-compose create-config --type claude --output client-configs
```

Or manually add to Claude Desktop settings:

```json
{
  "servers": [
    {
      "name": "filesystem",
      "httpEndpoint": "http://localhost:9876/filesystem",
      "capabilities": ["resources", "tools"],
      "description": "MCP filesystem server (via proxy)"
    }
  ]
}
```

### OpenWebUI

Use individual server OpenAPI specs:

- **Filesystem**: `http://localhost:9876/filesystem/openapi.json`
- **GitHub**: `http://localhost:9876/github/openapi.json`
- **API Key**: Use the same API key configured in `proxy_auth`

### Custom Clients

Access servers directly via HTTP:

```bash
# Initialize connection
curl -X POST http://localhost:9876/filesystem \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2024-11-05",
      "capabilities": {},
      "clientInfo": {"name": "test-client", "version": "1.0.0"}
    }
  }'

# Call a tool
curl -X POST http://localhost:9876/filesystem \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/call",
    "params": {
      "name": "read_file",
      "arguments": {"path": "/files/example.txt"}
    }
  }'
```

## Architecture

### Transport Modes

MCP-Compose supports three transport modes:

1. **STDIO**: Direct process communication via stdin/stdout
2. **HTTP**: Native HTTP communication with MCP servers
3. **Socat STDIO**: STDIO servers hosted via socat TCP proxy

### Proxy Architecture

```
Client -> HTTP Proxy -> [STDIO/HTTP/Socat] -> MCP Server
```

The proxy handles:
- Protocol translation between HTTP and STDIO/TCP
- Session management and connection pooling  
- Authentication and authorization
- Request routing to appropriate servers
- Response formatting and error handling

### Container Networking

Servers communicate via Docker networks:

```
mcp-net (bridge)
├── mcp-compose-filesystem
├── mcp-compose-github  
├── mcp-compose-memory
└── mcp-compose-proxy (when containerized)
```

## Development

### Project Structure

```
mcp-compose/
├── cmd/mcp-compose/          # Main application
├── internal/
│   ├── cmd/                  # CLI commands
│   ├── config/               # Configuration handling
│   ├── container/            # Container runtime abstraction
│   ├── server/               # Server management and proxy
│   ├── compose/              # Core orchestration logic
│   └── logging/              # Logging utilities
├── custom_mcp/               # Custom MCP server implementations
├── docker_utils/             # Docker utilities and base images
├── examples/                 # Example configurations
└── client-configs/           # Generated client configurations
```

### Building

```bash
# Build binary
go build -o mcp-compose cmd/mcp-compose/main.go

# Build with version info
go build -ldflags "-X main.version=1.0.0" -o mcp-compose cmd/mcp-compose/main.go

# Build for different platforms
GOOS=linux GOARCH=amd64 go build -o mcp-compose-linux cmd/mcp-compose/main.go
GOOS=darwin GOARCH=amd64 go build -o mcp-compose-darwin cmd/mcp-compose/main.go
```

### Adding New Servers

1. Create server directory in `custom_mcp/`
2. Add Dockerfile and implementation
3. Add server configuration to `mcp-compose.yaml`
4. Test with `./mcp-compose up your-server`

### Custom Transport Implementations

Implement the `Transport` interface in `internal/protocol/`:

```go
type Transport interface {
    Send(msg MCPMessage) error
    Receive() (MCPMessage, error)
    Close() error
}
```

## Troubleshooting

### Common Issues

**Server won't start**
- Check port conflicts with `./mcp-compose ls`
- Verify environment variables are set
- Check container logs with `./mcp-compose logs server-name`

**Proxy connection failed**
- Ensure servers are running and healthy
- Check network configurations
- Verify API key authentication

**Missing capabilities**
- Check server `capabilities` configuration
- Ensure server implements required MCP methods
- Verify server initialization succeeded

### Debug Mode

Enable verbose logging:

```bash
# Enable debug logging
./mcp-compose --verbose up

# Check proxy status
curl http://localhost:9876/api/status

# View active connections
curl http://localhost:9876/api/connections
```

### Health Checks

Monitor server health:

```yaml
servers:
  myserver:
    # ... other config
    lifecycle:
      health_check:
        endpoint: "/health"
        interval: "30s"
        timeout: "5s"
        retries: 3
        action: "restart"
```

## API Reference

### Proxy Endpoints

- `GET /` - Dashboard
- `POST /{server}` - Forward MCP request
- `GET /{server}` - Server details
- `GET /{server}/openapi.json` - Server OpenAPI spec
- `GET /{server}/docs` - Server documentation
- `GET /api/servers` - List servers
- `GET /api/status` - Proxy status
- `GET /api/discovery` - MCP discovery
- `POST /api/reload` - Reload configuration

### MCP Protocol

All servers support standard MCP methods:

- `initialize` - Initialize connection
- `tools/list` - List available tools
- `tools/call` - Call a tool
- `resources/list` - List resources
- `resources/read` - Read resource
- `prompts/list` - List prompts
- `prompts/get` - Get prompt

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

