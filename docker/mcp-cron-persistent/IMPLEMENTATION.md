# Task Execution Engine with Chat Integration

## Overview

This implementation provides a complete task execution engine for the mcp-cron-persistent service that integrates scheduled tasks with chat sessions. Tasks can inherit conversation context, execute with access to MCP tools, and post results back to the originating chat session.

## Architecture

### Core Components

#### 1. Model Layer (`internal/model/task.go`)
Defines the data structures for tasks and execution results:

- **Task**: Represents a scheduled task with full configuration including:
  - Chat integration fields (`ChatSessionID`, `InheritSessionContext`, `OutputToChat`)
  - AI configuration (provider, model, MCP servers)
  - Scheduling and execution metadata
  - Agent mode support with persistent memory

- **Result**: Tracks task execution outcomes with:
  - Execution timing and duration
  - Output, errors, and exit codes
  - Chat posting status
  - Token usage and cost estimates

- **ChatMessage**: Lightweight structure for chat context retrieval

#### 2. Agent Execution Engine (`internal/agent/run_task.go`)

The main execution engine with two execution paths:

##### Standard Execution Flow
```
Task → Build System Prompt → Fetch MCP Tools → Execute with AI → Save Result
```

##### Chat-Integrated Execution Flow
```
Task → Fetch Chat Context (last 10 messages)
     → Build Enhanced System Prompt with Conversation History
     → Fetch MCP Tools from Session Configuration
     → Execute with AI Provider
     → Save Result to Task Storage
     → Post Result to Chat Session (if enabled)
     → Update Chat Unread Count
```

## Key Features

### 1. Chat Context Integration

**Context Retrieval** (`fetchChatSessionContext`):
- Fetches last 10 chat messages from the dashboard service
- HTTP GET to `/api/internal/chat/sessions/{sessionID}/context?limit=10`
- Includes internal request headers for authentication
- Robust error handling with fallback to execution without context

**System Prompt Enhancement** (`buildSystemPromptWithContext`):
- Includes task metadata (name, description, schedule)
- Embeds recent conversation history (truncated to 200 chars per message)
- Adds agent mode instructions for persistent memory
- Explains tool availability and output destination

### 2. MCP Tool Access

**Dynamic Tool Discovery** (`getMCPToolsForTask`):
- Retrieves tools from each MCP server configured in the task
- Uses the MCP proxy service for tool listing
- Prefixes tool names with `mcp_{server}_{tool}` format
- Handles authentication with API keys

**Tool Fetching** (`fetchMCPServerTools`):
- JSON-RPC 2.0 protocol for tool discovery
- POST to `{proxyURL}/{serverName}` with `tools/list` method
- Converts MCP tool schemas to agent-compatible format
- Includes error handling and fallback behavior

### 3. Result Publishing

**Chat Webhook Integration** (`postResultToChat`):
- Posts task execution results back to the chat session
- HTTP POST to `/api/internal/task-output` endpoint
- Payload includes:
  - Session ID and role (always "assistant")
  - Task output content
  - Automation flags (`is_automated: true`)
  - Task run ID for traceability
  - Token usage and cost estimates

**Result Metadata**:
- Tracks whether result was successfully posted to chat
- Records chat message ID if available
- Updates task run record with posting status

### 4. Agent Mode Support

**Persistent Memory**:
- System prompts include memory instructions when `IsAgent: true`
- Tasks can reference previous executions
- Memory tools available through MCP server integration

**Context Continuity**:
- Chat context provides conversation history
- Agent can maintain state across executions
- Supports long-running autonomous workflows

## HTTP Integration Points

### Outbound Requests

1. **Chat Context Retrieval**
   - Endpoint: `GET /api/internal/chat/sessions/{sessionID}/context?limit={limit}`
   - Headers: `Content-Type: application/json`, `X-Internal-Request: true`
   - Response: Array of ChatMessage objects

2. **MCP Tool Discovery**
   - Endpoint: `POST {proxyURL}/{serverName}`
   - Method: JSON-RPC `tools/list`
   - Headers: `Authorization: Bearer {apiKey}`, `Content-Type: application/json`
   - Response: JSON-RPC result with tools array

3. **Result Publishing**
   - Endpoint: `POST /api/internal/task-output`
   - Headers: `Content-Type: application/json`, `X-Internal-Request: true`
   - Payload: Result data with automation flags
   - Response: Success status with optional message ID

### Environment Variables

- `DASHBOARD_INTERNAL_URL`: Dashboard service URL (default: `http://mcp-compose-dashboard:3001`)
- `MCP_PROXY_URL`: MCP proxy service URL (default: `http://mcp-compose-proxy:9876`)
- `MCP_PROXY_API_KEY`: Authentication key for MCP proxy

