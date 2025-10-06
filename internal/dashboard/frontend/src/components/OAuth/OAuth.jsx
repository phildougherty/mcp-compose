/**
 * OAuth Configuration Component
 * Main component for managing OAuth 2.1 server configuration
 */

import { useState, useEffect, useRef } from 'react';
import useOAuthStore from '../../store/oauthStore';
import { getOAuthStatus, getOAuthClients } from '../../api/oauth';
import { useToast } from '../../hooks';
import { Button, Spinner } from '../shared';
import OAuthStatus from './OAuthStatus';
import EndpointList from './EndpointList';
import ClientList from './ClientList';
import ClientForm from './ClientForm';
import ClientDetails from './ClientDetails';
import TestFlows from './TestFlows';

export default function OAuth() {
  const {
    oauthStatus,
    clients,
    loading,
    error,
    autoRefresh,
    setOAuthStatus,
    setClients,
    setLoading,
    setError,
    setAutoRefresh,
  } = useOAuthStore();

  const [showCreateClient, setShowCreateClient] = useState(false);
  const [showClientDetails, setShowClientDetails] = useState(null);
  const refreshIntervalRef = useRef(null);
  const { success: showSuccessToast, error: showErrorToast, warning: showWarningToast } = useToast();

  useEffect(() => {
    loadData();
  }, []);

  useEffect(() => {
    setupAutoRefresh();

    return () => {
      if (refreshIntervalRef.current) {
        clearInterval(refreshIntervalRef.current);
      }
    };
  }, [autoRefresh]);

  const loadData = async () => {
    setLoading(true);
    setError(null);

    try {
      const [statusRes, clientsRes] = await Promise.all([
        getOAuthStatus().catch((err) => {
          console.warn('OAuth status endpoint not available:', err);

          return { oauth_enabled: false, active_tokens: {} };
        }),
        getOAuthClients().catch((err) => {
          console.warn('OAuth clients endpoint not available:', err);

          return [];
        }),
      ]);

      setOAuthStatus(statusRes);
      setClients(Array.isArray(clientsRes) ? clientsRes : []);

      if (!statusRes.oauth_enabled && !Array.isArray(clientsRes)) {
        showWarningToast('OAuth endpoints not available');
      }
    } catch (err) {
      const errorMessage = `Failed to load OAuth data: ${err.message}`;
      setError(errorMessage);
      showErrorToast(errorMessage);

      setOAuthStatus({ oauth_enabled: false, active_tokens: {} });
      setClients([]);
    } finally {
      setLoading(false);
    }
  };

  const setupAutoRefresh = () => {
    if (refreshIntervalRef.current) {
      clearInterval(refreshIntervalRef.current);
      refreshIntervalRef.current = null;
    }

    if (autoRefresh) {
      refreshIntervalRef.current = setInterval(() => {
        loadData();
      }, 30000);
    }
  };

  const handleClientCreated = (client) => {
    setShowCreateClient(false);
    setShowClientDetails(client);
    loadData();
  };

  const handleClientDeleted = () => {
    loadData();
  };

  if (loading && clients.length === 0) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="text-center">
          <Spinner size="lg" className="mb-4" />
          <p className="text-xl font-bold text-white">Loading OAuth Configuration</p>
          <p className="text-sm text-slate-400 mt-2">
            Fetching clients and server status
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in max-w-full overflow-x-hidden">
      <div className="bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-xl shadow-sm p-4">
        <div className="flex flex-col space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <div className="flex-shrink-0">
                <div className="w-10 h-10 bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-lg flex items-center justify-center">
                  <svg className="w-6 h-6 text-green-600 dark:text-green-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"
                    />
                  </svg>
                </div>
              </div>
              <div>
                <h3 className="text-lg font-semibold text-gray-900 dark:text-white flex items-center">
                  OAuth 2.1 Security
                </h3>
                <p className="text-sm text-gray-500 dark:text-gray-400">Enterprise-grade authentication and authorization management</p>
              </div>
            </div>
          </div>

          <div className="flex flex-col sm:flex-row gap-3">
            <Button
              onClick={() => setShowCreateClient(true)}
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
                  d="M12 4v16m8-8H4"
                />
              </svg>
              Register Client
            </Button>
            <Button
              onClick={loadData}
              disabled={loading}
              variant="secondary"
            >
              <svg
                className={`w-5 h-5 mr-2 ${loading ? 'animate-spin' : ''}`}
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                />
              </svg>
              Refresh
            </Button>
          </div>

          <OAuthStatus />
        </div>
      </div>

      {error && (
        <div className="bg-red-50 dark:bg-red-900/50 border-l-4 border-red-400 p-4 rounded-r-lg">
          <div className="flex items-start">
            <svg
              className="h-6 w-6 text-red-600 dark:text-red-400 mt-0.5 flex-shrink-0"
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
            <div className="ml-3 flex-1">
              <div className="text-sm text-red-800 dark:text-red-200 font-medium">{error}</div>
              <Button
                onClick={() => setError(null)}
                variant="ghost"
                size="sm"
                className="mt-2 text-red-600 dark:text-red-400 hover:text-red-700 dark:hover:text-red-300"
              >
                Dismiss
              </Button>
            </div>
          </div>
        </div>
      )}

      <EndpointList />

      <ClientList
        onViewDetails={setShowClientDetails}
        onClientDeleted={handleClientDeleted}
      />

      <TestFlows />

      {showCreateClient && (
        <ClientForm
          onClose={() => setShowCreateClient(false)}
          onClientCreated={handleClientCreated}
        />
      )}

      {showClientDetails && (
        <ClientDetails
          client={showClientDetails}
          onClose={() => setShowClientDetails(null)}
        />
      )}
    </div>
  );
}
