// internal/protocol/websocket.go
package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport implements MCP transport over WebSocket (updated to be MCP compliant)
type WebSocketTransport struct {
	conn            *websocket.Conn
	url             string
	readChan        chan MCPMessage
	writeChan       chan MCPMessage
	errorChan       chan error
	mu              sync.RWMutex
	closed          bool
	progressManager *ProgressManager
	lastUsed        time.Time
	initialized     bool
	healthy         bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// WebSocketServer manages WebSocket connections for MCP
type WebSocketServer struct {
	upgrader    websocket.Upgrader
	connections map[string]*WebSocketTransport
	handlers    map[string]WebSocketHandler
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// WebSocketHandler defines the interface for handling WebSocket MCP connections
type WebSocketHandler interface {
	OnConnect(transport *WebSocketTransport) error
	OnDisconnect(transport *WebSocketTransport) error
	OnMessage(transport *WebSocketTransport, message MCPMessage) error
}

// WebSocketConnection represents a WebSocket MCP connection
type WebSocketConnection struct {
	ID           string
	Transport    *WebSocketTransport
	ClientInfo   *ClientInfo
	ServerInfo   *ServerInfo
	Capabilities *CapabilitiesOpts
	Initialized  bool
	LastActivity time.Time
	Context      map[string]interface{}
}

// NewWebSocketTransport creates a WebSocket transport from a URL
func NewWebSocketTransport(url string) *WebSocketTransport {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketTransport{
		url:             url,
		readChan:        make(chan MCPMessage, 100),
		writeChan:       make(chan MCPMessage, 100),
		errorChan:       make(chan error, 10),
		progressManager: NewProgressManager(),
		lastUsed:        time.Now(),
		healthy:         true,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// NewWebSocketServer creates a new WebSocket MCP server
func NewWebSocketServer() *WebSocketServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &WebSocketServer{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				// In production, implement proper origin checking
				return true
			},
		},
		connections: make(map[string]*WebSocketTransport),
		handlers:    make(map[string]WebSocketHandler),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// GetType returns the transport type
func (wst *WebSocketTransport) GetType() string {
	return "websocket"
}

// IsConnected returns true if transport is connected
func (wst *WebSocketTransport) IsConnected() bool {
	wst.mu.RLock()
	defer wst.mu.RUnlock()
	return wst.healthy && wst.initialized && !wst.closed
}

// GetLastActivity returns the last activity timestamp
func (wst *WebSocketTransport) GetLastActivity() time.Time {
	wst.mu.RLock()
	defer wst.mu.RUnlock()
	return wst.lastUsed
}

// Start starts the WebSocket transport
func (wst *WebSocketTransport) Start() error {
	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wst.url, nil)
	if err != nil {
		return fmt.Errorf("WebSocket dial failed: %w", err)
	}

	wst.mu.Lock()
	wst.conn = conn
	wst.initialized = true
	wst.healthy = true
	wst.mu.Unlock()

	// Start read goroutine
	go wst.readLoop()
	// Start write goroutine
	go wst.writeLoop()

	// Set up ping/pong handlers
	wst.conn.SetPongHandler(func(string) error {
		return wst.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	return nil
}

// Send implements the Transport interface
func (wst *WebSocketTransport) Send(msg MCPMessage) error {
	wst.mu.RLock()
	if wst.closed {
		wst.mu.RUnlock()
		return fmt.Errorf("transport is closed")
	}
	wst.mu.RUnlock()

	select {
	case wst.writeChan <- msg:
		wst.mu.Lock()
		wst.lastUsed = time.Now()
		wst.mu.Unlock()
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout")
	case <-wst.ctx.Done():
		return fmt.Errorf("transport closed")
	}
}

// Receive implements the Transport interface
func (wst *WebSocketTransport) Receive() (MCPMessage, error) {
	select {
	case msg := <-wst.readChan:
		wst.mu.Lock()
		wst.lastUsed = time.Now()
		wst.mu.Unlock()
		return msg, nil
	case err := <-wst.errorChan:
		return MCPMessage{}, err
	case <-wst.ctx.Done():
		return MCPMessage{}, fmt.Errorf("transport closed")
	}
}

// Close implements the Transport interface
func (wst *WebSocketTransport) Close() error {
	wst.mu.Lock()
	defer wst.mu.Unlock()

	if wst.closed {
		return nil
	}
	wst.closed = true
	wst.healthy = false

	if wst.cancel != nil {
		wst.cancel()
	}

	return wst.conn.Close()
}

// SupportsProgress implements the Transport interface
func (wst *WebSocketTransport) SupportsProgress() bool {
	return true
}

// SendProgress implements the Transport interface
func (wst *WebSocketTransport) SendProgress(notification *ProgressNotification) error {
	// Convert to generic message
	msg := MCPMessage{
		JSONRPC: notification.JSONRPC,
		Method:  notification.Method,
	}
	params, err := json.Marshal(notification.Params)
	if err != nil {
		return err
	}
	msg.Params = params

	return wst.Send(msg)
}

// readLoop reads messages from the WebSocket connection
func (wst *WebSocketTransport) readLoop() {
	defer wst.Close()
	for {
		select {
		case <-wst.ctx.Done():
			return
		default:
		}

		wst.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		var msg MCPMessage
		err := wst.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				wst.errorChan <- fmt.Errorf("websocket read error: %w", err)
			}
			return
		}

		// Validate message
		if err := ValidateMessage(msg); err != nil {
			wst.errorChan <- fmt.Errorf("invalid message: %w", err)
			continue
		}

		select {
		case wst.readChan <- msg:
		case <-wst.ctx.Done():
			return
		}
	}
}

// writeLoop writes messages to the WebSocket connection
func (wst *WebSocketTransport) writeLoop() {
	ticker := time.NewTicker(54 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-wst.writeChan:
			wst.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := wst.conn.WriteJSON(msg); err != nil {
				wst.errorChan <- fmt.Errorf("websocket write error: %w", err)
				return
			}
		case <-ticker.C:
			// Send ping
			wst.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := wst.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-wst.ctx.Done():
			return
		}
	}
}

// UpgradeHTTP upgrades an HTTP connection to WebSocket MCP
func (ws *WebSocketServer) UpgradeHTTP(w http.ResponseWriter, r *http.Request, handler WebSocketHandler) (*WebSocketTransport, error) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, fmt.Errorf("websocket upgrade failed: %w", err)
	}

	// Create transport with the established connection
	ctx, cancel := context.WithCancel(ws.ctx)
	transport := &WebSocketTransport{
		conn:            conn,
		readChan:        make(chan MCPMessage, 100),
		writeChan:       make(chan MCPMessage, 100),
		errorChan:       make(chan error, 10),
		progressManager: NewProgressManager(),
		lastUsed:        time.Now(),
		healthy:         true,
		initialized:     true,
		ctx:             ctx,
		cancel:          cancel,
	}

	connectionID := fmt.Sprintf("ws_%d", time.Now().UnixNano())
	ws.mu.Lock()
	ws.connections[connectionID] = transport
	ws.mu.Unlock()

	// Start transport goroutines
	go transport.readLoop()
	go transport.writeLoop()

	// Call connection handler
	if err := handler.OnConnect(transport); err != nil {
		transport.Close()
		ws.mu.Lock()
		delete(ws.connections, connectionID)
		ws.mu.Unlock()
		return nil, fmt.Errorf("connection handler failed: %w", err)
	}

	// Start message handling
	go ws.handleConnection(connectionID, transport, handler)

	return transport, nil
}

