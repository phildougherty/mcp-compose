# Quickstart Guide

Get MCP-Compose running in 5 minutes. This guide will take you from zero to a working MCP server setup.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Your First Server](#your-first-server)
- [Start the Servers](#start-the-servers)
- [Test Your Setup](#test-your-setup)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

- **Docker** (20.10+) or **Podman** (3.0+) installed and running
- **Linux, macOS, or Windows with WSL2**
- **5 minutes of your time**

Verify Docker/Podman is running:

```bash
docker ps
# or
podman ps
```

## Installation

### Option 1: Download Pre-built Binary (Recommended)

```bash
# Linux (amd64)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose

# macOS (amd64)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-darwin-amd64
chmod +x mcp-compose-darwin-amd64
sudo mv mcp-compose-darwin-amd64 /usr/local/bin/mcp-compose

# macOS (arm64 - Apple Silicon)
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-darwin-arm64
chmod +x mcp-compose-darwin-arm64
sudo mv mcp-compose-darwin-arm64 /usr/local/bin/mcp-compose
```

### Option 2: Build from Source

```bash
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
make build
sudo cp build/mcp-compose /usr/local/bin/
```

Verify installation:

```bash
mcp-compose --version
```

## Your First Server

Create a `mcp-compose.yaml` file in your project directory:

```yaml
version: '1'

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
```

That's it! This minimal configuration defines one MCP server that provides filesystem access.

### Understanding the Configuration

- `version: '1'` - Configuration format version
- `servers:` - List of MCP servers to run
- `filesystem:` - Server name (can be anything you want)
- `image:` - Docker image to use
- `capabilities:` - What the server can do (`resources`, `tools`, `prompts`)

## Start the Servers

### Step 1: Generate an API Key

```bash
export MCP_API_KEY=$(openssl rand -hex 32)
echo "Your API key: $MCP_API_KEY"
```

Save this key somewhere safe - you'll need it to access your servers.

### Step 2: Start the Servers

```bash
mcp-compose up
```

You should see output like:

```
Starting MCP servers...
filesystem server started successfully
All servers are running
```

### Step 3: Start the HTTP Proxy

In a new terminal window:

```bash
mcp-compose proxy --port 9876
```

Output:

```
MCP Proxy started on :9876
Ready to accept connections
```

Your MCP servers are now running and accessible via HTTP!

## Test Your Setup

### List Available Servers

```bash
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/api/servers
```

Expected response:

```json
{
  "servers": [
    {
      "name": "filesystem",
      "status": "running",
      "capabilities": ["resources", "tools"]
    }
  ]
}
```

### List Available Tools

```bash
curl -H "Authorization: Bearer $MCP_API_KEY" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' \
  http://localhost:9876/filesystem
```

Expected response:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "tools": [
      {
        "name": "read_file",
        "description": "Read contents of a file",
        "inputSchema": { ... }
      },
      {
        "name": "write_file",
        "description": "Write contents to a file",
        "inputSchema": { ... }
      }
    ]
  }
}
```

### Call a Tool

```bash
curl -H "Authorization: Bearer $MCP_API_KEY" \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc":"2.0",
    "id":1,
    "method":"tools/call",
    "params":{
      "name":"read_file",
      "arguments":{"path":"/workspace/README.md"}
    }
  }' \
  http://localhost:9876/filesystem
```

## Next Steps

Congratulations! You have a working MCP-Compose setup. Here's what to explore next:

### Add More Servers

Update your `mcp-compose.yaml`:

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
```

Restart your servers:

```bash
mcp-compose down
mcp-compose up
```

### Connect to Claude Desktop

Generate a Claude Desktop configuration:

```bash
mcp-compose create-config --type claude --output ./claude-config
```

Copy the generated configuration to:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Linux**: `~/.config/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

### Explore More

- **[Configuration Reference](../configuration/reference.md)** - All configuration options
- **[Architecture Overview](../architecture/overview.md)** - How MCP-Compose works
- **[Troubleshooting Guide](../troubleshooting/guide.md)** - Common issues and solutions
- **[Examples](../../examples/)** - More configuration examples

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
```

## Troubleshooting Quick Fixes

### "Server not found"

```bash
# Check server status
mcp-compose ls

# Check logs for errors
mcp-compose logs filesystem
```

### "Connection refused"

```bash
# Ensure proxy is running
mcp-compose proxy --port 9876

# Check if port is available
lsof -i :9876
```

### "Authentication failed"

```bash
# Verify API key is set
echo $MCP_API_KEY

# Check configuration
grep "api_key" mcp-compose.yaml
```

## Getting Help

- **Documentation**: [Full documentation](../)
- **GitHub Issues**: [Report bugs or request features](https://github.com/phildougherty/mcp-compose/issues)
- **Examples**: [Configuration examples](../../examples/)

You're all set! Start building with MCP-Compose.