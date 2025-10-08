import React, { useState, useCallback, useRef, useEffect } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  addEdge,
  Connection,
  Panel,
  BackgroundVariant,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';
import {
  PlayIcon,
  DocumentArrowDownIcon,
  XMarkIcon,
  Cog6ToothIcon,
  EyeIcon,
  FolderOpenIcon,
  PlusIcon,
} from '@heroicons/react/24/outline';
import NodePalette from './NodePalette';
import NodeConfigPanel from './NodeConfigPanel';
import WorkflowExecutionView from './WorkflowExecutionView';
import { WorkflowNode, WorkflowEdge, Workflow } from './types';
import { nodeTypes } from './nodes';

interface WorkflowBuilderProps {
  servers?: any[];
}

const STORAGE_KEY = 'mcp-compose-workflow';

const WorkflowBuilder: React.FC<WorkflowBuilderProps> = ({ servers = [] }) => {
  const reactFlowWrapper = useRef<HTMLDivElement>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState<WorkflowNode>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<WorkflowEdge>([]);
  const [reactFlowInstance, setReactFlowInstance] = useState<any>(null);
  const [selectedNode, setSelectedNode] = useState<WorkflowNode | null>(null);
  const [workflowName, setWorkflowName] = useState('Untitled Workflow');
  const [isExecutionView, setIsExecutionView] = useState(false);
  const [workflowId, setWorkflowId] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [showWorkflowList, setShowWorkflowList] = useState(false);
  const [workflows, setWorkflows] = useState<Workflow[]>([]);

  useEffect(() => {
    loadWorkflow();
    fetchWorkflows();
  }, []);

  const fetchWorkflows = async () => {
    try {
      const response = await fetch('/api/workflows');
      if (response.ok) {
        const data = await response.json();
        setWorkflows(data.workflows || []);
      }
    } catch (error) {
      console.error('Failed to fetch workflows:', error);
    }
  };

  const loadWorkflowFromAPI = async (id: string) => {
    try {
      const response = await fetch(`/api/workflows/${id}`);
      if (response.ok) {
        const workflow = await response.json();
        setNodes(workflow.nodes || []);
        setEdges(workflow.edges || []);
        setWorkflowName(workflow.name || 'Untitled Workflow');
        setWorkflowId(workflow.id);
        setShowWorkflowList(false);
      }
    } catch (error) {
      console.error('Failed to load workflow:', error);
      alert('Failed to load workflow');
    }
  };

  const createNewWorkflow = () => {
    setNodes([]);
    setEdges([]);
    setWorkflowName('Untitled Workflow');
    setWorkflowId(null);
    setShowWorkflowList(false);
    localStorage.removeItem(STORAGE_KEY);
  };

  const loadWorkflow = useCallback(() => {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);

      if (saved) {
        const workflow: Workflow = JSON.parse(saved);
        setNodes(workflow.nodes || []);
        setEdges(workflow.edges || []);
        setWorkflowName(workflow.name || 'Untitled Workflow');
      }
    } catch (error) {
      console.error('Failed to load workflow:', error);
    }
  }, [setNodes, setEdges]);

  const saveWorkflow = useCallback(async () => {
    try {
      setIsSaving(true);

      const workflow: Workflow = {
        id: workflowId || undefined,
        name: workflowName,
        nodes,
        edges,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      };

      const endpoint = workflowId ? `/api/workflows/${workflowId}` : '/api/workflows';
      const method = workflowId ? 'PUT' : 'POST';

      const response = await fetch(endpoint, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(workflow),
      });

      if (!response.ok) {
        throw new Error('Failed to save workflow');
      }

      const savedWorkflow = await response.json();
      setWorkflowId(savedWorkflow.id || savedWorkflow.workflow?.id);

      localStorage.setItem(STORAGE_KEY, JSON.stringify(workflow));
      alert('Workflow saved successfully!');
    } catch (error) {
      console.error('Failed to save workflow:', error);
      alert('Failed to save workflow: ' + (error instanceof Error ? error.message : 'Unknown error'));
    } finally {
      setIsSaving(false);
    }
  }, [nodes, edges, workflowName, workflowId]);

  const onConnect = useCallback(
    (params: Connection) => {
      const newEdge: WorkflowEdge = {
        ...params,
        id: `edge-${params.source}-${params.target}`,
        animated: true,
      };

      setEdges((eds) => addEdge(newEdge, eds));
    },
    [setEdges]
  );

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();

      const reactFlowBounds = reactFlowWrapper.current?.getBoundingClientRect();
      const type = event.dataTransfer.getData('application/reactflow');
      const label = event.dataTransfer.getData('nodeLabel');

      if (!type || !reactFlowInstance || !reactFlowBounds) {

        return;
      }

      const position = reactFlowInstance.screenToFlowPosition({
        x: event.clientX - reactFlowBounds.left,
        y: event.clientY - reactFlowBounds.top,
      });

      const defaultData: any = {
        label: label || type,
        type: type as 'trigger' | 'action' | 'condition' | 'integration',
        config: {},
      };

      if (type === 'trigger') {
        defaultData.schedule = '0 0 * * *';
        defaultData.enabled = true;
        defaultData.triggerType = 'cron';
      }

      const newNode: WorkflowNode = {
        id: `node-${Date.now()}`,
        type: type,
        position,
        data: defaultData,
      };

      setNodes((nds) => nds.concat(newNode));
    },
    [reactFlowInstance, setNodes]
  );

  const onNodeClick = useCallback((_event: React.MouseEvent, node: WorkflowNode) => {
    setSelectedNode(node);
  }, []);

  const onPaneClick = useCallback(() => {
    setSelectedNode(null);
  }, []);

  const updateNodeData = useCallback(
    (nodeId: string, newData: Partial<WorkflowNode['data']>) => {
      setNodes((nds) =>
        nds.map((node) =>
          node.id === nodeId
            ? { ...node, data: { ...node.data, ...newData } }
            : node
        )
      );

      if (selectedNode?.id === nodeId) {
        setSelectedNode((prev) =>
          prev ? { ...prev, data: { ...prev.data, ...newData } } : null
        );
      }
    },
    [setNodes, selectedNode]
  );

  const runWorkflow = useCallback(() => {
    setIsExecutionView(true);
  }, []);

  const clearWorkflow = useCallback(() => {
    if (confirm('Are you sure you want to clear the workflow? This cannot be undone.')) {
      setNodes([]);
      setEdges([]);
      setWorkflowName('Untitled Workflow');
      setSelectedNode(null);
      localStorage.removeItem(STORAGE_KEY);
    }
  }, [setNodes, setEdges]);

  if (isExecutionView) {
    if (!workflowId) {
      return (
        <div className="flex flex-col items-center justify-center h-screen bg-gray-50 dark:bg-gray-900">
          <div className="text-center">
            <h2 className="text-xl font-semibold text-gray-900 dark:text-white mb-4">
              Workflow Not Saved
            </h2>
            <p className="text-gray-600 dark:text-gray-400 mb-6">
              Please save your workflow before running or viewing executions.
            </p>
            <button
              onClick={() => setIsExecutionView(false)}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg"
            >
              Go Back
            </button>
          </div>
        </div>
      );
    }

    const workflow: Workflow = {
      id: workflowId,
      name: workflowName,
      nodes,
      edges,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    return (
      <WorkflowExecutionView
        workflow={workflow}
        onClose={() => setIsExecutionView(false)}
      />
    );
  }

  return (
    <>
      {showWorkflowList && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
          <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-2xl max-h-[80vh] overflow-hidden flex flex-col">
            <div className="flex items-center justify-between px-6 py-4 border-b border-gray-200 dark:border-gray-700">
              <h2 className="text-xl font-semibold text-gray-900 dark:text-white">Load Workflow</h2>
              <button
                onClick={() => setShowWorkflowList(false)}
                className="text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
              >
                <XMarkIcon className="h-6 w-6" />
              </button>
            </div>

            <div className="flex-1 overflow-y-auto p-6">
              {workflows.length === 0 ? (
                <div className="text-center py-12 text-gray-500 dark:text-gray-400">
                  No workflows found. Create a new workflow to get started.
                </div>
              ) : (
                <div className="space-y-3">
                  {workflows.map((wf) => (
                    <button
                      key={wf.id}
                      onClick={() => loadWorkflowFromAPI(wf.id)}
                      className="w-full text-left px-4 py-3 border border-gray-200 dark:border-gray-700 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
                    >
                      <div className="font-semibold text-gray-900 dark:text-white">{wf.name}</div>
                      <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                        Updated: {new Date(wf.updated_at).toLocaleString()}
                      </div>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      <div className="flex flex-col h-screen bg-gray-50 dark:bg-gray-900">
        <div className="flex items-center justify-between px-4 py-3 bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700">
        <div className="flex items-center space-x-4">
          <input
            type="text"
            value={workflowName}
            onChange={(e) => setWorkflowName(e.target.value)}
            className="
              px-3 py-1.5 text-lg font-semibold
              bg-transparent border-0
              text-gray-900 dark:text-white
              focus:outline-none focus:ring-2 focus:ring-blue-500 rounded
            "
            placeholder="Workflow name"
          />
        </div>

        <div className="flex items-center space-x-2">
          <button
            onClick={() => setShowWorkflowList(true)}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-gray-600 hover:bg-gray-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2
            "
          >
            <FolderOpenIcon className="h-5 w-5 mr-2" />
            Load
          </button>

          <button
            onClick={createNewWorkflow}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-gray-600 hover:bg-gray-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-gray-500 focus:ring-offset-2
            "
          >
            <PlusIcon className="h-5 w-5 mr-2" />
            New
          </button>

          <button
            onClick={saveWorkflow}
            disabled={isSaving}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-blue-600 hover:bg-blue-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2
              disabled:opacity-50 disabled:cursor-not-allowed
            "
          >
            <DocumentArrowDownIcon className="h-5 w-5 mr-2" />
            {isSaving ? 'Saving...' : 'Save'}
          </button>

          <button
            onClick={runWorkflow}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-green-600 hover:bg-green-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-green-500 focus:ring-offset-2
            "
          >
            <PlayIcon className="h-5 w-5 mr-2" />
            Run
          </button>

          <button
            onClick={() => setIsExecutionView(true)}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-purple-600 hover:bg-purple-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-purple-500 focus:ring-offset-2
            "
          >
            <EyeIcon className="h-5 w-5 mr-2" />
            View Executions
          </button>

          <button
            onClick={clearWorkflow}
            className="
              inline-flex items-center px-4 py-2 rounded-lg
              bg-red-600 hover:bg-red-700
              text-white font-medium text-sm
              transition-colors duration-150
              focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2
            "
          >
            <XMarkIcon className="h-5 w-5 mr-2" />
            Clear
          </button>
        </div>
      </div>

      <div className="flex-1 flex overflow-hidden">
        <div className="w-64 flex-shrink-0 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 overflow-hidden">
          <NodePalette />
        </div>

        <div className="flex-1 relative" ref={reactFlowWrapper}>
          <ReactFlow
            nodes={nodes}
            edges={edges}
            onNodesChange={onNodesChange}
            onEdgesChange={onEdgesChange}
            onConnect={onConnect}
            onInit={setReactFlowInstance}
            onDrop={onDrop}
            onDragOver={onDragOver}
            onNodeClick={onNodeClick}
            onPaneClick={onPaneClick}
            nodeTypes={nodeTypes}
            fitView
            className="bg-gray-50 dark:bg-gray-900"
          >
            <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
            <Controls />
            <MiniMap
              nodeStrokeWidth={3}
              zoomable
              pannable
              className="bg-white dark:bg-gray-800"
            />
            <Panel position="top-right" className="bg-white dark:bg-gray-800 rounded-lg shadow-lg p-2">
              <div className="text-xs text-gray-600 dark:text-gray-400">
                Nodes: {nodes.length} | Edges: {edges.length}
              </div>
            </Panel>
          </ReactFlow>
        </div>

        {selectedNode && (
          <div className="w-80 flex-shrink-0 bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700 overflow-y-auto">
            <NodeConfigPanel
              selectedNode={selectedNode}
              onUpdate={(nodeId, data) => updateNodeData(nodeId, data)}
              onClose={() => setSelectedNode(null)}
            />
          </div>
        )}
      </div>
    </div>
    </>
  );
};

export default WorkflowBuilder;
