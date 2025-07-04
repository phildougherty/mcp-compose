# Migration Guide to MCP-Compose

This guide helps you migrate from existing MCP setups to MCP-Compose with minimal disruption.

## Migration Scenarios

### 1. From Individual MCP Servers → MCP-Compose

**Current Setup (Before):**
```bash
# Terminal 1
npx @modelcontextprotocol/server-filesystem /home/user/documents

# Terminal 2  
npx @modelcontextprotocol/server-memory

# Terminal 3
npx @modelcontextprotocol/server-brave-search

# Claude Desktop config
{
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-filesystem", "/home/user/documents"]
    },
    "memory": {
      "command": "npx", 
      "args": ["@modelcontextprotocol/server-memory"]
    },
    "search": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-brave-search"]
    }
  }
}
```

**New Setup (After):**
```yaml
# mcp-compose.yaml
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "/home/user/documents:/workspace:ro"
  
  memory:
    image: "mcp/memory:latest" 
    capabilities: [tools, resources]
    env:
      DATABASE_URL: "sqlite:///data/memory.db"
    volumes:
      - "mcp-memory:/data"
  
  search:
    image: "mcp/brave-search:latest"
    capabilities: [tools]
    env:
      BRAVE_API_KEY: "${BRAVE_API_KEY}"

volumes:
  mcp-memory:
    driver: local
```

**Migration Steps:**
1. **Stop existing servers** - Close all terminal windows running MCP servers
2. **Create mcp-compose.yaml** - Use the configuration above
3. **Set environment variables:**
   ```bash
   export MCP_API_KEY=$(openssl rand -hex 32)
   export BRAVE_API_KEY="your-brave-api-key"
   ```
4. **Start MCP-Compose:**
   ```bash
   ./mcp-compose up
   ./mcp-compose proxy --port 9876
   ```
5. **Update Claude Desktop config:**
   ```bash
   ./mcp-compose create-config --type claude --output ./claude-config
   # Copy generated config to Claude Desktop
   ```

**Benefits After Migration:**
- ✅ Single command to start/stop all servers
- ✅ Unified HTTP endpoint
- ✅ Automatic health monitoring  
- ✅ Centralized logging
- ✅ Easy scaling and configuration changes

---

### 2. From Docker Compose → MCP-Compose

**Current Setup (Before):**
```yaml
# docker-compose.yml
version: '3.8'

services:
  mcp-filesystem:
    image: my-mcp/filesystem
    ports:
      - "3000:3000"
    volumes:
      - "/home/user/docs:/workspace:ro"
    environment:
      - NODE_ENV=production
  
  mcp-memory:
    image: my-mcp/memory
    ports:
      - "3001:3001"
    volumes:
      - "memory_data:/data"
    environment:
      - DATABASE_URL=sqlite:///data/memory.db

  nginx:
    image: nginx
    ports:
      - "80:80"
    depends_on:
      - mcp-filesystem
      - mcp-memory
    volumes:
      - "./nginx.conf:/etc/nginx/nginx.conf"

volumes:
  memory_data:
```

**New Setup (After):**
```yaml
# mcp-compose.yaml
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    image: "my-mcp/filesystem"  # Same image
    capabilities: [resources, tools]
    volumes:
      - "/home/user/docs:/workspace:ro"
    env:
      NODE_ENV: "production"
  
  memory:
    image: "my-mcp/memory"      # Same image
    capabilities: [tools, resources]
    volumes:
      - "memory_data:/data"
    env:
      DATABASE_URL: "sqlite:///data/memory.db"

volumes:
  memory_data:
    driver: local
```

**Migration Steps:**
1. **Stop Docker Compose:**
   ```bash
   docker-compose down
   ```
2. **Create MCP-Compose config** - Use same images and volumes
3. **Start MCP-Compose:**
   ```bash
   export MCP_API_KEY=$(openssl rand -hex 32)
   ./mcp-compose up
   ./mcp-compose proxy --port 9876
   ```
4. **Test endpoints:**
   ```bash
   # Old: http://localhost:3000, http://localhost:3001
   # New: http://localhost:9876/filesystem, http://localhost:9876/memory
   ```

**Key Differences:**
- ❌ **Remove:** Custom nginx configuration - MCP-Compose handles routing
- ❌ **Remove:** Port mappings - All servers accessible via single proxy port
- ✅ **Add:** MCP protocol compliance and automatic OpenAPI generation
- ✅ **Add:** Built-in authentication and authorization

