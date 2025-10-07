package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/memory"
)

// ServerManager defines the interface for server management operations
type ServerManager interface {
	GetServerStatus(name string) (string, error)
	StartServer(name string) error
	StopServer(name string) error
	GetServerInstance(serverName string) (interface{}, bool)
}

// TaskSchedulerManager defines the interface for task scheduler operations
type TaskSchedulerManager interface {
	Start() error
	Stop() error
	Status() (string, error)
}

// MemoryManager defines the interface for memory management operations
type MemoryManager interface {
	SearchSimilar(ctx context.Context, query string) ([]memory.SearchResult, error)
	HybridSearch(ctx context.Context, query string) ([]memory.SearchResult, error)
	StoreEntity(ctx context.Context, name, entityType, description string, metadata map[string]interface{}, importance float64) (string, error)
	GetMemoryStats(ctx context.Context) (map[string]interface{}, error)
	RunPruning(ctx context.Context) (*memory.PruningResult, error)
}

// SystemToolsManager provides system management tools for the chat interface
type SystemToolsManager struct {
	config        *config.ComposeConfig
	serverManager ServerManager
	taskScheduler TaskSchedulerManager
	memoryManager MemoryManager
}

// NewSystemToolsManager creates a new system tools manager
func NewSystemToolsManager(
	cfg *config.ComposeConfig,
	serverMgr ServerManager,
	taskSched TaskSchedulerManager,
	memMgr MemoryManager,
) *SystemToolsManager {
	return &SystemToolsManager{
		config:        cfg,
		serverManager: serverMgr,
		taskScheduler: taskSched,
		memoryManager: memMgr,
	}
}

// ToolDefinition represents an MCP tool definition
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// GetSystemTools returns all available system tools
func (stm *SystemToolsManager) GetSystemTools() []ToolDefinition {
	return []ToolDefinition{
		// Task Scheduler tools
		{
			Name:        "task_scheduler_status",
			Description: "Get the current status of the task scheduler service",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "task_scheduler_start",
			Description: "Start the task scheduler service",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "task_scheduler_stop",
			Description: "Stop the task scheduler service",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},

		// Server Management tools
		{
			Name:        "server_list",
			Description: "List all MCP servers and their current status",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "server_start",
			Description: "Start a specific MCP server",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the server to start",
					},
				},
				"required": []string{"server_name"},
			},
		},
		{
			Name:        "server_stop",
			Description: "Stop a specific MCP server",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the server to stop",
					},
				},
				"required": []string{"server_name"},
			},
		},
		{
			Name:        "server_restart",
			Description: "Restart a specific MCP server",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the server to restart",
					},
				},
				"required": []string{"server_name"},
			},
		},
		{
			Name:        "server_logs",
			Description: "Get recent logs from a specific MCP server",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"server_name": map[string]interface{}{
						"type":        "string",
						"description": "Name of the server",
					},
					"lines": map[string]interface{}{
						"type":        "integer",
						"description": "Number of log lines to retrieve",
						"default":     50,
					},
				},
				"required": []string{"server_name"},
			},
		},

		{
			Name:        "task_scheduler_create_task",
			Description: "Create a scheduled task that runs automatically at specified intervals. The task output will appear in this chat conversation. Use this when the user wants something done repeatedly, at specific times, or wants to set up an autonomous agent. Tasks inherit this session's AI provider, model, and MCP server access.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Short, descriptive name for the task (e.g., 'Glucose Monitor', 'Daily Summary')",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Detailed description of what this task does and why",
					},
					"task_type": map[string]interface{}{
						"type":        "string",
						"description": "Task type: 'ai' for tasks requiring AI reasoning and MCP tools, 'shell' for simple bash commands",
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "For AI tasks: the instruction/prompt to execute. The AI will have access to all MCP tools enabled in this session.",
					},
					"command": map[string]interface{}{
						"type":        "string",
						"description": "For shell tasks: the bash command to execute",
					},
					"schedule": map[string]interface{}{
						"type":        "string",
						"description": "Cron schedule expression. Examples: '*/5 * * * *' (every 5 min), '0 * * * *' (hourly), '0 9 * * *' (daily 9am), '0 9,21 * * *' (9am & 9pm), '0 0 * * 1' (weekly Monday)",
					},
					"enabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether to start the task immediately (true) or create it in paused state (false)",
						"default":     true,
					},
					"is_agent": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this task should maintain persistent memory and context across executions (agent mode)",
						"default":     false,
					},
				},
				"required": []string{"name", "task_type", "schedule"},
			},
		},

		{
			Name:        "task_scheduler_list_tasks",
			Description: "List all scheduled tasks associated with this chat session. Shows task status, schedule, and last execution time.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_disabled": map[string]interface{}{
						"type":        "boolean",
						"description": "Include paused/disabled tasks in the list",
						"default":     false,
					},
				},
			},
		},

		{
			Name:        "task_scheduler_get_task",
			Description: "Get detailed information about a specific task, including recent execution history",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to retrieve",
					},
				},
				"required": []string{"task_id"},
			},
		},

		{
			Name:        "task_scheduler_pause_task",
			Description: "Pause a scheduled task. The task will stop running but remain configured for later resumption.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to pause",
					},
				},
				"required": []string{"task_id"},
			},
		},

		{
			Name:        "task_scheduler_resume_task",
			Description: "Resume a paused task. It will start running on its configured schedule.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to resume",
					},
				},
				"required": []string{"task_id"},
			},
		},

		{
			Name:        "task_scheduler_delete_task",
			Description: "Permanently delete a scheduled task. This cannot be undone.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to delete",
					},
				},
				"required": []string{"task_id"},
			},
		},

		{
			Name:        "task_scheduler_update_schedule",
			Description: "Change the schedule of an existing task",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to update",
					},
					"schedule": map[string]interface{}{
						"type":        "string",
						"description": "New cron schedule expression",
					},
				},
				"required": []string{"task_id", "schedule"},
			},
		},

		{
			Name:        "task_scheduler_run_now",
			Description: "Immediately execute a task, outside its normal schedule",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"task_id": map[string]interface{}{
						"type":        "string",
						"description": "ID of the task to run now",
					},
				},
				"required": []string{"task_id"},
			},
		},
	}
}

