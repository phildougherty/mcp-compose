import React, { useState, useEffect } from 'react';
import { useMutation } from '../../hooks/useApi';
import { useToast } from '../../hooks/useToast';
import {
  inspectorConnect,
  inspectorRequest,
  inspectorDisconnect,
  getRequestTemplates,
} from '../../api/inspector';
import { Button, Spinner, Badge, EmptyState } from '../shared';
import { ServerSelector } from './ServerSelector';
import { RequestEditor } from './RequestEditor';
import { ResponseViewer } from './ResponseViewer';
import { ToolList } from './ToolList';

/**
 * Inspector - Main MCP Inspector interface
 * Allows testing MCP protocol methods against connected servers
 */
export function Inspector({ servers }) {
  const [selectedServer, setSelectedServer] = useState('');
  const [sessionId, setSessionId] = useState(null);
  const [response, setResponse] = useState(null);
  const [discoveredTools, setDiscoveredTools] = useState([]);
  const [availableMethods, setAvailableMethods] = useState([]);
  const [inspectorAvailable, setInspectorAvailable] = useState(null);

  const { success, error: showError, info } = useToast();

  const connectMutation = useMutation(inspectorConnect, {
    onSuccess: (data) => {
      setSessionId(data.sessionId);
      success(`Connected to ${selectedServer}`);

      if (data.result && data.result.capabilities) {
        discoverMethods(data.result);
      }

      discoverTools();
    },
    onError: (err) => {
      showError(`Failed to connect: ${err.message}`);

      if (err.message.includes('not available')) {
        setInspectorAvailable(false);
      }
    },
  });

  const requestMutation = useMutation(inspectorRequest, {
    onSuccess: (data) => {
      setResponse(data);

      if (data.result && data.result.tools) {
        setDiscoveredTools(data.result.tools);
      }
    },
    onError: (err) => {
      showError(`Request failed: ${err.message}`);
      setResponse({ error: { message: err.message } });
    },
  });

  const disconnectMutation = useMutation(inspectorDisconnect, {
    onSuccess: () => {
      setSessionId(null);
      setResponse(null);
      setDiscoveredTools([]);
      setAvailableMethods([]);
      info('Disconnected');
    },
    onError: (err) => {
      console.error('Disconnect error:', err);
    },
  });

  const checkInspectorAvailability = async () => {
    try {
      const result = await inspectorConnect('__healthcheck__');
      setInspectorAvailable(true);
    } catch (error) {
      setInspectorAvailable(false);
    }
  };

  useEffect(() => {
    checkInspectorAvailability();
  }, []);

  useEffect(() => {
    if (inspectorAvailable && servers && servers.length > 0 && !sessionId) {
      const firstServer = servers[0].name;
      setSelectedServer(firstServer);
    }
  }, [inspectorAvailable, servers, sessionId]);

  const discoverMethods = (initializeResult) => {
    const methods = ['initialize', 'shutdown'];

    if (initializeResult && initializeResult.capabilities) {
      const caps = initializeResult.capabilities;

      if (caps.resources) {
        methods.push('resources/list', 'resources/read');
      }

      if (caps.tools) {
        methods.push('tools/list', 'tools/call');
      }

      if (caps.prompts) {
        methods.push('prompts/list', 'prompts/get');
      }
    }

    setAvailableMethods(methods);
  };

  const discoverTools = async () => {
    try {
      const templates = getRequestTemplates();
      const data = await requestMutation.mutate(templates.listTools);

      if (data && data.result && data.result.tools) {
        setDiscoveredTools(data.result.tools);
      }
    } catch (error) {
      console.warn('Failed to discover tools:', error);
    }
  };

  const handleConnect = async () => {
    if (!selectedServer) {
      showError('Please select a server');
      return;
    }

    await connectMutation.mutate(selectedServer);
  };

  const handleDisconnect = async () => {
    await disconnectMutation.mutate();
  };

  const handleRequest = async (request) => {
    if (!sessionId) {
      showError('No active session');
      return;
    }

    await requestMutation.mutate({ sessionId, ...request });
  };

  const handleQuickMethod = async (method) => {
    const templates = getRequestTemplates();
    const templateKey = {
      initialize: 'initialize',
      'tools/list': 'listTools',
      'resources/list': 'listResources',
      'prompts/list': 'listPrompts',
      'tools/call': 'callTool',
    }[method];

    if (templateKey && templates[templateKey]) {
      await handleRequest(templates[templateKey]);
    }
  };

  const handleTestTool = async (tool) => {
    const templates = getRequestTemplates();
    const callToolTemplate = { ...templates.callTool };
    callToolTemplate.params.name = tool.name;
    callToolTemplate.params.arguments = {};

    await handleRequest(callToolTemplate);
  };

  if (inspectorAvailable === null) {
    return (
      <div className="p-6">
        <div className="flex items-center justify-center space-x-3 text-slate-400">
          <Spinner size="md" />
          <p className="text-sm">Checking inspector availability...</p>
        </div>
      </div>
    );
  }

  if (inspectorAvailable === false) {
    return (
      <div className="p-6">
        <EmptyState
          icon={
            <svg
              className="w-12 h-12"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          }
          title="Inspector Not Available"
          description="MCP inspection requires additional backend setup"
        />
      </div>
    );
  }

  return (
    <div className="space-y-6 p-4 sm:p-6">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-white flex items-center">
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
              d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z"
            />
          </svg>
          MCP Inspector
        </h3>
        <div>
          {sessionId ? (
            <Badge variant="success" className="flex items-center gap-2">
              <span className="w-2 h-2 bg-green-400 rounded-full" />
              Connected
            </Badge>
          ) : connectMutation.loading ? (
            <Badge variant="warning" className="flex items-center gap-2">
              <Spinner size="xs" />
              Connecting...
            </Badge>
          ) : (
            <Badge variant="secondary">Disconnected</Badge>
          )}
        </div>
      </div>

      {!sessionId && !connectMutation.loading && (
        <div className="bg-slate-700/50 rounded-lg border border-slate-600/50 p-6">
          <div className="text-center space-y-4">
            <svg
              className="mx-auto h-12 w-12 text-slate-400"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"
              />
            </svg>
            <div>
              <h4 className="text-sm font-medium text-white mb-1">
                Start Inspector Session
              </h4>
              <p className="text-sm text-slate-400">
                Select a server and connect to test MCP methods
              </p>
            </div>
            <div className="max-w-md mx-auto space-y-4">
              <ServerSelector
                servers={servers}
                selectedServer={selectedServer}
                onSelect={setSelectedServer}
                disabled={connectMutation.loading}
              />
              <Button
                onClick={handleConnect}
                disabled={!selectedServer || connectMutation.loading}
                variant="primary"
                className="w-full min-h-[44px]"
              >
                <svg
                  className="w-4 h-4 mr-2"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z"
                  />
                </svg>
                Connect Inspector
              </Button>
            </div>
          </div>
        </div>
      )}

      {sessionId && (
        <div className="space-y-6">
          {availableMethods.length > 0 && (
            <div>
              <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide mb-3">
                Quick Actions
              </h6>
              <div className="flex flex-wrap gap-2">
                {availableMethods.map((method) => (
                  <Button
                    key={method}
                    onClick={() => handleQuickMethod(method)}
                    variant="secondary"
                    size="sm"
                    disabled={requestMutation.loading}
                    className="min-h-[44px]"
                  >
                    {method}
                  </Button>
                ))}
              </div>
            </div>
          )}

          <RequestEditor
            onSubmit={handleRequest}
            disabled={requestMutation.loading}
          />

          {requestMutation.loading && (
            <div className="flex items-center justify-center space-x-3 text-slate-400 p-4">
              <Spinner size="md" />
              <p className="text-sm">Sending request...</p>
            </div>
          )}

          <ResponseViewer response={response} />

          <ToolList tools={discoveredTools} onTestTool={handleTestTool} />

          <div className="pt-4 border-t border-slate-700">
            <Button
              onClick={handleDisconnect}
              variant="secondary"
              disabled={disconnectMutation.loading}
              className="w-full min-h-[44px]"
            >
              Disconnect Inspector
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
