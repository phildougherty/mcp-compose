#!/bin/bash

set -e

echo "=== Testing Complete Task-to-Chat Flow ==="

# Step 1: Create a chat session
echo -e "\n1. Creating chat session..."
SESSION_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "INSERT INTO chat_sessions (id, title, provider, model, created_at)
   VALUES ('$SESSION_ID', 'Test Task Output Session', 'openrouter', 'anthropic/claude-3.5-sonnet', NOW());"

echo "   Created session: $SESSION_ID"

# Step 2: Create a task linked to this session
echo -e "\n2. Creating task linked to session..."
TASK_RESPONSE=$(curl -s -X POST http://localhost:9876/task-scheduler \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer myapikey" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"id\": 1,
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"add_task\",
      \"arguments\": {
        \"name\": \"test-output-to-chat-$(date +%s)\",
        \"description\": \"Test task that posts output to chat\",
        \"type\": \"shell\",
        \"command\": \"echo Task executed successfully at \$(date) && echo Session: $SESSION_ID\",
        \"schedule\": \"*/30 * * * *\",
        \"enabled\": true,
        \"_chat_session_id\": \"$SESSION_ID\",
        \"_output_to_chat\": true
      }
    }
  }")

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.result.content[0].text' | jq -r '.id')
echo "   Created task: $TASK_ID"

# Step 3: Verify task in database
echo -e "\n3. Verifying task in database..."
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, name, output_to_chat, chat_session_id FROM task_scheduler.scheduler_tasks WHERE id = '$TASK_ID';"

# Step 4: Trigger task to run
echo -e "\n4. Triggering task to run now..."
curl -s -X POST http://localhost:9876/task-scheduler \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer myapikey" \
  -d "{
    \"jsonrpc\": \"2.0\",
    \"id\": 1,
    \"method\": \"tools/call\",
    \"params\": {
      \"name\": \"run_task\",
      \"arguments\": {
        \"task_id\": \"$TASK_ID\"
      }
    }
  }" | jq -r '.result.content[0].text'

# Step 5: Wait and check
echo -e "\n5. Waiting 5 seconds for execution..."
sleep 5

echo -e "\n6. Checking task run results..."
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, status, posted_to_chat, LEFT(output, 80) FROM task_scheduler.scheduler_task_runs WHERE task_id = '$TASK_ID' ORDER BY started_at DESC LIMIT 1;"

echo -e "\n7. Checking for automated messages in chat..."
docker exec mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT role, is_automated, LEFT(content, 80) as preview FROM chat_messages WHERE session_id = '$SESSION_ID' ORDER BY created_at DESC LIMIT 3;"

echo -e "\n=== Summary ===\n"
echo "Session ID: $SESSION_ID"
echo "Task ID: $TASK_ID"
