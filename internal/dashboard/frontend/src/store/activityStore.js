import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

/**
 * @typedef {Object} ActivityEvent
 * @property {string} id - Unique event identifier
 * @property {string} timestamp - Event timestamp (ISO string)
 * @property {string} level - Event level (ERROR, WARN, INFO, DEBUG)
 * @property {string} type - Event type (request, connection, tool_call, tool, error)
 * @property {string} message - Event message
 * @property {string} [server] - Server name
 * @property {Object} [details] - Additional event details
 * @property {Array<ToolCall>} [toolCalls] - Tool calls associated with event
 * @property {boolean} [isHistorical] - Whether event is from historical data
 */

/**
 * @typedef {Object} ToolCall
 * @property {string} tool - Tool name
 * @property {Object} [arguments] - Tool arguments
 * @property {any} [result] - Tool execution result
 */

/**
 * @typedef {Object} ActivityStats
 * @property {number} total - Total number of events
 * @property {number} requests - Number of request events
 * @property {number} errors - Number of error events
 * @property {number} connections - Number of connection events
 * @property {number} toolCalls - Number of tool call events
 */

/**
 * @typedef {Object} HistoricalStats
 * @property {number} totalToday - Total events today
 * @property {number} requestsToday - Requests today
 * @property {number} errorsToday - Errors today
 * @property {number} toolCallsToday - Tool calls today
 */

/**
 * @typedef {Object} ActivityState
 * @property {ActivityEvent[]} activities - Real-time activity events
 * @property {ActivityEvent[]} historicalActivities - Historical activity events
 * @property {ActivityStats} activityStats - Real-time statistics
 * @property {HistoricalStats} historicalStats - Historical statistics
 * @property {string} levelFilter - Filter by level (empty = all)
 * @property {string} typeFilter - Filter by type (empty = all)
 * @property {string} searchFilter - Search query
 * @property {boolean} autoScroll - Auto-scroll to new events
 * @property {boolean} loading - Loading state for historical data
 * @property {string|null} error - Error message if any
 * @property {Object<string, boolean>} expandedToolCalls - Expanded tool calls map
 * @property {Object<string, boolean>} expandedDetails - Expanded details map
 */

/**
 * @typedef {Object} ActivityActions
 * @property {(activity: ActivityEvent) => void} addActivity - Add new real-time activity
 * @property {(activities: ActivityEvent[]) => void} setHistoricalActivities - Set historical activities
 * @property {(stats: HistoricalStats) => void} setHistoricalStats - Set historical stats
 * @property {(level: string) => void} setLevelFilter - Set level filter
 * @property {(type: string) => void} setTypeFilter - Set type filter
 * @property {(search: string) => void} setSearchFilter - Set search filter
 * @property {(enabled: boolean) => void} setAutoScroll - Set auto-scroll
 * @property {(loading: boolean) => void} setLoading - Set loading state
 * @property {(error: string|null) => void} setError - Set error message
 * @property {(activityId: string, toolCallId: string) => void} toggleToolCall - Toggle tool call expansion
 * @property {(activityId: string) => void} toggleDetails - Toggle details expansion
 * @property {() => void} clearActivities - Clear all activities
 * @property {() => void} resetFilters - Reset all filters
 * @property {() => void} reset - Reset entire store
 */

/**
 * @typedef {ActivityState & ActivityActions} ActivityStore
 */

const initialState = {
  activities: [],
  historicalActivities: [],
  activityStats: {
    total: 0,
    requests: 0,
    errors: 0,
    connections: 0,
    toolCalls: 0,
  },
  historicalStats: {
    totalToday: 0,
    requestsToday: 0,
    errorsToday: 0,
    toolCallsToday: 0,
  },
  levelFilter: '',
  typeFilter: '',
  searchFilter: '',
  autoScroll: true,
  loading: false,
  error: null,
  expandedToolCalls: {},
  expandedDetails: {},
};

/**
 * Activity Store - Manages activity events and statistics
 *
 * @description
 * Centralized state management for the activity monitor.
 * Handles real-time and historical activity events, filtering, and statistics.
 *
 * @example
 * // Add new activity
 * const addActivity = useActivityStore(state => state.addActivity);
 * addActivity({ id: '1', level: 'INFO', type: 'request', message: 'Request received' });
 *
 * @example
 * // Get filtered activities
 * const filteredActivities = useActivityStore(selectFilteredActivities);
 */
