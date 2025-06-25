// internal/server/notification_integration.go
package server

import (
	"encoding/json"
	"mcpcompose/internal/protocol"
	"net/http"
	"strings"
	"time"
)

// handleResourceSubscribe handles resources/subscribe requests
func (h *ProxyHandler) handleResourceSubscribe(w http.ResponseWriter, r *http.Request, _ string, requestPayload map[string]interface{}) {
	reqIDVal := requestPayload["id"]

	// Parse subscribe request
	paramsData, _ := json.Marshal(requestPayload["params"])
	var subscribeReq protocol.SubscribeRequest
	if err := json.Unmarshal(paramsData, &subscribeReq); err != nil {
		h.sendMCPError(w, reqIDVal, protocol.InvalidParams, "Invalid subscribe parameters", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Get client ID from request context or session
	clientID := h.getClientID(r)
	sessionID := r.Header.Get("Mcp-Session-Id")

	// Create notification function
	notifyFunc := func(notification *protocol.ResourceUpdateNotification) error {
		return h.sendNotificationToClient(clientID, notification)
	}

	// Subscribe to resource changes
	response, err := h.subscriptionManager.Subscribe(clientID, sessionID, subscribeReq, notifyFunc)
	if err != nil {
		h.sendMCPError(w, reqIDVal, protocol.ValidationError, err.Error())
		return
	}

	// Send success response
	successResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqIDVal,
		"result":  response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(successResponse)
}

// handleResourceUnsubscribe handles resources/unsubscribe requests
func (h *ProxyHandler) handleResourceUnsubscribe(w http.ResponseWriter, r *http.Request, _ string, requestPayload map[string]interface{}) {
	reqIDVal := requestPayload["id"]

	// Parse unsubscribe request
	paramsData, _ := json.Marshal(requestPayload["params"])
	var unsubscribeReq protocol.UnsubscribeRequest
	if err := json.Unmarshal(paramsData, &unsubscribeReq); err != nil {
		h.sendMCPError(w, reqIDVal, protocol.InvalidParams, "Invalid unsubscribe parameters", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Get client ID
	clientID := h.getClientID(r)

	// Unsubscribe from resource changes
	if err := h.subscriptionManager.Unsubscribe(clientID, unsubscribeReq); err != nil {
		h.sendMCPError(w, reqIDVal, protocol.ValidationError, err.Error())
		return
	}

	// Send success response
	successResponse := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      reqIDVal,
		"result":  map[string]interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(successResponse)
}

// Helper methods

func (h *ProxyHandler) getClientID(r *http.Request) string {
	// Try various methods to identify the client
	if sessionID := r.Header.Get("Mcp-Session-Id"); sessionID != "" {
		return sessionID
	}

	if clientID := r.Header.Get("X-Client-ID"); clientID != "" {
		return clientID
	}

	// Fallback to remote address
	return strings.Split(r.RemoteAddr, ":")[0]
}

func (h *ProxyHandler) supportsNotifications(r *http.Request) bool {
	// Check if client supports notifications
	capabilities := r.Header.Get("X-MCP-Capabilities")
	return strings.Contains(capabilities, "notifications") ||
		r.Header.Get("X-Supports-Notifications") == "true"
}

func (h *ProxyHandler) sendNotificationToClient(clientID string, notification *protocol.ResourceUpdateNotification) error {
	// Implementation depends on your transport mechanism
	// For HTTP, you might need WebSocket or Server-Sent Events
	// For now, log the notification
	h.logger.Info("Would send notification to client %s: %+v", clientID, notification)
	return nil
}

func (h *ProxyHandler) sendChangeNotificationToClient(clientID string, notification *protocol.ChangeNotification) error {
	// Implementation depends on your transport mechanism
	h.logger.Info("Would send change notification to client %s: %+v", clientID, notification)
	return nil
}

// Initialize notification support
func (h *ProxyHandler) initializeNotificationSupport() {
	// Managers are already initialized in NewProxyHandler
	// Start cleanup routine
	go h.startNotificationCleanup()
}

func (h *ProxyHandler) startNotificationCleanup() {
	// Cleanup inactive subscriptions every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.subscriptionManager.CleanupExpiredSubscriptions(30 * time.Minute)
			h.changeNotificationManager.CleanupInactiveSubscribers(30 * time.Minute)
		case <-h.ctx.Done():
			return
		}
	}
}
