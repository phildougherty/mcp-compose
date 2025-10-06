/**
 * AuditFilters Component - Filter controls for audit logs
 */

import React from 'react';
import useAuditStore from '../../store/auditStore';
import { SearchInput, Select, Button, Checkbox } from '../shared';
import { EVENT_TYPES, TIME_RANGE_OPTIONS } from './constants';

function AuditFilters() {
  const {
    filters,
    totalEntries,
    autoRefresh,
    setFilter,
    resetFilters,
    setAutoRefresh,
  } = useAuditStore();

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
      <div className="space-y-4">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4">
          <div className="flex flex-col sm:flex-row gap-3 flex-1 max-w-4xl">
            <SearchInput
              value={filters.search}
              onChange={(value) => setFilter('search', value)}
              placeholder="Search logs..."
              className="flex-1"
            />

            <Select
              value={filters.event}
              onChange={(value) => setFilter('event', value)}
              options={EVENT_TYPES}
              className="w-full sm:w-auto"
            />

            <Select
              value={filters.success}
              onChange={(value) => setFilter('success', value)}
              options={[
                { value: '', label: 'All Results' },
                { value: 'true', label: 'Success Only' },
                { value: 'false', label: 'Failures Only' },
              ]}
              className="w-full sm:w-auto"
            />

            <Select
              value={filters.timeRange}
              onChange={(value) => setFilter('timeRange', value)}
              options={TIME_RANGE_OPTIONS}
              className="w-full sm:w-auto"
            />
          </div>

          <div className="flex items-center gap-3">
            <Button
              onClick={resetFilters}
              variant="secondary"
              size="sm"
            >
              Clear
            </Button>
            <span className="text-sm text-gray-600 dark:text-gray-400">
              {totalEntries.toLocaleString()} entries
            </span>
          </div>
        </div>

        <div className="flex items-center">
          <Checkbox
            checked={autoRefresh}
            onChange={(checked) => setAutoRefresh(checked)}
            label="Auto-refresh"
          />
        </div>
      </div>
    </div>
  );
}

export default AuditFilters;
