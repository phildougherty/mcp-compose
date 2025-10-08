# Workflow Validation Implementation Summary

This document describes the comprehensive workflow validation system implemented for MCP-Compose, including cycle detection and schema validation.

## Overview

The workflow validation system ensures that workflows are valid before being saved or executed. It includes structural validation, cycle detection, node configuration validation, and connectivity checks.

## Architecture

### Core Components

1. **Validator** (`internal/workflow/validator.go`)
   - Main validation orchestrator
   - Coordinates all validation checks
   - Returns structured validation results

2. **Graph** (`internal/workflow/graph.go`)
   - DAG validation and cycle detection
   - Topological sorting
   - Reachability analysis

3. **Node Validator** (`internal/workflow/node_validator.go`)
   - Node-specific validation rules
   - JavaScript syntax validation
   - Cron schedule validation

## Validation Rules

### Basic Structure Validation

- Workflow must have a name
- At least one node must exist
- At least one trigger node must exist

### Node ID Validation

- All node IDs must be unique
- Node IDs cannot be empty
- No duplicate node IDs allowed

### Edge Validation

- All edge IDs must be unique
- Edge IDs cannot be empty
- Source and target nodes must exist
- No duplicate edge IDs allowed

### DAG Validation (Cycle Detection)

- Uses DFS-based cycle detection algorithm
- Detects and reports all cycles in the workflow graph
- Returns the path of nodes involved in each cycle
- Prevents infinite loops during execution

### Node Configuration Validation

#### TriggerNode
- Must have schedule, webhook, OR event type
- Cron schedule must be valid if present (validated using robfig/cron parser)
- Webhook path must start with / and contain only alphanumeric characters, hyphens, and slashes
- Event type cannot be empty if specified

#### AITaskNode
- Must have a prompt
- Provider must be valid: openrouter, openai, anthropic, or local
- Model hint must be 256 characters or less if specified

#### MCPServerNode
- Must have server_name
- Must have tool_name
- Parameters must be valid JSON if present

#### DecisionNode
- Must have condition code
- Condition must be valid JavaScript syntax (validated using otto VM)
- Must have exactly 2 outgoing edges
- Edges must be labeled 'true' and 'false'

#### TransformNode
- Must have transform code
- Code must be valid JavaScript syntax
- Error handling mode must be 'fail', 'continue', or 'retry' if specified

#### CodeNode
- Must have code
- Language must be javascript, python, or bash if specified
- Timeout must be positive if set

### Connectivity Validation

- Trigger nodes cannot have incoming edges
- Non-trigger nodes must have at least one incoming edge (prevents orphaned nodes)
- Decision nodes must have exactly 2 outgoing edges with 'true' and 'false' labels

## Cycle Detection Algorithm

The cycle detection uses a Depth-First Search (DFS) algorithm with recursion stack tracking:

```go
func (g *Graph) DetectCycles() [][]string
```

**Algorithm:**
1. Maintains visited and recursion stack maps
2. For each unvisited node, performs DFS
3. If a node is encountered that's already in the recursion stack, a cycle is detected
4. Returns the path of nodes forming the cycle
5. Supports multiple entry points (trigger nodes)

**Time Complexity:** O(V + E) where V is vertices (nodes) and E is edges

## Validation Error Reporting

### Error Structure

```go
type WorkflowValidationErrorDetail struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    NodeID  string `json:"node_id,omitempty"`
    EdgeID  string `json:"edge_id,omitempty"`
}

type ValidationResult struct {
    Valid  bool                            `json:"valid"`
    Errors []WorkflowValidationErrorDetail `json:"errors,omitempty"`
}

type WorkflowValidationError struct {
    Result *ValidationResult
}
```

### Error Response Format

When validation fails via API, the response is:

