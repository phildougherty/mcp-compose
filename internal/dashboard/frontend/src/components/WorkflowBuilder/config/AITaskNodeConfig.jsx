import React, { useState, useEffect } from 'react';
import { Input, Select, Button, Checkbox } from '../../shared';

const MODEL_PROVIDERS = [
  { value: 'openrouter', label: 'OpenRouter' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'local', label: 'Local (Ollama)' },
];

const MODEL_HINTS = [
  { value: 'fast', label: 'Fast', description: 'Quick responses, lower quality' },
  { value: 'balanced', label: 'Balanced', description: 'Good balance of speed and quality' },
  { value: 'quality', label: 'Quality', description: 'Best quality, slower responses' },
  { value: 'reasoning', label: 'Reasoning', description: 'Complex reasoning tasks' },
];

export default function AITaskNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    prompt: node.data?.config?.prompt || node.data?.prompt || '',
    provider: node.data?.config?.provider || node.data?.provider || 'openrouter',
    model: node.data?.config?.model || node.data?.model || '',
    modelHint: node.data?.modelHint || 'balanced',
    temperature: node.data?.config?.temperature || node.data?.temperature || 0.7,
    maxTokens: node.data?.config?.maxTokens || node.data?.maxTokens || 2000,
    maxCost: node.data?.maxCost || 1.0,
    requireLocal: node.data?.requireLocal || false,
    systemPrompt: node.data?.config?.systemPrompt || node.data?.systemPrompt || '',
    useContext: node.data?.useContext !== false,
  });

  const [hasChanges, setHasChanges] = useState(false);
  const [models, setModels] = useState([]);
  const [loadingModels, setLoadingModels] = useState(false);
  const [modelError, setModelError] = useState(null);

  useEffect(() => {
    fetchModels(formData.provider);
  }, [formData.provider]);

  const fetchModels = async (provider) => {
    setLoadingModels(true);
    setModelError(null);

    try {
      const response = await fetch(`/api/ai/models?provider=${provider}`);

      if (!response.ok) {
        throw new Error(`Failed to fetch models: ${response.statusText}`);
      }

      const data = await response.json();
      setModels(data.models || []);
    } catch (error) {
      console.error('Error fetching models:', error);
      setModelError(error.message);
      setModels([]);
    } finally {
      setLoadingModels(false);
    }
  };

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
      label: formData.label,
      config: {
        prompt: formData.prompt,
        provider: formData.provider,
        model: formData.model,
        temperature: formData.temperature,
        maxTokens: formData.maxTokens,
        systemPrompt: formData.systemPrompt,
      },
      modelHint: formData.modelHint,
      maxCost: formData.maxCost,
      requireLocal: formData.requireLocal,
      useContext: formData.useContext,
    });
    setHasChanges(false);
  };

  const handleCancel = () => {
    setFormData({
      label: node.data?.label || '',
      prompt: node.data?.config?.prompt || node.data?.prompt || '',
      provider: node.data?.config?.provider || node.data?.provider || 'openrouter',
      model: node.data?.config?.model || node.data?.model || '',
      modelHint: node.data?.modelHint || 'balanced',
      temperature: node.data?.config?.temperature || node.data?.temperature || 0.7,
      maxTokens: node.data?.config?.maxTokens || node.data?.maxTokens || 2000,
      maxCost: node.data?.maxCost || 1.0,
      requireLocal: node.data?.requireLocal || false,
      systemPrompt: node.data?.config?.systemPrompt || node.data?.systemPrompt || '',
      useContext: node.data?.useContext !== false,
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
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Prompt <span className="text-red-500">*</span>
        </label>
        <textarea
          value={formData.prompt}
          onChange={(e) => handleChange('prompt', e.target.value)}
          rows={6}
          required
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-y"
          placeholder="Describe what you want the AI to do..."
        />
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          Use {"{{input}}"} to reference data from previous nodes
        </p>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          System Prompt (Optional)
        </label>
        <textarea
          value={formData.systemPrompt}
          onChange={(e) => handleChange('systemPrompt', e.target.value)}
          rows={3}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white resize-y"
          placeholder="Set the AI's behavior and context..."
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <Select
          label="Provider"
          value={formData.provider}
          onChange={(value) => handleChange('provider', value)}
          options={MODEL_PROVIDERS}
        />

        <div>
          <Select
            label="Model"
            value={formData.model}
            onChange={(value) => handleChange('model', value)}
            options={models.map(m => ({ value: m, label: m }))}
            disabled={loadingModels}
            placeholder={loadingModels ? "Loading models..." : "Select a model"}
          />
          {modelError && (
            <p className="mt-1 text-xs text-red-500 dark:text-red-400">
              {modelError}
            </p>
          )}
        </div>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          Model Hint
        </label>
        <div className="grid grid-cols-2 gap-2">
          {MODEL_HINTS.map((hint) => (
            <button
              key={hint.value}
              type="button"
              onClick={() => handleChange('modelHint', hint.value)}
              className={`p-3 border-2 rounded-lg text-left transition-all ${
                formData.modelHint === hint.value
                  ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20'
                  : 'border-gray-300 dark:border-gray-600 hover:border-gray-400'
              }`}
            >
              <div className="text-sm font-medium text-gray-900 dark:text-white">{hint.label}</div>
              <div className="text-xs text-gray-500 dark:text-gray-400">{hint.description}</div>
            </button>
          ))}
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div>
          <Input
            label="Temperature"
            type="number"
            step="0.1"
            min="0"
            max="2"
            value={formData.temperature}
            onChange={(e) => handleChange('temperature', parseFloat(e.target.value))}
          />
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">0 = deterministic, 2 = creative</p>
        </div>

        <div>
          <Input
            label="Max Tokens"
            type="number"
            min="1"
            max="32000"
            value={formData.maxTokens}
            onChange={(e) => handleChange('maxTokens', parseInt(e.target.value))}
          />
        </div>

        <div>
          <Input
            label="Max Cost ($)"
            type="number"
            step="0.01"
            min="0"
            value={formData.maxCost}
            onChange={(e) => handleChange('maxCost', parseFloat(e.target.value))}
          />
        </div>
      </div>

      <div className="space-y-2">
        <Checkbox
          label="Require local model"
          checked={formData.requireLocal}
          onChange={(e) => handleChange('requireLocal', e.target.checked)}
        />
        <Checkbox
          label="Include workflow context"
          checked={formData.useContext}
          onChange={(e) => handleChange('useContext', e.target.checked)}
        />
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} disabled={!hasChanges || !formData.prompt}>
          Save Changes
        </Button>
      </div>
    </div>
  );
}
