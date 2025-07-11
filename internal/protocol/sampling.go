// internal/protocol/sampling.go
package protocol

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// SamplingManager manages LLM sampling requests from MCP servers
type SamplingManager struct {
	requests      map[string]*SamplingRequest
	handlers      map[string]SamplingHandler
	humanControls map[string]*HumanControlConfig
	mu            sync.RWMutex
}

// SamplingRequest represents a sampling/createMessage request
type SamplingRequest struct {
	ID           string             `json:"id"`
	ServerName   string             `json:"serverName"`
	Messages     []SamplingMessage  `json:"messages"`
	ModelPrefs   ModelPreferences   `json:"modelPrefs,omitempty"`
	MaxTokens    int                `json:"maxTokens,omitempty"`
	StopSequence []string           `json:"stopSequence,omitempty"`
	Temperature  float64            `json:"temperature,omitempty"`
	Context      SamplingContext    `json:"context,omitempty"`
	Created      time.Time          `json:"created"`
	Status       string             `json:"status"` // "pending", "approved", "rejected", "completed", "failed"
	HumanReview  *HumanReviewResult `json:"humanReview,omitempty"`
}

// SamplingMessage represents a message in the sampling request
type SamplingMessage struct {
	Role     string                 `json:"role"` // "system", "user", "assistant"
	Content  SamplingContent        `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SamplingContent represents content that can include text and embedded resources
type SamplingContent struct {
	Type      string            `json:"type"` // "text", "image", "resource"
	Text      string            `json:"text,omitempty"`
	ImageData string            `json:"imageData,omitempty"` // base64
	MimeType  string            `json:"mimeType,omitempty"`
	Resource  *EmbeddedResource `json:"resource,omitempty"`
}

// EmbeddedResource represents a resource embedded in sampling content
type EmbeddedResource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Content     string                 `json:"content,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ModelPreferences specifies model preferences for sampling
type ModelPreferences struct {
	Hints                []ModelHint `json:"hints,omitempty"`
	CostPriority         string      `json:"costPriority,omitempty"`         // "low", "medium", "high"
	SpeedPriority        string      `json:"speedPriority,omitempty"`        // "low", "medium", "high"
	IntelligencePriority string      `json:"intelligencePriority,omitempty"` // "low", "medium", "high"
}

// ModelHint provides hints about desired model characteristics
type ModelHint struct {
	Name string `json:"name"` // e.g., "claude-3-sonnet", "gpt-4"
}

// SamplingContext provides context for the sampling request
type SamplingContext struct {
	ServerInfo      map[string]interface{} `json:"serverInfo,omitempty"`
	ToolContext     []string               `json:"toolContext,omitempty"`     // Recently used tools
	ResourceContext []string               `json:"resourceContext,omitempty"` // Recently accessed resources
	SessionContext  map[string]interface{} `json:"sessionContext,omitempty"`
}

// HumanControlConfig configures human-in-the-loop controls
type HumanControlConfig struct {
	RequireApproval     bool     `json:"requireApproval"`
	AutoApprovePatterns []string `json:"autoApprovePatterns,omitempty"`
	BlockPatterns       []string `json:"blockPatterns,omitempty"`
	MaxTokens           int      `json:"maxTokens,omitempty"`
	AllowedModels       []string `json:"allowedModels,omitempty"`
	TimeoutSeconds      int      `json:"timeoutSeconds,omitempty"`
}

// HumanReviewResult represents the result of human review
type HumanReviewResult struct {
	Approved      bool                   `json:"approved"`
	Reviewer      string                 `json:"reviewer"`
	ReviewTime    time.Time              `json:"reviewTime"`
	Comments      string                 `json:"comments,omitempty"`
	Modifications map[string]interface{} `json:"modifications,omitempty"`
}

// SamplingResponse represents the response to a sampling request
type SamplingResponse struct {
	Content    SamplingContent        `json:"content"`
	Model      string                 `json:"model,omitempty"`
	StopReason string                 `json:"stopReason,omitempty"`
	Usage      SamplingUsage          `json:"usage,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SamplingUsage provides token usage information
type SamplingUsage struct {
	InputTokens  int `json:"inputTokens,omitempty"`
	OutputTokens int `json:"outputTokens,omitempty"`
	TotalTokens  int `json:"totalTokens,omitempty"`
}

// SamplingHandler defines the interface for handling sampling requests
type SamplingHandler interface {
	// HandleSamplingRequest processes a sampling request
	HandleSamplingRequest(request *SamplingRequest) (*SamplingResponse, error)
	// GetSupportedModels returns supported model names
	GetSupportedModels() []string
	// GetCapabilities returns handler capabilities
	GetCapabilities() SamplingCapabilities
}

// SamplingCapabilities describes what a sampling handler can do
type SamplingCapabilities struct {
	Models            []string `json:"models"`
	MaxTokens         int      `json:"maxTokens"`
	SupportsImages    bool     `json:"supportsImages"`
	SupportsTools     bool     `json:"supportsTools"`
	SupportsStreaming bool     `json:"supportsStreaming"`
}

// NewSamplingManager creates a new sampling manager
func NewSamplingManager() *SamplingManager {

	return &SamplingManager{
		requests:      make(map[string]*SamplingRequest),
		handlers:      make(map[string]SamplingHandler),
		humanControls: make(map[string]*HumanControlConfig),
	}
}

// RegisterHandler registers a sampling handler for a specific model or provider
func (sm *SamplingManager) RegisterHandler(name string, handler SamplingHandler) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.handlers[name] = handler
}

// SetHumanControls configures human-in-the-loop controls for a server
func (sm *SamplingManager) SetHumanControls(serverName string, config *HumanControlConfig) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.humanControls[serverName] = config
}

// CreateSamplingRequest creates a new sampling request
func (sm *SamplingManager) CreateSamplingRequest(serverName string, messages []SamplingMessage, prefs ModelPreferences, context SamplingContext) (*SamplingRequest, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	requestID := fmt.Sprintf("sampling_%s_%d", serverName, time.Now().UnixNano())

	request := &SamplingRequest{
		ID:         requestID,
		ServerName: serverName,
		Messages:   messages,
		ModelPrefs: prefs,
		Context:    context,
		Created:    time.Now(),
		Status:     "pending",
	}

	// Check if human review is required
	if humanConfig, exists := sm.humanControls[serverName]; exists {
		if humanConfig.RequireApproval {
			if sm.requiresHumanApproval(request, humanConfig) {
				request.Status = "awaiting_approval"
			}
		}
	}

	sm.requests[requestID] = request

	return request, nil
}

// ProcessSamplingRequest processes a sampling request
func (sm *SamplingManager) ProcessSamplingRequest(requestID string) (*SamplingResponse, error) {
	sm.mu.RLock()
	request, exists := sm.requests[requestID]
	sm.mu.RUnlock()

	if !exists {

		return nil, fmt.Errorf("sampling request %s not found", requestID)
	}

	// Check if request needs approval
	if request.Status == "awaiting_approval" {

		return nil, fmt.Errorf("sampling request %s is awaiting human approval", requestID)
	}

	if request.Status == "rejected" {

		return nil, fmt.Errorf("sampling request %s was rejected", requestID)
	}

	// Find appropriate handler
	handler := sm.selectHandler(request)
	if handler == nil {

		return nil, fmt.Errorf("no suitable handler found for sampling request")
	}

	// Process the request
	response, err := handler.HandleSamplingRequest(request)
	if err != nil {
		sm.mu.Lock()
		request.Status = "failed"
		sm.mu.Unlock()

		return nil, fmt.Errorf("sampling request failed: %w", err)
	}

	sm.mu.Lock()
	request.Status = "completed"
	sm.mu.Unlock()

	return response, nil
}

// ApproveRequest approves a sampling request
func (sm *SamplingManager) ApproveRequest(requestID, reviewer, comments string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	request, exists := sm.requests[requestID]
	if !exists {

		return fmt.Errorf("sampling request %s not found", requestID)
	}

	if request.Status != "awaiting_approval" {

		return fmt.Errorf("sampling request %s is not awaiting approval", requestID)
	}

	request.Status = "approved"
	request.HumanReview = &HumanReviewResult{
		Approved:   true,
		Reviewer:   reviewer,
		ReviewTime: time.Now(),
		Comments:   comments,
	}

	return nil
}

// RejectRequest rejects a sampling request
func (sm *SamplingManager) RejectRequest(requestID, reviewer, reason string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	request, exists := sm.requests[requestID]
	if !exists {

		return fmt.Errorf("sampling request %s not found", requestID)
	}

	if request.Status != "awaiting_approval" {

		return fmt.Errorf("sampling request %s is not awaiting approval", requestID)
	}

	request.Status = "rejected"
	request.HumanReview = &HumanReviewResult{
		Approved:   false,
		Reviewer:   reviewer,
		ReviewTime: time.Now(),
		Comments:   reason,
	}

	return nil
}

// requiresHumanApproval checks if a request requires human approval
func (sm *SamplingManager) requiresHumanApproval(request *SamplingRequest, config *HumanControlConfig) bool {
	// Check auto-approve patterns
	for _, pattern := range config.AutoApprovePatterns {
		if sm.matchesPattern(request, pattern) {

			return false
		}
	}

	// Check block patterns
	for _, pattern := range config.BlockPatterns {
		if sm.matchesPattern(request, pattern) {

			return true // Block and require approval
		}
	}

	// Check token limits
	if config.MaxTokens > 0 && request.MaxTokens > config.MaxTokens {

		return true
	}

	// Default to requiring approval if configured

	return config.RequireApproval
}

// matchesPattern checks if a request matches a pattern
func (sm *SamplingManager) matchesPattern(request *SamplingRequest, pattern string) bool {
	// Simple pattern matching - in production, you might want regex or more sophisticated matching
	for _, msg := range request.Messages {
		if strings.Contains(strings.ToLower(msg.Content.Text), strings.ToLower(pattern)) {

			return true
		}
	}

	return false
}

// selectHandler selects the best handler for a request
func (sm *SamplingManager) selectHandler(request *SamplingRequest) SamplingHandler {
	// Priority: specific model hints, then capabilities, then default
	for _, hint := range request.ModelPrefs.Hints {
		if handler, exists := sm.handlers[hint.Name]; exists {

			return handler
		}
	}

	// Fallback to first available handler
	for _, handler := range sm.handlers {

		return handler
	}

	return nil
}

// GetPendingRequests returns requests awaiting approval
func (sm *SamplingManager) GetPendingRequests() []*SamplingRequest {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var pending []*SamplingRequest
	for _, request := range sm.requests {
		if request.Status == "awaiting_approval" {
			pending = append(pending, request)
		}
	}

	return pending
}

// GetRequestStatus returns the status of a request
func (sm *SamplingManager) GetRequestStatus(requestID string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	request, exists := sm.requests[requestID]
	if !exists {

		return "", fmt.Errorf("sampling request %s not found", requestID)
	}

	return request.Status, nil
}

// CleanupOldRequests removes old completed/failed requests
func (sm *SamplingManager) CleanupOldRequests(maxAge time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for id, request := range sm.requests {
		if request.Created.Before(cutoff) &&
			(request.Status == "completed" || request.Status == "failed" || request.Status == "rejected") {
			delete(sm.requests, id)
		}
	}
}
