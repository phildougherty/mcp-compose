import { useActivityStore } from '../../store/activityStore';
import { formatRelativeTime } from '../../utils/format';
import { Badge, Button } from '../shared';
import ToolCallDetails from './ToolCallDetails';

/**
 * Activity Card Component
 *
 * Individual activity event display with:
 * - Level badge (ERROR, WARN, INFO, DEBUG)
 * - Type badge (request, connection, tool, error)
 * - Server name badge
 * - Timestamp
 * - Message
 * - Tool calls (expandable)
 * - Details (expandable)
 */
export default function ActivityCard({ activity }) {
  const expandedDetails = useActivityStore(state => state.expandedDetails[activity.id]);
  const toggleDetails = useActivityStore(state => state.toggleDetails);

  const getLevelVariant = (level) => {
    const variants = {
      'ERROR': 'danger',
      'WARN': 'warning',
      'INFO': 'info',
      'DEBUG': 'default',
    };

    return variants[level] || 'default';
  };

  const getTypeVariant = (type) => {
    const variants = {
      'request': 'success',
      'tool_call': 'primary',
      'tool': 'primary',
      'connection': 'info',
      'task': 'primary',
      'error': 'danger',
    };

    return variants[type] || 'default';
  };

  const hasDetails = activity.details && Object.keys(activity.details).length > 0;

  return (
    <div className="p-4 hover:bg-gray-50 dark:hover:bg-gray-700/30 transition-all duration-150 group min-h-[44px]">
      <div className="flex items-start justify-between">
        <div className="flex items-start gap-3 flex-1 min-w-0">
          <div className="flex-shrink-0 mt-0.5">
            <Badge variant={getLevelVariant(activity.level)} size="sm" className="w-12 justify-center font-bold">
              {activity.level}
            </Badge>
          </div>

          <div className="flex-1 min-w-0">
            <div className="flex items-center flex-wrap gap-2 mb-2">
              <Badge variant={getTypeVariant(activity.type)} size="sm">
                {activity.type || 'unknown'}
              </Badge>

              {activity.server && (
                <Badge variant="default" size="sm">
                  <svg className="w-3 h-3 mr-1.5" fill="currentColor" viewBox="0 0 20 20">
                    <path
                      fillRule="evenodd"
                      d="M2 5a2 2 0 012-2h12a2 2 0 012 2v10a2 2 0 01-2 2H4a2 2 0 01-2-2V5zm3.293 1.293a1 1 0 011.414 0l3 3a1 1 0 010 1.414l-3 3a1 1 0 01-1.414-1.414L7.586 10 5.293 7.707a1 1 0 010-1.414zM11 12a1 1 0 100 2h3a1 1 0 100-2h-3z"
                      clipRule="evenodd"
                    />
                  </svg>
                  {activity.server}
                </Badge>
              )}

              <div className="inline-flex items-center text-xs text-gray-500 dark:text-gray-400">
                <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm1-12a1 1 0 10-2 0v4a1 1 0 00.293.707l2.828 2.829a1 1 0 101.415-1.415L11 9.586V6z"
                    clipRule="evenodd"
                  />
                </svg>
                {formatRelativeTime(activity.timestamp)}
              </div>
            </div>

            <div className="text-gray-900 dark:text-gray-100 text-sm leading-relaxed mb-3">
              {activity.message}
            </div>

            {activity.toolCalls && activity.toolCalls.length > 0 && (
              <div className="space-y-2">
                {activity.toolCalls.map((call, index) => (
                  <ToolCallDetails
                    key={index}
                    activityId={activity.id}
                    toolCallIndex={index}
                    toolCall={call}
                  />
                ))}
              </div>
            )}

            {hasDetails && (
              <div className="mt-3">
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => toggleDetails(activity.id)}
                >
                  <svg
                    className={`w-3 h-3 mr-1.5 transition-transform duration-200 ${expandedDetails ? 'rotate-90' : ''}`}
                    fill="currentColor"
                    viewBox="0 0 20 20"
                  >
                    <path
                      fillRule="evenodd"
                      d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
                      clipRule="evenodd"
                    />
                  </svg>
                  {expandedDetails ? 'Hide Details' : 'Show Details'}
                </Button>

                {expandedDetails && (
                  <div className="mt-3 bg-gray-50 dark:bg-gray-900/60 border border-gray-200 dark:border-gray-700 rounded-lg p-4 animate-fade-in">
                    <pre className="text-xs text-gray-700 dark:text-gray-300 whitespace-pre-wrap font-mono overflow-x-auto">
                      {JSON.stringify(activity.details, null, 2)}
                    </pre>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
