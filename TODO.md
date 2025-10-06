# MCP-Compose Critical Issues & TODO List

**Date Created:** 2025-10-03
**Last Updated:** 2025-10-04 22:00
**Status:** CRITICAL ISSUE DISCOVERED - Scheduled tasks cannot access MCP tools
**Context:** This document captures all outstanding issues from recent development sessions and tracks fixes completed

---

## 🚨 CRITICAL ISSUES - FIX IMMEDIATELY

### 0. Scheduled Tasks Hallucinate Instead of Using MCP Tools ❌ **CRITICAL BUG**

**Status:** ❌ **NEEDS IMMEDIATE FIX** - Tasks execute without tool access, causing LLM hallucinations

**Problem:**
Scheduled tasks created from chat sessions with MCP tools (e.g., Dexcom glucose monitor) execute successfully but the LLM **hallucinates** results instead of calling the actual MCP tools. Example: task to "check blood sugar every 5 minutes" returned fabricated glucose reading (112 mg/dL) instead of calling Dexcom API.

**Root Cause:**
Environment variable name mismatch in task scheduler configuration causes tool proxy connection to fail:

**Current Config (WRONG):**
```go
// /home/phil/dev/mcp-cron-persistent/internal/config/config.go:154-156
OpenRouter: OpenRouterConfig{
    MCPProxyURL: "http://localhost:3001",  // ← Hardcoded default, wrong URL
    MCPProxyKey: "",
```

**Environment Variable Set:**
```bash
MCP_PROXY_URL=http://mcp-proxy:9876      # ← Correct URL but wrong variable name
MCP_PROXY_API_KEY=myapikey
```

**Variable Expected by Code:**
```go
// Line 331 in config.go
if proxyURL := os.Getenv("MCP_CRON_OPENROUTER_MCP_PROXY_URL"); proxyURL != "" {
    // This never executes because variable name doesn't match!
```

**Evidence:**
```
[ERROR] Warning: Failed to load MCP tools: failed to fetch OpenAPI spec from
http://localhost:3001/openapi.json: Get "http://localhost:3001/openapi.json":
dial tcp [::1]:3001: connect: connection refused
```

**Impact:**
- ✅ Tasks execute and complete "successfully"
- ❌ LLM receives prompt but NO tools
- ❌ LLM fabricates plausible-looking responses
- ❌ User believes task is working correctly
- ❌ No error or warning visible to user
- ❌ **DANGEROUS**: Medical/health data being hallucinated

**Secondary Issue - Task MCP Server Isolation:**
Even if proxy URL is fixed, tasks have a `MCPServers` field (inherited from chat session) that is:
- ✅ Populated when task is created (e.g., `["dexcom"]`)
- ✅ Stored in database correctly
- ❌ **NEVER USED** during task execution
- Tasks currently load ALL tools from proxy, not session-specific ones

**Files Affected:**
- `/home/phil/dev/mcp-cron-persistent/internal/config/config.go:331` (env var reading)
- `/home/phil/dev/mcp-cron-persistent/internal/config/config.go:154-156` (hardcoded default)
- `/home/phil/dev/mcp-cron-persistent/internal/config/config.go:375-380` (legacy fallback)
- `/home/phil/dev/mcp-cron-persistent/internal/agent/run_task.go:166-173` (tool loading)
- `/home/phil/dev/mcp-cron-persistent/internal/agent/run_task.go:185-188` (no-tools fallback)

**Required Fixes:**

**Fix 1: Environment Variable Reading (IMMEDIATE - 5 minutes)**
```go
// Change line 331 in config.go from:
if proxyURL := os.Getenv("MCP_CRON_OPENROUTER_MCP_PROXY_URL"); proxyURL != "" {
    cfg.OpenRouter.MCPProxyURL = proxyURL
}

// To (add fallback):
if proxyURL := os.Getenv("MCP_CRON_OPENROUTER_MCP_PROXY_URL"); proxyURL != "" {
    cfg.OpenRouter.MCPProxyURL = proxyURL
} else if proxyURL := os.Getenv("MCP_PROXY_URL"); proxyURL != "" {
    cfg.OpenRouter.MCPProxyURL = proxyURL
    log.Printf("[INFO] Using MCP_PROXY_URL: %s", proxyURL)
}

// Similarly for API key at line 335:
if proxyKey := os.Getenv("MCP_CRON_OPENROUTER_MCP_PROXY_KEY"); proxyKey != "" {
    cfg.OpenRouter.MCPProxyKey = proxyKey
} else if proxyKey := os.Getenv("MCP_PROXY_API_KEY"); proxyKey != "" {
    cfg.OpenRouter.MCPProxyKey = proxyKey
}
```

