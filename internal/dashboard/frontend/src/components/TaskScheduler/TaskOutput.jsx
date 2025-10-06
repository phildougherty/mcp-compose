/**
 * TaskOutput Component
 * Modal for viewing task execution output
 */

import React from 'react';
import { Modal, Button } from '../shared';
import { useToast } from '../../hooks';

export default function TaskOutput({ output, onClose }) {
  const { success } = useToast();

  if (!output) return null;

  const handleCopy = () => {
    navigator.clipboard.writeText(output.output);
    success('Output copied to clipboard');
  };

  return (
    <Modal
      isOpen={true}
      onClose={onClose}
      title="Task Output"
      size="large"
    >
      <div className="space-y-4">
        <div className="text-sm text-gray-500 dark:text-gray-400">
          <p>Task ID: {output.taskId}</p>
          {output.runId && <p>Run ID: {output.runId}</p>}
        </div>

        <div className="bg-gray-900 rounded-lg p-4 max-h-96 overflow-y-auto">
          <pre className="font-mono text-sm text-gray-300 whitespace-pre-wrap break-words">
            {output.output || 'No output available'}
          </pre>
        </div>

        <div className="flex items-center justify-end space-x-3">
          <Button variant="secondary" onClick={handleCopy}>
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
              />
            </svg>
            Copy Output
          </Button>
          <Button variant="primary" onClick={onClose}>
            Close
          </Button>
        </div>
      </div>
    </Modal>
  );
}
