const ServerOAuthConfig = {
    template: `
        <div class="server-oauth-config max-w-full overflow-x-hidden">
            <div class="enhanced-card">
                <div class="p-4 md:p-6">
                    <h3 class="text-xl font-semibold text-white mb-4">🖥️ Server OAuth Settings</h3>
                    <p class="text-gray-400 mb-6">Configure OAuth authentication requirements for each MCP server</p>
                    
                    <div v-if="loading" class="flex justify-center py-8">
                        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400"></div>
                    </div>
                    
                    <div v-else-if="Object.keys(servers).length === 0" class="text-center py-8 text-gray-400">
                        No servers configured
                    </div>
                    
                    <div v-else class="space-y-6">
                        <div v-for="(server, name) in servers" :key="name" 
                             class="bg-gray-800 rounded-lg p-4 md:p-6 border border-gray-700">
                            
                            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-4 gap-3">
                                <div class="flex items-center gap-3">
                                    <div :class="[
                                        'w-3 h-3 rounded-full border border-current',
                                        server.containerStatus === 'running' ? 'bg-green-500 border-green-400' : 'bg-gray-500 border-gray-400'
                                    ]"></div>
                                    <div>
                                        <h4 class="text-lg font-medium text-white">{{ name }}</h4>
                                        <span class="text-sm text-gray-400">
                                            {{ server.configProtocol || 'stdio' }}
                                            {{ server.configHttpPort ? ':' + server.configHttpPort : '' }}
                                        </span>
                                    </div>
                                </div>
                                <div class="flex items-center gap-2 self-start sm:self-center">
                                    <span :class="[
                                        'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border',
                                        server.containerStatus === 'running' 
                                            ? 'bg-green-900 text-green-200 border-green-700' 
                                            : 'bg-gray-800 text-gray-300 border-gray-600'
                                    ]">
                                        {{ server.containerStatus || 'Unknown' }}
                                    </span>
                                </div>
                            </div>
                            
                            <div class="space-y-4">
                                <!-- OAuth Toggle -->
                                <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                                    <div>
                                        <label class="text-sm font-medium text-gray-300">OAuth Required</label>
                                        <p class="text-xs text-gray-400">Require OAuth authentication for this server</p>
                                    </div>
                                    <label class="relative inline-flex items-center cursor-pointer">
                                        <input type="checkbox" :checked="getOAuthConfig(name).enabled" 
                                               @change="updateOAuthEnabled(name, $event.target.checked)" class="sr-only peer">
                                        <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                    </label>
                                </div>
                                
                                <!-- OAuth Settings (shown when enabled) -->
                                <div v-if="getOAuthConfig(name).enabled" class="space-y-4 pl-4 border-l-2 border-blue-600">
                                    <!-- Required Scope -->
                                    <div>
                                        <label class="block text-sm font-medium text-gray-300 mb-2">Required Scope</label>
                                        <select :value="getOAuthConfig(name).required_scope" @change="updateRequiredScope(name, $event.target.value)" 
                                                class="w-full px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                                style="font-size: 16px;">
                                            <option value="">No specific scope required</option>
                                            <option v-for="scope in scopes" :key="scope.name" :value="scope.name">
                                                {{ scope.name }} - {{ scope.description }}
                                            </option>
                                        </select>
                                    </div>
                                    
                                    <!-- API Key Fallback -->
                                    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                                        <div>
                                            <label class="text-sm font-medium text-gray-300">Allow API Key Fallback</label>
                                            <p class="text-xs text-gray-400">Allow traditional API key when OAuth fails</p>
                                        </div>
                                        <label class="relative inline-flex items-center cursor-pointer">
                                            <input type="checkbox" :checked="getOAuthConfig(name).allow_api_key_fallback" 
                                                   @change="updateAPIKeyFallback(name, $event.target.checked)" class="sr-only peer">
                                            <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                        </label>
                                    </div>
                                    
                                    <!-- Optional Auth -->
                                    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
                                        <div>
                                            <label class="text-sm font-medium text-gray-300">Optional Authentication</label>
                                            <p class="text-xs text-gray-400">Allow unauthenticated access to this server</p>
                                        </div>
                                        <label class="relative inline-flex items-center cursor-pointer">
                                            <input type="checkbox" :checked="getOAuthConfig(name).optional_auth" 
                                                   @change="updateOptionalAuth(name, $event.target.checked)" class="sr-only peer">
                                            <div class="w-11 h-6 bg-gray-600 peer-focus:outline-none peer-focus:ring-4 peer-focus:ring-blue-800 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all border-gray-500 peer-checked:bg-blue-600"></div>
                                        </label>
                                    </div>
                                    
                                    <!-- Allowed Clients -->
                                    <div>
                                        <label class="block text-sm font-medium text-gray-300 mb-2">Allowed Clients</label>
                                        <div class="max-h-32 overflow-y-auto border border-gray-600 rounded-md p-3 bg-gray-700" style="-webkit-overflow-scrolling: touch;">
                                            <div v-if="clients.length === 0" class="text-sm text-gray-400 text-center py-2">
                                                No OAuth clients available
                                            </div>
                                            <label v-for="client in clients" :key="client.client_id" class="flex items-start gap-2 mb-2 last:mb-0 cursor-pointer">
                                                <input type="checkbox" :value="client.client_id" 
                                                       :checked="isClientAllowed(name, client.client_id)"
                                                       @change="updateAllowedClients(name, client.client_id, $event.target.checked)"
                                                       class="rounded border-gray-500 bg-gray-600 text-blue-600 focus:ring-blue-500 mt-1 flex-shrink-0">
                                                <div class="min-w-0 flex-1">
                                                    <span class="text-sm font-medium text-gray-300 block">{{ client.name }}</span>
                                                    <code class="text-xs text-gray-400 break-all">{{ client.client_id }}</code>
                                                </div>
                                            </label>
                                        </div>
                                    </div>
                                </div>
                                
                                <!-- Action Buttons -->
                                <div class="flex flex-col sm:flex-row gap-3 pt-4 border-t border-gray-700">
                                    <button @click="testServerAccess(name)" 
                                            class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500 text-sm touch-target transition-colors">
                                        🧪 Test Access
                                    </button>
                                    <button @click="viewServerTokens(name)" 
                                            class="w-full sm:w-auto px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 focus:outline-none focus:ring-2 focus:ring-gray-500 text-sm touch-target transition-colors">
                                        🎫 View Tokens
                                    </button>
                                    <button @click="saveServerConfig(name)" :disabled="saving" 
                                            class="w-full sm:w-auto px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 focus:outline-none focus:ring-2 focus:ring-green-500 disabled:opacity-50 text-sm touch-target transition-colors">
                                        {{ saving ? '💾 Saving...' : '💾 Save' }}
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `,
    data() {
        return {
            servers: {},
            scopes: [],
            clients: [],
            serverOAuthConfigs: {},
            loading: false,
            saving: false
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        async loadData() {
            this.loading = true;
            try {
                const [serversRes, scopesRes, clientsRes] = await Promise.all([
                    fetch('/api/servers'),
                    fetch('/api/oauth/scopes').catch(() => ({ ok: false })),
                    fetch('/api/oauth/clients').catch(() => ({ ok: false }))
                ]);
                
                if (serversRes.ok) this.servers = await serversRes.json();
                
                if (scopesRes.ok && scopesRes.headers?.get('content-type')?.includes('application/json')) {
                    this.scopes = await scopesRes.json();
                } else {
                    console.warn('OAuth scopes endpoint not available');
                    this.scopes = [];
                }
                
                if (clientsRes.ok && clientsRes.headers?.get('content-type')?.includes('application/json')) {
                    this.clients = await clientsRes.json();
                } else {
                    console.warn('OAuth clients endpoint not available');
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
                console.error('Failed to load server OAuth data:', error);
                this.$emit('show-toast', { message: 'Some OAuth endpoints not available', type: 'warning' });
            } finally {
                this.loading = false;
            }
        },
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
        async testServerAccess(serverName) {
            try {
                const response = await fetch(`/api/servers/${serverName}/test-oauth`);
                if (response.ok) {
                    const result = await response.json();
                    if (result.success) {
                        this.$emit('show-toast', {
                            message: `OAuth test successful for ${serverName}`,
                            type: 'success'
                        });
                    } else {
                        this.$emit('show-toast', {
                            message: `OAuth test failed for ${serverName}: ${result.error}`,
                            type: 'error'
                        });
                    }
                } else {
                    throw new Error('Test endpoint not available');
                }
            } catch (error) {
                this.$emit('show-toast', {
                    message: `Failed to test OAuth for ${serverName}: ${error.message}`,
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
        }
    }
};