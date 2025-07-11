// internal/protocol/change_notifications.go
package protocol

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ChangeNotificationManager manages tool and prompt change notifications
type ChangeNotificationManager struct {
	toolSubscribers   map[string]*ChangeSubscriber
	promptSubscribers map[string]*ChangeSubscriber
	toolHashes        map[string]string
	promptHashes      map[string]string
	mu                sync.RWMutex
}

// ChangeSubscriber represents a client subscribed to change notifications
type ChangeSubscriber struct {
	ClientID   string
	SessionID  string
	NotifyFunc func(*ChangeNotification) error
	Subscribed time.Time
	LastNotify time.Time
}

// ChangeNotification represents a list changed notification
type ChangeNotification struct {
	JSONRPC string       `json:"jsonrpc"`
	Method  string       `json:"method"` // "notifications/tools/list_changed" or "notifications/prompts/list_changed"
	Params  ChangeParams `json:"params"`
}

// ChangeParams contains change notification parameters
type ChangeParams struct {
	// Empty params for basic notifications
	// Can be extended with change details in the future
}

// ToolDefinition represents a tool definition for change detection
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	// Add other tool properties as needed
}

// PromptDefinition represents a prompt definition for change detection
type PromptDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Arguments   []PromptArgument `json:"arguments,omitempty"`
	// Add other prompt properties as needed
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// NewChangeNotificationManager creates a new change notification manager
func NewChangeNotificationManager() *ChangeNotificationManager {

	return &ChangeNotificationManager{
		toolSubscribers:   make(map[string]*ChangeSubscriber),
		promptSubscribers: make(map[string]*ChangeSubscriber),
		toolHashes:        make(map[string]string),
		promptHashes:      make(map[string]string),
	}
}

// SubscribeToToolChanges subscribes a client to tool change notifications
func (cnm *ChangeNotificationManager) SubscribeToToolChanges(clientID, sessionID string, notifyFunc func(*ChangeNotification) error) {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	cnm.toolSubscribers[clientID] = &ChangeSubscriber{
		ClientID:   clientID,
		SessionID:  sessionID,
		NotifyFunc: notifyFunc,
		Subscribed: time.Now(),
	}
}

// SubscribeToPromptChanges subscribes a client to prompt change notifications
func (cnm *ChangeNotificationManager) SubscribeToPromptChanges(clientID, sessionID string, notifyFunc func(*ChangeNotification) error) {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	cnm.promptSubscribers[clientID] = &ChangeSubscriber{
		ClientID:   clientID,
		SessionID:  sessionID,
		NotifyFunc: notifyFunc,
		Subscribed: time.Now(),
	}
}

// UnsubscribeFromToolChanges unsubscribes a client from tool change notifications
func (cnm *ChangeNotificationManager) UnsubscribeFromToolChanges(clientID string) {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	delete(cnm.toolSubscribers, clientID)
}

// UnsubscribeFromPromptChanges unsubscribes a client from prompt change notifications
func (cnm *ChangeNotificationManager) UnsubscribeFromPromptChanges(clientID string) {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	delete(cnm.promptSubscribers, clientID)
}

// UpdateTools checks for tool changes and notifies subscribers
func (cnm *ChangeNotificationManager) UpdateTools(serverName string, tools []ToolDefinition) error {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	// Calculate hash of current tools
	currentHash, err := cnm.calculateToolsHash(tools)
	if err != nil {

		return fmt.Errorf("failed to calculate tools hash: %w", err)
	}

	// Check if tools have changed
	previousHash, exists := cnm.toolHashes[serverName]
	if exists && previousHash == currentHash {

		return nil // No changes
	}

	// Update hash
	cnm.toolHashes[serverName] = currentHash

	// Only notify if this isn't the first time we're seeing these tools
	if exists {
		// Notify all subscribers
		notification := &ChangeNotification{
			JSONRPC: "2.0",
			Method:  "notifications/tools/list_changed",
			Params:  ChangeParams{},
		}

		for clientID, subscriber := range cnm.toolSubscribers {
			if err := subscriber.NotifyFunc(notification); err != nil {
				// Log error but continue with other subscribers
				fmt.Printf("Failed to notify client %s of tool changes: %v\n", clientID, err)
			} else {
				subscriber.LastNotify = time.Now()
			}
		}
	}

	return nil
}

// UpdatePrompts checks for prompt changes and notifies subscribers
func (cnm *ChangeNotificationManager) UpdatePrompts(serverName string, prompts []PromptDefinition) error {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	// Calculate hash of current prompts
	currentHash, err := cnm.calculatePromptsHash(prompts)
	if err != nil {

		return fmt.Errorf("failed to calculate prompts hash: %w", err)
	}

	// Check if prompts have changed
	previousHash, exists := cnm.promptHashes[serverName]
	if exists && previousHash == currentHash {

		return nil // No changes
	}

	// Update hash
	cnm.promptHashes[serverName] = currentHash

	// Only notify if this isn't the first time we're seeing these prompts
	if exists {
		// Notify all subscribers
		notification := &ChangeNotification{
			JSONRPC: "2.0",
			Method:  "notifications/prompts/list_changed",
			Params:  ChangeParams{},
		}

		for clientID, subscriber := range cnm.promptSubscribers {
			if err := subscriber.NotifyFunc(notification); err != nil {
				// Log error but continue with other subscribers
				fmt.Printf("Failed to notify client %s of prompt changes: %v\n", clientID, err)
			} else {
				subscriber.LastNotify = time.Now()
			}
		}
	}

	return nil
}

