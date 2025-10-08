import React, { useState } from 'react';
import { Input, Button } from '../../shared';
import MonacoCodeEditor from '../../common/MonacoCodeEditor';

const TRANSFORM_EXAMPLES = [
  {
    label: 'Extract specific fields',
    code: 'return {\n  id: input.id,\n  name: input.name,\n  email: input.email\n};',
  },
  {
    label: 'Map array of items',
    code: 'return input.items.map(item => ({\n  id: item.id,\n  label: item.name.toUpperCase()\n}));',
  },
  {
    label: 'Filter and transform',
    code: 'return input.users\n  .filter(u => u.active)\n  .map(u => u.email);',
  },
  {
    label: 'Aggregate data',
    code: 'return {\n  total: input.items.length,\n  sum: input.items.reduce((a, b) => a + b.value, 0),\n  average: input.items.reduce((a, b) => a + b.value, 0) / input.items.length\n};',
  },
  {
    label: 'Merge with context',
    code: 'return {\n  ...input,\n  processedAt: new Date().toISOString(),\n  workflowId: context.workflowId\n};',
  },
];

export default function TransformNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    transform: node.data?.transform || 'return input;',
    errorHandling: node.data?.errorHandling || 'fail',
    defaultValue: node.data?.defaultValue || '{}',
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
      transform: node.data?.transform || 'return input;',
      errorHandling: node.data?.errorHandling || 'fail',
      defaultValue: node.data?.defaultValue || '{}',
    });
    setHasChanges(false);
  };

  const handleExampleClick = (example) => {
    handleChange('transform', example.code);
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
            Transform Code (JavaScript) <span className="text-red-500">*</span>
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
            {TRANSFORM_EXAMPLES.map((example, index) => (
              <button
                key={index}
                type="button"
                onClick={() => handleExampleClick(example)}
                className="w-full text-left p-2 rounded border border-gray-200 dark:border-gray-600 hover:bg-white dark:hover:bg-gray-700 transition-colors"
              >
                <div className="text-xs font-medium text-gray-700 dark:text-gray-300">
                  {example.label}
                </div>
                <div className="text-xs text-gray-500 dark:text-gray-400 font-mono mt-1 whitespace-pre-wrap">
                  {example.code}
                </div>
              </button>
            ))}
          </div>
        )}

        <MonacoCodeEditor
          value={formData.transform}
          onChange={(value) => handleChange('transform', value)}
          language="javascript"
          height="350px"
          showMinimap={true}
          showLineNumbers={true}
          placeholder="return input;"
        />
        <div className="mt-2 text-xs text-gray-500 dark:text-gray-400 space-y-1">
          <p>Write JavaScript code to transform the input data. Must return a value.</p>
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

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Error Handling
        </label>
        <div className="space-y-2">
          <label className="flex items-center space-x-2">
            <input
              type="radio"
              name="errorHandling"
              value="fail"
              checked={formData.errorHandling === 'fail'}
              onChange={(e) => handleChange('errorHandling', e.target.value)}
              className="border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700 dark:text-gray-300">Fail workflow on error</span>
          </label>
          <label className="flex items-center space-x-2">
            <input
              type="radio"
              name="errorHandling"
              value="default"
              checked={formData.errorHandling === 'default'}
              onChange={(e) => handleChange('errorHandling', e.target.value)}
              className="border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700 dark:text-gray-300">Return default value on error</span>
          </label>
          <label className="flex items-center space-x-2">
            <input
              type="radio"
              name="errorHandling"
              value="passthrough"
              checked={formData.errorHandling === 'passthrough'}
              onChange={(e) => handleChange('errorHandling', e.target.value)}
              className="border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
            />
            <span className="text-sm text-gray-700 dark:text-gray-300">Pass input through on error</span>
          </label>
        </div>
      </div>

      {formData.errorHandling === 'default' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Default Value (JSON)
          </label>
          <MonacoCodeEditor
            value={formData.defaultValue}
            onChange={(value) => handleChange('defaultValue', value)}
            language="json"
            height="120px"
            showMinimap={false}
            showLineNumbers={true}
            placeholder="{}"
          />
          <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
            Value to use if transformation fails
          </p>
        </div>
      )}

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
            <h3 className="text-sm font-medium text-blue-800 dark:text-blue-200">Transform Node</h3>
            <div className="mt-2 text-sm text-blue-700 dark:text-blue-300">
              <p>
                This node transforms data using JavaScript. The code runs in a sandboxed environment with
                limited access to prevent security issues.
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} disabled={!hasChanges || !formData.transform}>
          Save Changes
        </Button>
      </div>
    </div>
  );
}