// ExecuteSystemTool executes a system tool and returns the result
func (stm *SystemToolsManager) ExecuteSystemTool(ctx context.Context, toolName string, arguments map[string]interface{}) (interface{}, error) {
	switch toolName {
	// Task Scheduler tools
	case "task_scheduler_status":
		return stm.taskSchedulerStatus(ctx)
	case "task_scheduler_start":
		return stm.taskSchedulerStart(ctx)
	case "task_scheduler_stop":
		return stm.taskSchedulerStop(ctx)

	// Server Management tools
	case "server_list":
		return stm.serverList(ctx)
	case "server_start":
		serverName, _ := arguments["server_name"].(string)
		return stm.serverStart(ctx, serverName)
	case "server_stop":
		serverName, _ := arguments["server_name"].(string)
		return stm.serverStop(ctx, serverName)
	case "server_restart":
		serverName, _ := arguments["server_name"].(string)
		return stm.serverRestart(ctx, serverName)
	case "server_logs":
		serverName, _ := arguments["server_name"].(string)
		lines := 50
		if l, ok := arguments["lines"].(float64); ok {
			lines = int(l)
		}
		return stm.serverLogs(ctx, serverName, lines)

	case "task_scheduler_create_task":
		return stm.taskSchedulerCreateTask(ctx, arguments)
	case "task_scheduler_list_tasks":
		return stm.taskSchedulerListTasks(ctx, arguments)
	case "task_scheduler_get_task":
		return stm.taskSchedulerGetTask(ctx, arguments)
	case "task_scheduler_pause_task":
		return stm.taskSchedulerPauseTask(ctx, arguments)
	case "task_scheduler_resume_task":
		return stm.taskSchedulerResumeTask(ctx, arguments)
	case "task_scheduler_delete_task":
		return stm.taskSchedulerDeleteTask(ctx, arguments)
	case "task_scheduler_update_schedule":
		return stm.taskSchedulerUpdateSchedule(ctx, arguments)
	case "task_scheduler_run_now":
		return stm.taskSchedulerRunNow(ctx, arguments)

	default:
		return nil, fmt.Errorf("unknown system tool: %s", toolName)
	}
}

// Task Scheduler implementations

