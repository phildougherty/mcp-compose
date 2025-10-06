import { startServer, stopServer, restartServer } from '../../api/dashboard';
import { useToast } from '../../hooks/useToast';
import { Button } from '../shared';

const ServerActions = ({ server, loading, onActionComplete }) => {
  const { success, error: showError } = useToast();

  const isContainerRunning = () => {
    if (!server.containerStatus) return false;
    const status = server.containerStatus.toLowerCase().trim();
    return status === 'running' || status === 'up' || status.includes('up ');
  };

  const handleServerAction = async (action, serverName) => {
    try {
      let result;
      if (action === 'start') {
        result = await startServer(serverName);
      } else if (action === 'stop') {
        result = await stopServer(serverName);
      } else if (action === 'restart') {
        result = await restartServer(serverName);
      }

      success(`Server ${serverName} ${action}ed successfully`);

      if (onActionComplete) {
        setTimeout(() => onActionComplete(), 2000);
      }
    } catch (err) {
      const errorMsg = `Failed to ${action} server ${serverName}: ${err.message}`;
      showError(errorMsg);
      console.error(errorMsg, err);
    }
  };

  const handleViewLogs = () => {
    console.log('View logs for', server.name);
  };

  const running = isContainerRunning();

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3">
        {!running && (
          <Button
            onClick={() => handleServerAction('start', server.name)}
            disabled={loading}
            variant="success"
            className="min-h-[48px]"
          >
            <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM9.555 7.168A1 1 0 008 8v4a1 1 0 001.555.832l3-2a1 1 0 000-1.664l-3-2z"
                clipRule="evenodd"
              />
            </svg>
            Start Server
          </Button>
        )}

        {running && (
          <Button
            onClick={() => handleServerAction('stop', server.name)}
            disabled={loading}
            variant="danger"
            className="min-h-[48px]"
          >
            <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 00-1 1v4a1 1 0 001 1h4a1 1 0 001-1V8a1 1 0 00-1-1H8z"
                clipRule="evenodd"
              />
            </svg>
            Stop Server
          </Button>
        )}

        {running && (
          <Button
            onClick={() => handleServerAction('restart', server.name)}
            disabled={loading}
            variant="warning"
            className="min-h-[48px]"
          >
            <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
              <path
                fillRule="evenodd"
                d="M4 2a1 1 0 011 1v2.101a7.002 7.002 0 0111.601 2.566 1 1 0 11-1.885.666A5.002 5.002 0 005.999 7H9a1 1 0 010 2H4a1 1 0 01-1-1V3a1 1 0 011-1zm.008 9.057a1 1 0 011.276.61A5.002 5.002 0 0014.001 13H11a1 1 0 110-2h5a1 1 0 011 1v5a1 1 0 11-2 0v-2.101a7.002 7.002 0 01-11.601-2.566 1 1 0 01.61-1.276z"
                clipRule="evenodd"
              />
            </svg>
            Restart
          </Button>
        )}

        <Button onClick={handleViewLogs} variant="secondary" className="min-h-[48px]">
          <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
            <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
          </svg>
          View Logs
        </Button>
      </div>
    </div>
  );
};

export default ServerActions;
