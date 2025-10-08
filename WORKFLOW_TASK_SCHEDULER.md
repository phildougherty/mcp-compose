# Workflow Task Scheduler Integration

## Overview

This document describes the integration between the workflow creation canvas and the task-scheduler system, enabling workflows with schedule triggers to be automatically registered as scheduled tasks.

## Architecture Principles

**IMPORTANT**: The workflow system does NOT have its own cron executor. The task-scheduler is the ONLY system that handles scheduling in mcp-compose.

### Division of Responsibilities

**Workflow System** (`internal/workflow/`):
- Visual workflow builder UI
- Workflow definition storage (PostgreSQL)
- **Stateless** workflow execution engine
- NO scheduling capabilities
- Executes only when called via HTTP API

**Task Scheduler** (`mcp-cron-persistent`):
- **ONLY** cron/scheduling system
- Executes shell commands, AI tasks, AND workflows
- Handles ALL scheduling logic
- Provides ALL task features (OutputToChat, chat sessions, history, retries, etc.)
- Calls workflow executor when workflow tasks are triggered

### Consolidated System

Workflows are just another **execution type** alongside shell and AI tasks. All three types share:
- Same scheduling system (cron)
- Same execution history
- Same chat integration
- Same retry policies
- Same observability/metrics
- Same MCP tools for management

## Current Architecture

### Workflow System (mcp-compose)
**Location**: `internal/workflow/` and `internal/dashboard/frontend/src/components/WorkflowBuilder/`

**Components**:
- **WorkflowBuilder.tsx**: React Flow-based visual workflow canvas
- **TriggerNodeConfig.jsx**: Configures trigger nodes with cron schedules
- **workflow/executor.go**: **Stateless** executor that runs workflows when called
- **workflow/storage.go**: PostgreSQL persistence for workflow definitions
- **workflow/api.go**: REST API for workflow CRUD operations

**Trigger Types Supported in UI**:
- `cron`: Configured in UI, but creates a task-scheduler task
- `webhook`: HTTP webhook triggers (future)
- `event`: Event-based triggers (future)
- `manual`: Manual execution only

**CRITICAL**: The workflow executor does NOT schedule or trigger workflows. It only executes them when called.

**Workflow Structure**:
```json
{
  "id": "workflow-uuid",
  "name": "Workflow Name",
  "nodes": [...],
  "edges": [...],
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

**Trigger Node Data**:
```json
{
  "type": "trigger",
  "data": {
    "label": "Schedule Trigger",
    "triggerType": "cron",
    "cronSchedule": "0 0 * * *",
    "enabled": true
  }
}
```

### Task Scheduler (mcp-cron-persistent)
**Location**: `/home/phil/dev/mcp-cron-persistent/`

**Components**:
- **model/task.go**: Task data model
- **scheduler/scheduler.go**: **ONLY** cron-based task scheduler in the system
- **server/tools.go**: MCP tools for task management
- **storage/**: SQLite and PostgreSQL storage backends
- **agent/**: Execution engines for different task types

**Current Task Types**:
- `shell`: Execute shell commands
- `ai`: Execute AI prompts with LLM

**Task Structure**:
```go
type Task struct {
    ID          string
    Name        string
    Description string
    Command     string      // for shell tasks
    Prompt      string      // for AI tasks
    Schedule    string      // cron expression
    Type        string      // "shell" or "ai"
    Enabled     bool
    Status      TaskStatus

    // Chat integration
    ChatSessionID   string
    OutputToChat    bool
    ...
}
```

**MCP Tools**:
- `add_task`: Add shell command task
- `add_ai_task`: Add AI task
- `list_tasks`: List all tasks
- `get_task`: Get task details
- `update_task`: Update task
- `remove_task`: Remove task
- `enable_task`/`disable_task`: Toggle task status
- `run_task`: Manually trigger task

### Chat Interface Integration
**Location**: `internal/dashboard/frontend/src/components/Chat/Chat.jsx`

**Features**:
- Detects workflow suggestions from LLM responses
- Workflow deployment panel for suggested workflows
- System prompt customization
- Chat sessions with message history
- Task results sent back to chat sessions

**Current Integration**:
- Chat can suggest and deploy workflows
- Task scheduler has `ChatSessionID` and `OutputToChat` fields
- Dashboard internal URL: `http://mcp-compose-dashboard:3001`

