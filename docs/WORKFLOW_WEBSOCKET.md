# Workflow Execution WebSocket

Real-time workflow execution updates via WebSocket.

## Connection

Connect to the WebSocket endpoint for a specific workflow execution:

```
ws://localhost:3001/ws/workflows/{workflowId}/executions/{executionId}
```

### Authentication

If the dashboard requires authentication, pass the API key as a query parameter or header:

```javascript
const ws = new WebSocket(
  'ws://localhost:3001/ws/workflows/abc-123/executions/exec-456?api_key=YOUR_API_KEY'
);
```

Or use the Authorization header:

```javascript
const ws = new WebSocket('ws://localhost:3001/ws/workflows/abc-123/executions/exec-456');
ws.addEventListener('open', () => {
  // Note: WebSocket API doesn't support custom headers directly
  // Authentication is handled during the HTTP upgrade handshake
});
```

## Message Types

The WebSocket streams JSON messages with the following types:

### 1. Execution Started

Sent when workflow execution begins:

```json
{
  "type": "execution_started",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "status": "running",
  "timestamp": "2025-10-07T12:00:00.123456789Z"
}
```

### 2. Execution State

Initial state sent upon connection:

```json
{
  "type": "execution_state",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "status": "running",
  "timestamp": "2025-10-07T12:00:01.123456789Z"
}
```

### 3. Node Started

Sent when a node begins execution:

```json
{
  "type": "node_started",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "node_id": "node-1",
  "status": "running",
  "timestamp": "2025-10-07T12:00:01.123456789Z"
}
```

### 4. Node Completed

Sent when a node successfully completes:

```json
{
  "type": "node_completed",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "node_id": "node-1",
  "output": {
    "result": "Task completed successfully",
    "data": { ... }
  },
  "timestamp": "2025-10-07T12:00:05.123456789Z"
}
```

### 5. Node Error

Sent when a node fails:

```json
{
  "type": "node_error",
  "execution_id": "exec-456",
  "workflow_id": "abc-123",
  "node_id": "node-1",
  "error": "Failed to execute node: connection timeout",
  "timestamp": "2025-10-07T12:00:05.123456789Z"
}
```

### 6. Execution Completed

Sent when the entire workflow execution finishes:

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

Failed execution:

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

## JavaScript Client Example

```javascript
class WorkflowExecutionMonitor {
  constructor(workflowId, executionId) {
    this.workflowId = workflowId;
    this.executionId = executionId;
    this.ws = null;
  }

  connect() {
    const url = `ws://localhost:3001/ws/workflows/${this.workflowId}/executions/${this.executionId}`;
    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
    };

    this.ws.onmessage = (event) => {
      const message = JSON.parse(event.data);
      this.handleMessage(message);
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('WebSocket closed');
    };
  }

  handleMessage(message) {
    switch (message.type) {
      case 'execution_started':
        console.log('Execution started:', message.execution_id);
        break;

      case 'execution_state':
        console.log('Initial state:', message.status);
        break;

      case 'node_started':
        console.log(`Node ${message.node_id} started`);
        break;

      case 'node_completed':
        console.log(`Node ${message.node_id} completed:`, message.output);
        break;

      case 'node_error':
        console.error(`Node ${message.node_id} failed:`, message.error);
        break;

      case 'execution_completed':
        if (message.status === 'completed') {
          console.log(`Execution completed in ${message.duration}ms`);
        } else {
          console.error(`Execution failed: ${message.error}`);
        }
        this.disconnect();
        break;

      default:
        console.log('Unknown message type:', message.type);
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }
}

// Usage
const monitor = new WorkflowExecutionMonitor('abc-123', 'exec-456');
monitor.connect();
```

## React/Frontend Integration

```typescript
import { useEffect, useState } from 'react';

interface ExecutionUpdate {
  type: string;
  execution_id: string;
  workflow_id: string;
  node_id?: string;
  status?: string;
  output?: any;
  error?: string;
  duration?: number;
  timestamp: string;
}

export function useWorkflowExecution(workflowId: string, executionId: string) {
  const [updates, setUpdates] = useState<ExecutionUpdate[]>([]);
  const [status, setStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const url = `ws://localhost:3001/ws/workflows/${workflowId}/executions/${executionId}`;
    const ws = new WebSocket(url);

    ws.onopen = () => {
      setStatus('connected');
    };

    ws.onmessage = (event) => {
      const update = JSON.parse(event.data) as ExecutionUpdate;
      setUpdates((prev) => [...prev, update]);

      if (update.type === 'execution_completed') {
        ws.close();
      }
    };

    ws.onerror = (err) => {
      setError('WebSocket error occurred');
      console.error('WebSocket error:', err);
    };

    ws.onclose = () => {
      setStatus('disconnected');
    };

    return () => {
      if (ws.readyState === WebSocket.OPEN) {
        ws.close();
      }
    };
  }, [workflowId, executionId]);

  return { updates, status, error };
}

// Component usage
export function WorkflowExecutionView({ workflowId, executionId }) {
  const { updates, status, error } = useWorkflowExecution(workflowId, executionId);

  return (
    <div>
      <div>Status: {status}</div>
      {error && <div>Error: {error}</div>}

      <div>
        {updates.map((update, i) => (
          <div key={i}>
            <strong>{update.type}</strong> - {update.timestamp}
            {update.node_id && <span> (Node: {update.node_id})</span>}
            {update.error && <div>Error: {update.error}</div>}
          </div>
        ))}
      </div>
    </div>
  );
}
```

## Connection Lifecycle

1. Client connects to WebSocket endpoint
2. Server validates execution ID and workflow ID
3. Server sends initial execution state
4. Server broadcasts updates as execution progresses
5. Connection closes when execution completes or fails

## Health Monitoring

The WebSocket includes automatic ping/pong heartbeat:

- Server sends ping every 30 seconds
- Client must respond with pong
- Connection closes if pong not received within 10 seconds

## Error Handling

If the execution or workflow is not found:

```json
{
  "error": "Execution not found"
}
```

Then the connection is closed immediately.

## Multiple Clients

Multiple clients can connect to the same execution simultaneously. All connected clients receive the same updates in real-time.

## Best Practices

1. Always handle the `onclose` event to clean up resources
2. Implement reconnection logic for network failures
3. Display connection status to users
4. Store updates locally for offline viewing
5. Use the `execution_completed` message to know when to close the connection
