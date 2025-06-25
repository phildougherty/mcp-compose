const ServerManager = {
    props: ['servers', 'connections', 'loading'],
    emits: ['server-action', 'reload'],
    data() {
        return {
            expandedServers: [],
            searchTerm: '',
            filterStatus: 'all'
        }
    },
    computed: {
        filteredServers() {
            return this.servers.filter(server => {
                const matchesSearch = server.name.toLowerCase().includes(this.searchTerm.toLowerCase());
                const matchesFilter = this.filterStatus === 'all' || 
                    (this.filterStatus === 'running' && this.isContainerRunning(server)) ||
                    (this.filterStatus === 'stopped' && !this.isContainerRunning(server)) ||
                    (this.filterStatus === 'connected' && this.getConnectionStatus(server) === 'Connected');
                return matchesSearch && matchesFilter;
            });
        },
        statusCounts() {
            return {
                running: this.servers.filter(s => this.isContainerRunning(s)).length,
                stopped: this.servers.filter(s => !this.isContainerRunning(s)).length,
                connected: this.servers.filter(s => this.getConnectionStatus(s) === 'Connected').length
            };
        }
    },
    methods: {
        getServerStatus(server) {
            const status = server.containerStatus?.toLowerCase();
            if (status === 'running') return 'Running';
            if (status === 'stopped' || status === 'exited') return 'Stopped';
            return 'Unknown';
        },
        getConnectionStatus(server) {
            const connections = this.connections.activeHttpConnectionsManagedByProxy;
            if (!connections) return 'Disconnected';
            const connection = connections[server.name];
            if (connection && connection.initialized) return 'Connected';
            return 'Disconnected';
        },
        getHttpConnection(server) {
            if (!this.connections || !this.connections.activeHttpConnectionsManagedByProxy) {
                return null;
            }
            return this.connections.activeHttpConnectionsManagedByProxy[server.name] || null;
        },
        toggleServerDetails(serverName) {
            const index = this.expandedServers.indexOf(serverName);
            if (index > -1) {
                this.expandedServers.splice(index, 1);
            } else {
                this.expandedServers.push(serverName);
            }
        },
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                return new Date(timestamp).toLocaleString();
            } catch (e) {
                return timestamp;
            }
        },
        isContainerRunning(server) {
            return server.containerStatus?.toLowerCase() === 'running';
        },
        serverAction(action, serverName) {
            this.$emit('server-action', action, serverName);
        }
    },
    template: `
        <div class="space-y-6">
            <!-- Header with Search and Filters -->
            <div class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4 lg:p-6">
                <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-4 sm:space-y-0">
                    <div>
                        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">MCP Servers</h3>
                        <p class="text-sm text-gray-500 dark:text-gray-400 mt-1">Manage and monitor your Model Context Protocol servers</p>
                    </div>
                    
                    <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3">
                        <!-- Search -->
                        <div class="relative">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                            <input 
                                v-model="searchTerm"
                                type="text" 
                                placeholder="Search servers..." 
                                class="block w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-sm"
                            >
                        </div>
                        
                        <!-- Filter -->
                        <select 
                            v-model="filterStatus"
                            class="block w-full sm:w-auto px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500 text-sm"
                        >
                            <option value="all">All Servers ({{ servers.length }})</option>
                            <option value="running">Running ({{ statusCounts.running }})</option>
                            <option value="stopped">Stopped ({{ statusCounts.stopped }})</option>
                            <option value="connected">Connected ({{ statusCounts.connected }})</option>
                        </select>
                        
                        <!-- Refresh Button -->
                        <button
                            @click="$emit('reload')"
                            :disabled="loading"
                            class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50 transition-colors"
                        >
                            <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
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
                    <p class="mt-4 text-sm text-gray-600 dark:text-gray-400">Loading servers...</p>
                </div>
            </div>

            <!-- No Results -->
            <div v-else-if="filteredServers.length === 0" class="text-center py-12">
                <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 12h6m-6 4h6M5 16a3 3 0 01-3-3V9a3 3 0 013-3h14a3 3 0 013 3v4a3 3 0 01-3 3H5z"></path>
                </svg>
                <h3 class="mt-4 text-sm font-medium text-gray-900 dark:text-white">No servers found</h3>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Try adjusting your search or filter criteria.</p>
            </div>

            <!-- Server Grid -->
            <div v-else class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4 lg:gap-6">
                <div
                    v-for="server in filteredServers"
                    :key="server.name"
                    class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden hover:shadow-md transition-shadow card-hover"
                >
                    <!-- Server Header -->
                    <div class="p-4 lg:p-6 border-b border-gray-200 dark:border-gray-700">
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-3 min-w-0 flex-1">
                                <div
                                    :class="[
                                        'w-3 h-3 rounded-full flex-shrink-0',
                                        getServerStatus(server) === 'Running' ? 'bg-green-500' :
                                        getServerStatus(server) === 'Stopped' ? 'bg-red-500' : 'bg-yellow-500'
                                    ]"
                                ></div>
                                <div class="min-w-0 flex-1">
                                    <h4 class="text-lg font-semibold text-gray-900 dark:text-white truncate">
                                        {{ server.name }}
                                    </h4>
                                    <p class="text-sm text-gray-500 dark:text-gray-400">
                                        {{ server.configProtocol || 'stdio' }}
                                        <span v-if="server.configHttpPort" class="ml-1">• Port {{ server.configHttpPort }}</span>
                                    </p>
                                </div>
                            </div>
                            <button
                                @click="toggleServerDetails(server.name)"
                                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors p-1"
                            >
                                <svg
                                    :class="['w-5 h-5 transition-transform duration-200', expandedServers.includes(server.name) ? 'rotate-180' : '']"
                                    fill="currentColor"
                                    viewBox="0 0 20 20"
                                >
                                    <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                </svg>
                            </button>
                        </div>
                    </div>

                    <!-- Server Status Info -->
                    <div class="p-4 lg:p-6 space-y-4">
                        <div class="grid grid-cols-2 gap-4">
                            <div>
                                <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Container</span>
                                <div class="mt-1">
                                    <span :class="[
                                        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                        getServerStatus(server) === 'Running' 
                                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200' :
                                        getServerStatus(server) === 'Stopped' 
                                            ? 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200' :
                                            'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200'
                                    ]">
                                        {{ server.containerStatus || 'Unknown' }}
                                    </span>
                                </div>
                            </div>
                            
                            <div>
                                <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Proxy</span>
                                <div class="mt-1">
                                    <span :class="[
                                        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                        getConnectionStatus(server) === 'Connected' 
                                            ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                            : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                    ]">
                                        {{ getConnectionStatus(server) }}
                                    </span>
                                </div>
                            </div>
                        </div>

                        <!-- Capabilities -->
                        <div v-if="server.configCapabilities && server.configCapabilities.length">
                            <span class="text-xs font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Capabilities</span>
                            <div class="flex flex-wrap gap-1 mt-2">
                                <span
                                    v-for="capability in server.configCapabilities"
                                    :key="capability"
                                    class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                                >
                                    {{ capability }}
                                </span>
                            </div>
                        </div>

                        <!-- Action Buttons -->
                        <div class="flex space-x-2 pt-2">
                            <button
                                v-if="!isContainerRunning(server)"
                                @click="serverAction('start', server.name)"
                                :disabled="loading"
                                class="flex-1 bg-green-600 hover:bg-green-700 disabled:bg-gray-400 text-white text-sm font-medium py-2 px-3 rounded-lg transition-colors btn-hover focus:outline-none focus:ring-2 focus:ring-green-500"
                            >
                                <span class="flex items-center justify-center">
                                    <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                                    </svg>
                                    Start
                                </span>
                            </button>
                            
                            <button
                                v-if="isContainerRunning(server)"
                                @click="serverAction('stop', server.name)"
                                :disabled="loading"
                                class="flex-1 bg-red-600 hover:bg-red-700 disabled:bg-gray-400 text-white text-sm font-medium py-2 px-3 rounded-lg transition-colors btn-hover focus:outline-none focus:ring-2 focus:ring-red-500"
                            >
                                <span class="flex items-center justify-center">
                                    <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                                    </svg>
                                    Stop
                                </span>
                            </button>
                            
                            <button
                                v-if="isContainerRunning(server)"
                                @click="serverAction('restart', server.name)"
                                :disabled="loading"
                                class="flex-1 bg-yellow-600 hover:bg-yellow-700 disabled:bg-gray-400 text-white text-sm font-medium py-2 px-3 rounded-lg transition-colors btn-hover focus:outline-none focus:ring-2 focus:ring-yellow-500"
                            >
                                <span class="flex items-center justify-center">
                                    <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                    </svg>
                                    Restart
                                </span>
                            </button>
                        </div>
                    </div>

                    <!-- Expanded Details -->
                    <div v-if="expandedServers.includes(server.name)" class="border-t border-gray-200 dark:border-gray-700 slide-down">
                        <div class="p-4 lg:p-6 bg-gray-50 dark:bg-gray-700/30">
                            <h5 class="font-medium text-gray-900 dark:text-white mb-4 flex items-center">
                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                </svg>
                                Connection Details
                            </h5>
                            
                            <div v-if="getHttpConnection(server)" class="space-y-3">
                                <div class="grid grid-cols-1 gap-3 text-sm">
                                    <div class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                        <span class="font-medium text-gray-500 dark:text-gray-400 block mb-1">Target URL</span>
                                        <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded break-all">
                                            {{ getHttpConnection(server).targetBaseURL }}
                                        </code>
                                    </div>
                                    
                                    <div class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                        <span class="font-medium text-gray-500 dark:text-gray-400 block mb-1">Status</span>
                                        <span class="text-sm text-gray-900 dark:text-white">
                                            {{ getHttpConnection(server).status }}
                                        </span>
                                    </div>
                                    
                                    <div v-if="getHttpConnection(server).mcpSessionID" class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                        <span class="font-medium text-gray-500 dark:text-gray-400 block mb-1">Session ID</span>
                                        <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded break-all">
                                            {{ getHttpConnection(server).mcpSessionID }}
                                        </code>
                                    </div>
                                    
                                    <div v-if="getHttpConnection(server).lastUsedByProxy" class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                        <span class="font-medium text-gray-500 dark:text-gray-400 block mb-1">Last Used</span>
                                        <span class="text-sm text-gray-900 dark:text-white">
                                            {{ formatTimestamp(getHttpConnection(server).lastUsedByProxy) }}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            
                            <div v-else class="text-sm text-gray-500 dark:text-gray-400 py-4 text-center">
                                <svg class="w-8 h-8 mx-auto mb-2 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192L5.636 18.364M12 12l2.828 2.828M12 12l2.828-2.828M12 12L9.172 9.172M12 12l-2.828 2.828"></path>
                                </svg>
                                No active proxy connection
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};