import React from 'react';
import {
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
  StopCircleIcon,
  PlayCircleIcon,
} from '@heroicons/react/24/outline';

const ExecutionHistory = ({ executions, selectedExecution, onExecutionSelect }) => {
  const getStatusIcon = (status) => {
    const iconClass = "h-4 w-4";

    switch (status) {
      case 'running':
        return <PlayCircleIcon className={`${iconClass} text-yellow-500`} />;
      case 'completed':
        return <CheckCircleIcon className={`${iconClass} text-green-500`} />;
      case 'error':
        return <XCircleIcon className={`${iconClass} text-red-500`} />;
      case 'stopped':
        return <StopCircleIcon className={`${iconClass} text-gray-500`} />;
      default:
        return <ClockIcon className={`${iconClass} text-gray-400`} />;
    }
  };

  const getStatusBadge = (status) => {
    const baseClasses = "px-2 py-0.5 rounded text-xs font-medium";

    switch (status) {
      case 'running':
        return `${baseClasses} bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200`;
      case 'completed':
        return `${baseClasses} bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200`;
      case 'error':
        return `${baseClasses} bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200`;
      case 'stopped':
        return `${baseClasses} bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200`;
      default:
        return `${baseClasses} bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300`;
    }
  };

  const formatTime = (timestamp) => {
    if (!timestamp) {

      return 'N/A';
    }

    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffDays > 0) {

      return `${diffDays}d ago`;
    }

    if (diffHours > 0) {

      return `${diffHours}h ago`;
    }

    if (diffMins > 0) {

      return `${diffMins}m ago`;
    }

    return 'Just now';
  };

  const formatDuration = (startTime, endTime) => {
    if (!startTime) {

      return '';
    }

    const start = new Date(startTime);
    const end = endTime ? new Date(endTime) : new Date();
    const durationMs = end - start;

    const seconds = Math.floor(durationMs / 1000);
    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    if (minutes > 0) {

      return `${minutes}m ${remainingSeconds}s`;
    }

    return `${remainingSeconds}s`;
  };

  const sortedExecutions = [...executions].sort((a, b) => {
    return new Date(b.startTime) - new Date(a.startTime);
  });

  return (
    <div className="h-full flex flex-col">
      <div className="px-4 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
          Execution History
        </h2>
        <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
          {executions.length} {executions.length === 1 ? 'execution' : 'executions'}
        </p>
      </div>

      <div className="flex-1 overflow-y-auto">
        {sortedExecutions.length === 0 ? (
          <div className="p-4 text-center">
            <ClockIcon className="h-12 w-12 mx-auto text-gray-300 dark:text-gray-600 mb-2" />
            <p className="text-sm text-gray-500 dark:text-gray-400">
              No executions yet
            </p>
            <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
              Start an execution to see it here
            </p>
          </div>
        ) : (
          <div className="divide-y divide-gray-200 dark:divide-gray-700">
            {sortedExecutions.map((exec) => (
              <button
                key={exec.id}
                onClick={() => onExecutionSelect(exec)}
                className={`
                  w-full p-4 text-left transition-colors
                  hover:bg-gray-50 dark:hover:bg-gray-700
                  ${selectedExecution?.id === exec.id
                    ? 'bg-blue-50 dark:bg-blue-900/20 border-l-4 border-blue-500'
                    : ''
                  }
                `}
              >
                <div className="flex items-start justify-between mb-2">
                  <div className="flex items-center space-x-2">
                    {getStatusIcon(exec.status)}
                    <span className={getStatusBadge(exec.status)}>
                      {exec.status}
                    </span>
                  </div>
                  <span className="text-xs text-gray-500 dark:text-gray-400">
                    {formatTime(exec.startTime)}
                  </span>
                </div>

                <div className="text-xs font-mono text-gray-600 dark:text-gray-400 mb-2 truncate">
                  {exec.id}
                </div>

                <div className="flex items-center justify-between text-xs">
                  <span className="text-gray-600 dark:text-gray-400">
                    Duration: {formatDuration(exec.startTime, exec.endTime)}
                  </span>
                  <span className="text-gray-600 dark:text-gray-400">
                    {exec.completedNodes || 0}/{exec.totalNodes || 0} nodes
                  </span>
                </div>

                {exec.errors && exec.errors.length > 0 && (
                  <div className="mt-2 text-xs text-red-600 dark:text-red-400 flex items-center">
                    <XCircleIcon className="h-3 w-3 mr-1" />
                    {exec.errors.length} {exec.errors.length === 1 ? 'error' : 'errors'}
                  </div>
                )}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default ExecutionHistory;
