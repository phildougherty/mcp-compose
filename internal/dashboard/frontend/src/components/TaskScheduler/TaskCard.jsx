/**
 * TaskCard Component
 * Individual task display with actions and details
 */

import React from 'react';
import { Button, Badge } from '../shared';
import { formatTimestamp, formatDuration } from '../../utils/format';
import { getTaskTypeConfig, formatSchedule, getCronDescription } from './constants';
import useTaskStore from '../../store/taskStore';

const mapColorToBadgeVariant = (color) => {
  const colorMap = {
    green: 'success',
    purple: 'primary',
    blue: 'info',
    yellow: 'warning',
    indigo: 'primary',
    gray: 'default',
  };

  return colorMap[color] || 'default';
};

export default function TaskCard({
  task,
  onRun,
  onToggle,
  onDelete,
  onViewOutput,
  onViewRunOutput,
}) {
  const { isTaskExpanded, toggleTaskExpansion, getLastRun, getRecentRuns } = useTaskStore();
  const expanded = isTaskExpanded(task.id);
  const lastRun = getLastRun(task.id);
  const recentRuns = getRecentRuns(task.id, 5);
  const typeConfig = getTaskTypeConfig(task.type);

  const getStatusBadge = () => {
    if (!task.enabled) return { text: 'Disabled', variant: 'default' };
    if (!lastRun) return { text: 'Never Run', variant: 'primary' };

    switch (lastRun.status) {
      case 'completed':
        return { text: 'Success', variant: 'success' };
      case 'failed':
        return { text: 'Failed', variant: 'danger' };
      case 'running':
        return { text: 'Running', variant: 'warning' };
      default:
        return { text: 'Unknown', variant: 'default' };
    }
  };

  const statusBadge = getStatusBadge();

  const handleCopy = (text) => {
    navigator.clipboard.writeText(text);
  };

  return (
    <div className="relative">
      <button
        onClick={() => toggleTaskExpansion(task.id)}
        className="w-full p-4 sm:p-6 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
        aria-expanded={expanded}
      >
        <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between text-left space-y-3 sm:space-y-0">
          <div className="flex items-start space-x-4 flex-1 min-w-0">
            <div className="flex-shrink-0 relative mt-1">
              <div
                className={`w-3 h-3 rounded-full ${
                  statusBadge.variant === 'success'
                    ? 'bg-green-500'
                    : statusBadge.variant === 'error'
                    ? 'bg-red-500'
                    : statusBadge.variant === 'warning'
                    ? 'bg-yellow-500'
                    : 'bg-gray-400'
                }`}
              />
              {statusBadge.text === 'Running' && (
                <div className="absolute inset-0 w-3 h-3 bg-yellow-400 rounded-full animate-ping opacity-75" />
              )}
            </div>

            <div className="flex-1 min-w-0">
              <div className="flex flex-col sm:flex-row sm:items-center space-y-2 sm:space-y-0 sm:space-x-3 mb-2">
                <h3 className="text-base sm:text-lg font-medium text-gray-900 dark:text-white truncate">
                  {task.name}
                </h3>
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant={mapColorToBadgeVariant(typeConfig.color)}>{typeConfig.label}</Badge>
                  <Badge variant={statusBadge.variant}>{statusBadge.text}</Badge>
                  {!task.enabled && <Badge variant="default">Disabled</Badge>}
                </div>
              </div>

              {task.description && (
                <p className="text-sm text-gray-600 dark:text-gray-400 mb-3">{task.description}</p>
              )}

              <div className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 text-xs sm:text-sm text-gray-500 dark:text-gray-400">
                {task.schedule && (
                  <span className="flex items-center">
                    <svg className="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"
                        clipRule="evenodd"
                      />
                    </svg>
                    <span className="truncate">{formatSchedule(task.schedule)}</span>
                  </span>
                )}
                {lastRun && (
                  <span className="flex items-center">
                    <svg className="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-8.293l-3-3a1 1 0 00-1.414 1.414L10.586 9H7a1 1 0 100 2h3.586l-1.293 1.293a1 1 0 101.414 1.414l3-3a1 1 0 000-1.414z"
                        clipRule="evenodd"
                      />
                    </svg>
                    <span className="truncate">Last: {formatTimestamp(lastRun.last_run)}</span>
                  </span>
                )}
                {task.type === 'ai' && task.modelHint && (
                  <span className="flex items-center">
                    <svg className="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                    </svg>
                    <span className="truncate">{task.modelHint}</span>
                  </span>
                )}
                {task.type === 'ai' && task.mcpServers && task.mcpServers.length > 0 && (
                  <span className="flex items-center">
                    <svg className="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z" clipRule="evenodd" />
                    </svg>
                    <span className="truncate">{task.mcpServers.join(', ')}</span>
                  </span>
                )}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-1.5 sm:gap-2" onClick={(e) => e.stopPropagation()}>
            <Button
              size="small"
              variant="success"
              onClick={() => onRun(task.id)}
              disabled={!task.enabled}
              title={task.enabled ? 'Run task now' : 'Task is disabled'}
            >
              <svg className="w-3.5 h-3.5 mr-1" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z"
                  clipRule="evenodd"
                />
              </svg>
              Run
            </Button>
            <Button size="small" variant="primary" onClick={() => onViewOutput(task.id)} title="View output">
              <svg className="w-3.5 h-3.5 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                />
              </svg>
              View
            </Button>
            <Button
              size="small"
              variant={task.enabled ? 'warning' : 'success'}
              onClick={() => onToggle(task.id)}
              title={task.enabled ? 'Disable task' : 'Enable task'}
            >
              {task.enabled ? (
                <svg className="w-3.5 h-3.5 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M13.477 14.89A6 6 0 015.11 6.524l8.367 8.368zm1.414-1.414L6.524 5.11a6 6 0 018.367 8.367zM18 10a8 8 0 11-16 0 8 8 0 0116 0z"
                    clipRule="evenodd"
                  />
                </svg>
              ) : (
                <svg className="w-3.5 h-3.5 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clipRule="evenodd"
                  />
                </svg>
              )}
              {task.enabled ? 'Disable' : 'Enable'}
            </Button>
            <Button
              size="small"
              variant="error"
              onClick={() => onDelete(task.id)}
              title="Delete task"
              className="!px-2 !min-w-[36px]"
            >
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M9 2a1 1 0 00-2 0v1a1 1 0 002 0V2zm-4 3a1 1 0 011-1h2a1 1 0 010 2H6a1 1 0 01-1-1zm-2 4a2 2 0 012-2h10a2 2 0 012 2v7a2 2 0 01-2 2H5a2 2 0 01-2-2V9zm3 1a1 1 0 011 1v3a1 1 0 11-2 0v-3a1 1 0 011-1zm4 0a1 1 0 011 1v3a1 1 0 11-2 0v-3a1 1 0 011-1zm4 0a1 1 0 011 1v3a1 1 0 11-2 0v-3a1 1 0 011-1z"
                  clipRule="evenodd"
                />
              </svg>
            </Button>
            <div className="hidden sm:block w-px h-6 bg-gray-200 dark:bg-gray-600 mx-2" />
            <svg
              className={`w-5 h-5 text-gray-400 transition-transform duration-200 ${
                expanded ? 'rotate-180' : ''
              }`}
              fill="currentColor"
              viewBox="0 0 20 20"
            >
              <path
                fillRule="evenodd"
                d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                clipRule="evenodd"
              />
            </svg>
          </div>
        </div>
      </button>

      {expanded && (
        <div className="px-4 sm:px-6 pb-6 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-100 dark:border-gray-700">
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 pt-6">
            <div className="space-y-4">
              <h4 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wide flex items-center">
                <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z"
                    clipRule="evenodd"
                  />
                </svg>
                Configuration
              </h4>

              {(task.command || task.prompt) && (
                <div className="bg-gray-900 dark:bg-gray-900 rounded-lg p-4 font-mono text-sm">
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-xs font-medium text-gray-400 uppercase tracking-wide">
                      {task.type === 'ai' ? 'AI Prompt' : 'Command'}
                    </span>
                    <Button
                      size="xs"
                      variant="ghost"
                      onClick={() => handleCopy(task.command || task.prompt)}
                      className="!text-gray-400 hover:!text-gray-300 hover:!bg-gray-800"
                    >
                      <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                        />
                      </svg>
                      Copy
                    </Button>
                  </div>
                  <div className="text-gray-300 whitespace-pre-wrap break-words">
                    {task.type === 'shell' && <span className="text-green-400">$ </span>}
                    {task.type === 'ai' && <span className="text-purple-400">AI: </span>}
                    {task.command || task.prompt}
                  </div>
                </div>
              )}

              <div className="bg-white dark:bg-gray-700 rounded-lg p-4 space-y-3 border border-gray-200 dark:border-gray-600">
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                  <div>
                    <span className="font-medium text-gray-500 dark:text-gray-400">Type:</span>
                    <span className="ml-2 text-gray-900 dark:text-gray-100">{typeConfig.label}</span>
                  </div>
                  <div>
                    <span className="font-medium text-gray-500 dark:text-gray-400">Status:</span>
                    <span className="ml-2 text-gray-900 dark:text-gray-100">
                      {task.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>

                  {task.schedule && (
                    <div className="sm:col-span-2">
                      <span className="font-medium text-gray-500 dark:text-gray-400">Schedule:</span>
                      <div className="mt-1">
                        <div className="text-gray-900 dark:text-gray-100 font-medium">
                          {formatSchedule(task.schedule)}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1 bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                          {task.schedule}
                        </div>
                        <div className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                          {getCronDescription(task.schedule)}
                        </div>
                      </div>
                    </div>
                  )}

                  {task.type === 'ai' && task.mcpServers && task.mcpServers.length > 0 && (
                    <div className="sm:col-span-2">
                      <span className="font-medium text-gray-500 dark:text-gray-400">MCP Servers:</span>
                      <div className="mt-1 flex flex-wrap gap-2">
                        {task.mcpServers.map((server, idx) => (
                          <Badge key={idx} variant="info" className="font-mono text-xs">
                            {server}
                          </Badge>
                        ))}
                      </div>
                    </div>
                  )}

                  {task.type === 'ai' && task.modelHint && (
                    <div>
                      <span className="font-medium text-gray-500 dark:text-gray-400">Model Hint:</span>
                      <span className="ml-2 text-gray-900 dark:text-gray-100">{task.modelHint}</span>
                    </div>
                  )}
                  {task.type === 'ai' && task.maxCost && (
                    <div>
                      <span className="font-medium text-gray-500 dark:text-gray-400">Max Cost:</span>
                      <span className="ml-2 text-gray-900 dark:text-gray-100">${task.maxCost}</span>
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div className="space-y-4">
              <h4 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wide flex items-center">
                <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"
                  />
                </svg>
                Recent Runs
              </h4>

              {recentRuns.length === 0 ? (
                <div className="bg-white dark:bg-gray-700 rounded-lg p-6 text-center border border-gray-200 dark:border-gray-600">
                  <svg
                    className="w-10 h-10 mx-auto mb-3 text-gray-400 dark:text-gray-500"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                  <p className="text-sm font-medium text-gray-900 dark:text-gray-100">No runs yet</p>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Task executions will appear here once the task runs
                  </p>
                </div>
              ) : (
                <div className="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600">
                  <div className="p-4">
                    <div className="space-y-3 max-h-64 overflow-y-auto">
                      {recentRuns.map((run) => (
                        <div
                          key={run.id || run.timestamp}
                          className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-600"
                        >
                          <div className="flex items-center space-x-3 flex-1 min-w-0">
                            <div
                              className={`w-2 h-2 rounded-full flex-shrink-0 ${
                                run.status === 'completed'
                                  ? 'bg-green-500'
                                  : run.status === 'failed'
                                  ? 'bg-red-500'
                                  : run.status === 'running'
                                  ? 'bg-yellow-500 animate-pulse'
                                  : 'bg-gray-400'
                              }`}
                            />
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center space-x-2 mb-1">
                                <span className="text-sm font-medium text-gray-900 dark:text-gray-100">
                                  {formatTimestamp(run.timestamp || run.last_run || run.lastRun || run.started_at)}
                                </span>
                                <Badge
                                  variant={
                                    run.status === 'completed'
                                      ? 'success'
                                      : run.status === 'failed'
                                      ? 'danger'
                                      : run.status === 'running'
                                      ? 'warning'
                                      : 'default'
                                  }
                                >
                                  {run.status}
                                </Badge>
                              </div>
                              {run.duration && (
                                <div className="text-xs text-gray-500 dark:text-gray-400">
                                  Duration: {formatDuration(run.duration)}
                                </div>
                              )}
                            </div>
                          </div>
                          <Button size="small" variant="primary" onClick={() => onViewRunOutput(task.id, run.id)}>
                            <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path
                                strokeLinecap="round"
                                strokeLinejoin="round"
                                strokeWidth={2}
                                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                              />
                            </svg>
                            View
                          </Button>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
