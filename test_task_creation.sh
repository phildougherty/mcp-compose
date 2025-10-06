#!/bin/bash

# Test task creation with chat context
# This simulates what the dashboard does when creating a task from chat

SESSION_ID="test-session-$(date +%s)"
echo "Testing task creation with session ID: $SESSION_ID"

# Create test task via MCP proxy
curl -X POST http://localhost:9876/task-scheduler \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer myapikey" \
  -H "X-Chat-Session-ID: $SESSION_ID" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "add_task",
      "arguments": {
        "name": "test-chat-context-task",
        "description": "Testing chat context propagation",
        "type": "shell",
        "command": "echo \"Hello from scheduled task\"",
        "schedule": "*/5 * * * *",
        "enabled": true,
        "_chat_session_id": "'$SESSION_ID'",
        "_output_to_chat": true,
        "_provider": "openrouter",
        "_model": "anthropic/claude-3.5-sonnet"
      }
    }
  }' | jq '.'

echo -e "\n\nChecking database for task..."
sleep 2

# Query database to verify task was saved with chat context
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, name, output_to_chat, chat_session_id, provider, model FROM task_scheduler.scheduler_tasks WHERE name = 'test-chat-context-task';"
