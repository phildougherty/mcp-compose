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
                console.log(`[${type.toUpperCase()}] ${message}`);
            }
        },
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
            <!-- Enhanced Header with Security Gradient -->
            <div class="enhanced-card p-6 bg-gradient-to-br from-gray-800 via-gray-800 to-green-900/20 border-green-500/20">
                <div class="flex flex-col space-y-4">
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div class="flex items-center space-x-4">
                            <div class="w-14 h-14 bg-gradient-to-br from-green-500 to-emerald-600 rounded-2xl flex items-center justify-center shadow-lg shadow-green-500/30 ring-4 ring-green-500/10">
                                <svg class="w-8 h-8 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('shield-check')"></path>
                                </svg>
                            </div>
                            <div>
                                <h3 class="text-2xl font-bold text-gray-100 tracking-tight">OAuth 2.1 Security</h3>
                                <p class="text-sm text-gray-300 mt-1">Enterprise-grade authentication and authorization management</p>
                            </div>
                        </div>

                        <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                            <button
                                @click="showCreateClient = true"
                                class="inline-flex items-center justify-center px-5 py-2.5 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-xl hover:from-green-700 hover:to-emerald-700 transition-all duration-200 font-semibold text-sm touch-target shadow-lg shadow-green-600/30 hover:shadow-xl hover:shadow-green-600/40 hover:scale-105"
                            >
                                <svg class="w-5 h-5 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('plus')"></path>
                                </svg>
                                Register Client
                            </button>
                            <button
                                @click="loadData"
                                :disabled="loading"
                                class="inline-flex items-center justify-center px-5 py-2.5 border-2 border-gray-600 text-gray-300 bg-gray-700/50 rounded-xl hover:bg-gray-600/50 hover:border-gray-500 transition-all duration-200 font-semibold text-sm touch-target disabled:opacity-50 backdrop-blur-sm"
                            >
                                <svg class="w-5 h-5 mr-2 heroicon" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('refresh')"></path>
                                </svg>
                                Refresh
                            </button>
                        </div>
                    </div>

                    <!-- Enhanced Stats Grid with Glassmorphism -->
                    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 mt-2">
                        <div class="bg-gray-800/60 backdrop-blur-xl rounded-xl p-4 border border-gray-700/50 hover:border-blue-500/50 transition-all duration-300 hover:shadow-lg hover:shadow-blue-500/20">
                            <div class="flex items-center space-x-3">
                                <div class="w-12 h-12 bg-gradient-to-br from-blue-500 to-blue-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/30">
                                    <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('users')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-3xl font-bold text-gray-100">{{ statusCounts.total }}</p>
                                    <p class="text-xs text-gray-400 font-medium">Total Clients</p>
                                </div>
                            </div>
                        </div>

                        <div class="bg-gray-800/60 backdrop-blur-xl rounded-xl p-4 border border-gray-700/50 hover:border-green-500/50 transition-all duration-300 hover:shadow-lg hover:shadow-green-500/20">
                            <div class="flex items-center space-x-3">
                                <div class="w-12 h-12 bg-gradient-to-br from-green-500 to-emerald-600 rounded-xl flex items-center justify-center shadow-lg shadow-green-500/30">
                                    <div class="w-3 h-3 bg-white rounded-full animate-pulse"></div>
                                </div>
                                <div>
                                    <p class="text-3xl font-bold text-gray-100">{{ statusCounts.public }}</p>
                                    <p class="text-xs text-gray-400 font-medium">Public</p>
                                </div>
                            </div>
                        </div>

                        <div class="bg-gray-800/60 backdrop-blur-xl rounded-xl p-4 border border-gray-700/50 hover:border-orange-500/50 transition-all duration-300 hover:shadow-lg hover:shadow-orange-500/20">
                            <div class="flex items-center space-x-3">
                                <div class="w-12 h-12 bg-gradient-to-br from-orange-500 to-orange-600 rounded-xl flex items-center justify-center shadow-lg shadow-orange-500/30">
                                    <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('key')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-3xl font-bold text-gray-100">{{ statusCounts.confidential }}</p>
                                    <p class="text-xs text-gray-400 font-medium">Confidential</p>
                                </div>
                            </div>
                        </div>

                        <div class="bg-gray-800/60 backdrop-blur-xl rounded-xl p-4 border border-gray-700/50 hover:border-purple-500/50 transition-all duration-300 hover:shadow-lg hover:shadow-purple-500/20">
                            <div class="flex items-center space-x-3">
                                <div class="w-12 h-12 bg-gradient-to-br from-purple-500 to-purple-600 rounded-xl flex items-center justify-center shadow-lg shadow-purple-500/30">
                                    <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chart-bar')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <p class="text-3xl font-bold text-gray-100">{{ statusCounts.active }}</p>
                                    <p class="text-xs text-gray-400 font-medium">Active Tokens</p>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Error Display with Enhanced Styling -->
            <div v-if="error" class="enhanced-card border-2 border-red-500/50 bg-gradient-to-r from-red-900/30 to-red-800/20 p-4 backdrop-blur-sm animate-pulse">
                <div class="flex items-start">
                    <svg class="h-6 w-6 text-red-400 mt-0.5 flex-shrink-0 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                    </svg>
                    <div class="ml-3 flex-1">
                        <div class="text-sm text-red-200 font-medium">{{ error }}</div>
                        <button @click="error = null" class="mt-2 text-xs text-red-400 hover:text-red-300 underline touch-target font-semibold">
                            Dismiss
                        </button>
                    </div>
                </div>
            </div>

            <!-- Loading State with Better Animation -->
            <div v-if="loading && clients.length === 0" class="enhanced-card p-12 text-center">
                <div class="relative w-16 h-16 mx-auto mb-6">
                    <div class="absolute inset-0 rounded-full border-4 border-blue-500/20"></div>
                    <div class="absolute inset-0 rounded-full border-4 border-t-blue-500 animate-spin"></div>
                </div>
                <p class="text-xl font-bold text-gray-100">Loading OAuth Configuration</p>
                <p class="text-sm text-gray-400 mt-2">Fetching clients and server status</p>
            </div>

            <!-- Main Content -->
            <div v-else class="space-y-6">
                <!-- OAuth Server Status -->
                <div class="enhanced-card overflow-hidden hover:shadow-xl transition-shadow duration-300">
                    <div
                        @click="toggleSection('oauth-status')"
                        class="p-6 cursor-pointer hover:bg-gray-700/40 transition-all duration-200"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-4">
                                <div class="w-12 h-12 bg-gradient-to-br from-blue-500 to-blue-600 rounded-xl flex items-center justify-center shadow-lg shadow-blue-500/30">
                                    <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('cog')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <h4 class="text-lg font-bold text-gray-100">OAuth Server Status</h4>
                                    <p class="text-sm text-gray-400">Server configuration and active tokens</p>
                                </div>
                            </div>
                            <div class="flex items-center space-x-3">
                                <span :class="[
                                    'inline-flex items-center px-4 py-2 rounded-full text-sm font-bold shadow-lg transition-all duration-200',
                                    oauthStatus.oauth_enabled
                                        ? 'bg-green-500/20 text-green-300 border-2 border-green-500/50 shadow-green-500/30'
                                        : 'bg-red-500/20 text-red-300 border-2 border-red-500/50 shadow-red-500/30'
                                ]">
                                    <svg class="w-5 h-5 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(oauthStatus.oauth_enabled ? 'check-circle' : 'x-circle')"></path>
                                    </svg>
                                    {{ oauthStatus.oauth_enabled ? 'Enabled' : 'Disabled' }}
                                </span>
                                <svg
                                    :class="['w-6 h-6 text-gray-400 transition-all duration-300 heroicon', isSectionExpanded('oauth-status') ? 'rotate-180' : '']"
                                    fill="none"
                                    stroke="currentColor"
                                    viewBox="0 0 24 24"
                                >
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                </svg>
                            </div>
                        </div>
                    </div>

                    <div v-if="isSectionExpanded('oauth-status')" class="border-t border-gray-700 p-6 bg-gradient-to-b from-gray-800/50 to-gray-800 animate-fade-in">
                        <div v-if="oauthStatus.oauth_enabled" class="space-y-6">
                            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
                                <div class="text-center p-5 bg-gradient-to-br from-gray-700 to-gray-800 rounded-xl border-2 border-blue-500/30 shadow-lg hover:shadow-xl transition-shadow duration-300">
                                    <div class="text-3xl font-bold text-blue-400">
                                        {{ oauthStatus.active_tokens?.access_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300 font-medium mt-1">Access Tokens</div>
                                </div>
                                <div class="text-center p-5 bg-gradient-to-br from-gray-700 to-gray-800 rounded-xl border-2 border-green-500/30 shadow-lg hover:shadow-xl transition-shadow duration-300">
                                    <div class="text-3xl font-bold text-green-400">
                                        {{ oauthStatus.active_tokens?.refresh_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300 font-medium mt-1">Refresh Tokens</div>
                                </div>
                                <div class="text-center p-5 bg-gradient-to-br from-gray-700 to-gray-800 rounded-xl border-2 border-yellow-500/30 shadow-lg hover:shadow-xl transition-shadow duration-300">
                                    <div class="text-3xl font-bold text-yellow-400">
                                        {{ oauthStatus.active_tokens?.auth_codes || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-300 font-medium mt-1">Auth Codes</div>
                                </div>
                            </div>

                            <div v-if="oauthStatus.issuer">
                                <label class="block text-sm font-bold text-gray-300 mb-3">Issuer URL</label>
                                <div class="flex rounded-xl overflow-hidden border-2 border-gray-600">
                                    <code class="flex-1 px-4 py-3 bg-gray-900 text-sm break-all text-gray-200 font-mono">
                                        {{ oauthStatus.issuer }}
                                    </code>
                                    <button
                                        @click="copyToClipboard(oauthStatus.issuer)"
                                        class="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm touch-target transition-all duration-200 hover:shadow-lg"
                                    >
                                        <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                        <div v-else class="text-center py-12 text-gray-400">
                            <div class="w-20 h-20 mx-auto mb-4 bg-gray-700/50 rounded-2xl flex items-center justify-center">
                                <svg class="w-12 h-12 text-gray-500 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                                </svg>
                            </div>
                            <p class="text-xl font-bold">OAuth Server Disabled</p>
                            <p class="text-sm mt-2">OAuth authentication is not currently enabled on this server</p>
                        </div>
                    </div>
                </div>

                <!-- OAuth Endpoints -->
                <div class="enhanced-card overflow-hidden hover:shadow-xl transition-shadow duration-300">
                    <div
                        @click="toggleSection('oauth-endpoints')"
                        class="p-6 cursor-pointer hover:bg-gray-700/40 transition-all duration-200"
                    >
                        <div class="flex items-center justify-between">
                            <div class="flex items-center space-x-4">
                                <div class="w-12 h-12 bg-gradient-to-br from-purple-500 to-purple-600 rounded-xl flex items-center justify-center shadow-lg shadow-purple-500/30">
                                    <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('link')"></path>
                                    </svg>
                                </div>
                                <div>
                                    <h4 class="text-lg font-bold text-gray-100">OAuth Endpoints</h4>
                                    <p class="text-sm text-gray-400">Available OAuth 2.1 endpoints for integration</p>
                                </div>
                            </div>
                            <svg
                                :class="['w-6 h-6 text-gray-400 transition-all duration-300 heroicon', isSectionExpanded('oauth-endpoints') ? 'rotate-180' : '']"
                                fill="none"
                                stroke="currentColor"
                                viewBox="0 0 24 24"
                            >
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                            </svg>
                        </div>
                    </div>

                    <div v-if="isSectionExpanded('oauth-endpoints')" class="border-t border-gray-700 p-6 bg-gradient-to-b from-gray-800/50 to-gray-800 animate-fade-in">
                        <div class="space-y-5">
                            <div v-for="(endpoint, name) in {
                                'Authorization Endpoint': '/oauth/authorize',
                                'Token Endpoint': '/oauth/token',
                                'Discovery Endpoint': '/.well-known/oauth-authorization-server'
                            }" :key="name">
                                <label class="block text-sm font-bold text-gray-300 mb-3">{{ name }}</label>
                                <div class="flex rounded-xl overflow-hidden border-2 border-gray-600 hover:border-blue-500/50 transition-colors duration-200">
                                    <code class="flex-1 px-4 py-3 bg-gray-900 text-sm break-all text-gray-200 font-mono">
                                        {{ baseUrl }}{{ endpoint }}
                                    </code>
                                    <button
                                        @click="copyToClipboard(baseUrl + endpoint)"
                                        class="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm touch-target transition-all duration-200 hover:shadow-lg"
                                    >
                                        <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- OAuth Clients Management -->
                <div class="enhanced-card p-6">
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0 mb-6">
                        <div class="flex items-center space-x-4">
                            <div class="w-12 h-12 bg-gradient-to-br from-indigo-500 to-indigo-600 rounded-xl flex items-center justify-center shadow-lg shadow-indigo-500/30">
                                <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('users')"></path>
                                </svg>
                            </div>
                            <div>
                                <h4 class="text-lg font-bold text-gray-100">OAuth Clients</h4>
                                <p class="text-sm text-gray-400">Manage registered OAuth applications</p>
                            </div>
                        </div>
                    </div>

                    <!-- Enhanced Search and Filter Controls -->
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-3 lg:space-y-0 mb-6 gap-3">
                        <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-3xl">
                            <div class="relative flex-1">
                                <div class="absolute inset-y-0 left-0 pl-4 flex items-center pointer-events-none">
                                    <svg class="h-5 w-5 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('search')"></path>
                                    </svg>
                                </div>
                                <input
                                    v-model="clientSearchTerm"
                                    type="text"
                                    placeholder="Search clients..."
                                    class="form-input pl-12 w-full h-12 rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200"
                                >
                            </div>

                            <select v-model="clientFilter" class="form-input w-full sm:w-auto h-12 rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200 font-medium">
                                <option value="all">All Types</option>
                                <option value="public">Public</option>
                                <option value="confidential">Confidential</option>
                            </select>

                            <select v-model="sortBy" class="form-input w-full sm:w-auto h-12 rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200 font-medium">
                                <option value="name">Sort by Name</option>
                                <option value="type">Sort by Type</option>
                                <option value="created">Sort by Created</option>
                            </select>
                        </div>
                    </div>

                    <!-- Empty State -->
                    <div v-if="filteredClients.length === 0 && !loading" class="text-center py-16 bg-gradient-to-b from-gray-800/50 to-gray-800 rounded-2xl border-2 border-dashed border-gray-700">
                        <div class="w-20 h-20 mx-auto mb-4 bg-gray-700/50 rounded-2xl flex items-center justify-center">
                            <svg class="w-12 h-12 text-gray-500 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                            </svg>
                        </div>
                        <h3 class="text-xl font-bold text-gray-100 mb-2">No OAuth Clients Found</h3>
                        <p class="text-gray-400 mb-6">
                            {{ clientSearchTerm || clientFilter !== 'all'
                                ? 'Try adjusting your search or filters'
                                : 'Get started by registering your first OAuth client' }}
                        </p>
                        <button
                            v-if="!clientSearchTerm && clientFilter === 'all'"
                            @click="showCreateClient = true"
                            class="inline-flex items-center px-6 py-3 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-xl hover:from-green-700 hover:to-emerald-700 transition-all duration-200 font-semibold shadow-lg shadow-green-600/30 hover:shadow-xl hover:shadow-green-600/40 hover:scale-105"
                        >
                            <svg class="w-5 h-5 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('plus')"></path>
                            </svg>
                            Register First Client
                        </button>
                    </div>

                    <!-- Enhanced Clients Grid -->
                    <div v-else class="grid grid-cols-1 lg:grid-cols-2 xl:grid-cols-3 gap-5">
                        <div
                            v-for="client in filteredClients"
                            :key="client.client_id"
                            class="bg-gradient-to-br from-gray-800 to-gray-800/80 border-2 border-gray-700 rounded-xl p-5 hover:border-blue-500/50 hover:shadow-xl hover:shadow-blue-500/10 transition-all duration-300 hover:scale-[1.02]"
                        >
                            <div class="space-y-4">
                                <div class="flex items-start justify-between">
                                    <div class="flex-1 min-w-0">
                                        <h5 class="font-bold text-gray-100 truncate text-lg">{{ client.name }}</h5>
                                        <p v-if="client.description" class="text-xs text-gray-400 mt-1 line-clamp-2">
                                            {{ client.description }}
                                        </p>
                                    </div>
                                    <span :class="[
                                        'flex-shrink-0 inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ml-2 shadow-lg',
                                        client.public
                                            ? 'bg-blue-500/20 text-blue-300 border-2 border-blue-500/50'
                                            : 'bg-orange-500/20 text-orange-300 border-2 border-orange-500/50'
                                    ]">
                                        <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(client.public ? 'users' : 'key')"></path>
                                        </svg>
                                        {{ client.public ? 'Public' : 'Confidential' }}
                                    </span>
                                </div>

                                <div>
                                    <label class="block text-xs font-bold text-gray-400 mb-2">Client ID</label>
                                    <code class="text-xs bg-gray-900 text-gray-300 px-3 py-2 rounded-lg break-all block border border-gray-700">
                                        {{ client.client_id }}
                                    </code>
                                </div>

                                <div v-if="client.scope">
                                    <label class="block text-xs font-bold text-gray-400 mb-2">Scopes</label>
                                    <div class="flex flex-wrap gap-2">
                                        <span
                                            v-for="scope in (client.scope || '').split(' ').filter(s => s)"
                                            :key="scope"
                                            class="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-semibold bg-gray-700 text-gray-200 border border-gray-600"
                                        >
                                            {{ scope }}
                                        </span>
                                    </div>
                                </div>

                                <div class="flex flex-wrap gap-2 pt-3 border-t border-gray-700">
                                    <button
                                        @click="viewClientDetails(client)"
                                        class="flex items-center px-3 py-2 text-blue-400 hover:text-blue-300 hover:bg-blue-500/10 rounded-lg text-xs touch-target transition-all duration-200 font-semibold"
                                    >
                                        <svg class="w-4 h-4 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('eye')"></path>
                                        </svg>
                                        View
                                    </button>
                                    <button
                                        @click="deleteClient(client.client_id, client.name)"
                                        class="flex items-center px-3 py-2 text-red-400 hover:text-red-300 hover:bg-red-500/10 rounded-lg text-xs touch-target transition-all duration-200 font-semibold"
                                    >
                                        <svg class="w-4 h-4 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('trash')"></path>
                                        </svg>
                                        Delete
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Test OAuth Flow -->
                <div class="enhanced-card p-6 bg-gradient-to-br from-gray-800 via-gray-800 to-green-900/20 border-green-500/20">
                    <div class="flex items-center space-x-4 mb-6">
                        <div class="w-12 h-12 bg-gradient-to-br from-green-500 to-emerald-600 rounded-xl flex items-center justify-center shadow-lg shadow-green-500/30">
                            <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play')"></path>
                            </svg>
                        </div>
                        <div>
                            <h4 class="text-lg font-bold text-gray-100">Test OAuth Flow</h4>
                            <p class="text-sm text-gray-400">Test authentication flows with your clients</p>
                        </div>
                    </div>

                    <div class="space-y-5">
                        <div>
                            <label class="block text-sm font-bold text-gray-300 mb-3">Test Client</label>
                            <select v-model="selectedTestClient" class="form-input w-full h-12 rounded-xl border-2 border-gray-600 focus:border-green-500 transition-colors duration-200 font-medium">
                                <option :value="null">Select a client to test</option>
                                <option v-for="client in clients" :key="client.client_id" :value="client">
                                    {{ client.name }} ({{ client.public ? 'Public' : 'Confidential' }})
                                </option>
                            </select>
                        </div>

                        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <button
                                @click="testAuthFlow"
                                :disabled="!selectedTestClient"
                                class="flex items-center justify-center px-5 py-3.5 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-xl hover:from-green-700 hover:to-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold touch-target transition-all duration-200 shadow-lg shadow-green-600/30 hover:shadow-xl hover:shadow-green-600/40 disabled:hover:shadow-lg"
                            >
                                <svg class="w-5 h-5 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('play')"></path>
                                </svg>
                                Test Authorization Flow
                            </button>

                            <button
                                @click="testClientCredentials"
                                :disabled="!selectedTestClient || selectedTestClient?.public"
                                class="flex items-center justify-center px-5 py-3.5 bg-gradient-to-r from-blue-600 to-blue-700 text-white rounded-xl hover:from-blue-700 hover:to-blue-800 disabled:opacity-50 disabled:cursor-not-allowed text-sm font-bold touch-target transition-all duration-200 shadow-lg shadow-blue-600/30 hover:shadow-xl hover:shadow-blue-600/40 disabled:hover:shadow-lg"
                            >
                                <svg class="w-5 h-5 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('key')"></path>
                                </svg>
                                Test Client Credentials
                            </button>
                        </div>

                        <div v-if="selectedTestClient?.public" class="text-xs text-yellow-300 bg-yellow-500/10 p-4 rounded-xl border-2 border-yellow-500/30 backdrop-blur-sm">
                            <strong class="font-bold">Note:</strong> Client credentials flow is only available for confidential clients.
                            Public clients should use the authorization code flow.
                        </div>
                    </div>
                </div>
            </div>

            <!-- Enhanced Create Client Modal -->
            <div v-if="showCreateClient" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4 overflow-y-auto animate-fade-in">
                <div class="bg-gradient-to-br from-gray-800 to-gray-900 border-2 border-gray-700 rounded-2xl max-w-lg w-full shadow-2xl transform transition-all duration-300 scale-100">
                    <div class="flex items-center justify-between p-6 border-b-2 border-gray-700">
                        <h3 class="text-xl font-bold text-gray-100">Register New OAuth Client</h3>
                        <button
                            @click="showCreateClient = false; resetNewClient()"
                            class="text-gray-400 hover:text-gray-200 transition-colors touch-target p-2 hover:bg-gray-700 rounded-lg"
                        >
                            <svg class="w-6 h-6 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x')"></path>
                            </svg>
                        </button>
                    </div>

                    <form @submit.prevent="createClient" class="p-6">
                        <div class="space-y-5">
                            <div>
                                <label class="block text-sm font-bold text-gray-300 mb-2">Client Name *</label>
                                <input
                                    v-model="newClient.name"
                                    type="text"
                                    required
                                    class="form-input w-full h-12 rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200"
                                    placeholder="My Application"
                                >
                            </div>

                            <div>
                                <label class="block text-sm font-bold text-gray-300 mb-2">Description</label>
                                <input
                                    v-model="newClient.description"
                                    type="text"
                                    class="form-input w-full h-12 rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200"
                                    placeholder="Brief description of your application"
                                >
                            </div>

                            <div>
                                <label class="block text-sm font-bold text-gray-300 mb-2">Redirect URIs *</label>
                                <textarea
                                    v-model="newClient.redirect_uris"
                                    rows="3"
                                    required
                                    class="form-input w-full rounded-xl border-2 border-gray-600 focus:border-blue-500 transition-colors duration-200"
                                    placeholder="https://yourapp.com/oauth/callback&#10;http://localhost:3000/callback"
                                ></textarea>
                                <p class="text-xs text-gray-400 mt-2">One URI per line</p>
                            </div>

                            <div class="flex items-center bg-gray-700/50 p-4 rounded-xl">
                                <input
                                    v-model="newClient.public"
                                    type="checkbox"
                                    id="publicClient"
                                    class="form-checkbox h-5 w-5 text-blue-600 rounded-lg"
                                >
                                <label for="publicClient" class="ml-3 text-sm text-gray-300 font-medium">
                                    Public Client (mobile apps, SPAs - no client secret)
                                </label>
                            </div>
                        </div>

                        <div class="flex flex-col sm:flex-row justify-end gap-3 mt-6 pt-6 border-t-2 border-gray-700">
                            <button
                                type="button"
                                @click="showCreateClient = false; resetNewClient()"
                                class="w-full sm:w-auto px-5 py-3 border-2 border-gray-600 text-gray-300 bg-gray-700 rounded-xl hover:bg-gray-600 hover:border-gray-500 transition-all duration-200 touch-target font-semibold"
                            >
                                Cancel
                            </button>
                            <button
                                type="submit"
                                :disabled="creating || !newClient.name.trim()"
                                class="w-full sm:w-auto px-5 py-3 bg-gradient-to-r from-green-600 to-emerald-600 text-white rounded-xl hover:from-green-700 hover:to-emerald-700 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200 touch-target font-bold shadow-lg shadow-green-600/30 hover:shadow-xl hover:shadow-green-600/40"
                            >
                                <span v-if="creating" class="flex items-center justify-center">
                                    <svg class="animate-spin -ml-1 mr-2 h-5 w-5 text-white" fill="none" viewBox="0 0 24 24">
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

            <!-- Enhanced Client Details Modal -->
            <div v-if="showClientDetails" class="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center z-50 p-4 overflow-y-auto animate-fade-in">
                <div class="bg-gradient-to-br from-gray-800 to-gray-900 border-2 border-gray-700 rounded-2xl max-w-2xl w-full shadow-2xl transform transition-all duration-300 scale-100">
                    <div class="flex items-center justify-between p-6 border-b-2 border-gray-700">
                        <h3 class="text-xl font-bold text-gray-100">Client Details</h3>
                        <button
                            @click="showClientDetails = null"
                            class="text-gray-400 hover:text-gray-200 transition-colors touch-target p-2 hover:bg-gray-700 rounded-lg"
                        >
                            <svg class="w-6 h-6 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x')"></path>
                            </svg>
                        </button>
                    </div>

                    <div class="p-6 space-y-5">
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-5">
                            <div>
                                <label class="block text-sm font-bold text-gray-400 mb-2">Name</label>
                                <p class="text-gray-100 text-lg font-semibold">{{ showClientDetails.name }}</p>
                            </div>
                            <div>
                                <label class="block text-sm font-bold text-gray-400 mb-2">Type</label>
                                <span :class="[
                                    'inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold shadow-lg',
                                    showClientDetails.public
                                        ? 'bg-blue-500/20 text-blue-300 border-2 border-blue-500/50'
                                        : 'bg-orange-500/20 text-orange-300 border-2 border-orange-500/50'
                                ]">
                                    {{ showClientDetails.public ? 'Public' : 'Confidential' }}
                                </span>
                            </div>
                        </div>

                        <div v-if="showClientDetails.description">
                            <label class="block text-sm font-bold text-gray-400 mb-2">Description</label>
                            <p class="text-gray-100">{{ showClientDetails.description }}</p>
                        </div>

                        <div>
                            <label class="block text-sm font-bold text-gray-400 mb-2">Client ID</label>
                            <div class="flex rounded-xl overflow-hidden border-2 border-gray-600">
                                <code class="flex-1 px-4 py-3 bg-gray-900 text-sm break-all text-gray-200 font-mono">
                                    {{ showClientDetails.client_id }}
                                </code>
                                <button
                                    @click="copyToClipboard(showClientDetails.client_id)"
                                    class="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm touch-target transition-all duration-200"
                                >
                                    <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                    </svg>
                                </button>
                            </div>
                        </div>

                        <div v-if="!showClientDetails.public && showClientDetails.client_secret">
                            <label class="block text-sm font-bold text-gray-400 mb-2">Client Secret</label>
                            <div class="flex rounded-xl overflow-hidden border-2 border-gray-600">
                                <code class="flex-1 px-4 py-3 bg-gray-900 text-sm break-all text-gray-200 font-mono">
                                    {{ showClientDetails.client_secret }}
                                </code>
                                <button
                                    @click="copyToClipboard(showClientDetails.client_secret)"
                                    class="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm touch-target transition-all duration-200"
                                >
                                    <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clipboard-copy')"></path>
                                    </svg>
                                </button>
                            </div>
                            <p class="text-xs text-yellow-300 mt-2 bg-yellow-500/10 p-3 rounded-lg border border-yellow-500/30 font-semibold">Store this secret securely. It won't be shown again.</p>
                        </div>

                        <div v-if="showClientDetails.redirect_uris?.length">
                            <label class="block text-sm font-bold text-gray-400 mb-3">Redirect URIs</label>
                            <div class="space-y-2">
                                <code
                                    v-for="uri in showClientDetails.redirect_uris"
                                    :key="uri"
                                    class="block px-4 py-3 bg-gray-900 border-2 border-gray-700 rounded-xl text-sm break-all text-gray-200 font-mono"
                                >
                                    {{ uri }}
                                </code>
                            </div>
                        </div>

                        <div v-if="showClientDetails.scope">
                            <label class="block text-sm font-bold text-gray-400 mb-3">Scopes</label>
                            <div class="flex flex-wrap gap-2">
                                <span
                                    v-for="scope in showClientDetails.scope.split(' ').filter(s => s)"
                                    :key="scope"
                                    class="inline-flex items-center px-3 py-1.5 rounded-lg text-xs font-bold bg-gray-700 text-gray-200 border-2 border-gray-600"
                                >
                                    {{ scope }}
                                </span>
                            </div>
                        </div>
                    </div>

                    <div class="flex justify-end gap-3 p-6 border-t-2 border-gray-700">
                        <button
                            @click="showClientDetails = null"
                            class="px-5 py-3 border-2 border-gray-600 text-gray-300 bg-gray-700 rounded-xl hover:bg-gray-600 hover:border-gray-500 transition-all duration-200 touch-target font-semibold"
                        >
                            Close
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `
};