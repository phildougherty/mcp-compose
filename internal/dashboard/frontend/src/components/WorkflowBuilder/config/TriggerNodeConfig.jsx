import React, { useState } from 'react';
import { Input, Select, Button } from '../../shared';
import CronEditor from '../../TaskScheduler/CronEditor';

export default function TriggerNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    triggerType: node.data?.triggerType || 'cron',
    cronSchedule: node.data?.cronSchedule || '0 0 * * *',
    webhookPath: node.data?.webhookPath || '/webhook',
    eventType: node.data?.eventType || 'custom',
    enabled: node.data?.enabled !== false,
    passContext: node.data?.config?.passContext !== false,
  });

  const [hasChanges, setHasChanges] = useState(false);

  const handleChange = (field, value) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
    setHasChanges(true);
  };

  const handleSave = () => {
    const updatedData = {
      ...node.data,
      ...formData,
      config: {
        ...node.data?.config,
        passContext: formData.passContext,
      },
    };

    if (formData.triggerType === 'cron') {
      updatedData.schedule = formData.cronSchedule;
    } else if (formData.triggerType === 'webhook') {
      updatedData.webhook = formData.webhookPath;
    } else if (formData.triggerType === 'event') {
      updatedData.event = formData.eventType;
    }

    onUpdate(node.id, updatedData);
    setHasChanges(false);
  };

  const handleCancel = () => {
    setFormData({
      label: node.data?.label || '',
      triggerType: node.data?.triggerType || 'cron',
      cronSchedule: node.data?.cronSchedule || '0 0 * * *',
      webhookPath: node.data?.webhookPath || '/webhook',
      eventType: node.data?.eventType || 'custom',
      enabled: node.data?.enabled !== false,
      passContext: node.data?.config?.passContext !== false,
    });
    setHasChanges(false);
  };

  const triggerTypeOptions = [
    { value: 'cron', label: 'Cron Schedule' },
    { value: 'webhook', label: 'Webhook' },
    { value: 'event', label: 'Event' },
    { value: 'manual', label: 'Manual' },
  ];

  const eventTypeOptions = [
    { value: 'custom', label: 'Custom Event' },
    { value: 'task_completed', label: 'Task Completed' },
    { value: 'task_failed', label: 'Task Failed' },
    { value: 'server_started', label: 'Server Started' },
    { value: 'server_stopped', label: 'Server Stopped' },
    { value: 'file_changed', label: 'File Changed' },
  ];

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
          label="Trigger Type"
          value={formData.triggerType}
          onChange={(value) => handleChange('triggerType', value)}
          options={triggerTypeOptions}
        />
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {formData.triggerType === 'cron' && 'Execute on a scheduled time interval'}
          {formData.triggerType === 'webhook' && 'Execute when webhook endpoint is called'}
          {formData.triggerType === 'event' && 'Execute when a specific event occurs'}
          {formData.triggerType === 'manual' && 'Execute only when manually triggered'}
        </p>
      </div>

      {formData.triggerType === 'cron' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
            Cron Schedule
          </label>
          <CronEditor
            value={formData.cronSchedule}
            onChange={(value) => handleChange('cronSchedule', value)}
          />
        </div>
      )}

      {formData.triggerType === 'webhook' && (
        <div>
          <Input
            label="Webhook Path"
            value={formData.webhookPath}
            onChange={(e) => handleChange('webhookPath', e.target.value)}
            placeholder="/webhook/my-workflow"
          />
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Full URL will be: http://your-domain{formData.webhookPath}
          </p>
        </div>
      )}

      {formData.triggerType === 'event' && (
        <div>
          <Select
            label="Event Type"
            value={formData.eventType}
            onChange={(value) => handleChange('eventType', value)}
            options={eventTypeOptions}
          />
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Select the type of event that will trigger this workflow
          </p>
        </div>
      )}

      <div>
        <label className="flex items-center space-x-2">
          <input
            type="checkbox"
            checked={formData.enabled}
            onChange={(e) => handleChange('enabled', e.target.checked)}
            className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span className="text-sm text-gray-700 dark:text-gray-300">Enable trigger</span>
        </label>
        <p className="mt-1 ml-6 text-xs text-gray-500 dark:text-gray-400">
          Disabled triggers will not execute the workflow
        </p>
      </div>

      <div>
        <label className="flex items-center space-x-2">
          <input
            type="checkbox"
            checked={formData.passContext}
            onChange={(e) => handleChange('passContext', e.target.checked)}
            className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span className="text-sm text-gray-700 dark:text-gray-300">Pass context to next nodes</span>
        </label>
        <p className="mt-1 ml-6 text-xs text-gray-500 dark:text-gray-400">
          When enabled, trigger metadata (timestamp, trigger data) will be passed to downstream nodes. Disable to prevent context pollution in AI prompts.
        </p>
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button variant="primary" onClick={handleSave} disabled={!hasChanges}>
          Save Changes
        </Button>
      </div>
    </div>
  );
}
