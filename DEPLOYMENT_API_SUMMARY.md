# Workflow Deployment API - Implementation Summary

## Overview

The natural language workflow deployment API allows users to create workflows by providing plain text descriptions through the chat interface. The system uses AI to understand the request, match or generate workflows, extract parameters, and deploy fully functional workflows.

## Architecture

### Components Created

1. **internal/workflow/deployment_api.go** - Main deployment endpoint
2. **internal/workflow/deployment_processor.go** - AI-powered NLP processing
3. **internal/workflow/parameter_extractor.go** - Parameter extraction from descriptions
4. **internal/workflow/template_filler.go** - Template substitution and registry

### Integration Points

- **internal/dashboard/workflow_handlers.go** - Dashboard integration wrapper
- **internal/dashboard/server.go** - Route registration and AI manager injection
- **internal/workflow/api.go** - Exposed GetStorage() method for deployment handler

## API Endpoint

### POST /api/workflows/deploy

Processes natural language workflow deployment requests from the chat interface.

#### Request Body

```json
{
  "description": "Create a workflow that monitors GitHub for new PRs and sends Slack notifications",
  "templateId": "optional-template-id",
  "parameters": {
    "repo": "owner/repo",
    "slack_channel": "#notifications"
  },
  "autoStart": true
}
```

**Fields:**
- `description` (required): Natural language description of desired workflow
- `templateId` (optional): Specific template to use, skips AI matching
- `parameters` (optional): Pre-filled parameters, merged with AI-extracted params
- `autoStart` (optional): Whether to execute workflow immediately after creation

#### Response - Success (201 Created)

```json
{
  "workflowId": "550e8400-e29b-41d4-a716-446655440000",
  "name": "GitHub PR Monitor - owner/repo",
  "preview": "Monitor owner/repo for new pull requests and send notifications to #notifications",
  "nodes": [
    {
      "id": "trigger-1",
      "type": "trigger",
      "position": {"x": 100, "y": 100},
      "data": {
        "label": "GitHub Webhook",
        "config": {
          "type": "webhook",
          "repo": "owner/repo"
        }
      }
    },
    {
      "id": "server-1",
      "type": "mcp-server",
      "position": {"x": 300, "y": 100},
      "data": {
        "label": "Send Slack Notification",
        "config": {
          "server": "slack",
          "tool": "send_message",
          "channel": "#notifications"
        }
      }
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "trigger-1",
      "target": "server-1"
    }
  ],
  "deployed": true,
  "executionId": "uuid-if-autoStart"
}
```

#### Response - Error (400/404/500)

```json
{
  "code": "validation_error",
  "message": "Required parameter missing: repo",
  "details": "Parameter extraction failed"
}
```

**Error Codes:**
- `invalid_method` (405): Only POST allowed
- `invalid_request` (400): Malformed JSON
- `missing_description` (400): Description field required
- `validation_error` (400): Workflow validation failed
- `template_not_found` (404): Specified template doesn't exist
- `deployment_failed` (500): Internal server error

## Deployment Process Flow

### Step 1: Template Matching

If no `templateId` provided:

1. AI analyzes the description using the AI Manager
2. System prompt includes available template categories:
   - github-pr-monitor
   - scheduled-report
   - webhook-processor
   - data-sync
   - ai-content-generator

3. AI returns:
   ```json
   {
     "template": "github-pr-monitor",
     "confidence": 0.85,
     "reasoning": "User wants to monitor GitHub and send notifications"
   }
   ```

4. If confidence < 0.6, falls back to workflow generation from scratch

### Step 2: Parameter Extraction

**Regex-based extraction** (always runs first):
- Repository: `owner/repo` patterns
- Slack channels: `#channel` or `@user` patterns
- Email addresses: standard email regex
- URLs/Webhooks: `https?://...` patterns
- Cron schedules: cron expression patterns
- Intervals: "every N seconds/minutes/hours/days"

**AI-based extraction** (if AI manager available):
- Analyzes description against template parameter requirements
- Extracts structured data matching parameter types
- Validates types: string, number, boolean, array, object

**Merging strategy:**
1. Start with regex-extracted parameters
2. Add AI-extracted parameters (non-overlapping)
3. Apply template defaults for missing optional parameters
4. Merge user-provided parameters (highest priority)

### Step 3: Workflow Generation

**Option A: Template-based** (if template matched/provided)
1. Load template from registry
2. Validate all required parameters present
3. Fill placeholders using `{{variable}}` syntax
4. Support nested paths: `{{params.repo.owner}}`
5. Generate workflow with filled nodes and edges

