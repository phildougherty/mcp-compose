/**
 * OAuth Client Details Component
 * Modal displaying full client details with secret
 */

import { useState } from 'react';
import { Modal, Badge, Button } from '../shared';
import { copyToClipboard } from '../../utils/clipboard';
import { useToast } from '../../hooks';

export default function ClientDetails({ client, onClose }) {
  const [showSecret, setShowSecret] = useState(false);
  const { success } = useToast();

  const handleCopy = (text) => {
    copyToClipboard(text);
    success('Copied to clipboard!');
  };

  return (
    <Modal isOpen={true} onClose={onClose} title="Client Details" size="2xl">
      <div className="space-y-5">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-2">Name</label>
            <p className="text-white text-lg font-semibold">{client.name}</p>
          </div>
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-2">Type</label>
            <Badge
              variant={client.public ? 'info' : 'warning'}
              className="shadow-lg"
            >
              {client.public ? 'Public' : 'Confidential'}
            </Badge>
          </div>
        </div>

        {client.description && (
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-2">
              Description
            </label>
            <p className="text-white">{client.description}</p>
          </div>
        )}

        <div>
          <label className="block text-sm font-bold text-slate-400 mb-2">
            Client ID
          </label>
          <div className="flex rounded-xl overflow-hidden border-2 border-slate-600">
            <code className="flex-1 px-4 py-3 bg-slate-900 text-sm break-all text-slate-200 font-mono">
              {client.client_id}
            </code>
            <button
              onClick={() => handleCopy(client.client_id)}
              className="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm min-h-[44px] min-w-[44px] transition-all duration-200"
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
            </button>
          </div>
        </div>

        {!client.public && client.client_secret && (
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-2">
              Client Secret
            </label>
            <div className="flex rounded-xl overflow-hidden border-2 border-slate-600">
              <code className="flex-1 px-4 py-3 bg-slate-900 text-sm break-all text-slate-200 font-mono">
                {showSecret ? client.client_secret : '••••••••••••••••••••••••••••'}
              </code>
              <button
                onClick={() => setShowSecret(!showSecret)}
                className="px-4 py-3 bg-slate-700 text-white hover:bg-slate-600 text-sm min-h-[44px] min-w-[44px] transition-all duration-200"
                title={showSecret ? 'Hide secret' : 'Show secret'}
              >
                <svg
                  className="w-5 h-5"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  {showSecret ? (
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"
                    />
                  ) : (
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
                    />
                  )}
                </svg>
              </button>
              <button
                onClick={() => handleCopy(client.client_secret)}
                className="px-4 py-3 bg-blue-600 text-white hover:bg-blue-700 text-sm min-h-[44px] min-w-[44px] transition-all duration-200"
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
              </button>
            </div>
            <p className="text-xs text-yellow-300 mt-2 bg-yellow-500/10 p-3 rounded-lg border border-yellow-500/30 font-semibold">
              Store this secret securely. It won't be shown again.
            </p>
          </div>
        )}

        {client.redirect_uris?.length > 0 && (
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-3">
              Redirect URIs
            </label>
            <div className="space-y-2">
              {client.redirect_uris.map((uri, index) => (
                <code
                  key={index}
                  className="block px-4 py-3 bg-slate-900 border-2 border-slate-700 rounded-xl text-sm break-all text-slate-200 font-mono"
                >
                  {uri}
                </code>
              ))}
            </div>
          </div>
        )}

        {client.scope && (
          <div>
            <label className="block text-sm font-bold text-slate-400 mb-3">
              Scopes
            </label>
            <div className="flex flex-wrap gap-2">
              {client.scope
                .split(' ')
                .filter((s) => s)
                .map((scope) => (
                  <span
                    key={scope}
                    className="inline-flex items-center px-3 py-1.5 rounded-lg text-xs font-bold bg-slate-700 text-slate-200 border-2 border-slate-600"
                  >
                    {scope}
                  </span>
                ))}
            </div>
          </div>
        )}
      </div>

      <div className="flex justify-end gap-3 pt-6 border-t-2 border-slate-700 mt-6">
        <Button onClick={onClose} variant="secondary">
          Close
        </Button>
      </div>
    </Modal>
  );
}
