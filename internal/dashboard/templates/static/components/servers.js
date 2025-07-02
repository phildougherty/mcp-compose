const ServerManager = {
    props: ['servers', 'connections', 'loading'],
    emits: ['server-action', 'reload', 'view-logs'],
    data() {
        return {
            expandedServers: new Set(),
            searchTerm: '',
            filterStatus: 'all',
            sortBy: 'name',
            viewMode: 'grid', // 'grid' or 'list'
            showAdvancedFilters: false,
            selectedServers: new Set(),
            bulkActionLoading: false
        }
    },
    computed: {
        filteredAndSortedServers() {
            let filtered = this.servers.filter(server => {
                const matchesSearch = this.matchesSearch(server);
                const matchesFilter = this.matchesFilter(server);
                return matchesSearch && matchesFilter;
            });

            return filtered.sort((a, b) => {
                switch (this.sortBy) {
                    case 'status':
                        return this.getServerHealthScore(b) - this.getServerHealthScore(a);
                    case 'name':
                        return a.name.localeCompare(b.name);
                    case 'capabilities':
                        return (b.configCapabilities?.length || 0) - (a.configCapabilities?.length || 0);
                    case 'lastActivity':
                        return this.getLastActivity(b) - this.getLastActivity(a);
                    default:
                        return a.name.localeCompare(b.name);
                }
            });
        },
        statusCounts() {
            return {
                total: this.servers.length,
                running: this.servers.filter(s => this.isContainerRunning(s)).length,
                stopped: this.servers.filter(s => !this.isContainerRunning(s)).length,
                connected: this.servers.filter(s => this.getConnectionStatus(s) === 'Connected').length,
                healthy: this.servers.filter(s => this.isServerHealthy(s)).length,
                error: this.servers.filter(s => this.hasError(s)).length
            };
        },
        groupedCapabilities() {
            const capabilities = new Set();
            this.servers.forEach(server => {
                server.configCapabilities?.forEach(cap => capabilities.add(cap));
            });
            return Array.from(capabilities).sort();
        },
        canPerformBulkActions() {
            return this.selectedServers.size > 0;
        }
    },
    methods: {
        // Enhanced server status methods
        getServerStatus(server) {
            const containerStatus = server.containerStatus?.toLowerCase();
            const connectionStatus = this.getConnectionStatus(server);
            
            if (containerStatus === 'running' && connectionStatus === 'Connected') {
                return { label: 'Healthy', class: 'healthy', icon: 'check-circle' };
            } else if (containerStatus === 'running') {
                return { label: 'Running', class: 'running', icon: 'play-circle' };
            } else if (containerStatus === 'stopped' || containerStatus === 'exited') {
                return { label: 'Stopped', class: 'stopped', icon: 'stop-circle' };
            } else if (containerStatus?.includes('error') || this.hasError(server)) {
                return { label: 'Error', class: 'error', icon: 'x-circle' };
            } else {
                return { label: 'Unknown', class: 'unknown', icon: 'question-mark-circle' };
            }
        },

        getConnectionStatus(server) {
            if (!this.connections?.activeHttpConnectionsManagedByProxy) {
                return 'Disconnected';
            }
            const connection = this.connections.activeHttpConnectionsManagedByProxy[server.name];
            if (connection?.initialized && connection?.rawHealthyFlag) {
                return 'Connected';
            }
            return 'Disconnected';
        },

        getHttpConnection(server) {
            if (!this.connections?.activeHttpConnectionsManagedByProxy) {
                return null;
            }
            return this.connections.activeHttpConnectionsManagedByProxy[server.name] || null;
        },

        // Enhanced filtering and searching
        matchesSearch(server) {
            if (!this.searchTerm) return true;
            const searchLower = this.searchTerm.toLowerCase();
            return (
                server.name.toLowerCase().includes(searchLower) ||
                server.configProtocol?.toLowerCase().includes(searchLower) ||
                server.image?.toLowerCase().includes(searchLower) ||
                server.configCapabilities?.some(cap => cap.toLowerCase().includes(searchLower))
            );
        },

        matchesFilter(server) {
            switch (this.filterStatus) {
                case 'all': return true;
                case 'running': return this.isContainerRunning(server);
                case 'stopped': return !this.isContainerRunning(server);
                case 'connected': return this.getConnectionStatus(server) === 'Connected';
                case 'healthy': return this.isServerHealthy(server);
                case 'error': return this.hasError(server);
                default: return true;
            }
        },

        // Enhanced status checking
        isContainerRunning(server) {
            const status = server.containerStatus?.toLowerCase();
            return status === 'running' || status === 'up' || status?.includes('up ');
        },

        isServerHealthy(server) {
            return this.isContainerRunning(server) && this.getConnectionStatus(server) === 'Connected';
        },

        hasError(server) {
            const status = server.containerStatus?.toLowerCase();
            return status?.includes('error') || status?.includes('failed') || status === 'dead';
        },

        getServerHealthScore(server) {
            if (this.isServerHealthy(server)) return 4;
            if (this.isContainerRunning(server)) return 3;
            if (this.hasError(server)) return 1;
            return 2;
        },

        getLastActivity(server) {
            const connection = this.getHttpConnection(server);
            if (connection?.lastUsedByProxy) {
                return new Date(connection.lastUsedByProxy).getTime();
            }
            return 0;
        },

        // UI state management
        toggleServerDetails(serverName) {
            if (this.expandedServers.has(serverName)) {
                this.expandedServers.delete(serverName);
            } else {
                this.expandedServers.add(serverName);
            }
            this.$forceUpdate();
        },

        isServerExpanded(serverName) {
            return this.expandedServers.has(serverName);
        },

        toggleServerSelection(serverName) {
            if (this.selectedServers.has(serverName)) {
                this.selectedServers.delete(serverName);
            } else {
                this.selectedServers.add(serverName);
            }
            this.$forceUpdate();
        },

        selectAllServers() {
            this.filteredAndSortedServers.forEach(server => {
                this.selectedServers.add(server.name);
            });
            this.$forceUpdate();
        },

        clearSelection() {
            this.selectedServers.clear();
            this.$forceUpdate();
        },

        // Enhanced formatting
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                const date = new Date(timestamp);
                const now = new Date();
                const diffMs = now - date;
                const diffMinutes = Math.floor(diffMs / (1000 * 60));
                const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
                
                if (diffMinutes < 1) return 'Just now';
                if (diffMinutes < 60) return `${diffMinutes}m ago`;
                if (diffHours < 24) return `${diffHours}h ago`;
                
                return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } catch (e) {
                return timestamp;
            }
        },

        formatUptime(uptime) {
            if (!uptime) return 'Unknown';
            // Handle various uptime formats
            if (typeof uptime === 'string') {
                return uptime;
            }
            return 'N/A';
        },

        // Actions
        serverAction(action, serverName) {
            this.$emit('server-action', action, serverName);
        },

        async performBulkAction(action) {
            if (!this.canPerformBulkActions) return;
            
            const servers = Array.from(this.selectedServers);
            const confirmMessage = `${action.charAt(0).toUpperCase() + action.slice(1)} ${servers.length} server(s)?`;
            
            if (!confirm(confirmMessage)) return;
            
            this.bulkActionLoading = true;
            try {
                for (const serverName of servers) {
                    await new Promise(resolve => {
                        this.$emit('server-action', action, serverName);
                        setTimeout(resolve, 500); // Stagger requests
                    });
                }
                this.clearSelection();
            } finally {
                this.bulkActionLoading = false;
            }
        },

        viewServerLogs(serverName) {
            this.$emit('view-logs', serverName);
        },

        // Heroicon helper
        getHeroIcon(iconName) {
            const icons = {
                'check-circle': 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
                'play-circle': 'M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z',
                'stop-circle': 'M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z',
                'x-circle': 'M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z',
                'question-mark-circle': 'M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z',
                'server': 'M5 12a1 1 0 102 0V6.414l1.293 1.293a1 1 0 001.414-1.414l-3-3a1 1 0 00-1.414 0l-3 3a1 1 0 001.414 1.414L5 6.414V12zM15 8a1 1 0 10-2 0v5.586l-1.293-1.293a1 1 0 00-1.414 1.414l3 3a1 1 0 001.414 0l3-3a1 1 0 00-1.414-1.414L15 13.586V8z',
                'refresh': 'M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z',
                'search': 'M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z',
                'filter': 'M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z',
                'view-grid': 'M5 3a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2V5a2 2 0 00-2-2H5zM5 11a2 2 0 00-2 2v2a2 2 0 002 2h2a2 2 0 002-2v-2a2 2 0 00-2-2H5zM11 5a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V5zM11 13a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z',
                'view-list': 'M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z',
                'chevron-down': 'M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z',
                'cog': 'M11.049 2.927c.3-.921 1.603-.921 1.902 0l1.519 4.674a1 1 0 00.95.69h4.915c.969 0 1.371 1.24.588 1.81l-3.976 2.888a1 1 0 00-.363 1.118l1.518 4.674c.3.922-.755 1.688-1.538 1.118l-3.976-2.888a1 1 0 00-1.176 0l-3.976 2.888c-.783.57-1.838-.197-1.538-1.118l1.518-4.674a1 1 0 00-.363-1.118l-3.976-2.888c-.784-.57-.38-1.81.588-1.81h4.914a1 1 0 00.951-.69l1.519-4.674z',
                'document-text': 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z'
            };
            return icons[iconName] || icons['question-mark-circle'];
        }
    },

    template: `
        <div class="space-y-6 animate-fade-in">
            <!-- Enhanced Header with Stats -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col space-y-4">
                    <!-- Title and Description -->
                    <div class="flex items-center justify-between">
                        <div class="flex items-center space-x-3">
                            <div class="w-10 h-10 bg-gradient-to-r from-blue-500 to-indigo-600 rounded-xl flex items-center justify-center">
                                <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('server')"></path>
                                </svg>
                            </div>
                            <div>
                                <h3 class="text-lg font-semibold text-gray-100 dark:text-gray-100">MCP Servers</h3>
                                <p class="text-sm text-gray-300 dark:text-gray-300">Manage and monitor your Model Context Protocol servers</p>
                            </div>
                        </div>
                        
                        <!-- View Toggle -->
                        <div class="flex items-center space-x-2">
                            <div class="flex rounded-lg border border-gray-600">
                                <button
                                    @click="viewMode = 'grid'"
                                    :class="[
                                        'p-2 text-xs font-medium transition-colors',
                                        viewMode === 'grid' 
                                            ? 'bg-blue-600 text-white' 
                                            : 'text-gray-300 hover:text-white hover:bg-gray-700'
                                    ]"
                                >
                                    <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('view-grid')"></path>
                                    </svg>
                                </button>
                                <button
                                    @click="viewMode = 'list'"
                                    :class="[
                                        'p-2 text-xs font-medium transition-colors',
                                        viewMode === 'list' 
                                            ? 'bg-blue-600 text-white' 
                                            : 'text-gray-300 hover:text-white hover:bg-gray-700'
                                    ]"
                                >
                                    <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('view-list')"></path>
                                    </svg>
                                </button>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Stats Row -->
                    <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-4">
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('server')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.total }}</p>
                                    <p class="text-xs text-gray-300">Total</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('check-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.healthy }}</p>
                                    <p class="text-xs text-gray-300">Healthy</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.running }}</p>
                                    <p class="text-xs text-gray-300">Running</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-gray-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('stop-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.stopped }}</p>
                                    <p class="text-xs text-gray-300">Stopped</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-purple-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M12.586 4.586a2 2 0 112.828 2.828l-3 3a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3-3a4 4 0 00-5.656-5.656l-1.5 1.5a1 1 0 101.414 1.414l1.5-1.5zm-5 5a2 2 0 012.828 0 1 1 0 101.414-1.414 4 4 0 00-5.656 0l-3 3a4 4 0 105.656 5.656l1.5-1.5a1 1 0 10-1.414-1.414l-1.5 1.5a2 2 0 11-2.828-2.828l3-3z" clip-rule="evenodd"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.connected }}</p>
                                    <p class="text-xs text-gray-300">Connected</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-red-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.error }}</p>
                                    <p class="text-xs text-gray-300">Errors</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Enhanced Controls -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <!-- Search and Filters -->
                    <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-2xl">
                        <!-- Search -->
                        <div class="relative flex-1">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('search')"></path>
                                </svg>
                            </div>
                            <input 
                                v-model="searchTerm"
                                type="text" 
                                placeholder="Search servers, protocols, capabilities..." 
                                class="form-input pl-10 w-full"
                            >
                        </div>
                        
                        <!-- Filter -->
                        <select 
                            v-model="filterStatus"
                            class="form-input w-full sm:w-auto"
                        >
                            <option value="all">All ({{ statusCounts.total }})</option>
                            <option value="healthy">Healthy ({{ statusCounts.healthy }})</option>
                            <option value="running">Running ({{ statusCounts.running }})</option>
                            <option value="stopped">Stopped ({{ statusCounts.stopped }})</option>
                            <option value="connected">Connected ({{ statusCounts.connected }})</option>
                            <option value="error">Errors ({{ statusCounts.error }})</option>
                        </select>
                        
                        <!-- Sort -->
                        <select 
                            v-model="sortBy"
                            class="form-input w-full sm:w-auto"
                        >
                            <option value="name">Sort by Name</option>
                            <option value="status">Sort by Health</option>
                            <option value="capabilities">Sort by Capabilities</option>
                            <option value="lastActivity">Sort by Activity</option>
                        </select>
                    </div>
                    
                    <!-- Action Buttons -->
                    <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                        <!-- Bulk Actions -->
                        <div v-if="canPerformBulkActions" class="flex space-x-2">
                            <button
                                @click="performBulkAction('start')"
                                :disabled="bulkActionLoading"
                                class="px-3 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 text-xs font-medium transition-colors"
                            >
                                Start Selected
                            </button>
                            <button
                                @click="performBulkAction('stop')"
                                :disabled="bulkActionLoading"
                                class="px-3 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 disabled:opacity-50 text-xs font-medium transition-colors"
                            >
                                Stop Selected
                            </button>
                            <button
                                @click="clearSelection"
                                class="px-3 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 text-xs font-medium transition-colors"
                            >
                                Clear ({{ selectedServers.size }})
                            </button>
                        </div>
                        
                        <!-- Refresh Button -->
                        <button
                            @click="$emit('reload')"
                            :disabled="loading"
                            class="inline-flex items-center px-4 py-2 border border-gray-600 text-sm font-medium rounded-lg text-gray-300 bg-gray-700 hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 transition-colors touch-target"
                        >
                            <svg class="w-4 h-4 mr-2 heroicon" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('refresh')"></path>
                            </svg>
                            Refresh
                        </button>
                    </div>
                </div>
            </div>

            <!-- Loading State -->
            <div v-if="loading && !servers.length" class="flex items-center justify-center py-12">
                <div class="text-center">
                    <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
                    <p class="mt-4 text-sm text-gray-300">Loading servers...</p>
                </div>
            </div>
            
            <!-- No Results -->
            <div v-else-if="filteredAndSortedServers.length === 0" class="enhanced-card p-8 text-center">
                <svg class="mx-auto h-12 w-12 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                </svg>
                <h3 class="mt-4 text-sm font-medium text-gray-100">No servers found</h3>
                <p class="mt-2 text-sm text-gray-300">Try adjusting your search or filter criteria.</p>
                <button
                    @click="searchTerm = ''; filterStatus = 'all'"
                    class="mt-4 inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-sm font-medium transition-colors"
                >
                    Clear Filters
                </button>
            </div>
            
            <!-- Servers Grid/List -->
            <div v-else>
                <!-- Selection Helper -->
                <div v-if="filteredAndSortedServers.length > 1" class="flex items-center justify-between mb-4 text-sm">
                    <div class="flex items-center space-x-4">
                        <label class="inline-flex items-center">
                            <input 
                                type="checkbox" 
                                @change="$event.target.checked ? selectAllServers() : clearSelection()"
                                :checked="selectedServers.size === filteredAndSortedServers.length"
                                :indeterminate="selectedServers.size > 0 && selectedServers.size < filteredAndSortedServers.length"
                                class="form-checkbox h-4 w-4 text-blue-600 rounded"
                            >
                            <span class="ml-2 text-gray-300">
                                {{ selectedServers.size === filteredAndSortedServers.length ? 'Deselect All' : 'Select All' }}
                            </span>
                        </label>
                        <span v-if="selectedServers.size > 0" class="text-gray-400">
                            {{ selectedServers.size }} selected
                        </span>
                    </div>
                </div>
                
                <!-- Grid View -->
                <div v-if="viewMode === 'grid'" class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4 lg:gap-6">
                    <div
                        v-for="server in filteredAndSortedServers"
                        :key="server.name"
                        class="enhanced-card overflow-hidden hover:shadow-lg transition-all duration-200"
                        :class="{ 'ring-2 ring-blue-500': selectedServers.has(server.name) }"
                    >
                        <!-- Server Header -->
                        <div class="p-4 lg:p-6">
                            <div class="flex items-center justify-between mb-4">
                                <div class="flex items-center space-x-3 min-w-0 flex-1">
                                    <!-- Selection Checkbox -->
                                    <input 
                                        type="checkbox" 
                                        :checked="selectedServers.has(server.name)"
                                        @change="toggleServerSelection(server.name)"
                                        class="form-checkbox h-4 w-4 text-blue-600 rounded"
                                    >
                                    <!-- Status Indicator -->
                                    <div class="flex-shrink-0 relative">
                                        <div :class="[
                                            'w-3 h-3 rounded-full',
                                            getServerStatus(server).class === 'healthy' ? 'bg-green-500' :
                                            getServerStatus(server).class === 'running' ? 'bg-blue-500' :
                                            getServerStatus(server).class === 'stopped' ? 'bg-gray-400' :
                                            getServerStatus(server).class === 'error' ? 'bg-red-500' :
                                            'bg-yellow-500'
                                        ]"></div>
                                        <div v-if="getServerStatus(server).class === 'healthy'" class="absolute inset-0 w-3 h-3 bg-green-400 rounded-full animate-ping opacity-75"></div>
                                    </div>
                                    <!-- Server Info -->
                                    <div class="min-w-0 flex-1">
                                        <h4 class="text-lg font-semibold text-gray-100 truncate">
                                            {{ server.name }}
                                        </h4>
                                        <p class="text-sm text-gray-300">
                                            {{ server.configProtocol || 'stdio' }}
                                            <span v-if="server.configHttpPort" class="ml-1">• Port {{ server.configHttpPort }}</span>
                                        </p>
                                    </div>
                                </div>
                                <!-- Expand Button -->
                                <button
                                    @click="toggleServerDetails(server.name)"
                                    class="text-gray-400 hover:text-gray-300 transition-colors p-1 touch-target"
                                >
                                    <svg
                                        :class="['w-5 h-5 transition-transform duration-200 heroicon', isServerExpanded(server.name) ? 'rotate-180' : '']"
                                        fill="none"
                                        stroke="currentColor"
                                        viewBox="0 0 24 24"
                                    >
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                    </svg>
                                </button>
                            </div>

                            <!-- Status Badges -->
                            <div class="flex flex-wrap gap-2 mb-4">
                                <span :class="[
                                    'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                    getServerStatus(server).class === 'healthy' ? 'bg-green-900 text-green-200' :
                                    getServerStatus(server).class === 'running' ? 'bg-blue-900 text-blue-200' :
                                    getServerStatus(server).class === 'stopped' ? 'bg-gray-900 text-gray-200' :
                                    getServerStatus(server).class === 'error' ? 'bg-red-900 text-red-200' :
                                    'bg-yellow-900 text-yellow-200'
                                ]">
                                    <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(getServerStatus(server).icon)"></path>
                                    </svg>
                                    {{ server.containerStatus || 'Unknown' }}
                                </span>
                                <span :class="[
                                    'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border',
                                    getConnectionStatus(server) === 'Connected' 
                                        ? 'bg-green-900 text-green-200 border-green-700'
                                        : 'bg-red-900 text-red-200 border-red-700'
                                ]">
                                    {{ getConnectionStatus(server) }}
                                </span>
                            </div>

                            <!-- Capabilities -->
                            <div v-if="server.configCapabilities?.length" class="mb-4">
                                <span class="text-xs font-medium text-gray-400 uppercase tracking-wide block mb-2">Capabilities</span>
                                <div class="flex flex-wrap gap-1">
                                    <span
                                        v-for="capability in server.configCapabilities?.slice(0, 3)"
                                        :key="capability"
                                        class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-purple-900 text-purple-200"
                                    >
                                        {{ capability }}
                                    </span>
                                    <span v-if="server.configCapabilities?.length > 3" class="text-xs text-gray-400">
                                        +{{ server.configCapabilities.length - 3 }} more
                                    </span>
                                </div>
                            </div>

                            <!-- Action Buttons -->
                            <div class="space-y-2">
                                <div class="grid grid-cols-2 gap-2">
                                    <button
                                        v-if="!isContainerRunning(server)"
                                        @click="serverAction('start', server.name)"
                                        :disabled="loading"
                                        class="flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-green-600 hover:bg-green-700 disabled:bg-gray-500 transition-colors touch-target"
                                    >
                                        <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play-circle')"></path>
                                        </svg>
                                        Start
                                    </button>
                                    
                                    <button
                                        v-if="isContainerRunning(server)"
                                        @click="serverAction('stop', server.name)"
                                        :disabled="loading"
                                        class="flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-red-600 hover:bg-red-700 disabled:bg-gray-500 transition-colors touch-target"
                                    >
                                        <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('stop-circle')"></path>
                                        </svg>
                                        Stop
                                    </button>
                                    
                                    <button
                                        v-if="isContainerRunning(server)"
                                        @click="serverAction('restart', server.name)"
                                        :disabled="loading"
                                        class="flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-yellow-600 hover:bg-yellow-700 disabled:bg-gray-500 transition-colors touch-target"
                                    >
                                        <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('refresh')"></path>
                                        </svg>
                                        Restart
                                    </button>
                                </div>
                                
                                <button
                                    @click="viewServerLogs(server.name)"
                                    class="w-full flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-gray-300 bg-gray-700 hover:bg-gray-600 border border-gray-600 transition-colors touch-target"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('document-text')"></path>
                                    </svg>
                                    View Logs
                                </button>
                            </div>
                        </div>

                        <!-- Expanded Details -->
                        <div v-if="isServerExpanded(server.name)" class="border-t border-gray-700 bg-gray-800 p-4 lg:p-6">
                            <h5 class="font-medium text-gray-100 mb-4 flex items-center">
                                <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('cog')"></path>
                                </svg>
                                Connection Details
                            </h5>
                            
                            <div v-if="getHttpConnection(server)" class="space-y-3">
                                <div class="grid grid-cols-1 gap-3 text-sm">
                                    <div class="bg-gray-700 p-3 rounded-lg">
                                        <span class="font-medium text-gray-300 block mb-1">Target URL</span>
                                        <code class="text-xs bg-gray-900 text-gray-300 px-2 py-1 rounded break-all block">
                                            {{ getHttpConnection(server).targetBaseURL }}
                                        </code>
                                    </div>
                                    
                                    <div class="bg-gray-700 p-3 rounded-lg">
                                        <span class="font-medium text-gray-300 block mb-1">Status</span>
                                        <span class="text-sm text-gray-100">
                                            {{ getHttpConnection(server).status || 'Unknown' }}
                                        </span>
                                    </div>
                                    
                                    <div v-if="getHttpConnection(server).mcpSessionID" class="bg-gray-700 p-3 rounded-lg">
                                        <span class="font-medium text-gray-300 block mb-1">Session ID</span>
                                        <code class="text-xs bg-gray-900 text-gray-300 px-2 py-1 rounded break-all block">
                                            {{ getHttpConnection(server).mcpSessionID }}
                                        </code>
                                    </div>
                                    
                                    <div v-if="getHttpConnection(server).lastUsedByProxy" class="bg-gray-700 p-3 rounded-lg">
                                        <span class="font-medium text-gray-300 block mb-1">Last Used</span>
                                        <span class="text-sm text-gray-100">
                                            {{ formatTimestamp(getHttpConnection(server).lastUsedByProxy) }}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            
                            <div v-else class="text-sm text-gray-400 py-4 text-center">
                                <svg class="w-8 h-8 mx-auto mb-2 text-gray-500 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192L5.636 18.364M12 12l2.828 2.828M12 12l2.828-2.828M12 12L9.172 9.172M12 12l-2.828 2.828"></path>
                                </svg>
                                No active proxy connection
                            </div>
                        </div>
                    </div>
                </div>
                
                <!-- List View -->
                <div v-else class="enhanced-card overflow-hidden">
                    <div class="overflow-x-auto">
                        <table class="min-w-full divide-y divide-gray-700">
                            <thead class="bg-gray-800">
                                <tr>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">
                                        <input 
                                            type="checkbox" 
                                            @change="$event.target.checked ? selectAllServers() : clearSelection()"
                                            :checked="selectedServers.size === filteredAndSortedServers.length"
                                            class="form-checkbox h-4 w-4 text-blue-600 rounded"
                                        >
                                    </th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Server</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Status</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Connection</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Capabilities</th>
                                    <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Actions</th>
                                </tr>
                            </thead>
                            <tbody class="bg-gray-800 divide-y divide-gray-700">
                                <tr v-for="server in filteredAndSortedServers" :key="server.name" 
                                    :class="{ 'bg-gray-700': selectedServers.has(server.name) }"
                                    class="hover:bg-gray-700 transition-colors">
                                    <td class="px-6 py-4 whitespace-nowrap">
                                        <input 
                                            type="checkbox" 
                                            :checked="selectedServers.has(server.name)"
                                            @change="toggleServerSelection(server.name)"
                                            class="form-checkbox h-4 w-4 text-blue-600 rounded"
                                        >
                                    </td>
                                    <td class="px-6 py-4 whitespace-nowrap">
                                        <div class="flex items-center">
                                            <div :class="[
                                                'w-2 h-2 rounded-full mr-3',
                                                getServerStatus(server).class === 'healthy' ? 'bg-green-500' :
                                                getServerStatus(server).class === 'running' ? 'bg-blue-500' :
                                                getServerStatus(server).class === 'stopped' ? 'bg-gray-400' :
                                                getServerStatus(server).class === 'error' ? 'bg-red-500' :
                                                'bg-yellow-500'
                                            ]"></div>
                                            <div>
                                                <div class="text-sm font-medium text-gray-100">{{ server.name }}</div>
                                                <div class="text-sm text-gray-400">{{ server.configProtocol || 'stdio' }}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td class="px-6 py-4 whitespace-nowrap">
                                        <span :class="[
                                            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                            getServerStatus(server).class === 'healthy' ? 'bg-green-900 text-green-200' :
                                            getServerStatus(server).class === 'running' ? 'bg-blue-900 text-blue-200' :
                                            getServerStatus(server).class === 'stopped' ? 'bg-gray-900 text-gray-200' :
                                            getServerStatus(server).class === 'error' ? 'bg-red-900 text-red-200' :
                                            'bg-yellow-900 text-yellow-200'
                                        ]">
                                            {{ server.containerStatus || 'Unknown' }}
                                        </span>
                                    </td>
                                    <td class="px-6 py-4 whitespace-nowrap">
                                        <span :class="[
                                            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                            getConnectionStatus(server) === 'Connected' 
                                                ? 'bg-green-900 text-green-200'
                                                : 'bg-red-900 text-red-200'
                                        ]">
                                            {{ getConnectionStatus(server) }}
                                        </span>
                                    </td>
                                    <td class="px-6 py-4 whitespace-nowrap">
                                        <div class="flex flex-wrap gap-1">
                                            <span
                                                v-for="capability in server.configCapabilities?.slice(0, 2)"
                                                :key="capability"
                                                class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-purple-900 text-purple-200"
                                            >
                                                {{ capability }}
                                            </span>
                                            <span v-if="server.configCapabilities?.length > 2" class="text-xs text-gray-400">
                                                +{{ server.configCapabilities.length - 2 }}
                                            </span>
                                        </div>
                                    </td>
                                    <td class="px-6 py-4 whitespace-nowrap text-sm font-medium space-x-2">
                                        <button
                                            v-if="!isContainerRunning(server)"
                                            @click="serverAction('start', server.name)"
                                            :disabled="loading"
                                            class="text-green-400 hover:text-green-300 disabled:text-gray-500"
                                        >
                                            Start
                                        </button>
                                        <button
                                            v-if="isContainerRunning(server)"
                                            @click="serverAction('stop', server.name)"
                                            :disabled="loading"
                                            class="text-red-400 hover:text-red-300 disabled:text-gray-500"
                                        >
                                            Stop
                                        </button>
                                        <button
                                            @click="viewServerLogs(server.name)"
                                            class="text-blue-400 hover:text-blue-300"
                                        >
                                            Logs
                                        </button>
                                    </td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </div>
            </div>
        </div>
    `
};