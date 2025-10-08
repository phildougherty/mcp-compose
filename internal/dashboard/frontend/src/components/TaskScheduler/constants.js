/**
 * Task Scheduler Constants
 * Task types, cron presets, model hints
 */

export const TASK_TYPES = [
  {
    value: 'shell',
    label: 'Shell Command',
    icon: 'M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z',
    color: 'green',
    description: 'Execute shell commands on schedule',
  },
  {
    value: 'ai',
    label: 'AI Task',
    icon: 'M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z',
    color: 'purple',
    description: 'AI-powered tasks using LLMs',
  },
  {
    value: 'workflow',
    label: 'Workflow',
    icon: 'M4 4a2 2 0 00-2 2v1h16V6a2 2 0 00-2-2H4z M18 9H2v5a2 2 0 002 2h12a2 2 0 002-2V9zM4 13h4v1H4v-1z M14 13h1v1h-1v-1z M11 13h1v1h-1v-1z',
    color: 'blue',
    description: 'Multi-step workflow executions',
  },
  {
    value: 'manual',
    label: 'Manual Task',
    icon: 'M5.636 18.364a9 9 0 010-12.728m12.728 0a9 9 0 010 12.728m-9.9-2.829a5 5 0 010-7.07m7.072 0a5 5 0 010 7.07M13 12a1 1 0 11-2 0 1 1 0 012 0z',
    color: 'blue',
    description: 'Manually triggered tasks',
  },
  {
    value: 'dependency',
    label: 'Dependency Task',
    icon: 'M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1',
    color: 'yellow',
    description: 'Tasks that depend on other tasks',
  },
  {
    value: 'watcher',
    label: 'Watcher Task',
    icon: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z',
    color: 'indigo',
    description: 'File and event watchers',
  },
];

export const MODEL_HINTS = ['fast', 'cheap', 'powerful', 'local', 'balanced'];

export const CRON_PRESETS = [
  { label: 'Every minute', value: '* * * * *' },
  { label: 'Every 2 minutes', value: '*/2 * * * *' },
  { label: 'Every 5 minutes', value: '*/5 * * * *' },
  { label: 'Every 10 minutes', value: '*/10 * * * *' },
  { label: 'Every 15 minutes', value: '*/15 * * * *' },
  { label: 'Every 30 minutes', value: '*/30 * * * *' },
  { label: 'Every hour', value: '0 * * * *' },
  { label: 'Every 2 hours', value: '0 */2 * * *' },
  { label: 'Every 3 hours', value: '0 */3 * * *' },
  { label: 'Every 6 hours', value: '0 */6 * * *' },
  { label: 'Every 12 hours', value: '0 */12 * * *' },
  { label: 'Daily at midnight', value: '0 0 * * *' },
  { label: 'Daily at 6 AM', value: '0 6 * * *' },
  { label: 'Daily at 9 AM', value: '0 9 * * *' },
  { label: 'Daily at noon', value: '0 12 * * *' },
  { label: 'Daily at 6 PM', value: '0 18 * * *' },
  { label: 'Weekly on Monday', value: '0 0 * * 1' },
  { label: 'Weekly on Tuesday', value: '0 0 * * 2' },
  { label: 'Weekly on Wednesday', value: '0 0 * * 3' },
  { label: 'Weekly on Thursday', value: '0 0 * * 4' },
  { label: 'Weekly on Friday', value: '0 0 * * 5' },
  { label: 'Weekly on Saturday', value: '0 0 * * 6' },
  { label: 'Weekly on Sunday', value: '0 0 * * 0' },
  { label: 'Monthly on 1st', value: '0 0 1 * *' },
  { label: 'Monthly on 15th', value: '0 0 15 * *' },
  { label: 'Yearly on Jan 1st', value: '0 0 1 1 *' },
];

export const CRON_DESCRIPTIONS = {
  '* * * * *': 'Every minute',
  '*/5 * * * *': 'Every 5 minutes',
  '*/10 * * * *': 'Every 10 minutes',
  '*/15 * * * *': 'Every 15 minutes',
  '*/30 * * * *': 'Every 30 minutes',
  '0 * * * *': 'Every hour',
  '0 */2 * * *': 'Every 2 hours',
  '0 */3 * * *': 'Every 3 hours',
  '0 */6 * * *': 'Every 6 hours',
  '0 */12 * * *': 'Every 12 hours',
  '0 0 * * *': 'Daily at midnight',
  '0 9 * * *': 'Daily at 9:00 AM',
  '0 12 * * *': 'Daily at noon',
  '0 18 * * *': 'Daily at 6:00 PM',
  '0 0 * * 0': 'Weekly on Sunday at midnight',
  '0 0 * * 1': 'Weekly on Monday at midnight',
  '0 9 * * 1': 'Weekly on Monday at 9:00 AM',
  '0 0 1 * *': 'Monthly on the 1st at midnight',
  '0 0 1 1 *': 'Yearly on January 1st at midnight',
  '0 0 15 * *': 'Monthly on the 15th at midnight',
  '0 13 * * 4': 'Weekly on Thursday at 1:00 PM',
};

export function getTaskTypeConfig(type) {
  return (
    TASK_TYPES.find((t) => t.value === type) || {
      label: type.charAt(0).toUpperCase() + type.slice(1),
      icon: 'M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z',
      color: 'gray',
      description: 'Custom task type',
    }
  );
}

export function getCronDescription(cron) {
  if (!cron) return '';

  if (CRON_DESCRIPTIONS[cron]) {
    return CRON_DESCRIPTIONS[cron];
  }

  const parts = cron.split(' ');
  if (parts.length >= 5) {
    const [minute, hour, day, month, dayOfWeek] = parts;

    if (minute !== '*' && hour === '*' && day === '*' && month === '*' && dayOfWeek === '*') {
      return `Hourly at ${minute} minutes past the hour`;
    }

    if (hour !== '*' && minute !== '*' && day === '*' && month === '*' && dayOfWeek === '*') {
      const hourNum = parseInt(hour);
      const minNum = parseInt(minute);
      const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;

      return `Daily at ${time}`;
    }

    if (dayOfWeek !== '*' && hour !== '*' && minute !== '*') {
      const days = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
      const dayName = days[parseInt(dayOfWeek)] || `day ${dayOfWeek}`;
      const hourNum = parseInt(hour);
      const minNum = parseInt(minute);
      const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;

      return `Weekly on ${dayName} at ${time}`;
    }

    if (day !== '*' && hour !== '*' && minute !== '*' && month === '*' && dayOfWeek === '*') {
      const hourNum = parseInt(hour);
      const minNum = parseInt(minute);
      const time = `${hourNum.toString().padStart(2, '0')}:${minNum.toString().padStart(2, '0')}`;
      const dayNum = parseInt(day);
      const suffix = dayNum === 1 ? 'st' : dayNum === 2 ? 'nd' : dayNum === 3 ? 'rd' : 'th';

      return `Monthly on the ${dayNum}${suffix} at ${time}`;
    }

    if (minute.startsWith('*/')) {
      const interval = minute.substring(2);

      return `Every ${interval} minutes`;
    }

    if (hour.startsWith('*/')) {
      const interval = hour.substring(2);

      return `Every ${interval} hours`;
    }
  }

  return `Custom schedule: ${cron}`;
}

export function formatSchedule(schedule) {
  if (!schedule) return 'Manual only';
  const preset = CRON_PRESETS.find((p) => p.value === schedule);

  return preset ? preset.label : schedule;
}
