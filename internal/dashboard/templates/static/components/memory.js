const MemoryViewer = {
    props: ['config'],
    data() {
        return {
            // Core data
            entities: [],
            relations: [],
            searchResults: [],
            loadingState: {
                entities: false,
                relations: false,
                search: false,
                operations: false
            },
            error: null,
            
            // Search and filtering
            searchQuery: '',
            searchType: 'all', // all, entities, relations, observations
            filterEntityType: 'all',
            sortBy: 'name',
            sortDirection: 'asc',
            dateRange: {
                start: '',
                end: ''
            },
            
            // Pagination
            pagination: {
                page: 1,
                limit: 50,
                total: 0
            },
            
            // View state
            activeTab: 'browse',
            selectedEntity: null,
            expandedEntities: new Set(),
            viewMode: 'list', // list, graph, timeline
            
            // Graph visualization
            graphData: {
                nodes: [],
                edges: []
            },
            graphConfig: {
                physics: true,
                hierarchical: false,
                showLabels: true,
                maxNodes: 100
            },
            
            // Entity management
            showCreateEntity: false,
            showDeleteConfirm: null,
            editingEntity: null,
            newEntity: {
                name: '',
                type: '',
                observations: ['']
            },
            
            // Relationship management
            showCreateRelation: false,
            newRelation: {
                from: '',
                to: '',
                type: ''
            },
            
            // Bulk operations
            selectedItems: new Set(),
            bulkActionType: '',
            
            // Statistics
            stats: {
                totalEntities: 0,
                totalRelations: 0,
                entityTypes: {},
                relationTypes: {},
                recentActivity: []
            },
            
            // Auto-refresh
            autoRefresh: false,
            refreshInterval: null,
            lastRefreshTime: null
        }
    },
    
    computed: {
        tabs() {
            return [
                {
                    id: 'browse',
                    name: 'Browse',
                    icon: 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z',
                    description: 'Browse and manage entities and relationships'
                },
                {
                    id: 'search',
                    name: 'Search',
                    icon: 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
                    description: 'Advanced search across the memory graph'
                },
                {
                    id: 'visualization',
                    name: 'Graph',
                    icon: 'M8.111 16.404a5.5 5.5 0 017.778 0M12 20h.01m-7.08-7.071c3.904-3.905 10.236-3.905 14.141 0M1.394 9.393c5.857-5.857 15.355-5.857 21.213 0',
                    description: 'Visual graph representation'
                },
                {
                    id: 'analytics',
                    name: 'Analytics',
                    icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
                    description: 'Memory statistics and insights'
                }
            ];
        },
        
        filteredEntities() {
            let filtered = this.entities;
            
            // Filter by entity type
            if (this.filterEntityType !== 'all') {
                filtered = filtered.filter(entity => entity.entityType === this.filterEntityType);
            }
            
            // Filter by search query
            if (this.searchQuery) {
                const query = this.searchQuery.toLowerCase();
                filtered = filtered.filter(entity => {
                    return entity.name.toLowerCase().includes(query) ||
                           entity.entityType.toLowerCase().includes(query) ||
                           entity.observations.some(obs => obs.toLowerCase().includes(query));
                });
            }
            
            // Filter by date range
            if (this.dateRange.start || this.dateRange.end) {
                filtered = filtered.filter(entity => {
                    const entityDate = new Date(entity.createdAt || entity.updatedAt);
                    const start = this.dateRange.start ? new Date(this.dateRange.start) : null;
                    const end = this.dateRange.end ? new Date(this.dateRange.end) : null;
                    
                    if (start && entityDate < start) return false;
                    if (end && entityDate > end) return false;
                    return true;
                });
            }
            
            // Sort entities
            filtered.sort((a, b) => {
                let aVal, bVal;
                switch (this.sortBy) {
                    case 'type':
                        aVal = a.entityType;
                        bVal = b.entityType;
                        break;
                    case 'observations':
                        aVal = a.observations.length;
                        bVal = b.observations.length;
                        break;
                    case 'updated':
                        aVal = new Date(a.updatedAt || a.createdAt || 0);
                        bVal = new Date(b.updatedAt || b.createdAt || 0);
                        break;
                    default:
                        aVal = a.name;
                        bVal = b.name;
                }
                
                if (this.sortDirection === 'desc') {
                    [aVal, bVal] = [bVal, aVal];
                }
                
                if (typeof aVal === 'string') {
                    return aVal.localeCompare(bVal);
                }
                return aVal - bVal;
            });
            
            return filtered;
        },
        
        paginatedEntities() {
            const start = (this.pagination.page - 1) * this.pagination.limit;
            const end = start + this.pagination.limit;
            return this.filteredEntities.slice(start, end);
        },
        
        totalPages() {
            return Math.ceil(this.filteredEntities.length / this.pagination.limit);
        },
        
        uniqueEntityTypes() {
            const types = new Set(this.entities.map(e => e.entityType));
            return Array.from(types).sort();
        },
        
        selectAllChecked() {
            if (this.paginatedEntities.length === 0) return false;
            return this.paginatedEntities.every(entity => this.selectedItems.has(entity.name));
        },
        
        hasSelection() {
            return this.selectedItems.size > 0;
        }
    },
    
    methods: {
        // Core API methods
        async memoryRequest(endpoint, method = 'GET', data = null) {
            const url = endpoint.startsWith('/') ? endpoint : `/${endpoint}`;
            const options = {
                method,
                headers: {
                    'Content-Type': 'application/json'
                }
            };
            
            if (this.config.apiKey) {
                options.headers['Authorization'] = `Bearer ${this.config.apiKey}`;
            }
            
            if (data) {
                options.body = JSON.stringify(data);
            }
            
            // Use the MCP inspector to make the call to the memory server
            const inspectorRequest = {
                sessionId: await this.getOrCreateSession(),
                method: endpoint.startsWith('/tools/') ? endpoint.substring(7) : 'tools/call',
                params: {
                    name: this.getToolName(endpoint),
                    arguments: data || {}
                }
            };
            
            const response = await fetch('/api/inspector/request', {
                method: 'POST',
                headers: options.headers,
                body: JSON.stringify(inspectorRequest)
            });
            
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            
            const result = await response.json();
            if (result.error) {
                throw new Error(result.error.message || 'Memory server error');
            }
            
            return this.parseToolResult(result.result);
        },
        
        async getOrCreateSession() {
            if (this.sessionId) return this.sessionId;
            
            const response = await fetch('/api/inspector/connect', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    ...(this.config.apiKey && { 'Authorization': `Bearer ${this.config.apiKey}` })
                },
                body: JSON.stringify({ server: 'memory' })
            });
            
            if (!response.ok) {
                throw new Error('Failed to connect to memory server');
            }
            
            const result = await response.json();
            this.sessionId = result.sessionId;
            return this.sessionId;
        },
        
        getToolName(endpoint) {
            const toolMap = {
                '/entities': 'open_nodes',
                '/entities/search': 'search_nodes',
                '/entities/create': 'create_entities',
                '/entities/delete': 'delete_entities',
                '/relations/create': 'create_relations',
                '/relations/delete': 'delete_relations',
                '/observations/add': 'add_observations',
                '/observations/delete': 'delete_observations',
                '/graph': 'read_graph'
            };
            
            return toolMap[endpoint] || 'read_graph';
        },
        
        parseToolResult(result) {
            if (result && result.content && Array.isArray(result.content)) {
                const textContent = result.content.find(item => item.type === 'text');
                if (textContent && textContent.text) {
                    try {
                        return JSON.parse(textContent.text);
                    } catch {
                        return textContent.text;
                    }
                }
            }
            return result;
        },
        
        // Data loading methods
        async loadAllData() {
            await Promise.all([
                this.loadEntities(),
                this.loadRelations(),
                this.loadStats()
            ]);
        },
        
        async loadEntities() {
            this.loadingState.entities = true;
            this.error = null;
            
            try {
                // Load all entities from the graph
                const graph = await this.memoryRequest('/graph');
                this.entities = graph.entities || [];
                this.relations = graph.relations || [];
                
                // Update pagination total
                this.pagination.total = this.entities.length;
                
                this.lastRefreshTime = new Date();
                
            } catch (err) {
                this.error = `Failed to load entities: ${err.message}`;
                this.showToast(this.error, 'error');
            } finally {
                this.loadingState.entities = false;
            }
        },
        
        async loadRelations() {
            this.loadingState.relations = true;
            try {
                // Relations are loaded as part of the graph in loadEntities
                // This method exists for future separate relation loading if needed
            } catch (err) {
                this.showToast(`Failed to load relations: ${err.message}`, 'error');
            } finally {
                this.loadingState.relations = false;
            }
        },
        
        async loadStats() {
            try {
                this.stats = {
                    totalEntities: this.entities.length,
                    totalRelations: this.relations.length,
                    entityTypes: this.getEntityTypeCount(),
                    relationTypes: this.getRelationTypeCount(),
                    recentActivity: this.getRecentActivity()
                };
            } catch (err) {
                console.warn('Failed to calculate stats:', err);
            }
        },
        
        // Search methods
        async performSearch() {
            if (!this.searchQuery.trim()) {
                this.searchResults = [];
                return;
            }
            
            this.loadingState.search = true;
            try {
                const results = await this.memoryRequest('/entities/search', 'POST', {
                    query: this.searchQuery,
                    entityTypes: this.filterEntityType === 'all' ? undefined : [this.filterEntityType]
                });
                
                this.searchResults = results || [];
                
            } catch (err) {
                this.showToast(`Search failed: ${err.message}`, 'error');
            } finally {
                this.loadingState.search = false;
            }
        },
        
        // Entity management methods
        async createEntity() {
            if (!this.newEntity.name || !this.newEntity.type) {
                this.showToast('Entity name and type are required', 'warning');
                return;
            }
            
            this.loadingState.operations = true;
            try {
                const entities = [{
                    name: this.newEntity.name,
                    entityType: this.newEntity.type,
                    observations: this.newEntity.observations.filter(obs => obs.trim())
                }];
                
                await this.memoryRequest('/entities/create', 'POST', { entities });
                
                this.showCreateEntity = false;
                this.resetNewEntity();
                await this.loadEntities();
                this.showToast('Entity created successfully', 'success');
                
            } catch (err) {
                this.showToast(`Failed to create entity: ${err.message}`, 'error');
            } finally {
                this.loadingState.operations = false;
            }
        },
        
        async deleteEntity(entityName) {
            if (!confirm(`Are you sure you want to delete entity "${entityName}"? This will also delete all its relationships.`)) {
                return;
            }
            
            this.loadingState.operations = true;
            try {
                await this.memoryRequest('/entities/delete', 'POST', { 
                    entityNames: [entityName] 
                });
                
                await this.loadEntities();
                this.showToast('Entity deleted successfully', 'success');
                
            } catch (err) {
                this.showToast(`Failed to delete entity: ${err.message}`, 'error');
            } finally {
                this.loadingState.operations = false;
            }
        },
        
        async deleteSelectedEntities() {
            const entityNames = Array.from(this.selectedItems);
            if (!entityNames.length) return;
            
            if (!confirm(`Delete ${entityNames.length} selected entities? This action cannot be undone.`)) {
                return;
            }
            
            this.loadingState.operations = true;
            try {
                await this.memoryRequest('/entities/delete', 'POST', { entityNames });
                
                this.selectedItems.clear();
                await this.loadEntities();
                this.showToast(`${entityNames.length} entities deleted successfully`, 'success');
                
            } catch (err) {
                this.showToast(`Failed to delete entities: ${err.message}`, 'error');
            } finally {
                this.loadingState.operations = false;
            }
        },
        
        // Relationship methods
        async createRelation() {
            if (!this.newRelation.from || !this.newRelation.to || !this.newRelation.type) {
                this.showToast('All relation fields are required', 'warning');
                return;
            }
            
            this.loadingState.operations = true;
            try {
                const relations = [{
                    from: this.newRelation.from,
                    to: this.newRelation.to,
                    relationType: this.newRelation.type
                }];
                
                await this.memoryRequest('/relations/create', 'POST', { relations });
                
                this.showCreateRelation = false;
                this.resetNewRelation();
                await this.loadEntities();
                this.showToast('Relationship created successfully', 'success');
                
            } catch (err) {
                this.showToast(`Failed to create relationship: ${err.message}`, 'error');
            } finally {
                this.loadingState.operations = false;
            }
        },
        
        // Observation methods
        async addObservation(entityName, observation) {
            if (!observation.trim()) return;
            
            try {
                await this.memoryRequest('/observations/add', 'POST', {
                    observations: [{
                        entityName,
                        contents: [observation]
                    }]
                });
                
                await this.loadEntities();
                this.showToast('Observation added successfully', 'success');
                
            } catch (err) {
                this.showToast(`Failed to add observation: ${err.message}`, 'error');
            }
        },
        
        async deleteObservation(entityName, observation) {
            try {
                await this.memoryRequest('/observations/delete', 'POST', {
                    deletions: [{
                        entityName,
                        observations: [observation]
                    }]
                });
                
                await this.loadEntities();
                this.showToast('Observation deleted successfully', 'success');
                
            } catch (err) {
                this.showToast(`Failed to delete observation: ${err.message}`, 'error');
            }
        },
        
        // UI helper methods
        toggleEntityExpansion(entityName) {
            if (this.expandedEntities.has(entityName)) {
                this.expandedEntities.delete(entityName);
            } else {
                this.expandedEntities.add(entityName);
            }
            this.$forceUpdate();
        },
        
        isEntityExpanded(entityName) {
            return this.expandedEntities.has(entityName);
        },
        
        toggleEntitySelection(entityName) {
            if (this.selectedItems.has(entityName)) {
                this.selectedItems.delete(entityName);
            } else {
                this.selectedItems.add(entityName);
            }
            this.$forceUpdate();
        },
        
        toggleSelectAll() {
            if (this.selectAllChecked) {
                this.paginatedEntities.forEach(entity => {
                    this.selectedItems.delete(entity.name);
                });
            } else {
                this.paginatedEntities.forEach(entity => {
                    this.selectedItems.add(entity.name);
                });
            }
            this.$forceUpdate();
        },
        
        goToPage(page) {
            if (page >= 1 && page <= this.totalPages) {
                this.pagination.page = page;
            }
        },
        
        resetNewEntity() {
            this.newEntity = {
                name: '',
                type: '',
                observations: ['']
            };
        },
        
        resetNewRelation() {
            this.newRelation = {
                from: '',
                to: '',
                type: ''
            };
        },
        
        addObservationField() {
            this.newEntity.observations.push('');
        },
        
        removeObservationField(index) {
            if (this.newEntity.observations.length > 1) {
                this.newEntity.observations.splice(index, 1);
            }
        },
        
        // Statistics helpers
        getEntityTypeCount() {
            const counts = {};
            this.entities.forEach(entity => {
                counts[entity.entityType] = (counts[entity.entityType] || 0) + 1;
            });
            return counts;
        },
        
        getRelationTypeCount() {
            const counts = {};
            this.relations.forEach(relation => {
                counts[relation.relationType] = (counts[relation.relationType] || 0) + 1;
            });
            return counts;
        },
        
        getRecentActivity() {
            // Mock recent activity - in real implementation, this would come from the server
            return [
                { type: 'entity_created', entity: 'New Entity', timestamp: new Date().toISOString() },
                { type: 'relation_created', from: 'Entity A', to: 'Entity B', timestamp: new Date().toISOString() }
            ];
        },
        
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Unknown';
            try {
                return new Date(timestamp).toLocaleString();
            } catch {
                return timestamp;
            }
        },
        
        getEntityRelations(entityName) {
            return this.relations.filter(rel => 
                rel.from === entityName || rel.to === entityName
            );
        },
        
        // Auto-refresh
        setupAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
            
            if (this.autoRefresh) {
                this.refreshInterval = setInterval(() => {
                    this.loadEntities();
                }, 30000); // Refresh every 30 seconds
            }
        },
        
        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },
    
    async mounted() {
        await this.loadAllData();
        this.setupAutoRefresh();
    },
    
    beforeUnmount() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    },
    
    watch: {
        autoRefresh() {
            this.setupAutoRefresh();
        },
        
        searchQuery() {
            if (this.activeTab === 'search') {
                this.debounceSearch();
            }
        }
    },
    
    created() {
        this.debounceSearch = window.debounce(this.performSearch, 300);
    },

    template: `
    <div class="memory-viewer space-y-4 animate-fade-in max-w-full overflow-x-hidden">
        <!-- Enhanced Header -->
        <div class="enhanced-card p-4 lg:p-6">
            <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                <div class="flex items-center space-x-3">
                    <div class="flex-shrink-0">
                        <div class="w-10 h-10 bg-gradient-to-r from-purple-500 to-pink-600 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"></path>
                            </svg>
                        </div>
                    </div>
                    <div>
                        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">Memory Server</h1>
                        <p class="text-sm text-gray-500 dark:text-gray-400">PostgreSQL-backed knowledge graph with {{ stats.totalEntities }} entities and {{ stats.totalRelations }} relationships</p>
                    </div>
                </div>
                
                <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                    <button
                        @click="showCreateEntity = true"
                        class="touch-target inline-flex items-center px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 focus:ring-2 focus:ring-purple-500 focus:ring-offset-2 transition-all"
                    >
                        <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                        </svg>
                        Add Entity
                    </button>
                    <button
                        @click="showCreateRelation = true"
                        class="touch-target inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-all"
                    >
                        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                        </svg>
                        Add Relation
                    </button>
                    <button
                        @click="loadAllData"
                        :disabled="loadingState.entities || loadingState.relations"
                        class="touch-target inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 transition-all"
                    >
                        <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loadingState.entities || loadingState.relations }" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                        </svg>
                        Refresh
                    </button>
                    <label class="inline-flex items-center">
                        <input
                            v-model="autoRefresh"
                            type="checkbox"
                            class="form-checkbox h-4 w-4 text-purple-600 rounded focus:ring-purple-500"
                        >
                        <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-refresh</span>
                    </label>
                </div>
            </div>
        </div>

        <!-- Error Display -->
        <div v-if="error" class="enhanced-card border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-4">
            <div class="flex items-start">
                <svg class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                </svg>
                <div class="ml-3 flex-1">
                    <div class="text-sm text-red-800 dark:text-red-200">{{ error }}</div>
                    <button @click="error = null" class="mt-2 text-xs text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline">
                        Dismiss
                    </button>
                </div>
            </div>
        </div>

        <!-- Tab Navigation -->
        <div class="enhanced-card p-4">
            <nav class="flex space-x-1 bg-gray-800 p-1 rounded-lg border border-gray-700">
                <button
                    v-for="tab in tabs"
                    :key="tab.id"
                    @click="activeTab = tab.id"
                    :class="[
                        'px-4 py-2 text-sm font-medium rounded-md transition-colors touch-target',
                        activeTab === tab.id
                            ? 'bg-gray-700 text-white shadow-sm border border-gray-600'
                            : 'text-gray-400 hover:text-gray-200 hover:bg-gray-700'
                    ]"
                    :title="tab.description"
                >
                    <div class="flex items-center space-x-2">
                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="tab.icon"></path>
                        </svg>
                        <span>{{ tab.name }}</span>
                    </div>
                </button>
            </nav>
        </div>

        <!-- Browse Tab -->
        <div v-if="activeTab === 'browse'" class="space-y-4">
            <!-- Filters and Bulk Actions -->
            <div class="enhanced-card p-4">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <!-- Search and Filters -->
                    <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-4 flex-1">
                        <div class="flex-1 relative max-w-md">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                            <input
                                v-model="searchQuery"
                                type="text"
                                placeholder="Search entities..."
                                class="form-input pl-10 w-full"
                            >
                        </div>
                        <select v-model="filterEntityType" class="form-input w-full sm:w-auto">
                            <option value="all">All Types</option>
                            <option v-for="type in uniqueEntityTypes" :key="type" :value="type">{{ type }}</option>
                        </select>
                        <select v-model="sortBy" class="form-input w-full sm:w-auto">
                            <option value="name">Sort by Name</option>
                            <option value="type">Sort by Type</option>
                            <option value="observations">Sort by Observations</option>
                            <option value="updated">Sort by Updated</option>
                        </select>
                    </div>
                    
                    <!-- Bulk Actions -->
                    <div v-if="hasSelection" class="flex items-center space-x-2">
                        <span class="text-sm text-gray-600 dark:text-gray-400">{{ selectedItems.size }} selected</span>
                        <button
                            @click="deleteSelectedEntities"
                            :disabled="loadingState.operations"
                            class="inline-flex items-center px-3 py-1 text-sm bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 transition-all"
                        >
                            <svg class="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M9 2a1 1 0 000 2h2a1 1 0 100-2H9z M4 5a2 2 0 012-2v1a2 2 0 002 2h4a2 2 0 002-2V3a2 2 0 012 2v6.5l1.707 1.707A1 1 0 0017 10.414V5a4 4 0 00-8 0v5.586l1.707-1.707A1 1 0 0012 10.414z" clip-rule="evenodd"></path>
                            </svg>
                            Delete
                        </button>
                        <button
                            @click="selectedItems.clear(); $forceUpdate()"
                            class="text-sm text-gray-600 dark:text-gray-400 hover:text-gray-800 dark:hover:text-gray-200 underline"
                        >
                            Clear selection
                        </button>
                    </div>
                </div>
            </div>

            <!-- Entity List -->
            <div class="enhanced-card">
                <!-- List Header -->
                <div class="flex items-center justify-between p-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center space-x-3">
                        <label class="inline-flex items-center">
                            <input
                                type="checkbox"
                                :checked="selectAllChecked"
                                @change="toggleSelectAll"
                                class="form-checkbox h-4 w-4 text-purple-600 rounded"
                                :disabled="paginatedEntities.length === 0"
                            >
                            <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">
                                Select All ({{ paginatedEntities.length }})
                            </span>
                        </label>
                    </div>
                    <div class="text-sm text-gray-500 dark:text-gray-400">
                        {{ filteredEntities.length }} entities found
                    </div>
                </div>

                <!-- Loading State -->
                <div v-if="loadingState.entities" class="p-8 text-center">
                    <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-purple-500 mx-auto mb-4"></div>
                    <p class="text-lg font-medium text-gray-900 dark:text-white">Loading entities...</p>
                </div>

                <!-- Empty State -->
                <div v-else-if="filteredEntities.length === 0" class="p-8 text-center">
                    <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"></path>
                    </svg>
                    <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-2">No entities found</h2>
                    <p class="text-gray-500 dark:text-gray-400 mb-4">
                        {{ searchQuery || filterEntityType !== 'all' 
                            ? 'Try adjusting your search or filters'
                            : 'Start building your knowledge graph by creating entities' }}
                    </p>
                    <button
                        v-if="!searchQuery && filterEntityType === 'all'"
                        @click="showCreateEntity = true"
                        class="inline-flex items-center px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700"
                    >
                        <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                        </svg>
                        Create Your First Entity
                    </button>
                </div>

                <!-- Entity Cards -->
                <div v-else class="divide-y divide-gray-200 dark:divide-gray-700">
                    <div
                        v-for="entity in paginatedEntities"
                        :key="entity.name"
                        class="p-4 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors"
                    >
                        <div class="flex items-start space-x-4">
                            <!-- Selection Checkbox -->
                            <label class="flex-shrink-0 mt-1">
                                <input
                                    type="checkbox"
                                    :checked="selectedItems.has(entity.name)"
                                    @change="toggleEntitySelection(entity.name)"
                                    class="form-checkbox h-4 w-4 text-purple-600 rounded"
                                >
                            </label>
                            
                            <!-- Entity Content -->
                            <div class="flex-1 min-w-0">
                                <!-- Entity Header -->
                                <div class="flex items-start justify-between mb-2">
                                    <div class="flex-1 min-w-0">
                                        <h3 class="text-lg font-semibold text-gray-900 dark:text-white truncate">
                                            {{ entity.name }}
                                        </h3>
                                        <div class="flex items-center space-x-2 mt-1">
                                            <span class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-200">
                                                {{ entity.entityType }}
                                            </span>
                                            <span class="text-xs text-gray-500 dark:text-gray-400">
                                                {{ entity.observations.length }} observations
                                            </span>
                                            <span v-if="getEntityRelations(entity.name).length > 0" class="text-xs text-gray-500 dark:text-gray-400">
                                                {{ getEntityRelations(entity.name).length }} relationships
                                            </span>
                                        </div>
                                    </div>
                                    
                                    <!-- Entity Actions -->
                                    <div class="flex items-center space-x-2 ml-4">
                                        <button
                                            @click="toggleEntityExpansion(entity.name)"
                                            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded transition-colors"
                                        >
                                            <svg
                                                :class="['w-5 h-5 transition-transform', isEntityExpanded(entity.name) ? 'rotate-180' : '']"
                                                fill="currentColor"
                                                viewBox="0 0 20 20"
                                            >
                                                <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                            </svg>
                                        </button>
                                        <button
                                            @click="deleteEntity(entity.name)"
                                            :disabled="loadingState.operations"
                                            class="text-red-400 hover:text-red-600 p-1 rounded transition-colors disabled:opacity-50"
                                        >
                                            <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M9 2a1 1 0 000 2h2a1 1 0 100-2H9z M4 5a2 2 0 012-2v1a2 2 0 002 2h4a2 2 0 002-2V3a2 2 0 012 2v6.5l1.707 1.707A1 1 0 0017 10.414V5a4 4 0 00-8 0v5.586l1.707-1.707A1 1 0 0012 10.414z" clip-rule="evenodd"></path>
                                            </svg>
                                        </button>
                                    </div>
                                </div>
                                
                                <!-- Expanded Entity Details -->
                                <div v-if="isEntityExpanded(entity.name)" class="space-y-4 mt-4 p-4 bg-gray-50 dark:bg-gray-700/50 rounded-lg">
                                    <!-- Observations -->
                                    <div>
                                        <h4 class="text-sm font-semibold text-gray-900 dark:text-white mb-2 flex items-center">
                                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
                                            </svg>
                                            Observations
                                        </h4>
                                        <div v-if="entity.observations.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
                                            No observations
                                        </div>
                                        <div v-else class="space-y-2">
                                            <div
                                                v-for="(observation, index) in entity.observations"
                                                :key="index"
                                                class="flex items-start justify-between p-2 bg-white dark:bg-gray-800 rounded border"
                                            >
                                                <span class="text-sm text-gray-700 dark:text-gray-300 flex-1">{{ observation }}</span>
                                                <button
                                                    @click="deleteObservation(entity.name, observation)"
                                                    class="text-red-400 hover:text-red-600 ml-2 p-1 rounded transition-colors"
                                                >
                                                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                                    </svg>
                                                </button>
                                            </div>
                                        </div>
                                        
                                        <!-- Add Observation -->
                                        <div class="mt-3">
                                            <div class="flex space-x-2">
                                                <input
                                                    v-model="newObservation"
                                                    type="text"
                                                    placeholder="Add new observation..."
                                                    class="form-input flex-1 text-sm"
                                                    @keyup.enter="if(newObservation.trim()) { addObservation(entity.name, newObservation); newObservation = ''; }"
                                                >
                                                <button
                                                    @click="if(newObservation.trim()) { addObservation(entity.name, newObservation); newObservation = ''; }"
                                                    :disabled="!newObservation.trim()"
                                                    class="inline-flex items-center px-3 py-1 text-sm bg-purple-600 text-white rounded hover:bg-purple-700 disabled:opacity-50 disabled:cursor-not-allowed"
                                                >
                                                    <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                                                    </svg>
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                    
                                    <!-- Relationships -->
                                    <div v-if="getEntityRelations(entity.name).length > 0">
                                        <h4 class="text-sm font-semibold text-gray-900 dark:text-white mb-2 flex items-center">
                                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                                            </svg>
                                            Relationships
                                        </h4>
                                        <div class="space-y-1">
                                            <div
                                                v-for="relation in getEntityRelations(entity.name)"
                                                :key="\`\${relation.from}-\${relation.to}-\${relation.relationType}\`"
                                                class="text-sm p-2 bg-white dark:bg-gray-800 rounded border"
                                            >
                                                <span class="font-medium text-blue-600 dark:text-blue-400">{{ relation.from }}</span>
                                                <span class="text-gray-500 dark:text-gray-400 mx-2">{{ relation.relationType }}</span>
                                                <span class="font-medium text-green-600 dark:text-green-400">{{ relation.to }}</span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Pagination -->
                <div v-if="totalPages > 1" class="flex items-center justify-between px-4 py-3 border-t border-gray-200 dark:border-gray-700">
                    <div class="text-sm text-gray-700 dark:text-gray-300">
                        Showing {{ (pagination.page - 1) * pagination.limit + 1 }} to {{ Math.min(pagination.page * pagination.limit, filteredEntities.length) }} of {{ filteredEntities.length }}
                    </div>
                    <div class="flex items-center space-x-2">
                        <button
                            @click="goToPage(pagination.page - 1)"
                            :disabled="pagination.page <= 1"
                            class="px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Previous
                        </button>
                        <span class="text-sm text-gray-700 dark:text-gray-300">
                            Page {{ pagination.page }} of {{ totalPages }}
                        </span>
                        <button
                            @click="goToPage(pagination.page + 1)"
                            :disabled="pagination.page >= totalPages"
                            class="px-3 py-1 text-sm border border-gray-300 dark:border-gray-600 rounded hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 disabled:cursor-not-allowed"
                        >
                            Next
                        </button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Search Tab -->
        <div v-if="activeTab === 'search'" class="space-y-4">
            <div class="enhanced-card p-6">
                <h2 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Advanced Search</h2>
                
                <div class="space-y-4">
                    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
                        <div class="lg:col-span-2">
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Search Query</label>
                            <input
                                v-model="searchQuery"
                                type="text"
                                placeholder="Search entities, observations, relationships..."
                                class="form-input w-full"
                                @keyup.enter="performSearch"
                            >
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Search Type</label>
                            <select v-model="searchType" class="form-input w-full">
                                <option value="all">All</option>
                                <option value="entities">Entities Only</option>
                                <option value="relations">Relations Only</option>
                                <option value="observations">Observations Only</option>
                            </select>
                        </div>
                    </div>
                    
                    <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Entity Type</label>
                            <select v-model="filterEntityType" class="form-input w-full">
                                <option value="all">All Types</option>
                                <option v-for="type in uniqueEntityTypes" :key="type" :value="type">{{ type }}</option>
                            </select>
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Date From</label>
                            <input
                                v-model="dateRange.start"
                                type="date"
                                class="form-input w-full"
                            >
                        </div>
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Date To</label>
                            <input
                                v-model="dateRange.end"
                                type="date"
                                class="form-input w-full"
                            >
                        </div>
                    </div>
                    
                    <div class="flex justify-end">
                        <button
                            @click="performSearch"
                            :disabled="loadingState.search"
                            class="inline-flex items-center px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50"
                        >
                            <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loadingState.search }" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                            </svg>
                            Search
                        </button>
                    </div>
                </div>
                
                <!-- Search Results -->
                <div v-if="searchResults.length > 0" class="mt-6">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">{{ searchResults.length }} Results Found</h3>
                    <div class="space-y-2">
                        <div
                            v-for="result in searchResults"
                            :key="result.name"
                            class="p-3 border border-gray-200 dark:border-gray-600 rounded-lg hover:border-purple-300 dark:hover:border-purple-500 transition-colors"
                        >
                            <div class="flex items-center justify-between mb-2">
                                <h4 class="font-medium text-gray-900 dark:text-white">{{ result.name }}</h4>
                                <span class="text-xs text-purple-600 dark:text-purple-400">{{ result.entityType }}</span>
                            </div>
                            <p v-if="result.observations.length > 0" class="text-sm text-gray-600 dark:text-gray-400">
                                {{ result.observations[0] }}{{ result.observations.length > 1 ? '...' : '' }}
                            </p>
                        </div>
                    </div>
                </div>
                
                <div v-else-if="searchQuery && !loadingState.search" class="mt-6 text-center text-gray-500 dark:text-gray-400">
                    No results found for "{{ searchQuery }}"
                </div>
            </div>
        </div>

        <!-- Analytics Tab -->
        <div v-if="activeTab === 'analytics'" class="space-y-4">
            <!-- Statistics Cards -->
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-purple-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Total Entities</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.totalEntities }}</p>
                        </div>
                    </div>
                </div>

                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Relationships</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ stats.totalRelations }}</p>
                        </div>
                    </div>
                </div>

                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4z M3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6z M14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Entity Types</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ Object.keys(stats.entityTypes).length }}</p>
                        </div>
                    </div>
                </div>

                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Relation Types</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ Object.keys(stats.relationTypes).length }}</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Entity Types Distribution -->
            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <div class="enhanced-card p-6">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Entity Types</h3>
                    <div v-if="Object.keys(stats.entityTypes).length === 0" class="text-center text-gray-500 dark:text-gray-400 py-8">
                        No entity types to display
                    </div>
                    <div v-else class="space-y-3">
                        <div
                            v-for="(count, type) in stats.entityTypes"
                            :key="type"
                            class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                        >
                            <span class="font-medium text-gray-900 dark:text-white">{{ type }}</span>
                            <div class="flex items-center space-x-2">
                                <div class="w-20 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                    <div
                                        class="bg-purple-500 h-2 rounded-full"
                                        :style="{ width: \`\${(count / stats.totalEntities) * 100}%\` }"
                                    ></div>
                                </div>
                                <span class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ count }}</span>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="enhanced-card p-6">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-white mb-4">Relationship Types</h3>
                    <div v-if="Object.keys(stats.relationTypes).length === 0" class="text-center text-gray-500 dark:text-gray-400 py-8">
                        No relationship types to display
                    </div>
                    <div v-else class="space-y-3">
                        <div
                            v-for="(count, type) in stats.relationTypes"
                            :key="type"
                            class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700 rounded-lg"
                        >
                            <span class="font-medium text-gray-900 dark:text-white">{{ type }}</span>
                            <div class="flex items-center space-x-2">
                                <div class="w-20 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                                    <div
                                        class="bg-blue-500 h-2 rounded-full"
                                        :style="{ width: \`\${(count / stats.totalRelations) * 100}%\` }"
                                    ></div>
                                </div>
                                <span class="text-sm font-medium text-gray-600 dark:text-gray-300">{{ count }}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Create Entity Modal -->
        <div
            v-if="showCreateEntity"
            class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
            @click.self="showCreateEntity = false; resetNewEntity()"
        >
            <div class="bg-gray-900 dark:bg-gray-800 rounded-lg w-full max-w-md max-h-[90vh] overflow-y-auto">
                <div class="p-6">
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Create New Entity</h2>
                        <button
                            @click="showCreateEntity = false; resetNewEntity()"
                            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded"
                        >
                            <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                            </svg>
                        </button>
                    </div>

                    <form @submit.prevent="createEntity" class="space-y-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Name <span class="text-red-500">*</span>
                            </label>
                            <input
                                v-model="newEntity.name"
                                type="text"
                                required
                                class="form-input w-full"
                                placeholder="Enter entity name"
                            >
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Type <span class="text-red-500">*</span>
                            </label>
                            <input
                                v-model="newEntity.type"
                                type="text"
                                required
                                class="form-input w-full"
                                placeholder="Enter entity type (e.g., Person, Event, Concept)"
                            >
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Observations</label>
                            <div class="space-y-2">
                                <div
                                    v-for="(observation, index) in newEntity.observations"
                                    :key="index"
                                    class="flex space-x-2"
                                >
                                    <input
                                        v-model="newEntity.observations[index]"
                                        type="text"
                                        class="form-input flex-1"
                                        placeholder="Enter observation"
                                    >
                                    <button
                                        v-if="newEntity.observations.length > 1"
                                        type="button"
                                        @click="removeObservationField(index)"
                                        class="text-red-400 hover:text-red-600 p-1"
                                    >
                                        <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                        </svg>
                                    </button>
                                </div>
                                <button
                                    type="button"
                                    @click="addObservationField"
                                    class="inline-flex items-center text-sm text-purple-600 dark:text-purple-400 hover:text-purple-800"
                                >
                                    <svg class="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                                    </svg>
                                    Add Observation
                                </button>
                            </div>
                        </div>

                        <div class="flex justify-end space-x-3 pt-4">
                            <button
                                type="button"
                                @click="showCreateEntity = false; resetNewEntity()"
                                class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="!newEntity.name || !newEntity.type || loadingState.operations"
                                class="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 disabled:opacity-50"
                            >
                                Create Entity
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </div>

        <!-- Create Relation Modal -->
        <div
            v-if="showCreateRelation"
            class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
            @click.self="showCreateRelation = false; resetNewRelation()"
        >
            <div class="bg-gray-900 dark:bg-gray-800 rounded-lg w-full max-w-md max-h-[90vh] overflow-y-auto">
                <div class="p-6">
                    <div class="flex items-center justify-between mb-6">
                        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">Create New Relationship</h2>
                        <button
                            @click="showCreateRelation = false; resetNewRelation()"
                            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded"
                        >
                            <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                            </svg>
                        </button>
                    </div>

                    <form @submit.prevent="createRelation" class="space-y-4">
                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                From Entity <span class="text-red-500">*</span>
                            </label>
                            <select v-model="newRelation.from" required class="form-input w-full">
                                <option value="">Select source entity</option>
                                <option v-for="entity in entities" :key="entity.name" :value="entity.name">
                                    {{ entity.name }} ({{ entity.entityType }})
                                </option>
                            </select>
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Relationship Type <span class="text-red-500">*</span>
                            </label>
                            <input
                                v-model="newRelation.type"
                                type="text"
                                required
                                class="form-input w-full"
                                placeholder="e.g., works for, lives in, created by"
                            >
                        </div>

                        <div>
                            <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                To Entity <span class="text-red-500">*</span>
                            </label>
                            <select v-model="newRelation.to" required class="form-input w-full">
                                <option value="">Select target entity</option>
                                <option v-for="entity in entities" :key="entity.name" :value="entity.name">
                                    {{ entity.name }} ({{ entity.entityType }})
                                </option>
                            </select>
                        </div>

                        <div class="flex justify-end space-x-3 pt-4">
                            <button
                                type="button"
                                @click="showCreateRelation = false; resetNewRelation()"
                                class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="!newRelation.from || !newRelation.to || !newRelation.type || loadingState.operations"
                                class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50"
                            >
                                Create Relationship
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </div>
    </div>
    `
};
