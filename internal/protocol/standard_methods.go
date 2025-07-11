// internal/protocol/standard_methods.go
package protocol

import (
	"encoding/json"
	"fmt"
	"github.com/phildougherty/mcp-compose/internal/logging"
	"time"
)

// StandardMethodHandler handles standard MCP methods
type StandardMethodHandler struct {
	capabilities CapabilitiesOpts
	serverInfo   ServerInfo
	rootManager  *RootManager
	initialized  bool
	logger       *logging.Logger
}

// NewStandardMethodHandler creates a new standard method handler
func NewStandardMethodHandler(serverInfo ServerInfo, capabilities CapabilitiesOpts, logger *logging.Logger) *StandardMethodHandler {

	return &StandardMethodHandler{
		capabilities: capabilities,
		serverInfo:   serverInfo,
		rootManager:  NewRootManager(),
		initialized:  false,
		logger:       logger,
	}
}

// HandleStandardMethod handles a standard MCP method
func (h *StandardMethodHandler) HandleStandardMethod(method string, params json.RawMessage, requestID interface{}) (*MCPResponse, error) {
	switch method {
	case MethodInitialize:

		return h.handleInitialize(params, requestID)
	case MethodPing:

		return h.handlePing(requestID)
	case MethodRootsList:

		return h.handleRootsList(params, requestID)
	default:

		return nil, NewMethodNotFound(method)
	}
}

// HandleStandardNotification handles a standard MCP notification
func (h *StandardMethodHandler) HandleStandardNotification(method string, params json.RawMessage) error {
	switch method {
	case MethodInitialized:

		return h.handleInitialized(params)
	case NotificationCancelled:

		return h.handleCancelled(params)
	default:

		return NewMethodNotFound(method)
	}
}

// handleInitialize handles the initialize method
func (h *StandardMethodHandler) handleInitialize(params json.RawMessage, requestID interface{}) (*MCPResponse, error) {
	var initParams InitializeParams
	if err := json.Unmarshal(params, &initParams); err != nil {

		return nil, NewInvalidParams("failed to parse initialize parameters", string(params))
	}

	// Validate protocol version
	if initParams.ProtocolVersion != MCPVersion {

		return nil, NewProtocolError(MCPVersion, initParams.ProtocolVersion)
	}

	// Store client roots if provided
	if len(initParams.Roots) > 0 {
		for _, root := range initParams.Roots {
			// Add client roots with read-only permissions by default
			permissions := &RootPermissions{
				Read:  true,
				Write: false,
				List:  true,
				Watch: false,
			}
			if err := h.rootManager.AddRoot(root, permissions); err != nil {
				// Log warning but don't fail initialization
				fmt.Printf("Warning: failed to add client root %s: %v\n", root.URI, err)
			}
		}
	}

	// Create default roots if none provided
	if len(initParams.Roots) == 0 {
		if err := h.rootManager.CreateDefaultRoots(); err != nil {
			fmt.Printf("Warning: failed to create default roots: %v\n", err)
		}
	}

	// Prepare response
	result := InitializeResult{
		ProtocolVersion: MCPVersion,
		ServerInfo:      h.serverInfo,
		Capabilities:    h.capabilities,
		Roots:           h.rootManager.ListRoots(),
	}

	// Mark as initialized
	h.initialized = true

	return NewResponse(requestID, result, nil)
}

// handleInitialized handles the initialized notification
func (h *StandardMethodHandler) handleInitialized(params json.RawMessage) error {
	// Client has acknowledged initialization
	if !h.initialized {

		return NewStateError("initialized", "not_initialized")
	}

	// Parse and validate the initialized notification params if needed
	if len(params) > 0 {
		var initParams map[string]interface{}
		if err := json.Unmarshal(params, &initParams); err != nil {
			h.logger.Warning("Failed to parse initialized notification params: %v", err)
			// Don't fail the notification for parse errors
		} else {
			h.logger.Debug("Received initialized notification with params: %v", initParams)
		}
	}

	h.logger.Info("Client initialization acknowledged")

	return nil
}

// handlePing handles the ping method
func (h *StandardMethodHandler) handlePing(requestID interface{}) (*MCPResponse, error) {
	// Simple ping response
	result := map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"status":    "ok",
	}

	return NewResponse(requestID, result, nil)
}

// handleRootsList handles the roots/list method
func (h *StandardMethodHandler) handleRootsList(params json.RawMessage, requestID interface{}) (*MCPResponse, error) {
	if !h.initialized {

		return nil, NewStateError("initialized", "not_initialized")
	}

	// Parse parameters (currently no parameters defined)
	var listParams RootsListRequest
	if len(params) > 0 {
		if err := json.Unmarshal(params, &listParams); err != nil {

			return nil, NewInvalidParams("failed to parse roots/list parameters", string(params))
		}
	}

	// Get all roots
	roots := h.rootManager.ListRoots()

	result := RootsListResponse{
		Roots: roots,
	}

	return NewResponse(requestID, result, nil)
}

