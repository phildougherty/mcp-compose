/**
 * MCP Inspector API Client
 * Handles MCP protocol inspection and testing
 */

import apiClient from './client.js';

/**
 * Connect to an MCP server for inspection
 * @param {string} serverName - Name of the MCP server
 * @returns {Promise<Object>} Connection result
 */
export async function inspectorConnect(serverName) {
  return apiClient.post('/inspector/connect', { server: serverName });
}

/**
 * Send MCP request to connected server
 * @param {Object} request - MCP JSON-RPC 2.0 request
 * @param {string} request.jsonrpc - JSON-RPC version (2.0)
 * @param {string} request.method - MCP method name
 * @param {Object} request.params - Method parameters
 * @param {string|number} request.id - Request ID
 * @returns {Promise<Object>} MCP response
 */
export async function inspectorRequest(request) {
  return apiClient.post('/inspector/request', request);
}

/**
 * Disconnect from MCP server
 * @returns {Promise<Object>} Disconnection result
 */
export async function inspectorDisconnect() {
  return apiClient.post('/inspector/disconnect');
}

/**
 * Get available MCP request templates
 * @returns {Object} Request templates
 */
export function getRequestTemplates() {
  return {
    initialize: {
      jsonrpc: '2.0',
      method: 'initialize',
      params: {
        protocolVersion: '2024-11-05',
        capabilities: {
          roots: { listChanged: true },
          sampling: {},
        },
        clientInfo: {
          name: 'mcp-compose-inspector',
          version: '1.0.0',
        },
      },
      id: 1,
    },
    listTools: {
      jsonrpc: '2.0',
      method: 'tools/list',
      params: {},
      id: 2,
    },
    listResources: {
      jsonrpc: '2.0',
      method: 'resources/list',
      params: {},
      id: 3,
    },
    listPrompts: {
      jsonrpc: '2.0',
      method: 'prompts/list',
      params: {},
      id: 4,
    },
    callTool: {
      jsonrpc: '2.0',
      method: 'tools/call',
      params: {
        name: '',
        arguments: {},
      },
      id: 5,
    },
    readResource: {
      jsonrpc: '2.0',
      method: 'resources/read',
      params: {
        uri: '',
      },
      id: 6,
    },
    getPrompt: {
      jsonrpc: '2.0',
      method: 'prompts/get',
      params: {
        name: '',
        arguments: {},
      },
      id: 7,
    },
  };
}

/**
 * Validate MCP request
 * @param {Object} request - MCP request to validate
 * @returns {Object} Validation result
 */
export function validateMCPRequest(request) {
  const errors = [];

  if (!request.jsonrpc || request.jsonrpc !== '2.0') {
    errors.push('jsonrpc must be "2.0"');
  }

  if (!request.method || typeof request.method !== 'string') {
    errors.push('method must be a non-empty string');
  }

  if (!request.id && request.id !== 0) {
    errors.push('id must be provided');
  }

  if (request.params && typeof request.params !== 'object') {
    errors.push('params must be an object');
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

export default {
  inspectorConnect,
  inspectorRequest,
  inspectorDisconnect,
  getRequestTemplates,
  validateMCPRequest,
};
