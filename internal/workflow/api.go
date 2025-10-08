package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type APIHandler struct {
	storage  *Storage
	engine   *Engine
	logger   *logging.Logger
	hub      *Hub
	wsHandler *WebSocketHandler
}

func NewAPIHandler(storage *Storage, logger *logging.Logger) *APIHandler {
	engine := NewEngine(storage)
	hub := NewHub(logger)
	hub.Start()

	engine.SetHub(hub)

	wsHandler := NewWebSocketHandler(hub, storage, logger)

	return &APIHandler{
		storage:   storage,
		engine:    engine,
		logger:    logger,
		hub:       hub,
		wsHandler: wsHandler,
	}
}

func (h *APIHandler) SetAIManager(aiManager *ai.Manager) {
	if h.engine != nil {
		h.engine.SetAIManager(aiManager)
	}
}

func (h *APIHandler) HandleListWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	workflows, err := h.storage.ListWorkflows(ctx, 100)
	if err != nil {
		h.logger.Error("Failed to list workflows: %v", err)
		http.Error(w, "Failed to list workflows", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"workflows": workflows,
		"count":     len(workflows),
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		h.logger.Error("Failed to decode workflow: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.storage.CreateWorkflow(ctx, &workflow); err != nil {
		if validationErr, ok := err.(*WorkflowValidationError); ok {
			h.logger.Error("Workflow validation failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "Workflow validation failed",
				"validation_errors": validationErr.Result.Errors,
			}); err != nil {
				h.logger.Error("Failed to encode validation errors: %v", err)
			}

			return
		}

		h.logger.Error("Failed to create workflow: %v", err)
		http.Error(w, "Failed to create workflow", http.StatusInternalServerError)

		return
	}

	if err := h.createScheduledTaskForWorkflow(&workflow); err != nil {
		h.logger.Error("Failed to create scheduled task for workflow: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(workflow); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	workflowID := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	workflowID = strings.Split(workflowID, "/")[0]

	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	workflow, err := h.storage.GetWorkflow(ctx, workflowID)
	if err != nil {
		h.logger.Error("Failed to get workflow: %v", err)
		if err.Error() == "workflow not found" {
			http.Error(w, "Workflow not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get workflow", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workflow); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	workflowID := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	workflowID = strings.Split(workflowID, "/")[0]

	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)

		return
	}

	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		h.logger.Error("Failed to decode workflow: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	workflow.ID = workflowID

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.storage.UpdateWorkflow(ctx, &workflow); err != nil {
		if validationErr, ok := err.(*WorkflowValidationError); ok {
			h.logger.Error("Workflow validation failed: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			if err := json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "Workflow validation failed",
				"validation_errors": validationErr.Result.Errors,
			}); err != nil {
				h.logger.Error("Failed to encode validation errors: %v", err)
			}

			return
		}

		h.logger.Error("Failed to update workflow: %v", err)
		if err.Error() == "workflow not found" {
			http.Error(w, "Workflow not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update workflow", http.StatusInternalServerError)
		}

		return
	}

	if err := h.updateScheduledTaskForWorkflow(&workflow); err != nil {
		h.logger.Error("Failed to update scheduled task for workflow: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(workflow); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	workflowID := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	workflowID = strings.Split(workflowID, "/")[0]

	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.storage.DeleteWorkflow(ctx, workflowID); err != nil {
		h.logger.Error("Failed to delete workflow: %v", err)
		if err.Error() == "workflow not found" {
			http.Error(w, "Workflow not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete workflow", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Workflow deleted successfully",
		"id":      workflowID,
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleExecuteWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	workflowID := strings.TrimPrefix(r.URL.Path, "/api/workflows/")
	workflowID = strings.TrimSuffix(workflowID, "/execute")

	if workflowID == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)

		return
	}

	if r.Body != nil {
		var req struct {
			Input map[string]interface{} `json:"input"`
		}
		json.NewDecoder(r.Body).Decode(&req)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	startTime := time.Now()
	execution, err := h.engine.Execute(ctx, workflowID)
	duration := time.Since(startTime)

	response := map[string]interface{}{
		"executionId": "",
		"status":      "failed",
		"output":      make(map[string]interface{}),
		"duration":    duration.String(),
	}

	if execution != nil {
		response["executionId"] = execution.ID
		response["output"] = execution.Result
		response["nodeStates"] = execution.NodeStates
		if err == nil {
			response["status"] = execution.Status
		}
	}

	if err != nil {
		if validationErr, ok := err.(*WorkflowValidationError); ok {
			h.logger.Error("Workflow validation failed during execution: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			response["error"] = "Workflow validation failed"
			response["validation_errors"] = validationErr.Result.Errors
			if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
				h.logger.Error("Failed to encode validation errors: %v", encErr)
			}

			return
		}

		h.logger.Error("Failed to execute workflow: %v", err)
		response["error"] = err.Error()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
			h.logger.Error("Failed to encode response: %v", encErr)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleListExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/workflows/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)

		return
	}

	workflowID := parts[0]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	executions, err := h.storage.ListExecutions(ctx, workflowID, 50)
	if err != nil {
		h.logger.Error("Failed to list executions: %v", err)
		http.Error(w, "Failed to list executions", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"executions": executions,
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) HandleGetExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/workflows/"), "/")
	if len(parts) < 3 || parts[1] != "executions" {
		http.Error(w, "Invalid path", http.StatusBadRequest)

		return
	}

	executionID := parts[2]

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	execution, err := h.engine.GetExecutionStatus(ctx, executionID)
	if err != nil {
		h.logger.Error("Failed to get execution: %v", err)
		if err.Error() == "execution not found" {
			http.Error(w, "Execution not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get execution", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"execution": execution,
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *APIHandler) createScheduledTaskForWorkflow(workflow *Workflow) error {
	for _, node := range workflow.Nodes {
		if node.Type != "trigger" {
			continue
		}

		var nodeData map[string]interface{}
		if err := json.Unmarshal(node.Data, &nodeData); err != nil {
			continue
		}

		triggerType, _ := nodeData["triggerType"].(string)
		if triggerType != "cron" {
			continue
		}

		cronSchedule, _ := nodeData["cronSchedule"].(string)
		enabled, ok := nodeData["enabled"].(bool)
		if !ok {
			enabled = true
		}

		if cronSchedule == "" {
			continue
		}

		return h.createTaskSchedulerTask(workflow.ID, workflow.Name, cronSchedule, enabled)
	}

	return nil
}

func (h *APIHandler) updateScheduledTaskForWorkflow(workflow *Workflow) error {
	hasCronTrigger := false
	var cronSchedule string
	var enabled bool

	for _, node := range workflow.Nodes {
		if node.Type != "trigger" {
			continue
		}

		var nodeData map[string]interface{}
		if err := json.Unmarshal(node.Data, &nodeData); err != nil {
			continue
		}

		triggerType, _ := nodeData["triggerType"].(string)
		if triggerType != "cron" {
			continue
		}

		cronSchedule, _ = nodeData["cronSchedule"].(string)
		enabled, _ = nodeData["enabled"].(bool)

		if cronSchedule != "" {
			hasCronTrigger = true

			break
		}
	}

	taskID, err := h.findTaskByWorkflowID(workflow.ID)
	if err != nil {
		h.logger.Error("Error finding task for workflow: %v", err)
	}

	if !hasCronTrigger {
		if taskID != "" {
			return h.deleteTaskSchedulerTask(taskID)
		}

		return nil
	}

	if taskID != "" {
		return h.updateTaskSchedulerTask(taskID, workflow.Name, cronSchedule, enabled)
	}

	return h.createTaskSchedulerTask(workflow.ID, workflow.Name, cronSchedule, enabled)
}

func (h *APIHandler) createTaskSchedulerTask(workflowID, workflowName, schedule string, enabled bool) error {
	taskSchedulerURL := os.Getenv("TASK_SCHEDULER_URL")
	if taskSchedulerURL == "" {
		taskSchedulerURL = "http://mcp-compose-http-proxy:9876/task-scheduler"
	}

	apiKey := os.Getenv("MCP_API_KEY")

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "add_workflow_task",
			"arguments": map[string]interface{}{
				"name":         fmt.Sprintf("Workflow: %s", workflowName),
				"description":  fmt.Sprintf("Scheduled execution of workflow %s", workflowID),
				"workflowId":   workflowID,
				"workflowName": workflowName,
				"schedule":     schedule,
				"enabled":      enabled,
			},
		},
	}

	body, _ := json.Marshal(mcpRequest)

	req, err := http.NewRequest("POST", taskSchedulerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call task scheduler: %w", err)
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("task scheduler returned %d: %s", resp.StatusCode, responseBody)
	}

	var mcpResponse map[string]interface{}
	if err := json.Unmarshal(responseBody, &mcpResponse); err != nil {
		h.logger.Error("Failed to parse task scheduler response: %v, body: %s", err, responseBody)

		return fmt.Errorf("failed to parse response: %w", err)
	}

	if mcpError, hasError := mcpResponse["error"]; hasError {
		h.logger.Error("Task scheduler returned MCP error: %v", mcpError)

		return fmt.Errorf("task scheduler error: %v", mcpError)
	}

	h.logger.Info("Created scheduled task for workflow", "workflowId", workflowID, "schedule", schedule, "response", string(responseBody))

	return nil
}

func (h *APIHandler) findTaskByWorkflowID(workflowID string) (string, error) {
	taskSchedulerURL := os.Getenv("TASK_SCHEDULER_URL")
	if taskSchedulerURL == "" {
		taskSchedulerURL = "http://mcp-compose-http-proxy:9876/task-scheduler"
	}

	apiKey := os.Getenv("MCP_API_KEY")

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      "list_tasks",
			"arguments": map[string]interface{}{},
		},
	}

	body, _ := json.Marshal(mcpRequest)

	req, err := http.NewRequest("POST", taskSchedulerURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list tasks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("task scheduler returned %d", resp.StatusCode)
	}

	var mcpResponse struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&mcpResponse); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(mcpResponse.Result.Content) == 0 {
		return "", nil
	}

	var tasks []struct {
		ID         string `json:"id"`
		WorkflowID string `json:"workflowId"`
		Type       string `json:"type"`
	}

	if err := json.Unmarshal([]byte(mcpResponse.Result.Content[0].Text), &tasks); err != nil {
		return "", fmt.Errorf("failed to parse tasks: %w", err)
	}

	for _, task := range tasks {
		if task.Type == "workflow" && task.WorkflowID == workflowID {
			return task.ID, nil
		}
	}

	return "", nil
}

func (h *APIHandler) updateTaskSchedulerTask(taskID, workflowName, schedule string, enabled bool) error {
	taskSchedulerURL := os.Getenv("TASK_SCHEDULER_URL")
	if taskSchedulerURL == "" {
		taskSchedulerURL = "http://mcp-compose-http-proxy:9876/task-scheduler"
	}

	apiKey := os.Getenv("MCP_API_KEY")

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "update_task",
			"arguments": map[string]interface{}{
				"id":       taskID,
				"name":     fmt.Sprintf("Workflow: %s", workflowName),
				"schedule": schedule,
				"enabled":  enabled,
			},
		},
	}

	body, _ := json.Marshal(mcpRequest)

	req, err := http.NewRequest("POST", taskSchedulerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("task scheduler returned %d: %s", resp.StatusCode, responseBody)
	}

	h.logger.Info("Updated scheduled task for workflow", "taskId", taskID, "schedule", schedule)

	return nil
}

func (h *APIHandler) deleteTaskSchedulerTask(taskID string) error {
	taskSchedulerURL := os.Getenv("TASK_SCHEDULER_URL")
	if taskSchedulerURL == "" {
		taskSchedulerURL = "http://mcp-compose-http-proxy:9876/task-scheduler"
	}

	apiKey := os.Getenv("MCP_API_KEY")

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "remove_task",
			"arguments": map[string]interface{}{
				"id": taskID,
			},
		},
	}

	body, _ := json.Marshal(mcpRequest)

	req, err := http.NewRequest("POST", taskSchedulerURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("task scheduler returned %d: %s", resp.StatusCode, responseBody)
	}

	h.logger.Info("Deleted scheduled task", "taskId", taskID)

	return nil
}

