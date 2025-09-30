# Configuration Reference

Complete reference for all `mcp-compose.yaml` configuration options.

## Table of Contents

- [Top-Level Configuration](#top-level-configuration)
- [Proxy Authentication](#proxy-authentication)
- [OAuth 2.1 Configuration](#oauth-21-configuration)
- [Server Configuration](#server-configuration)
- [Security Configuration](#security-configuration)
- [Resource Limits](#resource-limits)
- [Networking](#networking)
- [Storage](#storage)
- [Health Checks & Lifecycle](#health-checks--lifecycle)
- [Audit Logging](#audit-logging)
- [RBAC](#rbac)
- [Dashboard](#dashboard)
- [Environment Variables](#environment-variables)
- [Best Practices](#best-practices)

## Top-Level Configuration

### version

**Required**. The configuration file format version.

```yaml
version: '1'  # Currently only '1' is supported
```

### servers

**Required**. Map of server names to server configurations.

```yaml
servers:
  server-name:
    # Server configuration options (see below)
```

## Proxy Authentication

Simple API key-based authentication for the HTTP proxy.

### proxy_auth

```yaml
proxy_auth:
  enabled: true                   # Enable/disable authentication
  api_key: "${MCP_API_KEY}"      # API key (use environment variable)
  oauth_fallback: true            # Allow OAuth as authentication fallback
```

**Environment Variables:**
- `MCP_API_KEY` - API key for proxy authentication (required if `enabled: true`)

**Best Practice:**
```bash
# Generate a secure API key
export MCP_API_KEY=$(openssl rand -hex 32)
```

## OAuth 2.1 Configuration

Advanced OAuth 2.1 authentication with PKCE support.

### oauth

```yaml
oauth:
  enabled: true
  issuer: "http://your-proxy-url"

  endpoints:
    authorization: "/oauth/authorize"
    token: "/oauth/token"
    userinfo: "/oauth/userinfo"
    revoke: "/oauth/revoke"
    discovery: "/.well-known/oauth-authorization-server"

  tokens:
    access_token_ttl: "1h"         # Access token lifetime
    refresh_token_ttl: "168h"      # Refresh token lifetime (7 days)
    authorization_code_ttl: "10m"  # Auth code lifetime
    algorithm: "HS256"             # JWT signing algorithm

  security:
    require_pkce: true             # Require PKCE for security

  grant_types:
    - "authorization_code"
    - "client_credentials"
    - "refresh_token"

  response_types:
    - "code"

  scopes_supported:
    - "mcp:*"        # Full access
    - "mcp:tools"    # Tools access only
    - "mcp:resources" # Resources access only
    - "mcp:prompts"  # Prompts access only
```

### oauth_clients

Pre-register OAuth clients.

```yaml
oauth_clients:
  my-client:
    client_id: "unique-client-id"
    client_secret: "${OAUTH_CLIENT_SECRET}"
    name: "My Application"
    description: "Description of the client"
    redirect_uris:
      - "http://localhost:3000/callback"
      - "https://app.example.com/callback"
    scopes:
      - "mcp:tools"
      - "mcp:resources"
    grant_types:
      - "authorization_code"
      - "refresh_token"
    public_client: false  # Set true for native/SPA clients
    auto_approve: false   # Skip user consent (use carefully)
```

## Server Configuration

### Basic Configuration

At least one of `image`, `build`, or `command` is required.

```yaml
servers:
  my-server:
    # Docker image (most common)
    image: "nginx:alpine"

    # OR build from source
    build:
      context: "./path/to/build"
      dockerfile: "Dockerfile"
      args:
        BUILD_ENV: "production"
      target: "production"
      no_cache: false
      pull: true
      platform: "linux/amd64"

    # OR run a command
    command: "/usr/bin/app"
    args: ["--flag", "value"]
```

### MCP Protocol Configuration

```yaml
servers:
  my-server:
    protocol: "http"           # "stdio", "http", "sse", or "tcp"
    http_port: 8080           # Port for HTTP/SSE protocols
    http_path: "/api"         # HTTP endpoint path
    sse_path: "/sse"          # SSE endpoint path
    sse_port: 8081            # Separate SSE port (optional)
    sse_heartbeat: 30         # SSE heartbeat interval in seconds
    stdio_hoster_port: 12345  # Port for stdio-over-socket

    capabilities:             # MCP capabilities
      - resources
      - tools
      - prompts
      - sampling
      - logging
```

### Environment Variables

```yaml
servers:
  my-server:
    env:
      NODE_ENV: "production"
      API_KEY: "${SECRET_KEY}"    # Environment variable expansion
      DEBUG: "false"
```

**Environment Variable Expansion:**
- `${VAR}` or `$VAR` - Expand environment variable
- Variables are loaded from `.env` file in config directory
- System environment variables override `.env` file

### Dependencies

```yaml
servers:
  web:
    depends_on:
      - database
      - redis

  database:
    image: "postgres:15"

  redis:
    image: "redis:7"
```

Servers are started in dependency order.

## Security Configuration

### Container Security

```yaml
servers:
  my-server:
    # User and group
    user: "1000:1000"           # Run as specific user:group
    groups: ["audio", "video"]  # Additional groups

    # Privilege settings
    privileged: false           # NEVER use true unless required
    read_only: true            # Read-only root filesystem

    # Temporary filesystems
    tmpfs:
      - "/tmp"
      - "/var/cache"

    # Linux capabilities
    cap_drop:
      - "ALL"                  # Drop all capabilities (recommended)
    cap_add:
      - "NET_BIND_SERVICE"    # Add only what's needed

    # Security options
    security_opt:
      - "no-new-privileges:true"  # Prevent privilege escalation
      - "apparmor:docker-default" # AppArmor profile
      - "seccomp:default"         # Seccomp profile
```

### MCP-Compose Security Policy

```yaml
servers:
  my-server:
    security:
      allow_docker_socket: false    # DANGEROUS - avoid unless necessary
      allow_host_mounts:            # Restrict allowed host mount paths
        - "/home/user/safe-dir"
        - "/tmp"
      allow_privileged_ops: false   # Disallow privileged operations
      trusted_image: true           # Mark as trusted image
      no_new_privileges: true       # Prevent privilege escalation
      apparmor: "default"           # AppArmor profile
      seccomp: "default"            # Seccomp profile
      selinux:                      # SELinux labels
        type: "container_t"
```

### Server-Level Authentication

```yaml
servers:
  my-server:
    authentication:
      enabled: true
      required_scope: "mcp:tools"   # Required OAuth scope
      optional_auth: false          # Allow unauthenticated access
      scopes: ["mcp:tools"]         # Allowed scopes
      allow_api_key: true           # Allow API key authentication

    # OR OAuth-specific settings
    oauth:
      enabled: true
      required_scope: "mcp:tools"
      allow_api_key_fallback: true
      optional_auth: false
      allowed_clients:              # Restrict to specific clients
        - "client1"
        - "client2"
```

## Resource Limits

### Deploy Configuration

```yaml
servers:
  my-server:
    deploy:
      resources:
        limits:                     # Maximum resources
          cpus: "1.0"              # CPU cores (1.0 = 1 core)
          memory: "512m"           # RAM limit
          memory_swap: "1g"        # Swap limit
          pids: 100                # Process limit
          blkio_weight: 500        # Block I/O weight (100-1000)

        reservations:              # Guaranteed resources
          cpus: "0.5"              # Guaranteed CPU
          memory: "256m"           # Guaranteed RAM

      restart_policy: "unless-stopped"  # "no", "always", "on-failure", "unless-stopped"
      replicas: 1                       # Number of instances

      update_config:                    # Rolling update settings
        parallelism: 1
        delay: "10s"
        failure_action: "pause"         # "pause", "continue", "rollback"
        monitor: "5s"
        max_failure_ratio: "0.3"
```

**Memory Formats:**
- `512m` - 512 megabytes
- `1g` - 1 gigabyte
- `1024k` - 1024 kilobytes
- `536870912` - bytes (without suffix)

## Networking

### Port Mappings

```yaml
servers:
  my-server:
    ports:
      - "8080:8080"                    # host:container
      - "127.0.0.1:8081:8081"         # bind to specific interface
      - "3000-3005:3000-3005"         # port range
```

### Network Configuration

```yaml
servers:
  my-server:
    networks:                          # Custom networks
      - "mcp-net"                      # Default network
      - "custom-net"                   # Additional network

    network_mode: "bridge"             # "bridge", "host", "none"
    hostname: "my-server"              # Container hostname
    domainname: "example.com"          # Container domain

    dns:                               # Custom DNS servers
      - "8.8.8.8"
      - "1.1.1.1"

    dns_search:                        # DNS search domains
      - "example.com"
      - "internal.com"

    extra_hosts:                       # Additional /etc/hosts entries
      - "host.docker.internal:host-gateway"
      - "api.local:192.168.1.10"
```

### Network Definitions

```yaml
networks:
  custom-net:
    driver: bridge                     # "bridge", "overlay", "host", "none"
    driver_opts:
      com.docker.network.bridge.name: "custom0"

    attachable: true                   # Allow manual container attachment
    enable_ipv6: false                 # Enable IPv6

    ipam:                              # IP address management
      driver: default
      config:
        - subnet: "172.20.0.0/16"
          gateway: "172.20.0.1"
      options:
        foo: bar

    internal: false                    # Internal network (no internet)
    labels:                            # Network labels
      environment: "production"

    external: false                    # Use existing external network
```

## Storage

### Volume Mounts

```yaml
servers:
  my-server:
    volumes:
      # Host path mounts
      - "/host/path:/container/path:ro"      # read-only
      - "/host/path:/container/path:rw"      # read-write
      - "${HOME}/data:/data:rw"              # environment variable

      # Named volumes
      - "named-volume:/data"

      # Temporary mounts
      - "/tmp:/tmp:rw"

    workdir: "/app"                          # Working directory
```

### Volume Definitions

```yaml
volumes:
  app-data:
    driver: local                            # "local", "nfs", etc.
    driver_opts:
      type: "nfs"
      o: "addr=192.168.1.1,rw"
      device: ":/path/to/dir"

    external: false                          # Use existing volume
    labels:
      backup: "daily"
      environment: "production"
```

## Health Checks & Lifecycle

### Health Checks

```yaml
servers:
  my-server:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
      # OR test: ["CMD-SHELL", "curl -f http://localhost/health || exit 1"]
      # OR test: ["NONE"]  # Disable health check

      interval: "30s"                 # Check every 30 seconds
      timeout: "10s"                  # Timeout per check
      retries: 3                      # Failures before unhealthy
      start_period: "40s"             # Initial grace period
```

### Lifecycle Hooks

```yaml
servers:
  my-server:
    lifecycle:
      pre_start: "echo 'Starting server...'"
      post_start: "curl http://localhost/warmup"
      pre_stop: "curl http://localhost/shutdown"
      post_stop: "echo 'Server stopped'"
```

### Stop Configuration

```yaml
servers:
  my-server:
    stop_signal: "SIGTERM"            # Signal to send on stop
    stop_grace_period: 30             # Seconds before SIGKILL
```

## Audit Logging

```yaml
audit:
  enabled: true
  log_level: "info"                   # "debug", "info", "warn", "error"
  storage: "memory"                   # "memory", "file", "postgres"

  retention:
    max_entries: 1000                 # Maximum audit entries
    max_age: "7d"                     # Keep entries for 7 days

  events:                             # Events to audit
    - "oauth.token.issued"
    - "oauth.token.revoked"
    - "oauth.user.login"
    - "server.access.granted"
    - "server.access.denied"
    - "tool.executed"
    - "resource.accessed"
```

## RBAC

```yaml
rbac:
  enabled: true

  scopes:
    - name: "mcp:*"
      description: "Full access to all MCP resources"
    - name: "mcp:tools"
      description: "Access to MCP tools"
    - name: "mcp:resources"
      description: "Access to MCP resources"
    - name: "mcp:prompts"
      description: "Access to MCP prompts"

  roles:
    admin:
      name: "admin"
      description: "Full administrative access"
      scopes: ["mcp:*"]

    developer:
      name: "developer"
      description: "Developer access"
      scopes: ["mcp:tools", "mcp:resources", "mcp:prompts"]

    readonly:
      name: "readonly"
      description: "Read-only access"
      scopes: ["mcp:resources"]
```

## Dashboard

```yaml
dashboard:
  enabled: true
  port: 3001
  host: "0.0.0.0"                     # "0.0.0.0" for all interfaces
  proxy_url: "http://localhost:9876"  # Required
  postgres_url: "postgres://user:pass@localhost:5432/db"

  theme: "dark"                       # "light" or "dark"
  log_streaming: true                 # Real-time log streaming
  config_editor: true                 # In-browser config editor
  metrics: true                       # Prometheus metrics

  security:
    enabled: true
    oauth_config: true                # OAuth management UI
    client_management: true           # Client management UI
    user_management: true             # User management UI
    audit_logs: true                  # Audit log viewer

  admin_login:
    enabled: true
    session_timeout: "24h"
```

## Timeouts

```yaml
connections:
  default:
    timeouts:
      connect: "10s"                  # Connection timeout
      read: "30s"                     # Read timeout
      write: "30s"                    # Write timeout
      idle: "60s"                     # Idle connection timeout
      health_check: "5s"              # Health check timeout
      shutdown: "30s"                 # Graceful shutdown timeout
      lifecycle_hook: "30s"           # Lifecycle hook timeout
```

## Logging

```yaml
servers:
  my-server:
    log_driver: "json-file"           # "json-file", "syslog", "journald", "none"
    log_options:
      max-size: "10m"                 # Maximum log file size
      max-file: "3"                   # Number of log files to keep
      compress: "true"                # Compress rotated logs

logging:
  level: "info"                       # "debug", "info", "warn", "error"
  format: "json"                      # "json" or "text"
  destinations:
    - type: stdout
    - type: file
      path: "/var/log/mcp-compose.log"
```

## Labels and Annotations

```yaml
servers:
  my-server:
    labels:
      com.example.service: "web"
      com.example.version: "1.0"
      environment: "production"

    annotations:
      description: "Web server component"
      maintainer: "team@example.com"
```

## Environment Variables

Environment variables are automatically loaded from:
1. System environment
2. `.env` file in config directory
3. Inline in configuration with `${}` or `$` syntax

### Common Variables

```bash
# Core authentication
export MCP_API_KEY="your-secure-api-key"

# OAuth
export OAUTH_CLIENT_SECRET="your-oauth-secret"

# Database
export POSTGRES_PASSWORD="your-db-password"
export DATABASE_URL="postgres://user:pass@host:5432/db"

# External services
export GITHUB_TOKEN="ghp_your-token"
export OPENROUTER_API_KEY="sk-or-v1-your-key"

# Environment selection
export MCP_ENV="production"  # "development", "staging", "production"
```

## Best Practices

### Security

```yaml
servers:
  secure-server:
    # Always run as non-root
    user: "1000:1000"

    # Drop all capabilities, add only what's needed
    cap_drop: ["ALL"]
    cap_add: ["NET_BIND_SERVICE"]

    # Use read-only filesystem
    read_only: true
    tmpfs: ["/tmp", "/var/cache"]

    # Prevent privilege escalation
    security_opt:
      - "no-new-privileges:true"

    # Limit resources
    deploy:
      resources:
        limits:
          cpus: "1.0"
          memory: "512m"
          pids: 100
```

### Performance

```yaml
servers:
  fast-server:
    # Use health checks for reliability
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
      interval: "30s"
      timeout: "5s"
      retries: 3

    # Configure appropriate timeouts
    connections:
      default:
        timeouts:
          connect: "5s"
          read: "15s"
          write: "15s"

    # Limit log size
    log_options:
      max-size: "10m"
      max-file: "3"
```

### Development vs Production

Use environment-specific configuration:

```yaml
environments:
  development:
    servers:
      my-server:
        env:
          DEBUG: "true"
          LOG_LEVEL: "debug"

  production:
    servers:
      my-server:
        env:
          DEBUG: "false"
          LOG_LEVEL: "info"
        deploy:
          resources:
            limits:
              memory: "1g"
```

Select environment:

```bash
export MCP_ENV="production"
mcp-compose up
```

## Examples

See the [examples directory](../../examples/) for complete working configurations:

- `examples/quickstart.yaml` - Minimal 1-server setup
- `examples/basic.yaml` - 3-server common setup
- `examples/advanced.yaml` - 10+ servers with OAuth and monitoring
- `examples/production.yaml` - Security-hardened production setup

## Validation

Validate your configuration:

```bash
mcp-compose validate
```

This checks:
- YAML syntax
- Required fields
- Type constraints
- Dependency cycles
- Security settings
- Resource format

## Migration from Docker Compose

Most Docker Compose configurations can be adapted with minimal changes:

```yaml
# docker-compose.yml → mcp-compose.yaml
version: '1'  # Change from '3.8' to '1'

servers:  # Change from 'services' to 'servers'
  my-service:
    image: "app:latest"
    # Most options are compatible
    ports: ["8080:8080"]
    volumes: ["data:/data"]
    environment:
      KEY: "value"
```

Key differences:
- `services:` becomes `servers:`
- Add `protocol:` and `capabilities:` for MCP support
- Use `proxy_auth:` instead of custom auth middleware
- Security defaults are more restrictive

See the [Migration Guide](../../MIGRATION.md) for details.