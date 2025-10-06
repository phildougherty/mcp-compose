import React from 'react';
import { Select } from '../shared';

/**
 * TemplateSelector - Dropdown for selecting pre-built request templates
 */
export function TemplateSelector({ onSelect, disabled }) {
  const templates = [
    { value: 'initialize', label: 'initialize - Server initialization' },
    { value: 'listTools', label: 'tools/list - List available tools' },
    { value: 'listResources', label: 'resources/list - List available resources' },
    { value: 'listPrompts', label: 'prompts/list - List available prompts' },
    { value: 'callTool', label: 'tools/call - Execute a tool' },
    { value: 'readResource', label: 'resources/read - Read a resource' },
    { value: 'getPrompt', label: 'prompts/get - Get a prompt' },
  ];

  return (
    <div className="space-y-2">
      <label className="block text-xs font-medium text-slate-400 uppercase tracking-wide">
        Load Template
      </label>
      <Select
        options={templates}
        value=""
        onChange={onSelect}
        disabled={disabled}
        placeholder="Choose a template..."
        className="w-full text-xs"
      />
    </div>
  );
}
