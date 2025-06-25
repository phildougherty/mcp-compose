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
            searchTerm: '',
            showDetails: {},
            activityStats: {
                total: 0,
                requests: 0,
                errors: 0,
                connections: 0
            }
        }
    },
    computed: {
        filteredActivities() {
            return this.activities.filter(activity => {
                const matchesSearch = !this.searchTerm || 
                    activity.message.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    activity.server?.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    activity.client?.toLowerCase().includes(this.searchTerm.toLowerCase());
                
                const matchesLevel = this.filterLevel === 'all' || activity.level === this.filterLevel;
                const matchesServer = this.filterServer === 'all' || activity.server === this.filterServer;
                
                return matchesSearch && matchesLevel && matchesServer;
            });
        },
        uniqueServers() {
            const servers = new Set(this.activities.map(a => a.server).filter(Boolean));
            return Array.from(servers).sort();
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
                // Attempt to reconnect after 3 seconds
                setTimeout(() => this.connectToActivityStream(), 3000);
            };
            
            this.wsConnection.onerror = (err) => {
                console.error('Activity WebSocket error:', err);
                this.connected = false;
            };
        },
        
        addActivity(activity) {
            const enrichedActivity = {
                ...activity,
                id: Date.now() + Math.random(),
                timestamp: activity.timestamp || new Date().toISOString()
            };
            
            this.activities.unshift(enrichedActivity);
            
            // Update stats
            this.activityStats.total++;
            if (activity.type === 'request') this.activityStats.requests++;
            if (activity.level === 'ERROR') this.activityStats.errors++;
            if (activity.type === 'connection') this.activityStats.connections++;
            
            // Keep only the latest activities
            if (this.activities.length > this.maxActivities) {
                this.activities = this.activities.slice(0, this.maxActivities);
            }
            
            // Auto scroll if enabled
            if (this.autoScroll) {
                this.$nextTick(() => this.scrollToTop());
            }
        },
        
        scrollToTop() {
            const container = this.$refs.activityContainer;
            if (container) {
                container.scrollTop = 0;
            }
        },
        
        clearActivities() {
            this.activities = [];
            this.activityStats = { total: 0, requests: 0, errors: 0, connections: 0 };
        },
        
        toggleDetails(activityId) {
            this.showDetails[activityId] = !this.showDetails[activityId];
            this.$forceUpdate();
        },
        
        getLevelClass(level) {
            switch (level) {
                case 'ERROR': return 'text-red-400 bg-red-900/20 border-red-500/30';
                case 'WARN': return 'text-yellow-400 bg-yellow-900/20 border-yellow-500/30';
                case 'INFO': return 'text-blue-400 bg-blue-900/20 border-blue-500/30';
                case 'DEBUG': return 'text-gray-400 bg-gray-900/20 border-gray-500/30';
                default: return 'text-green-400 bg-green-900/20 border-green-500/30';
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
        <div class="space-y-6">
            <!-- Header -->
            <div class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4 lg:p-6">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <div class="flex items-center space-x-3">
                        <div class="flex-shrink-0">
                            <div class="w-10 h-10 bg-gradient-to-r from-purple-500 to-pink-600 rounded-lg flex items-center justify-center">
                                <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"></path>
                                </svg>
                            </div>
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Activity Feed</h3>
                            <p class="text-sm text-gray-500 dark:text-gray-400 flex items-center">
                                <span :class="['w-2 h-2 rounded-full mr-2', connected ? 'bg-green-500' : 'bg-red-500']"></span>
                                Real-time proxy activity and client interactions
                            </p>
                        </div>
                    </div>
                    
                    <!-- Controls -->
                    <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3">
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
                                placeholder="Search activity..." 
                                class="block w-full pl-10 pr-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white placeholder-gray-500 dark:placeholder-gray-400 focus:ring-2 focus:ring-purple-500 focus:border-purple-500 text-sm"
                            >
                        </div>
                        
                        <!-- Filters -->
                        <select 
                            v-model="filterLevel"
                            class="block w-full sm:w-auto px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-purple-500 focus:border-purple-500 text-sm"
                        >
                            <option value="all">All Levels</option>
                            <option value="INFO">Info</option>
                            <option value="WARN">Warnings</option>
                            <option value="ERROR">Errors</option>
                            <option value="DEBUG">Debug</option>
                        </select>
                        
                        <select 
                            v-model="filterServer"
                            class="block w-full sm:w-auto px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-purple-500 focus:border-purple-500 text-sm"
                        >
                            <option value="all">All Servers</option>
                            <option v-for="server in uniqueServers" :key="server" :value="server">{{ server }}</option>
                        </select>
                        
                        <!-- Auto-scroll toggle -->
                        <label class="inline-flex items-center">
                            <input v-model="autoScroll" type="checkbox" class="form-checkbox h-4 w-4 text-purple-600 rounded focus:ring-purple-500">
                            <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-scroll</span>
                        </label>
                        
                        <!-- Clear button -->
                        <button
                            @click="clearActivities"
                            class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 transition-colors"
                        >
                            Clear
                        </button>
                    </div>
                </div>
            </div>

            <!-- Stats -->
            <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4">
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
                
                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4">
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
                
                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4">
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
                
                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center">
                                <svg class="h-5 w-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Connections</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ activityStats.connections }}</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Activity Feed -->
            <div class="bg-white dark:bg-gray-800 shadow-sm rounded-xl border border-gray-200 dark:border-gray-700 overflow-hidden">
                <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <h4 class="text-lg font-medium text-gray-900 dark:text-white flex items-center">
                        <span :class="['w-2 h-2 rounded-full mr-3', connected ? 'bg-green-500 animate-pulse' : 'bg-red-500']"></span>
                        Live Activity Stream
                        <span class="ml-auto text-sm font-normal text-gray-500 dark:text-gray-400">
                            {{ filteredActivities.length }} of {{ activities.length }} activities
                        </span>
                    </h4>
                </div>
                
                <div 
                    ref="activityContainer"
                    class="h-96 overflow-y-auto custom-scrollbar bg-gray-900"
                >
                    <div v-if="!connected" class="flex items-center justify-center h-full">
                        <div class="text-center">
                            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-purple-500 mx-auto mb-4"></div>
                            <p class="text-gray-400">Connecting to activity stream...</p>
                        </div>
                    </div>
                    
                    <div v-else-if="filteredActivities.length === 0" class="flex items-center justify-center h-full">
                        <div class="text-center">
                            <svg class="mx-auto h-12 w-12 text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-4.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 009.586 13H7"></path>
                            </svg>
                            <p class="mt-4 text-gray-400">No activity to show</p>
                            <p class="text-sm text-gray-500">Activities will appear here when proxy handles requests</p>
                        </div>
                    </div>
                    
                    <div v-else class="divide-y divide-gray-700">
                        <div 
                            v-for="activity in filteredActivities" 
                            :key="activity.id"
                            :class="['p-4 hover:bg-gray-800 transition-colors border-l-4', getLevelClass(activity.level)]"
                        >
                            <div class="flex items-start space-x-3">
                                <div class="flex-shrink-0 mt-1">
                                    <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getTypeIcon(activity.type)"></path>
                                    </svg>
                                </div>
                                <div class="flex-1 min-w-0">
                                    <div class="flex items-center justify-between">
                                        <div class="flex items-center space-x-2">
                                            <span class="text-xs font-medium tracking-wider uppercase opacity-75">
                                                {{ activity.level }}
                                            </span>
                                            <span v-if="activity.server" class="text-xs font-medium bg-gray-700 px-2 py-1 rounded">
                                                {{ activity.server }}
                                            </span>
                                            <span v-if="activity.client" class="text-xs opacity-75">
                                                {{ activity.client }}
                                            </span>
                                        </div>
                                        <div class="flex items-center space-x-2">
                                            <span class="text-xs text-gray-400 font-mono">
                                                {{ formatTimestamp(activity.timestamp) }}
                                            </span>
                                            <button 
                                                v-if="activity.details"
                                                @click="toggleDetails(activity.id)"
                                                class="text-gray-400 hover:text-gray-300 transition-colors"
                                            >
                                                <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                                                    <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1zm0 4a1 1 0 011-1h12a1 1 0 110 2H4a1 1 0 01-1-1z" clip-rule="evenodd"></path>
                                                </svg>
                                            </button>
                                        </div>
                                    </div>
                                    <p class="mt-1 text-sm leading-relaxed">
                                        {{ activity.message }}
                                    </p>
                                    
                                    <!-- Expandable details -->
                                    <div v-if="showDetails[activity.id] && activity.details" class="mt-3 p-3 bg-gray-800 rounded-lg">
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