## Integration Design

### Execution Flow for Scheduled Workflows

```
┌─────────────────────────────────────────────────────────────────┐
│ User creates workflow with schedule trigger in Workflow Builder │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Workflow saved to PostgreSQL (mcp-compose)                      │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Workflow API detects cron trigger → creates task-scheduler task │
│ POST /mcp/v1/call_tool → add_workflow_task                      │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Task-Scheduler stores task (type: "workflow")                   │
│ Registers with cron system using schedule                       │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Cron trigger fires at scheduled time                            │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Task-Scheduler executes workflow task                           │
│ Calls: POST /api/workflows/{id}/execute                         │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Workflow Executor runs workflow (stateless)                     │
│ Returns result to task-scheduler                                │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      v
┌─────────────────────────────────────────────────────────────────┐
│ Task-Scheduler handles result:                                  │
│ - Saves to execution history                                    │
│ - Sends to chat session if OutputToChat=true                    │
│ - Updates task status                                           │
│ - Broadcasts activity                                           │
└─────────────────────────────────────────────────────────────────┘
```

### Key Points

1. **Single Scheduling System**: Task-scheduler's cron is the ONLY scheduler
2. **Workflow = Execution Type**: Workflows are treated like AI or shell tasks
3. **All Features Work**: OutputToChat, chat sessions, history, retries all work for workflows
4. **No Duplication**: No duplicate scheduling logic between systems
5. **Stateless Workflows**: Workflow executor has no state, no triggers, no cron

## Implementation Requirements

### 1. New "workflow" Task Type

Add a third task type to the task-scheduler to represent workflow executions.

**Changes in mcp-cron-persistent**:

`internal/model/task.go`:
```go
const (
    TypeShellCommand TaskType = "shell"
    TypeAI           TaskType = "ai"
    TypeWorkflow     TaskType = "workflow"  // NEW
)

type Task struct {
    // ... existing fields ...

    // Workflow-specific fields
    WorkflowID   string `json:"workflowId,omitempty" description:"ID of workflow to execute"`
    WorkflowName string `json:"workflowName,omitempty" description:"Name of workflow for display"`
}
```

### 2. Workflow Execution Handler in Task Scheduler

The task scheduler needs to execute workflows by calling the mcp-compose workflow execution API.

**New file**: `internal/agent/workflow_executor.go`:
```go
package agent

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"

    "mcp-cron-persistent/internal/model"
)

type WorkflowExecutor struct {
    dashboardURL string
    httpClient   *http.Client
}

func NewWorkflowExecutor(dashboardURL string) *WorkflowExecutor {
    return &WorkflowExecutor{
        dashboardURL: dashboardURL,
        httpClient: &http.Client{
            Timeout: 5 * time.Minute,
        },
    }
}

// Execute implements the model.Executor interface for workflow tasks
func (we *WorkflowExecutor) Execute(ctx context.Context, task *model.Task, timeout time.Duration) error {
    if task.WorkflowID == "" {
        return fmt.Errorf("workflow task missing workflowId")
    }

    url := fmt.Sprintf("%s/api/workflows/%s/execute", we.dashboardURL, task.WorkflowID)

    // Optional: pass task metadata to workflow
    input := map[string]interface{}{
        "taskId":        task.ID,
        "taskName":      task.Name,
        "chatSessionId": task.ChatSessionID,
    }

    body, _ := json.Marshal(map[string]interface{}{"input": input})

    req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    resp, err := we.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("failed to execute workflow: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("workflow execution failed with status %d: %s", resp.StatusCode, body)
    }

    var result WorkflowExecutionResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return fmt.Errorf("failed to decode workflow result: %w", err)
    }

    if result.Error != "" {
        return fmt.Errorf("workflow execution error: %s", result.Error)
    }

    return nil
}

type WorkflowExecutionResult struct {
    ExecutionID string                 `json:"execution_id"`
    Status      string                 `json:"status"`
    Output      map[string]interface{} `json:"output"`
    Error       string                 `json:"error,omitempty"`
    Duration    string                 `json:"duration"`
}
```

