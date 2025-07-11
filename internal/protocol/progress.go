// internal/protocol/progress.go
package protocol

import (
	"encoding/json"
	"fmt"
	"time"
)

// ProgressToken represents a progress tracking token
type ProgressToken struct {
	Token     string      `json:"token"`
	RequestID interface{} `json:"requestId"`
	Created   time.Time   `json:"created"`
}

// ProgressNotification represents a progress notification
type ProgressNotification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"` // "notifications/progress"
	Params  ProgressParams `json:"params"`
}

// ProgressParams contains progress notification parameters
type ProgressParams struct {
	ProgressToken string      `json:"progressToken"`
	Progress      float64     `json:"progress"` // 0.0 to 1.0
	Total         *int64      `json:"total,omitempty"`
	Current       *int64      `json:"current,omitempty"`
	Message       string      `json:"message,omitempty"`
	Details       interface{} `json:"details,omitempty"`
}

// ProgressManager manages progress tokens and notifications
type ProgressManager struct {
	tokens    map[string]*ProgressToken
	listeners map[string][]ProgressListener
}

// ProgressListener defines the interface for progress listeners
type ProgressListener func(token string, progress ProgressParams)

// NewProgressManager creates a new progress manager
func NewProgressManager() *ProgressManager {

	return &ProgressManager{
		tokens:    make(map[string]*ProgressToken),
		listeners: make(map[string][]ProgressListener),
	}
}

// GenerateProgressToken creates a new progress token
func (pm *ProgressManager) GenerateProgressToken(requestID interface{}) string {
	token := fmt.Sprintf("prog_%d_%d", time.Now().UnixNano(), requestID)
	pm.tokens[token] = &ProgressToken{
		Token:     token,
		RequestID: requestID,
		Created:   time.Now(),
	}

	return token
}

// IsValidToken checks if a progress token is valid
func (pm *ProgressManager) IsValidToken(token string) bool {
	_, exists := pm.tokens[token]

	return exists
}

// AddProgressListener adds a listener for progress updates
func (pm *ProgressManager) AddProgressListener(token string, listener ProgressListener) {
	if pm.listeners[token] == nil {
		pm.listeners[token] = make([]ProgressListener, 0)
	}
	pm.listeners[token] = append(pm.listeners[token], listener)
}

// UpdateProgress sends a progress notification
func (pm *ProgressManager) UpdateProgress(token string, progress float64, message string, details interface{}) error {
	if !pm.IsValidToken(token) {

		return NewValidationError("progressToken", token, "token must be valid")
	}

	params := ProgressParams{
		ProgressToken: token,
		Progress:      progress,
		Message:       message,
		Details:       details,
	}

	// Notify all listeners
	if listeners, exists := pm.listeners[token]; exists {
		for _, listener := range listeners {
			listener(token, params)
		}
	}

	return nil
}

// UpdateDetailedProgress sends a progress notification with current/total counts
func (pm *ProgressManager) UpdateDetailedProgress(token string, current, total int64, message string, details interface{}) error {
	if !pm.IsValidToken(token) {

		return NewValidationError("progressToken", token, "token must be valid")
	}

	progress := float64(current) / float64(total)
	if total == 0 {
		progress = 0.0
	}

	params := ProgressParams{
		ProgressToken: token,
		Progress:      progress,
		Current:       &current,
		Total:         &total,
		Message:       message,
		Details:       details,
	}

	// Notify all listeners
	if listeners, exists := pm.listeners[token]; exists {
		for _, listener := range listeners {
			listener(token, params)
		}
	}

	return nil
}

// CompleteProgress marks progress as complete and cleans up
func (pm *ProgressManager) CompleteProgress(token string, message string) error {
	if !pm.IsValidToken(token) {

		return NewValidationError("progressToken", token, "token must be valid")
	}

	// Send final progress update
	params := ProgressParams{
		ProgressToken: token,
		Progress:      1.0,
		Message:       message,
	}

	// Notify all listeners
	if listeners, exists := pm.listeners[token]; exists {
		for _, listener := range listeners {
			listener(token, params)
		}
	}

	// Clean up
	delete(pm.tokens, token)
	delete(pm.listeners, token)

	return nil
}

// FailProgress marks progress as failed and cleans up
func (pm *ProgressManager) FailProgress(token string, err error) error {
	if !pm.IsValidToken(token) {

		return NewValidationError("progressToken", token, "token must be valid")
	}

	// Send failure notification
	params := ProgressParams{
		ProgressToken: token,
		Progress:      -1.0, // Indicates failure
		Message:       fmt.Sprintf("Failed: %v", err),
		Details:       map[string]interface{}{"error": err.Error()},
	}

	// Notify all listeners
	if listeners, exists := pm.listeners[token]; exists {
		for _, listener := range listeners {
			listener(token, params)
		}
	}

	// Clean up
	delete(pm.tokens, token)
	delete(pm.listeners, token)

	return nil
}

// CreateProgressNotification creates a JSON-RPC progress notification
func CreateProgressNotification(params ProgressParams) *ProgressNotification {

	return &ProgressNotification{
		JSONRPC: "2.0",
		Method:  "notifications/progress",
		Params:  params,
	}
}

// ParseProgressNotification parses a JSON-RPC progress notification
func ParseProgressNotification(data []byte) (*ProgressNotification, error) {
	var notification ProgressNotification
	if err := json.Unmarshal(data, &notification); err != nil {

		return nil, NewParseError(fmt.Sprintf("invalid progress notification: %v", err))
	}

	if notification.Method != "notifications/progress" {

		return nil, NewInvalidRequest(fmt.Sprintf("expected notifications/progress, got %s", notification.Method))
	}

	return &notification, nil
}

// ValidateProgressParams validates progress parameters
func ValidateProgressParams(params ProgressParams) error {
	if params.ProgressToken == "" {

		return NewValidationError("progressToken", params.ProgressToken, "progress token cannot be empty")
	}

	if params.Progress < -1.0 || params.Progress > 1.0 {

		return NewValidationError("progress", params.Progress, "progress must be between -1.0 and 1.0")
	}

	if params.Current != nil && params.Total != nil {
		if *params.Current < 0 {

			return NewValidationError("current", *params.Current, "current cannot be negative")
		}
		if *params.Total < 0 {

			return NewValidationError("total", *params.Total, "total cannot be negative")
		}
		if *params.Current > *params.Total {

			return NewValidationError("current", *params.Current, "current cannot exceed total")
		}
	}

	return nil
}
