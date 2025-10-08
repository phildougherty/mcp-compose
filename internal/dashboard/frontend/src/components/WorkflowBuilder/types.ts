import { Node, Edge } from '@xyflow/react';

export interface WorkflowNodeData {
  label: string;
  type: 'trigger' | 'action' | 'condition' | 'integration';
  config?: Record<string, any>;
  server?: string;
  tool?: string;
  resource?: string;
  prompt?: string;
}

export interface WorkflowNode extends Node {
  data: WorkflowNodeData;
}

export interface WorkflowEdge extends Edge {
  animated?: boolean;
}

export interface Workflow {
  id: string;
  name: string;
  description?: string;
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
  createdAt: string;
  updatedAt: string;
}

export interface NodePaletteItem {
  id: string;
  type: string;
  label: string;
  icon?: string;
  category: 'trigger' | 'action' | 'condition' | 'integration';
  description?: string;
}

export const DEFAULT_NODE_PALETTE: NodePaletteItem[] = [
  {
    id: 'schedule-trigger',
    type: 'trigger',
    label: 'Schedule Trigger',
    category: 'trigger',
    description: 'Trigger workflow on a schedule',
  },
  {
    id: 'webhook-trigger',
    type: 'trigger',
    label: 'Webhook Trigger',
    category: 'trigger',
    description: 'Trigger workflow via webhook',
  },
  {
    id: 'ai-task',
    type: 'ai-task',
    label: 'AI Task',
    category: 'action',
    description: 'Execute AI task with LLM',
  },
  {
    id: 'mcp-server',
    type: 'mcp-server',
    label: 'MCP Server',
    category: 'action',
    description: 'Call MCP server tool',
  },
  {
    id: 'transform',
    type: 'transform',
    label: 'Transform',
    category: 'action',
    description: 'Transform data with JavaScript',
  },
  {
    id: 'code',
    type: 'code',
    label: 'Code',
    category: 'action',
    description: 'Execute custom code',
  },
  {
    id: 'decision',
    type: 'decision',
    label: 'Decision',
    category: 'condition',
    description: 'Branch based on condition',
  },
];
