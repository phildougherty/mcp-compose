// /static/components/inspector.js
const MCPInspector = {
    props: ['serverName', 'serverConfig', 'isExpanded'],
    emits: ['tools-discovered'],
    data() {
        return {
            session: null,
            loading: false,
            connected: false,
            error: null,
            response: null,
            request: '',
            availableMethods: [],
            discoveredTools: [],
            inspectorAvailable: null, // null = unknown, true = available, false = not available
            requestTemplates: {
                'initialize': {
                    method: 'initialize',
                    params: {
                        protocolVersion: '2024-11-05',
                        capabilities: {
                            resources: { listChanged: true, subscribe: true },
                            tools: { listChanged: true },
                            prompts: { listChanged: true }
                        },
                        clientInfo: {
                            name: 'MCP Dashboard Inspector',
                            version: '1.0.0'
                        }
                    }
                },
                'tools/list': { method: 'tools/list', params: {} },
                'tools/get': {
                    method: 'tools/get',
                    params: { tool: 'example_tool', parameters: {} }
                },
                'resources/list': { method: 'resources/list', params: {} },
                'resources/get': {
                    method: 'resources/get',
                    params: { path: '/example' }
                },
                'prompts/list': { method: 'prompts/list', params: {} },
                'prompts/render': {
                    method: 'prompts/render',
                    params: { name: 'example_prompt', variables: {} }
                }
            }
        }
    },
    watch: {
        isExpanded: {
            handler(newVal) {
                if (newVal && !this.connected) {
                    if (this.inspectorAvailable === true) {
                        this.connect();
                    } else if (this.inspectorAvailable === null) {
                        this.checkInspectorAvailability().then(() => {
                            if (this.inspectorAvailable === true && this.isExpanded) {
                                this.connect();
                            }
                        });
                    }
                }
            },
            immediate: false
        }
    },
    computed: {
        isHealthy() {
            return this.connected && this.session;
        }
    },
    async mounted() {
        await this.checkInspectorAvailability();
        if (this.inspectorAvailable === true && this.isExpanded) {
            await this.connect();
        }
    },    
    beforeUnmount() {
        this.disconnect();
    },
    methods: {
        async checkInspectorAvailability() {
            try {
                // First check if basic API works
                const response = await fetch('/api/servers', { method: 'GET' });
                if (!response.ok) {
                    this.inspectorAvailable = false;
                    return;
                }
                // Test the inspector health check endpoint
                const testResponse = await fetch('/api/inspector/connect', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ server: '__healthcheck__' })
                });
                const contentType = testResponse.headers.get('content-type');
                if (contentType && contentType.includes('application/json')) {
                    this.inspectorAvailable = true;
                } else {
                    this.inspectorAvailable = false;
                }
            } catch (err) {
                console.warn('Inspector availability check failed:', err);
                this.inspectorAvailable = false;
            }
        },
        async connect() {
            if (this.connected || this.inspectorAvailable === false) return;
            if (this.inspectorAvailable === null) {
                await this.checkInspectorAvailability();
                if (this.inspectorAvailable === false) {
                    return;
                }
            }
            this.loading = true;
            this.error = null;
            try {
                const response = await fetch('/api/inspector/connect', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ server: this.serverName })
                });
                const contentType = response.headers.get('content-type');
                if (!contentType || !contentType.includes('application/json')) {
                    this.inspectorAvailable = false;
                    throw new Error('Inspector endpoints not available');
                }
                if (!response.ok) {
                    const errorData = await response.json();
                    throw new Error(errorData.error || `Connection failed: ${response.status}`);
                }
                const data = await response.json();
                this.session = data.sessionId;
                this.connected = true;
                this.inspectorAvailable = true;
                // Discover available methods from capabilities
                this.discoverMethods(data.result);
                // Try to discover tools automatically
                await this.discoverTools();
                this.$emit('tools-discovered', this.discoveredTools);
                this.showToast(`Connected to ${this.serverName}`, 'success');
            } catch (err) {
                this.error = err.message;
                this.connected = false;
                if (!err.message.includes('Inspector endpoints not available')) {
                    this.showToast(`Failed to connect inspector: ${err.message}`, 'error');
                }
            } finally {
                this.loading = false;
            }
        },
        async disconnect() {
            if (!this.session) return;
            try {
                await fetch('/api/inspector/disconnect', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ sessionId: this.session })
                });
            } catch (err) {
                console.warn('Failed to properly disconnect inspector:', err);
            }
            this.session = null;
            this.connected = false;
            this.response = null;
            this.error = null;
            this.availableMethods = [];
            this.discoveredTools = [];
            this.showToast('Disconnected', 'info');
        },
        discoverMethods(initializeResult) {
            const methods = ['initialize', 'shutdown'];
            if (initializeResult && initializeResult.capabilities) {
                const caps = initializeResult.capabilities;
                if (caps.resources) {
                    methods.push('resources/list', 'resources/get');
                }
                if (caps.tools) {
                    methods.push('tools/list', 'tools/get');
                }
                if (caps.prompts) {
                    methods.push('prompts/list', 'prompts/render');
                }
            }
            this.availableMethods = methods;
        },
        async discoverTools() {
            try {
                const response = await this.executeMethod('tools/list', {});
                if (response && response.result && response.result.tools) {
                    this.discoveredTools = response.result.tools;
                    this.$emit('tools-discovered', this.discoveredTools);
                }
            } catch (err) {
                console.warn(`Failed to discover tools for ${this.serverName}:`, err);
            }
        },
        async executeMethod(method, params = {}) {
            if (!this.session) {
                throw new Error('No active session');
            }
            try {
                const response = await fetch('/api/inspector/request', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        sessionId: this.session,
                        method: method,
                        params: params
                    })
                });
                const contentType = response.headers.get('content-type');
                if (!contentType || !contentType.includes('application/json')) {
                    throw new Error('Invalid response format - inspector may not be available');
                }
                if (!response.ok) {
                    const errorData = await response.json();
                    throw new Error(errorData.error || `Request failed: ${response.status}`);
                }
                const data = await response.json();
                this.response = data;
                // Update tools list if we just listed tools
                if (method === 'tools/list' && data.result && data.result.tools) {
                    this.discoveredTools = data.result.tools;
                    this.$emit('tools-discovered', this.discoveredTools);
                }
                return data;
            } catch (err) {
                if (err.message.includes('Unexpected token')) {
                    this.inspectorAvailable = false;
                    throw new Error('Inspector endpoints not available');
                }
                throw err;
            }
        },
        async executeTemplate(templateName) {
            const template = this.requestTemplates[templateName];
            if (!template) {
                this.showToast('Template not found', 'error');
                return;
            }
            try {
                await this.executeMethod(template.method, template.params);
            } catch (err) {
                this.error = err.message;
                this.showToast(`Error executing ${templateName}: ${err.message}`, 'error');
            }
        },
        async executeCustomRequest() {
            if (!this.request.trim()) {
                this.showToast('Enter a request', 'warning');
                return;
            }
            try {
                const requestObj = JSON.parse(this.request);
                await this.executeMethod(requestObj.method, requestObj.params || {});
            } catch (err) {
                this.error = err.message;
                this.showToast(`Error: ${err.message}`, 'error');
            }
        },
        loadTemplate(templateName) {
            const template = this.requestTemplates[templateName];
            if (template) {
                this.request = JSON.stringify({
                    method: template.method,
                    params: template.params
                }, null, 2);
            }
        },
        formatJSON(obj) {
            return JSON.stringify(obj, null, 2);
        },
        copyToClipboard(text) {
            if (navigator.clipboard) {
                navigator.clipboard.writeText(text).then(() => {
                    this.showToast('Copied', 'success');
                }).catch(err => {
                    this.showToast('Copy failed', 'error');
                });
            }
        },
        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },
    template: `
    <div v-if="inspectorAvailable !== false" class="space-y-4">
        <!-- Inspector Header -->
        <div class="flex items-center justify-between">
            <h4 class="text-sm font-medium text-white flex items-center">
                <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                </svg>
                MCP Inspector
            </h4>
            <div>
                <span v-if="connected" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-900/50 text-green-200 border border-green-700/50">
                    <span class="w-2 h-2 bg-green-400 rounded-full mr-2"></span>
                    Connected
                </span>
                <span v-else-if="loading" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-900/50 text-yellow-200 border border-yellow-700/50">
                    <div class="w-3 h-3 mr-2">
                        <div class="spinner"></div>
                    </div>
                    Connecting...
                </span>
                <span v-else-if="inspectorAvailable === null" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-700/50 text-slate-400 border border-slate-600/50">
                    <div class="w-3 h-3 mr-2">
                        <div class="spinner"></div>
                    </div>
                    Checking...
                </span>
                <span v-else class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-700/50 text-slate-400 border border-slate-600/50">
                    Disconnected
                </span>
            </div>
        </div>
        <!-- Connection Section -->
        <div v-if="!connected && !loading && inspectorAvailable !== null" class="text-center py-6 bg-slate-700/50 rounded-lg border border-slate-600/50">
            <svg class="mx-auto h-8 w-8 text-slate-400 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
            </svg>
            <p class="text-sm text-slate-400 mb-3">Start an inspector session to test MCP methods</p>
            <button
                @click="connect"
                :disabled="loading || inspectorAvailable === false"
                class="touch-target inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
                <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.50a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                </svg>
                Connect Inspector
            </button>
        </div>
        <!-- Checking Status -->
        <div v-if="inspectorAvailable === null" class="text-center py-6 bg-slate-700/50 rounded-lg border border-slate-600/50">
            <div class="w-6 h-6 mx-auto mb-3">
                <div class="spinner"></div>
            </div>
            <p class="text-sm text-slate-400">Checking inspector availability...</p>
        </div>
        <!-- Connected Interface -->
        <div v-if="connected" class="space-y-4">
            <!-- Quick Methods -->
            <div>
                <h6 class="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Quick Actions</h6>
                <div class="flex flex-wrap gap-2">
                    <button
                        v-for="method in availableMethods"
                        :key="method"
                        @click="executeTemplate(method)"
                        class="touch-target inline-flex items-center px-3 py-1.5 border border-slate-600 shadow-sm text-xs font-medium rounded text-slate-300 bg-slate-700 hover:bg-slate-600 transition-colors"
                    >
                        {{ method }}
                    </button>
                </div>
            </div>
            <!-- Custom Request -->
            <div>
                <div class="flex items-center justify-between mb-2">
                    <h6 class="text-xs font-medium text-slate-400 uppercase tracking-wide">Custom Request</h6>
                    <select
                        @change="loadTemplate($event.target.value); $event.target.value = ''"
                        class="text-xs border border-slate-600 rounded px-2 py-1 bg-slate-700 text-slate-300"
                    >
                        <option value="">Load template...</option>
                        <option v-for="(template, name) in requestTemplates" :key="name" :value="name">
                            {{ name }}
                        </option>
                    </select>
                </div>
                <div class="space-y-2">
                    <textarea
                        v-model="request"
                        placeholder='{"method": "tools/list", "params": {}}'
                        class="w-full h-20 px-3 py-2 border border-slate-600 rounded-lg bg-slate-700 text-white placeholder-slate-500 font-mono text-xs resize-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                    ></textarea>
                    <button
                        @click="executeCustomRequest"
                        :disabled="!request.trim()"
                        class="touch-target w-full inline-flex items-center justify-center px-3 py-2 border border-transparent text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                    >
                        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"></path>
                        </svg>
                        Send Request
                    </button>
                </div>
            </div>
            <!-- Response Display -->
            <div v-if="response" class="bg-gray-900 rounded-lg p-4 max-h-64 overflow-y-auto custom-scrollbar">
                <div class="flex items-center justify-between mb-2">
                    <span class="text-xs text-gray-400 font-medium">Response</span>
                    <button
                        @click="copyToClipboard(formatJSON(response))"
                        class="text-xs text-gray-400 hover:text-gray-300 transition-colors touch-target px-2 py-1 rounded hover:bg-gray-800"
                    >
                        <svg class="w-4 h-4 inline mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                        </svg>
                        Copy
                    </button>
                </div>
                <pre class="text-sm text-green-400 font-mono whitespace-pre-wrap">{{ formatJSON(response) }}</pre>
            </div>
            <!-- Discovered Tools Display -->
            <div v-if="discoveredTools.length > 0">
                <h6 class="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Discovered Tools ({{ discoveredTools.length }})</h6>
                <div class="space-y-2">
                    <div
                        v-for="tool in discoveredTools"
                        :key="tool.name"
                        class="bg-slate-700/50 p-3 rounded-lg border border-slate-600/50"
                    >
                        <div class="flex items-center justify-between gap-2">
                            <div class="min-w-0 flex-1">
                                <div class="font-medium text-sm text-white">{{ tool.name }}</div>
                                <div v-if="tool.description" class="text-xs text-slate-400 mt-1 truncate">
                                    {{ tool.description }}
                                </div>
                            </div>
                            <button
                                @click="executeTemplate('tools/get')"
                                class="text-xs text-blue-400 hover:text-blue-200 touch-target px-2 py-1 rounded hover:bg-blue-900/20 flex-shrink-0"
                            >
                                Test
                            </button>
                        </div>
                    </div>
                </div>
            </div>
            <!-- Error Display -->
            <div v-if="error" class="bg-red-900/20 border border-red-500/30 rounded-lg p-3">
                <div class="text-sm text-red-400">
                    {{ error }}
                </div>
            </div>
            <!-- Disconnect Button -->
            <div class="pt-2 border-t border-slate-700">
                <button
                    @click="disconnect"
                    class="touch-target w-full inline-flex items-center justify-center px-3 py-2 border border-slate-600 text-sm font-medium rounded-lg text-slate-300 bg-slate-700 hover:bg-slate-600 transition-colors"
                >
                    Disconnect Inspector
                </button>
            </div>
        </div>
        <!-- Inspector Not Available Message -->
        <div v-else-if="inspectorAvailable === false" class="text-center py-4 bg-slate-700/50 rounded-lg border border-slate-600/50">
            <svg class="mx-auto h-6 w-6 text-slate-400 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <p class="text-xs text-slate-400">Inspector endpoints not available</p>
            <p class="text-xs text-slate-500 mt-1">MCP inspection requires additional setup</p>
        </div>
    `
};