func (stm *SystemToolsManager) taskSchedulerStatus(ctx context.Context) (interface{}, error) {
	if stm.config == nil || !stm.config.TaskScheduler.Enabled {
		return map[string]interface{}{
			"enabled": false,
			"status":  "disabled",
			"message": "Task scheduler is not enabled in configuration",
		}, nil
	}

	port := stm.config.TaskScheduler.Port
	if port == 0 {
		port = 8018
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
		"params":  map[string]interface{}{},
	}

	requestBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return map[string]interface{}{
			"enabled": true,
			"status":  "error",
			"message": fmt.Sprintf("Failed to create request: %v", err),
		}, nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return map[string]interface{}{
			"enabled": true,
			"status":  "error",
			"message": fmt.Sprintf("Failed to create connection request: %v", err),
		}, nil
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"enabled": true,
			"status":  "unreachable",
			"message": fmt.Sprintf("Task scheduler is not reachable at %s: %v", url, err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return map[string]interface{}{
			"enabled": true,
			"status":  "unhealthy",
			"message": fmt.Sprintf("Task scheduler returned status %d", resp.StatusCode),
		}, nil
	}

	return map[string]interface{}{
		"enabled": true,
		"status":  "running",
		"message": "Task scheduler is running and healthy",
		"port":    port,
		"url":     url,
	}, nil
}

func (stm *SystemToolsManager) taskSchedulerStart(ctx context.Context) (interface{}, error) {
	if stm.taskScheduler == nil {
		return nil, fmt.Errorf("task scheduler is not enabled in configuration")
	}

	if err := stm.taskScheduler.Start(); err != nil {
		return nil, fmt.Errorf("failed to start task scheduler: %w", err)
	}

	return map[string]interface{}{
		"status":  "started",
		"message": "Task scheduler started successfully",
	}, nil
}

func (stm *SystemToolsManager) taskSchedulerStop(ctx context.Context) (interface{}, error) {
	if stm.taskScheduler == nil {
		return nil, fmt.Errorf("task scheduler is not enabled in configuration")
	}

	if err := stm.taskScheduler.Stop(); err != nil {
		return nil, fmt.Errorf("failed to stop task scheduler: %w", err)
	}

	return map[string]interface{}{
		"status":  "stopped",
		"message": "Task scheduler stopped successfully",
	}, nil
}

// Memory Management implementations

func (stm *SystemToolsManager) memorySearch(ctx context.Context, query string, searchType string) (interface{}, error) {
	if stm.memoryManager == nil {
		return nil, fmt.Errorf("memory manager is not available")
	}

	var results []memory.SearchResult
	var err error

	if searchType == "semantic" {
		results, err = stm.memoryManager.SearchSimilar(ctx, query)
	} else {
		results, err = stm.memoryManager.HybridSearch(ctx, query)
	}

	if err != nil {
		return nil, fmt.Errorf("memory search failed: %w", err)
	}

	return map[string]interface{}{
		"query":       query,
		"search_type": searchType,
		"results":     results,
		"count":       len(results),
	}, nil
}

func (stm *SystemToolsManager) memoryStoreEntity(ctx context.Context, name, entityType, description string, importance float64) (interface{}, error) {
	if stm.memoryManager == nil {
		return nil, fmt.Errorf("memory manager is not available")
	}

	metadata := map[string]interface{}{
		"stored_at": time.Now().Format(time.RFC3339),
		"source":    "chat_interface",
	}

	entityID, err := stm.memoryManager.StoreEntity(ctx, name, entityType, description, metadata, importance)
	if err != nil {
		return nil, fmt.Errorf("failed to store entity: %w", err)
	}

	return map[string]interface{}{
		"entity_id":   entityID,
		"name":        name,
		"entity_type": entityType,
		"importance":  importance,
		"message":     "Entity stored successfully",
	}, nil
}

func (stm *SystemToolsManager) memoryStats(ctx context.Context) (interface{}, error) {
	if stm.memoryManager == nil {
		return nil, fmt.Errorf("memory manager is not available")
	}

	stats, err := stm.memoryManager.GetMemoryStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %w", err)
	}

	return stats, nil
}

func (stm *SystemToolsManager) memoryPrune(ctx context.Context, retentionDays int) (interface{}, error) {
	if stm.memoryManager == nil {
		return nil, fmt.Errorf("memory manager is not available")
	}

	result, err := stm.memoryManager.RunPruning(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to prune memory: %w", err)
	}

	totalRemoved := result.EntitiesPruned + result.RelationsPruned + result.ObservationsPruned

	return map[string]interface{}{
		"retention_days": retentionDays,
		"result":         result,
		"message":        fmt.Sprintf("Memory pruning completed, removed %d entries", totalRemoved),
	}, nil
}

// Server Management implementations

