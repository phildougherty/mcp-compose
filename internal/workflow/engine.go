package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/ai"
)

type Engine struct {
	storage     *Storage
	hub         *Hub
	executor    *NodeExecutor
	aiManager   *ai.Manager
	mcpProxyURL string
	mcpAPIKey   string
}

func NewEngine(storage *Storage) *Engine {
	return &Engine{
		storage: storage,
	}
}

func (e *Engine) SetHub(hub *Hub) {
	e.hub = hub
}

func (e *Engine) SetAIManager(aiManager *ai.Manager) {
	e.aiManager = aiManager
	if e.mcpProxyURL == "" {
		e.mcpProxyURL = "http://localhost:9876"
	}
	e.executor = NewNodeExecutor(aiManager, e.mcpProxyURL, e.mcpAPIKey)
}

func (e *Engine) SetMCPProxyURL(url string) {
	e.mcpProxyURL = url
	if e.executor != nil {
		e.executor.mcpProxyURL = url
	}
}

func (e *Engine) SetMCPAPIKey(apiKey string) {
	e.mcpAPIKey = apiKey
	if e.executor != nil {
		e.executor.mcpAPIKey = apiKey
	}
}

func (e *Engine) Execute(ctx context.Context, workflowID string) (*WorkflowExecution, error) {
	workflow, err := e.storage.GetWorkflow(ctx, workflowID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	validator := NewValidator()
	validationResult := validator.Validate(workflow)

	if !validationResult.Valid {
		return nil, &WorkflowValidationError{Result: validationResult}
	}

	execution := &WorkflowExecution{
		WorkflowID: workflowID,
		Status:     ExecutionStatusPending,
		StartedAt:  time.Now(),
		Result:     make(map[string]interface{}),
		NodeStates: []NodeExecutionState{},
	}

	if err := e.storage.CreateExecution(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	execution.Status = ExecutionStatusRunning
	if err := e.storage.UpdateExecution(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to update execution status: %w", err)
	}

	e.broadcastUpdate(ExecutionUpdate{
		Type:        "execution_started",
		ExecutionID: execution.ID,
		WorkflowID:  workflowID,
		Status:      ExecutionStatusRunning,
		Timestamp:   time.Now().Format(time.RFC3339Nano),
	})

	if err := e.executeWorkflow(ctx, workflow, execution); err != nil {
		now := time.Now()
		execution.Status = ExecutionStatusFailed
		execution.CompletedAt = &now
		execution.Error = err.Error()
		e.storage.UpdateExecution(ctx, execution)

		e.broadcastUpdate(ExecutionUpdate{
			Type:        "execution_completed",
			ExecutionID: execution.ID,
			WorkflowID:  workflowID,
			Status:      ExecutionStatusFailed,
			Error:       err.Error(),
			Duration:    now.Sub(execution.StartedAt).Milliseconds(),
			Timestamp:   now.Format(time.RFC3339Nano),
		})

		return execution, err
	}

	now := time.Now()
	execution.Status = ExecutionStatusCompleted
	execution.CompletedAt = &now
	if err := e.storage.UpdateExecution(ctx, execution); err != nil {
		return nil, fmt.Errorf("failed to update execution completion: %w", err)
	}

	e.broadcastUpdate(ExecutionUpdate{
		Type:        "execution_completed",
		ExecutionID: execution.ID,
		WorkflowID:  workflowID,
		Status:      ExecutionStatusCompleted,
		Duration:    now.Sub(execution.StartedAt).Milliseconds(),
		Timestamp:   now.Format(time.RFC3339Nano),
	})

	return execution, nil
}

func (e *Engine) executeWorkflow(ctx context.Context, workflow *Workflow, execution *WorkflowExecution) error {
	if e.executor == nil {
		if e.mcpProxyURL == "" {
			e.mcpProxyURL = "http://localhost:9876"
		}
		e.executor = NewNodeExecutor(e.aiManager, e.mcpProxyURL, e.mcpAPIKey)
	}

	execCtx := NewExecutionContext()

	graph := BuildDependencyGraph(workflow)

	executionOrder, err := TopologicalSort(graph)
	if err != nil {
		return fmt.Errorf("failed to determine execution order: %w", err)
	}

	nodeMap := make(map[string]*WorkflowNode)
	for i := range workflow.Nodes {
		nodeMap[workflow.Nodes[i].ID] = &workflow.Nodes[i]
	}

	edgeMap := make(map[string][]WorkflowEdge)
	for _, edge := range workflow.Edges {
		edgeMap[edge.Target] = append(edgeMap[edge.Target], edge)
	}

	completedNodes := make(map[string]bool)
	skippedNodes := make(map[string]bool)
	nodeStateMutex := sync.Mutex{}

	for _, nodeID := range executionOrder {
		node, exists := nodeMap[nodeID]
		if !exists {
			continue
		}

		dependencies := graph[nodeID]

		for _, depID := range dependencies {
			if !completedNodes[depID] && !skippedNodes[depID] {
				return fmt.Errorf("dependency %s not completed for node %s", depID, nodeID)
			}
		}

		shouldSkip := false
		for _, edge := range edgeMap[nodeID] {
			sourceNode := nodeMap[edge.Source]
			if sourceNode != nil && sourceNode.Type == NodeTypeDecision {
				if decisionOutput, exists := execCtx.NodeOutputs[edge.Source]; exists {
					if decision, ok := decisionOutput["decision"].(bool); ok {
						expectedPath := "true"
						if !decision {
							expectedPath = "false"
						}
						if edge.SourceHandle != "" && edge.SourceHandle != expectedPath {
							shouldSkip = true

							break
						}
					}
				}
			}
		}

		if shouldSkip {
			skippedNodes[nodeID] = true

			continue
		}

		nodeState := NodeExecutionState{
			NodeID:    node.ID,
			Status:    ExecutionStatusRunning,
			StartedAt: time.Now(),
			Output:    make(map[string]interface{}),
		}

		e.broadcastUpdate(ExecutionUpdate{
			Type:        "node_started",
			ExecutionID: execution.ID,
			WorkflowID:  execution.WorkflowID,
			NodeID:      node.ID,
			Status:      ExecutionStatusRunning,
			Timestamp:   time.Now().Format(time.RFC3339Nano),
		})

		upstreamInput := make(map[string]interface{})
		for _, depID := range dependencies {
			if depOutput, exists := execCtx.NodeOutputs[depID]; exists {
				for key, value := range depOutput {
					upstreamInput[key] = value
				}
			}
		}
		execCtx.SetInput(upstreamInput)

		output, err := e.executor.ExecuteNode(ctx, node, execCtx)
		if err != nil {
			nodeState.Error = err.Error()
			nodeState.Status = ExecutionStatusFailed
			now := time.Now()
			nodeState.CompletedAt = &now

			nodeStateMutex.Lock()
			execution.NodeStates = append(execution.NodeStates, nodeState)
			nodeStateMutex.Unlock()

			e.broadcastUpdate(ExecutionUpdate{
				Type:        "node_error",
				ExecutionID: execution.ID,
				WorkflowID:  execution.WorkflowID,
				NodeID:      node.ID,
				Error:       err.Error(),
				Timestamp:   now.Format(time.RFC3339Nano),
			})

			return fmt.Errorf("node %s failed: %w", node.ID, err)
		}

		nodeState.Output = output
		execCtx.SetNodeOutput(node.ID, output)

		now := time.Now()
		nodeState.Status = ExecutionStatusCompleted
		nodeState.CompletedAt = &now

		nodeStateMutex.Lock()
		execution.NodeStates = append(execution.NodeStates, nodeState)
		nodeStateMutex.Unlock()

		completedNodes[node.ID] = true

		e.broadcastUpdate(ExecutionUpdate{
			Type:        "node_completed",
			ExecutionID: execution.ID,
			WorkflowID:  execution.WorkflowID,
			NodeID:      node.ID,
			Output:      nodeState.Output,
			Timestamp:   now.Format(time.RFC3339Nano),
		})
	}

	execution.Result["message"] = "Workflow execution completed successfully"
	execution.Result["nodes_executed"] = len(completedNodes)
	execution.Result["total_nodes"] = len(workflow.Nodes)

	return nil
}

func (e *Engine) GetExecutionStatus(ctx context.Context, executionID string) (*WorkflowExecution, error) {
	return e.storage.GetExecution(ctx, executionID)
}

func (e *Engine) broadcastUpdate(update ExecutionUpdate) {
	if e.hub != nil {
		e.hub.BroadcastUpdate(update)
	}
}
