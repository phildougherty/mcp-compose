import { useState } from 'react';
import { reloadProxy } from '../../api/dashboard';
import { useToast } from '../../hooks/useToast';
import { Button, Modal } from '../shared';
import { formatRelativeTime } from '../../utils/format';

const ProxyControls = ({
  autoRefresh,
  onToggleAutoRefresh,
  refreshFrequency,
  onSetRefreshFrequency,
  lastRefreshTime,
  onRefresh,
  loading,
}) => {
  const [showRefreshDropdown, setShowRefreshDropdown] = useState(false);
  const [showProxyConfirm, setShowProxyConfirm] = useState(false);
  const [proxyLoading, setProxyLoading] = useState(false);
  const { success, error: showError } = useToast();

  const refreshOptions = [
    { value: 5000, label: '5 seconds' },
    { value: 10000, label: '10 seconds' },
    { value: 30000, label: '30 seconds' },
    { value: 60000, label: '1 minute' },
    { value: 300000, label: '5 minutes' },
  ];

  const handleReloadProxy = async () => {
    setProxyLoading(true);
    try {
      await reloadProxy();
      success('Proxy restarted successfully');
      setShowProxyConfirm(false);
      setTimeout(() => {
        if (onRefresh) onRefresh();
      }, 2000);
    } catch (err) {
      showError(`Failed to restart proxy: ${err.message}`);
      console.error('Failed to restart proxy:', err);
    } finally {
      setProxyLoading(false);
    }
  };

  const timeAgoText = lastRefreshTime ? formatRelativeTime(lastRefreshTime) : 'Never refreshed';

  return (
    <>
      <div className="relative flex items-center space-x-3">
        <Button
          onClick={onRefresh}
          disabled={loading}
          variant={autoRefresh ? 'success' : 'secondary'}
          className="relative min-h-[44px]"
        >
          <svg
            className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`}
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
          <span>{autoRefresh ? 'Auto' : 'Refresh'}</span>
          {autoRefresh && (
            <span className="absolute -top-1 -right-1 flex h-3 w-3">
              <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-300 opacity-75" />
              <span className="relative inline-flex rounded-full h-3 w-3 bg-emerald-400 ring-2 ring-white" />
            </span>
          )}
        </Button>

        <div className="relative">
          <Button
            onClick={() => setShowRefreshDropdown(!showRefreshDropdown)}
            variant="secondary"
            className="min-h-[44px] min-w-[44px] px-3"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
              />
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            </svg>
          </Button>

          {showRefreshDropdown && (
            <div className="absolute right-0 mt-3 w-72 rounded-xl shadow-2xl bg-slate-800 ring-1 ring-slate-700 border border-slate-600 z-50 backdrop-blur-sm">
              <div className="p-3 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-medium text-gray-200">Auto Refresh</span>
                  <Button
                    onClick={() => {
                      onToggleAutoRefresh();
                      setShowRefreshDropdown(false);
                    }}
                    size="sm"
                    variant={autoRefresh ? 'primary' : 'secondary'}
                  >
                    <svg className="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      {autoRefresh ? (
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                      ) : (
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z" />
                      )}
                    </svg>
                    {autoRefresh ? 'On' : 'Off'}
                  </Button>
                </div>

                {autoRefresh && (
                  <div className="space-y-1">
                    <label className="text-xs font-medium text-gray-300">Interval</label>
                    <div className="space-y-1">
                      {refreshOptions.map((option) => (
                        <button
                          key={option.value}
                          onClick={() => {
                            onSetRefreshFrequency(option.value);
                            setShowRefreshDropdown(false);
                          }}
                          className={`w-full text-left px-2 py-1 text-xs rounded transition-colors ${
                            refreshFrequency === option.value
                              ? 'bg-blue-600 text-white'
                              : 'text-gray-300 hover:bg-gray-700'
                          }`}
                        >
                          {option.label}
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                <div className="border-t border-gray-600 pt-2 space-y-1 text-xs">
                  <div className="flex justify-between text-gray-400">
                    <span>Last updated:</span>
                    <span>{timeAgoText}</span>
                  </div>
                  {autoRefresh && (
                    <div className="flex justify-between text-gray-400">
                      <span>Next update:</span>
                      <span className="text-green-400">{refreshFrequency / 1000}s</span>
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>

        <Button
          onClick={() => setShowProxyConfirm(true)}
          disabled={loading || proxyLoading}
          variant="warning"
          className="min-h-[44px]"
        >
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
          <span>Restart Proxy</span>
        </Button>
      </div>

      <Modal
        isOpen={showProxyConfirm}
        onClose={() => setShowProxyConfirm(false)}
        title="Restart Proxy"
      >
        <div className="space-y-4">
          <p className="text-gray-300">
            This will drop all active connections and reload configuration. Are you sure you want to continue?
          </p>
          <div className="flex justify-end space-x-3">
            <Button onClick={() => setShowProxyConfirm(false)} variant="secondary">
              Cancel
            </Button>
            <Button onClick={handleReloadProxy} variant="warning" loading={proxyLoading}>
              Restart Proxy
            </Button>
          </div>
        </div>
      </Modal>
    </>
  );
};

export default ProxyControls;
