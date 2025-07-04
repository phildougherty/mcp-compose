# Getting Started with MCP-Compose

This guide will get you up and running with MCP-Compose in 10 minutes or less.

## Prerequisites

- Docker or Podman installed
- Basic command line knowledge
- 5 minutes of your time

## Quick Start (30 seconds)

### 1. Install MCP-Compose

```bash
# Download for Linux/macOS
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose

# Or build from source
git clone https://github.com/phildougherty/mcp-compose.git
cd mcp-compose && make build
sudo cp build/mcp-compose /usr/local/bin/
```

### 2. Create Your First Configuration

```bash
# Create a minimal configuration file
cat > mcp-compose.yaml << 'EOF'
version: '1'

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
EOF
```

### 3. Start Your Servers

```bash
# Generate a secure API key
export MCP_API_KEY=$(openssl rand -hex 32)
echo "Your API key: $MCP_API_KEY"

# Start the filesystem server
./mcp-compose up

# In another terminal, start the proxy
./mcp-compose proxy --port 9876 --api-key "$MCP_API_KEY"
```

### 4. Test It Works

```bash
# Test the connection
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/api/servers

# If successful, you'll see your filesystem server listed!
```

🎉 **Congratulations!** Your MCP server is now running at `http://localhost:9876`

## What's Next? (Choose Your Path)

