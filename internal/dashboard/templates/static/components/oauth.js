const OAuthConfig = {
    template: `
        <div class="oauth-config max-w-full overflow-x-hidden">
            <div class="enhanced-card">
                <div class="p-4 md:p-6">
                    <h3 class="text-xl font-semibold text-white mb-4">🔐 OAuth 2.1 Configuration</h3>
                    
                    <div v-if="loading" class="flex justify-center py-8">
                        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400"></div>
                    </div>
                    
                    <div v-else class="space-y-4 md:space-y-6">
                        <!-- OAuth Status -->
                        <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-4 gap-2">
                                <h4 class="text-lg font-medium text-white">OAuth Server Status</h4>
                                <span :class="[
                                    'inline-flex items-center px-3 py-1 rounded-full text-sm font-medium',
                                    oauthStatus.oauth_enabled ? 'bg-green-900 text-green-200 border border-green-700' : 'bg-red-900 text-red-200 border border-red-700'
                                ]">
                                    {{ oauthStatus.oauth_enabled ? 'Enabled' : 'Disabled' }}
                                </span>
                            </div>
                            
                            <div v-if="oauthStatus.oauth_enabled" class="grid grid-cols-1 sm:grid-cols-3 gap-4">
                                <div class="text-center p-3 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-blue-400">
                                        {{ oauthStatus.active_tokens?.access_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-400">Access Tokens</div>
                                </div>
                                <div class="text-center p-3 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-green-400">
                                        {{ oauthStatus.active_tokens?.refresh_tokens || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-400">Refresh Tokens</div>
                                </div>
                                <div class="text-center p-3 bg-gray-700 rounded-lg border border-gray-600">
                                    <div class="text-2xl font-bold text-yellow-400">
                                        {{ oauthStatus.active_tokens?.auth_codes || 0 }}
                                    </div>
                                    <div class="text-sm text-gray-400">Auth Codes</div>
                                </div>
                            </div>
                            
                            <div v-if="oauthStatus.issuer" class="mt-4">
                                <label class="block text-sm font-medium text-gray-300 mb-1">Issuer URL</label>
                                <div class="flex">
                                    <code class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                        {{ oauthStatus.issuer }}
                                    </code>
                                    <button @click="copyToClipboard(oauthStatus.issuer)" class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target">
                                        📋
                                    </button>
                                </div>
                            </div>
                        </div>

                        <!-- OAuth Endpoints -->
                        <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                            <h4 class="text-lg font-medium text-white mb-4">📍 OAuth Endpoints</h4>
                            <div class="space-y-3">
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-1">Authorization</label>
                                    <div class="flex">
                                        <code class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                            {{ baseUrl }}/oauth/authorize
                                        </code>
                                        <button @click="copyToClipboard(baseUrl + '/oauth/authorize')" class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target">
                                            📋
                                        </button>
                                    </div>
                                </div>
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-1">Token</label>
                                    <div class="flex">
                                        <code class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                            {{ baseUrl }}/oauth/token
                                        </code>
                                        <button @click="copyToClipboard(baseUrl + '/oauth/token')" class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target">
                                            📋
                                        </button>
                                    </div>
                                </div>
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-1">Discovery</label>
                                    <div class="flex">
                                        <code class="flex-1 px-3 py-2 bg-gray-700 border border-gray-600 rounded-l-md text-sm break-all text-gray-200">
                                            {{ baseUrl }}/.well-known/oauth-authorization-server
                                        </code>
                                        <button @click="copyToClipboard(baseUrl + '/.well-known/oauth-authorization-server')" class="px-3 py-2 bg-blue-600 text-white border border-blue-600 rounded-r-md hover:bg-blue-700 text-sm touch-target">
                                            📋
                                        </button>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- OAuth Clients -->
                        <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                            <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between mb-4 gap-3">
                                <h4 class="text-lg font-medium text-white">👥 OAuth Clients</h4>
                                <button @click="showCreateClient = true" class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 text-sm touch-target">
                                    ➕ Register Client
                                </button>
                            </div>
                            
                            <div v-if="clients.length === 0" class="text-center py-8 text-gray-400">
                                No OAuth clients registered
                            </div>
                            
                            <div v-else class="overflow-x-auto -mx-4 sm:mx-0">
                                <div class="inline-block min-w-full align-middle">
                                    <div class="overflow-hidden">
                                        <table class="min-w-full bg-gray-800">
                                            <thead>
                                                <tr class="border-b border-gray-700">
                                                    <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Name</th>
                                                    <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Client ID</th>
                                                    <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Type</th>
                                                    <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Scopes</th>
                                                    <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Actions</th>
                                                </tr>
                                            </thead>
                                            <tbody>
                                                <tr v-for="client in clients" :key="client.client_id" class="border-b border-gray-700">
                                                    <td class="py-4 px-4">
                                                        <div class="font-medium text-white text-sm">{{ client.name }}</div>
                                                        <div class="text-xs text-gray-400 break-words max-w-32">{{ client.description }}</div>
                                                    </td>
                                                    <td class="py-4 px-4">
                                                        <code class="text-xs bg-gray-700 text-gray-200 px-2 py-1 rounded break-all">
                                                            {{ client.client_id }}
                                                        </code>
                                                    </td>
                                                    <td class="py-4 px-4">
                                                        <span :class="[
                                                            'inline-flex items-center px-2 py-1 rounded-full text-xs font-medium',
                                                            client.public ? 'bg-blue-900 text-blue-200 border border-blue-700' : 'bg-orange-900 text-orange-200 border border-orange-700'
                                                        ]">
                                                            {{ client.public ? 'Public' : 'Confidential' }}
                                                        </span>
                                                    </td>
                                                    <td class="py-4 px-4">
                                                        <div class="flex flex-wrap gap-1">
                                                            <span v-for="scope in (client.scope || '').split(' ').filter(s => s)" :key="scope"
                                                                  class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-700 text-gray-200 border border-gray-600">
                                                                {{ scope }}
                                                            </span>
                                                        </div>
                                                    </td>
                                                    <td class="py-4 px-4">
                                                        <div class="flex flex-col sm:flex-row gap-2">
                                                            <button @click="viewClient(client)" class="text-blue-400 hover:text-blue-200 text-sm touch-target">
                                                                View
                                                            </button>
                                                            <button @click="deleteClient(client.client_id)" class="text-red-400 hover:text-red-200 text-sm touch-target">
                                                                Delete
                                                            </button>
                                                        </div>
                                                    </td>
                                                </tr>
                                            </tbody>
                                        </table>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Test OAuth Flow -->
                        <div class="bg-gray-800 rounded-lg p-4 border border-gray-700">
                            <h4 class="text-lg font-medium text-white mb-4">🧪 Test OAuth Flow</h4>
                            <div class="space-y-4">
                                <div>
                                    <label class="block text-sm font-medium text-gray-300 mb-1">Test Client</label>
                                    <select v-model="selectedTestClient" class="w-full px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" style="font-size: 16px;">
                                        <option value="">Select a client to test</option>
                                        <option v-for="client in clients" :key="client.client_id" :value="client">
                                            {{ client.name }} ({{ client.client_id }})
                                        </option>
                                    </select>
                                </div>
                                <div class="flex flex-col sm:flex-row gap-3">
                                    <button @click="testAuthFlow" :disabled="!selectedTestClient" class="w-full sm:w-auto px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50 text-sm touch-target">
                                        🚀 Test Authorization Flow
                                    </button>
                                    <button @click="testClientCredentials" :disabled="!selectedTestClient" class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 text-sm touch-target">
                                        🔑 Test Client Credentials
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Create Client Modal -->
            <div v-if="showCreateClient" class="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4">
                <div class="bg-gray-800 border border-gray-700 rounded-lg max-w-lg w-full max-h-[90vh] overflow-y-auto">
                    <div class="flex items-center justify-between p-4 border-b border-gray-700">
                        <h3 class="text-lg font-medium text-white">Register New OAuth Client</h3>
                        <button @click="showCreateClient = false" class="text-gray-400 hover:text-gray-200 text-2xl touch-target">&times;</button>
                    </div>
                    
                    <form @submit.prevent="createClient" class="p-4">
                        <div class="space-y-4">
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-1">Client Name</label>
                                <input v-model="newClient.name" type="text" required 
                                       class="w-full px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                       style="font-size: 16px;" placeholder="My Application">
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-1">Description</label>
                                <input v-model="newClient.description" type="text" 
                                       class="w-full px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                       style="font-size: 16px;" placeholder="Description of the client">
                            </div>
                            
                            <div>
                                <label class="block text-sm font-medium text-gray-300 mb-1">Redirect URIs (one per line)</label>
                                <textarea v-model="newClient.redirect_uris" rows="3" 
                                          class="w-full px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                          style="font-size: 16px;" placeholder="http://localhost:3000/callback"></textarea>
                            </div>
                            
                            <div class="flex items-center">
                                <input v-model="newClient.public" type="checkbox" id="publicClient" 
                                       class="rounded border-gray-600 bg-gray-700 text-blue-600 focus:ring-blue-500">
                                <label for="publicClient" class="ml-2 text-sm text-gray-300">
                                    Public Client (no client secret)
                                </label>
                            </div>
                        </div>
                        
                        <div class="flex flex-col sm:flex-row justify-end gap-3 mt-6 pt-4 border-t border-gray-700">
                            <button type="button" @click="showCreateClient = false" 
                                    class="w-full sm:w-auto px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-md hover:bg-gray-600 touch-target">
                                Cancel
                            </button>
                            <button type="submit" :disabled="creating" 
                                    class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 touch-target">
                                {{ creating ? 'Creating...' : 'Create Client' }}
                            </button>
                        </div>
                    </form>
                </div>
            </div>
        </div>
    `,
    data() {
        return {
            loading: false,
            oauthStatus: { active_tokens: {} },
            clients: [],
            selectedTestClient: null,
            showCreateClient: false,
            creating: false,
            newClient: {
                name: '',
                description: '',
                redirect_uris: `${window.location.origin}/oauth/callback`,
                public: true
            },
            baseUrl: window.location.origin
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        async loadData() {
            this.loading = true;
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
                console.error('Failed to load OAuth data:', error);
                this.oauthStatus = { oauth_enabled: false, active_tokens: {} };
                this.clients = [];
                this.$emit('show-toast', { message: 'OAuth endpoints not available', type: 'warning' });
            } finally {
                this.loading = false;
            }
        },
        async createClient() {
            this.creating = true;
            try {
                const clientData = {
                    client_name: this.newClient.name,
                    client_description: this.newClient.description,
                    redirect_uris: this.newClient.redirect_uris.split('\n').filter(uri => uri.trim()),
                    grant_types: this.newClient.public ? ['authorization_code', 'refresh_token'] : ['authorization_code', 'client_credentials', 'refresh_token'],
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
                    this.newClient = { 
                        name: '', 
                        description: '', 
                        redirect_uris: `${window.location.origin}/oauth/callback`, 
                        public: true 
                    };
                    this.$emit('show-toast', { message: 'OAuth client created successfully', type: 'success' });
                } else {
                    const errorText = await response.text();
                    throw new Error(`Failed to create client: ${response.status} - ${errorText}`);
                }
            } catch (error) {
                this.$emit('show-toast', { message: `Failed to create client: ${error.message}`, type: 'error' });
            } finally {
                this.creating = false;
            }
        },
        async deleteClient(clientId) {
            if (!confirm('Delete this OAuth client? This cannot be undone.')) return;
            try {
                const response = await fetch(`/api/oauth/clients/${clientId}`, { method: 'DELETE' });
                if (response.ok) {
                    this.clients = this.clients.filter(c => c.client_id !== clientId);
                    this.$emit('show-toast', { message: 'Client deleted successfully', type: 'success' });
                } else {
                    throw new Error('Failed to delete client');
                }
            } catch (error) {
                this.$emit('show-toast', { message: `Failed to delete client: ${error.message}`, type: 'error' });
            }
        },
        viewClient(client) {
            const details = `Client Details:
Name: ${client.name}
ID: ${client.client_id}
Type: ${client.public ? 'Public' : 'Confidential'}
${!client.public && client.client_secret ? 'Secret: ' + client.client_secret : ''}
Redirect URIs: 
${client.redirect_uris?.join('\n') || 'None'}
Scopes: ${client.scope || 'None'}`;
            alert(details);
        },
        testAuthFlow() {
            if (!this.selectedTestClient) return;
            const authUrl = `/oauth/authorize?response_type=code&client_id=${this.selectedTestClient.client_id}&redirect_uri=${encodeURIComponent(this.selectedTestClient.redirect_uris[0])}&scope=mcp:tools`;
            window.open(authUrl, '_blank');
        },
        async testClientCredentials() {
            if (!this.selectedTestClient || this.selectedTestClient.public) {
                this.$emit('show-toast', { message: 'Client credentials flow requires a confidential client', type: 'error' });
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
                    this.$emit('show-toast', { message: 'Client credentials flow successful!', type: 'success' });
                    console.log('Token:', token);
                } else {
                    const errorText = await response.text();
                    throw new Error(`Token request failed: ${response.status} - ${errorText}`);
                }
            } catch (error) {
                this.$emit('show-toast', { message: `Client credentials test failed: ${error.message}`, type: 'error' });
            }
        },
        copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                this.$emit('show-toast', { message: 'Copied to clipboard!', type: 'success' });
            }).catch(err => {
                this.$emit('show-toast', { message: 'Failed to copy to clipboard', type: 'error' });
            });
        }
    }
};