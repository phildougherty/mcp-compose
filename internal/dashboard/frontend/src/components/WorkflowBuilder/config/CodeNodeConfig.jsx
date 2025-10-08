import React, { useState } from 'react';
import { Input, Select, Button, Checkbox } from '../../shared';
import MonacoCodeEditor from '../../common/MonacoCodeEditor';

const LANGUAGE_OPTIONS = [
  { value: 'javascript', label: 'JavaScript', extension: 'js' },
  { value: 'python', label: 'Python', extension: 'py' },
  { value: 'bash', label: 'Bash', extension: 'sh' },
  { value: 'go', label: 'Go', extension: 'go' },
  { value: 'ruby', label: 'Ruby', extension: 'rb' },
  { value: 'php', label: 'PHP', extension: 'php' },
];

const CODE_TEMPLATES = {
  javascript: `// Input is available as 'input' variable
// Context is available as 'context' variable

function process(input, context) {
  // Your code here
  return input;
}

return process(input, context);`,
  python: `# Input is available as 'input' variable
# Context is available as 'context' variable

def process(input_data, context):
    # Your code here
    return input_data

print(process(input, context))`,
  bash: `#!/bin/bash
# Input is available via stdin
# Context is available as environment variables

# Read input
INPUT=$(cat)

# Your code here
echo "$INPUT"`,
  go: `package main

import (
    "encoding/json"
    "fmt"
    "os"
)

func main() {
    // Read input from stdin
    var input map[string]interface{}
    json.NewDecoder(os.Stdin).Decode(&input)

    // Your code here

    // Output result
    json.NewEncoder(os.Stdout).Encode(input)
}`,
  ruby: `# Input is available as 'input' variable (parsed JSON)
# Context is available as 'context' variable

def process(input, context)
  # Your code here
  input
end

puts process(input, context).to_json`,
  php: `<?php
// Input is available via stdin
// Context is available as environment variables

$input = json_decode(file_get_contents('php://stdin'), true);

// Your code here

echo json_encode($input);
?>`,
};

export default function CodeNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    language: node.data?.language || 'javascript',
    code: node.data?.code || CODE_TEMPLATES.javascript,
    timeout: node.data?.timeout || 30,
    retryOnFailure: node.data?.retryOnFailure || false,
    maxRetries: node.data?.maxRetries || 3,
    captureStderr: node.data?.captureStderr !== false,
    environment: node.data?.environment || '',
  });

  const [hasChanges, setHasChanges] = useState(false);

  const handleChange = (field, value) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
    setHasChanges(true);
  };

  const handleLanguageChange = (newLanguage) => {
    const shouldUseTemplate =
      !formData.code || formData.code === CODE_TEMPLATES[formData.language];

    setFormData((prev) => ({
      ...prev,
      language: newLanguage,
      code: shouldUseTemplate ? CODE_TEMPLATES[newLanguage] : prev.code,
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
      language: node.data?.language || 'javascript',
      code: node.data?.code || CODE_TEMPLATES.javascript,
      timeout: node.data?.timeout || 30,
      retryOnFailure: node.data?.retryOnFailure || false,
      maxRetries: node.data?.maxRetries || 3,
      captureStderr: node.data?.captureStderr !== false,
      environment: node.data?.environment || '',
    });
    setHasChanges(false);
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
        <Select
          label="Programming Language"
          value={formData.language}
          onChange={(value) => handleLanguageChange(value)}
          options={LANGUAGE_OPTIONS}
        />
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Code <span className="text-red-500">*</span>
        </label>
        <MonacoCodeEditor
          value={formData.code}
          onChange={(value) => handleChange('code', value)}
          language={formData.language}
          height="450px"
          showMinimap={true}
          showLineNumbers={true}
        />
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
          Code will execute in a containerized environment. Input data is passed via stdin or variables
          depending on the language.
        </p>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Environment Variables (Optional)
        </label>
        <MonacoCodeEditor
          value={formData.environment}
          onChange={(value) => handleChange('environment', value)}
          language="shell"
          height="120px"
          showMinimap={false}
          showLineNumbers={true}
          placeholder="KEY1=value1&#10;KEY2=value2"
        />
        <p className="mt-2 text-xs text-gray-500 dark:text-gray-400">
          One variable per line in KEY=value format
        </p>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <Input
            label="Timeout (seconds)"
            type="number"
            min="1"
            max="300"
            value={formData.timeout}
            onChange={(e) => handleChange('timeout', parseInt(e.target.value))}
          />
        </div>

        <div>
          <Input
            label="Max Retries"
            type="number"
            min="0"
            max="10"
            value={formData.maxRetries}
            onChange={(e) => handleChange('maxRetries', parseInt(e.target.value))}
            disabled={!formData.retryOnFailure}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Checkbox
          label="Retry on failure"
          checked={formData.retryOnFailure}
          onChange={(e) => handleChange('retryOnFailure', e.target.checked)}
        />
        <Checkbox
          label="Capture stderr output"
          checked={formData.captureStderr}
          onChange={(e) => handleChange('captureStderr', e.target.checked)}
        />
      </div>

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
            <h3 className="text-sm font-medium text-yellow-800 dark:text-yellow-200">Security Notice</h3>
            <div className="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
              <p>
                Code executes in a sandboxed container with limited access. Avoid including sensitive data
                directly in the code. Use environment variables for secrets.
              </p>
            </div>
          </div>
        </div>
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} disabled={!hasChanges || !formData.code}>
          Save Changes
        </Button>
      </div>
    </div>
  );
}
