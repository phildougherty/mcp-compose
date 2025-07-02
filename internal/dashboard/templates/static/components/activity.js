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
    <div class="activity-viewer space-y-4 animate-fade-in max-w-full overflow-x-hidden">
        <!-- Enhanced Header with proper dark theme -->
        <div class="enhanced-card p-4 lg:p-6">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-4 sm:space-y-0">
                <div>
                    <h2 class="text-2xl font-bold text-white mb-2">
                        🔄 Activity Monitor
                    </h2>
                    <p class="text-gray-300">
                        Real-time activity feed with persistent history
                    </p>
                </div>
                
                <!-- Stats Cards -->
                <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 sm:gap-4">
                    <div class="enhanced-card p-3 text-center">
                        <div class="text-lg sm:text-xl font-bold text-white">{{ combinedStats.total }}</div>
                        <div class="text-xs sm:text-sm text-gray-300">Total Activities</div>
                    </div>
                    <div class="enhanced-card p-3 text-center">
                        <div class="text-lg sm:text-xl font-bold text-white">{{ combinedStats.requests }}</div>
                        <div class="text-xs sm:text-sm text-gray-300">Requests</div>
                    </div>
                    <div class="enhanced-card p-3 text-center">
                        <div class="text-lg sm:text-xl font-bold text-white">{{ combinedStats.toolCalls }}</div>
                        <div class="text-xs sm:text-sm text-gray-300">Tool Calls</div>
                    </div>
                    <div class="enhanced-card p-3 text-center">
                        <div class="text-lg sm:text-xl font-bold text-white">{{ combinedStats.errors }}</div>
                        <div class="text-xs sm:text-sm text-gray-300">Errors</div>
                    </div>
                </div>
            </div>
        </div>

        <!-- Filters and Controls -->
        <div class="enhanced-card p-4">
            <div class="flex flex-col sm:flex-row sm:items-center sm:justify-between space-y-3 sm:space-y-0">
                <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-4">
                    <!-- Level Filter -->
                    <div class="flex items-center space-x-2">
                        <label for="levelFilter" class="text-sm font-medium text-gray-300">Level:</label>
                        <select v-model="levelFilter" id="levelFilter" 
                               class="form-input px-3 py-1 text-sm bg-gray-700 border-gray-600 text-white rounded">
                            <option value="">All Levels</option>
                            <option v-for="level in availableLevels" :key="level" :value="level">
                                {{ level }}
                            </option>
                        </select>
                    </div>

                    <!-- Type Filter -->
                    <div class="flex items-center space-x-2">
                        <label for="typeFilter" class="text-sm font-medium text-gray-300">Type:</label>
                        <select v-model="typeFilter" id="typeFilter" 
                               class="form-input px-3 py-1 text-sm bg-gray-700 border-gray-600 text-white rounded">
                            <option value="">All Types</option>
                            <option v-for="type in availableTypes" :key="type" :value="type">
                                {{ type }}
                            </option>
                        </select>
                    </div>

                    <!-- Search -->
                    <div class="flex items-center space-x-2">
                        <label for="searchFilter" class="text-sm font-medium text-gray-300">Search:</label>
                        <input v-model="searchFilter" id="searchFilter" type="text" placeholder="Search activities..."
                               class="form-input px-3 py-1 text-sm bg-gray-700 border-gray-600 text-white rounded w-48">
                    </div>
                </div>

                <!-- Control Buttons -->
                <div class="flex space-x-2">
                    <button @click="loadHistoricalActivities" 
                           :disabled="loading"
                           class="btn bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded text-sm transition-colors">
                        <span v-if="loading">🔄</span>
                        <span v-else>🔄</span>
                        {{ loading ? 'Loading...' : 'Refresh' }}
                    </button>
                    <button @click="clearActivities" 
                           class="btn bg-red-600 hover:bg-red-700 text-white px-4 py-2 rounded text-sm transition-colors">
                        🗑️ Clear
                    </button>
                </div>
            </div>
        </div>

        <!-- Activity Feed with reverse chronological order -->
        <div class="enhanced-card">
            <div v-if="loading && allActivities.length === 0" class="p-8 text-center text-gray-400">
                <div class="text-4xl mb-4">🔄</div>
                <div>Loading activities...</div>
            </div>

            <div v-else-if="filteredActivities.length === 0" class="p-8 text-center text-gray-400">
                <div class="text-4xl mb-4">📭</div>
                <div>No activities found</div>
                <div class="text-sm mt-2">Try adjusting your filters or check back later</div>
            </div>

            <div v-else class="divide-y divide-gray-600">
                <div v-for="activity in filteredActivities" :key="activity.id" 
                     class="p-4 hover:bg-gray-700 transition-colors">
                    <div class="flex items-start justify-between">
                        <div class="flex items-start space-x-3 flex-1 min-w-0">
                            <!-- Level Badge -->
                            <div :class="[
                                'inline-flex items-center px-2 py-1 rounded text-xs font-medium flex-shrink-0',
                                getLevelBadgeClass(activity.level)
                            ]">
                                {{ activity.level }}
                            </div>

                            <!-- Content -->
                            <div class="flex-1 min-w-0">
                                <div class="flex items-center space-x-2 mb-1">
                                    <div :class="[
                                        'inline-flex items-center px-2 py-1 rounded text-xs font-medium',
                                        getTypeBadgeClass(activity.type)
                                    ]">
                                        {{ activity.type || 'unknown' }}
                                    </div>
                                    <div v-if="activity.server" class="text-xs text-gray-400">
                                        {{ activity.server }}
                                    </div>
                                    <div class="text-xs text-gray-400">
                                        {{ formatTimestamp(activity.timestamp) }}
                                    </div>
                                </div>

                                <div class="text-white mb-2">{{ activity.message }}</div>

                                <!-- Tool Calls -->
                                <div v-if="activity.toolCalls && activity.toolCalls.length > 0" class="space-y-2">
                                    <div v-for="(call, index) in activity.toolCalls" :key="index" 
                                         class="bg-gray-800 rounded p-3 text-sm">
                                        <div class="flex items-center justify-between mb-2">
                                            <span class="font-medium text-blue-300">🔧 {{ call.tool }}</span>    
                                            <button @click="toggleToolCall(activity.id + '-' + index)"
                                                   class="text-gray-400 hover:text-white text-xs">
                                                {{ expandedToolCalls[activity.id + '-' + index] ? '🔽' : '▶️' }}
                                            </button>
                                        </div>
                                        <div v-if="expandedToolCalls[activity.id + '-' + index]" class="space-y-2">
                                            <div v-if="call.arguments" class="bg-gray-900 rounded p-2">
                                                <div class="text-xs text-gray-400 mb-1">Arguments:</div>
                                                <pre class="text-xs text-gray-300 whitespace-pre-wrap">{{ JSON.stringify(call.arguments, null, 2) }}</pre>
                                            </div>
                                            <div v-if="call.result" class="bg-gray-900 rounded p-2">
                                                <div class="text-xs text-gray-400 mb-1">Result:</div>
                                                <pre class="text-xs text-gray-300 whitespace-pre-wrap">{{ typeof call.result === 'object' ? JSON.stringify(call.result, null, 2) : call.result }}</pre>
                                            </div>
                                        </div>
                                    </div>
                                </div>

                                <!-- Details Toggle -->
                                <div v-if="activity.details && Object.keys(activity.details).length > 0" class="mt-2">
                                    <button @click="toggleDetails(activity.id)" 
                                           class="text-sm text-blue-400 hover:text-blue-300">
                                        {{ showDetails[activity.id] ? '🔽 Hide Details' : '▶️ Show Details' }}
                                    </button>
                                    <div v-if="showDetails[activity.id]" class="mt-2 bg-gray-800 rounded p-3">
                                        <pre class="text-xs text-gray-300 whitespace-pre-wrap">{{ JSON.stringify(activity.details, null, 2) }}</pre>
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