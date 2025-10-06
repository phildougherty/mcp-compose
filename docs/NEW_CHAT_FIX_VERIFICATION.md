# New Chat Connection Fix - Verification Guide

## Problem Fixed
When clicking "New Chat" button, sessions were created but WebSocket showed "Disconnected" status.

## Root Causes Fixed

### 1. Stale Closure Bug
**Location**: `Chat.jsx:174`
**Issue**: Used stale `sessions` array from component closure instead of current state
**Fix**: Use `useChatStore.getState().sessions` to get fresh state

### 2. Insufficient Logging
**Issue**: Hard to diagnose WebSocket connection failures
**Fix**: Added comprehensive logging throughout connection flow

## Files Modified

1. **src/components/Chat/Chat.jsx**
   - Fixed stale closure in `createNewSession()`
   - Added diagnostic logging for session creation flow

2. **src/hooks/useWebSocket.js**
   - Added logging for connection attempts
   - Added logging for connection errors
   - Added logging for close events with codes/reasons

## Verification Steps

### 1. Start the Application
```bash
./mcp-compose up
./mcp-compose proxy --port 9876
```

Access dashboard at http://localhost:3000 (or configured port)

### 2. Open Browser Console
Press F12 or right-click → Inspect → Console tab

### 3. Click "New Chat" Button

### 4. Expected Console Output
You should see logs in this sequence:

```
[Chat] Creating new session...
[Chat] New session created: <uuid>
[Chat] Session added to store, now loading session
[Chat] loadSession called for: <uuid>
[Chat] activeSessionId changed: <uuid> wsUrl: ws://localhost:3000/ws/chat/<uuid>
[useWebSocket] Effect triggered - url: ws://localhost:3000/ws/chat/<uuid> autoConnect: true
[useWebSocket] Closing existing connection (if any)
[useWebSocket] Attempting to connect to: ws://localhost:3000/ws/chat/<uuid>
[useWebSocket] Creating WebSocket connection to: ws://localhost:3000/ws/chat/<uuid>
[Chat] Session data loaded, messages: 0
[Chat] setActiveSession called with: <uuid>
[useWebSocket] WebSocket opened successfully
[Chat] Session loaded and activated
```

### 5. Expected UI Behavior
- Connection status indicator shows **"Connected"** (green)
- New session appears in session list
- Chat input is enabled
- Can send messages immediately

### 6. Error Case Testing

If you see "Disconnected" status, check console for errors:

**Scenario A: Backend not running**
```
[useWebSocket] WebSocket error: ...
[useWebSocket] WebSocket closed - code: 1006 reason:
```
**Solution**: Start mcp-compose backend

**Scenario B: Session creation fails**
```
[Chat] Failed to create session: <error message>
```
**Solution**: Check backend logs for database/service errors

**Scenario C: Session exists but WebSocket fails**
```
[Chat] Session loaded and activated
[useWebSocket] WebSocket closed - code: 1011 reason: internal error
```
**Solution**: Check backend chat service logs

## Technical Details

### State Management Flow

1. **Create Session** → API call creates session in database
2. **Add to Store** → Session added to Zustand store (FIXED: now uses fresh state)
3. **Load Session** → Fetches full session data including messages
4. **Set Active** → Updates `activeSessionId` in store
5. **WebSocket URL Update** → `useMemo` recalculates WebSocket URL
6. **WebSocket Connect** → `useWebSocket` effect detects URL change and connects

### Why Stale Closure Was a Problem

React functional components capture variables in closures. When `createNewSession` was defined, it captured the `sessions` array. If sessions changed after component render, the captured value was outdated.

**Before (buggy)**:
```javascript
const createNewSession = async () => {
  // 'sessions' is captured from component scope - might be stale
  setSessions([session, ...sessions]);
}
```

**After (fixed)**:
```javascript
const createNewSession = async () => {
  // Get fresh state directly from store
  const currentSessions = useChatStore.getState().sessions;
  setSessions([session, ...currentSessions]);
}
```

### WebSocket Connection Lifecycle

The `useWebSocket` hook manages connections through a `useEffect` that depends on `[url, autoConnect]`:

1. When `url` changes (from `null` to `ws://...`), effect runs
2. Existing connection is closed (if any)
3. State is reset (`isConnected = false`)
4. If `autoConnect && url`, new connection is created
5. On successful connection, `isConnected = true`
6. On error/close, attempts reconnection (up to 5 times by default)

## Monitoring Tips

### Browser Console Filters

To see only relevant logs:
- Filter by `[Chat]` - see chat-specific flow
- Filter by `[useWebSocket]` - see WebSocket connection details
- Filter by level `Error` - see only problems

### Network Tab

1. Open Network tab in DevTools
2. Filter by `WS` (WebSocket)
3. Click on the WebSocket connection
4. View frames being sent/received
5. Check status codes:
   - 101 = Successfully upgraded to WebSocket
   - 400 = Bad request (session ID missing)
   - 500 = Server error

### Redux DevTools

The Zustand store is integrated with Redux DevTools (in development mode):

1. Install Redux DevTools extension
2. Open Redux DevTools tab
3. Watch actions:
   - `setSessions` - session list updated
   - `setActiveSession` - active session changed
   - `setConnected` - WebSocket connection status
   - `setMessages` - messages loaded

## Known Limitations

1. **No offline support**: If network is down, WebSocket will fail
2. **No session validation**: Frontend assumes session exists after creation
3. **Single reconnect strategy**: Uses exponential backoff, max 5 attempts
4. **No connection pooling**: Each session gets its own WebSocket

## Future Improvements

1. **Add session existence check** before connecting WebSocket
2. **Implement optimistic updates** - show "Connecting..." state immediately
3. **Add connection timeout** - detect hung connections
4. **Add ping/pong heartbeat** - keep connection alive (already implemented in backend)
5. **Add retry with exponential backoff** - more resilient reconnection
