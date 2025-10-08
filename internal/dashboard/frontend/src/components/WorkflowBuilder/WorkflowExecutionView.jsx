import React, { useState, useEffect, useCallback } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Panel,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  PlayIcon,
  StopIcon,
  CheckCircleIcon,
  XCircleIcon,
  ClockIcon,
} from '@heroicons/react/24/outline';
import ExecutionPanel from './ExecutionPanel';
import ExecutionHistory from './ExecutionHistory';

const WorkflowExecutionView = ({ workflow, onClose }) => {
  const [execution, setExecution] = useState(null);
  const [nodeStatus, setNodeStatus] = useState({});
  const [isExecuting, setIsExecuting] = useState(false);
  const [executions, setExecutions] = useState([]);
  const [selectedExecution, setSelectedExecution] = useState(null);

  const normalizeExecution = useCallback((exec) => {
    const nodeOutputs = {};
    if (exec.node_states) {
      exec.node_states.forEach(state => {
        if (state.output) {
          nodeOutputs[state.node_id] = state.output;
        }
      });
    }

    return {
      ...exec,
      startTime: exec.started_at || exec.startTime,
      endTime: exec.completed_at || exec.endTime,
      completedNodes: exec.node_states?.filter(n => n.status === 'completed').length || 0,
      totalNodes: workflow.nodes.length,
      errors: exec.error ? [exec.error] : (exec.errors || []),
      nodeOutputs,
    };
  }, [workflow.nodes.length]);

  const loadExecutionHistory = useCallback(async () => {
    try {
      const response = await fetch(`/api/workflows/${workflow.id}/executions`);

      if (response.ok) {
        const data = await response.json();
        const normalizedExecutions = (data.executions || []).map(normalizeExecution);
        setExecutions(normalizedExecutions);
      }
    } catch (error) {
      console.error('Failed to load execution history:', error);
    }
  }, [workflow.id, normalizeExecution]);

  useEffect(() => {
    loadExecutionHistory();
  }, [loadExecutionHistory]);

  const startExecution = useCallback(async () => {
    try {
      setIsExecuting(true);

      const response = await fetch(`/api/workflows/${workflow.id}/execute`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          workflow: {
            nodes: workflow.nodes,
            edges: workflow.edges,
          },
        }),
      });

      if (!response.ok) {
        throw new Error('Failed to start execution');
      }

      const data = await response.json();

      setExecution({
        id: data.executionId || `exec-${Date.now()}`,
        status: 'running',
        startTime: new Date().toISOString(),
        completedNodes: 0,
        totalNodes: workflow.nodes.length,
        currentNode: workflow.nodes[0]?.id,
        errors: [],
      });

      pollExecutionStatus(data.executionId || `exec-${Date.now()}`);
    } catch (error) {
      console.error('Execution failed:', error);
      setIsExecuting(false);
      setExecution({
        id: `exec-${Date.now()}`,
        status: 'error',
        startTime: new Date().toISOString(),
        completedNodes: 0,
        totalNodes: workflow.nodes.length,
        errors: [error.message],
      });
    }
  }, [workflow]);

  const pollExecutionStatus = useCallback(async (executionId) => {
    const pollInterval = setInterval(async () => {
      try {
        const response = await fetch(`/api/workflows/${workflow.id}/executions/${executionId}`);

        if (response.ok) {
          const data = await response.json();

          if (data && data.execution) {
            const normalized = normalizeExecution(data.execution);
            setExecution(normalized);
            setNodeStatus(data.execution.nodeStatus || {});

            if (data.execution.status === 'completed' || data.execution.status === 'error') {
              clearInterval(pollInterval);
              setIsExecuting(false);
              loadExecutionHistory();
            }
          }
        }
      } catch (error) {
        console.error('Polling failed:', error);
        clearInterval(pollInterval);
        setIsExecuting(false);
      }
    }, 1000);

    return () => clearInterval(pollInterval);
  }, [workflow.id, normalizeExecution, loadExecutionHistory]);

  const stopExecution = useCallback(async () => {
    if (!execution?.id) {

      return;
    }

    try {
      const response = await fetch(`/api/workflows/${workflow.id}/executions/${execution.id}/stop`, {
        method: 'POST',
      });

      if (response.ok) {
        setIsExecuting(false);
        setExecution({
          ...execution,
          status: 'stopped',
        });
      }
    } catch (error) {
      console.error('Failed to stop execution:', error);
    }
  }, [execution, workflow.id]);

  const getNodeColor = (status) => {
    switch (status) {
      case 'running':
        return '#FCD34D';
      case 'success':
        return '#34D399';
      case 'error':
        return '#F87171';
      default:
        return '#9CA3AF';
    }
  };

  const getNodeIcon = (status) => {
    switch (status) {
      case 'running':
        return <ClockIcon className="h-4 w-4 animate-pulse" />;
      case 'success':
        return <CheckCircleIcon className="h-4 w-4" />;
      case 'error':
        return <XCircleIcon className="h-4 w-4" />;
      default:
        return null;
    }
  };

  const enhancedNodes = workflow.nodes.map((node) => {
    const status = nodeStatus[node.id] || 'idle';

    return {
      ...node,
      style: {
        ...node.style,
        borderColor: getNodeColor(status),
        borderWidth: 2,
        backgroundColor: status === 'running' ? `${getNodeColor(status)}20` : undefined,
      },
      data: {
        ...node.data,
        status,
        icon: getNodeIcon(status),
      },
    };
  });

  const handleExecutionSelect = (exec) => {
    const normalized = normalizeExecution(exec);
    setSelectedExecution(normalized);
    setExecution(normalized);
    setNodeStatus(exec.nodeStatus || {});
  };

  return (
    <div className="flex flex-col h-screen bg-gray-50 dark:bg-gray-900">
      <div className="flex items-center justify-between px-4 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center space-x-4">
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">
            {workflow.name} - Execution View
          </h2>
        </div>

        <div className="flex items-center space-x-2">
          {!isExecuting ? (
            <button
              onClick={startExecution}
              className="
                inline-flex items-center px-4 py-2 rounded-lg
                bg-green-600 hover:bg-green-700
                text-white font-medium text-sm
                transition-colors duration-150
                focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2
              "
            >
              <PlayIcon className="h-5 w-5 mr-2" />
              Start Execution
            </button>
          ) : (
            <button
              onClick={stopExecution}
              className="
                inline-flex items-center px-4 py-2 rounded-lg
                bg-red-600 hover:bg-red-700
                text-white font-medium text-sm
                transition-colors duration-150
                focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2
              "
            >
              <StopIcon className="h-5 w-5 mr-2" />
              Stop Execution
            </button>
          )}

          <button
            onClick={onClose}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-gray-600 hover:bg-gray-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2
            "
          >
            Close
          </button>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className="w-80 flex-shrink-0 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 overflow-y-auto">
          <ExecutionHistory
            executions={executions}
            selectedExecution={selectedExecution}
            onExecutionSelect={handleExecutionSelect}
          />
        </div>

        <div className="flex-1 relative">
          <ReactFlow
            nodes={enhancedNodes}
            edges={workflow.edges}
            fitView
            className="bg-gray-50 dark:bg-gray-900"
            nodesDraggable={false}
            nodesConnectable={false}
            elementsSelectable={false}
          >
            <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
            <Controls />
            <MiniMap
              nodeStrokeWidth={3}
              zoomable
              pannable
              className="bg-white dark:bg-gray-800"
            />

            {execution && (
              <Panel position="top-right">
                <ExecutionPanel execution={execution} workflow={workflow} />
              </Panel>
            )}
          </ReactFlow>
        </div>
      </div>
    </div>
  );
};

export default WorkflowExecutionView;
