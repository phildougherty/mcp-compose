/**
 * EventChart Component - Simple bar chart for event distribution
 */

import React from 'react';
import { EVENT_TYPES } from './constants';

function EventChart({ stats }) {
  if (!stats || !stats.event_counts) {
    return null;
  }

  const eventCounts = Object.entries(stats.event_counts)
    .map(([event, count]) => ({
      event,
      count,
      config: EVENT_TYPES.find((t) => t.value === event) || { label: event, color: 'gray' },
    }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 10);

  const maxCount = Math.max(...eventCounts.map((e) => e.count), 1);

  return (
    <div className="bg-slate-800 border border-slate-700 rounded-lg p-6">
      <h3 className="text-lg font-semibold text-slate-100 mb-4">Event Distribution</h3>
      <div className="space-y-3">
        {eventCounts.map(({ event, count, config }) => {
          const percentage = (count / maxCount) * 100;

          return (
            <div key={event} className="space-y-1">
              <div className="flex items-center justify-between text-sm">
                <span className="text-slate-300 font-medium">{config.label}</span>
                <span className="text-slate-400">{count.toLocaleString()}</span>
              </div>
              <div className="w-full bg-slate-700 rounded-full h-2">
                <div
                  className={`h-2 rounded-full transition-all duration-500 ${config.color}`}
                  style={{ width: `${percentage}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>

      {eventCounts.length === 0 && (
        <div className="text-center py-8 text-slate-400">
          <p>No events to display</p>
        </div>
      )}
    </div>
  );
}

export default EventChart;
