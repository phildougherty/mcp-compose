/**
 * API Client Index
 * Centralized exports for all API modules
 */

import apiClientInstance from './client.js';
import * as dashboardApiModule from './dashboard.js';
import * as chatApiModule from './chat.js';
import * as tasksApiModule from './tasks.js';
import * as memoryApiModule from './memory.js';
import * as activityApiModule from './activity.js';
import * as oauthApiModule from './oauth.js';
import * as auditApiModule from './audit.js';
import * as inspectorApiModule from './inspector.js';
import WebSocketManagerClass from './websocket.js';

export { default as apiClient, APIClient } from './client.js';
export * as dashboardApi from './dashboard.js';
export * as chatApi from './chat.js';
export * as tasksApi from './tasks.js';
export * as memoryApi from './memory.js';
export * as activityApi from './activity.js';
export * as oauthApi from './oauth.js';
export * as auditApi from './audit.js';
export * as inspectorApi from './inspector.js';
export { default as WebSocketManager, createWebSocketManager, createMetricsWebSocket, createLogsWebSocket } from './websocket.js';

export default {
  apiClient: apiClientInstance,
  dashboardApi: dashboardApiModule,
  chatApi: chatApiModule,
  tasksApi: tasksApiModule,
  memoryApi: memoryApiModule,
  activityApi: activityApiModule,
  oauthApi: oauthApiModule,
  auditApi: auditApiModule,
  inspectorApi: inspectorApiModule,
  WebSocketManager: WebSocketManagerClass,
};
