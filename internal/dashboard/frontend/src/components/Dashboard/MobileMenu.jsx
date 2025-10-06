import { reloadProxy } from '../../api/dashboard';
import { useToast } from '../../hooks/useToast';
import { Button } from '../shared';

const MobileMenu = ({ isOpen, onClose, autoRefresh, onToggleAutoRefresh, onRefresh, loading }) => {
  const { success, error: showError } = useToast();

  const handleReloadProxy = async () => {
    const confirmed = window.confirm(
      'Restart Proxy?\n\nThis will drop all active connections and reload configuration.'
    );
    if (!confirmed) return;

    try {
      await reloadProxy();
      success('Proxy restarted successfully');
      onClose();
      setTimeout(() => {
        if (onRefresh) onRefresh();
      }, 2000);
    } catch (err) {
      showError(`Failed to restart proxy: ${err.message}`);
      console.error('Failed to restart proxy:', err);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="md:hidden bg-gray-800 border-b border-gray-700 fixed top-14 left-0 right-0 z-50">
      <div className="px-4 py-3 space-y-3">
        <div className="space-y-2">
          <Button
            onClick={() => {
              onRefresh();
              onClose();
            }}
            disabled={loading}
            variant={autoRefresh ? 'success' : 'secondary'}
            fullWidth
            className="min-h-[48px]"
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
            {autoRefresh ? 'Auto Refresh' : 'Refresh'}
          </Button>

          <Button
            onClick={() => {
              handleReloadProxy();
            }}
            disabled={loading}
            variant="warning"
            fullWidth
            className="min-h-[48px]"
          >
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
              />
            </svg>
            Restart Proxy
          </Button>
        </div>

        <div className="flex items-center justify-between py-2 border-t border-gray-700">
          <span className="text-sm font-medium text-gray-200">Auto Refresh</span>
          <button
            onClick={onToggleAutoRefresh}
            className={`inline-flex items-center px-3 py-1.5 text-sm font-medium rounded-md border transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[44px] ${
              autoRefresh
                ? 'bg-blue-600 text-white border-blue-600'
                : 'bg-gray-600 text-gray-300 border-gray-600 hover:bg-gray-500'
            }`}
          >
            <svg className="w-4 h-4 mr-1.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              {autoRefresh ? (
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              ) : (
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
                />
              )}
            </svg>
            {autoRefresh ? 'On' : 'Off'}
          </button>
        </div>
      </div>
    </div>
  );
};

export default MobileMenu;
