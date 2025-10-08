package workflow

import (
	"encoding/json"
	"fmt"
)

type WorkflowValidationErrorDetail struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
	EdgeID  string `json:"edge_id,omitempty"`
}

type ValidationError = WorkflowValidationErrorDetail

type ValidationResult struct {
	Valid  bool                            `json:"valid"`
	Errors []WorkflowValidationErrorDetail `json:"errors,omitempty"`
}

type Validator struct {
	nodeValidator *NodeValidator
}

func NewValidator() *Validator {
	return &Validator{
		nodeValidator: NewNodeValidator(),
	}
}

func (v *Validator) Validate(workflow *Workflow) *ValidationResult {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	v.validateBasicStructure(workflow, result)

	if len(result.Errors) == 0 {
		v.validateNodeIDs(workflow, result)
		v.validateEdges(workflow, result)
		v.validateDAG(workflow, result)
		v.validateNodeConfiguration(workflow, result)
		v.validateConnectivity(workflow, result)
	}

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

func (v *Validator) validateBasicStructure(workflow *Workflow, result *ValidationResult) {
	if workflow.Name == "" {
		result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
			Field:   "name",
			Message: "workflow name is required",
		})
	}

	if len(workflow.Nodes) == 0 {
		result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
			Field:   "nodes",
			Message: "workflow must have at least one node",
		})

		return
	}

	hasTrigger := false
	for _, node := range workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			hasTrigger = true

			break
		}
	}

	if !hasTrigger {
		result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
			Field:   "nodes",
			Message: "workflow must have at least one trigger node",
		})
	}
}

func (v *Validator) validateNodeIDs(workflow *Workflow, result *ValidationResult) {
	nodeIDs := make(map[string]bool)

	for _, node := range workflow.Nodes {
		if node.ID == "" {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "nodes",
				Message: "node ID cannot be empty",
			})

			continue
		}

		if nodeIDs[node.ID] {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "nodes",
				Message: fmt.Sprintf("duplicate node ID: %s", node.ID),
				NodeID:  node.ID,
			})
		}

		nodeIDs[node.ID] = true
	}
}

func (v *Validator) validateEdges(workflow *Workflow, result *ValidationResult) {
	nodeIDs := make(map[string]bool)
	for _, node := range workflow.Nodes {
		nodeIDs[node.ID] = true
	}

	edgeIDs := make(map[string]bool)

	for _, edge := range workflow.Edges {
		if edge.ID == "" {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "edges",
				Message: "edge ID cannot be empty",
			})

			continue
		}

		if edgeIDs[edge.ID] {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "edges",
				Message: fmt.Sprintf("duplicate edge ID: %s", edge.ID),
				EdgeID:  edge.ID,
			})
		}

		edgeIDs[edge.ID] = true

		if !nodeIDs[edge.Source] {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "edges",
				Message: fmt.Sprintf("edge source node not found: %s", edge.Source),
				EdgeID:  edge.ID,
			})
		}

		if !nodeIDs[edge.Target] {
			result.Errors = append(result.Errors, WorkflowValidationErrorDetail{
				Field:   "edges",
				Message: fmt.Sprintf("edge target node not found: %s", edge.Target),
				EdgeID:  edge.ID,
			})
		}
	}
}

func (v *Validator) validateDAG(workflow *Workflow, result *ValidationResult) {
	graph := NewGraph(workflow)

	cycles := graph.DetectCycles()

	if len(cycles) > 0 {
		for _, cycle := range cycles {
			result.Errors = append(result.Errors, ValidationError{
				Field:   "edges",
				Message: fmt.Sprintf("cycle detected in workflow: %v", cycle),
			})
		}
	}
}

func (v *Validator) validateNodeConfiguration(workflow *Workflow, result *ValidationResult) {
	for _, node := range workflow.Nodes {
		nodeErrors := v.nodeValidator.ValidateNode(&node)
		result.Errors = append(result.Errors, nodeErrors...)
	}
}

func (v *Validator) validateConnectivity(workflow *Workflow, result *ValidationResult) {
	incomingEdges := make(map[string]int)
	outgoingEdges := make(map[string][]WorkflowEdge)

	for _, edge := range workflow.Edges {
		incomingEdges[edge.Target]++
		outgoingEdges[edge.Source] = append(outgoingEdges[edge.Source], edge)
	}

	for _, node := range workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			if incomingEdges[node.ID] > 0 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "edges",
					Message: "trigger nodes cannot have incoming edges",
					NodeID:  node.ID,
				})
			}
		} else {
			if incomingEdges[node.ID] == 0 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "edges",
					Message: fmt.Sprintf("non-trigger node has no incoming edges: %s", node.ID),
					NodeID:  node.ID,
				})
			}
		}

		if node.Type == NodeTypeDecision {
			edges := outgoingEdges[node.ID]
			if len(edges) != 2 {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "edges",
					Message: fmt.Sprintf("decision node must have exactly 2 outgoing edges, has %d", len(edges)),
					NodeID:  node.ID,
				})

				continue
			}

			hasTrue := false
			hasFalse := false

			for _, edge := range edges {
				if edge.SourceHandle == "true" {
					hasTrue = true
				}

				if edge.SourceHandle == "false" {
					hasFalse = true
				}
			}

			if !hasTrue || !hasFalse {
				result.Errors = append(result.Errors, ValidationError{
					Field:   "edges",
					Message: "decision node must have edges labeled 'true' and 'false'",
					NodeID:  node.ID,
				})
			}
		}
	}
}

func (v *ValidationResult) AddError(field, message string) {
	v.Errors = append(v.Errors, ValidationError{
		Field:   field,
		Message: message,
	})
	v.Valid = false
}

func (v *ValidationResult) AddNodeError(field, message, nodeID string) {
	v.Errors = append(v.Errors, ValidationError{
		Field:   field,
		Message: message,
		NodeID:  nodeID,
	})
	v.Valid = false
}

func (v *ValidationResult) AddEdgeError(field, message, edgeID string) {
	v.Errors = append(v.Errors, ValidationError{
		Field:   field,
		Message: message,
		EdgeID:  edgeID,
	})
	v.Valid = false
}

func (v *ValidationResult) ToJSON() ([]byte, error) {
	return json.Marshal(v)
}
