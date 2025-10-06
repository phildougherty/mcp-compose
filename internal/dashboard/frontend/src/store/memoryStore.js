import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

const useMemoryStore = create(
  devtools(
    (set, get) => ({
      entities: [],
      relations: [],
      searchResults: [],
      stats: {
        totalEntities: 0,
        totalRelations: 0,
        entityTypes: {},
        relationTypes: {},
      },

      loading: {
        entities: false,
        relations: false,
        search: false,
        operations: false,
      },

      error: null,

      filters: {
        searchQuery: '',
        entityType: 'all',
        sortBy: 'name',
        sortDirection: 'asc',
        dateRange: {
          start: '',
          end: '',
        },
      },

      pagination: {
        page: 1,
        limit: 50,
        total: 0,
      },

      ui: {
        activeTab: 'browse',
        selectedEntity: null,
        expandedEntities: new Set(),
        selectedItems: new Set(),
        showCreateEntity: false,
        showCreateRelation: false,
      },

      newEntity: {
        name: '',
        type: '',
        observations: [''],
      },

      newRelation: {
        from: '',
        to: '',
        type: '',
      },

      setEntities: (entities) =>
        set(
          (state) => ({
            entities,
            pagination: {
              ...state.pagination,
              total: entities.length,
            },
          }),
          false,
          'setEntities'
        ),

      setRelations: (relations) =>
        set({ relations }, false, 'setRelations'),

      setSearchResults: (searchResults) =>
        set({ searchResults }, false, 'setSearchResults'),

      setStats: (stats) =>
        set({ stats }, false, 'setStats'),

      setLoading: (key, value) =>
        set(
          (state) => ({
            loading: {
              ...state.loading,
              [key]: value,
            },
          }),
          false,
          'setLoading'
        ),

      setError: (error) =>
        set({ error }, false, 'setError'),

      setFilter: (key, value) =>
        set(
          (state) => ({
            filters: {
              ...state.filters,
              [key]: value,
            },
          }),
          false,
          'setFilter'
        ),

      setFilters: (filters) =>
        set(
          (state) => ({
            filters: {
              ...state.filters,
              ...filters,
            },
          }),
          false,
          'setFilters'
        ),

      setPagination: (updates) =>
        set(
          (state) => ({
            pagination: {
              ...state.pagination,
              ...updates,
            },
          }),
          false,
          'setPagination'
        ),

      setActiveTab: (activeTab) =>
        set(
          (state) => ({
            ui: {
              ...state.ui,
              activeTab,
            },
          }),
          false,
          'setActiveTab'
        ),

      setSelectedEntity: (selectedEntity) =>
        set(
          (state) => ({
            ui: {
              ...state.ui,
              selectedEntity,
            },
          }),
          false,
          'setSelectedEntity'
        ),

      toggleEntityExpansion: (entityName) =>
        set(
          (state) => {
            const expandedEntities = new Set(state.ui.expandedEntities);
            if (expandedEntities.has(entityName)) {
              expandedEntities.delete(entityName);
            } else {
              expandedEntities.add(entityName);
            }
            return {
              ui: {
                ...state.ui,
                expandedEntities,
              },
            };
          },
          false,
          'toggleEntityExpansion'
        ),

      toggleEntitySelection: (entityName) =>
        set(
          (state) => {
            const selectedItems = new Set(state.ui.selectedItems);
            if (selectedItems.has(entityName)) {
              selectedItems.delete(entityName);
            } else {
              selectedItems.add(entityName);
            }
            return {
              ui: {
                ...state.ui,
                selectedItems,
              },
            };
          },
          false,
          'toggleEntitySelection'
        ),

      toggleSelectAll: (entities) =>
        set(
          (state) => {
            const allSelected = entities.every((entity) =>
              state.ui.selectedItems.has(entity.name)
            );
            const selectedItems = new Set(state.ui.selectedItems);

            if (allSelected) {
              entities.forEach((entity) => selectedItems.delete(entity.name));
            } else {
              entities.forEach((entity) => selectedItems.add(entity.name));
            }

            return {
              ui: {
                ...state.ui,
                selectedItems,
              },
            };
          },
          false,
          'toggleSelectAll'
        ),

      clearSelection: () =>
        set(
          (state) => ({
            ui: {
              ...state.ui,
              selectedItems: new Set(),
            },
          }),
          false,
          'clearSelection'
        ),

      setShowCreateEntity: (show) =>
        set(
          (state) => ({
            ui: {
              ...state.ui,
              showCreateEntity: show,
            },
          }),
          false,
          'setShowCreateEntity'
        ),

      setShowCreateRelation: (show) =>
        set(
          (state) => ({
            ui: {
              ...state.ui,
              showCreateRelation: show,
            },
          }),
          false,
          'setShowCreateRelation'
        ),

      setNewEntity: (updates) =>
        set(
          (state) => ({
            newEntity: {
              ...state.newEntity,
              ...updates,
            },
          }),
          false,
          'setNewEntity'
        ),

      resetNewEntity: () =>
        set(
          {
            newEntity: {
              name: '',
              type: '',
              observations: [''],
            },
          },
          false,
          'resetNewEntity'
        ),

      addObservationField: () =>
        set(
          (state) => ({
            newEntity: {
              ...state.newEntity,
              observations: [...state.newEntity.observations, ''],
            },
          }),
          false,
          'addObservationField'
        ),

      removeObservationField: (index) =>
        set(
          (state) => ({
            newEntity: {
              ...state.newEntity,
              observations: state.newEntity.observations.filter(
                (_, i) => i !== index
              ),
            },
          }),
          false,
          'removeObservationField'
        ),

      updateObservationField: (index, value) =>
        set(
          (state) => ({
            newEntity: {
              ...state.newEntity,
              observations: state.newEntity.observations.map((obs, i) =>
                i === index ? value : obs
              ),
            },
          }),
          false,
          'updateObservationField'
        ),

      setNewRelation: (updates) =>
        set(
          (state) => ({
            newRelation: {
              ...state.newRelation,
              ...updates,
            },
          }),
          false,
          'setNewRelation'
        ),

      resetNewRelation: () =>
        set(
          {
            newRelation: {
              from: '',
              to: '',
              type: '',
            },
          },
          false,
          'resetNewRelation'
        ),

      getFilteredEntities: () => {
        const state = get();
        let filtered = [...state.entities];

        if (state.filters.entityType !== 'all') {
          filtered = filtered.filter(
            (entity) => entity.entityType === state.filters.entityType
          );
        }

        if (state.filters.searchQuery) {
          const query = state.filters.searchQuery.toLowerCase();
          filtered = filtered.filter((entity) => {
            return (
              entity.name.toLowerCase().includes(query) ||
              entity.entityType.toLowerCase().includes(query) ||
              (entity.observations &&
                entity.observations.some((obs) =>
                  obs.toLowerCase().includes(query)
                ))
            );
          });
        }

        if (state.filters.dateRange.start || state.filters.dateRange.end) {
          filtered = filtered.filter((entity) => {
            const entityDate = new Date(
              entity.updatedAt || entity.createdAt || 0
            );
            const start = state.filters.dateRange.start
              ? new Date(state.filters.dateRange.start)
              : null;
            const end = state.filters.dateRange.end
              ? new Date(state.filters.dateRange.end)
              : null;

            if (start && entityDate < start) return false;
            if (end && entityDate > end) return false;
            return true;
          });
        }

        filtered.sort((a, b) => {
          let aVal, bVal;
          switch (state.filters.sortBy) {
            case 'type':
              aVal = a.entityType;
              bVal = b.entityType;
              break;
            case 'observations':
              aVal = a.observations ? a.observations.length : 0;
              bVal = b.observations ? b.observations.length : 0;
              break;
            case 'updated':
              aVal = new Date(a.updatedAt || a.createdAt || 0);
              bVal = new Date(b.updatedAt || b.createdAt || 0);
              break;
            default:
              aVal = a.name;
              bVal = b.name;
          }

          if (state.filters.sortDirection === 'desc') {
            [aVal, bVal] = [bVal, aVal];
          }

          if (typeof aVal === 'string') {
            return aVal.localeCompare(bVal);
          }
          return aVal - bVal;
        });

        return filtered;
      },

      getPaginatedEntities: () => {
        const state = get();
        const filtered = state.getFilteredEntities();
        const start = (state.pagination.page - 1) * state.pagination.limit;
        const end = start + state.pagination.limit;
        return filtered.slice(start, end);
      },

      getUniqueEntityTypes: () => {
        const state = get();
        const types = new Set(state.entities.map((e) => e.entityType));
        return Array.from(types).sort();
      },

      getEntityRelations: (entityName) => {
        const state = get();
        return state.relations.filter(
          (rel) => rel.from === entityName || rel.to === entityName
        );
      },

      calculateStats: () => {
        const state = get();

        const entityTypes = {};
        state.entities.forEach((entity) => {
          entityTypes[entity.entityType] =
            (entityTypes[entity.entityType] || 0) + 1;
        });

        const relationTypes = {};
        state.relations.forEach((relation) => {
          relationTypes[relation.relationType] =
            (relationTypes[relation.relationType] || 0) + 1;
        });

        set(
          {
            stats: {
              totalEntities: state.entities.length,
              totalRelations: state.relations.length,
              entityTypes,
              relationTypes,
            },
          },
          false,
          'calculateStats'
        );
      },

      reset: () =>
        set(
          {
            entities: [],
            relations: [],
            searchResults: [],
            stats: {
              totalEntities: 0,
              totalRelations: 0,
              entityTypes: {},
              relationTypes: {},
            },
            loading: {
              entities: false,
              relations: false,
              search: false,
              operations: false,
            },
            error: null,
            filters: {
              searchQuery: '',
              entityType: 'all',
              sortBy: 'name',
              sortDirection: 'asc',
              dateRange: {
                start: '',
                end: '',
              },
            },
            pagination: {
              page: 1,
              limit: 50,
              total: 0,
            },
            ui: {
              activeTab: 'browse',
              selectedEntity: null,
              expandedEntities: new Set(),
              selectedItems: new Set(),
              showCreateEntity: false,
              showCreateRelation: false,
            },
            newEntity: {
              name: '',
              type: '',
              observations: [''],
            },
            newRelation: {
              from: '',
              to: '',
              type: '',
            },
          },
          false,
          'reset'
        ),
    }),
    {
      name: 'memory-store',
    }
  )
);

export default useMemoryStore;
