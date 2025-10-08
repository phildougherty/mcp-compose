import React from 'react';
import { Handle, Position } from '@xyflow/react';
import { ClockIcon } from '@heroicons/react/24/outline';

const TriggerNode = ({ data, isConnectable }) => {
  const getStatusColor = () => {
    switch (data.status) {
      case 'running':
        return 'border-yellow-400 bg-yellow-50 dark:bg-yellow-900/20';
      case 'success':
        return 'border-green-400 bg-green-50 dark:bg-green-900/20';
      case 'error':
        return 'border-red-400 bg-red-50 dark:bg-red-900/20';
      default:
        return 'border-blue-400 bg-blue-50 dark:bg-blue-900/20';
    }
  };

  const formatSchedule = (schedule) => {
    if (!schedule) return 'No schedule';
    if (schedule === '* * * * *') return 'Every minute';
    if (schedule === '0 * * * *') return 'Hourly';
    if (schedule === '0 0 * * *') return 'Daily';
    return schedule;
  };

  return (
    <div className={`px-4 py-3 shadow-lg rounded-lg border-2 transition-all duration-200 min-w-[200px] ${getStatusColor()}`}>
      <div className="flex items-center gap-2 mb-2">
        <ClockIcon className="w-5 h-5 text-blue-600 dark:text-blue-400 flex-shrink-0" />
        <div className="flex-1 min-w-0">
          <div className="font-bold text-gray-900 dark:text-white text-sm">
            {data.label || 'Trigger'}
          </div>
        </div>
      </div>

      <div className="text-xs text-gray-600 dark:text-gray-300 space-y-1">
        {data.config?.schedule && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Schedule:</span>
            <span className="font-mono">{formatSchedule(data.config.schedule)}</span>
          </div>
        )}
        {data.config?.webhook && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Webhook</span>
          </div>
        )}
        {data.config?.event && (
          <div className="flex items-center gap-1">
            <span className="font-medium">Event:</span>
            <span>{data.config.event}</span>
          </div>
        )}
      </div>

      {data.lastRun && (
        <div className="mt-2 pt-2 border-t border-blue-200 dark:border-blue-700 text-xs text-gray-500 dark:text-gray-400">
          Last: {new Date(data.lastRun).toLocaleString()}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        isConnectable={isConnectable}
        className="!bg-blue-500 !w-3 !h-3 !border-2 !border-white"
      />
    </div>
  );
};

export default TriggerNode;
