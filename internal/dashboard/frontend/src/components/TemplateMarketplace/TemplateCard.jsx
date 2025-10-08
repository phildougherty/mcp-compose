import React from 'react';
import {
  ArrowDownTrayIcon,
  StarIcon,
  TagIcon,
  UserIcon,
  ServerIcon,
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';
import { Card, Badge, Button } from '../shared';

const TemplateCard = ({ template, onInstall }) => {
  const getCategoryColor = (category) => {
    const colors = {
      'Data Engineering': 'bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400',
      'Monitoring & Alerts': 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400',
      'Content Generation': 'bg-purple-100 text-purple-800 dark:bg-purple-900/20 dark:text-purple-400',
      'Customer Support': 'bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400',
      'Marketing Automation': 'bg-pink-100 text-pink-800 dark:bg-pink-900/20 dark:text-pink-400',
      'DevOps': 'bg-orange-100 text-orange-800 dark:bg-orange-900/20 dark:text-orange-400',
    };
    return colors[category] || 'bg-gray-100 text-gray-800 dark:bg-gray-900/20 dark:text-gray-400';
  };

  const formatDownloads = (downloads) => {
    if (downloads >= 1000) {
      return `${(downloads / 1000).toFixed(1)}k`;
    }
    return downloads.toString();
  };

  const renderStars = (rating) => {
    const stars = [];
    const fullStars = Math.floor(rating);
    const hasHalfStar = rating % 1 >= 0.5;

    for (let i = 0; i < fullStars; i++) {
      stars.push(
        <StarIconSolid key={`full-${i}`} className="h-4 w-4 text-yellow-400" />
      );
    }

    if (hasHalfStar) {
      stars.push(
        <div key="half" className="relative h-4 w-4">
          <StarIcon className="absolute h-4 w-4 text-yellow-400" />
          <div className="absolute overflow-hidden" style={{ width: '50%' }}>
            <StarIconSolid className="h-4 w-4 text-yellow-400" />
          </div>
        </div>
      );
    }

    const emptyStars = 5 - stars.length;
    for (let i = 0; i < emptyStars; i++) {
      stars.push(
        <StarIcon key={`empty-${i}`} className="h-4 w-4 text-gray-300 dark:text-gray-600" />
      );
    }

    return stars;
  };

  return (
    <Card
      variant="default"
      padding="none"
      hoverable
      className="flex flex-col h-full overflow-hidden group"
    >
      <div className="relative h-40 bg-gradient-to-br from-blue-500 to-purple-600 dark:from-blue-600 dark:to-purple-700">
        <div className="absolute inset-0 flex items-center justify-center">
          <div className="text-white text-6xl font-bold opacity-20">
            {template.name.charAt(0)}
          </div>
        </div>
        <div className="absolute top-3 right-3">
          <Badge variant="default" size="sm" className="bg-white/90 dark:bg-gray-800/90">
            v{template.version}
          </Badge>
        </div>
      </div>

      <div className="flex-1 flex flex-col p-5">
        <div className="mb-3">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition-colors line-clamp-1">
            {template.name}
          </h3>
          <p className="text-sm text-gray-600 dark:text-gray-300 line-clamp-2 min-h-[2.5rem]">
            {template.description}
          </p>
        </div>

        <div className="flex items-center gap-2 mb-3">
          <span className={`inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs font-medium ${getCategoryColor(template.category)}`}>
            <TagIcon className="h-3 w-3" />
            {template.category}
          </span>
        </div>

        <div className="flex flex-wrap gap-1 mb-4">
          {template.tags.slice(0, 3).map((tag, index) => (
            <span
              key={index}
              className="inline-block px-2 py-0.5 rounded text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
            >
              {tag}
            </span>
          ))}
          {template.tags.length > 3 && (
            <span className="inline-block px-2 py-0.5 rounded text-xs text-gray-500 dark:text-gray-400">
              +{template.tags.length - 3}
            </span>
          )}
        </div>

        <div className="flex items-center gap-4 mb-4 text-sm text-gray-500 dark:text-gray-400">
          <div className="flex items-center gap-1">
            <UserIcon className="h-4 w-4" />
            <span className="truncate">{template.author}</span>
          </div>
          <div className="flex items-center gap-1">
            <ArrowDownTrayIcon className="h-4 w-4" />
            <span>{formatDownloads(template.downloads)}</span>
          </div>
        </div>

        <div className="flex items-center gap-2 mb-4">
          <div className="flex items-center gap-0.5">
            {renderStars(template.rating)}
          </div>
          <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
            {template.rating.toFixed(1)}
          </span>
        </div>

        <div className="mt-auto pt-4 border-t border-gray-200 dark:border-gray-700">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-1 text-xs text-gray-500 dark:text-gray-400">
              <ServerIcon className="h-4 w-4" />
              <span>{template.requiredServers.length} servers required</span>
            </div>
          </div>
          <Button
            onClick={() => onInstall(template)}
            variant="primary"
            size="md"
            className="w-full gap-2"
          >
            <ArrowDownTrayIcon className="h-4 w-4" />
            Install Template
          </Button>
        </div>
      </div>
    </Card>
  );
};

export default TemplateCard;
