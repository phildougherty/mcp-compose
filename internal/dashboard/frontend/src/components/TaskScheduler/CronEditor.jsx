/**
 * CronEditor Component
 * Cron expression editor with presets and validation
 */

import React from 'react';
import { Select, Input } from '../shared';
import { CRON_PRESETS, getCronDescription } from './constants';

export default function CronEditor({ value, onChange, className = '' }) {
  const cronOptions = [
    ...CRON_PRESETS.map(preset => ({
      value: preset.value,
      label: preset.label
    })),
    { value: 'custom', label: 'Custom Cron Expression' }
  ];

  const isCustom = !CRON_PRESETS.find(p => p.value === value);
  const selectValue = isCustom ? 'custom' : value;

  const handleSelectChange = (newValue) => {
    if (newValue !== 'custom') {
      onChange(newValue);
    }
  };

  return (
    <div className={className}>
      <label htmlFor="cron-schedule" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Schedule
      </label>
      <div className="space-y-3">
        <Select
          id="cron-schedule"
          value={selectValue}
          onChange={handleSelectChange}
          options={cronOptions}
          className="w-full"
        />
        <Input
          id="cron-expression"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder="*/5 * * * *"
          className="w-full font-mono text-sm"
        />
      </div>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {getCronDescription(value)}
      </p>
    </div>
  );
}
