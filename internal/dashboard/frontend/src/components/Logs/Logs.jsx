import React, { useEffect } from 'react';
import { useLogsStore } from '../../store/logsStore';
import { useWebSocket } from '../../hooks';
import { useToast } from '../../hooks';
import TerminalWindow from './TerminalWindow';
import LogControls from './LogControls';
import LogStats from './LogStats';

export default function Logs({ servers = [] }) {
  const {
    logs,
    selectedServer,
    loading,
    streaming,
    error,
    setLogs,
    setSelectedServer,
    setLoading,
    setStreaming,
    setError,
    addLog,
    clearLogs,
  } = useLogsStore();

  const { success, error: showError, info } = useToast();

  const detectLogLevel = (message) => {
    const msg = message.toLowerCase();
    if (msg.includes('error') || msg.includes('failed') || msg.includes('exception') || msg.includes('fatal')) return 'ERROR';
    if (msg.includes('warn') || msg.includes('warning')) return 'WARN';
    if (msg.includes('debug') || msg.includes('trace')) return 'DEBUG';
    return 'INFO';
  };

  const loadLogs = async () => {
    if (!selectedServer) return;

    setLoading(true);
    setError(null);

    try {
      const response = await fetch(`/api/containers/mcp-compose-${selectedServer}/logs?tail=100`);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      const data = await response.json();
      if (data.logs && Array.isArray(data.logs)) {
        const parsedLogs = data.logs.map((line, index) => ({
          id: Date.now() + index,
          timestamp: new Date().toISOString(),
          server: selectedServer,
          level: detectLogLevel(line),
          message: line,
          raw: line
        }));
        setLogs(parsedLogs);
        success('Logs loaded successfully');
      } else {
        setLogs([]);
        console.warn('No logs data received or logs is not an array:', data);
      }
    } catch (err) {
      console.error('Failed to load logs:', err);
      setError(err.message);
      showError(`Failed to load logs: ${err.message}`);
    } finally {
      setLoading(false);
    }
  };

  const protocol = typeof window !== 'undefined' && window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const wsUrl = selectedServer && typeof window !== 'undefined'
    ? `${protocol}//${window.location.host}/ws/logs?server=${selectedServer}`
    : null;

  const { isConnected: wsConnected, send, disconnect } = useWebSocket(
    wsUrl,
    {
      autoConnect: false,
      onOpen: () => {
        setStreaming(true);
        success('Log streaming started');
      },
      onClose: () => {
        setStreaming(false);
        info('Log streaming stopped');
      },
      onError: (err) => {
        console.error('WebSocket error:', err);
        setError('WebSocket connection error');
        setStreaming(false);
        showError('Log streaming error');
      },
      onMessage: (data) => {
        try {
          const logMessage = data;
          addLog({
            id: Date.now(),
            timestamp: logMessage.timestamp || new Date().toISOString(),
            server: logMessage.server || selectedServer,
            level: logMessage.level || detectLogLevel(logMessage.message),
            message: logMessage.message,
            raw: logMessage.message
          });
        } catch (err) {
          console.error('Failed to parse log message:', err);
        }
      },
    }
  );

  useEffect(() => {
    if (selectedServer) {
      loadLogs();
    }

    return () => {
      if (streaming) {
        disconnect();
      }
    };
  }, [selectedServer]);

  const handleServerChange = (server) => {
    if (streaming) {
      disconnect();
    }
    setSelectedServer(server);
  };

  const handleToggleStreaming = () => {
    if (!selectedServer) return;

    if (streaming) {
      disconnect();
    } else {
      send({ action: 'start' });
    }
  };

  const handleClearLogs = () => {
    clearLogs();
    info('Logs cleared');
  };

  const handleDownloadLogs = () => {
    const logsText = logs.map(log =>
      `[${log.timestamp}] [${log.level}] [${log.server}] ${log.message}`
    ).join('\n');

    const blob = new Blob([logsText], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${selectedServer}-logs-${new Date().toISOString().split('T')[0]}.txt`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    success('Logs downloaded');
  };

  return (
    <div className="space-y-4 animate-fade-in">
      <LogControls
        servers={servers}
        selectedServer={selectedServer}
        onServerChange={handleServerChange}
        streaming={streaming}
        loading={loading}
        onToggleStreaming={handleToggleStreaming}
        onRefresh={loadLogs}
        onClear={handleClearLogs}
        onDownload={handleDownloadLogs}
        logsCount={logs.length}
      />

      {error && (
        <div className="bg-red-500/10 border border-red-500/50 rounded-lg p-4">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg className="h-5 w-5 text-red-400" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clipRule="evenodd" />
              </svg>
            </div>
            <div className="ml-3 flex-1">
              <h3 className="text-sm font-medium text-red-300">Error loading logs</h3>
              <div className="mt-2 text-sm text-red-400 font-mono">{error}</div>
              <button
                onClick={() => setError(null)}
                className="mt-3 text-sm text-red-400 hover:text-red-300 underline"
              >
                Dismiss
              </button>
            </div>
          </div>
        </div>
      )}

      <TerminalWindow />

      <LogStats />
    </div>
  );
}