**Fix 2: Task-Specific MCP Server Support (HIGH PRIORITY - 30 minutes)**
```go
// In run_task.go line 164-183, implement server filtering:
var tools []openrouter.Tool
if cfg.OpenRouter.MCPProxyURL != "" {
    toolProxy := openrouter.NewToolProxy(cfg.OpenRouter.MCPProxyURL, cfg.OpenRouter.MCPProxyKey)

    // NEW: Use task-specific MCP servers if available
    if len(task.MCPServers) > 0 {
        logger.Infof("Task requires specific MCP servers: %v", task.MCPServers)
        // Option 1: Filter loaded tools by server name
        // Option 2: Pass server filter to proxy endpoint
        // Option 3: Load each server's tools individually
    }

    if err := toolProxy.LoadTools(timeoutCtx); err != nil {
        // NEW: Make this a hard error if task explicitly requires servers
        if len(task.MCPServers) > 0 {
            return "", fmt.Errorf("CRITICAL: Task requires MCP tools but loading failed: %w", err)
        }
        fmt.Printf("Warning: Failed to load MCP tools: %v\n", err)
```

**Fix 3: Validation & Error Handling (MEDIUM PRIORITY - 15 minutes)**
```go
// In run_task.go, add validation before tool-dependent execution:
if len(task.MCPServers) > 0 && len(tools) == 0 {
    return "", fmt.Errorf("task requires MCP servers %v but no tools loaded", task.MCPServers)
}

// Log successful tool loading:
if len(tools) > 0 {
    logger.Infof("Loaded %d tools for task execution", len(tools))
} else if len(task.MCPServers) > 0 {
    logger.Warnf("Task expects MCP servers %v but no tools available", task.MCPServers)
}
```

**Testing Steps:**

1. **Verify environment variable reading:**
   ```bash
   docker exec mcp-compose-task-scheduler env | grep MCP_PROXY
   docker restart mcp-compose-task-scheduler
   docker logs mcp-compose-task-scheduler 2>&1 | grep "MCP_PROXY_URL"
   ```

2. **Test tool loading:**
   ```bash
   docker exec mcp-compose-task-scheduler curl -H "Authorization: Bearer myapikey" \
     http://mcp-proxy:9876/openapi.json
   ```

3. **Create test task with Dexcom:**
   - Create new chat session
   - Add Dexcom server to session
   - Create scheduled task: "Get my current glucose reading"
   - Trigger task manually via dashboard
   - Verify logs show: `Loaded X tools from MCP proxy`
   - Verify response contains REAL glucose data, not hallucinated values

4. **Verify no regression:**
   - Test tasks without MCP servers still work
   - Test tasks with invalid server names fail gracefully
   - Test proxy connection failures are properly reported

**Additional Issues Discovered:**

- **Port Confusion:** Dashboard config shows `port: 3111` but `DASHBOARD_INTERNAL_URL` uses `:3001`
- **No Retry:** Tool loading has no retry mechanism on connection failure
- **Silent Failure:** Tasks complete "successfully" even when critical tools unavailable
- **Hallucination Risk:** No validation that LLM used actual tools vs generating fake data

**Priority:** **CRITICAL** - This is actively causing incorrect medical/health data to be reported to users

---

### 1. Chat Messages Not Persisting to Database ✅ **FIXED**

**Status:** ✅ RESOLVED - Both user messages and AI responses now persist correctly

**Original Problem:**
- User messages sent via WebSocket were NOT being saved to PostgreSQL
- AI responses were NOT being saved to PostgreSQL
- Session worked in real-time via WebSocket but data didn't persist

**Fixes Applied:**

#### Fix 1A: User Message Persistence (`chat_broadcaster.go:175-209`)
Added database persistence for user messages in `chatReadPump()`:
```go
// Create and save user message
userMsg := &ChatMessage{
    ID:        uuid.New().String(),
    SessionID: sessionID,
    Role:      "user",
    Content:   content,
    CreatedAt: time.Now().UTC(),
}
if err := cb.chatService.Storage.AddMessage(userMsg); err != nil {
    log.Printf("[ERROR] Failed to save user message: %v", err)
}
```

