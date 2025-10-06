/**
 * OAuth State Management
 * Manages OAuth server status, clients, and configuration
 */

import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

/**
 * @typedef {Object} OAuthStatus
 * @property {boolean} oauth_enabled - OAuth server enabled status
 * @property {Object} active_tokens - Active token counts
 * @property {number} active_tokens.access_tokens - Active access tokens
 * @property {number} active_tokens.refresh_tokens - Active refresh tokens
 * @property {number} active_tokens.auth_codes - Active auth codes
 * @property {string} issuer - OAuth issuer URL
 */

/**
 * @typedef {Object} OAuthClient
 * @property {string} client_id - Client ID
 * @property {string} client_secret - Client secret (confidential clients only)
 * @property {string} name - Client name
 * @property {string} description - Client description
 * @property {Array<string>} redirect_uris - Redirect URIs
 * @property {Array<string>} grant_types - Allowed grant types
 * @property {Array<string>} response_types - Allowed response types
 * @property {string} token_endpoint_auth_method - Token endpoint auth method
 * @property {string} scope - Allowed scopes
 * @property {boolean} public - Public client flag
 * @property {string} created_at - Creation timestamp
 */

const useOAuthStore = create(
  devtools(
    (set, get) => ({
      oauthStatus: {
        oauth_enabled: false,
        active_tokens: {
          access_tokens: 0,
          refresh_tokens: 0,
          auth_codes: 0,
        },
        issuer: '',
      },
      clients: [],
      selectedClient: null,
      loading: false,
      error: null,
      searchTerm: '',
      filter: 'all',
      sortBy: 'name',
      autoRefresh: false,

      setOAuthStatus: (status) =>
        set({ oauthStatus: status }, false, 'setOAuthStatus'),

      setClients: (clients) => set({ clients }, false, 'setClients'),

      addClient: (client) =>
        set(
          (state) => ({
            clients: [...state.clients, client],
          }),
          false,
          'addClient'
        ),

      updateClient: (clientId, updates) =>
        set(
          (state) => ({
            clients: state.clients.map((c) =>
              c.client_id === clientId ? { ...c, ...updates } : c
            ),
          }),
          false,
          'updateClient'
        ),

      deleteClient: (clientId) =>
        set(
          (state) => ({
            clients: state.clients.filter((c) => c.client_id !== clientId),
            selectedClient:
              state.selectedClient?.client_id === clientId
                ? null
                : state.selectedClient,
          }),
          false,
          'deleteClient'
        ),

      setSelectedClient: (client) =>
        set({ selectedClient: client }, false, 'setSelectedClient'),

      setLoading: (loading) => set({ loading }, false, 'setLoading'),

      setError: (error) => set({ error }, false, 'setError'),

      setSearchTerm: (searchTerm) =>
        set({ searchTerm }, false, 'setSearchTerm'),

      setFilter: (filter) => set({ filter }, false, 'setFilter'),

      setSortBy: (sortBy) => set({ sortBy }, false, 'setSortBy'),

      setAutoRefresh: (autoRefresh) =>
        set({ autoRefresh }, false, 'setAutoRefresh'),

      getFilteredClients: () => {
        const { clients, searchTerm, filter, sortBy } = get();

        let filtered = clients.filter((client) => {
          const matchesSearch =
            !searchTerm ||
            client.name?.toLowerCase().includes(searchTerm.toLowerCase()) ||
            client.client_id?.toLowerCase().includes(searchTerm.toLowerCase()) ||
            client.description?.toLowerCase().includes(searchTerm.toLowerCase());

          const matchesFilter =
            filter === 'all' ||
            (filter === 'public' && client.public) ||
            (filter === 'confidential' && !client.public);

          return matchesSearch && matchesFilter;
        });

        return filtered.sort((a, b) => {
          switch (sortBy) {
            case 'type':
              const aType = a.public ? 'Public' : 'Confidential';
              const bType = b.public ? 'Public' : 'Confidential';

              return aType.localeCompare(bType);
            case 'created':
              const aDate = new Date(a.created_at || 0);
              const bDate = new Date(b.created_at || 0);

              return bDate - aDate;
            default:
              const aName = a.name || '';
              const bName = b.name || '';

              return aName.localeCompare(bName);
          }
        });
      },

      getStatusCounts: () => {
        const { clients, oauthStatus } = get();

        return {
          total: clients.length,
          public: clients.filter((c) => c.public).length,
          confidential: clients.filter((c) => !c.public).length,
          active: oauthStatus.active_tokens?.access_tokens || 0,
        };
      },

      reset: () =>
        set(
          {
            oauthStatus: {
              oauth_enabled: false,
              active_tokens: {
                access_tokens: 0,
                refresh_tokens: 0,
                auth_codes: 0,
              },
              issuer: '',
            },
            clients: [],
            selectedClient: null,
            loading: false,
            error: null,
            searchTerm: '',
            filter: 'all',
            sortBy: 'name',
            autoRefresh: false,
          },
          false,
          'reset'
        ),
    }),
    { name: 'OAuth Store' }
  )
);

export default useOAuthStore;
