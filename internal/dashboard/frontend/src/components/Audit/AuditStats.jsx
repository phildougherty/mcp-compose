/**
 * AuditStats Component - Display audit statistics
 */

import React from 'react';
import { Card } from '../shared';

function AuditStats({ stats }) {
  const statusCounts = {
    total: stats.total_entries || 0,
    success: stats.success_count || 0,
    failure: stats.failure_count || 0,
    rate: stats.success_rate || 0,
  };

  const statCards = [
    {
      label: 'Total Events',
      value: statusCounts.total.toLocaleString(),
      icon: (
        <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
      ),
      bgColor: 'bg-blue-500',
    },
    {
      label: 'Success Rate',
      value: `${statusCounts.rate.toFixed(1)}%`,
      icon: (
        <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
      bgColor: 'bg-green-500',
    },
    {
      label: 'Successful',
      value: statusCounts.success.toLocaleString(),
      icon: <div className="w-2 h-2 bg-white rounded-full" />,
      bgColor: 'bg-emerald-500',
    },
    {
      label: 'Failed',
      value: statusCounts.failure.toLocaleString(),
      icon: (
        <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      ),
      bgColor: 'bg-red-500',
    },
  ];

  return (
    <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
      {statCards.map((stat, index) => (
        <Card key={index} padding="md">
          <div className="flex items-center gap-3">
            <div className={`w-10 h-10 ${stat.bgColor} rounded-lg flex items-center justify-center`}>
              {stat.icon}
            </div>
            <div>
              <p className="text-2xl font-bold text-gray-900 dark:text-gray-100">{stat.value}</p>
              <p className="text-xs text-gray-600 dark:text-gray-400">{stat.label}</p>
            </div>
          </div>
        </Card>
      ))}
    </div>
  );
}

export default AuditStats;
