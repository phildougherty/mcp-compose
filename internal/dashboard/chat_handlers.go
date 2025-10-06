package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type CreateSessionRequest struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	UserID   string `json:"user_id,omitempty"`
	Title    string `json:"title,omitempty"`
}

type UpdateSessionRequest struct {
	Title      string   `json:"title,omitempty"`
	Provider   string   `json:"provider,omitempty"`
	Model      string   `json:"model,omitempty"`
	MCPServers []string `json:"mcp_servers,omitempty"`
}

type SendMessageRequest struct {
	Message string `json:"message"`
}

type WebSocketMessage struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type StreamChunk struct {
	Type        string     `json:"type"`
	SessionID   string     `json:"sessionId,omitempty"`
	MessageID   string     `json:"message_id,omitempty"`
	Role        string     `json:"role,omitempty"`
	Content     string     `json:"content,omitempty"`
	ToolCalls   []ToolCall `json:"tool_calls,omitempty"`
	ToolResults []ToolCall `json:"tool_results,omitempty"`
	Done        bool       `json:"done"`
	Error       string     `json:"error,omitempty"`
}

type ProviderInfo struct {
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Healthy  bool     `json:"healthy"`
	Models   []string `json:"models"`
	Selected bool     `json:"selected"`
}

func (s *DashboardServer) registerChatRoutes() {
	s.mux.HandleFunc("/api/chat/sessions", s.handleChatSessions)
	s.logger.Info("Registered: /api/chat/sessions")
	s.mux.HandleFunc("/api/chat/sessions/", s.handleChatSession)
	s.logger.Info("Registered: /api/chat/sessions/")
	s.mux.HandleFunc("/ws/chat/", s.handleChatWebSocket)
	s.logger.Info("Registered: /ws/chat/")
	s.mux.HandleFunc("/api/chat/providers", s.handleChatProviders)
	s.logger.Info("Registered: /api/chat/providers")
	s.mux.HandleFunc("/api/chat/mcp-servers", s.handleMCPServers)
	s.logger.Info("Registered: /api/chat/mcp-servers")
	s.mux.HandleFunc("/api/chat/sessions/system-prompt", s.handleSystemPrompt)
	s.logger.Info("Registered: /api/chat/sessions/system-prompt")

	s.mux.HandleFunc("/api/internal/task-output", s.handleTaskOutput)
	s.logger.Info("Registered: /api/internal/task-output")
	s.mux.HandleFunc("/api/internal/chat/sessions/", s.handleGetChatContext)
	s.logger.Info("Registered: /api/internal/chat/sessions/")
}

