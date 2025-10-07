package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	Result   string                 `json:"result,omitempty"`
	Error    string                 `json:"error,omitempty"`
	Duration time.Duration          `json:"duration,omitempty"`
}

type ChatMessage struct {
	ID            string     `json:"id"`
	SessionID     string     `json:"session_id"`
	Role          string     `json:"role"`
	Content       string     `json:"content"`
	ToolCalls     []ToolCall `json:"tool_calls,omitempty"`
	ToolResults   []ToolCall `json:"tool_results,omitempty"`
	TokensUsed    int        `json:"tokens_used,omitempty"`
	CostEstimate  float64    `json:"cost_estimate,omitempty"`
	IsAutomated   bool       `json:"is_automated,omitempty"`
	FromTaskRunID string     `json:"from_task_run_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ChatService struct {
	aiManager       *ai.Manager
	Storage         *ChatStorage
	systemTools     *SystemToolsManager
	logger          *logging.Logger
	chatBroadcaster *ChatBroadcaster
	sessions        map[string]*ChatSession
	mu              sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	proxyURL        string
	apiKey          string
	httpClient      *http.Client
}

func NewChatService(aiManager *ai.Manager, storage *ChatStorage, systemTools *SystemToolsManager, logger *logging.Logger, chatBroadcaster *ChatBroadcaster) *ChatService {
	ctx, cancel := context.WithCancel(context.Background())

	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	apiKey := os.Getenv("MCP_API_KEY")

	logger.Info("ChatService initialized with proxy URL: %s", proxyURL)

	httpClient := &http.Client{Timeout: 10 * time.Second}

	testCtx, testCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer testCancel()

	testReq, testReqErr := http.NewRequestWithContext(testCtx, "GET", proxyURL+"/api/status", nil)
	if testReqErr != nil {
		logger.Error("Failed to create test request for MCP proxy: %v", testReqErr)
	} else {
		if apiKey != "" {
			testReq.Header.Set("Authorization", "Bearer "+apiKey)
		}

		testResp, testErr := httpClient.Do(testReq)
		if testErr != nil {
			logger.Error("WARNING: Cannot reach MCP proxy at %s: %v", proxyURL, testErr)
		} else {
			testResp.Body.Close()
			logger.Info("MCP proxy is reachable at %s (status: %d)", proxyURL, testResp.StatusCode)
		}
	}

	return &ChatService{
		aiManager:       aiManager,
		Storage:         storage,
		systemTools:     systemTools,
		logger:          logger,
		chatBroadcaster: chatBroadcaster,
		sessions:        make(map[string]*ChatSession),
		ctx:             ctx,
		cancel:          cancel,
		proxyURL:        proxyURL,
		apiKey:          apiKey,
		httpClient:      httpClient,
	}
}

func (cs *ChatService) broadcastMessage(sessionID string, message *ChatMessage) {
	if cs.chatBroadcaster == nil {
		return
	}

	cs.chatBroadcaster.BroadcastToSession(sessionID, "new_message", message)
}

func (cs *ChatService) CreateSession(userID, provider, model string) (*ChatSession, error) {
	var session *ChatSession

	if cs.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		dbSession, err := cs.Storage.CreateSession(ctx, userID, provider, model)
		if err != nil {
			return nil, fmt.Errorf("failed to create session in storage: %w", err)
		}
		session = dbSession
		session.Messages = []ChatMessage{}

		cs.logger.Info("Created chat session in database: session_id=%s, user_id=%s, provider=%s, model=%s", session.ID, userID, provider, model)
	} else {
		session = &ChatSession{
			ID:        uuid.New().String(),
			UserID:    userID,
			Provider:  provider,
			Model:     model,
			CreatedAt: time.Now(),
			LastUsed:  time.Now(),
			Title:     "New Chat",
			Metadata:  make(map[string]interface{}),
			Messages:  []ChatMessage{},
		}

		cs.logger.Info("Created in-memory chat session: session_id=%s (storage not available)", session.ID)
	}

	defaultMCPServers := cs.getDefaultMCPServers()
	session.MCPServers = defaultMCPServers

	if cs.Storage != nil && len(defaultMCPServers) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := cs.Storage.SetSessionMCPServers(ctx, session.ID, defaultMCPServers); err != nil {
			cs.logger.Warning("Failed to set default MCP servers in storage: %v", err)
		}
	}

	cs.mu.Lock()
	cs.sessions[session.ID] = session
	cs.mu.Unlock()

	return session, nil
}

func (cs *ChatService) GetSession(sessionID string) (*ChatSession, error) {
	if cs.Storage != nil {
		ctx := context.Background()
		loadedSession, err := cs.Storage.GetSession(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("session not found: %w", err)
		}

		messages, err := cs.Storage.GetMessages(ctx, sessionID, 100)
		if err != nil {
			cs.logger.Warning("Failed to load messages for session %s: %v", sessionID, err)
			messages = []*ChatMessage{}
		}

		sessionMessages := make([]ChatMessage, len(messages))
		for i, msg := range messages {
			sessionMessages[i] = *msg
		}

		session := &ChatSession{
			ID:         loadedSession.ID,
			UserID:     loadedSession.UserID,
			Provider:   loadedSession.Provider,
			Model:      loadedSession.Model,
			CreatedAt:  loadedSession.CreatedAt,
			LastUsed:   loadedSession.LastUsed,
			Title:      loadedSession.Title,
			Metadata:   loadedSession.Metadata,
			Messages:   sessionMessages,
			MCPServers: loadedSession.MCPServers,
		}

		cs.mu.Lock()
		cs.sessions[sessionID] = session
		cs.mu.Unlock()

		return session, nil
	}

	cs.mu.RLock()
	session, exists := cs.sessions[sessionID]
	cs.mu.RUnlock()

	if exists {
		return session, nil
	}

	return nil, fmt.Errorf("session not found")
}

func (cs *ChatService) ListSessions(userID string) ([]ChatSession, error) {
	if cs.Storage != nil {
		ctx := context.Background()
		storageSessions, err := cs.Storage.ListSessions(ctx, userID, 50)
		if err != nil {
			return nil, err
		}

		sessions := make([]ChatSession, len(storageSessions))
		for i, s := range storageSessions {
			sessions[i] = ChatSession{
				ID:                 s.ID,
				UserID:             s.UserID,
				Provider:           s.Provider,
				Model:              s.Model,
				CreatedAt:          s.CreatedAt,
				LastUsed:           s.LastUsed,
				Title:              s.Title,
				Metadata:           s.Metadata,
				Messages:           []ChatMessage{},
				MCPServers:         s.MCPServers,
				UnreadMessageCount: s.UnreadMessageCount,
				HasActiveAgents:    s.HasActiveAgents,
			}
		}

		return sessions, nil
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var sessions []ChatSession
	for _, session := range cs.sessions {
		if session.UserID == userID || userID == "" {
			sessions = append(sessions, *session)
		}
	}

	return sessions, nil
}

func (cs *ChatService) DeleteSession(sessionID string) error {
	cs.mu.Lock()
	delete(cs.sessions, sessionID)
	cs.mu.Unlock()

	if cs.Storage != nil {
		ctx := context.Background()

		return cs.Storage.DeleteSession(ctx, sessionID)
	}

	return nil
}

func (cs *ChatService) UpdateSession(sessionID string, updates map[string]interface{}) error {
	session, err := cs.GetSession(sessionID)
	if err != nil {
		if cs.Storage != nil {
			ctx := context.Background()
			_, storageErr := cs.Storage.GetSession(ctx, sessionID)
			if storageErr != nil {
				return nil
			}
		}
		return err
	}

	if title, ok := updates["title"].(string); ok {
		session.Title = title
	}
	if provider, ok := updates["provider"].(string); ok {
		session.Provider = provider
	}
	if model, ok := updates["model"].(string); ok {
		session.Model = model
	}

	session.LastUsed = time.Now()

	if cs.Storage != nil {
		ctx := context.Background()

		_, err := cs.Storage.GetSession(ctx, sessionID)
		if err != nil {
			return nil
		}

		return cs.Storage.UpdateSession(ctx, sessionID, updates)
	}

	return nil
}

func (cs *ChatService) SendMessage(sessionID, userMessage string, stream bool) (<-chan string, error) {
	session, err := cs.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	userMsg := ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "user",
		Content:   userMessage,
		CreatedAt: time.Now(),
	}

	session.Messages = append(session.Messages, userMsg)
	session.LastUsed = time.Now()

	if cs.Storage != nil {
		ctx := context.Background()
		storageMsg := &ChatMessage{
			ID:        userMsg.ID,
			SessionID: userMsg.SessionID,
			Role:      userMsg.Role,
			Content:   userMsg.Content,
			CreatedAt: userMsg.CreatedAt,
		}
		if err := cs.Storage.AddMessage(ctx, storageMsg); err != nil {
			cs.logger.Error("Failed to save user message to database: %v", err)
		}
	}

	cs.broadcastMessage(sessionID, &userMsg)

	if session.Title == "New Chat" && len(session.Messages) == 1 {
		go cs.generateSessionTitle(session)
	}

	aiMessages := cs.convertToAIMessages(session.Messages)
	aiMessages = cs.addSystemContextForSession(session.ID, aiMessages)

	tools, err := cs.buildToolDefinitions(session.ID)
	if err != nil {
		cs.logger.Error("Failed to build tool definitions: %v", err)
	}

	if stream {
		return cs.streamResponseWithTools(session, aiMessages, tools)
	}

	return cs.chatResponseWithTools(session, aiMessages, tools)
}

func (cs *ChatService) streamResponse(session *ChatSession, messages []ai.Message) (<-chan string, error) {
	outCh := make(chan string, 100)

	go func() {
		defer close(outCh)

		ctx, cancel := context.WithTimeout(cs.ctx, 5*time.Minute)
		defer cancel()

		streamCh, err := cs.aiManager.Stream(ctx, messages)
		if err != nil {
			errorMsg := fmt.Sprintf("Error: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		var fullResponse strings.Builder
		messageID := uuid.New().String()

		for {
			select {
			case chunk, ok := <-streamCh:
				if !ok {
					assistantMsg := ChatMessage{
						ID:        messageID,
						SessionID: session.ID,
						Role:      "assistant",
						Content:   fullResponse.String(),
						CreatedAt: time.Now(),
					}

					session.Messages = append(session.Messages, assistantMsg)

					toolCalls := cs.detectAndExecuteTools(session, &assistantMsg)

					if len(toolCalls) > 0 {
						assistantMsg.ToolCalls = toolCalls
						if cs.Storage != nil {
							if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
								cs.logger.Error("Failed to save assistant message with tool calls (legacy): %v", err)
							}
						}

						toolResults := cs.processToolCalls(session, toolCalls)
						assistantMsg.ToolResults = toolResults

						toolResultsMsg := cs.createToolResultsMessage(session, toolResults)
						session.Messages = append(session.Messages, toolResultsMsg)

						if cs.Storage != nil {
							if err := cs.Storage.AddMessage(ctx, &toolResultsMsg); err != nil {
								cs.logger.Error("Failed to save tool results message (legacy): %v", err)
							}
						}

						cs.broadcastMessage(session.ID, &toolResultsMsg)

						finalMessages := cs.convertToAIMessages(session.Messages)
						finalResponse, err := cs.aiManager.Chat(ctx, finalMessages)
						if err != nil {
							errorMsg := fmt.Sprintf("\n\nError synthesizing tool results: %v", err)
							select {
							case outCh <- errorMsg:
							case <-ctx.Done():
							}
							return
						}

						finalMsg := ChatMessage{
							ID:        uuid.New().String(),
							SessionID: session.ID,
							Role:      "assistant",
							Content:   finalResponse,
							CreatedAt: time.Now(),
						}

						session.Messages = append(session.Messages, finalMsg)

						if cs.Storage != nil {
							if err := cs.Storage.AddMessage(ctx, &finalMsg); err != nil {
								cs.logger.Error("Failed to save final assistant message (legacy): %v", err)
							}
						}

						select {
						case outCh <- "\n\n" + finalResponse:
						case <-ctx.Done():
						}
					} else {
						if cs.Storage != nil {
							if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
								cs.logger.Error("Failed to save assistant message (legacy, no tools): %v", err)
							}
						}
					}

					return
				}

				fullResponse.WriteString(chunk)

				select {
				case outCh <- chunk:
				case <-ctx.Done():
					return
				}

			case <-ctx.Done():
				return
			}
		}
	}()

	return outCh, nil
}

func (cs *ChatService) chatResponse(session *ChatSession, messages []ai.Message) (<-chan string, error) {
	outCh := make(chan string, 1)

	go func() {
		defer close(outCh)

		ctx, cancel := context.WithTimeout(cs.ctx, 5*time.Minute)
		defer cancel()

		response, err := cs.aiManager.Chat(ctx, messages)
		if err != nil {
			errorMsg := fmt.Sprintf("Error: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		messageID := uuid.New().String()
		assistantMsg := ChatMessage{
			ID:        messageID,
			SessionID: session.ID,
			Role:      "assistant",
			Content:   response,
			CreatedAt: time.Now(),
		}

		session.Messages = append(session.Messages, assistantMsg)

		toolCalls := cs.detectAndExecuteTools(session, &assistantMsg)

		if len(toolCalls) > 0 {
			assistantMsg.ToolCalls = toolCalls
			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save assistant message with tool calls (legacy non-stream): %v", err)
				}
			}

			toolResults := cs.processToolCalls(session, toolCalls)
			assistantMsg.ToolResults = toolResults

			toolResultsMsg := cs.createToolResultsMessage(session, toolResults)
			session.Messages = append(session.Messages, toolResultsMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &toolResultsMsg); err != nil {
					cs.logger.Error("Failed to save tool results message (legacy non-stream): %v", err)
				}
			}

			cs.broadcastMessage(session.ID, &toolResultsMsg)

			finalMessages := cs.convertToAIMessages(session.Messages)
			finalResponse, err := cs.aiManager.Chat(ctx, finalMessages)
			if err != nil {
				errorMsg := fmt.Sprintf("Error synthesizing tool results: %v", err)
				select {
				case outCh <- errorMsg:
				case <-ctx.Done():
				}
				return
			}

			finalMsg := ChatMessage{
				ID:        uuid.New().String(),
				SessionID: session.ID,
				Role:      "assistant",
				Content:   finalResponse,
				CreatedAt: time.Now(),
			}

			session.Messages = append(session.Messages, finalMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &finalMsg); err != nil {
					cs.logger.Error("Failed to save final assistant message (legacy non-stream): %v", err)
				}
			}

			select {
			case outCh <- response + "\n\n" + finalResponse:
			case <-ctx.Done():
			}
		} else {
			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save assistant message (legacy non-stream, no tools): %v", err)
				}
			}

			select {
			case outCh <- response:
			case <-ctx.Done():
			}
		}
	}()

	return outCh, nil
}

func (cs *ChatService) detectAndExecuteTools(session *ChatSession, message *ChatMessage) []ToolCall {
	content := message.Content

	cs.logger.Debug("detectAndExecuteTools: checking message for tool calls (length=%d)", len(content))

	jsonStart := -1
	jsonEnd := -1

	codeBlockStart := strings.Index(content, "```json")
	if codeBlockStart != -1 {
		jsonStart = codeBlockStart + 7
		jsonEnd = strings.Index(content[jsonStart:], "```")
		if jsonEnd != -1 {
			jsonEnd += jsonStart
		}
		cs.logger.Debug("Found ```json block: start=%d, end=%d", jsonStart, jsonEnd)
	}

	if jsonStart == -1 {
		rawJSONStart := strings.Index(content, "{\"tool_calls\":")
		if rawJSONStart != -1 {
			jsonStart = rawJSONStart
			braceCount := 0
			for i, ch := range content[jsonStart:] {
				if ch == '{' {
					braceCount++
				} else if ch == '}' {
					braceCount--
					if braceCount == 0 {
						jsonEnd = jsonStart + i + 1

						break
					}
				}
			}
			cs.logger.Debug("Found raw JSON: start=%d, end=%d", jsonStart, jsonEnd)
		}
	}

	if jsonStart == -1 || jsonEnd == -1 {
		cs.logger.Debug("No tool calls detected in message")

		return nil
	}

	jsonContent := strings.TrimSpace(content[jsonStart:jsonEnd])
	cs.logger.Debug("Extracted JSON content: %s", jsonContent)

	var parsed struct {
		ToolCalls []struct {
			ID   string                 `json:"id"`
			Name string                 `json:"name"`
			Args map[string]interface{} `json:"args"`
		} `json:"tool_calls"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &parsed); err != nil {
		cs.logger.Warning("Failed to parse tool calls JSON: %v, content: %s", err, jsonContent)

		return nil
	}

	if len(parsed.ToolCalls) == 0 {
		cs.logger.Debug("Parsed tool calls but array is empty")

		return nil
	}

	cs.logger.Debug("Detected %d tool calls", len(parsed.ToolCalls))

	toolCalls := make([]ToolCall, len(parsed.ToolCalls))
	for i, tc := range parsed.ToolCalls {
		toolCalls[i] = ToolCall{
			ID:   tc.ID,
			Name: tc.Name,
			Args: tc.Args,
		}
		cs.logger.Debug("Tool call %d: name=%s, args=%v", i, tc.Name, tc.Args)
	}

	return toolCalls
}

func (cs *ChatService) processToolCalls(session *ChatSession, toolCalls []ToolCall) []ToolCall {
	results := make([]ToolCall, len(toolCalls))

	for i, toolCall := range toolCalls {
		startTime := time.Now()

		result, err := cs.executeToolCall(toolCall)

		results[i] = ToolCall{
			ID:       toolCall.ID,
			Name:     toolCall.Name,
			Args:     toolCall.Args,
			Duration: time.Since(startTime),
		}

		if err != nil {
			results[i].Error = err.Error()
		} else {
			results[i].Result = result
		}
	}

	return results
}

func (cs *ChatService) executeToolCall(toolCall ToolCall) (string, error) {
	if cs.systemTools == nil {
		return "", fmt.Errorf("system tools not available")
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 30*time.Second)
	defer cancel()

	result, err := cs.systemTools.ExecuteSystemTool(ctx, toolCall.Name, toolCall.Args)
	if err != nil {
		return "", err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

func (cs *ChatService) createToolResultsMessage(session *ChatSession, toolResults []ToolCall) ChatMessage {
	var resultText strings.Builder
	resultText.WriteString("Tool execution results:\n\n")

	for _, result := range toolResults {
		resultText.WriteString(fmt.Sprintf("Tool: %s\n", result.Name))
		if result.Error != "" {
			resultText.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			resultText.WriteString(fmt.Sprintf("Result: %s\n", result.Result))
		}
		resultText.WriteString(fmt.Sprintf("Duration: %v\n\n", result.Duration))
	}

	return ChatMessage{
		ID:          uuid.New().String(),
		SessionID:   session.ID,
		Role:        "system",
		Content:     resultText.String(),
		ToolResults: toolResults,
		CreatedAt:   time.Now(),
	}
}

func (cs *ChatService) convertToAIMessages(messages []ChatMessage) []ai.Message {
	aiMessages := make([]ai.Message, 0, len(messages))

	for _, msg := range messages {
		content := msg.Content

		if len(msg.ToolCalls) > 0 {
			content += "\n\n[Tool Calls: "
			for i, tc := range msg.ToolCalls {
				if i > 0 {
					content += ", "
				}
				argsJSON, _ := json.Marshal(tc.Args)
				content += fmt.Sprintf("%s(%s)", tc.Name, string(argsJSON))
			}
			content += "]"
		}

		if len(msg.ToolResults) > 0 {
			content += "\n\n[Tool Results: "
			for i, tr := range msg.ToolResults {
				if i > 0 {
					content += "; "
				}
				if tr.Error != "" {
					content += fmt.Sprintf("%s: ERROR - %s", tr.Name, tr.Error)
				} else {
					content += fmt.Sprintf("%s: %s", tr.Name, tr.Result)
				}
			}
			content += "]"
		}

		aiMessages = append(aiMessages, ai.Message{
			Role:    msg.Role,
			Content: content,
		})
	}

	return aiMessages
}

func (cs *ChatService) addSystemContext(messages []ai.Message) []ai.Message {
	return cs.addSystemContextForSession("", messages)
}

func (cs *ChatService) addSystemContextForSession(sessionID string, messages []ai.Message) []ai.Message {
	systemContext := cs.BuildSystemContextForSession(sessionID)

	systemMessage := ai.Message{
		Role:    "system",
		Content: systemContext,
	}

	return append([]ai.Message{systemMessage}, messages...)
}

func (cs *ChatService) buildSystemContext() string {
	return cs.BuildSystemContextForSession("")
}

func (cs *ChatService) BuildSystemContextForSession(sessionID string) string {
	var ctx strings.Builder

	ctx.WriteString("You are an autonomous AI agent integrated into MCP-Compose, a Model Context Protocol server orchestration tool with scheduling capabilities.\n\n")

	ctx.WriteString("# CRITICAL: Autonomous Operation Mode\n\n")
	ctx.WriteString("You operate autonomously. When given a task:\n")
	ctx.WriteString("1. Execute ALL necessary tool calls to complete the task WITHOUT asking for permission\n")
	ctx.WriteString("2. Chain multiple tool calls together as needed - keep calling tools until the task is FULLY complete\n")
	ctx.WriteString("3. If a tool call fails, analyze the error and retry with corrected parameters or try an alternative approach\n")
	ctx.WriteString("4. Provide progress updates as you work through multi-step tasks\n")
	ctx.WriteString("5. Only stop when the task is completely finished or truly impossible\n")
	ctx.WriteString("6. Do NOT ask \"Would you like me to...?\" - just do it\n")
	ctx.WriteString("7. Do NOT wait for confirmation between steps - execute the full workflow\n\n")

	ctx.WriteString("Example of CORRECT autonomous behavior:\n")
	ctx.WriteString("User: \"Check my glucose and store it in memory\"\n")
	ctx.WriteString("You: [Call mcp_dexcom_get_glucose] → [Get result] → [Call memory_store_entity with result] → \"Done! Current glucose is 120 mg/dL, stored in memory.\"\n\n")

	ctx.WriteString("Example of INCORRECT behavior:\n")
	ctx.WriteString("User: \"Check my glucose and store it in memory\"\n")
	ctx.WriteString("You: \"I can check your glucose. Would you like me to proceed?\" ❌ NO! Just do it!\n\n")

	ctx.WriteString("# Task Scheduling & Automation\n\n")
	ctx.WriteString("You can create scheduled tasks that run automatically and post updates to this chat.\n\n")

	ctx.WriteString("## When to Create Tasks\n\n")
	ctx.WriteString("Create tasks when users want:\n")
	ctx.WriteString("- Repeated actions (e.g., \"check every 30 minutes\")\n")
	ctx.WriteString("- Scheduled notifications (e.g., \"remind me daily at 9am\")\n")
	ctx.WriteString("- Autonomous monitoring (e.g., \"watch for changes\")\n")
	ctx.WriteString("- Data collection (e.g., \"log my glucose hourly\")\n\n")

	ctx.WriteString("## Task Creation Examples\n\n")
	ctx.WriteString("User: \"Check my glucose every 30 minutes\"\n")
	ctx.WriteString("You: Use task_scheduler_create_task with:\n")
	ctx.WriteString("  - name: \"Glucose Monitor\"\n")
	ctx.WriteString("  - type: \"ai\"\n")
	ctx.WriteString("  - prompt: \"Check glucose via Dexcom MCP tool and report current value\"\n")
	ctx.WriteString("  - schedule: \"*/30 * * * *\"\n\n")

	ctx.WriteString("User: \"Remind me to take medicine at 9am and 9pm\"\n")
	ctx.WriteString("You: Create TWO tasks:\n")
	ctx.WriteString("  1. Morning: schedule \"0 9 * * *\"\n")
	ctx.WriteString("  2. Evening: schedule \"0 21 * * *\"\n\n")

	ctx.WriteString("User: \"Give me a daily summary at 8am\"\n")
	ctx.WriteString("You: Use task_scheduler_create_task with:\n")
	ctx.WriteString("  - name: \"Daily Summary\"\n")
	ctx.WriteString("  - type: \"ai\"\n")
	ctx.WriteString("  - prompt: \"Summarize important events from memory and provide daily overview\"\n")
	ctx.WriteString("  - schedule: \"0 8 * * *\"\n\n")

	ctx.WriteString("## Schedule Format (Cron)\n\n")
	ctx.WriteString("Common patterns:\n")
	ctx.WriteString("- Every 5 minutes: */5 * * * *\n")
	ctx.WriteString("- Every 30 minutes: */30 * * * *\n")
	ctx.WriteString("- Every hour: 0 * * * *\n")
	ctx.WriteString("- Every 6 hours: 0 */6 * * *\n")
	ctx.WriteString("- Daily at 9 AM: 0 9 * * *\n")
	ctx.WriteString("- Daily at 9 AM & 9 PM: 0 9,21 * * *\n")
	ctx.WriteString("- Weekdays at 9 AM: 0 9 * * 1-5\n")
	ctx.WriteString("- Weekly on Monday: 0 0 * * 1\n")
	ctx.WriteString("- Monthly on 1st: 0 0 1 * *\n\n")
	ctx.WriteString("Format: minute hour day month weekday\n\n")

	ctx.WriteString("## Task Management\n\n")
	ctx.WriteString("After creating a task, you can manage it:\n\n")
	ctx.WriteString("- **List tasks**: Use task_scheduler_list_tasks to see all tasks\n")
	ctx.WriteString("- **Update schedule**: Use task_scheduler_update_schedule(task_id, schedule) to change when a task runs\n")
	ctx.WriteString("  Example: Change to daily at 8am: task_scheduler_update_schedule(\"task_123\", \"0 8 * * *\")\n")
	ctx.WriteString("- **Pause**: Use task_scheduler_pause_task(task_id) to temporarily stop a task\n")
	ctx.WriteString("- **Resume**: Use task_scheduler_resume_task(task_id) to restart a paused task\n")
	ctx.WriteString("- **Delete**: Use task_scheduler_delete_task(task_id) to permanently remove a task\n")
	ctx.WriteString("- **Run now**: Use task_scheduler_run_now(task_id) to execute immediately\n\n")
	ctx.WriteString("**Important**: Get the task_id from task_scheduler_list_tasks first!\n\n")

	ctx.WriteString("## Important Guidelines\n\n")
	ctx.WriteString("1. Always confirm task creation with user-friendly language\n")
	ctx.WriteString("2. Show next run time in confirmation\n")
	ctx.WriteString("3. Explain that updates will appear in this chat\n")
	ctx.WriteString("4. For AI tasks, the task will have access to all MCP tools enabled in this session\n")
	ctx.WriteString("5. Tasks inherit this session's provider, model, and MCP configuration\n\n")

	ctx.WriteString("## Confirmation Format\n\n")
	ctx.WriteString("When creating a task, respond like:\n")
	ctx.WriteString("\"I've set up [task name] to [what it does] [when it runs]. You'll see updates here in chat. Next run: [time]\"\n\n")
	ctx.WriteString("Example:\n")
	ctx.WriteString("\"I've set up Glucose Monitor to check your levels every 30 minutes. You'll see updates here in chat. Next run: 2:30 PM\"\n\n")

	ctx.WriteString("# Tool Usage\n\n")
	ctx.WriteString("You have access to tools via native function calling. When you need to use a tool, simply call it using the native tool calling interface.\n\n")
	ctx.WriteString("Important:\n")
	ctx.WriteString("- For MCP server tools, use the full prefixed name like \"mcp_dexcom_get_current_glucose\"\n")
	ctx.WriteString("- All tool names are listed below with their descriptions and parameters\n")
	ctx.WriteString("- You can call multiple tools in sequence if needed\n\n")

	ctx.WriteString("# Available System Tools\n")
	ctx.WriteString("These tools are ALWAYS available and control MCP-Compose:\n\n")

	if cs.systemTools != nil {
		systemTools := cs.systemTools.GetSystemTools()
		for _, tool := range systemTools {
			ctx.WriteString(fmt.Sprintf("## %s\n", tool.Name))
			ctx.WriteString(fmt.Sprintf("Description: %s\n", tool.Description))
			if tool.InputSchema != nil {
				if props, ok := tool.InputSchema["properties"].(map[string]interface{}); ok && len(props) > 0 {
					ctx.WriteString("Parameters:\n")
					for propName, propDetails := range props {
						if propMap, ok := propDetails.(map[string]interface{}); ok {
							propDesc, _ := propMap["description"].(string)
							propType, _ := propMap["type"].(string)
							ctx.WriteString(fmt.Sprintf("  - %s (%s): %s\n", propName, propType, propDesc))
						}
					}
				} else {
					ctx.WriteString("Parameters: none\n")
				}
			}
			ctx.WriteString("\n")
		}
	}

	enabledMCPServers := []string{}
	if sessionID != "" {
		session, err := cs.GetSession(sessionID)
		if err == nil && len(session.MCPServers) > 0 {
			enabledMCPServers = session.MCPServers
		}
	}

	ctx.WriteString("\n# MCP Server Tools\n")
	if len(enabledMCPServers) > 0 {
		ctx.WriteString("The following MCP servers are ENABLED for this session. Use these tools by prefixing with 'mcp_{server}_{tool}':\n\n")

		for _, serverName := range enabledMCPServers {
			tools, err := cs.fetchMCPServerTools(serverName)
			if err != nil {
				ctx.WriteString(fmt.Sprintf("## %s (Error loading tools)\n", serverName))
				ctx.WriteString(fmt.Sprintf("Error: %v\n\n", err))

				continue
			}

			ctx.WriteString(fmt.Sprintf("## %s (%d tools available)\n", serverName, len(tools)))
			for _, tool := range tools {
				toolName, _ := tool["name"].(string)
				toolDesc, _ := tool["description"].(string)
				inputSchema, _ := tool["inputSchema"].(map[string]interface{})
				fullToolName := fmt.Sprintf("mcp_%s_%s", serverName, toolName)
				ctx.WriteString(fmt.Sprintf("\n### %s\n", fullToolName))
				ctx.WriteString(fmt.Sprintf("Description: %s\n", toolDesc))
				if inputSchema != nil {
					if props, ok := inputSchema["properties"].(map[string]interface{}); ok && len(props) > 0 {
						ctx.WriteString("Parameters:\n")
						for propName, propDetails := range props {
							if propMap, ok := propDetails.(map[string]interface{}); ok {
								propDesc, _ := propMap["description"].(string)
								propType, _ := propMap["type"].(string)
								ctx.WriteString(fmt.Sprintf("  - %s (%s): %s\n", propName, propType, propDesc))
							}
						}
					}
				}
			}
			ctx.WriteString("\n")
		}
	} else {
		ctx.WriteString("No MCP servers are currently enabled for this session.\n")
		ctx.WriteString("The user can enable MCP servers using the 'Configure MCPs' button in the UI.\n")
		ctx.WriteString("Available servers are listed in the 'Running MCP Servers' section below.\n\n")
	}

	ctx.WriteString("\n# Running MCP Servers\n")
	if cs.systemTools != nil {
		serverInfo, err := cs.systemTools.serverList(context.Background())
		if err == nil {
			if serverMap, ok := serverInfo.(map[string]interface{}); ok {
				if servers, ok := serverMap["servers"].([]map[string]interface{}); ok {
					if len(servers) > 0 {
						ctx.WriteString("The following MCP servers are currently running:\n\n")
						for _, server := range servers {
							name := server["name"]
							status := server["status"]
							protocol := server["protocol"]
							enabled := "disabled for this session"
							for _, enabledServer := range enabledMCPServers {
								if enabledServer == name {
									enabled = "ENABLED for this session"

									break
								}
							}
							ctx.WriteString(fmt.Sprintf("- **%s** (Status: %s, Protocol: %s, %s)\n", name, status, protocol, enabled))
						}
					} else {
						ctx.WriteString("No MCP servers are currently running.\n")
					}
				}
			}
		}
	}

	return ctx.String()
}

func (cs *ChatService) generateSessionTitle(session *ChatSession) {
	if len(session.Messages) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(cs.ctx, 30*time.Second)
	defer cancel()

	titlePrompt := []ai.Message{
		{
			Role:    "user",
			Content: session.Messages[0].Content,
		},
		{
			Role:    "user",
			Content: "Generate a short, descriptive title (3-5 words) for this conversation. Return only the title, no quotes or punctuation.",
		},
	}

	title, err := cs.aiManager.Chat(ctx, titlePrompt)
	if err != nil {
		cs.logger.Info("Failed to generate session title: %v", err)
		return
	}

	title = strings.TrimSpace(title)
	title = strings.Trim(title, "\"'")
	if len(title) > 50 {
		title = title[:50]
	}

	session.Title = title
	session.LastUsed = time.Now()

	if cs.Storage != nil {
		ctx := context.Background()
		updates := map[string]interface{}{"title": title}
		if err := cs.Storage.UpdateSession(ctx, session.ID, updates); err != nil {
			cs.logger.Info("Failed to update session title: %v", err)
		}
	}
}

func (cs *ChatService) GetProviders() ([]ProviderInfo, error) {
	if cs.aiManager == nil {
		return nil, fmt.Errorf("AI manager not available")
	}

	status := cs.aiManager.GetProviderStatus()

	providers := []ProviderInfo{
		{
			Name:    "openrouter",
			Enabled: false,
			Healthy: false,
			Models: []string{
				"anthropic/claude-3.5-sonnet",
				"anthropic/claude-3-opus",
				"anthropic/claude-3-sonnet",
				"anthropic/claude-3-haiku",
				"openai/gpt-4-turbo",
				"openai/gpt-4",
				"openai/gpt-3.5-turbo",
			},
		},
		{
			Name:    "claude",
			Enabled: false,
			Healthy: false,
			Models: []string{
				"claude-3-5-sonnet-20241022",
				"claude-3-opus-20240229",
				"claude-3-sonnet-20240229",
				"claude-3-haiku-20240307",
			},
		},
		{
			Name:    "openai",
			Enabled: false,
			Healthy: false,
			Models: []string{
				"gpt-4-turbo",
				"gpt-4",
				"gpt-3.5-turbo",
				"gpt-4o",
				"gpt-4o-mini",
			},
		},
		{
			Name:    "ollama",
			Enabled: false,
			Healthy: false,
			Models: []string{
				"llama2",
				"mistral",
				"codellama",
				"llama3",
				"gemma",
				"qwen",
			},
		},
	}

	for i := range providers {
		if providerStatus, exists := status[providers[i].Name]; exists {
			providers[i].Enabled = providerStatus.Enabled
			providers[i].Healthy = providerStatus.Healthy
		}
	}

	return providers, nil
}

func (cs *ChatService) GetAvailableTools() ([]map[string]interface{}, error) {
	tools := []map[string]interface{}{
		{
			"name":        "task_scheduler_status",
			"description": "Get the current status of the task scheduler",
			"parameters":  map[string]interface{}{},
		},
		{
			"name":        "task_scheduler_start",
			"description": "Start the task scheduler",
			"parameters":  map[string]interface{}{},
		},
		{
			"name":        "task_scheduler_stop",
			"description": "Stop the task scheduler",
			"parameters":  map[string]interface{}{},
		},
		{
			"name":        "memory_search",
			"description": "Search for information in the memory store",
			"parameters": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
					"required":    true,
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of results (default: 10)",
					"required":    false,
				},
			},
		},
		{
			"name":        "memory_store_entity",
			"description": "Store an entity in the memory store",
			"parameters": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Entity type",
					"required":    true,
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Entity name",
					"required":    true,
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Entity content",
					"required":    true,
				},
			},
		},
		{
			"name":        "memory_stats",
			"description": "Get memory store statistics",
			"parameters":  map[string]interface{}{},
		},
		{
			"name":        "memory_prune",
			"description": "Remove old entries from memory store",
			"parameters": map[string]interface{}{
				"days": map[string]interface{}{
					"type":        "integer",
					"description": "Remove entries older than this many days (default: 30)",
					"required":    false,
				},
			},
		},
		{
			"name":        "server_list",
			"description": "List all MCP servers and their status",
			"parameters":  map[string]interface{}{},
		},
		{
			"name":        "server_start",
			"description": "Start a specific MCP server",
			"parameters": map[string]interface{}{
				"server": map[string]interface{}{
					"type":        "string",
					"description": "Server name",
					"required":    true,
				},
			},
		},
		{
			"name":        "server_stop",
			"description": "Stop a specific MCP server",
			"parameters": map[string]interface{}{
				"server": map[string]interface{}{
					"type":        "string",
					"description": "Server name",
					"required":    true,
				},
			},
		},
		{
			"name":        "server_restart",
			"description": "Restart a specific MCP server",
			"parameters": map[string]interface{}{
				"server": map[string]interface{}{
					"type":        "string",
					"description": "Server name",
					"required":    true,
				},
			},
		},
		{
			"name":        "server_logs",
			"description": "Get logs from a specific MCP server",
			"parameters": map[string]interface{}{
				"server": map[string]interface{}{
					"type":        "string",
					"description": "Server name",
					"required":    true,
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of log lines to retrieve (default: 50)",
					"required":    false,
				},
			},
		},
	}

	return tools, nil
}

func (cs *ChatService) Stop() {
	cs.cancel()

	cs.mu.Lock()
	cs.sessions = make(map[string]*ChatSession)
	cs.mu.Unlock()
}

func (cs *ChatService) marshalToolCalls(toolCalls []ToolCall) ([]byte, error) {
	return json.Marshal(toolCalls)
}

func (cs *ChatService) unmarshalToolCalls(data []byte) ([]ToolCall, error) {
	var toolCalls []ToolCall
	if err := json.Unmarshal(data, &toolCalls); err != nil {
		return nil, err
	}

	return toolCalls, nil
}

func (cs *ChatService) GetAvailableProviders() map[string][]string {
	providers := make(map[string][]string)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Try to list models from each provider through the AI manager
	// For now, return common models for each provider type
	// TODO: Implement dynamic model fetching from provider APIs

	// Check which providers are configured by attempting to get their status
	testProviders := []string{"openrouter", "claude", "openai", "ollama"}

	for _, providerName := range testProviders {
		var models []string

		switch providerName {
		case "openrouter":
			// OpenRouter - check if API key is set
			if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
				models = cs.fetchOpenRouterModels(ctx)
			}
		case "claude":
			// Claude/Anthropic
			if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
				models = []string{
					"claude-3-5-sonnet-20241022",
					"claude-3-opus-20240229",
					"claude-3-sonnet-20240229",
					"claude-3-haiku-20240307",
				}
			}
		case "openai":
			// OpenAI
			if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
				models = []string{
					"gpt-4-turbo-preview",
					"gpt-4",
					"gpt-3.5-turbo",
					"gpt-3.5-turbo-16k",
				}
			}
		case "ollama":
			// Ollama - always try to add, use configured URL or default
			ollamaURL := os.Getenv("OLLAMA_BASE_URL")
			if ollamaURL == "" {
				ollamaURL = "http://localhost:11434"
			}
			cs.logger.Info("Attempting to fetch Ollama models from: %s", ollamaURL)
			models = cs.fetchOllamaModels(ctx, ollamaURL)
			cs.logger.Info("Ollama returned %d models", len(models))
		}

		if len(models) > 0 {
			providers[providerName] = models
		}
	}

	return providers
}

func (cs *ChatService) fetchOpenRouterModels(ctx context.Context) []string {
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	if apiKey == "" {
		return []string{}
	}

	// Fetch models from OpenRouter API
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		cs.logger.Error("Failed to create OpenRouter models request: %v", err)
		return cs.getDefaultOpenRouterModels()
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://mcp-compose.local")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		cs.logger.Error("Failed to fetch OpenRouter models: %v", err)
		return cs.getDefaultOpenRouterModels()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		cs.logger.Error("OpenRouter API returned status %d", resp.StatusCode)
		return cs.getDefaultOpenRouterModels()
	}

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		cs.logger.Error("Failed to decode OpenRouter response: %v", err)
		return cs.getDefaultOpenRouterModels()
	}

	models := make([]string, 0, len(result.Data))
	for _, model := range result.Data {
		models = append(models, model.ID)
	}

	if len(models) == 0 {
		return cs.getDefaultOpenRouterModels()
	}

	return models
}

func (cs *ChatService) getDefaultOpenRouterModels() []string {
	return []string{
		"anthropic/claude-3.5-sonnet",
		"anthropic/claude-3-opus",
		"anthropic/claude-3-sonnet",
		"anthropic/claude-3-haiku",
		"openai/gpt-4-turbo",
		"openai/gpt-4",
		"openai/gpt-3.5-turbo",
		"meta-llama/llama-3.1-70b-instruct",
		"meta-llama/llama-3.1-8b-instruct",
	}
}

func (cs *ChatService) fetchOllamaModels(ctx context.Context, baseURL string) []string {
	if cs.aiManager == nil {
		cs.logger.Warning("AI manager not available for Ollama model listing")

		return []string{}
	}

	provider, err := cs.aiManager.GetProvider("ollama")
	if err != nil {
		cs.logger.Warning("Failed to get Ollama provider: %v", err)

		return []string{}
	}

	models, err := provider.ListModels(ctx)
	if err != nil {
		cs.logger.Error("Failed to list Ollama models from API: %v", err)

		return []string{}
	}

	cs.logger.Info("Successfully fetched %d Ollama models from API", len(models))

	return models
}

func (cs *ChatService) GetAvailableMCPServers() ([]map[string]interface{}, error) {
	return cs.getRunningServersFromSystemTools()
}

func (cs *ChatService) getRunningServersFromSystemTools() ([]map[string]interface{}, error) {
	servers := []map[string]interface{}{}

	if cs.systemTools == nil {
		return servers, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverInfo, err := cs.systemTools.serverList(ctx)
	if err != nil {
		cs.logger.Error("Failed to get server list from system tools: %v", err)
		return servers, nil
	}

	if serverMap, ok := serverInfo.(map[string]interface{}); ok {
		if serversList, ok := serverMap["servers"].([]map[string]interface{}); ok {
			for _, server := range serversList {
				if name, ok := server["name"].(string); ok {
					status, _ := server["status"].(string)
					protocol, _ := server["protocol"].(string)

					// Include protocol information in the response
					serverData := map[string]interface{}{
						"name":     name,
						"status":   status,
						"protocol": protocol,
					}

					// Try to fetch tools, but don't fail if it errors
					// SSE servers may not have tools available until a connection is established
					tools, err := cs.fetchMCPServerTools(name)
					if err != nil {
						cs.logger.Debug("Could not fetch tools for server %s (protocol: %s): %v", name, protocol, err)
						// For SSE servers, this is expected if no connection is active
						serverData["tools"] = nil
						serverData["tool_count"] = 0
						serverData["tools_available"] = false
					} else {
						toolCount := 0
						if tools != nil {
							toolCount = len(tools)
						}
						serverData["tools"] = tools
						serverData["tool_count"] = toolCount
						serverData["tools_available"] = true
					}

					servers = append(servers, serverData)
				}
			}
		}
	}

	return servers, nil
}

func (cs *ChatService) buildToolDefinitions(sessionID string) ([]ai.Tool, error) {
	tools := make([]ai.Tool, 0)

	if cs.systemTools != nil {
		systemToolDefs := cs.systemTools.GetSystemTools()
		for _, toolDef := range systemToolDefs {
			tools = append(tools, ai.Tool{
				Name:        toolDef.Name,
				Description: toolDef.Description,
				InputSchema: toolDef.InputSchema,
			})
		}
	}

	session, err := cs.GetSession(sessionID)
	if err == nil && len(session.MCPServers) > 0 {
		cs.logger.Debug("buildToolDefinitions: session %s has %d MCP servers enabled: %v", sessionID, len(session.MCPServers), session.MCPServers)
		for _, serverName := range session.MCPServers {
			mcpTools, err := cs.fetchMCPServerTools(serverName)
			if err != nil {
				cs.logger.Warning("Failed to fetch tools from MCP server %s: %v", serverName, err)

				continue
			}

			for _, mcpTool := range mcpTools {
				name, _ := mcpTool["name"].(string)
				desc, _ := mcpTool["description"].(string)
				inputSchema, _ := mcpTool["inputSchema"].(map[string]interface{})

				if name != "" {
					fullToolName := fmt.Sprintf("mcp_%s_%s", serverName, name)

					testParts := strings.SplitN(fullToolName, "_", 3)
					if len(testParts) != 3 {
						cs.logger.Error("Invalid tool name generated: %s (parts: %v, expected 3 parts)", fullToolName, testParts)

						continue
					}

					cs.logger.Debug("Adding MCP tool: %s (server: %s, tool: %s)", fullToolName, serverName, name)

					tools = append(tools, ai.Tool{
						Name:        fullToolName,
						Description: fmt.Sprintf("[MCP:%s] %s", serverName, desc),
						InputSchema: inputSchema,
					})
				}
			}
		}
	}

	return tools, nil
}

func (cs *ChatService) streamResponseWithTools(session *ChatSession, messages []ai.Message, tools []ai.Tool) (<-chan string, error) {
	outCh := make(chan string, 100)

	go func() {
		defer close(outCh)

		ctx, cancel := context.WithTimeout(cs.ctx, 5*time.Minute)
		defer cancel()

		provider, err := cs.aiManager.GetHealthyProvider()
		if err != nil {
			errorMsg := fmt.Sprintf("Error: no healthy provider available: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		chatResp, err := provider.ChatWithTools(ctx, messages, tools)
		if err != nil {
			errorMsg := fmt.Sprintf("Error: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		cs.logger.Debug("Received response from provider: %d tool calls, text length: %d", len(chatResp.ToolCalls), len(chatResp.TextContent))

		assistantMsg := ChatMessage{
			ID:        uuid.New().String(),
			SessionID: session.ID,
			Role:      "assistant",
			Content:   chatResp.TextContent,
			CreatedAt: time.Now(),
		}

		select {
		case outCh <- chatResp.TextContent:
		case <-ctx.Done():
			return
		}

		maxIterations := 10
		iteration := 0

		for iteration < maxIterations {
			if len(chatResp.ToolCalls) == 0 {
				break
			}

			iteration++
			cs.logger.Debug("Agentic iteration %d: Processing %d tool calls", iteration, len(chatResp.ToolCalls))

			toolCalls := cs.convertToolCallsToLegacy(chatResp.ToolCalls)
			toolCallsJSON, _ := json.Marshal(toolCalls)
			select {
			case outCh <- fmt.Sprintf("__TOOL_CALLS__%s", string(toolCallsJSON)):
			case <-ctx.Done():
				return
			}

			toolResults := cs.executeToolCalls(ctx, session, chatResp.ToolCalls)
			assistantMsg.ToolCalls = toolCalls
			assistantMsg.ToolResults = toolResults

			for _, result := range toolResults {
				if result.Error != "" {
					cs.logger.Error("Tool call failed: tool=%s, error=%s", result.Name, result.Error)
					errorMsg := fmt.Sprintf("\n\nTool Error: %s failed: %s", result.Name, result.Error)
					select {
					case outCh <- errorMsg:
					case <-ctx.Done():
						return
					}
				} else {
					cs.logger.Debug("Tool call succeeded: tool=%s, result_length=%d", result.Name, len(result.Result))
				}
			}

			toolResultsJSON, _ := json.Marshal(toolResults)
			select {
			case outCh <- fmt.Sprintf("__TOOL_RESULTS__%s", string(toolResultsJSON)):
			case <-ctx.Done():
				return
			}

			session.Messages = append(session.Messages, assistantMsg)

			if cs.Storage != nil {
				cs.Storage.AddMessage(ctx, &assistantMsg)
			}

			cs.broadcastMessage(session.ID, &assistantMsg)

			toolResultsText := cs.formatToolResults(toolResults)

			messages = append(messages, ai.Message{
				Role:    "assistant",
				Content: chatResp.TextContent,
			})
			messages = append(messages, ai.Message{
				Role:    "user",
				Content: toolResultsText,
			})

			cs.logger.Debug("Agentic iteration %d: Making follow-up AI call (message count: %d)", iteration, len(messages))

			chatResp, err = provider.ChatWithTools(ctx, messages, tools)
			if err != nil {
				cs.logger.Error("Error in agentic iteration %d: %v", iteration, err)
				errorMsg := fmt.Sprintf("\n\nError continuing task: %v", err)
				select {
				case outCh <- errorMsg:
				case <-ctx.Done():
				}
				return
			}

			cs.logger.Debug("Iteration %d response: %d tool calls, text length: %d", iteration, len(chatResp.ToolCalls), len(chatResp.TextContent))

			if chatResp.TextContent != "" {
				assistantMsg = ChatMessage{
					ID:        uuid.New().String(),
					SessionID: session.ID,
					Role:      "assistant",
					Content:   chatResp.TextContent,
					CreatedAt: time.Now(),
				}

				select {
				case outCh <- "\n\n" + chatResp.TextContent:
				case <-ctx.Done():
					return
				}
			}
		}

		if iteration >= maxIterations {
			cs.logger.Warning("Reached maximum agentic iterations (%d), stopping", maxIterations)
			select {
			case outCh <- "\n\nWarning: Reached maximum iterations. Task may be incomplete.":
			case <-ctx.Done():
			}

			session.Messages = append(session.Messages, assistantMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save assistant message after max iterations: %v", err)
				}
			}

			cs.broadcastMessage(session.ID, &assistantMsg)

			return
		}

		if len(chatResp.ToolCalls) == 0 {
			session.Messages = append(session.Messages, assistantMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save final assistant message: %v", err)
				}
			}

			cs.broadcastMessage(session.ID, &assistantMsg)
		}
	}()

	return outCh, nil
}

func (cs *ChatService) chatResponseWithTools(session *ChatSession, messages []ai.Message, tools []ai.Tool) (<-chan string, error) {
	outCh := make(chan string, 1)

	go func() {
		defer close(outCh)

		ctx, cancel := context.WithTimeout(cs.ctx, 5*time.Minute)
		defer cancel()

		provider, err := cs.aiManager.GetHealthyProvider()
		if err != nil {
			errorMsg := fmt.Sprintf("Error: no healthy provider available: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		chatResp, err := provider.ChatWithTools(ctx, messages, tools)
		if err != nil {
			errorMsg := fmt.Sprintf("Error: %v", err)
			select {
			case outCh <- errorMsg:
			case <-ctx.Done():
			}
			return
		}

		assistantMsg := ChatMessage{
			ID:        uuid.New().String(),
			SessionID: session.ID,
			Role:      "assistant",
			Content:   chatResp.TextContent,
			CreatedAt: time.Now(),
		}

		fullResponse := chatResp.TextContent

		if len(chatResp.ToolCalls) > 0 {
			toolResults := cs.executeToolCalls(ctx, session, chatResp.ToolCalls)
			assistantMsg.ToolCalls = cs.convertToolCallsToLegacy(chatResp.ToolCalls)
			assistantMsg.ToolResults = toolResults

			session.Messages = append(session.Messages, assistantMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save assistant message with tool calls: %v", err)
				}
			}

			toolResultsText := cs.formatToolResults(toolResults)
			fullResponse += "\n\n" + toolResultsText

			messages = append(messages, ai.Message{
				Role:    "assistant",
				Content: chatResp.TextContent,
			})
			messages = append(messages, ai.Message{
				Role:    "user",
				Content: toolResultsText,
			})

			finalResp, err := provider.ChatWithTools(ctx, messages, tools)
			if err != nil {
				errorMsg := fmt.Sprintf("Error synthesizing results: %v", err)
				select {
				case outCh <- fullResponse + "\n\n" + errorMsg:
				case <-ctx.Done():
				}
				return
			}

			finalMsg := ChatMessage{
				ID:        uuid.New().String(),
				SessionID: session.ID,
				Role:      "assistant",
				Content:   finalResp.TextContent,
				CreatedAt: time.Now(),
			}

			session.Messages = append(session.Messages, finalMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &finalMsg); err != nil {
					cs.logger.Error("Failed to save final assistant message after tool synthesis: %v", err)
				}
			}

			cs.broadcastMessage(session.ID, &finalMsg)

			fullResponse += "\n\n" + finalResp.TextContent
		} else {
			session.Messages = append(session.Messages, assistantMsg)

			if cs.Storage != nil {
				if err := cs.Storage.AddMessage(ctx, &assistantMsg); err != nil {
					cs.logger.Error("Failed to save assistant message (no tools): %v", err)
				}
			}

			cs.broadcastMessage(session.ID, &assistantMsg)
		}

		select {
		case outCh <- fullResponse:
		case <-ctx.Done():
		}
	}()

	return outCh, nil
}

func (cs *ChatService) executeToolCalls(ctx context.Context, session *ChatSession, toolCalls []ai.ToolUseBlock) []ToolCall {
	results := make([]ToolCall, len(toolCalls))

	for i, toolCall := range toolCalls {
		startTime := time.Now()

		result, err := cs.executeToolCallByName(ctx, session, toolCall.Name, toolCall.Input)

		results[i] = ToolCall{
			ID:       toolCall.ID,
			Name:     toolCall.Name,
			Args:     toolCall.Input,
			Duration: time.Since(startTime),
		}

		if err != nil {
			results[i].Error = err.Error()
		} else {
			results[i].Result = result
		}
	}

	return results
}

func (cs *ChatService) executeToolCallByName(ctx context.Context, session *ChatSession, toolName string, args map[string]interface{}) (string, error) {
	cs.logger.Debug("executeToolCallByName: toolName=%s, args=%v", toolName, args)

	ctxWithSession := context.WithValue(ctx, "session_id", session.ID)
	ctxWithSession = context.WithValue(ctxWithSession, "session", session)

	if strings.HasPrefix(toolName, "mcp_") {
		parts := strings.SplitN(toolName, "_", 3)
		cs.logger.Debug("executeToolCallByName: MCP tool detected, parts=%v", parts)

		if len(parts) < 3 {
			return "", fmt.Errorf("invalid MCP tool name format: %s", toolName)
		}

		serverName := parts[1]
		actualToolName := parts[2]

		cs.logger.Debug("executeToolCallByName: Calling MCP server=%s, tool=%s", serverName, actualToolName)

		return cs.executeMCPTool(ctxWithSession, serverName, actualToolName, args)
	}

	if cs.systemTools == nil {
		return "", fmt.Errorf("system tools not available")
	}

	enrichedArgs := make(map[string]interface{})
	for k, v := range args {
		enrichedArgs[k] = v
	}

	if sessionID, ok := ctxWithSession.Value("session_id").(string); ok && sessionID != "" {
		enrichedArgs["_chat_session_id"] = sessionID
		enrichedArgs["_output_to_chat"] = true
		cs.logger.Info("SYSTEM TOOL ENRICHMENT: session_id=%s, tool=%s", sessionID, toolName)

		if session, ok := ctxWithSession.Value("session").(*ChatSession); ok && session != nil {
			if session.Provider != "" {
				enrichedArgs["_provider"] = session.Provider
				cs.logger.Info("SYSTEM TOOL ENRICHMENT: Added provider=%s", session.Provider)
			}
			if session.Model != "" {
				enrichedArgs["_model"] = session.Model
				cs.logger.Info("SYSTEM TOOL ENRICHMENT: Added model=%s", session.Model)
			}
			if len(session.MCPServers) > 0 {
				enrichedArgs["_mcp_servers"] = session.MCPServers
				cs.logger.Info("SYSTEM TOOL ENRICHMENT: Added %d MCP servers", len(session.MCPServers))
			}
		}
	}

	result, err := cs.systemTools.ExecuteSystemTool(ctxWithSession, toolName, enrichedArgs)
	if err != nil {
		return "", err
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	return string(resultJSON), nil
}

func (cs *ChatService) executeMCPTool(ctx context.Context, serverName, toolName string, args map[string]interface{}) (string, error) {
	url := fmt.Sprintf("%s/%s", cs.proxyURL, serverName)

	enrichedArgs := make(map[string]interface{})
	for k, v := range args {
		enrichedArgs[k] = v
	}

	if sessionID, ok := ctx.Value("session_id").(string); ok && sessionID != "" {
		enrichedArgs["_chat_session_id"] = sessionID
		enrichedArgs["_output_to_chat"] = true
		cs.logger.Debug("executeMCPTool: Enriched args with _chat_session_id=%s", sessionID)

		if session, ok := ctx.Value("session").(*ChatSession); ok && session != nil {
			if session.Provider != "" {
				enrichedArgs["_provider"] = session.Provider
				cs.logger.Debug("executeMCPTool: Enriched args with _provider=%s", session.Provider)
			}
			if session.Model != "" {
				enrichedArgs["_model"] = session.Model
				cs.logger.Debug("executeMCPTool: Enriched args with _model=%s", session.Model)
			}
			if len(session.MCPServers) > 0 {
				enrichedArgs["_mcp_servers"] = session.MCPServers
				cs.logger.Debug("executeMCPTool: Enriched args with _mcp_servers=%v", session.MCPServers)
			}
		}
	}

	reqBody := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": enrichedArgs,
		},
		"id": 1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	cs.logger.Debug("executeMCPTool: Calling URL=%s with payload=%s", url, string(jsonData))

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if cs.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+cs.apiKey)
		cs.logger.Debug("executeMCPTool: Authorization header set")
	} else {
		cs.logger.Warning("executeMCPTool: No API key available for authentication")
	}

	if sessionID, ok := ctx.Value("session_id").(string); ok && sessionID != "" {
		req.Header.Set("X-Chat-Session-ID", sessionID)
		cs.logger.Debug("executeMCPTool: X-Chat-Session-ID header set to %s", sessionID)
	}

	resp, err := cs.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute tool: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		cs.logger.Error("executeMCPTool: Server returned status %d: %s", resp.StatusCode, string(body))

		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	cs.logger.Debug("executeMCPTool: Response received: %s", string(bodyBytes))

	var result struct {
		Result interface{} `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if result.Error != nil {
		return "", fmt.Errorf("MCP error: %s", result.Error.Message)
	}

	resultJSON, err := json.Marshal(result.Result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}

	cs.logger.Debug("executeMCPTool: Successfully executed, result length=%d", len(resultJSON))

	return string(resultJSON), nil
}

func (cs *ChatService) convertToolCallsToLegacy(toolCalls []ai.ToolUseBlock) []ToolCall {
	legacy := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		legacy[i] = ToolCall{
			ID:   tc.ID,
			Name: tc.Name,
			Args: tc.Input,
		}
	}

	return legacy
}

