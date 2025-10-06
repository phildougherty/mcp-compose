package agent

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

	"github.com/google/uuid"
	"mcp-cron-persistent/internal/model"
)

type Agent struct {
	aiProvider AIProvider
	storage    TaskStorage
	logger     Logger
	httpClient *http.Client
}

type AIProvider interface {
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
}

type TaskStorage interface {
	RecordTaskRun(ctx context.Context, run *model.TaskRun) error
	GetTask(ctx context.Context, taskID string) (*model.Task, error)
}

type Logger interface {
	Info(format string, args ...interface{})
	Warning(format string, args ...interface{})
	Error(format string, args ...interface{})
	Debug(format string, args ...interface{})
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type ChatResponse struct {
	TextContent  string `json:"text_content"`
	TokensUsed   int    `json:"tokens_used,omitempty"`
	CostEstimate float64 `json:"cost_estimate,omitempty"`
}

func NewAgent(aiProvider AIProvider, storage TaskStorage, logger Logger) *Agent {
	return &Agent{
		aiProvider: aiProvider,
		storage:    storage,
		logger:     logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (a *Agent) Execute(ctx context.Context, task *model.Task) (*model.TaskRun, error) {
	run := &model.TaskRun{
		ID:        uuid.New().String(),
		TaskID:    task.ID,
		StartedAt: time.Now(),
		Status:    "running",
	}

	if task.ChatSessionID != "" && task.InheritSessionContext {
		return a.executeWithChatContext(ctx, task, run)
	}

	return a.executeStandard(ctx, task, run)
}

func (a *Agent) executeWithChatContext(ctx context.Context, task *model.Task, run *model.TaskRun) (*model.TaskRun, error) {
	chatContext, err := a.fetchChatSessionContext(task.ChatSessionID, 10)
	if err != nil {
		a.logger.Warning("Failed to fetch chat context: %v", err)
	}

	systemPrompt := a.buildSystemPromptWithContext(task, chatContext)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task.Prompt},
	}

	tools := a.getMCPToolsForTask(task)

	response, err := a.aiProvider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		run.Error = err.Error()
		run.Status = "failed"
		run.FinishedAt = time.Now()

		return run, err
	}

	run.Output = response.TextContent
	run.Status = "completed"
	run.FinishedAt = time.Now()
	run.TokensUsed = response.TokensUsed
	run.CostEstimate = response.CostEstimate

	if err := a.storage.RecordTaskRun(ctx, run); err != nil {
		a.logger.Error("Failed to save task run: %v", err)
	}

	if task.OutputToChat {
		if err := a.postResultToChat(task.ChatSessionID, run); err != nil {
			a.logger.Error("Failed to post to chat: %v", err)
		} else {
			run.PostedToChat = true
		}
	}

	return run, nil
}

func (a *Agent) executeStandard(ctx context.Context, task *model.Task, run *model.TaskRun) (*model.TaskRun, error) {
	systemPrompt := a.buildStandardSystemPrompt(task)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task.Prompt},
	}

	tools := a.getMCPToolsForTask(task)

	response, err := a.aiProvider.ChatWithTools(ctx, messages, tools)
	if err != nil {
		run.Error = err.Error()
		run.Status = "failed"
		run.FinishedAt = time.Now()

		return run, err
	}

	run.Output = response.TextContent
	run.Status = "completed"
	run.FinishedAt = time.Now()
	run.TokensUsed = response.TokensUsed
	run.CostEstimate = response.CostEstimate

	if err := a.storage.RecordTaskRun(ctx, run); err != nil {
		a.logger.Error("Failed to save task run: %v", err)
	}

	return run, nil
}

func (a *Agent) fetchChatSessionContext(sessionID string, limit int) ([]model.ChatMessage, error) {
	dashboardURL := os.Getenv("DASHBOARD_INTERNAL_URL")
	if dashboardURL == "" {
		dashboardURL = "http://mcp-compose-dashboard:3001"
	}

	url := fmt.Sprintf("%s/api/internal/chat/sessions/%s/context?limit=%d",
		dashboardURL, sessionID, limit)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Request", "true")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat context: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var messages []model.ChatMessage
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return messages, nil
}

