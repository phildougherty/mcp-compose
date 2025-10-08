package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/ai"
)

type NodeExecutor struct {
	aiManager     *ai.Manager
	mcpProxyURL   string
	mcpAPIKey     string
	jsVM          *JavaScriptVM
	codeTimeout   time.Duration
}

type NodeConfig struct {
	Prompt      string                 `json:"prompt,omitempty"`
	Model       string                 `json:"model,omitempty"`
	Provider    string                 `json:"provider,omitempty"`
	Server      string                 `json:"server,omitempty"`
	Tool        string                 `json:"tool,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	Condition   string                 `json:"condition,omitempty"`
	Code        string                 `json:"code,omitempty"`
	Language    string                 `json:"language,omitempty"`
	ErrorMode   string                 `json:"errorMode,omitempty"`
	Default     interface{}            `json:"default,omitempty"`
	Streaming   bool                   `json:"streaming,omitempty"`
}

type NodeData struct {
	Config NodeConfig             `json:"config"`
	Label  string                 `json:"label,omitempty"`
	Custom map[string]interface{} `json:"custom,omitempty"`
}

func NewNodeExecutor(aiManager *ai.Manager, mcpProxyURL, mcpAPIKey string) *NodeExecutor {
	return &NodeExecutor{
		aiManager:   aiManager,
		mcpProxyURL: mcpProxyURL,
		mcpAPIKey:   mcpAPIKey,
		jsVM:        NewJavaScriptVM(30 * time.Second),
		codeTimeout: 60 * time.Second,
	}
}

func (ne *NodeExecutor) ExecuteNode(ctx context.Context, node *WorkflowNode, execCtx *ExecutionContext) (map[string]interface{}, error) {
	var nodeData NodeData
	if err := json.Unmarshal(node.Data, &nodeData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node data: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(node.Data, &rawData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal raw node data: %w", err)
	}
	nodeData.Custom = rawData

	switch node.Type {
	case NodeTypeTrigger:
		return ne.executeTrigger(ctx, nodeData, execCtx)
	case NodeTypeAITask:
		return ne.executeAITask(ctx, nodeData, execCtx)
	case NodeTypeMCPServer:
		return ne.executeMCPServer(ctx, nodeData, execCtx)
	case NodeTypeDecision:
		return ne.executeDecision(ctx, nodeData, execCtx)
	case NodeTypeTransform:
		return ne.executeTransform(ctx, nodeData, execCtx)
	case NodeTypeCode:
		return ne.executeCode(ctx, nodeData, execCtx)
	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

func (ne *NodeExecutor) executeTrigger(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	return map[string]interface{}{
		"triggered": true,
		"timestamp": time.Now().Format(time.RFC3339),
		"data":      execCtx.Input,
	}, nil
}

func (ne *NodeExecutor) executeAITask(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	prompt, err := execCtx.ResolveTemplateString(data.Config.Prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve prompt template: %w", err)
	}

	contextStr := ne.buildNodeContext(execCtx)

	fullPrompt := prompt
	if contextStr != "" {
		fullPrompt = "Context from previous workflow steps:\n" + contextStr + "\n\nTask: " + prompt
	}

	messages := []ai.Message{
		{
			Role:    "user",
			Content: fullPrompt,
		},
	}

	if data.Config.Provider != "" && ne.aiManager != nil {
		providerName := ne.normalizeProviderName(data.Config.Provider)

		provider, err := ne.getProviderWithModel(providerName, data.Config.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI provider %s: %w", data.Config.Provider, err)
		}

		if data.Config.Streaming {
			ch, err := provider.Stream(ctx, messages)
			if err != nil {
				return nil, fmt.Errorf("failed to start AI streaming: %w", err)
			}

			var fullResponse strings.Builder
			for chunk := range ch {
				fullResponse.WriteString(chunk)
			}

			return map[string]interface{}{
				"response": fullResponse.String(),
				"model":    data.Config.Model,
				"provider": data.Config.Provider,
			}, nil
		}

		response, err := provider.Chat(ctx, messages)
		if err != nil {
			return nil, fmt.Errorf("failed to get AI response: %w", err)
		}

		return map[string]interface{}{
			"response": response,
			"model":    data.Config.Model,
			"provider": data.Config.Provider,
		}, nil
	}

	if ne.aiManager == nil {
		return nil, fmt.Errorf("AI manager not configured")
	}

	response, err := ne.aiManager.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to get AI response: %w", err)
	}

	return map[string]interface{}{
		"response": response,
	}, nil
}

func (ne *NodeExecutor) executeMCPServer(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	server := data.Config.Server
	tool := data.Config.Tool

	if server == "" {
		if serverName, ok := data.Custom["server_name"].(string); ok {
			server = serverName
		}
	}
	if tool == "" {
		if toolName, ok := data.Custom["tool_name"].(string); ok {
			tool = toolName
		}
	}

	if server == "" || tool == "" {
		return nil, fmt.Errorf("server and tool are required for MCP server node")
	}

	data.Config.Server = server
	data.Config.Tool = tool

	parameters, err := execCtx.ResolveTemplateMap(data.Config.Parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve parameters: %w", err)
	}

	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      data.Config.Tool,
			"arguments": parameters,
		},
	}

	requestBody, err := json.Marshal(mcpRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal MCP request: %w", err)
	}

	url := fmt.Sprintf("%s/%s", ne.mcpProxyURL, data.Config.Server)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if ne.mcpAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+ne.mcpAPIKey)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP server: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("MCP server returned status %d: %s", resp.StatusCode, string(body))
	}

	var mcpResponse map[string]interface{}
	if err := json.Unmarshal(body, &mcpResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal MCP response (body: %s): %w", string(body), err)
	}

	if mcpError, hasError := mcpResponse["error"].(map[string]interface{}); hasError {
		errorMsg := "MCP error"
		if msg, ok := mcpError["message"].(string); ok {
			errorMsg = msg
		}

		return nil, fmt.Errorf("%s", errorMsg)
	}

	if result, exists := mcpResponse["result"]; exists {
		if resultMap, ok := result.(map[string]interface{}); ok {
			return resultMap, nil
		}

		return map[string]interface{}{"result": result}, nil
	}

	return map[string]interface{}{"raw": mcpResponse}, nil
}

func (ne *NodeExecutor) executeDecision(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	conditionStr := data.Config.Condition
	if conditionStr == "" {
		if conditionAlt, ok := data.Custom["condition"].(string); ok {
			conditionStr = conditionAlt
		}
	}

	if conditionStr == "" {
		return nil, fmt.Errorf("condition is required for decision node")
	}

	condition, err := execCtx.ResolveTemplateString(conditionStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve condition template: %w", err)
	}

	inputData := make(map[string]interface{})
	inputData["input"] = execCtx.Input
	inputData["context"] = execCtx.Context
	inputData["nodes"] = execCtx.NodeOutputs

	result, err := ne.jsVM.ExecuteCondition(ctx, condition, inputData)
	if err != nil {
		return nil, fmt.Errorf("failed to execute condition: %w", err)
	}

	return map[string]interface{}{
		"decision": result,
		"path":     map[bool]string{true: "true", false: "false"}[result],
	}, nil
}

func (ne *NodeExecutor) executeTransform(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	codeStr := data.Config.Code
	if codeStr == "" {
		if codeAlt, ok := data.Custom["transform"].(string); ok {
			codeStr = codeAlt
		}
	}

	if codeStr == "" {
		return nil, fmt.Errorf("code is required for transform node")
	}

	code, err := execCtx.ResolveTemplateString(codeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve code template: %w", err)
	}

	inputData := make(map[string]interface{})
	inputData["input"] = execCtx.Input
	inputData["context"] = execCtx.Context
	inputData["nodes"] = execCtx.NodeOutputs

	result, err := ne.jsVM.ExecuteTransform(ctx, code, inputData)
	if err != nil {
		errorMode := data.Config.ErrorMode
		if errorMode == "" {
			errorMode = "fail"
		}

		switch errorMode {
		case "default":
			if data.Config.Default != nil {
				if defaultMap, ok := data.Config.Default.(map[string]interface{}); ok {
					return defaultMap, nil
				}

				return map[string]interface{}{"result": data.Config.Default}, nil
			}

			return map[string]interface{}{"error": err.Error()}, nil
		case "passthrough":
			return inputData, nil
		default:
			return nil, fmt.Errorf("failed to execute transform: %w", err)
		}
	}

	return result, nil
}

func (ne *NodeExecutor) executeCode(ctx context.Context, data NodeData, execCtx *ExecutionContext) (map[string]interface{}, error) {
	codeStr := data.Config.Code
	if codeStr == "" {
		if codeAlt, ok := data.Custom["code"].(string); ok {
			codeStr = codeAlt
		}
	}

	if codeStr == "" {
		return nil, fmt.Errorf("code is required for code node")
	}

	code, err := execCtx.ResolveTemplateString(codeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve code template: %w", err)
	}

	language := data.Config.Language
	if language == "" {
		language = "javascript"
	}

	switch strings.ToLower(language) {
	case "javascript", "js":
		return ne.executeJavaScriptCode(ctx, code, execCtx)
	case "python", "py":
		return ne.executePythonCode(ctx, code, execCtx)
	case "bash", "shell", "sh":
		return ne.executeShellCode(ctx, code, execCtx)
	case "go":
		return ne.executeGoCode(ctx, code, execCtx)
	case "ruby", "rb":
		return ne.executeRubyCode(ctx, code, execCtx)
	case "php":
		return ne.executePHPCode(ctx, code, execCtx)
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

func (ne *NodeExecutor) executeJavaScriptCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	inputData := make(map[string]interface{})
	inputData["input"] = execCtx.Input
	inputData["context"] = execCtx.Context
	inputData["nodes"] = execCtx.NodeOutputs

	return ne.jsVM.ExecuteCode(ctx, code, inputData)
}

func (ne *NodeExecutor) executePythonCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	inputJSON, err := json.Marshal(execCtx.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	wrappedCode := fmt.Sprintf(`
import json
import sys

input_data = json.loads('%s')

%s
`, string(inputJSON), code)

	return ne.executeSubprocess(ctx, "python3", wrappedCode)
}

func (ne *NodeExecutor) executeShellCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	return ne.executeSubprocess(ctx, "bash", code)
}

func (ne *NodeExecutor) executeGoCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	return nil, fmt.Errorf("Go code execution not yet implemented")
}

func (ne *NodeExecutor) executeRubyCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	return ne.executeSubprocess(ctx, "ruby", code)
}

func (ne *NodeExecutor) executePHPCode(ctx context.Context, code string, execCtx *ExecutionContext) (map[string]interface{}, error) {
	return ne.executeSubprocess(ctx, "php", code)
}

func (ne *NodeExecutor) executeSubprocess(ctx context.Context, command string, code string) (map[string]interface{}, error) {
	execCtx, cancel := context.WithTimeout(ctx, ne.codeTimeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, command, "-c", code)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		result["error"] = err.Error()
		result["exit_code"] = cmd.ProcessState.ExitCode()

		return result, fmt.Errorf("code execution failed: %w", err)
	}

	result["exit_code"] = 0

	return result, nil
}

func (ne *NodeExecutor) getProviderWithModel(providerName, model string) (ai.Provider, error) {
	if model == "" {
		return ne.aiManager.GetProvider(providerName)
	}

	switch providerName {
	case "ollama":
		return ai.NewOllamaProvider(&ai.OllamaConfig{
			BaseURL: ne.getOllamaBaseURL(),
			Model:   model,
		})
	case "openrouter":
		return ai.NewOpenRouterProvider(&ai.OpenRouterConfig{
			APIKey: ne.getOpenRouterAPIKey(),
			Model:  model,
		})
	case "openai":
		return ai.NewOpenAIProvider(&ai.OpenAIConfig{
			APIKey: ne.getOpenAIAPIKey(),
			Model:  model,
		})
	case "claude":
		return ai.NewClaudeProvider(&ai.ClaudeConfig{
			APIKey: ne.getClaudeAPIKey(),
			Model:  model,
		})
	default:
		return ne.aiManager.GetProvider(providerName)
	}
}

func (ne *NodeExecutor) getOllamaBaseURL() string {
	if url := os.Getenv("OLLAMA_BASE_URL"); url != "" {
		return url
	}

	return "http://localhost:11434"
}

func (ne *NodeExecutor) getOpenRouterAPIKey() string {
	return os.Getenv("OPENROUTER_API_KEY")
}

func (ne *NodeExecutor) getOpenAIAPIKey() string {
	return os.Getenv("OPENAI_API_KEY")
}

func (ne *NodeExecutor) getClaudeAPIKey() string {
	return os.Getenv("ANTHROPIC_API_KEY")
}

func (ne *NodeExecutor) normalizeProviderName(provider string) string {
	switch provider {
	case "local":
		return "ollama"
	case "anthropic":
		return "claude"
	default:
		return provider
	}
}

func (ne *NodeExecutor) buildNodeContext(execCtx *ExecutionContext) string {
	if len(execCtx.NodeOutputs) == 0 {
		return ""
	}

	var contextParts []string
	for nodeID, output := range execCtx.NodeOutputs {
		outputJSON, err := json.Marshal(output)
		if err != nil {
			continue
		}

		contextParts = append(contextParts, fmt.Sprintf("Output from node '%s': %s", nodeID, string(outputJSON)))
	}

	return strings.Join(contextParts, "\n")
}