// calculateToolsHash calculates a hash of the tool definitions
func (cnm *ChangeNotificationManager) calculateToolsHash(tools []ToolDefinition) (string, error) {
	// Sort tools by name for consistent hashing
	sortedTools := make([]ToolDefinition, len(tools))
	copy(sortedTools, tools)

	// Simple sorting - in production you might want a more sophisticated approach
	for i := 0; i < len(sortedTools); i++ {
		for j := i + 1; j < len(sortedTools); j++ {
			if sortedTools[i].Name > sortedTools[j].Name {
				sortedTools[i], sortedTools[j] = sortedTools[j], sortedTools[i]
			}
		}
	}

	// Marshal to JSON for hashing
	jsonData, err := json.Marshal(sortedTools)
	if err != nil {

		return "", err
	}

	// Calculate MD5 hash
	hash := md5.Sum(jsonData)

	return fmt.Sprintf("%x", hash), nil
}

// calculatePromptsHash calculates a hash of the prompt definitions
func (cnm *ChangeNotificationManager) calculatePromptsHash(prompts []PromptDefinition) (string, error) {
	// Sort prompts by name for consistent hashing
	sortedPrompts := make([]PromptDefinition, len(prompts))
	copy(sortedPrompts, prompts)

	// Simple sorting
	for i := 0; i < len(sortedPrompts); i++ {
		for j := i + 1; j < len(sortedPrompts); j++ {
			if sortedPrompts[i].Name > sortedPrompts[j].Name {
				sortedPrompts[i], sortedPrompts[j] = sortedPrompts[j], sortedPrompts[i]
			}
		}
	}

	// Marshal to JSON for hashing
	jsonData, err := json.Marshal(sortedPrompts)
	if err != nil {

		return "", err
	}

	// Calculate MD5 hash
	hash := md5.Sum(jsonData)

	return fmt.Sprintf("%x", hash), nil
}

// GetToolSubscribers returns the list of tool change subscribers
func (cnm *ChangeNotificationManager) GetToolSubscribers() map[string]*ChangeSubscriber {
	cnm.mu.RLock()
	defer cnm.mu.RUnlock()

	result := make(map[string]*ChangeSubscriber)
	for k, v := range cnm.toolSubscribers {
		result[k] = v
	}

	return result
}

// GetPromptSubscribers returns the list of prompt change subscribers
func (cnm *ChangeNotificationManager) GetPromptSubscribers() map[string]*ChangeSubscriber {
	cnm.mu.RLock()
	defer cnm.mu.RUnlock()

	result := make(map[string]*ChangeSubscriber)
	for k, v := range cnm.promptSubscribers {
		result[k] = v
	}

	return result
}

// CleanupInactiveSubscribers removes subscribers that haven't been seen recently
func (cnm *ChangeNotificationManager) CleanupInactiveSubscribers(maxAge time.Duration) {
	cnm.mu.Lock()
	defer cnm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	// Clean up tool subscribers
	for clientID, subscriber := range cnm.toolSubscribers {
		if subscriber.Subscribed.Before(cutoff) &&
			(subscriber.LastNotify.IsZero() || subscriber.LastNotify.Before(cutoff)) {
			delete(cnm.toolSubscribers, clientID)
		}
	}

	// Clean up prompt subscribers
	for clientID, subscriber := range cnm.promptSubscribers {
		if subscriber.Subscribed.Before(cutoff) &&
			(subscriber.LastNotify.IsZero() || subscriber.LastNotify.Before(cutoff)) {
			delete(cnm.promptSubscribers, clientID)
		}
	}
}

// ForceNotifyToolChanges forces a tools/list_changed notification to all subscribers
func (cnm *ChangeNotificationManager) ForceNotifyToolChanges() error {
	cnm.mu.RLock()
	defer cnm.mu.RUnlock()

	notification := &ChangeNotification{
		JSONRPC: "2.0",
		Method:  "notifications/tools/list_changed",
		Params:  ChangeParams{},
	}

	for clientID, subscriber := range cnm.toolSubscribers {
		if err := subscriber.NotifyFunc(notification); err != nil {
			fmt.Printf("Failed to notify client %s of forced tool changes: %v\n", clientID, err)
		} else {
			subscriber.LastNotify = time.Now()
		}
	}

	return nil
}

// ForceNotifyPromptChanges forces a prompts/list_changed notification to all subscribers
func (cnm *ChangeNotificationManager) ForceNotifyPromptChanges() error {
	cnm.mu.RLock()
	defer cnm.mu.RUnlock()

	notification := &ChangeNotification{
		JSONRPC: "2.0",
		Method:  "notifications/prompts/list_changed",
		Params:  ChangeParams{},
	}

	for clientID, subscriber := range cnm.promptSubscribers {
		if err := subscriber.NotifyFunc(notification); err != nil {
			fmt.Printf("Failed to notify client %s of forced prompt changes: %v\n", clientID, err)
		} else {
			subscriber.LastNotify = time.Now()
		}
	}

	return nil
}
