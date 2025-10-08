import React from 'react';
import {
  ClockIcon,
  CheckCircleIcon,
  XCircleIcon,
  PlayCircleIcon,
  StopCircleIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';

const ExecutionPanel = ({ execution, workflow }) => {
  if (!execution) {
    return null;
  }

  const getStatusIcon = (status) => {
    switch (status) {
      case 'running':
        return <PlayCircleIcon className="h-5 w-5 text-yellow-500 animate-pulse" />;
      case 'completed':
        return <CheckCircleIcon className="h-5 w-5 text-green-500" />;
      case 'error':
        return <XCircleIcon className="h-5 w-5 text-red-500" />;
      case 'stopped':
        return <StopCircleIcon className="h-5 w-5 text-gray-500" />;
      default:
        return <ClockIcon className="h-5 w-5 text-gray-400" />;
    }
  };

  const getStatusBadge = (status) => {
    const baseClasses = "px-2 py-1 rounded-full text-xs font-semibold uppercase tracking-wide";

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

  const formatDuration = (startTime, endTime) => {
    if (!startTime) {

      return 'N/A';
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

  const formatTime = (timestamp) => {
    if (!timestamp) {

      return 'N/A';
    }

    return new Date(timestamp).toLocaleTimeString();
  };

  const calculateProgress = () => {
    if (!execution.totalNodes) {

      return 0;
    }

    return Math.round((execution.completedNodes / execution.totalNodes) * 100);
  };

  return (
    <div className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-4 w-80">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-sm font-bold text-gray-900 dark:text-white uppercase tracking-wider">
          Execution Details
        </h3>
        {getStatusIcon(execution.status)}
      </div>

      <div className="space-y-3">
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Execution ID</div>
          <div className="text-sm font-mono text-gray-900 dark:text-white truncate">
            {execution.id}
          </div>
        </div>

        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Status</div>
          <span className={getStatusBadge(execution.status)}>
            {execution.status}
          </span>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Start Time</div>
            <div className="text-sm text-gray-900 dark:text-white">
              {formatTime(execution.startTime)}
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Duration</div>
            <div className="text-sm text-gray-900 dark:text-white">
              {formatDuration(execution.startTime, execution.endTime)}
            </div>
          </div>
        </div>

        <div>
          <div className="flex items-center justify-between text-xs text-gray-500 dark:text-gray-400 mb-2">
            <span>Progress</span>
            <span className="font-semibold">
              {execution.completedNodes || 0} / {execution.totalNodes || 0} nodes
            </span>
          </div>

          <div className="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2">
            <div
              className="bg-blue-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${calculateProgress()}%` }}
            />
          </div>

          <div className="text-xs text-gray-500 dark:text-gray-400 mt-1 text-right">
            {calculateProgress()}%
          </div>
        </div>

        {execution.currentNode && (
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Current Node</div>
            <div className="text-sm font-medium text-gray-900 dark:text-white flex items-center">
              <div className="w-2 h-2 bg-yellow-500 rounded-full mr-2 animate-pulse" />
              {workflow.nodes.find(n => n.id === execution.currentNode)?.data.label || execution.currentNode}
            </div>
          </div>
        )}

        {execution.errors && execution.errors.length > 0 && (
          <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
            <div className="flex items-center text-xs text-red-600 dark:text-red-400 mb-2">
              <ExclamationTriangleIcon className="h-4 w-4 mr-1" />
              <span className="font-semibold">Errors</span>
            </div>
            <div className="space-y-2">
              {execution.errors.map((error, index) => (
                <div
                  key={index}
                  className="text-xs text-red-700 dark:text-red-300 bg-red-50 dark:bg-red-900/20 p-2 rounded"
                >
                  {error}
                </div>
              ))}
            </div>
          </div>
        )}

        {execution.nodeOutputs && Object.keys(execution.nodeOutputs).length > 0 && (
          <div className="mt-4 pt-4 border-t border-gray-200 dark:border-gray-700">
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-2 font-semibold">
              Node Outputs
            </div>
            <div className="space-y-2 max-h-40 overflow-y-auto">
              {Object.entries(execution.nodeOutputs).map(([nodeId, output]) => {
                const node = workflow.nodes.find(n => n.id === nodeId);

                return (
                  <details key={nodeId} className="text-xs">
                    <summary className="cursor-pointer text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white">
                      {node?.data.label || nodeId}
                    </summary>
                    <pre className="mt-1 p-2 bg-gray-100 dark:bg-gray-900 rounded text-xs overflow-x-auto">
                      {typeof output === 'string' ? output : JSON.stringify(output, null, 2)}
                    </pre>
                  </details>
                );
              })}
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default ExecutionPanel;
