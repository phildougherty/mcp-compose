import { useState, useEffect } from 'react';
import { inspectorConnect, inspectorDisconnect } from '../../api/inspector';

export function InlineInspector({ serverName, isExpanded, onToolsDiscovered }) {
  const [sessionId, setSessionId] = useState(null);
  const [loading, setLoading] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState(null);
  const [response, setResponse] = useState(null);
  const [request, setRequest] = useState('');
  const [availableMethods, setAvailableMethods] = useState([]);
  const [discoveredTools, setDiscoveredTools] = useState([]);
  const [inspectorAvailable, setInspectorAvailable] = useState(null);

  const requestTemplates = {
    'initialize': {
      method: 'initialize',
      params: {
        protocolVersion: '2024-11-05',
        capabilities: {
          resources: { listChanged: true, subscribe: true },
          tools: { listChanged: true },
          prompts: { listChanged: true }
        },
        clientInfo: {
          name: 'MCP Dashboard Inspector',
          version: '1.0.0'
        }
      }
    },
    'tools/list': { method: 'tools/list', params: {} },
    'resources/list': { method: 'resources/list', params: {} },
    'prompts/list': { method: 'prompts/list', params: {} }
  };

  useEffect(() => {
    checkInspectorAvailability();
  }, []);

  useEffect(() => {
    if (isExpanded && !connected && inspectorAvailable === true) {
      handleConnect();
    }
  }, [isExpanded, connected, inspectorAvailable]);

  const checkInspectorAvailability = async () => {
    try {
      const response = await fetch('/api/servers', { method: 'GET' });
      if (!response.ok) {
        setInspectorAvailable(false);
        return;
      }

      const testResponse = await fetch('/api/inspector/connect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ server: '__healthcheck__' })
      });

      const contentType = testResponse.headers.get('content-type');
      setInspectorAvailable(contentType?.includes('application/json') ?? false);
    } catch (err) {
      console.warn('Inspector availability check failed:', err);
      setInspectorAvailable(false);
    }
  };

  const handleConnect = async () => {
    if (connected || inspectorAvailable === false) return;

    setLoading(true);
    setError(null);

    try {
      const data = await inspectorConnect(serverName);
      setSessionId(data.sessionId);
      setConnected(true);

      discoverMethods(data.result);
      await discoverTools(data.sessionId);
    } catch (err) {
      setError(err.message);
      setConnected(false);
    } finally {
      setLoading(false);
    }
  };

  const handleDisconnect = async () => {
    if (!sessionId) return;

    try {
      await inspectorDisconnect();
    } catch (err) {
      console.warn('Failed to disconnect inspector:', err);
    }

    setSessionId(null);
    setConnected(false);
    setResponse(null);
    setError(null);
    setAvailableMethods([]);
    setDiscoveredTools([]);
  };

  const discoverMethods = (initializeResult) => {
    const methods = ['initialize', 'shutdown'];
    if (initializeResult?.capabilities) {
      const caps = initializeResult.capabilities;
      if (caps.resources) methods.push('resources/list', 'resources/get');
      if (caps.tools) methods.push('tools/list', 'tools/get');
      if (caps.prompts) methods.push('prompts/list', 'prompts/render');
    }
    setAvailableMethods(methods);
  };

  const discoverTools = async (sid) => {
    try {
      const data = await executeMethod(sid || sessionId, 'tools/list', {});
      if (data?.result?.tools) {
        setDiscoveredTools(data.result.tools);
        onToolsDiscovered?.(data.result.tools);
      }
    } catch (err) {
      console.warn(`Failed to discover tools for ${serverName}:`, err);
    }
  };

  const executeMethod = async (sid, method, params = {}) => {
    if (!sid) throw new Error('No active session');

    try {
      const response = await fetch('/api/inspector/request', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sessionId: sid,
          method: method,
          params: params
        })
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error || `Request failed: ${response.status}`);
      }

      const data = await response.json();
      setResponse(data);

      if (method === 'tools/list' && data?.result?.tools) {
        setDiscoveredTools(data.result.tools);
        onToolsDiscovered?.(data.result.tools);
      }

      return data;
    } catch (err) {
      throw err;
    }
  };

  const executeTemplate = async (templateName) => {
    const template = requestTemplates[templateName];
    if (!template) return;

    try {
      await executeMethod(sessionId, template.method, template.params);
    } catch (err) {
      setError(err.message);
    }
  };

  const executeCustomRequest = async () => {
    if (!request.trim()) return;

    try {
      const requestObj = JSON.parse(request);
      await executeMethod(sessionId, requestObj.method, requestObj.params || {});
    } catch (err) {
      setError(err.message);
    }
  };

  const loadTemplate = (templateName) => {
    const template = requestTemplates[templateName];
    if (template) {
      setRequest(JSON.stringify({ method: template.method, params: template.params }, null, 2));
    }
  };

  if (inspectorAvailable === false) {
    return null;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium text-white flex items-center">
          <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          MCP Inspector
        </h4>
        <div>
          {connected ? (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-green-900/50 text-green-200 border border-green-700/50">
              <span className="w-2 h-2 bg-green-400 rounded-full mr-2"></span>
              Connected
            </span>
          ) : loading ? (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-yellow-900/50 text-yellow-200 border border-yellow-700/50">
              Connecting...
            </span>
          ) : inspectorAvailable === null ? (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-700/50 text-slate-400 border border-slate-600/50">
              Checking...
            </span>
          ) : (
            <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium bg-slate-700/50 text-slate-400 border border-slate-600/50">
              Disconnected
            </span>
          )}
        </div>
      </div>

      {!connected && !loading && inspectorAvailable !== null && (
        <div className="text-center py-6 bg-slate-700/50 rounded-lg border border-slate-600/50">
          <svg className="mx-auto h-8 w-8 text-slate-400 mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
          </svg>
          <p className="text-sm text-slate-400 mb-3">Start an inspector session to test MCP methods</p>
          <button
            onClick={handleConnect}
            disabled={loading || inspectorAvailable === false}
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-blue-500 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.50a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            </svg>
            Connect Inspector
          </button>
        </div>
      )}

      {inspectorAvailable === null && (
        <div className="text-center py-6 bg-slate-700/50 rounded-lg border border-slate-600/50">
          <div className="w-6 h-6 mx-auto mb-3 animate-spin rounded-full border-2 border-slate-400 border-t-transparent"></div>
          <p className="text-sm text-slate-400">Checking inspector availability...</p>
        </div>
      )}

      {connected && (
        <div className="space-y-4">
          <div>
            <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">Quick Actions</h6>
            <div className="flex flex-wrap gap-2">
              {availableMethods.map((method) => (
                <button
                  key={method}
                  onClick={() => executeTemplate(method)}
                  className="inline-flex items-center px-3 py-1.5 border border-slate-600 shadow-sm text-xs font-medium rounded text-slate-300 bg-slate-700 hover:bg-slate-600 transition-colors"
                >
                  {method}
                </button>
              ))}
            </div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-2">
              <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide">Custom Request</h6>
              <select
                onChange={(e) => { loadTemplate(e.target.value); e.target.value = ''; }}
                className="text-xs border border-slate-600 rounded px-2 py-1 bg-slate-700 text-slate-300"
              >
                <option value="">Load template...</option>
                {Object.keys(requestTemplates).map((name) => (
                  <option key={name} value={name}>{name}</option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <textarea
                value={request}
                onChange={(e) => setRequest(e.target.value)}
                placeholder='{"method": "tools/list", "params": {}}'
                className="w-full h-20 px-3 py-2 border border-slate-600 rounded-lg bg-slate-700 text-white placeholder-slate-500 font-mono text-xs resize-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
              />
              <button
                onClick={executeCustomRequest}
                disabled={!request.trim()}
                className="w-full inline-flex items-center justify-center px-3 py-2 border border-transparent text-sm font-medium rounded-lg text-white bg-blue-600 hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              >
                <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                </svg>
                Send Request
              </button>
            </div>
          </div>

          {response && (
            <div className="bg-gray-900 rounded-lg p-4 max-h-64 overflow-y-auto">
              <div className="flex items-center justify-between mb-2">
                <span className="text-xs text-gray-400 font-medium">Response</span>
              </div>
              <pre className="text-sm text-green-400 font-mono whitespace-pre-wrap">{JSON.stringify(response, null, 2)}</pre>
            </div>
          )}

          {discoveredTools.length > 0 && (
            <div>
              <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide mb-2">
                Discovered Tools ({discoveredTools.length})
              </h6>
              <div className="space-y-2">
                {discoveredTools.map((tool) => (
                  <div key={tool.name} className="bg-slate-700/50 p-3 rounded-lg border border-slate-600/50">
                    <div className="flex items-center justify-between gap-2">
                      <div className="min-w-0 flex-1">
                        <div className="font-medium text-sm text-white">{tool.name}</div>
                        {tool.description && (
                          <div className="text-xs text-slate-400 mt-1 truncate">{tool.description}</div>
                        )}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {error && (
            <div className="bg-red-900/20 border border-red-500/30 rounded-lg p-3">
              <div className="text-sm text-red-400">{error}</div>
            </div>
          )}

          <div className="pt-2 border-t border-slate-700">
            <button
              onClick={handleDisconnect}
              className="w-full inline-flex items-center justify-center px-3 py-2 border border-slate-600 text-sm font-medium rounded-lg text-slate-300 bg-slate-700 hover:bg-slate-600 transition-colors"
            >
              Disconnect Inspector
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
