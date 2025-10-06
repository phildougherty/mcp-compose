/**
 * OAuth API Client
 * Handles OAuth clients, tokens, and endpoints
 */

import apiClient from './client.js';

/**
 * Get OAuth server status
 * @returns {Promise<Object>} OAuth server status
 */
export async function getOAuthStatus() {
  return apiClient.get('/oauth/status');
}

/**
 * Get all OAuth clients
 * @returns {Promise<Array>} List of OAuth clients
 */
export async function getOAuthClients() {
  return apiClient.get('/oauth/clients');
}

/**
 * Get a specific OAuth client
 * @param {string} clientId - Client ID
 * @returns {Promise<Object>} OAuth client details
 */
export async function getOAuthClient(clientId) {
  return apiClient.get(`/oauth/clients/${clientId}`);
}

/**
 * Create a new OAuth client
 * @param {Object} clientData - Client data
 * @param {string} clientData.name - Client name
 * @param {Array<string>} clientData.redirect_uris - Redirect URIs
 * @param {string} clientData.client_type - Client type (public or confidential)
 * @param {Array<string>} clientData.grant_types - Grant types
 * @param {Array<string>} clientData.scopes - Allowed scopes
 * @returns {Promise<Object>} Created OAuth client
 */
export async function createOAuthClient(clientData) {
  return apiClient.post('/oauth/clients', clientData);
}

/**
 * Update an OAuth client
 * @param {string} clientId - Client ID
 * @param {Object} updates - Client updates
 * @returns {Promise<Object>} Updated OAuth client
 */
export async function updateOAuthClient(clientId, updates) {
  return apiClient.put(`/oauth/clients/${clientId}`, updates);
}

/**
 * Delete an OAuth client
 * @param {string} clientId - Client ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteOAuthClient(clientId) {
  return apiClient.delete(`/oauth/clients/${clientId}`);
}

/**
 * Get available OAuth scopes
 * @returns {Promise<Array>} List of available scopes
 */
export async function getOAuthScopes() {
  return apiClient.get('/oauth/scopes');
}

/**
 * Get OAuth endpoints information
 * @returns {Promise<Object>} OAuth endpoints
 */
export async function getOAuthEndpoints() {
  return apiClient.get('/oauth/endpoints');
}

/**
 * Test authorization code flow
 * @param {string} clientId - Client ID
 * @param {string} redirectUri - Redirect URI
 * @param {Array<string>} scopes - Requested scopes
 * @returns {Promise<Object>} Test result
 */
export async function testAuthorizationCodeFlow(clientId, redirectUri, scopes) {
  return apiClient.post('/oauth/test/authorization-code', {
    client_id: clientId,
    redirect_uri: redirectUri,
    scopes: scopes,
  });
}

/**
 * Test client credentials flow
 * @param {string} clientId - Client ID
 * @param {string} clientSecret - Client secret
 * @param {Array<string>} scopes - Requested scopes
 * @returns {Promise<Object>} Test result
 */
export async function testClientCredentialsFlow(clientId, clientSecret, scopes) {
  return apiClient.post('/oauth/test/client-credentials', {
    client_id: clientId,
    client_secret: clientSecret,
    scopes: scopes,
  });
}

/**
 * Register OAuth client (public endpoint)
 * @param {Object} clientData - Client registration data
 * @returns {Promise<Object>} Registered client
 */
export async function registerOAuthClient(clientData) {
  return apiClient.post('/oauth/register', clientData);
}

/**
 * Exchange authorization code for token
 * @param {Object} tokenRequest - Token request data
 * @param {string} tokenRequest.code - Authorization code
 * @param {string} tokenRequest.client_id - Client ID
 * @param {string} tokenRequest.client_secret - Client secret
 * @param {string} tokenRequest.redirect_uri - Redirect URI
 * @returns {Promise<Object>} Token response
 */
export async function exchangeToken(tokenRequest) {
  return apiClient.post('/oauth/token', tokenRequest);
}

/**
 * Revoke OAuth token
 * @param {string} token - Token to revoke
 * @param {string} tokenTypeHint - Token type hint (access_token or refresh_token)
 * @returns {Promise<Object>} Revocation result
 */
export async function revokeToken(token, tokenTypeHint) {
  return apiClient.post('/oauth/revoke', {
    token,
    token_type_hint: tokenTypeHint,
  });
}

export default {
  getOAuthStatus,
  getOAuthClients,
  getOAuthClient,
  createOAuthClient,
  updateOAuthClient,
  deleteOAuthClient,
  getOAuthScopes,
  getOAuthEndpoints,
  testAuthorizationCodeFlow,
  testClientCredentialsFlow,
  registerOAuthClient,
  exchangeToken,
  revokeToken,
};
