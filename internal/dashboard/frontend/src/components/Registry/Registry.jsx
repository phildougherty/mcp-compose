import React, { useEffect, useState } from 'react';
import {
  MagnifyingGlassIcon,
  FunnelIcon,
  CheckCircleIcon,
  ExclamationCircleIcon,
} from '@heroicons/react/24/outline';
import useRegistryStore from '../../store/registryStore';
import ServerCard from './ServerCard';
import ServerDetails from './ServerDetails';
import CategoryFilter from './CategoryFilter';
import { useToast } from '../shared/Toast';
import Spinner from '../shared/Spinner';

const Registry = () => {
  const {
    servers,
    categories,
    featuredServers,
    installedServers,
    selectedServer,
    loading,
    error,
    filter,
    health,
    setFilter,
    clearFilter,
    fetchServers,
    fetchCategories,
    fetchFeatured,
    fetchInstalledServers,
    fetchServerDetails,
    installServer,
    uninstallServer,
    isServerInstalled,
    clearSelectedServer,
    checkHealth,
  } = useRegistryStore();

  const { showToast } = useToast();
  const [showFilters, setShowFilters] = useState(false);
  const [detailsModalOpen, setDetailsModalOpen] = useState(false);

  useEffect(() => {
    fetchCategories();
    fetchFeatured();
    fetchInstalledServers();
    fetchServers();
    checkHealth();
  }, []);

  useEffect(() => {
    fetchServers();
  }, [filter]);

  const handleSearchChange = (e) => {
    setFilter({ search: e.target.value });
  };

  const handleCategoryChange = (category) => {
    setFilter({ category });
  };

  const handleFeaturedToggle = () => {
    setFilter({ featured: !filter.featured });
  };

  const handleServerClick = async (server) => {
    try {
      await fetchServerDetails(server.id);
      setDetailsModalOpen(true);
    } catch (err) {
      showToast(`Failed to load server details: ${err.message}`, 'error');
    }
  };

  const handleInstall = async (serverId, config) => {
    try {
      const result = await installServer(serverId, config);
      showToast(result.message || 'Server installed successfully', 'success');
      setDetailsModalOpen(false);
      clearSelectedServer();
    } catch (err) {
      showToast(`Installation failed: ${err.message}`, 'error');
    }
  };

  const handleUninstall = async (serverId) => {
    if (!confirm('Are you sure you want to uninstall this server?')) {
      return;
    }

    try {
      const result = await uninstallServer(serverId);
      showToast(result.message || 'Server uninstalled successfully', 'success');
      setDetailsModalOpen(false);
      clearSelectedServer();
    } catch (err) {
      showToast(`Uninstallation failed: ${err.message}`, 'error');
    }
  };

  const handleCloseDetails = () => {
    setDetailsModalOpen(false);
    clearSelectedServer();
  };

  const healthStatus = health?.status || 'unknown';

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            MCP Server Registry
          </h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Discover and install MCP servers with one click
          </p>
        </div>

        <div className="flex items-center gap-2">
          {healthStatus === 'healthy' && (
            <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400">
              <CheckCircleIcon className="h-4 w-4" />
              Healthy
            </span>
          )}
          {healthStatus === 'degraded' && (
            <span className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400">
              <ExclamationCircleIcon className="h-4 w-4" />
              Degraded
            </span>
          )}
        </div>
      </div>

      <div className="flex flex-col sm:flex-row gap-4">
        <div className="flex-1 relative">
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
            <MagnifyingGlassIcon className="h-5 w-5 text-gray-400" aria-hidden="true" />
          </div>
          <input
            type="text"
            value={filter.search}
            onChange={handleSearchChange}
            placeholder="Search servers..."
            className="block w-full rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 pl-10 pr-3 py-2 text-sm text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:border-blue-500 focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
        </div>

        <button
          onClick={() => setShowFilters(!showFilters)}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-blue-500"
        >
          <FunnelIcon className="h-5 w-5" />
          Filters
          {(filter.category || filter.featured) && (
            <span className="inline-flex items-center justify-center w-5 h-5 text-xs font-bold rounded-full bg-blue-500 text-white">
              {(filter.category ? 1 : 0) + (filter.featured ? 1 : 0)}
            </span>
          )}
        </button>

        {(filter.category || filter.search || filter.featured) && (
          <button
            onClick={clearFilter}
            className="px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white"
          >
            Clear Filters
          </button>
        )}
      </div>

      {showFilters && (
        <CategoryFilter
          categories={categories}
          selectedCategory={filter.category}
          onCategoryChange={handleCategoryChange}
          featuredOnly={filter.featured}
          onFeaturedToggle={handleFeaturedToggle}
        />
      )}

      {error && (
        <div className="rounded-lg bg-red-50 dark:bg-red-900/20 p-4">
          <div className="flex">
            <ExclamationCircleIcon className="h-5 w-5 text-red-400" aria-hidden="true" />
            <div className="ml-3">
              <h3 className="text-sm font-medium text-red-800 dark:text-red-400">
                Error loading servers
              </h3>
              <p className="mt-1 text-sm text-red-700 dark:text-red-300">{error}</p>
            </div>
          </div>
        </div>
      )}

      {loading && !servers.length ? (
        <div className="flex items-center justify-center py-12">
          <Spinner size="lg" label="Loading servers..." />
        </div>
      ) : (
        <>
          {installedServers.length > 0 && (
            <div>
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                Installed Servers ({installedServers.length})
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 relative z-0">
                {installedServers.map((installed) => (
                  <ServerCard
                    key={installed.server.id}
                    server={installed.server}
                    installed={true}
                    onClick={() => handleServerClick(installed.server)}
                  />
                ))}
              </div>
            </div>
          )}

          {!filter.search && !filter.category && featuredServers.length > 0 && (
            <div>
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
                Featured Servers
              </h2>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 relative z-0">
                {featuredServers.map((server) => (
                  <ServerCard
                    key={server.id}
                    server={server}
                    installed={isServerInstalled(server.id)}
                    featured={true}
                    onClick={() => handleServerClick(server)}
                  />
                ))}
              </div>
            </div>
          )}

          <div>
            <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
              {filter.search || filter.category ? 'Search Results' : 'All Servers'} ({servers.length})
            </h2>
            {servers.length === 0 ? (
              <div className="text-center py-12">
                <p className="text-gray-500 dark:text-gray-400">
                  No servers found matching your criteria
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 relative z-0">
                {servers.map((server) => (
                  <ServerCard
                    key={server.id}
                    server={server}
                    installed={isServerInstalled(server.id)}
                    onClick={() => handleServerClick(server)}
                  />
                ))}
              </div>
            )}
          </div>
        </>
      )}

      {selectedServer && (
        <ServerDetails
          server={selectedServer}
          open={detailsModalOpen}
          onClose={handleCloseDetails}
          onInstall={handleInstall}
          onUninstall={handleUninstall}
          installed={isServerInstalled(selectedServer.id)}
        />
      )}
    </div>
  );
};

export default Registry;