**Update**: `internal/command/executor.go` or create new dispatch logic:
```go
func (s *Scheduler) executeTask(ctx context.Context, task *model.Task) error {
    var executor model.Executor

    switch model.TaskType(task.Type) {
    case model.TypeShellCommand:
        executor = s.shellExecutor
    case model.TypeAI:
        executor = s.aiExecutor
    case model.TypeWorkflow:
        executor = s.workflowExecutor  // NEW
    default:
        return fmt.Errorf("unknown task type: %s", task.Type)
    }

    return executor.Execute(ctx, task, task.MaxExecutionTime)
}
```

### 3. Workflow API Endpoint for Execution

Add workflow execution endpoint to mcp-compose workflow API.

**Update** `internal/workflow/api.go`:
```go
func (h *APIHandler) HandleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Extract workflow ID from URL path
    workflowID := extractWorkflowID(r.URL.Path)

    // Load workflow from storage
    workflow, err := h.storage.GetWorkflow(workflowID)
    if err != nil {
        http.Error(w, fmt.Sprintf("Workflow not found: %v", err), http.StatusNotFound)
        return
    }

    // Parse optional input
    var input map[string]interface{}
    if r.Body != nil {
        var req struct {
            Input map[string]interface{} `json:"input"`
        }
        if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
            input = req.Input
        }
    }

    // Create executor
    executor := NewWorkflowExecutor(h.storage, h.aiManager, h.mcpProxyURL, h.mcpAPIKey)

    // Execute workflow
    ctx := r.Context()
    result, err := executor.Execute(ctx, workflow, input)

    // Format response
    response := map[string]interface{}{
        "execution_id": result.ExecutionID,
        "status":       result.Status,
        "output":       result.Output,
        "duration":     result.Duration,
    }

    if err != nil {
        response["error"] = err.Error()
        response["status"] = "failed"
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func extractWorkflowID(path string) string {
    // Extract ID from /api/workflows/{id}/execute
    parts := strings.Split(path, "/")
    if len(parts) >= 4 {
        return parts[3]
    }
    return ""
}
```

**Register route in** `internal/workflow/api.go`:
```go
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
    // ... existing routes ...
    mux.HandleFunc("/api/workflows/", h.handleWorkflowRoutes)
}

func (h *APIHandler) handleWorkflowRoutes(w http.ResponseWriter, r *http.Request) {
    if strings.HasSuffix(r.URL.Path, "/execute") {
        h.HandleExecuteWorkflow(w, r)
        return
    }

    // ... other workflow routes ...
}
```

### 4. Auto-Create Task When Workflow is Saved

When a workflow with a cron trigger is saved, automatically create a task in the task-scheduler.