export const useActivityStore = create(
  devtools(
    (set, get) => ({
      ...initialState,

      addActivity: (activity) => {
        const enrichedActivity = {
          ...activity,
          id: activity.id || `live-${Date.now()}-${Math.random()}`,
          timestamp: activity.timestamp || new Date().toISOString(),
          toolCalls: extractToolCalls(activity),
          isHistorical: false,
        };

        set((state) => {
          const newActivities = [enrichedActivity, ...state.activities];
          const trimmedActivities = newActivities.length > 500
            ? newActivities.slice(0, 500)
            : newActivities;

          const newStats = updateStats(state.activityStats, enrichedActivity);

          return {
            activities: trimmedActivities,
            activityStats: newStats,
          };
        }, false, 'addActivity');
      },

      setHistoricalActivities: (activities) => set({
        historicalActivities: activities.map(activity => ({
          ...activity,
          id: activity.id || activity.activity_id || `hist-${Date.now()}-${Math.random()}`,
          isHistorical: true,
          toolCalls: extractToolCalls(activity),
        })),
      }, false, 'setHistoricalActivities'),

      setHistoricalStats: (stats) => set({
        historicalStats: stats,
      }, false, 'setHistoricalStats'),

      setLevelFilter: (levelFilter) => set({
        levelFilter,
      }, false, 'setLevelFilter'),

      setTypeFilter: (typeFilter) => set({
        typeFilter,
      }, false, 'setTypeFilter'),

      setSearchFilter: (searchFilter) => set({
        searchFilter,
      }, false, 'setSearchFilter'),

      setAutoScroll: (autoScroll) => set({
        autoScroll,
      }, false, 'setAutoScroll'),

      setLoading: (loading) => set({
        loading,
      }, false, 'setLoading'),

      setError: (error) => set({
        error,
      }, false, 'setError'),

      toggleToolCall: (activityId, toolCallId) => {
        const key = `${activityId}-${toolCallId}`;

        set((state) => ({
          expandedToolCalls: {
            ...state.expandedToolCalls,
            [key]: !state.expandedToolCalls[key],
          },
        }), false, 'toggleToolCall');
      },

      toggleDetails: (activityId) => set((state) => ({
        expandedDetails: {
          ...state.expandedDetails,
          [activityId]: !state.expandedDetails[activityId],
        },
      }), false, 'toggleDetails'),

      clearActivities: () => set({
        activities: [],
        historicalActivities: [],
        activityStats: {
          total: 0,
          requests: 0,
          errors: 0,
          connections: 0,
          toolCalls: 0,
        },
        historicalStats: {
          totalToday: 0,
          requestsToday: 0,
          errorsToday: 0,
          toolCallsToday: 0,
        },
        expandedToolCalls: {},
        expandedDetails: {},
      }, false, 'clearActivities'),

      resetFilters: () => set({
        levelFilter: '',
        typeFilter: '',
        searchFilter: '',
      }, false, 'resetFilters'),

      reset: () => set(initialState, false, 'reset'),
    }),
    {
      name: 'activity-store',
      enabled: process.env.NODE_ENV === 'development',
    }
  )
);

/**
 * Extract tool calls from activity details
 */
function extractToolCalls(activity) {
  const toolCalls = [];

  if (activity.details) {
    if (activity.details.toolCall || activity.details.tool_call) {
      const toolCall = activity.details.toolCall || activity.details.tool_call;
      toolCalls.push({
        tool: toolCall.tool || toolCall.name || 'Unknown Tool',
        arguments: toolCall.arguments || toolCall.args,
        result: toolCall.result,
      });
    }

    if (activity.details.tools) {
      activity.details.tools.forEach(tool => {
        toolCalls.push({
          tool: tool.name || 'Unknown Tool',
          arguments: tool.arguments,
          result: tool.result,
        });
      });
    }
  }

  return toolCalls;
}

/**
 * Update statistics with new activity
 */
function updateStats(stats, activity) {
  const newStats = { ...stats };

  newStats.total++;

  switch (activity.type) {
    case 'request':
      newStats.requests++;
      break;
    case 'connection':
      newStats.connections++;
      break;
    case 'tool':
    case 'tool_call':
      newStats.toolCalls++;
      break;
  }

  if (activity.level === 'ERROR') {
    newStats.errors++;
  }

  if (activity.toolCalls && activity.toolCalls.length > 0) {
    newStats.toolCalls += activity.toolCalls.length;
  }

  return newStats;
}

/**
 * Optimized selectors to prevent unnecessary re-renders
 */

export const selectActivities = (state) => state.activities;

export const selectHistoricalActivities = (state) => state.historicalActivities;

export const selectAllActivities = (state) => {
  const combined = [...state.historicalActivities, ...state.activities];
  const unique = combined.filter((activity, index, self) =>
    index === self.findIndex(a => a.id === activity.id)
  );

  return unique.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
};

export const selectFilteredActivities = (state) => {
  const allActivities = selectAllActivities(state);

  return allActivities.filter(activity => {
    const matchesLevel = !state.levelFilter || activity.level === state.levelFilter;
    const matchesType = !state.typeFilter || activity.type === state.typeFilter;
    const matchesSearch = !state.searchFilter ||
      activity.message.toLowerCase().includes(state.searchFilter.toLowerCase()) ||
      (activity.server && activity.server.toLowerCase().includes(state.searchFilter.toLowerCase()));

    return matchesLevel && matchesType && matchesSearch;
  });
};

export const selectAvailableLevels = (state) => {
  const allActivities = selectAllActivities(state);
  const levels = new Set(allActivities.map(a => a.level).filter(Boolean));

  return Array.from(levels).sort();
};

export const selectAvailableTypes = (state) => {
  const allActivities = selectAllActivities(state);
  const types = new Set(allActivities.map(a => a.type).filter(Boolean));

  return Array.from(types).sort();
};

export const selectCombinedStats = (state) => ({
  total: state.activityStats.total + state.historicalStats.totalToday,
  requests: state.activityStats.requests + state.historicalStats.requestsToday,
  errors: state.activityStats.errors + state.historicalStats.errorsToday,
  toolCalls: state.activityStats.toolCalls + state.historicalStats.toolCallsToday,
  connections: state.activityStats.connections,
});

export const selectIsLoading = (state) => state.loading;

export const selectError = (state) => state.error;

export const selectAutoScroll = (state) => state.autoScroll;

export const selectIsToolCallExpanded = (activityId, toolCallId) => (state) =>
  !!state.expandedToolCalls[`${activityId}-${toolCallId}`];

export const selectIsDetailsExpanded = (activityId) => (state) =>
  !!state.expandedDetails[activityId];
