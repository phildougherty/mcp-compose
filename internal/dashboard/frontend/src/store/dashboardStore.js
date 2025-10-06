import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

/**
 * @typedef {Object} ServerCapabilities
 * @property {boolean} [tools] - Server supports tools
 * @property {boolean} [resources] - Server supports resources
 * @property {boolean} [prompts] - Server supports prompts
 * @property {boolean} [logging] - Server supports logging
 */

/**
 * @typedef {Object} ServerHealth
 * @property {string} status - Health status (healthy, unhealthy, unknown)
 * @property {string} [message] - Health check message
 * @property {number} [lastCheck] - Timestamp of last health check
 */

/**
 * @typedef {Object} Server
 * @property {string} id - Unique server identifier
 * @property {string} name - Server name
 * @property {string} status - Server status (running, stopped, starting, stopping)
 * @property {ServerHealth} health - Health information
 * @property {ServerCapabilities} capabilities - Server capabilities
 * @property {number} toolCount - Number of available tools
 * @property {number} connectionCount - Number of active connections
 * @property {string} [transport] - Transport protocol (stdio, http, sse, tcp)
 * @property {string} [version] - MCP protocol version
 * @property {number} [uptime] - Server uptime in seconds
 * @property {Object} [metadata] - Additional server metadata
 */

/**
 * @typedef {Object} ServerMetrics
 * @property {number} totalServers - Total number of servers
 * @property {number} runningServers - Number of running servers
 * @property {number} healthyServers - Number of healthy servers
 * @property {number} totalConnections - Total active connections across all servers
 * @property {number} proxyUptime - Proxy uptime in seconds
 */

/**
 * @typedef {Object} DashboardState
 * @property {Server[]} servers - Array of all servers
 * @property {ServerMetrics} metrics - Dashboard metrics
 * @property {string|null} selectedServerId - Currently selected server ID
 * @property {boolean} isLoading - Loading state
 * @property {string|null} error - Error message if any
 * @property {number} lastUpdate - Timestamp of last update
 * @property {string} searchQuery - Search query for filtering servers
 * @property {string} statusFilter - Status filter (all, running, stopped, healthy)
 * @property {string} sortBy - Sort field (name, status, health, tools)
 * @property {string} sortOrder - Sort order (asc, desc)
 * @property {number} autoRefreshInterval - Auto-refresh interval in seconds (0 = disabled)
 * @property {string} viewMode - View mode (my-servers, browse-registry)
 * @property {Array} registryServers - Registry servers catalog
 * @property {Array} categories - Registry categories
 * @property {Array} featuredServers - Featured registry servers
 * @property {Array} installedServers - Installed registry servers
 * @property {Object|null} selectedRegistryServer - Currently selected registry server
 * @property {string} categoryFilter - Category filter for registry
 * @property {boolean} featuredOnly - Show only featured servers
 */