func (s *DashboardServer) handleChatSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createChatSession(w, r)
	case http.MethodGet:
		s.listChatSessions(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *DashboardServer) createChatSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)

		return
	}

	if req.Provider == "" {
		req.Provider = "openrouter"
	}

	if req.Model == "" {
		req.Model = "z-ai/glm-4.6"
	}

	if req.UserID == "" {
		req.UserID = "default"
	}

	if req.Title == "" {
		req.Title = "New Chat"
	}

	session, err := s.chatService.CreateSession(req.UserID, req.Provider, req.Model)
	if err != nil {
		s.logger.Error("Failed to create chat session: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create session: %v", err), http.StatusInternalServerError)

		return
	}

	if req.Title != "" && req.Title != "New Chat" {
		updates := map[string]interface{}{"title": req.Title}
		if err := s.chatService.UpdateSession(session.ID, updates); err != nil {
			s.logger.Error("Failed to set session title: %v", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *DashboardServer) listChatSessions(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = "default"
	}

	sessions, err := s.chatService.ListSessions(userID)
	if err != nil {
		s.logger.Error("Failed to list chat sessions: %v", err)
		http.Error(w, fmt.Sprintf("Failed to list sessions: %v", err), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sessions)
}

func (s *DashboardServer) handleChatSession(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)

		return
	}

	sessionID := parts[0]

	switch r.Method {
	case http.MethodGet:
		if len(parts) > 1 && parts[1] == "system-prompt" {
			s.getSessionSystemPrompt(w, r, sessionID)
		} else {
			s.getChatSession(w, r, sessionID)
		}
	case http.MethodDelete:
		s.deleteChatSession(w, r, sessionID)
	case http.MethodPatch:
		s.updateChatSession(w, r, sessionID)
	case http.MethodPost:
		if len(parts) > 1 && parts[1] == "messages" {
			s.sendChatMessage(w, r, sessionID)
		} else {
			http.Error(w, "Invalid endpoint", http.StatusNotFound)
		}
	case http.MethodPut:
		if len(parts) > 1 && parts[1] == "mcp-servers" {
			s.setSessionMCPServers(w, r, sessionID)
		} else {
			http.Error(w, "Invalid endpoint", http.StatusNotFound)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *DashboardServer) getChatSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	session, err := s.chatService.GetSession(sessionID)
	if err != nil {
		s.logger.Error("Failed to get chat session (session_id=%s): %v", sessionID, err)
		http.Error(w, fmt.Sprintf("Failed to get session: %v", err), http.StatusInternalServerError)

		return
	}

	if s.chatService.Storage != nil {
		ctx := r.Context()
		messages, err := s.chatService.Storage.GetMessages(ctx, sessionID, 100)
		if err == nil && len(messages) > 0 {
			session.Messages = make([]ChatMessage, len(messages))
			for i, msg := range messages {
				session.Messages[i] = *msg
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

func (s *DashboardServer) deleteChatSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	if err := s.chatService.DeleteSession(sessionID); err != nil {
		s.logger.Error("Failed to delete chat session (session_id=%s): %v", sessionID, err)
		http.Error(w, fmt.Sprintf("Failed to delete session: %v", err), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *DashboardServer) updateChatSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req UpdateSessionRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)

		return
	}

	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Provider != "" {
		updates["provider"] = req.Provider
	}
	if req.Model != "" {
		updates["model"] = req.Model
	}

	if err := s.chatService.UpdateSession(sessionID, updates); err != nil {
		s.logger.Error("Failed to update chat session (session_id=%s): %v", sessionID, err)
		http.Error(w, fmt.Sprintf("Failed to update session: %v", err), http.StatusInternalServerError)

		return
	}

	if req.MCPServers != nil {
		s.logger.Info("Updating MCP servers for session %s: %v", sessionID, req.MCPServers)
		if err := s.chatService.SetSessionMCPServers(sessionID, req.MCPServers); err != nil {
			s.logger.Error("Failed to set session MCP servers (session_id=%s): %v", sessionID, err)
			http.Error(w, fmt.Sprintf("Failed to set MCP servers: %v", err), http.StatusInternalServerError)

			return
		}
		s.logger.Info("Successfully updated MCP servers for session %s", sessionID)
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *DashboardServer) sendChatMessage(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req SendMessageRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)

		return
	}

	if req.Message == "" {
		http.Error(w, "Message cannot be empty", http.StatusBadRequest)

		return
	}

	responseCh, err := s.chatService.SendMessage(sessionID, req.Message, false)
	if err != nil {
		s.logger.Error("Failed to send chat message (session_id=%s): %v", sessionID, err)
		http.Error(w, fmt.Sprintf("Failed to send message: %v", err), http.StatusInternalServerError)

		return
	}

	var fullResponse strings.Builder
	for chunk := range responseCh {
		fullResponse.WriteString(chunk)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"response": fullResponse.String(),
	})
}

func (s *DashboardServer) handleChatWebSocket(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/ws/chat/")
	sessionID := strings.TrimSuffix(path, "/")

	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)

		return
	}

	_, err := s.chatService.GetSession(sessionID)
	if err != nil {
		s.logger.Error("WebSocket connection rejected: session %s not found: %v", sessionID, err)
		http.Error(w, "Session not found", http.StatusNotFound)

		return
	}

	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade WebSocket connection: %v", err)

		return
	}

	safeConn := &SafeWebSocketConn{conn: conn}
	s.logger.Info("Chat WebSocket connected for session: %s", sessionID)

	clientChan := make(chan StreamChunk, 100)
	done := make(chan struct{})

	client := &chatClient{
		conn:      safeConn,
		send:      clientChan,
		sessionID: sessionID,
	}

	s.chatBroadcaster.register <- client

	defer func() {
		s.chatBroadcaster.unregister <- client
		close(done)
		s.logger.Info("Chat WebSocket disconnected for session: %s", sessionID)
	}()

	go s.chatReadPump(safeConn, sessionID, s.chatBroadcaster, done)
	s.chatWritePump(safeConn, clientChan, done)
}

