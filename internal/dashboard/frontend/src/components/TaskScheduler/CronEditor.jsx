/**
 * CronEditor Component
 * Cron expression editor with presets and validation
 */

import React from 'react';
import { Select } from '../shared';
import { CRON_PRESETS, getCronDescription } from './constants';

export default function CronEditor({ value, onChange, className = '' }) {
  return (
    <div className={className}>
      <label htmlFor="cron-schedule" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        Schedule
      </label>
      <Select
        id="cron-schedule"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full"
      >
        {CRON_PRESETS.map((preset) => (
          <option key={preset.value} value={preset.value}>
            {preset.label} ({preset.value})
          </option>
        ))}
      </Select>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {getCronDescription(value)}
      </p>
    </div>
  );
}
