/**
 * Audit API Client
 * Handles audit logs and export
 */

import apiClient from './client.js';

/**
 * Get audit entries
 * @param {Object} params - Query parameters
 * @param {string} params.event_type - Filter by event type
 * @param {boolean} params.success - Filter by success status
 * @param {string} params.time_range - Time range (1h, 24h, 7d, 30d, all)
 * @param {string} params.search - Search term
 * @param {number} params.limit - Number of entries per page
 * @param {number} params.offset - Offset for pagination
 * @returns {Promise<Object>} Audit entries with pagination
 */
export async function getAuditEntries(params = {}) {
  const query = new URLSearchParams(params).toString();

  return apiClient.get(`/audit/entries${query ? `?${query}` : ''}`);
}

/**
 * Get audit statistics
 * @param {Object} params - Query parameters
 * @param {string} params.time_range - Time range (1h, 24h, 7d, 30d, all)
 * @returns {Promise<Object>} Audit statistics
 */
export async function getAuditStats(params = {}) {
  const defaultParams = {
    time_range: '24h',
    ...params,
  };
  const query = new URLSearchParams(defaultParams).toString();

  return apiClient.get(`/audit/stats?${query}`);
}

/**
 * Get event type distribution
 * @param {Object} params - Query parameters
 * @param {string} params.time_range - Time range (1h, 24h, 7d, 30d, all)
 * @returns {Promise<Object>} Event distribution data
 */
export async function getEventDistribution(params = {}) {
  const defaultParams = {
    time_range: '24h',
    ...params,
  };
  const query = new URLSearchParams(defaultParams).toString();

  return apiClient.get(`/audit/distribution?${query}`);
}

/**
 * Export audit logs to CSV
 * @param {Object} params - Query parameters for filtering
 * @returns {Promise<Blob>} CSV file blob
 */
export async function exportAuditLogs(params = {}) {
  const query = new URLSearchParams(params).toString();
  const endpoint = `/audit/export${query ? `?${query}` : ''}`;

  const response = await fetch(endpoint, {
    method: 'GET',
    headers: apiClient.buildHeaders(),
  });

  if (!response.ok) {
    await apiClient.handleError(response);
  }

  return await response.blob();
}

/**
 * Download audit logs CSV
 * @param {Object} params - Query parameters for filtering
 * @param {string} filename - Filename for download
 */
export async function downloadAuditLogs(params = {}, filename = 'audit-logs.csv') {
  const blob = await exportAuditLogs(params);
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
}

/**
 * Get audit event types
 * @returns {Promise<Array>} List of event types
 */
export async function getAuditEventTypes() {
  return apiClient.get('/audit/event-types');
}

export default {
  getAuditEntries,
  getAuditStats,
  getEventDistribution,
  exportAuditLogs,
  downloadAuditLogs,
  getAuditEventTypes,
};