func (s *DashboardServer) streamChatResponse(conn *websocket.Conn, sessionID, userMessage string) error {
	streamCh, err := s.chatService.SendMessage(sessionID, userMessage, true)
	if err != nil {
		errMsg := StreamChunk{
			Type:  "error",
			Error: fmt.Sprintf("%v", err),
			Done:  true,
		}
		conn.WriteJSON(errMsg)

		return fmt.Errorf("failed to start streaming: %w", err)
	}

	messageID := ""
	var allToolCalls []ToolCall
	var allToolResults []ToolCall

	for chunk := range streamCh {
		if strings.HasPrefix(chunk, "__TOOL_CALLS__") {
			toolCallsJSON := strings.TrimPrefix(chunk, "__TOOL_CALLS__")
			var toolCalls []ToolCall
			if err := json.Unmarshal([]byte(toolCallsJSON), &toolCalls); err != nil {
				s.logger.Error("Failed to unmarshal tool calls: %v", err)
			} else {
				s.logger.Info("Captured %d tool calls (total: %d)", len(toolCalls), len(allToolCalls)+len(toolCalls))
				allToolCalls = append(allToolCalls, toolCalls...)

				liveToolMsg := StreamChunk{
					Type:      "chunk",
					ToolCalls: toolCalls,
					Done:      false,
				}
				if err := conn.WriteJSON(liveToolMsg); err != nil {
					s.logger.Error("Failed to send live tool calls: %v", err)
				}
			}

			continue
		}

		if strings.HasPrefix(chunk, "__TOOL_RESULTS__") {
			toolResultsJSON := strings.TrimPrefix(chunk, "__TOOL_RESULTS__")
			var toolResults []ToolCall
			if err := json.Unmarshal([]byte(toolResultsJSON), &toolResults); err != nil {
				s.logger.Error("Failed to unmarshal tool results: %v", err)
			} else {
				s.logger.Info("Captured %d tool results (total: %d)", len(toolResults), len(allToolResults)+len(toolResults))
				allToolResults = append(allToolResults, toolResults...)

				liveResultMsg := StreamChunk{
					Type:        "chunk",
					ToolResults: toolResults,
					Done:        false,
				}
				if err := conn.WriteJSON(liveResultMsg); err != nil {
					s.logger.Error("Failed to send live tool results: %v", err)
				}
			}

			continue
		}

		streamMsg := StreamChunk{
			Type:    "chunk",
			Content: chunk,
			Done:    false,
		}

		if err := conn.WriteJSON(streamMsg); err != nil {
			return fmt.Errorf("failed to send chunk: %w", err)
		}
	}

	s.logger.Info("Sending done message with %d tool calls and %d tool results", len(allToolCalls), len(allToolResults))

	session, err := s.chatService.GetSession(sessionID)
	if err == nil && len(session.Messages) > 0 {
		lastMsg := session.Messages[len(session.Messages)-1]
		if lastMsg.Role == "assistant" {
			messageID = lastMsg.ID
		}
	}

	doneMsg := StreamChunk{
		Type:        "chunk",
		MessageID:   messageID,
		Content:     "",
		ToolCalls:   allToolCalls,
		ToolResults: allToolResults,
		Done:        true,
	}

	if err := conn.WriteJSON(doneMsg); err != nil {
		return fmt.Errorf("failed to send done message: %w", err)
	}

	s.logger.Debug("Chat message streamed successfully (session_id=%s)", sessionID)

	return nil
}

func (s *DashboardServer) sendWSError(conn *websocket.Conn, errorMsg string) {
	errChunk := StreamChunk{
		Type:  "error",
		Error: errorMsg,
		Done:  true,
	}

	conn.WriteJSON(errChunk)
}

func (s *DashboardServer) handleChatProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	if s.chatService == nil {
		http.Error(w, "Chat service not available", http.StatusServiceUnavailable)

		return
	}

	providers := s.chatService.GetAvailableProviders()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(providers)
}

