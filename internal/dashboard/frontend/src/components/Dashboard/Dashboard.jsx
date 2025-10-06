import { useState, useEffect, useCallback } from 'react';
import {
  MagnifyingGlassIcon,
  FunnelIcon,
  RectangleStackIcon,
  Squares2X2Icon,
} from '@heroicons/react/24/outline';
import { useDashboardStore, selectFilteredServers, selectMetrics } from '../../store/dashboardStore';
import { useWebSocket } from '../../hooks/useWebSocket';
import { getServers, getStatus, getConnections } from '../../api/dashboard';
import { useToast } from '../../hooks/useToast';
import ServerMetrics from './ServerMetrics';
import ServerFilters from './ServerFilters';
import ServerCard from './ServerCard';
import ProxyControls from './ProxyControls';
import { Spinner, EmptyState, Button, Badge } from '../shared';
import RegistryServerCard from '../Registry/ServerCard';
import RegistryServerDetails from '../Registry/ServerDetails';
import CategoryFilter from '../Registry/CategoryFilter';

const Dashboard = () => {
  const [loading, setLoading] = useState(false);
  const [dashboardError, setDashboardError] = useState(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [refreshFrequency, setRefreshFrequency] = useState(5000);
  const [lastRefreshTime, setLastRefreshTime] = useState(null);
  const [expandedServers, setExpandedServers] = useState(new Set());
  const [serverTools, setServerTools] = useState({});
  const [showCategoryFilters, setShowCategoryFilters] = useState(false);
  const [detailsModalOpen, setDetailsModalOpen] = useState(false);

  const servers = useDashboardStore(selectFilteredServers);
  const metrics = useDashboardStore(selectMetrics);
  const viewMode = useDashboardStore((state) => state.viewMode);
  const registryServers = useDashboardStore((state) => state.registryServers);
  const categories = useDashboardStore((state) => state.categories);
  const featuredServers = useDashboardStore((state) => state.featuredServers);
  const installedServers = useDashboardStore((state) => state.installedServers);
  const selectedRegistryServer = useDashboardStore((state) => state.selectedRegistryServer);
  const categoryFilter = useDashboardStore((state) => state.categoryFilter);
  const featuredOnly = useDashboardStore((state) => state.featuredOnly);
  const searchQuery = useDashboardStore((state) => state.searchQuery);

  const setServers = useDashboardStore((state) => state.setServers);
  const setMetrics = useDashboardStore((state) => state.setMetrics);
  const setViewMode = useDashboardStore((state) => state.setViewMode);
  const setSearchQuery = useDashboardStore((state) => state.setSearchQuery);
  const setCategoryFilter = useDashboardStore((state) => state.setCategoryFilter);
  const setFeaturedOnly = useDashboardStore((state) => state.setFeaturedOnly);
  const fetchRegistryServers = useDashboardStore((state) => state.fetchRegistryServers);
  const fetchCategories = useDashboardStore((state) => state.fetchCategories);
  const fetchFeatured = useDashboardStore((state) => state.fetchFeatured);
  const fetchInstalledServers = useDashboardStore((state) => state.fetchInstalledServers);
  const fetchServerDetails = useDashboardStore((state) => state.fetchServerDetails);
  const installServer = useDashboardStore((state) => state.installServer);
  const uninstallServer = useDashboardStore((state) => state.uninstallServer);
  const isServerInstalled = useDashboardStore((state) => state.isServerInstalled);
  const setSelectedRegistryServer = useDashboardStore((state) => state.setSelectedRegistryServer);

  const toast = useToast();

  const loadData = useCallback(async () => {
    if (loading) return;

    setLoading(true);
    setDashboardError(null);

    try {
      const [serversData, statusData, connectionsData] = await Promise.all([
        getServers(),
        getStatus(),
        getConnections(),
      ]);

      const serversList = Object.entries(serversData).map(([name, config]) => ({
        id: name,
        name,
        ...config,
      }));

      setServers(serversList);

      const statusCounts = calculateStatusCounts(serversList, connectionsData);
      setMetrics({
        totalServers: statusCounts.total,
        runningServers: statusCounts.running,
        healthyServers: statusCounts.healthy,
        totalConnections: statusData.activeHttpConnectionsToServers || 0,
        proxyUptime: statusData.proxyUptime || 0,
      });

      setLastRefreshTime(new Date());
    } catch (err) {
      console.error('Failed to load dashboard data:', err);
      setDashboardError(err.message);
      toast.error('Failed to load dashboard data');
    } finally {
      setLoading(false);
    }
  }, [loading, setServers, setMetrics, toast]);

  const calculateStatusCounts = (serversList, connections) => {
    const stats = {
      total: serversList.length,
      running: 0,
      stopped: 0,
      connected: 0,
      healthy: 0,
    };

    serversList.forEach((server) => {
      const isRunning = isContainerRunning(server);
      if (isRunning) {
        stats.running++;
      } else {
        stats.stopped++;
      }

      const connectionStatus = getConnectionStatus(server, connections);
      if (connectionStatus === 'Connected') {
        stats.connected++;
      }

      if (isServerHealthy(server, connections)) {
        stats.healthy++;
      }
    });

    return stats;
  };

  const isContainerRunning = (server) => {
    if (!server.containerStatus) return false;
    const status = server.containerStatus.toLowerCase().trim();

    return status === 'running' || status === 'up' || status.includes('up ');
  };

  const getConnectionStatus = (server, connections) => {
    if (!connections?.activeHttpConnectionsManagedByProxy) {
      return 'Disconnected';
    }
    const connection = connections.activeHttpConnectionsManagedByProxy[server.name];
    if (!connection) {
      return 'Disconnected';
    }

    return connection.initialized && connection.rawHealthyFlag ? 'Connected' : 'Disconnected';
  };

  const isServerHealthy = (server, connections) => {
    const connectionStatus = getConnectionStatus(server, connections);
    if (connectionStatus === 'Connected') {
      return true;
    }

    return isContainerRunning(server) && server.configProtocol !== 'http';
  };

  const toggleServerExpansion = (serverName) => {
    setExpandedServers((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(serverName)) {
        newSet.delete(serverName);
      } else {
        newSet.add(serverName);
      }

      return newSet;
    });
  };

  const handleToolsDiscovered = (serverName, tools) => {
    setServerTools((prev) => ({
      ...prev,
      [serverName]: tools,
    }));
  };

  const handleToggleAutoRefresh = () => {
    setAutoRefresh((prev) => !prev);
  };

  const handleRegistryServerClick = async (server) => {
    try {
      await fetchServerDetails(server.id);
      setDetailsModalOpen(true);
    } catch (err) {
      toast.error(`Failed to load server details: ${err.message}`);
    }
  };

  const handleInstall = async (serverId, config) => {
    try {
      const result = await installServer(serverId, config);
      toast.success(result.message || 'Server installed successfully');
      setDetailsModalOpen(false);
      setSelectedRegistryServer(null);
      await loadData();
    } catch (err) {
      toast.error(`Installation failed: ${err.message}`);
    }
  };

  const handleUninstall = async (serverId) => {
    if (!confirm('Are you sure you want to uninstall this server?')) {
      return;
    }

    try {
      const result = await uninstallServer(serverId);
      toast.success(result.message || 'Server uninstalled successfully');
      setDetailsModalOpen(false);
      setSelectedRegistryServer(null);
      await loadData();
    } catch (err) {
      toast.error(`Uninstallation failed: ${err.message}`);
    }
  };

  const handleCloseDetails = () => {
    setDetailsModalOpen(false);
    setSelectedRegistryServer(null);
  };

  const handleSearchChange = (e) => {
    setSearchQuery(e.target.value);
  };

  const handleCategoryChange = (category) => {
    setCategoryFilter(category);
  };

  const handleFeaturedToggle = () => {
    setFeaturedOnly(!featuredOnly);
  };

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    let interval;
    if (autoRefresh) {
      interval = setInterval(loadData, refreshFrequency);
    }

    return () => {
      if (interval) clearInterval(interval);
    };
  }, [autoRefresh, refreshFrequency, loadData]);

  useEffect(() => {
    if (viewMode === 'browse-registry') {
      fetchCategories();
      fetchFeatured();
      fetchInstalledServers();
      fetchRegistryServers();
    }
  }, [viewMode]);

  useEffect(() => {
    if (viewMode === 'browse-registry') {
      fetchRegistryServers();
    }
  }, [searchQuery, categoryFilter, featuredOnly]);

  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = `${protocol}//${window.location.host}/ws/dashboard`;

  useWebSocket(wsUrl, {
    autoConnect: true,
    onMessage: (data) => {
      if (data.type === 'server_update') {
        loadData();
      }
    },
  });

  return (
    <div className="w-full max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6 mb-6 mx-4 sm:mx-6 lg:mx-8">
        <div className="flex flex-col space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-gradient-to-br from-blue-500 to-blue-600 rounded-lg flex items-center justify-center shadow-lg">
                  {viewMode === 'my-servers' ? (
                    <svg className="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                    </svg>
                  ) : (
                    <RectangleStackIcon className="w-6 h-6 text-white" />
                  )}
                </div>
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
                  {viewMode === 'my-servers' ? 'MCP Servers' : 'MCP Server Registry'}
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {viewMode === 'my-servers' ? 'Manage and monitor your MCP servers' : 'Discover and install MCP servers'}
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
            <div className="inline-flex rounded-lg border border-gray-300 dark:border-gray-600 p-1 bg-white dark:bg-gray-800">
              <Button
                onClick={() => setViewMode('my-servers')}
                variant={viewMode === 'my-servers' ? 'primary' : 'ghost'}
                size="sm"
                className="gap-2"
              >
                <Squares2X2Icon className="h-4 w-4" />
                My Servers
              </Button>
              <Button
                onClick={() => setViewMode('browse-registry')}
                variant={viewMode === 'browse-registry' ? 'primary' : 'ghost'}
                size="sm"
                className="gap-2"
              >
                <RectangleStackIcon className="h-4 w-4" />
                Browse Registry
              </Button>
            </div>

            {viewMode === 'my-servers' && (
              <ProxyControls
                autoRefresh={autoRefresh}
                onToggleAutoRefresh={handleToggleAutoRefresh}
                refreshFrequency={refreshFrequency}
                onSetRefreshFrequency={setRefreshFrequency}
                lastRefreshTime={lastRefreshTime}
                onRefresh={loadData}
                loading={loading}
              />
            )}
          </div>
        </div>
      </div>

      <main className="px-3 sm:px-4 lg:px-6 py-4 max-w-full overflow-x-hidden w-full">
        {dashboardError && (
          <div className="mb-4 bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg animate-fade-in">
            <div className="flex items-start">
              <div className="flex-shrink-0">
                <svg className="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
                </svg>
              </div>
              <div className="ml-3 flex-1">
                <h3 className="text-sm font-medium text-red-800 dark:text-red-200">Dashboard Error</h3>
                <div className="mt-2 text-sm text-red-700 dark:text-red-300">{dashboardError}</div>
                <div className="mt-3">
                  <Button
                    onClick={() => setDashboardError(null)}
                    variant="danger"
                    size="sm"
                  >
                    Dismiss
                  </Button>
                </div>
              </div>
            </div>
          </div>
        )}

        {viewMode === 'my-servers' ? (
          <div className="space-y-6 animate-fade-in">
            <ServerMetrics metrics={metrics} />

            <ServerFilters />

            {loading && servers.length === 0 ? (
              <div className="flex items-center justify-center py-12">
                <Spinner size="lg" />
              </div>
            ) : servers.length === 0 ? (
              <EmptyState
                icon={
                  <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.172 16.172a4 4 0 015.656 0M9 12h6m-6 4h6M5 16a3 3 0 01-3-3V9a3 3 0 013-3h14a3 3 0 013 3v4a3 3 0 01-3 3H5z" />
                  </svg>
                }
                title="No servers found"
                description="Try adjusting your search or filter criteria, or browse the registry to install new servers."
              />
            ) : (
              <div className="space-y-4">
                {servers.map((server) => (
                  <ServerCard
                    key={server.name}
                    server={server}
                    isExpanded={expandedServers.has(server.name)}
                    onToggleExpansion={() => toggleServerExpansion(server.name)}
                    onToolsDiscovered={(tools) => handleToolsDiscovered(server.name, tools)}
                    serverTools={serverTools[server.name]}
                    loading={loading}
                  />
                ))}
              </div>
            )}
          </div>
        ) : (
          <div className="space-y-6 animate-fade-in">
            <div className="flex flex-col sm:flex-row gap-4">
              <div className="flex-1 relative">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                  <MagnifyingGlassIcon className="h-5 w-5 text-gray-400" aria-hidden="true" />
                </div>
                <input
                  type="text"
                  value={searchQuery}
                  onChange={handleSearchChange}
                  placeholder="Search servers..."
                  className="block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 pl-10 pr-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
                />
              </div>

              <Button
                onClick={() => setShowCategoryFilters(!showCategoryFilters)}
                variant="secondary"
                size="md"
                className="gap-2"
              >
                <FunnelIcon className="h-5 w-5" />
                Filters
                {(categoryFilter || featuredOnly) && (
                  <Badge variant="primary" size="sm">
                    {(categoryFilter ? 1 : 0) + (featuredOnly ? 1 : 0)}
                  </Badge>
                )}
              </Button>

              {(categoryFilter || searchQuery || featuredOnly) && (
                <Button
                  onClick={() => {
                    setSearchQuery('');
                    setCategoryFilter('');
                    setFeaturedOnly(false);
                  }}
                  variant="ghost"
                  size="md"
                >
                  Clear Filters
                </Button>
              )}
            </div>

            {showCategoryFilters && (
              <CategoryFilter
                categories={categories}
                selectedCategory={categoryFilter}
                onCategoryChange={handleCategoryChange}
                featuredOnly={featuredOnly}
                onFeaturedToggle={handleFeaturedToggle}
              />
            )}

            {loading && !registryServers.length ? (
              <div className="flex items-center justify-center py-12">
                <Spinner size="lg" label="Loading servers..." />
              </div>
            ) : (
              <>
                {installedServers.length > 0 && !searchQuery && !categoryFilter && (
                  <div>
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                      Installed Servers ({installedServers.length})
                    </h2>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {installedServers.map((installed) => (
                        <RegistryServerCard
                          key={installed.server.id}
                          server={installed.server}
                          installed={true}
                          onClick={() => handleRegistryServerClick(installed.server)}
                        />
                      ))}
                    </div>
                  </div>
                )}

                {!searchQuery && !categoryFilter && featuredServers.length > 0 && (
                  <div>
                    <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                      Featured Servers
                    </h2>
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {featuredServers.map((server) => (
                        <RegistryServerCard
                          key={server.id}
                          server={server}
                          installed={isServerInstalled(server.id)}
                          featured={true}
                          onClick={() => handleRegistryServerClick(server)}
                        />
                      ))}
                    </div>
                  </div>
                )}

                <div>
                  <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                    {searchQuery || categoryFilter ? 'Search Results' : 'All Servers'} ({registryServers.length})
                  </h2>
                  {registryServers.length === 0 ? (
                    <div className="text-center py-12">
                      <p className="text-gray-500 dark:text-gray-400">
                        No servers found matching your criteria
                      </p>
                    </div>
                  ) : (
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                      {registryServers.map((server) => (
                        <RegistryServerCard
                          key={server.id}
                          server={server}
                          installed={isServerInstalled(server.id)}
                          onClick={() => handleRegistryServerClick(server)}
                        />
                      ))}
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        )}
      </main>

      {selectedRegistryServer && (
        <RegistryServerDetails
          server={selectedRegistryServer}
          open={detailsModalOpen}
          onClose={handleCloseDetails}
          onInstall={handleInstall}
          onUninstall={handleUninstall}
          installed={isServerInstalled(selectedRegistryServer.id)}
        />
      )}
    </div>
  );
};

export default Dashboard;
