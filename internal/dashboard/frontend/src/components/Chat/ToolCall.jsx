import React, { useState } from 'react';
import { Badge } from '../shared';
import clsx from 'clsx';

export default function ToolCall({ toolCalls, toolResults }) {
  const [expandedSection, setExpandedSection] = useState(false);
  const [expandedResults, setExpandedResults] = useState({});

  const toggleSection = () => {
    setExpandedSection(!expandedSection);
  };

  const toggleResult = (index) => {
    setExpandedResults((prev) => ({
      ...prev,
      [index]: !prev[index],
    }));
  };

  const getToolStatusClass = (index) => {
    if (!toolResults || !toolResults[index]) {
      return 'pending';
    }

    const result = toolResults[index];
    if (result.error || result.Error) {
      return 'error';
    }

    return 'success';
  };

  const getToolStatusIcon = (index) => {
    if (!toolResults || !toolResults[index]) {
      return '○';
    }

    const result = toolResults[index];
    if (result.error || result.Error) {
      return '✗';
    }

    return '✓';
  };

  const formatToolResult = (result) => {
    if (typeof result === 'string') {
      try {
        const parsed = JSON.parse(result);
        if (parsed && parsed.content && Array.isArray(parsed.content)) {
          return parsed.content.map((c) => c.text || JSON.stringify(c)).join('\n');
        }
        return result;
      } catch (e) {
        return result;
      }
    }

    if (result && result.content && Array.isArray(result.content)) {
      return result.content.map((c) => c.text || JSON.stringify(c)).join('\n');
    }

    return JSON.stringify(result, null, 2);
  };

  const formatDuration = (duration) => {
    if (!duration) return '0ms';
    if (typeof duration === 'string') return duration;
    if (duration < 1000) return Math.round(duration) + 'ms';
    return (duration / 1000).toFixed(2) + 's';
  };

  return (
    <div className="tool-execution-section rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
      <button
        onClick={toggleSection}
        className="tool-accordion-header w-full flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors min-h-[44px]"
      >
        <span className="tool-status-indicator">
          <svg className="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </span>

        <span className="tool-accordion-icon text-gray-600 dark:text-gray-400">
          {expandedSection ? '▼' : '▶'}
        </span>

        <span className="tool-accordion-title flex-1 text-left text-sm font-medium text-gray-900 dark:text-white">
          <strong>{toolCalls.length}</strong> tool call{toolCalls.length !== 1 ? 's' : ''} executed
        </span>

        <Badge variant="secondary" className="tool-count-badge">
          {toolCalls.length}
        </Badge>
      </button>

      {expandedSection && (
        <div className="tool-accordion-content p-3 space-y-3">
          {toolCalls.map((call, index) => (
            <div key={index} className="tool-call-item border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden">
              <div className="tool-call-header flex items-center gap-3 p-3 bg-white dark:bg-gray-900">
                <span
                  className={clsx(
                    'tool-status-icon w-6 h-6 flex items-center justify-center rounded-full text-sm font-bold',
                    getToolStatusClass(index) === 'success' && 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400',
                    getToolStatusClass(index) === 'error' && 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400',
                    getToolStatusClass(index) === 'pending' && 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
                  )}
                >
                  {getToolStatusIcon(index)}
                </span>

                <span className="tool-name flex-1 font-medium text-gray-900 dark:text-white">
                  {call.name || call.Name}
                </span>

                <span className="tool-index text-xs text-gray-500 dark:text-gray-400">
                  #{index + 1}
                </span>
              </div>

              {(call.args || call.Args) && (
                <div className="tool-call-args p-3 bg-gray-50 dark:bg-gray-800 border-t border-gray-200 dark:border-gray-700">
                  <strong className="text-xs text-gray-700 dark:text-gray-300 block mb-2">Arguments:</strong>
                  <pre className="text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap bg-white dark:bg-gray-900 p-2 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto">
                    {JSON.stringify(call.args || call.Args, null, 2)}
                  </pre>
                </div>
              )}

              {toolResults && toolResults[index] && (
                <div className="tool-result-section border-t border-gray-200 dark:border-gray-700">
                  <button
                    onClick={() => toggleResult(index)}
                    className="tool-result-toggle w-full flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors min-h-[44px]"
                  >
                    <span className="tool-result-icon text-gray-600 dark:text-gray-400">
                      {expandedResults[index] ? '▼' : '▶'}
                    </span>

                    <strong className="text-sm text-gray-900 dark:text-white">Result</strong>

                    <span className="tool-result-meta-inline text-xs text-gray-500 dark:text-gray-400 ml-auto">
                      {formatDuration(toolResults[index].duration || toolResults[index].Duration)}
                    </span>
                  </button>

                  {expandedResults[index] && (
                    <div className="p-3">
                      {(toolResults[index].error || toolResults[index].Error) ? (
                        <div className="tool-result-error p-3 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded text-sm text-red-800 dark:text-red-200">
                          <strong>Error:</strong> {toolResults[index].error || toolResults[index].Error}
                        </div>
                      ) : (
                        <div className="tool-result-success">
                          <pre className="text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap bg-white dark:bg-gray-900 p-3 rounded border border-gray-200 dark:border-gray-700 overflow-x-auto">
                            {formatToolResult(toolResults[index].result || toolResults[index].Result)}
                          </pre>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
