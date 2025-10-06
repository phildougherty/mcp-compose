/**
 * Audit Store - Zustand state management for audit logs
 */

import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

const useAuditStore = create(
  devtools(
    (set, get) => ({
      entries: [],
      stats: null,
      loading: false,
      error: null,
      selectedEntry: null,

      filters: {
        event: '',
        success: '',
        timeRange: '24h',
        search: '',
      },

      currentPage: 1,
      pageSize: 20,
      totalEntries: 0,
      totalPages: 0,

      sortBy: 'timestamp',
      sortOrder: 'desc',
      autoRefresh: false,

      setEntries: (entries) => set({ entries }),

      setStats: (stats) => set({ stats }),

      setLoading: (loading) => set({ loading }),

      setError: (error) => set({ error }),

      setSelectedEntry: (entry) => set({ selectedEntry: entry }),

      clearSelectedEntry: () => set({ selectedEntry: null }),

      setFilter: (key, value) =>
        set((state) => ({
          filters: { ...state.filters, [key]: value },
          currentPage: 1,
        })),

      setFilters: (filters) =>
        set({
          filters: { ...filters },
          currentPage: 1,
        }),

      resetFilters: () =>
        set({
          filters: {
            event: '',
            success: '',
            timeRange: '24h',
            search: '',
          },
          currentPage: 1,
        }),

      setPage: (page) => set({ currentPage: page }),

      setPageSize: (pageSize) =>
        set({
          pageSize,
          currentPage: 1,
        }),

      setPagination: (totalEntries, totalPages) =>
        set({ totalEntries, totalPages }),

      setSort: (field) => {
        const state = get();
        const sortOrder = state.sortBy === field && state.sortOrder === 'asc' ? 'desc' : 'asc';
        set({ sortBy: field, sortOrder });
      },

      setAutoRefresh: (autoRefresh) => set({ autoRefresh }),

      nextPage: () => {
        const state = get();
        if (state.currentPage < state.totalPages) {
          set({ currentPage: state.currentPage + 1 });
        }
      },

      previousPage: () => {
        const state = get();
        if (state.currentPage > 1) {
          set({ currentPage: state.currentPage - 1 });
        }
      },

      clearError: () => set({ error: null }),
    }),
    { name: 'auditStore' }
  )
);

export default useAuditStore;
