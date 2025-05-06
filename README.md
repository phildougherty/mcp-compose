# MCP-Compose: Model Context Protocol Server Orchestration

MCP-Compose is a powerful tool for defining, configuring, and managing Model Context Protocol (MCP) servers. It simplifies the process of setting up and orchestrating multiple MCP servers that can be used with AI models like Claude, GPT, and other LLM clients.


!!! DISCLAIMER !!! 
This is a work in progress and things are likely broken

## Table of Contents

- [Overview](#overview)
- [Installation](#installation)
- [Core Concepts](#core-concepts)
- [Basic Commands](#basic-commands)
- [Advanced Features](#advanced-features)
- [Server Configuration](#server-configuration)
- [Network Configuration](#network-configuration)
- [Development & Debugging](#development--debugging)
- [Client Configurations](#client-configurations)
- [Proxy Server](#proxy-server)
- [Examples](#examples)
- [Contributing](#contributing)
- [License](#license)

## Overview

MCP-Compose allows you to:
- Define multiple MCP servers in a single YAML file
- Start, stop, and manage the lifecycle of all your MCP services
- Run MCP servers as Docker containers or direct processes
- Configure capabilities, tools, and resources for each server
- Generate client configuration for popular LLM clients like Claude Desktop and OpenAI API
- Debug MCP servers with the built-in inspector
- Create a proxy to expose your MCP servers through a unified endpoint

## Installation

### Building from Source

```bash
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose
go build -o mcp-compose cmd/mcp-compose/main.go
```

## Core Concepts

MCP-Compose is built around a few key concepts:

- **Servers**: Individual MCP servers that provide specific capabilities
- **Capabilities**: Features that a server provides (resources, tools, etc.)
- **Connections**: How MCP clients communicate with the servers
- **Networks**: How containers communicate with each other

## Basic Commands

```bash
# Start all servers
mcp-compose up

# Start specific servers
mcp-compose up server1 server2

# Stop all servers
mcp-compose down

# List all servers and their status
mcp-compose ls

# View server logs
mcp-compose logs [SERVER...]

# Follow logs in real-time
mcp-compose logs -f [SERVER...]

# Start specific servers
mcp-compose start server1 server2

# Stop specific servers
mcp-compose stop server1 server2

# Validate your configuration
mcp-compose validate

# Generate shell completion
mcp-compose completion bash|zsh|fish|powershell
```

Global flags:
- `-c, --file`: Specify compose file (default: "mcp-compose.yaml")
- `-v, --verbose`: Enable verbose output

## Advanced Features

### Create Client Configuration

Generate client configurations for various LLM clients.

```bash
# Create config for all client types
mcp-compose create-config

# Create config for a specific client type
mcp-compose create-config --type claude

# Specify output directory
mcp-compose create-config --output my-configs
```

Flags:
- `-o, --output`: Output directory for client configurations (default: "client-configs")
- `-t, --type`: Client type (claude, anthropic, openai, all) (default: "all")

### Inspector

Launch an interactive debugger for MCP servers.

```bash
# Launch inspector for all servers
mcp-compose inspector

# Launch inspector for a specific server
mcp-compose inspector weather

# Specify port
mcp-compose inspector --port 8080
```

Flags:
- `-p, --port`: Port to run the inspector on (default: 8090)

### Proxy Server

Run a proxy that exposes all MCP servers through a unified HTTP endpoint.

```bash
# Start a proxy server
mcp-compose proxy

# Specify port
mcp-compose proxy --port 9000

# Run in background
mcp-compose proxy --detach

# Only generate client configuration
mcp-compose proxy --generate-config
```

Flags:
- `-p, --port`: Port to run the proxy on (default: 9876)
- `-g, --generate-config`: Generate client configuration file only
- `-t, --client`: Client type for config generation (claude, openai, anthropic, all) (default: "claude")
- `-o, --output`: Output directory for client configuration (default: "client-config")
- `-d, --detach`: Run proxy server in the background
- `-C, --container`: Run proxy server as a container (default: true)

## Server Configuration

MCP-Compose allows detailed configuration of servers through a YAML file. Here's a comprehensive example:

```yaml
version: '1'
servers:
  filesystem:
    # Process-based server
    command: node
    args: ["server.js", "/data"]
    # OR container-based server
    image: mcp/filesystem-server
    runtime: docker  # docker or podman
    pull: true       # pull image before starting
    
    # Common settings
    workdir: /app
    env:
      DEBUG: "true"
      ROOT_DIR: "/data"
    ports:
      - "3000:3000"
    volumes:
      - "./data:/data"
    capabilities:
      - resources
      - tools
    depends_on:
      - another-server
      
    # Enhanced settings
    resources:
      paths:
        - source: "./data"
          target: "/data"
          watch: true
          read_only: false
      sync_interval: "5s"
      cache_ttl: 300
    
    tools:
      - name: "readFile"
        description: "Read a file from the filesystem"
        parameters:
          - name: "path"
            type: "string"
            required: true
            description: "Path to the file"
        
    lifecycle:
      pre_start: "./scripts/pre-start.sh"
      post_start: "./scripts/post-start.sh"
      pre_stop: "./scripts/pre-stop.sh"
      post_stop: "./scripts/post-stop.sh"
      health_check:
        endpoint: "/health"
        interval: "10s"
        timeout: "2s"
        retries: 3
        action: "restart"  # Action to take when health check fails
    
    networks:
      - mcp-net
```

## Network Configuration

Configure how your MCP servers communicate with each other.

```yaml
networks:
  mcp-net:
    driver: bridge
  external-net:
    external: true
```

## Development & Debugging

Configure development-specific settings.

```yaml
development:
  inspector:
    enabled: true
    port: 8090
  testing:
    scenarios:
      - name: "read-file-test"
        tools:
          - name: "readFile"
            input:
              path: "/test.txt"
            expected_status: "success"
```

## Client Configurations

MCP-Compose can generate configuration files for various LLM clients:

### Claude Desktop Configuration

```json
{
  "servers": [
    {
      "name": "filesystem",
      "httpEndpoint": "http://localhost:9876/filesystem",
      "capabilities": ["resources", "tools"],
      "description": "MCP filesystem server"
    },
    {
      "name": "memory",
      "httpEndpoint": "http://localhost:9876/memory",
      "capabilities": ["tools", "resources"],
      "description": "MCP memory server"
    }
  ]
}
```

### Anthropic API Example

```python
"""
Anthropic MCP Tools Configuration
Generated by MCP-Compose
"""
import os
import requests
from anthropic import Anthropic

# Initialize the Anthropic client
client = Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY", ""))

# MCP Server Proxy Configuration
MCP_PROXY_URL = 'http://localhost:9876'
MCP_SERVERS = ["filesystem", "memory", "weather"]
```

### OpenAI API Example

```javascript
/**
 * OpenAI MCP Tools Configuration
 * Generated by MCP-Compose
 */
const { OpenAI } = require('openai');

// Initialize the OpenAI client
const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
});

// MCP Server Proxy Configuration
const MCP_PROXY_URL = 'http://localhost:9876';
const MCP_SERVERS = ["filesystem", "memory", "weather"];
```

## Proxy Server

The proxy server feature provides an HTTP interface for your MCP servers, making them accessible to AI models like Claude or GPT, even if they're not directly designed to connect to them.

```bash
# Start the proxy server
mcp-compose proxy

# The proxy will run on http://localhost:9876 by default
# Endpoints will be available at:
# - http://localhost:9876/filesystem
# - http://localhost:9876/memory
# - etc.
```

## Examples

### Minimal MCP Compose File

```yaml
version: '1'
servers:
  filesystem:
    image: node:18-slim
    runtime: docker
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    capabilities:
      - resources
      - tools
    resources:
      paths:
        - source: "/tmp"
          target: "/tmp"
          watch: true
  memory:
    image: node:18-slim
    runtime: docker
    command: npx
    args: ["-y", "@modelcontextprotocol/server-memory"]
    capabilities:
      - tools
      - resources
```

### Persistent MCP Servers

```yaml
version: '1'
servers:
  filesystem:
    image: node:18-slim
    runtime: docker
    command: bash
    args: ["-c", "npx -y @modelcontextprotocol/server-filesystem /tmp & NODE_PID=$! && trap 'kill $NODE_PID' SIGTERM SIGINT && tail -f /dev/null"]
    capabilities:
      - resources
      - tools
    resources:
      paths:
        - source: "/tmp"
          target: "/tmp"
          watch: true
connections:
  default:
    transport: stdio
networks:
  mcp-net:
    driver: bridge
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Add some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create a new Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

MCP-Compose is not affiliated with Anthropic, OpenAI, or any other AI company. It's an open-source tool to simplify working with the Model Context Protocol.