/**
 * @typedef {Object} DashboardActions
 * @property {(servers: Server[]) => void} setServers - Set all servers
 * @property {(server: Server) => void} updateServer - Update a single server
 * @property {(serverId: string, status: string) => void} updateServerStatus - Update server status
 * @property {(serverId: string, health: ServerHealth) => void} updateServerHealth - Update server health
 * @property {(metrics: ServerMetrics) => void} setMetrics - Set dashboard metrics
 * @property {(serverId: string|null) => void} selectServer - Select a server
 * @property {(loading: boolean) => void} setLoading - Set loading state
 * @property {(error: string|null) => void} setError - Set error message
 * @property {(query: string) => void} setSearchQuery - Set search query
 * @property {(filter: string) => void} setStatusFilter - Set status filter
 * @property {(sortBy: string, sortOrder?: string) => void} setSorting - Set sorting
 * @property {(interval: number) => void} setAutoRefreshInterval - Set auto-refresh interval
 * @property {() => void} refreshServers - Trigger server refresh
 * @property {() => void} resetFilters - Reset all filters to defaults
 * @property {() => void} reset - Reset entire store to initial state
 * @property {(mode: string) => void} setViewMode - Set view mode
 * @property {(servers: Array) => void} setRegistryServers - Set registry servers
 * @property {(categories: Array) => void} setCategories - Set categories
 * @property {(servers: Array) => void} setFeaturedServers - Set featured servers
 * @property {(servers: Array) => void} setInstalledServers - Set installed servers
 * @property {(server: Object|null) => void} setSelectedRegistryServer - Set selected registry server
 * @property {(category: string) => void} setCategoryFilter - Set category filter
 * @property {(featured: boolean) => void} setFeaturedOnly - Set featured only filter
 * @property {() => Promise} fetchRegistryServers - Fetch registry servers
 * @property {() => Promise} fetchCategories - Fetch categories
 * @property {() => Promise} fetchFeatured - Fetch featured servers
 * @property {() => Promise} fetchInstalledServers - Fetch installed servers
 * @property {(serverId: number) => Promise} fetchServerDetails - Fetch server details
 * @property {(serverId: number, config: Object) => Promise} installServer - Install server
 * @property {(serverId: number) => Promise} uninstallServer - Uninstall server
 * @property {(serverId: number) => boolean} isServerInstalled - Check if server is installed
 */

/**
 * @typedef {DashboardState & DashboardActions} DashboardStore
 */

const initialState = {
  servers: [],
  metrics: {
    totalServers: 0,
    runningServers: 0,
    healthyServers: 0,
    totalConnections: 0,
    proxyUptime: 0,
  },
  selectedServerId: null,
  isLoading: false,
  error: null,
  lastUpdate: Date.now(),
  searchQuery: '',
  statusFilter: 'all',
  sortBy: 'name',
  sortOrder: 'asc',
  autoRefreshInterval: 0,
  viewMode: 'my-servers',
  registryServers: [],
  categories: [],
  featuredServers: [],
  installedServers: [],
  selectedRegistryServer: null,
  categoryFilter: '',
  featuredOnly: false,
};

/**
 * Dashboard Store - Manages server state, metrics, and filtering
 *
 * @description
 * Centralized state management for the MCP Compose dashboard.
 * Handles server data, metrics, selection, and filtering/sorting.
 * Integrates with DevTools for debugging.
 *
 * @example
 * // Get filtered and sorted servers
 * const servers = useDashboardStore(state => state.servers);
 *
 * @example
 * // Update server status
 * const updateStatus = useDashboardStore(state => state.updateServerStatus);
 * updateStatus('server-1', 'running');
 *
 * @example
 * // Use optimized selector
 * const selectedServer = useDashboardStore(
 *   state => state.servers.find(s => s.id === state.selectedServerId)
 * );
 */
