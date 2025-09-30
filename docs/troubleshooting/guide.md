# Troubleshooting Guide

Common issues, solutions, debugging techniques, and performance tuning for MCP-Compose.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Common Errors](#common-errors)
- [Authentication Issues](#authentication-issues)
- [Connection Problems](#connection-problems)
- [Container Issues](#container-issues)
- [Performance Problems](#performance-problems)
- [Debug Mode](#debug-mode)
- [Performance Tuning](#performance-tuning)
- [FAQ](#faq)

## Quick Diagnostics

Run these commands to quickly diagnose issues:

```bash
# Check server status
mcp-compose ls

# View recent logs
mcp-compose logs --tail 50

# Validate configuration
mcp-compose validate

# Check Docker/Podman
docker ps -a
# or
podman ps -a

# Test proxy connectivity
curl http://localhost:9876/health
```

## Common Errors

### "Server not found"

**Error message:**
```
Error: server 'filesystem' not found
```

**Causes:**
1. Server not defined in `mcp-compose.yaml`
2. Server failed to start
3. Typo in server name

**Solutions:**

```bash
# Check if server is defined
grep -A5 "filesystem:" mcp-compose.yaml

# Check server status
mcp-compose ls

# View startup logs
mcp-compose logs filesystem

# Restart the server
mcp-compose restart filesystem
```

### "Connection refused"

**Error message:**
```
Error: connect: connection refused
```

**Causes:**
1. Proxy not running
2. Wrong port
3. Server not listening
4. Firewall blocking connection

**Solutions:**

```bash
# Check if proxy is running
ps aux | grep mcp-compose

# Verify port
lsof -i :9876

# Start proxy if not running
mcp-compose proxy --port 9876

# Check firewall (Linux)
sudo iptables -L -n | grep 9876

# Check firewall (macOS)
sudo pfctl -s rules | grep 9876
```

### "Image not found"

**Error message:**
```
Error: image 'mcp/filesystem:latest' not found
```

**Causes:**
1. Image doesn't exist
2. Network issues preventing pull
3. Wrong image name
4. Registry authentication required

**Solutions:**

```bash
# Pull image manually
docker pull mcp/filesystem:latest
# or
podman pull mcp/filesystem:latest

# Check available images
docker images | grep mcp

# Use specific version
# In mcp-compose.yaml:
image: "mcp/filesystem:v1.0.0"

# Build locally if needed
build:
  context: ./path/to/dockerfile
```

### "Port already in use"

**Error message:**
```
Error: bind: address already in use
```

**Causes:**
1. Another process using the port
2. Previous instance still running
3. Port conflict in configuration

**Solutions:**

```bash
# Find process using port
lsof -i :9876
# or
netstat -tuln | grep 9876

# Kill process
kill -9 <PID>

# Use different port
mcp-compose proxy --port 9877

# Or update configuration
ports:
  - "9877:8080"
```

### "Permission denied"

**Error message:**
```
Error: permission denied
```

**Causes:**
1. Docker/Podman requires root
2. File permissions
3. Volume mount permissions
4. SELinux/AppArmor blocking

**Solutions:**

```bash
# Add user to docker group (Linux)
sudo usermod -aG docker $USER
newgrp docker

# Use Podman rootless
podman run --user 1000:1000 ...

# Check file permissions
ls -la mcp-compose.yaml

# Fix ownership
chown $USER:$USER mcp-compose.yaml

# SELinux: relabel volumes
volumes:
  - "/host/path:/container/path:z"  # Private
  - "/host/path:/container/path:Z"  # Shared
```

## Authentication Issues

### "401 Unauthorized"

**Error message:**
```json
{
  "error": "unauthorized",
  "message": "invalid or missing authentication"
}
```

**Solutions:**

```bash
# Check API key is set
echo $MCP_API_KEY

# Generate new key if needed
export MCP_API_KEY=$(openssl rand -hex 32)

# Include in request
curl -H "Authorization: Bearer $MCP_API_KEY" \
  http://localhost:9876/api/servers

# Check configuration
grep -A3 "proxy_auth:" mcp-compose.yaml
```

### "403 Forbidden"

**Error message:**
```json
{
  "error": "forbidden",
  "message": "insufficient permissions"
}
```

**Solutions:**

```bash
# Check OAuth scopes
# User needs appropriate scope (e.g., mcp:tools)

# Check RBAC configuration
grep -A10 "rbac:" mcp-compose.yaml

# Check server authentication
grep -A5 "authentication:" mcp-compose.yaml

# Request with correct scope
# In OAuth client configuration:
scopes: ["mcp:tools", "mcp:resources"]
```

### "Invalid OAuth token"

**Error message:**
```json
{
  "error": "invalid_token",
  "message": "token expired or invalid"
}
```

**Solutions:**

```bash
# Check token expiration
# Access tokens expire after 1h by default

# Refresh token
curl -X POST http://localhost:9876/oauth/token \
  -d "grant_type=refresh_token" \
  -d "refresh_token=YOUR_REFRESH_TOKEN" \
  -d "client_id=YOUR_CLIENT_ID"

# Check OAuth configuration
grep -A20 "oauth:" mcp-compose.yaml

# Verify issuer matches
# oauth.issuer should match your proxy URL
```

## Connection Problems

### "Timeout waiting for response"

**Error message:**
```
Error: context deadline exceeded
```

**Causes:**
1. Server slow to respond
2. Network latency
3. Timeout too short
4. Server hung

**Solutions:**

```yaml
# Increase timeouts in configuration
connections:
  default:
    timeouts:
      connect: "30s"      # Increase from 10s
      read: "60s"         # Increase from 30s
      write: "60s"        # Increase from 30s
```

```bash
# Check server logs for delays
mcp-compose logs filesystem

# Monitor server resource usage
docker stats

# Test connectivity
curl -v http://localhost:9876/filesystem
```

### "Session initialization failed"

**Error message:**
```
Error: failed to initialize MCP session
```

**Causes:**
1. Server not implementing MCP protocol correctly
2. Version mismatch
3. Server crashed during initialization

**Solutions:**

```bash
# Check server logs
mcp-compose logs filesystem --tail 100

# Verify server supports MCP
docker exec mcp-compose-filesystem-1 /app/server --version

# Restart server
mcp-compose restart filesystem

# Check protocol configuration
protocol: "http"  # or "stdio", "sse", "tcp"
http_port: 8080
```

### "Socat bridge failed"

**Error message:**
```
Error: socat bridge not responding
```

**Causes:**
1. Socat not installed in container
2. Port conflict
3. STDIO server crashed

**Solutions:**

```bash
# Check container has socat
docker exec mcp-compose-filesystem-1 which socat

# Check socat process
docker exec mcp-compose-filesystem-1 ps aux | grep socat

# View container logs
docker logs mcp-compose-filesystem-1

# Use HTTP protocol instead
protocol: "http"  # More reliable than stdio
```

## Container Issues

### "Container keeps restarting"

**Symptoms:**
```bash
$ docker ps
# Container shows "Restarting" status
```

**Solutions:**

```bash
# Check why container is restarting
docker logs mcp-compose-filesystem-1

# Disable auto-restart temporarily
docker update --restart=no mcp-compose-filesystem-1

# Check resource limits
docker stats

# Increase limits if needed
deploy:
  resources:
    limits:
      memory: "1g"  # Increase from 512m
```

### "Container exits immediately"

**Symptoms:**
```bash
$ mcp-compose ls
# Server shows "exited" status
```

**Solutions:**

```bash
# View exit logs
mcp-compose logs filesystem

# Check command and args
command: "/app/server"
args: ["--port", "8080"]

# Verify entrypoint
docker inspect mcp-compose-filesystem-1 | grep -A5 Entrypoint

# Test command manually
docker run -it mcp/filesystem:latest /bin/sh
# Then run: /app/server --port 8080
```

### "Volume mount issues"

**Error message:**
```
Error: failed to mount volume
```

**Solutions:**

```bash
# Check host path exists
ls -la /host/path

# Check permissions
stat /host/path

# Use absolute paths
volumes:
  - "/home/user/data:/data:rw"  # Not ~/data

# For SELinux, add :z or :Z
volumes:
  - "/host/path:/container/path:z"

# Check volume exists (named volumes)
docker volume ls | grep mcp-memory
```

### "Out of memory"

**Error message:**
```
Error: container killed (OOM)
```

**Solutions:**

```yaml
# Increase memory limit
deploy:
  resources:
    limits:
      memory: "2g"      # Increase from 512m
      memory_swap: "4g" # Add swap

# Monitor memory usage
```

```bash
docker stats

# Check container logs
docker logs mcp-compose-filesystem-1 | grep -i "memory\|oom"

# Identify memory leak
docker exec mcp-compose-filesystem-1 ps aux --sort=-%mem | head
```

## Performance Problems

### "Slow response times"

**Symptoms:**
- Requests taking >1 second
- High latency in logs

**Diagnosis:**

```bash
# Monitor request latency
curl -w "@curl-format.txt" -o /dev/null -s http://localhost:9876/filesystem

# curl-format.txt:
# time_namelookup: %{time_namelookup}\n
# time_connect: %{time_connect}\n
# time_starttransfer: %{time_starttransfer}\n
# time_total: %{time_total}\n

# Check server resource usage
docker stats

# Enable debug logging
export MCP_LOG_LEVEL=debug
mcp-compose logs filesystem
```

**Solutions:**

```yaml
# Optimize timeouts
connections:
  default:
    timeouts:
      connect: "5s"   # Reduce from 10s
      read: "15s"     # Reduce from 30s
      idle: "30s"     # Reduce from 60s

# Enable connection pooling (automatic in proxy)

# Add caching if applicable
# (future feature)

# Increase server resources
deploy:
  resources:
    limits:
      cpus: "2.0"     # Increase from 1.0
      memory: "1g"    # Increase from 512m
```

### "High memory usage"

**Symptoms:**
- Proxy using >1GB RAM
- Containers using excessive memory

**Solutions:**

```bash
# Check memory usage
docker stats

# Profile memory (Go pprof)
curl http://localhost:9876/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Reduce session cache
# (configure in future versions)

# Limit concurrent connections
# (configure in future versions)

# Restart proxy periodically
# (as a temporary workaround)
```

### "High CPU usage"

**Symptoms:**
- CPU usage >80%
- System slowness

**Solutions:**

```bash
# Identify CPU hog
docker stats

# Profile CPU (Go pprof)
curl http://localhost:9876/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Limit CPU per server
deploy:
  resources:
    limits:
      cpus: "1.0"  # Limit to 1 core

# Check for infinite loops in logs
mcp-compose logs filesystem | grep -i "error\|panic"
```

## Debug Mode

Enable comprehensive debugging:

### Environment Variables

```bash
# Enable debug logging
export MCP_LOG_LEVEL=debug

# Enable Go profiling
export GODEBUG=gctrace=1

# Enable Docker debug
export DOCKER_BUILDKIT=1
export BUILDKIT_PROGRESS=plain
```

### Debug Configuration

```yaml
logging:
  level: "debug"
  format: "json"
  destinations:
    - type: stdout
    - type: file
      path: "/var/log/mcp-compose-debug.log"

development:
  inspector:
    enabled: true
    port: 3002
```

### Proxy Debug Mode

```bash
# Start proxy with debug logging
mcp-compose proxy --port 9876 --debug

# Enable profiling endpoints
mcp-compose proxy --port 9876 --enable-profiling

# Access profiling data
curl http://localhost:9876/debug/pprof/
```

### Container Debug

```bash
# Run container with debug shell
docker run -it --entrypoint /bin/sh mcp/filesystem:latest

# Exec into running container
docker exec -it mcp-compose-filesystem-1 /bin/sh

# View all environment variables
docker exec mcp-compose-filesystem-1 env

# Check network connectivity
docker exec mcp-compose-filesystem-1 ping google.com

# View process list
docker exec mcp-compose-filesystem-1 ps aux
```

### Network Debug

```bash
# Inspect Docker networks
docker network inspect mcp-net

# Test connectivity between containers
docker exec mcp-compose-web-1 ping mcp-compose-database-1

# Monitor network traffic
docker exec mcp-compose-filesystem-1 tcpdump -i any port 8080

# Check DNS resolution
docker exec mcp-compose-filesystem-1 nslookup mcp-compose-database-1
```

## Performance Tuning

### Optimize Container Startup

```yaml
servers:
  filesystem:
    # Use specific versions for faster pulls
    image: "mcp/filesystem:v1.0.0"

    # Disable pull if image exists
    pull: false

    # Reduce health check frequency
    healthcheck:
      interval: "60s"  # Increase from 30s
      timeout: "5s"
      retries: 3

    # Faster startup with pre-start hook
    lifecycle:
      pre_start: "echo 'Starting...'"  # Fast operation
```

### Optimize Proxy Performance

```yaml
connections:
  default:
    timeouts:
      connect: "5s"    # Reduce connection timeout
      read: "15s"      # Reduce read timeout
      idle: "30s"      # Reduce idle timeout

# Enable connection pooling (default)
# Configure pool size (future feature)
```

### Optimize Server Resources

```yaml
servers:
  filesystem:
    deploy:
      resources:
        limits:
          cpus: "2.0"      # Allocate sufficient CPU
          memory: "1g"     # Allocate sufficient memory
        reservations:
          cpus: "1.0"      # Guarantee minimum CPU
          memory: "512m"   # Guarantee minimum memory

    # Use read-only filesystem where possible
    read_only: true
    tmpfs: ["/tmp", "/var/cache"]

    # Optimize restart policy
    restart: "unless-stopped"
```

### Optimize Docker/Podman

```bash
# Use BuildKit for faster builds
export DOCKER_BUILDKIT=1

# Prune unused resources
docker system prune -a

# Increase Docker memory limit
# Docker Desktop: Settings → Resources → Memory (4GB+)

# Use overlay2 storage driver (Linux)
# /etc/docker/daemon.json:
{
  "storage-driver": "overlay2"
}
```

### Optimize Network

```yaml
networks:
  mcp-net:
    driver: bridge
    driver_opts:
      com.docker.network.bridge.name: "mcp0"
      # Increase MTU for better performance
      com.docker.network.driver.mtu: "9000"  # Jumbo frames
```

## FAQ

### Q: Why is the proxy slow on first request?

**A:** The first request initializes the MCP session. Subsequent requests reuse the session and are much faster. This is expected behavior.

To minimize impact:
- Use session warming (send a dummy request at startup)
- Configure shorter health check intervals
- Use HTTP protocol instead of STDIO (faster initialization)

### Q: Can I run MCP-Compose without Docker?

**A:** Not currently. MCP-Compose is designed around container orchestration. However, you can:
- Use Podman as a Docker alternative (rootless)
- Run MCP servers directly and use only the proxy (manual setup)

### Q: How do I backup my data?

**A:** Backup Docker/Podman volumes:

```bash
# List volumes
docker volume ls | grep mcp

# Backup volume
docker run --rm -v mcp-memory-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/mcp-memory-backup.tar.gz /data

# Restore volume
docker run --rm -v mcp-memory-data:/data -v $(pwd):/backup \
  alpine tar xzf /backup/mcp-memory-backup.tar.gz -C /
```

### Q: How do I update MCP-Compose?

**A:**

```bash
# Update binary
curl -LO https://github.com/phildougherty/mcp-compose/releases/latest/download/mcp-compose-linux-amd64
chmod +x mcp-compose-linux-amd64
sudo mv mcp-compose-linux-amd64 /usr/local/bin/mcp-compose

# Update server images
mcp-compose down
docker pull mcp/filesystem:latest
docker pull mcp/memory:latest
mcp-compose up
```

### Q: Can I use custom MCP servers?

**A:** Yes! Use any Docker image:

```yaml
servers:
  custom:
    image: "your-registry/your-mcp-server:latest"
    protocol: "http"
    http_port: 8080
    capabilities: [tools]
```

Or build from source:

```yaml
servers:
  custom:
    build:
      context: ./my-server
      dockerfile: Dockerfile
    protocol: "http"
    http_port: 8080
```

### Q: How do I use multiple environments?

**A:**

```yaml
environments:
  development:
    servers:
      filesystem:
        env:
          DEBUG: "true"

  production:
    servers:
      filesystem:
        env:
          DEBUG: "false"
        deploy:
          resources:
            limits:
              memory: "2g"
```

```bash
# Use specific environment
export MCP_ENV=production
mcp-compose up
```

### Q: How do I report bugs?

**A:**

1. Check this troubleshooting guide
2. Search [existing issues](https://github.com/phildougherty/mcp-compose/issues)
3. Collect diagnostic information:
   ```bash
   mcp-compose ls > status.txt
   mcp-compose logs > logs.txt
   cat mcp-compose.yaml > config.yaml
   ```
4. Create a [new issue](https://github.com/phildougherty/mcp-compose/issues/new) with:
   - MCP-Compose version
   - Docker/Podman version
   - Operating system
   - Configuration file (sanitized)
   - Error messages and logs
   - Steps to reproduce

### Q: How do I contribute?

**A:** See [CONTRIBUTING.md](../../CONTRIBUTING.md) for:
- Code style guide
- Testing requirements
- Pull request process
- Development setup

## Getting More Help

- **Documentation**: [Full documentation](../)
- **Examples**: [Configuration examples](../../examples/)
- **GitHub Issues**: [Report bugs or request features](https://github.com/phildougherty/mcp-compose/issues)
- **Community**: (Coming soon)

## Performance Benchmarks

Typical performance characteristics:

| Metric | Value |
|--------|-------|
| Proxy latency (p50) | 5-10ms |
| Proxy latency (p99) | 15-30ms |
| STDIO overhead | +2-5ms |
| Cold start | 100-500ms |
| Concurrent connections | 1000+ |
| Request throughput | 10,000+ req/sec |
| Memory (proxy) | 50-100MB base |
| Memory per session | ~1MB |

Your results may vary based on:
- Server implementation
- Network conditions
- Hardware specifications
- Container resource limits