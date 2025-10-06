import React from 'react';
import { Modal, Button, Input } from '../shared';
import useMemoryStore from '../../store/memoryStore';
import { useToast } from '../../hooks';
import memoryApi from '../../api/memory';

const EntityForm = () => {
  const {
    ui,
    newEntity,
    loading,
    setShowCreateEntity,
    setNewEntity,
    resetNewEntity,
    addObservationField,
    removeObservationField,
    updateObservationField,
    setLoading,
    setEntities,
    setRelations,
    calculateStats,
  } = useMemoryStore();

  const { success, error, warning } = useToast();

  const handleSubmit = async (e) => {
    e.preventDefault();

    if (!newEntity.name || !newEntity.type) {
      warning('Entity name and type are required');
      return;
    }

    setLoading('operations', true);
    try {
      const entityData = {
        entities: [{
          name: newEntity.name,
          entityType: newEntity.type,
          observations: newEntity.observations.filter((obs) => obs.trim()),
        }]
      };

      await memoryApi.createEntity(entityData);

      const graph = await memoryApi.getMemoryStats();
      setEntities(graph.entities || []);
      setRelations(graph.relations || []);
      calculateStats();

      setShowCreateEntity(false);
      resetNewEntity();
      success('Entity created successfully');
    } catch (err) {
      error(`Failed to create entity: ${err.message}`);
    } finally {
      setLoading('operations', false);
    }
  };

  const handleClose = () => {
    setShowCreateEntity(false);
    resetNewEntity();
  };

  return (
    <Modal
      isOpen={ui.showCreateEntity}
      onClose={handleClose}
      title="Create New Entity"
      size="md"
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Name <span className="text-red-500">*</span>
          </label>
          <Input
            value={newEntity.name}
            onChange={(e) => setNewEntity({ name: e.target.value })}
            placeholder="Enter entity name"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Type <span className="text-red-500">*</span>
          </label>
          <Input
            value={newEntity.type}
            onChange={(e) => setNewEntity({ type: e.target.value })}
            placeholder="e.g., Person, Organization, Event, Concept"
            required
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Observations
          </label>
          <div className="space-y-2">
            {newEntity.observations.map((observation, index) => (
              <div key={index} className="flex space-x-2">
                <Input
                  value={observation}
                  onChange={(e) => updateObservationField(index, e.target.value)}
                  placeholder="Enter observation"
                  className="flex-1"
                />
                {newEntity.observations.length > 1 && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeObservationField(index)}
                    className="min-w-[44px] min-h-[44px] text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
                  >
                    <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                        clipRule="evenodd"
                      />
                    </svg>
                  </Button>
                )}
              </div>
            ))}
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={addObservationField}
              className="text-purple-600 dark:text-purple-400 hover:text-purple-800 dark:hover:text-purple-300"
            >
              <svg className="w-4 h-4 mr-1" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
                  clipRule="evenodd"
                />
              </svg>
              Add Observation
            </Button>
          </div>
        </div>

        <div className="flex justify-end space-x-3 pt-4">
          <Button type="button" variant="ghost" onClick={handleClose}>
            Cancel
          </Button>
          <Button
            type="submit"
            variant="primary"
            disabled={!newEntity.name || !newEntity.type || loading.operations}
            loading={loading.operations}
          >
            Create Entity
          </Button>
        </div>
      </form>
    </Modal>
  );
};

export default EntityForm;
