import React from 'react';
import useMemoryStore from '../../store/memoryStore';
import { Badge, Button } from '../shared';
import { useToast } from '../../hooks';
import memoryApi from '../../api/memory';
import ObservationList from './ObservationList';

const EntityCard = ({ entity }) => {
  const {
    ui,
    relations,
    toggleEntityExpansion,
    toggleEntitySelection,
    setLoading,
    setEntities,
    setRelations,
    getEntityRelations,
    calculateStats,
  } = useMemoryStore();

  const { success, error: showError } = useToast();

  const isExpanded = ui.expandedEntities.has(entity.name);
  const isSelected = ui.selectedItems.has(entity.name);
  const entityRelations = getEntityRelations(entity.name);

  const handleDelete = async () => {
    if (
      !window.confirm(
        `Are you sure you want to delete entity "${entity.name}"? This will also delete all its relationships.`
      )
    ) {
      return;
    }

    setLoading('operations', true);
    try {
      await memoryApi.deleteEntity(entity.name);

      const graph = await memoryApi.getMemoryStats();
      setEntities(graph.entities || []);
      setRelations(graph.relations || []);
      calculateStats();

      success('Entity deleted successfully');
    } catch (err) {
      showError(`Failed to delete entity: ${err.message}`);
    } finally {
      setLoading('operations', false);
    }
  };

  return (
    <div className="p-5 hover:bg-gray-50 dark:hover:bg-gray-800/40 transition-all duration-150 group">
      <div className="flex items-start space-x-4">
        <label className="flex-shrink-0 mt-1.5 cursor-pointer">
          <input
            type="checkbox"
            checked={isSelected}
            onChange={() => toggleEntitySelection(entity.name)}
            className="form-checkbox h-4 w-4 text-purple-600 rounded border-gray-300 dark:border-gray-600 focus:ring-purple-500 focus:ring-offset-gray-900 transition-colors"
          />
        </label>

        <div className="flex-1 min-w-0">
          <div className="flex items-start justify-between mb-3">
            <div className="flex-1 min-w-0">
              <div className="flex items-center space-x-3 mb-2">
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate">
                  {entity.name}
                </h3>
                <Badge variant="info" size="sm">
                  {entity.entityType}
                </Badge>
              </div>
              <div className="flex items-center flex-wrap gap-2">
                <Badge variant="default" size="sm" dot>
                  {entity.observations?.length || 0} observations
                </Badge>
                {entityRelations.length > 0 && (
                  <Badge variant="success" size="sm" dot>
                    {entityRelations.length} relationships
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex items-center space-x-1 ml-4 opacity-0 group-hover:opacity-100 transition-opacity duration-200">
              <Button
                variant="ghost"
                size="sm"
                onClick={() => toggleEntityExpansion(entity.name)}
                className="min-w-[44px] min-h-[44px]"
              >
                <svg
                  className={`w-5 h-5 transition-transform duration-200 ${
                    isExpanded ? 'rotate-180' : ''
                  }`}
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                    clipRule="evenodd"
                  />
                </svg>
              </Button>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleDelete}
                className="min-w-[44px] min-h-[44px] text-red-600 hover:text-red-700 dark:text-red-400 dark:hover:text-red-300 hover:bg-red-50 dark:hover:bg-red-900/20"
              >
                <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M9 2a1 1 0 00-2 0H5a2 2 0 00-2 2v1a1 1 0 001 1h10a1 1 0 001-1V4a2 2 0 00-2-2h-2zm3 4H6v9a2 2 0 002 2h2a2 2 0 002-2V6z"
                    clipRule="evenodd"
                  />
                </svg>
              </Button>
            </div>
          </div>

          {isExpanded && (
            <div className="space-y-4 mt-4 p-5 bg-gray-100 dark:bg-gray-800/60 border border-gray-200 dark:border-gray-700/50 rounded-xl animate-fade-in">
              <ObservationList entity={entity} />

              {entityRelations.length > 0 && (
                <div>
                  <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3 flex items-center">
                    <div className="w-6 h-6 bg-green-500/20 dark:bg-green-500/10 rounded-lg flex items-center justify-center mr-2">
                      <svg
                        className="w-4 h-4 text-green-600 dark:text-green-400"
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth="2"
                          d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
                        />
                      </svg>
                    </div>
                    Relationships
                  </h4>
                  <div className="space-y-2">
                    {entityRelations.map((relation) => (
                      <div
                        key={`${relation.from}-${relation.to}-${relation.relationType}`}
                        className="flex items-center p-3 bg-white dark:bg-gray-900/60 border border-gray-200 dark:border-gray-700/50 rounded-lg"
                      >
                        <span className="font-medium text-blue-600 dark:text-blue-400">
                          {relation.from}
                        </span>
                        <svg
                          className="w-4 h-4 mx-2 text-gray-500 dark:text-gray-400"
                          fill="currentColor"
                          viewBox="0 0 20 20"
                        >
                          <path
                            fillRule="evenodd"
                            d="M10.293 15.707a1 1 0 010-1.414L14.586 10l-4.293-4.293a1 1 0 111.414-1.414l5 5a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0z"
                            clipRule="evenodd"
                          />
                        </svg>
                        <Badge variant="default" size="sm" className="mx-2">
                          {relation.relationType}
                        </Badge>
                        <svg
                          className="w-4 h-4 mx-2 text-gray-500 dark:text-gray-400"
                          fill="currentColor"
                          viewBox="0 0 20 20"
                        >
                          <path
                            fillRule="evenodd"
                            d="M10.293 15.707a1 1 0 010-1.414L14.586 10l-4.293-4.293a1 1 0 111.414-1.414l5 5a1 1 0 010 1.414l-5 5a1 1 0 01-1.414 0z"
                            clipRule="evenodd"
                          />
                        </svg>
                        <span className="font-medium text-green-600 dark:text-green-400">
                          {relation.to}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

export default EntityCard;