**Update** `internal/workflow/api.go`:
```go
func (h *APIHandler) HandleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
    // ... existing workflow creation code ...

    // After successfully saving workflow
    if err := h.createScheduledTaskForWorkflow(workflow); err != nil {
        h.logger.Error("Failed to create scheduled task for workflow", "error", err)
        // Don't fail the workflow creation, just log the error
    }

    // ... rest of handler ...
}

func (h *APIHandler) HandleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
    // ... existing workflow update code ...

    // Update scheduled task if schedule changed
    if err := h.updateScheduledTaskForWorkflow(workflow); err != nil {
        h.logger.Error("Failed to update scheduled task for workflow", "error", err)
    }

    // ... rest of handler ...
}

func (h *APIHandler) createScheduledTaskForWorkflow(workflow *Workflow) error {
    // Find trigger nodes with cron schedules
    for _, node := range workflow.Nodes {
        if node.Type != "trigger" {
            continue
        }

        var nodeData map[string]interface{}
        if err := json.Unmarshal(node.Data, &nodeData); err != nil {
            continue
        }

        triggerType, _ := nodeData["triggerType"].(string)
        if triggerType != "cron" {
            continue
        }

        cronSchedule, _ := nodeData["cronSchedule"].(string)
        enabled, _ := nodeData["enabled"].(bool)

        if cronSchedule == "" {
            continue
        }

        // Create task in task-scheduler via MCP API
        return h.createTaskSchedulerTask(workflow.ID, workflow.Name, cronSchedule, enabled)
    }

    return nil
}

func (h *APIHandler) createTaskSchedulerTask(workflowID, workflowName, schedule string, enabled bool) error {
    taskSchedulerURL := os.Getenv("TASK_SCHEDULER_URL")
    if taskSchedulerURL == "" {
        taskSchedulerURL = "http://mcp-compose-task-scheduler:8080"
    }

    // Call MCP tool to add workflow task
    mcpRequest := map[string]interface{}{
        "jsonrpc": "2.0",
        "id":      1,
        "method":  "tools/call",
        "params": map[string]interface{}{
            "name": "add_workflow_task",
            "arguments": map[string]interface{}{
                "name":        fmt.Sprintf("Workflow: %s", workflowName),
                "description": fmt.Sprintf("Scheduled execution of workflow %s", workflowID),
                "workflowId":  workflowID,
                "workflowName": workflowName,
                "schedule":    schedule,
                "enabled":     enabled,
            },
        },
    }

    body, _ := json.Marshal(mcpRequest)

    resp, err := http.Post(
        taskSchedulerURL+"/mcp/v1/call_tool",
        "application/json",
        bytes.NewReader(body),
    )
    if err != nil {
        return fmt.Errorf("failed to call task scheduler: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("task scheduler returned %d: %s", resp.StatusCode, body)
    }

    h.logger.Info("Created scheduled task for workflow", "workflowId", workflowID, "schedule", schedule)
    return nil
}
```

### 5. Task Scheduler MCP Tool

Add new MCP tool for adding workflow tasks.

**Add to** `internal/server/tools.go`:
```go
{
    Name:        "add_workflow_task",
    Description: "Adds a new scheduled workflow task that will execute a workflow on a cron schedule",
    Handler:     s.handleAddWorkflowTask,
    Parameters:  WorkflowTaskParams{},
},
```

**Add to** `internal/server/handlers.go`:
```go
type WorkflowTaskParams struct {
    Name         string `json:"name" description:"Name of the workflow task"`
    Description  string `json:"description,omitempty" description:"Description of the workflow task"`
    WorkflowID   string `json:"workflowId" description:"ID of the workflow to execute"`
    WorkflowName string `json:"workflowName,omitempty" description:"Name of the workflow for display"`
    Schedule     string `json:"schedule" description:"Cron schedule expression"`
    Enabled      bool   `json:"enabled,omitempty" description:"Whether task is enabled (default: true)"`
    OutputToChat bool   `json:"outputToChat,omitempty" description:"Send results to chat session"`
    ChatSessionID string `json:"chatSessionId,omitempty" description:"Chat session ID for output"`
}

func (s *MCPServer) handleAddWorkflowTask(req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
    var params WorkflowTaskParams
    if err := unmarshalParams(req.Params.Arguments, &params); err != nil {
        return errorResult(err)
    }

    // Validate workflow ID
    if params.WorkflowID == "" {
        return errorResult(fmt.Errorf("workflowId is required"))
    }

    // Validate schedule
    if params.Schedule == "" {
        return errorResult(fmt.Errorf("schedule is required"))
    }

    task := &model.Task{
        ID:            generateTaskID(),
        Name:          params.Name,
        Description:   params.Description,
        Type:          string(model.TypeWorkflow),
        WorkflowID:    params.WorkflowID,
        WorkflowName:  params.WorkflowName,
        Schedule:      params.Schedule,
        Enabled:       params.Enabled,
        Status:        model.StatusPending,
        OutputToChat:  params.OutputToChat,
        ChatSessionID: params.ChatSessionID,
        CreatedAt:     time.Now(),
        UpdatedAt:     time.Now(),
    }

    if err := s.scheduler.AddTask(task); err != nil {
        return errorResult(fmt.Errorf("failed to add workflow task: %w", err))
    }

    return &protocol.CallToolResult{
        Content: []protocol.Content{
            {
                Type: "text",
                Text: fmt.Sprintf("Workflow task '%s' added successfully with ID: %s\nSchedule: %s\nWorkflow ID: %s",
                    task.Name, task.ID, task.Schedule, task.WorkflowID),
            },
        },
    }, nil
}
```

