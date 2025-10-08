# Chat Provider and Model Selection Fixes

## Problem Summary

Two critical bugs were identified when creating chat sessions from tasks:

1. **Chat session card shows wrong provider and model**
   - Expected: Shows "ollama" as provider and "gpt-oss:latest" as model (from task configuration)
   - Actual: Shows "openrouter" and "anthropic/claude-sonnet-4.5"

2. **Chat messages fail with Ollama 404 error for "llama2" model**
   - Error: ollama 404 error about llama2 model
   - llama2 is NOT installed and was an old hardcoded fallback
   - The chat should be using "gpt-oss:latest" model with ollama provider

## Root Causes

### Issue 1: Hardcoded Default Values in Chat Session Creation

**Location**: `/home/phil/dev/mcp-compose/internal/dashboard/chat_handlers.go` lines 99-105

**Problem**: When creating a chat session, if provider/model were empty, they would default to hardcoded values:
```go
if req.Provider == "" {
    req.Provider = "openrouter"  // Hardcoded default
}

if req.Model == "" {
    req.Model = "z-ai/glm-4.6"   // Hardcoded default
}
```

**Impact**: Even when TaskScheduler.jsx correctly passed task.providerHint and task.modelHint, these would be ignored if they were empty strings, replacing them with defaults.

### Issue 2: Chat Service Not Using Session Provider/Model

**Location**: `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go` lines 1628 and 1817

**Problem**: The SendMessage functions called:
```go
provider, err := cs.aiManager.GetHealthyProvider()
```

This method returns ANY healthy provider, completely ignoring the session's configured provider and model.

**Impact**: Chat sessions would use whatever provider was available, not the one specified in the session configuration.

### Issue 3: Hardcoded "llama2" Model

**Location**: `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go` line 1169

**Problem**: Ollama provider had hardcoded model list including "llama2":
```go
{
    Name:    "ollama",
    Enabled: false,
    Healthy: false,
    Models: []string{
        "llama2",  // Hardcoded, not installed
        "mistral",
        "codellama",
        // ...
    },
}
```

**Impact**: If ollama was used, it might fall back to non-existent "llama2" model.

## Solutions Implemented

### 1. Added Provider/Model Helper Functions to ChatService

Added the same helper functions that already existed in workflow executor:

```go
func (cs *ChatService) normalizeProviderName(provider string) string {
    switch provider {
    case "local":
        return "ollama"
    case "anthropic":
        return "claude"
    default:
        return provider
    }
}

func (cs *ChatService) getProviderWithModel(providerName, model string) (ai.Provider, error) {
    if cs.aiManager == nil {
        return nil, fmt.Errorf("AI manager not available")
    }

    if model == "" {
        return cs.aiManager.GetProvider(providerName)
    }

    switch providerName {
    case "ollama":
        return ai.NewOllamaProvider(&ai.OllamaConfig{
            BaseURL: cs.getOllamaBaseURL(),
            Model:   model,
        })
    case "openrouter":
        return ai.NewOpenRouterProvider(&ai.OpenRouterConfig{
            APIKey: cs.getOpenRouterAPIKey(),
            Model:  model,
        })
    // ... other providers
    }
}
```

### 2. Updated streamResponseWithTools to Use Session Provider/Model

**Before**:
```go
provider, err := cs.aiManager.GetHealthyProvider()
```

**After**:
```go
providerName := cs.normalizeProviderName(session.Provider)
cs.logger.Info("Getting provider for session %s: original=%s, normalized=%s, model=%s",
    session.ID, session.Provider, providerName, session.Model)

provider, err := cs.getProviderWithModel(providerName, session.Model)
```

### 3. Updated chatResponseWithTools (Non-Streaming)

Applied the same fix to the non-streaming chat response function.

### 4. Removed Hardcoded "llama2" Model

**Before**:
```go
{
    Name:    "ollama",
    Enabled: false,
    Healthy: false,
    Models: []string{
        "llama2",
        "mistral",
        "codellama",
        "llama3",
        "gemma",
        "qwen",
    },
}
```

**After**:
```go
{
    Name:    "ollama",
    Enabled: false,
    Healthy: false,
    Models:  []string{},  // Will be populated dynamically from API
}
```

### 5. Improved Logging in Chat Session Creation

Added detailed logging to track provider/model values:
```go
s.logger.Info("Creating chat session: user_id=%s, provider=%s, model=%s, title=%s",
    req.UserID, req.Provider, req.Model, req.Title)
```

### 6. Better Default Model

Changed the default model from "z-ai/glm-4.6" to "anthropic/claude-sonnet-4.5" for better compatibility:
```go
if req.Model == "" {
    req.Model = "anthropic/claude-sonnet-4.5"
}
```

## Files Modified

1. `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go`
   - Added normalizeProviderName() function
   - Added getProviderWithModel() function
   - Added helper functions: getOllamaBaseURL(), getOpenRouterAPIKey(), getOpenAIAPIKey(), getClaudeAPIKey()
   - Updated streamResponseWithTools() to use session provider/model
   - Updated chatResponseWithTools() to use session provider/model
   - Removed hardcoded "llama2" model from ollama provider defaults

2. `/home/phil/dev/mcp-compose/internal/dashboard/chat_handlers.go`
   - Improved default model selection
   - Added detailed logging for session creation
   - Reordered default assignment logic for clarity

## Testing Recommendations

1. **Test chat from task with ollama/gpt-oss:latest**:
   - Create a task with providerHint="ollama" and modelHint="gpt-oss:latest"
   - Click "Chat with Task"
   - Verify chat session card shows "ollama" and "gpt-oss:latest"
   - Send a message and verify it uses ollama with gpt-oss:latest (not llama2)

2. **Test provider name translation**:
   - Create a task with providerHint="local" (should normalize to "ollama")
   - Verify chat uses ollama provider
   - Create a task with providerHint="anthropic" (should normalize to "claude")
   - Verify chat uses claude provider

3. **Test default values**:
   - Create a chat without specifying provider/model
   - Verify it uses "openrouter" and "anthropic/claude-sonnet-4.5"

4. **Test multiple providers**:
   - Create chats with different provider/model combinations
   - Verify each chat uses its configured provider/model
   - Verify sessions maintain their configuration across restarts

## Expected Outcomes

After these fixes:

1. Chat session created from a task with ollama/gpt-oss:latest will:
   - Display "ollama" as provider on the session card
   - Display "gpt-oss:latest" as model on the session card
   - Use ollama provider with gpt-oss:latest model for all AI calls
   - Never attempt to use "llama2" model

2. Provider name translation works consistently:
   - "local" → "ollama"
   - "anthropic" → "claude"
   - Other names pass through unchanged

3. Chat sessions respect their configured provider/model:
   - No more "any healthy provider" behavior
   - Each session uses its specific provider instance with specific model
   - Errors clearly indicate which provider/model failed

## Consistency with Workflow System

These fixes bring the chat system in line with the workflow executor, which already had:
- Provider name normalization ("local" → "ollama", "anthropic" → "claude")
- Model-specific provider instances via getProviderWithModel()
- Proper use of task-specified provider/model configurations

The chat system now follows the same pattern, ensuring consistent behavior across the application.
