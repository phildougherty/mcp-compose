import React, { useState, useEffect } from 'react';
import { useChatStore } from '../../store/chatStore';
import { chatApi } from '../../api';
import { Button, Select } from '../shared';
import clsx from 'clsx';

export default function SessionList({ sidebarOpen, sidebarCollapsed, onToggleCollapse, onClose, onCreateNew, onLoadSession }) {
  const sessions = useChatStore((state) => Array.isArray(state.sessions) ? state.sessions : []);
  const activeSessionId = useChatStore((state) => state.activeSessionId);
  const removeSession = useChatStore((state) => state.removeSession);
  const updateSession = useChatStore((state) => state.updateSession);
  const [providers, setProviders] = useState({});
  const [loading, setLoading] = useState(true);

  const activeSession = Array.isArray(sessions) ? sessions.find((s) => s.id === activeSessionId) : null;
  const selectedProvider = activeSession?.provider || 'openrouter';
  const selectedModel = activeSession?.model || 'z-ai/glm-4.6';

  useEffect(() => {
    loadProviders();
  }, []);

  const loadProviders = async () => {
    try {
      setLoading(true);
      const data = await chatApi.getChatProviders();
      setProviders(data || {});
    } catch (err) {
      console.error('Failed to load providers:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleProviderChange = async (provider) => {
    if (!activeSessionId) return;

    const models = providers[provider] || [];
    const defaultModel = models[0] || '';

    try {
      await chatApi.updateChatSession(activeSessionId, {
        provider,
        model: defaultModel,
      });

      updateSession(activeSessionId, { provider, model: defaultModel });
    } catch (err) {
      console.error('Failed to update provider:', err);
    }
  };

  const handleModelChange = async (model) => {
    if (!activeSessionId) return;

    try {
      await chatApi.updateChatSession(activeSessionId, { model });
      updateSession(activeSessionId, { model });
    } catch (err) {
      console.error('Failed to update model:', err);
    }
  };

  const handleDeleteSession = async (sessionId, e) => {
    e.stopPropagation();

    if (!confirm('Delete this conversation?')) return;

    try {
      await chatApi.deleteChatSession(sessionId);
      removeSession(sessionId);
    } catch (err) {
      console.error('Failed to delete session:', err);
    }
  };

  const handleRenameSession = async (sessionId, e) => {
    e.stopPropagation();

    const newTitle = prompt('Enter new title:');
    if (!newTitle) return;

    try {
      await chatApi.updateChatSession(sessionId, { title: newTitle });
      updateSession(sessionId, { title: newTitle });
    } catch (err) {
      console.error('Failed to rename session:', err);
    }
  };

  const formatDate = (dateString) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString();
  };

  return (
    <div
      className={clsx(
        'chat-sidebar bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700',
        'fixed md:relative inset-y-0 left-0 z-50 transition-transform duration-300 flex flex-col',
        sidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
        sidebarCollapsed ? 'w-16' : 'w-80'
      )}
    >
      <div className="sidebar-header flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
        {!sidebarCollapsed && (
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            Conversations
          </h3>
        )}

        <button
          onClick={onToggleCollapse}
          className="sidebar-collapse-btn min-w-[44px] min-h-[44px] flex items-center justify-center text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg"
          title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {sidebarCollapsed ? (
            <span className="text-xl">▶</span>
          ) : (
            <span className="text-xl">◀</span>
          )}
        </button>

        <button
          onClick={onClose}
          className="sidebar-close md:hidden min-w-[44px] min-h-[44px] flex items-center justify-center text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg text-2xl"
        >
          ×
        </button>
      </div>

      {!sidebarCollapsed && (
        <>
          <div className="sidebar-controls p-4 space-y-4">
            <Button
              onClick={onCreateNew}
              variant="primary"
              className="w-full"
            >
              + New Chat
            </Button>

            {activeSessionId && (
              <div className="model-selector space-y-2">
                <label className="text-xs font-semibold text-gray-700 dark:text-gray-300 block">
                  AI Provider & Model
                </label>

                {loading ? (
                  <div className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 p-2">
                    <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600" />
                    Loading...
                  </div>
                ) : (
                  <>
                    <Select
                      value={selectedProvider}
                      onChange={handleProviderChange}
                      options={Object.keys(providers).map((p) => ({
                        value: p,
                        label: p.charAt(0).toUpperCase() + p.slice(1),
                      }))}
                      placeholder="Provider"
                      className="w-full"
                    />

                    <Select
                      value={selectedModel}
                      onChange={handleModelChange}
                      options={(providers[selectedProvider] || []).map((m) => ({
                        value: m,
                        label: m,
                      }))}
                      placeholder="Model"
                      className="w-full"
                    />
                  </>
                )}
              </div>
            )}
          </div>

          <div className="sessions-list overflow-y-auto flex-1 min-h-0">
            {sessions.length === 0 ? (
              <div className="p-4 text-center text-sm text-gray-500 dark:text-gray-400">
                No conversations yet
              </div>
            ) : (
              sessions.map((session) => (
                <div
                  key={session.id}
                  className={clsx(
                    'session-item cursor-pointer border-b border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors',
                    activeSessionId === session.id && 'bg-blue-50 dark:bg-blue-900/20'
                  )}
                  onClick={() => onLoadSession(session.id)}
                >
                  <div className="p-4">
                    <div className="session-title font-medium text-gray-900 dark:text-white mb-1 truncate flex items-center gap-2">
                      <span className="flex-1 truncate">{session.title || session.name || 'Untitled'}</span>
                      {session.unread_message_count > 0 && (
                        <span className="flex-shrink-0 inline-flex items-center justify-center min-w-[20px] h-5 px-1.5 text-xs font-semibold text-white bg-red-500 rounded-full">
                          {session.unread_message_count}
                        </span>
                      )}
                      {session.has_active_agents && (
                        <span className="flex-shrink-0 inline-flex items-center justify-center w-5 h-5 text-purple-600 dark:text-purple-400" title="Active scheduled agents">
                          <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-5.5-2.5l7.51-3.49L17.5 6.5 9.99 9.99 6.5 17.5zm5.5-6.6c.61 0 1.1.49 1.1 1.1s-.49 1.1-1.1 1.1-1.1-.49-1.1-1.1.49-1.1 1.1-1.1z"/>
                          </svg>
                        </span>
                      )}
                    </div>

                    <div className="session-meta text-xs text-gray-500 dark:text-gray-400 mb-2 space-y-1">
                      <div className="flex items-center gap-1">
                        <span className="font-medium">{session.provider || 'openrouter'}</span>
                      </div>
                      <div className="truncate" title={session.model}>
                        {session.model || 'z-ai/glm-4.6'}
                      </div>
                      <div>{formatDate(session.updatedAt || session.last_used)}</div>
                    </div>

                    <div className="session-actions flex gap-2">
                      <button
                        onClick={(e) => handleRenameSession(session.id, e)}
                        className="session-action-btn px-3 py-1 text-xs text-blue-600 dark:text-blue-400 hover:bg-blue-100 dark:hover:bg-blue-900/30 rounded min-h-[44px]"
                        title="Rename"
                      >
                        Edit
                      </button>
                      <button
                        onClick={(e) => handleDeleteSession(session.id, e)}
                        className="session-action-btn px-3 py-1 text-xs text-red-600 dark:text-red-400 hover:bg-red-100 dark:hover:bg-red-900/30 rounded min-h-[44px]"
                        title="Delete"
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                </div>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}