### 6. Chat System Prompt Update

Update the chat system prompt to make the LLM aware of workflows in the task scheduler.

**Addition to system prompt** (location TBD in codebase):
```
# Workflow and Task Scheduling System

You have access to a unified task scheduling system that can execute three types of tasks:
1. Shell commands (type: "shell")
2. AI prompts with MCP tool access (type: "ai")
3. Multi-step workflows (type: "workflow")

## Workflows
Workflows are visual, multi-step processes created in the Workflow Builder. When a workflow has a schedule trigger with a cron expression, it is automatically registered as a task in the task scheduler.

All task scheduler features work with workflows:
- Scheduled execution via cron
- Manual execution via run_task tool
- Execution history and logging
- Output to chat sessions (OutputToChat)
- Retry policies and error handling
- Metrics and monitoring

## Available MCP Tools
Use these tools to manage all task types including workflows:
- list_tasks: List all scheduled tasks (shell, ai, workflow)
- get_task: Get details of any task type
- run_task: Manually trigger any task including workflows
- add_workflow_task: Schedule a workflow for automatic execution
- enable_task/disable_task: Control task execution
- list_run_status: View execution history for all task types

## When Users Ask About Workflows
- To schedule a workflow: Use add_workflow_task with the workflow ID and cron schedule
- To view scheduled workflows: Use list_tasks and filter by type "workflow"
- To execute a workflow now: Use run_task with the workflow task ID
- To see workflow execution history: Use list_run_status

The workflow system and task scheduler are fully integrated - workflows are just another type of scheduled task.
```

### 7. Frontend Task Scheduler UI

Update the task scheduler UI to display and differentiate workflow tasks.

**Location**: `internal/dashboard/frontend/src/components/TaskScheduler/TaskScheduler.jsx`

**Changes**:

```jsx
import { WorkflowIcon } from '@heroicons/react/24/outline';

const getTaskTypeIcon = (type) => {
  switch(type) {
    case 'shell':
      return <CommandLineIcon className="h-5 w-5 text-gray-600" />;
    case 'ai':
      return <SparklesIcon className="h-5 w-5 text-purple-600" />;
    case 'workflow':
      return <WorkflowIcon className="h-5 w-5 text-blue-600" />;
    default:
      return <QuestionMarkCircleIcon className="h-5 w-5" />;
  }
};

const getTaskTypeBadge = (type) => {
  const badges = {
    shell: 'bg-gray-100 text-gray-800 border-gray-300',
    ai: 'bg-purple-100 text-purple-800 border-purple-300',
    workflow: 'bg-blue-100 text-blue-800 border-blue-300',
  };

  return (
    <span className={`px-2 py-1 text-xs font-medium rounded border ${badges[type] || badges.shell}`}>
      {type.toUpperCase()}
    </span>
  );
};

const TaskRow = ({ task, onRun, onEdit, onDelete }) => (
  <tr className="hover:bg-gray-50 dark:hover:bg-gray-800">
    <td className="px-4 py-3">
      <div className="flex items-center space-x-3">
        {getTaskTypeIcon(task.type)}
        <div>
          <div className="font-medium text-gray-900 dark:text-white">
            {task.name}
          </div>
          {task.type === 'workflow' && task.workflowName && (
            <div className="text-sm text-gray-500 dark:text-gray-400">
              {task.workflowName}
            </div>
          )}
        </div>
      </div>
    </td>
    <td className="px-4 py-3">
      {getTaskTypeBadge(task.type)}
    </td>
    <td className="px-4 py-3">
      <code className="text-xs bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
        {task.schedule}
      </code>
    </td>
    <td className="px-4 py-3">
      {task.type === 'workflow' && task.workflowId && (
        <a
          href={`/workflow-builder?id=${task.workflowId}`}
          className="text-blue-600 hover:text-blue-800 text-sm"
          target="_blank"
          rel="noopener noreferrer"
        >
          View Workflow →
        </a>
      )}
    </td>
    {/* ... other columns ... */}
  </tr>
);
```

