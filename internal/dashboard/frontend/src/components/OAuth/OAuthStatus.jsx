/**
 * OAuth Status Component
 * Displays OAuth server status and token statistics
 */

import { useState } from 'react';
import useOAuthStore from '../../store/oauthStore';
import { Badge, Button } from '../shared';
import { copyToClipboard } from '../../utils/clipboard';
import { useToast } from '../../hooks';

export default function OAuthStatus() {
  const { oauthStatus, getStatusCounts } = useOAuthStore();
  const [expanded, setExpanded] = useState(false);
  const { success } = useToast();
  const statusCounts = getStatusCounts();

  const handleCopy = (text) => {
    copyToClipboard(text);
    success('Copied to clipboard!');
  };

  return (
    <>
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
          <div className="flex items-center space-x-3">
            <div className="w-12 h-12 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-xl flex items-center justify-center">
              <svg
                className="w-6 h-6 text-blue-600 dark:text-blue-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            </div>
            <div>
              <p className="text-3xl font-bold text-gray-900 dark:text-white">{statusCounts.total}</p>
              <p className="text-xs text-gray-500 dark:text-gray-400 font-medium">Total Clients</p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
          <div className="flex items-center space-x-3">
            <div className="w-12 h-12 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-xl flex items-center justify-center">
              <div className="w-3 h-3 bg-green-600 dark:bg-green-400 rounded-full animate-pulse"></div>
            </div>
            <div>
              <p className="text-3xl font-bold text-gray-900 dark:text-white">{statusCounts.public}</p>
              <p className="text-xs text-gray-500 dark:text-gray-400 font-medium">Public</p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
          <div className="flex items-center space-x-3">
            <div className="w-12 h-12 bg-orange-50 dark:bg-orange-900/20 border border-orange-200 dark:border-orange-800 rounded-xl flex items-center justify-center">
              <svg
                className="w-6 h-6 text-orange-600 dark:text-orange-400"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                />
              </svg>
            </div>
            <div>
              <p className="text-3xl font-bold text-gray-900 dark:text-white">
                {statusCounts.confidential}
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400 font-medium">Confidential</p>
            </div>
          </div>
        </div>

        <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
          <div className="flex items-center space-x-3">
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
                  d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
                />
              </svg>
            </div>
            <div>
              <p className="text-3xl font-bold text-gray-900 dark:text-white">{statusCounts.active}</p>
              <p className="text-xs text-gray-500 dark:text-gray-400 font-medium">Active Tokens</p>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm overflow-hidden mt-4">
        <div
          onClick={() => setExpanded(!expanded)}
          className="p-6 cursor-pointer hover:bg-gray-50 dark:hover:bg-gray-700/50 transition-colors duration-200 min-h-[44px]"
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <div className="w-12 h-12 bg-blue-50 dark:bg-blue-900/20 border border-blue-200 dark:border-blue-800 rounded-xl flex items-center justify-center">
                <svg
                  className="w-6 h-6 text-blue-600 dark:text-blue-400"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"
                  />
                </svg>
              </div>
              <div>
                <h4 className="text-lg font-bold text-gray-900 dark:text-white">OAuth Server Status</h4>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  Server configuration and active tokens
                </p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              <Badge
                variant={oauthStatus.oauth_enabled ? 'success' : 'danger'}
              >
                <svg
                  className="w-4 h-4 mr-1.5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d={
                      oauthStatus.oauth_enabled
                        ? 'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z'
                        : 'M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z'
                    }
                  />
                </svg>
                {oauthStatus.oauth_enabled ? 'Enabled' : 'Disabled'}
              </Badge>
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
        </div>

        {expanded && (
          <div className="border-t border-gray-200 dark:border-gray-700 p-6 bg-gray-50 dark:bg-gray-800/50 animate-fade-in">
            {oauthStatus.oauth_enabled ? (
              <div className="space-y-6">
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <div className="text-center p-5 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm">
                    <div className="text-3xl font-bold text-blue-600 dark:text-blue-400">
                      {oauthStatus.active_tokens?.access_tokens || 0}
                    </div>
                    <div className="text-sm text-gray-600 dark:text-gray-400 font-medium mt-1">
                      Access Tokens
                    </div>
                  </div>
                  <div className="text-center p-5 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm">
                    <div className="text-3xl font-bold text-green-600 dark:text-green-400">
                      {oauthStatus.active_tokens?.refresh_tokens || 0}
                    </div>
                    <div className="text-sm text-gray-600 dark:text-gray-400 font-medium mt-1">
                      Refresh Tokens
                    </div>
                  </div>
                  <div className="text-center p-5 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm">
                    <div className="text-3xl font-bold text-yellow-600 dark:text-yellow-400">
                      {oauthStatus.active_tokens?.auth_codes || 0}
                    </div>
                    <div className="text-sm text-gray-600 dark:text-gray-400 font-medium mt-1">
                      Auth Codes
                    </div>
                  </div>
                </div>

                {oauthStatus.issuer && (
                  <div>
                    <label className="block text-sm font-bold text-gray-700 dark:text-gray-300 mb-3">
                      Issuer URL
                    </label>
                    <div className="flex rounded-lg overflow-hidden border border-gray-200 dark:border-gray-700">
                      <code className="flex-1 px-4 py-3 bg-gray-50 dark:bg-gray-900 text-sm break-all text-gray-900 dark:text-gray-100 font-mono">
                        {oauthStatus.issuer}
                      </code>
                      <Button
                        onClick={() => handleCopy(oauthStatus.issuer)}
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
                )}
              </div>
            ) : (
              <div className="text-center py-12 text-gray-500 dark:text-gray-400">
                <div className="w-20 h-20 mx-auto mb-4 bg-gray-100 dark:bg-gray-700/50 border border-gray-200 dark:border-gray-700 rounded-2xl flex items-center justify-center">
                  <svg
                    className="w-12 h-12 text-gray-400 dark:text-gray-500"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
                    />
                  </svg>
                </div>
                <p className="text-xl font-bold text-gray-900 dark:text-white">OAuth Server Disabled</p>
                <p className="text-sm mt-2">
                  OAuth authentication is not currently enabled on this server
                </p>
              </div>
            )}
          </div>
        )}
      </div>
    </>
  );
}
