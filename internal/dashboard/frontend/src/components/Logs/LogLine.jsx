import React from 'react';
import { useLogsStore } from '../../store/logsStore';
import clsx from 'clsx';

export default function LogLine({ log }) {
  const { showTimestamps, lineWrap, highlightErrors, searchTerm } = useLogsStore();

  const getLogLevelColor = (level) => {
    switch (level) {
      case 'ERROR':
        return 'text-red-400';
      case 'WARN':
        return 'text-yellow-400';
      case 'INFO':
        return 'text-cyan-400';
      case 'DEBUG':
        return 'text-purple-400';
      default:
        return 'text-green-400';
    }
  };

  const getLogLevelBadgeClass = (level) => {
    switch (level) {
      case 'ERROR':
        return 'bg-red-500/20 text-red-300 border-red-500/30';
      case 'WARN':
        return 'bg-yellow-500/20 text-yellow-300 border-yellow-500/30';
      case 'INFO':
        return 'bg-cyan-500/20 text-cyan-300 border-cyan-500/30';
      case 'DEBUG':
        return 'bg-purple-500/20 text-purple-300 border-purple-500/30';
      default:
        return 'bg-green-500/20 text-green-300 border-green-500/30';
    }
  };

  const getLogLevelIcon = (level) => {
    switch (level) {
      case 'ERROR':
        return (
          <path
            fillRule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
            clipRule="evenodd"
          />
        );
      case 'WARN':
        return (
          <path
            fillRule="evenodd"
            d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
            clipRule="evenodd"
          />
        );
      case 'INFO':
        return (
          <path
            fillRule="evenodd"
            d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z"
            clipRule="evenodd"
          />
        );
      case 'DEBUG':
        return (
          <>
            <path d="M10 12a2 2 0 100-4 2 2 0 000 4z" />
            <path
              fillRule="evenodd"
              d="M.458 10C1.732 5.943 5.522 3 10 3s8.268 2.943 9.542 7c-1.274 4.057-5.064 7-9.542 7S1.732 14.057.458 10zM14 10a4 4 0 11-8 0 4 4 0 018 0z"
              clipRule="evenodd"
            />
          </>
        );
      default:
        return (
          <path
            fillRule="evenodd"
            d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
            clipRule="evenodd"
          />
        );
    }
  };

  const formatLogTimestamp = (timestamp) => {
    try {
      const date = new Date(timestamp);
      const hours = date.getHours().toString().padStart(2, '0');
      const minutes = date.getMinutes().toString().padStart(2, '0');
      const seconds = date.getSeconds().toString().padStart(2, '0');
      const ms = date.getMilliseconds().toString().padStart(3, '0');
      return `${hours}:${minutes}:${seconds}.${ms}`;
    } catch (e) {
      return timestamp;
    }
  };

  const highlightSearchTerm = (text) => {
    if (!searchTerm) return text;

    const parts = text.split(new RegExp(`(${searchTerm})`, 'gi'));
    return parts.map((part, index) =>
      part.toLowerCase() === searchTerm.toLowerCase() ? (
        <span key={index} className="bg-yellow-500/30 text-yellow-200">
          {part}
        </span>
      ) : (
        part
      )
    );
  };

  return (
    <div
      className={clsx(
        'group flex items-start py-1.5 px-2 rounded transition-colors duration-100',
        'hover:bg-gray-900/50',
        log.level === 'ERROR' && highlightErrors && 'bg-red-500/5 border-l-2 border-red-500/50',
        log.level === 'WARN' && highlightErrors && 'bg-yellow-500/5 border-l-2 border-yellow-500/50'
      )}
    >
      <div
        className={clsx(
          'flex items-start space-x-3 flex-1',
          lineWrap ? 'flex-wrap' : ''
        )}
      >
        {showTimestamps && (
          <div className="text-gray-600 select-none flex-shrink-0 w-24">
            {formatLogTimestamp(log.timestamp)}
          </div>
        )}

        <div
          className={clsx(
            'inline-flex items-center px-2 py-0.5 rounded border font-medium flex-shrink-0 w-16 justify-center',
            getLogLevelBadgeClass(log.level)
          )}
        >
          <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
            {getLogLevelIcon(log.level)}
          </svg>
          <span className="text-[10px]">{log.level}</span>
        </div>

        <div
          className={clsx(
            'flex-1 min-w-0',
            getLogLevelColor(log.level),
            lineWrap ? 'break-all' : 'truncate'
          )}
        >
          {highlightSearchTerm(log.message)}
        </div>
      </div>
    </div>
  );
}
