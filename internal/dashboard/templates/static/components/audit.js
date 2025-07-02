const AuditLog = {
    emits: ['show-toast'],
    data() {
        return {
            loading: false,
            error: null,
            entries: [],
            stats: null,
            selectedEntry: null,
            // Enhanced filtering
            filters: {
                event: '',
                success: '',
                timeRange: '24h',
                search: ''
            },
            // Pagination
            currentPage: 1,
            pageSize: 20,
            totalEntries: 0,
            totalPages: 0,
            // UI state
            sortBy: 'timestamp',
            sortOrder: 'desc',
            autoRefresh: false,
            refreshInterval: null,
            expandedRows: new Set(),
            selectedRows: new Set(),
            // Advanced view
            showAdvancedFilters: false,
            viewMode: 'table'
        }
    },
    computed: {
        eventTypes() {
            return [
                { value: '', label: 'All Events' },
                { value: 'oauth.token.issued', label: 'Token Issued', icon: 'key', color: 'green' },
                { value: 'oauth.token.revoked', label: 'Token Revoked', icon: 'x-circle', color: 'red' },
                { value: 'oauth.user.login', label: 'User Login', icon: 'user-circle', color: 'blue' },
                { value: 'oauth.user.logout', label: 'User Logout', icon: 'logout', color: 'gray' },
                { value: 'server.access.granted', label: 'Access Granted', icon: 'check-circle', color: 'green' },
                { value: 'server.access.denied', label: 'Access Denied', icon: 'shield-exclamation', color: 'red' },
                { value: 'oauth.client.created', label: 'Client Created', icon: 'plus-circle', color: 'blue' },
                { value: 'oauth.client.deleted', label: 'Client Deleted', icon: 'trash', color: 'red' },
                { value: 'system.config.changed', label: 'Config Changed', icon: 'cog', color: 'yellow' }
            ];
        },
        timeRangeOptions() {
            return [
                { value: '1h', label: 'Last Hour' },
                { value: '24h', label: 'Last 24 Hours' },
                { value: '7d', label: 'Last 7 Days' },
                { value: '30d', label: 'Last 30 Days' },
                { value: 'all', label: 'All Time' }
            ];
        },
        statusCounts() {
            if (!this.stats) return { total: 0, success: 0, failure: 0, rate: 0 };
            return {
                total: this.stats.total_entries || 0,
                success: this.stats.success_count || 0,
                failure: this.stats.failure_count || 0,
                rate: this.stats.success_rate || 0
            };
        },
        filteredStats() {
            const eventCounts = this.stats?.event_counts || {};
            return Object.entries(eventCounts)
                .map(([event, count]) => ({
                    event,
                    count,
                    config: this.eventTypes.find(t => t.value === event) || { label: event, color: 'gray' }
                }))
                .sort((a, b) => b.count - a.count);
        }
    },
    async mounted() {
        await this.loadData();
        this.setupAutoRefresh();
    },
    beforeUnmount() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    },
    methods: {
        // Heroicon helper
        getHeroIcon(iconName) {
            const icons = {
                'document-text': 'M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z',
                'search': 'M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z',
                'filter': 'M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.207A1 1 0 013 6.5V4z',
                'refresh': 'M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15',
                'chevron-down': 'M19 9l-7 7-7-7',
                'chevron-left': 'M15 19l-7-7 7-7',
                'chevron-right': 'M9 5l7 7-7 7',
                'chart-bar': 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
                'eye': 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z',
                'x': 'M6 18L18 6M6 6l12 12',
                'check-circle': 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z',
                'x-circle': 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z',
                'exclamation-triangle': 'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.732-.833-2.464 0L4.34 16.5c-.77.833.192 2.5 1.732 2.5z',
                'key': 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z',
                'user-circle': 'M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z',
                'logout': 'M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1',
                'shield-exclamation': 'M12 9v2m0 4h.01M5 19h14a2 2 0 002-2v-5a2 2 0 00-2-2M5 19a2 2 0 01-2-2v-5a2 2 0 012-2m0 6V9a2 2 0 012-2h10a2 2 0 012 2v10m-9 2h4a2 2 0 002-2v-5a2 2 0 00-2-2H8a2 2 0 00-2 2v5a2 2 0 002 2z',
                'plus-circle': 'M12 4v16m8-8H4',
                'trash': 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16',
                'cog': 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z',
                'clock': 'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z',
                'calendar': 'M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
                'download': 'M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4',
                'view-grid': 'M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z',
                'view-list': 'M4 6h16v2H4zm0 5h16v2H4zm0 5h16v2H4z'
            };
            return icons[iconName] || icons['document-text'];
        },

        async loadData() {
            await Promise.all([
                this.loadEntries(),
                this.loadStats()
            ]);
        },

        async loadEntries() {
            this.loading = true;
            this.error = null;
            try {
                const params = new URLSearchParams({
                    page: this.currentPage,
                    limit: this.pageSize,
                    sort: this.sortBy,
                    order: this.sortOrder,
                    ...(this.filters.event && { event: this.filters.event }),
                    ...(this.filters.success !== '' && { success: this.filters.success }),
                    ...(this.filters.timeRange !== 'all' && { timeRange: this.filters.timeRange }),
                    ...(this.filters.search && { search: this.filters.search })
                });
                
                const response = await fetch(`/api/audit/entries?${params}`);
                if (response.ok && response.headers.get('content-type')?.includes('application/json')) {
                    const data = await response.json();
                    this.entries = data.entries || [];
                    this.totalEntries = data.total || 0;
                    this.totalPages = Math.ceil(this.totalEntries / this.pageSize);
                } else {
                    console.warn('Audit entries endpoint not available');
                    this.entries = [];
                    this.totalEntries = 0;
                    this.totalPages = 0;
                    if (!response.ok) {
                        this.error = 'Failed to load audit entries';
                    }
                }
            } catch (error) {
                this.error = `Failed to load audit entries: ${error.message}`;
                console.error('Failed to load audit entries:', error);
                this.entries = [];
                this.totalEntries = 0;
                this.totalPages = 0;
            } finally {
                this.loading = false;
            }
        },

        async loadStats() {
            try {
                const response = await fetch('/api/audit/stats');
                if (response.ok && response.headers.get('content-type')?.includes('application/json')) {
                    this.stats = await response.json();
                } else {
                    console.warn('Audit stats endpoint not available');
                    this.stats = null;
                }
            } catch (error) {
                console.error('Failed to load audit stats:', error);
                this.stats = null;
            }
        },

        // UI management methods
        toggleRowExpansion(entryId) {
            if (this.expandedRows.has(entryId)) {
                this.expandedRows.delete(entryId);
            } else {
                this.expandedRows.add(entryId);
            }
            this.$forceUpdate();
        },

        isRowExpanded(entryId) {
            return this.expandedRows.has(entryId);
        },

        setupAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
            if (this.autoRefresh) {
                this.refreshInterval = setInterval(() => {
                    this.loadEntries();
                }, 30000);
            }
        },

        // Formatting methods
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                const date = new Date(timestamp);
                const now = new Date();
                const diffMs = now - date;
                const diffMinutes = Math.floor(diffMs / (1000 * 60));
                const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
                const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
                
                if (diffMinutes < 1) return 'Just now';
                if (diffMinutes < 60) return `${diffMinutes}m ago`;
                if (diffHours < 24) return `${diffHours}h ago`;
                if (diffDays < 7) return `${diffDays}d ago`;
                
                return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], {
                    hour: '2-digit',
                    minute: '2-digit'
                });
            } catch (e) {
                return timestamp;
            }
        },

        formatEventName(event) {
            const type = this.eventTypes.find(t => t.value === event);
            return type ? type.label : event.replace(/\./g, ' ').replace(/\b\w/g, l => l.toUpperCase());
        },

        getEventIcon(event) {
            const type = this.eventTypes.find(t => t.value === event);
            return type ? type.icon : 'document-text';
        },

        getEventColor(event) {
            const type = this.eventTypes.find(t => t.value === event);
            return type ? type.color : 'gray';
        },

        // Modal and detail methods
        viewDetails(entry) {
            this.selectedEntry = entry;
        },

        closeDetails() {
            this.selectedEntry = null;
        },

        // Pagination methods
        nextPage() {
            if (this.currentPage < this.totalPages) {
                this.currentPage++;
                this.loadEntries();
            }
        },

        previousPage() {
            if (this.currentPage > 1) {
                this.currentPage--;
                this.loadEntries();
            }
        },

        goToPage(page) {
            if (page >= 1 && page <= this.totalPages && page !== this.currentPage) {
                this.currentPage = page;
                this.loadEntries();
            }
        },

        // Filter and sort methods
        resetFilters() {
            this.filters = {
                event: '',
                success: '',
                timeRange: '24h',
                search: ''
            };
            this.currentPage = 1;
            this.loadEntries();
        },

        applySort(field) {
            if (this.sortBy === field) {
                this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortBy = field;
                this.sortOrder = 'desc';
            }
            this.loadEntries();
        },

        // Export functionality
        async exportData() {
            try {
                const params = new URLSearchParams({
                    ...this.filters,
                    format: 'csv',
                    limit: 10000 // Large limit for export
                });
                
                const response = await fetch(`/api/audit/export?${params}`);
                if (response.ok) {
                    const blob = await response.blob();
                    const url = window.URL.createObjectURL(blob);
                    const a = document.createElement('a');
                    a.href = url;
                    a.download = `audit-log-${new Date().toISOString().split('T')[0]}.csv`;
                    document.body.appendChild(a);
                    a.click();
                    a.remove();
                    window.URL.revokeObjectURL(url);
                    this.$emit('show-toast', { message: 'Audit log exported successfully', type: 'success' });
                } else {
                    throw new Error('Export failed');
                }
            } catch (error) {
                this.$emit('show-toast', { message: `Export failed: ${error.message}`, type: 'error' });
            }
        }
    },

    watch: {
        'filters.event'() { this.currentPage = 1; this.loadEntries(); },
        'filters.success'() { this.currentPage = 1; this.loadEntries(); },
        'filters.timeRange'() { this.currentPage = 1; this.loadEntries(); },
        'filters.search'() { 
            clearTimeout(this.searchTimeout);
            this.searchTimeout = setTimeout(() => {
                this.currentPage = 1; 
                this.loadEntries();
            }, 500);
        },
        autoRefresh() { this.setupAutoRefresh(); }
    },

    template: `
        <div class="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
            <!-- Enhanced Header -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="flex flex-col space-y-4">
                    <!-- Title and Controls -->
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div class="flex items-center space-x-3">
                            <div class="w-10 h-10 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-xl flex items-center justify-center">
                                <svg class="w-6 h-6 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('document-text')"></path>
                                </svg>
                            </div>
                            <div>
                                <h3 class="text-lg font-semibold text-gray-100">Audit Logs</h3>
                                <p class="text-sm text-gray-300">Track security events and system activities</p>
                            </div>
                        </div>
                        
                        <!-- Action Buttons -->
                        <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                            <button
                                @click="exportData"
                                class="inline-flex items-center px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 transition-colors font-medium text-sm touch-target"
                            >
                                <svg class="w-4 h-4 mr-2 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('download')"></path>
                                </svg>
                                Export CSV
                            </button>
                            
                            <button
                                @click="loadEntries"
                                :disabled="loading"
                                class="inline-flex items-center px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors font-medium text-sm touch-target disabled:opacity-50"
                            >
                                <svg class="w-4 h-4 mr-2 heroicon" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('refresh')"></path>
                                </svg>
                                Refresh
                            </button>
                            
                            <!-- Auto-refresh Toggle -->
                            <label class="inline-flex items-center">
                                <input v-model="autoRefresh" type="checkbox" class="form-checkbox h-4 w-4 text-blue-600 rounded">
                                <span class="ml-2 text-sm text-gray-300">Auto-refresh</span>
                            </label>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Stats Overview -->
            <div v-if="stats" class="grid grid-cols-2 lg:grid-cols-4 gap-4">
                <div class="enhanced-card p-4">
                    <div class="flex items-center space-x-3">
                        <div class="w-10 h-10 bg-blue-500 rounded-lg flex items-center justify-center">
                            <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chart-bar')"></path>
                            </svg>
                        </div>
                        <div>
                            <p class="text-2xl font-bold text-gray-100">{{ statusCounts.total.toLocaleString() }}</p>
                            <p class="text-xs text-gray-300">Total Events</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center space-x-3">
                        <div class="w-10 h-10 bg-green-500 rounded-lg flex items-center justify-center">
                            <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('check-circle')"></path>
                            </svg>
                        </div>
                        <div>
                            <p class="text-2xl font-bold text-gray-100">{{ statusCounts.rate.toFixed(1) }}%</p>
                            <p class="text-xs text-gray-300">Success Rate</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center space-x-3">
                        <div class="w-10 h-10 bg-emerald-500 rounded-lg flex items-center justify-center">
                            <div class="w-2 h-2 bg-white rounded-full"></div>
                        </div>
                        <div>
                            <p class="text-2xl font-bold text-gray-100">{{ statusCounts.success.toLocaleString() }}</p>
                            <p class="text-xs text-gray-300">Successful</p>
                        </div>
                    </div>
                </div>
                
                <div class="enhanced-card p-4">
                    <div class="flex items-center space-x-3">
                        <div class="w-10 h-10 bg-red-500 rounded-lg flex items-center justify-center">
                            <svg class="w-5 h-5 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                            </svg>
                        </div>
                        <div>
                            <p class="text-2xl font-bold text-gray-100">{{ statusCounts.failure.toLocaleString() }}</p>
                            <p class="text-xs text-gray-300">Failed</p>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Error Display -->
            <div v-if="error" class="enhanced-card border-red-500 bg-red-900/20 p-4">
                <div class="flex items-start">
                    <svg class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x-circle')"></path>
                    </svg>
                    <div class="ml-3 flex-1">
                        <div class="text-sm text-red-200">{{ error }}</div>
                        <button @click="error = null" class="mt-2 text-xs text-red-400 hover:text-red-300 underline touch-target">
                            Dismiss
                        </button>
                    </div>
                </div>
            </div>

            <!-- Filters and Controls -->
            <div class="enhanced-card p-4 lg:p-6">
                <div class="space-y-4">
                    <!-- Primary Filters -->
                    <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                        <div class="flex flex-col sm:flex-row space-y-3 sm:space-y-0 sm:space-x-3 flex-1 max-w-4xl">
                            <!-- Search -->
                            <div class="relative flex-1">
                                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                                    <svg class="h-4 w-4 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('search')"></path>
                                    </svg>
                                </div>
                                <input 
                                    v-model="filters.search"
                                    type="text" 
                                    placeholder="Search logs..." 
                                    class="form-input pl-10 w-full"
                                >
                            </div>
                            
                            <!-- Event Filter -->
                            <select v-model="filters.event" class="form-input w-full sm:w-auto">
                                <option v-for="eventType in eventTypes" :key="eventType.value" :value="eventType.value">
                                    {{ eventType.label }}
                                </option>
                            </select>
                            
                            <!-- Success Filter -->
                            <select v-model="filters.success" class="form-input w-full sm:w-auto">
                                <option value="">All Results</option>
                                <option value="true">Success Only</option>
                                <option value="false">Failures Only</option>
                            </select>
                            
                            <!-- Time Range Filter -->
                            <select v-model="filters.timeRange" class="form-input w-full sm:w-auto">
                                <option v-for="range in timeRangeOptions" :key="range.value" :value="range.value">
                                    {{ range.label }}
                                </option>
                            </select>
                        </div>
                        
                        <!-- Filter Actions -->
                        <div class="flex items-center space-x-3">
                            <button
                                @click="resetFilters"
                                class="px-3 py-2 text-sm text-gray-300 border border-gray-600 rounded-lg hover:bg-gray-700 transition-colors touch-target"
                            >
                                Clear
                            </button>
                            <span class="text-sm text-gray-400">
                                {{ totalEntries.toLocaleString() }} entries
                            </span>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Loading State -->
            <div v-if="loading && entries.length === 0" class="enhanced-card p-8 text-center">
                <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4"></div>
                <p class="text-lg font-medium text-gray-100">Loading audit entries...</p>
                <p class="text-sm text-gray-300">Fetching security and activity logs</p>
            </div>

            <!-- Empty State -->
            <div v-else-if="entries.length === 0 && !loading" class="enhanced-card p-8 text-center">
                <svg class="mx-auto h-12 w-12 text-gray-500 mb-4 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('document-text')"></path>
                </svg>
                <h3 class="text-lg font-medium text-gray-100 mb-2">No audit entries found</h3>
                <p class="text-gray-300 mb-4">
                    {{ Object.values(filters).some(f => f) 
                        ? 'Try adjusting your filters or time range'
                        : 'Audit entries will appear here when available' }}
                </p>
                <button
                    v-if="Object.values(filters).some(f => f)"
                    @click="resetFilters"
                    class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors font-medium"
                >
                    Clear Filters
                </button>
            </div>

            <!-- Audit Table -->
            <div v-else class="enhanced-card overflow-hidden">
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-700">
                        <thead class="bg-gray-800">
                            <tr>
                                <th 
                                    @click="applySort('timestamp')"
                                    class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider cursor-pointer hover:bg-gray-700 transition-colors"
                                >
                                    <div class="flex items-center space-x-1">
                                        <span>Timestamp</span>
                                        <svg v-if="sortBy === 'timestamp'" :class="['w-4 h-4 heroicon', sortOrder === 'asc' ? '' : 'rotate-180']" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                        </svg>
                                    </div>
                                </th>
                                <th 
                                    @click="applySort('event')"
                                    class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider cursor-pointer hover:bg-gray-700 transition-colors"
                                >
                                    <div class="flex items-center space-x-1">
                                        <span>Event</span>
                                        <svg v-if="sortBy === 'event'" :class="['w-4 h-4 heroicon', sortOrder === 'asc' ? '' : 'rotate-180']" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                        </svg>
                                    </div>
                                </th>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">User/Client</th>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Source</th>
                                <th 
                                    @click="applySort('success')"
                                    class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider cursor-pointer hover:bg-gray-700 transition-colors"
                                >
                                    <div class="flex items-center space-x-1">
                                        <span>Result</span>
                                        <svg v-if="sortBy === 'success'" :class="['w-4 h-4 heroicon', sortOrder === 'asc' ? '' : 'rotate-180']" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-down')"></path>
                                        </svg>
                                    </div>
                                </th>
                                <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Actions</th>
                            </tr>
                        </thead>
                        <tbody class="bg-gray-800 divide-y divide-gray-700">
                            <tr 
                                v-for="entry in entries" 
                                :key="entry.id"
                                :class="[
                                    'hover:bg-gray-700/30 transition-colors',
                                    !entry.success ? 'bg-red-900/10' : ''
                                ]"
                            >
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <div class="flex items-center space-x-2">
                                        <svg class="w-3 h-3 text-gray-400 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('clock')"></path>
                                        </svg>
                                        <div class="text-sm text-gray-100">{{ formatTimestamp(entry.timestamp) }}</div>
                                    </div>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <div class="flex items-center space-x-2">
                                        <div :class="[
                                            'w-8 h-8 rounded-lg flex items-center justify-center',
                                            \`bg-\${getEventColor(entry.event)}-500\`
                                        ]">
                                            <svg class="w-4 h-4 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(getEventIcon(entry.event))"></path>
                                            </svg>
                                        </div>
                                        <span class="text-sm font-medium text-gray-100">{{ formatEventName(entry.event) }}</span>
                                    </div>
                                </td>
                                <td class="px-6 py-4">
                                    <div class="text-sm">
                                        <div v-if="entry.user_id" class="text-gray-100 font-medium">{{ entry.user_id }}</div>
                                        <div v-if="entry.client_id" class="text-gray-400 truncate max-w-32">{{ entry.client_id }}</div>
                                        <div v-if="!entry.user_id && !entry.client_id" class="text-gray-500">System</div>
                                    </div>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <code v-if="entry.ip_address" class="text-xs bg-gray-700 text-gray-200 px-2 py-1 rounded">
                                        {{ entry.ip_address }}
                                    </code>
                                    <span v-else class="text-gray-500 text-sm">-</span>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <div class="space-y-1">
                                        <span :class="[
                                            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                            entry.success 
                                                ? 'bg-green-900 text-green-200' 
                                                : 'bg-red-900 text-red-200'
                                        ]">
                                            <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(entry.success ? 'check-circle' : 'x-circle')"></path>
                                            </svg>
                                            {{ entry.success ? 'Success' : 'Failed' }}
                                        </span>
                                        <div v-if="entry.error" class="text-xs text-red-400 truncate max-w-32" :title="entry.error">
                                            {{ entry.error }}
                                        </div>
                                    </div>
                                </td>
                                <td class="px-6 py-4 whitespace-nowrap">
                                    <button 
                                        @click="viewDetails(entry)" 
                                        class="inline-flex items-center px-2 py-1 text-blue-400 hover:text-blue-300 text-sm touch-target transition-colors"
                                    >
                                        <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('eye')"></path>
                                        </svg>
                                        Details
                                    </button>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                </div>

                <!-- Pagination -->
                <div v-if="totalPages > 1" class="bg-gray-800 px-6 py-3 border-t border-gray-700">
                    <div class="flex flex-col sm:flex-row items-center justify-between space-y-3 sm:space-y-0">
                        <div class="flex items-center space-x-2">
                            <span class="text-sm text-gray-400">
                                Showing {{ (currentPage - 1) * pageSize + 1 }}-{{ Math.min(currentPage * pageSize, totalEntries) }} of {{ totalEntries.toLocaleString() }}
                            </span>
                        </div>
                        
                        <div class="flex items-center space-x-2">
                            <button 
                                @click="previousPage" 
                                :disabled="currentPage <= 1"
                                class="p-2 text-gray-400 hover:text-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors touch-target"
                            >
                                <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-left')"></path>
                                </svg>
                            </button>
                            
                            <!-- Page Numbers -->
                            <div class="flex items-center space-x-1">
                                <button 
                                    v-for="page in Math.min(5, totalPages)" 
                                    :key="page"
                                    @click="goToPage(page)"
                                    :class="[
                                        'px-3 py-1 text-sm rounded transition-colors touch-target',
                                        page === currentPage 
                                            ? 'bg-blue-600 text-white' 
                                            : 'text-gray-400 hover:text-gray-200 hover:bg-gray-700'
                                    ]"
                                >
                                    {{ page }}
                                </button>
                                
                                <span v-if="totalPages > 5" class="text-gray-500 px-2">...</span>
                                
                                <button 
                                    v-if="totalPages > 5 && currentPage < totalPages"
                                    @click="goToPage(totalPages)"
                                    :class="[
                                        'px-3 py-1 text-sm rounded transition-colors touch-target',
                                        totalPages === currentPage 
                                            ? 'bg-blue-600 text-white' 
                                            : 'text-gray-400 hover:text-gray-200 hover:bg-gray-700'
                                    ]"
                                >
                                    {{ totalPages }}
                                </button>
                            </div>
                            
                            <button 
                                @click="nextPage" 
                                :disabled="currentPage >= totalPages"
                                class="p-2 text-gray-400 hover:text-gray-200 disabled:opacity-50 disabled:cursor-not-allowed transition-colors touch-target"
                            >
                                <svg class="w-5 h-5 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('chevron-right')"></path>
                                </svg>
                            </button>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Details Modal -->
            <div v-if="selectedEntry" class="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4 overflow-y-auto">
                <div class="bg-gray-800 border border-gray-700 rounded-lg max-w-4xl w-full max-h-[90vh] overflow-y-auto">
                    <div class="flex items-center justify-between p-6 border-b border-gray-700">
                        <h3 class="text-lg font-medium text-gray-100">Audit Entry Details</h3>
                        <button 
                            @click="closeDetails" 
                            class="text-gray-400 hover:text-gray-200 transition-colors touch-target"
                        >
                            <svg class="w-6 h-6 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon('x')"></path>
                            </svg>
                        </button>
                    </div>
                    
                    <div class="p-6 space-y-6">
                        <!-- Basic Information -->
                        <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
                            <div class="space-y-4">
                                <h4 class="text-sm font-medium text-gray-300 uppercase tracking-wide">Basic Information</h4>
                                
                                <div class="space-y-3">
                                    <div>
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Event ID</label>
                                        <code class="text-sm bg-gray-700 text-gray-200 px-2 py-1 rounded block">{{ selectedEntry.id }}</code>
                                    </div>
                                    
                                    <div>
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Timestamp</label>
                                        <p class="text-sm text-gray-100">{{ formatTimestamp(selectedEntry.timestamp) }}</p>
                                    </div>
                                    
                                    <div>
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Event Type</label>
                                        <div class="flex items-center space-x-2">
                                            <div :class="[
                                                'w-6 h-6 rounded flex items-center justify-center',
                                                \`bg-\${getEventColor(selectedEntry.event)}-500\`
                                            ]">
                                                <svg class="w-3 h-3 text-white heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(getEventIcon(selectedEntry.event))"></path>
                                                </svg>
                                            </div>
                                            <span class="text-sm text-gray-100">{{ formatEventName(selectedEntry.event) }}</span>
                                        </div>
                                    </div>
                                    
                                    <div>
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Result</label>
                                        <span :class="[
                                            'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                            selectedEntry.success 
                                                ? 'bg-green-900 text-green-200' 
                                                : 'bg-red-900 text-red-200'
                                        ]">
                                            <svg class="w-3 h-3 mr-1 heroicon" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="getHeroIcon(selectedEntry.success ? 'check-circle' : 'x-circle')"></path>
                                            </svg>
                                            {{ selectedEntry.success ? 'Success' : 'Failed' }}
                                        </span>
                                    </div>
                                </div>
                            </div>
                            
                            <div class="space-y-4">
                                <h4 class="text-sm font-medium text-gray-300 uppercase tracking-wide">Context Information</h4>
                                
                                <div class="space-y-3">
                                    <div v-if="selectedEntry.user_id">
                                        <label class="block text-xs font-medium text-gray-400 mb-1">User ID</label>
                                        <p class="text-sm text-gray-100">{{ selectedEntry.user_id }}</p>
                                    </div>
                                    
                                    <div v-if="selectedEntry.client_id">
                                        <label class="block text-xs font-medium text-gray-400 mb-1">Client ID</label>
                                        <code class="text-sm bg-gray-700 text-gray-200 px-2 py-1 rounded break-all block">{{ selectedEntry.client_id }}</code>
                                    </div>
                                    
                                    <div v-if="selectedEntry.ip_address">
                                        <label class="block text-xs font-medium text-gray-400 mb-1">IP Address</label>
                                        <code class="text-sm bg-gray-700 text-gray-200 px-2 py-1 rounded">{{ selectedEntry.ip_address }}</code>
                                    </div>
                                    
                                    <div v-if="selectedEntry.user_agent">
                                        <label class="block text-xs font-medium text-gray-400 mb-1">User Agent</label>
                                        <p class="text-sm text-gray-100 break-words">{{ selectedEntry.user_agent }}</p>
                                    </div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Error Information -->
                        <div v-if="selectedEntry.error" class="bg-red-900/20 border border-red-800 rounded-lg p-4">
                            <h4 class="text-sm font-medium text-red-200 mb-2">Error Details</h4>
                            <p class="text-sm text-red-300 break-words">{{ selectedEntry.error }}</p>
                        </div>
                        
                        <!-- Additional Details -->
                        <div v-if="selectedEntry.details && Object.keys(selectedEntry.details).length > 0" class="space-y-3">
                            <h4 class="text-sm font-medium text-gray-300 uppercase tracking-wide">Additional Details</h4>
                            <div class="bg-gray-900 rounded-lg p-4">
                                <pre class="text-xs text-gray-300 overflow-auto whitespace-pre-wrap break-words max-h-96">{{ JSON.stringify(selectedEntry.details, null, 2) }}</pre>
                            </div>
                        </div>
                    </div>
                    
                    <div class="flex justify-end gap-3 p-6 border-t border-gray-700">
                        <button 
                            @click="closeDetails" 
                            class="px-4 py-2 border border-gray-600 text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors touch-target"
                        >
                            Close
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `
};