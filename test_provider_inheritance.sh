#!/bin/bash

set -e

BASE_URL="http://localhost:3001"

echo "=== Testing Provider/Model Inheritance ==="
echo

echo "1. Creating a new chat session with provider=anthropic, model=claude-3-5-sonnet-20241022..."
SESSION_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/chat/sessions" \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "anthropic",
    "model": "claude-3-5-sonnet-20241022",
    "title": "Provider Inheritance Test"
  }')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.id')
echo "Created session: $SESSION_ID"
echo

echo "2. Sending a message to create a scheduled task..."
MESSAGE_RESPONSE=$(curl -s -X POST "${BASE_URL}/api/chat/sessions/${SESSION_ID}/messages" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Create a task named \"Provider Test Task\" that runs every 2 minutes with the prompt \"Report your provider and model\""
  }')

echo "Message sent. Waiting for task creation..."
sleep 3
echo

echo "3. Checking tasks in scheduler database..."
TASK_DATA=$(docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -t -A -c \
  "SELECT id, name, provider, model, chat_session_id FROM task_scheduler.scheduler_tasks WHERE name = 'Provider Test Task';")

if [ -z "$TASK_DATA" ]; then
  echo "ERROR: Task not found in database!"
  exit 1
fi

echo "Task found:"
echo "$TASK_DATA" | tr '|' '\n' | sed 's/^/  /'
echo

TASK_ID=$(echo "$TASK_DATA" | cut -d'|' -f1)
TASK_PROVIDER=$(echo "$TASK_DATA" | cut -d'|' -f3)
TASK_MODEL=$(echo "$TASK_DATA" | cut -d'|' -f4)
TASK_SESSION=$(echo "$TASK_DATA" | cut -d'|' -f5)

echo "4. Verifying inheritance..."
echo "  Session ID: $SESSION_ID"
echo "  Task Session ID: $TASK_SESSION"
echo "  Task Provider: $TASK_PROVIDER"
echo "  Task Model: $TASK_MODEL"
echo

if [ "$TASK_SESSION" = "$SESSION_ID" ]; then
  echo "✓ Task correctly linked to chat session"
else
  echo "✗ Task session ID mismatch!"
fi

if [ "$TASK_PROVIDER" = "anthropic" ]; then
  echo "✓ Provider correctly inherited: $TASK_PROVIDER"
else
  echo "✗ Provider NOT inherited (expected: anthropic, got: $TASK_PROVIDER)"
fi

if [ "$TASK_MODEL" = "claude-3-5-sonnet-20241022" ]; then
  echo "✓ Model correctly inherited: $TASK_MODEL"
else
  echo "✗ Model NOT inherited (expected: claude-3-5-sonnet-20241022, got: $TASK_MODEL)"
fi

echo
echo "=== Test Complete ==="
