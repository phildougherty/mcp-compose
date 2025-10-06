import React, { useState } from 'react';
import { Button } from '../shared';
import { copyToClipboard } from '../../utils/clipboard';

/**
 * ResponseViewer - JSON formatted response display with syntax highlighting
 */
export function ResponseViewer({ response }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      const jsonString = JSON.stringify(response, null, 2);
      await copyToClipboard(jsonString);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  if (!response) {
    return null;
  }

  const jsonString = JSON.stringify(response, null, 2);
  const isError = response.error !== undefined;

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide">
          Response
        </h6>
        <Button
          onClick={handleCopy}
          variant="ghost"
          size="sm"
          className="text-xs text-slate-400 hover:text-slate-300"
          aria-label="Copy response to clipboard"
        >
          <svg
            className="w-4 h-4 mr-1"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
            />
          </svg>
          {copied ? 'Copied!' : 'Copy'}
        </Button>
      </div>

      <div className="bg-gray-900 rounded-lg p-4 max-h-96 overflow-y-auto custom-scrollbar border border-slate-700">
        <pre
          className={`text-sm font-mono whitespace-pre-wrap break-words ${
            isError ? 'text-red-400' : 'text-green-400'
          }`}
        >
          {jsonString}
        </pre>
      </div>
    </div>
  );
}
