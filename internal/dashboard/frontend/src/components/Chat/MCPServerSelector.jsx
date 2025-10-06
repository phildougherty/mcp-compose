import React, { useEffect, useState } from 'react';
import { useChatStore } from '../../store/chatStore';
import { chatApi } from '../../api';
import { Button, Checkbox } from '../shared';
import clsx from 'clsx';

export default function MCPServerSelector({ sessionId }) {
  const [showDropdown, setShowDropdown] = useState(false);
  const [availableServers, setAvailableServers] = useState([]);
  const [enabledServers, setEnabledServers] = useState([]);
  const [loading, setLoading] = useState(false);

  const sessions = useChatStore((state) => Array.isArray(state.sessions) ? state.sessions : []);
  const updateSession = useChatStore((state) => state.updateSession);
  const activeSession = Array.isArray(sessions) ? sessions.find((s) => s.id === sessionId) : null;

  useEffect(() => {
    if (activeSession) {
      setEnabledServers(activeSession.mcpServers || []);
    }
  }, [activeSession]);

  useEffect(() => {
    if (showDropdown && availableServers.length === 0) {
      loadMCPServers();
    }
  }, [showDropdown]);

  useEffect(() => {
    const handleClickOutside = (e) => {
      if (!e.target.closest('.mcp-dropdown')) {
        setShowDropdown(false);
      }
    };

    document.addEventListener('click', handleClickOutside);

    return () => {
      document.removeEventListener('click', handleClickOutside);
    };
  }, []);

  const loadMCPServers = async () => {
    setLoading(true);

    try {
      const data = await chatApi.getMCPServers();
      setAvailableServers(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('Failed to load MCP servers:', err);
      setAvailableServers([]);
    }

    setLoading(false);
  };

  const toggleDropdown = (e) => {
    e.stopPropagation();
    setShowDropdown(!showDropdown);
  };

  const toggleServer = async (serverName) => {
    if (!sessionId) return;

    const newServers = enabledServers.includes(serverName)
      ? enabledServers.filter((s) => s !== serverName)
      : [...enabledServers, serverName];

    try {
      await chatApi.updateChatSession(sessionId, { mcp_servers: newServers });
      setEnabledServers(newServers);
      updateSession(sessionId, { mcpServers: newServers });
    } catch (err) {
      console.error('Failed to update MCP servers:', err);
    }
  };

  const selectAll = async () => {
    if (!sessionId) return;

    const allServerNames = availableServers.map((s) => s.name);

    try {
      await chatApi.updateChatSession(sessionId, { mcp_servers: allServerNames });
      setEnabledServers(allServerNames);
      updateSession(sessionId, { mcpServers: allServerNames });
    } catch (err) {
      console.error('Failed to update MCP servers:', err);
    }
  };

  const deselectAll = async () => {
    if (!sessionId) return;

    try {
      await chatApi.updateChatSession(sessionId, { mcp_servers: [] });
      setEnabledServers([]);
      updateSession(sessionId, { mcpServers: [] });
    } catch (err) {
      console.error('Failed to update MCP servers:', err);
    }
  };

  return (
    <div className="mcp-control-bar mb-4 relative" onClick={(e) => e.stopPropagation()}>
      <div className="mcp-dropdown">
        <button
          onClick={toggleDropdown}
          className="mcp-dropdown-btn flex items-center gap-2 px-4 py-2 bg-gray-100 dark:bg-gray-800 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors min-h-[44px] text-sm font-medium text-gray-900 dark:text-white"
        >
          <span className="mcp-icon">MCP</span>
          <span className="text-gray-600 dark:text-gray-400">({enabledServers.length})</span>
          <span className="dropdown-arrow ml-2">{showDropdown ? '▲' : '▼'}</span>
        </button>

        {showDropdown && (
          <div className="mcp-dropdown-panel fixed left-4 bottom-20 w-96 max-w-[calc(100vw-2rem)] bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-2xl z-[9999] max-h-96 overflow-hidden flex flex-col">
            <div className="mcp-dropdown-header p-4 border-b border-gray-200 dark:border-gray-700 flex items-start justify-between">
              <div className="mcp-dropdown-title flex-1">
                <h4 className="font-semibold text-gray-900 dark:text-white mb-1">Configure MCP Servers</h4>
                <p className="text-xs text-gray-600 dark:text-gray-400">Select servers for this chat session</p>
              </div>

              <button
                onClick={toggleDropdown}
                className="mcp-dropdown-close min-w-[44px] min-h-[44px] flex items-center justify-center text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 text-2xl"
                title="Close"
              >
                ×
              </button>
            </div>

            {!loading && availableServers.length > 0 && (
              <div className="mcp-bulk-actions p-3 border-b border-gray-200 dark:border-gray-700 flex gap-2">
                <Button onClick={selectAll} variant="secondary" size="sm">
                  Select All
                </Button>
                <Button onClick={deselectAll} variant="secondary" size="sm">
                  Deselect All
                </Button>
              </div>
            )}

            <div className="flex-1 overflow-y-auto">
              {loading ? (
                <div className="mcp-loading p-6 flex items-center justify-center gap-2 text-sm text-gray-600 dark:text-gray-400">
                  <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-blue-600" />
                  <span>Loading...</span>
                </div>
              ) : availableServers.length === 0 ? (
                <div className="mcp-no-servers p-6 text-center text-sm text-gray-500 dark:text-gray-400">
                  <p>No MCP servers available</p>
                </div>
              ) : (
                <div className="mcp-server-list-dropdown p-2">
                  {availableServers.map((server) => (
                    <label
                      key={server.name}
                      className="mcp-server-checkbox-item flex items-center gap-3 p-3 hover:bg-gray-50 dark:hover:bg-gray-700/50 rounded-lg cursor-pointer transition-colors min-h-[44px]"
                    >
                      <Checkbox
                        checked={enabledServers.includes(server.name)}
                        onChange={() => toggleServer(server.name)}
                      />

                      <div className="mcp-server-info-dropdown flex-1">
                        <span className="mcp-server-name-dropdown block text-sm font-medium text-gray-900 dark:text-white">
                          {server.name}
                        </span>
                        {server.tool_count !== undefined && (
                          <span className="mcp-server-tools-count block text-xs text-gray-500 dark:text-gray-400">
                            {server.tool_count} tool{server.tool_count !== 1 ? 's' : ''}
                          </span>
                        )}
                      </div>
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
