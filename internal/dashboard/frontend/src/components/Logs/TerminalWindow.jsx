import React, { useEffect, useRef } from 'react';
import { useLogsStore } from '../../store/logsStore';
import LogList from './LogList';
import { EmptyState } from '../shared';
import clsx from 'clsx';

export default function TerminalWindow() {
  const {
    selectedServer,
    loading,
    autoScroll,
    filteredLogs,
    logs,
  } = useLogsStore();

  const containerRef = useRef(null);
  const filtered = filteredLogs();

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [logs, autoScroll]);

  const scrollToTop = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = 0;
    }
  };

  const scrollToBottom = () => {
    if (containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  };

  return (
    <div className="bg-gray-800/50 dark:bg-gray-900/50 backdrop-blur-sm border border-gray-700 rounded-lg overflow-hidden shadow-2xl">
      <div className="bg-gradient-to-r from-gray-800 via-gray-850 to-gray-900 px-4 py-3 border-b border-gray-700">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-3">
            <div className="flex space-x-1.5">
              <div className="w-3 h-3 rounded-full bg-red-500 shadow-lg shadow-red-500/50" />
              <div className="w-3 h-3 rounded-full bg-yellow-500 shadow-lg shadow-yellow-500/50" />
              <div className="w-3 h-3 rounded-full bg-green-500 shadow-lg shadow-green-500/50" />
            </div>
            <div className="text-xs font-medium text-gray-400 font-mono">
              {selectedServer ? selectedServer : 'No server selected'}
            </div>
          </div>
          <div className="flex items-center space-x-2">
            {logs.length > 0 && (
              <>
                <button
                  onClick={scrollToTop}
                  className="min-h-[44px] min-w-[44px] flex items-center justify-center text-gray-400 hover:text-gray-300 transition-colors focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-900 rounded"
                  title="Scroll to top"
                  aria-label="Scroll to top"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 10l7-7m0 0l7 7m-7-7v18" />
                  </svg>
                </button>
                <button
                  onClick={scrollToBottom}
                  className="min-h-[44px] min-w-[44px] flex items-center justify-center text-gray-400 hover:text-gray-300 transition-colors focus:outline-none focus:ring-2 focus:ring-cyan-500 focus:ring-offset-2 focus:ring-offset-gray-900 rounded"
                  title="Scroll to bottom"
                  aria-label="Scroll to bottom"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M19 14l-7 7m0 0l-7-7m7 7V3" />
                  </svg>
                </button>
              </>
            )}
            <div className="text-xs text-gray-500 font-mono">
              {filtered.length} / {logs.length}
            </div>
          </div>
        </div>
      </div>

      <div className="bg-gray-950 relative">
        {loading && (
          <div className="absolute inset-0 flex items-center justify-center bg-gray-950/90 z-10 backdrop-blur-sm">
            <div className="text-center">
              <div className="animate-spin rounded-full h-10 w-10 border-2 border-gray-700 border-t-cyan-500 mx-auto mb-4" />
              <p className="text-gray-400 font-mono text-sm">Loading logs...</p>
            </div>
          </div>
        )}

        <div
          ref={containerRef}
          className={clsx(
            'h-[600px] overflow-y-auto font-mono text-xs leading-relaxed',
            'scrollbar-thin scrollbar-thumb-gray-700 scrollbar-track-gray-900'
          )}
          style={{
            scrollbarWidth: 'thin',
            scrollbarColor: '#374151 #1f2937'
          }}
        >
          {!selectedServer ? (
            <EmptyState
              icon={
                <svg className="mx-auto h-16 w-16 text-gray-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M8 9l3 3-3 3m5 0h3M5 20h14a2 2 0 002-2V6a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                </svg>
              }
              title="Select a server to view logs"
              description="Choose from the dropdown above"
            />
          ) : filtered.length === 0 ? (
            <EmptyState
              icon={
                <svg className="mx-auto h-16 w-16 text-gray-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
                </svg>
              }
              title="No logs available"
              description={`${selectedServer} has no log entries`}
              action={{
                label: 'Refresh logs',
                onClick: () => window.location.reload()
              }}
            />
          ) : (
            <LogList logs={filtered} />
          )}
        </div>
      </div>

      <div className="bg-gradient-to-r from-gray-800 via-gray-850 to-gray-900 px-4 py-2 border-t border-gray-700">
        <div className="flex items-center justify-between text-xs font-mono">
          <div className="flex items-center space-x-4">
            <div className="flex items-center space-x-2">
              <div className="w-2 h-2 rounded-full bg-cyan-400" />
              <span className="text-gray-400">{logs.length} total</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