## Error Handling

### Graceful Degradation
- Missing chat context: Logs warning, executes without context
- Failed tool fetching: Logs warning, continues with available tools
- Chat posting failure: Logs error, marks result but doesn't fail task

### HTTP Error Handling
- Validates status codes before processing responses
- Reads error bodies for detailed diagnostics
- Returns formatted error messages with context
- Implements timeouts (30s for HTTP requests)

### Context Management
- All operations accept `context.Context` for cancellation
- Supports timeout propagation from parent contexts
- Cleans up HTTP resources with `defer resp.Body.Close()`

## Integration Requirements

### Dashboard Service Requirements

The dashboard service must implement these internal endpoints:

1. **GET /api/internal/chat/sessions/{sessionID}/context**
   - Query params: `limit` (number of messages)
   - Returns: JSON array of recent messages
   - Fields: `id`, `session_id`, `role`, `content`, `created_at`

2. **POST /api/internal/task-output**
   - Accepts: Task result payload
   - Creates: New chat message in session
   - Increments: Unread message count
   - Returns: Created message ID

### Task Scheduler Requirements

The task scheduler service must provide:

1. **TaskStorage Interface**:
   - `RecordTaskRun(ctx, result)`: Persist execution results
   - `GetTask(ctx, taskID)`: Retrieve task configuration

2. **AIProvider Interface**:
   - `ChatWithTools(ctx, messages, tools)`: Execute with tool support
   - Returns: ChatResponse with text, tokens, and cost

3. **Logger Interface**:
   - `Info`, `Warning`, `Error`, `Debug` methods
   - Structured logging support

## Security Considerations

### Internal Service Communication
- Uses `X-Internal-Request: true` header for internal APIs
- Restricts access to internal endpoints via header validation
- Network-level isolation via Docker networking (`mcp-net`)

### Authentication
- MCP proxy requires API key authentication
- Dashboard internal endpoints use header-based auth
- No user credentials stored in task configuration

### Data Handling
- Truncates chat context to prevent prompt injection
- Validates JSON-RPC responses before processing
- Sanitizes error messages before logging

## Performance Characteristics

### HTTP Timeouts
- Default: 30 seconds per HTTP request
- Configurable via `httpClient.Timeout`
- Context-based cancellation support

### Memory Efficiency
- Chat context limited to last 10 messages
- Message content truncated to 200 characters in prompts
- Streaming not required for background task execution

### Concurrency
- Safe for concurrent task execution
- No shared mutable state in Agent struct
- HTTP client reusable across requests

## Testing Recommendations

### Unit Tests
1. Test `buildSystemPromptWithContext` with various chat histories
2. Test `truncate` function with edge cases
3. Test error handling for HTTP failures
4. Test tool name formatting and validation

### Integration Tests
1. Mock dashboard service for context retrieval
2. Mock MCP proxy for tool discovery
3. Test complete execution flow with chat integration
4. Test fallback behavior when services unavailable

### End-to-End Tests
1. Create task in chat session
2. Verify task executes with chat context
3. Verify result posted back to chat
4. Verify token usage and cost tracking

## Future Enhancements

### Potential Improvements
1. **Retry Logic**: Add exponential backoff for transient failures
2. **Circuit Breaker**: Prevent cascade failures to dashboard
3. **Result Streaming**: Stream large outputs back to chat
4. **Tool Result Caching**: Cache MCP tool definitions
5. **Metrics**: Add Prometheus metrics for execution tracking
6. **Webhook Verification**: Add HMAC signatures for webhooks

### Monitoring
- Track execution duration by task type
- Monitor chat posting success rate
- Alert on repeated execution failures
- Track token usage and costs per task

## Migration Notes

### From Standalone Tasks
Existing tasks without chat integration will:
- Execute normally with `executeStandard` flow
- Skip chat context retrieval
- Skip result posting to chat
- Maintain backward compatibility

### Database Schema
Requires PostgreSQL task_scheduler schema with:
- `scheduler_tasks` table with chat integration fields
- `scheduler_task_runs` table with metrics fields
- Foreign key constraints to `chat_sessions`

## Summary

This implementation provides a production-ready task execution engine with robust chat integration. It follows the specification from TASK_CHAT.md (lines 1084-1241) and includes:

1. ✅ Main execution flow with chat context checking
2. ✅ Context retrieval mechanism via HTTP
3. ✅ Enhanced system prompt building
4. ✅ MCP tool discovery and integration
5. ✅ Webhook integration for result posting
6. ✅ Robust error handling throughout
7. ✅ Token usage and cost tracking
8. ✅ Agent mode support with memory

The code is ready for integration into the mcp-cron-persistent service and requires corresponding dashboard endpoints for full functionality.
