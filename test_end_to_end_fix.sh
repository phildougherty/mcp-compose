#!/bin/bash

set -e

echo "=== End-to-End Test: Task Creation → Execution → Chat Output ==="

# Step 1: Create a real chat session
echo -e "\n[1/6] Creating chat session..."
SESSION_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
docker exec -i mcp-compose-postgres-memory psql -U postgres -d mcp_compose <<EOF
INSERT INTO chat_sessions (id, title, provider, model, created_at)
VALUES ('$SESSION_ID', 'E2E Test Session', 'openrouter', 'anthropic/claude-3.5-sonnet', NOW());
EOF
echo "   ✓ Session created: $SESSION_ID"

# Step 2: Create a task with chat context
echo -e "\n[2/6] Creating task with chat context..."
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
        \"name\": \"e2e-test-task\",
        \"description\": \"End-to-end test of chat output\",
        \"type\": \"shell\",
        \"command\": \"echo \\\"✓ Task executed at \$(date)\\\" && echo \\\"Session: $SESSION_ID\\\"\",
        \"schedule\": \"* * * * *\",
        \"enabled\": true,
        \"_chat_session_id\": \"$SESSION_ID\",
        \"_output_to_chat\": true,
        \"_provider\": \"openrouter\",
        \"_model\": \"anthropic/claude-3.5-sonnet\"
      }
    }
  }")

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.result.content[0].text' | jq -r '.id')
echo "   ✓ Task created: $TASK_ID"

# Step 3: Verify task in database
echo -e "\n[3/6] Verifying task in PostgreSQL..."
docker exec -i mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, name, output_to_chat, chat_session_id FROM task_scheduler.scheduler_tasks WHERE id = '$TASK_ID';"

# Step 4: Check debug logs for OutputToChat
echo -e "\n[4/6] Checking debug logs..."
docker logs mcp-compose-task-scheduler 2>&1 | grep -A 2 "createBaseTask.*$TASK_ID" | tail -5
docker logs mcp-compose-task-scheduler 2>&1 | grep -A 2 "applyChatContext.*$TASK_ID" | tail -5

# Step 5: Manually trigger task execution
echo -e "\n[5/6] Triggering task execution..."
RUN_RESULT=$(curl -s -X POST http://localhost:9876/task-scheduler \
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
  }")

echo "$RUN_RESULT" | jq -r '.result.content[0].text' | jq '.success, .status, .output'

# Step 6: Wait and check for posted output
echo -e "\n[6/6] Checking for automated chat message..."
sleep 3

# Check task run
echo -e "\nTask Run Record:"
docker exec -i mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, status, posted_to_chat, LEFT(output, 80) FROM task_scheduler.scheduler_task_runs WHERE task_id = '$TASK_ID' ORDER BY started_at DESC LIMIT 1;"

# Check chat message
echo -e "\nChat Message:"
docker exec -i mcp-compose-postgres-memory psql -U postgres -d mcp_compose -c \
  "SELECT id, role, is_automated, LEFT(content, 80) FROM chat_messages WHERE session_id = '$SESSION_ID' ORDER BY created_at DESC LIMIT 1;"

# Check task scheduler logs for posting attempt
echo -e "\nTask Scheduler Logs (posting to chat):"
docker logs mcp-compose-task-scheduler 2>&1 | grep -A 5 "postTaskResultToChat.*$TASK_ID" | tail -10

# Final summary
echo -e "\n=== Test Summary ==="
echo "Session ID: $SESSION_ID"
echo "Task ID: $TASK_ID"
echo ""
echo "Expected Results:"
echo "  ✓ Task should have output_to_chat = t in database"
echo "  ✓ Task should execute successfully"
echo "  ✓ Task run should show posted_to_chat = t"
echo "  ✓ Chat message should exist with is_automated = t"
echo ""
echo "View in dashboard: http://localhost:3111"