func (a *Agent) buildSystemPromptWithContext(task *model.Task, chatContext []model.ChatMessage) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are an automated agent running as part of a scheduled task.

Task Name: %s
Task Description: %s
Schedule: %s

`, task.Name, task.Description, task.Schedule))

	if len(chatContext) > 0 {
		prompt.WriteString("Recent conversation context:\n")
		for i, msg := range chatContext {
			content := truncate(msg.Content, 200)
			prompt.WriteString(fmt.Sprintf("[%d] %s: %s\n", i+1, msg.Role, content))
		}
		prompt.WriteString("\n")
	}

	if task.IsAgent {
		prompt.WriteString(`You are running in agent mode with persistent memory.
You can store important information for future executions using the memory tools.
Reference previous executions to maintain continuity.

`)
	}

	prompt.WriteString(`Execute your task and provide a concise update.
You have access to the same MCP tools as the chat session.
Your output will appear in the chat conversation.`)

	return prompt.String()
}

func (a *Agent) buildStandardSystemPrompt(task *model.Task) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf(`You are an automated agent running a scheduled task.

Task Name: %s
Task Description: %s
Schedule: %s

`, task.Name, task.Description, task.Schedule))

	if task.IsAgent {
		prompt.WriteString(`You are running in agent mode with persistent memory.
You can store important information for future executions.

`)
	}

	prompt.WriteString("Execute your task and provide a concise update.")

	return prompt.String()
}

func (a *Agent) getMCPToolsForTask(task *model.Task) []Tool {
	tools := make([]Tool, 0)

	if len(task.MCPServers) == 0 {
		return tools
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://mcp-compose-proxy:9876"
	}

	apiKey := os.Getenv("MCP_PROXY_API_KEY")

	for _, serverName := range task.MCPServers {
		serverTools, err := a.fetchMCPServerTools(proxyURL, apiKey, serverName)
		if err != nil {
			a.logger.Warning("Failed to fetch tools from MCP server %s: %v", serverName, err)

			continue
		}

		tools = append(tools, serverTools...)
	}

	return tools
}

func (a *Agent) fetchMCPServerTools(proxyURL, apiKey, serverName string) ([]Tool, error) {
	url := fmt.Sprintf("%s/%s", proxyURL, serverName)

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/list",
		"params":  map[string]interface{}{},
		"id":      1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tools: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Result struct {
			Tools []map[string]interface{} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("MCP error: %s", result.Error.Message)
	}

	tools := make([]Tool, 0, len(result.Result.Tools))
	for _, mcpTool := range result.Result.Tools {
		name, _ := mcpTool["name"].(string)
		desc, _ := mcpTool["description"].(string)
		inputSchema, _ := mcpTool["inputSchema"].(map[string]interface{})

		if name != "" {
			fullToolName := fmt.Sprintf("mcp_%s_%s", serverName, name)

			tools = append(tools, Tool{
				Name:        fullToolName,
				Description: fmt.Sprintf("[MCP:%s] %s", serverName, desc),
				InputSchema: inputSchema,
			})
		}
	}

	return tools, nil
}

func (a *Agent) postResultToChat(sessionID string, run *model.TaskRun) error {
	dashboardURL := os.Getenv("DASHBOARD_INTERNAL_URL")
	if dashboardURL == "" {
		dashboardURL = "http://mcp-compose-dashboard:3001"
	}

	payload := map[string]interface{}{
		"session_id":       sessionID,
		"role":             "assistant",
		"content":          run.Output,
		"is_automated":     true,
		"from_task_run_id": run.ID,
		"tokens_used":      run.TokensUsed,
		"cost_estimate":    run.CostEstimate,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/api/internal/task-output", dashboardURL)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Request", "true")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post to chat: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)

		return fmt.Errorf("failed to post to chat: status %d: %s", resp.StatusCode, string(body))
	}

	var respData struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err == nil {
		run.ChatMessageID = respData.MessageID
	}

	return nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	return s[:maxLen-3] + "..."
}
