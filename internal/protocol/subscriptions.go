// internal/protocol/subscriptions.go
package protocol

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SubscriptionManager manages MCP resource subscriptions
type SubscriptionManager struct {
	subscriptions map[string]*ResourceSubscription
	clients       map[string]*ClientSubscriptions
	mu            sync.RWMutex
}

// ClientSubscriptions tracks all subscriptions for a client
type ClientSubscriptions struct {
	ClientID      string
	SessionID     string
	Subscriptions map[string]*ResourceSubscription
	NotifyFunc    func(*ResourceUpdateNotification) error
	LastSeen      time.Time
}

// ResourceSubscription represents a subscription to resource changes
type ResourceSubscription struct {
	ID           string              `json:"id"`
	ClientID     string              `json:"clientId"`
	SessionID    string              `json:"sessionId"`
	URI          string              `json:"uri"`
	IsTemplate   bool                `json:"isTemplate"`
	Filters      []ResourceFilter    `json:"filters,omitempty"`
	Options      SubscriptionOptions `json:"options"`
	Created      time.Time           `json:"created"`
	LastNotified time.Time           `json:"lastNotified,omitempty"`
}

// ResourceFilter defines filtering criteria for subscriptions
type ResourceFilter struct {
	Type     string      `json:"type"`     // "glob", "regex", "exact", "prefix"
	Pattern  string      `json:"pattern"`  // Pattern to match
	Property string      `json:"property"` // Resource property to filter on
	Value    interface{} `json:"value"`    // Expected value
}

// SubscriptionOptions configures subscription behavior
type SubscriptionOptions struct {
	IncludeContent  bool     `json:"includeContent,omitempty"`  // Include resource content in notifications
	Debounce        int      `json:"debounce,omitempty"`        // Debounce period in milliseconds
	BatchSize       int      `json:"batchSize,omitempty"`       // Max resources per notification
	ContentTypes    []string `json:"contentTypes,omitempty"`    // Filter by content types
	MaxSize         int64    `json:"maxSize,omitempty"`         // Max resource size to include
	IncludeMetadata bool     `json:"includeMetadata,omitempty"` // Include resource metadata
}

// ResourceUpdateNotification represents a resource update notification
type ResourceUpdateNotification struct {
	JSONRPC string               `json:"jsonrpc"`
	Method  string               `json:"method"` // "notifications/resources/updated"
	Params  ResourceUpdateParams `json:"params"`
}

// ResourceUpdateParams contains the notification parameters
type ResourceUpdateParams struct {
	Resources []ResourceUpdate `json:"resources"`
	BatchInfo *BatchInfo       `json:"batchInfo,omitempty"`
}

// ResourceUpdate represents a single resource update
type ResourceUpdate struct {
	URI       string                 `json:"uri"`
	Type      string                 `json:"type"` // "created", "updated", "deleted"
	Content   *ResourceContent       `json:"content,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ResourceContent represents resource content
type ResourceContent struct {
	Type     string `json:"type"`           // "text", "blob"
	Data     string `json:"data,omitempty"` // Base64 for blob, direct for text
	MimeType string `json:"mimeType,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Encoding string `json:"encoding,omitempty"` // "utf-8", "base64"
}

// BatchInfo provides information about batched notifications
type BatchInfo struct {
	Total     int    `json:"total"`
	Current   int    `json:"current"`
	BatchID   string `json:"batchId"`
	LastBatch bool   `json:"lastBatch"`
}

// SubscribeRequest represents a resources/subscribe request
type SubscribeRequest struct {
	URI     string              `json:"uri"`
	Filters []ResourceFilter    `json:"filters,omitempty"`
	Options SubscriptionOptions `json:"options,omitempty"`
}

// SubscribeResponse represents a resources/subscribe response
type SubscribeResponse struct {
	SubscriptionID string `json:"subscriptionId"`
}

