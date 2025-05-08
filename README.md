# MCP-Compose

A tool for orchestrating, managing, and running Model Context Protocol (MCP) servers in Docker containers.

## 📋 Overview

MCP-Compose simplifies the deployment and management of MCP servers, making it easy to define, configure, and run multiple services through a single YAML configuration file. It's inspired by Docker Compose but specifically tailored for MCP servers used with AI assistants.

## ✨ Features

- **Simple Configuration**: Define all your MCP servers in a single YAML file
- **Multiple Server Types**: Support for filesystem, memory, weather, and custom servers
- **Docker Integration**: Run servers in containers or as local processes
- **Resource Sharing**: Mount local directories as resources
- **Proxy Server**: Expose all MCP servers through a unified HTTP endpoint
- **Inspector Tool**: Debug and test MCP servers interactively
- **Authentication**: Secure your endpoints with API key authentication
- **OpenAPI Integration**: Auto-generated OpenAPI schemas for tools
- **Client Integration**: Generate configuration for LLM clients

## 🚀 Getting Started

### Prerequisites

- Docker
- Go 1.19+

### Installation

```bash
# Clone the repository
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose

# Build the tool
make build
```

### Basic Usage

1. Create an `mcp-compose.yaml` file:

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

2. Start all servers:

```bash
./mcp-compose up
```

3. Check server status:

```bash
./mcp-compose ls
```

## 📑 Configuration Reference

### MCP-Compose YAML Structure

The configuration file uses the following structure:

```yaml
version: '1'  # Configuration version

servers:       # Define your MCP servers
  server-name:  # Name for your server
    # Container configuration (for Docker-based servers)
    image: node:18-slim  # Docker image
    runtime: docker      # Container runtime (docker or podman)
    
    # Process configuration (for process-based servers)
    command: npx         # Command to run
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]  # Command arguments
    
    # Common settings
    env:                 # Environment variables
      DEBUG: "true"
    ports:               # Port mappings
      - "3000:3000"
    volumes:             # Volume mappings
      - "./data:/data"
    capabilities:        # MCP capabilities
      - resources
      - tools
    depends_on:          # Dependencies
      - other-server
      
    # Enhanced settings
    resources:           # Resource configuration
      paths:
        - source: "./data"
          target: "/data"
          watch: true
    networks:
      - mcp-net

connections:    # Connection configuration
  default:
    transport: stdio
  
networks:       # Network configuration
  mcp-net:
    driver: bridge
  
development:    # Development tools configuration
  inspector:
    enabled: true
    port: 8090
```

### Basic Commands

```bash
# Start all servers
./mcp-compose up

# Stop all servers
./mcp-compose down

# List all servers and their status
./mcp-compose ls

# View logs
./mcp-compose logs [SERVER...]

# Start the MCP inspector
./mcp-compose inspector [SERVER]

# Start the MCP proxy
./mcp-compose proxy
```

## 🔄 Proxy Server

The proxy server lets you access all your MCP services through a unified HTTP endpoint, making them accessible to AI models and other clients.

```bash
# Start the proxy server
./mcp-compose proxy
```

### Proxy Features

- **Unified HTTP Endpoint**: Access all MCP servers through a single entry point
- **OpenAPI Integration**: Auto-generated OpenAPI schema and Swagger UI documentation
- **API Authentication**: Secure your endpoints with API key authentication
- **Cross-Origin Support**: Full CORS support for browser-based clients
- **HTTP/JSON Bridge**: Converts between JSON-RPC over HTTP and stdio protocols
- **Container Discovery**: Automatically detects and exposes running MCP containers
- **Tool Forwarding**: Properly routes tool calls to appropriate servers

Once running, the proxy is available at `http://localhost:9876` with:

- Server endpoints: `http://localhost:9876/{server-name}`
- Tool endpoints: `http://localhost:9876/{server-name}/{tool-name}`
- OpenAPI schema: `http://localhost:9876/openapi.json`
- Swagger UI docs: `http://localhost:9876/docs`
- Server-specific docs: `http://localhost:9876/{server-name}/docs`

## 🔍 Inspector Tool

The inspector provides a web interface for debugging and testing MCP servers:

```bash
# Launch inspector UI
./mcp-compose inspector
```

The inspector lets you:
- Browse available servers and their capabilities
- Execute tool calls and view responses
- View server metadata and resources
- Test and diagnose MCP protocol issues

## 🔌 Client Integration

MCP-Compose can generate configuration for various LLM clients:

```bash
# Generate client configurations
./mcp-compose create-config
```

This creates configuration files for:
- Claude Desktop
- OpenAI API clients
- Anthropic API clients

## ✨ MCP-Compose-Proxy-Shim

For users who want to use Docker-based MCP servers with clients that expect local servers (like the free version of Claude Desktop), check out [MCP-Compose-Proxy-Shim](https://github.com/phildougherty/mcp-compose-proxy-shim).

The shim works by:
1. Intercepting calls from Claude Desktop to local MCP servers
2. Forwarding these calls to your MCP-Compose proxy
3. Returning results back to Claude as if they came from local servers

This gives you the best of both worlds: the power of Docker-based servers with clients that need local servers.

## 🛠️ Advanced Features

- **Lifecycle Hooks**: Run custom scripts at different stages of server lifecycle
- **Health Checks**: Monitor server health and automatically restart unhealthy servers
- **Resource Watching**: Track changes to files and directories in real-time
- **Network Management**: Create isolated or shared networks for MCP servers
- **Environment Variables**: Configure servers using environment variables
- **Custom Tool Definitions**: Define custom tools with rich parameters and schemas

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a pull request.

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.
```
