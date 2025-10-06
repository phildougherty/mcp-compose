/**
 * Chat API Client
 * Handles chat sessions, messages, and streaming
 */

import apiClient from './client.js';

/**
 * Get all chat sessions
 * @returns {Promise<Array>} List of chat sessions
 */
export async function getChatSessions() {
  return apiClient.get('/chat/sessions');
}

/**
 * Get a specific chat session
 * @param {string} sessionId - Session ID
 * @returns {Promise<Object>} Chat session details
 */
export async function getChatSession(sessionId) {
  return apiClient.get(`/chat/sessions/${sessionId}`);
}

/**
 * Create a new chat session
 * @param {Object} sessionData - Session data
 * @param {string} sessionData.name - Session name
 * @param {string} sessionData.provider - AI provider (openai, anthropic, ollama, openrouter)
 * @param {string} sessionData.model - Model name
 * @param {Array<string>} sessionData.mcpServers - MCP servers to connect
 * @returns {Promise<Object>} Created session
 */
export async function createChatSession(sessionData) {
  return apiClient.post('/chat/sessions', sessionData);
}

/**
 * Update a chat session
 * @param {string} sessionId - Session ID
 * @param {Object} updates - Session updates
 * @returns {Promise<Object>} Updated session
 */
export async function updateChatSession(sessionId, updates) {
  return apiClient.patch(`/chat/sessions/${sessionId}`, updates);
}

/**
 * Delete a chat session
 * @param {string} sessionId - Session ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteChatSession(sessionId) {
  return apiClient.delete(`/chat/sessions/${sessionId}`);
}

/**
 * Get messages for a chat session
 * @param {string} sessionId - Session ID
 * @param {number} limit - Number of messages to retrieve
 * @param {number} offset - Offset for pagination
 * @returns {Promise<Array>} List of messages
 */
export async function getChatMessages(sessionId, limit = 50, offset = 0) {
  return apiClient.get(`/chat/sessions/${sessionId}/messages?limit=${limit}&offset=${offset}`);
}

/**
 * Send a message in a chat session
 * @param {string} sessionId - Session ID
 * @param {Object} message - Message data
 * @param {string} message.content - Message content
 * @param {string} message.role - Message role (user, assistant, system)
 * @returns {Promise<Object>} Sent message
 */
export async function sendChatMessage(sessionId, message) {
  return apiClient.post(`/chat/sessions/${sessionId}/messages`, message);
}

/**
 * Get available AI providers
 * @returns {Promise<Array>} List of available providers
 */
export async function getChatProviders() {
  return apiClient.get('/chat/providers');
}

/**
 * Get available MCP servers for chat
 * @returns {Promise<Array>} List of MCP servers
 */
export async function getMCPServers() {
  return apiClient.get('/chat/mcp-servers');
}

/**
 * Get system prompt for a session
 * @param {string} sessionId - Session ID
 * @returns {Promise<Object>} System prompt details
 */
export async function getSystemPrompt(sessionId) {
  return apiClient.get(`/chat/sessions/${sessionId}/system-prompt`);
}

/**
 * Update system prompt for a session
 * @param {string} sessionId - Session ID
 * @param {string} systemPrompt - New system prompt content
 * @returns {Promise<Object>} Update result
 */
export async function updateSystemPrompt(sessionId, systemPrompt) {
  return apiClient.put(`/chat/sessions/${sessionId}/system-prompt`, { system_prompt: systemPrompt });
}

/**
 * Create WebSocket connection for chat streaming
 * @param {string} sessionId - Session ID
 * @returns {WebSocket} WebSocket connection
 */
export function createChatWebSocket(sessionId) {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsURL = `${protocol}//${window.location.host}/ws/chat/${sessionId}`;

  return new WebSocket(wsURL);
}

/**
 * Get active tasks for a chat session
 * @param {string} sessionId - Session ID
 * @returns {Promise<Array>} List of active tasks/agents for this session
 */
export async function getSessionTasks(sessionId) {
  return apiClient.get(`/chat/sessions/${sessionId}/tasks`);
}

export default {
  getChatSessions,
  getChatSession,
  createChatSession,
  updateChatSession,
  deleteChatSession,
  getChatMessages,
  sendChatMessage,
  getChatProviders,
  getMCPServers,
  getSystemPrompt,
  updateSystemPrompt,
  createChatWebSocket,
  getSessionTasks,
};
