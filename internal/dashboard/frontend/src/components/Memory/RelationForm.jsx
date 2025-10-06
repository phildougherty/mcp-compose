import React from 'react';
import { Modal, Button, Input, Select } from '../shared';
import useMemoryStore from '../../store/memoryStore';
import { useToast } from '../../hooks';
import memoryApi from '../../api/memory';

const RelationForm = () => {
  const {
    ui,
    entities,
    newRelation,
    loading,
    setShowCreateRelation,
    setNewRelation,
    resetNewRelation,
    setLoading,
    setEntities,
    setRelations,
    calculateStats,
  } = useMemoryStore();

  const { success, error, warning } = useToast();

  const handleSubmit = async (e) => {
    e.preventDefault();

    if (!newRelation.from || !newRelation.to || !newRelation.type) {
      warning('All relation fields are required');
      return;
    }

    setLoading('operations', true);
    try {
      const relationData = {
        fromEntityId: newRelation.from,
        toEntityId: newRelation.to,
        type: newRelation.type,
      };

      await memoryApi.createRelationship(relationData);

      const graph = await memoryApi.getMemoryStats();
      setEntities(graph.entities || []);
      setRelations(graph.relations || []);
      calculateStats();

      setShowCreateRelation(false);
      resetNewRelation();
      success('Relationship created successfully');
    } catch (err) {
      error(`Failed to create relationship: ${err.message}`);
    } finally {
      setLoading('operations', false);
    }
  };

  const handleClose = () => {
    setShowCreateRelation(false);
    resetNewRelation();
  };

  return (
    <Modal
      isOpen={ui.showCreateRelation}
      onClose={handleClose}
      title="Create New Relationship"
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            From Entity <span className="text-red-500">*</span>
          </label>
          <Select
            value={newRelation.from}
            onChange={(value) => setNewRelation({ from: value })}
            required
          >
            <option value="">Select source entity</option>
            {entities.map((entity) => (
              <option key={entity.name} value={entity.name}>
                {entity.name} ({entity.entityType})
              </option>
            ))}
          </Select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Relationship Type <span className="text-red-500">*</span>
          </label>
          <Input
            value={newRelation.type}
            onChange={(e) => setNewRelation({ type: e.target.value })}
            placeholder="e.g., works for, lives in, created by"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            To Entity <span className="text-red-500">*</span>
          </label>
          <Select
            value={newRelation.to}
            onChange={(value) => setNewRelation({ to: value })}
            required
          >
            <option value="">Select target entity</option>
            {entities.map((entity) => (
              <option key={entity.name} value={entity.name}>
                {entity.name} ({entity.entityType})
              </option>
            ))}
          </Select>
        </div>

        <div className="flex justify-end space-x-3 pt-4">
          <Button type="button" variant="ghost" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={
              !newRelation.from || !newRelation.to || !newRelation.type || loading.operations
            }
            loading={loading.operations}
          >
            Create Relationship
          </Button>
        </div>
      </form>
    </Modal>
  );
};

export default RelationForm;
