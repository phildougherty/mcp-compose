/**
 * Task Scheduler Store
 * Manages task state, execution status, and filtering
 */

import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

const useTaskStore = create(
  devtools(
    (set, get) => ({
      tasks: [],
      taskRuns: [],
      metrics: {},
      loading: false,
      error: null,
      searchTerm: '',
      filterType: 'all',
      filterStatus: 'all',
      sortBy: 'name',
      autoRefresh: false,
      expandedTasks: new Set(),
      expandedGroups: new Set(),
      showTaskDetails: {},
      showRunOutput: {},

      setTasks: (tasks) => set({ tasks }),
      setTaskRuns: (taskRuns) => set({ taskRuns }),
      setMetrics: (metrics) => set({ metrics }),
      setLoading: (loading) => set({ loading }),
      setError: (error) => set({ error }),
      setSearchTerm: (searchTerm) => set({ searchTerm }),
      setFilterType: (filterType) => set({ filterType }),
      setFilterStatus: (filterStatus) => set({ filterStatus }),
      setSortBy: (sortBy) => set({ sortBy }),
      setAutoRefresh: (autoRefresh) => set({ autoRefresh }),

      toggleTaskExpansion: (taskId) => set((state) => {
        const newSet = new Set(state.expandedTasks);
        if (newSet.has(taskId)) {
          newSet.delete(taskId);
        } else {
          newSet.add(taskId);
        }

        return { expandedTasks: newSet };
      }),

      toggleGroupExpansion: (groupKey) => set((state) => {
        const newSet = new Set(state.expandedGroups);
        if (newSet.has(groupKey)) {
          newSet.delete(groupKey);
        } else {
          newSet.add(groupKey);
        }

        return { expandedGroups: newSet };
      }),

      isTaskExpanded: (taskId) => {
        return get().expandedTasks.has(taskId);
      },

      isGroupExpanded: (groupKey) => {
        return get().expandedGroups.has(groupKey);
      },

      showRunOutputModal: (outputKey, data) => set((state) => ({
        showRunOutput: {
          ...state.showRunOutput,
          [outputKey]: data,
        },
      })),

      closeRunOutputModal: (outputKey) => set((state) => {
        const newShowRunOutput = { ...state.showRunOutput };
        delete newShowRunOutput[outputKey];

        return { showRunOutput: newShowRunOutput };
      }),

      getFilteredTasks: () => {
        const { tasks, searchTerm, filterType, filterStatus, sortBy } = get();

        let filtered = tasks.filter((task) => {
          const matchesSearch =
            !searchTerm ||
            task.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
            (task.description && task.description.toLowerCase().includes(searchTerm.toLowerCase())) ||
            (task.command && task.command.toLowerCase().includes(searchTerm.toLowerCase())) ||
            (task.prompt && task.prompt.toLowerCase().includes(searchTerm.toLowerCase()));

          const matchesType = filterType === 'all' || task.type === filterType;

          const matchesStatus =
            filterStatus === 'all' ||
            (filterStatus === 'enabled' && task.enabled) ||
            (filterStatus === 'disabled' && !task.enabled);

          return matchesSearch && matchesType && matchesStatus;
        });

        return filtered.sort((a, b) => {
          switch (sortBy) {
            case 'type':
              return a.type.localeCompare(b.type);
            case 'schedule':
              return (a.schedule || '').localeCompare(b.schedule || '');
            case 'status':
              return b.enabled - a.enabled;
            case 'lastRun': {
              const aRun = get().getLastRun(a.id);
              const bRun = get().getLastRun(b.id);
              if (!aRun && !bRun) return 0;
              if (!aRun) return 1;
              if (!bRun) return -1;

              return new Date(bRun.timestamp) - new Date(aRun.timestamp);
            }
            default:
              return a.name.localeCompare(b.name);
          }
        });
      },

      getTaskGroups: () => {
        const filteredTasks = get().getFilteredTasks();
        const expandedGroups = get().expandedGroups;
        const groups = {};

        filteredTasks.forEach((task) => {
          let normalizedType = task.type;
          if (task.type === 'AIs' || task.type === 'ais') {
            normalizedType = 'ai';
          } else if (task.type === 'Shell_commands' || task.type === 'shell_commands' || task.type === 'shells') {
            normalizedType = 'shell';
          }

          const groupKey = normalizedType;

          if (!groups[groupKey]) {
            groups[groupKey] = {
              type: normalizedType,
              tasks: [],
              expanded: expandedGroups.has(groupKey),
            };
          }

          groups[groupKey].tasks.push(task);
        });

        Object.keys(groups).forEach((key) => {
          if (!groups[key].tasks || groups[key].tasks.length === 0) {
            delete groups[key];
          }
        });

        return groups;
      },

      getLastRun: (taskId) => {
        const { taskRuns } = get();

        return taskRuns
          .filter((run) => run.task_id === taskId)
          .sort((a, b) => {
            const timestampA = a.last_run || a.lastRun || a.timestamp;
            const timestampB = b.last_run || b.lastRun || b.timestamp;
            if (!timestampA || !timestampB) return 0;

            return new Date(timestampB) - new Date(timestampA);
          })[0];
      },

      getRecentRuns: (taskId, limit = 10) => {
        const { taskRuns } = get();

        return taskRuns
          .filter((run) => run.task_id === taskId)
          .sort((a, b) => {
            const timestampA = a.last_run || a.lastRun || a.timestamp;
            const timestampB = b.last_run || b.lastRun || b.timestamp;
            if (!timestampA || !timestampB) return 0;

            return new Date(timestampB) - new Date(timestampA);
          })
          .slice(0, limit);
      },

      getTaskStats: () => {
        const { tasks, taskRuns } = get();
        const stats = {
          total: tasks.length,
          enabled: tasks.filter((t) => t.enabled).length,
          shell: tasks.filter((t) => t.type === 'shell').length,
          ai: tasks.filter((t) => t.type === 'ai').length,
          dependency: tasks.filter((t) => t.type === 'dependency').length,
          watcher: tasks.filter((t) => t.type === 'watcher').length,
          manual: tasks.filter((t) => t.type === 'manual').length,
          runningNow: taskRuns.filter((r) => r.status === 'running').length,
          completedToday: 0,
          failedRecent: 0,
        };

        const twentyFourHoursAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);

        taskRuns.forEach((run) => {
          const timestamp = run.last_run || run.lastRun || run.timestamp || run.created_at || run.finished_at;

          if (!timestamp) return;

          try {
            const runDate = new Date(timestamp);

            if (isNaN(runDate.getTime())) return;

            if (runDate > twentyFourHoursAgo) {
              const status = run.status?.toLowerCase();
              if (status === 'completed' || status === 'success') {
                stats.completedToday++;
              } else if (status === 'failed' || status === 'error' || status === 'failure') {
                stats.failedRecent++;
              }
            }
          } catch (error) {
            console.error('Error parsing date for run:', run, error);
          }
        });

        return stats;
      },

      getUniqueTaskTypes: () => {
        const { tasks } = get();
        const types = new Set(tasks.map((t) => t.type));

        return Array.from(types);
      },

      getAvailableDependencies: () => {
        const { tasks } = get();

        return tasks.filter((t) => t.type !== 'dependency');
      },
    }),
    { name: 'TaskStore' }
  )
);

export default useTaskStore;