---

### 3. From Manual Claude Desktop Config → MCP-Compose

**Current Setup (Before):**
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "python",
      "args": ["-m", "mcp_server_filesystem", "/home/user/documents"],
      "env": {
        "PYTHONPATH": "/opt/mcp-servers"
      }
    },
    "database": {
      "command": "/usr/local/bin/mcp-database-server",
      "args": ["--connection", "postgresql://localhost:5432/mydb"],
      "env": {
        "DB_PASSWORD": "secret123"
      }
    },
    "custom-tools": {
      "command": "node",
      "args": ["/home/user/my-mcp-server/index.js"],
      "env": {
        "API_KEY": "my-api-key",
        "DEBUG": "1"
      }
    }
  }
}
```

**New Setup (After):**
```yaml
# mcp-compose.yaml
version: '1'

proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

servers:
  filesystem:
    # Option 1: Use containerized version
    image: "mcp/filesystem:latest"
    capabilities: [resources, tools]
    volumes:
      - "/home/user/documents:/workspace:ro"
    
    # Option 2: Use existing Python command
    # command: "python"
    # args: ["-m", "mcp_server_filesystem", "/workspace"]
    # env:
    #   PYTHONPATH: "/opt/mcp-servers"
  
  database:
    # Option 1: Containerize existing binary
    build:
      context: .
      dockerfile: |
        FROM alpine:latest
        RUN apk add --no-cache libc6-compat
        COPY /usr/local/bin/mcp-database-server /usr/local/bin/
        CMD ["/usr/local/bin/mcp-database-server"]
    capabilities: [tools, resources]
    env:
      DB_CONNECTION: "postgresql://localhost:5432/mydb"
      DB_PASSWORD: "${DB_PASSWORD}"
  
  custom-tools:
    # Option 1: Use existing Node.js server as-is
    command: "node"
    args: ["/app/index.js"]
    capabilities: [tools]
    volumes:
      - "/home/user/my-mcp-server:/app:ro"
    env:
      API_KEY: "${CUSTOM_API_KEY}"
      DEBUG: "1"
    
    # Option 2: Containerize for better isolation
    # build:
    #   context: /home/user/my-mcp-server
    #   dockerfile: Dockerfile

volumes:
  db-data:
    driver: local
```

**Migration Steps:**
1. **Backup current config:**
   ```bash
   cp ~/.config/Claude/claude_desktop_config.json ~/.config/Claude/claude_desktop_config.json.backup
   ```
2. **Choose containerization strategy:**
   - **Simple:** Use `command` and `args` for existing binaries
   - **Recommended:** Create Docker images for better isolation
3. **Set environment variables:**
   ```bash
   export MCP_API_KEY=$(openssl rand -hex 32)
   export DB_PASSWORD="secret123"
   export CUSTOM_API_KEY="my-api-key"
   ```
4. **Test servers individually:**
   ```bash
   ./mcp-compose up filesystem
   ./mcp-compose logs filesystem
   ```
5. **Generate new Claude Desktop config:**
   ```bash
   ./mcp-compose create-config --type claude --output ./claude-config
   ```

**Benefits After Migration:**
- ✅ **Centralized management** - All servers in one config file
- ✅ **Environment variable management** - Secure secret handling
- ✅ **Health monitoring** - Automatic restart of failed servers
- ✅ **Unified logging** - All server logs in one place
- ✅ **Hot reloading** - Update configs without full restart

---

## Common Migration Challenges & Solutions

### Challenge 1: Custom Binary Dependencies

**Problem:** Your MCP server requires specific system libraries or binaries.

**Solution:** Create a custom Docker image:
```dockerfile
# Dockerfile.custom-mcp-server
FROM ubuntu:22.04

# Install dependencies
RUN apt-get update && apt-get install -y \
    python3 \
    python3-pip \
    libpq-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy your server
COPY my-mcp-server /usr/local/bin/
COPY requirements.txt /tmp/
RUN pip3 install -r /tmp/requirements.txt

# Set up runtime
USER 1000:1000
CMD ["/usr/local/bin/my-mcp-server"]
```

```yaml
# mcp-compose.yaml
servers:
  custom-server:
    build:
      context: .
      dockerfile: Dockerfile.custom-mcp-server
    capabilities: [tools, resources]
