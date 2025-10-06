const ChatComponent = {
    template: `
        <div class="chat-container">
            <div class="sidebar-backdrop" :class="{ 'show': sidebarOpen }" @click="closeSidebar"></div>

            <div class="chat-sidebar" :class="{ 'sidebar-open': sidebarOpen, 'sidebar-collapsed': sidebarCollapsed }">
                <div class="sidebar-header">
                    <h3 v-if="!sidebarCollapsed">Conversations</h3>
                    <button @click="toggleSidebarCollapse" class="sidebar-collapse-btn" :title="sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'">
                        <span v-if="sidebarCollapsed">▶</span>
                        <span v-else>◀</span>
                    </button>
                    <button @click="closeSidebar" class="sidebar-close">×</button>
                </div>
                <div class="sidebar-controls" v-show="!sidebarCollapsed">
                    <div class="provider-selector-sidebar">
                        <label class="sidebar-label">Provider</label>
                        <select v-model="selectedProvider" @change="onProviderChange" class="provider-select-sidebar">
                            <option v-for="provider in availableProviders" :key="provider" :value="provider">
                                {{ provider }}
                            </option>
                        </select>
                    </div>
                    <div class="model-selector-sidebar">
                        <label class="sidebar-label">Model</label>
                        <select v-model="selectedModel" @change="onModelChange" class="model-select-sidebar">
                            <option v-for="model in availableModels" :key="model" :value="model">
                                {{ model }}
                            </option>
                        </select>
                    </div>
                    <button @click="createNewSession" class="new-chat-btn-sidebar">
                        + New Chat
                    </button>
                </div>
                <div class="sessions-list" v-show="!sidebarCollapsed">
                    <div v-for="session in sessions"
                         :key="session.id"
                         :class="['session-item', { active: currentSessionId === session.id }]"
                         @click="loadSession(session.id)">
                        <div class="session-header">
                            <span class="session-title">{{ session.title }}</span>
                            <span v-if="session.unread_message_count > 0"
                                  class="unread-badge"
                                  :title="`${session.unread_message_count} unread messages`">
                                {{ session.unread_message_count }}
                            </span>
                            <svg v-if="session.has_active_agents"
                                 class="agent-icon"
                                 title="Active agents running"
                                 width="16" height="16"
                                 viewBox="0 0 24 24">
                                <path fill="currentColor" d="M12 2a2 2 0 012 2v1a1 1 0 001 1h2a2 2 0 012 2v10a2 2 0 01-2 2H7a2 2 0 01-2-2V8a2 2 0 012-2h2a1 1 0 001-1V4a2 2 0 012-2zm0 11a2 2 0 100 4 2 2 0 000-4z"/>
                            </svg>
                        </div>
                        <div class="session-meta">
                            {{ session.provider }} • {{ formatDate(session.last_used) }}
                        </div>
                        <div class="session-actions">
                            <button @click.stop="renameSession(session.id)" class="session-action-btn" title="Rename">
                                Edit
                            </button>
                            <button @click.stop="deleteSession(session.id)" class="session-action-btn" title="Delete">
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            </div>


            <div class="chat-main">
                <div class="chat-header">
                    <button @click="toggleSidebar" class="hamburger-menu" title="Menu">
                        ☰
                    </button>
                    <div class="chat-header-left">
                        <div class="chat-title">{{ currentSessionTitle }}</div>
                        <div class="chat-config-controls">
                            <button @click="toggleSystemPrompt" class="system-prompt-btn" :title="showSystemPrompt ? 'Hide System Prompt' : 'View System Prompt'">
                                <svg v-if="!showSystemPrompt" xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
                                    <circle cx="12" cy="12" r="3"/>
                                </svg>
                                <svg v-else xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                                    <line x1="18" y1="6" x2="6" y2="18"/>
                                    <line x1="6" y1="6" x2="18" y2="18"/>
                                </svg>
                            </button>
                        </div>
                    </div>
                    <div class="connection-status" :class="connectionStatusClass" :title="connectionStatus">
                        <svg v-if="connectionStatus === 'connected'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <polyline points="20 6 9 17 4 12"/>
                        </svg>
                        <svg v-else-if="connectionStatus === 'connecting'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <line x1="12" y1="2" x2="12" y2="6"/>
                            <line x1="12" y1="18" x2="12" y2="22"/>
                            <line x1="4.93" y1="4.93" x2="7.76" y2="7.76"/>
                            <line x1="16.24" y1="16.24" x2="19.07" y2="19.07"/>
                            <line x1="2" y1="12" x2="6" y2="12"/>
                            <line x1="18" y1="12" x2="22" y2="12"/>
                            <line x1="4.93" y1="19.07" x2="7.76" y2="16.24"/>
                            <line x1="16.24" y1="7.76" x2="19.07" y2="4.93"/>
                        </svg>
                        <svg v-else-if="connectionStatus === 'error'" xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <circle cx="12" cy="12" r="10"/>
                            <line x1="15" y1="9" x2="9" y2="15"/>
                            <line x1="9" y1="9" x2="15" y2="15"/>
                        </svg>
                        <svg v-else xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                            <circle cx="12" cy="12" r="10"/>
                        </svg>
                    </div>
                </div>

                <div v-if="showSystemPrompt" class="system-prompt-viewer">
                    <div class="system-prompt-header">
                        <h4>System Prompt</h4>
                        <button @click="toggleSystemPrompt" class="close-btn">×</button>
                    </div>
                    <div v-if="loadingSystemPrompt" class="system-prompt-loading">
                        <div class="spinner"></div>
                        <span>Loading system prompt...</span>
                    </div>
                    <div v-else class="system-prompt-content">
                        <pre>{{ systemPrompt }}</pre>
                    </div>
                </div>

                <div class="messages-container" ref="messagesContainer">
                    <div v-if="messages.length === 0" class="empty-state">
                        <h2>Start a Conversation</h2>
                        <p>Ask me anything about your MCP servers, or try these:</p>
                        <div class="suggestion-chips">
                            <button @click="sendSuggestion('List all running servers')" class="suggestion-chip">
                                List all servers
                            </button>
                            <button @click="sendSuggestion('Search my memory for authentication')" class="suggestion-chip">
                                Search memory
                            </button>
                            <button @click="sendSuggestion('Show task scheduler status')" class="suggestion-chip">
                                Scheduler status
                            </button>
                        </div>
                    </div>

                    <div v-for="message in messages" :key="message.id" :class="['message', message.role, { 'automated': message.is_automated }]">
                        <div class="message-avatar">
                            <span v-if="message.role === 'user'">U</span>
                            <svg v-else-if="message.is_automated"
                                 class="robot-icon"
                                 width="24" height="24"
                                 viewBox="0 0 24 24">
                                <path fill="currentColor" d="M12 2a2 2 0 012 2v1h2a2 2 0 012 2v10a2 2 0 01-2 2H8a2 2 0 01-2-2V7a2 2 0 012-2h2V4a2 2 0 012-2zm0 11a2 2 0 100 4 2 2 0 000-4z"/>
                            </svg>
                            <span v-else>AI</span>
                        </div>
                        <div class="message-content">
                            <div class="message-header">
                                <span class="message-role">
                                    <span v-if="message.is_automated" class="automation-badge">
                                        🤖 Scheduled Agent
                                    </span>
                                    <span v-else>
                                        {{ message.role === 'user' ? 'You' : 'Assistant' }}
                                    </span>
                                </span>
                                <span class="message-time">{{ formatTime(message.created_at) }}</span>
                            </div>
                            <div class="message-text" v-html="renderMarkdown(message.content)"></div>

                            <div v-if="message.tool_calls && message.tool_calls.length > 0" class="tool-execution-section">
                                <button @click="toggleToolSection(message.id)" class="tool-accordion-header">
                                    <span class="tool-status-indicator">
                                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                            <path d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"/>
                                        </svg>
                                    </span>
                                    <span class="tool-accordion-icon">{{ expandedToolSections[message.id] ? '▼' : '▶' }}</span>
                                    <span class="tool-accordion-title">
                                        <strong>{{ message.tool_calls.length }}</strong> tool call{{ message.tool_calls.length !== 1 ? 's' : '' }} executed
                                    </span>
                                    <span class="tool-count-badge">{{ message.tool_calls.length }}</span>
                                </button>
                                <div v-if="expandedToolSections[message.id]" class="tool-accordion-content">
                                    <div v-for="(call, index) in message.tool_calls" :key="index" class="tool-call-item">
                                        <div class="tool-call-header">
                                            <span class="tool-status-icon" :class="getToolStatusClass(message, index)">
                                                {{ getToolStatusIcon(message, index) }}
                                            </span>
                                            <span class="tool-name">{{ call.name || call.Name }}</span>
                                            <span class="tool-index">#{{ index + 1 }}</span>
                                        </div>
                                        <div v-if="call.args || call.Args" class="tool-call-args">
                                            <strong>Arguments:</strong>
                                            <pre>{{ JSON.stringify(call.args || call.Args, null, 2) }}</pre>
                                        </div>
                                        <div v-if="message.tool_results && message.tool_results[index]" class="tool-result-section">
                                            <button @click="toggleToolResult(message.id, index)" class="tool-result-toggle">
                                                <span class="tool-result-icon">{{ expandedToolResults[message.id + '_' + index] ? '▼' : '▶' }}</span>
                                                <strong>Result</strong>
                                                <span class="tool-result-meta-inline">
                                                    {{ formatDuration(message.tool_results[index].duration || message.tool_results[index].Duration) }}
                                                </span>
                                            </button>
                                            <div v-if="expandedToolResults[message.id + '_' + index]">
                                                <div v-if="message.tool_results[index].error || message.tool_results[index].Error" class="tool-result-error">
                                                    Error: {{ message.tool_results[index].error || message.tool_results[index].Error }}
                                                </div>
                                                <div v-else class="tool-result-success">
                                                    <pre>{{ formatToolResult(message.tool_results[index].result || message.tool_results[index].Result) }}</pre>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            <div v-if="message.tokens_used" class="message-meta">
                                <span>{{ message.tokens_used }} tokens</span>
                                <span v-if="message.cost_estimate"> • \${{ formatCost(message.cost_estimate) }}</span>
                            </div>
                        </div>
                    </div>

                    <div v-for="toolActivity in liveToolActivity" :key="toolActivity.id" class="tool-activity-item">
                        <span class="tool-activity-icon" :class="toolActivity.status">{{ getToolActivityIcon(toolActivity.status) }}</span>
                        <span class="tool-activity-name">{{ toolActivity.name }}</span>
                        <span v-if="toolActivity.duration" class="tool-activity-duration">{{ toolActivity.duration }}</span>
                    </div>

                    <div v-if="isStreaming" class="message assistant streaming">
                        <div class="message-avatar">
                            <span>AI</span>
                        </div>
                        <div class="message-content">
                            <div class="message-header">
                                <span class="message-role">Assistant</span>
                                <span class="typing-indicator">●●●</span>
                            </div>
                            <div class="message-text" v-html="renderMarkdown(streamingContent)"></div>
                        </div>
                    </div>

                    <div v-if="showAgentIndicator && activeTasks.length > 0"
                         class="active-tasks-panel">
                        <h4>Active Agents</h4>
                        <div v-for="task in activeTasks" :key="task.id" class="task-summary">
                            <div class="task-name">{{ task.name }}</div>
                            <div class="task-schedule">{{ formatSchedule(task.schedule) }}</div>
                            <div class="task-status">
                                Next run: {{ formatTime(task.next_run) }}
                            </div>
                        </div>
                    </div>
                </div>

                <div class="input-container">
                    <div v-if="error" class="error-message">
                        {{ error }}
                        <button @click="clearError" class="error-close">×</button>
                    </div>
                    <div class="mcp-control-bar">
                        <div class="mcp-dropdown" @click.stop="toggleMCPDropdown">
                            <button class="mcp-dropdown-btn">
                                <span class="mcp-icon">MCP</span>
                                <span>({{ enabledMCPServers.length }})</span>
                                <span class="dropdown-arrow">{{ showMCPDropdown ? '▲' : '▼' }}</span>
                            </button>
                            <div v-if="showMCPDropdown" class="mcp-dropdown-panel" @click.stop>
                                <div class="mcp-dropdown-header">
                                    <div class="mcp-dropdown-title">
                                        <h4>Configure MCP Servers</h4>
                                        <p>Select servers for this chat session</p>
                                    </div>
                                    <button @click="toggleMCPDropdown" class="mcp-dropdown-close" title="Close">×</button>
                                </div>
                                <div v-if="!loadingMCPServers && availableMCPServers.length > 0" class="mcp-bulk-actions">
                                    <button @click="selectAllMCPServers" class="mcp-bulk-btn">Select All</button>
                                    <button @click="deselectAllMCPServers" class="mcp-bulk-btn">Deselect All</button>
                                </div>
                                <div v-if="loadingMCPServers" class="mcp-loading">
                                    <div class="spinner"></div>
                                    <span>Loading...</span>
                                </div>
                                <div v-else-if="availableMCPServers.length === 0" class="mcp-no-servers">
                                    <p>No MCP servers available</p>
                                </div>
                                <div v-else class="mcp-server-list-dropdown">
                                    <label v-for="server in availableMCPServers" :key="server.name" class="mcp-server-checkbox-item">
                                        <input
                                            type="checkbox"
                                            :value="server.name"
                                            :checked="enabledMCPServers.includes(server.name)"
                                            @change="toggleMCPServer(server.name)"
                                        />
                                        <div class="mcp-server-info-dropdown">
                                            <span class="mcp-server-name-dropdown">{{ server.name }}</span>
                                            <span class="mcp-server-tools-count" v-if="server.tool_count !== undefined">
                                                {{ server.tool_count }} tool{{ server.tool_count !== 1 ? 's' : '' }}
                                            </span>
                                        </div>
                                    </label>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="input-wrapper">
                        <textarea
                            v-model="inputMessage"
                            @keydown.enter.exact.prevent="sendMessage"
                            @keydown.enter.shift.exact="addNewLine"
                            @input="autoResizeTextarea"
                            placeholder="Type your message..."
                            class="message-input"
                            :disabled="isStreaming"
                            ref="messageInput"
                            rows="1"
                        ></textarea>
                        <button @click="sendMessage" :disabled="!inputMessage.trim() || isStreaming" class="send-btn">
                            <span v-if="!isStreaming">Send</span>
                            <span v-else>...</span>
                        </button>
                    </div>
                </div>
            </div>
        </div>
    `,

    data() {
        return {
            sessions: [],
            currentSessionId: null,
            currentSessionTitle: 'New Chat',
            messages: [],
            inputMessage: '',
            isStreaming: false,
            streamingContent: '',
            streamingToolCalls: [],
            streamingToolResults: [],
            ws: null,
            connectionStatus: 'disconnected',
            error: null,
            sidebarOpen: false,
            sidebarCollapsed: false,

            availableProviders: [],
            availableModels: [],
            selectedProvider: 'openrouter',
            selectedModel: 'z-ai/glm-4.6',
            providerModels: {},

            showMCPDropdown: false,
            availableMCPServers: [],
            enabledMCPServers: [],
            loadingMCPServers: false,

            showSystemPrompt: false,
            systemPrompt: '',
            loadingSystemPrompt: false,

            expandedToolSections: {},
            expandedToolResults: {},
            liveToolActivity: [],

            showAgentIndicator: false,
            activeTasks: []
        };
    },

    computed: {
        connectionStatusClass() {
            return {
                'status-connected': this.connectionStatus === 'connected',
                'status-connecting': this.connectionStatus === 'connecting',
                'status-disconnected': this.connectionStatus === 'disconnected',
                'status-error': this.connectionStatus === 'error'
            };
        }
    },

    mounted() {
        this.loadSessions();
        this.loadProviders();
        this.loadAvailableMCPServers();

        window.addEventListener('beforeunload', () => {
            this.closeWebSocket();
        });

        document.addEventListener('click', (e) => {
            if (!e.target.closest('.mcp-dropdown')) {
                this.showMCPDropdown = false;
            }
        });

        this.$watch('showMCPDropdown', (newVal) => {
            if (newVal) {
                document.body.classList.add('modal-open');
            } else {
                document.body.classList.remove('modal-open');
            }
        });
    },

    beforeUnmount() {
        this.closeWebSocket();
    },

    methods: {
        async loadSessions() {
            try {
                const response = await fetch('/api/chat/sessions');
                if (response.ok) {
                    this.sessions = await response.json();

                    if (this.sessions.length > 0 && !this.currentSessionId) {
                        await this.loadSession(this.sessions[0].id);
                    }
                }
            } catch (err) {
                console.error('Failed to load sessions:', err);
            }
        },

        async loadProviders() {
            try {
                const response = await fetch('/api/chat/providers');
                if (response.ok) {
                    const data = await response.json();
                    this.providerModels = data;
                    this.availableProviders = Object.keys(data);

                    if (this.selectedProvider && data[this.selectedProvider]) {
                        this.availableModels = data[this.selectedProvider];
                    }
                }
            } catch (err) {
                console.error('Failed to load providers:', err);
            }
        },

        async createNewSession() {
            try {
                const response = await fetch('/api/chat/sessions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: this.selectedProvider,
                        model: this.selectedModel
                    })
                });

                if (response.ok) {
                    const session = await response.json();
                    this.sessions.unshift(session);
                    await this.loadSession(session.id);
                }
            } catch (err) {
                this.showError('Failed to create session: ' + err.message);
            }
        },

        async loadActiveTasks() {
            if (!this.currentSessionId) return;

            try {
                const response = await fetch(`/api/chat/sessions/${this.currentSessionId}/tasks`);
                const data = await response.json();
                this.activeTasks = data.tasks || [];
                this.showAgentIndicator = this.activeTasks.length > 0;
            } catch (err) {
                console.error('Failed to load active tasks:', err);
            }
        },

        async loadSession(sessionId) {
            try {
                const response = await fetch(`/api/chat/sessions/${sessionId}`);
                if (response.ok) {
                    const data = await response.json();
                    this.currentSessionId = sessionId;
                    this.currentSessionTitle = data.title;
                    this.messages = data.messages || [];
                    this.selectedProvider = data.provider;
                    this.selectedModel = data.model;
                    this.enabledMCPServers = data.mcp_servers || [];

                    this.closeWebSocket();
                    this.connectWebSocket();
                    this.closeSidebar();

                    await this.loadActiveTasks();

                    this.$nextTick(() => {
                        this.scrollToBottom();
                    });
                }
            } catch (err) {
                this.showError('Failed to load session: ' + err.message);
            }
        },

        async deleteSession(sessionId) {
            if (!confirm('Delete this conversation?')) return;

            try {
                const response = await fetch(`/api/chat/sessions/${sessionId}`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    this.sessions = this.sessions.filter(s => s.id !== sessionId);

                    if (this.currentSessionId === sessionId) {
                        if (this.sessions.length > 0) {
                            await this.loadSession(this.sessions[0].id);
                        } else {
                            this.currentSessionId = null;
                            this.messages = [];
                            this.closeWebSocket();
                        }
                    }
                }
            } catch (err) {
                this.showError('Failed to delete session: ' + err.message);
            }
        },

        async renameSession(sessionId) {
            const newTitle = prompt('Enter new title:');
            if (!newTitle) return;

            try {
                const response = await fetch(`/api/chat/sessions/${sessionId}`, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ title: newTitle })
                });

                if (response.ok) {
                    const session = this.sessions.find(s => s.id === sessionId);
                    if (session) {
                        session.title = newTitle;
                    }
                    if (this.currentSessionId === sessionId) {
                        this.currentSessionTitle = newTitle;
                    }
                }
            } catch (err) {
                this.showError('Failed to rename session: ' + err.message);
            }
        },

        connectWebSocket() {
            if (!this.currentSessionId) return;

            this.connectionStatus = 'connecting';

            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${window.location.host}/ws/chat/${this.currentSessionId}`;

            this.ws = new WebSocket(wsUrl);

            this.ws.onopen = () => {
                this.connectionStatus = 'connected';
                console.log('WebSocket connected');
            };

            this.ws.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    this.handleWebSocketMessage(data);
                } catch (err) {
                    console.error('Failed to parse WebSocket message:', err);
                }
            };

            this.ws.onerror = (error) => {
                console.error('WebSocket error:', error);
                this.connectionStatus = 'error';
                this.showError('Connection error occurred');
            };

            this.ws.onclose = () => {
                this.connectionStatus = 'disconnected';
                console.log('WebSocket disconnected');
            };
        },

        closeWebSocket() {
            if (this.ws) {
                this.ws.close();
                this.ws = null;
            }
        },

        handleWebSocketMessage(data) {
            if (data.type === 'chunk') {
                if (!this.isStreaming && !data.done) {
                    this.isStreaming = true;
                    this.streamingContent = '';
                    this.streamingToolCalls = [];
                    this.streamingToolResults = [];
                }

                if (!data.done) {
                    this.streamingContent += data.content || '';

                    if (data.tool_calls && data.tool_calls.length > 0) {
                        this.streamingToolCalls = data.tool_calls;
                        this.addLiveToolCalls(data.tool_calls);
                    }

                    if (data.tool_results && data.tool_results.length > 0) {
                        this.streamingToolResults = data.tool_results;
                        this.updateLiveToolResults(data.tool_results);
                    }

                    this.$nextTick(() => {
                        this.scrollToBottom();
                    });
                } else {
                    if (data.tool_calls && data.tool_calls.length > 0) {
                        this.finalizeLiveToolActivity(data.tool_calls, data.tool_results || []);
                    }

                    if (this.isStreaming) {
                        this.isStreaming = false;

                        const message = {
                            id: data.message_id || this.generateId(),
                            role: 'assistant',
                            content: this.streamingContent,
                            tool_calls: data.tool_calls || this.streamingToolCalls || [],
                            tool_results: data.tool_results || this.streamingToolResults || [],
                            tokens_used: data.tokens_used,
                            cost_estimate: data.cost_estimate,
                            created_at: new Date().toISOString()
                        };

                        this.messages.push(message);
                        this.streamingContent = '';
                        this.streamingToolCalls = [];
                        this.streamingToolResults = [];
                    }

                    this.$nextTick(() => {
                        this.scrollToBottom();
                    });
                }
            } else if (data.type === 'error') {
                this.isStreaming = false;
                let errorMsg = data.error || data.message || 'An error occurred';

                if (errorMsg.includes('401') || errorMsg.includes('User not found')) {
                    errorMsg = 'Authentication error: Please check your API key configuration (OPENROUTER_API_KEY)';
                } else if (errorMsg.includes('openrouter')) {
                    errorMsg = 'OpenRouter API error: ' + errorMsg;
                }

                this.showError(errorMsg);
            }
        },

        async sendMessage() {
            const message = this.inputMessage.trim();
            if (!message || this.isStreaming) return;

            if (!this.currentSessionId) {
                await this.createNewSession();
            }

            if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
                this.showError('Not connected. Reconnecting...');
                this.connectWebSocket();
                return;
            }

            const userMessage = {
                id: this.generateId(),
                role: 'user',
                content: message,
                created_at: new Date().toISOString()
            };

            this.messages.push(userMessage);
            this.inputMessage = '';

            this.$nextTick(() => {
                this.scrollToBottom();
                this.$refs.messageInput.style.height = 'auto';
            });

            try {
                this.ws.send(JSON.stringify({
                    type: 'message',
                    message: message
                }));
            } catch (err) {
                this.showError('Failed to send message: ' + err.message);
            }
        },

        sendSuggestion(text) {
            this.inputMessage = text;
            this.sendMessage();
        },

        addNewLine() {
            this.inputMessage += '\n';
        },

        async onProviderChange() {
            if (this.providerModels[this.selectedProvider]) {
                this.availableModels = this.providerModels[this.selectedProvider];
                this.selectedModel = this.availableModels[0];
            }

            if (this.currentSessionId) {
                await this.updateSessionConfig();
            }
        },

        async onModelChange() {
            if (this.currentSessionId) {
                await this.updateSessionConfig();
            }
        },

        async updateSessionConfig() {
            try {
                await fetch(`/api/chat/sessions/${this.currentSessionId}`, {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: this.selectedProvider,
                        model: this.selectedModel
                    })
                });
            } catch (err) {
                console.error('Failed to update session config:', err);
            }
        },

        toggleSidebar() {
            this.sidebarOpen = !this.sidebarOpen;
            if (this.sidebarOpen) {
                document.body.classList.add('sidebar-open');
            } else {
                document.body.classList.remove('sidebar-open');
            }
        },

        closeSidebar() {
            this.sidebarOpen = false;
            document.body.classList.remove('sidebar-open');
        },

        toggleSidebarCollapse() {
            this.sidebarCollapsed = !this.sidebarCollapsed;
        },

        renderMarkdown(content) {
            if (!content) return '';

            let html = content
                .replace(/&/g, '&amp;')
                .replace(/</g, '&lt;')
                .replace(/>/g, '&gt;')
                .replace(/```(\w+)?\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>')
                .replace(/`([^`]+)`/g, '<code>$1</code>')
                .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
                .replace(/\*(.+?)\*/g, '<em>$1</em>')
                .replace(/\n\n/g, '</p><p>')
                .replace(/\n/g, '<br>');

            return `<p>${html}</p>`;
        },

        formatToolResult(result) {
            if (typeof result === 'string') {
                try {
                    const parsed = JSON.parse(result);
                    if (parsed && parsed.content && Array.isArray(parsed.content)) {
                        return parsed.content.map(c => c.text || JSON.stringify(c)).join('\n');
                    }
                    return result;
                } catch (e) {
                    return result;
                }
            }
            if (result && result.content && Array.isArray(result.content)) {
                return result.content.map(c => c.text || JSON.stringify(c)).join('\n');
            }
            return JSON.stringify(result, null, 2);
        },

        formatDuration(duration) {
            if (!duration) return '0ms';
            if (typeof duration === 'string') return duration;
            if (duration < 1000) return Math.round(duration) + 'ms';
            return (duration / 1000).toFixed(2) + 's';
        },

        toggleToolSection(messageId) {
            this.expandedToolSections[messageId] = !this.expandedToolSections[messageId];
        },

        formatDate(dateString) {
            const date = new Date(dateString);
            const now = new Date();
            const diffMs = now - date;
            const diffMins = Math.floor(diffMs / 60000);
            const diffHours = Math.floor(diffMs / 3600000);
            const diffDays = Math.floor(diffMs / 86400000);

            if (diffMins < 1) return 'Just now';
            if (diffMins < 60) return `${diffMins}m ago`;
            if (diffHours < 24) return `${diffHours}h ago`;
            if (diffDays < 7) return `${diffDays}d ago`;

            return date.toLocaleDateString();
        },

        formatTime(dateString) {
            const date = new Date(dateString);
            return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        },

        formatSchedule(schedule) {
            if (!schedule) return 'Not scheduled';
            if (schedule.startsWith('@')) return schedule;
            return `Cron: ${schedule}`;
        },

        scrollToBottom() {
            const container = this.$refs.messagesContainer;
            if (container) {
                container.scrollTop = container.scrollHeight;
            }
        },

        showError(message) {
            this.error = message;
            setTimeout(() => {
                this.error = null;
            }, 5000);
        },

        clearError() {
            this.error = null;
        },

        generateId() {
            return 'msg_' + Date.now() + '_' + Math.random().toString(36).substr(2, 9);
        },

        formatCost(cost) {
            return cost.toFixed(4);
        },

        getToolStatusClass(message, index) {
            if (!message.tool_results || !message.tool_results[index]) {
                return 'pending';
            }
            const result = message.tool_results[index];
            if (result.error || result.Error) {
                return 'error';
            }
            return 'success';
        },

        getToolStatusIcon(message, index) {
            if (!message.tool_results || !message.tool_results[index]) {
                return '-';
            }
            const result = message.tool_results[index];
            if (result.error || result.Error) {
                return 'x';
            }
            return '+';
        },

        addLiveToolCalls(toolCalls) {
            toolCalls.forEach(call => {
                const existingIndex = this.liveToolActivity.findIndex(t => t.name === (call.name || call.Name));
                if (existingIndex === -1) {
                    this.liveToolActivity.push({
                        id: this.generateId(),
                        name: call.name || call.Name,
                        args: call.args || call.Args || {},
                        status: 'running',
                        statusText: 'Running...',
                        result: null,
                        duration: null
                    });
                }
            });
            this.$nextTick(() => this.scrollToBottom());
        },

        updateLiveToolResults(toolResults) {
            toolResults.forEach((result, index) => {
                if (index < this.liveToolActivity.length) {
                    const activity = this.liveToolActivity[index];
                    const hasError = result.error || result.Error;
                    activity.status = hasError ? 'error' : 'success';
                    activity.statusText = hasError ? 'Failed' : 'Complete';
                    activity.result = hasError ? (result.error || result.Error) : (result.result || result.Result);
                    activity.duration = this.formatDuration(result.duration || result.Duration);
                }
            });
            this.$nextTick(() => this.scrollToBottom());
        },

        finalizeLiveToolActivity(toolCalls, toolResults) {
            setTimeout(() => {
                this.liveToolActivity = [];
            }, 500);
        },

        getToolActivityIcon(status) {
            switch (status) {
                case 'running':
                    return '>';
                case 'success':
                    return '+';
                case 'error':
                    return 'x';
                default:
                    return '-';
            }
        },

        formatArgValue(value) {
            if (typeof value === 'string' && value.length > 100) {
                return value.substring(0, 100) + '...';
            }
            if (typeof value === 'object') {
                return JSON.stringify(value).substring(0, 100) + '...';
            }
            return value;
        },

        formatToolActivityResult(result) {
            if (typeof result === 'string') {
                if (result.length > 200) {
                    return result.substring(0, 200) + '...';
                }
                return result;
            }
            if (result && typeof result === 'object') {
                const str = JSON.stringify(result);
                if (str.length > 200) {
                    return str.substring(0, 200) + '...';
                }
                return str;
            }
            return String(result);
        },

        toggleToolResult(messageId, index) {
            const key = messageId + '_' + index;
            this.$set(this.expandedToolResults, key, !this.expandedToolResults[key]);
        },

        autoResizeTextarea() {
            const textarea = this.$refs.messageInput;
            if (!textarea) return;

            textarea.style.height = 'auto';
            const maxHeight = window.innerWidth < 768 ? 140 : 200;
            const newHeight = Math.min(textarea.scrollHeight, maxHeight);
            textarea.style.height = newHeight + 'px';
        },

        async loadAvailableMCPServers() {
            this.loadingMCPServers = true;

            try {
                const response = await fetch('/api/chat/mcp-servers');
                const data = await response.json();

                this.availableMCPServers = Array.isArray(data) ? data : [];

            } catch (err) {
                console.error('Failed to load MCP servers:', err);
                this.availableMCPServers = [];
            }

            this.loadingMCPServers = false;
        },

        toggleMCPDropdown() {
            this.showMCPDropdown = !this.showMCPDropdown;

            if (this.showMCPDropdown && this.availableMCPServers.length === 0) {
                this.loadAvailableMCPServers();
            }
        },

        async toggleMCPServer(serverName) {
            if (!this.currentSessionId) {
                this.showError('No session selected');
                return;
            }

            const newServers = this.enabledMCPServers.includes(serverName)
                ? this.enabledMCPServers.filter(s => s !== serverName)
                : [...this.enabledMCPServers, serverName];

            await this.updateMCPServers(newServers);
        },

        async selectAllMCPServers() {
            if (!this.currentSessionId) {
                this.showError('No session selected');
                return;
            }

            const allServerNames = this.availableMCPServers.map(s => s.name);
            await this.updateMCPServers(allServerNames);
        },

        async deselectAllMCPServers() {
            if (!this.currentSessionId) {
                this.showError('No session selected');
                return;
            }

            await this.updateMCPServers([]);
        },

        async updateMCPServers(newServers) {
            try {
                const response = await fetch(`/api/chat/sessions/${this.currentSessionId}/mcp-servers`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        mcp_servers: newServers
                    })
                });

                if (response.ok) {
                    this.enabledMCPServers = newServers;

                    const session = this.sessions.find(s => s.id === this.currentSessionId);
                    if (session) {
                        session.mcp_servers = this.enabledMCPServers;
                    }

                    this.systemPrompt = '';
                } else {
                    this.showError('Failed to update MCP server configuration');
                }
            } catch (err) {
                this.showError('Failed to update MCP servers: ' + err.message);
            }
        },

        async toggleSystemPrompt() {
            this.showSystemPrompt = !this.showSystemPrompt;

            if (this.showSystemPrompt && !this.systemPrompt && this.currentSessionId) {
                this.loadingSystemPrompt = true;

                try {
                    const response = await fetch(`/api/chat/sessions/${this.currentSessionId}/system-prompt`);
                    if (response.ok) {
                        const data = await response.json();
                        this.systemPrompt = data.system_prompt || 'No system prompt available';
                    } else {
                        this.systemPrompt = 'Failed to load system prompt';
                    }
                } catch (err) {
                    console.error('Failed to load system prompt:', err);
                    this.systemPrompt = 'Error loading system prompt';
                }

                this.loadingSystemPrompt = false;
            }
        }
    }
};