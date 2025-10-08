/**
 * Example Workflow Configuration
 *
 * This file demonstrates how to use the custom workflow nodes
 * in a React Flow canvas to create a complete data processing workflow.
 */

export const exampleWorkflow = {
  nodes: [
    {
      id: 'trigger-1',
      type: 'trigger',
      position: { x: 50, y: 200 },
      data: {
        label: 'Daily at 9 AM',
        config: {
          schedule: '0 9 * * *'
        },
        status: 'idle'
      }
    },
    {
      id: 'mcp-1',
      type: 'mcp-server',
      position: { x: 350, y: 100 },
      data: {
        label: 'Read Input File',
        config: {
          server: 'filesystem',
          tool: 'read_file',
          parameters: {
            path: '/data/daily-input.json'
          }
        },
        status: 'success',
        lastRun: '2025-10-07T09:00:15Z'
      }
    },
    {
      id: 'transform-1',
      type: 'transform',
      position: { x: 650, y: 100 },
      data: {
        label: 'Parse JSON',
        config: {
          transformCode: 'return JSON.parse(input.content)'
        },
        status: 'success',
        lastRun: '2025-10-07T09:00:16Z'
      }
    },
    {
      id: 'decision-1',
      type: 'decision',
      position: { x: 950, y: 100 },
      data: {
        label: 'Check Data Size',
        config: {
          condition: 'input.records && input.records.length > 0'
        },
        status: 'success',
        lastRun: '2025-10-07T09:00:17Z'
      }
    },
    {
      id: 'ai-1',
      type: 'ai-task',
      position: { x: 1300, y: 50 },
      data: {
        label: 'Analyze Data',
        config: {
          provider: 'openrouter',
          model: 'anthropic/claude-3.5-sonnet',
          prompt: 'Analyze this data and provide insights:\n\n{{input}}'
        },
        status: 'running'
      }
    },
    {
      id: 'code-1',
      type: 'code',
      position: { x: 1300, y: 250 },
      data: {
        label: 'Log Error',
        config: {
          language: 'javascript',
          code: 'console.error("No data to process"); return { error: "Empty dataset" };'
        },
        status: 'idle'
      }
    },
    {
      id: 'mcp-2',
      type: 'mcp-server',
      position: { x: 1650, y: 50 },
      data: {
        label: 'Save Results',
        config: {
          server: 'filesystem',
          tool: 'write_file',
          parameters: {
            path: '/data/analysis-results.json',
            content: '{{input}}'
          }
        },
        status: 'idle'
      }
    },
    {
      id: 'mcp-3',
      type: 'mcp-server',
      position: { x: 350, y: 300 },
      data: {
        label: 'Watch for Changes',
        config: {
          server: 'filesystem',
          tool: 'watch_file',
          parameters: {
            path: '/data',
            pattern: '*.json'
          }
        },
        status: 'running'
      }
    }
  ],
  edges: [
    { id: 'e-trigger-mcp1', source: 'trigger-1', target: 'mcp-1', animated: true },
    { id: 'e-trigger-mcp3', source: 'trigger-1', target: 'mcp-3', animated: true },
    { id: 'e-mcp1-transform', source: 'mcp-1', target: 'transform-1', animated: true },
    { id: 'e-transform-decision', source: 'transform-1', target: 'decision-1', animated: true },
    { id: 'e-decision-ai', source: 'decision-1', sourceHandle: 'true', target: 'ai-1', animated: true, style: { stroke: '#22c55e' } },
    { id: 'e-decision-code', source: 'decision-1', sourceHandle: 'false', target: 'code-1', animated: true, style: { stroke: '#ef4444' } },
    { id: 'e-ai-mcp2', source: 'ai-1', target: 'mcp-2', animated: true }
  ]
};

export const exampleWorkflowDescription = `
# Daily Data Processing Workflow

This workflow demonstrates a complete data processing pipeline with conditional logic:

1. **Trigger**: Runs daily at 9 AM
2. **Read Input**: Fetches daily data file using filesystem MCP server
3. **Transform**: Parses JSON content
4. **Decision**: Checks if data is present
   - **True path**: Sends to AI for analysis
   - **False path**: Logs error using JavaScript
5. **AI Analysis**: Claude analyzes the data and provides insights
6. **Save Results**: Writes analysis results back to filesystem
7. **File Watcher**: Continuously monitors for new data files

## Status Indicators

- Green: Successfully completed
- Yellow: Currently running
- Gray: Idle/waiting
- Red: Error occurred
`;
