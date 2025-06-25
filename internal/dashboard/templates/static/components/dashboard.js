const DashboardApp = {
    props: ['config'],
    data() {
        return {
            activeTab: 'overview', // Add active tab tracking
            servers: [],
            status: {},
            connections: {},
            loading: false,
            error: '',
            refreshInterval: null,
            autoRefresh: false,
            refreshFrequency: 5000,
            lastRefreshTime: null,
            showRefreshDropdown: false,
            expandedServers: new Set(),
            searchTerm: '',
            filterStatus: 'all',
            sortBy: 'name'
        }
    },
    computed: {
        tabs() {
            return [
                { id: 'overview', name: 'Overview', icon: 'M4 6a2 2 0 012-2h8a2 2 0 012 2v7a2 2 0 01-2 2H8l-4 4V6z', enabled: true },
                { id: 'logs', name: 'Logs', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', enabled: this.config.enabledTabs.logs },
                { id: 'metrics', name: 'Metrics', icon: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z', enabled: this.config.enabledTabs.metrics },
                { id: 'activity', name: 'Activity', icon: 'M13 10V3L4 14h7v7l9-11h-7z', enabled: true }
            ].filter(tab => tab.enabled);
        },
        filteredAndSortedServers() {
            let filtered = this.servers.filter(server => {
                const matchesSearch = server.name.toLowerCase().includes(this.searchTerm.toLowerCase());
                const matchesFilter = this.getFilterMatch(server);
                return matchesSearch && matchesFilter;
            });

            return filtered.sort((a, b) => {
                switch (this.sortBy) {
                    case 'status':
                        return this.getServerStatusPriority(b) - this.getServerStatusPriority(a);
                    case 'tools':
                        return (this.getServerToolCount(b) || 0) - (this.getServerToolCount(a) || 0);
                    case 'health':
                        return this.getHealthPriority(b) - this.getHealthPriority(a);
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
                healthy: this.servers.filter(s => this.isServerHealthy(s)).length
            };
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
            if (seconds < 60) return `${seconds}s ago`;
            if (minutes < 60) return `${minutes}m ago`;
            return `${Math.floor(minutes / 60)}h ago`;
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
            } catch (err) {
                console.error('Failed to load dashboard data:', err);
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },

        async apiCall(endpoint, options = {}) {
            const url = endpoint;
            const headers = {
                'Content-Type': 'application/json'
            };
            if (this.config.apiKey) {
                headers['Authorization'] = `Bearer ${this.config.apiKey}`;
            }
            
            const response = await fetch(url, { headers, ...options });
            if (!response.ok) {
                const contentType = response.headers.get('content-type');
                if (contentType && contentType.includes('application/json')) {
                    const errorData = await response.json();
                    throw new Error(`HTTP ${response.status}: ${errorData.message || errorData.error || 'Unknown error'}`);
                } else {
                    throw new Error(`HTTP ${response.status}: Server returned HTML instead of JSON`);
                }
            }
            return response.json();
        },

        // Helper methods for generating URLs
        getServerDocUrl(serverName) {
            return `/api/server-docs/${serverName}`;
        },
        
        getServerOpenApiUrl(serverName) {
            return `/api/server-openapi/${serverName}`;
        },
        
        getServerDirectUrl(serverName) {
            return `/api/server-direct/${serverName}`;
        },
        
        getContainerLogsUrl(serverName) {
            return `/api/server-logs/${serverName}`;
        },

        // Method to navigate to logs tab with server pre-selected
        viewServerLogs(serverName) {
            this.activeTab = 'logs';
            this.$nextTick(() => {
                // Pass the server name to the log viewer component
                this.$refs.logViewer?.setSelectedServer(serverName);
            });
        },

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

        getFilterMatch(server) {
            switch (this.filterStatus) {
                case 'running': return this.isContainerRunning(server);
                case 'stopped': return !this.isContainerRunning(server);
                case 'connected': return this.getConnectionStatus(server) === 'Connected';
                case 'healthy': return this.isServerHealthy(server);
                default: return true;
            }
        },

        getServerStatusPriority(server) {
            if (this.isContainerRunning(server)) return 3;
            if (server.containerStatus === 'stopped') return 1;
            return 0;
        },

        getHealthPriority(server) {
            const connection = this.getHttpConnection(server);
            if (connection && connection.initialized && connection.rawHealthyFlag) return 3;
            if (this.isContainerRunning(server)) return 2;
            return 0;
        },

        getServerToolCount(server) {
            const connection = this.getHttpConnection(server);
            if (connection && connection.serverReportedCapabilities && connection.serverReportedCapabilities.tools) {
                return connection.serverReportedCapabilities.tools.length || 0;
            }
            return server.configCapabilities ? server.configCapabilities.length : 0;
        },

        isContainerRunning(server) {
            return server.containerStatus?.toLowerCase() === 'running';
        },

        isServerHealthy(server) {
            const connection = this.getHttpConnection(server);
            return connection && connection.initialized && connection.rawHealthyFlag;
        },

        getConnectionStatus(server) {
            const connection = this.getHttpConnection(server);
            if (connection && connection.initialized && connection.rawHealthyFlag) {
                return 'Connected';
            }
            return 'Disconnected';
        },

        getHttpConnection(server) {
            if (!this.connections || !this.connections.activeHttpConnectionsManagedByProxy) {
                return null;
            }
            return this.connections.activeHttpConnectionsManagedByProxy[server.name] || null;
        },

        getServerCapabilities(server) {
            const connection = this.getHttpConnection(server);
            if (connection && connection.serverReportedCapabilities) {
                return connection.serverReportedCapabilities;
            }
            return server.configCapabilities || {};
        },

        getServerInfo(server) {
            const connection = this.getHttpConnection(server);
            if (connection && connection.serverReportedInfo) {
                return connection.serverReportedInfo;
            }
            return {};
        },

        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                return new Date(timestamp).toLocaleString();
            } catch (e) {
                return timestamp;
            }
        },

        formatUptime(uptime) {
            return uptime || '0s';
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

        async reloadProxy() {
            const confirmed = confirm('Restart Proxy?\n\nThis will drop all active connections and reload configuration.');
            if (!confirmed) return;
            
            try {
                this.loading = true;
                await this.apiCall('/api/reload', { method: 'POST' });
                this.showSuccess('Proxy restarted successfully');
                setTimeout(() => this.loadData(), 2000);
            } catch (err) {
                this.error = `Failed to restart proxy: ${err.message}`;
            } finally {
                this.loading = false;
            }
        },

        showSuccess(message) {
            console.log('Success:', message);
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
        }
    },

    mounted() {
        this.loadData();
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
    },

    template: `
        <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
            <!-- Header -->
            <header class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700 sticky top-0 z-50">
                <div class="px-4 sm:px-6 lg:px-8">
                    <div class="flex justify-between items-center h-16">
                        <div class="flex items-center min-w-0 flex-1">
                            <div class="w-8 h-8 bg-gradient-to-r from-blue-500 to-purple-600 rounded-lg flex items-center justify-center mr-3">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                            <div>
                                <h1 class="text-xl font-semibold text-gray-900 dark:text-white">MCP Dashboard</h1>
                                <p class="text-sm text-gray-500 dark:text-gray-400">Model Context Protocol Server Management</p>
                            </div>
                        </div>
                        
                        <!-- Controls -->
                        <div class="flex items-center space-x-2">
                            <!-- Auto Refresh Controls -->
                            <div class="relative" ref="refreshDropdown">
                                <div class="flex rounded-lg shadow-sm">
                                    <button
                                        @click="loadData"
                                        :disabled="loading"
                                        :class="[
                                            'relative inline-flex items-center px-3 py-2 rounded-l-lg border text-sm font-medium focus:z-10 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:opacity-50 transition-all',
                                            autoRefresh
                                                ? 'border-green-300 dark:border-green-600 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-200'
                                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200'
                                        ]"
                                    >
                                        <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                        </svg>
                                        {{ autoRefresh ? 'Auto' : 'Refresh' }}
                                        <span v-if="autoRefresh" class="absolute -top-1 -right-1 w-3 h-3 bg-green-500 border-2 border-white dark:border-gray-800 rounded-full animate-pulse"></span>
                                    </button>
                                    <button
                                        @click="showRefreshDropdown = !showRefreshDropdown"
                                        :class="[
                                            'relative inline-flex items-center px-2 py-2 rounded-r-lg border border-l-0 text-sm font-medium focus:z-10 focus:outline-none transition-colors',
                                            autoRefresh
                                                ? 'border-green-300 dark:border-green-600 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-200'
                                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200'
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
                            
                            <!-- Restart Proxy Button -->
                            <button
                                @click="reloadProxy"
                                :disabled="loading"
                                class="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-lg text-white bg-orange-600 hover:bg-orange-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 disabled:opacity-50 transition-colors shadow-sm"
                            >
                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                </svg>
                                Restart
                            </button>
                        </div>
                    </div>
                </div>
            </header>

            <!-- Navigation Tabs -->
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
                            <button @click="error = ''" class="mt-3 text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline">
                                Dismiss
                            </button>
                        </div>
                    </div>
                </div>

                <!-- Overview Tab Content -->
                <div v-if="activeTab === 'overview'">
                    <!-- Stats Overview -->
                    <div class="grid grid-cols-2 lg:grid-cols-5 gap-4 lg:gap-6 mb-8">
                        <!-- Total Servers -->
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
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ statusCounts.total }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Running Containers -->
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
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ statusCounts.running }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Stopped Containers -->
                        <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700">
                            <div class="p-4 lg:p-6">
                                <div class="flex items-center">
                                    <div class="flex-shrink-0">
                                        <div class="w-10 h-10 bg-gradient-to-r from-red-400 to-red-600 rounded-lg flex items-center justify-center">
                                            <svg class="w-6 h-6 text-white" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                                            </svg>
                                        </div>
                                    </div>
                                    <div class="ml-4 flex-1 min-w-0">
                                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Stopped</p>
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ statusCounts.stopped }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Connected via Proxy -->
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
                                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ statusCounts.connected }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>

                        <!-- Proxy Uptime -->
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
                                        <p class="text-lg lg:text-2xl font-bold text-gray-900 dark:text-white truncate">{{ formatUptime(status.proxyUptime) }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>

                    <!-- Search and Filter Controls -->
                    <div class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4 lg:p-6 mb-6">
                        <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-4 sm:space-y-0">
                            <div class="flex-1 max-w-lg">
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
                                        class="block w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                                    >
                                </div>
                            </div>
                            
                            <div class="flex space-x-3">
                                <!-- Filter -->
                                <select
                                    v-model="filterStatus"
                                    class="block px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                                >
                                    <option value="all">All ({{ statusCounts.total }})</option>
                                    <option value="running">Running ({{ statusCounts.running }})</option>
                                    <option value="stopped">Stopped ({{ statusCounts.stopped }})</option>
                                    <option value="connected">Connected ({{ statusCounts.connected }})</option>
                                    <option value="healthy">Healthy ({{ statusCounts.healthy }})</option>
                                </select>
                                
                                <!-- Sort -->
                                <select
                                    v-model="sortBy"
                                    class="block px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                                >
                                    <option value="name">Sort by Name</option>
                                    <option value="status">Sort by Status</option>
                                    <option value="health">Sort by Health</option>
                                    <option value="tools">Sort by Tools</option>
                                </select>
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
                    <div v-else-if="filteredAndSortedServers.length === 0" class="text-center py-12">
                        <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.172 16.172a4 4 0 015.656 0M9 12h6m-6 4h6M5 16a3 3 0 01-3-3V9a3 3 0 013-3h14a3 3 0 013 3v4a3 3 0 01-3 3H5z"></path>
                        </svg>
                        <h3 class="mt-4 text-sm font-medium text-gray-900 dark:text-white">No servers found</h3>
                        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">Try adjusting your search or filter criteria.</p>
                    </div>

                    <!-- Server Accordions -->
                    <div v-else class="space-y-4">
                        <div
                            v-for="server in filteredAndSortedServers"
                            :key="server.name"
                            class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden transition-all duration-200"
                            :class="{ 'ring-2 ring-blue-500 ring-opacity-50': isServerExpanded(server.name) }"
                        >
                            <!-- Server Header (Collapsed State) -->
                            <div 
                                @click="toggleServerExpansion(server.name)"
                                class="p-4 lg:p-6 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
                            >
                                <div class="flex items-center justify-between">
                                    <div class="flex items-center space-x-4 min-w-0 flex-1">
                                        <!-- Status Indicator -->
                                        <div class="flex-shrink-0">
                                            <div
                                                :class="[
                                                    'w-4 h-4 rounded-full',
                                                    isContainerRunning(server) ? 'bg-green-500' :
                                                    server.containerStatus === 'stopped' ? 'bg-red-500' : 'bg-yellow-500'
                                                ]"
                                            ></div>
                                        </div>
                                        
                                        <!-- Server Name and Basic Info -->
                                        <div class="min-w-0 flex-1">
                                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white truncate">
                                                {{ server.name }}
                                            </h3>
                                            <div class="flex items-center space-x-4 mt-1">
                                                <span class="text-sm text-gray-500 dark:text-gray-400">
                                                    {{ server.configProtocol || 'stdio' }}
                                                </span>
                                                <span v-if="server.configHttpPort" class="text-sm text-gray-500 dark:text-gray-400">
                                                    Port {{ server.configHttpPort }}
                                                </span>
                                                <span class="text-sm text-gray-500 dark:text-gray-400">
                                                    {{ getServerToolCount(server) || 0 }} tools
                                                </span>
                                            </div>
                                        </div>

                                        <!-- Quick Status Badges -->
                                        <div class="flex items-center space-x-3">
                                            <!-- Container Status -->
                                            <span :class="[
                                                'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                                isContainerRunning(server)
                                                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                                    : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                            ]">
                                                {{ server.containerStatus || 'Unknown' }}
                                            </span>
                                            
                                            <!-- Proxy Status -->
                                            <span :class="[
                                                'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                                getConnectionStatus(server) === 'Connected'
                                                    ? 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
                                                    : 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200'
                                            ]">
                                                {{ getConnectionStatus(server) }}
                                            </span>

                                            <!-- Health Status -->
                                            <div class="flex items-center">
                                                <div
                                                    :class="[
                                                        'w-2 h-2 rounded-full',
                                                        isServerHealthy(server) ? 'bg-green-500' : 'bg-gray-400'
                                                    ]"
                                                ></div>
                                            </div>
                                        </div>
                                    </div>

                                    <!-- Expand/Collapse Button -->
                                    <div class="flex items-center space-x-2 ml-4">
                                        <!-- Quick Action Buttons (visible when collapsed) -->
                                        <div v-if="!isServerExpanded(server.name)" class="flex space-x-1" @click.stop>
                                            <a 
                                                :href="getServerDocUrl(server.name)" 
                                                target="_blank"
                                                class="inline-flex items-center px-2 py-1 text-xs font-medium text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-200 transition-colors"
                                                title="View Documentation"
                                            >
                                                Docs
                                            </a>
                                            <button 
                                                @click="viewServerLogs(server.name)"
                                                class="inline-flex items-center px-2 py-1 text-xs font-medium text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200 transition-colors"
                                                title="View Logs"
                                            >
                                                Logs
                                            </button>
                                        </div>

                                        <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors p-1">
                                            <svg
                                                :class="['w-5 h-5 transition-transform duration-200', isServerExpanded(server.name) ? 'rotate-180' : '']"
                                                fill="currentColor"
                                                viewBox="0 0 20 20"
                                            >
                                                <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                            </svg>
                                        </button>
                                    </div>
                                </div>
                            </div>

                            <!-- Expanded Content -->
                            <div v-if="isServerExpanded(server.name)" class="border-t border-gray-200 dark:border-gray-700">
                                <div class="p-4 lg:p-6 bg-gray-50 dark:bg-gray-700/30">
                                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                                        <!-- Left Column: Connection Details -->
                                        <div class="space-y-6">
                                            <div>
                                                <h4 class="text-lg font-medium text-gray-900 dark:text-white mb-4 flex items-center">
                                                    <svg class="w-5 h-5 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                                    </svg>
                                                    Connection Details
                                                </h4>
                                                
                                                <div v-if="getHttpConnection(server)" class="space-y-4">
                                                    <div class="bg-white dark:bg-gray-800 p-4 rounded-lg">
                                                        <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3">HTTP Connection</h5>
                                                        <div class="space-y-3 text-sm">
                                                            <div class="flex justify-between">
                                                                <span class="font-medium text-gray-500 dark:text-gray-400">Status:</span>
                                                                <span :class="[
                                                                    'px-2 py-1 rounded text-xs font-medium',
                                                                    getHttpConnection(server).initialized && getHttpConnection(server).rawHealthyFlag
                                                                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                                                        : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                                                ]">
                                                                    {{ getHttpConnection(server).status }}
                                                                </span>
                                                            </div>
                                                            <div class="flex justify-between">
                                                                <span class="font-medium text-gray-500 dark:text-gray-400">Target URL:</span>
                                                                <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                                                                    {{ getHttpConnection(server).targetBaseURL }}
                                                                </code>
                                                            </div>
                                                            <div v-if="getHttpConnection(server).mcpSessionID" class="flex justify-between">
                                                                <span class="font-medium text-gray-500 dark:text-gray-400">Session ID:</span>
                                                                <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">
                                                                    {{ getHttpConnection(server).mcpSessionID }}
                                                                </code>
                                                            </div>
                                                            <div v-if="getHttpConnection(server).lastUsedByProxy">
                                                                <span class="font-medium text-gray-500 dark:text-gray-400">Last Used:</span>
                                                                <div class="text-gray-700 dark:text-gray-300">
                                                                    {{ formatTimestamp(getHttpConnection(server).lastUsedByProxy) }}
                                                                </div>
                                                            </div>
                                                        </div>
                                                    </div>
                                                </div>
                                                
                                                <div v-else class="text-center py-8 text-gray-500 dark:text-gray-400">
                                                    <svg class="w-12 h-12 mx-auto mb-3 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192L5.636 18.364M12 12l2.828 2.828M12 12l2.828-2.828M12 12L9.172 9.172M12 12l-2.828 2.828"></path>
                                                    </svg>
                                                    <p>No active proxy connection</p>
                                                    <p class="text-xs mt-1">Server needs to be initialized via proxy</p>
                                                </div>
                                            </div>

                                            <!-- Action Buttons -->
                                            <div class="bg-white dark:bg-gray-800 p-4 rounded-lg">
                                                <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3">Actions</h5>
                                                <div class="grid grid-cols-2 gap-3">
                                                    <button
                                                        v-if="!isContainerRunning(server)"
                                                        @click="serverAction('start', server.name)"
                                                        :disabled="loading"
                                                        class="flex items-center justify-center px-3 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-green-600 hover:bg-green-700 disabled:bg-gray-400 transition-colors"
                                                    >
                                                        <svg class="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                                                        </svg>
                                                        Start
                                                    </button>
                                                    
                                                    <button
                                                        v-if="isContainerRunning(server)"
                                                        @click="serverAction('stop', server.name)"
                                                        :disabled="loading"
                                                        class="flex items-center justify-center px-3 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-red-600 hover:bg-red-700 disabled:bg-gray-400 transition-colors"
                                                    >
                                                        <svg class="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                                                        </svg>
                                                        Stop
                                                    </button>
                                                    
                                                    <button
                                                        v-if="isContainerRunning(server)"
                                                        @click="serverAction('restart', server.name)"
                                                        :disabled="loading"
                                                        class="flex items-center justify-center px-3 py-2 border border-transparent text-sm font-medium rounded-md text-white bg-yellow-600 hover:bg-yellow-700 disabled:bg-gray-400 transition-colors"
                                                    >
                                                        <svg class="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                                        </svg>
                                                        Restart
                                                    </button>

                                                    <!-- External Links -->
                                                    <a 
                                                        :href="getServerDocUrl(server.name)" 
                                                        target="_blank"
                                                        class="flex items-center justify-center px-3 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                                    >
                                                        Documentation
                                                    </a>
                                                    
                                                    <button 
                                                        @click="viewServerLogs(server.name)"
                                                        class="flex items-center justify-center px-3 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                                    >
                                                        View Logs
                                                    </button>
                                                    
                                                    <a 
                                                        :href="getServerOpenApiUrl(server.name)" 
                                                        target="_blank"
                                                        class="flex items-center justify-center px-3 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                                    >
                                                        OpenAPI Spec
                                                    </a>

                                                    <a 
                                                        :href="getServerDirectUrl(server.name)" 
                                                        target="_blank"
                                                        class="flex items-center justify-center px-3 py-2 border border-gray-300 dark:border-gray-600 text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                                    >
                                                        Direct Access
                                                    </a>
                                                </div>
                                            </div>
                                        </div>

                                        <!-- Right Column: Configuration & Capabilities -->
                                        <div class="space-y-6">
                                            <!-- Server Configuration -->
                                            <div class="bg-white dark:bg-gray-800 p-4 rounded-lg">
                                                <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3">Configuration</h5>
                                                <div class="space-y-2 text-sm">
                                                    <div class="flex justify-between">
                                                        <span class="font-medium text-gray-500 dark:text-gray-400">Protocol:</span>
                                                        <span class="text-gray-700 dark:text-gray-300">{{ server.configProtocol || 'stdio' }}</span>
                                                    </div>
                                                    <div v-if="server.configHttpPort" class="flex justify-between">
                                                        <span class="font-medium text-gray-500 dark:text-gray-400">HTTP Port:</span>
                                                        <span class="text-gray-700 dark:text-gray-300">{{ server.configHttpPort }}</span>
                                                    </div>
                                                    <div class="flex justify-between">
                                                        <span class="font-medium text-gray-500 dark:text-gray-400">Container:</span>
                                                        <span class="text-gray-700 dark:text-gray-300">{{ server.isContainer ? 'Yes' : 'No' }}</span>
                                                    </div>
                                                    <div class="flex justify-between">
                                                        <span class="font-medium text-gray-500 dark:text-gray-400">Transport:</span>
                                                        <span class="text-gray-700 dark:text-gray-300">{{ server.proxyTransportMode || 'HTTP' }}</span>
                                                    </div>
                                                </div>
                                            </div>

                                            <!-- Capabilities -->
                                            <div class="bg-white dark:bg-gray-800 p-4 rounded-lg">
                                                <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3">Capabilities</h5>
                                                <div v-if="Object.keys(getServerCapabilities(server)).length > 0">
                                                    <div class="flex flex-wrap gap-2">
                                                        <span
                                                            v-for="(value, capability) in getServerCapabilities(server)"
                                                            :key="capability"
                                                            class="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                                                        >
                                                            {{ capability }}
                                                        </span>
                                                    </div>
                                                </div>
                                                <div v-else class="text-sm text-gray-500 dark:text-gray-400">
                                                    No capabilities reported
                                                </div>
                                            </div>

                                            <!-- Server Info -->
                                            <div class="bg-white dark:bg-gray-800 p-4 rounded-lg">
                                                <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3">Server Info</h5>
                                                <div v-if="Object.keys(getServerInfo(server)).length > 0" class="space-y-2 text-sm">
                                                    <div
                                                        v-for="(value, key) in getServerInfo(server)"
                                                        :key="key"
                                                        class="flex justify-between"
                                                    >
                                                        <span class="font-medium text-gray-500 dark:text-gray-400 capitalize">{{ key.replace(/([A-Z])/g, ' $1') }}:</span>
                                                        <span class="text-gray-700 dark:text-gray-300">{{ value }}</span>
                                                    </div>
                                                </div>
                                                <div v-else class="text-sm text-gray-500 dark:text-gray-400">
                                                    No server info available
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Other tabs -->
                <log-viewer
                    v-if="activeTab === 'logs'"
                    ref="logViewer"
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
                    :config="config"
                ></activity-viewer>
            </main>
        </div>
    `
};