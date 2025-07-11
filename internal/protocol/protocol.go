// internal/protocol/protocol.go
package protocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// MCPVersion represents the current Model Context Protocol version
const MCPVersion = "2024-11-05"

// MessageType defines the type of MCP message
type MessageType string

const (
	// Request is an RPC request
	Request MessageType = "request"
	// Response is an RPC response
	Response MessageType = "response"
	// Notification is a one-way notification
	Notification MessageType = "notification"
)

// MCPMessage represents a generic MCP message
type MCPMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	// Progress support
	ProgressToken string `json:"progressToken,omitempty"`
}

// MCPRequest represents an MCP request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	// Progress support
	ProgressToken string `json:"progressToken,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
	// Progress support
	ProgressToken string `json:"progressToken,omitempty"`
}

// MCPNotification represents an MCP notification
type MCPNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Capability represents an MCP capability
type Capability string

const (
	// ResourcesCapability provides file system access
	ResourcesCapability Capability = "resources"
	// ToolsCapability provides function calling
	ToolsCapability Capability = "tools"
	// PromptsCapability provides prompt templates
	PromptsCapability Capability = "prompts"
	// SamplingCapability provides text generation
	SamplingCapability Capability = "sampling"
	// LoggingCapability provides logging
	LoggingCapability Capability = "logging"
	// RootsCapability provides root management
	RootsCapability Capability = "roots"
)

// InitializeParams represents the parameters for an initialize request
type InitializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    CapabilitiesOpts `json:"capabilities"`
	ClientInfo      ClientInfo       `json:"clientInfo"`
	// Add roots support
	Roots []Root `json:"roots,omitempty"`
}

// InitializeResult represents the result of an initialize request
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ServerInfo      ServerInfo       `json:"serverInfo"`
	Capabilities    CapabilitiesOpts `json:"capabilities"`
	Instructions    string           `json:"instructions,omitempty"`
	// Server can announce its roots
	Roots []Root `json:"roots,omitempty"`
}

// Root represents an MCP root
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// ClientInfo represents information about the client
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo represents information about the server
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// CapabilitiesOpts represents MCP capability options with full specification compliance
type CapabilitiesOpts struct {
	Resources *ResourcesOpts `json:"resources,omitempty"`
	Tools     *ToolsOpts     `json:"tools,omitempty"`
	Prompts   *PromptsOpts   `json:"prompts,omitempty"`
	Sampling  *SamplingOpts  `json:"sampling,omitempty"`
	Logging   *LoggingOpts   `json:"logging,omitempty"`
	Roots     *RootsOpts     `json:"roots,omitempty"`
}

// ResourcesOpts represents resources capability options with subscription support
type ResourcesOpts struct {
	ListChanged bool `json:"listChanged,omitempty"`
	Subscribe   bool `json:"subscribe,omitempty"`
}

// ToolsOpts represents tools capability options
type ToolsOpts struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsOpts represents prompts capability options
type PromptsOpts struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingOpts represents sampling capability options
type SamplingOpts struct {
	// Sampling has no specific options in current spec
}

// LoggingOpts represents logging capability options
type LoggingOpts struct {
	// Logging has no specific options in current spec
}

// RootsOpts represents roots capability options
type RootsOpts struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// Transport defines the interface for MCP transports
type Transport interface {
	// Send sends an MCP message
	Send(msg MCPMessage) error
	// Receive receives an MCP message
	Receive() (MCPMessage, error)
	// Close closes the transport
	Close() error
	// SupportsProgress returns true if transport supports progress notifications
	SupportsProgress() bool
	// SendProgress sends a progress notification
	SendProgress(notification *ProgressNotification) error
}

// StdioTransport implements the stdio transport
type StdioTransport struct {
	reader           *json.Decoder
	writer           *json.Encoder
	progressManager  *ProgressManager
	progressListener ProgressListener
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(r io.Reader, w io.Writer) *StdioTransport {
	transport := &StdioTransport{
		reader:          json.NewDecoder(r),
		writer:          json.NewEncoder(w),
		progressManager: NewProgressManager(),
	}
	// Set up built-in progress listener
	transport.progressListener = func(token string, progress ProgressParams) {
		notification := CreateProgressNotification(progress)
		if err := transport.SendProgress(notification); err != nil {
			// Log error but don't fail initialization
			fmt.Printf("Warning: failed to send progress notification: %v\n", err)
		}
	}

	return transport
}

// Send sends an MCP message via stdio
func (t *StdioTransport) Send(msg MCPMessage) error {

	return t.writer.Encode(msg)
}

// Receive receives an MCP message via stdio
func (t *StdioTransport) Receive() (MCPMessage, error) {
	var msg MCPMessage
	if err := t.reader.Decode(&msg); err != nil {

		return msg, err
	}

	return msg, nil
}

// Close closes the stdio transport
func (t *StdioTransport) Close() error {

	return nil // Nothing to close for stdio
}

// SupportsProgress returns true since stdio supports progress notifications
func (t *StdioTransport) SupportsProgress() bool {

	return true
}

// SendProgress sends a progress notification
func (t *StdioTransport) SendProgress(notification *ProgressNotification) error {

	return t.writer.Encode(notification)
}

// GetProgressManager returns the progress manager
func (t *StdioTransport) GetProgressManager() *ProgressManager {

	return t.progressManager
}

// NewRequest creates a new MCP request with optional progress token
func NewRequest(id interface{}, method string, params interface{}, progressToken ...string) (*MCPRequest, error) {
	var paramsBytes json.RawMessage
	if params != nil {
		var err error
		paramsBytes, err = json.Marshal(params)
		if err != nil {

			return nil, err
		}
	}

	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsBytes,
	}

	if len(progressToken) > 0 && progressToken[0] != "" {
		req.ProgressToken = progressToken[0]
	}

	return req, nil
}

// NewResponse creates a new MCP response with optional progress token
func NewResponse(id interface{}, result interface{}, err *MCPError, progressToken ...string) (*MCPResponse, error) {
	var resultBytes json.RawMessage
	if result != nil && err == nil {
		var marshalErr error
		resultBytes, marshalErr = json.Marshal(result)
		if marshalErr != nil {

			return nil, marshalErr
		}
	}

	resp := &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultBytes,
		Error:   err,
	}

	if len(progressToken) > 0 && progressToken[0] != "" {
		resp.ProgressToken = progressToken[0]
	}

	return resp, nil
}

// NewNotification creates a new MCP notification
func NewNotification(method string, params interface{}) (*MCPNotification, error) {
	var paramsBytes json.RawMessage
	if params != nil {
		var err error
		paramsBytes, err = json.Marshal(params)
		if err != nil {

			return nil, err
		}
	}

	return &MCPNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  paramsBytes,
	}, nil
}

// ValidateCapabilities checks if the server capabilities match the given capabilities
func ValidateCapabilities(serverCapabilities CapabilitiesOpts, requiredCapabilities []string) error {
	for _, cap := range requiredCapabilities {
		switch cap {
		case "resources":
			if serverCapabilities.Resources == nil {

				return NewCapabilityError("resources", "server does not support resources capability")
			}
		case "tools":
			if serverCapabilities.Tools == nil {

				return NewCapabilityError("tools", "server does not support tools capability")
			}
		case "prompts":
			if serverCapabilities.Prompts == nil {

				return NewCapabilityError("prompts", "server does not support prompts capability")
			}
		case "sampling":
			if serverCapabilities.Sampling == nil {

				return NewCapabilityError("sampling", "server does not support sampling capability")
			}
		case "logging":
			if serverCapabilities.Logging == nil {

				return NewCapabilityError("logging", "server does not support logging capability")
			}
		case "roots":
			if serverCapabilities.Roots == nil {

				return NewCapabilityError("roots", "server does not support roots capability")
			}
		}
	}

	return nil
}

// CapabilityOptsFromConfig converts capability configuration to MCP capability options
func CapabilityOptsFromConfig(capOpt interface{}) CapabilitiesOpts {

	return CapabilitiesOpts{
		Resources: &ResourcesOpts{
			ListChanged: true,
			Subscribe:   true,
		},
		Tools: &ToolsOpts{
			ListChanged: true,
		},
		Prompts: &PromptsOpts{
			ListChanged: true,
		},
		Sampling: &SamplingOpts{},
		Logging:  &LoggingOpts{},
		Roots: &RootsOpts{
			ListChanged: true,
		},
	}
}

// ValidateMessage performs comprehensive MCP message validation
func ValidateMessage(msg MCPMessage) error {
	if msg.JSONRPC != "2.0" {

		return NewInvalidRequest("jsonrpc field must be '2.0'")
	}

	// If it has an ID, it's a request or response
	if msg.ID != nil {
		if msg.Method != "" {
			// It's a request
			if msg.Result != nil || msg.Error != nil {

				return NewInvalidRequest("request cannot have result or error fields")
			}
		} else {
			// It's a response
			if msg.Method != "" || msg.Params != nil {

				return NewInvalidRequest("response cannot have method or params fields")
			}
			if msg.Result != nil && msg.Error != nil {

				return NewInvalidRequest("response cannot have both result and error")
			}
			if msg.Result == nil && msg.Error == nil {

				return NewInvalidRequest("response must have either result or error")
			}
		}
	} else {
		// It's a notification
		if msg.Method == "" {

			return NewInvalidRequest("notification must have method field")
		}
		if msg.Result != nil || msg.Error != nil {

			return NewInvalidRequest("notification cannot have result or error fields")
		}
	}

	return nil
}

// IsProgressSupported checks if a method supports progress reporting
func IsProgressSupported(method string) bool {
	progressSupportedMethods := map[string]bool{
		"tools/call":      true,
		"resources/read":  true,
		"resources/list":  true,
		"prompts/get":     true,
		"sampling/create": true,
		// Add other long-running methods as needed
	}

	return progressSupportedMethods[method]
}

// Standard MCP methods
const (
	MethodInitialize           = "initialize"
	MethodInitialized          = "notifications/initialized"
	MethodPing                 = "ping"
	MethodResourcesList        = "resources/list"
	MethodResourcesRead        = "resources/read"
	MethodResourcesSubscribe   = "resources/subscribe"
	MethodResourcesUnsubscribe = "resources/unsubscribe"
	MethodToolsList            = "tools/list"
	MethodToolsCall            = "tools/call"
	MethodPromptsList          = "prompts/list"
	MethodPromptsGet           = "prompts/get"
	MethodSamplingCreate       = "sampling/createMessage"
	MethodLoggingSetLevel      = "logging/setLevel"
	MethodRootsList            = "roots/list"
	MethodCompletionComplete   = "completion/complete"
	// Notifications
	NotificationCancelled            = "notifications/cancelled"
	NotificationProgress             = "notifications/progress"
	NotificationResourcesUpdated     = "notifications/resources/updated"
	NotificationResourcesListChanged = "notifications/resources/list_changed"
	NotificationToolsListChanged     = "notifications/tools/list_changed"
	NotificationPromptsListChanged   = "notifications/prompts/list_changed"
	NotificationRootsListChanged     = "notifications/roots/list_changed"
)

// IsStandardMethod checks if a method is part of the MCP specification
func IsStandardMethod(method string) bool {
	standardMethods := map[string]bool{
		MethodInitialize:                 true,
		MethodInitialized:                true,
		MethodPing:                       true,
		MethodResourcesList:              true,
		MethodResourcesRead:              true,
		MethodResourcesSubscribe:         true,
		MethodResourcesUnsubscribe:       true,
		MethodToolsList:                  true,
		MethodToolsCall:                  true,
		MethodPromptsList:                true,
		MethodPromptsGet:                 true,
		MethodSamplingCreate:             true,
		MethodLoggingSetLevel:            true,
		MethodRootsList:                  true,
		MethodCompletionComplete:         true,
		NotificationCancelled:            true,
		NotificationProgress:             true,
		NotificationResourcesUpdated:     true,
		NotificationResourcesListChanged: true,
		NotificationToolsListChanged:     true,
		NotificationPromptsListChanged:   true,
		NotificationRootsListChanged:     true,
	}

	return standardMethods[method]
}