// UnsubscribeRequest represents a resources/unsubscribe request
type UnsubscribeRequest struct {
	SubscriptionID string `json:"subscriptionId"`
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager() *SubscriptionManager {

	return &SubscriptionManager{
		subscriptions: make(map[string]*ResourceSubscription),
		clients:       make(map[string]*ClientSubscriptions),
	}
}

// Subscribe creates a new resource subscription
func (sm *SubscriptionManager) Subscribe(clientID, sessionID string, req SubscribeRequest, notifyFunc func(*ResourceUpdateNotification) error) (*SubscribeResponse, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate URI
	if req.URI == "" {

		return nil, NewValidationError("uri", req.URI, "URI cannot be empty")
	}

	// Generate subscription ID
	subscriptionID := fmt.Sprintf("sub_%s_%d", clientID, time.Now().UnixNano())

	// Check if URI is a template
	isTemplate := sm.isURITemplate(req.URI)

	// Validate filters
	if err := sm.validateFilters(req.Filters); err != nil {

		return nil, err
	}

	// Create subscription
	subscription := &ResourceSubscription{
		ID:         subscriptionID,
		ClientID:   clientID,
		SessionID:  sessionID,
		URI:        req.URI,
		IsTemplate: isTemplate,
		Filters:    req.Filters,
		Options:    req.Options,
		Created:    time.Now(),
	}

	// Store subscription
	sm.subscriptions[subscriptionID] = subscription

	// Track client subscriptions
	if sm.clients[clientID] == nil {
		sm.clients[clientID] = &ClientSubscriptions{
			ClientID:      clientID,
			SessionID:     sessionID,
			Subscriptions: make(map[string]*ResourceSubscription),
			NotifyFunc:    notifyFunc,
			LastSeen:      time.Now(),
		}
	}

	sm.clients[clientID].Subscriptions[subscriptionID] = subscription
	sm.clients[clientID].LastSeen = time.Now()

	return &SubscribeResponse{
		SubscriptionID: subscriptionID,
	}, nil
}

// Unsubscribe removes a resource subscription
func (sm *SubscriptionManager) Unsubscribe(clientID string, req UnsubscribeRequest) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	subscription, exists := sm.subscriptions[req.SubscriptionID]
	if !exists {

		return NewValidationError("subscriptionId", req.SubscriptionID, "subscription not found")
	}

	// Verify ownership
	if subscription.ClientID != clientID {

		return NewAuthorizationError(req.SubscriptionID, "unsubscribe")
	}

	// Remove subscription
	delete(sm.subscriptions, req.SubscriptionID)

	// Remove from client tracking
	if client, exists := sm.clients[clientID]; exists {
		delete(client.Subscriptions, req.SubscriptionID)

		// Clean up empty client
		if len(client.Subscriptions) == 0 {
			delete(sm.clients, clientID)
		}
	}

	return nil
}

// NotifyResourceUpdate sends notifications to matching subscriptions
func (sm *SubscriptionManager) NotifyResourceUpdate(uri string, updateType string, content *ResourceContent, metadata map[string]interface{}) error {
	sm.mu.RLock()
	matchingSubscriptions := sm.findMatchingSubscriptions(uri)
	sm.mu.RUnlock()

	if len(matchingSubscriptions) == 0 {

		return nil // No subscribers
	}

	update := ResourceUpdate{
		URI:       uri,
		Type:      updateType,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
	}

	// Group by client for batching
	clientUpdates := make(map[string][]ResourceUpdate)

	for _, sub := range matchingSubscriptions {
		if sm.shouldIncludeUpdate(sub, update) {
			clientUpdates[sub.ClientID] = append(clientUpdates[sub.ClientID], update)
		}
	}

	// Send notifications
	for clientID, updates := range clientUpdates {
		if err := sm.sendNotificationToClient(clientID, updates); err != nil {
			// Log error but continue with other clients
			fmt.Printf("Failed to notify client %s: %v\n", clientID, err)
		}
	}

	return nil
}

// findMatchingSubscriptions finds all subscriptions that match a URI
func (sm *SubscriptionManager) findMatchingSubscriptions(uri string) []*ResourceSubscription {
	var matches []*ResourceSubscription

	for _, subscription := range sm.subscriptions {
		if sm.doesURIMatch(subscription, uri) {
			matches = append(matches, subscription)
		}
	}

	return matches
}

// doesURIMatch checks if a URI matches a subscription
func (sm *SubscriptionManager) doesURIMatch(subscription *ResourceSubscription, uri string) bool {
	if subscription.IsTemplate {

		return sm.matchURITemplate(subscription.URI, uri)
	}

	// Exact match for non-template URIs

	return subscription.URI == uri
}

// matchURITemplate matches a URI against an RFC 6570 template
func (sm *SubscriptionManager) matchURITemplate(template, uri string) bool {
	// Convert RFC 6570 template to regex
	// Simple implementation - full RFC 6570 would be more complex
	pattern := template

	// Replace {var} with regex pattern
	varRegex := regexp.MustCompile(`\{([^}]+)\}`)
	pattern = varRegex.ReplaceAllString(pattern, `([^/]+)`)

	// Escape other regex characters
	pattern = strings.ReplaceAll(pattern, ".", `\.`)
	pattern = strings.ReplaceAll(pattern, "*", `.*`)

	// Anchor the pattern
	pattern = "^" + pattern + "$"

	matched, _ := regexp.MatchString(pattern, uri)

	return matched
}

