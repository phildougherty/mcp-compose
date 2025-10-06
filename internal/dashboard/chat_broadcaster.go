package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type chatClient struct {
	conn      *SafeWebSocketConn
	send      chan StreamChunk
	sessionID string
}

type ChatBroadcaster struct {
	clients    map[*chatClient]bool
	register   chan *chatClient
	unregister chan *chatClient
	broadcast  chan StreamChunk
	logger     *logging.Logger
	mu         sync.RWMutex
}

func newChatBroadcaster() *ChatBroadcaster {
	return &ChatBroadcaster{
		clients:    make(map[*chatClient]bool),
		register:   make(chan *chatClient, 10),
		unregister: make(chan *chatClient, 10),
		broadcast:  make(chan StreamChunk, 100),
	}
}

func NewChatBroadcaster(logger *logging.Logger) *ChatBroadcaster {
	return &ChatBroadcaster{
		clients:    make(map[*chatClient]bool),
		register:   make(chan *chatClient, 10),
		unregister: make(chan *chatClient, 10),
		broadcast:  make(chan StreamChunk, 100),
		logger:     logger,
	}
}

func (cb *ChatBroadcaster) start() {
	go cb.run()
}

func (cb *ChatBroadcaster) run() {
	for {
		select {
		case client := <-cb.register:
			cb.mu.Lock()
			cb.clients[client] = true
			cb.mu.Unlock()

		case client := <-cb.unregister:
			cb.mu.Lock()
			if _, exists := cb.clients[client]; exists {
				delete(cb.clients, client)
				close(client.send)
			}
			cb.mu.Unlock()

		case message := <-cb.broadcast:
			cb.mu.RLock()
			for client := range cb.clients {
				select {
				case client.send <- message:
				default:
					if cb.logger != nil {
						cb.logger.Warning("Client channel full, removing dead connection (session=%s)", client.sessionID)
					}
					cb.mu.RUnlock()
					cb.mu.Lock()
					delete(cb.clients, client)
					close(client.send)
					cb.mu.Unlock()
					cb.mu.RLock()
				}
			}
			cb.mu.RUnlock()
		}
	}
}

func (cb *ChatBroadcaster) Stop() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	for client := range cb.clients {
		close(client.send)
	}
	cb.clients = make(map[*chatClient]bool)
}

func (cb *ChatBroadcaster) BroadcastToSession(sessionID, messageType string, message *ChatMessage) {
	chunk := StreamChunk{
		Type:      messageType,
		SessionID: sessionID,
		MessageID: message.ID,
		Role:      message.Role,
		Content:   message.Content,
		Done:      true,
	}

	if len(message.ToolCalls) > 0 {
		chunk.ToolCalls = message.ToolCalls
	}
	if len(message.ToolResults) > 0 {
		chunk.ToolResults = message.ToolResults
	}

	cb.BroadcastChunkToSession(sessionID, chunk)
}

func (cb *ChatBroadcaster) BroadcastChunkToSession(sessionID string, chunk StreamChunk) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	for client := range cb.clients {
		if client.sessionID == sessionID {
			select {
			case client.send <- chunk:
			default:
				if cb.logger != nil {
					cb.logger.Warning("Failed to send to client in session %s - channel full", sessionID)
				}
			}
		}
	}
}

func (s *DashboardServer) chatReadPump(conn *SafeWebSocketConn, sessionID string, broadcaster *ChatBroadcaster, done <-chan struct{}) {
	defer func() {
		s.logger.Debug("chatReadPump exiting for session: %s", sessionID)
	}()

	conn.conn.SetReadDeadline(time.Time{})

	for {
		select {
		case <-done:
			return
		default:
		}

		var msg WebSocketMessage
		if err := conn.conn.ReadJSON(&msg); err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				s.logger.Debug("Chat WebSocket closed normally for session: %s", sessionID)
			} else {
				s.logger.Error("Error reading chat WebSocket message for session %s: %v", sessionID, err)
			}

			return
		}

		s.logger.Debug("Received chat WebSocket message: type=%s, session=%s", msg.Type, sessionID)

		switch msg.Type {
		case "ping":
			pongChunk := StreamChunk{
				Type: "pong",
			}
			select {
			case broadcaster.broadcast <- pongChunk:
			case <-done:
				return
			}

		case "message":
			if msg.Message == "" {
				s.logger.Warning("Empty message received for session: %s", sessionID)

				continue
			}

			if s.chatService != nil && s.chatService.Storage != nil {
				userMsg := &ChatMessage{
					ID:        uuid.New().String(),
					SessionID: sessionID,
					Role:      "user",
					Content:   msg.Message,
					CreatedAt: time.Now(),
				}

				ctx := context.Background()
				if err := s.chatService.Storage.AddMessage(ctx, userMsg); err != nil {
					s.logger.Error("Failed to save user message to database (session_id=%s): %v", sessionID, err)
				} else {
					s.logger.Debug("User message saved to database (session_id=%s, message_id=%s)", sessionID, userMsg.ID)
				}
			}

			go s.streamChatResponseViaBroadcaster(broadcaster, sessionID, msg.Message, done)
		}
	}
}

func (s *DashboardServer) chatWritePump(conn *SafeWebSocketConn, clientChan <-chan StreamChunk, done <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			s.logger.Debug("chatWritePump received done signal")

			return

		case chunk, ok := <-clientChan:
			if !ok {
				s.logger.Debug("chatWritePump: client channel closed")

				return
			}

			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Error("Failed to set write deadline: %v", err)

				return
			}

			if err := conn.WriteJSON(chunk); err != nil {
				s.logger.Error("Failed to write chunk to WebSocket: %v", err)

				return
			}

		case <-ticker.C:
			if err := conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				s.logger.Error("Failed to set write deadline for ping: %v", err)

				return
			}

			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.logger.Error("Failed to send ping: %v", err)

				return
			}
		}
	}
}

func (s *DashboardServer) streamChatResponseViaBroadcaster(broadcaster *ChatBroadcaster, sessionID, userMessage string, done <-chan struct{}) {
	streamCh, err := s.chatService.SendMessage(sessionID, userMessage, true)
	if err != nil {
		errMsg := StreamChunk{
			Type:  "error",
			Error: fmt.Sprintf("%v", err),
			Done:  true,
		}
		broadcaster.BroadcastChunkToSession(sessionID, errMsg)

		return
	}

	messageID := ""
	var allToolCalls []ToolCall
	var allToolResults []ToolCall

	for chunk := range streamCh {
		select {
		case <-done:
			return
		default:
		}

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
				broadcaster.BroadcastChunkToSession(sessionID, liveToolMsg)
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
				broadcaster.BroadcastChunkToSession(sessionID, liveResultMsg)
			}

			continue
		}

		streamMsg := StreamChunk{
			Type:    "chunk",
			Content: chunk,
			Done:    false,
		}

		broadcaster.BroadcastChunkToSession(sessionID, streamMsg)
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

	broadcaster.BroadcastChunkToSession(sessionID, doneMsg)

	s.logger.Debug("Chat message streamed successfully (session_id=%s)", sessionID)
}