func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/workflows", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.HandleListWorkflows(w, r)
		case http.MethodPost:
			h.HandleCreateWorkflow(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workflows/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/execute") {
			h.HandleExecuteWorkflow(w, r)

			return
		}

		if strings.HasSuffix(path, "/executions") {
			h.HandleListExecutions(w, r)

			return
		}

		if strings.Contains(path, "/executions/") {
			h.HandleGetExecution(w, r)

			return
		}

		switch r.Method {
		case http.MethodGet:
			h.HandleGetWorkflow(w, r)
		case http.MethodPut:
			h.HandleUpdateWorkflow(w, r)
		case http.MethodDelete:
			h.HandleDeleteWorkflow(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/ws/workflows/", h.wsHandler.HandleWorkflowExecutionWebSocket)

	fmt.Println("Registered workflow API routes:")
	fmt.Println("  GET    /api/workflows          - List all workflows")
	fmt.Println("  POST   /api/workflows          - Create workflow")
	fmt.Println("  GET    /api/workflows/:id      - Get workflow")
	fmt.Println("  PUT    /api/workflows/:id      - Update workflow")
	fmt.Println("  DELETE /api/workflows/:id      - Delete workflow")
	fmt.Println("  POST   /api/workflows/:id/execute - Execute workflow")
	fmt.Println("  GET    /api/workflows/:id/executions/:exec_id - Get execution status")
	fmt.Println("  WS     /ws/workflows/:id/executions/:exec_id - WebSocket for execution updates")
}

func (h *APIHandler) Shutdown() {
	if h.hub != nil {
		h.hub.Stop()
	}
}

func (h *APIHandler) GetStorage() *Storage {
	return h.storage
}

func (h *APIHandler) SetMCPProxyURL(url string) {
	if h.engine != nil {
		h.engine.SetMCPProxyURL(url)
	}
}

func (h *APIHandler) SetMCPAPIKey(apiKey string) {
	if h.engine != nil {
		h.engine.SetMCPAPIKey(apiKey)
	}
}
