import React, { useState } from 'react';
import {
  XMarkIcon,
  ArrowTopRightOnSquareIcon,
  CodeBracketIcon,
  CheckCircleIcon,
  ExclamationTriangleIcon,
} from '@heroicons/react/24/outline';
import Spinner from '../shared/Spinner';

const ServerDetails = ({ server, open, onClose, onInstall, onUninstall, installed }) => {
  const [installing, setInstalling] = useState(false);
  const [customConfig, setCustomConfig] = useState({});

  if (!open || !server) return null;

  const handleInstall = async () => {
    setInstalling(true);
    try {
      await onInstall(server.id, customConfig);
    } finally {
      setInstalling(false);
    }
  };

  const handleUninstall = async () => {
    setInstalling(true);
    try {
      await onUninstall(server.id);
    } finally {
      setInstalling(false);
    }
  };

  let configTemplate = {};
  try {
    if (typeof server.configTemplate === 'string') {
      configTemplate = JSON.parse(server.configTemplate);
    } else {
      configTemplate = server.configTemplate;
    }
  } catch (e) {
    console.error('Failed to parse config template:', e);
  }

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      <div className="flex min-h-screen items-center justify-center p-4">
        <div className="fixed inset-0 bg-black bg-opacity-50 transition-opacity" onClick={onClose} />

        <div className="relative bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-3xl w-full max-h-[90vh] overflow-y-auto">
          <div className="sticky top-0 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-6 py-4 flex items-center justify-between z-10">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
              {server.displayName}
            </h2>
            <button
              onClick={onClose}
              className="text-gray-400 hover:text-gray-500 dark:hover:text-gray-300"
            >
              <XMarkIcon className="h-6 w-6" />
            </button>
          </div>

          <div className="px-6 py-6 space-y-6">
            <div>
              <div className="flex items-center gap-2 mb-2">
                {installed && (
                  <span className="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-green-100 text-green-800 dark:bg-green-900/20 dark:text-green-400">
                    <CheckCircleIcon className="h-3 w-3" />
                    Installed
                  </span>
                )}
                {server.featured && (
                  <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-yellow-100 text-yellow-800 dark:bg-yellow-900/20 dark:text-yellow-400">
                    Featured
                  </span>
                )}
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  by {server.author || 'Unknown'}
                </span>
                <span className="text-sm text-gray-500 dark:text-gray-400">
                  v{server.version || '1.0.0'}
                </span>
              </div>
              <p className="text-gray-700 dark:text-gray-300">
                {server.description}
              </p>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Category</dt>
                <dd className="mt-1 text-sm text-gray-900 dark:text-white capitalize">{server.category}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Protocol</dt>
                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{server.protocol}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Downloads</dt>
                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{server.downloads || 0}</dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">Rating</dt>
                <dd className="mt-1 text-sm text-gray-900 dark:text-white">{server.rating?.toFixed(1) || '0.0'} / 5.0</dd>
              </div>
            </div>

            {server.capabilities && server.capabilities.length > 0 && (
              <div>
                <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Capabilities</h3>
                <div className="flex flex-wrap gap-2">
                  {server.capabilities.map((cap, index) => (
                    <span
                      key={index}
                      className="inline-flex items-center px-3 py-1 rounded-full text-sm font-medium bg-blue-100 text-blue-800 dark:bg-blue-900/20 dark:text-blue-400"
                    >
                      {cap}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {server.tags && server.tags.length > 0 && (
              <div>
                <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-2">Tags</h3>
                <div className="flex flex-wrap gap-2">
                  {server.tags.map((tag, index) => (
                    <span
                      key={index}
                      className="inline-block px-2 py-1 rounded text-xs bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-400"
                    >
                      {tag}
                    </span>
                  ))}
                </div>
              </div>
            )}

            <div>
              <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-2 flex items-center gap-2">
                <CodeBracketIcon className="h-5 w-5" />
                Configuration Template
              </h3>
              <pre className="bg-gray-50 dark:bg-gray-900 rounded-lg p-4 text-xs overflow-x-auto">
                <code className="text-gray-800 dark:text-gray-200">
                  {JSON.stringify(configTemplate, null, 2)}
                </code>
              </pre>
            </div>

            {configTemplate.env && Object.keys(configTemplate.env).length > 0 && (
              <div className="rounded-lg bg-yellow-50 dark:bg-yellow-900/20 p-4">
                <div className="flex">
                  <ExclamationTriangleIcon className="h-5 w-5 text-yellow-400" aria-hidden="true" />
                  <div className="ml-3">
                    <h3 className="text-sm font-medium text-yellow-800 dark:text-yellow-400">
                      Environment Variables Required
                    </h3>
                    <div className="mt-2 text-sm text-yellow-700 dark:text-yellow-300">
                      <p>This server requires the following environment variables:</p>
                      <ul className="list-disc list-inside mt-1">
                        {Object.keys(configTemplate.env).map((key) => (
                          <li key={key}><code className="text-xs">{key}</code></li>
                        ))}
                      </ul>
                      <p className="mt-2">Make sure these are set in your environment before installing.</p>
                    </div>
                  </div>
                </div>
              </div>
            )}

            <div className="flex gap-3">
              {server.repositoryUrl && (
                <a
                  href={server.repositoryUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  <ArrowTopRightOnSquareIcon className="h-4 w-4" />
                  View Repository
                </a>
              )}
              {server.documentationUrl && (
                <a
                  href={server.documentationUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700"
                >
                  <ArrowTopRightOnSquareIcon className="h-4 w-4" />
                  Documentation
                </a>
              )}
            </div>
          </div>

          <div className="sticky bottom-0 bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 px-6 py-4 flex items-center justify-end gap-3">
            <button
              onClick={onClose}
              disabled={installing}
              className="px-4 py-2 rounded-lg border border-gray-300 dark:border-gray-600 text-sm font-medium text-gray-700 dark:text-gray-300 hover:bg-white dark:hover:bg-gray-800 disabled:opacity-50"
            >
              Cancel
            </button>
            {installed ? (
              <button
                onClick={handleUninstall}
                disabled={installing}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-red-600 text-white text-sm font-medium hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {installing ? (
                  <>
                    <Spinner size="sm" />
                    Uninstalling...
                  </>
                ) : (
                  'Uninstall'
                )}
              </button>
            ) : (
              <button
                onClick={handleInstall}
                disabled={installing}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-600 text-white text-sm font-medium hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                {installing ? (
                  <>
                    <Spinner size="sm" />
                    Installing...
                  </>
                ) : (
                  'Install'
                )}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ServerDetails;
