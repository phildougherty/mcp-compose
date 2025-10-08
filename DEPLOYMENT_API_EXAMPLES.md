# Workflow Deployment API - Testing Examples

## Prerequisites

1. MCP-Compose running with dashboard enabled
2. PostgreSQL configured (POSTGRES_URL environment variable)
3. At least one AI provider configured:
   - ANTHROPIC_API_KEY for Claude
   - OPENAI_API_KEY for OpenAI
   - OPENROUTER_API_KEY for OpenRouter
   - Or Ollama running locally at http://localhost:11434

## Example 1: GitHub PR Monitor (Template-based)

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Monitor phildougherty/mcp-compose for new pull requests and send notifications to #dev-team on Slack",
    "autoStart": false
  }'
```

### Expected Processing

1. AI matches template "github-pr-monitor" with high confidence
2. Extracts parameters:
   - `repo`: "phildougherty/mcp-compose"
   - `slack_channel`: "#dev-team"
3. Fills template with parameters
4. Validates and saves workflow

### Expected Response

```json
{
  "workflowId": "550e8400-e29b-41d4-a716-446655440000",
  "name": "GitHub PR Monitor - phildougherty/mcp-compose",
  "preview": "Monitor phildougherty/mcp-compose for new pull requests and send notifications to #dev-team",
  "nodes": [
    {
      "id": "trigger-1",
      "type": "trigger",
      "position": {"x": 100, "y": 100},
      "data": "{\"label\":\"GitHub Webhook\",\"config\":{\"type\":\"webhook\",\"repo\":\"phildougherty/mcp-compose\"}}"
    },
    {
      "id": "server-1",
      "type": "mcp-server",
      "position": {"x": 300, "y": 100},
      "data": "{\"label\":\"Send Slack Notification\",\"config\":{\"server\":\"slack\",\"tool\":\"send_message\",\"channel\":\"#dev-team\",\"message\":\"New PR: {{trigger.pr_title}}\"}}"
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "trigger-1",
      "target": "server-1"
    }
  ],
  "deployed": true
}
```

## Example 2: Scheduled Report (Direct Template)

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Generate daily status report",
    "templateId": "scheduled-report",
    "parameters": {
      "report_name": "Daily Status Report",
      "schedule": "0 9 * * *",
      "model": "gpt-4"
    },
    "autoStart": true
  }'
```

### Expected Processing

1. Load template "scheduled-report" directly (skip AI matching)
2. Use provided parameters (skip extraction)
3. Fill template
4. Validate and save
5. Auto-start execution

### Expected Response

```json
{
  "workflowId": "abc-123-def-456",
  "name": "Scheduled Report - Daily Status Report",
  "preview": "Generate and send Daily Status Report report 0 9 * * *",
  "nodes": [
    {
      "id": "trigger-1",
      "type": "trigger",
      "position": {"x": 100, "y": 100},
      "data": "{\"label\":\"Schedule Trigger\",\"config\":{\"type\":\"schedule\",\"cron\":\"0 9 * * *\"}}"
    },
    {
      "id": "ai-1",
      "type": "ai-task",
      "position": {"x": 300, "y": 100},
      "data": "{\"label\":\"Generate Report\",\"config\":{\"model\":\"gpt-4\",\"prompt\":\"Generate Daily Status Report report\"}}"
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "trigger-1",
      "target": "ai-1"
    }
  ],
  "deployed": true,
  "executionId": "exec-789-xyz"
}
```

## Example 3: Custom Workflow Generation

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Every morning at 9am, fetch my GitHub notifications from the last 24 hours, use AI to create a summary, and email it to me at user@example.com"
  }'
```

### Expected Processing

1. AI template matching returns low confidence (no exact match)
2. Falls back to custom workflow generation
3. AI generates workflow with:
   - Schedule trigger (9am daily)
   - GitHub MCP server node (fetch notifications)
   - AI task node (summarize with GPT-4)
   - Email MCP server node (send summary)
4. Validates generated workflow
5. Saves to database

### Expected Response

```json
{
  "workflowId": "custom-workflow-123",
  "name": "Daily GitHub Notification Summary",
  "preview": "Every morning at 9am, fetch GitHub notifications, summarize with AI, and email results",
  "nodes": [
    {
      "id": "node-abc123",
      "type": "trigger",
      "position": {"x": 100, "y": 100},
      "data": "{\"label\":\"Daily Schedule\",\"config\":{\"type\":\"schedule\",\"cron\":\"0 9 * * *\"}}"
    },
    {
      "id": "node-def456",
      "type": "mcp-server",
      "position": {"x": 300, "y": 100},
      "data": "{\"label\":\"Fetch GitHub Notifications\",\"config\":{\"server\":\"github\",\"tool\":\"list_notifications\",\"since\":\"24h\"}}"
    },
    {
      "id": "node-ghi789",
      "type": "ai-task",
      "position": {"x": 500, "y": 100},
      "data": "{\"label\":\"Summarize Notifications\",\"config\":{\"model\":\"gpt-4\",\"prompt\":\"Summarize these GitHub notifications: {{previous.output}}\"}}"
    },
    {
      "id": "node-jkl012",
      "type": "mcp-server",
      "position": {"x": 700, "y": 100},
      "data": "{\"label\":\"Email Summary\",\"config\":{\"server\":\"email\",\"tool\":\"send_email\",\"to\":\"user@example.com\",\"subject\":\"Daily GitHub Summary\",\"body\":\"{{previous.output}}\"}}"
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "node-abc123",
      "target": "node-def456"
    },
    {
      "id": "edge-2",
      "source": "node-def456",
      "target": "node-ghi789"
    },
    {
      "id": "edge-3",
      "source": "node-ghi789",
      "target": "node-jkl012"
    }
  ],
  "deployed": true
}
```

## Example 4: Error - Missing Parameters

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Monitor GitHub for PRs",
    "templateId": "github-pr-monitor"
  }'
```

