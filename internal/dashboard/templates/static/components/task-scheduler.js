// /static/components/task-scheduler.js
const TaskScheduler = {
    props: ['config'],
    data() {
        return {
            tasks: [],
            taskRuns: [],
            metrics: {},
            loading: false,
            error: null,
            searchTerm: '',
            filterType: 'all',
            filterStatus: 'all',
            sortBy: 'name',
            autoRefresh: false,
            refreshInterval: null,
            // Accordion state
            expandedTasks: new Set(),
            expandedGroups: new Set(),
            showTaskDetails: {},
            showRunOutput: {},
            // Task creation
            showCreateTask: false,
            newTask: {
                type: 'shell',
                name: '',
                description: '',
                command: '',
                prompt: '',
                schedule: '0 0 * * *',
                enabled: true,
                model: '',
                modelHint: 'balanced',
                maxCost: '1.0',
                requireLocal: false,
                dependsOn: []
            },
            // Task types with detailed configuration
            taskTypes: [
                {
                    value: 'shell',
                    label: 'Shell Command',
                    icon: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z', // terminal icon
                    color: 'green',
                    description: 'Execute shell commands on schedule'
                },
                {
                    value: 'ai',
                    label: 'AI Task',
                    icon: 'M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z', // light-bulb icon
                    color: 'purple',
                    description: 'AI-powered tasks using LLMs'
                },
                {
                    value: 'manual',
                    label: 'Manual Task',
                    icon: 'M5.636 18.364a9 9 0 010-12.728m12.728 0a9 9 0 010 12.728m-9.9-2.829a5 5 0 010-7.07m7.072 0a5 5 0 010 7.07M13 12a1 1 0 11-2 0 1 1 0 012 0z', // cursor-click icon
                    color: 'blue',
                    description: 'Manually triggered tasks'
                },
                {
                    value: 'dependency',
                    label: 'Dependency Task',
                    icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1', // link icon
                    color: 'yellow',
                    description: 'Tasks that depend on other tasks'
                },
                {
                    value: 'watcher',
                    label: 'Watcher Task',
                    icon: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z', // eye icon
                    color: 'indigo',
                    description: 'File and event watchers'
                }
            ],
            modelHints: ['fast', 'cheap', 'powerful', 'local', 'balanced'],
            cronPresets: [
                { label: 'Every minute', value: '* * * * *' },
                { label: 'Every 5 minutes', value: '*/5 * * * *' },
                { label: 'Every 15 minutes', value: '*/15 * * * *' },
                { label: 'Every hour', value: '0 * * * *' },
                { label: 'Every 6 hours', value: '0 */6 * * *' },
                { label: 'Daily at midnight', value: '0 0 * * *' },
                { label: 'Daily at 9 AM', value: '0 9 * * *' },
                { label: 'Weekly on Monday', value: '0 0 * * 1' },
                { label: 'Monthly on 1st', value: '0 0 1 * *' }
            ]
        }
    },
    computed: {
        filteredTasks() {
            let filtered = this.tasks.filter(task => {
                const matchesSearch = !this.searchTerm ||
                    task.name.toLowerCase().includes(this.searchTerm.toLowerCase()) ||
                    (task.description && task.description.toLowerCase().includes(this.searchTerm.toLowerCase())) ||
                    (task.command && task.command.toLowerCase().includes(this.searchTerm.toLowerCase())) ||
                    (task.prompt && task.prompt.toLowerCase().includes(this.searchTerm.toLowerCase()));
                const matchesType = this.filterType === 'all' || task.type === this.filterType;
                const matchesStatus = this.filterStatus === 'all' ||
                    (this.filterStatus === 'enabled' && task.enabled) ||
                    (this.filterStatus === 'disabled' && !task.enabled);
                return matchesSearch && matchesType && matchesStatus;
            });
            return filtered.sort((a, b) => {
                switch (this.sortBy) {
                    case 'type': return a.type.localeCompare(b.type);
                    case 'schedule': return (a.schedule || '').localeCompare(b.schedule || '');
                    case 'status': return b.enabled - a.enabled;
                    case 'lastRun':
                        const aRun = this.getLastRun(a.id);
                        const bRun = this.getLastRun(b.id);
                        if (!aRun && !bRun) return 0;
                        if (!aRun) return 1;
                        if (!bRun) return -1;
                        return new Date(bRun.timestamp) - new Date(aRun.timestamp);
                    default: return a.name.localeCompare(b.name);
                }
            });
        },
        taskGroups() {
            const groups = {};
            // Group all tasks by their actual type
            this.filteredTasks.forEach(task => {
                let groupKey, groupConfig;
                // Handle special case for watcher tasks
                if (task.type === 'watcher') {
                    groupKey = 'watchers';
                    groupConfig = {
                        type: 'watchers',
                        name: 'File & Event Watchers',
                        description: 'Automated monitoring tasks',
                        icon: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z',
                        color: 'indigo'
                    };
                } else {
                    // Use task type as group key
                    groupKey = task.type;
                    const typeConfig = this.getTaskTypeConfig(task.type);
                    groupConfig = {
                        type: 'type',
                        name: typeConfig.label + 's',  // pluralize
                        description: typeConfig.description,
                        icon: typeConfig.icon,
                        color: typeConfig.color
                    };
                }
                // Initialize group if it doesn't exist
                if (!groups[groupKey]) {
                    groups[groupKey] = {
                        ...groupConfig,
                        tasks: [],
                        expanded: this.expandedGroups.has(groupKey)
                    };
                }
                // Add task to group
                groups[groupKey].tasks.push(task);
            });
            // Remove empty groups
            Object.keys(groups).forEach(key => {
                const group = groups[key];
                if (!group.tasks || group.tasks.length === 0) {
                    delete groups[key];
                }
            });
            return groups;
        },
        taskStats() {
            const stats = {
                total: this.tasks.length,
                enabled: this.tasks.filter(t => t.enabled).length,
                shell: this.tasks.filter(t => t.type === 'shell').length,
                ai: this.tasks.filter(t => t.type === 'ai').length,
                dependency: this.tasks.filter(t => t.type === 'dependency').length,
                watcher: this.tasks.filter(t => t.type === 'watcher').length,
                manual: this.tasks.filter(t => t.type === 'manual').length,
                runningNow: this.taskRuns.filter(r => r.status === 'running').length,
                completedToday: 0,
                failedRecent: 0
            };
        
            // Calculate 24 hours ago timestamp - FIXED
            const twentyFourHoursAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
        
            // Debug: Log the actual data structure
            console.log('=== TASK STATS DEBUG ===');
            console.log('Task runs data sample:', this.taskRuns.slice(0, 3));
            console.log('Fields available:', this.taskRuns.length > 0 ? Object.keys(this.taskRuns[0]) : 'No runs');
        
            // Count completed and failed in last 24 hours
            this.taskRuns.forEach(run => {
                // Try multiple timestamp field variations
                const timestamp = run.last_run || run.lastRun || run.timestamp || run.created_at || run.finished_at;
                
                if (!timestamp) {
                    console.log('No timestamp found for run:', Object.keys(run));
                    return;
                }
        
                try {
                    const runDate = new Date(timestamp);
                    
                    // Check if the date is valid
                    if (isNaN(runDate.getTime())) {
                        console.log('Invalid date for run:', run, 'timestamp:', timestamp);
                        return;
                    }
        
                    // Check if it's within the last 24 hours
                    if (runDate > twentyFourHoursAgo) {
                        // Check both 'status' and potential status variations
                        const status = run.status?.toLowerCase();
                        if (status === 'completed' || status === 'success') {
                            stats.completedToday++;
                        } else if (status === 'failed' || status === 'error' || status === 'failure') {
                            stats.failedRecent++;
                        }
                        console.log('Found recent run:', {status, timestamp, runDate});
                    }
                } catch (error) {
                    console.error('Error parsing date for run:', run, error);
                }
            });
        
            console.log('Calculated stats:', stats);
            return stats;
        },
        uniqueTaskTypes() {
            const types = new Set(this.tasks.map(t => t.type));
            return Array.from(types);
        },
        availableDependencies() {
            return this.tasks.filter(t => t.type !== 'dependency');
        }
    },
    methods: {
        // Core API methods
        async loadTasks() {
            this.loading = true;
            this.error = null;
            try {
                const response = await this.taskSchedulerRequest('/api/tasks', 'GET');
                this.tasks = Array.isArray(response) ? response : [];
                await Promise.all([
                    this.loadTaskRuns(),
                    this.loadMetrics()
                ]);
            } catch (err) {
                this.error = `Failed to load tasks: ${err.message}`;
                this.showToast(this.error, 'error');
            } finally {
                this.loading = false;
            }
        },
        async loadTaskRuns() {
            try {
                const response = await this.taskSchedulerRequest('/api/runs/status', 'GET');
                this.taskRuns = Array.isArray(response) ? response : [];
            } catch (err) {
                console.warn('Failed to load task runs:', err);
                this.taskRuns = [];
            }
        },
        async loadMetrics() {
            try {
                const response = await this.taskSchedulerRequest('/api/metrics', 'GET');
                this.metrics = response || {};
            } catch (err) {
                console.warn('Failed to load metrics:', err);
                this.metrics = {};
            }
        },
        async taskSchedulerRequest(endpoint, method = 'GET', data = null) {
            const dashboardUrl = endpoint.replace('/api/', '/api/task-scheduler/');
            const options = {
                method,
                headers: {
                    'Content-Type': 'application/json'
                }
            };
            if (this.config.apiKey) {
                options.headers['Authorization'] = `Bearer ${this.config.apiKey}`;
            }
            if (data) {
                options.body = JSON.stringify(data);
            }
            const response = await fetch(dashboardUrl, options);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }
            return response.json();
        },
        // Task management methods
        async createTask() {
            try {
                let endpoint, requestData;
                // Prepare request data based on task type
                const baseData = {
                    name: this.newTask.name,
                    description: this.newTask.description,
                    enabled: this.newTask.enabled,
                    type: this.newTask.type
                };
                switch (this.newTask.type) {
                    case 'shell':
                        endpoint = '/api/tasks';
                        requestData = {
                            ...baseData,
                            command: this.newTask.command,
                            schedule: this.newTask.schedule
                        };
                        break;
                    case 'ai':
                        endpoint = '/api/tasks/ai';
                        requestData = {
                            ...baseData,
                            prompt: this.newTask.prompt,
                            schedule: this.newTask.schedule,
                            model: this.newTask.model,
                            modelHint: this.newTask.modelHint,
                            maxCost: parseFloat(this.newTask.maxCost) || 1.0,
                            requireLocal: this.newTask.requireLocal
                        };
                        break;
                    case 'manual':
                        endpoint = '/api/tasks/manual';
                        requestData = {
                            ...baseData,
                            command: this.newTask.command,
                            prompt: this.newTask.prompt
                        };
                        break;
                    case 'dependency':
                        endpoint = '/api/tasks/dependency';
                        requestData = {
                            ...baseData,
                            command: this.newTask.command,
                            dependsOn: this.newTask.dependsOn
                        };
                        break;
                    case 'watcher':
                        endpoint = '/api/tasks/watcher';
                        requestData = {
                            ...baseData,
                            command: this.newTask.command,
                            watcherConfig: {
                                type: 'file_change',
                                triggerOnce: false,
                                watchPath: '/workspace',
                                checkInterval: '30s'
                            }
                        };
                        break;
                    default:
                        throw new Error('Unknown task type');
                }
                await this.taskSchedulerRequest(endpoint, 'POST', requestData);
                this.showCreateTask = false;
                this.resetNewTask();
                await this.loadTasks();
                this.showToast('Task created successfully', 'success');
            } catch (err) {
                this.showToast(`Failed to create task: ${err.message}`, 'error');
            }
        },
        async deleteTask(taskId) {
            if (!confirm('Are you sure you want to delete this task? This action cannot be undone.')) return;
            try {
                await this.taskSchedulerRequest(`/api/tasks/${taskId}`, 'DELETE');
                await this.loadTasks();
                this.showToast('Task deleted successfully', 'success');
            } catch (err) {
                this.showToast(`Failed to delete task: ${err.message}`, 'error');
            }
        },
        async toggleTask(taskId) {
            const task = this.tasks.find(t => t.id === taskId);
            if (!task) return;
            try {
                const endpoint = task.enabled ? `/api/tasks/${taskId}/disable` : `/api/tasks/${taskId}/enable`;
                await this.taskSchedulerRequest(endpoint, 'POST');
                await this.loadTasks();
                this.showToast(`Task ${task.enabled ? 'disabled' : 'enabled'} successfully`, 'success');
            } catch (err) {
                this.showToast(`Failed to toggle task: ${err.message}`, 'error');
            }
        },
        async runTask(taskId) {
            const task = this.tasks.find(t => t.id === taskId);
            if (!task) return;
            if (!confirm(`Run task "${task.name}" now?`)) return;
            try {
                await this.taskSchedulerRequest(`/api/tasks/${taskId}/run`, 'POST');
                this.showToast('Task execution started', 'success');
                setTimeout(() => this.loadTaskRuns(), 2000);
            } catch (err) {
                this.showToast(`Failed to run task: ${err.message}`, 'error');
            }
        },
        async viewTaskOutput(taskId, runId = null) {
            try {
                const endpoint = runId
                    ? `/api/tasks/${taskId}/runs/${runId}/output`
                    : `/api/tasks/${taskId}/output`;
                const output = await this.taskSchedulerRequest(endpoint, 'GET');
                const outputKey = runId ? `${taskId}-${runId}` : taskId;
                this.showRunOutput[outputKey] = {
                    taskId,
                    runId,
                    output: typeof output === 'string' ? output : JSON.stringify(output, null, 2),
                    timestamp: new Date().toISOString()
                };
                this.$forceUpdate();
            } catch (err) {
                this.showToast(`Failed to get task output: ${err.message}`, 'error');
            }
        },
        // UI Helper methods
        toggleTaskExpansion(taskId) {
            if (this.expandedTasks.has(taskId)) {
                this.expandedTasks.delete(taskId);
            } else {
                this.expandedTasks.add(taskId);
            }
            this.$forceUpdate();
        },
        toggleGroupExpansion(groupKey) {
            if (this.expandedGroups.has(groupKey)) {
                this.expandedGroups.delete(groupKey);
            } else {
                this.expandedGroups.add(groupKey);
            }
            this.$forceUpdate();
        },
        isTaskExpanded(taskId) {
            return this.expandedTasks.has(taskId);
        },
        isGroupExpanded(groupKey) {
            return this.expandedGroups.has(groupKey);
        },
        closeRunOutput(outputKey) {
            delete this.showRunOutput[outputKey];
            this.$forceUpdate();
        },
        // Task analysis methods
        findRootTask(dependencyTask) {
            if (!dependencyTask.dependsOn || dependencyTask.dependsOn.length === 0) {
                return dependencyTask;
            }
            const parentId = dependencyTask.dependsOn[0];
            const parentTask = this.tasks.find(t => t.id === parentId);
            if (!parentTask) return dependencyTask;
            if (parentTask.type === 'dependency' && parentTask.dependsOn && parentTask.dependsOn.length > 0) {
                return this.findRootTask(parentTask);
            }
            return parentTask;
        },
        getTaskTypeConfig(type) {
            return this.taskTypes.find(t => t.value === type) || {
                label: type.charAt(0).toUpperCase() + type.slice(1),
                icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
                color: 'gray',
                description: 'Custom task type'
            };
        },
        getLastRun(taskId) {
            return this.taskRuns
                .filter(run => run.task_id === taskId)
                .sort((a, b) => {
                    // Use the correct field names from the API response
                    const timestampA = a.last_run || a.lastRun || a.timestamp;
                    const timestampB = b.last_run || b.lastRun || b.timestamp;
                    if (!timestampA || !timestampB) return 0;
                    return new Date(timestampB) - new Date(timestampA);
                })[0];
        },
        formatTimestamp(timestamp) {
            if (!timestamp) return 'Never';
            try {
                const date = new Date(timestamp);
                if (isNaN(date.getTime())) return timestamp;
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
        getRecentRuns(taskId, limit = 10) {
            return this.taskRuns
                .filter(run => run.task_id === taskId)
                .sort((a, b) => {
                    const timestampA = a.last_run || a.lastRun || a.timestamp;
                    const timestampB = b.last_run || b.lastRun || b.timestamp;
                    if (!timestampA || !timestampB) return 0;
                    return new Date(timestampB) - new Date(timestampA);
                })
                .slice(0, limit);
        },
        getTaskStatusClass(task) {
            if (!task.enabled) return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400';
            const lastRun = this.getLastRun(task.id);
            if (!lastRun) return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300';
            switch (lastRun.status) {
                case 'completed': return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300';
                case 'failed': return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300';
                case 'running': return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300';
                default: return 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400';
            }
        },
        getStatusBadge(task) {
            if (!task.enabled) return { text: 'Disabled', class: 'bg-gray-500' };
            const lastRun = this.getLastRun(task.id);
            if (!lastRun) return { text: 'Never Run', class: 'bg-blue-500' };
            switch (lastRun.status) {
                case 'completed': return { text: 'Success', class: 'bg-green-500' };
                case 'failed': return { text: 'Failed', class: 'bg-red-500' };
                case 'running': return { text: 'Running', class: 'bg-yellow-500 animate-pulse' };
                default: return { text: 'Unknown', class: 'bg-gray-500' };
            }
        },
        getDependencyChain(task) {
            const chain = [];
            if (task.dependsOn && task.dependsOn.length > 0) {
                task.dependsOn.forEach(depId => {
                    const depTask = this.tasks.find(t => t.id === depId);
                    if (depTask) {
                        chain.push(depTask);
                        const subChain = this.getDependencyChain(depTask);
                        chain.push(...subChain);
                    }
                });
            }
            return chain;
        },
        formatSchedule(schedule) {
            if (!schedule) return 'Manual only';
            const preset = this.cronPresets.find(p => p.value === schedule);
            return preset ? preset.label : schedule;
        },
        getCronDescription(cron) {
            if (!cron) return '';
            
            // Enhanced cron description patterns
            const descriptions = {
                '* * * * *': 'Every minute',
                '*/5 * * * *': 'Every 5 minutes',
                '*/10 * * * *': 'Every 10 minutes',
                '*/15 * * * *': 'Every 15 minutes',
                '*/30 * * * *': 'Every 30 minutes',
                '0 * * * *': 'Every hour',
                '0 */2 * * *': 'Every 2 hours',
                '0 */3 * * *': 'Every 3 hours',
                '0 */6 * * *': 'Every 6 hours',
                '0 */12 * * *': 'Every 12 hours',
                '0 0 * * *': 'Daily at midnight',
                '0 9 * * *': 'Daily at 9:00 AM',
                '0 12 * * *': 'Daily at noon',
                '0 18 * * *': 'Daily at 6:00 PM',
                '0 0 * * 0': 'Weekly on Sunday at midnight',
                '0 0 * * 1': 'Weekly on Monday at midnight',
                '0 9 * * 1': 'Weekly on Monday at 9:00 AM',
                '0 0 1 * *': 'Monthly on the 1st at midnight',
                '0 0 1 1 *': 'Yearly on January 1st at midnight',
                '0 0 15 * *': 'Monthly on the 15th at midnight',
                '0 13 * * 4': 'Weekly on Thursday at 1:00 PM (Every 3rd Thursday pattern)',
            };
            
            // Check for exact matches first
            if (descriptions[cron]) {
                return descriptions[cron];
            }
            
            // Parse common patterns
            const parts = cron.split(' ');
            if (parts.length >= 5) {
                const [minute, hour, day, month, dayOfWeek] = parts;
                
                // Hourly patterns
                if (minute !== '*' && hour === '*' && day === '*' && month === '*' && dayOfWeek === '*') {
                    return `Hourly at ${minute} minutes past the hour`;
                }
                
                // Daily patterns
                if (hour !== '*' && minute !== '*' && day === '*' && month === '*' && dayOfWeek === '*') {
                    const hourNum = parseInt(hour);
                    const minNum = parseInt(minute);
                    const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;
                    return `Daily at ${time}`;
                }
                
                // Weekly patterns
                if (dayOfWeek !== '*' && hour !== '*' && minute !== '*') {
                    const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
                    const dayName = days[parseInt(dayOfWeek)] || `day ${dayOfWeek}`;
                    const hourNum = parseInt(hour);
                    const minNum = parseInt(minute);
                    const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;
                    return `Weekly on ${dayName} at ${time}`;
                }
                
                // Monthly patterns
                if (day !== '*' && hour !== '*' && minute !== '*' && month === '*' && dayOfWeek === '*') {
                    const hourNum = parseInt(hour);
                    const minNum = parseInt(minute);
                    const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;
                    const dayNum = parseInt(day);
                    const suffix = dayNum === 1 ? 'st' : dayNum === 2 ? 'nd' : dayNum === 3 ? 'rd' : 'th';
                    return `Monthly on the ${dayNum}${suffix} at ${time}`;
                }
                
                // Minute interval patterns
                if (minute.startsWith('*/')) {
                    const interval = minute.substring(2);
                    return `Every ${interval} minutes`;
                }
                
                // Hour interval patterns
                if (hour.startsWith('*/')) {
                    const interval = hour.substring(2);
                    return `Every ${interval} hours`;
                }
            }
            
            // Fallback for complex patterns
            return `Custom schedule: ${cron}`;
        },
        formatDuration(ms) {
            if (!ms || ms < 0) return '0s';
            const seconds = Math.floor(ms / 1000);
            const minutes = Math.floor(seconds / 60);
            const hours = Math.floor(minutes / 60);
            if (hours > 0) return `${hours}h ${minutes % 60}m`;
            if (minutes > 0) return `${minutes}m ${seconds % 60}s`;
            return `${seconds}s`;
        },
        formatFileSize(bytes) {
            if (!bytes || bytes === 0) return '0 B';
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(1024));
            return Math.round(bytes / Math.pow(1024, i) * 100) / 100 + ' ' + sizes[i];
        },
        resetNewTask() {
            this.newTask = {
                type: 'shell',
                name: '',
                description: '',
                command: '',
                prompt: '',
                schedule: '0 0 * * *',
                enabled: true,
                model: '',
                modelHint: 'balanced',
                maxCost: '1.0',
                requireLocal: false,
                dependsOn: []
            };
        },
        setupAutoRefresh() {
            if (this.refreshInterval) {
                clearInterval(this.refreshInterval);
                this.refreshInterval = null;
            }
            if (this.autoRefresh) {
                this.refreshInterval = setInterval(() => {
                    this.loadTasks();
                }, 30000);
            }
        },
        showToast(message, type = 'info') {
            window.showToast && window.showToast(message, type);
        }
    },
    async mounted() {
        await this.loadTasks();
        this.setupAutoRefresh();
    },
    beforeUnmount() {
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    },
    watch: {
        autoRefresh() {
            this.setupAutoRefresh();
        }
    },
    template: `
    <div class="task-scheduler space-y-4 animate-fade-in max-w-full overflow-x-hidden">
        <!-- Enhanced Header -->
        <div class="enhanced-card p-4 lg:p-6">
            <div class="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
                <div class="flex items-center space-x-3">
                    <div class="flex-shrink-0">
                        <div class="w-10 h-10 bg-gradient-to-r from-indigo-500 to-purple-600 rounded-xl flex items-center justify-center">
                            <svg class="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                        </div>
                    </div>
                    <div>
                        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">Task Scheduler</h1>
                        <p class="text-sm text-gray-500 dark:text-gray-400">Manage scheduled AI tasks, shell commands, and automation workflows</p>
                    </div>
                </div>
                <div class="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
                    <button
                        @click="showCreateTask = true"
                        class="touch-target inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-all"
                        aria-label="Create new task"
                    >
                        <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                            <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                        </svg>
                        Create Task
                    </button>
                    <button
                        @click="loadTasks"
                        :disabled="loading"
                        class="touch-target inline-flex items-center px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-600 focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                        :aria-label="loading ? 'Refreshing tasks...' : 'Refresh tasks'"
                    >
                        <svg class="w-4 h-4 mr-2" :class="{ 'animate-spin': loading }" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                            <path fill-rule="evenodd" d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z" clip-rule="evenodd"></path>
                        </svg>
                        Refresh
                    </button>
                    <label class="inline-flex items-center">
                        <input
                            v-model="autoRefresh"
                            type="checkbox"
                            class="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500 focus:ring-offset-2"
                            aria-describedby="auto-refresh-description"
                        >
                        <span id="auto-refresh-description" class="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-refresh</span>
                    </label>
                </div>
            </div>
            <!-- Search and Filters -->
            <div class="mt-6 space-y-4">
                <div class="flex flex-col lg:flex-row space-y-3 lg:space-y-0 lg:space-x-4">
                    <div class="flex-1 relative">
                        <label for="task-search" class="sr-only">Search tasks</label>
                        <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                            <svg class="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                <path fill-rule="evenodd" d="M8 4a4 4 0 100 8 4 4 0 000-8zM2 8a6 6 0 1110.89 3.476l4.817 4.817a1 1 0 01-1.414 1.414l-4.816-4.816A6 6 0 012 8z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                        <input
                            id="task-search"
                            v-model="searchTerm"
                            type="text"
                            placeholder="Search tasks by name, description, command, or prompt..."
                            class="form-input pl-10 w-full"
                            aria-label="Search tasks"
                        >
                    </div>
                    <div class="flex flex-col sm:flex-row sm:space-x-4 space-y-3 sm:space-y-0">
                        <div class="sm:w-40">
                            <label for="filter-type" class="sr-only">Filter by type</label>
                            <select id="filter-type" v-model="filterType" class="form-input w-full" aria-label="Filter tasks by type">
                                <option value="all">All Types</option>
                                <option v-for="type in uniqueTaskTypes" :key="type" :value="type">
                                    {{ getTaskTypeConfig(type).label }}
                                </option>
                            </select>
                        </div>
                        <div class="sm:w-32">
                            <label for="filter-status" class="sr-only">Filter by status</label>
                            <select id="filter-status" v-model="filterStatus" class="form-input w-full" aria-label="Filter tasks by status">
                                <option value="all">All Status</option>
                                <option value="enabled">Enabled</option>
                                <option value="disabled">Disabled</option>
                            </select>
                        </div>
                        <div class="sm:w-40">
                            <label for="sort-by" class="sr-only">Sort tasks by</label>
                            <select id="sort-by" v-model="sortBy" class="form-input w-full" aria-label="Sort tasks by">
                                <option value="name">Sort by Name</option>
                                <option value="type">Sort by Type</option>
                                <option value="status">Sort by Status</option>
                                <option value="schedule">Sort by Schedule</option>
                                <option value="lastRun">Sort by Last Run</option>
                            </select>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <!-- Error Display -->
        <div v-if="error" class="enhanced-card border-red-200 dark:border-red-800 bg-red-50 dark:bg-red-900/20 p-4" role="alert" aria-live="polite">
            <div class="flex items-start">
                <svg class="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd"></path>
                </svg>
                <div class="ml-3 flex-1">
                    <div class="text-sm text-red-800 dark:text-red-200">{{ error }}</div>
                    <button @click="error = null" class="mt-2 text-xs text-red-600 hover:text-red-800 dark:text-red-400 dark:hover:text-red-200 underline focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 rounded">
                        Dismiss error
                    </button>
                </div>
            </div>
        </div>
        <!-- Enhanced Stats Overview -->
        <div class="responsive-grid cols-2 lg:cols-5 gap-3 sm:gap-4" role="region" aria-label="Task statistics">
            <div class="enhanced-card p-4">
                <div class="flex items-center">
                    <div class="flex-shrink-0">
                        <div class="w-8 h-8 bg-blue-500 rounded-lg flex items-center justify-center" aria-hidden="true">
                            <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                            </svg>
                        </div>
                    </div>
                    <div class="ml-3">
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Total Tasks</p>
                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ taskStats.total }}</p>
                    </div>
                </div>
            </div>
            <div class="enhanced-card p-4">
                <div class="flex items-center">
                    <div class="flex-shrink-0">
                        <div class="w-8 h-8 bg-green-500 rounded-lg flex items-center justify-center" aria-hidden="true">
                            <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                    </div>
                    <div class="ml-3">
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Enabled</p>
                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ taskStats.enabled }}</p>
                    </div>
                </div>
            </div>
            <div class="enhanced-card p-4">
                <div class="flex items-center">
                    <div class="flex-shrink-0">
                        <div class="w-8 h-8 bg-yellow-500 rounded-lg flex items-center justify-center" aria-hidden="true">
                            <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path>
                            </svg>
                        </div>
                    </div>
                    <div class="ml-3">
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Currently Running</p>
                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ taskStats.runningNow }}</p>
                    </div>
                </div>
            </div>
            <div class="enhanced-card p-4">
                <div class="flex items-center">
                    <div class="flex-shrink-0">
                        <div class="w-8 h-8 bg-emerald-500 rounded-lg flex items-center justify-center" aria-hidden="true">
                            <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M6.267 3.455a3.066 3.066 0 001.745-.723 3.066 3.066 0 013.976 0 3.066 3.066 0 001.745.723 3.066 3.066 0 012.812 2.812c.051.643.304 1.254.723 1.745a3.066 3.066 0 010 3.976 3.066 3.066 0 00-.723 1.745 3.066 3.066 0 01-2.812 2.812 3.066 3.066 0 00-1.745.723 3.066 3.066 0 01-3.976 0 3.066 3.066 0 00-1.745-.723 3.066 3.066 0 01-2.812-2.812 3.066 3.066 0 00-.723-1.745 3.066 3.066 0 010-3.976 3.066 3.066 0 00.723-1.745 3.066 3.066 0 012.812-2.812zm7.44 5.252a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                    </div>
                    <div class="ml-3">
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Completed (24h)</p>
                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ taskStats.completedToday }}</p>
                    </div>
                </div>
            </div>
            <div class="enhanced-card p-4">
                <div class="flex items-center">
                    <div class="flex-shrink-0">
                        <div class="w-8 h-8 bg-red-500 rounded-lg flex items-center justify-center" aria-hidden="true">
                            <svg class="h-5 w-5 text-white" fill="currentColor" viewBox="0 0 20 20">
                                <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clip-rule="evenodd"></path>
                            </svg>
                        </div>
                    </div>
                    <div class="ml-3">
                        <p class="text-sm font-medium text-gray-500 dark:text-gray-400">Failed (24h)</p>
                        <p class="text-2xl font-bold text-gray-900 dark:text-white">{{ taskStats.failedRecent }}</p>
                    </div>
                </div>
            </div>
        </div>
        <!-- Loading State -->
        <div v-if="loading && tasks.length === 0" class="enhanced-card p-8 text-center" role="status" aria-live="polite">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500 mx-auto mb-4" aria-hidden="true"></div>
            <p class="text-lg font-medium text-gray-900 dark:text-white">Loading tasks...</p>
            <p class="text-sm text-gray-500 dark:text-gray-400">Fetching task data from scheduler</p>
        </div>
        <!-- Empty State -->
        <div v-else-if="Object.keys(taskGroups).length === 0 && !loading" class="enhanced-card p-8 text-center">
            <svg class="mx-auto h-12 w-12 text-gray-400 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
            <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-2">No tasks found</h2>
            <p class="text-gray-500 dark:text-gray-400 mb-4">
                {{ searchTerm || filterType !== 'all' || filterStatus !== 'all'
                    ? 'Try adjusting your search or filters to find tasks'
                    : 'Get started by creating your first automated task' }}
            </p>
            <button
                v-if="!searchTerm && filterType === 'all' && filterStatus === 'all'"
                @click="showCreateTask = true"
                class="inline-flex items-center px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-all"
            >
                <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                    <path fill-rule="evenodd" d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z" clip-rule="evenodd"></path>
                </svg>
                Create Your First Task
            </button>
        </div>
        <!-- Task Groups with Mobile-Optimized Layout -->
        <div v-else class="space-y-4" role="region" aria-label="Task groups">
            <div v-for="(group, groupKey) in taskGroups" :key="groupKey" class="enhanced-card overflow-hidden">
                <!-- Group Header -->
                <button
                    @click="toggleGroupExpansion(groupKey)"
                    class="w-full px-6 py-4 border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                    :aria-expanded="isGroupExpanded(groupKey)"
                    :aria-controls="'group-content-' + groupKey"
                    :id="'group-header-' + groupKey"
                >
                    <div class="flex items-center justify-between text-left">
                        <div class="flex items-center space-x-3">
                            <div :class="['w-10 h-10 rounded-lg flex items-center justify-center', \`bg-\${group.color}-500\`]" aria-hidden="true">
                                <svg class="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="group.icon"></path>
                                </svg>
                            </div>
                            <div>
                                <h2 class="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
                                    {{ group.name }}
                                    <span class="ml-2 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-200">
                                        {{ group.tasks.length }}
                                    </span>
                                </h2>
                                <p class="text-sm text-gray-500 dark:text-gray-400">{{ group.description }}</p>
                            </div>
                        </div>
                        <svg
                            :class="[
                                'w-5 h-5 text-gray-400 transition-transform duration-200',
                                isGroupExpanded(groupKey) ? 'rotate-180' : ''
                            ]"
                            fill="currentColor"
                            viewBox="0 0 20 20"
                            aria-hidden="true"
                        >
                            <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                        </svg>
                    </div>
                </button>
                <!-- Group Content -->
                <div
                    v-if="isGroupExpanded(groupKey)"
                    :id="'group-content-' + groupKey"
                    :aria-labelledby="'group-header-' + groupKey"
                    class="divide-y divide-gray-200 dark:divide-gray-700"
                >
                    <div v-for="task in group.tasks" :key="task.id" class="relative">
                        <!-- Task Header -->
                        <button
                            @click="toggleTaskExpansion(task.id)"
                            class="w-full p-4 sm:p-6 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
                            :aria-expanded="isTaskExpanded(task.id)"
                            :aria-controls="'task-content-' + task.id"
                            :id="'task-header-' + task.id"
                        >
                            <div class="flex flex-col sm:flex-row sm:items-start sm:justify-between text-left space-y-3 sm:space-y-0">
                                <!-- Main Task Info -->
                                <div class="flex items-start space-x-4 flex-1 min-w-0">
                                    <!-- Status Indicator -->
                                    <div class="flex-shrink-0 relative mt-1">
                                        <div :class="['w-3 h-3 rounded-full', getStatusBadge(task).class]" :aria-label="getStatusBadge(task).text"></div>
                                        <div v-if="getStatusBadge(task).text === 'Running'" class="absolute inset-0 w-3 h-3 bg-yellow-400 rounded-full animate-ping opacity-75" aria-hidden="true"></div>
                                    </div>
                                    <!-- Task Details -->
                                    <div class="flex-1 min-w-0">
                                        <div class="flex flex-col sm:flex-row sm:items-center space-y-2 sm:space-y-0 sm:space-x-3 mb-2">
                                            <h3 class="text-base sm:text-lg font-medium text-gray-900 dark:text-white truncate">
                                                {{ task.name }}
                                            </h3>
                                            <div class="flex flex-wrap items-center gap-2">
                                                <span :class="[
                                                    'inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium',
                                                    \`bg-\${getTaskTypeConfig(task.type).color}-100 text-\${getTaskTypeConfig(task.type).color}-800 dark:bg-\${getTaskTypeConfig(task.type).color}-900/30 dark:text-\${getTaskTypeConfig(task.type).color}-200\`
                                                ]">
                                                    {{ getTaskTypeConfig(task.type).label }}
                                                </span>
                                                <span :class="[
                                                    'inline-flex items-center px-2 py-0.5 rounded text-xs font-medium',
                                                    getStatusBadge(task).class,
                                                    'text-white'
                                                ]">
                                                    {{ getStatusBadge(task).text }}
                                                </span>
                                                <span v-if="!task.enabled" class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400">
                                                    Disabled
                                                </span>
                                            </div>
                                        </div>
                                        <p v-if="task.description" class="text-sm text-gray-600 dark:text-gray-400 mb-3">
                                            {{ task.description }}
                                        </p>
                                        <!-- Task Summary Info -->
                                        <div class="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-4 text-xs sm:text-sm text-gray-500 dark:text-gray-400">
                                            <span v-if="task.schedule" class="flex items-center">
                                                <svg class="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z" clip-rule="evenodd"></path>
                                                </svg>
                                                <span class="truncate">{{ formatSchedule(task.schedule) }}</span>
                                            </span>
                                            <span v-if="getLastRun(task.id)" class="flex items-center">
                                                <svg class="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-8.293l-3-3a1 1 0 00-1.414 1.414L10.586 9H7a1 1 0 100 2h3.586l-1.293 1.293a1 1 0 101.414 1.414l3-3a1 1 0 000-1.414z" clip-rule="evenodd"></path>
                                                </svg>
                                                <span class="truncate">Last: {{ formatTimestamp(getLastRun(task.id).last_run) }}</span>
                                            </span>
                                            <span v-if="task.type === 'ai'" class="flex items-center">
                                                <svg class="w-4 h-4 mr-1 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z"></path>
                                                </svg>
                                                <span class="truncate">{{ task.modelHint || 'balanced' }}</span>
                                            </span>
                                        </div>
                                    </div>
                                </div>
                                <!-- Mobile: Task Actions Below Content, Desktop: Actions on Right -->
                                <div class="w-full sm:w-auto">
                                    <div class="flex sm:items-center justify-between sm:justify-end sm:space-x-2" @click.stop role="group" aria-label="Task actions">
                                        <!-- Action Buttons -->
                                        <div class="flex items-center space-x-1 sm:space-x-2">
                                            <button
                                                @click="runTask(task.id)"
                                                :disabled="!task.enabled"
                                                class="touch-target flex items-center px-2 py-1 text-xs sm:text-sm font-medium rounded-lg text-white bg-green-600 hover:bg-green-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-all"
                                                :title="task.enabled ? 'Run task now' : 'Task is disabled'"
                                                :aria-label="task.enabled ? 'Run task now' : 'Task is disabled'"
                                            >
                                                <svg class="w-3 h-3 sm:w-4 sm:h-4 mr-1" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clip-rule="evenodd"></path>
                                                </svg>
                                                <span class="hidden sm:inline">Run</span>
                                            </button>
                                            <button
                                                @click="viewTaskOutput(task.id)"
                                                class="touch-target flex items-center px-2 py-1 text-xs sm:text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 transition-all"
                                                title="View latest output"
                                                aria-label="View latest output"
                                            >
                                                <svg class="w-3 h-3 sm:w-4 sm:h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
                                                </svg>
                                                <span class="hidden sm:inline">View</span>
                                            </button>
                                            <button
                                                @click="toggleTask(task.id)"
                                                :class="[
                                                    'touch-target flex items-center px-2 py-1 text-xs sm:text-sm font-medium rounded-lg transition-all',
                                                    task.enabled
                                                        ? 'text-white bg-yellow-600 hover:bg-yellow-700'
                                                        : 'text-white bg-green-600 hover:bg-green-700'
                                                ]"
                                                :title="task.enabled ? 'Disable task' : 'Enable task'"
                                                :aria-label="task.enabled ? 'Disable task' : 'Enable task'"
                                            >
                                                <svg v-if="task.enabled" class="w-3 h-3 sm:w-4 sm:h-4 mr-1" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M13.477 14.89A6 6 0 015.11 6.524l8.367 8.368zm1.414-1.414L6.524 5.11a6 6 0 018.367 8.367zM18 10a8 8 0 11-16 0 8 8 0 0116 0z" clip-rule="evenodd"></path>
                                                </svg>
                                                <svg v-else class="w-3 h-3 sm:w-4 sm:h-4 mr-1" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"></path>
                                                </svg>
                                                <span class="hidden sm:inline">{{ task.enabled ? 'Disable' : 'Enable' }}</span>
                                            </button>
                                            <button
                                                @click="deleteTask(task.id)"
                                                class="touch-target flex items-center px-2 py-1 text-xs sm:text-sm font-medium rounded-lg text-white bg-red-600 hover:bg-red-700 transition-all"
                                                title="Delete task"
                                                aria-label="Delete task"
                                            >
                                                <svg class="w-3 h-3 sm:w-4 sm:h-4 mr-1" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                    <path fill-rule="evenodd" d="M9 2a1 1 0 000 2h2a1 1 0 100-2H9z M4 5a2 2 0 012-2v1a2 2 0 002 2h4a2 2 0 002-2V3a2 2 0 012 2v6.5l1.707 1.707A1 1 0 0017 10.414V5a4 4 0 00-8 0v5.586l1.707-1.707A1 1 0 0012 10.414z" clip-rule="evenodd"></path>
                                                </svg>
                                                <span class="hidden sm:inline">Delete</span>
                                            </button>
                                        </div>
                                        <!-- Expand/Collapse Indicator -->
                                        <div class="flex items-center">
                                            <div class="hidden sm:block w-px h-6 bg-gray-200 dark:bg-gray-600 mx-2" aria-hidden="true"></div>
                                            <svg :class="['w-4 h-4 text-gray-400 transition-transform duration-200', isTaskExpanded(task.id) ? 'rotate-180' : '']" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                                <path fill-rule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                                            </svg>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </button>
                        <!-- Expanded Task Details -->
                        <div
                            v-if="isTaskExpanded(task.id)"
                            :id="'task-content-' + task.id"
                            :aria-labelledby="'task-header-' + task.id"
                            class="px-4 sm:px-6 pb-6 bg-gray-50 dark:bg-gray-800/50 border-t border-gray-100 dark:border-gray-700"
                        >
                            <div class="grid grid-cols-1 lg:grid-cols-2 gap-6 pt-6">
                                <!-- Configuration Details with Enhanced Cron Display -->
                                <div class="space-y-4">
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wide flex items-center">
                                        <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                            <path fill-rule="evenodd" d="M11.49 3.17c-.38-1.56-2.6-1.56-2.98 0a1.532 1.532 0 01-2.286.948c-1.372-.836-2.942.734-2.106 2.106.54.886.061 2.042-.947 2.287-1.561.379-1.561 2.6 0 2.978a1.532 1.532 0 01.947 2.287c-.836 1.372.734 2.942 2.106 2.106a1.532 1.532 0 012.287.947c.379 1.561 2.6 1.561 2.978 0a1.533 1.533 0 012.287-.947c1.372.836 2.942-.734 2.106-2.106a1.533 1.533 0 01.947-2.287c1.561-.379 1.561-2.6 0-2.978a1.532 1.532 0 01-.947-2.287c.836-1.372-.734-2.942-2.106-2.106a1.532 1.532 0 01-2.287-.947zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd"></path>
                                        </svg>
                                        Configuration
                                    </h4>
                                    <!-- Command/Prompt Display -->
                                    <div v-if="task.command || task.prompt" class="bg-gray-900 dark:bg-gray-900 rounded-lg p-4 font-mono text-sm">
                                        <div class="flex items-center justify-between mb-3">
                                            <span class="text-xs font-medium text-gray-400 uppercase tracking-wide">
                                                {{ task.type === 'ai' ? 'AI Prompt' : 'Command' }}
                                            </span>
                                            <button
                                                @click="() => { navigator.clipboard.writeText(task.command || task.prompt); showToast('Copied to clipboard', 'success'); }"
                                                class="inline-flex items-center text-xs text-gray-400 hover:text-gray-300 px-2 py-1 rounded hover:bg-gray-800 focus:ring-2 focus:ring-gray-500 transition-all"
                                                aria-label="Copy to clipboard"
                                            >
                                                <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                                                </svg>
                                                Copy
                                            </button>
                                        </div>
                                        <div class="text-gray-300 whitespace-pre-wrap break-words">
                                            <span v-if="task.type === 'shell'" class="text-green-400">$ </span>
                                            <span v-if="task.type === 'ai'" class="text-purple-400">AI: </span>
                                            {{ task.command || task.prompt }}
                                        </div>
                                    </div>
                                    <!-- Enhanced Task Settings with Complete Configuration Display -->
                                    <div class="bg-white dark:bg-gray-700 rounded-lg p-4 space-y-3 border border-gray-200 dark:border-gray-600">
                                        <div class="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                                            <!-- Basic Settings -->
                                            <div>
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Type:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ getTaskTypeConfig(task.type).label }}</span>
                                            </div>
                                            <div>
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Status:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.enabled ? 'Enabled' : 'Disabled' }}</span>
                                            </div>
                                            
                                            <!-- Enhanced Schedule Display -->
                                            <div v-if="task.schedule" class="sm:col-span-2">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Schedule:</span>
                                                <div class="mt-1">
                                                    <div class="text-gray-900 dark:text-gray-100 font-medium">
                                                        {{ formatSchedule(task.schedule) }}
                                                    </div>
                                                    <div class="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1 bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                                                        {{ task.schedule }}
                                                    </div>
                                                    <div class="text-xs text-gray-500 dark:text-gray-400 mt-1">
                                                        {{ getCronDescription(task.schedule) }}
                                                    </div>
                                                </div>
                                            </div>
                                            
                                            <!-- AI-Specific Settings -->
                                            <div v-if="task.type === 'ai' && task.modelHint">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Model Hint:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.modelHint }}</span>
                                            </div>
                                            <div v-if="task.type === 'ai' && task.model">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Specific Model:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100 font-mono text-xs">{{ task.model }}</span>
                                            </div>
                                            <div v-if="task.type === 'ai' && task.maxCost">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Max Cost:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">\${{ task.maxCost }}</span>
                                            </div>
                                            <div v-if="task.type === 'ai' && task.requireLocal !== undefined">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Local Only:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.requireLocal ? 'Yes' : 'No' }}</span>
                                            </div>
                                            
                                            <!-- Agent-Specific Settings -->
                                            <div v-if="task.isAgent">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Agent Mode:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">Autonomous Agent</span>
                                            </div>
                                            <div v-if="task.agentPersonality" class="sm:col-span-2">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Personality:</span>
                                                <div class="ml-2 text-gray-900 dark:text-gray-100 text-xs mt-1 bg-gray-100 dark:bg-gray-800 px-2 py-1 rounded">
                                                    {{ task.agentPersonality }}
                                                </div>
                                            </div>
                                            <div v-if="task.conversationName">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Conversation:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100 text-xs">{{ task.conversationName }}</span>
                                            </div>
                                            
                                            <!-- Dependency Settings -->
                                            <div v-if="task.dependsOn && task.dependsOn.length > 0">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Dependencies:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.dependsOn.length }} task(s)</span>
                                            </div>
                                            
                                            <!-- Watcher Settings -->
                                            <div v-if="task.watcherConfig">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Watcher Type:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.watcherConfig.type }}</span>
                                            </div>
                                            <div v-if="task.watcherConfig && task.watcherConfig.checkInterval">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Check Interval:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.watcherConfig.checkInterval }}</span>
                                            </div>
                                            <div v-if="task.watcherConfig && task.watcherConfig.watchPath">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Watch Path:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100 font-mono text-xs">{{ task.watcherConfig.watchPath }}</span>
                                            </div>
                                            <div v-if="task.watcherConfig && task.watcherConfig.filePattern">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">File Pattern:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100 font-mono text-xs">{{ task.watcherConfig.filePattern }}</span>
                                            </div>
                                            
                                            <!-- Execution Settings -->
                                            <div v-if="task.maxExecutionTime">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Max Runtime:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ formatDuration(task.maxExecutionTime) }}</span>
                                            </div>
                                            <div v-if="task.timezone">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Timezone:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.timezone }}</span>
                                            </div>
                                            <div v-if="task.skipHolidays">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Skip Holidays:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">Yes</span>
                                            </div>
                                            <div v-if="task.timeWindowID">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Time Window:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.timeWindowID }}</span>
                                            </div>
                                            
                                            <!-- Manual/Trigger Type -->
                                            <div v-if="task.runOnDemandOnly">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Execution:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">Manual Only</span>
                                            </div>
                                            <div v-if="task.triggerType && task.triggerType !== 'schedule'">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Trigger Type:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100">{{ task.triggerType }}</span>
                                            </div>
                                            
                                            <!-- Creation/Update Info -->
                                            <div v-if="task.createdAt" class="sm:col-span-2">
                                                <span class="font-medium text-gray-500 dark:text-gray-400">Created:</span>
                                                <span class="ml-2 text-gray-900 dark:text-gray-100 text-xs">{{ formatTimestamp(task.createdAt) }}</span>
                                                <span v-if="task.updatedAt && task.updatedAt !== task.createdAt" class="ml-4">
                                                    <span class="font-medium text-gray-500 dark:text-gray-400">Updated:</span>
                                                    <span class="ml-1 text-gray-900 dark:text-gray-100 text-xs">{{ formatTimestamp(task.updatedAt) }}</span>
                                                </span>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                                <!-- Recent Runs -->
                                <div class="space-y-4">
                                    <h4 class="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wide flex items-center">
                                        <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                            <path fill-rule="evenodd" d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zM3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6zM14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z"></path>
                                        </svg>
                                        Recent Runs
                                    </h4>
                                    <div v-if="getRecentRuns(task.id).length === 0" class="bg-white dark:bg-gray-700 rounded-lg p-6 text-center border border-gray-200 dark:border-gray-600">
                                        <svg class="w-10 h-10 mx-auto mb-3 text-gray-400 dark:text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"></path>
                                        </svg>
                                        <p class="text-sm font-medium text-gray-900 dark:text-gray-100">No runs yet</p>
                                        <p class="text-xs text-gray-500 dark:text-gray-400 mt-1">Task executions will appear here once the task runs</p>
                                    </div>
                                    <div v-else class="bg-white dark:bg-gray-700 rounded-lg border border-gray-200 dark:border-gray-600">
                                        <div class="p-4">
                                            <div class="space-y-3 max-h-64 overflow-y-auto">
                                                <div
                                                    v-for="run in getRecentRuns(task.id, 5)"
                                                    :key="run.id || run.timestamp"
                                                    class="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-600 hover:border-gray-300 dark:hover:border-gray-500 transition-colors"
                                                >
                                                    <div class="flex items-center space-x-3 flex-1 min-w-0">
                                                        <div :class="[
                                                            'w-2 h-2 rounded-full flex-shrink-0',
                                                            run.status === 'completed' ? 'bg-green-500' :
                                                            run.status === 'failed' ? 'bg-red-500' :
                                                            run.status === 'running' ? 'bg-yellow-500 animate-pulse' : 'bg-gray-400'
                                                        ]" :aria-label="'Status: ' + run.status"></div>
                                                        <div class="flex-1 min-w-0">
                                                            <div class="flex items-center space-x-2 mb-1">
                                                                <span class="text-sm font-medium text-gray-900 dark:text-gray-100">
                                                                    {{ formatTimestamp(run.timestamp) }}
                                                                </span>
                                                                <span :class="[
                                                                    'text-xs px-2 py-1 rounded-full font-medium',
                                                                    run.status === 'completed' ? 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-200' :
                                                                    run.status === 'failed' ? 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-200' :
                                                                    run.status === 'running' ? 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-200' :
                                                                    'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-200'
                                                                ]">
                                                                    {{ run.status }}
                                                                </span>
                                                            </div>
                                                            <div v-if="run.duration" class="text-xs text-gray-500 dark:text-gray-400">
                                                                Duration: {{ formatDuration(run.duration) }}
                                                            </div>
                                                        </div>
                                                    </div>
                                                    <button
                                                        @click="viewTaskOutput(task.id, run.id)"
                                                        class="inline-flex items-center px-3 py-1 text-xs font-medium text-blue-700 dark:text-blue-300 hover:text-blue-900 dark:hover:text-blue-100 bg-blue-50 dark:bg-blue-900/30 hover:bg-blue-100 dark:hover:bg-blue-900/50 border border-blue-200 dark:border-blue-700 rounded focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-all"
                                                        aria-label="View run output"
                                                    >
                                                        <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                                                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
                                                        </svg>
                                                        View
                                                    </button>
                                                </div>
                                            </div>
                                            <div v-if="getRecentRuns(task.id).length > 5" class="mt-3 pt-3 border-t border-gray-200 dark:border-gray-600 text-center">
                                                <button
                                                    class="text-sm text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-200 font-medium focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 rounded transition-all"
                                                    @click="viewTaskOutput(task.id)"
                                                >
                                                    View all {{ getRecentRuns(task.id).length }} runs
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
        <!-- Run Output Modals -->
        <div
            v-for="(output, outputKey) in showRunOutput"
            :key="outputKey"
            class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
            style="z-index: 9999;"
            role="dialog"
            aria-modal="true"
            aria-labelledby="output-modal-title"
            @click.self="closeRunOutput(outputKey)"
        >
            <div class="bg-white dark:bg-gray-800 rounded-lg w-full max-w-4xl max-h-[90vh] flex flex-col shadow-2xl">
                <div class="flex items-center justify-between p-6 border-b border-gray-200 dark:border-gray-700">
                    <div>
                        <h3 id="output-modal-title" class="text-lg font-semibold text-gray-900 dark:text-white">Task Output</h3>
                        <p class="text-sm text-gray-500 dark:text-gray-400">
                            Task ID: {{ output.taskId }}{{ output.runId ? \` | Run ID: \${output.runId}\` : '' }}
                        </p>
                    </div>
                    <button
                        @click="closeRunOutput(outputKey)"
                        class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 transition-all"
                        aria-label="Close output modal"
                    >
                        <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                            <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                        </svg>
                    </button>
                </div>
                <div class="flex-1 overflow-hidden p-6">
                    <div class="bg-gray-900 rounded-lg p-4 h-full overflow-y-auto">
                        <pre class="font-mono text-sm text-gray-300 whitespace-pre-wrap break-words">{{ output.output || 'No output available' }}</pre>
                    </div>
                </div>
                <div class="flex items-center justify-end space-x-3 p-6 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-800">
                    <button
                        @click="() => { navigator.clipboard.writeText(output.output); showToast('Output copied to clipboard', 'success'); }"
                        class="inline-flex items-center px-4 py-2 text-sm font-medium text-gray-700 dark:text-gray-300 bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-lg focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 transition-all"
                    >
                        <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                        </svg>
                        Copy Output
                    </button>
                    <button
                        @click="closeRunOutput(outputKey)"
                        class="px-4 py-2 text-sm font-medium text-white bg-blue-600 hover:bg-blue-700 rounded-lg focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 transition-all"
                    >
                        Close
                    </button>
                </div>
            </div>
        </div>
        <!-- Create Task Modal -->
        <div
            v-if="showCreateTask"
            class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4"
            style="z-index: 9999;"
            role="dialog"
            aria-modal="true"
            aria-labelledby="create-task-title"
            @click.self="showCreateTask = false; resetNewTask()"
        >
            <div class="bg-gray-900 dark:bg-gray-800 rounded-lg w-full max-w-2xl max-h-[90vh] overflow-y-auto shadow-2xl">
                <form @submit.prevent="createTask" class="p-6">
                    <div class="flex items-center justify-between mb-6">
                        <h2 id="create-task-title" class="text-lg font-semibold text-gray-900 dark:text-white">Create New Task</h2>
                        <button
                            type="button"
                            @click="showCreateTask = false; resetNewTask()"
                            class="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-1 rounded focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 transition-all"
                            aria-label="Close create task modal"
                        >
                            <svg class="w-6 h-6" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                                <path fill-rule="evenodd" d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z" clip-rule="evenodd"></path>
                            </svg>
                        </button>
                    </div>
                    <div class="space-y-6">
                        <!-- Task Type Selection -->
                        <fieldset>
                            <legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">Task Type</legend>
                            <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
                                <button
                                    v-for="taskType in taskTypes"
                                    :key="taskType.value"
                                    type="button"
                                    @click="newTask.type = taskType.value"
                                    :class="[
                                        'p-4 border-2 rounded-lg text-left transition-all transform hover:scale-105 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2',
                                        newTask.type === taskType.value
                                            ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 shadow-md'
                                            : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'
                                    ]"
                                    :aria-pressed="newTask.type === taskType.value"
                                >
                                    <div class="flex items-center space-x-2 mb-2">
                                        <div :class="['w-6 h-6 rounded flex items-center justify-center', \`bg-\${taskType.color}-500\`]" aria-hidden="true">
                                            <svg class="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" :d="taskType.icon"></path>
                                            </svg>
                                        </div>
                                        <span class="text-sm font-medium text-gray-900 dark:text-white">{{ taskType.label }}</span>
                                    </div>
                                    <p class="text-xs text-gray-500 dark:text-gray-400">{{ taskType.description }}</p>
                                </button>
                            </div>
                        </fieldset>
                        <!-- Basic Information -->
                        <div class="grid grid-cols-1 lg:grid-cols-2 gap-4">
                            <div>
                                <label for="task-name" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Task Name <span class="text-red-500">*</span>
                                </label>
                                <input
                                    id="task-name"
                                    v-model="newTask.name"
                                    type="text"
                                    required
                                    class="form-input w-full"
                                    placeholder="Enter descriptive task name"
                                    aria-describedby="task-name-help"
                                >
                                <p id="task-name-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">Choose a clear, descriptive name for your task</p>
                            </div>
                            <div>
                                <label for="task-description" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Description</label>
                                <input
                                    id="task-description"
                                    v-model="newTask.description"
                                    type="text"
                                    class="form-input w-full"
                                    placeholder="Brief description of what this task does"
                                    aria-describedby="task-description-help"
                                >
                                <p id="task-description-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">Optional brief description</p>
                            </div>
                        </div>
                        <!-- Schedule (for scheduled tasks) -->
                        <div v-if="newTask.type !== 'manual' && newTask.type !== 'dependency'">
                            <label for="task-schedule" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Schedule</label>
                            <select id="task-schedule" v-model="newTask.schedule" class="form-input w-full" aria-describedby="schedule-help">
                                <option v-for="preset in cronPresets" :key="preset.value" :value="preset.value">
                                    {{ preset.label }} ({{ preset.value }})
                                </option>
                            </select>
                            <p id="schedule-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">When should this task run automatically</p>
                        </div>
                        <!-- Task Type Specific Fields -->
                        <div v-if="newTask.type === 'shell'">
                            <label for="shell-command" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                Shell Command <span class="text-red-500">*</span>
                            </label>
                            <textarea
                                id="shell-command"
                                v-model="newTask.command"
                                rows="3"
                                required
                                class="form-input w-full font-mono text-sm"
                                placeholder="echo 'Hello World'"
                                aria-describedby="shell-command-help"
                            ></textarea>
                            <p id="shell-command-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">Enter the shell command to execute</p>
                        </div>
                        <div v-if="newTask.type === 'ai'" class="space-y-4">
                            <div>
                                <label for="ai-prompt" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    AI Prompt <span class="text-red-500">*</span>
                                </label>
                                <textarea
                                    id="ai-prompt"
                                    v-model="newTask.prompt"
                                    rows="4"
                                    required
                                    class="form-input w-full"
                                    placeholder="Describe what you want the AI to do..."
                                    aria-describedby="ai-prompt-help"
                                ></textarea>
                                <p id="ai-prompt-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">Describe what you want the AI to accomplish</p>
                            </div>
                            <div class="grid grid-cols-1 sm:grid-cols-3 gap-4">
                                <div>
                                    <label for="model-hint" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Model Hint</label>
                                    <select id="model-hint" v-model="newTask.modelHint" class="form-input w-full">
                                        <option v-for="hint in modelHints" :key="hint" :value="hint">{{ hint }}</option>
                                    </select>
                                </div>
                                <div>
                                    <label for="max-cost" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Max Cost ($)</label>
                                    <input
                                        id="max-cost"
                                        v-model="newTask.maxCost"
                                        type="number"
                                        step="0.01"
                                        min="0"
                                        class="form-input w-full"
                                        placeholder="1.00"
                                    >
                                </div>
                                <div class="flex items-end">
                                    <label class="inline-flex items-center">
                                        <input v-model="newTask.requireLocal" type="checkbox" class="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500">
                                        <span class="ml-2 text-sm text-gray-700 dark:text-gray-300">Local Only</span>
                                    </label>
                                </div>
                            </div>
                        </div>
                        <div v-if="newTask.type === 'manual'">
                            <label for="manual-command" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">Command or Prompt</label>
                            <textarea
                                id="manual-command"
                                v-model="newTask.command"
                                rows="3"
                                class="form-input w-full"
                                placeholder="Command or AI prompt to execute manually..."
                                aria-describedby="manual-command-help"
                            ></textarea>
                            <p id="manual-command-help" class="mt-1 text-xs text-gray-500 dark:text-gray-400">This will only run when triggered manually</p>
                        </div>
                        <div v-if="newTask.type === 'dependency'" class="space-y-4">
                            <div>
                                <label for="dependency-command" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                                    Command <span class="text-red-500">*</span>
                                </label>
                                <textarea
                                    id="dependency-command"
                                    v-model="newTask.command"
                                    rows="3"
                                    required
                                    class="form-input w-full font-mono text-sm"
                                    placeholder="echo 'Dependency task completed'"
                                ></textarea>
                            </div>
                            <fieldset>
                                <legend class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">Dependencies</legend>
                                <div class="max-h-32 overflow-y-auto border border-gray-300 dark:border-gray-600 rounded-lg p-3 space-y-2 bg-gray-50 dark:bg-gray-700">
                                    <div v-if="availableDependencies.length === 0" class="text-sm text-gray-500 dark:text-gray-400 text-center py-2">
                                        No tasks available for dependencies
                                    </div>
                                    <label v-for="task in availableDependencies" :key="task.id" class="flex items-center cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-600 rounded p-1">
                                        <input
                                            type="checkbox"
                                            :value="task.id"
                                            v-model="newTask.dependsOn"
                                            class="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500 mr-3"
                                        >
                                        <span class="text-sm text-gray-700 dark:text-gray-300">{{ task.name }}</span>
                                    </label>
                                </div>
                            </fieldset>
                        </div>
                        <!-- Enable Task -->
                        <div class="flex items-center">
                            <input id="enable-task" v-model="newTask.enabled" type="checkbox" class="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500">
                            <label for="enable-task" class="ml-2 text-sm text-gray-700 dark:text-gray-300">Enable task immediately after creation</label>
                        </div>
                    </div>
                    <!-- Modal Actions -->
                    <div class="flex items-center justify-end space-x-3 mt-8 pt-6 border-t border-gray-200 dark:border-gray-700">
                        <button
                            type="button"
                            @click="showCreateTask = false; resetNewTask()"
                            class="px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700 focus:ring-2 focus:ring-gray-500 focus:ring-offset-2 transition-all"
                        >
                            Cancel
                        </button>
                        <button
                            type="submit"
                            :disabled="!newTask.name"
                            class="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-all"
                        >
                            Create Task
                        </button>
                    </div>
                </form>
            </div>
        </div>
    </div>
`
};