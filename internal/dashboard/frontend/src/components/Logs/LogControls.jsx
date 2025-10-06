import React from 'react';
import { useLogsStore } from '../../store/logsStore';
import { Button, Select, SearchInput } from '../shared';
import clsx from 'clsx';

export default function LogControls({
  servers = [],
  selectedServer,
  onServerChange,
  streaming,
  loading,
  onToggleStreaming,
  onRefresh,
  onClear,
  onDownload,
  logsCount,
}) {
  const {
    searchTerm,
    filterLevel,
    showTimestamps,
    autoScroll,
    lineWrap,
    setSearchTerm,
    setFilterLevel,
    setShowTimestamps,
    setAutoScroll,
    setLineWrap,
  } = useLogsStore();

  const serverOptions = [
    { value: '', label: 'Select server...' },
    ...servers.map(server => ({
      value: server.name,
      label: server.name
    }))
  ];

  const levelOptions = [
    { value: 'all', label: 'All Levels' },
    { value: 'ERROR', label: 'ERROR' },
    { value: 'WARN', label: 'WARN' },
    { value: 'INFO', label: 'INFO' },
    { value: 'DEBUG', label: 'DEBUG' },
  ];

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4 lg:p-6">
      <div className="flex flex-col space-y-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="flex-shrink-0">
              <div className="w-10 h-10 bg-gradient-to-br from-green-500 to-green-600 rounded-lg flex items-center justify-center shadow-md">
                <svg className="w-6 h-6 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
              </div>
            </div>
            <div>
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
                Terminal Logs
                {streaming && (
                  <span className="ml-3 inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-green-500/20 text-green-400 border border-green-500/30">
                    <span className="w-2 h-2 bg-green-400 rounded-full mr-1.5 animate-pulse" />
                    LIVE
                  </span>
                )}
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400">Real-time log streaming and monitoring</p>
            </div>
          </div>
        </div>

        <div className="flex flex-col lg:flex-row gap-3">
          <div className="flex-1">
            <SearchInput
              value={searchTerm}
              onChange={setSearchTerm}
              placeholder="Search logs..."
              className="font-mono text-sm"
            />
          </div>
          <Select
            value={typeof selectedServer === 'string' ? selectedServer : selectedServer?.name || ''}
            onChange={onServerChange}
            options={serverOptions}
            className="w-full lg:w-56"
          />
          <Select
            value={filterLevel}
            onChange={setFilterLevel}
            options={levelOptions}
            className="w-full lg:w-40"
          />
        </div>

        <div className="flex flex-wrap gap-2">
          <Button
            onClick={onToggleStreaming}
            disabled={!selectedServer}
            variant={streaming ? 'danger' : 'success'}
            className={clsx(
              'min-h-[44px] inline-flex items-center px-3 py-2 text-sm font-medium',
              streaming
                ? 'shadow-lg shadow-red-500/25'
                : 'shadow-lg shadow-green-500/25'
            )}
          >
            {streaming ? (
              <>
                <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z" clipRule="evenodd" />
                </svg>
                Stop
              </>
            ) : (
              <>
                <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                  <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z" clipRule="evenodd" />
                </svg>
                Stream
              </>
            )}
          </Button>

          <Button
            onClick={onRefresh}
            disabled={!selectedServer || loading}
            variant="secondary"
            className="min-h-[44px] inline-flex items-center px-3 py-2 text-sm font-medium"
          >
            <svg
              className={clsx('w-4 h-4 mr-2', loading && 'animate-spin')}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Refresh
          </Button>

          <Button
            onClick={onDownload}
            disabled={!logsCount}
            variant="secondary"
            className="min-h-[44px] inline-flex items-center px-3 py-2 text-sm font-medium"
          >
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            Download
          </Button>

          <Button
            onClick={onClear}
            disabled={!logsCount}
            variant="secondary"
            className="min-h-[44px] inline-flex items-center px-3 py-2 text-sm font-medium"
          >
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            Clear
          </Button>

          <div className="flex-1" />

          <div className="flex items-center space-x-4">
            <label className="inline-flex items-center cursor-pointer min-h-[44px]">
              <input
                type="checkbox"
                checked={showTimestamps}
                onChange={(e) => setShowTimestamps(e.target.checked)}
                className="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500 focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"
              />
              <span className="ml-2 text-xs text-gray-700 dark:text-gray-300">Timestamps</span>
            </label>
            <label className="inline-flex items-center cursor-pointer min-h-[44px]">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
                className="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500 focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"
              />
              <span className="ml-2 text-xs text-gray-700 dark:text-gray-300">Auto-scroll</span>
            </label>
            <label className="inline-flex items-center cursor-pointer min-h-[44px]">
              <input
                type="checkbox"
                checked={lineWrap}
                onChange={(e) => setLineWrap(e.target.checked)}
                className="form-checkbox h-4 w-4 text-cyan-600 rounded focus:ring-cyan-500 focus:ring-2 focus:ring-offset-2 focus:ring-offset-gray-900"
              />
              <span className="ml-2 text-xs text-gray-700 dark:text-gray-300">Wrap</span>
            </label>
          </div>
        </div>
      </div>
    </div>
  );
}
