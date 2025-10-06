import { useActivityStore, selectAvailableLevels, selectAvailableTypes } from '../../store/activityStore';
import { Select, SearchInput, Button } from '../shared';

/**
 * Activity Filters Component
 *
 * Provides filtering controls for activity events:
 * - Level filter (ERROR, WARN, INFO, DEBUG)
 * - Type filter (request, connection, tool, error)
 * - Search input
 * - Refresh and clear buttons
 */
export default function ActivityFilters({ onRefresh, onClear, isLoading }) {
  const availableLevels = useActivityStore(selectAvailableLevels);
  const availableTypes = useActivityStore(selectAvailableTypes);
  const levelFilter = useActivityStore(state => state.levelFilter);
  const typeFilter = useActivityStore(state => state.typeFilter);
  const searchFilter = useActivityStore(state => state.searchFilter);

  const {
    setLevelFilter,
    setTypeFilter,
    setSearchFilter,
  } = useActivityStore();

  const levelOptions = [
    { value: '', label: 'All Levels' },
    ...availableLevels.map(level => ({ value: level, label: level })),
  ];

  const typeOptions = [
    { value: '', label: 'All Types' },
    ...availableTypes.map(type => ({ value: type, label: type })),
  ];

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
      <div className="flex flex-col lg:flex-row lg:items-end lg:justify-between gap-4">
        <div className="flex flex-col sm:flex-row flex-1 gap-3">
          <div className="relative flex-1 sm:flex-initial sm:min-w-[140px]">
            <label htmlFor="levelFilter" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
              Level
            </label>
            <Select
              id="levelFilter"
              value={levelFilter}
              onChange={setLevelFilter}
              options={levelOptions}
              className="w-full"
            />
          </div>

          <div className="relative flex-1 sm:flex-initial sm:min-w-[140px]">
            <label htmlFor="typeFilter" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
              Type
            </label>
            <Select
              id="typeFilter"
              value={typeFilter}
              onChange={setTypeFilter}
              options={typeOptions}
              className="w-full"
            />
          </div>

          <div className="relative flex-1 sm:min-w-[200px]">
            <label htmlFor="searchFilter" className="block text-xs font-medium text-gray-700 dark:text-gray-300 mb-1.5">
              Search
            </label>
            <SearchInput
              id="searchFilter"
              value={searchFilter}
              onChange={setSearchFilter}
              placeholder="Search activities..."
            />
          </div>
        </div>

        <div className="flex gap-2">
          <Button
            variant="primary"
            onClick={onRefresh}
            disabled={isLoading}
          >
            <svg
              className={`w-4 h-4 mr-2 ${isLoading ? 'animate-spin' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            {isLoading ? 'Loading...' : 'Refresh'}
          </Button>

          <Button
            variant="danger"
            onClick={onClear}
          >
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
            Clear
          </Button>
        </div>
      </div>
    </div>
  );
}
