# Workflow Builder Custom Nodes

This directory contains the custom React Flow node components for the visual workflow builder.

## Node Types

### 1. TriggerNode (Blue Theme)
Entry point for workflows. Supports:
- **Schedule triggers**: Cron-based scheduling
- **Webhook triggers**: HTTP webhook endpoints
- **Event triggers**: System or custom events

**Props:**
```javascript
{
  label: 'Daily Backup',
  config: {
    schedule: '0 0 * * *',  // Cron expression
    webhook: '/webhooks/backup',
    event: 'file.created'
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:00:00Z'
}
```

### 2. AITaskNode (Purple Theme)
AI-powered task execution. Supports:
- Multiple AI providers (OpenRouter, Claude, OpenAI, Ollama)
- Custom prompts
- Model selection

**Props:**
```javascript
{
  label: 'Analyze Code',
  config: {
    provider: 'openrouter',
    model: 'anthropic/claude-3.5-sonnet',
    prompt: 'Review this code for security issues...'
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:05:00Z'
}
```

### 3. MCPServerNode (Green Theme)
MCP server tool invocation. Supports:
- Server selection
- Tool selection
- Parameter configuration

**Props:**
```javascript
{
  label: 'Read File',
  config: {
    server: 'filesystem',
    tool: 'read_file',
    parameters: {
      path: '/data/input.json'
    }
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:10:00Z'
}
```

### 4. DecisionNode (Yellow Theme)
Conditional branching. Supports:
- JavaScript expressions
- Dual output handles (true/false paths)

**Props:**
```javascript
{
  label: 'Check File Size',
  config: {
    condition: 'input.size > 1024 * 1024'  // JavaScript expression
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:15:00Z'
}
```

### 5. TransformNode (Orange Theme)
Data transformation. Supports:
- JavaScript transformation code
- Input/output mapping

**Props:**
```javascript
{
  label: 'Format Data',
  config: {
    transformCode: 'return { ...input, timestamp: Date.now() }'
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:20:00Z'
}
```

### 6. CodeNode (Gray Theme)
Custom code execution. Supports:
- JavaScript, Python, Shell
- Full code editor integration

**Props:**
```javascript
{
  label: 'Process Data',
  config: {
    language: 'python',
    code: 'import json\ndata = json.loads(input)\nreturn data["result"]'
  },
  status: 'idle' | 'running' | 'success' | 'error',
  lastRun: '2025-10-07T10:25:00Z'
}
```

## Status Colors

All nodes support status indicators:
- **idle**: Default border color (blue/purple/green/etc.)
- **running**: Yellow border with pulsing animation
- **success**: Green border
- **error**: Red border

## Usage Example

```javascript
import { ReactFlow } from '@xyflow/react';
import { nodeTypes } from './components/WorkflowBuilder/nodes';

const WorkflowCanvas = () => {
  const nodes = [
    {
      id: '1',
      type: 'trigger',
      position: { x: 0, y: 0 },
      data: {
        label: 'Daily at 9 AM',
        config: { schedule: '0 9 * * *' }
      }
    },
    {
      id: '2',
      type: 'ai-task',
      position: { x: 300, y: 0 },
      data: {
        label: 'Generate Report',
        config: {
          provider: 'openrouter',
          model: 'anthropic/claude-3.5-sonnet',
          prompt: 'Create a daily summary report'
        }
      }
    }
  ];

  const edges = [
    { id: 'e1-2', source: '1', target: '2' }
  ];

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      fitView
    />
  );
};
```

## Dependencies

- `@xyflow/react` - React Flow library (already installed)
- `@heroicons/react` - Icon library (already installed)
- `clsx` - Utility for conditional classes (already installed)

## Future Enhancements

- Add drag handles for node repositioning
- Implement node validation
- Add custom handle styles per node type
- Support for node grouping/nesting
- Add minimap markers
- Implement node templates
