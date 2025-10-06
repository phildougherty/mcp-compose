# Chat Components

React-based chat interface for MCP-Compose Dashboard with multi-session management, AI provider selection, and real-time streaming via WebSocket.

## Components Overview

### Chat.jsx (Main Component)
Main chat interface component that orchestrates all child components and manages WebSocket connections.

**Features:**
- WebSocket integration for real-time message streaming
- Session management (load, create, switch)
- System prompt viewer with collapsible panel
- Error handling with dismissible notifications
- Hamburger menu for mobile navigation
- Dark mode support

**State Management:**
- Uses `useChatStore` from Zustand for global state
- Uses `useWebSocket` hook for WebSocket connection
- Local state for UI elements (sidebar, system prompt, errors)

**Dependencies:**
- All child components (SessionList, MessageList, ChatInput, etc.)
- chatApi for API calls
- useWebSocket hook for real-time streaming
- Shared components (Button, Modal)

### SessionList.jsx
Sidebar component displaying all chat sessions with create/delete/rename functionality.

**Features:**
- Session list with active session highlighting
- Create new session button
- Rename session (prompt dialog)
- Delete session (with confirmation)
- Collapsible sidebar (desktop only)
- Responsive mobile view with backdrop

**Props:**
- `sidebarOpen` - Boolean for mobile sidebar visibility
- `sidebarCollapsed` - Boolean for sidebar collapsed state
- `onToggleCollapse` - Callback to toggle sidebar collapse
- `onClose` - Callback to close mobile sidebar
- `onCreateNew` - Callback to create new session
- `onLoadSession` - Callback to load a session

### MessageList.jsx
Container component for displaying message history with scrolling and empty states.

**Features:**
- Auto-scroll to bottom on new messages
- Empty state with suggestion chips
- Streaming message display
- Scrollable message container

**Props:**
- `messages` - Array of message objects
- `streamingContent` - Current streaming message content
- `isStreaming` - Boolean indicating streaming state

### ChatInput.jsx
Message input component with auto-resize textarea and send functionality.

**Features:**
- Auto-resizing textarea (48px min-height, 200px max-height)
- Keyboard shortcuts (Enter to send, Shift+Enter for new line)
- Disabled state during streaming
- Visual loading indicator when sending

**Props:**
- `onSend` - Callback to send message
- `disabled` - Boolean to disable input

### Message.jsx
Individual message component with markdown rendering and tool call visualization.

**Features:**
- Markdown rendering with `react-markdown`
- Code block syntax highlighting with `react-syntax-highlighter` (vscDarkPlus theme)
- User vs. Assistant message styling
- Token count and cost display
- Tool call expansion
- Timestamp formatting

**Props:**
- `message` - Message object with content, role, timestamps, tool calls
- `isStreaming` - Boolean indicating if message is currently streaming

**Markdown Support:**
- Inline code with backticks
- Code blocks with syntax highlighting
- Lists (ordered and unordered)
- Links with external opening
- Paragraphs with proper spacing
- Bold and italic text

### ToolCall.jsx
Expandable component for displaying tool execution details.

**Features:**
- Collapsible accordion view
- Tool call parameters display (JSON formatted)
- Tool execution results with success/error states
- Duration display for each tool call
- Visual status indicators (✓ success, ✗ error, ○ pending)
- Individual result expansion

**Props:**
- `toolCalls` - Array of tool call objects
- `toolResults` - Array of tool result objects

### ModelSelector.jsx
Dropdown component for selecting AI provider and model.

**Features:**
- Provider selection (OpenAI, Anthropic, Ollama, OpenRouter)
- Model selection based on provider
- Auto-update session config on change
- Loading state while fetching providers
- Responsive dropdown widths

**Dependencies:**
- chatApi for provider/model data
- useChatStore for session management

### MCPServerSelector.jsx
Dropdown component for selecting MCP servers to enable for the chat session.

**Features:**
- Dropdown panel with server list
- Checkbox selection for each server
- Bulk actions (Select All, Deselect All)
- Server tool count display
- Click-outside to close
- Loading state while fetching servers

**Props:**
- `sessionId` - Current session ID

**Dependencies:**
- chatApi for MCP server data
- useChatStore for session management

### ConnectionStatus.jsx
Simple indicator component showing WebSocket connection status.

**Features:**
- Visual indicator (green for connected, gray for disconnected)
- Icon display with checkmark or circle
- Status text (hidden on mobile)
- Tooltip with status

**Dependencies:**
- useChatStore for connection state

## Usage Example

```jsx
import { Chat } from './components/Chat';

function App() {
  return (
    <div className="h-screen">
      <Chat />
    </div>
  );
}
```

## State Management

The Chat components use Zustand for global state management via `chatStore`:

**State:**
- `sessions` - Array of all chat sessions
- `activeSessionId` - Currently active session ID
- `messages` - Object mapping session IDs to message arrays
- `streaming` - Streaming state (isStreaming, currentContent, pendingToolCalls)
- `isConnected` - WebSocket connection status
- `availableProviders` - Array of available AI providers
- `availableModels` - Object mapping providers to model arrays

