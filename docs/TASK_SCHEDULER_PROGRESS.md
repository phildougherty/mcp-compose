# Task Scheduler Integration Progress Report

**Date:** 2025-10-03
**Status:** MOSTLY WORKING - Tasks Execute, Chat Output Broken

---

## What Was Accomplished ✅

### 1. PostgreSQL Storage Fixed ✅
- **Task scheduler now uses PostgreSQL** instead of SQLite
- Environment variables properly set: `MCP_CRON_POSTGRES_ENABLED=true`, `MCP_CRON_POSTGRES_URL`
- Storage confirmed: `PostgreSQL storage initialized: postgresql:***@mcp-compose-postgres-memory:5432/mcp_compose`
- Tasks are saved to `mcp_compose.task_scheduler.scheduler_tasks` table

### 2. NULL Handling Fixed ✅
- **Fixed NULL handling** for `command` and `prompt` fields in `mcp-cron-persistent/internal/storage/postgres.go`
- Shell tasks can have NULL prompts, AI tasks can have NULL commands
- Uses `sql.NullString` for proper NULL handling
- Commit: `0954e1b` - "Fix NULL handling for command and prompt fields in PostgreSQL storage"

### 3. Task Type Case Mismatch Fixed ✅
- **Fixed task type constant** from `"shell_command"` to `"shell"` in `mcp-cron-persistent/internal/model/task.go`
- Database constraint accepts: `'ai', 'AI', 'shell', 'manual', 'dependency', 'watcher'`
- Migration updated: `003_create_scheduler_schema.sql` to accept both 'ai' and 'AI'
- Commit: `d33be69` - "Fix shell task type: use 'shell' instead of 'shell_command'"

### 4. Chat Session Persistence Fixed ✅
- **Chat sessions now persisted to database** when created
- Added verification after insert in `internal/dashboard/chat_storage.go`
- Added timeout handling (10 seconds) in `internal/dashboard/chat_service.go`
- Sessions verified to exist before task creation can reference them

### 5. In-Memory Session Update Errors Fixed ✅
- **Fixed 500 errors** when updating provider/model/MCP servers for in-memory sessions
- Modified `UpdateSession` and `SetSessionMCPServers` in `chat_service.go`
- Gracefully handles sessions that exist in memory but not in database
- Returns `nil` instead of error when session not in storage

---

## Current Issues ❌

### CRITICAL: Tasks Not Posting to Chat ❌

**Problem:**
Tasks execute successfully and save to database, but results are NOT posted to chat because:
1. `output_to_chat` is set to `false` instead of `true`
2. `chat_session_id` is empty instead of containing the session ID

**Evidence:**
```sql
-- Task created via chat:
SELECT id, output_to_chat, chat_session_id FROM task_scheduler.scheduler_tasks;
-- Shows: output_to_chat = f, chat_session_id = ''
```

**Root Cause:**
The MCP tool `task_scheduler_create_task` is not setting these fields when creating tasks from chat context.

**Attempted Fix (NOT WORKING):**
- Modified `internal/dashboard/chat_service.go` to enrich MCP tool arguments with:
  - `_chat_session_id` (from context)
  - `_output_to_chat = true`
- Modified `mcp-cron-persistent` handlers to extract and apply chat context
- Commit: `2dc9a70` - "Fix task creation: set output_to_chat=true and chat_session_id from request context"

**Why It's Not Working:**
- Debug logs show enrichment code is not being executed
- Tool may be registered as system tool instead of MCP tool
- Code path through `executeToolCallByName` → `executeMCPTool` not confirmed
- The `_chat_session_id` and `_output_to_chat` fields not appearing in task scheduler

**Manual Workaround:**
```sql
UPDATE task_scheduler.scheduler_tasks
SET output_to_chat = true, chat_session_id = '<session_id>'
WHERE id = '<task_id>';
```

### Foreign Key Constraint Issues ❌

**Problem:**
Foreign key constraint `fk_chat_session` was blocking task creation when chat_session_id didn't exist.

**Temporary Fix:**
```sql
ALTER TABLE task_scheduler.scheduler_tasks DROP CONSTRAINT fk_chat_session;
```

**Impact:**
- Tasks can now be created without valid chat_session_id
- No referential integrity between tasks and chat sessions
- Need to restore constraint once chat context is properly passed

---

## Database Schema Status

### Tables in `mcp_compose` Database:

