/**
 * OAuth Client List Component
 * Displays list of OAuth clients with search and filter
 */

import useOAuthStore from '../../store/oauthStore';
import { SearchInput, Select, EmptyState, Button } from '../shared';
import ClientCard from './ClientCard';

export default function ClientList({ onViewDetails, onClientDeleted }) {
  const {
    searchTerm,
    filter,
    sortBy,
    setSearchTerm,
    setFilter,
    setSortBy,
    getFilteredClients,
    loading,
  } = useOAuthStore();

  const filteredClients = getFilteredClients();

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-6">
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0 mb-6">
        <div className="flex items-center space-x-4">
          <div className="w-12 h-12 bg-indigo-50 dark:bg-indigo-900/20 border border-indigo-200 dark:border-indigo-800 rounded-xl flex items-center justify-center">
            <svg
              className="w-6 h-6 text-indigo-600 dark:text-indigo-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
          </div>
          <div>
            <h4 className="text-lg font-bold text-gray-900 dark:text-white">OAuth Clients</h4>
            <p className="text-sm text-gray-500 dark:text-gray-400">Manage registered OAuth applications</p>
          </div>
        </div>
      </div>

      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-3 lg:space-y-0 mb-6 gap-3">
        <div className="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-3xl">
          <SearchInput
            value={searchTerm}
            onChange={setSearchTerm}
            placeholder="Search clients..."
            className="flex-1"
          />

          <Select
            value={filter}
            onChange={setFilter}
            options={[
              { value: 'all', label: 'All Types' },
              { value: 'public', label: 'Public' },
              { value: 'confidential', label: 'Confidential' },
            ]}
            className="w-full sm:w-auto"
          />

          <Select
            value={sortBy}
            onChange={setSortBy}
            options={[
              { value: 'name', label: 'Sort by Name' },
              { value: 'type', label: 'Sort by Type' },
              { value: 'created', label: 'Sort by Created' },
            ]}
            className="w-full sm:w-auto"
          />
        </div>
      </div>

      {filteredClients.length === 0 && !loading ? (
        <EmptyState
          icon={
            <svg
              className="w-12 h-12 text-slate-500"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
              />
            </svg>
          }
          title="No OAuth Clients Found"
          description={
            searchTerm || filter !== 'all'
              ? 'Try adjusting your search or filters'
              : 'Get started by registering your first OAuth client'
          }
        />
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
          {filteredClients.map((client) => (
            <ClientCard
              key={client.client_id}
              client={client}
              onViewDetails={onViewDetails}
              onClientDeleted={onClientDeleted}
            />
          ))}
        </div>
      )}
    </div>
  );
}