**Update constants** in `internal/dashboard/frontend/src/components/TaskScheduler/constants.js`:
```js
export const TASK_TYPES = {
  SHELL: 'shell',
  AI: 'ai',
  WORKFLOW: 'workflow',
};

export const TASK_TYPE_OPTIONS = [
  { value: 'shell', label: 'Shell Command' },
  { value: 'ai', label: 'AI Task' },
  { value: 'workflow', label: 'Workflow' },
];
```

## Implementation Sequence

### Phase 1: Backend Foundation ✅
- [x] Review workflow system architecture
- [x] Review task-scheduler system
- [x] Review chat interface integration
- [x] Document current state and integration plan
- [x] Clarify consolidated architecture

### Phase 2: Task Scheduler Updates
- [ ] Add `TypeWorkflow` constant to `model/task.go`
- [ ] Add `WorkflowID` and `WorkflowName` fields to Task struct
- [ ] Create `agent/workflow_executor.go` for workflow execution
- [ ] Add `add_workflow_task` MCP tool to `server/tools.go`
- [ ] Implement `handleAddWorkflowTask` handler
- [ ] Update task execution dispatcher to handle workflow type
- [ ] Initialize WorkflowExecutor in server startup

### Phase 3: Workflow API Updates
- [ ] Add `/api/workflows/{id}/execute` endpoint
- [ ] Implement workflow execution handler
- [ ] Add `createScheduledTaskForWorkflow` to workflow API
- [ ] Add `updateScheduledTaskForWorkflow` for schedule changes
- [ ] Update workflow create/update handlers to auto-create tasks
- [ ] Add `TASK_SCHEDULER_URL` environment variable

### Phase 4: Chat Integration
- [ ] Update system prompt to include workflow awareness
- [ ] Test LLM understanding of workflow tasks
- [ ] Verify workflow suggestions work with scheduling
- [ ] Test OutputToChat with workflow executions

### Phase 5: Frontend Updates
- [ ] Update TaskScheduler.jsx to display workflow tasks
- [ ] Add workflow type icon and styling
- [ ] Add link to open workflow in builder
- [ ] Update task type constants and options
- [ ] Test UI with workflow tasks

### Phase 6: Testing
- [ ] Test creating workflow with cron trigger
- [ ] Verify task is auto-created in task-scheduler
- [ ] Test scheduled workflow execution
- [ ] Test manual workflow execution via run_task
- [ ] Test workflow OutputToChat to chat session
- [ ] Test workflow execution history
- [ ] Test chat interface workflow awareness
- [ ] End-to-end integration test

## API Contracts

### Task Scheduler → Workflow Execution

**Request**: `POST /api/workflows/{workflowId}/execute`
```json
{
  "input": {
    "taskId": "task_123",
    "taskName": "Workflow: Daily Processing",
    "chatSessionId": "session-abc"
  }
}
```