// handleCancelled handles the cancel notification
func (h *StandardMethodHandler) handleCancelled(params json.RawMessage) error {
	// Parse cancel parameters
	var cancelParams struct {
		RequestID interface{} `json:"requestId"`
		Reason    string      `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(params, &cancelParams); err != nil {

		return NewInvalidParams("failed to parse cancel parameters", string(params))
	}

	// Handle cancellation
	// Implementation would track ongoing requests and cancel the specified one
	// For now, just acknowledge the cancellation
	fmt.Printf("Request %v cancelled: %s\n", cancelParams.RequestID, cancelParams.Reason)

	return nil
}

// IsInitialized returns whether the handler has been initialized
func (h *StandardMethodHandler) IsInitialized() bool {

	return h.initialized
}

// GetCapabilities returns the server capabilities
func (h *StandardMethodHandler) GetCapabilities() CapabilitiesOpts {

	return h.capabilities
}

// GetServerInfo returns the server information
func (h *StandardMethodHandler) GetServerInfo() ServerInfo {

	return h.serverInfo
}

// GetRootManager returns the root manager
func (h *StandardMethodHandler) GetRootManager() *RootManager {

	return h.rootManager
}

// SetCapability enables or disables a specific capability
func (h *StandardMethodHandler) SetCapability(capability string, enabled bool, options interface{}) error {
	switch capability {
	case "resources":
		if enabled {
			if h.capabilities.Resources == nil {
				h.capabilities.Resources = &ResourcesOpts{
					ListChanged: true,
					Subscribe:   true,
				}
			}
		} else {
			h.capabilities.Resources = nil
		}
	case "tools":
		if enabled {
			if h.capabilities.Tools == nil {
				h.capabilities.Tools = &ToolsOpts{
					ListChanged: true,
				}
			}
		} else {
			h.capabilities.Tools = nil
		}
	case "prompts":
		if enabled {
			if h.capabilities.Prompts == nil {
				h.capabilities.Prompts = &PromptsOpts{
					ListChanged: true,
				}
			}
		} else {
			h.capabilities.Prompts = nil
		}
	case "sampling":
		if enabled {
			if h.capabilities.Sampling == nil {
				h.capabilities.Sampling = &SamplingOpts{}
			}
		} else {
			h.capabilities.Sampling = nil
		}
	case "logging":
		if enabled {
			if h.capabilities.Logging == nil {
				h.capabilities.Logging = &LoggingOpts{}
			}
		} else {
			h.capabilities.Logging = nil
		}
	case "roots":
		if enabled {
			if h.capabilities.Roots == nil {
				h.capabilities.Roots = &RootsOpts{
					ListChanged: true,
				}
			}
		} else {
			h.capabilities.Roots = nil
		}
	default:

		return NewValidationError("capability", capability, "unknown capability")
	}

	return nil
}

// CreateInitializeResponse creates a proper initialize response
func CreateInitializeResponse(serverInfo ServerInfo, capabilities CapabilitiesOpts, roots []Root) InitializeResult {

	return InitializeResult{
		ProtocolVersion: MCPVersion,
		ServerInfo:      serverInfo,
		Capabilities:    capabilities,
		Roots:           roots,
	}
}

// CreatePingResponse creates a proper ping response
func CreatePingResponse() map[string]interface{} {

	return map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"status":    "ok",
		"version":   MCPVersion,
	}
}

// ValidateInitializeRequest validates an initialize request
func ValidateInitializeRequest(params InitializeParams) error {
	if params.ProtocolVersion == "" {

		return NewValidationError("protocolVersion", params.ProtocolVersion, "protocol version is required")
	}

	if params.ProtocolVersion != MCPVersion {

		return NewProtocolError(MCPVersion, params.ProtocolVersion)
	}

	if params.ClientInfo.Name == "" {

		return NewValidationError("clientInfo.name", params.ClientInfo.Name, "client name is required")
	}

	if params.ClientInfo.Version == "" {

		return NewValidationError("clientInfo.version", params.ClientInfo.Version, "client version is required")
	}

	// Validate roots if provided
	for i, root := range params.Roots {
		if root.URI == "" {

			return NewValidationError(fmt.Sprintf("roots[%d].uri", i), root.URI, "root URI is required")
		}
		if root.Name == "" {

			return NewValidationError(fmt.Sprintf("roots[%d].name", i), root.Name, "root name is required")
		}
	}

	return nil
}
