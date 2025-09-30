# AI Provider Infrastructure

Production-ready AI provider infrastructure for mcp-compose with automatic failover, health monitoring, and circuit breaker patterns.

## Features

- **Multiple Provider Support**: Claude, OpenAI, Ollama, and OpenRouter
- **Automatic Failover**: Falls back to next provider on failure
- **Health Monitoring**: Checks provider health every 30 seconds
- **Circuit Breaker**: Prevents cascading failures with fail-fast logic
- **Streaming Support**: SSE streaming for real-time responses
- **Retry Logic**: Exponential backoff with 3 retry attempts
- **Rate Limiting**: Respects Retry-After headers
- **Thread-Safe**: Safe for concurrent operations

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        AI Manager                           │
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────┐       │
│  │   Health    │  │   Circuit   │  │   Fallback   │       │
│  │  Monitoring │  │   Breaker   │  │    Chain     │       │
│  └─────────────┘  └─────────────┘  └──────────────┘       │
└─────────────────────────────────────────────────────────────┘
                          │
         ┌────────────────┼────────────────┬──────────────┐
         ▼                ▼                ▼              ▼
    ┌─────────┐      ┌─────────┐     ┌─────────┐   ┌──────────┐
    │ Claude  │      │ OpenAI  │     │ Ollama  │   │OpenRouter│
    │Provider │      │Provider │     │Provider │   │ Provider │
    └─────────┘      └─────────┘     └─────────┘   └──────────┘
         │                │                │              │
         ▼                ▼                ▼              ▼
    Anthropic API    OpenAI API      Local HTTP      OpenRouter API
```

## Quick Start

### 1. Basic Usage

```go
import "github.com/phildougherty/mcp-compose/internal/ai"

// Create providers
claudeProvider, _ := ai.NewClaudeProvider(&ai.ClaudeConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
})

openaiProvider, _ := ai.NewOpenAIProvider(&ai.OpenAIConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
})

// Create manager with fallback chain
manager, _ := ai.NewManager(&ai.ManagerConfig{
    Providers: []ai.Provider{claudeProvider, openaiProvider},
    FallbackOrder: []string{"claude", "openai"},
})
defer manager.Stop()

// Use the manager
ctx := context.Background()
messages := []ai.Message{
    {Role: "user", Content: "Hello, AI!"},
}

response, err := manager.Chat(ctx, messages)
if err != nil {
    log.Fatal(err)
}

fmt.Println(response)
```

### 2. Streaming

```go
stream, err := manager.Stream(ctx, messages)
if err != nil {
    log.Fatal(err)
}

for chunk := range stream {
    fmt.Print(chunk)
}
```

### 3. Health Monitoring

```go
status := manager.GetProviderStatus()
for name, info := range status {
    fmt.Printf("%s: healthy=%v, failures=%d\n",
        name, info.Healthy, info.ConsecutiveFailures)
}
```

## Providers

### Claude (Anthropic)

```go
provider, err := ai.NewClaudeProvider(&ai.ClaudeConfig{
    APIKey:         os.Getenv("ANTHROPIC_API_KEY"),
    Model:          "claude-3-5-sonnet-20241022",
    MaxTokens:      4096,
    Temperature:    1.0,
    RequestTimeout: 60 * time.Second,
})
```

**Features:**
- SSE streaming support
- Exponential backoff retry (3 attempts)
- Rate limit handling with Retry-After
- Context timeout support

### OpenAI

```go
provider, err := ai.NewOpenAIProvider(&ai.OpenAIConfig{
    APIKey:         os.Getenv("OPENAI_API_KEY"),
    Model:          "gpt-4",
    MaxTokens:      4096,
    Temperature:    1.0,
    RequestTimeout: 60 * time.Second,
})
```

**Models:**
- `gpt-4` - Most capable model
- `gpt-4-turbo` - Fast GPT-4 variant
- `gpt-3.5-turbo` - Cost-effective option

### Ollama (Local)

```go
provider, err := ai.NewOllamaProvider(&ai.OllamaConfig{
    BaseURL:        "http://localhost:11434",
    Model:          "llama2",
    RequestTimeout: 120 * time.Second,
})
```

**Features:**
- Local inference (no API keys needed)
- Model management via HTTP API
- Health checks via `/api/tags` endpoint
- List available models

### OpenRouter

```go
provider, err := ai.NewOpenRouterProvider(&ai.OpenRouterConfig{
    APIKey:         os.Getenv("OPENROUTER_API_KEY"),
    Model:          "anthropic/claude-3-5-sonnet",
    MaxTokens:      4096,
    Temperature:    1.0,
    RequestTimeout: 60 * time.Second,
    SiteURL:        "https://github.com/phildougherty/mcp-compose",
    AppName:        "mcp-compose",
})
```

**Features:**
- Multi-model access through single API
- Cost tracking per request
- Model fallback on errors
- Unified interface across providers

## Circuit Breaker

The circuit breaker prevents cascading failures:

```go
// Circuit opens after 5 consecutive failures
const circuitBreakerThreshold = 5

