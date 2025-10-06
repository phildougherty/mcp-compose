import React, { useState, useEffect } from 'react';
import { Button } from '../shared';
import { TemplateSelector } from './TemplateSelector';
import { getRequestTemplates, validateMCPRequest } from '../../api/inspector';

/**
 * RequestEditor - JSON-RPC 2.0 request editor with validation
 */
export function RequestEditor({ onSubmit, disabled }) {
  const [request, setRequest] = useState('');
  const [validationError, setValidationError] = useState(null);

  const handleTemplateSelect = (templateKey) => {
    const templates = getRequestTemplates();
    const template = templates[templateKey];
    if (template) {
      setRequest(JSON.stringify(template, null, 2));
      setValidationError(null);
    }
  };

  const handleSubmit = () => {
    if (!request.trim()) {
      setValidationError('Request cannot be empty');
      return;
    }

    try {
      const parsedRequest = JSON.parse(request);
      const validation = validateMCPRequest(parsedRequest);

      if (!validation.valid) {
        setValidationError(validation.errors.join(', '));
        return;
      }

      setValidationError(null);
      onSubmit(parsedRequest);
    } catch (err) {
      setValidationError(`Invalid JSON: ${err.message}`);
    }
  };

  const handleKeyDown = (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide">
          Custom Request
        </h6>
        <TemplateSelector onSelect={handleTemplateSelect} disabled={disabled} />
      </div>

      <div className="space-y-2">
        <textarea
          value={request}
          onChange={(e) => setRequest(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder='{"jsonrpc": "2.0", "method": "tools/list", "params": {}, "id": 1}'
          disabled={disabled}
          className="w-full h-32 px-3 py-2 border border-slate-600 rounded-lg bg-slate-700 text-white placeholder-slate-500 font-mono text-xs resize-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500 disabled:opacity-50 disabled:cursor-not-allowed"
          aria-label="MCP Request JSON"
        />
        {validationError && (
          <p className="text-xs text-red-400" role="alert">
            {validationError}
          </p>
        )}
        <p className="text-xs text-slate-500">
          Press Ctrl+Enter (or Cmd+Enter) to send
        </p>
      </div>

      <Button
        onClick={handleSubmit}
        disabled={disabled || !request.trim()}
        variant="primary"
        className="w-full min-h-[44px]"
        aria-label="Send MCP request"
      >
        <svg
          className="w-4 h-4 mr-2"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8"
          />
        </svg>
        Send Request
      </Button>
    </div>
  );
}
