const ServerOAuthConfig = {
    emits: ['show-toast'],
    data() {
        return {
            servers: {},
            scopes: [],
            clients: [],
            serverOAuthConfigs: {},
            loading: false,
            saving: false,
            error: null,
            // Enhanced filtering and sorting
            serverSearchTerm: '',
            serverFilter: 'all',
            sortBy: 'name',
            expandedServers: new Set()
        }
    },
    computed: {
        filteredAndSortedServers() {
            const serverEntries = Object.entries(this.servers);
            let filtered = serverEntries.filter(([name, server]) => {
                const matchesSearch = !this.serverSearchTerm || 
                    name.toLowerCase().includes(this.serverSearchTerm.toLowerCase()) ||
                    server.configProtocol?.toLowerCase().includes(this.serverSearchTerm.toLowerCase());
                
                const matchesFilter = this.serverFilter === 'all' ||
                    (this.serverFilter === 'running' && server.containerStatus === 'running') ||
                    (this.serverFilter === 'stopped' && server.containerStatus !== 'running') ||
                    (this.serverFilter === 'oauth-enabled' && this.getOAuthConfig(name).enabled) ||
                    (this.serverFilter === 'oauth-disabled' && !this.getOAuthConfig(name).enabled);
                
                return matchesSearch && matchesFilter;
            });

            return filtered.sort(([nameA, serverA], [nameB, serverB]) => {
                switch (this.sortBy) {
                    case 'status':
                        return serverA.containerStatus?.localeCompare(serverB.containerStatus || '') || 0;
                    case 'oauth':
                        const aEnabled = this.getOAuthConfig(nameA).enabled;
                        const bEnabled = this.getOAuthConfig(nameB).enabled;
                        return Number(bEnabled) - Number(aEnabled);
                    default:
                        return nameA.localeCompare(nameB);
                }
            });
        },
        serverStats() {
            const totalServers = Object.keys(this.servers).length;
            const runningServers = Object.values(this.servers).filter(s => s.containerStatus === 'running').length;
            const oauthEnabledServers = Object.keys(this.servers).filter(name => this.getOAuthConfig(name).enabled).length;
            
            return {
                total: totalServers,
                running: runningServers,
                stopped: totalServers - runningServers,
                oauthEnabled: oauthEnabledServers,
                oauthDisabled: totalServers - oauthEnabledServers
            };
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        // Heroicon helper
        getHeroIcon(iconName) {
            const icons = {
                'server': 'M5 12a1 1 0 102 0V6.414l1.293 1.293a1 1 0 001.414-1.414l-3-3a1 1 0 00-1.414 0l-3 3a1 1 0 001.414 1.414L5 6.414V12zM15 8a1 1 0 10-2 0v5.586l-1.293-1.293a1 1 0 00-1.414 1.414l3 3a1 1 0 001.414 0l3-3a1 1 0 00-1.414-1.414L15 13.586V8z',
                'shield-check': 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
                'cog': 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z',
                'search': 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
                'filter': 'M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.207A1 1 0 013 6.5V4z',
                'refresh': 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15',
                'check-circle': 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
                'x-circle': 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
                'play': 'M14.828 14.828a4 4 0 01-5.656 0M9 10h1.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293H15a2 2 0 002-2V9a2 2 0 00-2-2h-1.586a1 1 0 01-.707-.293L10.293 4.293A1 1 0 009.586 4H8a2 2 0 00-2 2v5a2 2 0 002 2z',
                'document-text': 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
                'save': 'M8 7H5a2 2 0 00-2 2v9a2 2 0 002 2h8a2 2 0 002-2V9a2 2 0 00-2-2h-3m-1 4l-3 3m0 0l3 3m-3-3h12',
                'key': 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
                'users': 'M12 4.354a4 4 0 110 5.292M12 4.354a4 4 0 000 5.292M12 4.354v5.292M16 14a4 4 0 11-8 0 4 4 0 018 0zm-8 0a4 4 0 110-8 4 4 0 010 8z',
                'chevron-down': 'M19 9l-7 7-7-7'
            };
            return icons[iconName] || icons['cog'];
        },

        async loadData() {
            this.loading = true;
            this.error = null;
            try {
                const [serversRes, scopesRes, clientsRes] = await Promise.all([
                    fetch('/api/servers'),
                    fetch('/api/oauth/scopes').catch(() => ({ ok: false })),
                    fetch('/api/oauth/clients').catch(() => ({ ok: false }))
                ]);
                
                if (serversRes.ok) {
                    this.servers = await serversRes.json();
                }
                
                if (scopesRes.ok && scopesRes.headers?.get('content-type')?.includes('application/json')) {
                    this.scopes = await scopesRes.json();
                } else {
                    this.scopes = [
                        { name: 'mcp:tools', description: 'Access to MCP tools' },
                        { name: 'mcp:resources', description: 'Access to MCP resources' },
                        { name: 'mcp:prompts', description: 'Access to MCP prompts' },
                        { name: 'admin', description: 'Administrative access' }
                    ];
                }
                
                if (clientsRes.ok && clientsRes.headers?.get('content-type')?.includes('application/json')) {
                    this.clients = await clientsRes.json();
                } else {
                    this.clients = [];
                }
                
                // Load OAuth config for each server
                for (const serverName of Object.keys(this.servers)) {
                    try {
                        const configRes = await fetch(`/api/servers/${serverName}/oauth`);
                        if (configRes.ok && configRes.headers.get('content-type')?.includes('application/json')) {
                            const config = await configRes.json();
                            this.serverOAuthConfigs[serverName] = config;
                        }
                    } catch (error) {
                        console.error(`Failed to load OAuth config for ${serverName}:`, error);
                    }
                }
            } catch (error) {
                this.error = `Failed to load server OAuth data: ${error.message}`;
                console.error('Failed to load server OAuth data:', error);
                this.showToast('Some OAuth endpoints not available', 'warning');
            } finally {
                this.loading = false;
            }
        },

        // UI state management
        toggleServerExpansion(serverName) {
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

        // OAuth configuration methods
        getOAuthConfig(serverName) {
            return this.serverOAuthConfigs[serverName] || {
                enabled: false,
                required_scope: '',
                allow_api_key_fallback: true,
                optional_auth: false,
                allowed_clients: []
            };
        },

        async updateOAuthEnabled(serverName, enabled) {
            await this.updateServerOAuth(serverName, { enabled });
        },

        async updateRequiredScope(serverName, scope) {
            await this.updateServerOAuth(serverName, { required_scope: scope });
        },

        async updateAPIKeyFallback(serverName, allowFallback) {
            await this.updateServerOAuth(serverName, { allow_api_key_fallback: allowFallback });
        },

        async updateOptionalAuth(serverName, optional) {
            await this.updateServerOAuth(serverName, { optional_auth: optional });
        },

        async updateAllowedClients(serverName, clientId, allowed) {
            const config = this.getOAuthConfig(serverName);
            const allowedClients = [...(config.allowed_clients || [])];
            
            if (allowed && !allowedClients.includes(clientId)) {
                allowedClients.push(clientId);
            } else if (!allowed) {
                const index = allowedClients.indexOf(clientId);
                if (index > -1) allowedClients.splice(index, 1);
            }
            
            await this.updateServerOAuth(serverName, { allowed_clients: allowedClients });
        },

        isClientAllowed(serverName, clientId) {
            const config = this.getOAuthConfig(serverName);
            return (config.allowed_clients || []).includes(clientId);
        },

        async updateServerOAuth(serverName, updates) {
            try {
                const currentConfig = this.getOAuthConfig(serverName);
                const newConfig = { ...currentConfig, ...updates };
                
                const response = await fetch(`/api/servers/${serverName}/oauth`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(newConfig)
                });
                
                if (response.ok) {
                    this.serverOAuthConfigs[serverName] = newConfig;
                    this.$emit('show-toast', {
                        message: `Updated OAuth settings for ${serverName}`,
                        type: 'success'
                    });
                } else {
                    throw new Error('Failed to update OAuth settings');
                }
            } catch (error) {
                this.$emit('show-toast', {
                    message: `Failed to update OAuth for ${serverName}: ${error.message}`,
                    type: 'error'
                });
            }
        },

        async saveServerConfig(serverName) {
            this.saving = true;
            try {
                const config = this.getOAuthConfig(serverName);
                const response = await fetch(`/api/servers/${serverName}/oauth`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });
                
                if (response.ok) {
                    this.$emit('show-toast', {
                        message: `Saved OAuth configuration for ${serverName}`,
                        type: 'success'
                    });
                } else {
                    throw new Error('Failed to save configuration');
                }
            } catch (error) {
                this.$emit('show-toast', {
                    message: `Failed to save configuration for ${serverName}: ${error.message}`,
                    type: 'error'
                });
            } finally {
                this.saving = false;
            }
        },
        showToast(message, type = 'info') {
            if (window.showToast) {
                window.showToast(message, type);
            } else {
                // Fallback for when global toast isn't available
                console.log(`[${type.toUpperCase()}] ${message}`);
            }
        },
        // Testing and utility methods
        async testServerAccess(serverName) {
            try {
                const response = await fetch(`/api/servers/${serverName}/test-oauth`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' }
                });
                
                if (response.ok && response.headers.get('content-type')?.includes('application/json')) {
                    const result = await response.json();
                    
                    if (result.success) {
                        this.$emit('show-toast', {
                            message: `✅ OAuth test successful for ${serverName}`,
                            type: 'success'
                        });
                    } else {
                        this.$emit('show-toast', {
                            message: `❌ OAuth test failed for ${serverName}: ${result.error || 'Unknown error'}`,
                            type: 'error'
                        });
                    }
                } else {
                    throw new Error(`HTTP ${response.status}: Expected JSON but got ${response.headers.get('content-type')}`);
                }
            } catch (error) {
                this.$emit('show-toast', {
                    message: `❌ Failed to test OAuth for ${serverName}: ${error.message}`,
                    type: 'error'
                });
            }
        },

        async viewServerTokens(serverName) {
            try {
                const response = await fetch(`/api/servers/${serverName}/tokens`);
                if (response.ok) {
                    const tokens = await response.json();
                    alert(`Active tokens for ${serverName}:\n${JSON.stringify(tokens, null, 2)}`);
                } else {
                    throw new Error('Tokens endpoint not available');
                }
            } catch (error) {
                this.$emit('show-toast', {
                    message: `Failed to retrieve tokens for ${serverName}: ${error.message}`,
                    type: 'error'
                });
            }
        },

        getServerStatusClass(server) {
            if (server.containerStatus === 'running') {
                return 'bg-green-500';
            } else {
                return 'bg-gray-400';
            }
        }
    },

    template: `
        <div class="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
            <!-- Enhanced Header -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col space-y-4">
                    <!-- Title and Description -->
                    <div class="flex items-center space-x-3">
                        <div class="w-10 h-10 bg-gradient-to-r from-purple-500 to-pink-600 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('server')"></path>
                            </svg>
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-gray-100">Server OAuth Settings</h3>
                            <p class="text-sm text-gray-300">Configure OAuth authentication requirements for each MCP server</p>
                        </div>
                    </div>
                    
                    <!-- Stats Grid -->
                    <div class="grid grid-cols-2 lg:grid-cols-5 gap-4">
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('server')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ serverStats.total }}</p>
                                    <p class="text-xs text-gray-300">Total</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('check-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ serverStats.running }}</p>
                                    <p class="text-xs text-gray-300">Running</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-gray-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ serverStats.stopped }}</p>
                                    <p class="text-xs text-gray-300">Stopped</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('shield-check')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ serverStats.oauthEnabled }}</p>
                                    <p class="text-xs text-gray-300">OAuth Enabled</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-red-500 rounded-lg flex items-center justify-center">
                                    <div class="w-2 h-2 bg-white rounded-full"></div>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ serverStats.oauthDisabled }}</p>
                                    <p class="text-xs text-gray-300">OAuth Disabled</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Error Display -->
            <div v-if="error" class="enhanced-card border-red-500 bg-red-900/20 p-4">
                <div class="flex items-start">
                    <svg class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                    </svg>
                    <div class="ml-3 flex-1">
                        <div class="text-sm text-red-200">{{ error }}</div>
                        <button @click="error = null" class="mt-2 text-xs text-red-400 hover:text-red-300 underline touch-target">
                            Dismiss
                        </button>
                    </div>
                </div>
            </div>

            <!-- Controls -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-2xl">
                        <!-- Search -->
                        <div class="relative flex-1">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('search')"></path>
                                </svg>
                            </div>
                            <input 
                                v-model="serverSearchTerm"
                                type="text" 
                                placeholder="Search servers..." 
                                class="form-input pl-10 w-full"
                            >
                        </div>
                        
                        <!-- Filter -->
                        <select v-model="serverFilter" class="form-input w-full sm:w-auto">
                            <option value="all">All Servers ({{ serverStats.total }})</option>
                            <option value="running">Running ({{ serverStats.running }})</option>
                            <option value="stopped">Stopped ({{ serverStats.stopped }})</option>
                            <option value="oauth-enabled">OAuth Enabled ({{ serverStats.oauthEnabled }})</option>
                            <option value="oauth-disabled">OAuth Disabled ({{ serverStats.oauthDisabled }})</option>
                        </select>
                        
                        <!-- Sort -->
                        <select v-model="sortBy" class="form-input w-full sm:w-auto">
                            <option value="name">Sort by Name</option>
                            <option value="status">Sort by Status</option>
                            <option value="oauth">Sort by OAuth</option>
                        </select>
                    </div>
                    
                    <!-- Refresh Button -->
                    <button
                        @click="loadData"
                        :disabled="loading"
                        class="inline-flex items-center px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors font-medium text-sm touch-target disabled:opacity-50"
                    >
                        <svg class="w-4 h-4 mr-2 heroicon" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('refresh')"></path>
                        </svg>
                        Refresh
                    </button>
                </div>
            </div>

            <!-- Loading State -->
            <div v-if="loading && Object.keys(servers).length === 0" class="enhanced-card p-8 text-center">
                <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
                <p class="text-lg font-medium text-gray-100">Loading server configurations...</p>
                <p class="text-sm text-gray-300">Fetching OAuth settings for each server</p>
            </div>

            <!-- No Servers State -->
            <div v-else-if="filteredAndSortedServers.length === 0 && !loading" class="enhanced-card p-8 text-center">
                <svg class="mx-auto h-12 w-12 text-gray-500 mb-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('server')"></path>
                </svg>
                <h3 class="text-lg font-medium text-gray-100 mb-2">No servers found</h3>
                <p class="text-gray-300 mb-4">
                    {{ serverSearchTerm || serverFilter !== 'all'
                        ? 'Try adjusting your search or filters'
                        : 'No MCP servers are currently configured' }}
                </p>
                <button
                    v-if="serverSearchTerm || serverFilter !== 'all'"
                    @click="serverSearchTerm = ''; serverFilter = 'all'"
                    class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-medium"
                >
                    Clear Filters
                </button>
            </div>

            <!-- Server OAuth Configuration -->
            <div v-else class="space-y-4">
                <div 
                    v-for="[serverName, server] in filteredAndSortedServers" 
                    :key="serverName"
                    class="enhanced-card overflow-hidden"
                >
                    <!-- Server Header -->
                    <div
                        @click="toggleServerExpansion(serverName)"
                        class="p-4 lg:p-6 cursor-pointer hover:bg-gray-700/30 transition-colors"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-3 min-w-0 flex-1">
                                <!-- Status Indicator -->
                                <div class="flex-shrink-0 relative">
                                    <div :class="['w-3 h-3 rounded-full border border-current', getServerStatusClass(server)]"></div>
                                </div>
                                <!-- Server Info -->
                                <div class="min-w-0 flex-1">
                                    <h4 class="text-lg font-medium text-gray-100 truncate">{{ serverName }}</h4>
                                    <div class="flex items-center space-x-3 text-sm text-gray-400">
                                        <span>{{ server.configProtocol || 'stdio' }}</span>
                                        <span v-if="server.configHttpPort">Port {{ server.configHttpPort }}</span>
                                    </div>
                                </div>
                                <!-- OAuth Status Badge -->
                                <div class="flex items-center space-x-2">
                                    <span :class="[
                                        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                        getOAuthConfig(serverName).enabled 
                                            ? 'bg-green-900 text-green-200 border border-green-700' 
                                            : 'bg-gray-900 text-gray-300 border border-gray-600'
                                    ]">
                                        <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(getOAuthConfig(serverName).enabled ? 'shield-check' : 'x-circle')"></path>
                                        </svg>
                                        OAuth {{ getOAuthConfig(serverName).enabled ? 'Enabled' : 'Disabled' }}
                                    </span>
                                    <span :class="[
                                        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                        server.containerStatus === 'running' 
                                            ? 'bg-green-900 text-green-200 border border-green-700' 
                                            : 'bg-gray-900 text-gray-300 border border-gray-600'
                                    ]">
                                        {{ server.containerStatus || 'Unknown' }}
                                    </span>
                                </div>
                            </div>
                            <!-- Expand Button -->
                            <div class="ml-2">
                                <svg
                                    :class="['w-5 h-5 text-gray-400 transition-transform duration-200 heroicon', isServerExpanded(serverName) ? 'rotate-180' : '']"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                >
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                </svg>
                            </div>
                        </div>
                    </div>

                    <!-- Expanded OAuth Configuration -->
                    <div v-if="isServerExpanded(serverName)" class="border-t border-gray-700 p-4 lg:p-6 bg-gray-800">
                        <div class="space-y-6">
                            <!-- OAuth Toggle -->
                            <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-gray-700 rounded-lg">
                                <div>
                                    <h5 class="text-sm font-medium text-gray-100">OAuth Authentication</h5>
                                    <p class="text-xs text-gray-400">Require OAuth tokens to access this server</p>
                                </div>
                                <label class="relative inline-flex items-center cursor-pointer">
                                    <input 
                                        type="checkbox" 
                                        :checked="getOAuthConfig(serverName).enabled" 
                                        @change="updateOAuthEnabled(serverName, $event.target.checked)" 
                                        class="sr-only peer"
                                    >
                                    <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                </label>
                            </div>
                            
                            <!-- OAuth Settings (shown when enabled) -->
                            <div v-if="getOAuthConfig(serverName).enabled" class="space-y-4 pl-4 border-l-2 border-blue-600">
                                <!-- Required Scope -->
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-2">Required Scope</label>
                                    <select 
                                        :value="getOAuthConfig(serverName).required_scope" 
                                        @change="updateRequiredScope(serverName, $event.target.value)"
                                        class="form-input w-full"
                                    >
                                        <option value="">No specific scope required</option>
                                        <option v-for="scope in scopes" :key="scope.name" :value="scope.name">
                                            {{ scope.name }} - {{ scope.description }}
                                        </option>
                                    </select>
                                </div>
                                
                                <!-- Settings Grid -->
                                <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                    <!-- API Key Fallback -->
                                    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-gray-700 rounded-lg">
                                        <div>
                                            <label class="text-sm font-medium text-gray-300">API Key Fallback</label>
                                            <p class="text-xs text-gray-400">Allow API key when OAuth fails</p>
                                        </div>
                                        <label class="relative inline-flex items-center cursor-pointer">
                                            <input 
                                                type="checkbox" 
                                                :checked="getOAuthConfig(serverName).allow_api_key_fallback" 
                                                @change="updateAPIKeyFallback(serverName, $event.target.checked)" 
                                                class="sr-only peer"
                                            >
                                            <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                        </label>
                                    </div>
                                    
                                    <!-- Optional Auth -->
                                    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 p-4 bg-gray-700 rounded-lg">
                                        <div>
                                            <label class="text-sm font-medium text-gray-300">Optional Authentication</label>
                                            <p class="text-xs text-gray-400">Allow unauthenticated access</p>
                                        </div>
                                        <label class="relative inline-flex items-center cursor-pointer">
                                            <input 
                                                type="checkbox" 
                                                :checked="getOAuthConfig(serverName).optional_auth" 
                                                @change="updateOptionalAuth(serverName, $event.target.checked)" 
                                                class="sr-only peer"
                                            >
                                            <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                        </label>
                                    </div>
                                </div>
                                
                                <!-- Allowed Clients -->
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-3">Allowed OAuth Clients</label>
                                    <div v-if="clients.length === 0" class="text-sm text-gray-400 text-center py-6 bg-gray-700 rounded-lg border-2 border-dashed border-gray-600">
                                        <svg class="w-8 h-8 mx-auto mb-2 text-gray-500 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('users')"></path>
                                        </svg>
                                        No OAuth clients available
                                    </div>
                                    <div v-else class="grid grid-cols-1 lg:grid-cols-2 gap-3">
                                        <label 
                                            v-for="client in clients" 
                                            :key="client.client_id" 
                                            class="flex items-start gap-3 p-3 bg-gray-700 rounded-lg border border-gray-600 hover:border-gray-500 transition-colors cursor-pointer"
                                        >
                                            <input 
                                                type="checkbox" 
                                                :value="client.client_id" 
                                                :checked="isClientAllowed(serverName, client.client_id)"
                                                @change="updateAllowedClients(serverName, client.client_id, $event.target.checked)"
                                                class="form-checkbox h-4 w-4 text-blue-600 rounded mt-1 flex-shrink-0"
                                            >
                                            <div class="min-w-0 flex-1">
                                                <div class="flex items-center gap-2 mb-1">
                                                    <span class="text-sm font-medium text-gray-100">{{ client.name }}</span>
                                                    <span :class="[
                                                        'inline-flex items-center px-1.5 py-0.5 rounded text-xs font-medium',
                                                        client.public ? 'bg-blue-800 text-blue-200' : 'bg-orange-800 text-orange-200'
                                                    ]">
                                                        {{ client.public ? 'Public' : 'Confidential' }}
                                                    </span>
                                                </div>
                                                <code class="text-xs text-gray-400 break-all block">{{ client.client_id }}</code>
                                                <p v-if="client.description" class="text-xs text-gray-500 mt-1">{{ client.description }}</p>
                                            </div>
                                        </label>
                                    </div>
                                </div>
                            </div>
                            
                            <!-- Action Buttons -->
                            <div class="flex flex-col sm:flex-row gap-3 pt-4 border-t border-gray-700">
                                <button 
                                    @click="testServerAccess(serverName)" 
                                    class="flex items-center justify-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm touch-target transition-colors font-medium"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play')"></path>
                                    </svg>
                                    Test Access
                                </button>
                                <button 
                                    @click="viewServerTokens(serverName)" 
                                    class="flex items-center justify-center px-4 py-2 bg-gray-600 text-white rounded-lg hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500 text-sm touch-target transition-colors font-medium"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('key')"></path>
                                    </svg>
                                    View Tokens
                                </button>
                                <button 
                                    @click="saveServerConfig(serverName)" 
                                    :disabled="saving" 
                                    class="flex items-center justify-center px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500 disabled:opacity-50 text-sm touch-target transition-colors font-medium"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('save')"></path>
                                    </svg>
                                    {{ saving ? 'Saving...' : 'Save Config' }}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};