**Response**:
```json
{
  "execution_id": "exec-uuid-123",
  "status": "completed",
  "output": {
    "final_result": "...",
    "node_outputs": {...}
  },
  "duration": "5.2s",
  "error": null
}
```

### Workflow API → Task Scheduler

**Request**: `POST /mcp/v1/call_tool` (MCP protocol)
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "add_workflow_task",
    "arguments": {
      "name": "Workflow: Daily Data Processing",
      "description": "Scheduled execution of workflow abc-123",
      "workflowId": "abc-123",
      "workflowName": "Daily Data Processing",
      "schedule": "0 0 * * *",
      "enabled": true,
      "outputToChat": false
    }
  }
}
```

**Response**:
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [{
      "type": "text",
      "text": "Workflow task 'Workflow: Daily Data Processing' added successfully with ID: task_1234567890\nSchedule: 0 0 * * *\nWorkflow ID: abc-123"
    }]
  }
}
```

## Configuration

### Environment Variables

**mcp-compose** (dashboard):
- `TASK_SCHEDULER_URL`: URL of task scheduler service (default: `http://mcp-compose-task-scheduler:8080`)

**mcp-cron-persistent** (task scheduler):
- `DASHBOARD_INTERNAL_URL`: URL of dashboard service (default: `http://mcp-compose-dashboard:3001`)
- Existing variables remain unchanged

### Docker Network

Both services must be on the same Docker network (`mcp-net`) for internal HTTP communication.

## Database Schema

### Workflow Storage (PostgreSQL in mcp-compose)
Existing workflow tables remain unchanged. Workflows store only their definition, not scheduling information.

### Task Storage (PostgreSQL/SQLite in task-scheduler)
The existing `tasks` table supports workflow tasks via the Type field.

**Additional index recommended**:
```sql
CREATE INDEX idx_tasks_workflow_id ON tasks(workflow_id) WHERE type = 'workflow';
```

## Error Handling

### Workflow Execution Failures
- Task scheduler logs execution errors
- Workflow execution errors stored in task result
- Failed workflow executions trigger retry logic if configured
- Alert notifications sent if task has `AlertOnFailure` enabled
- Execution result sent to chat if `OutputToChat=true`

### Task Creation Failures
- Workflow creation succeeds even if task creation fails
- Error logged for debugging
- User can manually create task via task scheduler UI or MCP tools
- Workflow remains executable via manual triggers

### Network Failures
- Task scheduler retries HTTP calls to workflow API
- Timeout configuration via task `MaxExecutionTime`
- Circuit breaker pattern for repeated failures

## Benefits of Consolidated Architecture

1. **No Duplication**: Single cron system, no duplicate scheduling logic
2. **All Features Work**: OutputToChat, retries, history, metrics work for workflows
3. **Unified Management**: Same MCP tools manage all task types
4. **Consistent UX**: Same UI for viewing/managing all scheduled tasks
5. **Simpler Debugging**: One place to check for scheduling issues
6. **Chat Integration**: Workflows can send results to chat sessions
7. **Extensibility**: Easy to add new execution types (e.g., "http", "python")

## Future Enhancements

1. **Bidirectional Sync**: Update workflow when task schedule is modified in task scheduler
2. **Workflow Versioning**: Track which workflow version was executed by each task run
3. **Conditional Scheduling**: Enable/disable workflow tasks based on conditions
4. **Workflow Templates**: Pre-configured workflows with schedules
5. **Execution History UI**: View workflow execution history in workflow builder
6. **Live Execution Monitoring**: Stream workflow execution progress to UI
7. **Workflow Chaining**: Trigger one workflow from another's completion via task dependencies
8. **Webhook Triggers**: Task scheduler handles incoming webhooks that trigger workflows
9. **Event-Based Triggers**: Watch for events (file changes, task completions) to trigger workflows

## Status

**Current Phase**: Phase 1 - Backend Foundation (COMPLETED ✅)

**Next Steps**: Begin Phase 2 - Task Scheduler Updates
