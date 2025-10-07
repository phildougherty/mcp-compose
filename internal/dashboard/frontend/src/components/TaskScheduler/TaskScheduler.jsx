/**
 * TaskScheduler Component
 * Main task scheduler interface with CRUD operations, filtering, and stats
 */

import React, { useEffect, useState } from 'react';
import { Button, SearchInput, Select, Card, Spinner } from '../shared';
import { useToast } from '../../hooks';
import useTaskStore from '../../store/taskStore';
import * as taskApi from '../../api/tasks';
import * as chatApi from '../../api/chat';
import TaskStats from './TaskStats';
import TaskList from './TaskList';
import TaskForm from './TaskForm';
import TaskOutput from './TaskOutput';
import { getTaskTypeConfig } from './constants';

export default function TaskScheduler({ onNavigateToChat }) {
  const {
    tasks,
    taskRuns,
    loading,
    error,
    searchTerm,
    filterType,
    filterStatus,
    sortBy,
    autoRefresh,
    showRunOutput,
    setTasks,
    setTaskRuns,
    setLoading,
    setError,
    setSearchTerm,
    setFilterType,
    setFilterStatus,
    setSortBy,
    setAutoRefresh,
    showRunOutputModal,
    closeRunOutputModal,
    getTaskGroups,
    getTaskStats,
    getUniqueTaskTypes,
  } = useTaskStore();

  const { success, error: showError } = useToast();
  const [showCreateTask, setShowCreateTask] = useState(false);
  const [refreshIntervalId, setRefreshIntervalId] = useState(null);

  const taskGroups = getTaskGroups();
  const taskStats = getTaskStats();
  const uniqueTaskTypes = getUniqueTaskTypes();

  useEffect(() => {
    loadTasks();
  }, []);

  useEffect(() => {
    if (refreshIntervalId) {
      clearInterval(refreshIntervalId);
    }

    if (autoRefresh) {
      const id = setInterval(() => {
        loadTasks();
      }, 30000);
      setRefreshIntervalId(id);
    }

    return () => {
      if (refreshIntervalId) {
        clearInterval(refreshIntervalId);
      }
    };
  }, [autoRefresh]);

  const loadTasks = async () => {
    setLoading(true);
    setError(null);

    try {
      const [tasksData, runsData] = await Promise.all([
        taskApi.getTasks(),
        taskApi.getTaskHistory().catch(() => []),
      ]);

      setTasks(Array.isArray(tasksData) ? tasksData : []);
      setTaskRuns(Array.isArray(runsData) ? runsData : []);
    } catch (err) {
      const errorMessage = `Failed to load tasks: ${err.message}`;
      setError(errorMessage);
      showError(errorMessage);
    } finally {
      setLoading(false);
    }
  };

  const handleCreateTask = async (endpoint, taskData) => {
    try {
      await taskApi.createTask(taskData);
      setShowCreateTask(false);
      await loadTasks();
      success('Task created successfully');
    } catch (err) {
      showError(`Failed to create task: ${err.message}`);
    }
  };

  const handleDeleteTask = async (taskId) => {
    if (!confirm('Are you sure you want to delete this task? This action cannot be undone.')) return;

    try {
      await taskApi.deleteTask(taskId);
      await loadTasks();
      success('Task deleted successfully');
    } catch (err) {
      showError(`Failed to delete task: ${err.message}`);
    }
  };

  const handleToggleTask = async (taskId) => {
    const task = tasks.find((t) => t.id === taskId);
    if (!task) return;

    try {
      if (task.enabled) {
        await taskApi.disableTask(taskId);
      } else {
        await taskApi.enableTask(taskId);
      }
      await loadTasks();
      success(`Task ${task.enabled ? 'disabled' : 'enabled'} successfully`);
    } catch (err) {
      showError(`Failed to toggle task: ${err.message}`);
    }
  };

  const handleRunTask = async (taskId) => {
    const task = tasks.find((t) => t.id === taskId);
    if (!task) return;
    if (!confirm(`Run task "${task.name}" now?`)) return;

    try {
      await taskApi.executeTask(taskId);
      success('Task execution started');
      setTimeout(() => loadTasks(), 2000);
    } catch (err) {
      showError(`Failed to run task: ${err.message}`);
    }
  };

  const handleViewTaskOutput = async (taskId, runId = null) => {
    try {
      const output = await taskApi.getTaskOutput(taskId, runId);
      const outputKey = runId ? `${taskId}-${runId}` : taskId;
      showRunOutputModal(outputKey, {
        taskId,
        runId,
        output: typeof output === 'string' ? output : JSON.stringify(output, null, 2),
        timestamp: new Date().toISOString(),
      });
    } catch (err) {
      showError(`Failed to get task output: ${err.message}`);
    }
  };

  const handleCreateChat = async (task) => {
    try {
      const provider = task.providerHint || 'openrouter';
      const model = task.modelHint || 'anthropic/claude-sonnet-4.5';
      const mcpServers = task.mcpServers || [];

      const sessionData = {
        title: task.name,
        provider,
        model,
      };

      const session = await chatApi.createChatSession(sessionData);

      const updates = {
        title: task.name,
        mcp_servers: mcpServers,
      };

      if (task.id) {
        const metadata = session.metadata || {};
        metadata.task_id = task.id;
        metadata.task_name = task.name;
        metadata.task_type = task.type;
        metadata.task_schedule = task.schedule;
        metadata.task_description = task.description;
        if (task.prompt) {
          metadata.task_prompt = task.prompt;
        }
        if (task.command) {
          metadata.task_command = task.command;
        }
        updates.metadata = metadata;
      }

      await chatApi.updateChatSession(session.id, updates);

      if (task.id) {
        await taskApi.updateTask(task.id, {
          chat_session_id: session.id,
          output_to_chat: true,
          inherit_session_context: task.type === 'ai',
        });
      }

      success(`Chat session created and linked to "${task.name}"`);

      if (onNavigateToChat) {
        onNavigateToChat(session.id);
      }
    } catch (err) {
      showError(`Failed to create chat session: ${err.message}`);
    }
  };

  return (
    <div className="space-y-4 animate-fade-in max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
        <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between space-y-4 lg:space-y-0">
          <div className="flex items-center space-x-3">
            <div className="flex-shrink-0">
              <div className="w-10 h-10 bg-gradient-to-br from-purple-500 to-purple-600 rounded-lg flex items-center justify-center shadow-md">
                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              </div>
            </div>
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Task Scheduler</h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Manage scheduled AI tasks, shell commands, and automation workflows
              </p>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row space-y-2 sm:space-y-0 sm:space-x-3">
            <Button variant="primary" onClick={() => setShowCreateTask(true)}>
              <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
                  clipRule="evenodd"
                />
              </svg>
              Create Task
            </Button>
            <Button variant="secondary" onClick={loadTasks} disabled={loading}>
              <svg
                className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`}
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z"
                  clipRule="evenodd"
                />
              </svg>
              Refresh
            </Button>
            <label className="inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={(e) => setAutoRefresh(e.target.checked)}
                className="form-checkbox h-4 w-4 text-blue-600 rounded focus:ring-blue-500 focus:ring-offset-2"
              />
              <span className="ml-2 text-sm text-gray-700 dark:text-gray-300">Auto-refresh</span>
            </label>
          </div>
        </div>

        <div className="mt-6 space-y-4">
          <div className="flex flex-col lg:flex-row space-y-3 lg:space-y-0 lg:space-x-4">
            <div className="flex-1">
              <SearchInput
                value={searchTerm}
                onChange={(value) => setSearchTerm(value)}
                placeholder="Search tasks by name, description, command, or prompt..."
              />
            </div>
            <div className="flex flex-col sm:flex-row sm:space-x-4 space-y-3 sm:space-y-0">
              <div className="sm:w-40">
                <Select value={filterType} onChange={(e) => setFilterType(e.target.value)}>
                  <option value="all">All Types</option>
                  {uniqueTaskTypes.map((type) => (
                    <option key={type} value={type}>
                      {getTaskTypeConfig(type).label}
                    </option>
                  ))}
                </Select>
              </div>
              <div className="sm:w-32">
                <Select value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
                  <option value="all">All Status</option>
                  <option value="enabled">Enabled</option>
                  <option value="disabled">Disabled</option>
                </Select>
              </div>
              <div className="sm:w-40">
                <Select value={sortBy} onChange={(e) => setSortBy(e.target.value)}>
                  <option value="name">Sort by Name</option>
                  <option value="type">Sort by Type</option>
                  <option value="status">Sort by Status</option>
                  <option value="schedule">Sort by Schedule</option>
                  <option value="lastRun">Sort by Last Run</option>
                </Select>
              </div>
            </div>
          </div>
        </div>
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
          <div className="flex items-start">
            <svg className="h-5 w-5 text-red-400 mt-0.5 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                clipRule="evenodd"
              />
            </svg>
            <div className="ml-3 flex-1">
              <div className="text-sm text-red-800 dark:text-red-200">{error}</div>
              <Button
                variant="ghost"
                size="xs"
                onClick={() => setError(null)}
                className="mt-2 !text-red-600 hover:!text-red-800 dark:!text-red-400 dark:hover:!text-red-200 !px-0 underline"
              >
                Dismiss error
              </Button>
            </div>
          </div>
        </div>
      )}

      <TaskStats stats={taskStats} />

      {loading && tasks.length === 0 ? (
        <Card className="p-8 text-center">
          <Spinner size="large" />
          <p className="text-lg font-medium text-gray-900 dark:text-white mt-4">Loading tasks...</p>
          <p className="text-sm text-gray-500 dark:text-gray-400">Fetching task data from scheduler</p>
        </Card>
      ) : (
        <TaskList
          groups={taskGroups}
          onRun={handleRunTask}
          onToggle={handleToggleTask}
          onDelete={handleDeleteTask}
          onViewOutput={handleViewTaskOutput}
          onViewRunOutput={(taskId, runId) => handleViewTaskOutput(taskId, runId)}
          onCreateTask={() => setShowCreateTask(true)}
          onCreateChat={handleCreateChat}
        />
      )}

      {showCreateTask && (
        <TaskForm
          isOpen={showCreateTask}
          onClose={() => setShowCreateTask(false)}
          onSubmit={handleCreateTask}
        />
      )}

      {Object.entries(showRunOutput).map(([outputKey, output]) => (
        <TaskOutput key={outputKey} output={output} onClose={() => closeRunOutputModal(outputKey)} />
      ))}
    </div>
  );
}
