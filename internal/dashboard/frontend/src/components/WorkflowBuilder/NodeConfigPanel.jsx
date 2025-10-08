import React from 'react';
import {
  TriggerNodeConfig,
  AITaskNodeConfig,
  MCPServerNodeConfig,
  DecisionNodeConfig,
  TransformNodeConfig,
  CodeNodeConfig,
} from './config';

export default function NodeConfigPanel({ selectedNode, onUpdate, onClose }) {
  if (!selectedNode) {
    return (
      <div className="h-full flex items-center justify-center p-6">
        <div className="text-center">
          <svg
            className="mx-auto h-12 w-12 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M13 10V3L4 14h7v7l9-11h-7z"
            />
          </svg>
          <h3 className="mt-2 text-sm font-medium text-gray-900 dark:text-white">No node selected</h3>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            Select a node to view and edit its configuration
          </p>
        </div>
      </div>
    );
  }

  const renderConfigForm = () => {
    switch (selectedNode.type) {
      case 'trigger':
        return <TriggerNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      case 'ai-task':
        return <AITaskNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      case 'mcp-server':
        return <MCPServerNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      case 'decision':
        return <DecisionNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      case 'transform':
        return <TransformNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      case 'code':
        return <CodeNodeConfig node={selectedNode} onUpdate={onUpdate} onClose={onClose} />;
      default:
        return (
          <div className="p-6">
            <div className="rounded-md bg-yellow-50 dark:bg-yellow-900/20 p-4">
              <div className="flex">
                <div className="flex-shrink-0">
                  <svg
                    className="h-5 w-5 text-yellow-400"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <path
                      fillRule="evenodd"
                      d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                      clipRule="evenodd"
                    />
                  </svg>
                </div>
                <div className="ml-3">
                  <h3 className="text-sm font-medium text-yellow-800 dark:text-yellow-200">
                    Unknown node type
                  </h3>
                  <div className="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
                    <p>The selected node type "{selectedNode.type}" is not supported.</p>
                  </div>
                </div>
              </div>
            </div>
          </div>
        );
    }
  };

  return (
    <div className="h-full flex flex-col bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700">
      <div className="flex-shrink-0 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-medium text-gray-900 dark:text-white">Node Configuration</h2>
            <div className="mt-1 flex items-center space-x-4 text-xs text-gray-500 dark:text-gray-400">
              <span>ID: {selectedNode.id}</span>
              <span>Type: {selectedNode.type}</span>
              {selectedNode.data?.status && (
                <span className="flex items-center">
                  <span
                    className={`inline-block w-2 h-2 rounded-full mr-1 ${
                      selectedNode.data.status === 'active'
                        ? 'bg-green-500'
                        : selectedNode.data.status === 'error'
                        ? 'bg-red-500'
                        : 'bg-gray-400'
                    }`}
                  />
                  {selectedNode.data.status}
                </span>
              )}
            </div>
          </div>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
          >
            <svg className="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">{renderConfigForm()}</div>
    </div>
  );
}
