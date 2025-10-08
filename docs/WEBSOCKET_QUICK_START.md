# WebSocket Quick Start Guide

## Connect to Workflow Execution

```javascript
const ws = new WebSocket(
  `ws://localhost:3001/ws/workflows/${workflowId}/executions/${executionId}`
);

ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  console.log(update.type, update);
};
```

## Message Types You'll Receive

| Type | Description | Key Fields |
|------|-------------|------------|
| `execution_state` | Initial state on connect | `status` |
| `execution_started` | Workflow begins | `status: "running"` |
| `node_started` | Node begins processing | `node_id`, `status: "running"` |
| `node_completed` | Node finishes successfully | `node_id`, `output` |
| `node_error` | Node fails | `node_id`, `error` |
| `execution_completed` | Workflow finishes | `status: "completed"/"failed"`, `duration` |

## Complete Flow Example

### 1. Start Workflow Execution

```bash
curl -X POST http://localhost:3001/api/workflows/abc-123/execute
```

Response:
```json
{
  "id": "exec-456",
  "workflow_id": "abc-123",
  "status": "running"
}
```

### 2. Connect WebSocket

```javascript
const workflowId = 'abc-123';
const executionId = 'exec-456';
const ws = new WebSocket(
  `ws://localhost:3001/ws/workflows/${workflowId}/executions/${executionId}`
);
```

### 3. Handle Messages

```javascript
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  switch (msg.type) {
    case 'execution_state':
      console.log('Current status:', msg.status);
      break;

    case 'node_started':
      console.log(`Starting node: ${msg.node_id}`);
      break;

    case 'node_completed':
      console.log(`Node ${msg.node_id} done:`, msg.output);
      break;

    case 'node_error':
      console.error(`Node ${msg.node_id} failed:`, msg.error);
      break;

    case 'execution_completed':
      console.log(`Workflow ${msg.status} in ${msg.duration}ms`);
      ws.close();
      break;
  }
};
```

## React Hook

```typescript
import { useEffect, useState } from 'react';

function useWorkflowExecution(workflowId: string, executionId: string) {
  const [updates, setUpdates] = useState([]);

  useEffect(() => {
    const ws = new WebSocket(
      `ws://localhost:3001/ws/workflows/${workflowId}/executions/${executionId}`
    );

    ws.onmessage = (event) => {
      const update = JSON.parse(event.data);
      setUpdates(prev => [...prev, update]);

      if (update.type === 'execution_completed') {
        ws.close();
      }
    };

    return () => ws.close();
  }, [workflowId, executionId]);

  return updates;
}
```

## Testing with wscat

```bash
# Install
npm install -g wscat

# Connect
wscat -c ws://localhost:3001/ws/workflows/abc-123/executions/exec-456

# You'll see messages like:
< {"type":"execution_state","execution_id":"exec-456","workflow_id":"abc-123","status":"running","timestamp":"2025-10-07T12:00:00Z"}
< {"type":"node_started","execution_id":"exec-456","workflow_id":"abc-123","node_id":"node-1","status":"running","timestamp":"2025-10-07T12:00:01Z"}
```

## Common Patterns

### Progress Tracking

```javascript
let completedNodes = 0;
let totalNodes = 0;

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  if (msg.type === 'execution_started') {
    // Fetch workflow to get total nodes
  }

  if (msg.type === 'node_completed') {
    completedNodes++;
    console.log(`Progress: ${completedNodes}/${totalNodes}`);
  }
};
```

### Error Handling

```javascript
ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = (event) => {
  if (!event.wasClean) {
    console.error('Connection died unexpectedly');
  }
};
```

### Reconnection

```javascript
function connectWithRetry(workflowId, executionId, maxRetries = 3) {
  let retries = 0;

  function connect() {
    const ws = new WebSocket(
      `ws://localhost:3001/ws/workflows/${workflowId}/executions/${executionId}`
    );

    ws.onclose = () => {
      if (retries < maxRetries) {
        retries++;
        setTimeout(connect, 1000 * retries);
      }
    };

    return ws;
  }

  return connect();
}
```

## Message Examples

### Execution Started
```json
{
  "type": "execution_started",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "status": "running",
  "timestamp": "2025-10-07T12:00:00.123456789Z"
}
```

### Node Completed
```json
{
  "type": "node_completed",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "node_id": "node-1",
  "output": {
    "result": "Task completed",
    "data": { "items": 42 }
  },
  "timestamp": "2025-10-07T12:00:05.123456789Z"
}
```

### Execution Completed (Success)
```json
{
  "type": "execution_completed",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "status": "completed",
  "duration": 5000,
  "timestamp": "2025-10-07T12:00:10.123456789Z"
}
```

### Execution Completed (Failed)
```json
{
  "type": "execution_completed",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "status": "failed",
  "error": "Node node-1 failed: connection timeout",
  "duration": 3000,
  "timestamp": "2025-10-07T12:00:08.123456789Z"
}
```

## Tips

1. Always close WebSocket when execution completes
2. Handle reconnection for network failures
3. Display connection status to users
4. Store updates locally for offline viewing
5. Use `execution_completed` to know when to stop
6. Heartbeat is automatic (server pings every 30s)

## See Also

- [Full Documentation](/docs/WORKFLOW_WEBSOCKET.md) - Complete integration guide
- [Implementation Details](/docs/WEBSOCKET_IMPLEMENTATION_SUMMARY.md) - Architecture and design