**Actions:**
- `setSessions`, `addSession`, `removeSession`, `updateSession`
- `setActiveSession`, `setMessages`, `addMessage`, `removeMessage`
- `startStreaming`, `updateStreamingContent`, `stopStreaming`
- `setConnected`, `setLoading`, `setError`

## API Integration

The Chat components use the `chatApi` client for all API calls:

**Endpoints:**
- `GET /api/chat/sessions` - Get all sessions
- `POST /api/chat/sessions` - Create new session
- `GET /api/chat/sessions/:id` - Get session details
- `PATCH /api/chat/sessions/:id` - Update session
- `DELETE /api/chat/sessions/:id` - Delete session
- `GET /api/chat/providers` - Get available providers
- `GET /api/chat/mcp-servers` - Get available MCP servers
- `GET /api/chat/sessions/:id/system-prompt` - Get system prompt
- `WS /ws/chat/:id` - WebSocket for message streaming

## WebSocket Protocol

**Client → Server:**
```json
{
  "type": "message",
  "message": "User message content"
}
```

**Server → Client (Streaming Chunk):**
```json
{
  "type": "chunk",
  "content": "Partial response content",
  "tool_calls": [...],
  "tool_results": [...],
  "done": false
}
```

**Server → Client (Final Chunk):**
```json
{
  "type": "chunk",
  "content": "Final response content",
  "tool_calls": [...],
  "tool_results": [...],
  "tokens_used": 150,
  "cost_estimate": 0.0025,
  "message_id": "msg_12345",
  "done": true
}
```

**Server → Client (Error):**
```json
{
  "type": "error",
  "error": "Error message"
}
```

## Mobile-First Design

All Chat components follow mobile-first design principles:

**Breakpoints:**
- `xs`: 320px (extra small phones)
- `sm`: 640px (small phones)
- `md`: 768px (tablets) - Sidebar becomes collapsible
- `lg`: 1024px (laptops)
- `xl`: 1280px (desktops)

**Touch Targets:**
- All interactive elements: 44×44px minimum
- Primary buttons: 48×48px minimum
- Input fields: 48px height minimum

**Mobile Features:**
- Hamburger menu for navigation
- Collapsible sidebar with backdrop
- Auto-resizing textarea with proper touch handling
- Responsive dropdown panels
- Mobile-optimized spacing and typography

## Dark Mode

All components support dark mode using Tailwind's `dark:` variant:

**Theme Classes:**
- Background: `bg-white dark:bg-gray-900`
- Text: `text-gray-900 dark:text-white`
- Borders: `border-gray-200 dark:border-gray-700`
- Hover states: `hover:bg-gray-100 dark:hover:bg-gray-800`

Theme preference is managed by the `uiStore` and persisted to localStorage.

## Accessibility

All components meet WCAG 2.1 Level AA standards:

- All interactive elements are keyboard accessible
- All touch targets meet 44×44px minimum
- Color contrast meets 4.5:1 minimum
- Proper ARIA labels and roles
- Focus indicators on all interactive elements
- Semantic HTML structure
- Screen reader support

## Performance Considerations

**Optimization Techniques:**
- React.memo for message components (future enhancement)
- Virtualized list for large message histories (future enhancement)
- Debounced API calls for search/filter
- Code splitting for markdown renderer
- Optimized Zustand selectors to prevent unnecessary re-renders

**Bundle Size:**
- react-markdown: ~50KB gzipped
- react-syntax-highlighter: ~30KB gzipped (vscDarkPlus theme only)
- Total Chat components: ~15KB gzipped

## Testing

**Unit Tests (Future):**
- Component rendering tests
- User interaction tests
- WebSocket message handling tests
- State management tests

**Integration Tests (Future):**
- End-to-end chat flow
- Session management flow
- Tool call visualization flow

## Future Enhancements

**Planned Features:**
- Voice input support
- Message search within session
- Message export (markdown, PDF)
- Image attachments
- File attachments
- Message reactions
- Message editing
- Message deletion
- Conversation branching
- Conversation sharing
- Conversation templates

**Performance Improvements:**
- Virtualized message list for large histories
- Message pagination/infinite scroll
- Debounced streaming updates
- Optimistic UI updates

**Accessibility Improvements:**
- Keyboard shortcuts documentation
- Screen reader announcements for streaming
- High contrast mode support
- Reduced motion support

## Dependencies

**Required Packages:**
- `react` ^18.2.0
- `react-dom` ^18.2.0
- `zustand` ^4.4.0
- `clsx` ^2.0.0
- `react-markdown` ^10.1.0
- `react-syntax-highlighter` ^15.6.6

**Internal Dependencies:**
- `../../store/chatStore` - Zustand store
- `../../api/chat` - API client
- `../../hooks/useWebSocket` - WebSocket hook
- `../shared` - Shared UI components

## License

Part of the MCP-Compose project. See main project LICENSE for details.
