import React from 'react';
import {
  CheckCircleIcon,
  TagIcon,
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';

const ServerCard = ({ server, installed, featured, onClick }) => {
  const getCategoryColor = (category) => {
    const colors = {
      filesystem: 'bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400',
      database: 'bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400',
      search: 'bg-purple-100 text-purple-800 dark:bg-purple-900/20 dark:text-purple-400',
      productivity: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400',
      development: 'bg-red-100 text-red-800 dark:bg-red-900/20 dark:text-red-400',
      ai: 'bg-pink-100 text-pink-800 dark:bg-pink-900/20 dark:text-pink-400',
      communication: 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900/20 dark:text-indigo-400',
      storage: 'bg-orange-100 text-orange-800 dark:bg-orange-900/20 dark:text-orange-400',
    };
    return colors[category] || 'bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400';
  };

  return (
    <div
      onClick={onClick}
      className="group relative bg-white dark:bg-gray-800 rounded-lg border border-gray-200 dark:border-gray-700 p-5 hover:border-blue-500 dark:hover:border-blue-500 hover:shadow-lg transition-all cursor-pointer touch-manipulation active:scale-95 min-h-[200px]"
      style={{ WebkitTapHighlightColor: 'transparent' }}
    >
      {featured && (
        <div className="absolute -top-2 -right-2">
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400 shadow-sm">
            <StarIconSolid className="h-3 w-3" />
            Featured
          </span>
        </div>
      )}

      {installed && (
        <div className="absolute -top-2 -left-2">
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400 shadow-sm">
            <CheckCircleIcon className="h-3 w-3" />
            Installed
          </span>
        </div>
      )}

      <div className="flex items-start justify-between mb-3">
        <div className="flex-1 min-w-0">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white truncate group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors">
            {server.displayName}
          </h3>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            {server.author || 'Unknown'}
          </p>
        </div>
      </div>

      <p className="text-sm text-gray-600 dark:text-gray-300 mb-4 line-clamp-2">
        {server.description}
      </p>

      <div className="flex flex-wrap gap-2 mb-4">
        <span className={`inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs font-medium ${getCategoryColor(server.category)}`}>
          <TagIcon className="h-3 w-3" />
          {server.category}
        </span>
        {server.protocol && (
          <span className="inline-flex items-center px-2 py-1 rounded-md text-xs font-medium bg-gray-100 text-gray-800 dark:bg-gray-700 dark:text-gray-300">
            {server.protocol}
          </span>
        )}
      </div>

      <div className="flex items-center justify-end pt-4 border-t border-gray-200 dark:border-gray-700">
        <span className="text-xs text-gray-400 dark:text-gray-500">
          v{server.version || '1.0.0'}
        </span>
      </div>

      {server.tags && server.tags.length > 0 && (
        <div className="mt-3 flex flex-wrap gap-1">
          {server.tags.slice(0, 3).map((tag, index) => (
            <span
              key={index}
              className="inline-block px-2 py-0.5 rounded text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
            >
              {tag}
            </span>
          ))}
          {server.tags.length > 3 && (
            <span className="inline-block px-2 py-0.5 rounded text-xs text-gray-500 dark:text-gray-400">
              +{server.tags.length - 3} more
            </span>
          )}
        </div>
      )}
    </div>
  );
};

export default ServerCard;