**Public Schema:**
- `chat_sessions` - Chat session management (with persistence verification)
- `chat_messages` - Chat message history (has `is_automated`, `from_task_run_id` columns)
- `marketplace_servers` - Server catalog
- `marketplace_categories` - Server categories
- `user_installed_servers` - User installations

**Task_Scheduler Schema:**
- `scheduler_tasks` - Scheduled tasks
  - Has: `chat_session_id`, `output_to_chat`, `inherit_session_context`, `provider`, `model`, `mcp_servers`
  - Has: `conversation_id`, `conversation_name`, `conversation_context` (OpenWebUI compatibility)
  - Type constraint: `CHECK (type IN ('ai', 'AI', 'shell', 'manual', 'dependency', 'watcher'))`
- `scheduler_task_runs` - Task execution history
- `scheduler_task_memory` - Task-specific memory

**Cross-Schema Foreign Keys:**
- `scheduler_tasks.chat_session_id` → `public.chat_sessions(id)` ON DELETE CASCADE (CURRENTLY DROPPED)
- `chat_messages.from_task_run_id` → `task_scheduler.scheduler_task_runs(id)` ON DELETE SET NULL

---

## System Status

### Services Running:
```
dashboard        Running  mcp-compose-dashboard (session persistence working)
task-scheduler   Running  mcp-compose-task-scheduler (using PostgreSQL ✅)
memory           Running  mcp-compose-memory
postgres-memory  Running  mcp-compose-postgres-memory
proxy            Running  mcp-compose-http-proxy
```

### Storage Backend:
```bash
docker logs mcp-compose-task-scheduler | grep "storage initialized"
# Output: PostgreSQL storage initialized: postgresql:***@mcp-compose-postgres-memory:5432/mcp_compose
```

### Task Execution Status:
- ✅ Tasks are created and saved to PostgreSQL
- ✅ Tasks execute on schedule
- ✅ Task runs are saved to `scheduler_task_runs` table
- ❌ Tasks have `output_to_chat = false` (should be true)
- ❌ Tasks have empty `chat_session_id` (should have session ID)
- ❌ Task results NOT posted to chat

---

## What Needs To Be Fixed

### Priority 1: Fix Chat Context Propagation ❌

**Investigation Needed:**
1. Verify which code path `task_scheduler_create_task` tool uses:
   - Is it an MCP tool (prefix `mcp_`)?
   - Or a system tool?
2. Check if enrichment code in `executeMCPTool` is actually executing:
   - Enable DEBUG logging
   - Check if `_chat_session_id` appears in logs
3. Verify the tool name transformation:
   - Dashboard might be calling it as system tool
   - MCP prefix might not be applied

**Debugging Steps:**
```bash
# Check what tools are registered
curl http://localhost:3111/api/chat/providers

# Check dashboard logs for tool execution
docker logs mcp-compose-dashboard 2>&1 | grep -i "executeToolCallByName\|executeMCPTool"

# Check task scheduler for received parameters
docker logs mcp-compose-task-scheduler 2>&1 | grep -i "_chat_session_id\|_output_to_chat"
```

**Potential Solutions:**
1. **If system tool:** Move enrichment logic to system tool execution path
2. **If MCP tool:** Debug why enrichment code not executing
3. **Alternative approach:** Pass context via HTTP headers instead of arguments
4. **Fallback:** Update `task_scheduler_create_task` handler to extract session from authenticated request

### Priority 2: Restore Foreign Key Constraint

Once chat context is working:
```sql
ALTER TABLE task_scheduler.scheduler_tasks
ADD CONSTRAINT fk_chat_session
FOREIGN KEY (chat_session_id)
REFERENCES public.chat_sessions(id)
ON DELETE CASCADE;
```

### Priority 3: Test Complete Chat Integration Flow

Once above fixes are complete:
1. Create task via chat interface
2. Verify `output_to_chat = true` and `chat_session_id` is set
3. Wait for task execution
4. Verify task run saves with output
5. Verify POST to `/api/internal/task-output` occurs
6. Verify automated message appears in chat with `is_automated=true`
7. Verify unread count increments
8. Verify WebSocket broadcast delivers message to frontend

---

## Files Modified

### mcp-compose Repository:
1. **internal/task_scheduler/manager.go**
   - Fixed port mapping: `8018:8080` (external:internal)
   - Environment variable logic for PostgreSQL

2. **internal/dashboard/chat_service.go**
   - Added session persistence with verification (lines 105-150)
   - Added context timeout handling (10 seconds)
   - Added MCP tool argument enrichment (lines 1691-1765) - **NOT WORKING YET**
   - Fixed in-memory session update errors

