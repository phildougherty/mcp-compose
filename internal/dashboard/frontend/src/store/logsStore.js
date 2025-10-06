import { create } from 'zustand';

export const useLogsStore = create((set, get) => ({
  logs: [],
  selectedServer: '',
  loading: false,
  streaming: false,
  error: null,

  autoScroll: true,
  searchTerm: '',
  filterLevel: 'all',
  showTimestamps: true,
  lineWrap: false,
  highlightErrors: true,

  setLogs: (logs) => set({ logs }),

  setSelectedServer: (server) => set({
    selectedServer: server,
    logs: [],
    error: null
  }),

  setLoading: (loading) => set({ loading }),

  setStreaming: (streaming) => set({ streaming }),

  setError: (error) => set({ error }),

  addLog: (log) => set((state) => {
    const newLogs = [...state.logs, log];
    if (newLogs.length > 1000) {
      return { logs: newLogs.slice(-1000) };
    }
    return { logs: newLogs };
  }),

  clearLogs: () => set({ logs: [] }),

  setAutoScroll: (autoScroll) => set({ autoScroll }),

  setSearchTerm: (searchTerm) => set({ searchTerm }),

  setFilterLevel: (filterLevel) => set({ filterLevel }),

  setShowTimestamps: (showTimestamps) => set({ showTimestamps }),

  setLineWrap: (lineWrap) => set({ lineWrap }),

  setHighlightErrors: (highlightErrors) => set({ highlightErrors }),

  filteredLogs: () => {
    const { logs, searchTerm, filterLevel } = get();
    return logs.filter(log => {
      const matchesSearch = !searchTerm ||
        log.message.toLowerCase().includes(searchTerm.toLowerCase());
      const matchesLevel = filterLevel === 'all' || log.level === filterLevel;
      return matchesSearch && matchesLevel;
    });
  },

  logStats: () => {
    const { logs } = get();
    return {
      total: logs.length,
      errors: logs.filter(log => log.level === 'ERROR').length,
      warnings: logs.filter(log => log.level === 'WARN').length,
      info: logs.filter(log => log.level === 'INFO').length,
      debug: logs.filter(log => log.level === 'DEBUG').length,
    };
  },
}));
