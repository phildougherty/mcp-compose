import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

/**
 * @typedef {Object} ChatMessage
 * @property {string} id - Unique message identifier
 * @property {string} sessionId - Session this message belongs to
 * @property {string} role - Message role (user, assistant, system, tool)
 * @property {string} content - Message content
 * @property {number} timestamp - Message timestamp
 * @property {Object} [metadata] - Additional message metadata
 * @property {ToolCall[]} [toolCalls] - Tool calls made during this message
 * @property {string} [model] - Model used for this message
 * @property {number} [tokens] - Token count for this message
 * @property {boolean} [is_automated] - Whether message is from a scheduled task/agent
 * @property {string} [from_task_run_id] - Task run ID if message is automated
 */

/**
 * @typedef {Object} ToolCall
 * @property {string} id - Tool call identifier
 * @property {string} name - Tool name
 * @property {Object} parameters - Tool parameters
 * @property {Object} [result] - Tool execution result
 * @property {string} status - Tool call status (pending, success, error)
 * @property {number} timestamp - Tool call timestamp
 * @property {number} [duration] - Execution duration in ms
 */

/**
 * @typedef {Object} ChatSession
 * @property {string} id - Unique session identifier
 * @property {string} name - Session name
 * @property {string} provider - AI provider (openai, anthropic, ollama, openrouter)
 * @property {string} model - Model identifier
 * @property {string[]} mcpServers - Connected MCP server IDs
 * @property {number} createdAt - Creation timestamp
 * @property {number} updatedAt - Last update timestamp
 * @property {number} messageCount - Number of messages in session
 * @property {Object} [metadata] - Additional session metadata
 * @property {number} [unread_message_count] - Count of unread messages
 * @property {boolean} [has_active_agents] - Whether session has active scheduled agents/tasks
 */

/**
 * @typedef {Object} StreamingState
 * @property {boolean} isStreaming - Whether currently streaming
 * @property {string} currentContent - Current streaming content
 * @property {string|null} sessionId - Session being streamed to
 * @property {ToolCall[]} pendingToolCalls - Tool calls in progress
 */

/**
 * @typedef {Object} ChatState
 * @property {ChatSession[]} sessions - All chat sessions
 * @property {Record<string, ChatMessage[]>} messages - Messages by session ID
 * @property {string|null} activeSessionId - Currently active session ID
 * @property {StreamingState} streaming - Streaming state
 * @property {boolean} isLoading - Loading state
 * @property {string|null} error - Error message if any
 * @property {boolean} isConnected - WebSocket connection status
 * @property {string[]} availableProviders - Available AI providers
 * @property {Record<string, string[]>} availableModels - Available models by provider
 */

/**
 * @typedef {Object} ChatActions
 * @property {(sessions: ChatSession[]) => void} setSessions - Set all sessions
 * @property {(session: ChatSession) => void} addSession - Add a new session
 * @property {(sessionId: string) => void} removeSession - Remove a session
 * @property {(sessionId: string, updates: Partial<ChatSession>) => void} updateSession - Update session
 * @property {(sessionId: string) => void} setActiveSession - Set active session and clear unread count
 * @property {(sessionId: string, messages: ChatMessage[]) => void} setMessages - Set messages for session
 * @property {(message: ChatMessage) => void} addMessage - Add a message to active session with deduplication
 * @property {(sessionId: string, message: ChatMessage) => void} addMessageToSession - Add message to specific session with deduplication and unread tracking
 * @property {(sessionId: string, messageId: string) => void} removeMessage - Remove a message
 * @property {(sessionId: string) => void} incrementUnreadCount - Increment unread message count for session
 * @property {(sessionId: string) => void} clearUnreadCount - Clear unread message count for session
 * @property {(content: string) => void} startStreaming - Start streaming a response
 * @property {(content: string) => void} updateStreamingContent - Update streaming content
 * @property {(toolCall: ToolCall) => void} addStreamingToolCall - Add streaming tool call
 * @property {(toolCallId: string, result: Object) => void} updateToolCallResult - Update tool call result
 * @property {() => void} stopStreaming - Stop streaming
 * @property {(connected: boolean) => void} setConnected - Set connection status
 * @property {(loading: boolean) => void} setLoading - Set loading state
 * @property {(error: string|null) => void} setError - Set error message
 * @property {(sessionId: string) => void} clearSessionMessages - Clear all messages in session
 * @property {() => void} reset - Reset entire store to initial state
 */

