# MCP-Compose Production Readiness Roadmap

**Generated:** 2025-09-29
**Branch Strategy:** Work on `main` branch (Docker/Podman focus)
**Purpose:** Production-ready Docker/Podman-based MCP orchestration
**Philosophy:** Keep it simple - Docker Compose for MCP servers

---

## Branch Strategy

### Main Branch (Public - Simple & Accessible)
- **Focus**: Docker/Podman only - no Kubernetes complexity
- **Target Users**: Individual developers, small teams, local development
- **Value Prop**: "5-minute setup, Docker Compose for MCP servers"

### kubernetes-native Branch (Private - Advanced)
- **Focus**: Full Kubernetes support with CRDs, Controllers, Operators
- **Target Users**: Enterprise teams with existing K8s infrastructure
- **Value Prop**: "Production-scale MCP orchestration in Kubernetes"

**Decision:** Keep branches separate. Main stays simple, K8s is opt-in advanced feature.

---

## Executive Summary

### Current State (main branch on GitHub)
- **Clean codebase** - Docker/Podman focus, no K8s complexity
- **40,026 LOC** focused on core functionality
- **Working features**: Docker/Podman runtime, HTTP proxy, basic auth
- **Missing**: OAuth completion, AI features, better UX, comprehensive docs

### Target State (12 weeks)
- **Production-ready security** - Complete OAuth, rate limiting, audit logging
- **AI-powered features** - Context management, smart task scheduling
- **80%+ test coverage** with comprehensive integration tests
- **Excellent UX** - 5-minute quickstart, interactive wizards, great docs
- **World-class docs** - Getting started, API reference, video tutorials

### Priority System
- **P0 (Critical)**: Security vulnerabilities, broken functionality - 2 weeks
- **P1 (High)**: Core features for production readiness - 6 weeks
- **P2 (Medium)**: Quality of life, polish, documentation - 4 weeks

---

## PHASE 1: SECURITY & STABILITY (P0) - 2 Weeks

> **Goal**: Production-ready authentication, fix broken stubs, security hardening

### 1.1 Complete OAuth Implementation (P0) - 1 Week

#### 1.1.1 Authorization Endpoint
- [ ] **Implement HandleAuthorize** (internal/auth/oauth.go)
  - Generate authorization code
  - Render consent screen HTML
  - Validate PKCE code_challenge
  - Validate state parameter
  - Store authorization code with expiry (10 minutes)
  - Redirect to redirect_uri with code
  - Location: `internal/auth/oauth.go:150-250`

#### 1.1.2 Token Endpoint
- [ ] **Implement HandleToken** (internal/auth/oauth.go)
  - Handle authorization_code grant
  - Handle refresh_token grant
  - Handle client_credentials grant
  - Verify PKCE code_verifier
  - Generate access token (JWT, 1 hour TTL)
  - Generate refresh token (7 days TTL)
  - Implement token rotation for refresh
  - Location: `internal/auth/oauth.go:250-400`

#### 1.1.3 Userinfo Endpoint
- [ ] **Implement HandleUserinfo** (internal/auth/oauth.go)
  - Extract token from Authorization header
  - Validate JWT signature
  - Return user claims based on scopes
  - Filter claims by requested scopes
  - Location: `internal/auth/oauth.go:400-450`

#### 1.1.4 Additional OAuth Endpoints
- [ ] **Implement HandleRevoke** for token revocation
  - Revoke access and refresh tokens
  - Add to revocation list
  - Location: `internal/auth/oauth.go:450-500`

- [ ] **Implement discovery endpoint** (RFC 8414)
  - Route: `/.well-known/oauth-authorization-server`
  - Return metadata: endpoints, grants, scopes, algorithms
  - Location: `internal/auth/oauth.go:500-550`

### 1.2 Rate Limiting & Request Validation (P0) - 0.5 Weeks

- [ ] **Add rate limiting middleware** (internal/server/rate_limit.go - new file)
  - Use golang.org/x/time/rate for token bucket
  - Per-IP rate limiting (configurable, default: 100 req/min)
  - Per-API-key rate limiting (default: 1000 req/min)
  - Per-OAuth-client rate limiting (default: 500 req/min)
  - Return 429 Too Many Requests with Retry-After header
  - Add metrics: requests_rate_limited_total
  - Location: `internal/server/rate_limit.go`

