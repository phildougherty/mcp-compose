import React from 'react';
import {
  ClockIcon,
  BoltIcon,
  CubeIcon,
  DocumentTextIcon,
  ChatBubbleLeftRightIcon,
  CodeBracketIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';
import { NodePaletteItem, DEFAULT_NODE_PALETTE } from './types';

interface NodePaletteProps {
  className?: string;
}

const CATEGORY_ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  trigger: ClockIcon,
  action: BoltIcon,
  condition: ArrowPathIcon,
  integration: CodeBracketIcon,
};

const NODE_TYPE_ICONS: Record<string, React.ComponentType<{ className?: string }>> = {
  'schedule-trigger': ClockIcon,
  'webhook-trigger': BoltIcon,
  'ai-task': ChatBubbleLeftRightIcon,
  'mcp-server': CubeIcon,
  'transform': ArrowPathIcon,
  'code': CodeBracketIcon,
  'decision': DocumentTextIcon,
};

const NodePalette: React.FC<NodePaletteProps> = ({ className = '' }) => {
  const onDragStart = (event: React.DragEvent<HTMLDivElement>, nodeType: string, label: string) => {
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('application/reactflow', nodeType);
    event.dataTransfer.setData('nodeLabel', label);
  };

  const groupedNodes = DEFAULT_NODE_PALETTE.reduce((acc, node) => {
    if (!acc[node.category]) {
      acc[node.category] = [];
    }

    acc[node.category].push(node);

    return acc;
  }, {} as Record<string, NodePaletteItem[]>);

  const categories = Object.keys(groupedNodes) as Array<keyof typeof groupedNodes>;

  return (
    <div className={`flex flex-col h-full ${className}`}>
      <div className="px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <h2 className="text-sm font-semibold text-gray-900 dark:text-white uppercase tracking-wider">
          Node Palette
        </h2>
      </div>

      <div className="flex-1 overflow-y-auto p-4 space-y-6">
        {categories.map((category) => {
          const CategoryIcon = CATEGORY_ICONS[category];

          return (
            <div key={category} className="space-y-2">
              <div className="flex items-center space-x-2 px-2">
                {CategoryIcon && <CategoryIcon className="h-4 w-4 text-gray-500 dark:text-gray-400" />}
                <h3 className="text-xs font-semibold text-gray-700 dark:text-gray-300 uppercase tracking-wider">
                  {category}
                </h3>
              </div>

              <div className="space-y-2">
                {groupedNodes[category].map((node) => {
                  const NodeIcon = NODE_TYPE_ICONS[node.id] || CubeIcon;

                  return (
                    <div
                      key={node.id}
                      draggable
                      onDragStart={(e) => onDragStart(e, node.type, node.label)}
                      className="
                        flex items-center space-x-3 p-3 rounded-lg
                        bg-white dark:bg-gray-800
                        border border-gray-200 dark:border-gray-700
                        cursor-grab active:cursor-grabbing
                        hover:border-blue-400 dark:hover:border-blue-500
                        hover:shadow-md
                        transition-all duration-150
                      "
                    >
                      <div className="flex-shrink-0 p-2 rounded-md bg-blue-50 dark:bg-blue-900/20">
                        <NodeIcon className="h-5 w-5 text-blue-600 dark:text-blue-400" />
                      </div>

                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 dark:text-white truncate">
                          {node.label}
                        </p>
                        {node.description && (
                          <p className="text-xs text-gray-500 dark:text-gray-400 truncate">
                            {node.description}
                          </p>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default NodePalette;