func (stm *SystemToolsManager) serverList(ctx context.Context) (interface{}, error) {
	if stm.config == nil || stm.config.Servers == nil {
		return map[string]interface{}{
			"servers": []interface{}{},
			"count":   0,
		}, nil
	}

	servers := make([]map[string]interface{}, 0)

	for serverName, serverConfig := range stm.config.Servers {
		status := "unknown"
		if stm.serverManager != nil {
			if s, err := stm.serverManager.GetServerStatus(serverName); err == nil {
				status = s
			}
		}

		serverInfo := map[string]interface{}{
			"name":     serverName,
			"status":   status,
			"protocol": serverConfig.Protocol,
		}

		if serverConfig.Image != "" {
			serverInfo["image"] = serverConfig.Image
		}
		if serverConfig.Command != "" {
			serverInfo["command"] = serverConfig.Command
		}

		servers = append(servers, serverInfo)
	}

	if stm.config.TaskScheduler.Enabled {
		servers = append(servers, map[string]interface{}{
			"name":     "task-scheduler",
			"status":   "running",
			"protocol": "sse",
		})
	}

	if stm.config.Memory.Enabled {
		servers = append(servers, map[string]interface{}{
			"name":     "memory",
			"status":   "running",
			"protocol": "sse",
		})
	}

	return map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	}, nil
}

func (stm *SystemToolsManager) serverStart(ctx context.Context, serverName string) (interface{}, error) {
	if stm.serverManager == nil {
		return nil, fmt.Errorf("server manager is not available")
	}

	if serverName == "" {
		return nil, fmt.Errorf("server_name is required")
	}

	if err := stm.serverManager.StartServer(serverName); err != nil {
		return nil, fmt.Errorf("failed to start server %s: %w", serverName, err)
	}

	return map[string]interface{}{
		"server_name": serverName,
		"status":      "started",
		"message":     fmt.Sprintf("Server %s started successfully", serverName),
	}, nil
}

func (stm *SystemToolsManager) serverStop(ctx context.Context, serverName string) (interface{}, error) {
	if stm.serverManager == nil {
		return nil, fmt.Errorf("server manager is not available")
	}

	if serverName == "" {
		return nil, fmt.Errorf("server_name is required")
	}

	if err := stm.serverManager.StopServer(serverName); err != nil {
		return nil, fmt.Errorf("failed to stop server %s: %w", serverName, err)
	}

	return map[string]interface{}{
		"server_name": serverName,
		"status":      "stopped",
		"message":     fmt.Sprintf("Server %s stopped successfully", serverName),
	}, nil
}

func (stm *SystemToolsManager) serverRestart(ctx context.Context, serverName string) (interface{}, error) {
	if stm.serverManager == nil {
		return nil, fmt.Errorf("server manager is not available")
	}

	if serverName == "" {
		return nil, fmt.Errorf("server_name is required")
	}

	if err := stm.serverManager.StopServer(serverName); err != nil {
		return nil, fmt.Errorf("failed to stop server %s: %w", serverName, err)
	}

	time.Sleep(2 * time.Second)

	if err := stm.serverManager.StartServer(serverName); err != nil {
		return nil, fmt.Errorf("failed to start server %s: %w", serverName, err)
	}

	return map[string]interface{}{
		"server_name": serverName,
		"status":      "restarted",
		"message":     fmt.Sprintf("Server %s restarted successfully", serverName),
	}, nil
}

func (stm *SystemToolsManager) serverLogs(ctx context.Context, serverName string, lines int) (interface{}, error) {
	if stm.serverManager == nil {
		return nil, fmt.Errorf("server manager is not available")
	}

	if serverName == "" {
		return nil, fmt.Errorf("server_name is required")
	}

	// Check if server exists
	_, exists := stm.serverManager.GetServerInstance(serverName)
	if !exists {
		return nil, fmt.Errorf("server %s not found", serverName)
	}

	// Note: Log streaming is handled by the dashboard's dedicated logs view
	logs := fmt.Sprintf("Logs for %s are available in the dashboard logs view", serverName)

	return map[string]interface{}{
		"server_name": serverName,
		"lines":       lines,
		"logs":        logs,
		"message":     "Use the dashboard's logs view for detailed real-time log streaming",
	}, nil
}

// FormatToolResult formats a tool execution result for display
func FormatToolResult(toolName string, result interface{}) string {
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Sprintf("Tool %s result: %v", toolName, result)
	}

	return fmt.Sprintf("Tool %s result:\n```json\n%s\n```", toolName, string(resultJSON))
}

