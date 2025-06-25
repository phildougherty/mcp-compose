const DashboardApp = {
    props: ['config'],
    data() {
        return {
            activeTab: 'overview',
            servers: [],
            status: {},
            connections: {},
            loading: false,
            error: '',
            wsConnections: {},
            refreshInterval: null,
            autoRefresh: false,
            refreshFrequency: 5000, // 5 seconds
            lastRefreshTime: null,
            showRefreshDropdown: false
        }
    },
    computed: {
        tabs() {
            return [
                { id: 'overview', name: 'Overview', icon: 'M4 6a2 2 0 012-2h8a2 2 0 012 2v7a2 2 0 01-2 2H8l-4 4V6z', enabled: true },
                { id: 'servers', name: 'Servers', icon: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2', enabled: true },
                { id: 'logs', name: 'Logs', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', enabled: this.config.enabledTabs.logs },
                { id: 'metrics', name: 'Metrics', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z', enabled: this.config.enabledTabs.metrics },
                { id: 'activity', name: 'Activity', icon: 'M13 10V3L4 14h7v7l9-11h-7z', enabled: true }
            ].filter(tab => tab.enabled);
        },
        runningServers() {
            return this.servers.filter(s => s.containerStatus === 'running').length;
        },
        totalServers() {
            return this.servers.length;
        },
        activeConnections() {
            return Object.keys(this.connections.activeHttpConnectionsManagedByProxy || {}).length;
        },
        proxyUptime() {
            return this.status.proxyUptime || '0s';
        },
        refreshFrequencyOptions() {
            return [
                { value: 5000, label: '5 seconds' },
                { value: 10000, label: '10 seconds' },
                { value: 30000, label: '30 seconds' },
                { value: 60000, label: '1 minute' },
                { value: 300000, label: '5 minutes' }
            ];
        },
        timeAgoText() {
            if (!this.lastRefreshTime) return 'Never refreshed';
            const now = new Date();
            const diff = now - this.lastRefreshTime;
            const seconds = Math.floor(diff / 1000);
            const minutes = Math.floor(seconds / 60);
            const hours = Math.floor(minutes / 60);
            
            if (seconds < 60) return `Refreshed ${seconds} second${seconds !== 1 ? 's' : ''} ago`;
            if (minutes < 60) return `Refreshed ${minutes} minute${minutes !== 1 ? 's' : ''} ago`;
            return `Refreshed ${hours} hour${hours !== 1 ? 's' : ''} ago`;
        }
    },
    methods: {
        async loadData() {
            if (this.loading) return;
            this.loading = true;
            this.error = '';
            try {
                const [servers, status, connections] = await Promise.all([
                    this.apiCall('/api/servers'),
                    this.apiCall('/api/status'),
                    this.apiCall('/api/connections')
                ]);
                this.servers = Object.entries(servers).map(([name, config]) => ({
                    name,
                    ...config
                }));
                this.status = status;
                this.connections = connections;
                this.lastRefreshTime = new Date();
                console.log('Dashboard data loaded:', { servers: this.servers.length, status: !!this.status });
            } catch (err) {
                console.error('Failed to load dashboard data:', err);
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },
        setupAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
            
            if (this.autoRefresh) {
                this.refreshInterval = setInterval(() => this.loadData(), this.refreshFrequency);
            }
        },
        toggleAutoRefresh() {
            this.autoRefresh = !this.autoRefresh;
            this.setupAutoRefresh();
        },
        setRefreshFrequency(frequency) {
            this.refreshFrequency = frequency;
            if (this.autoRefresh) {
                this.setupAutoRefresh();
            }
            this.showRefreshDropdown = false;
        },
        async apiCall(endpoint, options = {}) {
            const url = endpoint;
            const headers = {
                'Content-Type': 'application/json'
            };
            if (this.config.apiKey) {
                headers['Authorization'] = `Bearer ${this.config.apiKey}`;
            }
            
            try {
                const response = await fetch(url, {
                    headers,
                    ...options
                });
                
                if (!response.ok) {
                    // Check if the response is JSON or HTML
                    const contentType = response.headers.get('content-type');
                    if (contentType && contentType.includes('application/json')) {
                        const errorData = await response.json();
                        throw new Error(`HTTP ${response.status}: ${errorData.message || errorData.error || 'Unknown error'}`);
                    } else {
                        // It's likely an HTML error page
                        const errorText = await response.text();
                        if (errorText.includes('<!DOCTYPE')) {
                            throw new Error(`HTTP ${response.status}: Server returned HTML instead of JSON. Check if the proxy server is running correctly.`);
                        } else {
                            throw new Error(`HTTP ${response.status}: ${errorText}`);
                        }
                    }
                }
                
                return response.json();
            } catch (error) {
                if (error instanceof TypeError && error.message.includes('fetch')) {
                    // Network error
                    throw new Error('Network error: Unable to connect to the proxy server. Please check if it\'s running.');
                }
                throw error;
            }
        },
        async reloadProxy() {
            const confirmed = confirm(
                'Restart Proxy?\n\n' +
                'This will:\n' +
                '• Drop all active MCP connections\n' +
                '• Clear tool cache\n' +
                '• Force reconnection to all servers\n' +
                '• May briefly interrupt service\n\n' +
                'Continue?'
            );
            if (!confirmed) return;
            
            try {
                this.loading = true;
                this.error = ''; // Clear any existing errors
                
                // Check if we can reach the proxy first
                await this.apiCall('/api/status');
                
                // Now try to reload
                const result = await this.apiCall('/api/reload', { method: 'POST' });
                
                this.showSuccess('Proxy restarted successfully. Connections will be re-established.');
                console.log('Proxy reload result:', result);
                
                // Delay reload to let proxy restart
                setTimeout(() => this.loadData(), 2000);
            } catch (err) {
                console.error('Proxy reload error:', err);
                this.error = `Failed to restart proxy: ${err.message}`;
                
                // If it's a connection error, suggest checking the proxy server
                if (err.message.includes('Network error') || err.message.includes('HTML instead of JSON')) {
                    this.error += '\n\nTroubleshooting:\n• Check if the proxy server is running\n• Verify the proxy URL is correct\n• Check container logs for errors';
                }
            } finally {
                this.loading = false;
            }
        },
        showSuccess(message) {
            console.log('Success:', message);
        },
        async serverAction(action, serverName) {
            try {
                this.loading = true;
                await this.apiCall(`/api/servers/${action}`, {
                    method: 'POST',
                    body: JSON.stringify({ server: serverName })
                });
                setTimeout(() => this.loadData(), 2000);
            } catch (err) {
                this.error = `Failed to ${action} server ${serverName}: ${err.message}`;
            } finally {
                this.loading = false;
            }
        },
        formatUptime(uptime) {
            if (!uptime) return '0s';
            return uptime;
        },
        getServerStatusClass(server) {
            const status = server.containerStatus?.toLowerCase();
            if (status === 'running') return 'status-running';
            if (status === 'stopped' || status === 'exited') return 'status-stopped';
            return 'status-unknown';
        },
        getConnectionStatusClass(server) {
            const connections = this.connections.activeHttpConnectionsManagedByProxy;
            if (!connections) return 'status-unknown';
            const connection = connections[server.name];
            if (connection && connection.initialized) return 'status-running';
            return 'status-stopped';
        }
    },
    mounted() {
        this.loadData();
        // Click outside to close dropdown
        document.addEventListener('click', (e) => {
            if (!this.$refs.refreshDropdown?.contains(e.target)) {
                this.showRefreshDropdown = false;
            }
        });
    },
    beforeUnmount() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        Object.values(this.wsConnections).forEach(ws => {
            if (ws && ws.close) ws.close();
        });
    },
    template: `
        <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
            <!-- Mobile Header -->
            <header class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700 sticky top-0 z-50">
                <div class="px-4 sm:px-6 lg:px-8">
                    <div class="flex justify-between items-center h-16">
                        <div class="flex items-center min-w-0 flex-1">
                            <div class="flex-shrink-0">
                                <div class="flex items-center">
                                    <div class="w-8 h-8 bg-gradient-to-r from-blue-500 to-purple-600 rounded-lg flex items-center justify-center mr-3">
                                        <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                        </svg>
                                    </div>
                                    <div class="hidden sm:block">
                                        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">MCP Dashboard</h1>
                                        <p class="text-sm text-gray-500 dark:text-gray-400 hidden lg:block">Model Context Protocol Server Management</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                        <div class="flex items-center space-x-2">
                            <div class="relative" ref="refreshDropdown">
                                <div class="flex rounded-lg shadow-sm">
                                    <button
                                        @click="loadData"
                                        :disabled="loading"
                                        :class="[
                                            'relative inline-flex items-center px-3 py-2 rounded-l-lg border text-sm font-medium focus:z-10 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200',
                                            autoRefresh 
                                                ? 'border-green-300 dark:border-green-600 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-200 hover:bg-green-100 dark:hover:bg-green-900/50' 
                                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600'
                                        ]"
                                    >
                                        <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading, 'text-green-600 dark:text-green-400': autoRefresh }" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                        </svg>
                                        <span class="hidden sm:inline">{{ autoRefresh ? 'Auto' : 'Refresh' }}</span>
                                        <!-- Auto-refresh indicator dot -->
                                        <span v-if="autoRefresh" class="absolute -top-1 -right-1 w-3 h-3 bg-green-500 border-2 border-white dark:border-gray-800 rounded-full animate-pulse"></span>
                                    </button>
                                    <button
                                        @click="showRefreshDropdown = !showRefreshDropdown"
                                        :class="[
                                            'relative inline-flex items-center px-2 py-2 rounded-r-lg border border-l-0 text-sm font-medium focus:z-10 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors',
                                            autoRefresh 
                                                ? 'border-green-300 dark:border-green-600 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-200 hover:bg-green-100 dark:hover:bg-green-900/50' 
                                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600'
                                        ]"
                                    >
                                        <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                        </svg>
                                    </button>
                                </div>
                                
                                <!-- Refresh Dropdown -->
                                <div v-if="showRefreshDropdown" class="origin-top-right absolute right-0 mt-2 w-64 rounded-lg shadow-lg bg-white dark:bg-gray-800 ring-1 ring-black ring-opacity-5 border border-gray-200 dark:border-gray-600 z-50">
                                    <div class="p-4 space-y-4">
                                        <div class="flex items-center justify-between">
                                            <label class="text-sm font-medium text-gray-700 dark:text-gray-200">Auto Refresh</label>
                                            <button
                                                @click="toggleAutoRefresh"
                                                :class="[
                                                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
                                                    autoRefresh ? 'bg-blue-600' : 'bg-gray-200 dark:bg-gray-600'
                                                ]"
                                            >
                                                <span :class="[
                                                    'pointer-events-none inline-block h-5 w-5 rounded-full bg-white shadow transform ring-0 transition duration-200 ease-in-out',
                                                    autoRefresh ? 'translate-x-5' : 'translate-x-0'
                                                ]"></span>
                                            </button>
                                        </div>
                                        <div v-if="autoRefresh">
                                            <label class="text-sm font-medium text-gray-700 dark:text-gray-200">Frequency</label>
                                            <div class="mt-2 space-y-2">
                                                <button
                                                    v-for="option in refreshFrequencyOptions"
                                                    :key="option.value"
                                                    @click="setRefreshFrequency(option.value)"
                                                    :class="[
                                                        'w-full text-left px-3 py-2 text-sm rounded-md transition-colors',
                                                        refreshFrequency === option.value
                                                            ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                                                            : 'text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-700'
                                                    ]"
                                                >
                                                    {{ option.label }}
                                                </button>
                                            </div>
                                        </div>
                                        <div class="border-t border-gray-200 dark:border-gray-600 pt-3">
                                            <p class="text-xs text-gray-500 dark:text-gray-400">{{ timeAgoText }}</p>
                                        </div>
                                    </div>
                                </div>
                            </div>
                            <button
                                @click="reloadProxy"
                                :disabled="loading"
                                class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-lg text-white bg-orange-600 hover:bg-orange-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors shadow-sm"
                            >
                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                </svg>
                                <span class="hidden sm:inline">Restart</span>
                            </button>
                        </div>
                    </div>
                </div>
            </header>

            <!-- Mobile Navigation -->
            <nav class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 sticky top-16 z-40">
                <div class="px-4 sm:px-6 lg:px-8">
                    <div class="flex space-x-1 overflow-x-auto scrollbar-hide py-2">
                        <button
                            v-for="tab in tabs"
                            :key="tab.id"
                            @click="activeTab = tab.id"
                            :class="[
                                'whitespace-nowrap flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors',
                                activeTab === tab.id
                                    ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                                    : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
                            ]"
                        >
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="tab.icon"></path>
                            </svg>
                            {{ tab.name }}
                        </button>
                    </div>
                </div>
            </nav>

            <!-- Main Content -->
            <main class="px-4 sm:px-6 lg:px-8 py-6">
                <!-- Loading State -->
                <div v-if="loading && !servers.length" class="flex items-center justify-center py-12">
                    <div class="text-center">
                        <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto"></div>
                        <p class="mt-4 text-sm text-gray-600 dark:text-gray-400">Loading dashboard...</p>
                    </div>
                </div>

                <!-- Error Display -->
                <div v-if="error" class="mb-6 bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <div class="ml-3 flex-1">
                            <h3 class="text-sm font-medium text-red-800 dark:text-red-200">Dashboard Error</h3>
                            <div class="mt-2 text-sm text-red-700 dark:text-red-300">{{ error }}</div>
                            <button
                                @click="error = ''"
                                class="mt-3 text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline"
                            >
                                Dismiss
                            </button>
                        </div>
                    </div>
                </div>

                <!-- Overview Tab -->
                <div v-if="activeTab === 'overview'" class="space-y-6">
                    <!-- Stats Grid -->
                    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4 lg:gap-6">
                        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                            <div class="p-4 lg:p-6">
                                <div class="flex items-center">
                                    <div class="flex-shrink-0">
                                        <div class="w-10 h-10 bg-gradient-to-r from-green-400 to-green-600 rounded-lg flex items-center justify-center">
                                            <svg class="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                            </svg>
                                        </div>
                                    </div>
                                    <div class="ml-4 flex-1 min-w-0">
                                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Running</p>
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ runningServers }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                            <div class="p-4 lg:p-6">
                                <div class="flex items-center">
                                    <div class="flex-shrink-0">
                                        <div class="w-10 h-10 bg-gradient-to-r from-blue-400 to-blue-600 rounded-lg flex items-center justify-center">
                                            <svg class="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path>
                                            </svg>
                                        </div>
                                    </div>
                                    <div class="ml-4 flex-1 min-w-0">
                                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Total</p>
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ totalServers }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                            <div class="p-4 lg:p-6">
                                <div class="flex items-center">
                                    <div class="flex-shrink-0">
                                        <div class="w-10 h-10 bg-gradient-to-r from-yellow-400 to-orange-500 rounded-lg flex items-center justify-center">
                                            <svg class="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                            </svg>
                                        </div>
                                    </div>
                                    <div class="ml-4 flex-1 min-w-0">
                                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Connected</p>
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activeConnections }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                            <div class="p-4 lg:p-6">
                                <div class="flex items-center">
                                    <div class="flex-shrink-0">
                                        <div class="w-10 h-10 bg-gradient-to-r from-purple-400 to-purple-600 rounded-lg flex items-center justify-center">
                                            <svg class="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd"></path>
                                            </svg>
                                        </div>
                                    </div>
                                    <div class="ml-4 flex-1 min-w-0">
                                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Uptime</p>
                                        <p class="text-lg lg:text-2xl font-bold text-gray-900 dark:text-white truncate">{{ formatUptime(proxyUptime) }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Quick Server Overview -->
                    <div class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                        <div class="px-4 py-5 sm:p-6">
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-4">Server Status</h3>
                            <div class="space-y-3">
                                <div v-for="server in servers.slice(0, 6)" :key="server.name"
                                     class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors">
                                    <div class="flex items-center space-x-3 min-w-0 flex-1">
                                        <div :class="['w-3 h-3 rounded-full flex-shrink-0', getServerStatusClass(server)]"></div>
                                        <div class="min-w-0 flex-1">
                                            <p class="text-sm font-medium text-gray-900 dark:text-white truncate">{{ server.name }}</p>
                                            <p class="text-xs text-gray-500 dark:text-gray-400 truncate">
                                                {{ server.configProtocol || 'stdio' }}
                                                <span v-if="server.configHttpPort">• :{{ server.configHttpPort }}</span>
                                            </p>
                                        </div>
                                    </div>
                                    <div class="flex items-center space-x-2 flex-shrink-0">
                                        <div :class="['w-2 h-2 rounded-full', getConnectionStatusClass(server)]"></div>
                                        <span class="text-xs text-gray-500 dark:text-gray-400">Proxy</span>
                                    </div>
                                </div>
                                <div v-if="servers.length > 6" class="text-center pt-2">
                                    <button
                                        @click="activeTab = 'servers'"
                                        class="inline-flex items-center text-sm text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-200 font-medium"
                                    >
                                        View all {{ servers.length }} servers
                                        <svg class="ml-1 w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"></path>
                                        </svg>
                                    </button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Other Tabs -->
                <server-manager
                    v-if="activeTab === 'servers'"
                    :servers="servers"
                    :connections="connections"
                    :loading="loading"
                    @server-action="serverAction"
                    @reload="loadData"
                ></server-manager>

                <log-viewer
                    v-if="activeTab === 'logs'"
                    :servers="servers"
                    :config="config"
                ></log-viewer>

                <metrics-display
                    v-if="activeTab === 'metrics'"
                    :servers="servers"
                    :status="status"
                    :connections="connections"
                ></metrics-display>
                <activity-viewer
                    v-if="activeTab === 'activity'"
                    ref="activityViewer"
                    :config="config"
                ></activity-viewer>
            </main>
        </div>
    `
};