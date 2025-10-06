import React from 'react';
import { useLogsStore } from '../../store/logsStore';
import { Badge } from '../shared';

export default function LogStats() {
  const { logStats, selectedServer } = useLogsStore();
  const stats = logStats();

  if (!selectedServer || stats.total === 0) {
    return null;
  }

  return (
    <div className="bg-gray-800/50 dark:bg-gray-900/50 backdrop-blur-sm border border-gray-700 rounded-lg p-4 shadow-xl">
      <h3 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">Log Statistics</h3>
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-4">
        <div className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
          <div className="w-2 h-2 bg-cyan-400 rounded-full" />
          <div className="flex-1">
            <div className="text-xs text-gray-600 dark:text-gray-400">Total</div>
            <div className="text-lg font-semibold text-gray-900 dark:text-white">{stats.total}</div>
          </div>
        </div>

        {stats.errors > 0 && (
          <div className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20">
            <div className="w-2 h-2 bg-red-400 rounded-full" />
            <div className="flex-1">
              <div className="text-xs text-red-600 dark:text-red-400">Errors</div>
              <div className="text-lg font-semibold text-red-700 dark:text-red-300">{stats.errors}</div>
            </div>
          </div>
        )}

        {stats.warnings > 0 && (
          <div className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-yellow-50 dark:bg-yellow-500/10 border border-yellow-200 dark:border-yellow-500/20">
            <div className="w-2 h-2 bg-yellow-400 rounded-full" />
            <div className="flex-1">
              <div className="text-xs text-yellow-600 dark:text-yellow-400">Warnings</div>
              <div className="text-lg font-semibold text-yellow-700 dark:text-yellow-300">{stats.warnings}</div>
            </div>
          </div>
        )}

        {stats.info > 0 && (
          <div className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-cyan-50 dark:bg-cyan-500/10 border border-cyan-200 dark:border-cyan-500/20">
            <div className="w-2 h-2 bg-cyan-400 rounded-full" />
            <div className="flex-1">
              <div className="text-xs text-cyan-600 dark:text-cyan-400">Info</div>
              <div className="text-lg font-semibold text-cyan-700 dark:text-cyan-300">{stats.info}</div>
            </div>
          </div>
        )}

        {stats.debug > 0 && (
          <div className="flex items-center space-x-2 px-3 py-2 rounded-lg bg-purple-50 dark:bg-purple-500/10 border border-purple-200 dark:border-purple-500/20">
            <div className="w-2 h-2 bg-purple-400 rounded-full" />
            <div className="flex-1">
              <div className="text-xs text-purple-600 dark:text-purple-400">Debug</div>
              <div className="text-lg font-semibold text-purple-700 dark:text-purple-300">{stats.debug}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
