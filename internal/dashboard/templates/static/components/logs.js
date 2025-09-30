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
            filterLevel: 'all',
            highlightErrors: true,
            showTimestamps: true,
            lineWrap: false,
            fontSizeClass: 'text-xs'
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
        getLogLevelColor(level) {
            switch (level) {
                case 'ERROR': return 'text-red-400';
                case 'WARN': return 'text-yellow-400';
                case 'INFO': return 'text-cyan-400';
                case 'DEBUG': return 'text-purple-400';
                default: return 'text-green-400';
            }
        },
        getLogLevelBadgeClass(level) {
            switch (level) {
                case 'ERROR': return 'bg-red-500/20 text-red-300 border-red-500/30';
                case 'WARN': return 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30';
                case 'INFO': return 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30';
                case 'DEBUG': return 'bg-purple-500/20 text-purple-300 border-purple-500/30';
                default: return 'bg-green-500/20 text-green-300 border-green-500/30';
            }
        },
        getLogLevelIcon(level) {
            switch (level) {
                case 'ERROR':
                    return '<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>';
                case 'WARN':
                    return '<path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>';
                case 'INFO':
                    return '<path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd"></path>';
                case 'DEBUG':
                    return '<path d="M10 12a2 2 0 100-4 2 2 0 000 4z"></path><path fill-rule="evenodd" d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z" clip-rule="evenodd"></path>';
                default:
                    return '<path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>';
            }
        },
        formatLogTimestamp(timestamp) {
            try {
                const date = new Date(timestamp);
                const hours = date.getHours().toString().padStart(2, '0');
                const minutes = date.getMinutes().toString().padStart(2, '0');
                const seconds = date.getSeconds().toString().padStart(2, '0');
                const ms = date.getMilliseconds().toString().padStart(3, '0');

                return `${hours}:${minutes}:${seconds}.${ms}`;
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
        scrollToTop() {
            const container = this.$refs.logContainer;
            if (container) {
                container.scrollTop = 0;
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
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col space-y-4">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center space-x-3">
                            <div class="flex-shrink-0">
                                <div class="w-10 h-10 bg-gradient-to-br from-gray-700 via-gray-800 to-gray-900 rounded-lg flex items-center justify-center border border-gray-700 shadow-lg">
                                    <svg class="w-6 h-6 text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"></path>
                                    </svg>
                                </div>
                            </div>
                            <div>
                                <h3 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
                                    Terminal Logs
                                    <span v-if="streaming" class="ml-3 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-500/20 text-green-400 border border-green-500/30">
                                        <span class="w-2 h-2 bg-green-400 rounded-full mr-1.5 animate-pulse"></span>
                                        LIVE
                                    </span>
                                </h3>
                                <p class="text-sm text-gray-500 dark:text-gray-400">Real-time log streaming and monitoring</p>
                            </div>
                        </div>
                        <div v-if="selectedServer && logs.length > 0" class="hidden lg:flex items-center space-x-4 text-xs">
                            <div class="flex items-center space-x-2 px-3 py-1.5 rounded-lg bg-gray-100 dark:bg-gray-800 border border-gray-200 dark:border-gray-700">
                                <div class="w-2 h-2 bg-cyan-400 rounded-full"></div>
                                <span class="text-gray-600 dark:text-gray-400">{{ logStats.total }} lines</span>
                            </div>
                            <div v-if="logStats.errors > 0" class="flex items-center space-x-2 px-3 py-1.5 rounded-lg bg-red-50 dark:bg-red-500/10 border border-red-200 dark:border-red-500/20">
                                <div class="w-2 h-2 bg-red-400 rounded-full"></div>
                                <span class="text-red-600 dark:text-red-400">{{ logStats.errors }} errors</span>
                            </div>
                            <div v-if="logStats.warnings > 0" class="flex items-center space-x-2 px-3 py-1.5 rounded-lg bg-yellow-50 dark:bg-yellow-500/10 border border-yellow-200 dark:border-yellow-500/20">
                                <div class="w-2 h-2 bg-yellow-400 rounded-full"></div>
                                <span class="text-yellow-600 dark:text-yellow-400">{{ logStats.warnings }} warnings</span>
                            </div>
                        </div>
                    </div>

                    <div class="flex flex-col lg:flex-row gap-3">
                        <div class="flex-1 relative">
                            <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                <svg class="h-4 w-4 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"></path>
                                </svg>
                            </div>
                            <input
                                v-model="searchTerm"
                                type="text"
                                placeholder="Search logs..."
                                class="form-input pl-10 pr-10 w-full font-mono text-sm"
                            >
                            <div v-if="searchTerm" class="absolute inset-y-0 right-0 pr-3 flex items-center">
                                <button @click="searchTerm = ''" class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
                                    <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                                        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                                    </svg>
                                </button>
                            </div>
                        </div>
                        <select
                            v-model="selectedServer"
                            class="form-input w-full lg:w-56 font-mono text-sm"
                        >
                            <option value="">Select server...</option>
                            <option v-for="server in servers" :key="server.name" :value="server.name">
                                {{ server.name }}
                            </option>
                        </select>
                        <select v-model="filterLevel" class="form-input w-full lg:w-40 font-mono text-sm">
                            <option value="all">All Levels</option>
                            <option value="ERROR">ERROR</option>
                            <option value="WARN">WARN</option>
                            <option value="INFO">INFO</option>
                            <option value="DEBUG">DEBUG</option>
                        </select>
                    </div>

                    <div class="flex flex-wrap gap-2">
                        <button
                            @click="toggleStreaming"
                            :disabled="!selectedServer"
                            :class="[
                                'touch-target inline-flex items-center px-3 py-2 text-sm font-medium rounded-lg transition-all duration-200',
                                streaming
                                    ? 'text-white bg-red-600 hover:bg-red-700 shadow-lg shadow-red-500/25'
                                    : 'text-white bg-green-600 hover:bg-green-700 shadow-lg shadow-green-500/25',
                                'disabled:opacity-50 disabled:cursor-not-allowed disabled:shadow-none'
                            ]"
                        >
                            <svg v-if="streaming" class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clip-rule="evenodd"></path>
                            </svg>
                            <svg v-else class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                            </svg>
                            {{ streaming ? 'Stop' : 'Stream' }}
                        </button>
                        <button
                            @click="loadLogs"
                            :disabled="!selectedServer || loading"
                            class="touch-target inline-flex items-center px-3 py-2 text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 transition-all duration-200"
                        >
                            <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"></path>
                            </svg>
                            Refresh
                        </button>
                        <button
                            @click="downloadLogs"
                            :disabled="!logs.length"
                            class="touch-target inline-flex items-center px-3 py-2 text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 transition-all duration-200"
                        >
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"></path>
                            </svg>
                            Download
                        </button>
                        <button
                            @click="clearLogs"
                            :disabled="!logs.length"
                            class="touch-target inline-flex items-center px-3 py-2 text-sm font-medium rounded-lg text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 transition-all duration-200"
                        >
                            <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
                            </svg>
                            Clear
                        </button>
                        <div class="flex-1"></div>
                        <div class="flex items-center space-x-4">
                            <label class="inline-flex items-center cursor-pointer">
                                <input v-model="showTimestamps" type="checkbox" class="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500">
                                <span class="ml-2 text-xs text-gray-700 dark:text-gray-300">Timestamps</span>
                            </label>
                            <label class="inline-flex items-center cursor-pointer">
                                <input v-model="autoScroll" type="checkbox" class="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500">
                                <span class="ml-2 text-xs text-gray-700 dark:text-gray-300">Auto-scroll</span>
                            </label>
                            <label class="inline-flex items-center cursor-pointer">
                                <input v-model="lineWrap" type="checkbox" class="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500">
                                <span class="ml-2 text-xs text-gray-700 dark:text-gray-300">Wrap</span>
                            </label>
                        </div>
                    </div>
                </div>
            </div>

            <div v-if="error" class="enhanced-card border-red-500/50 bg-red-500/10 p-4">
                <div class="flex items-start">
                    <div class="flex-shrink-0">
                        <svg class="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                        </svg>
                    </div>
                    <div class="ml-3 flex-1">
                        <h3 class="text-sm font-medium text-red-300">Error loading logs</h3>
                        <div class="mt-2 text-sm text-red-400 font-mono">{{ error }}</div>
                        <button @click="error = ''" class="mt-3 text-sm text-red-400 hover:text-red-300 underline">
                            Dismiss
                        </button>
                    </div>
                </div>
            </div>

            <div class="enhanced-card overflow-hidden border border-gray-700 shadow-2xl">
                <div class="bg-gradient-to-r from-gray-800 via-gray-850 to-gray-900 px-4 py-3 border-b border-gray-700">
                    <div class="flex items-center justify-between">
                        <div class="flex items-center space-x-3">
                            <div class="flex space-x-1.5">
                                <div class="w-3 h-3 rounded-full bg-red-500 shadow-lg shadow-red-500/50"></div>
                                <div class="w-3 h-3 rounded-full bg-yellow-500 shadow-lg shadow-yellow-500/50"></div>
                                <div class="w-3 h-3 rounded-full bg-green-500 shadow-lg shadow-green-500/50"></div>
                            </div>
                            <div class="text-xs font-medium text-gray-400 font-mono">
                                <span v-if="selectedServer">{{ selectedServer }}</span>
                                <span v-else>No server selected</span>
                            </div>
                        </div>
                        <div class="flex items-center space-x-2">
                            <button
                                v-if="logs.length > 0"
                                @click="scrollToTop"
                                class="text-gray-400 hover:text-gray-300 transition-colors"
                                title="Scroll to top"
                            >
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18"></path>
                                </svg>
                            </button>
                            <button
                                v-if="logs.length > 0"
                                @click="scrollToBottom"
                                class="text-gray-400 hover:text-gray-300 transition-colors"
                                title="Scroll to bottom"
                            >
                                <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3"></path>
                                </svg>
                            </button>
                            <div class="text-xs text-gray-500 font-mono">
                                {{ filteredLogs.length }} / {{ logs.length }}
                            </div>
                        </div>
                    </div>
                </div>

                <div class="bg-gray-950 relative">
                    <div v-if="loading" class="absolute inset-0 flex items-center justify-center bg-gray-950/90 z-10 backdrop-blur-sm">
                        <div class="text-center">
                            <div class="animate-spin rounded-full h-10 w-10 border-2 border-gray-700 border-t-cyan-500 mx-auto mb-4"></div>
                            <p class="text-gray-400 font-mono text-sm">Loading logs...</p>
                        </div>
                    </div>

                    <div
                        ref="logContainer"
                        class="h-[600px] overflow-y-auto custom-scrollbar font-mono text-xs leading-relaxed"
                        style="scrollbar-width: thin; scrollbar-color: #374151 #1f2937;"
                    >
                        <div v-if="!selectedServer" class="flex items-center justify-center h-full">
                            <div class="text-center p-8">
                                <svg class="mx-auto h-16 w-16 text-gray-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"></path>
                                </svg>
                                <p class="text-gray-500 font-mono text-sm">Select a server to view logs</p>
                                <p class="text-gray-600 font-mono text-xs mt-2">Choose from the dropdown above</p>
                            </div>
                        </div>

                        <div v-else-if="filteredLogs.length === 0" class="flex items-center justify-center h-full">
                            <div class="text-center p-8">
                                <svg class="mx-auto h-16 w-16 text-gray-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                                </svg>
                                <p class="text-gray-500 font-mono text-sm">No logs available</p>
                                <p class="text-gray-600 font-mono text-xs mt-2">{{ selectedServer }} has no log entries</p>
                                <button
                                    @click="loadLogs"
                                    class="mt-4 text-cyan-400 hover:text-cyan-300 font-mono text-xs underline"
                                >
                                    Refresh logs
                                </button>
                            </div>
                        </div>

                        <div v-else class="p-3">
                            <div
                                v-for="log in filteredLogs"
                                :key="log.id"
                                :class="[
                                    'group flex items-start py-1.5 px-2 rounded transition-colors duration-100',
                                    'hover:bg-gray-900/50',
                                    log.level === 'ERROR' && highlightErrors ? 'bg-red-500/5 border-l-2 border-red-500/50' : '',
                                    log.level === 'WARN' && highlightErrors ? 'bg-yellow-500/5 border-l-2 border-yellow-500/50' : ''
                                ]"
                            >
                                <div class="flex items-start space-x-3 flex-1" :class="lineWrap ? 'flex-wrap' : ''">
                                    <div v-if="showTimestamps" class="text-gray-600 select-none flex-shrink-0 w-24">
                                        {{ formatLogTimestamp(log.timestamp) }}
                                    </div>

                                    <div :class="['inline-flex items-center px-2 py-0.5 rounded border font-medium flex-shrink-0 w-16 justify-center', getLogLevelBadgeClass(log.level)]">
                                        <svg class="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20" v-html="getLogLevelIcon(log.level)"></svg>
                                        <span class="text-[10px]">{{ log.level }}</span>
                                    </div>

                                    <div :class="['flex-1 min-w-0', getLogLevelColor(log.level), lineWrap ? 'break-all' : 'truncate']">
                                        {{ log.message }}
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>

                <div class="bg-gradient-to-r from-gray-800 via-gray-850 to-gray-900 px-4 py-2 border-t border-gray-700">
                    <div class="flex items-center justify-between text-xs font-mono">
                        <div class="flex items-center space-x-4">
                            <div class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full bg-cyan-400"></div>
                                <span class="text-gray-400">{{ logStats.total }} total</span>
                            </div>
                            <div v-if="logStats.errors > 0" class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full bg-red-400"></div>
                                <span class="text-gray-400">{{ logStats.errors }} errors</span>
                            </div>
                            <div v-if="logStats.warnings > 0" class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full bg-yellow-400"></div>
                                <span class="text-gray-400">{{ logStats.warnings }} warnings</span>
                            </div>
                            <div v-if="logStats.info > 0" class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full bg-cyan-400"></div>
                                <span class="text-gray-400">{{ logStats.info }} info</span>
                            </div>
                            <div v-if="logStats.debug > 0" class="flex items-center space-x-2">
                                <div class="w-2 h-2 rounded-full bg-purple-400"></div>
                                <span class="text-gray-400">{{ logStats.debug }} debug</span>
                            </div>
                        </div>
                        <div class="text-gray-500">
                            <span v-if="streaming" class="text-green-400">Streaming active</span>
                            <span v-else>Ready</span>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `
};