import React, { useState } from 'react';
import { Input, Button } from '../../shared';
import MonacoCodeEditor from '../../common/MonacoCodeEditor';

const CONDITION_EXAMPLES = [
  {
    label: 'Check if value is true',
    code: 'return input.success === true;',
  },
  {
    label: 'Check if number exceeds threshold',
    code: 'return input.count > 100;',
  },
  {
    label: 'Check if string contains text',
    code: 'return input.message.includes("error");',
  },
  {
    label: 'Check if array has items',
    code: 'return Array.isArray(input.items) && input.items.length > 0;',
  },
];

export default function DecisionNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    condition: node.data?.condition || 'return input.value === true;',
    trueLabel: node.data?.trueLabel || 'True',
    falseLabel: node.data?.falseLabel || 'False',
  });

  const [hasChanges, setHasChanges] = useState(false);
  const [showExamples, setShowExamples] = useState(false);

  const handleChange = (field, value) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
    setHasChanges(true);
  };

  const handleSave = () => {
    onUpdate(node.id, {
      ...node.data,
      ...formData,
    });
    setHasChanges(false);
  };

  const handleCancel = () => {
    setFormData({
      label: node.data?.label || '',
      condition: node.data?.condition || 'return input.value === true;',
      trueLabel: node.data?.trueLabel || 'True',
      falseLabel: node.data?.falseLabel || 'False',
    });
    setHasChanges(false);
  };

  const handleExampleClick = (example) => {
    handleChange('condition', example.code);
    setShowExamples(false);
  };

  return (
    <div className="p-6 space-y-6">
      <div>
        <Input
          label="Node Label"
          value={formData.label}
          onChange={(e) => handleChange('label', e.target.value)}
          placeholder="Enter node label"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-1">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">
            Condition (JavaScript) <span className="text-red-500">*</span>
          </label>
          <button
            type="button"
            onClick={() => setShowExamples(!showExamples)}
            className="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
          >
            {showExamples ? 'Hide' : 'Show'} Examples
          </button>
        </div>

        {showExamples && (
          <div className="mb-3 space-y-2 p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg border border-gray-200 dark:border-gray-600">
            <div className="text-xs font-medium text-gray-700 dark:text-gray-300 mb-2">
              Click an example to use it:
            </div>
            {CONDITION_EXAMPLES.map((example, index) => (
              <button
                key={index}
                type="button"
                onClick={() => handleExampleClick(example)}
                className="w-full text-left p-2 rounded border border-gray-200 dark:border-gray-600 hover:bg-white dark:hover:bg-gray-700 transition-colors"
              >
                <div className="text-xs font-medium text-gray-700 dark:text-gray-300">
                  {example.label}
                </div>
                <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1">
                  {example.code}
                </div>
              </button>
            ))}
          </div>
        )}

        <MonacoCodeEditor
          value={formData.condition}
          onChange={(value) => handleChange('condition', value)}
          language="javascript"
          height="250px"
          showMinimap={false}
          showLineNumbers={true}
          placeholder="return input.value === true;"
        />
        <div className="mt-2 text-xs text-gray-500 dark:text-gray-400 space-y-1">
          <p>Write a JavaScript expression that returns true or false.</p>
          <p>Available variables:</p>
          <ul className="ml-4 list-disc">
            <li>
              <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded">input</code> - Data from previous
              node
            </li>
            <li>
              <code className="bg-gray-100 dark:bg-gray-800 px-1 rounded">context</code> - Workflow context
            </li>
          </ul>
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Input
          label="True Branch Label"
          value={formData.trueLabel}
          onChange={(e) => handleChange('trueLabel', e.target.value)}
          placeholder="True"
        />
        <Input
          label="False Branch Label"
          value={formData.falseLabel}
          onChange={(e) => handleChange('falseLabel', e.target.value)}
          placeholder="False"
        />
      </div>

      <div className="rounded-md bg-blue-50 dark:bg-blue-900/20 p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <svg
              className="h-5 w-5 text-blue-400"
              viewBox="0 0 20 20"
              fill="currentColor"
            >
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
                clipRule="evenodd"
              />
            </svg>
          </div>
          <div className="ml-3">
            <h3 className="text-sm font-medium text-blue-800 dark:text-blue-200">Decision Node</h3>
            <div className="mt-2 text-sm text-blue-700 dark:text-blue-300">
              <p>
                This node evaluates the condition and routes the workflow to the appropriate branch. Make
                sure to connect both the true and false outputs.
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} disabled={!hasChanges || !formData.condition}>
          Save Changes
        </Button>
      </div>
    </div>
  );
}