// Provider is marked unhealthy and excluded from fallback chain
// until it recovers or is manually reset

// Manual reset
err := manager.ResetCircuitBreaker("claude")
```

**States:**
- **Closed**: Provider is healthy, requests flow normally
- **Open**: Provider has failed 5+ times, no requests sent
- **Half-Open**: After reset, testing provider health

## Error Handling

All providers return `ProviderError` with context:

```go
type ProviderError struct {
    Provider string
    Message  string
    Err      error
}

// Example error
err := &ProviderError{
    Provider: "claude",
    Message:  "rate limit exceeded",
    Err:      originalError,
}
```

**Retryable Errors:**
- Rate limit exceeded
- Timeout occurred
- Connection failed

**Non-Retryable Errors:**
- Invalid API key
- Invalid request format
- Model not found

## Configuration

### Via YAML (mcp-compose.yaml)

```yaml
ai_providers:
  enabled: true
  fallback_order: ["claude", "openai", "ollama", "openrouter"]

  claude:
    enabled: true
    api_key: "${ANTHROPIC_API_KEY}"
    model: "claude-3-5-sonnet-20241022"

  openai:
    enabled: true
    api_key: "${OPENAI_API_KEY}"
    model: "gpt-4"
```

### Via Environment Variables

```bash
# .env file
ANTHROPIC_API_KEY=sk-ant-api03-...
OPENAI_API_KEY=sk-...
OPENROUTER_API_KEY=sk-or-v1-...
```

## Testing

Run tests with coverage:

```bash
go test ./internal/ai/... -cover
```

Current coverage: **54%+**

## Performance

- **Latency**: <50ms additional overhead for failover logic
- **Health Checks**: Every 30 seconds per provider
- **Retry Backoff**: 500ms, 1s, 2s (exponential)
- **Circuit Breaker**: 5 consecutive failures before opening

## Thread Safety

All operations are thread-safe:

- Provider status is protected by `sync.RWMutex`
- Concurrent requests are safe
- Health checks run in separate goroutines

## Best Practices

1. **Always use the Manager**: Don't use providers directly in production
2. **Configure fallbacks**: Have at least 2 providers in fallback chain
3. **Monitor health**: Check provider status regularly
4. **Handle errors**: Always check for errors and log appropriately
5. **Use contexts**: Set appropriate timeouts for your use case
6. **Test failover**: Simulate provider failures in staging

## Example: Production Setup

```go
package main

import (
    "context"
    "log"
    "os"
    "time"

    "github.com/phildougherty/mcp-compose/internal/ai"
)

func main() {
    // Initialize providers
    providers := []ai.Provider{}

    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        p, _ := ai.NewClaudeProvider(&ai.ClaudeConfig{APIKey: key})
        providers = append(providers, p)
    }

    if key := os.Getenv("OPENAI_API_KEY"); key != "" {
        p, _ := ai.NewOpenAIProvider(&ai.OpenAIConfig{APIKey: key})
        providers = append(providers, p)
    }

    // Local fallback
    ollama, _ := ai.NewOllamaProvider(&ai.OllamaConfig{
        BaseURL: "http://localhost:11434",
    })
    providers = append(providers, ollama)

    // Create manager
    manager, err := ai.NewManager(&ai.ManagerConfig{
        Providers:     providers,
        FallbackOrder: []string{"claude", "openai", "ollama"},
    })
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Stop()

    // Monitor health in background
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()

        for range ticker.C {
            status := manager.GetProviderStatus()
            for name, info := range status {
                if !info.Healthy {
                    log.Printf("WARNING: Provider %s is unhealthy", name)
                }
            }
        }
    }()

    // Use the manager
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    response, err := manager.Chat(ctx, []ai.Message{
        {Role: "user", Content: "Hello!"},
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Println(response)
}
```

## License

This is part of mcp-compose, licensed under AGPL v3.