# API Client Layer

This directory contains the API client layer for the MCP Compose Dashboard. All API communication with the backend is handled through these modules.

## Architecture

### Base Client (`client.js`)
- Provides a fetch wrapper with authentication, error handling, and interceptors
- Automatically adds `Authorization` header from localStorage
- Supports request/response interceptors
- Consistent error handling across all API calls
- Base URL configured for `/api/*` endpoints

### API Modules

#### Dashboard API (`dashboard.js`)
- **Server Management**: `getServers()`, `startServer()`, `stopServer()`, `restartServer()`
- **Status & Metrics**: `getStatus()`, `getConnections()`, `getMetrics()`
- **Proxy**: `reloadProxy()`
- **Server Info**: `getServerDocs()`, `getServerOpenAPI()`, `getServerLogs()`

#### Chat API (`chat.js`)
- **Sessions**: `getChatSessions()`, `createChatSession()`, `updateChatSession()`, `deleteChatSession()`
- **Messages**: `getChatMessages()`, `sendChatMessage()`
- **Configuration**: `getChatProviders()`, `getMCPServers()`, `getSystemPrompt()`
- **WebSocket**: `createChatWebSocket()` for streaming responses

#### Tasks API (`tasks.js`)
- **CRUD**: `getTasks()`, `getTask()`, `createTask()`, `updateTask()`, `deleteTask()`
- **Execution**: `executeTask()`, `enableTask()`, `disableTask()`
- **History**: `getTaskHistory()`, `getTaskOutput()`
- **Stats**: `getTaskStats()`, `getTaskSchedulerHealth()`

#### Memory API (`memory.js`)
- **Entities**: `getEntities()`, `createEntity()`, `updateEntity()`, `deleteEntity()`, `bulkDeleteEntities()`
- **Observations**: `getObservations()`, `addObservation()`, `deleteObservation()`
- **Relationships**: `getRelationships()`, `createRelationship()`, `deleteRelationship()`
- **Search**: `searchMemory()`, `getMemoryStats()`, `getEntityTypes()`

#### Activity API (`activity.js`)
- **History**: `getActivityHistory()` - Load historical events
- **Stats**: `getActivityStats()` - Activity statistics
- **Events**: `sendActivityEvent()` - Send activity events
- **WebSocket**: `createActivityWebSocket()` - Real-time activity stream

#### OAuth API (`oauth.js`)
- **Status**: `getOAuthStatus()` - OAuth server status
- **Clients**: `getOAuthClients()`, `createOAuthClient()`, `updateOAuthClient()`, `deleteOAuthClient()`
- **Scopes**: `getOAuthScopes()`, `getOAuthEndpoints()`
- **Testing**: `testAuthorizationCodeFlow()`, `testClientCredentialsFlow()`
- **Tokens**: `registerOAuthClient()`, `exchangeToken()`, `revokeToken()`

#### Audit API (`audit.js`)
- **Entries**: `getAuditEntries()` - Get audit logs with filtering
- **Stats**: `getAuditStats()`, `getEventDistribution()`
- **Export**: `exportAuditLogs()`, `downloadAuditLogs()` - CSV export
- **Event Types**: `getAuditEventTypes()`

#### Inspector API (`inspector.js`)
- **Connection**: `inspectorConnect()`, `inspectorDisconnect()`
- **Requests**: `inspectorRequest()` - Send MCP JSON-RPC 2.0 requests
- **Templates**: `getRequestTemplates()` - Pre-built MCP request templates
- **Validation**: `validateMCPRequest()` - Validate MCP requests

#### WebSocket Manager (`websocket.js`)
- **Connection Management**: Auto-reconnection with exponential backoff
- **Heartbeat**: Keep-alive mechanism
- **Events**: `on()`, `off()`, `emit()` - Event system
- **State**: `getState()`, `isConnected()` - Connection state
- **Factories**: `createMetricsWebSocket()`, `createLogsWebSocket()`

## Usage Examples

### Basic API Call
```javascript
import { dashboardApi } from '@/api';

// Get all servers
const servers = await dashboardApi.getServers();

// Start a server
await dashboardApi.startServer('my-server');
```

### Using the Base Client
```javascript
import apiClient from '@/api/client';

// Set authentication token
apiClient.setAuthToken('your-api-key');

// Add request interceptor
apiClient.addRequestInterceptor(async (config) => {
  console.log('Request:', config);
  return config;
});

// Add response interceptor
apiClient.addResponseInterceptor(async (response) => {
  console.log('Response:', response);
  return response;
});

// Make custom request
const data = await apiClient.get('/custom-endpoint');
```

### WebSocket with Auto-Reconnection
```javascript
import { createActivityWebSocket } from '@/api/websocket';

const ws = createActivityWebSocket();

ws.on('open', () => {
  console.log('Connected to activity stream');
});

ws.on('message', (event) => {
  const data = JSON.parse(event.data);
  console.log('Activity event:', data);
});

ws.on('error', (error) => {
  console.error('WebSocket error:', error);
});

ws.on('reconnect', ({ attempt }) => {
  console.log(`Reconnecting (attempt ${attempt})...`);
});

// Connect
ws.connect();

// Send message
ws.send({ type: 'subscribe', channels: ['activity'] });

// Disconnect when done
ws.disconnect();
```

