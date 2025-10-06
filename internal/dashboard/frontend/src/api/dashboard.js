/**
 * Dashboard API Client
 * Handles server status, metrics, and actions (start/stop/restart)
 */

import apiClient from './client.js';

/**
 * Get all servers status
 * @returns {Promise<Object>} Server list with status
 */
export async function getServers() {
  return apiClient.get('/servers');
}

/**
 * Get dashboard status
 * @returns {Promise<Object>} Dashboard status including proxy info
 */
export async function getStatus() {
  return apiClient.get('/status');
}

/**
 * Get active connections
 * @returns {Promise<Object>} Active connections data
 */
export async function getConnections() {
  return apiClient.get('/connections');
}

/**
 * Get server metrics
 * @returns {Promise<Object>} Server metrics data
 */
export async function getMetrics() {
  return apiClient.get('/metrics');
}

/**
 * Start a server
 * @param {string} serverName - Name of the server to start
 * @returns {Promise<Object>} Operation result
 */
export async function startServer(serverName) {
  return apiClient.post('/servers/start', { server: serverName });
}

/**
 * Stop a server
 * @param {string} serverName - Name of the server to stop
 * @returns {Promise<Object>} Operation result
 */
export async function stopServer(serverName) {
  return apiClient.post('/servers/stop', { server: serverName });
}

/**
 * Restart a server
 * @param {string} serverName - Name of the server to restart
 * @returns {Promise<Object>} Operation result
 */
export async function restartServer(serverName) {
  return apiClient.post('/servers/restart', { server: serverName });
}

/**
 * Reload proxy configuration
 * @returns {Promise<Object>} Operation result
 */
export async function reloadProxy() {
  return apiClient.post('/proxy/reload');
}

/**
 * Get server documentation
 * @param {string} serverName - Name of the server
 * @returns {Promise<Object>} Server documentation
 */
export async function getServerDocs(serverName) {
  return apiClient.get(`/server-docs/${serverName}`);
}

/**
 * Get server OpenAPI specification
 * @param {string} serverName - Name of the server
 * @returns {Promise<Object>} OpenAPI specification
 */
export async function getServerOpenAPI(serverName) {
  return apiClient.get(`/server-openapi/${serverName}`);
}

/**
 * Make direct server request
 * @param {string} serverName - Name of the server
 * @param {Object} requestData - Request payload
 * @returns {Promise<Object>} Server response
 */
export async function serverDirectRequest(serverName, requestData) {
  return apiClient.post(`/server-direct/${serverName}`, requestData);
}

/**
 * Get server logs
 * @param {string} serverName - Name of the server
 * @param {number} lines - Number of lines to retrieve
 * @returns {Promise<Object>} Server logs
 */
export async function getServerLogs(serverName, lines = 100) {
  return apiClient.get(`/server-logs/${serverName}?lines=${lines}`);
}

/**
 * Get container information
 * @param {string} containerName - Name of the container
 * @returns {Promise<Object>} Container information
 */
export async function getContainerInfo(containerName) {
  return apiClient.get(`/containers/${containerName}`);
}

export default {
  getServers,
  getStatus,
  getConnections,
  getMetrics,
  startServer,
  stopServer,
  restartServer,
  reloadProxy,
  getServerDocs,
  getServerOpenAPI,
  serverDirectRequest,
  getServerLogs,
  getContainerInfo,
};
