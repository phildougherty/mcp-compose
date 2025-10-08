package workflow

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketHandler struct {
	hub     *Hub
	storage *Storage
	logger  *logging.Logger
}

func NewWebSocketHandler(hub *Hub, storage *Storage, logger *logging.Logger) *WebSocketHandler {
	return &WebSocketHandler{
		hub:     hub,
		storage: storage,
		logger:  logger,
	}
}

func (wsh *WebSocketHandler) HandleWorkflowExecutionWebSocket(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/ws/workflows/"), "/")
	if len(parts) < 3 || parts[1] != "executions" {
		http.Error(w, "Invalid WebSocket path", http.StatusBadRequest)
		return
	}

	workflowID := parts[0]
	executionID := parts[2]

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		wsh.logger.Error("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	execution, err := wsh.storage.GetExecution(ctx, executionID)
	if err != nil {
		wsh.logger.Error("Failed to get execution for WebSocket: %v", err)
		conn.WriteJSON(map[string]string{
			"error": "Execution not found",
		})
		conn.Close()
		return
	}

	if execution.WorkflowID != workflowID {
		wsh.logger.Error("Execution %s does not belong to workflow %s", executionID, workflowID)
		conn.WriteJSON(map[string]string{
			"error": "Invalid workflow/execution combination",
		})
		conn.Close()
		return
	}

	client := wsh.hub.RegisterClient(conn, executionID)

	initialUpdate := ExecutionUpdate{
		Type:        "execution_state",
		ExecutionID: execution.ID,
		WorkflowID:  execution.WorkflowID,
		Status:      execution.Status,
		Timestamp:   time.Now().Format(time.RFC3339Nano),
	}

	if err := client.writeJSON(initialUpdate); err != nil {
		wsh.logger.Error("Failed to send initial state: %v", err)
		wsh.hub.UnregisterClient(client)
		return
	}

	go wsh.writePump(client)
	wsh.readPump(client)
}

func (wsh *WebSocketHandler) readPump(client *wsClient) {
	defer func() {
		wsh.hub.UnregisterClient(client)
		client.close()
	}()

	client.conn.SetReadDeadline(time.Time{})
	client.conn.SetPongHandler(func(string) error {
		return nil
	})

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				wsh.logger.Debug("WebSocket closed normally for execution: %s", client.executionID)
			} else {
				wsh.logger.Error("WebSocket read error for execution %s: %v", client.executionID, err)
			}
			break
		}
	}
}

func (wsh *WebSocketHandler) writePump(client *wsClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		client.close()
	}()

	for {
		select {
		case update, ok := <-client.send:
			if !ok {
				client.setWriteDeadline(time.Now().Add(10 * time.Second))
				client.writeMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.setWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				wsh.logger.Error("Failed to set write deadline: %v", err)
				return
			}

			if err := client.writeJSON(update); err != nil {
				wsh.logger.Error("Failed to write update: %v", err)
				return
			}

		case <-ticker.C:
			if err := client.setWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				wsh.logger.Error("Failed to set write deadline for ping: %v", err)
				return
			}

			if err := client.writeMessage(websocket.PingMessage, nil); err != nil {
				wsh.logger.Error("Failed to send ping: %v", err)
				return
			}
		}
	}
}