### Chat Streaming
```javascript
import { chatApi } from '@/api';

// Create session
const session = await chatApi.createChatSession({
  name: 'My Chat',
  provider: 'openai',
  model: 'gpt-4',
  mcpServers: ['filesystem', 'memory'],
});

// Connect WebSocket for streaming
const ws = chatApi.createChatWebSocket(session.id);

ws.onopen = () => {
  // Send message
  ws.send(JSON.stringify({
    type: 'message',
    content: 'Hello, AI!',
  }));
};

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);

  if (data.type === 'token') {
    console.log('Token:', data.content);
  } else if (data.type === 'tool_call') {
    console.log('Tool call:', data.tool, data.params);
  } else if (data.type === 'done') {
    console.log('Response complete');
  }
};
```

### Memory Management
```javascript
import { memoryApi } from '@/api';

// Create entity
const entity = await memoryApi.createEntity({
  name: 'John Doe',
  type: 'person',
  content: {
    role: 'Developer',
    company: 'ACME Corp',
  },
});

// Add observation
await memoryApi.addObservation(entity.id, {
  content: 'Met at conference',
  type: 'observation',
});

// Create relationship
await memoryApi.createRelationship({
  fromEntityId: entity.id,
  toEntityId: otherEntity.id,
  type: 'knows',
  metadata: { since: '2024-01-01' },
});

// Search
const results = await memoryApi.searchMemory('developer');
```

### Error Handling
```javascript
import { dashboardApi } from '@/api';

try {
  await dashboardApi.startServer('my-server');
} catch (error) {
  if (error.status === 404) {
    console.error('Server not found');
  } else if (error.status === 401) {
    console.error('Unauthorized - check API key');
  } else {
    console.error('Error:', error.message);
    console.error('Details:', error.details);
  }
}
```

## Configuration

### Authentication
The API client automatically reads the authentication token from `localStorage`:

```javascript
// Token is stored with key 'mcp_api_key'
localStorage.setItem('mcp_api_key', 'your-api-key');

// Or use the client method
import apiClient from '@/api/client';
apiClient.setAuthToken('your-api-key');
```

### Base URL
The default base URL is `/api`, which works when the frontend is served from the same domain as the API. To use a different base URL:

```javascript
import { APIClient } from '@/api/client';

const customClient = new APIClient('https://api.example.com');
```

### WebSocket Configuration
WebSocket connections support several configuration options:

```javascript
import { createWebSocketManager } from '@/api/websocket';

const ws = createWebSocketManager('/ws/custom', {
  reconnectInterval: 3000,        // Initial reconnect delay (ms)
  maxReconnectAttempts: 10,       // Maximum reconnection attempts
  reconnectBackoff: 1.5,          // Backoff multiplier for reconnect delay
  heartbeatInterval: 30000,       // Heartbeat interval (ms)
  debug: true,                    // Enable debug logging
});
```

## Best Practices

1. **Always handle errors**: Wrap API calls in try-catch blocks
2. **Use TypeScript/JSDoc**: All functions have JSDoc comments for type safety
3. **Clean up WebSockets**: Always call `disconnect()` when done with WebSocket connections
4. **Use interceptors wisely**: Add logging or auth token refresh via interceptors
5. **Batch requests**: Use the batch endpoint if available instead of multiple individual requests
6. **Cache responses**: Consider caching frequently requested data in state management

## API Endpoint Reference

All endpoints are relative to `/api`:

| Module | Endpoint | Method | Description |
|--------|----------|--------|-------------|
| Dashboard | `/servers` | GET | Get all servers |
| Dashboard | `/status` | GET | Get dashboard status |
| Dashboard | `/servers/start` | POST | Start server |
| Dashboard | `/servers/stop` | POST | Stop server |
| Dashboard | `/servers/restart` | POST | Restart server |
| Chat | `/chat/sessions` | GET/POST | List/create sessions |
| Chat | `/chat/sessions/:id` | GET/PUT/DELETE | Get/update/delete session |
| Chat | `/chat/sessions/:id/messages` | GET/POST | Get/send messages |
| Tasks | `/task-scheduler/tasks` | GET/POST | List/create tasks |
| Tasks | `/task-scheduler/tasks/:id` | GET/PUT/DELETE | Get/update/delete task |
| Tasks | `/task-scheduler/tasks/:id/execute` | POST | Execute task |
| Memory | `/memory/entities` | GET/POST | List/create entities |
| Memory | `/memory/entities/:id` | GET/PUT/DELETE | Get/update/delete entity |
| Memory | `/memory/relationships` | POST | Create relationship |
| Activity | `/activity/history` | GET | Get activity history |
| Activity | `/activity/stats` | GET | Get activity stats |
| OAuth | `/oauth/clients` | GET/POST | List/create clients |
| OAuth | `/oauth/clients/:id` | GET/PUT/DELETE | Get/update/delete client |
| Audit | `/audit/entries` | GET | Get audit entries |
| Audit | `/audit/stats` | GET | Get audit statistics |
| Audit | `/audit/export` | GET | Export audit logs (CSV) |
| Inspector | `/inspector/connect` | POST | Connect to server |
| Inspector | `/inspector/request` | POST | Send MCP request |

## WebSocket Endpoints

| Endpoint | Description |
|----------|-------------|
| `/ws/chat/:sessionId` | Chat message streaming |
| `/ws/activity` | Real-time activity events |
| `/ws/logs` | Container log streaming |
| `/ws/metrics` | Real-time metrics updates |
