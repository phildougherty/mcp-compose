import React, { useState } from 'react';
import useMemoryStore from '../../store/memoryStore';
import { SearchInput, Select, Button, EmptyState, Spinner, Badge, Input } from '../shared';
import { useToast } from '../../hooks';
import memoryApi from '../../api/memory';

const SearchView = () => {
  const { searchResults, loading, filters, setLoading, setSearchResults, getUniqueEntityTypes } =
    useMemoryStore();
  const { success, error, warning } = useToast();

  const [searchQuery, setSearchQuery] = useState('');
  const [searchType, setSearchType] = useState('all');
  const [filterEntityType, setFilterEntityType] = useState('all');
  const [dateRange, setDateRange] = useState({ start: '', end: '' });

  const uniqueEntityTypes = getUniqueEntityTypes();

  const handleSearch = async () => {
    if (!searchQuery.trim()) {
      warning('Please enter a search query');
      return;
    }

    setLoading('search', true);
    try {
      const options = {
        type: searchType !== 'all' ? searchType : undefined,
        entityType: filterEntityType !== 'all' ? filterEntityType : undefined,
        dateStart: dateRange.start || undefined,
        dateEnd: dateRange.end || undefined,
      };

      const results = await memoryApi.searchMemory(searchQuery, options);
      setSearchResults(results || []);

      success(`Found ${results?.length || 0} results`);
    } catch (err) {
      error(`Search failed: ${err.message}`);
      setSearchResults([]);
    } finally {
      setLoading('search', false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl p-6 shadow-xl">
        <h2 className="text-lg font-semibold text-gray-900 dark:text-white mb-4">
          Advanced Search
        </h2>

        <div className="space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            <div className="lg:col-span-2">
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Search Query
              </label>
              <SearchInput
                value={searchQuery}
                onChange={setSearchQuery}
                placeholder="Search entities, observations, relationships..."
                onKeyPress={(e) => {
                  if (e.key === 'Enter') {
                    handleSearch();
                  }
                }}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Search Type
              </label>
              <Select value={searchType} onChange={setSearchType}>
                <option value="all">All</option>
                <option value="entities">Entities Only</option>
                <option value="relations">Relations Only</option>
                <option value="observations">Observations Only</option>
              </Select>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Entity Type
              </label>
              <Select value={filterEntityType} onChange={setFilterEntityType}>
                <option value="all">All Types</option>
                {uniqueEntityTypes.map((type) => (
                  <option key={type} value={type}>
                    {type}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Date From
              </label>
              <Input
                type="date"
                value={dateRange.start}
                onChange={(e) => setDateRange({ ...dateRange, start: e.target.value })}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Date To
              </label>
              <Input
                type="date"
                value={dateRange.end}
                onChange={(e) => setDateRange({ ...dateRange, end: e.target.value })}
              />
            </div>
          </div>

          <div className="flex justify-end">
            <Button
              variant="primary"
              onClick={handleSearch}
              disabled={loading.search}
              loading={loading.search}
            >
              <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z"
                  clipRule="evenodd"
                />
              </svg>
              Search
            </Button>
          </div>
        </div>

        {loading.search ? (
          <div className="mt-6 py-12 text-center">
            <Spinner size="lg" className="mx-auto mb-4" />
            <p className="text-gray-500 dark:text-gray-400">Searching...</p>
          </div>
        ) : searchResults.length > 0 ? (
          <div className="mt-6">
            <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
              {searchResults.length} Results Found
            </h3>
            <div className="space-y-2">
              {searchResults.map((result, index) => (
                <div
                  key={index}
                  className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 hover:border-blue-300 dark:hover:border-blue-700 transition-colors"
                >
                  <div className="flex items-center justify-between mb-2">
                    <h4 className="font-medium text-gray-900 dark:text-white">{result.name}</h4>
                    <Badge variant="info" size="sm">
                      {result.entityType}
                    </Badge>
                  </div>
                  {result.observations && result.observations.length > 0 && (
                    <p className="text-sm text-gray-600 dark:text-gray-400">
                      {result.observations[0]}
                      {result.observations.length > 1 ? '...' : ''}
                    </p>
                  )}
                </div>
              ))}
            </div>
          </div>
        ) : searchQuery ? (
          <div className="mt-6">
            <EmptyState
              icon={
                <svg className="w-8 h-8" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z"
                    clipRule="evenodd"
                  />
                </svg>
              }
              title="No results found"
              description={`No results found for "${searchQuery}"`}
            />
          </div>
        ) : null}
      </div>
    </div>
  );
};

export default SearchView;
