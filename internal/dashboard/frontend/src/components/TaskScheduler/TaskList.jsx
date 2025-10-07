/**
 * TaskList Component
 * Displays all task groups
 */

import React from 'react';
import { EmptyState, Button } from '../shared';
import TaskGroup from './TaskGroup';

export default function TaskList({ groups, onRun, onToggle, onDelete, onViewOutput, onViewRunOutput, onCreateTask, onCreateChat }) {
  if (Object.keys(groups).length === 0) {
    return (
      <EmptyState
        icon={
          <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        }
        title="No tasks found"
        description="Try adjusting your search or filters to find tasks, or create your first automated task"
        action={
          <Button onClick={onCreateTask} variant="primary">
            Create Your First Task
          </Button>
        }
      />
    );
  }

  return (
    <div className="space-y-4">
      {Object.entries(groups).map(([groupKey, group]) => (
        <TaskGroup
          key={groupKey}
          groupKey={groupKey}
          group={group}
          onRun={onRun}
          onToggle={onToggle}
          onDelete={onDelete}
          onViewOutput={onViewOutput}
          onViewRunOutput={onViewRunOutput}
          onCreateChat={onCreateChat}
        />
      ))}
    </div>
  );
}
