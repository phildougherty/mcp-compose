/**
 * Activity Statistics Component
 *
 * Displays activity statistics cards with totals, requests, tool calls, and errors
 */
export default function ActivityStats({ stats }) {
  const statCards = [
    {
      label: 'Total',
      value: stats.total,
      color: 'blue',
      dotColor: 'bg-blue-500',
      borderHover: 'hover:border-blue-500/30',
    },
    {
      label: 'Requests',
      value: stats.requests,
      color: 'green',
      dotColor: 'bg-green-500',
      borderHover: 'hover:border-green-500/30',
    },
    {
      label: 'Tools',
      value: stats.toolCalls,
      color: 'purple',
      dotColor: 'bg-purple-500',
      borderHover: 'hover:border-purple-500/30',
    },
    {
      label: 'Errors',
      value: stats.errors,
      color: 'red',
      dotColor: 'bg-red-500',
      borderHover: 'hover:border-red-500/30',
    },
  ];

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
      {statCards.map((stat) => (
        <div
          key={stat.label}
          className={`bg-gray-50 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-600 rounded-lg p-3 ${stat.borderHover} transition-all duration-200`}
        >
          <div className="flex items-center gap-2 mb-1">
            <div className={`w-2 h-2 ${stat.dotColor} rounded-full`} />
            <div className="text-xs font-medium text-gray-600 dark:text-gray-400 uppercase tracking-wider">
              {stat.label}
            </div>
          </div>
          <div className="text-2xl font-bold text-gray-900 dark:text-white">{stat.value}</div>
        </div>
      ))}
    </div>
  );
}
