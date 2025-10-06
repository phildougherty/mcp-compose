import React, { useState } from 'react';
import { Button, Input } from '../shared';
import { useToast } from '../../hooks';
import useMemoryStore from '../../store/memoryStore';
import memoryApi from '../../api/memory';

const ObservationList = ({ entity }) => {
  const [newObservation, setNewObservation] = useState('');
  const { setLoading, setEntities, setRelations, calculateStats } = useMemoryStore();
  const { success, error: showError } = useToast();

  const handleAddObservation = async () => {
    if (!newObservation.trim()) return;

    setLoading('operations', true);
    try {
      await memoryApi.addObservation(entity.name, {
        content: newObservation,
      });

      const graph = await memoryApi.getMemoryStats();
      setEntities(graph.entities || []);
      setRelations(graph.relations || []);
      calculateStats();

      setNewObservation('');
      success('Observation added successfully');
    } catch (err) {
      showError(`Failed to add observation: ${err.message}`);
    } finally {
      setLoading('operations', false);
    }
  };

  const handleDeleteObservation = async (observationId) => {
    setLoading('operations', true);
    try {
      await memoryApi.deleteObservation(entity.name, observationId);

      const graph = await memoryApi.getMemoryStats();
      setEntities(graph.entities || []);
      setRelations(graph.relations || []);
      calculateStats();

      success('Observation deleted successfully');
    } catch (err) {
      showError(`Failed to delete observation: ${err.message}`);
    } finally {
      setLoading('operations', false);
    }
  };

  return (
    <div>
      <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 flex items-center">
        <div className="w-6 h-6 bg-blue-500/20 dark:bg-blue-500/10 rounded-lg flex items-center justify-center mr-2">
          <svg
            className="w-4 h-4 text-blue-600 dark:text-blue-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
            />
          </svg>
        </div>
        Observations
      </h4>
      {(!entity.observations || entity.observations.length === 0) ? (
        <div className="text-sm text-gray-500 dark:text-gray-400 p-3 bg-gray-100 dark:bg-gray-700/30 rounded-lg border border-gray-200 dark:border-gray-600/30">
          No observations yet
        </div>
      ) : (
        <div className="space-y-2">
          {entity.observations.map((observation, index) => (
            <div
              key={index}
              className="group/obs flex items-start justify-between p-3 bg-white dark:bg-gray-900/60 border border-gray-200 dark:border-gray-700/50 rounded-lg hover:border-gray-300 dark:hover:border-gray-600 transition-all duration-150"
            >
              <span className="text-sm text-gray-700 dark:text-gray-300 flex-1 leading-relaxed">
                {observation}
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleDeleteObservation(index)}
                className="opacity-0 group-hover/obs:opacity-100 min-w-[44px] min-h-[44px] text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20 ml-3"
              >
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                    clipRule="evenodd"
                  />
                </svg>
              </Button>
            </div>
          ))}
        </div>
      )}

      <div className="mt-4">
        <div className="flex space-x-2">
          <Input
            value={newObservation}
            onChange={(e) => setNewObservation(e.target.value)}
            placeholder="Add new observation..."
            className="flex-1"
            onKeyPress={(e) => {
              if (e.key === 'Enter') {
                handleAddObservation();
              }
            }}
          />
          <Button
            variant="primary"
            size="md"
            onClick={handleAddObservation}
            disabled={!newObservation.trim()}
            className="min-w-[44px] min-h-[44px]"
          >
            <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 3a1 1 0 011 1v5h5a1 1 0 110 2h-5v5a1 1 0 11-2 0v-5H4a1 1 0 110-2h5V4a1 1 0 011-1z"
                clipRule="evenodd"
              />
            </svg>
          </Button>
        </div>
      </div>
    </div>
  );
};

export default ObservationList;
