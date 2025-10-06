import React, { useEffect, useRef, useState } from 'react';
import Message from './Message';
import { EmptyState } from '../shared';
import { chatApi } from '../../api';

export default function MessageList({ messages, streamingContent, isStreaming, onSendMessage, onRegenerate, activeSession }) {
  const messagesEndRef = useRef(null);
  const containerRef = useRef(null);
  const [availableServers, setAvailableServers] = useState([]);

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, streamingContent]);

  useEffect(() => {
    loadMCPServers();
  }, []);

  const loadMCPServers = async () => {
    try {
      const data = await chatApi.getMCPServers();
      setAvailableServers(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('Failed to load MCP servers:', err);
      setAvailableServers([]);
    }
  };

  const handleSuggestion = (text) => {
    if (onSendMessage) {
      onSendMessage(text);
    }
  };

  const generateSuggestions = () => {
    const suggestions = [];
    const enabledServers = activeSession?.mcpServers || [];

    if (availableServers.some(s => s.name === 'filesystem' && enabledServers.includes('filesystem'))) {
      suggestions.push(
        {
          text: 'Create a new React component called UserProfile with TypeScript',
          label: '🎨 Create component'
        },
        {
          text: 'Find all TODO comments in my project',
          label: '📝 Find TODOs'
        }
      );
    }

    if (availableServers.some(s => s.name === 'github' && enabledServers.includes('github'))) {
      suggestions.push(
        {
          text: 'Create a new GitHub issue for the bug I just described',
          label: 'Create issue'
        },
        {
          text: 'Show my open pull requests across all repos',
          label: 'My PRs'
        }
      );
    }

    if (availableServers.some(s => s.name === 'memory' && enabledServers.includes('memory'))) {
      suggestions.push(
        {
          text: 'Remember that our staging server is at staging.example.com with API key xyz',
          label: 'Save to memory'
        },
        {
          text: 'What do you remember about our authentication system?',
          label: 'Recall info'
        }
      );
    }

    if (availableServers.some(s => s.name === 'postgres' && enabledServers.includes('postgres'))) {
      suggestions.push(
        {
          text: 'Show me the schema for the users table',
          label: 'Table schema'
        },
        {
          text: 'How many rows are in the orders table?',
          label: 'Count rows'
        }
      );
    }

    if (availableServers.some(s => s.name === 'brave-search' && enabledServers.includes('brave-search'))) {
      suggestions.push(
        {
          text: 'Search for the latest React 19 features and best practices',
          label: 'Search web'
        }
      );
    }

    if (availableServers.some(s => s.name === 'slack' && enabledServers.includes('slack'))) {
      suggestions.push(
        {
          text: 'Send a message to #engineering about the deployment',
          label: 'Send to Slack'
        }
      );
    }

    if (suggestions.length === 0) {
      suggestions.push(
        { text: 'Write a Python function that validates email addresses with regex', label: 'Write Python' },
        { text: 'Explain the difference between REST and GraphQL APIs', label: 'Explain APIs' },
        { text: 'Help me debug why my React component is re-rendering too often', label: 'Debug React' }
      );
    }

    return suggestions.slice(0, 3);
  };

  const suggestions = generateSuggestions();
  const enabledServers = activeSession?.mcpServers || [];

  return (
    <div
      ref={containerRef}
      className="messages-container flex-1 overflow-y-auto p-4 space-y-4"
    >
      {messages.length === 0 && !isStreaming ? (
        <EmptyState
          icon={
            <svg className="w-16 h-16 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
          }
          title="Start a Conversation"
          description={
            enabledServers.length > 0
              ? `You have ${enabledServers.length} MCP server${enabledServers.length !== 1 ? 's' : ''} enabled. Try these:`
              : "Ask me anything, or try these:"
          }
          action={
            <div className="suggestion-chips flex flex-wrap gap-2 justify-center max-w-2xl mx-auto">
              {suggestions.map((suggestion, index) => (
                <button
                  key={index}
                  onClick={() => handleSuggestion(suggestion.text)}
                  className="suggestion-chip px-4 py-2 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300 rounded-lg hover:bg-blue-100 dark:hover:bg-blue-900/40 transition-colors text-sm min-h-[44px] active:scale-95"
                >
                  {suggestion.label}
                </button>
              ))}
            </div>
          }
        />
      ) : (
        <>
          {messages.map((message, index) => (
            <Message
              key={message.id}
              message={message}
              messageIndex={index}
              onRegenerate={onRegenerate}
            />
          ))}

          {isStreaming && streamingContent && (
            <Message message={{ content: streamingContent, role: 'assistant' }} isStreaming />
          )}
        </>
      )}

      <div ref={messagesEndRef} />
    </div>
  );
}