### 🔰 Beginner: Add More Servers
[→ Continue to "Adding More Servers"](#adding-more-servers)

### 🎯 Practical: Connect to Claude Desktop  
[→ Jump to "Claude Desktop Setup"](#claude-desktop-setup)

### 🚀 Advanced: Enterprise Features
[→ See Advanced Configuration](mcp-compose-advanced.yaml)

---

## Adding More Servers (2 minutes)

Let's add memory and search capabilities to create a more powerful setup:

### 1. Update Your Configuration

```bash
# Stop current setup
./mcp-compose down

# Create enhanced configuration
cat > mcp-compose.yaml << 'EOF'
version: '1'

# Enable authentication
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  # File system access
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "${HOME}/Documents:/workspace:ro"  # Safe read-only access
  
  # Persistent memory/notes
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/memory.db"
    volumes:
      - "mcp-memory:/data"

  # Web search (no API key needed)
  search:
    image: "mcp/search:latest"
    capabilities: [tools]
    env:
      SEARCH_ENGINE: "duckduckgo"

# Define persistent storage
volumes:
  mcp-memory:
    driver: local
EOF
```

### 2. Restart with New Configuration

```bash
# Start all servers
./mcp-compose up

# Start proxy (in another terminal)
./mcp-compose proxy --port 9876
```

### 3. Verify All Servers Are Running

```bash
# Check server status
./mcp-compose ls

# Test each server
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/filesystem -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/memory -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/search -X POST \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

**Success indicators:**
- ✅ `./mcp-compose ls` shows all servers as "running"
- ✅ Each curl command returns a JSON response with available tools
- ✅ No error messages in the logs

---

## Claude Desktop Setup (3 minutes)

Connect your MCP servers to Claude Desktop for the full AI experience:

### 1. Generate Claude Desktop Configuration

```bash
# Generate configuration files
./mcp-compose create-config --type claude --output ./claude-config

# View the generated configuration
cat ./claude-config/claude-desktop-servers.json
```

### 2. Install Configuration in Claude Desktop

**macOS:**
```bash
cp ./claude-config/claude-desktop-servers.json \
  ~/Library/Application\ Support/Claude/claude_desktop_config.json
```

**Linux:**
```bash
mkdir -p ~/.config/Claude
cp ./claude-config/claude-desktop-servers.json \
  ~/.config/Claude/claude_desktop_config.json
```

**Windows:**
```powershell
copy .\claude-config\claude-desktop-servers.json %APPDATA%\Claude\claude_desktop_config.json
```

### 3. Restart Claude Desktop

1. Quit Claude Desktop completely
2. Restart Claude Desktop
3. Look for your servers in the Claude interface

### 4. Test the Integration

In Claude Desktop, try asking:

> "What files are in my Documents folder?"

> "Remember that I'm working on a project called MCP-Compose"

> "Search for information about Model Context Protocol"

**Success indicators:**
- ✅ Claude can list files from your Documents folder
- ✅ Claude can store and retrieve information using the memory server
- ✅ Claude can search the web for current information

---

## Troubleshooting

### Common Issues and Solutions

#### ❌ "Server not found" error

**Symptoms:** `curl` returns 404 or "server not found"

**Solutions:**
```bash
# Check if servers are actually running
./mcp-compose ls

# Check specific server logs
./mcp-compose logs filesystem

# Restart servers if needed
./mcp-compose restart filesystem
```

#### ❌ "Connection refused" error

**Symptoms:** `curl` returns "connection refused"

**Solutions:**
```bash
# Check if proxy is running
ps aux | grep mcp-compose

# Check if port is available
lsof -i :9876

# Kill any conflicting processes
kill $(lsof -t -i:9876)

# Restart proxy
./mcp-compose proxy --port 9876
```

#### ❌ Authentication errors

**Symptoms:** 401 Unauthorized responses

**Solutions:**
```bash
# Check if API key is set
echo $MCP_API_KEY

# Regenerate API key if needed
export MCP_API_KEY=$(openssl rand -hex 32)

# Verify proxy_auth config
grep -A5 proxy_auth mcp-compose.yaml
```

#### ❌ Claude Desktop not connecting

**Symptoms:** Servers don't appear in Claude Desktop

**Solutions:**
```bash
# Verify config file location and content
cat ~/.config/Claude/claude_desktop_config.json  # Linux
cat ~/Library/Application\ Support/Claude/claude_desktop_config.json  # macOS

# Check proxy is accessible
curl -H "Authorization: Bearer $MCP_API_KEY" http://localhost:9876/api/servers

# Restart Claude Desktop completely
```

### Debug Mode

Enable detailed logging for troubleshooting:

```bash
# Enable debug logging
export MCP_LOG_LEVEL=debug

# Start with debug output
./mcp-compose up --debug

# Start proxy with debug output  
./mcp-compose proxy --port 9876 --debug
```

### Getting Help

If you're still having issues:

1. **Check the logs:** `./mcp-compose logs [server-name]`
2. **Search existing issues:** [GitHub Issues](https://github.com/phildougherty/mcp-compose/issues)
3. **Create a new issue:** Include your config file and error messages
4. **Join the community:** [Discord/Slack/Forum links]

---

## What's Next?

### 🎓 Learn More

- **[Configuration Reference](mcp-compose-advanced.yaml)** - Complete configuration options
- **[Architecture Guide](README.md#architecture)** - How MCP-Compose works
- **[Security Best Practices](README.md#security-notice)** - Production deployment guide

### 🔧 Customize Your Setup

- **Development Setup:** Add Git, Docker, and development tools
- **Content Creation:** Add research, writing, and media tools  
- **Enterprise:** OAuth, audit logging, and monitoring

### 🚀 Advanced Features

- **Multiple Environments:** Development, staging, production configs
- **Custom Servers:** Build your own MCP servers
- **Monitoring & Alerts:** Production monitoring setup
- **CI/CD Integration:** Automated deployments

---

## Configuration Examples by Use Case

### 👨‍💻 Software Developer

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
      - "${HOME}/.gitconfig:/root/.gitconfig:ro"
      - "${HOME}/code:/workspace:rw"
  
  docker:
    image: "mcp/docker:latest"
    capabilities: [tools]
    volumes:
      - "/var/run/docker.sock:/var/run/docker.sock"
  
  database:
    image: "mcp/database:latest"
    capabilities: [tools, resources]
    env:
      DB_CONNECTIONS: "postgresql://localhost:5432,mysql://localhost:3306"
```

### 📝 Content Creator

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
      - "${HOME}/Downloads:/downloads:ro"
  
  search:
    image: "mcp/search:latest"
    capabilities: [tools]
    env:
      SEARCH_ENGINES: "duckduckgo,wikipedia,arxiv"
  
  memory:
    image: "mcp/memory:latest"
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/research.db"
    volumes:
      - "research-data:/data"
  
  web:
    image: "mcp/web:latest"
    capabilities: [tools, resources]
    env:
      BROWSER_HEADLESS: "true"

volumes:
  research-data:
    driver: local
```

### 🏢 Business Analyst

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
      - "${HOME}/Business:/business:rw"
  
  spreadsheet:
    image: "mcp/spreadsheet:latest"
    capabilities: [tools, resources]
    env:
      EXCEL_SUPPORT: "true"
      CSV_ENCODING: "utf-8"
  
  email:
    image: "mcp/email:latest"
    capabilities: [tools, resources]
    env:
      EMAIL_PROVIDER: "outlook"  # or gmail, etc.
  
  calendar:
    image: "mcp/calendar:latest"
    capabilities: [tools, resources]
    env:
      CALENDAR_PROVIDER: "outlook"
```

**Remember:** Replace `"mcp/[service]:latest"` with actual available images or build configurations for your specific tools.

---

## Next Steps

You now have a working MCP-Compose setup! Here are suggested next steps:

1. **Explore your servers** - Try different commands and see what each server can do
2. **Customize for your workflow** - Add servers specific to your work
3. **Set up monitoring** - Add health checks and logging
4. **Share with your team** - Create configurations for different team members
5. **Contribute back** - Share useful configurations and improvements

Happy composing! 🚀