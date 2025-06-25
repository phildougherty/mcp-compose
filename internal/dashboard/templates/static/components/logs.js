const LogViewer = {
    props: ['servers', 'config'],
    data() {
        return {
            selectedServer: '',
            logs: [],
            loading: false,
            error: '',
            streaming: false,
            wsConnection: null,
            autoScroll: true
        }
    },
    computed: {
        logStats() {
            return {
                total: this.logs.length,
                errors: this.logs.filter(log => log.level === 'ERROR').length,
                warnings: this.logs.filter(log => log.level === 'WARN').length,
                info: this.logs.filter(log => log.level === 'INFO').length
            };
        }
    },
    methods: {
        async loadLogs() {
            if (!this.selectedServer) return;
            
            this.loading = true;
            this.error = '';
            
            try {
                const response = await fetch(`/api/containers/mcp-compose-${this.selectedServer}/logs?tail=100`);
                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }
                
                const data = await response.json();
                this.logs = data.logs.map((line, index) => ({
                    id: index,
                    timestamp: new Date().toISOString(),
                    server: this.selectedServer,
                    level: this.detectLogLevel(line),
                    message: line
                }));
                
                this.scrollToBottom();
                
            } catch (err) {
                console.error('Failed to load logs:', err);
                this.error = err.message;
            } finally {
                this.loading = false;
            }
        },

        startStreaming() {
            if (!this.selectedServer || this.streaming) return;
            
            this.streaming = true;
            this.error = '';
            
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws/logs?server=${this.selectedServer}`;
            
            this.wsConnection = new WebSocket(wsUrl);
            
            this.wsConnection.onopen = () => {
                console.log('Log stream connected for server:', this.selectedServer);
            };
            
            this.wsConnection.onmessage = (event) => {
                try {
                    const logMessage = JSON.parse(event.data);
                    this.logs.push({
                        id: this.logs.length,
                        ...logMessage
                    });
                    
                    // Keep only last 1000 logs
                    if (this.logs.length > 1000) {
                        this.logs = this.logs.slice(-1000);
                    }
                    
                    if (this.autoScroll) {
                        this.$nextTick(() => this.scrollToBottom());
                    }
                } catch (err) {
                    console.error('Failed to parse log message:', err);
                }
            };
            
            this.wsConnection.onclose = () => {
                console.log('Log stream disconnected');
                this.streaming = false;
            };
            
            this.wsConnection.onerror = (err) => {
                console.error('WebSocket error:', err);
                this.error = 'WebSocket connection error';
                this.streaming = false;
            };
        },

        stopStreaming() {
            if (this.wsConnection) {
                this.wsConnection.close();
                this.wsConnection = null;
            }
            this.streaming = false;
        },

        toggleStreaming() {
            if (this.streaming) {
                this.stopStreaming();
            } else {
                this.startStreaming();
            }
        },

        clearLogs() {
            this.logs = [];
        },

        detectLogLevel(message) {
            const msg = message.toLowerCase();
            if (msg.includes('error') || msg.includes('failed') || msg.includes('exception')) return 'ERROR';
            if (msg.includes('warn') || msg.includes('warning')) return 'WARN';
            if (msg.includes('info') || msg.includes('started') || msg.includes('listening')) return 'INFO';
            if (msg.includes('debug')) return 'DEBUG';
            return 'INFO';
        },

        getLogLevelClass(level) {
            switch (level) {
                case 'ERROR': return 'text-red-400';
                case 'WARN': return 'text-yellow-400';
                case 'INFO': return 'text-blue-400';
                case 'DEBUG': return 'text-gray-400';
                default: return 'text-green-400';
            }
        },

        formatLogTimestamp(timestamp) {
            return new Date(timestamp).toLocaleTimeString();
        },

        scrollToBottom() {
            const container = this.$refs.logContainer;
            if (container) {
                container.scrollTop = container.scrollHeight;
            }
        },

        onServerChange() {
            this.stopStreaming();
            this.logs = [];
            this.error = '';
            
            if (this.selectedServer) {
                this.loadLogs();
            }
        },

        setSelectedServer(serverName) {
            this.selectedServer = serverName;
            if (serverName) {
                this.loadLogs();
            }
        },
    },

    watch: {
        selectedServer: 'onServerChange'
    },

    beforeUnmount() {
        this.stopStreaming();
    },

    template: `
        <div class="space-y-6 fade-in">
            <!-- Header -->
            <div class="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
                <div class="flex justify-between items-center">
                    <h3 class="text-lg font-medium text-gray-900 dark:text-white">
                        Container Logs
                    </h3>
                    <div class="flex space-x-3">
                        <select
                            v-model="selectedServer"
                            class="block pl-3 pr-10 py-2 text-base border-gray-300 dark:border-gray-600 focus:outline-none focus:ring-blue-500 focus:border-blue-500 sm:text-sm rounded-md dark:bg-gray-700 dark:text-white"
                        >
                            <option value="">Select a server</option>
                            <option v-for="server in servers" :key="server.name" :value="server.name">
                                {{ server.name }}
                            </option>
                        </select>
                        <button
                            @click="toggleStreaming"
                            :disabled="!selectedServer"
                            :class="[
                                'inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md',
                                streaming
                                    ? 'text-white bg-red-600 hover:bg-red-700'
                                    : 'text-white bg-green-600 hover:bg-green-700',
                                'disabled:opacity-50 disabled:cursor-not-allowed'
                            ]"
                        >
                            <svg v-if="streaming" class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                            </svg>
                            <svg v-else class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                            </svg>
                            {{ streaming ? 'Stop' : 'Start' }} Stream
                        </button>
                        <button
                            @click="loadLogs"
                            :disabled="!selectedServer || loading"
                            class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50"
                        >
                            <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                            </svg>
                            Refresh
                        </button>
                        <button
                            @click="clearLogs"
                            class="inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-md text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600"
                        >
                            Clear
                        </button>
                    </div>
                </div>
                
                <!-- Auto-scroll toggle -->
                <div class="mt-4">
                    <label class="inline-flex items-center">
                        <input v-model="autoScroll" type="checkbox" class="form-checkbox h-4 w-4 text-blue-600">
                        <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-scroll to bottom</span>
                    </label>
                </div>
            </div>

            <!-- Error Message -->
            <div v-if="error" class="bg-red-50 dark:bg-red-900 border border-red-200 dark:border-red-800 rounded-md p-4">
                <div class="flex">
                    <div class="flex-shrink-0">
                        <svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                        </svg>
                    </div>
                    <div class="ml-3">
                        <h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error loading logs</h3>
                        <div class="mt-2 text-sm text-red-700 dark:text-red-300">{{ error }}</div>
                    </div>
                </div>
            </div>

            <!-- Log Display -->
            <div class="bg-gray-900 rounded-lg shadow overflow-hidden">
                <div class="p-4">
                    <div class="flex justify-between items-center mb-3 text-gray-400 text-xs">
                        <div v-if="selectedServer">
                            Container: mcp-compose-{{ selectedServer }}
                            <span v-if="streaming" class="ml-2 text-green-400">(Live)</span>
                        </div>
                        <div v-if="logs.length > 0">
                            {{ logs.length }} lines
                        </div>
                    </div>
                    
                    <div v-if="loading" class="text-center py-8">
                        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-green-500 mx-auto"></div>
                        <p class="mt-2 text-gray-400">Loading logs...</p>
                    </div>
                    
                    <div
                        v-else
                        ref="logContainer"
                        class="log-container h-96 overflow-y-auto font-mono text-sm text-green-400 whitespace-pre-wrap"
                    >
                        <div v-if="!selectedServer" class="text-gray-500 text-center py-8">
                            Please select a server to view logs
                        </div>
                        <div v-else-if="logs.length === 0" class="text-gray-500 text-center py-8">
                            No logs available for {{ selectedServer }}
                        </div>
                        <div v-else class="space-y-1">
                            <div
                                v-for="log in logs"
                                :key="log.id"
                                class="leading-tight"
                            >
                                <span class="text-gray-400">{{ formatLogTimestamp(log.timestamp) }}</span>
                                <span class="text-blue-300">[{{ log.server }}]</span>
                                <span :class="getLogLevelClass(log.level)">[{{ log.level }}]</span>
                                {{ log.message }}
                            </div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Log Statistics -->
            <div class="grid grid-cols-1 md:grid-cols-4 gap-6">
                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
                    <div class="p-5">
                        <div class="flex items-center">
                            <div class="flex-shrink-0">
                                <div class="w-8 h-8 bg-blue-500 rounded-full flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                    </svg>
                                </div>
                            </div>
                            <div class="ml-5 w-0 flex-1">
                                <dl>
                                    <dt class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Total Lines</dt>
                                    <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ logStats.total }}</dd>
                                </dl>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
                    <div class="p-5">
                        <div class="flex items-center">
                            <div class="flex-shrink-0">
                                <div class="w-8 h-8 bg-red-500 rounded-full flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                    </svg>
                                </div>
                            </div>
                            <div class="ml-5 w-0 flex-1">
                                <dl>
                                    <dt class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Errors</dt>
                                    <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ logStats.errors }}</dd>
                                </dl>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
                    <div class="p-5">
                        <div class="flex items-center">
                            <div class="flex-shrink-0">
                                <div class="w-8 h-8 bg-yellow-500 rounded-full flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                    </svg>
                                </div>
                            </div>
                            <div class="ml-5 w-0 flex-1">
                                <dl>
                                    <dt class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Warnings</dt>
                                    <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ logStats.warnings }}</dd>
                                </dl>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="bg-white dark:bg-gray-800 overflow-hidden shadow rounded-lg">
                    <div class="p-5">
                        <div class="flex items-center">
                            <div class="flex-shrink-0">
                                <div class="w-8 h-8 bg-green-500 rounded-full flex items-center justify-center">
                                    <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                    </svg>
                                </div>
                            </div>
                            <div class="ml-5 w-0 flex-1">
                                <dl>
                                    <dt class="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">Info</dt>
                                    <dd class="text-lg font-medium text-gray-900 dark:text-white">{{ logStats.info }}</dd>
                                </dl>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};