- [ ] **Add comprehensive request validation** (internal/server/validation.go - new file)
  - Validate Content-Type (application/json)
  - Validate request body size (max 10MB)
  - JSON schema validation for all endpoints
  - Parameter sanitization (strip HTML, SQL)
  - Return 400 Bad Request with detailed errors
  - Location: `internal/server/validation.go`

### 1.3 Audit Logging & Secret Management (P0) - 0.5 Weeks

#### 1.3.1 Complete Audit Storage
- [ ] **Implement file-based audit storage** (internal/audit/file_storage.go - new file)
  - JSON format with one event per line
  - Rotation policy (size: 100MB, age: 7 days)
  - Compression of old logs (gzip)
  - Location: `internal/audit/file_storage.go`

- [ ] **Implement PostgreSQL audit storage** (internal/audit/postgres_storage.go - new file)
  - Table schema: `audit_events` with indexes
  - Bulk insert for performance
  - Connection pooling
  - Location: `internal/audit/postgres_storage.go`

#### 1.3.2 Secret Management
- [ ] **Implement secret rotation** (internal/auth/secrets.go - new file)
  - Support multiple active API keys (primary + secondary)
  - Rotate without downtime
  - Add `mcp-compose rotate-secret` command
  - Location: `internal/auth/secrets.go` + `internal/cmd/rotate-secret.go`

- [ ] **Encrypt secrets at rest** (internal/config/encryption.go - new file)
  - Use AES-256-GCM for encryption
  - Derive key from master password or env var
  - Transparent decrypt on load
  - Location: `internal/config/encryption.go`

---

## PHASE 2: AI & SMART FEATURES (P1) - 3 Weeks

> **Goal**: Add intelligent features without complexity

### 2.1 AI Provider Infrastructure (P1) - 1 Week

#### 2.1.1 Provider Abstraction
- [ ] **Create provider interface** (internal/ai/provider.go - new package)
  ```go
  type Provider interface {
      Chat(ctx context.Context, messages []Message) (string, error)
      Stream(ctx context.Context, messages []Message) (<-chan string, error)
      Health(ctx context.Context) error
  }
  ```

- [ ] **Implement Claude provider** (internal/ai/claude.go)
  - Anthropic API integration
  - SSE streaming support
  - Error handling and retries
  - Location: `internal/ai/claude.go`

- [ ] **Implement OpenAI provider** (internal/ai/openai.go)
  - OpenAI API integration
  - Streaming support
  - Model selection
  - Location: `internal/ai/openai.go`

- [ ] **Implement Ollama provider** (internal/ai/ollama.go)
  - Local Ollama HTTP API
  - Streaming support
  - Model management
  - Location: `internal/ai/ollama.go`

- [ ] **Implement OpenRouter provider** (internal/ai/openrouter.go)
  - OpenRouter API integration
  - Multi-model support
  - Cost tracking
  - Location: `internal/ai/openrouter.go`

#### 2.1.2 Provider Manager
- [ ] **Create provider manager** (internal/ai/manager.go)
  - Fallback chain: Claude → OpenAI → Ollama → OpenRouter
  - Health monitoring (every 30 seconds)
  - Automatic failover
  - Circuit breaker pattern
  - Location: `internal/ai/manager.go`

### 2.2 Context Management (P1) - 1 Week

- [ ] **Create context window manager** (internal/context/manager.go - new package)
  - Token counting via tiktoken-go
  - Configurable max tokens (default: 32,000)
  - Priority-based ordering
  - Thread-safe operations
  - Location: `internal/context/manager.go`

- [ ] **Implement truncation strategies** (internal/context/truncation.go)
  - Oldest, LRU, ByType, ByPriority, Intelligent
  - Configurable strategy
  - Location: `internal/context/truncation.go`

- [ ] **Add context persistence** (internal/context/persistence.go)
  - Save to SQLite
  - Restore on restart
  - Expire old context (24h TTL)
  - Location: `internal/context/persistence.go`

### 2.3 Enhanced Memory Service (P1) - 1 Week

- [ ] **Create enhanced PostgreSQL schema** (internal/memory/schema.sql - new file)
  - Entity-Relation-Observation model
  - Full-text search indexes
  - Vector embeddings (pgvector)
  - Location: `internal/memory/schema.sql`

