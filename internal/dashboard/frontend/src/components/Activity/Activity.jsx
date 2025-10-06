import { useEffect, useCallback } from 'react';
import { useActivityStore, selectFilteredActivities, selectCombinedStats, selectIsLoading } from '../../store/activityStore';
import { useWebSocket } from '../../hooks/useWebSocket';
import { getActivityHistory, getActivityStats } from '../../api/activity';
import { useToast } from '../../hooks/useToast';
import ActivityStats from './ActivityStats';
import ActivityFilters from './ActivityFilters';
import ActivityList from './ActivityList';
import { Spinner } from '../shared';

/**
 * Activity Monitor Component
 *
 * Main activity monitoring interface with real-time WebSocket updates
 * and historical data from PostgreSQL.
 *
 * Features:
 * - Real-time activity stream via WebSocket
 * - Historical data loading (6 hours)
 * - Event filtering by level, type, and search
 * - Statistics display
 * - Tool call details expansion
 * - Auto-scroll to new events
 */
export default function Activity() {
  const filteredActivities = useActivityStore(selectFilteredActivities);
  const combinedStats = useActivityStore(selectCombinedStats);
  const isLoading = useActivityStore(selectIsLoading);
  const autoScroll = useActivityStore(state => state.autoScroll);

  const {
    addActivity,
    setHistoricalActivities,
    setHistoricalStats,
    setLoading,
    setError,
    clearActivities,
  } = useActivityStore();

  const { success, error: showError, info } = useToast();

  const loadHistoricalData = useCallback(async () => {
    setLoading(true);

    try {
      const [historyResponse, statsResponse] = await Promise.all([
        getActivityHistory({ hours: 6 }),
        getActivityStats({ hours: 24 }),
      ]);

      setHistoricalActivities(historyResponse.activities || []);
      setHistoricalStats(statsResponse);

      console.log('Loaded historical activities:', historyResponse.activities?.length || 0);
    } catch (err) {
      console.warn('Failed to load historical activities:', err);

      if (err.message.includes('503')) {
        info('Activity storage not configured - configure PostgreSQL URL for persistent storage');
      } else if (err.message.includes('404')) {
        info('Activity history not available in this version');
      } else {
        showError('Failed to load historical activities');
      }

      setHistoricalActivities([]);
      setHistoricalStats({
        totalToday: 0,
        requestsToday: 0,
        errorsToday: 0,
        toolCallsToday: 0,
      });
    } finally {
      setLoading(false);
    }
  }, [setLoading, setHistoricalActivities, setHistoricalStats, info, showError]);

  const handleWebSocketMessage = useCallback((data) => {
    try {
      if (data && typeof data === 'object') {
        addActivity(data);
      }
    } catch (err) {
      console.error('Failed to parse activity message:', err);
    }
  }, [addActivity]);

  const handleWebSocketOpen = useCallback(() => {
    console.log('Activity stream connected');
    success('Connected to activity stream');
  }, [success]);

  const handleWebSocketClose = useCallback(() => {
    console.log('Activity stream disconnected');
  }, []);

  const handleWebSocketError = useCallback((err) => {
    console.error('Activity WebSocket error:', err);
    showError('Connection error - attempting to reconnect');
  }, [showError]);

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/activity`;

  const { isConnected } = useWebSocket(wsUrl, {
    autoConnect: true,
    reconnectDelay: 3000,
    maxReconnectAttempts: 10,
    onMessage: handleWebSocketMessage,
    onOpen: handleWebSocketOpen,
    onClose: handleWebSocketClose,
    onError: handleWebSocketError,
  });

  useEffect(() => {
    loadHistoricalData();
  }, [loadHistoricalData]);

  const handleRefresh = () => {
    loadHistoricalData();
  };

  const handleClear = () => {
    clearActivities();
    info('Activity feed cleared');
  };

  return (
    <div className="activity-viewer space-y-6 animate-fade-in max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
        <div className="flex flex-col space-y-4">
          <div className="flex items-center gap-3 min-w-0">
            <div className="flex-shrink-0">
              <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center shadow-md">
                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 10V3L4 14h7v7l9-11h-7z" />
                </svg>
              </div>
            </div>
            <div className="min-w-0 flex-1">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-3 flex-wrap">
                Activity Monitor
                {isConnected && (
                  <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400 border border-green-200 dark:border-green-800">
                    <span className="w-2 h-2 bg-green-500 rounded-full mr-1.5 animate-pulse" />
                    LIVE
                  </span>
                )}
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                Real-time activity stream with 6-hour persistent history
              </p>
            </div>
          </div>

          <ActivityStats stats={combinedStats} />
        </div>
      </div>

      <ActivityFilters
        onRefresh={handleRefresh}
        onClear={handleClear}
        isLoading={isLoading}
      />

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden">
        {isLoading && filteredActivities.length === 0 ? (
          <div className="p-12 text-center">
            <div className="inline-flex items-center justify-center w-16 h-16 bg-blue-50 dark:bg-blue-900/20 rounded-2xl mb-4">
              <Spinner className="w-8 h-8 text-blue-600 dark:text-blue-400" />
            </div>
            <p className="text-lg font-medium text-gray-900 dark:text-gray-100">Loading activities...</p>
            <p className="text-sm text-gray-600 dark:text-gray-400 mt-2">Fetching historical data</p>
          </div>
        ) : filteredActivities.length === 0 ? (
          <div className="p-12 text-center">
            <div className="inline-flex items-center justify-center w-16 h-16 bg-gray-100 dark:bg-gray-700/50 rounded-2xl mb-4">
              <svg className="w-8 h-8 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
              </svg>
            </div>
            <p className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-2">No activities found</p>
            <p className="text-sm text-gray-600 dark:text-gray-400">Try adjusting your filters or wait for new events</p>
          </div>
        ) : (
          <ActivityList activities={filteredActivities} autoScroll={autoScroll} />
        )}
      </div>
    </div>
  );
}