```json
{
  "error": "Workflow validation failed",
  "validation_errors": [
    {
      "field": "nodes",
      "message": "workflow must have at least one trigger node"
    },
    {
      "field": "edges",
      "message": "cycle detected in workflow: [node1, node2, node3, node1]"
    },
    {
      "field": "data.schedule",
      "message": "invalid cron schedule: ...",
      "node_id": "trigger-1"
    }
  ]
}
```

## Integration Points

### Storage Layer

**CreateWorkflow** (`internal/workflow/storage.go`)
```go
func (s *Storage) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
    validator := NewValidator()
    validationResult := validator.Validate(workflow)

    if !validationResult.Valid {
        return &WorkflowValidationError{Result: validationResult}
    }
    // ... create workflow
}
```

**UpdateWorkflow** (`internal/workflow/storage.go`)
```go
func (s *Storage) UpdateWorkflow(ctx context.Context, workflow *Workflow) error {
    validator := NewValidator()
    validationResult := validator.Validate(workflow)

    if !validationResult.Valid {
        return &WorkflowValidationError{Result: validationResult}
    }
    // ... update workflow
}
```

### Engine Layer

**Execute** (`internal/workflow/engine.go`)
```go
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
    // ... execute workflow
}
```

### API Layer

**HandleCreateWorkflow** (`internal/workflow/api.go`)
```go
if err := h.storage.CreateWorkflow(ctx, &workflow); err != nil {
    if validationErr, ok := err.(*WorkflowValidationError); ok {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusBadRequest)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "error": "Workflow validation failed",
            "validation_errors": validationErr.Result.Errors,
        })
        return
    }
    // ... other errors
}
```

**HandleUpdateWorkflow** (`internal/workflow/api.go`)
- Same pattern as CreateWorkflow

**HandleExecuteWorkflow** (`internal/workflow/api.go`)
- Same pattern as CreateWorkflow

## Dependencies

The validation system requires these Go packages:

- `github.com/robertkrimen/otto` - JavaScript syntax validation
- `github.com/robfig/cron/v3` - Cron schedule validation

## Usage Example

```go
// Create a validator
validator := NewValidator()

// Validate a workflow
result := validator.Validate(workflow)

// Check if valid
if !result.Valid {
    for _, err := range result.Errors {
        fmt.Printf("Validation error in %s: %s\n", err.Field, err.Message)
        if err.NodeID != "" {
            fmt.Printf("  Node: %s\n", err.NodeID)
        }
        if err.EdgeID != "" {
            fmt.Printf("  Edge: %s\n", err.EdgeID)
        }
    }
}
```

## Testing Validation

To test the validation system:

```bash
# Test basic compilation
go build ./internal/workflow/validator.go ./internal/workflow/node_validator.go ./internal/workflow/graph.go

# Create a workflow with validation errors via API
curl -X POST http://localhost:9876/api/workflows \
  -H "Content-Type: application/json" \
  -d '{
    "name": "",
    "nodes": [],
    "edges": []
  }'

# Expected response:
# {
#   "error": "Workflow validation failed",
#   "validation_errors": [
#     {"field": "name", "message": "workflow name is required"},
#     {"field": "nodes", "message": "workflow must have at least one node"}
#   ]
# }
```

## Validation Execution Flow

1. **API Request** - User submits workflow create/update/execute request
2. **Validation** - Validator runs all validation checks:
   - Basic structure
   - Node IDs
   - Edges
   - DAG (cycle detection)
   - Node configuration
   - Connectivity
3. **Error Handling** - If validation fails:
   - Returns HTTP 400 Bad Request
   - Returns structured JSON with all validation errors
4. **Success** - If validation passes:
   - Proceeds with create/update/execute operation
   - Returns success response

## Future Enhancements

Potential improvements to the validation system:

1. **Validation Levels** - Warning vs Error severity
2. **Custom Validators** - Plugin system for custom validation rules
3. **Schema Validation** - JSON Schema validation for node data
4. **Performance** - Caching validation results for unchanged workflows
5. **Linting** - Style and best practice checks
6. **Async Validation** - Long-running validation for complex workflows
