import React from 'react';
import useMemoryStore from '../../store/memoryStore';

const AnalyticsView = () => {
  const { stats } = useMemoryStore();

  const statCards = [
    {
      title: 'Total Entities',
      value: stats.totalEntities,
      icon: (
        <path d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
      ),
      color: 'purple',
    },
    {
      title: 'Relationships',
      value: stats.totalRelations,
      icon: (
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth="2"
          d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
        />
      ),
      color: 'blue',
    },
    {
      title: 'Entity Types',
      value: Object.keys(stats.entityTypes).length,
      icon: (
        <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4z M3 10a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H4a1 1 0 01-1-1v-6z M14 9a1 1 0 00-1 1v6a1 1 0 001 1h2a1 1 0 001-1v-6a1 1 0 00-1-1h-2z" />
      ),
      color: 'green',
    },
    {
      title: 'Relation Types',
      value: Object.keys(stats.relationTypes).length,
      icon: (
        <path
          fillRule="evenodd"
          d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3z"
        />
      ),
      color: 'yellow',
    },
  ];

  const colorClasses = {
    purple: {
      bg: 'bg-purple-500',
      text: 'text-purple-500',
      lightBg: 'bg-purple-100 dark:bg-purple-900/20',
      border: 'border-purple-200 dark:border-purple-800',
    },
    blue: {
      bg: 'bg-blue-500',
      text: 'text-blue-500',
      lightBg: 'bg-blue-100 dark:bg-blue-900/20',
      border: 'border-blue-200 dark:border-blue-800',
    },
    green: {
      bg: 'bg-green-500',
      text: 'text-green-500',
      lightBg: 'bg-green-100 dark:bg-green-900/20',
      border: 'border-green-200 dark:border-green-800',
    },
    yellow: {
      bg: 'bg-yellow-500',
      text: 'text-yellow-500',
      lightBg: 'bg-yellow-100 dark:bg-yellow-900/20',
      border: 'border-yellow-200 dark:border-yellow-800',
    },
  };

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {statCards.map((stat) => {
          const colors = colorClasses[stat.color];
          return (
            <div key={stat.title} className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
              <div className="flex items-center">
                <div className="flex-shrink-0">
                  <div
                    className={`w-12 h-12 ${colors.lightBg} border ${colors.border} rounded-lg flex items-center justify-center`}
                  >
                    <svg className={`w-6 h-6 ${colors.text}`} fill="currentColor" viewBox="0 0 20 20">
                      {stat.icon}
                    </svg>
                  </div>
                </div>
                <div className="ml-4">
                  <p className="text-sm font-medium text-gray-600 dark:text-gray-400">
                    {stat.title}
                  </p>
                  <p className="text-2xl font-bold text-gray-900 dark:text-white">{stat.value}</p>
                </div>
              </div>
            </div>
          );
        })}
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">Entity Types</h3>
          {Object.keys(stats.entityTypes).length === 0 ? (
            <div className="text-center text-gray-500 dark:text-gray-400 py-8">
              No entity types to display
            </div>
          ) : (
            <div className="space-y-3">
              {Object.entries(stats.entityTypes).map(([type, count]) => {
                const percentage = (count / stats.totalEntities) * 100;
                return (
                  <div
                    key={type}
                    className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
                  >
                    <span className="font-medium text-gray-900 dark:text-white">{type}</span>
                    <div className="flex items-center space-x-2">
                      <div className="w-20 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                        <div
                          className="bg-purple-500 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                      <span className="text-sm font-medium text-gray-600 dark:text-gray-300 w-12 text-right">
                        {count}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-6">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
            Relationship Types
          </h3>
          {Object.keys(stats.relationTypes).length === 0 ? (
            <div className="text-center text-gray-500 dark:text-gray-400 py-8">
              No relationship types to display
            </div>
          ) : (
            <div className="space-y-3">
              {Object.entries(stats.relationTypes).map(([type, count]) => {
                const percentage = (count / stats.totalRelations) * 100;
                return (
                  <div
                    key={type}
                    className="flex items-center justify-between p-3 bg-gray-50 dark:bg-gray-700/50 rounded-lg"
                  >
                    <span className="font-medium text-gray-900 dark:text-white">{type}</span>
                    <div className="flex items-center space-x-2">
                      <div className="w-20 bg-gray-200 dark:bg-gray-600 rounded-full h-2">
                        <div
                          className="bg-blue-500 h-2 rounded-full transition-all duration-300"
                          style={{ width: `${percentage}%` }}
                        />
                      </div>
                      <span className="text-sm font-medium text-gray-600 dark:text-gray-300 w-12 text-right">
                        {count}
                      </span>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-6">
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-4">
          Memory Distribution
        </h3>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
              Entity Statistics
            </h4>
            <div className="space-y-2">
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">Total Entities:</span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {stats.totalEntities}
                </span>
              </div>
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">Entity Types:</span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {Object.keys(stats.entityTypes).length}
                </span>
              </div>
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  Avg. per Type:
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {Object.keys(stats.entityTypes).length > 0
                    ? (
                        stats.totalEntities / Object.keys(stats.entityTypes).length
                      ).toFixed(1)
                    : '0'}
                </span>
              </div>
            </div>
          </div>
          <div>
            <h4 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-3">
              Relationship Statistics
            </h4>
            <div className="space-y-2">
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  Total Relations:
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {stats.totalRelations}
                </span>
              </div>
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  Relation Types:
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {Object.keys(stats.relationTypes).length}
                </span>
              </div>
              <div className="flex justify-between items-center p-2 bg-gray-50 dark:bg-gray-700/50 rounded">
                <span className="text-sm text-gray-600 dark:text-gray-400">
                  Connectivity:
                </span>
                <span className="text-sm font-medium text-gray-900 dark:text-white">
                  {stats.totalEntities > 0
                    ? ((stats.totalRelations / stats.totalEntities) * 100).toFixed(1)
                    : '0'}
                  %
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AnalyticsView;
