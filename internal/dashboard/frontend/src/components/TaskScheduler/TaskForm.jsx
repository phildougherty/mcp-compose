/**
 * TaskForm Component
 * Create/edit task form with validation and type-specific fields
 */

import React, { useState } from 'react';
import { Modal, Button, Input, Select, Checkbox } from '../shared';
import { TASK_TYPES, MODEL_HINTS } from './constants';
import CronEditor from './CronEditor';
import useTaskStore from '../../store/taskStore';

export default function TaskForm({ isOpen, onClose, onSubmit }) {
  const { getAvailableDependencies } = useTaskStore();
  const availableDependencies = getAvailableDependencies();

  const [formData, setFormData] = useState({
    type: 'shell',
    name: '',
    description: '',
    command: '',
    prompt: '',
    schedule: '0 0 * * *',
    enabled: true,
    model: '',
    modelHint: 'balanced',
    maxCost: '1.0',
    requireLocal: false,
    dependsOn: [],
  });

  const handleChange = (field, value) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleSubmit = (e) => {
    e.preventDefault();

    let endpoint;
    let requestData = {
      name: formData.name,
      description: formData.description,
      enabled: formData.enabled,
      type: formData.type,
    };

    switch (formData.type) {
      case 'shell':
        endpoint = '/api/tasks';
        requestData = {
          ...requestData,
          command: formData.command,
          schedule: formData.schedule,
        };
        break;
      case 'ai':
        endpoint = '/api/tasks/ai';
        requestData = {
          ...requestData,
          prompt: formData.prompt,
          schedule: formData.schedule,
          model: formData.model,
          modelHint: formData.modelHint,
          maxCost: parseFloat(formData.maxCost) || 1.0,
          requireLocal: formData.requireLocal,
        };
        break;
      case 'manual':
        endpoint = '/api/tasks/manual';
        requestData = {
          ...requestData,
          command: formData.command,
          prompt: formData.prompt,
        };
        break;
      case 'dependency':
        endpoint = '/api/tasks/dependency';
        requestData = {
          ...requestData,
          command: formData.command,
          dependsOn: formData.dependsOn,
        };
        break;
      case 'watcher':
        endpoint = '/api/tasks/watcher';
        requestData = {
          ...requestData,
          command: formData.command,
          watcherConfig: {
            type: 'file_change',
            triggerOnce: false,
            watchPath: '/workspace',
            checkInterval: '30s',
          },
        };
        break;
      default:
        throw new Error('Unknown task type');
    }

    onSubmit(endpoint, requestData);
    setFormData({
      type: 'shell',
      name: '',
      description: '',
      command: '',
      prompt: '',
      schedule: '0 0 * * *',
      enabled: true,
      model: '',
      modelHint: 'balanced',
      maxCost: '1.0',
      requireLocal: false,
      dependsOn: [],
    });
  };

  const handleDependencyToggle = (taskId) => {
    setFormData((prev) => ({
      ...prev,
      dependsOn: prev.dependsOn.includes(taskId)
        ? prev.dependsOn.filter((id) => id !== taskId)
        : [...prev.dependsOn, taskId],
    }));
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="Create New Task" size="large">
      <form onSubmit={handleSubmit} className="space-y-6">
        <fieldset>
          <legend className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-3">
            Task Type
          </legend>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {TASK_TYPES.map((taskType) => (
              <button
                key={taskType.value}
                type="button"
                onClick={() => handleChange('type', taskType.value)}
                className={`p-4 border-2 rounded-lg text-left transition-all transform hover:scale-105 focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 ${
                  formData.type === taskType.value
                    ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/20 shadow-md'
                    : 'border-gray-300 dark:border-gray-600 hover:border-gray-400 dark:hover:border-gray-500'
                }`}
              >
                <div className="flex items-center space-x-2 mb-2">
                  <div className={`w-6 h-6 rounded flex items-center justify-center bg-${taskType.color}-500`}>
                    <svg className="w-4 h-4 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={taskType.icon} />
                    </svg>
                  </div>
                  <span className="text-sm font-medium text-gray-900 dark:text-white">{taskType.label}</span>
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">{taskType.description}</p>
              </button>
            ))}
          </div>
        </fieldset>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          <Input
            label="Task Name"
            value={formData.name}
            onChange={(e) => handleChange('name', e.target.value)}
            placeholder="Enter descriptive task name"
            required
          />
          <Input
            label="Description"
            value={formData.description}
            onChange={(e) => handleChange('description', e.target.value)}
            placeholder="Brief description of what this task does"
          />
        </div>

        {formData.type !== 'manual' && formData.type !== 'dependency' && (
          <CronEditor value={formData.schedule} onChange={(value) => handleChange('schedule', value)} />
        )}

        {formData.type === 'shell' && (
          <div>
            <label
              htmlFor="shell-command"
              className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
            >
              Shell Command <span className="text-red-500">*</span>
            </label>
            <textarea
              id="shell-command"
              value={formData.command}
              onChange={(e) => handleChange('command', e.target.value)}
              rows={3}
              required
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono text-sm"
              placeholder="echo 'Hello World'"
            />
          </div>
        )}

        {formData.type === 'ai' && (
          <div className="space-y-4">
            <div>
              <label htmlFor="ai-prompt" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                AI Prompt <span className="text-red-500">*</span>
              </label>
              <textarea
                id="ai-prompt"
                value={formData.prompt}
                onChange={(e) => handleChange('prompt', e.target.value)}
                rows={4}
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
                placeholder="Describe what you want the AI to do..."
              />
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
              <Select
                label="Model Hint"
                value={formData.modelHint}
                onChange={(e) => handleChange('modelHint', e.target.value)}
              >
                {MODEL_HINTS.map((hint) => (
                  <option key={hint} value={hint}>
                    {hint}
                  </option>
                ))}
              </Select>
              <Input
                label="Max Cost ($)"
                type="number"
                step="0.01"
                min="0"
                value={formData.maxCost}
                onChange={(e) => handleChange('maxCost', e.target.value)}
                placeholder="1.00"
              />
              <div className="flex items-end">
                <Checkbox
                  label="Local Only"
                  checked={formData.requireLocal}
                  onChange={(e) => handleChange('requireLocal', e.target.checked)}
                />
              </div>
            </div>
          </div>
        )}

        {formData.type === 'manual' && (
          <div>
            <label htmlFor="manual-command" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Command or Prompt
            </label>
            <textarea
              id="manual-command"
              value={formData.command}
              onChange={(e) => handleChange('command', e.target.value)}
              rows={3}
              className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white"
              placeholder="Command or AI prompt to execute manually..."
            />
          </div>
        )}

        {formData.type === 'dependency' && (
          <div className="space-y-4">
            <div>
              <label
                htmlFor="dependency-command"
                className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1"
              >
                Command <span className="text-red-500">*</span>
              </label>
              <textarea
                id="dependency-command"
                value={formData.command}
                onChange={(e) => handleChange('command', e.target.value)}
                rows={3}
                required
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono text-sm"
                placeholder="echo 'Dependency task completed'"
              />
            </div>
            <fieldset>
              <legend className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
                Dependencies
              </legend>
              <div className="max-h-32 overflow-y-auto border border-gray-300 dark:border-gray-600 rounded-lg p-3 space-y-2 bg-gray-50 dark:bg-gray-700">
                {availableDependencies.length === 0 ? (
                  <div className="text-sm text-gray-500 dark:text-gray-400 text-center py-2">
                    No tasks available for dependencies
                  </div>
                ) : (
                  availableDependencies.map((task) => (
                    <Checkbox
                      key={task.id}
                      label={task.name}
                      checked={formData.dependsOn.includes(task.id)}
                      onChange={() => handleDependencyToggle(task.id)}
                    />
                  ))
                )}
              </div>
            </fieldset>
          </div>
        )}

        <Checkbox
          label="Enable task immediately after creation"
          checked={formData.enabled}
          onChange={(e) => handleChange('enabled', e.target.checked)}
        />

        <div className="flex items-center justify-end space-x-3 pt-6 border-t border-gray-200 dark:border-gray-700">
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" type="submit" disabled={!formData.name}>
            Create Task
          </Button>
        </div>
      </form>
    </Modal>
  );
}
