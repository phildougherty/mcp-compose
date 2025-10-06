# Autonomous Agent System Prompt

## Overview

The chat interface (both TUI and dashboard UI) now uses an autonomous agent system prompt that instructs the AI to operate in a fully autonomous mode, executing tool calls in a loop until tasks are complete.

## Changes Made

### Updated System Prompt Location
**File**: `internal/dashboard/chat_service.go`
**Function**: `BuildSystemContextForSession(sessionID string)` (lines 766-948)

### New Autonomous Instructions

The system prompt now includes a "CRITICAL: Autonomous Operation Mode" section that instructs the AI to:

1. **Execute ALL necessary tool calls** to complete tasks WITHOUT asking for permission
2. **Chain multiple tool calls together** - keep calling tools until the task is FULLY complete
3. **Handle errors intelligently** - analyze failures and retry with corrected parameters or alternative approaches
4. **Provide progress updates** as it works through multi-step tasks
5. **Only stop when the task is completely finished** or truly impossible
6. **Never ask "Would you like me to...?"** - just do it
7. **Never wait for confirmation between steps** - execute the full workflow

### Examples in System Prompt

**Correct autonomous behavior:**
```
User: "Check my glucose and store it in memory"
You: [Call mcp_dexcom_get_glucose] → [Get result] → [Call memory_store_entity with result] → "Done! Current glucose is 120 mg/dL, stored in memory."
```

**Incorrect behavior:**
```
User: "Check my glucose and store it in memory"
You: "I can check your glucose. Would you like me to proceed?" ❌ NO! Just do it!
```

## Implementation Details

### Agentic Loop Already Exists
The code already implements an agentic loop in `streamResponseWithTools` (lines 1512-1625):
- Max 10 iterations
- Continuously calls tools and makes follow-up AI calls
- Stops when no more tool calls are made or max iterations reached

### What Was Missing
The original system prompt didn't explicitly tell the model to BE autonomous. It had the tools and the loop infrastructure, but didn't instruct the model to keep working until done.

### Consistency Across Interfaces
Both the **TUI chat** (`./mcp-compose chat`) and **Dashboard UI chat** use the same:
- ChatService
- SendMessage method
- BuildSystemContextForSession function

This ensures **identical behavior and system prompts** across both interfaces.

## Usage

### Testing the Autonomous Behavior

Start a chat session and try multi-step tasks:

```bash
# TUI
./mcp-compose chat

# Then try: "Check my glucose and store it in memory with a note about the time"
```

Or use the dashboard UI at `http://localhost:8080` and test similar multi-step workflows.

### Expected Behavior

The AI should:
- Immediately call necessary tools without asking permission
- Chain multiple tool calls together
- Provide progress updates
- Only respond when the task is complete

### Not Working?

If the AI still asks for permission or doesn't chain tool calls:
1. Check that you're using a model that supports tool calling (Claude 3.5 Sonnet, GPT-4, etc.)
2. Verify the system prompt is being loaded (check the "View System Prompt" button in the UI)
3. Make sure MCP servers are enabled for the chat session

## Future Enhancements

Potential improvements:
1. Allow per-session custom system prompt overrides (requires database schema update)
2. Add UI to edit autonomous behavior settings (strictness level, max iterations, etc.)
3. Add safety rails for destructive operations
4. Implement approval required for certain tool categories