// IsSystemTool checks if a tool name is a system tool
func IsSystemTool(toolName string) bool {
	systemTools := []string{
		"task_scheduler_status",
		"task_scheduler_start",
		"task_scheduler_stop",
		"task_scheduler_create_task",
		"task_scheduler_list_tasks",
		"task_scheduler_get_task",
		"task_scheduler_pause_task",
		"task_scheduler_resume_task",
		"task_scheduler_delete_task",
		"task_scheduler_update_schedule",
		"task_scheduler_run_now",
		"server_list",
		"server_start",
		"server_stop",
		"server_restart",
		"server_logs",
	}

	for _, tool := range systemTools {
		if strings.EqualFold(tool, toolName) {
			return true
		}
	}

	return false
}

func (stm *SystemToolsManager) taskSchedulerCreateTask(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	sessionID, ok := ctx.Value("session_id").(string)
	if !ok {
		return nil, fmt.Errorf("session ID not found in context")
	}

	name := getStringArg(arguments, "name")
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	taskType := getStringArg(arguments, "task_type")
	if taskType == "" {
		return nil, fmt.Errorf("task_type is required")
	}

	schedule := getStringArg(arguments, "schedule")
	if schedule == "" {
		return nil, fmt.Errorf("schedule is required")
	}

	description := getStringArg(arguments, "description")
	enabled := getBoolArg(arguments, "enabled", true)
	isAgent := getBoolArg(arguments, "is_agent", false)

	var prompt, command string
	if taskType == "ai" {
		prompt = getStringArg(arguments, "prompt")
		if prompt == "" {
			return nil, fmt.Errorf("prompt required for AI tasks")
		}
	} else if taskType == "shell" {
		command = getStringArg(arguments, "command")
		if command == "" {
			return nil, fmt.Errorf("command required for shell tasks")
		}
	} else {
		return nil, fmt.Errorf("task_type must be 'ai' or 'shell'")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	toolName := "add_task"
	if taskType == "ai" {
		toolName = "add_ai_task"
	}

	taskArgs := map[string]interface{}{
		"name":              name,
		"description":       description,
		"type":              taskType,
		"prompt":            prompt,
		"command":           command,
		"schedule":          schedule,
		"enabled":           enabled,
		"is_agent":          isAgent,
		"_chat_session_id":  sessionID,
		"_output_to_chat":   true,
	}

	if provider, ok := arguments["_provider"].(string); ok && provider != "" {
		taskArgs["_provider"] = provider
	}
	if model, ok := arguments["_model"].(string); ok && model != "" {
		taskArgs["_model"] = model
	}
	if mcpServers, ok := arguments["_mcp_servers"]; ok {
		taskArgs["_mcp_servers"] = mcpServers
	}

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": taskArgs,
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerListTasks(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	sessionID, ok := ctx.Value("session_id").(string)
	if !ok {
		return nil, fmt.Errorf("session ID not found in context")
	}

	includeDisabled := getBoolArg(arguments, "include_disabled", false)

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "list_tasks",
			"arguments": map[string]interface{}{
				"session_id":       sessionID,
				"include_disabled": includeDisabled,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerGetTask(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "get_task",
			"arguments": map[string]interface{}{
				"task_id": taskID,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerPauseTask(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "disable_task",
			"arguments": map[string]interface{}{
				"id": taskID,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerResumeTask(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "enable_task",
			"arguments": map[string]interface{}{
				"id": taskID,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerDeleteTask(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

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

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerUpdateSchedule(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	schedule := getStringArg(arguments, "schedule")
	if schedule == "" {
		return nil, fmt.Errorf("schedule is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "update_task",
			"arguments": map[string]interface{}{
				"id":       taskID,
				"schedule": schedule,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) taskSchedulerRunNow(ctx context.Context, arguments map[string]interface{}) (interface{}, error) {
	taskID := getStringArg(arguments, "task_id")
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/task-scheduler", proxyURL)

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "run_task",
			"arguments": map[string]interface{}{
				"id": taskID,
			},
		},
	}

	return stm.callTaskScheduler(ctx, url, mcpRequest)
}

func (stm *SystemToolsManager) callTaskScheduler(ctx context.Context, url string, mcpRequest map[string]interface{}) (interface{}, error) {
	requestBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(requestBody)))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call task scheduler: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("task scheduler returned status %d", resp.StatusCode)
	}

	var result struct {
		Result interface{} `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("task scheduler error: %s", result.Error.Message)
	}

	return result.Result, nil
}

func getStringArg(args map[string]interface{}, key string) string {
	if val, ok := args[key].(string); ok {
		return val
	}

	return ""
}

func getBoolArg(args map[string]interface{}, key string, defaultVal bool) bool {
	if val, ok := args[key].(bool); ok {
		return val
	}

	return defaultVal
}