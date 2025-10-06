/**
 * AuditEntry Component - Individual audit entry with expandable details
 */

import React from 'react';
import { Badge } from '../shared';
import { formatTimestamp, getEventIcon, getEventColor, formatEventName } from './utils';

function AuditEntry({ entry }) {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        <div className="space-y-4">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wide">Basic Information</h4>

          <div className="space-y-3">
            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Event ID</label>
              <code className="text-sm bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 px-2 py-1 rounded block">{entry.id}</code>
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Timestamp</label>
              <p className="text-sm text-gray-900 dark:text-gray-100">{formatTimestamp(entry.timestamp)}</p>
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Event Type</label>
              <div className="flex items-center gap-2">
                <div className={`w-6 h-6 rounded flex items-center justify-center ${getEventColor(entry.event)}`}>
                  <svg className="w-3 h-3 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={getEventIcon(entry.event)} />
                  </svg>
                </div>
                <span className="text-sm text-gray-900 dark:text-gray-100">{formatEventName(entry.event)}</span>
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Result</label>
              <Badge
                variant={entry.success ? 'success' : 'danger'}
                size="sm"
              >
                <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d={
                      entry.success
                        ? 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z'
                        : 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'
                    }
                  />
                </svg>
                {entry.success ? 'Success' : 'Failed'}
              </Badge>
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wide">Context Information</h4>

          <div className="space-y-3">
            {entry.user_id && (
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">User ID</label>
                <p className="text-sm text-gray-900 dark:text-gray-100">{entry.user_id}</p>
              </div>
            )}

            {entry.client_id && (
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Client ID</label>
                <code className="text-sm bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 px-2 py-1 rounded break-all block">
                  {entry.client_id}
                </code>
              </div>
            )}

            {entry.ip_address && (
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">IP Address</label>
                <code className="text-sm bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 px-2 py-1 rounded">{entry.ip_address}</code>
              </div>
            )}

            {entry.user_agent && (
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">User Agent</label>
                <p className="text-sm text-gray-900 dark:text-gray-100 break-words">{entry.user_agent}</p>
              </div>
            )}
          </div>
        </div>
      </div>

      {entry.error && (
        <div className="bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
          <h4 className="text-sm font-medium text-red-800 dark:text-red-200 mb-2">Error Details</h4>
          <p className="text-sm text-red-700 dark:text-red-300 break-words">{entry.error}</p>
        </div>
      )}

      {entry.details && Object.keys(entry.details).length > 0 && (
        <div className="space-y-3">
          <h4 className="text-sm font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wide">Additional Details</h4>
          <div className="bg-gray-50 dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg p-4">
            <pre className="text-xs text-gray-800 dark:text-gray-300 overflow-auto whitespace-pre-wrap break-words max-h-96">
              {JSON.stringify(entry.details, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}

export default AuditEntry;