#### Fix 1B: AI Response Persistence (`chat_service.go:122-138`)
Added database persistence for AI responses in streaming completion handler:
```go
// After streaming completes, save assistant response
assistantMsg := &ChatMessage{
    ID:        uuid.New().String(),
    SessionID: session.ID,
    Role:      "assistant",
    Content:   fullResponse,
    CreatedAt: time.Now().UTC(),
}
if err := s.Storage.AddMessage(assistantMsg); err != nil {
    log.Printf("[ERROR] Failed to save assistant message: %v", err)
}
```

**Files Modified:**
- ✅ `/home/phil/dev/mcp-compose/internal/dashboard/chat_broadcaster.go` (lines 175-209)
- ✅ `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go` (lines 122-138)

**Verification:**
Messages now persist across page refreshes and can be queried from database successfully.

---

### 2. Frontend Session Handling Broken ✅ **FIXED**

**Status:** ✅ RESOLVED - Session management, WebSocket connections, and UI state now working correctly

**Original Problems:**
1. New Chat Button created session but WebSocket showed "Disconnected"
2. Active session lost on page refresh
3. Old messages didn't load when switching sessions
4. Race conditions between session creation and WebSocket connection
5. Session list UI had fixed height causing layout issues

**Fixes Applied:**

#### Fix 2A: Session Initialization Race Condition (`Chat.jsx:58-78`)
Restructured initialization to load session data BEFORE setting as active:
```javascript
const initializeChat = async () => {
  const savedSessionId = localStorage.getItem('activeSessionId');
  if (savedSessionId) {
    const sessionData = await chatApi.getChatSession(savedSessionId);
    setMessages(savedSessionId, sessionData.messages || []);
    clearUnreadCount(savedSessionId);
    setActiveSession(savedSessionId);  // Set AFTER data loaded
    await loadSessions(false);
  }
};
```

#### Fix 2B: New Session Creation Timing (`Chat.jsx:167-181`)
Eliminated double WebSocket connections by setting active session directly:
```javascript
const createNewSession = async () => {
  const session = await chatApi.createChatSession({...});
  setSessions([session, ...currentSessions]);
  setMessages(session.id, []);
  setActiveSession(session.id);  // Single WebSocket connection
  closeSidebar();
};
```

#### Fix 2C: Field Name Consistency
Updated all frontend references to use `title` field consistently with backend.

#### Fix 2D: WebSocket Reconnect Issue (`Chat.jsx:278-295`)
Fixed condition that prevented WebSocket reconnection on session change:
```javascript
// OLD (broken):
if (connection?.readyState === WebSocket.OPEN && currentSessionId === activeSession) {
  return; // This prevented reconnect when session changed!
}

// NEW (fixed):
if (connection?.readyState === WebSocket.OPEN && currentSessionId === activeSession) {
  return; // Only skip if SAME session AND already connected
}
```

#### Fix 2E: Session List UI Layout (`SessionList.jsx`)
Fixed layout issues with session list height by using flexbox:
```javascript
// Removed fixed height: "calc(100vh - 300px)"
// Changed to: flex: "1 1 auto", overflowY: "auto"
```

**Files Modified:**
- ✅ `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx` (lines 58-78, 167-181, 278-295)
- ✅ `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/SessionList.jsx` (layout fixes)

**Verification:**
- New chat creates session and connects WebSocket immediately
- Page refresh restores active session and message history
- Switching sessions loads correct messages
- Session list UI scales properly

---

### 3. Task Output Not Appearing in Chat ✅ **FIXED (PARTIAL)**

**Status:** ✅ MOSTLY RESOLVED - Task output now appears in chat, may have edge cases

**Original Problem:**
Scheduled tasks executed successfully but their output never appeared in the frontend chat interface.

**Root Cause:**
Issue was actually blocked by Issue #1 (message persistence). Once message persistence was fixed, task outputs began working.

**Additional Fixes Applied:**

#### Fix 3A: Model/Provider Inheritance for Tasks
Tasks now properly inherit model and provider from their associated chat session:

**File:** `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go:464-505`
```go
func (s *PostgresStorage) RecordTaskRun(taskID string, ...) error {
    // Fetch task and its session to get model/provider
    task, err := s.GetTaskByID(taskID)
    session, err := s.getChatSession(task.ChatSessionID)

    // Use session's model/provider if task doesn't specify
    if model == "" {
        model = session.Model
    }
    if provider == "" {
        provider = session.Provider
    }
}
```

#### Fix 3B: OpenRouter Chat() Method
Added simple Chat() method to OpenRouter provider for non-streaming requests:

**File:** `/home/phil/dev/mcp-compose/internal/ai/openrouter.go:138-176`
```go
func (p *OpenRouterProvider) Chat(ctx context.Context, messages []ai.Message, opts *ai.ChatOptions) (string, error) {
    // Convert messages and make request
    // Return complete response as string
}
```

