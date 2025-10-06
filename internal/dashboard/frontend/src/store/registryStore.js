import { create } from 'zustand';

const useRegistryStore = create((set, get) => ({
  servers: [],
  categories: [],
  featuredServers: [],
  installedServers: [],
  selectedServer: null,
  loading: false,
  error: null,
  filter: {
    category: '',
    search: '',
    featured: false,
  },
  health: {
    registry: 'unknown',
    database: 'unknown',
    status: 'unknown',
  },

  setLoading: (loading) => set({ loading }),
  setError: (error) => set({ error }),

  setFilter: (filter) => set((state) => ({
    filter: { ...state.filter, ...filter },
  })),

  clearFilter: () => set({
    filter: { category: '', search: '', featured: false },
  }),

  fetchServers: async () => {
    set({ loading: true, error: null });
    try {
      const { filter } = get();
      const params = new URLSearchParams();

      if (filter.category) params.append('category', filter.category);
      if (filter.search) params.append('search', filter.search);
      if (filter.featured) params.append('featured', 'true');

      const url = `/api/registry/servers${params.toString() ? `?${params.toString()}` : ''}`;
      const response = await fetch(url);

      if (!response.ok) {
        throw new Error(`Failed to fetch servers: ${response.statusText}`);
      }

      const data = await response.json();
      set({ servers: data.servers || [], loading: false });
    } catch (error) {
      console.error('Failed to fetch servers:', error);
      set({ error: error.message, loading: false });
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
    set({ loading: true, error: null });
    try {
      const response = await fetch(`/api/registry/servers/${serverId}`);

      if (!response.ok) {
        throw new Error(`Failed to fetch server details: ${response.statusText}`);
      }

      const data = await response.json();
      set({ selectedServer: data.server, loading: false });
      return data;
    } catch (error) {
      console.error('Failed to fetch server details:', error);
      set({ error: error.message, loading: false });
      throw error;
    }
  },

  installServer: async (serverId, config = {}) => {
    set({ loading: true, error: null });
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
      set({ loading: false });

      await get().fetchInstalledServers();
      await get().fetchServers();

      return data;
    } catch (error) {
      console.error('Failed to install server:', error);
      set({ error: error.message, loading: false });
      throw error;
    }
  },

  uninstallServer: async (serverId) => {
    set({ loading: true, error: null });
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
      set({ loading: false });

      await get().fetchInstalledServers();
      await get().fetchServers();

      return data;
    } catch (error) {
      console.error('Failed to uninstall server:', error);
      set({ error: error.message, loading: false });
      throw error;
    }
  },

  checkHealth: async () => {
    try {
      const response = await fetch('/api/registry/health');

      if (!response.ok) {
        throw new Error(`Health check failed: ${response.statusText}`);
      }

      const data = await response.json();
      set({ health: data });
      return data;
    } catch (error) {
      console.error('Health check failed:', error);
      set({
        health: {
          registry: 'unhealthy',
          database: 'unhealthy',
          status: 'unhealthy',
          error: error.message,
        }
      });
    }
  },

  isServerInstalled: (serverId) => {
    const { installedServers } = get();
    return installedServers.some(
      (installed) => installed.installation.serverId === serverId
    );
  },

  clearSelectedServer: () => set({ selectedServer: null }),

  reset: () => set({
    servers: [],
    categories: [],
    featuredServers: [],
    installedServers: [],
    selectedServer: null,
    loading: false,
    error: null,
    filter: {
      category: '',
      search: '',
      featured: false,
    },
  }),
}));

export default useRegistryStore;