### Expected Response (400 Bad Request)

```json
{
  "code": "validation_error",
  "message": "validation error for repo: Required parameter missing: repo",
  "details": ""
}
```

## Example 5: Error - Invalid Template

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Test workflow",
    "templateId": "nonexistent-template"
  }'
```

### Expected Response (404 Not Found)

```json
{
  "code": "template_not_found",
  "message": "template not found: nonexistent-template",
  "details": ""
}
```

## Example 6: Error - Missing Description

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "autoStart": false
  }'
```

### Expected Response (400 Bad Request)

```json
{
  "code": "missing_description",
  "message": "Workflow description is required",
  "details": ""
}
```

## Example 7: With Partial Parameters

### Request

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Set up PR notifications for my project",
    "templateId": "github-pr-monitor",
    "parameters": {
      "repo": "owner/repo"
    }
  }'
```

### Expected Processing

1. Load template "github-pr-monitor"
2. User provided `repo` parameter
3. AI/regex extracts `slack_channel` from description
4. If extraction fails, returns error about missing required parameter

### Expected Response (if extraction succeeds)

```json
{
  "workflowId": "partial-params-workflow",
  "name": "GitHub PR Monitor - owner/repo",
  "preview": "Monitor owner/repo for new pull requests and send notifications",
  "nodes": [...],
  "edges": [...],
  "deployed": true
}
```

## Verification Steps

After deploying a workflow, you can verify it was created:

### 1. Check workflow was saved

```bash
curl http://localhost:8080/api/workflows
```

### 2. Get specific workflow

```bash
curl http://localhost:8080/api/workflows/{workflowId}
```

### 3. Execute workflow manually

```bash
curl -X POST http://localhost:8080/api/workflows/{workflowId}/execute
```

### 4. Check execution status

```bash
curl http://localhost:8080/api/workflows/{workflowId}/executions/{executionId}
```

## Testing Without AI Provider

If no AI provider is configured, you can still test with:

### Direct Template + All Parameters

```bash
curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Test workflow",
    "templateId": "github-pr-monitor",
    "parameters": {
      "repo": "owner/repo",
      "slack_channel": "#notifications"
    }
  }'
```

This will work because:
- Template ID is explicitly provided (no AI matching needed)
- All parameters are manually provided (no AI extraction needed)
- Only template filling and validation are performed

## Common Testing Scenarios

### 1. Template Matching Quality

Test different description phrasings:

```bash
# Clear description
"Monitor GitHub repository owner/repo for new PRs and notify #dev-team"

# Vague description
"Set up notifications for my repo"

# Detailed description
"I need a workflow that watches for pull request events on my GitHub repository at owner/repo and sends a formatted message to the #dev-team Slack channel whenever a new PR is created"
```

### 2. Parameter Extraction Accuracy

Test with various formats:

```bash
# Standard format
"repo: owner/repo channel: #notifications"

# Natural language
"for the phildougherty/mcp-compose repository, send to #dev-team"

# Mixed format
"Monitor owner/repo and send messages to the dev-team Slack channel"
```

### 3. Workflow Validation

Test invalid workflows:

```bash
# Missing required node type
"description": "Just send a notification"

# Circular dependency (should fail validation)
# AI should not generate this, but test if it does

# Invalid configuration
"description": "Run cron job with invalid schedule 99 99 99 99 99"
```

## Database Verification

Check PostgreSQL directly:

```sql
-- View deployed workflows
SELECT id, name, description, created_at
FROM workflows.workflows
ORDER BY created_at DESC
LIMIT 10;

-- View workflow nodes
SELECT wn.workflow_id, wn.type, wn.data
FROM workflows.workflow_nodes wn
WHERE wn.workflow_id = 'your-workflow-id';

-- View workflow executions
SELECT id, workflow_id, status, started_at, completed_at
FROM workflows.workflow_executions
WHERE workflow_id = 'your-workflow-id'
ORDER BY started_at DESC;
```

## Performance Testing

### Measure AI Latency

```bash
time curl -X POST http://localhost:8080/api/workflows/deploy \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Monitor GitHub for PRs and notify Slack"
  }'
```

Expected times:
- With AI (template matching + extraction): 3-7 seconds
- Direct template + AI extraction: 1-3 seconds
- Direct template + all parameters: <1 second

### Concurrent Requests

```bash
# Test 10 concurrent deployments
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/workflows/deploy \
    -H "Content-Type: application/json" \
    -d "{\"description\": \"Test workflow $i\"}" &
done
wait
```

## Troubleshooting

### AI Provider Issues

If deployment fails with "no AI providers configured":

1. Check environment variables:
   ```bash
   echo $ANTHROPIC_API_KEY
   echo $OPENAI_API_KEY
   echo $OPENROUTER_API_KEY
   ```

2. Check Ollama:
   ```bash
   curl http://localhost:11434/api/tags
   ```

3. Use direct template + parameters as workaround

### Database Issues

If deployment fails with database errors:

1. Check PostgreSQL connection:
   ```bash
   echo $POSTGRES_URL
   ```

2. Verify schema exists:
   ```sql
   SELECT schema_name FROM information_schema.schemata WHERE schema_name = 'workflows';
   ```

3. Check table initialization:
   ```sql
   SELECT table_name FROM information_schema.tables WHERE table_schema = 'workflows';
   ```

### Validation Errors

If workflow fails validation:

1. Check logs for specific validation errors
2. Verify template requirements match provided parameters
3. Test with simpler descriptions
4. Use direct template + manual parameters to isolate issue
