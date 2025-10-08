import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { CodeBracketIcon } from '@heroicons/react/24/outline';

const CodeNode = ({ data, isConnectable }) => {
  const getStatusColor = () => {
    switch (data.status) {
      case 'running':
        return 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/20';
      case 'success':
        return 'border-green-400 bg-green-50 dark:bg-green-900/20';
      case 'error':
        return 'border-red-400 bg-red-50 dark:bg-red-900/20';
      default:
        return 'border-gray-400 bg-gray-50 dark:bg-gray-900/20';
    }
  };

  const getLanguageColor = (language) => {
    switch (language?.toLowerCase()) {
      case 'javascript':
      case 'js':
        return 'text-yellow-600 dark:text-yellow-400';
      case 'python':
      case 'py':
        return 'text-blue-600 dark:text-blue-400';
      case 'shell':
      case 'bash':
        return 'text-green-600 dark:text-green-400';
      default:
        return 'text-gray-600 dark:text-gray-400';
    }
  };

  return (
    <div className={`px-4 py-3 shadow-lg rounded-lg border-2 transition-all duration-200 min-w-[200px] ${getStatusColor()}`}>
      <Handle
        type="target"
        position={Position.Left}
        isConnectable={isConnectable}
        className="!bg-gray-500 !w-3 !h-3 !border-2 !border-white"
      />

      <div className="flex items-center gap-2 mb-2">
        <CodeBracketIcon className="w-5 h-5 text-gray-600 dark:text-gray-400 flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="font-bold text-gray-900 dark:text-white text-sm truncate">
            {data.label || 'Code'}
          </div>
        </div>
      </div>

      <div className="text-xs text-gray-600 dark:text-gray-300 space-y-1">
        {data.config?.language && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Language:</span>
            <span className={`font-mono ${getLanguageColor(data.config.language)}`}>
              {data.config.language}
            </span>
          </div>
        )}
        {data.config?.code && (
          <div className="mt-2">
            <div className="font-medium mb-1">Code:</div>
            <div className="text-xs bg-white dark:bg-gray-800 p-2 rounded border border-gray-300 dark:border-gray-700 font-mono max-h-16 overflow-y-auto">
              {data.config.code.length > 100
                ? `${data.config.code.substring(0, 100)}...`
                : data.config.code}
            </div>
          </div>
        )}
      </div>

      {data.lastRun && (
        <div className="mt-2 pt-2 border-t border-gray-300 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400">
          Last: {new Date(data.lastRun).toLocaleString()}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        isConnectable={isConnectable}
        className="!bg-gray-500 !w-3 !h-3 !border-2 !border-white"
      />
    </div>
  );
};

export default CodeNode;