func (cs *ChatService) formatToolResults(results []ToolCall) string {
	var sb strings.Builder
	sb.WriteString("Tool Results:\n\n")

	for _, result := range results {
		sb.WriteString(fmt.Sprintf("Tool: %s\n", result.Name))
		if result.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", result.Error))
		} else {
			sb.WriteString(fmt.Sprintf("Result: %s\n", result.Result))
		}
		sb.WriteString(fmt.Sprintf("Duration: %v\n\n", result.Duration))
	}

	return sb.String()
}

func (cs *ChatService) fetchMCPServerTools(serverName string) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/%s", cs.proxyURL, serverName)

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
	if cs.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+cs.apiKey)
	} else {
		cs.logger.Warning("fetchMCPServerTools: No API key set for request to %s", serverName)
	}

	cs.logger.Debug("Fetching tools from %s (proxyURL=%s, apiKey=%v)", serverName, cs.proxyURL, cs.apiKey != "")

	resp, err := cs.httpClient.Do(req)
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

	return result.Result.Tools, nil
}

func (cs *ChatService) SetSessionMCPServers(sessionID string, mcpServers []string) error {
	session, err := cs.GetSession(sessionID)
	if err != nil {
		if cs.Storage != nil {
			ctx := context.Background()
			_, storageErr := cs.Storage.GetSession(ctx, sessionID)
			if storageErr != nil {
				return nil
			}
		}
		return err
	}

	session.MCPServers = mcpServers

	if cs.Storage != nil {
		ctx := context.Background()

		_, err := cs.Storage.GetSession(ctx, sessionID)
		if err != nil {
			return nil
		}

		return cs.Storage.SetSessionMCPServers(ctx, sessionID, mcpServers)
	}

	return nil
}

func (cs *ChatService) GetSessionMCPServers(sessionID string) ([]string, error) {
	session, err := cs.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	return session.MCPServers, nil
}

func (cs *ChatService) getDefaultMCPServers() []string {
	defaultServers := []string{}

	if cs.systemTools != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		serverInfo, err := cs.systemTools.serverList(ctx)
		if err == nil {
			if serverMap, ok := serverInfo.(map[string]interface{}); ok {
				if servers, ok := serverMap["servers"].([]map[string]interface{}); ok {
					for _, server := range servers {
						if name, ok := server["name"].(string); ok {
							status, _ := server["status"].(string)
							if status == "Running" || status == "Process" {
								defaultServers = append(defaultServers, name)
							}
						}
					}
				}
			}
		}
	}

	return defaultServers
}