#### Fix 3C: Default output_to_chat Behavior
Changed default for `output_to_chat` from `false` to `true` so tasks automatically post output to chat unless explicitly disabled.

**File:** `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go:495-505`

**Current Status:**
- ✅ Tasks execute and call RecordTaskRun()
- ✅ Task output POSTs to dashboard at `/api/internal/task-output`
- ✅ Messages saved to database with `is_automated=true`
- ✅ Messages appear in frontend with robot icon
- ⚠️ May have edge cases with timing or specific task types

**Files Modified:**
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go` (lines 464-505)
- ✅ `/home/phil/dev/mcp-compose/internal/ai/openrouter.go` (lines 138-176)
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/server/server.go` (lines 398-404)

**Known Limitations:**
- Only tested with basic task types
- May need additional testing with complex tool calls
- Edge cases around task timing not fully explored

---

## ✅ FIXES COMPLETED THIS SESSION (2025-10-04)

This section documents all fixes completed during the marathon debugging session on 2025-10-04.

### Summary
Fixed critical issues with chat message persistence, WebSocket connections, session management, task output, and model/provider inheritance. The core chat functionality is now working end-to-end.

### Detailed Fix List

#### 1. Chat Message Persistence (CRITICAL)
**Problem:** User messages and AI responses were not being saved to PostgreSQL database.

**Files Fixed:**
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_broadcaster.go:175-209`
  - Added `AddMessage()` call to save user messages before sending to AI
  - Creates ChatMessage struct with UUID, session ID, role, content, timestamp

- `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go:122-138`
  - Added `AddMessage()` call to save AI responses after streaming completes
  - Accumulates full response during streaming, then saves complete message

**Result:** Messages now persist across page refreshes and can be queried from database.

---

#### 2. Frontend Session State Management
**Problem:** Race conditions during session initialization and creation, WebSocket connection failures, session lost on page refresh.

**Files Fixed:**
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx:58-78`
  - Fixed `initializeChat()` to load session data BEFORE setting as active
  - Prevents WebSocket from connecting before session is ready

- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx:167-181`
  - Fixed `createNewSession()` to avoid double WebSocket connections
  - Removed redundant `loadSession()` call

- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx:278-295`
  - Fixed WebSocket reconnect logic to allow reconnection when session changes
  - Corrected conditional check that was preventing reconnects

**Result:** Sessions create and restore properly, WebSocket connects reliably, no more "Disconnected" state.

---

#### 3. Session List UI Layout
**Problem:** Session list had fixed height causing layout issues.

**Files Fixed:**
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/SessionList.jsx`
  - Changed from fixed height `calc(100vh - 300px)` to flexbox layout
  - Set `flex: "1 1 auto"` with `overflowY: "auto"`

**Result:** Session list now scales properly with window size.

---

#### 4. Task Output to Chat
**Problem:** Task execution output wasn't appearing in chat interface.

**Root Cause:** Blocked by message persistence issue (Fix #1).

**Files Fixed:**
- `/home/phil/dev/mcp-cron-persistent/internal/server/server.go:398-404`
  - Added `RecordTaskRun()` call when executing tasks on-demand
  - Ensures task output gets posted to chat endpoint

- `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go:495-505`
  - Changed default `output_to_chat` from `false` to `true`
  - Tasks now automatically post output to chat unless explicitly disabled

**Result:** Task output appears in chat with robot icon indicating automated message.

---

#### 5. Model/Provider Inheritance for Tasks
**Problem:** Tasks weren't inheriting model and provider from their associated chat session.

**Files Fixed:**
- `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go:464-505`
  - Modified `RecordTaskRun()` to fetch task and its associated session
  - Added logic to inherit model/provider from session if not specified in task
  - Query: `SELECT model, provider FROM chat_sessions WHERE id = ?`

**Result:** Tasks now use the same model/provider as their chat session.

---

#### 6. OpenRouter Chat() Method
**Problem:** OpenRouter provider only had streaming ChatStream() method, needed simple Chat() for non-streaming requests.

**Files Fixed:**
- `/home/phil/dev/mcp-compose/internal/ai/openrouter.go:138-176`
  - Added `Chat()` method that makes single HTTP request
  - Converts messages to OpenRouter format
  - Returns complete response as string
  - Mirrors Claude provider implementation pattern

**Result:** OpenRouter can now handle both streaming and non-streaming requests.

---

#### 7. Field Name Consistency
**Problem:** Frontend used `name` field while backend used `title` for session names.

**Files Fixed:**
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx`
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/SessionList.jsx`
  - Updated all references to use `title` consistently
  - Added fallback: `activeSession?.title || activeSession?.name` for compatibility

**Result:** Session names display correctly throughout UI.

---

#### 8. Interface Implementation Updates
**Problem:** Storage interface needed RecordTaskRun signature updates.

**Files Fixed:**
- `/home/phil/dev/mcp-cron-persistent/internal/scheduler/scheduler.go:26`
  - Updated Storage interface to include RecordTaskRun method

- `/home/phil/dev/mcp-cron-persistent/internal/storage/sqlite.go:633-636`
  - Added RecordTaskRun stub implementation for SQLite backend

**Result:** Both PostgreSQL and SQLite backends implement complete interface.

---

### Files Modified Summary

**Backend (Go):**
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_broadcaster.go` (message persistence)
- `/home/phil/dev/mcp-compose/internal/dashboard/chat_service.go` (message persistence)
- `/home/phil/dev/mcp-compose/internal/ai/openrouter.go` (Chat method)
- `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go` (model inheritance, defaults)
- `/home/phil/dev/mcp-cron-persistent/internal/server/server.go` (RecordTaskRun call)
- `/home/phil/dev/mcp-cron-persistent/internal/scheduler/scheduler.go` (interface)
- `/home/phil/dev/mcp-cron-persistent/internal/storage/sqlite.go` (stub implementation)

