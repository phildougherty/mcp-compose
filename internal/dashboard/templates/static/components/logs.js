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
            autoScroll: true,
            searchTerm: '',
            filterLevel: 'all'
        }
    },
    computed: {
        filteredLogs() {
            return this.logs.filter(log => {
                const matchesSearch = !this.searchTerm ||
                    log.message.toLowerCase().includes(this.searchTerm.toLowerCase());
                const matchesLevel = this.filterLevel === 'all' || log.level === this.filterLevel;
                return matchesSearch && matchesLevel;
            });
        },
        logStats() {
            return {
                total: this.logs.length,
                errors: this.logs.filter(log => log.level === 'ERROR').length,
                warnings: this.logs.filter(log => log.level === 'WARN').length,
                info: this.logs.filter(log => log.level === 'INFO').length,
                debug: this.logs.filter(log => log.level === 'DEBUG').length
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
                if (data.logs && Array.isArray(data.logs)) {
                    this.logs = data.logs.map((line, index) => ({
                        id: index,
                        timestamp: new Date().toISOString(),
                        server: this.selectedServer,
                        level: this.detectLogLevel(line),
                        message: line,
                        raw: line
                    }));
                } else {
                    this.logs = [];
                    console.warn('No logs data received or logs is not an array:', data);
                }
                this.scrollToBottom();
                this.showToast('Logs loaded successfully', 'success');
            } catch (err) {
                console.error('Failed to load logs:', err);
                this.error = err.message;
                this.showToast(`Failed to load logs: ${err.message}`, 'error');
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
                this.showToast('Log streaming started', 'success');
            };
            this.wsConnection.onmessage = (event) => {
                try {
                    const logMessage = JSON.parse(event.data);
                    this.addLogEntry(logMessage);
                } catch (err) {
                    console.error('Failed to parse log message:', err);
                }
            };
            this.wsConnection.onclose = () => {
                console.log('Log stream disconnected');
                this.streaming = false;
                this.showToast('Log streaming stopped', 'info');
            };
            this.wsConnection.onerror = (err) => {
                console.error('WebSocket error:', err);
                this.error = 'WebSocket connection error';
                this.streaming = false;
                this.showToast('Log streaming error', 'error');
            };
        },
        addLogEntry(logMessage) {
            this.logs.push({
                id: this.logs.length,
                timestamp: logMessage.timestamp,
                server: logMessage.server,
                level: logMessage.level,
                message: logMessage.message,
                raw: logMessage.message
            });
            // Keep only last 1000 logs to prevent memory issues
            if (this.logs.length > 1000) {
                this.logs = this.logs.slice(-1000);
            }
            if (this.autoScroll) {
                this.$nextTick(() => this.scrollToBottom());
            }
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
            this.showToast('Logs cleared', 'info');
        },
        downloadLogs() {
            const logsText = this.logs.map(log =>
                `[${log.timestamp}] [${log.level}] [${log.server}] ${log.message}`
            ).join('\n');
            const blob = new Blob([logsText], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `${this.selectedServer}-logs-${new Date().toISOString().split('T')[0]}.txt`;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            URL.revokeObjectURL(url);
            this.showToast('Logs downloaded', 'success');
        },
        detectLogLevel(message) {
            const msg = message.toLowerCase();
            if (msg.includes('error') || msg.includes('failed') || msg.includes('exception') || msg.includes('fatal')) return 'ERROR';
            if (msg.includes('warn') || msg.includes('warning')) return 'WARN';
            if (msg.includes('debug') || msg.includes('trace')) return 'DEBUG';
            return 'INFO';
        },
        getLogLevelClass(level) {
            switch (level) {
                case 'ERROR': return 'text-red-400 bg-red-900/20';
                case 'WARN': return 'text-yellow-400 bg-yellow-900/20';
                case 'INFO': return 'text-blue-400 bg-blue-900/20';
                case 'DEBUG': return 'text-gray-400 bg-gray-900/20';
                default: return 'text-green-400 bg-green-900/20';
            }
        },
        formatLogTimestamp(timestamp) {
            try {
                const date = new Date(timestamp);
                return date.toLocaleTimeString() + '.' + date.getMilliseconds().toString().padStart(3, '0');
            } catch (e) {
                return timestamp;
            }
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
        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },
    watch: {
        selectedServer: 'onServerChange'
    },
    beforeUnmount() {
        this.stopStreaming();
    },
    template: `
        <div class="space-y-4 animate-fade-in">
            <!-- Enhanced Header -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                    <div class="flex items-center space-x-3">
                        <div class="flex-shrink-0">
                            <div class="w-10 h-10 bg-gradient-to-r from-blue-500 to-cyan-600 rounded-xl flex items-center justify-center">
                                <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                                </svg>
                            </div>
                        </div>
                        <div>
                            <h3 class="text-lg font-semibold text-gray-900 dark:text-white">Logs</h3>
                            <p class="text-sm text-gray-500 dark:text-gray-400">Real-time container log monitoring</p>
                        </div>
                    </div>
                    <!-- Controls -->
                    <div class="space-y-3 lg:space-y-0 lg:space-x-3 lg:flex">
                        <select
                            v-model="selectedServer"
                            class="form-input w-full lg:w-48"
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
                                'touch-target inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-lg transition-colors',
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
                            class="touch-target inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors"
                        >
                            <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" clip-rule="evenodd"></path>
                            </svg>
                            Refresh
                        </button>
                        <button
                            @click="downloadLogs"
                            :disabled="!logs.length"
                            class="touch-target inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors"
                        >
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path>
                            </svg>
                            Download
                        </button>
                        <button
                            @click="clearLogs"
                            :disabled="!logs.length"
                            class="touch-target inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 shadow-sm text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 hover:bg-gray-50 dark:hover:bg-gray-600 disabled:opacity-50 transition-colors"
                        >
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                            </svg>
                            Clear
                        </button>
                    </div>
                </div>
                <!-- Search and Filter -->
                <div class="mt-4 flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-4">
                    <div class="flex-1 relative">
                        <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                            <svg class="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <input
                            v-model="searchTerm"
                            type="text"
                            placeholder="Search logs..."
                            class="form-input pl-10"
                        >
                    </div>
                    <select v-model="filterLevel" class="form-input w-full sm:w-32">
                        <option value="all">All Levels</option>
                        <option value="ERROR">Errors</option>
                        <option value="WARN">Warnings</option>
                        <option value="INFO">Info</option>
                        <option value="DEBUG">Debug</option>
                    </select>
                    <label class="inline-flex items-center">
                        <input v-model="autoScroll" type="checkbox" class="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500">
                        <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-scroll</span>
                    </label>
                </div>
            </div>
            <!-- Error Message -->
            <div v-if="error" class="enhanced-card border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-4">
                <div class="flex items-start">
                    <div class="flex-shrink-0">
                        <svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                        </svg>
                    </div>
                    <div class="ml-3 flex-1">
                        <h3 class="text-sm font-medium text-red-800 dark:text-red-200">Error loading logs</h3>
                        <div class="mt-2 text-sm text-red-700 dark:text-red-300">{{ error }}</div>
                        <button @click="error = ''" class="mt-3 text-sm text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline">
                            Dismiss
                        </button>
                    </div>
                </div>
            </div>
            <!-- Enhanced Log Display -->
            <div class="enhanced-card overflow-hidden">
                <div class="px-6 py-4 border-b border-gray-200 dark:border-gray-700">
                    <div class="flex items-center justify-between">
                        <h4 class="text-lg font-medium text-gray-900 dark:text-white">
                            <span v-if="selectedServer">{{ selectedServer }}</span>
                            <span v-else>Select a server to view logs</span>
                            <span v-if="streaming" class="ml-2">
                                <span class="status-indicator">
                                    <span class="status-dot status-running pulse"></span>
                                    Live
                                </span>
                            </span>
                        </h4>
                        <div v-if="filteredLogs.length > 0" class="text-sm text-gray-500 dark:text-gray-400">
                            {{ filteredLogs.length }} of {{ logs.length }} lines
                        </div>
                    </div>
                </div>
                <div class="bg-gray-900 relative">
                    <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-gray-900/75 z-10">
                        <div class="text-center">
                            <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-green-500 mx-auto mb-4"></div>
                            <p class="text-gray-400">Loading logs...</p>
                        </div>
                    </div>
                    <div
                        ref="logContainer"
                        class="h-96 overflow-y-auto custom-scrollbar font-mono text-sm"
                    >
                        <div v-if="!selectedServer" class="flex items-center justify-center h-full text-gray-500">
                            <div class="text-center">
                                <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                                </svg>
                                <p class="text-lg font-medium">Please select a server to view logs</p>
                            </div>
                        </div>
                        <div v-else-if="filteredLogs.length === 0" class="flex items-center justify-center h-full text-gray-500">
                            <div class="text-center p-8">
                                <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-4.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 009.586 13H7"></path>
                                </svg>
                                <p class="text-lg font-medium">No logs available</p>
                                <p class="text-gray-400 mt-2">Logs for {{ selectedServer }} will appear here</p>
                            </div>
                        </div>
                        <div v-else class="p-4">
                            <div
                                v-for="log in filteredLogs"
                                :key="log.id"
                                :class="['py-1 px-2 rounded mb-1 leading-tight', getLogLevelClass(log.level)]"
                            >
                                <span class="text-gray-400 text-xs mr-2">{{ formatLogTimestamp(log.timestamp) }}</span>
                                <span :class="['text-xs font-medium mr-2 px-2 py-1 rounded', getLogLevelClass(log.level)]">{{ log.level }}</span>
                                <span class="text-gray-300">{{ log.message }}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            <!-- Enhanced Statistics -->
            <div class="responsive-grid cols-2 lg:cols-4">
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Total Lines</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ logStats.total }}</p>
                        </div>
                    </div>
                </div>
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-red-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Errors</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ logStats.errors }}</p>
                        </div>
                    </div>
                </div>
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Warnings</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ logStats.warnings }}</p>
                        </div>
                    </div>
                </div>
                <div class="enhanced-card p-4">
                    <div class="flex items-center">
                        <div class="flex-shrink-0">
                            <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center">
                                <svg class="w-5 h-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                </svg>
                            </div>
                        </div>
                        <div class="ml-3">
                            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Info</p>
                            <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ logStats.info }}</p>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};