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
            refreshInterval: null,
            autoRefresh: false,
            refreshFrequency: 5000,
            lastRefreshTime: null,
            showRefreshDropdown: false,
            expandedServers: new Set(),
            searchTerm: '',
            filterStatus: 'all',
            sortBy: 'name',
            
            // Mobile state
            isMobileView: false,
            
            // Server tools discovered by inspector
            serverTools: {}
        }
    },
    
    computed: {
        tabs() {
            return [
                { id: 'overview', name: 'Overview', icon: 'M4 6a2 2 0 012-2h8a2 2 0 012 2v7a2 2 0 01-2 2H8l-4 4V6z', enabled: true },
                { id: 'logs', name: 'Logs', icon: 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z', enabled: this.config.enabledTabs.logs },
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
        // Enhanced uptime formatting
        formatUptime(uptimeString) {
            if (!uptimeString) return '0s';
            
            let seconds;
            
            if (typeof uptimeString === 'string') {
                // Handle duration strings like "1h23m45.123456789s"
                const timeRegex = /(?:(\d+)h)?(?:(\d+)m)?(?:(\d+(?:\.\d+)?)s)?/;
                const match = uptimeString.match(timeRegex);
                
                if (match) {
                    const hours = parseInt(match[1] || '0');
                    const minutes = parseInt(match[2] || '0');
                    const secs = parseFloat(match[3] || '0');
                    seconds = hours * 3600 + minutes * 60 + secs;
                } else {
                    seconds = parseFloat(uptimeString);
                }
            } else {
                seconds = parseFloat(uptimeString);
            }
            
            if (isNaN(seconds) || seconds < 0) return '0s';
            
            // Convert to whole seconds (remove decimal precision)
            seconds = Math.floor(seconds);
            
            const days = Math.floor(seconds / 86400);
            const hours = Math.floor((seconds % 86400) / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            
            const parts = [];
            if (days > 0) parts.push(`${days}d`);
            if (hours > 0) parts.push(`${hours}h`);
            if (minutes > 0) parts.push(`${minutes}m`);
            
            // If no significant time parts, show seconds
            if (parts.length === 0) {
                parts.push(`${seconds % 60}s`);
            }
            
            // Limit to most significant 2 parts
            return parts.slice(0, 2).join(' ');
        },
        
        // Enhanced timestamp formatting
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            
            try {
                const date = new Date(timestamp);
                if (isNaN(date.getTime())) return timestamp;
                
                const now = new Date();
                const diffMs = now - date;
                const diffMinutes = Math.floor(diffMs / (1000 * 60));
                const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
                const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
                
                // Show relative time for recent timestamps
                if (diffMinutes < 1) return 'Just now';
                if (diffMinutes < 60) return `${diffMinutes}m ago`;
                if (diffHours < 24) return `${diffHours}h ago`;
                if (diffDays < 7) return `${diffDays}d ago`;
                
                // Show formatted date for older timestamps
                return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { 
                    hour: '2-digit', 
                    minute: '2-digit' 
                });
            } catch (e) {
                return timestamp;
            }
        },
        
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
        
        // Inspector callback
        onToolsDiscovered(serverName, tools) {
            this.serverTools[serverName] = tools;
        },
        
        // UI Methods
        viewServerLogs(serverName) {
            this.activeTab = 'logs';
            this.$nextTick(() => {
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
            const tools = this.serverTools[server.name];
            return tools ? tools.length : (server.configCapabilities ? server.configCapabilities.length : 0);
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
        
        getHealthStatusClass(status) {
            switch (status) {
                case 'healthy': return 'text-green-600 bg-green-100 border-green-200 dark:text-green-400 dark:bg-green-900/20 dark:border-green-800';
                case 'running': return 'text-blue-600 bg-blue-100 border-blue-200 dark:text-blue-400 dark:bg-blue-900/20 dark:border-blue-800';
                case 'stopped': return 'text-gray-600 bg-gray-100 border-gray-200 dark:text-gray-400 dark:bg-gray-900/20 dark:border-gray-800';
                case 'error': return 'text-red-600 bg-red-100 border-red-200 dark:text-red-400 dark:bg-red-900/20 dark:border-red-800';
                default: return 'text-yellow-600 bg-yellow-100 border-yellow-200 dark:text-yellow-400 dark:bg-yellow-900/20 dark:border-yellow-800';
            }
        },
        
        async serverAction(action, serverName) {
            try {
                this.loading = true;
                await this.apiCall(`/api/servers/${action}`, {
                    method: 'POST',
                    body: JSON.stringify({ server: serverName })
                });
                
                this.showToast(`Server ${serverName} ${action}ed successfully`, 'success');
                setTimeout(() => this.loadData(), 2000);
                
            } catch (err) {
                this.error = `Failed to ${action} server ${serverName}: ${err.message}`;
                this.showToast(this.error, 'error');
            } finally {
                this.loading = false;
            }
        },
        
        async reloadProxy() {
            const confirmed = confirm('Restart Proxy?\n\nThis will drop all active connections and reload configuration.');
            if (!confirmed) return;
            
            try {
                this.loading = true;
                await this.apiCall('/api/proxy/reload', { method: 'POST' });
                this.showToast('Proxy restarted successfully', 'success');
                setTimeout(() => this.loadData(), 2000);
            } catch (err) {
                this.error = `Failed to restart proxy: ${err.message}`;
                this.showToast(this.error, 'error');
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
        
        checkMobileView() {
            this.isMobileView = window.innerWidth < 768;
        },
        
        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },
    
    mounted() {
        this.loadData();
        this.checkMobileView();
        
        window.addEventListener('resize', this.checkMobileView);
        
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
        window.removeEventListener('resize', this.checkMobileView);
    },
    
    template: `
        <div class="min-h-screen bg-gray-50 dark:bg-gray-900">
            <!-- Enhanced Header -->
            <header class="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700 sticky top-0 z-50">
                <div class="px-3 sm:px-4 lg:px-6">
                    <div class="flex justify-between items-center h-14">
                        <!-- Logo and Title -->
                        <div class="flex items-center space-x-3">
                            <div class="w-8 h-8 bg-gradient-to-r from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                            <div class="hidden sm:block">
                                <h1 class="text-lg font-semibold text-gray-900 dark:text-white">MCP Dashboard</h1>
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
                                            'relative inline-flex items-center px-2 sm:px-3 py-2 rounded-l-lg border text-xs sm:text-sm font-medium focus:z-10 focus:outline-none focus:ring-1 focus:ring-blue-500 disabled:opacity-50 transition-all touch-target',
                                            autoRefresh
                                                ? 'border-green-300 dark:border-green-600 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-200'
                                                : 'border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 text-gray-700 dark:text-gray-200'
                                        ]"
                                    >
                                        <svg class="w-4 h-4 sm:mr-2" :class="{ 'animate-spin': loading }" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                        </svg>
                                        <span class="hidden sm:inline">{{ autoRefresh ? 'Auto' : 'Refresh' }}</span>
                                        <span v-if="autoRefresh" class="absolute -top-1 -right-1 w-2 h-2 bg-green-500 border border-white dark:border-gray-800 rounded-full animate-pulse"></span>
                                    </button>
                                    <button
                                        @click="showRefreshDropdown = !showRefreshDropdown"
                                        :class="[
                                            'relative inline-flex items-center px-2 py-2 rounded-r-lg border border-l-0 text-xs sm:text-sm font-medium focus:z-10 focus:outline-none transition-colors touch-target',
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
                                                        'w-full text-left px-3 py-2 text-sm rounded-md transition-colors touch-target',
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
                                class="touch-target inline-flex items-center px-2 sm:px-3 py-2 border border-transparent text-xs sm:text-sm leading-4 font-medium rounded-lg text-white bg-orange-600 hover:bg-orange-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-orange-500 disabled:opacity-50 transition-colors shadow-sm"
                            >
                                <svg class="w-4 h-4 sm:mr-2" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                </svg>
                                <span class="hidden sm:inline">Restart</span>
                            </button>
                        </div>
                    </div>
                </div>
            </header>
            
            <!-- Mobile-First Navigation Tabs -->
            <nav class="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 sticky top-14 z-40">
                <div class="px-3 sm:px-4 lg:px-6">
                    <div class="mobile-nav py-2">
                        <button
                            v-for="tab in tabs"
                            :key="tab.id"
                            @click="activeTab = tab.id"
                            :class="[
                                'mobile-nav-item touch-target flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-colors',
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
            <main class="px-3 sm:px-4 lg:px-6 py-4">
                <!-- Error Display -->
                <div v-if="error" class="mb-4 bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg animate-fade-in">
                    <div class="flex items-start">
                        <div class="flex-shrink-0">
                            <svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <div class="ml-3 flex-1">
                            <h3 class="text-sm font-medium text-red-800 dark:text-red-200">Dashboard Error</h3>
                            <div class="mt-2 text-sm text-red-700 dark:text-red-300">{{ error }}</div>
                            <button @click="error = ''" class="mt-3 text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline touch-target">
                                Dismiss
                            </button>
                        </div>
                    </div>
                </div>
                
                <!-- Overview Tab Content -->
                <div v-if="activeTab === 'overview'" class="space-y-6 animate-fade-in">
                    <!-- Enhanced Stats Overview -->
                    <div class="responsive-grid cols-2 sm:cols-3 lg:cols-5 gap-3 sm:gap-4">
                        <!-- Total Servers -->
                        <div class="enhanced-card p-3 sm:p-4">
                            <div class="flex items-center">
                                <div class="flex-shrink-0">
                                    <div class="w-8 h-8 bg-gradient-to-r from-blue-400 to-blue-600 rounded-lg flex items-center justify-center">
                                        <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path>
                                        </svg>
                                    </div>
                                </div>
                                <div class="ml-3 flex-1 min-w-0">
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">Total</p>
                                    <p class="text-lg font-bold text-gray-900 dark:text-white">{{ statusCounts.total }}</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Running -->
                        <div class="enhanced-card p-3 sm:p-4">
                            <div class="flex items-center">
                                <div class="flex-shrink-0">
                                    <div class="w-8 h-8 bg-gradient-to-r from-green-400 to-green-600 rounded-lg flex items-center justify-center">
                                        <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                        </svg>
                                    </div>
                                </div>
                                <div class="ml-3 flex-1 min-w-0">
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">Running</p>
                                    <p class="text-lg font-bold text-gray-900 dark:text-white">{{ statusCounts.running }}</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Healthy -->
                        <div class="enhanced-card p-3 sm:p-4">
                            <div class="flex items-center">
                                <div class="flex-shrink-0">
                                    <div class="w-8 h-8 bg-gradient-to-r from-emerald-400 to-emerald-600 rounded-lg flex items-center justify-center">
                                        <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                        </svg>
                                    </div>
                                </div>
                                <div class="ml-3 flex-1 min-w-0">
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">Healthy</p>
                                    <p class="text-lg font-bold text-gray-900 dark:text-white">{{ statusCounts.healthy }}</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Proxy Uptime -->
                        <div class="col-span-2 sm:col-span-1 enhanced-card p-3 sm:p-4">
                            <div class="flex items-center">
                                <div class="flex-shrink-0">
                                    <div class="w-8 h-8 bg-gradient-to-r from-purple-400 to-purple-600 rounded-lg flex items-center justify-center">
                                        <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd"></path>
                                        </svg>
                                    </div>
                                </div>
                                <div class="ml-3 flex-1 min-w-0">
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">Uptime</p>
                                    <p class="text-lg font-bold text-gray-900 dark:text-white truncate">{{ formatUptime(status.proxyUptime) }}</p>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Active Connections -->
                        <div class="col-span-2 sm:col-span-2 lg:col-span-1 enhanced-card p-3 sm:p-4">
                            <div class="flex items-center">
                                <div class="flex-shrink-0">
                                    <div class="w-8 h-8 bg-gradient-to-r from-indigo-400 to-indigo-600 rounded-lg flex items-center justify-center">
                                        <svg class="w-4 h-4 text-white" fill="currentColor" viewBox="0 0 20 20">
                                            <path d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                                        </svg>
                                    </div>
                                </div>
                                <div class="ml-3 flex-1 min-w-0">
                                    <p class="text-xs font-medium text-gray-500 dark:text-gray-400 truncate">Active</p>
                                    <p class="text-lg font-bold text-gray-900 dark:text-white">{{ status.activeHttpConnectionsToServers || 0 }}</p>
                                </div>
                            </div>
                        </div>
                    </div>
                    
                    <!-- Enhanced Search and Filter Controls -->
                    <div class="enhanced-card p-4 lg:p-6">
                        <div class="flex flex-col space-y-3 lg:flex-row lg:items-center lg:justify-between lg:space-y-0">
                            <div class="flex-1 max-w-lg">
                                <div class="relative">
                                    <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                        <svg class="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                                        </svg>
                                    </div>
                                    <input
                                        v-model="searchTerm"
                                        type="text"
                                        placeholder="Search servers..."
                                        class="form-input pl-10 w-full"
                                    >
                                </div>
                            </div>
                            
                            <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                                <!-- Filter -->
                                <select
                                    v-model="filterStatus"
                                    class="form-input w-full sm:w-auto"
                                >
                                    <option value="all">All ({{ statusCounts.total }})</option>
                                    <option value="running">Running ({{ statusCounts.running }})</option>
                                    <option value="stopped">Stopped ({{ statusCounts.stopped }})</option>
                                    <option value="healthy">Healthy ({{ statusCounts.healthy }})</option>
                                </select>
                                
                                <!-- Sort -->
                                <select
                                    v-model="sortBy"
                                    class="form-input w-full sm:w-auto"
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
                    
                    <!-- Enhanced Server Accordions -->
                    <div v-else class="space-y-3">
                        <div
                            v-for="server in filteredAndSortedServers"
                            :key="server.name"
                            class="enhanced-card overflow-hidden transition-all duration-200"
                            :class="{ 'ring-2 ring-blue-500 ring-opacity-50': isServerExpanded(server.name) }"
                        >
                            <!-- Server Header (Accordion Trigger) -->
                            <div
                                @click="toggleServerExpansion(server.name)"
                                class="p-4 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors"
                            >
                                <div class="flex items-center justify-between">
                                    <div class="flex items-center space-x-3 min-w-0 flex-1">
                                        <!-- Enhanced Status Indicators -->
                                        <div class="flex-shrink-0 relative">
                                            <div :class="[
                                                'w-3 h-3 rounded-full',
                                                isServerHealthy(server) ? 'bg-green-500' :
                                                isContainerRunning(server) ? 'bg-blue-500' :
                                                'bg-gray-400'
                                            ]"></div>
                                            <div v-if="isServerHealthy(server)" class="absolute inset-0 w-3 h-3 bg-green-400 rounded-full animate-ping opacity-75"></div>
                                        </div>
                                        
                                        <!-- Server Info -->
                                        <div class="min-w-0 flex-1">
                                            <div class="flex items-center space-x-2 mb-1">
                                                <h3 class="text-base font-semibold text-gray-900 dark:text-white truncate">
                                                    {{ server.name }}
                                                </h3>
                                                <span v-if="getServerToolCount(server) > 0" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200">
                                                    {{ getServerToolCount(server) }} tool{{ getServerToolCount(server) !== 1 ? 's' : '' }}
                                                </span>
                                            </div>
                                            <div class="flex items-center space-x-3 text-xs text-gray-500 dark:text-gray-400">
                                                <span>{{ server.configProtocol || 'stdio' }}</span>
                                                <span v-if="server.configHttpPort">Port {{ server.configHttpPort }}</span>
                                            </div>
                                        </div>
                                        
                                        <!-- Status Badges -->
                                        <div class="flex items-center space-x-2">
                                            <span :class="[
                                                'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
                                                isContainerRunning(server)
                                                    ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                                    : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                            ]">
                                                {{ server.containerStatus || 'Unknown' }}
                                            </span>
                                            
                                            <span :class="[
                                                'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border',
                                                getHealthStatusClass(isServerHealthy(server) ? 'healthy' : 'disconnected')
                                            ]">
                                                {{ getConnectionStatus(server) }}
                                            </span>
                                        </div>
                                    </div>
                                    
                                    <!-- Expand/Collapse Button -->
                                    <div class="ml-2">
                                        <div v-if="!isServerExpanded(server.name)" class="flex items-center space-x-2" @click.stop>
                                            <button
                                                @click="viewServerLogs(server.name)"
                                                class="text-xs px-2 py-1 text-gray-600 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200 transition-colors touch-target"
                                                title="View Logs"
                                            >
                                                Logs
                                            </button>
                                        </div>
                                        <button class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors p-1 ml-2 touch-target">
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
                                    <!-- Connection Status Section -->
                                    <div class="mb-6">
                                        <h4 class="text-sm font-medium text-gray-900 dark:text-white mb-3 flex items-center">
                                            <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                <path fill-rule="evenodd" d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                            </svg>
                                            Server Status
                                        </h4>
                                        
                                        <div v-if="getHttpConnection(server)" class="bg-white dark:bg-gray-800 p-3 rounded-lg space-y-2 text-sm">
                                            <div class="flex justify-between items-center">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Proxy Status:</span>
                                                <span :class="[
                                                    'px-2 py-1 rounded text-xs font-medium',
                                                    getHttpConnection(server).initialized && getHttpConnection(server).rawHealthyFlag
                                                        ? 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
                                                        : 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
                                                ]">
                                                    {{ getConnectionStatus(server) }}
                                                </span>
                                            </div>
                                            <div class="flex justify-between items-start">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Target URL:</span>
                                                <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded break-all max-w-xs">
                                                    {{ getHttpConnection(server).targetBaseURL }}
                                                </code>
                                            </div>
                                            <div v-if="getHttpConnection(server).lastUsedByProxy" class="flex justify-between items-center">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Last Used:</span>
                                                <span class="text-gray-700 dark:text-gray-300 text-xs">
                                                    {{ formatTimestamp(getHttpConnection(server).lastUsedByProxy) }}
                                                </span>
                                            </div>
                                        </div>
                                        <div v-else class="text-center py-6 text-gray-500 dark:text-gray-400">
                                            <svg class="w-8 h-8 mx-auto mb-2 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192L5.636 18.364M12 12l2.828 2.828M12 12l2.828-2.828M12 12L9.172 9.172M12 12l-2.828 2.828"></path>
                                            </svg>
                                            <p class="text-sm">No active proxy connection</p>
                                        </div>
                                    </div>
                                    
                                    <!-- Configuration & Tools -->
                                    <div class="responsive-grid cols-1 lg:cols-2 gap-4 mb-6">
                                        <!-- Configuration -->
                                        <div class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                            <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3 text-sm">Configuration</h5>
                                            <div class="space-y-2 text-sm">
                                                <div class="flex justify-between">
                                                    <span class="text-gray-500 dark:text-gray-400">Protocol:</span>
                                                    <span class="text-gray-700 dark:text-gray-300">{{ server.configProtocol || 'stdio' }}</span>
                                                </div>
                                                <div v-if="server.configHttpPort" class="flex justify-between">
                                                    <span class="text-gray-500 dark:text-gray-400">HTTP Port:</span>
                                                    <span class="text-gray-700 dark:text-gray-300">{{ server.configHttpPort }}</span>
                                                </div>
                                                <div class="flex justify-between">
                                                    <span class="text-gray-500 dark:text-gray-400">Container:</span>
                                                    <span class="text-gray-700 dark:text-gray-300">{{ server.isContainer ? 'Yes' : 'No' }}</span>
                                                </div>
                                                <div v-if="server.image" class="flex justify-between">
                                                    <span class="text-gray-500 dark:text-gray-400">Image:</span>
                                                    <code class="text-xs bg-gray-100 dark:bg-gray-700 px-2 py-1 rounded">{{ server.image }}</code>
                                                </div>
                                            </div>
                                        </div>
                                        
                                        <!-- Capabilities & Tools -->
                                        <div class="bg-white dark:bg-gray-800 p-3 rounded-lg">
                                            <h5 class="font-medium text-gray-700 dark:text-gray-300 mb-3 text-sm">Tools & Capabilities</h5>
                                            <div v-if="serverTools[server.name] && serverTools[server.name].length > 0">
                                                <div class="space-y-2 mb-3">
                                                    <div
                                                        v-for="tool in serverTools[server.name].slice(0, 3)"
                                                        :key="tool.name"
                                                        class="text-sm"
                                                    >
                                                        <div class="font-medium text-gray-900 dark:text-white">{{ tool.name }}</div>
                                                        <div v-if="tool.description" class="text-xs text-gray-500 dark:text-gray-400 truncate">
                                                            {{ tool.description }}
                                                        </div>
                                                    </div>
                                                </div>
                                                <div v-if="serverTools[server.name].length > 3" class="text-xs text-gray-500 dark:text-gray-400">
                                                    +{{ serverTools[server.name].length - 3 }} more tools
                                                </div>
                                            </div>
                                            <div v-else-if="Object.keys(getServerCapabilities(server)).length > 0">
                                                <div class="flex flex-wrap gap-1">
                                                    <span
                                                        v-for="(value, capability) in getServerCapabilities(server)"
                                                        :key="capability"
                                                        class="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200"
                                                    >
                                                        {{ capability }}
                                                    </span>
                                                </div>
                                                <p class="text-xs text-gray-500 dark:text-gray-400 mt-2">{{ getServerToolCount(server) || 0 }} tools available</p>
                                            </div>
                                            <div v-else class="text-sm text-gray-500 dark:text-gray-400">
                                                No capabilities reported
                                            </div>
                                        </div>
                                    </div>
                                    
                                    <!-- Integrated MCP Inspector -->
                                    <div class="mb-6">
                                        <mcp-inspector 
                                            :server-name="server.name"
                                            :server-config="server"
                                            @tools-discovered="(tools) => onToolsDiscovered(server.name, tools)"
                                        ></mcp-inspector>
                                    </div>
                                    
                                    <!-- Action Buttons -->
                                    <div class="space-y-3">
                                        <!-- Primary Actions -->
                                        <div class="responsive-grid cols-3 gap-2">
                                            <button
                                                v-if="!isContainerRunning(server)"
                                                @click="serverAction('start', server.name)"
                                                :disabled="loading"
                                                class="touch-target flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-green-600 hover:bg-green-700 disabled:bg-gray-400 transition-colors"
                                            >
                                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                                                </svg>
                                                Start
                                            </button>
                                            
                                            <button
                                                v-if="isContainerRunning(server)"
                                                @click="serverAction('stop', server.name)"
                                                :disabled="loading"
                                                class="touch-target flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-red-600 hover:bg-red-700 disabled:bg-gray-400 transition-colors"
                                            >
                                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                                                </svg>
                                                Stop
                                            </button>
                                            
                                            <button
                                                v-if="isContainerRunning(server)"
                                                @click="serverAction('restart', server.name)"
                                                :disabled="loading"
                                                class="touch-target flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-white bg-yellow-600 hover:bg-yellow-700 disabled:bg-gray-400 transition-colors"
                                            >
                                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                                                </svg>
                                                Restart
                                            </button>
                                            
                                            <button
                                                @click="viewServerLogs(server.name)"
                                                class="touch-target flex items-center justify-center px-3 py-2 text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                                            >
                                                View Logs
                                            </button>
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
                
                <activity-viewer
                    v-if="activeTab === 'activity'"
                    :config="config"
                ></activity-viewer>
            </main>
        </div>
    `
};