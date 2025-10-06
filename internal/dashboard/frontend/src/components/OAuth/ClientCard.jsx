/**
 * OAuth Client Card Component
 * Individual client display card
 */

import { Badge, Button } from '../shared';
import { deleteOAuthClient } from '../../api/oauth';
import useOAuthStore from '../../store/oauthStore';
import { useToast } from '../../hooks';

export default function ClientCard({ client, onViewDetails, onClientDeleted }) {
  const { deleteClient } = useOAuthStore();
  const { success, error: showError } = useToast();

  const handleDelete = async () => {
    const confirmed = window.confirm(
      `Delete OAuth client "${client.name}"?\n\nThis action cannot be undone and will invalidate all tokens for this client.`
    );

    if (!confirmed) return;

    try {
      await deleteOAuthClient(client.client_id);
      deleteClient(client.client_id);
      success('Client deleted successfully');
      onClientDeleted?.();
    } catch (err) {
      showError(`Failed to delete client: ${err.message}`);
    }
  };

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
      <div className="space-y-4">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            <h5 className="font-bold text-gray-900 dark:text-white truncate text-lg">{client.name}</h5>
            {client.description && (
              <p className="text-xs text-gray-500 dark:text-gray-400 mt-1 line-clamp-2">
                {client.description}
              </p>
            )}
          </div>
          <Badge
            variant={client.public ? 'info' : 'warning'}
            className="flex-shrink-0 ml-2"
          >
            <svg
              className="w-3 h-3 mr-1"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d={
                  client.public
                    ? 'M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z'
                    : 'M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z'
                }
              />
            </svg>
            {client.public ? 'Public' : 'Confidential'}
          </Badge>
        </div>

        <div>
          <label className="block text-xs font-bold text-gray-600 dark:text-gray-400 mb-2">Client ID</label>
          <code className="text-xs bg-gray-50 dark:bg-gray-900 text-gray-900 dark:text-gray-100 px-3 py-2 rounded-lg break-all block border border-gray-200 dark:border-gray-700">
            {client.client_id}
          </code>
        </div>

        {client.scope && (
          <div>
            <label className="block text-xs font-bold text-gray-600 dark:text-gray-400 mb-2">Scopes</label>
            <div className="flex flex-wrap gap-2">
              {client.scope
                .split(' ')
                .filter((s) => s)
                .map((scope) => (
                  <Badge
                    key={scope}
                    variant="default"
                    size="sm"
                  >
                    {scope}
                  </Badge>
                ))}
            </div>
          </div>
        )}

        <div className="flex flex-wrap gap-2 pt-3 border-t border-gray-200 dark:border-gray-700">
          <Button
            onClick={() => onViewDetails(client)}
            variant="primary"
            size="sm"
          >
            <svg
              className="w-4 h-4 mr-1"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 12a3 3 0 11-6 0 3 3 0 016 0z M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"
              />
            </svg>
            View
          </Button>
          <Button
            onClick={handleDelete}
            variant="danger"
            size="sm"
          >
            <svg
              className="w-4 h-4 mr-1"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
              />
            </svg>
            Delete
          </Button>
        </div>
      </div>
    </div>
  );
}