func (s *DashboardServer) handleMCPServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Method not allowed",
		})

		return
	}

	if s.chatService == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "Chat service not available",
			"servers": []interface{}{},
		})

		return
	}

	servers, err := s.chatService.GetAvailableMCPServers()
	if err != nil {
		s.logger.Error("Failed to get MCP servers: %v", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode([]interface{}{})

		return
	}

	if servers == nil {
		servers = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(servers)
}

func (s *DashboardServer) setSessionMCPServers(w http.ResponseWriter, r *http.Request, sessionID string) {
	var req struct {
		MCPServers []string `json:"mcp_servers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)

		return
	}

	if err := s.chatService.SetSessionMCPServers(sessionID, req.MCPServers); err != nil {
		s.logger.Error("Failed to set session MCP servers (session_id=%s): %v", sessionID, err)
		http.Error(w, fmt.Sprintf("Failed to set MCP servers: %v", err), http.StatusInternalServerError)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *DashboardServer) getSessionSystemPrompt(w http.ResponseWriter, r *http.Request, sessionID string) {
	systemPrompt := s.chatService.BuildSystemContextForSession(sessionID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"system_prompt": systemPrompt,
	})
}

func (s *DashboardServer) handleSystemPrompt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/chat/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) < 2 || parts[0] == "" || parts[1] != "system-prompt" {
		http.Error(w, "Invalid endpoint", http.StatusBadRequest)

		return
	}

	sessionID := parts[0]
	s.getSessionSystemPrompt(w, r, sessionID)
}

func (s *DashboardServer) handleTaskOutput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	var payload struct {
		SessionID     string `json:"session_id"`
		Role          string `json:"role"`
		Content       string `json:"content"`
		IsAutomated   bool   `json:"is_automated"`
		FromTaskRunID string `json:"from_task_run_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)

		return
	}

	ctx := r.Context()

	msg := &ChatMessage{
		ID:            uuid.New().String(),
		SessionID:     payload.SessionID,
		Role:          payload.Role,
		Content:       payload.Content,
		IsAutomated:   payload.IsAutomated,
		FromTaskRunID: payload.FromTaskRunID,
		CreatedAt:     time.Now(),
	}

	if err := s.chatService.Storage.AddMessage(ctx, msg); err != nil {
		s.logger.Error("Failed to save task output message: %v", err)
		http.Error(w, "Failed to save message", http.StatusInternalServerError)

		return
	}

	session, err := s.chatService.GetSession(payload.SessionID)
	if err == nil {
		session.Messages = append(session.Messages, *msg)
	}

	if err := s.chatService.Storage.IncrementUnreadCount(ctx, payload.SessionID); err != nil {
		s.logger.Warning("Failed to increment unread count: %v", err)
	}

	if s.chatBroadcaster != nil {
		s.chatBroadcaster.BroadcastToSession(payload.SessionID, "new_message", msg)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":     "ok",
		"message_id": msg.ID,
	})
}

func (s *DashboardServer) handleGetChatContext(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/internal/chat/sessions/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)

		return
	}

	sessionID := parts[0]

	if len(parts) < 2 || parts[1] != "context" {
		http.Error(w, "Invalid endpoint - expected /context", http.StatusBadRequest)

		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	ctx := r.Context()
	messages, err := s.chatService.Storage.GetMessages(ctx, sessionID, limit)
	if err != nil {
		http.Error(w, "Failed to fetch messages", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

func (s *DashboardServer) broadcastToSession(sessionID string, message map[string]interface{}) {
	msgJSON, err := json.Marshal(message)
	if err != nil {
		s.logger.Error("Failed to marshal broadcast message: %v", err)

		return
	}

	activity := ActivityMessage{
		ID:        uuid.New().String(),
		Timestamp: time.Now().Format(time.RFC3339),
		Level:     "info",
		Type:      "chat",
		Message:   string(msgJSON),
		Details: map[string]interface{}{
			"session_id": sessionID,
		},
	}

	select {
	case activityBroadcaster.broadcast <- activity:
	default:
		s.logger.Warning("Failed to broadcast message - channel full")
	}
}