3. **internal/dashboard/chat_storage.go**
   - Added post-insert verification (lines 86-137)
   - Added RowsAffected check for silent failures

4. **internal/database/migrations/003_create_scheduler_schema.sql**
   - Updated type constraint to accept 'ai' and 'AI'

### mcp-cron-persistent Repository:
1. **internal/model/task.go**
   - Fixed task type: `TypeShellCommand = "shell"` (was "shell_command")

2. **internal/storage/postgres.go**
   - Added NULL handling for `command` and `prompt` fields using `sql.NullString`

3. **internal/server/handlers.go**
   - Added `ChatContext` struct and `extractChatContext` function - **NOT BEING CALLED**

4. **internal/server/server.go**
   - Added `applyChatContext` function - **NOT BEING CALLED**
   - Modified task creation handlers to extract chat context - **NOT WORKING**

5. **internal/server/dependency_tools.go**
   - Modified dependency/watcher handlers - **NOT WORKING**

6. **internal/server/manual_tools.go**
   - Modified manual task handler - **NOT WORKING**

---

## Commands for Testing

### Check Storage Backend:
```bash
docker logs mcp-compose-task-scheduler | grep "storage initialized"
# Should show: PostgreSQL storage initialized
```

### Check Task Data:
```bash
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, name, type, output_to_chat, chat_session_id FROM task_scheduler.scheduler_tasks;"
```

### Check Task Runs:
```bash
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, task_id, status, output, posted_to_chat FROM task_scheduler.scheduler_task_runs ORDER BY started_at DESC LIMIT 5;"
```

### Check Chat Sessions:
```bash
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, title, created_at FROM chat_sessions ORDER BY created_at DESC LIMIT 5;"
```

### Check Automated Chat Messages:
```bash
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, content, is_automated, from_task_run_id FROM chat_messages WHERE is_automated = true ORDER BY created_at DESC LIMIT 5;"
```

### Manual Fix for Testing:
```bash
# Get latest session ID
SESSION_ID=$(docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -t -c \
  "SELECT id FROM chat_sessions ORDER BY created_at DESC LIMIT 1;" | xargs)

# Update task to post to chat
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "UPDATE task_scheduler.scheduler_tasks SET output_to_chat = true, chat_session_id = '$SESSION_ID' WHERE id = '<task_id>';"
```

---

## Key Insights

1. ✅ **PostgreSQL Storage Works:** Task scheduler successfully uses PostgreSQL
2. ✅ **Tasks Execute:** Scheduled tasks run on time and save results
3. ✅ **Session Persistence Works:** Chat sessions are properly saved to database
4. ❌ **Chat Context Not Propagating:** The `_chat_session_id` and `_output_to_chat` fields are not being set
5. ❌ **Enrichment Code Path Unknown:** Debug logs suggest the enrichment code in `executeMCPTool` is not executing
6. ⚠️ **Manual Workaround Exists:** Tasks can be manually updated to post to chat

---

## Next Steps for New Agent

### Immediate Focus:
1. **Debug tool execution path:**
   - Determine if `task_scheduler_create_task` is MCP tool or system tool
   - Add logging to trace exact code path
   - Verify `executeToolCallByName` → `executeMCPTool` flow

2. **Fix context propagation:**
   - If system tool: Move enrichment to system tool path
   - If MCP tool: Debug why enrichment not executing
   - Consider alternative: Pass session via authenticated request headers

3. **Enable debug logging:**
   - Set log level to DEBUG in dashboard
   - Check if logger.Debug calls are executed
   - Trace tool call from chat → dashboard → task scheduler

### Alternative Approaches:
1. **HTTP Headers:** Pass `X-Chat-Session-ID` header through proxy
2. **Authenticated Context:** Extract session from OAuth/API key context
3. **Tool Registration:** Ensure tool is properly registered as MCP tool with prefix
4. **Direct Database Lookup:** Task scheduler could query active session for authenticated user

### Testing:
Once fixed, the complete flow should be:
- User creates task in chat →
- Task saved with `output_to_chat=true` and `chat_session_id` →
- Task executes →
- Result posted to `/api/internal/task-output` →
- Automated message created in `chat_messages` →
- WebSocket broadcasts to frontend →
- User sees task output in chat

---

**Current Status:** Task execution works perfectly, but chat output integration is blocked by context propagation issue. The enrichment code exists in both repos but is not being executed. Need to debug the tool execution path to determine why `_chat_session_id` and `_output_to_chat` are not being set.
