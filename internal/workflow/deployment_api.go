package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type DeploymentRequest struct {
	Description string                 `json:"description"`
	TemplateID  string                 `json:"templateId,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	AutoStart   bool                   `json:"autoStart"`
}

type DeploymentResponse struct {
	WorkflowID  string         `json:"workflowId"`
	Name        string         `json:"name"`
	Preview     string         `json:"preview"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	Deployed    bool           `json:"deployed"`
	ExecutionID string         `json:"executionId,omitempty"`
}

type DeploymentError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type DeploymentHandler struct {
	storage          *Storage
	engine           *Engine
	processor        *DeploymentProcessor
	templateFiller   *TemplateFiller
	paramExtractor   *ParameterExtractor
	logger           *logging.Logger
	aiManager        *ai.Manager
	templateRegistry *TemplateRegistry
}

func NewDeploymentHandler(storage *Storage, aiManager *ai.Manager, logger *logging.Logger) *DeploymentHandler {
	engine := NewEngine(storage)
	processor := NewDeploymentProcessor(aiManager, logger)
	templateFiller := NewTemplateFiller(logger)
	paramExtractor := NewParameterExtractor(aiManager, logger)

	return &DeploymentHandler{
		storage:          storage,
		engine:           engine,
		processor:        processor,
		templateFiller:   templateFiller,
		paramExtractor:   paramExtractor,
		logger:           logger,
		aiManager:        aiManager,
		templateRegistry: NewTemplateRegistry(),
	}
}

func (h *DeploymentHandler) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.sendError(w, http.StatusMethodNotAllowed, "invalid_method", "Only POST method is allowed", "")

		return
	}

	var req DeploymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, http.StatusBadRequest, "invalid_request", "Failed to parse request body", err.Error())

		return
	}

	if req.Description == "" {
		h.sendError(w, http.StatusBadRequest, "missing_description", "Workflow description is required", "")

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	resp, err := h.deployWorkflow(ctx, &req)
	if err != nil {
		h.logger.Error("Deployment failed: %v", err)

		switch err.(type) {
		case *DeploymentValidationError:
			h.sendError(w, http.StatusBadRequest, "validation_error", err.Error(), "")
		case *WorkflowValidationError:
			h.sendError(w, http.StatusBadRequest, "validation_error", err.Error(), "")
		case *TemplateNotFoundError:
			h.sendError(w, http.StatusNotFound, "template_not_found", err.Error(), "")
		default:
			h.sendError(w, http.StatusInternalServerError, "deployment_failed", "Failed to deploy workflow", err.Error())
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *DeploymentHandler) deployWorkflow(ctx context.Context, req *DeploymentRequest) (*DeploymentResponse, error) {
	var workflow *Workflow
	var err error
	var template *WorkflowTemplate

	if req.TemplateID != "" {
		template, err = h.templateRegistry.GetTemplate(req.TemplateID)
		if err != nil {
			return nil, &TemplateNotFoundError{TemplateID: req.TemplateID}
		}

		extractedParams, err := h.paramExtractor.ExtractFromDescription(ctx, req.Description, template)
		if err != nil {
			return nil, fmt.Errorf("failed to extract parameters: %w", err)
		}

		for k, v := range req.Parameters {
			extractedParams[k] = v
		}

		workflow, err = h.templateFiller.FillTemplate(template, extractedParams)
		if err != nil {
			return nil, err
		}
	} else {
		template, extractedParams, err := h.processor.MatchTemplate(ctx, req.Description)
		if err != nil {
			h.logger.Info("No template matched, generating workflow from scratch")

			workflow, err = h.processor.GenerateWorkflow(ctx, req.Description)
			if err != nil {
				return nil, fmt.Errorf("failed to generate workflow: %w", err)
			}
		} else {
			for k, v := range req.Parameters {
				extractedParams[k] = v
			}

			workflow, err = h.templateFiller.FillTemplate(template, extractedParams)
			if err != nil {
				return nil, err
			}
		}
	}

	if err := h.validateWorkflow(workflow); err != nil {
		return nil, err
	}

	workflow.ID = uuid.New().String()
	workflow.CreatedAt = time.Now()
	workflow.UpdatedAt = time.Now()

	if err := h.storage.CreateWorkflow(ctx, workflow); err != nil {
		return nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	resp := &DeploymentResponse{
		WorkflowID: workflow.ID,
		Name:       workflow.Name,
		Preview:    workflow.Description,
		Nodes:      workflow.Nodes,
		Edges:      workflow.Edges,
		Deployed:   true,
	}

	if req.AutoStart {
		execution, err := h.engine.Execute(ctx, workflow.ID)
		if err != nil {
			h.logger.Error("Failed to auto-start workflow: %v", err)
		} else {
			resp.ExecutionID = execution.ID
		}
	}

	return resp, nil
}

func (h *DeploymentHandler) validateWorkflow(workflow *Workflow) error {
	if workflow.Name == "" {
		return &DeploymentValidationError{Field: "name", Message: "Workflow name is required"}
	}

	if len(workflow.Nodes) == 0 {
		return &DeploymentValidationError{Field: "nodes", Message: "Workflow must have at least one node"}
	}

	nodeIDs := make(map[string]bool)
	for _, node := range workflow.Nodes {
		if node.ID == "" {
			return &DeploymentValidationError{Field: "node.id", Message: "Node ID is required"}
		}

		if nodeIDs[node.ID] {
			return &DeploymentValidationError{Field: "node.id", Message: fmt.Sprintf("Duplicate node ID: %s", node.ID)}
		}
		nodeIDs[node.ID] = true

		if node.Type == "" {
			return &DeploymentValidationError{Field: "node.type", Message: fmt.Sprintf("Node %s type is required", node.ID)}
		}

		if !isValidNodeType(node.Type) {
			return &DeploymentValidationError{
				Field:   "node.type",
				Message: fmt.Sprintf("Invalid node type: %s", node.Type),
			}
		}
	}

	for _, edge := range workflow.Edges {
		if !nodeIDs[edge.Source] {
			return &DeploymentValidationError{
				Field:   "edge.source",
				Message: fmt.Sprintf("Edge references non-existent source node: %s", edge.Source),
			}
		}

		if !nodeIDs[edge.Target] {
			return &DeploymentValidationError{
				Field:   "edge.target",
				Message: fmt.Sprintf("Edge references non-existent target node: %s", edge.Target),
			}
		}
	}

	return nil
}

func isValidNodeType(nodeType string) bool {
	validTypes := map[string]bool{
		NodeTypeTrigger:   true,
		NodeTypeAITask:    true,
		NodeTypeMCPServer: true,
		NodeTypeDecision:  true,
		NodeTypeTransform: true,
		NodeTypeCode:      true,
	}

	return validTypes[nodeType]
}

func (h *DeploymentHandler) sendError(w http.ResponseWriter, status int, code, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	err := DeploymentError{
		Code:    code,
		Message: message,
		Details: details,
	}

	if encodeErr := json.NewEncoder(w).Encode(err); encodeErr != nil {
		h.logger.Error("Failed to encode error response: %v", encodeErr)
	}
}

type DeploymentValidationError struct {
	Field   string
	Message string
}

func (e *DeploymentValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)
}

type TemplateNotFoundError struct {
	TemplateID string
}

func (e *TemplateNotFoundError) Error() string {
	return fmt.Sprintf("template not found: %s", e.TemplateID)
}
