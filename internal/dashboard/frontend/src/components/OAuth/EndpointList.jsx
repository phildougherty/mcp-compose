/**
 * OAuth Endpoint List Component
 * Displays OAuth 2.1 endpoints with copy functionality
 */

import { useState } from 'react';
import { copyToClipboard } from '../../utils/clipboard';
import { useToast } from '../../hooks';
import { Button } from '../shared';

export default function EndpointList() {
  const [expanded, setExpanded] = useState(false);
  const { success } = useToast();
  const baseUrl = window.location.origin;

  const endpoints = {
    'Authorization Endpoint': '/oauth/authorize',
    'Token Endpoint': '/oauth/token',
    'Discovery Endpoint': '/.well-known/oauth-authorization-server',
  };

  const handleCopy = (text) => {
    copyToClipboard(text);
    success('Copied to clipboard!');
  };

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden">
      <div
        onClick={() => setExpanded(!expanded)}
        className="p-6 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-200 min-h-[44px]"
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <div className="w-12 h-12 bg-purple-50 dark:bg-purple-900/20 border border-purple-200 dark:border-purple-800 rounded-xl flex items-center justify-center">
              <svg
                className="w-6 h-6 text-purple-600 dark:text-purple-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
                />
              </svg>
            </div>
            <div>
              <h4 className="text-lg font-bold text-gray-900 dark:text-white">OAuth Endpoints</h4>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Available OAuth 2.1 endpoints for integration
              </p>
            </div>
          </div>
          <svg
            className={`w-6 h-6 text-gray-400 dark:text-gray-500 transition-all duration-300 ${
              expanded ? 'rotate-180' : ''
            }`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M19 9l-7 7-7-7"
            />
          </svg>
        </div>
      </div>

      {expanded && (
        <div className="border-t border-gray-200 dark:border-gray-700 p-6 bg-gray-50 dark:bg-gray-800/50 animate-fade-in">
          <div className="space-y-5">
            {Object.entries(endpoints).map(([name, endpoint]) => (
              <div key={name}>
                <label className="block text-sm font-bold text-gray-700 dark:text-gray-300 mb-3">
                  {name}
                </label>
                <div className="flex rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700">
                  <code className="flex-1 px-4 py-3 bg-gray-50 dark:bg-gray-900 text-sm break-all text-gray-900 dark:text-gray-100 font-mono">
                    {baseUrl}
                    {endpoint}
                  </code>
                  <Button
                    onClick={() => handleCopy(baseUrl + endpoint)}
                    variant="primary"
                    size="sm"
                    className="rounded-none"
                  >
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
                      />
                    </svg>
                  </Button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
