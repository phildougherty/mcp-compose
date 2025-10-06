import { useEffect, useRef } from 'react';
import ActivityCard from './ActivityCard';

/**
 * Activity List Component
 *
 * Scrollable list of activity events with auto-scroll support
 */
export default function ActivityList({ activities, autoScroll }) {
  const containerRef = useRef(null);
  const previousLengthRef = useRef(activities.length);

  useEffect(() => {
    if (autoScroll && containerRef.current && activities.length > previousLengthRef.current) {
      containerRef.current.scrollTop = 0;
    }

    previousLengthRef.current = activities.length;
  }, [activities.length, autoScroll]);

  return (
    <div
      ref={containerRef}
      className="divide-y divide-gray-200 dark:divide-gray-700 max-h-[800px] overflow-y-auto"
    >
      {activities.map((activity) => (
        <ActivityCard key={activity.id} activity={activity} />
      ))}
    </div>
  );
}
