/**
 * Audit Component - Main audit log viewer
 */

import React, { useEffect, useRef } from 'react';
import useAuditStore from '../../store/auditStore';
import { getAuditEntries, getAuditStats } from '../../api/audit';
import { useToast } from '../../hooks/useToast';
import { Button, Spinner, Modal } from '../shared';
import AuditStats from './AuditStats';
import AuditFilters from './AuditFilters';
import AuditList from './AuditList';
import AuditEntry from './AuditEntry';
import ExportButton from './ExportButton';

function Audit() {
  const {
    entries,
    stats,
    loading,
    error,
    selectedEntry,
    filters,
    currentPage,
    pageSize,
    sortBy,
    sortOrder,
    autoRefresh,
    setEntries,
    setStats,
    setLoading,
    setError,
    clearSelectedEntry,
    setPagination,
    clearError,
  } = useAuditStore();

  const { success } = useToast();
  const refreshIntervalRef = useRef(null);

  useEffect(() => {
    loadData();
  }, [currentPage, pageSize, sortBy, sortOrder, filters]);

  useEffect(() => {
    setupAutoRefresh();
    return () => {
      if (refreshIntervalRef.current) {
        clearInterval(refreshIntervalRef.current);
      }
    };
  }, [autoRefresh]);

  const setupAutoRefresh = () => {
    if (refreshIntervalRef.current) {
      clearInterval(refreshIntervalRef.current);
      refreshIntervalRef.current = null;
    }

    if (autoRefresh) {
      refreshIntervalRef.current = setInterval(() => {
        loadEntries();
      }, 30000);
    }
  };

  const loadData = async () => {
    await Promise.all([loadEntries(), loadStats()]);
  };

  const loadEntries = async () => {
    setLoading(true);
    setError(null);

    try {
      const params = {
        page: currentPage,
        limit: pageSize,
        sort: sortBy,
        order: sortOrder,
        ...(filters.event && { event: filters.event }),
        ...(filters.success !== '' && { success: filters.success }),
        ...(filters.timeRange !== 'all' && { timeRange: filters.timeRange }),
        ...(filters.search && { search: filters.search }),
      };

      const data = await getAuditEntries(params);
      setEntries(data.entries || []);
      setPagination(data.total || 0, Math.ceil((data.total || 0) / pageSize));
    } catch (err) {
      console.error('Failed to load audit entries:', err);
      setError(`Failed to load audit entries: ${err.message}`);
      setEntries([]);
      setPagination(0, 0);
    } finally {
      setLoading(false);
    }
  };

  const loadStats = async () => {
    try {
      const data = await getAuditStats({ time_range: filters.timeRange });
      setStats(data);
    } catch (err) {
      console.error('Failed to load audit stats:', err);
      setStats(null);
    }
  };

  const handleRefresh = () => {
    loadEntries();
    success('Audit logs refreshed');
  };

  return (
    <div className="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
        <div className="flex flex-col space-y-4">
          <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
            <div className="flex items-center space-x-3">
              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-gradient-to-br from-purple-500 to-purple-600 rounded-lg flex items-center justify-center shadow-sm">
                  <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                  </svg>
                </div>
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Audit Logs</h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">Track security events and system activities</p>
              </div>
            </div>

            <div className="flex flex-col sm:flex-row gap-2 sm:gap-3">
              <ExportButton />

              <Button
                onClick={handleRefresh}
                disabled={loading}
                variant="secondary"
                size="md"
              >
                <svg className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Refresh
              </Button>
            </div>
          </div>
        </div>
      </div>

      {stats && <AuditStats stats={stats} />}

      {error && (
        <div className="bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
          <div className="flex items-start">
            <svg className="h-5 w-5 text-red-600 dark:text-red-400 mt-0.5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <div className="ml-3 flex-1">
              <div className="text-sm text-red-800 dark:text-red-200">{error}</div>
              <Button
                onClick={clearError}
                variant="ghost"
                size="sm"
                className="mt-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
              >
                Dismiss
              </Button>
            </div>
          </div>
        </div>
      )}

      <AuditFilters />

      {loading && entries.length === 0 ? (
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-8 text-center">
          <Spinner size="lg" className="mx-auto mb-4" />
          <p className="text-lg font-medium text-gray-900 dark:text-white">Loading audit entries...</p>
          <p className="text-sm text-gray-500 dark:text-gray-400">Fetching security and activity logs</p>
        </div>
      ) : (
        <AuditList entries={entries} loading={loading} />
      )}

      <Modal
        isOpen={!!selectedEntry}
        onClose={clearSelectedEntry}
        title="Audit Entry Details"
        size="4xl"
      >
        {selectedEntry && <AuditEntry entry={selectedEntry} />}
      </Modal>
    </div>
  );
}

export default Audit;