export const useDashboardStore = create(
  devtools(
    (set, get) => ({
      ...initialState,

      setServers: (servers) => set({
        servers,
        lastUpdate: Date.now()
      }, false, 'setServers'),

      updateServer: (server) => set((state) => ({
        servers: state.servers.map(s => s.id === server.id ? { ...s, ...server } : s),
        lastUpdate: Date.now(),
      }), false, 'updateServer'),

      updateServerStatus: (serverId, status) => set((state) => ({
        servers: state.servers.map(s =>
          s.id === serverId ? { ...s, status } : s
        ),
        lastUpdate: Date.now(),
      }), false, 'updateServerStatus'),

      updateServerHealth: (serverId, health) => set((state) => ({
        servers: state.servers.map(s =>
          s.id === serverId ? { ...s, health: { ...s.health, ...health } } : s
        ),
        lastUpdate: Date.now(),
      }), false, 'updateServerHealth'),

      setMetrics: (metrics) => set({
        metrics,
        lastUpdate: Date.now()
      }, false, 'setMetrics'),

      selectServer: (serverId) => set({
        selectedServerId: serverId
      }, false, 'selectServer'),

      setLoading: (isLoading) => set({ isLoading }, false, 'setLoading'),

      setError: (error) => set({ error }, false, 'setError'),

      setSearchQuery: (searchQuery) => set({
        searchQuery
      }, false, 'setSearchQuery'),

      setStatusFilter: (statusFilter) => set({
        statusFilter
      }, false, 'setStatusFilter'),

      setSorting: (sortBy, sortOrder) => set((state) => ({
        sortBy,
        sortOrder: sortOrder || (state.sortBy === sortBy && state.sortOrder === 'asc' ? 'desc' : 'asc'),
      }), false, 'setSorting'),

      setAutoRefreshInterval: (autoRefreshInterval) => set({
        autoRefreshInterval
      }, false, 'setAutoRefreshInterval'),

      refreshServers: () => set({
        lastUpdate: Date.now()
      }, false, 'refreshServers'),

      resetFilters: () => set({
        searchQuery: '',
        statusFilter: 'all',
        sortBy: 'name',
        sortOrder: 'asc',
      }, false, 'resetFilters'),

      reset: () => set(initialState, false, 'reset'),

      setViewMode: (viewMode) => set({ viewMode }, false, 'setViewMode'),

      setRegistryServers: (registryServers) => set({
        registryServers,
        lastUpdate: Date.now()
      }, false, 'setRegistryServers'),

      setCategories: (categories) => set({ categories }, false, 'setCategories'),

      setFeaturedServers: (featuredServers) => set({ featuredServers }, false, 'setFeaturedServers'),

      setInstalledServers: (installedServers) => set({ installedServers }, false, 'setInstalledServers'),

      setSelectedRegistryServer: (selectedRegistryServer) => set({
        selectedRegistryServer
      }, false, 'setSelectedRegistryServer'),

      setCategoryFilter: (categoryFilter) => set({ categoryFilter }, false, 'setCategoryFilter'),

      setFeaturedOnly: (featuredOnly) => set({ featuredOnly }, false, 'setFeaturedOnly'),

      fetchRegistryServers: async () => {
        set({ isLoading: true, error: null });
        try {
          const { searchQuery, categoryFilter, featuredOnly } = get();
          const params = new URLSearchParams();

          if (categoryFilter) params.append('category', categoryFilter);
          if (searchQuery) params.append('search', searchQuery);
          if (featuredOnly) params.append('featured', 'true');

          const url = `/api/registry/servers${params.toString() ? `?${params.toString()}` : ''}`;
          const response = await fetch(url);

          if (!response.ok) {
            throw new Error(`Failed to fetch registry servers: ${response.statusText}`);
          }

          const data = await response.json();
          set({ registryServers: data.servers || [], isLoading: false });
        } catch (error) {
          console.error('Failed to fetch registry servers:', error);
          set({ error: error.message, isLoading: false });
        }
      },

      fetchCategories: async () => {
        try {
          const response = await fetch('/api/registry/categories');

          if (!response.ok) {
            throw new Error(`Failed to fetch categories: ${response.statusText}`);
          }

          const data = await response.json();
          set({ categories: data.categories || [] });
        } catch (error) {
          console.error('Failed to fetch categories:', error);
          set({ error: error.message });
        }
      },

      fetchFeatured: async () => {
        try {
          const response = await fetch('/api/registry/featured');

          if (!response.ok) {
            throw new Error(`Failed to fetch featured servers: ${response.statusText}`);
          }

          const data = await response.json();
          set({ featuredServers: data.servers || [] });
        } catch (error) {
          console.error('Failed to fetch featured servers:', error);
          set({ error: error.message });
        }
      },

      fetchInstalledServers: async () => {
        try {
          const response = await fetch('/api/registry/installed');

          if (!response.ok) {
            throw new Error(`Failed to fetch installed servers: ${response.statusText}`);
          }

          const data = await response.json();
          set({ installedServers: data.installed || [] });
        } catch (error) {
          console.error('Failed to fetch installed servers:', error);
          set({ error: error.message });
        }
      },

      fetchServerDetails: async (serverId) => {
        set({ isLoading: true, error: null });
        try {
          const response = await fetch(`/api/registry/servers/${serverId}`);

          if (!response.ok) {
            throw new Error(`Failed to fetch server details: ${response.statusText}`);
          }

          const data = await response.json();
          set({ selectedRegistryServer: data.server, isLoading: false });

          return data;
        } catch (error) {
          console.error('Failed to fetch server details:', error);
          set({ error: error.message, isLoading: false });
          throw error;
        }
      },

      installServer: async (serverId, config = {}) => {
        set({ isLoading: true, error: null });
        try {
          const response = await fetch('/api/registry/install', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({
              serverId,
              config,
            }),
          });

          if (!response.ok) {
            const error = await response.text();
            throw new Error(error || `Installation failed: ${response.statusText}`);
          }

          const data = await response.json();
          set({ isLoading: false });

          await get().fetchInstalledServers();
          await get().fetchRegistryServers();

          return data;
        } catch (error) {
          console.error('Failed to install server:', error);
          set({ error: error.message, isLoading: false });
          throw error;
        }
      },

      uninstallServer: async (serverId) => {
        set({ isLoading: true, error: null });
        try {
          const response = await fetch('/api/registry/uninstall', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ serverId }),
          });

          if (!response.ok) {
            const error = await response.text();
            throw new Error(error || `Uninstallation failed: ${response.statusText}`);
          }

          const data = await response.json();
          set({ isLoading: false });

          await get().fetchInstalledServers();
          await get().fetchRegistryServers();

          return data;
        } catch (error) {
          console.error('Failed to uninstall server:', error);
          set({ error: error.message, isLoading: false });
          throw error;
        }
      },

      isServerInstalled: (serverId) => {
        const { installedServers } = get();
        return installedServers.some(
          (installed) => installed.installation.serverId === serverId
        );
      },
    }),
    {
      name: 'dashboard-store',
      enabled: process.env.NODE_ENV === 'development',
    }
  )
);

