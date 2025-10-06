/**
 * Task Scheduler API Client
 * Handles task CRUD, execution, and history
 */

import apiClient from './client.js';

/**
 * Get all tasks
 * @returns {Promise<Array>} List of tasks
 */
export async function getTasks() {
  return apiClient.get('/task-scheduler/tasks');
}

/**
 * Get a specific task
 * @param {string} taskId - Task ID
 * @returns {Promise<Object>} Task details
 */
export async function getTask(taskId) {
  return apiClient.get(`/task-scheduler/tasks/${taskId}`);
}

/**
 * Create a new task
 * @param {Object} taskData - Task data
 * @param {string} taskData.name - Task name
 * @param {string} taskData.type - Task type (shell, ai, manual, dependency, watcher)
 * @param {string} taskData.schedule - Cron schedule (optional)
 * @param {string} taskData.command - Shell command (for shell tasks)
 * @param {string} taskData.prompt - AI prompt (for AI tasks)
 * @param {boolean} taskData.enabled - Whether task is enabled
 * @returns {Promise<Object>} Created task
 */
export async function createTask(taskData) {
  return apiClient.post('/task-scheduler/tasks', taskData);
}

/**
 * Update a task
 * @param {string} taskId - Task ID
 * @param {Object} updates - Task updates
 * @returns {Promise<Object>} Updated task
 */
export async function updateTask(taskId, updates) {
  return apiClient.put(`/task-scheduler/tasks/${taskId}`, updates);
}

/**
 * Delete a task
 * @param {string} taskId - Task ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteTask(taskId) {
  return apiClient.delete(`/task-scheduler/tasks/${taskId}`);
}

/**
 * Execute a task manually
 * @param {string} taskId - Task ID
 * @returns {Promise<Object>} Execution result
 */
export async function executeTask(taskId) {
  return apiClient.post(`/task-scheduler/tasks/${taskId}/run`);
}

/**
 * Enable a task
 * @param {string} taskId - Task ID
 * @returns {Promise<Object>} Operation result
 */
export async function enableTask(taskId) {
  return apiClient.post(`/task-scheduler/tasks/${taskId}/enable`);
}

/**
 * Disable a task
 * @param {string} taskId - Task ID
 * @returns {Promise<Object>} Operation result
 */
export async function disableTask(taskId) {
  return apiClient.post(`/task-scheduler/tasks/${taskId}/disable`);
}

/**
 * Get all task runs status
 * @returns {Promise<Array>} All task runs
 */
export async function getTaskRuns() {
  return apiClient.get('/task-scheduler/runs/status');
}

/**
 * Get task execution history
 * @param {string} taskId - Task ID (optional)
 * @param {number} limit - Number of executions to retrieve
 * @returns {Promise<Array>} Task execution history
 */
export async function getTaskHistory(taskId = null, limit = 50) {
  if (taskId) {
    return apiClient.get(`/task-scheduler/tasks/${taskId}/history?limit=${limit}`);
  }
  return getTaskRuns();
}

/**
 * Get task output
 * @param {string} taskId - Task ID
 * @param {string} runId - Run ID (optional)
 * @returns {Promise<Object>} Task execution output
 */
export async function getTaskOutput(taskId, runId = null) {
  if (runId) {
    return apiClient.get(`/task-scheduler/tasks/${taskId}/runs/${runId}/output`);
  }
  return apiClient.get(`/task-scheduler/tasks/${taskId}/output`);
}

/**
 * Get task statistics
 * @returns {Promise<Object>} Task statistics
 */
export async function getTaskStats() {
  return apiClient.get('/task-scheduler/stats');
}

/**
 * Get task scheduler health status
 * @returns {Promise<Object>} Health status
 */
export async function getTaskSchedulerHealth() {
  return apiClient.get('/task-scheduler/health');
}

export default {
  getTasks,
  getTask,
  createTask,
  updateTask,
  deleteTask,
  executeTask,
  enableTask,
  disableTask,
  getTaskRuns,
  getTaskHistory,
  getTaskOutput,
  getTaskStats,
  getTaskSchedulerHealth,
};