**Option B: From scratch** (if no template matched)
1. AI generates complete workflow specification
2. System prompt defines available node types:
   - trigger: Schedule, webhook, manual
   - ai-task: Text generation, analysis
   - mcp-server: GitHub, Slack, file operations
   - decision: Conditional branching
   - transform: Data manipulation
   - code: Custom JavaScript execution

3. AI returns full workflow JSON with nodes and edges
4. Auto-generate UUIDs for nodes/edges if missing

### Step 4: Validation

**Structural validation:**
- Workflow name required
- At least one node required
- All node IDs unique
- Valid node types
- All edges reference existing nodes

**Full workflow validation** (via existing validator):
- At least one trigger node
- No cycles (DAG validation)
- Proper connectivity
- Node-specific configuration validation

### Step 5: Deployment

1. Assign UUID to workflow
2. Set timestamps (createdAt, updatedAt)
3. Save to PostgreSQL database
4. If `autoStart: true`:
   - For scheduled workflows: Register with task scheduler
   - For webhook workflows: Register webhook endpoint
   - For manual workflows: Execute immediately
   - Return execution ID in response

## Built-in Template Registry

### github-pr-monitor

**Description:** Monitor GitHub repository for new pull requests

**Required Parameters:**
- `repo` (string): GitHub repository (owner/repo)
- `slack_channel` (string): Slack channel for notifications

**Nodes:**
1. GitHub Webhook Trigger
2. Slack Notification Server

### scheduled-report

**Description:** Generate and send reports on schedule

**Required Parameters:**
- `report_name` (string): Name of the report
- `schedule` (string): Cron schedule expression

**Optional Parameters:**
- `model` (string): AI model to use (default: "gpt-4")

**Nodes:**
1. Schedule Trigger
2. AI Report Generator

## Error Handling

### Validation Errors

**DeploymentValidationError** - Parameter/template validation
```go
type DeploymentValidationError struct {
    Field   string
    Message string
}
```

**WorkflowValidationError** - Workflow structure validation
```go
type WorkflowValidationError struct {
    Result *ValidationResult
}

type ValidationResult struct {
    Valid  bool
    Errors []WorkflowValidationErrorDetail
}
```

### Error Scenarios

1. **Description too vague** (400)
   - No template matched with confidence > 0.6
   - AI generation failed to produce valid workflow

2. **Missing required parameters** (400)
   - Template requires params not found in description
   - User didn't provide required parameters

3. **Template not found** (404)
   - Specified templateId doesn't exist in registry

4. **Validation errors** (400)
   - Invalid node types
   - Circular dependencies
   - Missing trigger nodes
   - Invalid edge connections

5. **Server availability errors** (500)
   - Required MCP servers not running
   - Database connection issues
   - AI provider unavailable

## AI Integration

### System Prompts

**Template Matching:**
```
You are a workflow template matching expert.
Analyze the user's workflow description and determine which template best matches.
Available templates: [list]
Return JSON with template, confidence, and reasoning.
```

**Parameter Extraction:**
```
You are a parameter extraction expert.
Extract the following parameters from the description:
Required: [list]
Optional: [list]
Return JSON with extracted parameters or null for missing.
```

**Workflow Generation:**
```
You are a workflow generation expert.
Generate a complete workflow from the description.
Available node types: [list]
Return JSON with name, description, nodes, and edges.
```

### AI Provider Fallback

