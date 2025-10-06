/**
 * Audit Utility Functions
 */

import { EVENT_TYPES, ICON_PATHS } from './constants';

export function formatTimestamp(timestamp) {
  if (!timestamp) return 'Never';

  try {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now - date;
    const diffMinutes = Math.floor(diffMs / (1000 * 60));
    const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

    if (diffMinutes < 1) return 'Just now';
    if (diffMinutes < 60) return `${diffMinutes}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return (
      date.toLocaleDateString() +
      ' ' +
      date.toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
      })
    );
  } catch (e) {
    return timestamp;
  }
}

export function formatEventName(event) {
  const type = EVENT_TYPES.find((t) => t.value === event);
  return type ? type.label : event.replace(/\./g, ' ').replace(/\b\w/g, (l) => l.toUpperCase());
}

export function getEventIcon(event) {
  const type = EVENT_TYPES.find((t) => t.value === event);
  const iconName = type ? type.icon : 'document-text';
  return ICON_PATHS[iconName] || ICON_PATHS['document-text'];
}

export function getEventColor(event) {
  const type = EVENT_TYPES.find((t) => t.value === event);
  return type ? type.color : 'bg-gray-500';
}
