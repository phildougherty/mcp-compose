import React from 'react';
import { Card, Badge, Button } from '../shared';

/**
 * ToolList - Display discovered tools from MCP server
 */
export function ToolList({ tools, onTestTool }) {
  if (!tools || tools.length === 0) {
    return null;
  }

  return (
    <div className="space-y-3">
      <h6 className="text-xs font-medium text-slate-400 uppercase tracking-wide">
        Discovered Tools ({tools.length})
      </h6>

      <div className="space-y-2">
        {tools.map((tool) => (
          <Card
            key={tool.name}
            className="bg-slate-700/50 border-slate-600/50"
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <h4 className="font-medium text-sm text-white truncate">
                    {tool.name}
                  </h4>
                  {tool.experimental && (
                    <Badge variant="warning" size="sm">
                      Experimental
                    </Badge>
                  )}
                </div>
                {tool.description && (
                  <p className="text-xs text-slate-400 line-clamp-2">
                    {tool.description}
                  </p>
                )}
                {tool.inputSchema && (
                  <details className="mt-2">
                    <summary className="text-xs text-slate-500 cursor-pointer hover:text-slate-400">
                      View Schema
                    </summary>
                    <pre className="mt-2 text-xs text-slate-400 bg-slate-800 p-2 rounded overflow-x-auto">
                      {JSON.stringify(tool.inputSchema, null, 2)}
                    </pre>
                  </details>
                )}
              </div>
              <Button
                onClick={() => onTestTool(tool)}
                variant="secondary"
                size="sm"
                className="flex-shrink-0 min-h-[44px] min-w-[44px]"
                aria-label={`Test ${tool.name} tool`}
              >
                Test
              </Button>
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
}
