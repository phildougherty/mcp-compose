import React, { useState } from 'react';
import ReactMarkdown from 'react-markdown';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import ToolCall from './ToolCall';
import clsx from 'clsx';
import { copyToClipboard } from '../../utils';
import { useToast } from '../shared/Toast';

export default function Message({ message, isStreaming = false, onRegenerate, messageIndex }) {
  const [copySuccess, setCopySuccess] = useState(false);
  const { toast } = useToast();
  const formatTime = (dateString) => {
    const date = new Date(dateString);
    return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
  };

  const formatCost = (cost) => {
    return cost?.toFixed(4) || '0.0000';
  };

  const handleCopy = async () => {
    try {
      const success = await copyToClipboard(message.content);

      if (success) {
        setCopySuccess(true);
        toast.success('Copied to clipboard');
        setTimeout(() => setCopySuccess(false), 2000);
      } else {
        toast.error('Failed to copy');
      }
    } catch (err) {
      toast.error('Failed to copy');
    }
  };

  const handleRegenerate = () => {
    if (onRegenerate && messageIndex !== undefined) {
      onRegenerate(messageIndex);
    }
  };

  const isAutomated = message.is_automated || false;

  return (
    <div className={clsx(
      'message flex gap-2 sm:gap-4 w-full max-w-full',
      message.role === 'user' ? 'justify-end' : 'justify-start'
    )}>
      {message.role !== 'user' && (
        <div className={clsx(
          "message-avatar flex-shrink-0 w-8 h-8 sm:w-10 sm:h-10 rounded-full flex items-center justify-center text-white font-semibold text-xs sm:text-sm",
          isAutomated ? 'bg-purple-600' : 'bg-blue-600'
        )}>
          {isAutomated ? (
            <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 24 24">
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-5.5-2.5l7.51-3.49L17.5 6.5 9.99 9.99 6.5 17.5zm5.5-6.6c.61 0 1.1.49 1.1 1.1s-.49 1.1-1.1 1.1-1.1-.49-1.1-1.1.49-1.1 1.1-1.1z"/>
            </svg>
          ) : (
            'AI'
          )}
        </div>
      )}

      <div className={clsx(
        'message-content max-w-[85%] sm:max-w-3xl break-words',
        message.role === 'user' && 'order-first'
      )}>
        <div className="message-header flex items-center gap-2 mb-1.5">
          <span className="message-role text-xs sm:text-sm font-medium text-gray-900 dark:text-white">
            {message.role === 'user' ? 'You' : 'Assistant'}
          </span>
          {isAutomated && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-purple-700 bg-purple-100 dark:text-purple-300 dark:bg-purple-900/30 rounded-full">
              <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 24 24">
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8zm-5.5-2.5l7.51-3.49L17.5 6.5 9.99 9.99 6.5 17.5zm5.5-6.6c.61 0 1.1.49 1.1 1.1s-.49 1.1-1.1 1.1-1.1-.49-1.1-1.1.49-1.1 1.1-1.1z"/>
              </svg>
              Scheduled Agent
            </span>
          )}
          {message.created_at && !isStreaming && (
            <span className="message-time text-xs text-gray-500 dark:text-gray-400">
              {formatTime(message.created_at)}
            </span>
          )}
        </div>

        <div
          className={clsx(
            'message-text rounded-2xl p-3 sm:p-4 break-words overflow-hidden',
            message.role === 'user'
              ? 'bg-blue-600 text-white rounded-br-sm'
              : isAutomated
              ? 'bg-purple-50 dark:bg-purple-900/10 text-gray-900 dark:text-white rounded-bl-sm border-l-4 border-purple-500'
              : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-white rounded-bl-sm'
          )}
        >
          {message.role === 'user' ? (
            <div className="whitespace-pre-wrap break-words text-sm sm:text-base">{message.content}</div>
          ) : (
            <ReactMarkdown
              className="prose prose-sm sm:prose max-w-none dark:prose-invert"
              components={{
                code({ node, inline, className, children, ...props }) {
                  const match = /language-(\w+)/.exec(className || '');

                  return !inline && match ? (
                    <div className="overflow-x-auto max-w-full my-2">
                      <SyntaxHighlighter
                        style={vscDarkPlus}
                        language={match[1]}
                        PreTag="div"
                        className="rounded-lg text-xs sm:text-sm"
                        {...props}
                      >
                        {String(children).replace(/\n$/, '')}
                      </SyntaxHighlighter>
                    </div>
                  ) : (
                    <code
                      className="bg-gray-200 dark:bg-gray-700 px-1.5 py-0.5 rounded text-xs sm:text-sm font-mono break-all"
                      {...props}
                    >
                      {children}
                    </code>
                  );
                },
                p({ children }) {
                  return <p className="mb-2 last:mb-0 break-words text-sm sm:text-base">{children}</p>;
                },
                ul({ children }) {
                  return <ul className="list-disc list-inside mb-2 space-y-1 text-sm sm:text-base">{children}</ul>;
                },
                ol({ children }) {
                  return <ol className="list-decimal list-inside mb-2 space-y-1 text-sm sm:text-base">{children}</ol>;
                },
                a({ href, children }) {
                  return (
                    <a
                      href={href}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-blue-400 hover:text-blue-300 underline break-all"
                    >
                      {children}
                    </a>
                  );
                },
              }}
            >
              {message.content || ''}
            </ReactMarkdown>
          )}
        </div>

        {message.tool_calls && message.tool_calls.length > 0 && (
          <div className="tool-execution-section mt-3">
            <ToolCall toolCalls={message.tool_calls} toolResults={message.tool_results} />
          </div>
        )}

        {message.tokens_used && !isStreaming && (
          <div className="message-meta mt-2 flex flex-wrap gap-2 sm:gap-3 text-xs text-gray-500 dark:text-gray-400">
            <span>{message.tokens_used} tokens</span>
            {message.cost_estimate && (
              <span>• ${formatCost(message.cost_estimate)}</span>
            )}
          </div>
        )}

        {message.role === 'assistant' && !isStreaming && (
          <div className="message-actions mt-2 flex items-center gap-2">
            <button
              onClick={handleCopy}
              className="action-btn p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
              title="Copy to clipboard"
              aria-label="Copy message to clipboard"
            >
              {copySuccess ? (
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                </svg>
              ) : (
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              )}
            </button>

            {onRegenerate && messageIndex !== undefined && (
              <button
                onClick={handleRegenerate}
                className="action-btn p-1.5 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200 hover:bg-gray-200 dark:hover:bg-gray-700 rounded transition-colors"
                title="Regenerate response"
                aria-label="Regenerate this response"
              >
                <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
              </button>
            )}
          </div>
        )}
      </div>

      {message.role === 'user' && (
        <div className="message-avatar flex-shrink-0 w-8 h-8 sm:w-10 sm:h-10 rounded-full bg-gray-600 flex items-center justify-center text-white font-semibold text-xs sm:text-sm">
          U
        </div>
      )}
    </div>
  );
}
