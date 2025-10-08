import React, { useEffect, useState, useRef, useMemo, useCallback } from 'react';
import { useChatStore } from '../../store/chatStore';
import { chatApi, apiClient } from '../../api';
import { useWebSocket } from '../../hooks';
import SessionList from './SessionList';
import MessageList from './MessageList';
import ChatInput from './ChatInput';
import MCPServerSelector from './MCPServerSelector';
import ConnectionStatus from './ConnectionStatus';
import WorkflowDeploymentPanel from './WorkflowDeploymentPanel';
import WorkflowSuggestionCard from './WorkflowSuggestionCard';
import DeploymentWizard from './DeploymentWizard';
import { Modal } from '../shared';
import Button from '../shared/Button';
import clsx from 'clsx';

export default function Chat({ initialSessionId }) {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [showSystemPrompt, setShowSystemPrompt] = useState(false);
  const [systemPrompt, setSystemPrompt] = useState('');
  const [editedSystemPrompt, setEditedSystemPrompt] = useState('');
  const [loadingSystemPrompt, setLoadingSystemPrompt] = useState(false);
  const [savingSystemPrompt, setSavingSystemPrompt] = useState(false);
  const [error, setError] = useState(null);
  const [showWorkflowDeployment, setShowWorkflowDeployment] = useState(false);
  const [suggestedWorkflow, setSuggestedWorkflow] = useState(null);
  const [workflowSuggestions, setWorkflowSuggestions] = useState([]);
  const [showDeploymentWizard, setShowDeploymentWizard] = useState(false);

  const sessions = useChatStore((state) => state.sessions);
  const activeSessionId = useChatStore((state) => state.activeSessionId);
  const messages = useChatStore((state) => state.messages);
  const streaming = useChatStore((state) => state.streaming);
  const isConnected = useChatStore((state) => state.isConnected);
  const setSessions = useChatStore((state) => state.setSessions);
  const setActiveSession = useChatStore((state) => state.setActiveSession);
  const updateSession = useChatStore((state) => state.updateSession);
  const setMessages = useChatStore((state) => state.setMessages);
  const addMessage = useChatStore((state) => state.addMessage);
  const addMessageToSession = useChatStore((state) => state.addMessageToSession);
  const incrementUnreadCount = useChatStore((state) => state.incrementUnreadCount);
  const clearUnreadCount = useChatStore((state) => state.clearUnreadCount);
  const updateMessage = useChatStore((state) => state.updateMessage);
  const startStreaming = useChatStore((state) => state.startStreaming);
  const updateStreamingContent = useChatStore((state) => state.updateStreamingContent);
  const stopStreaming = useChatStore((state) => state.stopStreaming);
  const setConnected = useChatStore((state) => state.setConnected);
  const removeMessagesFromIndex = useChatStore((state) => state.removeMessagesFromIndex);

  const activeSession = Array.isArray(sessions) ? sessions.find(s => s.id === activeSessionId) : null;
  const sessionMessages = messages[activeSessionId] || [];

  const wsUrl = useMemo(() => {
    return activeSessionId
      ? `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/chat/${activeSessionId}`
      : null;
  }, [activeSessionId]);

  useEffect(() => {
    console.log('[Chat] activeSessionId changed:', activeSessionId, 'wsUrl:', wsUrl);
  }, [activeSessionId, wsUrl]);

  useEffect(() => {
    const initializeChat = async () => {
      console.log('[Chat] Initializing chat...');
      await loadProviders();

      const sessionToLoad = initialSessionId || localStorage.getItem('activeSessionId');
      console.log('[Chat] Session to load:', sessionToLoad, '(initial:', initialSessionId, ')');

      if (sessionToLoad) {
        try {
          console.log('[Chat] Loading session:', sessionToLoad);
          const sessionData = await chatApi.getChatSession(sessionToLoad);
          console.log('[Chat] Session data loaded:', sessionData);

          updateSession(sessionToLoad, {
            provider: sessionData.provider,
            model: sessionData.model,
            title: sessionData.title,
            mcp_servers: sessionData.mcp_servers || [],
            metadata: sessionData.metadata,
          });

          setActiveSession(sessionToLoad);
          setMessages(sessionToLoad, sessionData.messages || []);
          clearUnreadCount(sessionToLoad);

          await loadSessions(false);
          console.log('[Chat] Successfully loaded session');
        } catch (err) {
          console.log('[Chat] Failed to load session, loading sessions list');
          if (!initialSessionId) {
            localStorage.removeItem('activeSessionId');
          }
          setActiveSession(null);
          await loadSessions(true);
        }
      } else {
        console.log('[Chat] No session to load, loading sessions list');
        await loadSessions(true);
      }
      console.log('[Chat] Initialization complete');
    };

    initializeChat();
  }, [initialSessionId]);

  const sendRef = useRef(null);

  const { isConnected: wsConnected, send, lastMessage } = useWebSocket(
    wsUrl,
    {
      autoConnect: !!activeSessionId,
      onOpen: () => setConnected(true),
      onClose: () => setConnected(false),
      onError: (err) => {
        setConnected(false);
        showError('WebSocket connection error');
      },
      onMessage: (data) => {
        if (data.type === 'ping') {
          if (sendRef.current) {
            sendRef.current({ type: 'pong' });
          }
          return;
        } else if (data.type === 'pong') {
          return;
        }
        handleWebSocketMessage(data);
      },
    }
  );

  useEffect(() => {
    sendRef.current = send;
  }, [send]);

  useEffect(() => {
    setConnected(wsConnected);
  }, [wsConnected, setConnected]);

  const loadSessions = async (autoSelectFirst = false) => {
    try {
      console.log('[Chat] loadSessions called, autoSelectFirst:', autoSelectFirst);
      const data = await chatApi.getChatSessions();
      console.log('[Chat] Fetched sessions:', data?.length, 'sessions');
      setSessions(data || []);

      if (autoSelectFirst && data && data.length > 0) {
        const currentActiveId = useChatStore.getState().activeSessionId;
        console.log('[Chat] Current active ID from store:', currentActiveId);
        if (!currentActiveId) {
          console.log('[Chat] No active session, selecting first:', data[0].id);
          await loadSession(data[0].id);
        } else {
          console.log('[Chat] Active session already set, skipping auto-select');
        }
      }
    } catch (err) {
      console.error('Failed to load sessions:', err);
    }
  };

  const loadProviders = async () => {
    try {
      await chatApi.getChatProviders();
    } catch (err) {
      console.error('Failed to load providers:', err);
    }
  };

  const loadSession = async (sessionId) => {
    try {
      console.log('[Chat] loadSession called for:', sessionId);
      const data = await chatApi.getChatSession(sessionId);
      console.log('[Chat] Session data loaded, messages:', data.messages?.length);

      updateSession(sessionId, {
        provider: data.provider,
        model: data.model,
        title: data.title,
        mcp_servers: data.mcp_servers || [],
        metadata: data.metadata,
      });

      setMessages(sessionId, data.messages || []);

      setActiveSession(sessionId);
      console.log('[Chat] setActiveSession called with:', sessionId);
      clearUnreadCount(sessionId);
      closeSidebar();
    } catch (err) {
      console.error('[Chat] Failed to load session:', err);
      showError('Failed to load session: ' + err.message);
      throw err;
    }
  };

  const createNewSession = async () => {
    try {
      console.log('[Chat] Creating new session...');
      const session = await chatApi.createChatSession({
        title: 'New Chat',
        provider: 'openrouter',
        model: 'z-ai/glm-4.6',
      });
      console.log('[Chat] New session created:', session.id);

      const currentSessions = useChatStore.getState().sessions;
      setSessions([session, ...currentSessions]);
      console.log('[Chat] Session added to store');

      setMessages(session.id, []);

      setActiveSession(session.id);
      console.log('[Chat] Session activated:', session.id);

      closeSidebar();
    } catch (err) {
      console.error('[Chat] Failed to create session:', err);
      showError('Failed to create session: ' + err.message);
    }
  };

  const lastProcessedMessageRef = useRef(null);

  const detectWorkflowSuggestion = (content) => {
    try {
      const workflowMatch = content.match(/WORKFLOW_SUGGESTION:(.*?)(?:END_WORKFLOW|$)/s);

      if (workflowMatch) {
        const workflowData = JSON.parse(workflowMatch[1].trim());

        return workflowData;
      }
    } catch (err) {
      console.error('Failed to parse workflow suggestion:', err);
    }

    return null;
  };

  const handleWorkflowDeploy = async (workflow) => {
    try {
      console.log('Deploying workflow:', workflow);

      const result = await apiClient.post('/workflows/deploy', workflow);
      showError('Workflow deployed successfully!');
      setShowWorkflowDeployment(false);
      setSuggestedWorkflow(null);

      return result;
    } catch (err) {
      console.error('Failed to deploy workflow:', err);
      showError('Failed to deploy workflow: ' + err.message);
      throw err;
    }
  };

  const handleWorkflowCustomize = (workflow) => {
    console.log('Customizing workflow:', workflow);
    setShowWorkflowDeployment(false);
    setSuggestedWorkflow(null);
  };

  const handleWebSocketMessage = (data) => {
    if (data.type === 'chunk') {
      if (!data.done) {
        if (!streaming.isStreaming) {
          startStreaming('');
        }

        const currentContent = streaming.currentContent + (data.content || '');
        updateStreamingContent(data.content || '');

        const workflowData = detectWorkflowSuggestion(currentContent);

        if (workflowData) {
          setSuggestedWorkflow(workflowData);
          setShowWorkflowDeployment(true);
        }
      } else {
        // Streaming is done - stop streaming and store the message ID to prevent duplicate from new_message
        if (streaming.isStreaming) {
          stopStreaming();
          if (data.message_id) {
            lastProcessedMessageRef.current = data.message_id;
          }
        }
      }
    } else if (data.type === 'new_message') {
      const targetSessionId = data.sessionId || data.SessionID;

      if (!targetSessionId) {
        console.warn('Received new_message without sessionId:', data);

        return;
      }

      const message = data.payload || data.message || {
        id: data.message_id,
        role: data.role || 'assistant',
        content: data.content,
        tool_calls: data.tool_calls || [],
        tool_results: data.tool_results || [],
        created_at: new Date().toISOString(),
      };

      if (!message.id) {
        console.warn('Received new_message without message data:', data);

        return;
      }

      // Skip user messages - they're already added when sending
      if (message.role === 'user') {
        return;
      }

      // Skip if we already processed this message from the chunk handler
      if (lastProcessedMessageRef.current === message.id) {
        console.log('[Chat] Skipping duplicate new_message for:', message.id);
        lastProcessedMessageRef.current = null; // Reset after use

        return;
      }

      if (targetSessionId === activeSessionId) {
        addMessage(message);

        const workflowData = detectWorkflowSuggestion(message.content);

        if (workflowData) {
          setSuggestedWorkflow(workflowData);
          setShowWorkflowDeployment(true);
        }
      } else {
        addMessageToSession(targetSessionId, message);
        incrementUnreadCount(targetSessionId);
      }
    } else if (data.type === 'message_update') {
      const targetSessionId = data.sessionId;
      const messageId = data.messageId;
      const updates = data.updates;

      if (!targetSessionId || !messageId || !updates) {
        console.warn('Received message_update without required fields:', data);

        return;
      }

      updateMessage(targetSessionId, messageId, updates);
    } else if (data.type === 'error') {
      stopStreaming();
      let errorMsg = data.error || data.message || 'An error occurred';

      if (errorMsg.includes('401') || errorMsg.includes('User not found')) {
        errorMsg = 'Authentication error: Please check your API key configuration';
      }

      showError(errorMsg);
    }
  };

  const sendMessage = async (content) => {
    if (!content.trim() || streaming.isStreaming) return;

    let currentSessionId = activeSessionId;

    if (!currentSessionId) {
      await createNewSession();
      currentSessionId = useChatStore.getState().activeSessionId;

      if (!currentSessionId) {
        showError('Failed to create session');

        return;
      }
    }

    if (!isConnected) {
      showError('Not connected. Please wait...');

      return;
    }

    const userMessage = {
      id: `msg-${Date.now()}`,
      sessionId: currentSessionId,
      role: 'user',
      content: content,
      created_at: new Date().toISOString(),
    };

    addMessage(userMessage);

    try {
      send({
        type: 'message',
        message: content,
      });
    } catch (err) {
      showError('Failed to send message: ' + err.message);
    }
  };

  const regenerateMessage = async (messageIndex) => {
    if (streaming.isStreaming || !activeSessionId) return;

    const currentMessages = sessionMessages;
    if (messageIndex < 0 || messageIndex >= currentMessages.length) return;

    const messageToRegenerate = currentMessages[messageIndex];
    if (messageToRegenerate.role !== 'assistant') return;

    if (messageIndex === 0) {
      showError('Cannot regenerate the first message');

      return;
    }

    const previousMessage = currentMessages[messageIndex - 1];
    if (previousMessage.role !== 'user') {
      showError('Cannot regenerate: previous message is not from user');

      return;
    }

    if (!isConnected) {
      showError('Not connected. Please wait...');

      return;
    }

    removeMessagesFromIndex(activeSessionId, messageIndex);

    try {
      send({
        type: 'message',
        message: previousMessage.content,
      });
    } catch (err) {
      showError('Failed to regenerate: ' + err.message);
    }
  };

  const toggleSidebar = () => {
    setSidebarOpen(!sidebarOpen);
  };

  const closeSidebar = () => {
    setSidebarOpen(false);
  };

  const toggleSidebarCollapse = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  const handleToggleSystemPrompt = async () => {
    setShowSystemPrompt(!showSystemPrompt);

    if (!showSystemPrompt && !systemPrompt && activeSessionId) {
      setLoadingSystemPrompt(true);

      try {
        const data = await chatApi.getSystemPrompt(activeSessionId);
        const prompt = data.system_prompt || 'No system prompt available';
        setSystemPrompt(prompt);
        setEditedSystemPrompt(prompt);
      } catch (err) {
        console.error('Failed to load system prompt:', err);
        const errorMsg = 'Error loading system prompt';
        setSystemPrompt(errorMsg);
        setEditedSystemPrompt(errorMsg);
      }

      setLoadingSystemPrompt(false);
    }
  };

  const handleSaveSystemPrompt = async () => {
    if (!activeSessionId || editedSystemPrompt === systemPrompt) return;

    setSavingSystemPrompt(true);

    try {
      await chatApi.updateSystemPrompt(activeSessionId, editedSystemPrompt);
      setSystemPrompt(editedSystemPrompt);
      setError(null);
    } catch (err) {
      console.error('Failed to save system prompt:', err);
      setError('Failed to save system prompt');
    }

    setSavingSystemPrompt(false);
  };

  const showError = (message) => {
    setError(message);
    setTimeout(() => {
      setError(null);
    }, 5000);
  };

  const clearError = () => {
    setError(null);
  };

  return (
    <div className="chat-container flex h-full bg-white dark:bg-gray-900 w-full max-w-full overflow-x-hidden">
      <div
        className={clsx(
          'sidebar-backdrop fixed inset-0 bg-black bg-opacity-50 z-40 lg:hidden',
          sidebarOpen ? 'block' : 'hidden'
        )}
        onClick={closeSidebar}
      />

      <SessionList
        sidebarOpen={sidebarOpen}
        sidebarCollapsed={sidebarCollapsed}
        onToggleCollapse={toggleSidebarCollapse}
        onClose={closeSidebar}
        onCreateNew={createNewSession}
        onLoadSession={loadSession}
      />

      <div className="chat-main flex-1 flex flex-col min-w-0 w-full max-w-full overflow-x-hidden">
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6 m-4">
          <div className="flex items-center justify-between gap-4">
            <div className="flex items-center gap-3 flex-1 min-w-0">
              <Button
                onClick={toggleSidebar}
                variant="ghost"
                size="lg"
                className="hamburger-menu lg:hidden flex-shrink-0"
                title="Menu"
              >
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </Button>

              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-gradient-to-br from-gray-700 via-gray-800 to-gray-900 rounded-lg flex items-center justify-center border border-gray-700 shadow-lg">
                  <svg className="w-6 h-6 text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
                  </svg>
                </div>
              </div>
              <div className="flex-1 min-w-0 hidden lg:block">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                  <span className="truncate">{activeSession?.title || activeSession?.name || 'New Chat'}</span>
                  {streaming.isStreaming && (
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-400 border border-blue-200 dark:border-blue-800 flex-shrink-0">
                      <span className="w-2 h-2 bg-blue-500 dark:bg-blue-400 rounded-full mr-1.5 animate-pulse" />
                      STREAMING
                    </span>
                  )}
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">AI-powered conversation with MCP tools</p>
              </div>
            </div>

            <div className="flex items-center gap-2 flex-shrink-0">
              <Button
                onClick={handleToggleSystemPrompt}
                variant="ghost"
                size="lg"
                className="system-prompt-btn flex-shrink-0"
                title={showSystemPrompt ? 'Hide System Prompt' : 'View System Prompt'}
              >
                {!showSystemPrompt ? (
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                )}
              </Button>
              <ConnectionStatus />
            </div>
          </div>
        </div>

        {showSystemPrompt && (
          <div className="system-prompt-viewer bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 m-4 mt-0 w-full max-w-full overflow-x-hidden">
            <div className="flex items-center justify-between mb-3 gap-3">
              <h4 className="text-sm font-semibold text-gray-900 dark:text-white">System Prompt</h4>
              <div className="flex items-center gap-2">
                {editedSystemPrompt !== systemPrompt && (
                  <Button
                    onClick={handleSaveSystemPrompt}
                    variant="primary"
                    size="sm"
                    disabled={savingSystemPrompt}
                  >
                    {savingSystemPrompt ? (
                      <>
                        <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-current mr-2" />
                        Saving...
                      </>
                    ) : (
                      'Save'
                    )}
                  </Button>
                )}
                <Button
                  onClick={handleToggleSystemPrompt}
                  variant="ghost"
                  size="sm"
                  className="close-btn flex-shrink-0"
                >
                  <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </Button>
              </div>
            </div>

            {loadingSystemPrompt ? (
              <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600 flex-shrink-0" />
                <span>Loading system prompt...</span>
              </div>
            ) : (
              <textarea
                value={editedSystemPrompt}
                onChange={(e) => setEditedSystemPrompt(e.target.value)}
                className="w-full text-sm text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-900 p-3 rounded-lg border border-gray-200 dark:border-gray-700 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500 resize-vertical min-h-[120px] max-h-[400px] font-mono"
                placeholder="Enter system prompt..."
              />
            )}
          </div>
        )}

        <MessageList
          messages={sessionMessages}
          streamingContent={streaming.currentContent}
          isStreaming={streaming.isStreaming}
          onSendMessage={sendMessage}
          onRegenerate={regenerateMessage}
          activeSession={activeSession}
        />

        {showWorkflowDeployment && suggestedWorkflow && (
          <div className="px-4">
            <WorkflowDeploymentPanel
              workflow={suggestedWorkflow}
              onDeploy={handleWorkflowDeploy}
              onCustomize={handleWorkflowCustomize}
              onCancel={() => {
                setShowWorkflowDeployment(false);
                setSuggestedWorkflow(null);
              }}
            />
          </div>
        )}

        {showDeploymentWizard && (
          <div className="px-4">
            <DeploymentWizard
              suggestedWorkflows={workflowSuggestions}
              onDeploy={handleWorkflowDeploy}
              onCancel={() => {
                setShowDeploymentWizard(false);
                setWorkflowSuggestions([]);
              }}
              onCustomize={handleWorkflowCustomize}
            />
          </div>
        )}

        <div className="input-container border-t border-gray-200 dark:border-gray-700 p-4 w-full max-w-full overflow-x-hidden">
          {error && (
            <div className="error-message mb-4 bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg w-full">
              <div className="flex items-start gap-3">
                <div className="flex-shrink-0">
                  <svg className="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                  </svg>
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
                  <div className="mt-1 text-sm text-red-700 dark:text-red-300 break-words">{error}</div>
                  <div className="mt-3">
                    <Button
                      onClick={clearError}
                      variant="ghost"
                      size="sm"
                      className="error-close !text-red-600 hover:!text-red-800 dark:!text-red-400 dark:hover:!text-red-200"
                    >
                      Dismiss
                    </Button>
                  </div>
                </div>
              </div>
            </div>
          )}

          <MCPServerSelector sessionId={activeSessionId} />

          <ChatInput
            onSend={sendMessage}
            disabled={streaming.isStreaming}
          />
        </div>
      </div>
    </div>
  );
}
