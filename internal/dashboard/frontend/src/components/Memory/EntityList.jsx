import React from 'react';
import useMemoryStore from '../../store/memoryStore';
import { SearchInput, Select, Button, EmptyState, Spinner, Pagination } from '../shared';
import EntityCard from './EntityCard';

const EntityList = () => {
  const {
    loading,
    filters,
    pagination,
    ui,
    setFilter,
    setPagination,
    toggleSelectAll,
    clearSelection,
    getFilteredEntities,
    getPaginatedEntities,
    getUniqueEntityTypes,
    setShowCreateEntity,
  } = useMemoryStore();

  const filteredEntities = getFilteredEntities();
  const paginatedEntities = getPaginatedEntities();
  const uniqueEntityTypes = getUniqueEntityTypes();
  const totalPages = Math.ceil(filteredEntities.length / pagination.limit);

  const allSelected =
    paginatedEntities.length > 0 &&
    paginatedEntities.every((entity) => ui.selectedItems.has(entity.name));

  const hasSelection = ui.selectedItems.size > 0;

  const handleDeleteSelected = async () => {
    console.log('Delete selected:', Array.from(ui.selectedItems));
  };

  return (
    <div className="space-y-4">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl p-4 shadow-xl">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
          <div className="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-4 flex-1">
            <div className="flex-1 max-w-md">
              <SearchInput
                value={filters.searchQuery}
                onChange={(value) => setFilter('searchQuery', value)}
                placeholder="Search entities..."
              />
            </div>
            <Select
              value={filters.entityType}
              onChange={(value) => setFilter('entityType', value)}
              className="w-full sm:w-auto"
            >
              <option value="all">All Types</option>
              {uniqueEntityTypes.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </Select>
            <Select
              value={filters.sortBy}
              onChange={(value) => setFilter('sortBy', value)}
              className="w-full sm:w-auto"
            >
              <option value="name">Sort by Name</option>
              <option value="type">Sort by Type</option>
              <option value="observations">Sort by Observations</option>
              <option value="updated">Sort by Updated</option>
            </Select>
          </div>

          {hasSelection && (
            <div className="flex items-center space-x-2">
              <span className="text-sm text-gray-600 dark:text-gray-400">
                {ui.selectedItems.size} selected
              </span>
              <Button variant="danger" size="sm" onClick={handleDeleteSelected}>
                <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M9 2a1 1 0 00-2 0H5a2 2 0 00-2 2v1a1 1 0 001 1h10a1 1 0 001-1V4a2 2 0 00-2-2h-2zM3 8a2 2 0 012-2h6a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V8z"
                    clipRule="evenodd"
                  />
                </svg>
                Delete
              </Button>
              <Button variant="ghost" size="sm" onClick={clearSelection}>
                Clear
              </Button>
            </div>
          )}
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-xl">
        <div className="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center space-x-3">
            <label className="inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={() => toggleSelectAll(paginatedEntities)}
                disabled={paginatedEntities.length === 0}
                className="form-checkbox h-4 w-4 text-purple-600 rounded border-gray-300 dark:border-gray-600 focus:ring-purple-500 focus:ring-offset-gray-900"
              />
              <span className="ml-2 text-sm text-gray-700 dark:text-gray-300">
                Select All ({paginatedEntities.length})
              </span>
            </label>
          </div>
          <div className="text-sm text-gray-500 dark:text-gray-400">
            {filteredEntities.length} entities found
          </div>
        </div>

        {loading.entities ? (
          <div className="p-12 text-center">
            <Spinner size="lg" className="mx-auto mb-4" />
            <p className="text-lg font-medium text-gray-300">Loading entities...</p>
            <p className="text-sm text-gray-500 mt-2">Fetching knowledge graph data</p>
          </div>
        ) : filteredEntities.length === 0 ? (
          <EmptyState
            icon={
              <svg className="w-8 h-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth="2"
                  d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z"
                />
              </svg>
            }
            title="No entities found"
            description={
              filters.searchQuery || filters.entityType !== 'all'
                ? 'Try adjusting your search or filters'
                : 'Start building your knowledge graph by creating entities'
            }
            action={
              !filters.searchQuery && filters.entityType === 'all' ? (
                <Button variant="primary" onClick={() => setShowCreateEntity(true)}>
                  <svg className="w-5 h-5 mr-2" fill="currentColor" viewBox="0 0 20 20">
                    <path
                      fillRule="evenodd"
                      d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
                      clipRule="evenodd"
                    />
                  </svg>
                  Create Your First Entity
                </Button>
              ) : null
            }
          />
        ) : (
          <>
            <div className="divide-y divide-gray-200 dark:divide-gray-700/50">
              {paginatedEntities.map((entity) => (
                <EntityCard key={entity.name} entity={entity} />
              ))}
            </div>

            {totalPages > 1 && (
              <div className="p-4 border-t border-gray-200 dark:border-gray-700">
                <Pagination
                  currentPage={pagination.page}
                  totalPages={totalPages}
                  onPageChange={(page) => setPagination({ page })}
                  totalItems={filteredEntities.length}
                  itemsPerPage={pagination.limit}
                />
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
};

export default EntityList;
