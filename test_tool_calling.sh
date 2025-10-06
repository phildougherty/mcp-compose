#!/bin/bash

# Test MCP tool calling through the dashboard

DASHBOARD_URL="http://localhost:3001"

echo "=== Testing MCP Tool Calling ==="
echo ""

# Step 1: Create a chat session
echo "1. Creating chat session..."
SESSION_RESPONSE=$(curl -s -X POST "$DASHBOARD_URL/api/chat/sessions" \
  -H "Content-Type: application/json" \
  -d '{"provider":"openrouter","model":"anthropic/claude-3.5-sonnet","user_id":"test-user"}')

SESSION_ID=$(echo "$SESSION_RESPONSE" | jq -r '.id')
echo "Session ID: $SESSION_ID"
echo ""

# Step 2: Add dexcom server to session
echo "2. Adding dexcom server to session..."
curl -s -X POST "$DASHBOARD_URL/api/chat/sessions/$SESSION_ID/servers" \
  -H "Content-Type: application/json" \
  -d '{"server_name":"dexcom"}' | jq '.'
echo ""

# Step 3: Send a message that should trigger tool use
echo "3. Sending message to trigger dexcom tool..."
echo "   Message: 'What is my current glucose reading?'"
echo ""
echo "   Response:"
curl -s -X POST "$DASHBOARD_URL/api/chat/sessions/$SESSION_ID/messages" \
  -H "Content-Type: application/json" \
  -d '{"message":"What is my current glucose reading?"}' | jq '.'

echo ""
echo "=== Test complete ==="
echo ""
echo "Check dashboard logs for tool execution:"
echo "docker logs mcp-compose-dashboard 2>&1 | grep -E 'executeToolCallByName|executeMCPTool|Tool call'"
