/**
 * Memory API Client
 * Handles entities, observations, and relationships
 */

import apiClient from './client.js';

/**
 * Get all entities
 * @param {Object} params - Query parameters
 * @param {number} params.limit - Number of entities per page
 * @param {number} params.offset - Offset for pagination
 * @param {string} params.type - Filter by entity type
 * @param {string} params.search - Search term
 * @returns {Promise<Object>} Entities list with pagination info
 */
export async function getEntities(params = {}) {
  const query = new URLSearchParams(params).toString();

  return apiClient.get(`/memory/entities${query ? `?${query}` : ''}`);
}

/**
 * Get a specific entity
 * @param {string} entityId - Entity ID
 * @returns {Promise<Object>} Entity details
 */
export async function getEntity(entityId) {
  return apiClient.get(`/memory/entities/${entityId}`);
}

/**
 * Create a new entity
 * @param {Object} entityData - Entity data
 * @param {string} entityData.name - Entity name
 * @param {string} entityData.type - Entity type
 * @param {Object} entityData.content - Entity content/metadata
 * @returns {Promise<Object>} Created entity
 */
export async function createEntity(entityData) {
  return apiClient.post('/memory/entities', entityData);
}

/**
 * Update an entity
 * @param {string} entityId - Entity ID
 * @param {Object} updates - Entity updates
 * @returns {Promise<Object>} Updated entity
 */
export async function updateEntity(entityId, updates) {
  return apiClient.put(`/memory/entities/${entityId}`, updates);
}

/**
 * Delete an entity
 * @param {string} entityId - Entity ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteEntity(entityId) {
  return apiClient.delete(`/memory/entities/${entityId}`);
}

/**
 * Bulk delete entities
 * @param {Array<string>} entityIds - Array of entity IDs
 * @returns {Promise<Object>} Deletion result
 */
export async function bulkDeleteEntities(entityIds) {
  return apiClient.post('/memory/entities/bulk-delete', { ids: entityIds });
}

/**
 * Get observations for an entity
 * @param {string} entityId - Entity ID
 * @returns {Promise<Array>} List of observations
 */
export async function getObservations(entityId) {
  return apiClient.get(`/memory/entities/${entityId}/observations`);
}

/**
 * Add an observation to an entity
 * @param {string} entityId - Entity ID
 * @param {Object} observationData - Observation data
 * @param {string} observationData.content - Observation content
 * @param {string} observationData.type - Observation type
 * @returns {Promise<Object>} Created observation
 */
export async function addObservation(entityId, observationData) {
  return apiClient.post(`/memory/entities/${entityId}/observations`, observationData);
}

/**
 * Delete an observation
 * @param {string} entityId - Entity ID
 * @param {string} observationId - Observation ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteObservation(entityId, observationId) {
  return apiClient.delete(`/memory/entities/${entityId}/observations/${observationId}`);
}

/**
 * Get relationships for an entity
 * @param {string} entityId - Entity ID
 * @returns {Promise<Array>} List of relationships
 */
export async function getRelationships(entityId) {
  return apiClient.get(`/memory/entities/${entityId}/relationships`);
}

/**
 * Create a relationship between entities
 * @param {Object} relationshipData - Relationship data
 * @param {string} relationshipData.fromEntityId - Source entity ID
 * @param {string} relationshipData.toEntityId - Target entity ID
 * @param {string} relationshipData.type - Relationship type
 * @param {Object} relationshipData.metadata - Additional metadata
 * @returns {Promise<Object>} Created relationship
 */
export async function createRelationship(relationshipData) {
  return apiClient.post('/memory/relationships', relationshipData);
}

/**
 * Delete a relationship
 * @param {string} relationshipId - Relationship ID
 * @returns {Promise<Object>} Deletion result
 */
export async function deleteRelationship(relationshipId) {
  return apiClient.delete(`/memory/relationships/${relationshipId}`);
}

/**
 * Get memory statistics
 * @returns {Promise<Object>} Memory statistics
 */
export async function getMemoryStats() {
  return apiClient.get('/memory/stats');
}

/**
 * Search entities and observations
 * @param {string} query - Search query
 * @param {Object} options - Search options
 * @returns {Promise<Object>} Search results
 */
export async function searchMemory(query, options = {}) {
  return apiClient.post('/memory/search', { query, ...options });
}

/**
 * Get entity types distribution
 * @returns {Promise<Object>} Entity types with counts
 */
export async function getEntityTypes() {
  return apiClient.get('/memory/entity-types');
}

export default {
  getEntities,
  getEntity,
  createEntity,
  updateEntity,
  deleteEntity,
  bulkDeleteEntities,
  getObservations,
  addObservation,
  deleteObservation,
  getRelationships,
  createRelationship,
  deleteRelationship,
  getMemoryStats,
  searchMemory,
  getEntityTypes,
};
