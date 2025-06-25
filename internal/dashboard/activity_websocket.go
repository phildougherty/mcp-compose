package dashboard

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type ActivityMessage struct {
	ID        string                 `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Type      string                 `json:"type"` // request, connection, tool, error
	Server    string                 `json:"server,omitempty"`
	Client    string                 `json:"client,omitempty"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

type ActivityBroadcaster struct {
	clients       map[*SafeWebSocketConn]bool
	mu            sync.RWMutex
	register      chan *SafeWebSocketConn
	unregister    chan *SafeWebSocketConn
	broadcast     chan ActivityMessage
	shutdown      chan struct{}
	running       bool
	runMutex      sync.Mutex
	clientCounter int64
}

var activityBroadcaster = &ActivityBroadcaster{
	clients:    make(map[*SafeWebSocketConn]bool),
	register:   make(chan *SafeWebSocketConn, 10),
	unregister: make(chan *SafeWebSocketConn, 10),
	broadcast:  make(chan ActivityMessage, 1000), // Increased buffer
	shutdown:   make(chan struct{}),
}

func init() {
	activityBroadcaster.start()
}

// start ensures the broadcaster only runs once
func (ab *ActivityBroadcaster) start() {
	ab.runMutex.Lock()
	defer ab.runMutex.Unlock()

	if !ab.running {
		ab.running = true
		go ab.run()
		log.Println("[ACTIVITY] Activity broadcaster started")
	}
}

func (ab *ActivityBroadcaster) run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[ACTIVITY] Broadcaster panic recovered: %v", r)
			// Restart broadcaster after a brief delay
			time.Sleep(time.Second)
			ab.runMutex.Lock()
			ab.running = false
			ab.runMutex.Unlock()
			ab.start()
		}
	}()

	log.Println("[ACTIVITY] Activity broadcaster running")

	for {
		select {
		case client := <-ab.register:
			ab.handleClientRegistration(client)

		case client := <-ab.unregister:
			ab.handleClientUnregistration(client)

		case message := <-ab.broadcast:
			ab.handleBroadcast(message)

		case <-ab.shutdown:
			ab.handleShutdown()
			return
		}
	}
}

func (ab *ActivityBroadcaster) handleClientRegistration(client *SafeWebSocketConn) {
	ab.mu.Lock()
	ab.clients[client] = true
	ab.clientCounter++
	clientCount := len(ab.clients)
	clientID := ab.clientCounter
	ab.mu.Unlock()

	log.Printf("[ACTIVITY] ✅ Client #%d registered (total: %d)", clientID, clientCount)

	// Send immediate welcome message to confirm registration
	welcomeMsg := ActivityMessage{
		ID:        generateID(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     "INFO",
		Type:      "connection",
		Message:   fmt.Sprintf("Client #%d successfully registered to activity stream", clientID),
		Details: map[string]interface{}{
			"client_id":     clientID,
			"total_clients": clientCount,
		},
	}

	// Send welcome message directly (not through broadcast channel to avoid race)
	go func() {
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := client.WriteJSON(welcomeMsg); err != nil {
			log.Printf("[ACTIVITY] ❌ Failed to send welcome message to client #%d: %v", clientID, err)
		} else {
			log.Printf("[ACTIVITY] ✅ Welcome message sent to client #%d", clientID)
		}
	}()
}

func (ab *ActivityBroadcaster) handleClientUnregistration(client *SafeWebSocketConn) {
	ab.mu.Lock()
	if _, exists := ab.clients[client]; exists {
		delete(ab.clients, client)
		client.Close()
	}
	clientCount := len(ab.clients)
	ab.mu.Unlock()

	log.Printf("[ACTIVITY] ❌ Client unregistered (remaining: %d)", clientCount)
}

func (ab *ActivityBroadcaster) handleBroadcast(message ActivityMessage) {
	ab.mu.RLock()
	clientCount := len(ab.clients)
	ab.mu.RUnlock()

	if clientCount == 0 {
		log.Printf("[ACTIVITY] 📭 No clients to broadcast to: %s", message.Message)
		return
	}

	log.Printf("[ACTIVITY] 📢 Broadcasting to %d clients: %s", clientCount, message.Message)

	ab.mu.Lock()
	defer ab.mu.Unlock()

	sentCount := 0
	failedCount := 0

	for client := range ab.clients {
		if ab.sendToClient(client, message) {
			sentCount++
		} else {
			failedCount++
			delete(ab.clients, client)
		}
	}

	log.Printf("[ACTIVITY] 📊 Message delivered to %d/%d clients (%d failed)", sentCount, sentCount+failedCount, failedCount)
}

func (ab *ActivityBroadcaster) sendToClient(client *SafeWebSocketConn, message ActivityMessage) bool {
	// Use a timeout to prevent blocking
	done := make(chan bool, 1)

	go func() {
		client.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := client.WriteJSON(message)
		done <- (err == nil)
		if err != nil {
			log.Printf("[ACTIVITY] ❌ Failed to send to client: %v", err)
			client.Close()
		}
	}()

	select {
	case success := <-done:
		return success
	case <-time.After(3 * time.Second):
		log.Printf("[ACTIVITY] ⏰ Client send timeout, disconnecting slow client")
		client.Close()
		return false
	}
}

func (ab *ActivityBroadcaster) handleShutdown() {
	log.Println("[ACTIVITY] Shutting down broadcaster...")

	ab.mu.Lock()
	for client := range ab.clients {
		client.Close()
	}
	ab.clients = make(map[*SafeWebSocketConn]bool)
	ab.mu.Unlock()

	log.Println("[ACTIVITY] All clients disconnected")
}

// BroadcastActivity sends an activity message to all connected clients
func BroadcastActivity(level, activityType, server, client, message string, details map[string]interface{}) {
	activity := ActivityMessage{
		ID:        generateID(),
		Timestamp: time.Now().Format(time.RFC3339Nano),
		Level:     level,
		Type:      activityType,
		Server:    server,
		Client:    client,
		Message:   message,
		Details:   details,
	}

	jsonData, err := json.Marshal(activity)
	if err != nil {
		return
	}

	// Send to dashboard - adjust the URL/port as needed
	go func() {
		resp, err := http.Post("http://mcp-compose-dashboard:3001/api/activity", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[ACTIVITY] Failed to send to dashboard: %v", err)
			return
		}
		resp.Body.Close()
	}()
}

// Utility functions
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(6))
}

func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {
		// Fallback to time-based generation if crypto/rand fails
		for i := range bytes {
			bytes[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
	} else {
		for i, b := range bytes {
			bytes[i] = charset[b%byte(len(charset))]
		}
	}

	return string(bytes)
}
