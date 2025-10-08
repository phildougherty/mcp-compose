import React, { useState } from 'react';
import { ReactFlow, Background, Controls, BackgroundVariant } from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import Button from '../shared/Button';
import clsx from 'clsx';

export default function WorkflowDeploymentPanel({ workflow, onDeploy, onCustomize, onCancel }) {
  const [parameters, setParameters] = useState(workflow.parameters || {});
  const [deploying, setDeploying] = useState(false);
  const [showPreview, setShowPreview] = useState(false);

  const handleParameterChange = (key, value) => {
    setParameters(prev => ({
      ...prev,
      [key]: value
    }));
  };

  const handleDeploy = async () => {
    setDeploying(true);

    try {
      await onDeploy({
        ...workflow,
        parameters
      });
    } catch (err) {
      console.error('Deployment failed:', err);
    } finally {
      setDeploying(false);
    }
  };

  const estimatedCost = workflow.estimated_cost || 0;
  const complexity = workflow.complexity || 'medium';

  const complexityColors = {
    low: 'bg-green-100 text-green-800 border-green-200',
    medium: 'bg-yellow-100 text-yellow-800 border-yellow-200',
    high: 'bg-red-100 text-red-800 border-red-200'
  };

  const nodes = workflow.nodes || [];
  const edges = workflow.edges || [];

  return (
    <div className="workflow-deployment-panel bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg shadow-lg p-6 my-4">
      <div className="flex items-start justify-between mb-4">
        <div className="flex-1">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
            {workflow.name || 'Workflow'}
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-400">
            {workflow.description || 'No description available'}
          </p>
        </div>
        <button
          onClick={onCancel}
          className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 p-1"
          title="Close"
        >
          <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div className="grid grid-cols-3 gap-4 mb-6">
        <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Nodes</div>
          <div className="text-2xl font-bold text-gray-900 dark:text-white">{nodes.length}</div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Est. Cost</div>
          <div className="text-2xl font-bold text-gray-900 dark:text-white">
            ${estimatedCost.toFixed(4)}
          </div>
        </div>
        <div className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4">
          <div className="text-sm text-gray-500 dark:text-gray-400 mb-1">Complexity</div>
          <div className={clsx(
            'inline-flex items-center px-3 py-1 rounded-full text-sm font-medium border',
            complexityColors[complexity]
          )}>
            {complexity.charAt(0).toUpperCase() + complexity.slice(1)}
          </div>
        </div>
      </div>

      {workflow.parameterSchema && Object.keys(workflow.parameterSchema).length > 0 && (
        <div className="mb-6">
          <h4 className="text-sm font-semibold text-gray-900 dark:text-white mb-3">
            Configuration Parameters
          </h4>
          <div className="space-y-3">
            {Object.entries(workflow.parameterSchema).map(([key, schema]) => (
              <div key={key}>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  {schema.label || key}
                  {schema.required && <span className="text-red-500 ml-1">*</span>}
                </label>
                {schema.type === 'select' ? (
                  <select
                    value={parameters[key] || ''}
                    onChange={(e) => handleParameterChange(key, e.target.value)}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  >
                    <option value="">Select {schema.label || key}</option>
                    {schema.options?.map(opt => (
                      <option key={opt} value={opt}>{opt}</option>
                    ))}
                  </select>
                ) : schema.type === 'textarea' ? (
                  <textarea
                    value={parameters[key] || ''}
                    onChange={(e) => handleParameterChange(key, e.target.value)}
                    placeholder={schema.placeholder || `Enter ${schema.label || key}`}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent resize-vertical"
                    rows={3}
                  />
                ) : (
                  <input
                    type={schema.type || 'text'}
                    value={parameters[key] || ''}
                    onChange={(e) => handleParameterChange(key, e.target.value)}
                    placeholder={schema.placeholder || `Enter ${schema.label || key}`}
                    className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-md bg-white dark:bg-gray-700 text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
                  />
                )}
                {schema.description && (
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    {schema.description}
                  </p>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="mb-6">
        <div className="flex items-center justify-between mb-2">
          <h4 className="text-sm font-semibold text-gray-900 dark:text-white">
            Workflow Preview
          </h4>
          <button
            onClick={() => setShowPreview(!showPreview)}
            className="text-sm text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            {showPreview ? 'Hide' : 'Show'} Diagram
          </button>
        </div>

        {showPreview && (
          <div className="border border-gray-200 dark:border-gray-700 rounded-lg overflow-hidden bg-gray-50 dark:bg-gray-900" style={{ height: '300px' }}>
            <ReactFlow
              nodes={nodes.map(node => ({
                ...node,
                data: { ...node.data, label: node.data.label || node.type }
              }))}
              edges={edges}
              fitView
              nodesDraggable={false}
              nodesConnectable={false}
              elementsSelectable={false}
              zoomOnScroll={false}
              panOnDrag={false}
            >
              <Background variant={BackgroundVariant.Dots} />
              <Controls showInteractive={false} />
            </ReactFlow>
          </div>
        )}
      </div>

      {workflow.required_servers && workflow.required_servers.length > 0 && (
        <div className="mb-6 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-lg p-4">
          <h4 className="text-sm font-semibold text-blue-900 dark:text-blue-200 mb-2">
            Required MCP Servers
          </h4>
          <div className="flex flex-wrap gap-2">
            {workflow.required_servers.map(server => (
              <span
                key={server}
                className="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-blue-100 dark:bg-blue-900/40 text-blue-800 dark:text-blue-200 border border-blue-200 dark:border-blue-700"
              >
                {server}
              </span>
            ))}
          </div>
        </div>
      )}

      <div className="flex items-center gap-3">
        <Button
          onClick={handleDeploy}
          variant="primary"
          disabled={deploying}
          className="flex-1"
        >
          {deploying ? (
            <>
              <div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
              Deploying...
            </>
          ) : (
            <>
              <svg className="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14 5l7 7m0 0l-7 7m7-7H3" />
              </svg>
              Deploy Now
            </>
          )}
        </Button>
        {onCustomize && (
          <Button
            onClick={() => onCustomize(workflow)}
            variant="outline"
            disabled={deploying}
          >
            <svg className="w-4 h-4 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
            </svg>
            Customize First
          </Button>
        )}
      </div>
    </div>
  );
}
