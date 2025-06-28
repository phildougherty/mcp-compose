const AuditLog = {
    template: `
        <div class="audit-log max-w-full overflow-x-hidden">
            <div class="enhanced-card">
                <div class="p-4 md:p-6">
                    <h3 class="text-xl font-semibold text-white mb-4">📋 Audit Logs</h3>
                    
                    <div class="flex flex-col lg:flex-row items-start lg:items-center justify-between gap-4 mb-6">
                        <div class="flex flex-col sm:flex-row flex-wrap items-start sm:items-center gap-3 w-full lg:w-auto">
                            <select v-model="filters.event" @change="loadEntries" 
                                    class="w-full sm:w-auto min-w-40 px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                    style="font-size: 16px;">
                                <option value="">All Events</option>
                                <option value="oauth.token.issued">Token Issued</option>
                                <option value="oauth.token.revoked">Token Revoked</option>
                                <option value="oauth.user.login">User Login</option>
                                <option value="server.access.granted">Access Granted</option>
                                <option value="server.access.denied">Access Denied</option>
                            </select>
                            <select v-model="filters.success" @change="loadEntries" 
                                    class="w-full sm:w-auto min-w-32 px-4 py-2 bg-gray-700 border border-gray-600 text-white rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500" 
                                    style="font-size: 16px;">
                                <option value="">All Results</option>
                                <option value="true">Success Only</option>
                                <option value="false">Failures Only</option>
                            </select>
                            <button @click="loadEntries" class="w-full sm:w-auto px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 text-sm touch-target transition-colors">
                                🔄 Refresh
                            </button>
                        </div>
                        <div class="text-sm text-gray-400 text-center lg:text-right">
                            Page {{ currentPage }} of {{ totalPages }} ({{ totalEntries }} entries)
                        </div>
                    </div>
                    
                    <!-- Stats -->
                    <div v-if="stats" class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6">
                        <div class="bg-blue-900/30 p-4 rounded-lg border border-blue-800">
                            <div class="text-2xl font-bold text-blue-400">{{ stats.total_entries }}</div>
                            <div class="text-sm text-blue-300">Total Events</div>
                        </div>
                        <div class="bg-green-900/30 p-4 rounded-lg border border-green-800">
                            <div class="text-2xl font-bold text-green-400">{{ stats.success_rate?.toFixed(1) || 0 }}%</div>
                            <div class="text-sm text-green-300">Success Rate</div>
                        </div>
                        <div class="bg-purple-900/30 p-4 rounded-lg border border-purple-800 sm:col-span-2 lg:col-span-1">
                            <div class="text-2xl font-bold text-purple-400">{{ Object.keys(stats.event_counts || {}).length }}</div>
                            <div class="text-sm text-purple-300">Event Types</div>
                        </div>
                    </div>
                    
                    <!-- Loading -->
                    <div v-if="loading" class="text-center py-8">
                        <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400 mx-auto"></div>
                        <p class="mt-2 text-gray-400">Loading audit entries...</p>
                    </div>
                    
                    <!-- Entries Table -->
                    <div v-else-if="entries.length > 0" class="overflow-x-auto -mx-4 sm:mx-0">
                        <div class="inline-block min-w-full align-middle">
                            <div class="overflow-hidden">
                                <table class="min-w-full bg-gray-800">
                                    <thead>
                                        <tr class="border-b border-gray-700">
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Timestamp</th>
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Event</th>
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">User/Client</th>
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">IP Address</th>
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Result</th>
                                            <th class="text-left py-3 px-4 text-sm font-medium text-gray-300">Actions</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <tr v-for="entry in entries" :key="entry.id"
                                            :class="['border-b border-gray-700', !entry.success ? 'bg-red-900/20' : '']">
                                            <td class="py-3 px-4 text-sm">
                                                <div class="text-white">{{ formatTimestamp(entry.timestamp) }}</div>
                                            </td>
                                            <td class="py-3 px-4 text-sm">
                                                <span class="font-medium text-white">{{ formatEventName(entry.event) }}</span>
                                            </td>
                                            <td class="py-3 px-4 text-sm">
                                                <div v-if="entry.user_id" class="text-white">User: {{ entry.user_id }}</div>
                                                <div v-if="entry.client_id" class="text-gray-300 break-words">Client: {{ entry.client_id }}</div>
                                                <div v-if="!entry.user_id && !entry.client_id" class="text-gray-400">-</div>
                                            </td>
                                            <td class="py-3 px-4 text-sm">
                                                <code v-if="entry.ip_address" class="text-xs bg-gray-700 text-gray-200 px-2 py-1 rounded break-words">{{ entry.ip_address }}</code>
                                                <span v-else class="text-gray-400">-</span>
                                            </td>
                                            <td class="py-3 px-4 text-sm">
                                                <span :class="[
                                                    'inline-flex items-center px-2 py-1 rounded-full text-xs font-medium border',
                                                    entry.success 
                                                        ? 'bg-green-900/30 text-green-200 border-green-700' 
                                                        : 'bg-red-900/30 text-red-200 border-red-700'
                                                ]">
                                                    {{ entry.success ? 'Success' : 'Failed' }}
                                                </span>
                                                <div v-if="entry.error" class="text-xs text-red-400 mt-1 break-words">{{ entry.error }}</div>
                                            </td>
                                            <td class="py-3 px-4 text-sm">
                                                <button @click="viewDetails(entry)" class="text-blue-400 hover:text-blue-200 touch-target transition-colors">
                                                    View Details
                                                </button>
                                            </td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </div>
                    
                    <!-- No entries -->
                    <div v-else class="text-center py-8 text-gray-400">
                        <svg class="mx-auto h-12 w-12 text-gray-500 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"></path>
                        </svg>
                        <p class="text-lg font-medium">No audit entries found</p>
                        <p class="text-sm text-gray-500 mt-1">Audit entries will appear here when available</p>
                    </div>
                    
                    <!-- Pagination -->
                    <div v-if="totalPages > 1" class="flex flex-col sm:flex-row items-center justify-between mt-6 gap-3">
                        <button @click="previousPage" :disabled="currentPage <= 1"
                                class="w-full sm:w-auto px-4 py-2 bg-gray-700 text-gray-300 rounded-md hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed touch-target transition-colors">
                            Previous
                        </button>
                        <span class="text-sm text-gray-400">
                            Page {{ currentPage }} of {{ totalPages }}
                        </span>
                        <button @click="nextPage" :disabled="currentPage >= totalPages"
                                class="w-full sm:w-auto px-4 py-2 bg-gray-700 text-gray-300 rounded-md hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed touch-target transition-colors">
                            Next
                        </button>
                    </div>
                </div>
            </div>
            
            <!-- Details Modal -->
            <div v-if="selectedEntry" class="fixed inset-0 bg-black bg-opacity-75 flex items-center justify-center z-50 p-4">
                <div class="bg-gray-800 border border-gray-700 rounded-lg max-w-2xl w-full max-h-[90vh] overflow-y-auto" style="-webkit-overflow-scrolling: touch;">
                    <div class="flex items-center justify-between p-4 border-b border-gray-700">
                        <h3 class="text-lg font-medium text-white">Audit Entry Details</h3>
                        <button @click="selectedEntry = null" class="text-gray-400 hover:text-gray-200 text-2xl touch-target">&times;</button>
                    </div>
                    <div class="p-4 space-y-3 text-sm text-gray-300">
                        <div><strong class="text-white">ID:</strong> {{ selectedEntry.id }}</div>
                        <div><strong class="text-white">Timestamp:</strong> {{ formatTimestamp(selectedEntry.timestamp) }}</div>
                        <div><strong class="text-white">Event:</strong> {{ selectedEntry.event }}</div>
                        <div v-if="selectedEntry.user_id"><strong class="text-white">User ID:</strong> {{ selectedEntry.user_id }}</div>
                        <div v-if="selectedEntry.client_id"><strong class="text-white">Client ID:</strong> 
                            <code class="bg-gray-700 text-gray-200 px-2 py-1 rounded text-xs break-all ml-2">{{ selectedEntry.client_id }}</code>
                        </div>
                        <div v-if="selectedEntry.ip_address"><strong class="text-white">IP Address:</strong> {{ selectedEntry.ip_address }}</div>
                        <div v-if="selectedEntry.user_agent"><strong class="text-white">User Agent:</strong> 
                            <span class="break-words">{{ selectedEntry.user_agent }}</span>
                        </div>
                        <div><strong class="text-white">Success:</strong> 
                            <span :class="selectedEntry.success ? 'text-green-400' : 'text-red-400'">
                                {{ selectedEntry.success ? 'Yes' : 'No' }}
                            </span>
                        </div>
                        <div v-if="selectedEntry.error"><strong class="text-white">Error:</strong> 
                            <span class="text-red-400 break-words">{{ selectedEntry.error }}</span>
                        </div>
                        <div v-if="selectedEntry.details && Object.keys(selectedEntry.details).length > 0">
                            <strong class="text-white">Details:</strong>
                            <pre class="mt-2 p-3 bg-gray-700 text-gray-200 rounded text-xs overflow-auto break-words whitespace-pre-wrap">{{ JSON.stringify(selectedEntry.details, null, 2) }}</pre>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    `,
    data() {
        return {
            loading: false,
            entries: [],
            stats: null,
            selectedEntry: null,
            filters: {
                event: '',
                success: ''
            },
            currentPage: 1,
            pageSize: 20,
            totalEntries: 0,
            totalPages: 0
        }
    },
    async mounted() {
        await this.loadData();
    },
    methods: {
        async loadData() {
            await Promise.all([
                this.loadEntries(),
                this.loadStats()
            ]);
        },
        async loadEntries() {
            this.loading = true;
            try {
                const params = new URLSearchParams({
                    page: this.currentPage,
                    limit: this.pageSize,
                    ...(this.filters.event && { event: this.filters.event }),
                    ...(this.filters.success !== '' && { success: this.filters.success })
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
                }
            } catch (error) {
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
        formatTimestamp(timestamp) {
            try {
                return new Date(timestamp).toLocaleString();
            } catch (e) {
                return timestamp;
            }
        },
        formatEventName(event) {
            return event.replace(/\./g, ' ').replace(/\b\w/g, l => l.toUpperCase());
        },
        viewDetails(entry) {
            this.selectedEntry = entry;
        },
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
        }
    }
};