// handleConnection handles messages for a WebSocket connection
func (ws *WebSocketServer) handleConnection(connectionID string, transport *WebSocketTransport, handler WebSocketHandler) {
	defer func() {
		handler.OnDisconnect(transport)
		ws.mu.Lock()
		delete(ws.connections, connectionID)
		ws.mu.Unlock()
		transport.Close()
	}()

	for {
		select {
		case <-ws.ctx.Done():
			return
		case <-transport.ctx.Done():
			return
		default:
		}

		msg, err := transport.Receive()
		if err != nil {
			return
		}

		if err := handler.OnMessage(transport, msg); err != nil {
			// Send error response if message had an ID
			if msg.ID != nil {
				errorResponse := MCPMessage{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error:   NewInternalError(err.Error()),
				}
				transport.Send(errorResponse)
			}
		}
	}
}

// Close closes all connections and shuts down the server
func (ws *WebSocketServer) Close() error {
	ws.cancel()
	ws.mu.Lock()
	for _, transport := range ws.connections {
		transport.Close()
	}
	ws.connections = make(map[string]*WebSocketTransport)
	ws.mu.Unlock()
	return nil
}

// GetConnections returns all active connections
func (ws *WebSocketServer) GetConnections() map[string]*WebSocketTransport {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	result := make(map[string]*WebSocketTransport)
	for k, v := range ws.connections {
		result[k] = v
	}
	return result
}

// RegisterHandler registers a handler for WebSocket connections
func (ws *WebSocketServer) RegisterHandler(name string, handler WebSocketHandler) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.handlers[name] = handler
}

// GetHandler gets a registered handler
func (ws *WebSocketServer) GetHandler(name string) (WebSocketHandler, bool) {
	ws.mu.RLock()
	defer ws.mu.RUnlock()
	handler, exists := ws.handlers[name]
	return handler, exists
}
