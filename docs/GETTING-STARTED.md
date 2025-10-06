# Getting Started with MCP-Compose

This guide will get you running MCP servers in under 5 minutes.

## Prerequisites

- Docker or Podman installed
- Node.js installed (for npx)
- Git (for building from source)

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

## Option 1: Interactive Setup

The easiest way to get started:

```bash
mcp-compose init
```

Follow the prompts to:
1. Choose which MCP servers to enable
2. Configure paths and options
3. Generate your `mcp-compose.yaml`

Then start everything:

```bash
mcp-compose up
mcp-compose proxy --port 9876
```

## Option 2: Manual Configuration

### Minimal Setup

Create `mcp-compose.yaml`:

```yaml
version: '1'
servers:
  filesystem:
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "/tmp"
    capabilities: [resources, tools]
```

Start it:

```bash
mcp-compose up
```

### Basic Setup (Recommended)

`mcp-compose.yaml` with filesystem, memory, and search:

```yaml
version: '1'
servers:
  filesystem:
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "${HOME}"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}:${HOME}:ro"

  memory:
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-memory"
    capabilities: [resources, tools]

  brave-search:
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-brave-search"
    capabilities: [tools]
    env:
      BRAVE_API_KEY: "${BRAVE_API_KEY}"
```

Set required environment variables:

```bash
export BRAVE_API_KEY="your-api-key"  # Get from https://brave.com/search/api/
```

Start everything:

```bash
mcp-compose up
```

## Starting the Proxy

The proxy provides a unified HTTP endpoint for all your servers:

```bash
mcp-compose proxy --port 9876
```

Access endpoints:
- Dashboard: http://localhost:9876/
- API Status: http://localhost:9876/api/servers
- OpenAPI Spec: http://localhost:9876/openapi.json

## Testing Your Setup

### Check Server Status

```bash
mcp-compose ps
```

### View Logs

```bash
mcp-compose logs filesystem
```

### Test the API

```bash
# List available servers
curl http://localhost:9876/api/servers

# List tools for filesystem server
curl -X POST http://localhost:9876/filesystem \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Read a file
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

## Available MCP Servers

All official MCP servers from Anthropic work with mcp-compose:

| Server | Package | Description |
|--------|---------|-------------|
| filesystem | @modelcontextprotocol/server-filesystem | Read/write/search files |
| memory | @modelcontextprotocol/server-memory | Persistent key-value storage |
| git | @modelcontextprotocol/server-git | Git repository operations |
| github | @modelcontextprotocol/server-github | GitHub API access |
| brave-search | @modelcontextprotocol/server-brave-search | Web search |
| postgres | @modelcontextprotocol/server-postgres | PostgreSQL database |
| puppeteer | @modelcontextprotocol/server-puppeteer | Browser automation |
| slack | @modelcontextprotocol/server-slack | Slack API access |
| gdrive | @modelcontextprotocol/server-gdrive | Google Drive |
| google-maps | @modelcontextprotocol/server-google-maps | Location services |

See [examples/complete.yaml](examples/complete.yaml) for configuration of all servers.

## Common Tasks

### Add a New Server

Edit `mcp-compose.yaml` and add:

```yaml
servers:
  github:
    protocol: stdio
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-github"
    capabilities: [tools, resources]
    env:
      GITHUB_PERSONAL_ACCESS_TOKEN: "${GITHUB_TOKEN}"
```

Set environment variable:
```bash
export GITHUB_TOKEN="ghp_your_token_here"
```

Restart:
```bash
mcp-compose restart
```

### Enable Authentication

Add to your `mcp-compose.yaml`:

```yaml
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"
```

Generate and set API key:
```bash
export MCP_API_KEY=$(openssl rand -hex 32)
mcp-compose proxy --port 9876
```

Use in requests:
```bash
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/api/servers
```

### Configure Claude Desktop

Generate Claude Desktop config:

```bash
mcp-compose create-config --type claude --output ./claude-config
```

Copy the generated config to Claude Desktop settings.

## Troubleshooting

### Server Won't Start

Check logs:
```bash
mcp-compose logs my-server
```

Validate config:
```bash
mcp-compose validate
```

### Port Already in Use

Use a different port:
```bash
mcp-compose proxy --port 9877
```

### Environment Variable Not Set

Check what's set:
```bash
echo $BRAVE_API_KEY
```

### Container Issues

Check Docker:
```bash
docker ps -a
```

View container logs:
```bash
docker logs mcp-compose-my-server
```

## Next Steps

- See [README.md](README.md) for complete documentation
- Check [examples/](examples/) for more configurations
- View [mcp-compose-advanced.yaml](mcp-compose-advanced.yaml) for all options

## Getting Help

- GitHub Issues: https://github.com/phildougherty/mcp-compose/issues
- MCP Documentation: https://modelcontextprotocol.io/