/**
 * @typedef {ChatState & ChatActions} ChatStore
 */

const initialState = {
  sessions: [],
  messages: {},
  activeSessionId: null,
  streaming: {
    isStreaming: false,
    currentContent: '',
    sessionId: null,
    pendingToolCalls: [],
  },
  isLoading: false,
  error: null,
  isConnected: false,
  availableProviders: ['openai', 'anthropic', 'ollama', 'openrouter'],
  availableModels: {
    openai: ['gpt-4', 'gpt-4-turbo', 'gpt-3.5-turbo'],
    anthropic: ['claude-3-opus', 'claude-3-sonnet', 'claude-3-haiku'],
    ollama: ['llama2', 'mistral', 'codellama'],
    openrouter: ['auto'],
  },
};

/**
 * Chat Store - Manages chat sessions, messages, and streaming state
 *
 * @description
 * Centralized state management for the chat interface.
 * Handles multiple chat sessions, message history, streaming responses,
 * and tool call visualization. Integrates with DevTools for debugging.
 *
 * @example
 * // Get active session messages
 * const messages = useChatStore(state =>
 *   state.messages[state.activeSessionId] || []
 * );
 *
 * @example
 * // Start a new streaming response
 * const startStreaming = useChatStore(state => state.startStreaming);
 * startStreaming('Hello, how can I help you?');
 *
 * @example
 * // Use optimized selector for active session
 * const activeSession = useChatStore(selectActiveSession);
 */
