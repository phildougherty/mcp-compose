import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { ServerIcon } from '@heroicons/react/24/outline';

const MCPServerNode = ({ data, isConnectable }) => {
  const getStatusColor = () => {
    switch (data.status) {
      case 'running':
        return 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/20';
      case 'success':
        return 'border-green-400 bg-green-50 dark:bg-green-900/20';
      case 'error':
        return 'border-red-400 bg-red-50 dark:bg-red-900/20';
      default:
        return 'border-green-400 bg-green-50 dark:bg-green-900/20';
    }
  };

  return (
    <div className={`px-4 py-3 shadow-lg rounded-lg border-2 transition-all duration-200 min-w-[220px] ${getStatusColor()}`}>
      <Handle
        type="target"
        position={Position.Left}
        isConnectable={isConnectable}
        className="!bg-green-500 !w-3 !h-3 !border-2 !border-white"
      />

      <div className="flex items-center gap-2 mb-2">
        <ServerIcon className="w-5 h-5 text-green-600 dark:text-green-400 flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="font-bold text-gray-900 dark:text-white text-sm truncate">
            {data.label || 'MCP Server'}
          </div>
        </div>
      </div>

      <div className="text-xs text-gray-600 dark:text-gray-300 space-y-1">
        {data.config?.server && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Server:</span>
            <span className="font-mono truncate">{data.config.server}</span>
          </div>
        )}
        {data.config?.tool && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Tool:</span>
            <span className="font-mono truncate">{data.config.tool}</span>
          </div>
        )}
        {data.config?.parameters && Object.keys(data.config.parameters).length > 0 && (
          <div className="mt-2">
            <div className="font-medium mb-1">Parameters:</div>
            <div className="text-xs bg-white dark:bg-gray-800 p-2 rounded border border-green-200 dark:border-green-700 max-h-16 overflow-y-auto space-y-1">
              {Object.entries(data.config.parameters).slice(0, 3).map(([key, value]) => (
                <div key={key} className="flex gap-1">
                  <span className="font-medium">{key}:</span>
                  <span className="truncate">{String(value)}</span>
                </div>
              ))}
              {Object.keys(data.config.parameters).length > 3 && (
                <div className="text-gray-500">
                  +{Object.keys(data.config.parameters).length - 3} more
                </div>
              )}
            </div>
          </div>
        )}
      </div>

      {data.lastRun && (
        <div className="mt-2 pt-2 border-t border-green-200 dark:border-green-700 text-xs text-gray-500 dark:text-gray-400">
          Last: {new Date(data.lastRun).toLocaleString()}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        isConnectable={isConnectable}
        className="!bg-green-500 !w-3 !h-3 !border-2 !border-white"
      />
    </div>
  );
};

export default MCPServerNode;
