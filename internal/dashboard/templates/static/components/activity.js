const ActivityViewer = {
    props: ['config'],
    data() {
        return {
            activities: [],
            wsConnection: null,
            connected: false,
            maxActivities: 500,
            autoScroll: true,
            filterLevel: 'all',
            filterServer: 'all',
            filterType: 'all',
            searchTerm: '',
            showDetails: {},
            expandedToolCalls: {},
            activityStats: {
                total: 0,
                requests: 0,
                errors: 0,
                connections: 0,
                toolCalls: 0
            }
        }
    },
    computed: {
        filteredActivities() {
            return this.activities.filter(activity => {
                const matchesSearch = !this.searchTerm ||
                    activity.message.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    activity.server?.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    activity.client?.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    this.searchInToolData(activity);
                
                const matchesLevel = this.filterLevel === 'all' || activity.level === this.filterLevel;
                const matchesServer = this.filterServer === 'all' || activity.server === this.filterServer;
                const matchesType = this.filterType === 'all' || activity.type === this.filterType;
                
                return matchesSearch && matchesLevel && matchesServer && matchesType;
            });
        },
        
        uniqueServers() {
            const servers = new Set(this.activities.map(a => a.server).filter(Boolean));
            return Array.from(servers).sort();
        },
        
        uniqueTypes() {
            const types = new Set(this.activities.map(a => a.type).filter(Boolean));
            return Array.from(types).sort();
        }
    },
    
    methods: {
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
                id: Date.now() + Math.random(),
                timestamp: activity.timestamp || new Date().toISOString(),
                toolCalls: this.extractToolCalls(activity)
            };
            
            this.activities.unshift(enrichedActivity);
            this.updateStats(enrichedActivity);
            
            // Keep only the latest activities
            if (this.activities.length > this.maxActivities) {
                this.activities = this.activities.slice(0, this.maxActivities);
            }
            
            // Auto scroll if enabled
            if (this.autoScroll) {
                this.$nextTick(() => this.scrollToTop());
            }
        },
        
        extractToolCalls(activity) {
            const toolCalls = [];
            
            // Check for tool calls in details
            if (activity.details) {
                if (activity.details.toolCall) {
                    toolCalls.push({
                        type: 'call',
                        name: activity.details.toolCall.name,
                        parameters: activity.details.toolCall.parameters,
                        id: activity.details.toolCall.id || `call-${Date.now()}`
                    });
                }
                
                if (activity.details.toolResult) {
                    toolCalls.push({
                        type: 'result',
                        content: activity.details.toolResult.content,
                        isError: activity.details.toolResult.isError,
                        id: activity.details.toolResult.toolCallId || `result-${Date.now()}`
                    });
                }
                
                // Check for multiple tool calls in arrays
                if (Array.isArray(activity.details.tools)) {
                    activity.details.tools.forEach((tool, index) => {
                        toolCalls.push({
                            type: 'call',
                            name: tool.name,
                            parameters: tool.parameters,
                            id: `multi-call-${index}-${Date.now()}`
                        });
                    });
                }
            }
            
            return toolCalls;
        },
        
        updateStats(activity) {
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
            this.activityStats = { total: 0, requests: 0, errors: 0, connections: 0, toolCalls: 0 };
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
        
        getTypeIcon(type) {
            switch (type) {
                case 'request': return 'M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h8a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293L12 17';
                case 'connection': return 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1';
                case 'error': return 'M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
                case 'tool': return 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z';
                default: return 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z';
            }
        },
        
        formatTimestamp(timestamp) {
            const date = new Date(timestamp);
            return date.toLocaleTimeString() + '.' + date.getMilliseconds().toString().padStart(3, '0');
        },
        
        showToast(message, type = 'info') {
            // This would be handled by a global toast system
            window.showToast && window.showToast(message, type);
        }
    },
    
    mounted() {
        this.connectToActivityStream();
    },
    
    beforeUnmount() {
        if (this.wsConnection) {
            this.wsConnection.close();
        }
    },
    
    template: `
        <div class="space-y-4 animate-fade-in">
            <!-- Enhanced Header -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <div class="flex items-center space-x-3">
                        <div class="flex-shrink-0">
                            <div class="w-10 h-10 bg-gradient-to-r from-purple-500 to-pink-600 rounded-xl flex items-center justify-center">
                                <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                                </svg>
                            </div>
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Activity Feed</h3>
                            <p class="text-sm text-gray-500 dark:text-gray-400 flex items-center">
                                <span class="status-indicator">
                                    <span :class="['status-dot', connected ? 'status-running pulse' : 'status-stopped']"></span>
                                    {{ connected ? 'Connected' : 'Disconnected' }} • Real-time activity monitoring
                                </span>
                            </p>
                        </div>
                    </div>
                    
                    <!-- Enhanced Controls -->
                    <div class="space-y-3 lg:space-y-0 lg:space-x-3 lg:flex">
                        <!-- Search -->
                        <div class="relative">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                            <input
                                v-model="searchTerm"
                                type="text"
                                placeholder="Search activity, tools, parameters..."
                                class="form-input pl-10 w-full lg:w-64"
                            >
                        </div>
                        
                        <!-- Filters -->
                        <div class="flex space-x-2">
                            <select v-model="filterLevel" class="form-input text-sm">
                                <option value="all">All Levels</option>
                                <option value="INFO">Info</option>
                                <option value="WARN">Warnings</option>
                                <option value="ERROR">Errors</option>
                                <option value="DEBUG">Debug</option>
                            </select>
                            
                            <select v-model="filterType" class="form-input text-sm">
                                <option value="all">All Types</option>
                                <option value="request">Requests</option>
                                <option value="tool">Tool Calls</option>
                                <option value="connection">Connections</option>
                                <option value="error">Errors</option>
                            </select>
                            
                            <select v-model="filterServer" class="form-input text-sm">
                                <option value="all">All Servers</option>
                                <option v-for="server in uniqueServers" :key="server" :value="server">{{ server }}</option>
                            </select>
                        </div>
                        
                        <!-- Settings -->
                        <div class="flex items-center space-x-3">
                            <label class="inline-flex items-center">
                                <input v-model="autoScroll" type="checkbox" class="form-checkbox h-4 w-4 text-purple-600 rounded focus:ring-purple-500">
                                <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-scroll</span>
                            </label>
                            
                            <button
                                @click="clearActivities"
                                class="touch-target inline-flex items-center px-3 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                            >
                                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M9 2a1 1 0 000 2h2a1 1 0 100-2H9z M4 5a2 2 0 012-2v1a1 1 0 001 1h6a1 1 0 001-1V3a2 2 0 012 2v6.5l1.707 1.707A1 1 0 0117 10.414V5a4 4 0 00-8 0v5.586l1.707-1.707A1 1 0 0112 10.414z" clip-rule="evenodd"></path>
                                </svg>
                                Clear
                            </button>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- Enhanced Stats -->
            <div class="responsive-grid cols-2 lg:cols-4">
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Total</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activityStats.total }}</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                <svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 4H6a2 2 0 00-2 2v12a2 2 0 002 2h8a2 2 0 002-2V6a2 2 0 00-2-2h-2m-4-1v8m0 0l3-3m-3 3L9 8m-5 5h2.586a1 1 0 01.707.293L12 17"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Requests</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activityStats.requests }}</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-purple-500 rounded-lg flex items-center justify-center">
                                <svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Tool Calls</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activityStats.toolCalls }}</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-red-500 rounded-lg flex items-center justify-center">
                                <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Errors</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activityStats.errors }}</p>
                        </div>
                    </div>
                </div>
            </div>
            
            <!-- Enhanced Activity Feed -->
            <div class="enhanced-card overflow-hidden">
                <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <h4 class="text-lg font-medium text-gray-900 dark:text-white flex items-center">
                        <span class="status-indicator mr-3">
                            <span :class="['status-dot', connected ? 'status-running pulse' : 'status-stopped']"></span>
                        </span>
                        Live Activity Stream
                        <span class="ml-auto text-sm font-normal text-gray-500 dark:text-gray-400">
                            {{ filteredActivities.length }} of {{ activities.length }} activities
                        </span>
                    </h4>
                </div>
                
                <div
                    ref="activityContainer"
                    class="h-96 overflow-y-auto custom-scrollbar bg-gray-50 dark:bg-gray-900"
                >
                    <div v-if="!connected" class="flex items-center justify-center h-full">
                        <div class="text-center">
                            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-500 mx-auto mb-4"></div>
                            <p class="text-gray-400">Connecting to activity stream...</p>
                        </div>
                    </div>
                    
                    <div v-else-if="filteredActivities.length === 0" class="flex items-center justify-center h-full">
                        <div class="text-center p-8">
                            <svg class="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-4.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 009.586 13H7"></path>
                            </svg>
                            <p class="mt-4 text-lg font-medium text-gray-400">No activity to show</p>
                            <p class="text-sm text-gray-500">Activities will appear here when the proxy handles requests</p>
                        </div>
                    </div>
                    
                    <div v-else class="space-y-2 p-4">
                        <div
                            v-for="activity in filteredActivities"
                            :key="activity.id"
                            :class="['activity-item', activity.level.toLowerCase(), 'rounded-lg p-4 transition-all duration-200', getLevelClass(activity.level)]"
                        >
                            <!-- Activity Header -->
                            <div class="flex items-start space-x-3">
                                <div class="flex-shrink-0 mt-1">
                                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getTypeIcon(activity.type)"></path>
                                    </svg>
                                </div>
                                
                                <div class="flex-1 min-w-0">
                                    <div class="flex items-center justify-between mb-2">
                                        <div class="flex items-center space-x-2 flex-wrap">
                                            <span class="text-xs font-medium tracking-wider uppercase px-2 py-1 rounded bg-opacity-75">
                                                {{ activity.level }}
                                            </span>
                                            <span v-if="activity.server" class="text-xs font-medium bg-gray-700 dark:bg-gray-600 text-white px-2 py-1 rounded">
                                                {{ activity.server }}
                                            </span>
                                            <span v-if="activity.client" class="text-xs opacity-75 bg-gray-600 dark:bg-gray-500 text-white px-2 py-1 rounded">
                                                {{ activity.client }}
                                            </span>
                                            <span v-if="activity.toolCalls && activity.toolCalls.length > 0" class="text-xs bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200 px-2 py-1 rounded">
                                                {{ activity.toolCalls.length }} tool call{{ activity.toolCalls.length !== 1 ? 's' : '' }}
                                            </span>
                                        </div>
                                        
                                        <div class="flex items-center space-x-2">
                                            <span class="text-xs text-gray-500 dark:text-gray-400 font-mono">
                                                {{ formatTimestamp(activity.timestamp) }}
                                            </span>
                                            <button
                                                v-if="activity.details || (activity.toolCalls && activity.toolCalls.length > 0)"
                                                @click="toggleDetails(activity.id)"
                                                class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 transition-colors p-1"
                                            >
                                                <svg :class="['w-4 h-4 transition-transform', showDetails[activity.id] ? 'rotate-180' : '']" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                                </svg>
                                            </button>
                                        </div>
                                    </div>
                                    
                                    <p class="text-sm leading-relaxed mb-3">
                                        {{ activity.message }}
                                    </p>
                                    
                                    <!-- Tool Calls Display -->
                                    <div v-if="activity.toolCalls && activity.toolCalls.length > 0" class="space-y-2">
                                        <div
                                            v-for="toolCall in activity.toolCalls"
                                            :key="toolCall.id"
                                            class="tool-call-item"
                                        >
                                            <div
                                                @click="toggleToolCall(activity.id, toolCall.id)"
                                                class="tool-call-header cursor-pointer"
                                            >
                                                <div class="flex items-center justify-between">
                                                    <div class="flex items-center space-x-2">
                                                        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"></path>
                                                        </svg>
                                                        <span class="font-medium text-sm">
                                                            {{ toolCall.type === 'call' ? 'Tool Call' : 'Tool Result' }}: 
                                                            <code class="text-purple-600 dark:text-purple-400 font-mono">{{ toolCall.name || 'Result' }}</code>
                                                        </span>
                                                        <span v-if="toolCall.isError" class="text-xs bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200 px-2 py-1 rounded">
                                                            Error
                                                        </span>
                                                    </div>
                                                    <svg :class="['w-4 h-4 transition-transform', isToolCallExpanded(activity.id, toolCall.id) ? 'rotate-180' : '']" fill="currentColor" viewBox="0 0 20 20">
                                                        <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                                    </svg>
                                                </div>
                                            </div>
                                            
                                            <!-- Tool Call Details -->
                                            <div v-if="isToolCallExpanded(activity.id, toolCall.id)" class="tool-call-details animate-slide-up">
                                                <div v-if="toolCall.type === 'call'">
                                                    <div class="mb-3">
                                                        <h6 class="text-sm font-medium text-gray-300 mb-2">Parameters:</h6>
                                                        <div class="tool-call-params">
                                                            <pre class="text-sm text-gray-200 whitespace-pre-wrap">{{ formatToolParameters(toolCall.parameters) }}</pre>
                                                        </div>
                                                    </div>
                                                </div>
                                                
                                                <div v-if="toolCall.type === 'result'">
                                                    <div class="mb-3">
                                                        <h6 class="text-sm font-medium text-gray-300 mb-2">Result:</h6>
                                                        <div :class="['p-3 rounded', toolCall.isError ? 'tool-call-error' : 'tool-call-result']">
                                                            <pre class="text-sm text-gray-200 whitespace-pre-wrap">{{ formatToolResult(toolCall.content) }}</pre>
                                                        </div>
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                    
                                    <!-- Expandable Raw Details -->
                                    <div v-if="showDetails[activity.id] && activity.details" class="mt-3 p-3 bg-gray-800 dark:bg-gray-700 rounded-lg">
                                        <h6 class="text-sm font-medium text-gray-300 mb-2">Raw Details:</h6>
                                        <pre class="text-xs text-gray-300 whitespace-pre-wrap overflow-x-auto">{{ JSON.stringify(activity.details, null, 2) }}</pre>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};