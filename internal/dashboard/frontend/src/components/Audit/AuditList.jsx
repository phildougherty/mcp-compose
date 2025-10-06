/**
 * AuditList Component - Paginated audit log list
 */

import React from 'react';
import useAuditStore from '../../store/auditStore';
import { EmptyState, Pagination, Button, Badge } from '../shared';
import { formatTimestamp, getEventIcon, getEventColor, formatEventName } from './utils';

function AuditList({ entries, loading }) {
  const {
    filters,
    sortBy,
    sortOrder,
    currentPage,
    pageSize,
    totalEntries,
    totalPages,
    setSort,
    setPage,
    nextPage,
    previousPage,
    setSelectedEntry,
    resetFilters,
  } = useAuditStore();

  if (entries.length === 0 && !loading) {
    return (
      <EmptyState
        icon={
          <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
        }
        title="No audit entries found"
        description={
          Object.values(filters).some((f) => f)
            ? 'Try adjusting your filters or time range'
            : 'Audit entries will appear here when available'
        }
        action={
          Object.values(filters).some((f) => f) && (
            <Button
              onClick={resetFilters}
              variant="primary"
              size="md"
            >
              Clear Filters
            </Button>
          )
        }
      />
    );
  }

  const SortHeader = ({ field, children }) => (
    <th
      onClick={() => setSort(field)}
      className="px-6 py-3 text-left text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
    >
      <div className="flex items-center gap-1">
        <span>{children}</span>
        {sortBy === field && (
          <svg
            className={`w-4 h-4 transform ${sortOrder === 'desc' ? 'rotate-180' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        )}
      </div>
    </th>
  );

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden w-full max-w-full">
      <div className="overflow-x-auto w-full">
        <table className="min-w-full divide-y divide-gray-200 dark:divide-gray-700 w-full">
          <thead className="bg-gray-50 dark:bg-gray-900">
            <tr>
              <SortHeader field="timestamp">Timestamp</SortHeader>
              <SortHeader field="event">Event</SortHeader>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                User/Client
              </th>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                Source
              </th>
              <SortHeader field="success">Result</SortHeader>
              <th className="px-6 py-3 text-left text-xs font-medium text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                Actions
              </th>
            </tr>
          </thead>
          <tbody className="bg-white dark:bg-gray-800 divide-y divide-gray-200 dark:divide-gray-700">
            {entries.map((entry) => (
              <tr
                key={entry.id}
                className={`hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors ${
                  !entry.success ? 'bg-red-50 dark:bg-red-900/10' : ''
                }`}
              >
                <td className="px-6 py-4 whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <svg className="w-3 h-3 text-gray-500 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <div className="text-sm text-gray-900 dark:text-gray-100">{formatTimestamp(entry.timestamp)}</div>
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <div className="flex items-center gap-2">
                    <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${getEventColor(entry.event)}`}>
                      <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={getEventIcon(entry.event)} />
                      </svg>
                    </div>
                    <span className="text-sm font-medium text-gray-900 dark:text-gray-100">{formatEventName(entry.event)}</span>
                  </div>
                </td>
                <td className="px-6 py-4">
                  <div className="text-sm max-w-xs">
                    {entry.user_id && <div className="text-gray-900 dark:text-gray-100 font-medium break-words">{entry.user_id}</div>}
                    {entry.client_id && <div className="text-gray-500 dark:text-gray-400 break-words">{entry.client_id}</div>}
                    {!entry.user_id && !entry.client_id && <div className="text-gray-500 dark:text-gray-500">System</div>}
                  </div>
                </td>
                <td className="px-6 py-4">
                  {entry.ip_address ? (
                    <code className="text-xs bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200 px-2 py-1 rounded break-all max-w-xs overflow-x-auto">
                      {entry.ip_address}
                    </code>
                  ) : (
                    <span className="text-gray-500 dark:text-gray-500 text-sm">-</span>
                  )}
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <div className="space-y-1">
                    <Badge
                      variant={entry.success ? 'success' : 'danger'}
                      size="sm"
                    >
                      <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d={entry.success ? 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z' : 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'}
                        />
                      </svg>
                      {entry.success ? 'Success' : 'Failed'}
                    </Badge>
                    {entry.error && (
                      <div className="text-xs text-red-600 dark:text-red-400 break-words max-w-xs overflow-hidden" title={entry.error}>
                        {entry.error}
                      </div>
                    )}
                  </div>
                </td>
                <td className="px-6 py-4 whitespace-nowrap">
                  <Button
                    onClick={() => setSelectedEntry(entry)}
                    variant="ghost"
                    size="sm"
                    className="text-blue-600 dark:text-blue-400 hover:text-blue-700 dark:hover:text-blue-300"
                  >
                    <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                    </svg>
                    Details
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {totalPages > 1 && (
        <div className="bg-gray-50 dark:bg-gray-900 px-6 py-3 border-t border-gray-200 dark:border-gray-700">
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            pageSize={pageSize}
            totalItems={totalEntries}
            onPageChange={setPage}
            onNextPage={nextPage}
            onPreviousPage={previousPage}
          />
        </div>
      )}
    </div>
  );
}

export default AuditList;
