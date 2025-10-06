import { useState } from 'react';
import { Badge } from '../shared';
import ServerActions from './ServerActions';
import { formatUptime, formatTimestamp } from '../../utils/format';
import { InlineInspector } from '../Inspector/InlineInspector';

const ServerCard = ({ server, isExpanded, onToggleExpansion, onToolsDiscovered, serverTools, loading }) => {
  const [httpConnection, setHttpConnection] = useState(null);

  const isContainerRunning = () => {
    if (!server.containerStatus) return false;
    const status = server.containerStatus.toLowerCase().trim();
    return status === 'running' || status === 'up' || status.includes('up ');
  };

  const getConnectionStatus = () => {
    if (!server.connections?.activeHttpConnectionsManagedByProxy) {
      return 'Disconnected';
    }
    const connection = server.connections.activeHttpConnectionsManagedByProxy[server.name];
    if (!connection) {
      return 'Disconnected';
    }
    setHttpConnection(connection);
    return connection.initialized && connection.rawHealthyFlag ? 'Connected' : 'Disconnected';
  };

  const isServerHealthy = () => {
    const connectionStatus = getConnectionStatus();
    if (connectionStatus === 'Connected') {
      return true;
    }
    return isContainerRunning() && server.configProtocol !== 'http';
  };

  const getServerToolCount = () => {
    if (serverTools) return serverTools.length;
    if (server.configCapabilities) return server.configCapabilities.length;
    return 0;
  };

  const getServerCapabilities = () => {
    if (httpConnection?.serverReportedCapabilities) {
      return httpConnection.serverReportedCapabilities;
    }
    return server.configCapabilities || {};
  };

  const healthStatus = isServerHealthy();
  const running = isContainerRunning();
  const connectionStatus = getConnectionStatus();
  const toolCount = getServerToolCount();

  return (
    <div
      className={`group rounded-xl bg-slate-800 border overflow-hidden transition-all duration-300 w-full max-w-full ${
        isExpanded ? 'border-blue-500 shadow-lg' : 'border-slate-700 hover:border-slate-600 hover:shadow-md'
      }`}
    >
      <div onClick={onToggleExpansion} className="p-3 sm:p-5 cursor-pointer hover:bg-slate-700/50 transition-all active:bg-slate-700">
        <div className="sm:hidden">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center space-x-3 min-w-0 flex-1">
              <div className="flex-shrink-0 relative">
                <div
                  className={`w-3 h-3 rounded-full ring-2 ring-slate-800/50 ${
                    healthStatus
                      ? 'bg-emerald-400 shadow-lg shadow-emerald-500/50'
                      : running
                      ? 'bg-blue-400 shadow-lg shadow-blue-500/50'
                      : 'bg-slate-500 shadow-lg shadow-slate-500/50'
                  }`}
                />
                {healthStatus && (
                  <div className="absolute inset-0 w-3 h-3 bg-emerald-400 rounded-full animate-ping opacity-40" />
                )}
              </div>
              <h3 className="text-base font-bold text-white truncate flex-1">{server.name}</h3>
            </div>
            <button className="text-slate-400 hover:text-white transition-all p-2 hover:bg-slate-700/50 rounded-lg -mr-2 min-h-[44px] min-w-[44px] active:scale-95 active:bg-slate-700">
              <svg
                className={`w-4 h-4 transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                  clipRule="evenodd"
                />
              </svg>
            </button>
          </div>

          {toolCount > 0 && (
            <div className="mb-2 pl-6">
              <Badge variant="purple">
                <svg className="w-2.5 h-2.5 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"
                    clipRule="evenodd"
                  />
                </svg>
                {toolCount} tool{toolCount !== 1 ? 's' : ''}
              </Badge>
            </div>
          )}

          <div className="flex items-center gap-2 text-xs text-slate-400 mb-2 pl-6">
            <span className="inline-flex items-center">
              <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                  clipRule="evenodd"
                />
              </svg>
              {server.configProtocol || 'stdio'}
            </span>
            {server.configHttpPort && (
              <span className="inline-flex items-center">
                <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M12.586 4.586a2 2 0 112.828 2.828l-3 3a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3-3a4 4 0 00-5.656-5.656l-1.5 1.5a1 1 0 101.414 1.414l1.5-1.5zm-5 5a2 2 0 012.828 0 1 1 0 101.414-1.414 4 4 0 00-5.656 0l-3 3a4 4 0 105.656 5.656l1.5-1.5a1 1 0 10-1.414-1.414l-1.5 1.5a2 2 0 11-2.828-2.828l3-3z"
                    clipRule="evenodd"
                  />
                </svg>
                Port {server.configHttpPort}
              </span>
            )}
          </div>

          <div className="flex items-center gap-2 flex-wrap pl-6">
            <Badge variant={running ? 'success' : 'error'}>
              <span className={`w-1.5 h-1.5 rounded-full mr-1.5 ${running ? 'bg-emerald-400' : 'bg-red-400'}`} />
              {server.containerStatus || 'Unknown'}
            </Badge>

            <Badge variant={connectionStatus === 'Connected' ? 'info' : 'default'}>
              <svg className="w-2.5 h-2.5 mr-1" fill="currentColor" viewBox="0 0 20 20">
                <path d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
              {connectionStatus}
            </Badge>
          </div>
        </div>

        <div className="hidden sm:flex items-center justify-between">
          <div className="flex items-center space-x-4 min-w-0 flex-1">
            <div className="flex-shrink-0 relative">
              <div
                className={`w-4 h-4 rounded-full ring-4 ring-slate-800/50 ${
                  healthStatus
                    ? 'bg-emerald-400 shadow-lg shadow-emerald-500/50'
                    : running
                    ? 'bg-blue-400 shadow-lg shadow-blue-500/50'
                    : 'bg-slate-500 shadow-lg shadow-slate-500/50'
                }`}
              />
              {healthStatus && (
                <div className="absolute inset-0 w-4 h-4 bg-emerald-400 rounded-full animate-ping opacity-40" />
              )}
            </div>

            <div className="min-w-0 flex-1">
              <div className="flex items-center flex-wrap gap-2 mb-2">
                <h3 className="text-lg font-bold text-white truncate">{server.name}</h3>
                {toolCount > 0 && (
                  <Badge variant="purple">
                    <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M11.3 1.046A1 1 0 0112 2v5h4a1 1 0 01.82 1.573l-7 10A1 1 0 018 18v-5H4a1 1 0 01-.82-1.573l7-10a1 1 0 011.12-.38z"
                        clipRule="evenodd"
                      />
                    </svg>
                    {toolCount} tool{toolCount !== 1 ? 's' : ''}
                  </Badge>
                )}
              </div>
              <div className="flex items-center flex-wrap gap-2 text-sm text-slate-400">
                <span className="inline-flex items-center">
                  <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                    <path
                      fillRule="evenodd"
                      d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z"
                      clipRule="evenodd"
                    />
                  </svg>
                  {server.configProtocol || 'stdio'}
                </span>
                {server.configHttpPort && (
                  <span className="inline-flex items-center">
                    <svg className="w-3 h-3 mr-1" fill="currentColor" viewBox="0 0 20 20">
                      <path
                        fillRule="evenodd"
                        d="M12.586 4.586a2 2 0 112.828 2.828l-3 3a2 2 0 01-2.828 0 1 1 0 00-1.414 1.414 4 4 0 005.656 0l3-3a4 4 0 00-5.656-5.656l-1.5 1.5a1 1 0 101.414 1.414l1.5-1.5zm-5 5a2 2 0 012.828 0 1 1 0 101.414-1.414 4 4 0 00-5.656 0l-3 3a4 4 0 105.656 5.656l1.5-1.5a1 1 0 10-1.414-1.414l-1.5 1.5a2 2 0 11-2.828-2.828l3-3z"
                        clipRule="evenodd"
                      />
                    </svg>
                    Port {server.configHttpPort}
                  </span>
                )}
              </div>
            </div>

            <div className="flex items-center gap-2 flex-wrap">
              <Badge variant={running ? 'success' : 'error'}>
                <span className={`w-1.5 h-1.5 rounded-full mr-2 ${running ? 'bg-emerald-400' : 'bg-red-400'}`} />
                {server.containerStatus || 'Unknown'}
              </Badge>

              <Badge variant={connectionStatus === 'Connected' ? 'info' : 'default'}>
                <svg className="w-3 h-3 mr-1.5" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
                {connectionStatus}
              </Badge>
            </div>
          </div>

          <div className="ml-4 flex items-center space-x-2">
            <button className="text-slate-400 hover:text-white transition-all p-2 hover:bg-slate-700/50 rounded-lg min-h-[44px] min-w-[44px] active:scale-95 active:bg-slate-700">
              <svg
                className={`w-5 h-5 transition-transform duration-300 ${isExpanded ? 'rotate-180' : ''}`}
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                  clipRule="evenodd"
                />
              </svg>
            </button>
          </div>
        </div>
      </div>

      {isExpanded && (
        <div className="border-t border-slate-700/50">
          <div className="p-3 sm:p-6 lg:p-8 bg-slate-900/50">
            <div className="mb-6">
              <h4 className="text-sm font-medium text-white mb-3 flex items-center">
                <svg className="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M3 3a1 1 0 000 2v8a2 2 0 002 2h2.586l-1.293 1.293a1 1 0 101.414 1.414L10 15.414l2.293 2.293a1 1 0 001.414-1.414L12.414 15H15a2 2 0 002-2V5a1 1 0 100-2H3zm11.707 4.707a1 1 0 00-1.414-1.414L10 9.586 8.707 8.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clipRule="evenodd"
                  />
                </svg>
                Server Status
              </h4>

              {httpConnection ? (
                <div className="bg-slate-700/50 border border-slate-600/50 p-3 rounded-lg space-y-2 text-sm">
                  <div className="flex justify-between items-center gap-2">
                    <span className="font-medium text-slate-400">Proxy Status:</span>
                    <Badge
                      variant={
                        httpConnection.initialized && httpConnection.rawHealthyFlag ? 'success' : 'error'
                      }
                    >
                      {connectionStatus}
                    </Badge>
                  </div>
                  <div className="flex flex-col sm:flex-row sm:justify-between sm:items-start gap-2">
                    <span className="font-medium text-slate-400 flex-shrink-0">Target URL:</span>
                    <code className="text-xs bg-slate-800/70 text-slate-300 px-2 py-1 rounded break-all overflow-x-auto max-w-full">
                      {httpConnection.targetBaseURL}
                    </code>
                  </div>
                  {httpConnection.lastUsedByProxy && (
                    <div className="flex justify-between items-center gap-2">
                      <span className="font-medium text-slate-400">Last Used:</span>
                      <span className="text-slate-300 text-xs">
                        {formatTimestamp(httpConnection.lastUsedByProxy)}
                      </span>
                    </div>
                  )}
                </div>
              ) : (
                <div className="text-center py-6 text-slate-400 bg-slate-700/30 rounded-lg border border-slate-600/30">
                  <svg className="w-8 h-8 mx-auto mb-2 text-slate-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M18.364 5.636l-3.536 3.536m0 5.656l3.536 3.536M9.172 9.172L5.636 5.636m3.536 9.192L5.636 18.364M12 12l2.828 2.828M12 12l2.828-2.828M12 12L9.172 9.172M12 12l-2.828 2.828"
                    />
                  </svg>
                  <p className="text-sm">No active proxy connection</p>
                </div>
              )}
            </div>

            <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
              <div className="bg-slate-700/50 border border-slate-600/50 p-3 rounded-lg">
                <h5 className="font-medium text-slate-300 mb-3 text-sm">Configuration</h5>
                <div className="space-y-2 text-sm">
                  <div className="flex justify-between">
                    <span className="text-slate-400">Protocol:</span>
                    <span className="text-slate-300">{server.configProtocol || 'stdio'}</span>
                  </div>
                  {server.configHttpPort && (
                    <div className="flex justify-between">
                      <span className="text-slate-400">HTTP Port:</span>
                      <span className="text-slate-300">{server.configHttpPort}</span>
                    </div>
                  )}
                  <div className="flex justify-between">
                    <span className="text-slate-400">Container:</span>
                    <span className="text-slate-300">{server.isContainer ? 'Yes' : 'No'}</span>
                  </div>
                  {server.image && (
                    <div className="flex flex-col sm:flex-row sm:justify-between gap-2">
                      <span className="text-slate-400 flex-shrink-0">Image:</span>
                      <code className="text-xs bg-slate-800/70 text-slate-300 px-2 py-1 rounded break-all overflow-x-auto max-w-full">
                        {server.image}
                      </code>
                    </div>
                  )}
                </div>
              </div>

              <div className="bg-slate-700/50 border border-slate-600/50 p-3 rounded-lg">
                <h5 className="font-medium text-slate-300 mb-3 text-sm">Tools & Capabilities</h5>
                {serverTools && serverTools.length > 0 ? (
                  <div>
                    <div className="space-y-2 mb-3">
                      {serverTools.slice(0, 3).map((tool) => (
                        <div key={tool.name} className="text-sm w-full max-w-full">
                          <div className="font-medium text-white break-words">{tool.name}</div>
                          {tool.description && (
                            <div className="text-xs text-slate-400 break-words overflow-hidden">{tool.description}</div>
                          )}
                        </div>
                      ))}
                    </div>
                    {serverTools.length > 3 && (
                      <div className="text-xs text-slate-400">+{serverTools.length - 3} more tools</div>
                    )}
                  </div>
                ) : Object.keys(getServerCapabilities()).length > 0 ? (
                  <div>
                    <div className="flex flex-wrap gap-1">
                      {Object.keys(getServerCapabilities()).map((capability) => (
                        <Badge key={capability} variant="info">
                          {capability}
                        </Badge>
                      ))}
                    </div>
                    <p className="text-xs text-slate-400 mt-2">{toolCount || 0} tools available</p>
                  </div>
                ) : (
                  <div className="text-sm text-slate-400">No capabilities reported</div>
                )}
              </div>
            </div>

            <div className="mb-6">
              <InlineInspector
                serverName={server.name}
                isExpanded={isExpanded}
                onToolsDiscovered={onToolsDiscovered}
              />
            </div>

            <ServerActions server={server} loading={loading} />
          </div>
        </div>
      )}
    </div>
  );
};

export default ServerCard;
