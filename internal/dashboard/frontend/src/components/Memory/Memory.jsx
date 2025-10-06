import React, { useEffect } from 'react';
import useMemoryStore from '../../store/memoryStore';
import { Button, Spinner, Badge } from '../shared';
import { useToast } from '../../hooks';
import memoryApi from '../../api/memory';
import EntityList from './EntityList';
import EntityForm from './EntityForm';
import RelationForm from './RelationForm';
import SearchView from './SearchView';
import GraphView from './GraphView';
import AnalyticsView from './AnalyticsView';

const Memory = () => {
  const {
    entities,
    relations,
    stats,
    loading,
    error,
    ui,
    setEntities,
    setRelations,
    setLoading,
    setError,
    setActiveTab,
    setShowCreateEntity,
    setShowCreateRelation,
    calculateStats,
  } = useMemoryStore();

  const { success, error: showError } = useToast();

  const tabs = [
    {
      id: 'browse',
      name: 'Browse',
      icon: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z',
      description: 'Browse and manage entities and relationships',
    },
    {
      id: 'search',
      name: 'Search',
      icon: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
      description: 'Advanced search across the memory graph',
    },
    {
      id: 'visualization',
      name: 'Graph',
      icon: 'M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0',
      description: 'Visual graph representation',
    },
    {
      id: 'analytics',
      name: 'Analytics',
      icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
      description: 'Memory statistics and insights',
    },
  ];

  const loadAllData = async () => {
    setLoading('entities', true);
    setError(null);

    try {
      const graph = await memoryApi.getMemoryStats();

      setEntities(graph.entities || []);
      setRelations(graph.relations || []);

      calculateStats();

      success('Memory data loaded successfully');
    } catch (err) {
      const errorMessage = `Failed to load memory data: ${err.message}`;
      setError(errorMessage);
      showError(errorMessage);
    } finally {
      setLoading('entities', false);
    }
  };

  useEffect(() => {
    loadAllData();
  }, []);

  return (
    <div className="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
        <div className="flex flex-col space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-blue-50 dark:bg-blue-900/20 rounded-lg flex items-center justify-center border border-blue-200 dark:border-blue-800">
                  <svg
                    className="w-6 h-6 text-blue-600 dark:text-blue-400"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"
                    />
                  </svg>
                </div>
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center flex-wrap gap-2">
                  Memory Server
                  <Badge variant="primary" size="sm">
                    {stats.totalEntities} entities
                  </Badge>
                  <Badge variant="success" size="sm">
                    {stats.totalRelations} relations
                  </Badge>
                </h3>
                <p className="text-sm text-gray-600 dark:text-gray-400">PostgreSQL-backed persistent knowledge graph</p>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => setShowCreateEntity(true)}
              variant="primary"
              size="md"
            >
              <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
                  clipRule="evenodd"
                />
              </svg>
              Add Entity
            </Button>

            <Button
              onClick={() => setShowCreateRelation(true)}
              variant="secondary"
              size="md"
            >
              <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
                />
              </svg>
              Add Relation
            </Button>

            <Button
              onClick={loadAllData}
              disabled={loading.entities}
              variant="secondary"
              size="md"
            >
              <svg
                className={`w-4 h-4 mr-2 ${loading.entities ? 'animate-spin' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
              Refresh
            </Button>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
            <div className="ml-3 flex-1">
              <h3 className="text-sm font-medium text-red-800 dark:text-red-200">Error</h3>
              <div className="mt-1 text-sm text-red-700 dark:text-red-300">{error}</div>
              <div className="mt-3">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setError(null)}
                >
                  Dismiss
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-2">
        <nav className="flex space-x-1 bg-gray-50 dark:bg-gray-900/60 p-1 rounded-lg">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex-1 px-4 py-2.5 text-sm font-medium rounded-lg transition-all duration-200 min-h-[44px] ${
                ui.activeTab === tab.id
                  ? 'bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-400 border border-blue-200 dark:border-blue-800'
                  : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
              title={tab.description}
            >
              <div className="flex items-center justify-center space-x-2">
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d={tab.icon}
                  />
                </svg>
                <span className="hidden sm:inline">{tab.name}</span>
              </div>
            </button>
          ))}
        </nav>
      </div>

      {ui.activeTab === 'browse' && <EntityList />}
      {ui.activeTab === 'search' && <SearchView />}
      {ui.activeTab === 'visualization' && <GraphView />}
      {ui.activeTab === 'analytics' && <AnalyticsView />}

      <EntityForm />
      <RelationForm />
    </div>
  );
};

export default Memory;
