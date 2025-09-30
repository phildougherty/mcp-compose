# Architecture Overview

This document provides a comprehensive overview of MCP-Compose's architecture, design decisions, and component interactions.

## Table of Contents

- [System Architecture](#system-architecture)
- [Request Flow](#request-flow)
- [Authentication Flow](#authentication-flow)
- [Protocol Translation](#protocol-translation)
- [Container Orchestration](#container-orchestration)
- [Session Management](#session-management)
- [Component Descriptions](#component-descriptions)
- [Design Decisions](#design-decisions)

## System Architecture

MCP-Compose follows a layered architecture with clear separation of concerns:

```mermaid
graph TB
    subgraph "Client Layer"
        Claude[Claude Desktop]
        OpenWebUI[OpenWebUI]
        Custom[Custom Client]
    end

    subgraph "MCP-Compose Proxy"
        API[HTTP API Server]
        Auth[Authentication Layer]
        Router[Request Router]
        Session[Session Manager]
        Protocol[Protocol Translator]
    end

    subgraph "MCP Servers"
        FS[Filesystem Server<br/>STDIO]
        Mem[Memory Server<br/>HTTP]
        Search[Search Server<br/>SSE]
        Git[Git Server<br/>TCP]
    end

    subgraph "Container Runtime"
        Docker[Docker/Podman]
        Socat[Socat Bridges]
    end

    Claude --> API
    OpenWebUI --> API
    Custom --> API

    API --> Auth
    Auth --> Router
    Router --> Session
    Session --> Protocol

    Protocol --> FS
    Protocol --> Mem
    Protocol --> Search
    Protocol --> Git

    FS --> Docker
    Mem --> Docker
    Search --> Docker
    Git --> Docker

    FS -.->|STDIO| Socat
    Socat --> Docker

    style API fill:#4CAF50
    style Auth fill:#FF9800
    style Protocol fill:#2196F3
    style Docker fill:#00BCD4
```

## Request Flow

### HTTP Request Processing

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant Auth
    participant Session
    participant Protocol
    participant Server

    Client->>Proxy: HTTP POST /filesystem
    Proxy->>Auth: Validate credentials
    Auth->>Auth: Check API key/OAuth token

    alt Invalid credentials
        Auth-->>Client: 401 Unauthorized
    else Valid credentials
        Auth->>Proxy: Authorized
        Proxy->>Session: Get/create session
        Session->>Session: Check existing connection

        alt No session exists
            Session->>Protocol: Create new connection
            Protocol->>Server: Initialize MCP session
            Server-->>Protocol: Session initialized
            Protocol-->>Session: Connection ready
        end

        Session->>Protocol: Translate HTTP to MCP
        Protocol->>Server: MCP JSON-RPC request
        Server->>Server: Execute tool/fetch resource
        Server-->>Protocol: MCP JSON-RPC response
        Protocol->>Session: Translate MCP to HTTP
        Session-->>Proxy: HTTP response
        Proxy-->>Client: 200 OK + JSON response
    end
```

### Tool Execution Flow

```mermaid
graph LR
    A[Client Request] --> B{Authentication}
    B -->|Valid| C[Session Manager]
    B -->|Invalid| D[401 Error]

    C --> E{Session Exists?}
    E -->|Yes| F[Use Existing]
    E -->|No| G[Create New]

    G --> H[MCP Initialize]
    H --> I[Store Session]
    I --> F

    F --> J[Translate to JSON-RPC]
    J --> K{Protocol Type}

    K -->|HTTP| L[HTTP Client]
    K -->|STDIO| M[Socat Bridge]
    K -->|SSE| N[SSE Client]
    K -->|TCP| O[TCP Client]

    L --> P[MCP Server]
    M --> P
    N --> P
    O --> P

    P --> Q[Execute Tool]
    Q --> R[Return Result]
    R --> S[Translate Response]
    S --> T[Send to Client]

    style B fill:#FF9800
    style K fill:#2196F3
    style P fill:#4CAF50
```

## Authentication Flow

### OAuth 2.1 with PKCE

```mermaid
sequenceDiagram
    participant Client
    participant Browser
    participant Proxy as MCP-Compose Proxy
    participant AuthServer as OAuth Server

    Note over Client,AuthServer: Authorization Code Flow with PKCE

    Client->>Client: Generate code_verifier
    Client->>Client: Generate code_challenge = SHA256(code_verifier)

    Client->>Browser: Redirect to /oauth/authorize
    Browser->>Proxy: GET /oauth/authorize?<br/>response_type=code&<br/>client_id=xxx&<br/>redirect_uri=xxx&<br/>scope=mcp:tools&<br/>state=xxx&<br/>code_challenge=xxx&<br/>code_challenge_method=S256

    Proxy->>Proxy: Validate parameters
    Proxy->>Browser: Show consent screen
    Browser->>Browser: User approves

    Browser->>Proxy: POST /oauth/authorize (approval)
    Proxy->>Proxy: Generate authorization_code
    Proxy->>Proxy: Store code + code_challenge
    Proxy->>Browser: Redirect to redirect_uri?code=xxx&state=xxx

    Browser->>Client: Return to app with code

    Client->>Proxy: POST /oauth/token<br/>grant_type=authorization_code&<br/>code=xxx&<br/>redirect_uri=xxx&<br/>code_verifier=xxx&<br/>client_id=xxx

    Proxy->>Proxy: Verify code_challenge matches<br/>SHA256(code_verifier)
    Proxy->>Proxy: Generate access_token (JWT)
    Proxy->>Proxy: Generate refresh_token

    Proxy-->>Client: {<br/>"access_token": "...",<br/>"token_type": "Bearer",<br/>"expires_in": 3600,<br/>"refresh_token": "..."<br/>}

    Client->>Proxy: Request with Authorization: Bearer access_token
    Proxy->>Proxy: Validate JWT
    Proxy-->>Client: Protected resource

    Note over Client,Proxy: Token Refresh
    Client->>Proxy: POST /oauth/token<br/>grant_type=refresh_token&<br/>refresh_token=xxx
    Proxy->>Proxy: Validate refresh token
    Proxy->>Proxy: Generate new access_token
    Proxy-->>Client: New access_token
```

### API Key Authentication

```mermaid
graph LR
    A[Client Request] --> B[Extract Authorization Header]
    B --> C{Header Present?}
    C -->|No| D[401 Unauthorized]
    C -->|Yes| E[Parse Bearer Token]

    E --> F{Format Valid?}
    F -->|No| D
    F -->|Yes| G[Compare with MCP_API_KEY]

    G --> H{Match?}
    H -->|No| D
    H -->|Yes| I[Request Allowed]

    I --> J[Execute Request]
    J --> K[Return Response]

    style D fill:#F44336
    style I fill:#4CAF50
```

## Protocol Translation

MCP-Compose translates between HTTP and various MCP transport protocols:

```mermaid
graph TB
    subgraph "HTTP Layer"
        HTTPReq[HTTP Request]
        HTTPResp[HTTP Response]
    end

    subgraph "Protocol Translator"
        Parser[JSON-RPC Parser]
        Encoder[JSON-RPC Encoder]
        Router[Transport Router]
    end

    subgraph "Transport Layer"
        STDIO[STDIO via Socat]
        HTTP[Native HTTP]
        SSE[Server-Sent Events]
        TCP[Raw TCP]
    end

    HTTPReq --> Parser
    Parser --> Router

    Router --> STDIO
    Router --> HTTP
    Router --> SSE
    Router --> TCP

    STDIO --> Encoder
    HTTP --> Encoder
    SSE --> Encoder
    TCP --> Encoder

    Encoder --> HTTPResp

    style Parser fill:#2196F3
    style Encoder fill:#2196F3
    style Router fill:#FF9800
```

### STDIO Protocol Bridge

STDIO servers communicate via standard input/output, which MCP-Compose bridges to TCP using socat:

```mermaid
graph LR
    A[HTTP Client] --> B[MCP Proxy]
    B --> C[TCP Socket]
    C <--> D[Socat Bridge]
    D <--> E[STDIO Server]
    E <--> F[Container Process]

    style D fill:#FF9800
    style E fill:#4CAF50
```

Process:
1. Container starts with socat listening on TCP port
2. Socat forwards TCP to server's STDIO
3. Proxy connects via TCP, translates HTTP to JSON-RPC
4. JSON-RPC flows through socat to server's stdin
5. Server writes response to stdout, flows back through socat

## Container Orchestration

```mermaid
stateDiagram-v2
    [*] --> Parsing: Load config
    Parsing --> Validating: Parse YAML
    Validating --> NetworkCreation: Validate

    NetworkCreation --> VolumeCreation: Create networks
    VolumeCreation --> DependencySort: Create volumes

    DependencySort --> Starting: Sort by depends_on
    Starting --> ImagePull: Start servers

    ImagePull --> Building: Pull if needed
    Building --> Creating: Build if needed
    Creating --> Hooks: Create container

    Hooks --> Running: Execute pre_start
    Running --> HealthCheck: Start container

    HealthCheck --> Healthy: Monitor health
    Healthy --> [*]: All running

    HealthCheck --> Unhealthy: Failed checks
    Unhealthy --> Restarting: restart_policy
    Restarting --> Creating: Restart

    Running --> Stopping: User stop
    Stopping --> Cleanup: Execute pre_stop
    Cleanup --> [*]: Execute post_stop
```

### Dependency Resolution

```mermaid
graph TB
    A[web] --> B[redis]
    A --> C[postgres]
    A --> D[memory]
    D --> C

    subgraph "Start Order"
        direction LR
        S1[1. postgres]
        S2[2. redis]
        S3[3. memory]
        S4[4. web]

        S1 --> S2
        S2 --> S3
        S3 --> S4
    end

    style S1 fill:#4CAF50
    style S2 fill:#4CAF50
    style S3 fill:#4CAF50
    style S4 fill:#4CAF50
```

## Session Management

MCP-Compose maintains persistent sessions between the proxy and MCP servers:

```mermaid
graph TB
    subgraph "Session Manager"
        SM[Session Store]
        Pool[Connection Pool]
        Health[Health Monitor]
    end

    subgraph "Sessions"
        S1[Session 1<br/>filesystem<br/>HTTP]
        S2[Session 2<br/>memory<br/>STDIO]
        S3[Session 3<br/>search<br/>SSE]
    end

    subgraph "Servers"
        FS[Filesystem Server]
        Mem[Memory Server]
        Search[Search Server]
    end

    SM --> S1
    SM --> S2
    SM --> S3

    Pool --> S1
    Pool --> S2
    Pool --> S3

    Health --> S1
    Health --> S2
    Health --> S3

    S1 <--> FS
    S2 <--> Mem
    S3 <--> Search

    style SM fill:#4CAF50
    style Health fill:#FF9800
```

### Session Lifecycle

```mermaid
sequenceDiagram
    participant Client
    participant Proxy
    participant SessionMgr as Session Manager
    participant Server

    Client->>Proxy: First request
    Proxy->>SessionMgr: Get session for "filesystem"
    SessionMgr->>SessionMgr: Check if session exists

    alt Session doesn't exist
        SessionMgr->>Server: Connect to server
        Server-->>SessionMgr: Connection established
        SessionMgr->>Server: Send initialize request
        Server-->>SessionMgr: Server capabilities
        SessionMgr->>SessionMgr: Store session
    end

    SessionMgr-->>Proxy: Return session
    Proxy->>Server: Send MCP request (via session)
    Server-->>Proxy: MCP response
    Proxy-->>Client: HTTP response

    Note over SessionMgr,Server: Session kept alive for subsequent requests

    Client->>Proxy: Second request (same server)
    Proxy->>SessionMgr: Get session for "filesystem"
    SessionMgr-->>Proxy: Return existing session (fast)
    Proxy->>Server: Send MCP request
    Server-->>Proxy: MCP response
    Proxy-->>Client: HTTP response
```

## Component Descriptions

### HTTP API Server

**Location:** `internal/server/proxy.go`

Responsibilities:
- HTTP request handling
- Route matching and dispatch
- OpenAPI specification generation
- CORS configuration
- Error handling and logging

Key features:
- RESTful API endpoints
- WebSocket support for real-time communication
- Automatic OpenAPI documentation per server
- Request/response logging

### Authentication Layer

**Location:** `internal/auth/`

Components:
- `api_key.go` - API key validation
- `oauth.go` - OAuth 2.1 server implementation
- `middleware.go` - Authentication middleware
- `jwt.go` - JWT token generation and validation

Features:
- API key authentication
- OAuth 2.1 with PKCE
- JWT-based access tokens
- Refresh token rotation
- Token revocation
- RBAC integration

### Session Manager

**Location:** `internal/server/session_manager.go`

Responsibilities:
- Connection pooling
- Session lifecycle management
- Health monitoring
- Automatic reconnection
- Connection cleanup

Features:
- Persistent connections to MCP servers
- Per-server session isolation
- Idle connection cleanup
- Health check integration
- Graceful shutdown

### Protocol Translator

**Location:** `internal/protocol/`

Components:
- `translator.go` - Core translation logic
- `http_client.go` - HTTP transport
- `stdio_client.go` - STDIO transport (via socat)
- `sse_client.go` - SSE transport
- `tcp_client.go` - TCP transport

Features:
- HTTP to JSON-RPC translation
- Protocol-specific connection handling
- Request/response streaming
- Error mapping
- Timeout management

### Container Runtime

**Location:** `internal/container/`

Components:
- `runtime.go` - Runtime abstraction interface
- `docker.go` - Docker implementation
- `podman.go` - Podman implementation
- `manager.go` - Lifecycle orchestration

Features:
- Docker and Podman support
- Image building and pulling
- Container lifecycle management
- Network creation and management
- Volume management
- Health check execution

### Configuration Manager

**Location:** `internal/config/`

Components:
- `config.go` - Configuration structures
- `validation.go` - Validation logic
- `loader.go` - YAML parsing

Features:
- YAML configuration parsing
- Environment variable expansion
- Configuration validation
- Environment-specific overrides
- Default value handling

## Design Decisions

### Why Docker/Podman?

**Decision:** Use containerization as the primary deployment model.

**Rationale:**
- Consistent environment across platforms
- Dependency isolation
- Resource limits and security controls
- Easy distribution and updates
- Familiar to developers (Docker Compose-style)

**Trade-offs:**
- Requires Docker/Podman installation
- Slightly higher resource usage
- Additional layer of complexity

### Why HTTP Proxy?

**Decision:** Expose all MCP servers through a unified HTTP proxy.

**Rationale:**
- Single endpoint for clients
- Protocol translation (STDIO/SSE/TCP to HTTP)
- Centralized authentication and authorization
- Session management and connection pooling
- OpenAPI documentation generation
- CORS and security headers

**Trade-offs:**
- Additional network hop
- Slight latency increase (~5-10ms)
- Single point of failure (mitigated by health checks)

### Why Session Persistence?

**Decision:** Maintain persistent connections between proxy and servers.

**Rationale:**
- Avoid repeated MCP initialize handshakes
- Better performance for sequential requests
- Maintain server-side state across requests
- Support for streaming operations

**Trade-offs:**
- Memory overhead for session storage
- Complexity in cleanup and health monitoring
- Potential for stale connections

### Why YAML Configuration?

**Decision:** Use YAML for configuration with Docker Compose-style syntax.

**Rationale:**
- Familiar to Docker users
- Human-readable and writable
- Comments support
- Good tooling ecosystem
- Environment variable expansion

**Trade-offs:**
- YAML parsing complexity
- Indentation sensitivity
- Limited validation at parse time

### Why OAuth 2.1 with PKCE?

**Decision:** Implement full OAuth 2.1 specification with PKCE.

**Rationale:**
- Industry-standard authentication
- Supports multiple clients and applications
- Secure for native/mobile apps (PKCE)
- Fine-grained access control (scopes)
- Token refresh and revocation

**Trade-offs:**
- More complex than simple API keys
- Requires token management
- OAuth flow adds setup complexity

### Why Multiple Protocols?

**Decision:** Support STDIO, HTTP, SSE, and TCP transports.

**Rationale:**
- STDIO: Legacy MCP server compatibility
- HTTP: Modern, stateless, easy to debug
- SSE: Real-time streaming, server-push
- TCP: Raw connections for custom protocols

**Trade-offs:**
- Multiple code paths to maintain
- Different error handling per protocol
- Testing complexity

## Performance Characteristics

### Latency

- HTTP Proxy overhead: 5-10ms (p50), 15-30ms (p99)
- STDIO translation: +2-5ms (socat overhead)
- Session reuse: ~0ms (no reconnection)
- Cold start: 100-500ms (initialize + first request)

### Throughput

- Concurrent connections: 1000+ per server
- Request rate: 10,000+ req/sec (HTTP protocol)
- STDIO throughput: Limited by socat buffer (typically 500-1000 req/sec)

### Resource Usage

- Proxy memory: 50-100MB base + ~1MB per active session
- Container overhead: ~10MB per container
- Network overhead: Minimal (<1% CPU for routing)

## Scalability Considerations

### Horizontal Scaling

MCP-Compose can be scaled horizontally by:
1. Running multiple proxy instances behind a load balancer
2. Using shared session storage (Redis/PostgreSQL)
3. Deploying replicas of MCP servers

### Vertical Scaling

Resource limits can be adjusted per server:
```yaml
deploy:
  resources:
    limits:
      cpus: "2.0"
      memory: "2g"
```

### High Availability

For production deployments:
1. Run multiple proxy instances
2. Use external session storage
3. Configure health checks and auto-restart
4. Set up monitoring and alerting
5. Use deployment strategies (rolling updates)

## Security Architecture

### Defense in Depth

```mermaid
graph TB
    A[Network Layer] --> B[TLS/HTTPS]
    B --> C[Authentication Layer]
    C --> D[Authorization/RBAC]
    D --> E[Container Security]
    E --> F[Resource Limits]
    F --> G[Audit Logging]

    style A fill:#FF9800
    style C fill:#FF9800
    style D fill:#FF9800
    style E fill:#FF9800
    style G fill:#4CAF50
```

Layers:
1. Network security (TLS, firewall)
2. Authentication (API key, OAuth)
3. Authorization (RBAC, scopes)
4. Container isolation (user namespaces, capabilities)
5. Resource limits (CPU, memory, PIDs)
6. Audit logging (all operations tracked)

## Monitoring and Observability

### Metrics

Prometheus-compatible metrics:
- Request count and latency
- Error rates
- Active connections
- Session pool utilization
- Container resource usage

### Logging

Structured JSON logging:
- Request/response logs
- Authentication events
- Error traces
- Audit events

### Health Checks

Multiple health check levels:
- Container health (Docker healthcheck)
- Server health (MCP protocol check)
- Proxy health (HTTP endpoint)

## Future Architecture Considerations

Potential improvements for future versions:

1. **Kubernetes Support** (branch `kubernetes-native`)
   - Custom Resource Definitions (CRDs)
   - Operators for automated management
   - Service mesh integration

2. **Distributed Tracing**
   - OpenTelemetry integration
   - Request tracing across services
   - Performance profiling

3. **Advanced Caching**
   - Response caching
   - Tool result memoization
   - Distributed cache (Redis)

4. **Plugin System**
   - Custom authentication providers
   - Protocol extensions
   - Middleware plugins

5. **Multi-Tenancy**
   - Per-tenant isolation
   - Quota management
   - Billing integration