The system uses the AI Manager with provider fallback:
1. Claude (if ANTHROPIC_API_KEY set)
2. OpenAI (if OPENAI_API_KEY set)
3. OpenRouter (if OPENROUTER_API_KEY set)
4. Ollama (local, default: http://localhost:11434)

If no AI providers available, deployment API still works with:
- Direct template ID specification
- Manual parameter provision
- Regex-based parameter extraction

## Example Request/Response Flows

### Flow 1: Template-based with AI Parameter Extraction

**Request:**
```json
{
  "description": "Monitor phildougherty/mcp-compose for new PRs and notify #dev-team",
  "autoStart": false
}
```

**Processing:**
1. AI matches template: "github-pr-monitor" (confidence: 0.92)
2. Regex extracts: `repo: "phildougherty/mcp-compose"`, `slack_channel: "#dev-team"`
3. Template filled with parameters
4. Workflow validated and saved

**Response:**
```json
{
  "workflowId": "abc-123",
  "name": "GitHub PR Monitor - phildougherty/mcp-compose",
  "preview": "Monitor phildougherty/mcp-compose for new pull requests...",
  "nodes": [...],
  "edges": [...],
  "deployed": true
}
```

### Flow 2: Direct Template with Manual Parameters

**Request:**
```json
{
  "description": "Set up PR monitoring",
  "templateId": "github-pr-monitor",
  "parameters": {
    "repo": "owner/repo",
    "slack_channel": "#notifications"
  },
  "autoStart": true
}
```

**Processing:**
1. Load template directly (skip AI matching)
2. Use provided parameters (skip extraction)
3. Fill template and validate
4. Save and execute immediately

**Response:**
```json
{
  "workflowId": "def-456",
  "name": "GitHub PR Monitor - owner/repo",
  "preview": "Monitor owner/repo...",
  "nodes": [...],
  "edges": [...],
  "deployed": true,
  "executionId": "exec-789"
}
```

### Flow 3: Custom Workflow Generation

**Request:**
```json
{
  "description": "Every morning at 9am, use AI to summarize my GitHub notifications from the last 24 hours and email the summary to me at user@example.com"
}
```

**Processing:**
1. AI template matching fails (confidence < 0.6)
2. AI generates custom workflow:
   - Schedule trigger: "0 9 * * *"
   - GitHub MCP server: fetch notifications
   - AI task: summarize with GPT-4
   - Email MCP server: send summary
3. Validates generated workflow
4. Saves to database

**Response:**
```json
{
  "workflowId": "ghi-789",
  "name": "Daily GitHub Notification Summary",
  "preview": "Every morning at 9am...",
  "nodes": [
    {"type": "trigger", ...},
    {"type": "mcp-server", ...},
    {"type": "ai-task", ...},
    {"type": "mcp-server", ...}
  ],
  "edges": [...],
  "deployed": true
}
```

## Security Considerations

1. **Parameter Validation**
   - All extracted parameters validated against expected types
   - Template requirements enforced
   - No SQL injection via parameter substitution

2. **Workflow Validation**
   - Full DAG validation prevents infinite loops
   - Node configuration validated before execution
   - Required MCP servers checked for availability

3. **AI Prompt Injection Prevention**
   - User descriptions sanitized before AI processing
   - System prompts clearly separated from user input
   - Response parsing validates JSON structure

4. **Rate Limiting**
   - Should be implemented at API gateway level
   - AI provider has built-in rate limiting via Manager

## Frontend Integration

The Chat component sends deployment requests via:

```javascript
const response = await fetch('/api/workflows/deploy', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    description: userMessage,
    autoStart: false
  })
});
```

The frontend system prompt (workflowDeployment.js) guides the user through:
- Asking clarifying questions about workflow requirements
- Suggesting available templates
- Extracting parameters from conversation
- Submitting deployment requests

## Performance Considerations

1. **AI Latency**
   - Template matching: ~1-3 seconds
   - Parameter extraction: ~1-2 seconds
   - Workflow generation: ~3-5 seconds
   - Total: ~5-10 seconds for full AI-powered deployment

2. **Optimization Strategies**
   - Regex extraction runs first (instant)
   - Template matching skipped if templateId provided
   - AI extraction skipped if all parameters provided
   - Parallel AI calls for matching + extraction (future optimization)

3. **Database Performance**
   - Workflow save is single transaction
   - Indexes on workflow ID and creation time
   - No N+1 queries for nodes/edges

## Testing Recommendations

1. **Unit Tests**
   - Parameter extraction with various description formats
   - Template placeholder filling
   - Validation error cases

2. **Integration Tests**
   - Full deployment flow with mock AI provider
   - Template registry operations
   - Database transaction rollback on errors

3. **E2E Tests**
   - Deploy via API and verify in database
   - Auto-start workflows execute correctly
   - Chat interface → API → workflow creation

## Future Enhancements

1. **Template Marketplace**
   - User-contributed templates
   - Template versioning
   - Template popularity and ratings

2. **Improved AI Processing**
   - Multi-turn conversation for parameter refinement
   - Learning from user corrections
   - Template recommendation based on user history

3. **Advanced Parameter Types**
   - File uploads
   - Secret references
   - Environment variable substitution

4. **Workflow Previewing**
   - Visual workflow preview before deployment
   - Parameter validation feedback
   - Cost estimation for AI/API operations

## Deployment Checklist

- [x] Create deployment API endpoint
- [x] Implement AI-powered template matching
- [x] Implement parameter extraction (regex + AI)
- [x] Implement template substitution system
- [x] Add template registry with default templates
- [x] Integrate with workflow validation system
- [x] Register route in dashboard server
- [x] Pass AI manager to deployment handler
- [x] Handle all error scenarios
- [x] Backend builds successfully
- [ ] Frontend build issues (separate from backend)
- [ ] Add rate limiting
- [ ] Add comprehensive logging
- [ ] Add metrics/observability
- [ ] Add unit tests
- [ ] Add integration tests
- [ ] Update API documentation
