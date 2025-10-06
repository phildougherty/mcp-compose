import React from 'react';
import { useChatStore } from '../../store/chatStore';
import clsx from 'clsx';

export default function ConnectionStatus() {
  const { isConnected } = useChatStore();

  const status = isConnected ? 'connected' : 'disconnected';

  return (
    <div
      className={clsx(
        'connection-status flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium',
        status === 'connected' && 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400',
        status === 'disconnected' && 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400'
      )}
      title={status === 'connected' ? 'Connected' : 'Disconnected'}
    >
      {status === 'connected' ? (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
        </svg>
      ) : (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <circle cx="12" cy="12" r="10" strokeWidth={2} />
        </svg>
      )}
      <span className="hidden sm:inline">{status === 'connected' ? 'Connected' : 'Disconnected'}</span>
    </div>
  );
}
