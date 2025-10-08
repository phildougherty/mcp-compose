package workflow

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type ExecutionUpdate struct {
	Type        string                 `json:"type"`
	ExecutionID string                 `json:"execution_id"`
	WorkflowID  string                 `json:"workflow_id"`
	NodeID      string                 `json:"node_id,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Output      map[string]interface{} `json:"output,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Duration    int64                  `json:"duration,omitempty"`
	Timestamp   string                 `json:"timestamp"`
}

type wsClient struct {
	conn        *websocket.Conn
	send        chan ExecutionUpdate
	executionID string
	mu          sync.Mutex
}

func (c *wsClient) writeJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.WriteJSON(v)
}

func (c *wsClient) writeMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.WriteMessage(messageType, data)
}

func (c *wsClient) setWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.SetWriteDeadline(t)
}

func (c *wsClient) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn.Close()
}

type Hub struct {
	clients    map[string]map[*wsClient]bool
	register   chan *wsClient
	unregister chan *wsClient
	broadcast  chan ExecutionUpdate
	logger     *logging.Logger
	mu         sync.RWMutex
}

func NewHub(logger *logging.Logger) *Hub {
	return &Hub{
		clients:    make(map[string]map[*wsClient]bool),
		register:   make(chan *wsClient, 10),
		unregister: make(chan *wsClient, 10),
		broadcast:  make(chan ExecutionUpdate, 100),
		logger:     logger,
	}
}

func (h *Hub) Start() {
	go h.run()
}

func (h *Hub) run() {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.executionID] == nil {
				h.clients[client.executionID] = make(map[*wsClient]bool)
			}
			h.clients[client.executionID][client] = true
			h.mu.Unlock()

			h.logger.Info("WebSocket client registered for execution: %s", client.executionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.executionID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.executionID)
					}
				}
			}
			h.mu.Unlock()

			h.logger.Info("WebSocket client unregistered for execution: %s", client.executionID)

		case update := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[update.ExecutionID]
			h.mu.RUnlock()

			for client := range clients {
				select {
				case client.send <- update:
				default:
					h.logger.Warning("Client send channel full for execution %s, closing connection", update.ExecutionID)
					h.unregister <- client
				}
			}

		case <-pingTicker.C:
			h.mu.RLock()
			for _, clients := range h.clients {
				for client := range clients {
					go func(c *wsClient) {
						if err := c.setWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
							h.logger.Debug("Failed to set write deadline for ping: %v", err)
							return
						}
						if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
							h.logger.Debug("Failed to send ping: %v", err)
							h.unregister <- c
						}
					}(client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) BroadcastUpdate(update ExecutionUpdate) {
	select {
	case h.broadcast <- update:
	default:
		h.logger.Warning("Broadcast channel full, dropping update for execution: %s", update.ExecutionID)
	}
}

func (h *Hub) RegisterClient(conn *websocket.Conn, executionID string) *wsClient {
	client := &wsClient{
		conn:        conn,
		send:        make(chan ExecutionUpdate, 10),
		executionID: executionID,
	}

	h.register <- client

	return client
}

func (h *Hub) UnregisterClient(client *wsClient) {
	h.unregister <- client
}

func (h *Hub) ClientCount(executionID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[executionID]; ok {
		return len(clients)
	}

	return 0
}

func (h *Hub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, clients := range h.clients {
		for client := range clients {
			close(client.send)
			client.close()
		}
	}

	h.clients = make(map[string]map[*wsClient]bool)
}
