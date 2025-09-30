const ActivityViewer = {
    props: ['config'],
    data() {
        return {
            activities: [],
            historicalActivities: [],
            websocket: null,
            connectionStatus: 'disconnected',
            showDetails: {},
            expandedToolCalls: {},
            levelFilter: '',
            typeFilter: '',
            searchFilter: '',
            activityStats: {
                total: 0,
                requests: 0,
                errors: 0,
                connections: 0,
                toolCalls: 0
            },
            historicalStats: {
                totalToday: 0,
                requestsToday: 0,
                errorsToday: 0,
                toolCallsToday: 0
            },
            loading: false
        }
    },
    computed: {
        allActivities() {
            // Merge historical and real-time activities, avoiding duplicates
            const combined = [...this.historicalActivities, ...this.activities];
            const unique = combined.filter((activity, index, self) =>
                index === self.findIndex(a => a.id === activity.id)
            );
            
            // Sort in reverse chronological order (newest first)
            return unique.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
        },
        availableLevels() {
            const levels = new Set(this.allActivities.map(a => a.level).filter(Boolean));
            return Array.from(levels).sort();
        },
        availableTypes() {
            const types = new Set(this.allActivities.map(a => a.type).filter(Boolean));
            return Array.from(types).sort();
        },
        filteredActivities() {
            return this.allActivities.filter(activity => {
                const matchesLevel = !this.levelFilter || activity.level === this.levelFilter;
                const matchesType = !this.typeFilter || activity.type === this.typeFilter;
                const matchesSearch = !this.searchFilter || 
                    activity.message.toLowerCase().includes(this.searchFilter.toLowerCase()) ||
                    (activity.server && activity.server.toLowerCase().includes(this.searchFilter.toLowerCase()));
                
                return matchesLevel && matchesType && matchesSearch;
            });
        },
        uniqueServers() {
            const servers = new Set(this.allActivities.map(a => a.server).filter(Boolean));
            return Array.from(servers).sort();
        },
        uniqueTypes() {
            const types = new Set(this.allActivities.map(a => a.type).filter(Boolean));
            return Array.from(types).sort();
        },
        combinedStats() {
            return {
                total: this.activityStats.total + this.historicalStats.totalToday,
                requests: this.activityStats.requests + this.historicalStats.requestsToday,
                errors: this.activityStats.errors + this.historicalStats.errorsToday,
                toolCalls: this.activityStats.toolCalls + this.historicalStats.toolCallsToday,
                connections: this.activityStats.connections
            };
        }
    },
    methods: {
        async loadHistoricalActivities() {
            this.loading = true;
            try {
                // Load last 6 hours of historical data  
                const response = await this.apiCall('/api/activity/history?hours=6');
                this.historicalActivities = response.activities.map(activity => ({
                    ...activity,
                    id: activity.id || activity.activity_id || `hist-${Date.now()}-${Math.random()}`,
                    isHistorical: true,
                    toolCalls: this.extractToolCalls(activity)
                }));

                // Load today's stats
                const statsResponse = await this.apiCall('/api/activity/stats');
                this.historicalStats = statsResponse;

                console.log('Loaded historical activities:', this.historicalActivities.length);
            } catch (err) {
                console.warn('Failed to load historical activities:', err);
                
                if (err.message.includes('503')) {
                    this.showToast('Activity storage not configured - configure PostgreSQL URL for persistent storage', 'info');
                } else if (err.message.includes('404')) {
                    this.showToast('Activity history not available in this version', 'info');
                } else {
                    this.showToast('Failed to load historical activities', 'warning');
                }
                
                this.historicalActivities = [];
                this.historicalStats = {
                    totalToday: 0,
                    requestsToday: 0,
                    errorsToday: 0,
                    toolCallsToday: 0
                };
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
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        },

        connectToActivityStream() {
            if (this.wsConnection) {
                this.wsConnection.close();
            }
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws/activity`;
            this.wsConnection = new WebSocket(wsUrl);
            this.wsConnection.onopen = () => {
                console.log('Activity stream connected');
                this.connected = true;
                this.showToast('Connected to activity stream', 'success');
            };
            this.wsConnection.onmessage = (event) => {
                try {
                    const activity = JSON.parse(event.data);
                    this.addActivity(activity);
                } catch (err) {
                    console.error('Failed to parse activity message:', err);
                }
            };
            this.wsConnection.onclose = () => {
                console.log('Activity stream disconnected');
                this.connected = false;
                setTimeout(() => this.connectToActivityStream(), 3000);
            };
            this.wsConnection.onerror = (err) => {
                console.error('Activity WebSocket error:', err);
                this.connected = false;
                this.showToast('Connection error - attempting to reconnect', 'error');
            };
        },

        addActivity(activity) {
            const enrichedActivity = {
                ...activity,
                id: activity.id || `live-${Date.now()}-${Math.random()}`,
                timestamp: activity.timestamp || new Date().toISOString(),
                toolCalls: this.extractToolCalls(activity),
                isHistorical: false
            };
            
            this.activities.unshift(enrichedActivity);
            this.updateStats(enrichedActivity);
            
            // Keep only the latest real-time activities
            if (this.activities.length > 500) {
                this.activities = this.activities.slice(0, 500);
            }
            
            // Auto scroll if enabled
            if (this.autoScroll) {
                this.$nextTick(() => this.scrollToTop());
            }
        },

        extractToolCalls(activity) {
            const toolCalls = [];
            
            if (activity.details) {
                // Check for direct tool calls
                if (activity.details.toolCall || activity.details.tool_call) {
                    const toolCall = activity.details.toolCall || activity.details.tool_call;
                    toolCalls.push({
                        tool: toolCall.tool || toolCall.name || 'Unknown Tool',
                        arguments: toolCall.arguments || toolCall.args,
                        result: toolCall.result
                    });
                }
                
                // Check for tool calls in message details
                if (activity.details.tools) {
                    activity.details.tools.forEach(tool => {
                        toolCalls.push({
                            tool: tool.name || 'Unknown Tool',
                            arguments: tool.arguments,
                            result: tool.result
                        });
                    });
                }
            }
            
            return toolCalls;
        },

        updateStats(activity) {
            if (activity.isHistorical) return; // Don't double-count historical
            
            this.activityStats.total++;
            switch (activity.type) {
                case 'request':
                    this.activityStats.requests++;
                    break;
                case 'connection':
                    this.activityStats.connections++;
                    break;
                case 'tool':
                    this.activityStats.toolCalls++;
                    break;
            }
            if (activity.level === 'ERROR') {
                this.activityStats.errors++;
            }
            if (activity.toolCalls && activity.toolCalls.length > 0) {
                this.activityStats.toolCalls += activity.toolCalls.length;
            }
        },

        searchInToolData(activity) {
            if (!activity.toolCalls || activity.toolCalls.length === 0) return false;
            const searchLower = this.searchTerm.toLowerCase();
            return activity.toolCalls.some(tool => {
                return tool.name?.toLowerCase().includes(searchLower) ||
                       JSON.stringify(tool.parameters || {}).toLowerCase().includes(searchLower) ||
                       JSON.stringify(tool.content || {}).toLowerCase().includes(searchLower);
            });
        },

        toggleToolCall(activityId, toolCallId) {
            const key = `${activityId}-${toolCallId}`;
            this.expandedToolCalls[key] = !this.expandedToolCalls[key];
            this.$forceUpdate();
        },

        isToolCallExpanded(activityId, toolCallId) {
            const key = `${activityId}-${toolCallId}`;
            return !!this.expandedToolCalls[key];
        },

        formatToolParameters(params) {
            return JSON.stringify(params, null, 2);
        },

        formatToolResult(result) {
            if (typeof result === 'string') return result;
            return JSON.stringify(result, null, 2);
        },

        scrollToTop() {
            const container = this.$refs.activityContainer;
            if (container) {
                container.scrollTop = 0;
            }
        },

        clearActivities() {
            this.activities = [];
            this.historicalActivities = [];
            this.activityStats = { total: 0, requests: 0, errors: 0, connections: 0, toolCalls: 0 };
            this.historicalStats = { totalToday: 0, requestsToday: 0, errorsToday: 0, toolCallsToday: 0 };
            this.expandedToolCalls = {};
            this.showToast('Activity feed cleared', 'info');
        },

        toggleDetails(activityId) {
            this.showDetails[activityId] = !this.showDetails[activityId];
            this.$forceUpdate();
        },

        getLevelClass(level) {
            switch (level) {
                case 'ERROR': return 'border-red-500 bg-red-50 dark:bg-red-900/20 text-red-700 dark:text-red-300';
                case 'WARN': return 'border-yellow-500 bg-yellow-50 dark:bg-yellow-900/20 text-yellow-700 dark:text-yellow-300';
                case 'INFO': return 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 text-blue-700 dark:text-blue-300';
                case 'DEBUG': return 'border-gray-500 bg-gray-50 dark:bg-gray-900/20 text-gray-700 dark:text-gray-300';
                default: return 'border-green-500 bg-green-50 dark:bg-green-900/20 text-green-700 dark:text-green-300';
            }
        },
        getLevelBadgeClass(level) {
            const classes = {
                'ERROR': 'bg-red-600 text-white',
                'WARN': 'bg-yellow-600 text-black', 
                'INFO': 'bg-blue-600 text-white',
                'DEBUG': 'bg-gray-600 text-white'
            };
            return classes[level] || 'bg-gray-600 text-white';
        },

        getTypeBadgeClass(type) {
            const classes = {
                'request': 'bg-green-600 text-white',
                'tool_call': 'bg-purple-600 text-white',
                'tool': 'bg-purple-600 text-white', 
                'connection': 'bg-blue-600 text-white',
                'task': 'bg-indigo-600 text-white',
                'error': 'bg-red-600 text-white'
            };
            return classes[type] || 'bg-gray-600 text-white';
        },
        getTypeIcon(type) {
            switch (type) {
                case 'request':
                    return 'M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h8a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293L12 17';
                case 'connection':
                    return 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1';
                case 'error':
                    return 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z';
                case 'tool':
                    return 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z';
                default:
                    return 'M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1';
            }
        },

        formatTimestamp(timestamp) {
            try {
                const date = new Date(timestamp);
                const now = new Date();
                const diffMs = now - date;
                const diffSecs = Math.floor(diffMs / 1000);
                const diffMins = Math.floor(diffMs / (1000 * 60));
                const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
                const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

                if (diffSecs < 60) return `${diffSecs}s ago`;
                if (diffMins < 60) return `${diffMins}m ago`;
                if (diffHours < 24) return `${diffHours}h ago`;
                if (diffDays < 7) return `${diffDays}d ago`;
                
                return date.toLocaleDateString();
            } catch (err) {
                return timestamp;
            }
        },

        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },

    async mounted() {
        await this.loadHistoricalActivities();
        this.connectToActivityStream();
    },

    beforeUnmount() {
        if (this.wsConnection) {
            this.wsConnection.close();
        }
    },

    template: `
    <div class="activity-viewer space-y-6 animate-fade-in max-w-full overflow-x-hidden">
        <!-- Modern Header with Gradient -->
        <div class="enhanced-card p-6 lg:p-8 bg-gradient-to-br from-gray-800 via-gray-850 to-gray-900 border-gray-700">
            <div class="flex flex-col lg:flex-row lg:items-start lg:justify-between space-y-6 lg:space-y-0 lg:space-x-8">
                <div class="flex items-start space-x-4">
                    <div class="flex-shrink-0">
                        <div class="w-14 h-14 bg-gradient-to-br from-blue-500 to-cyan-600 rounded-2xl flex items-center justify-center shadow-lg">
                            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                            </svg>
                        </div>
                    </div>
                    <div>
                        <h2 class="text-2xl lg:text-3xl font-bold text-white mb-2 tracking-tight">
                            Activity Monitor
                        </h2>
                        <p class="text-gray-400 text-sm lg:text-base">
                            Real-time activity stream with 6-hour persistent history
                        </p>
                        <div class="flex items-center space-x-2 mt-3">
                            <div class="flex items-center space-x-1.5">
                                <div class="w-2 h-2 bg-green-500 rounded-full animate-pulse"></div>
                                <span class="text-xs text-gray-400">Live</span>
                            </div>
                            <span class="text-gray-600">•</span>
                            <span class="text-xs text-gray-400">{{ filteredActivities.length }} events</span>
                        </div>
                    </div>
                </div>

                <!-- Modern Stats Grid -->
                <div class="grid grid-cols-2 lg:grid-cols-4 gap-3 lg:gap-4">
                    <div class="bg-gray-800/60 backdrop-blur-sm border border-gray-700/50 rounded-xl p-4 hover:border-blue-500/30 transition-all duration-200">
                        <div class="flex items-center space-x-2 mb-1">
                            <div class="w-2 h-2 bg-blue-500 rounded-full"></div>
                            <div class="text-xs font-medium text-gray-400 uppercase tracking-wider">Total</div>
                        </div>
                        <div class="text-2xl font-bold text-white">{{ combinedStats.total }}</div>
                    </div>
                    <div class="bg-gray-800/60 backdrop-blur-sm border border-gray-700/50 rounded-xl p-4 hover:border-green-500/30 transition-all duration-200">
                        <div class="flex items-center space-x-2 mb-1">
                            <div class="w-2 h-2 bg-green-500 rounded-full"></div>
                            <div class="text-xs font-medium text-gray-400 uppercase tracking-wider">Requests</div>
                        </div>
                        <div class="text-2xl font-bold text-white">{{ combinedStats.requests }}</div>
                    </div>
                    <div class="bg-gray-800/60 backdrop-blur-sm border border-gray-700/50 rounded-xl p-4 hover:border-purple-500/30 transition-all duration-200">
                        <div class="flex items-center space-x-2 mb-1">
                            <div class="w-2 h-2 bg-purple-500 rounded-full"></div>
                            <div class="text-xs font-medium text-gray-400 uppercase tracking-wider">Tools</div>
                        </div>
                        <div class="text-2xl font-bold text-white">{{ combinedStats.toolCalls }}</div>
                    </div>
                    <div class="bg-gray-800/60 backdrop-blur-sm border border-gray-700/50 rounded-xl p-4 hover:border-red-500/30 transition-all duration-200">
                        <div class="flex items-center space-x-2 mb-1">
                            <div class="w-2 h-2 bg-red-500 rounded-full"></div>
                            <div class="text-xs font-medium text-gray-400 uppercase tracking-wider">Errors</div>
                        </div>
                        <div class="text-2xl font-bold text-white">{{ combinedStats.errors }}</div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Modern Filters and Controls -->
        <div class="enhanced-card p-5 border-gray-700/50">
            <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0 gap-4">
                <div class="flex flex-col sm:flex-row flex-1 space-y-3 sm:space-y-0 sm:space-x-3">
                    <!-- Level Filter -->
                    <div class="relative flex-1 sm:flex-initial sm:min-w-[140px]">
                        <label for="levelFilter" class="block text-xs font-medium text-gray-400 mb-1.5">Level</label>
                        <select v-model="levelFilter" id="levelFilter"
                               class="form-input w-full px-3 py-2 text-sm bg-gray-800 border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all">
                            <option value="">All Levels</option>
                            <option v-for="level in availableLevels" :key="level" :value="level">
                                {{ level }}
                            </option>
                        </select>
                    </div>

                    <!-- Type Filter -->
                    <div class="relative flex-1 sm:flex-initial sm:min-w-[140px]">
                        <label for="typeFilter" class="block text-xs font-medium text-gray-400 mb-1.5">Type</label>
                        <select v-model="typeFilter" id="typeFilter"
                               class="form-input w-full px-3 py-2 text-sm bg-gray-800 border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all">
                            <option value="">All Types</option>
                            <option v-for="type in availableTypes" :key="type" :value="type">
                                {{ type }}
                            </option>
                        </select>
                    </div>

                    <!-- Search -->
                    <div class="relative flex-1 sm:min-w-[200px]">
                        <label for="searchFilter" class="block text-xs font-medium text-gray-400 mb-1.5">Search</label>
                        <div class="relative">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                                </svg>
                            </div>
                            <input v-model="searchFilter" id="searchFilter" type="text" placeholder="Search activities..."
                                   class="form-input w-full pl-10 pr-3 py-2 text-sm bg-gray-800 border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-all">
                        </div>
                    </div>
                </div>

                <!-- Control Buttons -->
                <div class="flex space-x-2">
                    <button @click="loadHistoricalActivities"
                           :disabled="loading"
                           class="inline-flex items-center px-4 py-2 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-all duration-200 shadow-lg hover:shadow-blue-500/20">
                        <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                        </svg>
                        {{ loading ? 'Loading...' : 'Refresh' }}
                    </button>
                    <button @click="clearActivities"
                           class="inline-flex items-center px-4 py-2 bg-gray-700 hover:bg-red-600 text-white text-sm font-medium rounded-lg transition-all duration-200 shadow-lg hover:shadow-red-500/20">
                        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                        </svg>
                        Clear
                    </button>
                </div>
            </div>
        </div>

        <!-- Activity Feed with reverse chronological order -->
        <div class="enhanced-card border-gray-700/50 overflow-hidden">
            <div v-if="loading && allActivities.length === 0" class="p-12 text-center">
                <div class="inline-flex items-center justify-center w-16 h-16 bg-blue-500/10 rounded-2xl mb-4">
                    <svg class="w-8 h-8 text-blue-500 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                    </svg>
                </div>
                <p class="text-lg font-medium text-gray-300">Loading activities...</p>
                <p class="text-sm text-gray-500 mt-2">Fetching historical data</p>
            </div>

            <div v-else-if="filteredActivities.length === 0" class="p-12 text-center">
                <div class="inline-flex items-center justify-center w-16 h-16 bg-gray-700/50 rounded-2xl mb-4">
                    <svg class="w-8 h-8 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"></path>
                    </svg>
                </div>
                <p class="text-lg font-medium text-gray-300 mb-2">No activities found</p>
                <p class="text-sm text-gray-500">Try adjusting your filters or wait for new events</p>
            </div>

            <div v-else class="divide-y divide-gray-700/50">
                <div v-for="activity in filteredActivities" :key="activity.id"
                     class="p-4 hover:bg-gray-800/40 transition-all duration-150 group">
                    <div class="flex items-start justify-between">
                        <div class="flex items-start space-x-3 flex-1 min-w-0">
                            <!-- Level Indicator -->
                            <div class="flex-shrink-0 mt-1">
                                <div :class="[
                                    'w-8 h-8 rounded-lg flex items-center justify-center text-xs font-bold transition-all duration-200',
                                    getLevelBadgeClass(activity.level)
                                ]">
                                    {{ activity.level.charAt(0) }}
                                </div>
                            </div>

                            <!-- Content -->
                            <div class="flex-1 min-w-0">
                                <div class="flex items-center flex-wrap gap-2 mb-2">
                                    <div :class="[
                                        'inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium shadow-sm',
                                        getTypeBadgeClass(activity.type)
                                    ]">
                                        {{ activity.type || 'unknown' }}
                                    </div>
                                    <div v-if="activity.server" class="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-medium bg-gray-700 text-gray-300 border border-gray-600">
                                        <svg class="w-3 h-3 mr-1.5" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M2 5a2 2 0 012-2h12a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V5zm3.293 1.293a1 1 0 011.414 0l3 3a1 1 0 010 1.414l-3 3a1 1 0 01-1.414-1.414L7.586 10 5.293 7.707a1 1 0 010-1.414zM11 12a1 1 0 100 2h3a1 1 0 100-2h-3z" clip-rule="evenodd"></path>
                                        </svg>
                                        {{ activity.server }}
                                    </div>
                                    <div class="inline-flex items-center text-xs text-gray-400">
                                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd"></path>
                                        </svg>
                                        {{ formatTimestamp(activity.timestamp) }}
                                    </div>
                                </div>

                                <div class="text-white text-sm leading-relaxed mb-3">{{ activity.message }}</div>

                                <!-- Tool Calls -->
                                <div v-if="activity.toolCalls && activity.toolCalls.length > 0" class="space-y-2">
                                    <div v-for="(call, index) in activity.toolCalls" :key="index"
                                         class="bg-gradient-to-br from-purple-900/20 to-purple-800/10 border border-purple-500/30 rounded-lg p-3 text-sm hover:border-purple-500/50 transition-all duration-200">
                                        <div class="flex items-center justify-between mb-2">
                                            <div class="flex items-center space-x-2">
                                                <svg class="w-4 h-4 text-purple-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                                                </svg>
                                                <span class="font-semibold text-purple-300">{{ call.tool }}</span>
                                            </div>
                                            <button @click="toggleToolCall(activity.id + '-' + index)"
                                                   class="text-gray-400 hover:text-white transition-colors p-1 rounded hover:bg-gray-700/50">
                                                <svg class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-90': expandedToolCalls[activity.id + '-' + index] }" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"></path>
                                                </svg>
                                            </button>
                                        </div>
                                        <div v-if="expandedToolCalls[activity.id + '-' + index]" class="space-y-2 animate-fade-in">
                                            <div v-if="call.arguments" class="bg-gray-900/60 border border-gray-700/50 rounded-lg p-3">
                                                <div class="flex items-center space-x-1.5 mb-2">
                                                    <svg class="w-3 h-3 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z" clip-rule="evenodd"></path>
                                                    </svg>
                                                    <span class="text-xs font-medium text-blue-400">Arguments</span>
                                                </div>
                                                <pre class="text-xs text-gray-300 whitespace-pre-wrap font-mono">{{ JSON.stringify(call.arguments, null, 2) }}</pre>
                                            </div>
                                            <div v-if="call.result" class="bg-gray-900/60 border border-gray-700/50 rounded-lg p-3">
                                                <div class="flex items-center space-x-1.5 mb-2">
                                                    <svg class="w-3 h-3 text-green-400" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                                    </svg>
                                                    <span class="text-xs font-medium text-green-400">Result</span>
                                                </div>
                                                <pre class="text-xs text-gray-300 whitespace-pre-wrap font-mono">{{ typeof call.result === 'object' ? JSON.stringify(call.result, null, 2) : call.result }}</pre>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Details Toggle -->
                                <div v-if="activity.details && Object.keys(activity.details).length > 0" class="mt-3">
                                    <button @click="toggleDetails(activity.id)"
                                           class="inline-flex items-center px-3 py-1.5 text-xs font-medium text-blue-400 hover:text-blue-300 bg-blue-500/10 hover:bg-blue-500/20 border border-blue-500/30 rounded-lg transition-all duration-200">
                                        <svg class="w-3 h-3 mr-1.5 transition-transform duration-200" :class="{ 'rotate-90': showDetails[activity.id] }" fill="currentColor" viewBox="0 0 20 20">
                                            <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd"></path>
                                        </svg>
                                        {{ showDetails[activity.id] ? 'Hide Details' : 'Show Details' }}
                                    </button>
                                    <div v-if="showDetails[activity.id]" class="mt-3 bg-gray-900/60 border border-gray-700/50 rounded-lg p-4 animate-fade-in">
                                        <pre class="text-xs text-gray-300 whitespace-pre-wrap font-mono overflow-x-auto">{{ JSON.stringify(activity.details, null, 2) }}</pre>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
    `,
};