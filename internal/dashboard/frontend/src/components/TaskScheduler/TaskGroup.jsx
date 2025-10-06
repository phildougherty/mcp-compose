/**
 * TaskGroup Component
 * Collapsible group of tasks organized by type
 */

import React from 'react';
import { Badge } from '../shared';
import { getTaskTypeConfig } from './constants';
import useTaskStore from '../../store/taskStore';
import TaskCard from './TaskCard';

const getIconBgClass = (color) => {
  const colorMap = {
    blue: 'bg-blue-500',
    green: 'bg-green-500',
    yellow: 'bg-yellow-500',
    purple: 'bg-purple-500',
    red: 'bg-red-500',
    indigo: 'bg-indigo-500',
    pink: 'bg-pink-500',
    orange: 'bg-orange-500',
  };

  return colorMap[color] || 'bg-gray-500';
};

export default function TaskGroup({ groupKey, group, onRun, onToggle, onDelete, onViewOutput, onViewRunOutput }) {
  const { toggleGroupExpansion, isGroupExpanded } = useTaskStore();
  const expanded = isGroupExpanded(groupKey);
  const typeConfig = getTaskTypeConfig(groupKey);

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden">
      <button
        onClick={() => toggleGroupExpansion(groupKey)}
        className="w-full min-h-[44px] px-6 py-4 border-b border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-colors focus:outline-none focus:ring-2 focus:ring-inset focus:ring-blue-500"
        aria-expanded={expanded}
      >
        <div className="flex items-center justify-between text-left">
          <div className="flex items-center space-x-3">
            <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${getIconBgClass(typeConfig.color)}`}>
              <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={typeConfig.icon} />
              </svg>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center gap-2">
                {typeConfig.label}s
                <Badge variant="default" size="sm">
                  {group.tasks.length}
                </Badge>
              </h2>
              <p className="text-sm text-gray-500 dark:text-gray-400">{typeConfig.description}</p>
            </div>
          </div>
          <svg
            className={`w-5 h-5 text-gray-400 transition-transform duration-200 ${
              expanded ? 'rotate-180' : ''
            }`}
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fillRule="evenodd"
              d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
              clipRule="evenodd"
            />
          </svg>
        </div>
      </button>

      {expanded && (
        <div className="divide-y divide-gray-200 dark:divide-gray-700">
          {group.tasks.map((task) => (
            <TaskCard
              key={task.id}
              task={task}
              onRun={onRun}
              onToggle={onToggle}
              onDelete={onDelete}
              onViewOutput={onViewOutput}
              onViewRunOutput={onViewRunOutput}
            />
          ))}
        </div>
      )}
    </div>
  );
}
