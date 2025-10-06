/**
 * OAuth Test Flows Component
 * Test authorization code flow and client credentials flow
 */

import { useState } from 'react';
import useOAuthStore from '../../store/oauthStore';
import { Select, Button } from '../shared';
import { useToast } from '../../hooks';

export default function TestFlows() {
  const { clients } = useOAuthStore();
  const [selectedClient, setSelectedClient] = useState(null);
  const { success, error: showError, warning } = useToast();

  const handleTestAuthFlow = () => {
    if (!selectedClient) {
      warning('Please select a client to test');

      return;
    }

    const state = Math.random().toString(36).substring(2, 15);
    sessionStorage.setItem('oauth_test_return', window.location.href);

    const authParams = new URLSearchParams({
      response_type: 'code',
      client_id: selectedClient.client_id,
      redirect_uri: selectedClient.redirect_uris[0],
      scope: 'mcp:tools',
      state: state,
    });

    window.location.href = `/oauth/authorize?${authParams.toString()}`;
  };

  const handleTestClientCredentials = async () => {
    if (!selectedClient) {
      warning('Please select a client to test');

      return;
    }

    if (selectedClient.public) {
      showError('Client credentials flow requires a confidential client');

      return;
    }

    try {
      const response = await fetch('/oauth/token', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: `grant_type=client_credentials&client_id=${selectedClient.client_id}&client_secret=${selectedClient.client_secret}&scope=mcp:tools`,
      });

      if (response.ok) {
        const token = await response.json();
        success('Client credentials flow successful!');
        console.log('Token:', token);
      } else {
        const errorText = await response.text();

        throw new Error(`Token request failed: ${response.status} - ${errorText}`);
      }
    } catch (err) {
      showError(`Client credentials test failed: ${err.message}`);
    }
  };

  return (
    <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-6">
      <div className="flex items-center space-x-4 mb-6">
        <div className="w-12 h-12 bg-green-50 dark:bg-green-900/20 border border-green-200 dark:border-green-800 rounded-xl flex items-center justify-center">
          <svg
            className="w-6 h-6 text-green-600 dark:text-green-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </div>
        <div>
          <h4 className="text-lg font-bold text-gray-900 dark:text-white">Test OAuth Flow</h4>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Test authentication flows with your clients
          </p>
        </div>
      </div>

      <div className="space-y-5">
        <div>
          <label className="block text-sm font-bold text-gray-700 dark:text-gray-300 mb-3">
            Test Client
          </label>
          <Select
            value={selectedClient?.client_id || ''}
            onChange={(value) => {
              const client = clients.find((c) => c.client_id === value);
              setSelectedClient(client || null);
            }}
            options={[
              { value: '', label: 'Select a client to test' },
              ...clients.map((client) => ({
                value: client.client_id,
                label: `${client.name} (${client.public ? 'Public' : 'Confidential'})`,
              })),
            ]}
            className="w-full"
          />
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <Button
            onClick={handleTestAuthFlow}
            disabled={!selectedClient}
            variant="success"
          >
            <svg
              className="w-5 h-5 mr-2"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            Test Authorization Flow
          </Button>

          <Button
            onClick={handleTestClientCredentials}
            disabled={!selectedClient || selectedClient?.public}
            variant="primary"
          >
            <svg
              className="w-5 h-5 mr-2"
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
            Test Client Credentials
          </Button>
        </div>

        {selectedClient?.public && (
          <div className="bg-yellow-50 dark:bg-yellow-900/50 border-l-4 border-yellow-400 p-4 rounded-r-lg">
            <p className="text-sm text-yellow-800 dark:text-yellow-200">
              <strong className="font-bold">Note:</strong> Client credentials flow is only
              available for confidential clients. Public clients should use the authorization
              code flow.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
