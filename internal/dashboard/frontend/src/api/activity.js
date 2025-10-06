/**
 * Activity API Client
 * Handles activity events and historical data
 */

import apiClient from './client.js';

/**
 * Get activity history
 * @param {Object} params - Query parameters
 * @param {number} params.hours - Number of hours of history to retrieve
 * @param {string} params.level - Filter by level (ERROR, WARN, INFO, DEBUG)
 * @param {string} params.type - Filter by type (request, connection, tool_call, error)
 * @param {number} params.limit - Number of events to retrieve
 * @returns {Promise<Array>} List of activity events
 */
export async function getActivityHistory(params = {}) {
  const defaultParams = {
    hours: 6,
    limit: 1000,
    ...params,
  };
  const query = new URLSearchParams(defaultParams).toString();

  return apiClient.get(`/activity/history?${query}`);
}

/**
 * Get activity statistics
 * @param {Object} params - Query parameters
 * @param {number} params.hours - Number of hours for statistics
 * @returns {Promise<Object>} Activity statistics
 */
export async function getActivityStats(params = {}) {
  const defaultParams = {
    hours: 24,
    ...params,
  };
  const query = new URLSearchParams(defaultParams).toString();

  return apiClient.get(`/activity/stats?${query}`);
}

/**
 * Send activity event (internal use)
 * @param {Object} eventData - Activity event data
 * @param {string} eventData.level - Event level (ERROR, WARN, INFO, DEBUG)
 * @param {string} eventData.type - Event type
 * @param {string} eventData.message - Event message
 * @param {Object} eventData.metadata - Additional metadata
 * @returns {Promise<Object>} Response
 */
export async function sendActivityEvent(eventData) {
  return apiClient.post('/activity', eventData);
}

/**
 * Create WebSocket connection for real-time activity
 * @returns {WebSocket} WebSocket connection
 */
export function createActivityWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsURL = `${protocol}//${window.location.host}/ws/activity`;

  return new WebSocket(wsURL);
}

export default {
  getActivityHistory,
  getActivityStats,
  sendActivityEvent,
  createActivityWebSocket,
};