/**
 * Optimized selectors to prevent unnecessary re-renders
 */

export const selectServers = (state) => state.servers;

export const selectMetrics = (state) => state.metrics;

export const selectSelectedServer = (state) =>
  state.servers.find(s => s.id === state.selectedServerId) || null;

export const selectFilteredServers = (state) => {
  let filtered = state.servers;

  if (state.searchQuery) {
    const query = state.searchQuery.toLowerCase();
    filtered = filtered.filter(s =>
      s.name.toLowerCase().includes(query) ||
      s.id.toLowerCase().includes(query)
    );
  }

  if (state.statusFilter !== 'all') {
    if (state.statusFilter === 'healthy') {
      filtered = filtered.filter(s => s.health?.status === 'healthy');
    } else {
      filtered = filtered.filter(s => s.status === state.statusFilter);
    }
  }

  filtered = [...filtered].sort((a, b) => {
    let compareA, compareB;

    switch (state.sortBy) {
      case 'name':
        compareA = a.name.toLowerCase();
        compareB = b.name.toLowerCase();
        break;
      case 'status':
        compareA = a.status;
        compareB = b.status;
        break;
      case 'health':
        compareA = a.health?.status || 'unknown';
        compareB = b.health?.status || 'unknown';
        break;
      case 'tools':
        compareA = a.toolCount || 0;
        compareB = b.toolCount || 0;
        break;
      default:
        return 0;
    }

    if (compareA < compareB) return state.sortOrder === 'asc' ? -1 : 1;
    if (compareA > compareB) return state.sortOrder === 'asc' ? 1 : -1;

    return 0;
  });

  return filtered;
};

export const selectIsLoading = (state) => state.isLoading;

export const selectError = (state) => state.error;

export const selectLastUpdate = (state) => state.lastUpdate;

export const selectRunningServers = (state) =>
  state.servers.filter(s => s.status === 'running');

export const selectHealthyServers = (state) =>
  state.servers.filter(s => s.health?.status === 'healthy');

export const selectServerById = (serverId) => (state) =>
  state.servers.find(s => s.id === serverId) || null;
