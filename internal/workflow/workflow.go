package workflow

import (
	"encoding/json"
	"time"
)

type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	Version     int            `json:"version"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	CreatedBy   *string        `json:"created_by,omitempty"`
	Metadata    Metadata       `json:"metadata,omitempty"`
}

type WorkflowNode struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Position NodePosition    `json:"position"`
	Data     json.RawMessage `json:"data"`
}

type WorkflowEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	TargetHandle string `json:"targetHandle,omitempty"`
}

type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Metadata struct {
	Tags        []string               `json:"tags,omitempty"`
	Category    string                 `json:"category,omitempty"`
	CustomData  map[string]interface{} `json:"custom_data,omitempty"`
}

type WorkflowExecution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	NodeStates  []NodeExecutionState   `json:"node_states,omitempty"`
}

type NodeExecutionState struct {
	NodeID      string                 `json:"node_id"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
}

const (
	NodeTypeTrigger    = "trigger"
	NodeTypeAITask     = "ai-task"
	NodeTypeMCPServer  = "mcp-server"
	NodeTypeDecision   = "decision"
	NodeTypeTransform  = "transform"
	NodeTypeCode       = "code"

	ExecutionStatusPending   = "pending"
	ExecutionStatusRunning   = "running"
	ExecutionStatusCompleted = "completed"
	ExecutionStatusFailed    = "failed"
)

type WorkflowValidationError struct {
	Result *ValidationResult
}

func (e *WorkflowValidationError) Error() string {
	if e.Result != nil && len(e.Result.Errors) > 0 {
		return "workflow validation failed: " + e.Result.Errors[0].Message
	}

	return "workflow validation failed"
}