export const useChatStore = create(
  devtools(
    (set, get) => ({
      ...initialState,

      setSessions: (sessions) => set({ sessions }, false, 'setSessions'),

      addSession: (session) => set((state) => ({
        sessions: [...state.sessions, session],
        activeSessionId: session.id,
        messages: { ...state.messages, [session.id]: [] },
      }), false, 'addSession'),

      removeSession: (sessionId) => set((state) => {
        const { [sessionId]: removed, ...remainingMessages } = state.messages;

        return {
          sessions: state.sessions.filter(s => s.id !== sessionId),
          messages: remainingMessages,
          activeSessionId: state.activeSessionId === sessionId
            ? (state.sessions[0]?.id || null)
            : state.activeSessionId,
        };
      }, false, 'removeSession'),

      updateSession: (sessionId, updates) => set((state) => ({
        sessions: state.sessions.map(s =>
          s.id === sessionId ? { ...s, ...updates, updatedAt: Date.now() } : s
        ),
      }), false, 'updateSession'),

      setActiveSession: (sessionId) => set((state) => {
        if (sessionId) {
          localStorage.setItem('activeSessionId', sessionId);
        } else {
          localStorage.removeItem('activeSessionId');
        }

        return {
          activeSessionId: sessionId,
          sessions: state.sessions.map(s =>
            s.id === sessionId ? { ...s, unread_message_count: 0 } : s
          ),
        };
      }, false, 'setActiveSession'),

      setMessages: (sessionId, messages) => set((state) => ({
        messages: { ...state.messages, [sessionId]: messages },
      }), false, 'setMessages'),

      addMessage: (message) => set((state) => {
        const sessionId = state.activeSessionId;
        if (!sessionId) return state;

        const sessionMessages = state.messages[sessionId] || [];

        if (sessionMessages.some(m => m.id === message.id)) {
          return state;
        }

        return {
          messages: {
            ...state.messages,
            [sessionId]: [...sessionMessages, message],
          },
          sessions: state.sessions.map(s =>
            s.id === sessionId
              ? { ...s, messageCount: sessionMessages.length + 1, updatedAt: Date.now() }
              : s
          ),
        };
      }, false, 'addMessage'),

      removeMessage: (sessionId, messageId) => set((state) => ({
        messages: {
          ...state.messages,
          [sessionId]: (state.messages[sessionId] || []).filter(m => m.id !== messageId),
        },
      }), false, 'removeMessage'),

      updateMessage: (sessionId, messageId, updates) => set((state) => ({
        messages: {
          ...state.messages,
          [sessionId]: (state.messages[sessionId] || []).map(m =>
            m.id === messageId ? { ...m, ...updates } : m
          ),
        },
      }), false, 'updateMessage'),

      removeMessagesFromIndex: (sessionId, messageIndex) => set((state) => {
        const sessionMessages = state.messages[sessionId] || [];

        return {
          messages: {
            ...state.messages,
            [sessionId]: sessionMessages.slice(0, messageIndex),
          },
        };
      }, false, 'removeMessagesFromIndex'),

      startStreaming: (content) => set((state) => ({
        streaming: {
          isStreaming: true,
          currentContent: content,
          sessionId: state.activeSessionId,
          pendingToolCalls: [],
        },
      }), false, 'startStreaming'),

      updateStreamingContent: (content) => set((state) => ({
        streaming: {
          ...state.streaming,
          currentContent: state.streaming.currentContent + content,
        },
      }), false, 'updateStreamingContent'),

      addStreamingToolCall: (toolCall) => set((state) => ({
        streaming: {
          ...state.streaming,
          pendingToolCalls: [...state.streaming.pendingToolCalls, toolCall],
        },
      }), false, 'addStreamingToolCall'),

      updateToolCallResult: (toolCallId, result) => set((state) => ({
        streaming: {
          ...state.streaming,
          pendingToolCalls: state.streaming.pendingToolCalls.map(tc =>
            tc.id === toolCallId ? { ...tc, result, status: 'success' } : tc
          ),
        },
      }), false, 'updateToolCallResult'),

      stopStreaming: () => set((state) => {
        return {
          streaming: {
            isStreaming: false,
            currentContent: '',
            sessionId: null,
            pendingToolCalls: [],
          },
        };
      }, false, 'stopStreaming'),

      setConnected: (isConnected) => set({ isConnected }, false, 'setConnected'),

      setLoading: (isLoading) => set({ isLoading }, false, 'setLoading'),

      setError: (error) => set({ error }, false, 'setError'),

      clearSessionMessages: (sessionId) => set((state) => ({
        messages: { ...state.messages, [sessionId]: [] },
        sessions: state.sessions.map(s =>
          s.id === sessionId ? { ...s, messageCount: 0 } : s
        ),
      }), false, 'clearSessionMessages'),

      addMessageToSession: (sessionId, message) => set((state) => {
        const sessionMessages = state.messages[sessionId] || [];

        if (sessionMessages.some(m => m.id === message.id)) {
          return state;
        }

        const isActiveSession = sessionId === state.activeSessionId;

        return {
          messages: {
            ...state.messages,
            [sessionId]: [...sessionMessages, message],
          },
          sessions: state.sessions.map(s => {
            if (s.id !== sessionId) return s;

            return {
              ...s,
              messageCount: sessionMessages.length + 1,
              updatedAt: Date.now(),
              unread_message_count: isActiveSession
                ? 0
                : (s.unread_message_count || 0) + 1,
            };
          }),
        };
      }, false, 'addMessageToSession'),

      incrementUnreadCount: (sessionId) => set((state) => ({
        sessions: state.sessions.map(s =>
          s.id === sessionId
            ? { ...s, unread_message_count: (s.unread_message_count || 0) + 1 }
            : s
        ),
      }), false, 'incrementUnreadCount'),

      clearUnreadCount: (sessionId) => set((state) => ({
        sessions: state.sessions.map(s =>
          s.id === sessionId ? { ...s, unread_message_count: 0 } : s
        ),
      }), false, 'clearUnreadCount'),

      reset: () => set(initialState, false, 'reset'),
    }),
    {
      name: 'chat-store',
      enabled: process.env.NODE_ENV === 'development',
    }
  )
);

/**
 * Optimized selectors to prevent unnecessary re-renders
 */

export const selectSessions = (state) => state.sessions;

export const selectActiveSessionId = (state) => state.activeSessionId;

export const selectActiveSession = (state) =>
  state.sessions.find(s => s.id === state.activeSessionId) || null;

export const selectActiveSessionMessages = (state) =>
  state.messages[state.activeSessionId] || [];

export const selectSessionMessages = (sessionId) => (state) =>
  state.messages[sessionId] || [];

export const selectStreamingState = (state) => state.streaming;

export const selectIsStreaming = (state) => state.streaming.isStreaming;

export const selectStreamingContent = (state) => state.streaming.currentContent;

export const selectIsConnected = (state) => state.isConnected;

export const selectIsLoading = (state) => state.isLoading;

export const selectError = (state) => state.error;

export const selectAvailableProviders = (state) => state.availableProviders;

export const selectAvailableModels = (provider) => (state) =>
  state.availableModels[provider] || [];

export const selectRecentSessions = (limit = 5) => (state) =>
  [...state.sessions]
    .sort((a, b) => b.updatedAt - a.updatedAt)
    .slice(0, limit);