// shouldIncludeUpdate checks if an update should be included based on filters
func (sm *SubscriptionManager) shouldIncludeUpdate(subscription *ResourceSubscription, update ResourceUpdate) bool {
	for _, filter := range subscription.Filters {
		if !sm.applyFilter(filter, update) {

			return false
		}
	}

	return true
}

// applyFilter applies a single filter to an update
func (sm *SubscriptionManager) applyFilter(filter ResourceFilter, update ResourceUpdate) bool {
	var value interface{}

	switch filter.Property {
	case "type":
		value = update.Type
	case "uri":
		value = update.URI
	case "mimeType":
		if update.Content != nil {
			value = update.Content.MimeType
		}
	case "size":
		if update.Content != nil {
			value = update.Content.Size
		}
	default:
		if update.Metadata != nil {
			value = update.Metadata[filter.Property]
		}
	}

	return sm.matchFilterValue(filter, value)
}

// matchFilterValue matches a filter value against the actual value
func (sm *SubscriptionManager) matchFilterValue(filter ResourceFilter, actualValue interface{}) bool {
	switch filter.Type {
	case "exact":

		return actualValue == filter.Value
	case "prefix":
		if str, ok := actualValue.(string); ok {
			if prefix, ok := filter.Value.(string); ok {

				return strings.HasPrefix(str, prefix)
			}
		}
	case "glob":
		if str, ok := actualValue.(string); ok {
			if pattern, ok := filter.Value.(string); ok {
				matched, _ := regexp.MatchString(globToRegex(pattern), str)

				return matched
			}
		}
	case "regex":
		if str, ok := actualValue.(string); ok {
			if pattern, ok := filter.Value.(string); ok {
				matched, _ := regexp.MatchString(pattern, str)

				return matched
			}
		}
	}

	return false
}

// sendNotificationToClient sends a notification to a specific client
func (sm *SubscriptionManager) sendNotificationToClient(clientID string, updates []ResourceUpdate) error {
	sm.mu.RLock()
	client, exists := sm.clients[clientID]
	sm.mu.RUnlock()

	if !exists {

		return fmt.Errorf("client %s not found", clientID)
	}

	// Create notification
	notification := &ResourceUpdateNotification{
		JSONRPC: "2.0",
		Method:  "notifications/resources/updated",
		Params: ResourceUpdateParams{
			Resources: updates,
		},
	}

	// Send notification

	return client.NotifyFunc(notification)
}

// isURITemplate checks if a URI contains template variables
func (sm *SubscriptionManager) isURITemplate(uri string) bool {

	return strings.Contains(uri, "{") && strings.Contains(uri, "}")
}

// validateFilters validates resource filters
func (sm *SubscriptionManager) validateFilters(filters []ResourceFilter) error {
	for i, filter := range filters {
		if filter.Type == "" {

			return NewValidationError(fmt.Sprintf("filters[%d].type", i), filter.Type, "filter type cannot be empty")
		}

		validTypes := []string{"exact", "prefix", "glob", "regex"}
		valid := false
		for _, validType := range validTypes {
			if filter.Type == validType {
				valid = true

				break
			}
		}

		if !valid {

			return NewValidationError(fmt.Sprintf("filters[%d].type", i), filter.Type, fmt.Sprintf("must be one of: %v", validTypes))
		}

		if filter.Property == "" {

			return NewValidationError(fmt.Sprintf("filters[%d].property", i), filter.Property, "filter property cannot be empty")
		}
	}

	return nil
}

// globToRegex converts a glob pattern to a regex pattern
func globToRegex(glob string) string {
	regex := strings.ReplaceAll(glob, "*", ".*")
	regex = strings.ReplaceAll(regex, "?", ".")

	return "^" + regex + "$"
}

// GetSubscriptions returns all subscriptions for a client
func (sm *SubscriptionManager) GetSubscriptions(clientID string) []*ResourceSubscription {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	client, exists := sm.clients[clientID]
	if !exists {

		return nil
	}

	var subscriptions []*ResourceSubscription
	for _, sub := range client.Subscriptions {
		subscriptions = append(subscriptions, sub)
	}

	return subscriptions
}

// CleanupExpiredSubscriptions removes expired client subscriptions
func (sm *SubscriptionManager) CleanupExpiredSubscriptions(maxAge time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for clientID, client := range sm.clients {
		if client.LastSeen.Before(cutoff) {
			// Remove all subscriptions for this client
			for subID := range client.Subscriptions {
				delete(sm.subscriptions, subID)
			}
			delete(sm.clients, clientID)
		}
	}
}