- [ ] **Implement semantic search** (internal/memory/semantic.go)
  - pgvector support
  - Generate embeddings via AI
  - Similarity search
  - Location: `internal/memory/semantic.go`

- [ ] **Add memory pruning** (internal/memory/pruning.go)
  - LRU eviction
  - Importance-based retention
  - Archive to cold storage
  - Location: `internal/memory/pruning.go`

---

## PHASE 3: DEVELOPER EXPERIENCE (P1) - 2 Weeks

> **Goal**: Make mcp-compose a joy to use

### 3.1 Interactive Setup Wizard (P1) - 0.5 Weeks

- [ ] **Create `mcp-compose init` command** (internal/cmd/init.go)
  - Interactive TUI with charmbracelet/bubbletea
  - Choose profile (development, staging, production)
  - Select servers (filesystem, memory, search, git)
  - Configure authentication
  - Generate mcp-compose.yaml
  - Validate prerequisites
  - Location: `internal/cmd/init.go`

### 3.2 Configuration Validation (P1) - 0.5 Weeks

- [ ] **Create JSON schema** (config/schema.json)
  - Full schema for mcp-compose.yaml
  - Required vs optional fields
  - Type constraints
  - Location: `config/schema.json`

- [ ] **Implement validation on load** (internal/config/validation.go)
  - Validate against schema
  - Friendly error messages
  - Suggest fixes
  - Location: `internal/config/validation.go`

- [ ] **Enhance `mcp-compose validate` command** (internal/cmd/validate.go)
  - Check image availability
  - Validate OAuth setup
  - Test environment variables
  - Location: `internal/cmd/validate.go`

### 3.3 Enhanced CLI Commands (P1) - 0.5 Weeks

- [ ] **Implement `mcp-compose top` command** (internal/cmd/top.go)
  - Real-time resource usage
  - Terminal UI with bubbletea
  - Sortable columns
  - Location: `internal/cmd/top.go`

- [ ] **Implement `mcp-compose inspect` command** (internal/cmd/inspect.go)
  - Detailed diagnostics
  - Configuration dump
  - Connection status
  - Health check results
  - Location: `internal/cmd/inspect.go`

- [ ] **Enhance `mcp-compose logs` command** (internal/cmd/logs.go)
  - Filter by level, timestamp
  - Grep pattern matching
  - Colored output
  - Location: `internal/cmd/logs.go`

### 3.4 Service Discovery (P1) - 0.5 Weeks

- [ ] **Implement Docker label-based discovery** (internal/discovery/docker.go - new package)
  - Watch containers with `mcp.compose/role=server` label
  - Extract metadata
  - Track health status
  - Auto-register with proxy
  - Location: `internal/discovery/docker.go`

- [ ] **Add health check system** (internal/discovery/health.go)
  - Periodic checks (every 10s)
  - Mark unhealthy after 3 failures
  - Re-add when healthy
  - Location: `internal/discovery/health.go`

---

## PHASE 4: DOCUMENTATION & POLISH (P2) - 2 Weeks

> **Goal**: World-class documentation and examples

### 4.1 Documentation Overhaul (P2) - 1 Week

- [ ] **Create docs/ directory structure**
  - getting-started/, configuration/, architecture/, troubleshooting/
  - Location: `docs/`

- [ ] **Write 5-minute quickstart** (docs/getting-started/quickstart.md)
  - Prerequisites
  - Install
  - Run init
  - Start servers
  - Test

- [ ] **Document all config options** (docs/configuration/reference.md)
  - Every option with examples
  - Environment variables
  - Best practices

- [ ] **Create architecture diagrams** (docs/architecture/overview.md)
  - System architecture
  - Request flow
  - Authentication flow
  - Use Mermaid diagrams

- [ ] **Write troubleshooting guide** (docs/troubleshooting/guide.md)
  - Common errors and solutions
  - Debug mode
  - Performance tuning

- [ ] **Create example progression** (examples/)
  - quickstart.yaml (1 server)
  - basic.yaml (3 servers)
  - advanced.yaml (10+ servers)

- [ ] **Create video tutorial** (YouTube)
  - 10-minute walkthrough
  - Screen recording
  - Published and linked in README

### 4.2 Configuration Simplification (P2) - 0.5 Weeks

