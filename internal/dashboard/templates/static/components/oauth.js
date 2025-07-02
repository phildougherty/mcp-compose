const OAuthConfig = {
    emits: ['show-toast'],
    data() {
        return {
            loading: false,
            error: null,
            oauthStatus: { active_tokens: {}, oauth_enabled: false },
            clients: [],
            selectedTestClient: null,
            showCreateClient: false,
            showClientDetails: null,
            creating: false,
            newClient: {
                name: '',
                description: '',
                redirect_uris: `${window.location.origin}/oauth/callback`,
                public: true
            },
            baseUrl: window.location.origin,
            // Enhanced filtering and sorting
            clientSearchTerm: '',
            clientFilter: 'all',
            sortBy: 'name',
            expandedSections: new Set(),
            autoRefresh: false,
            refreshInterval: null
        }
    },
    computed: {
        filteredClients() {
            let filtered = this.clients.filter(client => {
                const matchesSearch = !this.clientSearchTerm || 
                    client.name?.toLowerCase().includes(this.clientSearchTerm.toLowerCase()) ||
                    client.client_id?.toLowerCase().includes(this.clientSearchTerm.toLowerCase()) ||
                    client.description?.toLowerCase().includes(this.clientSearchTerm.toLowerCase());
                
                const matchesFilter = this.clientFilter === 'all' ||
                    (this.clientFilter === 'public' && client.public) ||
                    (this.clientFilter === 'confidential' && !client.public);
                
                return matchesSearch && matchesFilter;
            });
    
            return filtered.sort((a, b) => {
                switch (this.sortBy) {
                    case 'type':
                        const aType = a.public ? 'Public' : 'Confidential';
                        const bType = b.public ? 'Public' : 'Confidential';
                        return aType.localeCompare(bType);
                    case 'created':
                        const aDate = new Date(a.created_at || 0);
                        const bDate = new Date(b.created_at || 0);
                        return bDate - aDate;
                    default:
                        // Safely handle name comparison
                        const aName = a.name || '';
                        const bName = b.name || '';
                        return aName.localeCompare(bName);
                }
            });
        },
    
        statusCounts() {
            return {
                total: this.clients.length,
                public: this.clients.filter(c => c.public).length,
                confidential: this.clients.filter(c => !c.public).length,
                active: this.oauthStatus.active_tokens?.access_tokens || 0
            };
        },
        availableScopes() {
            return [
                { name: 'mcp:tools', description: 'Access to MCP tools' },
                { name: 'mcp:resources', description: 'Access to MCP resources' },
                { name: 'mcp:prompts', description: 'Access to MCP prompts' },
                { name: 'admin', description: 'Administrative access' }
            ];
        }
    },
    async mounted() {
        await this.loadData();
        this.setupAutoRefresh();
    },
    beforeUnmount() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    },
    methods: {
        // Heroicon helper
        getHeroIcon(iconName) {
            const icons = {
                'shield-check': 'M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z',
                'users': 'M12 4.354a4 4 0 110 5.292M12 4.354a4 4 0 000 5.292M12 4.354v5.292M16 14a4 4 0 11-8 0 4 4 0 018 0zm-8 0a4 4 0 110-8 4 4 0 010 8z',
                'key': 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
                'link': 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1',
                'check-circle': 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
                'x-circle': 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
                'plus': 'M12 4v16m8-8H4',
                'refresh': 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15',
                'search': 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
                'filter': 'M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.207A1 1 0 013 6.5V4z',
                'cog': 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z',
                'eye': 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z',
                'trash': 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16',
                'clipboard-copy': 'M8 5H6a2 2 0 00-2 2v6a2 2 0 002 2h8a2 2 0 002-2v-3m-4-3V3a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 3h4v4',
                'play': 'M14.828 14.828a4 4 0 01-5.656 0M9 10h1.586a1 1 0 01.707.293l2.414 2.414a1 1 0 00.707.293H15a2 2 0 002-2V9a2 2 0 00-2-2h-1.586a1 1 0 01-.707-.293L10.293 4.293A1 1 0 009.586 4H8a2 2 0 00-2 2v5a2 2 0 002 2z',
                'chart-bar': 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
                'chevron-down': 'M19 9l-7 7-7-7',
                'x': 'M6 18L18 6M6 6l12 12'
            };
            return icons[iconName] || icons['cog'];
        },

        async loadData() {
            this.loading = true;
            this.error = null;
            try {
                const [statusRes, clientsRes] = await Promise.all([
                    fetch('/api/oauth/status'),
                    fetch('/api/oauth/clients')
                ]);
                
                if (statusRes.ok && statusRes.headers.get('content-type')?.includes('application/json')) {
                    this.oauthStatus = await statusRes.json();
                } else {
                    console.warn('OAuth status endpoint not available or returned non-JSON');
                    this.oauthStatus = { oauth_enabled: false, active_tokens: {} };
                }
                
                if (clientsRes.ok && clientsRes.headers.get('content-type')?.includes('application/json')) {
                    this.clients = await clientsRes.json();
                } else {
                    console.warn('OAuth clients endpoint not available or returned non-JSON');
                    this.clients = [];
                }
            } catch (error) {
                this.error = `Failed to load OAuth data: ${error.message}`;
                console.error('Failed to load OAuth data:', error);
                this.oauthStatus = { oauth_enabled: false, active_tokens: {} };
                this.clients = [];
                this.showToast('OAuth endpoints not available', 'warning');
            } finally {
                this.loading = false;
            }
        },

        // Enhanced UI methods
        toggleSection(sectionId) {
            if (this.expandedSections.has(sectionId)) {
                this.expandedSections.delete(sectionId);
            } else {
                this.expandedSections.add(sectionId);
            }
            this.$forceUpdate();
        },

        isSectionExpanded(sectionId) {
            return this.expandedSections.has(sectionId);
        },

        setupAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
            if (this.autoRefresh) {
                this.refreshInterval = setInterval(() => {
                    this.loadData();
                }, 30000);
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
        // Client management methods
        async createClient() {
            this.creating = true;
            try {
                const clientData = {
                    client_name: this.newClient.name,
                    client_description: this.newClient.description,
                    redirect_uris: this.newClient.redirect_uris.split('\n').filter(uri => uri.trim()),
                    grant_types: this.newClient.public 
                        ? ['authorization_code', 'refresh_token'] 
                        : ['authorization_code', 'client_credentials', 'refresh_token'],
                    response_types: ['code'],
                    token_endpoint_auth_method: this.newClient.public ? 'none' : 'client_secret_post'
                };
                
                const response = await fetch('/oauth/register', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(clientData)
                });
                
                if (response.ok) {
                    const client = await response.json();
                    this.clients.push(client);
                    this.showCreateClient = false;
                    this.resetNewClient();
                    this.showToast('OAuth client created successfully','success');
                } else {
                    const errorText = await response.text();
                    throw new Error(`Failed to create client: ${response.status} - ${errorText}`);
                }
            } catch (error) {
                this.showToast(`Failed to create client: ${error.message}`, 'error');
            } finally {
                this.creating = false;
            }
        },

        async deleteClient(clientId, clientName) {
            if (!confirm(`Delete OAuth client "${clientName}"?\n\nThis action cannot be undone and will invalidate all tokens for this client.`)) return;
            
            try {
                const response = await fetch(`/api/oauth/clients/${clientId}`, { method: 'DELETE' });
                if (response.ok) {
                    this.clients = this.clients.filter(c => c.client_id !== clientId);
                    this.showToast('Client deleted successfully','success');
                } else {
                    throw new Error('Failed to delete client');
                }
            } catch (error) {
                this.showToast(`Failed to delete client: ${error.message}`, 'error');
            }
        },

        viewClientDetails(client) {
            this.showClientDetails = client;
        },

        resetNewClient() {
            this.newClient = { 
                name: '', 
                description: '', 
                redirect_uris: `${window.location.origin}/oauth/callback`, 
                public: true 
            };
        },

        // OAuth testing methods
        testAuthFlow() {
            if (!this.selectedTestClient) return;
            
            const state = Math.random().toString(36).substring(2, 15);
            sessionStorage.setItem('oauth_test_return', window.location.href);
            
            const authParams = new URLSearchParams({
                response_type: 'code',
                client_id: this.selectedTestClient.client_id,
                redirect_uri: this.selectedTestClient.redirect_uris[0],
                scope: 'mcp:tools',
                state: state
            });
            
            const authUrl = `/oauth/authorize?${authParams.toString()}`;
            window.location.href = authUrl;
        },

        async testClientCredentials() {
            if (!this.selectedTestClient || this.selectedTestClient.public) {
                this.showToast('Client credentials flow requires a confidential client', 'error');
                return;
            }
            
            try {
                const response = await fetch('/oauth/token', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                    body: `grant_type=client_credentials&client_id=${this.selectedTestClient.client_id}&client_secret=${this.selectedTestClient.client_secret}&scope=mcp:tools`
                });
                
                if (response.ok) {
                    const token = await response.json();
                    this.showToast('Client credentials flow successful!','success');
                    console.log('Token:', token);
                } else {
                    const errorText = await response.text();
                    throw new Error(`Token request failed: ${response.status} - ${errorText}`);
                }
            } catch (error) {
                this.showToast(`Client credentials test failed: ${error.message}`, 'error');
            }
        },

        // Utility methods
        copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                this.showToast('Copied to clipboard!', 'success');
            }).catch(err => {
                this.showToast('Failed to copy to clipboard', 'error');
            });
        },

        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                const date = new Date(timestamp);
                return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } catch (e) {
                return timestamp;
            }
        }
    },

    template: `
        <div class="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
            <!-- Enhanced Header -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col space-y-4">
                    <!-- Title and Controls -->
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div class="flex items-center space-x-3">
                            <div class="w-10 h-10 bg-gradient-to-r from-green-500 to-blue-600 rounded-xl flex items-center justify-center">
                                <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('shield-check')"></path>
                                </svg>
                            </div>
                            <div>
                                <h3 class="text-lg font-semibold text-gray-100">OAuth 2.1 Configuration</h3>
                                <p class="text-sm text-gray-300">Manage authentication clients and test OAuth flows</p>
                            </div>
                        </div>
                        
                        <!-- Action Buttons -->
                        <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                            <button
                                @click="showCreateClient = true"
                                class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-medium text-sm touch-target"
                            >
                                <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('plus')"></path>
                                </svg>
                                Register Client
                            </button>
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
                    
                    <!-- Stats Grid -->
                    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('users')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.total }}</p>
                                    <p class="text-xs text-gray-300">Total Clients</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                    <div class="w-2 h-2 bg-white rounded-full"></div>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.public }}</p>
                                    <p class="text-xs text-gray-300">Public</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-orange-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('key')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.confidential }}</p>
                                    <p class="text-xs text-gray-300">Confidential</p>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 rounded-lg p-3 border border-gray-700">
                            <div class="flex items-center space-x-2">
                                <div class="w-8 h-8 bg-purple-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chart-bar')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-2xl font-bold text-gray-100">{{ statusCounts.active }}</p>
                                    <p class="text-xs text-gray-300">Active Tokens</p>
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

            <!-- Loading State -->
            <div v-if="loading && clients.length === 0" class="enhanced-card p-8 text-center">
                <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
                <p class="text-lg font-medium text-gray-100">Loading OAuth configuration...</p>
                <p class="text-sm text-gray-300">Fetching clients and server status</p>
            </div>

            <!-- OAuth Status Section -->
            <div v-else class="space-y-6">
                <!-- OAuth Server Status -->
                <div class="enhanced-card overflow-hidden">
                    <div
                        @click="toggleSection('oauth-status')"
                        class="p-4 lg:p-6 cursor-pointer hover:bg-gray-700/30 transition-colors"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-3">
                                <div class="w-10 h-10 bg-blue-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('cog')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <h4 class="text-lg font-medium text-gray-100">OAuth Server Status</h4>
                                    <p class="text-sm text-gray-300">Server configuration and active tokens</p>
                                </div>
                            </div>
                            <div class="flex items-center space-x-3">
                                <span :class="[
                                    'inline-flex items-center px-3 py-1 rounded-full text-sm font-medium',
                                    oauthStatus.oauth_enabled ? 'bg-green-900 text-green-200 border border-green-700' : 'bg-red-900 text-red-200 border border-red-700'
                                ]">
                                    <svg class="w-4 h-4 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(oauthStatus.oauth_enabled ? 'check-circle' : 'x-circle')"></path>
                                    </svg>
                                    {{ oauthStatus.oauth_enabled ? 'Enabled' : 'Disabled' }}
                                </span>
                                <svg
                                    :class="['w-5 h-5 text-gray-400 transition-transform duration-200 heroicon', isSectionExpanded('oauth-status') ? 'rotate-180' : '']"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                >
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                </svg>
                            </div>
                        </div>
                    </div>
                    
                    <div v-if="isSectionExpanded('oauth-status')" class="border-t border-gray-700 p-4 lg:p-6 bg-gray-800">
                        <div v-if="oauthStatus.oauth_enabled" class="space-y-4">
                            <!-- Token Stats -->
                            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
                                <div class="text-center p-4 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-blue-400">
                                        {{ oauthStatus.active_tokens?.access_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300">Access Tokens</div>
                                </div>
                                <div class="text-center p-4 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-green-400">
                                        {{ oauthStatus.active_tokens?.refresh_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300">Refresh Tokens</div>
                                </div>
                                <div class="text-center p-4 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-yellow-400">
                                        {{ oauthStatus.active_tokens?.auth_codes || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300">Auth Codes</div>
                                </div>
                            </div>
                            
                            <!-- Issuer URL -->
                            <div v-if="oauthStatus.issuer">
                                <label class="block text-sm font-medium text-gray-300 mb-2">Issuer URL</label>
                                <div class="flex">
                                    <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                        {{ oauthStatus.issuer }}
                                    </code>
                                    <button 
                                        @click="copyToClipboard(oauthStatus.issuer)" 
                                        class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                    >
                                        <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                        <div v-else class="text-center py-8 text-gray-400">
                            <svg class="w-12 h-12 mx-auto mb-4 text-gray-500 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                            </svg>
                            <p class="text-lg font-medium">OAuth Server Disabled</p>
                            <p class="text-sm">OAuth authentication is not currently enabled on this server</p>
                        </div>
                    </div>
                </div>

                <!-- OAuth Endpoints -->
                <div class="enhanced-card overflow-hidden">
                    <div
                        @click="toggleSection('oauth-endpoints')"
                        class="p-4 lg:p-6 cursor-pointer hover:bg-gray-700/30 transition-colors"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-3">
                                <div class="w-10 h-10 bg-purple-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('link')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <h4 class="text-lg font-medium text-gray-100">OAuth Endpoints</h4>
                                    <p class="text-sm text-gray-300">Available OAuth 2.1 endpoints for integration</p>
                                </div>
                            </div>
                            <svg
                                :class="['w-5 h-5 text-gray-400 transition-transform duration-200 heroicon', isSectionExpanded('oauth-endpoints') ? 'rotate-180' : '']"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                            </svg>
                        </div>
                    </div>
                    
                    <div v-if="isSectionExpanded('oauth-endpoints')" class="border-t border-gray-700 p-4 lg:p-6 bg-gray-800">
                        <div class="space-y-4">
                            <!-- Authorization Endpoint -->
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Authorization Endpoint</label>
                                <div class="flex">
                                    <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                        {{ baseUrl }}/oauth/authorize
                                    </code>
                                    <button 
                                        @click="copyToClipboard(baseUrl + '/oauth/authorize')" 
                                        class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                    >
                                        <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                            
                            <!-- Token Endpoint -->
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Token Endpoint</label>
                                <div class="flex">
                                    <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                        {{ baseUrl }}/oauth/token
                                    </code>
                                    <button 
                                        @click="copyToClipboard(baseUrl + '/oauth/token')" 
                                        class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                    >
                                        <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                            
                            <!-- Discovery Endpoint -->
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Discovery Endpoint</label>
                                <div class="flex">
                                    <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                        {{ baseUrl }}/.well-known/oauth-authorization-server
                                    </code>
                                    <button 
                                        @click="copyToClipboard(baseUrl + '/.well-known/oauth-authorization-server')" 
                                        class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                    >
                                        <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- OAuth Clients Management -->
                <div class="enhanced-card">
                    <div class="p-4 lg:p-6">
                        <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0 mb-6">
                            <div class="flex items-center space-x-3">
                                <div class="w-10 h-10 bg-indigo-500 rounded-lg flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('users')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <h4 class="text-lg font-medium text-gray-100">OAuth Clients</h4>
                                    <p class="text-sm text-gray-300">Manage registered OAuth applications</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Search and Filter Controls -->
                        <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-3 lg:space-y-0 mb-6">
                            <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-2xl">
                                <div class="relative flex-1">
                                    <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                        <svg class="h-4 w-4 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('search')"></path>
                                        </svg>
                                    </div>
                                    <input 
                                        v-model="clientSearchTerm"
                                        type="text" 
                                        placeholder="Search clients..." 
                                        class="form-input pl-10 w-full"
                                    >
                                </div>
                                
                                <select v-model="clientFilter" class="form-input w-full sm:w-auto">
                                    <option value="all">All Types</option>
                                    <option value="public">Public</option>
                                    <option value="confidential">Confidential</option>
                                </select>
                                
                                <select v-model="sortBy" class="form-input w-full sm:w-auto">
                                    <option value="name">Sort by Name</option>
                                    <option value="type">Sort by Type</option>
                                    <option value="created">Sort by Created</option>
                                </select>
                            </div>
                        </div>
                        
                        <!-- Empty State -->
                        <div v-if="filteredClients.length === 0 && !loading" class="text-center py-12">
                            <svg class="mx-auto h-12 w-12 text-gray-500 mb-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                            </svg>
                            <h3 class="text-lg font-medium text-gray-100 mb-2">No OAuth clients found</h3>
                            <p class="text-gray-300 mb-4">
                                {{ clientSearchTerm || clientFilter !== 'all'
                                    ? 'Try adjusting your search or filters'
                                    : 'Get started by registering your first OAuth client' }}
                            </p>
                            <button
                                v-if="!clientSearchTerm && clientFilter === 'all'"
                                @click="showCreateClient = true"
                                class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-medium"
                            >
                                <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('plus')"></path>
                                </svg>
                                Register First Client
                            </button>
                        </div>
                        
                        <!-- Clients Grid -->
                        <div v-else class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-4">
                            <div
                                v-for="client in filteredClients"
                                :key="client.client_id"
                                class="bg-gray-800 border border-gray-700 rounded-lg p-4 hover:border-gray-600 transition-colors"
                            >
                                <div class="space-y-3">
                                    <!-- Client Header -->
                                    <div class="flex items-start justify-between">
                                        <div class="flex-1 min-w-0">
                                            <h5 class="font-medium text-gray-100 truncate">{{ client.name }}</h5>
                                            <p v-if="client.description" class="text-xs text-gray-400 mt-1 line-clamp-2">
                                                {{ client.description }}
                                            </p>
                                        </div>
                                        <span :class="[
                                            'flex-shrink-0 inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ml-2',
                                            client.public ? 'bg-blue-900 text-blue-200 border border-blue-700' : 'bg-orange-900 text-orange-200 border border-orange-700'
                                        ]">
                                            <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(client.public ? 'users' : 'key')"></path>
                                            </svg>
                                            {{ client.public ? 'Public' : 'Confidential' }}
                                        </span>
                                    </div>
                                    
                                    <!-- Client ID -->
                                    <div>
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Client ID</label>
                                        <code class="text-xs bg-gray-900 text-gray-300 px-2 py-1 rounded break-all block">
                                            {{ client.client_id }}
                                        </code>
                                    </div>
                                    
                                    <!-- Scopes -->
                                    <div v-if="client.scope">
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Scopes</label>
                                        <div class="flex flex-wrap gap-1">
                                            <span 
                                                v-for="scope in (client.scope || '').split(' ').filter(s => s)" 
                                                :key="scope"
                                                class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-700 text-gray-200 border border-gray-600"
                                            >
                                                {{ scope }}
                                            </span>
                                        </div>
                                    </div>
                                    
                                    <!-- Actions -->
                                    <div class="flex flex-wrap gap-2 pt-2">
                                        <button 
                                            @click="viewClientDetails(client)" 
                                            class="flex items-center px-2 py-1 text-blue-400 hover:text-blue-300 text-xs touch-target transition-colors"
                                        >
                                            <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('eye')"></path>
                                            </svg>
                                            View
                                        </button>
                                        <button 
                                            @click="deleteClient(client.client_id, client.name)" 
                                            class="flex items-center px-2 py-1 text-red-400 hover:text-red-300 text-xs touch-target transition-colors"
                                        >
                                            <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('trash')"></path>
                                            </svg>
                                            Delete
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Test OAuth Flow -->
                <div class="enhanced-card">
                    <div class="p-4 lg:p-6">
                        <div class="flex items-center space-x-3 mb-6">
                            <div class="w-10 h-10 bg-green-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play')"></path>
                                </svg>
                            </div>
                            <div>
                                <h4 class="text-lg font-medium text-gray-100">Test OAuth Flow</h4>
                                <p class="text-sm text-gray-300">Test authentication flows with your clients</p>
                            </div>
                        </div>
                        
                        <div class="space-y-4">
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Test Client</label>
                                <select v-model="selectedTestClient" class="form-input w-full">
                                    <option :value="null">Select a client to test</option>
                                    <option v-for="client in clients" :key="client.client_id" :value="client">
                                        {{ client.name }} ({{ client.public ? 'Public' : 'Confidential' }})
                                    </option>
                                </select>
                            </div>
                            
                            <div class="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                <button 
                                    @click="testAuthFlow" 
                                    :disabled="!selectedTestClient" 
                                    class="flex items-center justify-center px-4 py-3 bg-green-600 text-white rounded-lg hover:bg-green-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium touch-target transition-colors"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play')"></path>
                                    </svg>
                                    Test Authorization Flow
                                </button>
                                
                                <button 
                                    @click="testClientCredentials" 
                                    :disabled="!selectedTestClient || selectedTestClient?.public" 
                                    class="flex items-center justify-center px-4 py-3 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-medium touch-target transition-colors"
                                >
                                    <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('key')"></path>
                                    </svg>
                                    Test Client Credentials
                                </button>
                            </div>
                            
                            <div v-if="selectedTestClient?.public" class="text-xs text-gray-400 bg-gray-800 p-3 rounded border border-gray-700">
                                <strong>Note:</strong> Client credentials flow is only available for confidential clients. 
                                Public clients should use the authorization code flow.
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Create Client Modal -->
            <div v-if="showCreateClient" class="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4 overflow-y-auto">
                <div class="bg-gray-800 border border-gray-700 rounded-lg max-w-lg w-full">
                    <div class="flex items-center justify-between p-6 border-b border-gray-700">
                        <h3 class="text-lg font-medium text-gray-100">Register New OAuth Client</h3>
                        <button 
                            @click="showCreateClient = false; resetNewClient()" 
                            class="text-gray-400 hover:text-gray-200 transition-colors touch-target"
                        >
                            <svg class="w-6 h-6 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x')"></path>
                            </svg>
                        </button>
                    </div>
                    
                    <form @submit.prevent="createClient" class="p-6">
                        <div class="space-y-4">
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Client Name *</label>
                                <input 
                                    v-model="newClient.name" 
                                    type="text" 
                                    required 
                                    class="form-input w-full"
                                    placeholder="My Application"
                                >
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Description</label>
                                <input 
                                    v-model="newClient.description" 
                                    type="text" 
                                    class="form-input w-full"
                                    placeholder="Brief description of your application"
                                >
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-2">Redirect URIs *</label>
                                <textarea 
                                    v-model="newClient.redirect_uris" 
                                    rows="3" 
                                    required
                                    class="form-input w-full"
                                    placeholder="https://yourapp.com/oauth/callback&#10;http://localhost:3000/callback"
                                ></textarea>
                                <p class="text-xs text-gray-400 mt-1">One URI per line</p>
                            </div>
                            
                            <div class="flex items-center">
                                <input 
                                    v-model="newClient.public" 
                                    type="checkbox" 
                                    id="publicClient" 
                                    class="form-checkbox h-4 w-4 text-blue-600 rounded"
                                >
                                <label for="publicClient" class="ml-2 text-sm text-gray-300">
                                    Public Client (mobile apps, SPAs - no client secret)
                                </label>
                            </div>
                        </div>
                        
                        <div class="flex flex-col sm:flex-row justify-end gap-3 mt-6 pt-6 border-t border-gray-700">
                            <button 
                                type="button" 
                                @click="showCreateClient = false; resetNewClient()" 
                                class="w-full sm:w-auto px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors touch-target"
                            >
                                Cancel
                            </button>
                            <button 
                                type="submit" 
                                :disabled="creating || !newClient.name.trim()" 
                                class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors touch-target"
                            >
                                <span v-if="creating" class="flex items-center justify-center">
                                    <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" fill="none" viewBox="0 0 24 24">
                                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                                    </svg>
                                    Creating...
                                </span>
                                <span v-else>Create Client</span>
                            </button>
                        </div>
                    </form>
                </div>
            </div>

            <!-- Client Details Modal -->
            <div v-if="showClientDetails" class="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4 overflow-y-auto">
                <div class="bg-gray-800 border border-gray-700 rounded-lg max-w-2xl w-full">
                    <div class="flex items-center justify-between p-6 border-b border-gray-700">
                        <h3 class="text-lg font-medium text-gray-100">Client Details</h3>
                        <button 
                            @click="showClientDetails = null" 
                            class="text-gray-400 hover:text-gray-200 transition-colors touch-target"
                        >
                            <svg class="w-6 h-6 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x')"></path>
                            </svg>
                        </button>
                    </div>
                    
                    <div class="p-6 space-y-4">
                        <!-- Client Info -->
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                            <div>
                                <label class="block text-sm font-medium text-gray-400 mb-1">Name</label>
                                <p class="text-gray-100">{{ showClientDetails.name }}</p>
                            </div>
                            <div>
                                <label class="block text-sm font-medium text-gray-400 mb-1">Type</label>
                                <span :class="[
                                    'inline-flex items-center px-2 py-1 rounded-full text-xs font-medium',
                                    showClientDetails.public ? 'bg-blue-900 text-blue-200' : 'bg-orange-900 text-orange-200'
                                ]">
                                    {{ showClientDetails.public ? 'Public' : 'Confidential' }}
                                </span>
                            </div>
                        </div>
                        
                        <!-- Description -->
                        <div v-if="showClientDetails.description">
                            <label class="block text-sm font-medium text-gray-400 mb-1">Description</label>
                            <p class="text-gray-100">{{ showClientDetails.description }}</p>
                        </div>
                        
                        <!-- Client ID -->
                        <div>
                            <label class="block text-sm font-medium text-gray-400 mb-1">Client ID</label>
                            <div class="flex">
                                <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                    {{ showClientDetails.client_id }}
                                </code>
                                <button 
                                    @click="copyToClipboard(showClientDetails.client_id)" 
                                    class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                >
                                    <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                    </svg>
                                </button>
                            </div>
                        </div>
                        
                        <!-- Client Secret (for confidential clients) -->
                        <div v-if="!showClientDetails.public && showClientDetails.client_secret">
                            <label class="block text-sm font-medium text-gray-400 mb-1">Client Secret</label>
                            <div class="flex">
                                <code class="flex-1 px-3 py-2 bg-gray-900 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                    {{ showClientDetails.client_secret }}
                                </code>
                                <button 
                                    @click="copyToClipboard(showClientDetails.client_secret)" 
                                    class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target transition-colors"
                                >
                                    <svg class="w-4 h-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                    </svg>
                                </button>
                            </div>
                            <p class="text-xs text-yellow-300 mt-1">⚠️ Store this secret securely. It won't be shown again.</p>
                        </div>
                        
                        <!-- Redirect URIs -->
                        <div v-if="showClientDetails.redirect_uris?.length">
                            <label class="block text-sm font-medium text-gray-400 mb-1">Redirect URIs</label>
                            <div class="space-y-2">
                                <code 
                                    v-for="uri in showClientDetails.redirect_uris" 
                                    :key="uri"
                                    class="block px-3 py-2 bg-gray-900 border border-gray-600 rounded text-sm break-all text-gray-200"
                                >
                                    {{ uri }}
                                </code>
                            </div>
                        </div>
                        
                        <!-- Scopes -->
                        <div v-if="showClientDetails.scope">
                            <label class="block text-sm font-medium text-gray-400 mb-1">Scopes</label>
                            <div class="flex flex-wrap gap-1">
                                <span 
                                    v-for="scope in showClientDetails.scope.split(' ').filter(s => s)" 
                                    :key="scope"
                                    class="inline-flex items-center px-2 py-1 rounded text-xs font-medium bg-gray-700 text-gray-200 border border-gray-600"
                                >
                                    {{ scope }}
                                </span>
                            </div>
                        </div>
                    </div>
                    
                    <div class="flex justify-end gap-3 p-6 border-t border-gray-700">
                        <button 
                            @click="showClientDetails = null" 
                            class="px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors touch-target"
                        >
                            Close
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `
};