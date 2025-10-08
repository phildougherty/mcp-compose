import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { SparklesIcon } from '@heroicons/react/24/outline';

const AITaskNode = ({ data, isConnectable }) => {
  const getStatusColor = () => {
    switch (data.status) {
      case 'running':
        return 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/20';
      case 'success':
        return 'border-green-400 bg-green-50 dark:bg-green-900/20';
      case 'error':
        return 'border-red-400 bg-red-50 dark:bg-red-900/20';
      default:
        return 'border-purple-400 bg-purple-50 dark:bg-purple-900/20';
    }
  };

  return (
    <div className={`px-4 py-3 shadow-lg rounded-lg border-2 transition-all duration-200 min-w-[220px] ${getStatusColor()}`}>
      <Handle
        type="target"
        position={Position.Left}
        isConnectable={isConnectable}
        className="!bg-purple-500 !w-3 !h-3 !border-2 !border-white"
      />

      <div className="flex items-center gap-2 mb-2">
        <SparklesIcon className="w-5 h-5 text-purple-600 dark:text-purple-400 flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="font-bold text-gray-900 dark:text-white text-sm truncate">
            {data.label || 'AI Task'}
          </div>
        </div>
      </div>

      <div className="text-xs text-gray-600 dark:text-gray-300 space-y-1">
        {data.config?.provider && data.config?.model && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Model:</span>
            <span className="font-mono truncate">
              {data.config.provider}/{data.config.model}
            </span>
          </div>
        )}
        {data.config?.prompt && (
          <div className="mt-2">
            <div className="font-medium mb-1">Prompt:</div>
            <div className="text-xs bg-white dark:bg-gray-800 p-2 rounded border border-purple-200 dark:border-purple-700 max-h-16 overflow-y-auto">
              {data.config.prompt.length > 100
                ? `${data.config.prompt.substring(0, 100)}...`
                : data.config.prompt}
            </div>
          </div>
        )}
      </div>

      {data.lastRun && (
        <div className="mt-2 pt-2 border-t border-purple-200 dark:border-purple-700 text-xs text-gray-500 dark:text-gray-400">
          Last: {new Date(data.lastRun).toLocaleString()}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        isConnectable={isConnectable}
        className="!bg-purple-500 !w-3 !h-3 !border-2 !border-white"
      />
    </div>
  );
};

export default AITaskNode;
