import React, { useState, useEffect } from 'react';
import { Input, Select, Button } from '../../shared';

export default function MCPServerNodeConfig({ node, onUpdate, onClose }) {
  const [formData, setFormData] = useState({
    label: node.data?.label || '',
    server: node.data?.server_name || node.data?.server || '',
    tool: node.data?.tool_name || node.data?.tool || '',
    parameters: node.data?.parameters || '{}',
    timeout: node.data?.timeout || 30,
    retryOnFailure: node.data?.retryOnFailure !== false,
    maxRetries: node.data?.maxRetries || 3,
  });

  const [hasChanges, setHasChanges] = useState(false);
  const [servers, setServers] = useState([]);
  const [tools, setTools] = useState([]);
  const [parametersError, setParametersError] = useState('');

  useEffect(() => {
    fetchServers();
  }, []);

  useEffect(() => {
    if (formData.server) {
      fetchTools(formData.server);
    }
  }, [formData.server]);

  const fetchServers = async () => {
    try {
      const response = await fetch('/api/servers');

      if (response.ok) {
        const data = await response.json();
        const serversArray = Object.keys(data || {}).map(name => ({
          name,
          status: data[name].containerStatus || 'unknown',
          ...data[name]
        }));
        setServers(serversArray);
      }
    } catch (error) {
      console.error('Failed to fetch servers:', error);
    }
  };

  const fetchTools = async (serverName) => {
    try {
      const response = await fetch(`/api/server-openapi/${serverName}`);

      if (response.ok) {
        const openapi = await response.json();
        const paths = openapi.paths || {};
        const toolsList = Object.keys(paths).map(path => {
          const pathData = paths[path];
          const operation = pathData.post || pathData.get || {};
          return {
            name: operation.operationId || path.replace(/^\//, ''),
            description: operation.description || operation.summary || ''
          };
        });
        setTools(toolsList);
      }
    } catch (error) {
      console.error('Failed to fetch tools:', error);
      setTools([]);
    }
  };

  const handleChange = (field, value) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
    setHasChanges(true);

    if (field === 'parameters') {
      try {
        JSON.parse(value);
        setParametersError('');
      } catch (error) {
        setParametersError('Invalid JSON format');
      }
    }
  };

  const handleSave = () => {
    if (parametersError) {

      return;
    }

    onUpdate(node.id, {
      ...node.data,
      label: formData.label,
      server_name: formData.server,
      tool_name: formData.tool,
      parameters: formData.parameters,
      timeout: formData.timeout,
      retryOnFailure: formData.retryOnFailure,
      maxRetries: formData.maxRetries,
    });
    setHasChanges(false);
  };

  const handleCancel = () => {
    setFormData({
      label: node.data?.label || '',
      server: node.data?.server_name || node.data?.server || '',
      tool: node.data?.tool_name || node.data?.tool || '',
      parameters: node.data?.parameters || '{}',
      timeout: node.data?.timeout || 30,
      retryOnFailure: node.data?.retryOnFailure !== false,
      maxRetries: node.data?.maxRetries || 3,
    });
    setHasChanges(false);
    setParametersError('');
  };

  return (
    <div className="p-6 space-y-6">
      <div>
        <Input
          label="Node Label"
          value={formData.label}
          onChange={(e) => handleChange('label', e.target.value)}
          placeholder="Enter node label"
        />
      </div>

      <div>
        <Select
          label="MCP Server"
          value={formData.server}
          onChange={(value) => handleChange('server', value)}
          options={[
            { value: '', label: 'Select a server' },
            ...servers.map((server) => ({
              value: server.name,
              label: `${server.name}${server.status !== 'running' && server.status !== 'unknown' ? ` (${server.status})` : ''}`,
            })),
          ]}
        />
        {servers.length === 0 && (
          <p className="mt-1 text-xs text-yellow-600 dark:text-yellow-400">
            No MCP servers available. Start a server first.
          </p>
        )}
      </div>

      <div>
        <Select
          label="Tool"
          value={formData.tool}
          onChange={(value) => handleChange('tool', value)}
          disabled={!formData.server}
          options={[
            { value: '', label: 'Select a tool' },
            ...tools.map((tool) => ({
              value: tool.name,
              label: tool.name,
            })),
          ]}
        />
        {formData.server && tools.length === 0 && (
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">Loading tools...</p>
        )}
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
          Tool Parameters (JSON)
        </label>
        <textarea
          value={formData.parameters}
          onChange={(e) => handleChange('parameters', e.target.value)}
          rows={8}
          className={`w-full px-3 py-2 border rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500 bg-white dark:bg-gray-700 text-gray-900 dark:text-white font-mono text-sm resize-y ${
            parametersError
              ? 'border-red-300 dark:border-red-600'
              : 'border-gray-300 dark:border-gray-600'
          }`}
          placeholder={'{\n  "param1": "value1",\n  "param2": "{{input}}"\n}'}
        />
        {parametersError && <p className="mt-1 text-xs text-red-600 dark:text-red-400">{parametersError}</p>}
        <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
          Use {"{{input}}"} to reference data from previous nodes
        </p>
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div>
          <Input
            label="Timeout (seconds)"
            type="number"
            min="1"
            max="300"
            value={formData.timeout}
            onChange={(e) => handleChange('timeout', parseInt(e.target.value))}
          />
        </div>

        <div>
          <Input
            label="Max Retries"
            type="number"
            min="0"
            max="10"
            value={formData.maxRetries}
            onChange={(e) => handleChange('maxRetries', parseInt(e.target.value))}
            disabled={!formData.retryOnFailure}
          />
        </div>
      </div>

      <div>
        <label className="flex items-center space-x-2">
          <input
            type="checkbox"
            checked={formData.retryOnFailure}
            onChange={(e) => handleChange('retryOnFailure', e.target.checked)}
            className="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
          />
          <span className="text-sm text-gray-700 dark:text-gray-300">Retry on failure</span>
        </label>
      </div>

      <div className="pt-4 border-t border-gray-200 dark:border-gray-700 flex items-center justify-end space-x-3">
        <Button variant="secondary" onClick={handleCancel} disabled={!hasChanges}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={handleSave}
          disabled={!hasChanges || !formData.server || !formData.tool || parametersError}
        >
          Save Changes
        </Button>
      </div>
    </div>
  );
}