```

### Challenge 2: Complex Environment Setup

**Problem:** Your servers need complex environment setup or multiple services.

**Solution:** Use Docker Compose-style dependencies:
```yaml
# mcp-compose.yaml
servers:
  database:
    image: postgres:15
    env:
      POSTGRES_DB: "mcpdb"
      POSTGRES_USER: "mcp"
      POSTGRES_PASSWORD: "${POSTGRES_PASSWORD}"
    volumes:
      - "postgres-data:/var/lib/postgresql/data"
  
  app-server:
    build:
      context: ./my-app
    capabilities: [tools, resources]
    depends_on:
      - database
    env:
      DATABASE_URL: "postgresql://mcp:${POSTGRES_PASSWORD}@database:5432/mcpdb"

volumes:
  postgres-data:
    driver: local
```

### Challenge 3: Port Conflicts

**Problem:** Your existing setup uses specific ports that conflict.

**Solution:** Use MCP-Compose's internal networking:
```yaml
# Before: Servers on different ports
# http://localhost:3000 - filesystem
# http://localhost:3001 - memory

# After: All servers via proxy
# http://localhost:9876/filesystem
# http://localhost:9876/memory

servers:
  filesystem:
    image: "mcp/filesystem:latest"
    # No port mapping needed - internal networking
  
  memory:
    image: "mcp/memory:latest"
    # No port mapping needed - internal networking
```

### Challenge 4: Authentication Integration

**Problem:** Your current setup uses different authentication methods.

**Solution:** Migrate to unified authentication:
```yaml
# Option 1: Simple API key (recommended for start)
proxy_auth:
  enabled: true
  api_key: "${MCP_API_KEY}"

# Option 2: OAuth for enterprise (after basic migration)
oauth:
  enabled: true
  issuer: "https://your-auth-server.com"
  # ... additional OAuth config
```

---

## Migration Checklist

### Pre-Migration
- [ ] **Backup current configurations** - Save existing Claude Desktop config
- [ ] **Document current setup** - List all running servers and their purposes  
- [ ] **Test current functionality** - Ensure everything works before migration
- [ ] **Identify dependencies** - Note any custom libraries or system requirements
- [ ] **Plan downtime** - Schedule migration during low-usage periods

### During Migration
- [ ] **Stop existing servers** - Gracefully shut down current MCP servers
- [ ] **Create mcp-compose.yaml** - Start with minimal config, add complexity gradually
- [ ] **Set environment variables** - Securely configure all secrets
- [ ] **Test each server individually** - Migrate and test one server at a time
- [ ] **Verify functionality** - Test all tools and resources work as expected
- [ ] **Update client configs** - Generate new Claude Desktop configuration

### Post-Migration  
- [ ] **Monitor performance** - Check logs and server health
- [ ] **Update documentation** - Document new setup for team members
- [ ] **Clean up old files** - Remove old configurations and unused containers
- [ ] **Set up monitoring** - Configure health checks and alerting
- [ ] **Train team members** - Share new commands and workflows

---

## Rollback Plan

If migration doesn't go as planned, here's how to quickly rollback:

### 1. Stop MCP-Compose
```bash
./mcp-compose down
```

### 2. Restore Claude Desktop Config
```bash
# Restore backup
cp ~/.config/Claude/claude_desktop_config.json.backup \
   ~/.config/Claude/claude_desktop_config.json
```

### 3. Restart Original Servers
```bash
# Example: restart original commands
npx @modelcontextprotocol/server-filesystem /home/user/documents &
npx @modelcontextprotocol/server-memory &
# ... other servers
```

### 4. Verify Functionality
```bash
# Test that Claude Desktop can connect to original servers
# Check that all tools and resources work as before
```

---

## Getting Help During Migration

### Resources
- **[Getting Started Guide](GETTING-STARTED.md)** - Step-by-step setup instructions
- **[Configuration Reference](mcp-compose-advanced.yaml)** - Complete configuration options
- **[GitHub Issues](https://github.com/phildougherty/mcp-compose/issues)** - Search existing issues

### Support Channels
- **GitHub Discussions** - Community help and best practices
- **Discord/Slack** - Real-time migration assistance
- **Documentation** - Comprehensive guides and examples

### What to Include in Support Requests
1. **Current setup description** - What you're migrating from
2. **Target setup description** - What you want to achieve
3. **Configuration files** - Your mcp-compose.yaml (remove secrets!)
4. **Error messages** - Full error output and logs
5. **Environment details** - OS, Docker version, etc.

**Remember:** Migration is often iterative. Start simple, test thoroughly, then add complexity gradually. 🚀