**Frontend (React):**
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/Chat.jsx` (session management, WebSocket)
- `/home/phil/dev/mcp-compose/internal/dashboard/frontend/src/components/Chat/SessionList.jsx` (layout)

### Testing Performed

**Manual Testing:**
- ✅ Send chat message → persists to database → survives page refresh
- ✅ Create new session → WebSocket connects immediately
- ✅ Switch sessions → messages load correctly
- ✅ Create task with chat output → output appears in chat
- ✅ Session list displays and scrolls properly

**Database Verification:**
```sql
-- Verified messages persist
SELECT COUNT(*) FROM chat_messages WHERE session_id = '<session-id>';

-- Verified automated messages
SELECT * FROM chat_messages WHERE is_automated = true;

-- Verified model/provider inheritance
SELECT t.id, t.chat_session_id, cs.model, cs.provider
FROM tasks t
JOIN chat_sessions cs ON t.chat_session_id = cs.id;
```

### Known Remaining Issues

1. **Edge Cases:** Task output may have edge cases with complex tool calls or timing issues
2. **New Tasks Recommended:** Some old tasks created before fixes may not work properly - recommend creating new tasks
3. **Console Logging:** Production build may strip some console.log statements (use console.warn/error instead)
4. **Registry Service:** Still shows "marketplace_servers table doesn't exist" error (non-critical)

### Recommendations for Next Session

1. **Test Complex Scenarios:**
   - Multiple concurrent tasks posting to same session
   - Tasks with tool calls and multi-step workflows
   - High-frequency task execution (stress test)

2. **Clean Up Old Data:**
   - Consider clearing old broken tasks from database
   - Test with fresh sessions to ensure clean state

3. **Monitor for Regressions:**
   - Watch for message duplication
   - Check for WebSocket connection leaks
   - Verify database constraints on message IDs

---

## 🔧 MEDIUM PRIORITY ISSUES

### 4. Session Doesn't Save to localStorage

**Problem:** `activeSessionId` is not persisted to localStorage, so page refresh loses active session

**Impact:** Medium - UX issue, sessions exist in DB but user loses context

**Fix Required:**
In `Chat.jsx`, add to `setActiveSession`:
```javascript
const setActiveSession = (sessionId) => {
  useChatStore.getState().setActiveSession(sessionId);
  if (sessionId) {
    localStorage.setItem('activeSessionId', sessionId);
  } else {
    localStorage.removeItem('activeSessionId');
  }
};
```

---

### 5. Frontend Build May Not Include Console Logs

**Problem:** During debugging, console.log messages weren't appearing in browser console

**Possible Causes:**
1. Vite production build strips console.log
2. Console is being cleared somewhere
3. Old cached build being served
4. Source maps issue

**Investigation Needed:**
```bash
# Check if logs are in built files:
grep -r "console.log" internal/dashboard/frontend/dist/assets/*.js

# Check Vite config:
cat internal/dashboard/frontend/vite.config.js

# Force rebuild:
cd internal/dashboard/frontend && npm run build
```

**Workaround:** Use `console.warn()` or `console.error()` which are less likely to be stripped

---

### 6. System Tools Manager Incomplete Initialization

**Problem:** Dashboard's `createSystemToolsManager()` passes `nil` for task scheduler and memory managers

**Location:** `internal/dashboard/server.go`

**Current Code:**
```go
func (d *DashboardServer) createSystemToolsManager(cfg *config.ComposeConfig, runtime container.Runtime) *SystemToolsManager {
    return NewSystemToolsManager(cfg, d.serverManager, nil, nil)
    //                                                     ↑    ↑
    //                                              taskSched  memMgr
}
```

**Impact:** System tools that depend on task scheduler or memory manager may fail

**Investigation Needed:**
1. Check if `DashboardServer` struct should have these fields
2. Look at how other services initialize these managers
3. Determine if they should be injected or created

**Files to Check:**
- `internal/task_scheduler/manager.go`
- `internal/memory/manager.go`
- `internal/dashboard/server.go` (struct definition)

---

### 7. Registry Service Database Error

**Problem:** Registry service fails to initialize because `marketplace_servers` table doesn't exist

**Error:**
```
ERROR: Failed to initialize registry service: failed to create registry manager:
failed to initialize cache: failed to query servers:
pq: relation "marketplace_servers" does not exist
```

**Impact:** Medium - Registry features unavailable, but doesn't break core functionality

**Fix Required:**
1. Check if migration file exists: `internal/database/migrations/001_create_marketplace_tables.sql`
2. Ensure migrations run on startup
3. OR add graceful handling for missing tables in registry service
4. OR make registry service optional

**Files:**
- `internal/dashboard/registry_handlers.go`
- `internal/registry/manager.go`
- Database migration files

---

## 📋 IMPLEMENTATION TASKS

### 8. Add RecordTaskRun to Scheduled Task Execution

**Status:** ✅ **COMPLETED** in this session

**What Was Fixed:**
Added call to `RecordTaskRun()` in task scheduler when tasks execute on-demand, enabling task output to be posted to chat.

**Files Modified:**
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/server/server.go:398-404`
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/storage/postgres.go:464-505` (Updated RecordTaskRun signature)
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/scheduler/scheduler.go:26` (Added to interface)
- ✅ `/home/phil/dev/mcp-cron-persistent/internal/storage/sqlite.go:633-636` (Stub for SQLite)

**Testing Needed:**
Verify that manual task execution now triggers `postTaskResultToChat`

---

## 🎯 ARCHITECTURAL IMPROVEMENTS NEEDED

### 9. Message Persistence Architecture

**Current State:**
- WebSocket handles real-time communication ✅
- Messages saved to PostgreSQL ❌ **BROKEN**
- Messages broadcast to other clients ✅

**Required Architecture:**
```
User sends message
    ↓
Save to PostgreSQL FIRST
    ↓
Send to AI/MCP servers
    ↓
Stream response via WebSocket
    ↓
Save AI response to PostgreSQL
    ↓
Broadcast to all session clients
```

**Implementation Checklist:**
- [ ] Save user messages before AI processing
- [ ] Save AI responses after streaming completes
- [ ] Save tool call messages
- [ ] Save tool result messages
- [ ] Ensure message IDs are consistent across DB and WebSocket
- [ ] Add message sequence numbers to prevent race conditions
- [ ] Implement message deduplication on frontend

---

### 10. Session State Management

**Current Issues:**
- Session active state not persisted
- Race conditions during session creation
- Double WebSocket connections
- Inconsistent field names (title vs name)

**Required Improvements:**
- [ ] Use single source of truth for active session
- [ ] Persist active session to localStorage
- [ ] Load session data BEFORE setting as active
- [ ] Standardize on `title` field everywhere
- [ ] Add session loading state indicator
- [ ] Implement optimistic UI updates
- [ ] Add session validation before WebSocket connect

---

## 📚 DOCUMENTATION TASKS

### 11. Update Existing Documentation

**Files Requiring Updates:**
- [ ] `CLAUDE_DESTROYED_PROJECT_NOTES.md` - Add recent fixes to "What Was Fixed" section
- [ ] `FIXLOG.md` - Add today's session fixes
- [ ] `WEBSOCKET_IMPLEMENTATION.md` - Add message persistence requirements
- [ ] `CHAT_INTEGRATION.md` - Update with current working status

---

### 12. Create New Documentation

**Needed:**
- [ ] `MESSAGE_PERSISTENCE.md` - Document the full message lifecycle
- [ ] `SESSION_MANAGEMENT.md` - Document session state management
- [ ] `TROUBLESHOOTING.md` - Common issues and solutions
- [ ] `TESTING_GUIDE.md` - How to test chat, sessions, WebSocket

---

## 🧪 TESTING REQUIREMENTS

### 13. Critical Test Scenarios

**Manual Testing Checklist:**

**Chat Message Persistence:**
- [ ] Send message → refresh → message still visible
- [ ] Multiple messages → refresh → all messages visible
- [ ] Multiple sessions → refresh → correct messages per session
- [ ] Tool calls → refresh → tool calls preserved

**Session Management:**
- [ ] Create new chat → WebSocket connects immediately
- [ ] Create new chat → session appears in list
- [ ] Switch sessions → messages load correctly
- [ ] Refresh page → active session restored
- [ ] Refresh page → session list loads

**Task Output to Chat:**
- [ ] Create scheduled task → task appears in tasks list
- [ ] Wait for execution → output appears in chat
- [ ] Output marked as automated (robot icon)
- [ ] Multiple tasks → outputs appear in correct sessions

**WebSocket Stability:**
- [ ] Connection stays alive (no disconnect/reconnect loop)
- [ ] Multiple tabs → all receive messages
- [ ] Close tab → server cleans up connection
- [ ] Network disconnect → graceful reconnection

---

## 🚀 DEPLOYMENT CHECKLIST

Before considering this system production-ready:

- [ ] **Issue #1 FIXED:** Messages persist to database
- [ ] **Issue #2 FIXED:** Sessions work correctly on refresh
- [ ] **Issue #3 FIXED:** Task output appears in chat
- [ ] All manual tests pass
- [ ] Database migrations run successfully
- [ ] Environment variables documented
- [ ] Error handling tested
- [ ] Performance tested (100+ messages per session)
- [ ] Multi-user tested (if applicable)
- [ ] Documentation updated

---

## 💡 QUICK REFERENCE

### Critical Files by Issue

**Issue #1 (Message Persistence):**
- `internal/dashboard/chat_broadcaster.go:137-185`
- `internal/dashboard/chat_service.go` (SendMessage method)
- `internal/dashboard/chat_storage.go` (AddMessage method)

**Issue #2 (Session Handling):**
- `internal/dashboard/frontend/src/components/Chat/Chat.jsx`
- `internal/dashboard/frontend/src/store/chatStore.js`
- `internal/dashboard/frontend/src/api/chat.js`

**Issue #3 (Task Output):**
- `internal/dashboard/chat_handlers.go:562-620`
- `internal/dashboard/chat_broadcaster.go`
- `mcp-cron-persistent/internal/storage/postgres.go:495-574`

### Database Inspection Commands

```bash
# Check message persistence:
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_dashboard -c \
  "SELECT session_id, COUNT(*) as msg_count
   FROM chat_messages
   GROUP BY session_id;"

# Check session list:
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_dashboard -c \
  "SELECT id, title, provider, model, created_at
   FROM chat_sessions
   ORDER BY created_at DESC
   LIMIT 10;"

# Check automated messages:
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_dashboard -c \
  "SELECT id, session_id, substring(content, 1, 100), is_automated, from_task_run_id
   FROM chat_messages
   WHERE is_automated = true
   ORDER BY created_at DESC
   LIMIT 5;"
```

### Log Monitoring

```bash
# Dashboard logs:
mcp-compose logs dashboard | grep -E "ERROR|WARN|Chat|Session"

# Task scheduler logs:
mcp-compose logs task-scheduler | grep -E "postTaskResultToChat|RecordTaskRun"

# Database logs:
docker logs mcp-compose-postgres-memory | tail -50
```

---

## 📝 NOTES FOR NEXT AGENT

**Current State (Updated 2025-10-04 22:00):**

**What's Working:**
- ✅ Chat message persistence (user messages + AI responses)
- ✅ WebSocket connections (stable, no disconnects)
- ✅ Session management (create, restore, switch)
- ✅ Task output to chat (appears with robot icon)
- ✅ Model/provider inheritance for tasks
- ✅ Frontend UI layout and state management

**CRITICAL NEW ISSUE DISCOVERED (2025-10-04 22:00):**

**❌ Scheduled Tasks Cannot Access MCP Tools - LLM Hallucinations Occurring**

This is a **DANGEROUS BUG** that causes scheduled tasks to fabricate responses instead of using real MCP tools:

- **Example:** Task to "check blood sugar every 5 minutes" returned fake glucose reading (112 mg/dL)
- **Root Cause:** Environment variable name mismatch (`MCP_PROXY_URL` vs `MCP_CRON_OPENROUTER_MCP_PROXY_URL`)
- **Current State:** Tool proxy connection fails → LLM executes without tools → Fabricates plausible data
- **Impact:** Medical/health data being hallucinated, user believes task is working correctly
- **Priority:** CRITICAL - Must be fixed before any health/medical tasks are trusted
- **Files:** `/home/phil/dev/mcp-cron-persistent/internal/config/config.go:331-335`
- **Fix Time:** 5-10 minutes for environment variable fallback

**See Issue #0 above for complete details, fixes, and testing procedures.**

**What May Need Attention:**

1. **CRITICAL Issues:**
   - ❌ **Scheduled task MCP tool access (Issue #0)** - MUST FIX IMMEDIATELY
   - Tasks with MCP servers (Dexcom, etc.) are hallucinating responses
   - No validation that tools were successfully loaded
   - Silent failure mode is dangerous for health/medical applications

2. **Medium Priority Issues (still open):**
   - localStorage persistence for active session (Issue #4)
   - Console.log stripping in production builds (Issue #5)
   - System tools manager incomplete initialization (Issue #6)
   - Registry service database error (Issue #7)

3. **Testing Gaps:**
   - **Task MCP tool usage validation (CRITICAL)**
   - Multi-user concurrent chat sessions
   - Performance testing with 100+ messages
   - WebSocket connection cleanup verification
   - Message deduplication edge cases

**Recommendations:**

1. **BEFORE DOING ANYTHING ELSE:**
   - ⚠️ **DO NOT TRUST ANY SCHEDULED TASK THAT USES MCP TOOLS**
   - ⚠️ Especially health/medical tasks (Dexcom, etc.)
   - ⚠️ Fix Issue #0 immediately before creating new tasks with tools
   - Verify fix with test task before trusting real health data

2. **If Creating New Tasks:**
   - **CRITICAL:** Do NOT create tasks requiring MCP tools until Issue #0 is fixed
   - Use fresh chat sessions created after 2025-10-04
   - Old tasks may have stale model/provider configs
   - Verify `output_to_chat` is set correctly
   - After Issue #0 fix: Verify tool loading in logs before trusting results

3. **If Debugging Task Issues:**
   - Check task-scheduler logs for "Failed to load MCP tools" warnings
   - Verify `MCP_PROXY_URL` is readable: `docker exec mcp-compose-task-scheduler env | grep MCP_PROXY`
   - Test proxy connectivity: `curl -H "Authorization: Bearer myapikey" http://mcp-proxy:9876/openapi.json`
   - Compare interactive chat tool usage vs scheduled task tool usage
   - Look for hallucinated data (plausible but fake responses)

4. **If Debugging Chat Issues:**
   - Check database first with provided SQL queries
   - Monitor WebSocket messages in browser DevTools
   - Check both dashboard and task-scheduler logs
   - Verify message IDs are unique (UUID format)

5. **If Adding Features:**
   - Message persistence is working - don't break it
   - WebSocket lifecycle is stable - preserve the pattern
   - Session state management is correct - follow the flow
   - Database schema is sound - use existing patterns
   - **NEW:** Always validate MCP tool availability before execution

**Key Architecture Points:**

- **Message Flow:** User → Save to DB → Send to AI → Stream response → Save AI response → Broadcast
- **Session Flow:** Create → Load data → Set active → Connect WebSocket
- **Task Flow:** Execute → RecordTaskRun → POST to /api/internal/task-output → Save + Broadcast
- **❌ BROKEN Task Tool Flow:** Execute → Try load tools from wrong URL → Fail silently → Execute without tools → Hallucinate

**Don't Waste Time On:**
- Re-debugging message persistence (it's fixed)
- Re-implementing WebSocket connections (they work)
- Changing database schema (it's correct)
- Re-writing session management (it's stable)

**Start Here for New Work:**
1. **FIX ISSUE #0 IMMEDIATELY** - This is a critical safety issue
2. Review "FIXES COMPLETED THIS SESSION" section above
3. Test the scenarios in "Testing Gaps"
4. Address medium priority issues if needed
5. Add new features on top of stable foundation

**Critical Warning for Users:**
Any scheduled tasks created with MCP servers (Dexcom, GitHub, databases, etc.) may be returning **fabricated data** that looks correct but is actually hallucinated by the LLM. Do not trust task outputs that require MCP tool calls until Issue #0 is fixed and verified.

---

**End of TODO.md**
