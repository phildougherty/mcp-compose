# Context Management Package

Production-ready context window management for MCP-Compose with intelligent token counting, truncation strategies, and persistence.

## Features

- **Token Counting**: Accurate token counting using tiktoken-go (cl100k_base encoding)
- **Multiple Truncation Strategies**: Oldest, LRU, ByType, ByPriority, and Intelligent
- **Thread-Safe**: All operations use sync.RWMutex for concurrent access
- **Persistence**: SQLite-based storage with automatic vacuum and TTL
- **Configurable**: Easily configure via mcp-compose.yaml

## Quick Start

### Configuration

Add to your `mcp-compose.yaml`:

```yaml
context:
  enabled: true
  max_tokens: 32000
  model: "gpt-4"
  truncation_strategy: "intelligent"
  persistence_enabled: true
  database_path: "./context.db"
  context_ttl: "24h"
  vacuum_interval: "1h"
```

### Usage Example

```go
package main

import (
    "fmt"
    "time"

    "github.com/phildougherty/mcp-compose/internal/context"
)

func main() {
    // Create manager with config
    cfg := context.Config{
        MaxTokens:          32000,
        Model:              "gpt-4",
        TruncationStrategy: context.TruncationIntelligent,
        PersistenceEnabled: true,
        DatabasePath:       "./context.db",
        ContextTTL:         24 * time.Hour,
        VacuumInterval:     1 * time.Hour,
    }

    manager, err := context.NewManager(cfg)
    if err != nil {
        panic(err)
    }
    defer manager.Close()

    // Add messages to a conversation
    conversationID := "user-123"

    systemMsg := context.Message{
        Type:     context.MessageTypeSystem,
        Content:  "You are a helpful AI assistant.",
        Priority: 10,
    }

    userMsg := context.Message{
        Type:     context.MessageTypeUser,
        Content:  "What is the capital of France?",
        Priority: 5,
    }

    assistantMsg := context.Message{
        Type:     context.MessageTypeAssistant,
        Content:  "The capital of France is Paris.",
        Priority: 5,
    }

    manager.AddMessage(conversationID, systemMsg)
    manager.AddMessage(conversationID, userMsg)
    manager.AddMessage(conversationID, assistantMsg)

    // Retrieve messages
    messages, err := manager.GetMessages(conversationID)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Conversation has %d messages\n", len(messages))

    // Check token usage
    totalTokens := manager.GetTotalTokens(conversationID)
    fmt.Printf("Total tokens: %d / %d\n", totalTokens, cfg.MaxTokens)
}
```

## Truncation Strategies

### Oldest
Removes the oldest messages first while preserving system messages.

**Use case**: Simple FIFO approach when you want to keep the most recent conversation.

```go
TruncationStrategy: context.TruncationOldest
```

### LRU (Least Recently Used)
Removes messages based on last access time, keeping frequently accessed messages.

**Use case**: When you want to preserve messages that are referenced often.

```go
TruncationStrategy: context.TruncationLRU
```

### ByType
Prioritizes messages by type: System > User > Assistant > Tool.

**Use case**: When you want to preserve important message types.

```go
TruncationStrategy: context.TruncationByType
```

### ByPriority
Uses the message priority field to determine which messages to keep.

**Use case**: When you have explicit importance rankings for messages.

```go
TruncationStrategy: context.TruncationByPriority
```

### Intelligent
Uses a scoring algorithm considering type, priority, recency, and access patterns.

**Use case**: Default recommendation for most scenarios.

```go
TruncationStrategy: context.TruncationIntelligent
```

## Message Types

```go
const (
    MessageTypeSystem     MessageType = "system"
    MessageTypeUser       MessageType = "user"
    MessageTypeAssistant  MessageType = "assistant"
    MessageTypeToolUse    MessageType = "tool_use"
    MessageTypeToolResult MessageType = "tool_result"
)
```

## Persistence

The package automatically persists conversations to SQLite when `PersistenceEnabled: true`.

### Features

- **Auto-vacuum**: Runs periodically to clean up old data
- **TTL**: Automatically expires old conversations
- **Export/Import**: JSON export/import for backups
- **Pagination**: Retrieve large conversations in chunks
- **Statistics**: Get conversation stats (message count, token count, age)

### Example: Export/Import

```go
// Export conversation
data, err := manager.persistence.ExportConversation(conversationID)
if err != nil {
    panic(err)
}

// Save to file
os.WriteFile("conversation-backup.json", data, 0644)

// Import later
data, _ := os.ReadFile("conversation-backup.json")
err = manager.persistence.ImportConversation(conversationID, data)
```

## Performance

- **Token Counting**: Cached per message (computed once on add)
- **Thread-Safe**: Uses RWMutex for read-heavy workloads
- **Database**: Indexed queries for fast retrieval
- **Memory**: Efficient memory usage with pagination support

## Testing

The package includes comprehensive tests with 84.5%+ coverage:

```bash
go test ./internal/context/... -v -cover
```

## Architecture

```
internal/context/
├── manager.go           # Main context manager with token counting
├── truncation.go        # Truncation strategy implementations
├── persistence.go       # SQLite persistence layer
├── manager_test.go      # Manager tests
├── truncation_test.go   # Truncation strategy tests
├── persistence_test.go  # Persistence tests
└── README.md           # This file
```

## Error Handling

All methods return errors following Go conventions:

```go
if err := manager.AddMessage(convID, msg); err != nil {
    log.Printf("Failed to add message: %v", err)
    return err
}
```

## Thread Safety

The manager is fully thread-safe and can be used from multiple goroutines:

```go
// Safe to call from multiple goroutines
go manager.AddMessage(convID, msg1)
go manager.AddMessage(convID, msg2)
go manager.GetMessages(convID)
```

## Integration with AI Providers

Before calling an AI provider API:

```go
// Get messages for the conversation
messages, _ := manager.GetMessages(conversationID)

// Check token count
totalTokens := manager.GetTotalTokens(conversationID)

// Truncation is automatic, but you can verify
if totalTokens > providerLimit {
    // This should not happen - manager auto-truncates
    log.Printf("Warning: token count exceeds limit")
}

// Convert to provider format and send
apiMessages := convertToProviderFormat(messages)
response, _ := provider.Chat(apiMessages)
```

## Configuration Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | false | Enable context management |
| `max_tokens` | int | 32000 | Maximum tokens per conversation |
| `model` | string | "gpt-4" | Model for token counting |
| `truncation_strategy` | string | "intelligent" | Strategy: oldest, lru, by_type, by_priority, intelligent |
| `persistence_enabled` | bool | false | Enable SQLite persistence |
| `database_path` | string | "./mcp-compose-context.db" | Path to SQLite database |
| `context_ttl` | string | "24h" | How long to keep conversations |
| `vacuum_interval` | string | "1h" | How often to vacuum database |

## Limitations

- **SQLite Write Locks**: High-concurrency writes may encounter SQLITE_BUSY errors
- **Token Counting**: Uses cl100k_base encoding (GPT-4), may differ for other models
- **Memory**: All messages for a conversation are loaded into memory
- **Single Process**: SQLite database is designed for single-process use

## Future Enhancements

- [ ] PostgreSQL backend for high-concurrency scenarios
- [ ] Streaming token counting for very large messages
- [ ] Custom encoding support for different models
- [ ] Compression for old messages
- [ ] Semantic search integration
- [ ] Conversation summaries for older messages

## License

AGPL v3 - See LICENSE file in repository root.