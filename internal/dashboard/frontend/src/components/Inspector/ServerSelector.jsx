import React from 'react';
import { Select } from '../shared';

/**
 * ServerSelector - Dropdown for selecting MCP server to inspect
 */
export function ServerSelector({ servers, selectedServer, onSelect, disabled }) {
  const options = (servers || []).map(server => ({
    value: server.name,
    label: server.name,
  }));

  return (
    <div className="space-y-2">
      <label className="block text-sm font-medium text-slate-300">
        Select Server
      </label>
      <Select
        options={options}
        value={selectedServer}
        onChange={onSelect}
        disabled={disabled}
        placeholder="Choose an MCP server..."
        className="w-full"
      />
    </div>
  );
}