- [ ] **Simplify config structure** (internal/config/config.go)
  - Group related settings
  - Reduce top-level keys to <15
  - Better defaults

- [ ] **Create configuration profiles** (config/profiles/)
  - development.yaml
  - staging.yaml
  - production.yaml

### 4.3 README Overhaul (P2) - 0.5 Weeks

- [ ] **Rewrite README.md**
  - Clear value proposition
  - 30-second quickstart
  - Feature highlights
  - Updated badges

---

## PHASE 5: TESTING & QUALITY (P2) - 2 Weeks

> **Goal**: 80%+ test coverage, production quality

### 5.1 Unit Tests (P1) - 1 Week

- [ ] **Test internal/auth/** (90%+ coverage)
  - OAuth flows
  - API key validation
  - Rate limiting
  - Location: `internal/auth/*_test.go`

- [ ] **Test internal/config/** (85%+ coverage)
  - Parsing, validation
  - Environment expansion
  - Location: `internal/config/*_test.go`

- [ ] **Test internal/protocol/** (80%+ coverage)
  - JSON-RPC parsing
  - Request/response handling
  - Location: `internal/protocol/*_test.go`

- [ ] **Test internal/server/** (80%+ coverage)
  - Routing, connection management
  - Protocol translation
  - Location: `internal/server/*_test.go`

- [ ] **Test internal/ai/** (85%+ coverage)
  - Mock providers
  - Fallback logic
  - Location: `internal/ai/*_test.go`

### 5.2 Integration Tests (P1) - 1 Week

- [ ] **Create integration test framework** (test/integration/)
  - Docker Compose test environment
  - Test helpers
  - Mock servers
  - Location: `test/integration/framework.go`

- [ ] **Test end-to-end workflows** (test/integration/e2e_test.go)
  - Full lifecycle
  - OAuth flow
  - Multi-server scenarios
  - Location: `test/integration/e2e_test.go`

- [ ] **Test Docker runtime** (test/integration/docker_test.go)
  - Container lifecycle
  - Volume management
  - Health checks
  - Location: `test/integration/docker_test.go`

### 5.3 Coverage & CI (P2)

- [ ] **Set up coverage reporting** (scripts/coverage.sh)
  - HTML reports
  - Track per package
  - Fail if <80%
  - Upload to codecov.io

- [ ] **Enhance CI/CD workflow** (.github/workflows/ci.yml)
  - Tests, linting, security scans
  - Build Docker image
  - Publish on tag

### 5.4 Code Quality (P2)

- [ ] **Remove all debug code**
  - Search for fmt.Printf
  - Replace with structured logging

- [ ] **Fix context usage**
  - Replace context.TODO()
  - Add cancellation
  - Add timeouts

- [ ] **Improve error handling**
  - Wrap errors with context
  - Structured error types
  - Error codes

---

## PHASE 6: PERFORMANCE & POLISH (P2) - 2 Weeks

> **Goal**: Fast, scalable, production-ready

### 6.1 Connection Management (P1) - 1 Week

- [ ] **Implement connection pooling** (internal/server/pool.go - new file)
  - Pool per server (default: 10)
  - Connection reuse
  - Idle timeout (60s)
  - Location: `internal/server/pool.go`

- [ ] **Add connection limits** (internal/server/connection_manager.go)
  - Global limit (1000)
  - Per-server limit (100)
  - Queue excess
  - Location: `internal/server/connection_manager.go`

- [ ] **Fix memory leaks** (internal/server/*_connections.go)
  - Proper cleanup
  - Context cancellation
  - Profile with pprof

- [ ] **Add connection metrics** (internal/server/metrics.go)
  - Active connections
  - Pool utilization
  - Errors
  - Expose via /metrics

### 6.2 Caching (P2) - 0.5 Weeks

- [ ] **Implement proper cache** (internal/server/cache.go - new file)
  - Use patrickmn/go-cache
  - TTL per item (5 min)
  - Max size (1000 items)
  - LRU eviction
  - Location: `internal/server/cache.go`

- [ ] **Add cache invalidation**
  - On server restart
  - On config change
  - Manual endpoint
  - TTL expiry

- [ ] **Add cache metrics**
  - Hit/miss rate
  - Eviction rate
  - Size

### 6.3 Monitoring (P2) - 0.5 Weeks

- [ ] **Add Prometheus metrics** (internal/server/metrics.go)
  - Request count, duration, errors
  - Active connections
  - Expose via /metrics
  - Location: `internal/server/metrics.go`

- [ ] **Add profiling endpoints** (internal/server/profiling.go - new file)
  - CPU, memory, goroutine profiling
  - Enabled via --enable-profiling
  - Location: `internal/server/profiling.go`

---

## SUCCESS CRITERIA

### Code Quality Metrics
- [ ] **Test coverage > 80%**
- [ ] **Zero critical security vulnerabilities**
- [ ] **Linting score > 95%**
- [ ] **All stubbed functions implemented**

### Feature Metrics
- [ ] **Complete OAuth** (authorization, token, userinfo, revoke)
- [ ] **AI providers working** (Claude, OpenAI, Ollama, OpenRouter)
- [ ] **Context management functional**
- [ ] **Enhanced memory service** (semantic search)

### Documentation Metrics
- [ ] **Complete docs** (getting-started, configuration, API)
- [ ] **10+ examples** (quickstart → advanced)
- [ ] **Video tutorial** (YouTube, 10+ min)
- [ ] **Troubleshooting guide**

### Performance Metrics
- [ ] **Proxy latency < 10ms (p50)**
- [ ] **Proxy latency < 50ms (p99)**
- [ ] **Support 1000+ concurrent connections**
- [ ] **Memory usage < 500MB base**
- [ ] **Startup time < 5 seconds**

---

## TIMELINE

### Week-by-Week (12 weeks total)

**Week 1-2: Security (Phase 1)**
- Complete OAuth, rate limiting, audit logging, secret management

**Week 3-5: AI Features (Phase 2)**
- AI providers, context management, enhanced memory

**Week 6-7: Developer Experience (Phase 3)**
- Interactive wizard, enhanced CLI, service discovery

**Week 8-9: Documentation (Phase 4)**
- Docs overhaul, examples, video tutorial

**Week 10-11: Testing (Phase 5)**
- Unit tests, integration tests, 80%+ coverage

**Week 12: Performance (Phase 6)**
- Connection management, caching, monitoring

---

## QUICK START FOR SUBAGENTS

### How to Use This TODO

1. **Work on main branch** - Docker/Podman focus
2. **Pick a Phase** - Start with P0 or P1
3. **Choose a Section** - Independent work items
4. **Create Tests** - 80%+ coverage target
5. **Update Docs** - User-facing changes
6. **Submit PR** - Reference TODO item

### Recommended Starting Points

**Security Engineers:** Phase 1 - OAuth, rate limiting, audit logging
**AI/ML Engineers:** Phase 2.1 - AI providers, Phase 2.2 - Context management
**Backend Engineers:** Phase 2.3 - Enhanced memory, Phase 6.1 - Connection management
**Frontend/UX Engineers:** Phase 3 - CLI wizard, enhanced commands
**DevOps Engineers:** Phase 6 - Performance, monitoring
**Technical Writers:** Phase 4 - Documentation overhaul

---

## APPENDIX: DEPENDENCIES TO ADD

```bash
# AI providers
go get github.com/anthropics/anthropic-sdk-go
go get github.com/openai/openai-go
go get github.com/ollama/ollama/api

# Context management
go get github.com/pkoukk/tiktoken-go

# Caching
go get github.com/patrickmn/go-cache

# Terminal UI
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/lipgloss

# Vector embeddings
go get github.com/pgvector/pgvector-go

# Rate limiting
go get golang.org/x/time/rate
```

---

**Document Version:** 4.0 - Main Branch Focus (Docker/Podman)
**Last Updated:** 2025-09-29
**Timeline:** 12 weeks
**Target:** Production-ready Docker/Podman MCP orchestration

---

## SUCCESS VISION

When complete, mcp-compose will be:

- **Simple** - 5-minute quickstart, Docker Compose familiarity
- **Secure** - Production OAuth, rate limiting, audit logging
- **Smart** - AI context management, intelligent task scheduling
- **Fast** - <10ms proxy latency, 1000+ connections
- **Documented** - World-class docs, examples, video tutorials
- **Tested** - 80%+ coverage, comprehensive integration tests
- **Production-Ready** - The de-facto standard for MCP server management

**Let's make mcp-compose the standard!**