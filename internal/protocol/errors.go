// internal/protocol/errors.go
package protocol

import (
	"encoding/json"
	"fmt"
)

// Standard JSON-RPC 2.0 error codes
const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

// MCP-specific errors (following JSON-RPC reserved range)
const (
	// Implementation-specific errors
	RequestFailed    = -32000
	RequestCancelled = -32001
	RequestTimeout   = -32002

	// Extended MCP errors (using safe range > -32000)
	TransportError      = -31999
	SessionError        = -31998
	CapabilityError     = -31997
	ProtocolError       = -31996
	AuthenticationError = -31995
	AuthorizationError  = -31994
	RateLimitError      = -31993
	ResourceError       = -31992
	ValidationError     = -31991
	ExecutionError      = -31990
	StateError          = -31989
	ConfigurationError  = -31988
)

// MCPError represents a complete MCP protocol error
type MCPError struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *MCPError) Error() string {
	if len(e.Data) > 0 {
		dataJson, _ := json.Marshal(e.Data)
		return fmt.Sprintf("MCP Error %d: %s (data: %s)", e.Code, e.Message, string(dataJson))
	}
	return fmt.Sprintf("MCP Error %d: %s", e.Code, e.Message)
}

// NewMCPError creates a new MCP error with optional data
func NewMCPError(code int, message string, data ...map[string]interface{}) *MCPError {
	err := &MCPError{
		Code:    code,
		Message: message,
	}
	if len(data) > 0 && data[0] != nil {
		err.Data = data[0]
	}
	return err
}

// Standard JSON-RPC error constructors
func NewParseError(details string) *MCPError {
	return NewMCPError(ParseError, "Parse error", map[string]interface{}{
		"details": details,
		"type":    "parse_error",
	})
}

func NewInvalidRequest(details string) *MCPError {
	return NewMCPError(InvalidRequest, "Invalid Request", map[string]interface{}{
		"details": details,
		"type":    "invalid_request",
	})
}

func NewMethodNotFound(method string) *MCPError {
	return NewMCPError(MethodNotFound, fmt.Sprintf("Method not found: %s", method), map[string]interface{}{
		"method": method,
		"type":   "method_not_found",
	})
}

func NewInvalidParams(details string, params interface{}) *MCPError {
	data := map[string]interface{}{
		"details": details,
		"type":    "invalid_params",
	}
	if params != nil {
		data["provided_params"] = params
	}
	return NewMCPError(InvalidParams, "Invalid params", data)
}

func NewInternalError(details string) *MCPError {
	return NewMCPError(InternalError, "Internal error", map[string]interface{}{
		"details": details,
		"type":    "internal_error",
	})
}

// MCP-specific error constructors using safe error codes
func NewRequestTimeout(operation string, timeout string) *MCPError {
	return NewMCPError(RequestTimeout, "Request timed out", map[string]interface{}{
		"operation": operation,
		"timeout":   timeout,
		"type":      "timeout_error",
	})
}

func NewTransportError(transport string, details string) *MCPError {
	return NewMCPError(TransportError, "Transport error", map[string]interface{}{
		"transport": transport,
		"details":   details,
		"type":      "transport_error",
	})
}

func NewSessionError(sessionId string, details string) *MCPError {
	return NewMCPError(SessionError, "Session error", map[string]interface{}{
		"session_id": sessionId,
		"details":    details,
		"type":       "session_error",
	})
}

func NewCapabilityError(capability string, details string) *MCPError {
	return NewMCPError(CapabilityError, "Capability error", map[string]interface{}{
		"capability": capability,
		"details":    details,
		"type":       "capability_error",
	})
}

func NewProtocolError(expectedVersion string, actualVersion string) *MCPError {
	return NewMCPError(ProtocolError, "Protocol version mismatch", map[string]interface{}{
		"expected_version": expectedVersion,
		"actual_version":   actualVersion,
		"type":             "protocol_error",
	})
}

func NewAuthenticationError(details string) *MCPError {
	return NewMCPError(AuthenticationError, "Authentication failed", map[string]interface{}{
		"details": details,
		"type":    "authentication_error",
	})
}

func NewAuthorizationError(resource string, action string) *MCPError {
	return NewMCPError(AuthorizationError, "Authorization failed", map[string]interface{}{
		"resource": resource,
		"action":   action,
		"type":     "authorization_error",
	})
}

func NewRateLimitError(limit string, window string) *MCPError {
	return NewMCPError(RateLimitError, "Rate limit exceeded", map[string]interface{}{
		"limit":  limit,
		"window": window,
		"type":   "rate_limit_error",
	})
}

func NewResourceError(resource string, operation string, details string) *MCPError {
	return NewMCPError(ResourceError, "Resource access error", map[string]interface{}{
		"resource":  resource,
		"operation": operation,
		"details":   details,
		"type":      "resource_error",
	})
}

func NewValidationError(field string, value interface{}, constraint string) *MCPError {
	return NewMCPError(ValidationError, "Input validation failed", map[string]interface{}{
		"field":      field,
		"value":      value,
		"constraint": constraint,
		"type":       "validation_error",
	})
}

func NewExecutionError(tool string, details string) *MCPError {
	return NewMCPError(ExecutionError, "Execution failed", map[string]interface{}{
		"tool":    tool,
		"details": details,
		"type":    "execution_error",
	})
}

func NewStateError(expectedState string, actualState string) *MCPError {
	return NewMCPError(StateError, "Invalid server state", map[string]interface{}{
		"expected_state": expectedState,
		"actual_state":   actualState,
		"type":           "state_error",
	})
}

func NewConfigurationError(component string, details string) *MCPError {
	return NewMCPError(ConfigurationError, "Configuration error", map[string]interface{}{
		"component": component,
		"details":   details,
		"type":      "configuration_error",
	})
}

// IsRetryable returns true if the error is potentially retryable
func (e *MCPError) IsRetryable() bool {
	switch e.Code {
	case RequestTimeout, TransportError, SessionError, RateLimitError:
		return true
	default:
		return false
	}
}

// IsTemporary returns true if the error is likely temporary
func (e *MCPError) IsTemporary() bool {
	return e.IsRetryable()
}

// GetRetryDelay suggests a delay before retry (in seconds)
func (e *MCPError) GetRetryDelay() int {
	switch e.Code {
	case RequestTimeout, TransportError:
		return 5
	case SessionError:
		return 30
	case RateLimitError:
		return 60
	default:
		return 0
	}
}
