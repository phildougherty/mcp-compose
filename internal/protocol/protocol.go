// internal/protocol/protocol.go
package protocol

import (
	"encoding/json"
	"fmt"
	"io"
)

// MCPVersion represents the Model Context Protocol version
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
}

// MCPRequest represents an MCP request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPNotification represents an MCP notification
type MCPNotification struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
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
)

// InitializeParams represents the parameters for an initialize request
type InitializeParams struct {
	ProtocolVersion string           `json:"protocolVersion"`
	Capabilities    CapabilitiesOpts `json:"capabilities"`
	ClientInfo      ClientInfo       `json:"clientInfo"`
}

// InitializeResult represents the result of an initialize request
type InitializeResult struct {
	ProtocolVersion string           `json:"protocolVersion"`
	ServerInfo      ServerInfo       `json:"serverInfo"`
	Capabilities    CapabilitiesOpts `json:"capabilities"`
	Instructions    string           `json:"instructions,omitempty"`
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

// CapabilitiesOpts represents MCP capability options
type CapabilitiesOpts struct {
	Resources *ResourcesOpts `json:"resources,omitempty"`
	Tools     *ToolsOpts     `json:"tools,omitempty"`
	Prompts   *PromptsOpts   `json:"prompts,omitempty"`
	Sampling  *SamplingOpts  `json:"sampling,omitempty"`
	Logging   *LoggingOpts   `json:"logging,omitempty"`
}

// ResourcesOpts represents resources capability options
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

// Transport defines the interface for MCP transports
type Transport interface {
	// Send sends an MCP message
	Send(msg MCPMessage) error
	// Receive receives an MCP message
	Receive() (MCPMessage, error)
	// Close closes the transport
	Close() error
}

// StdioTransport implements the stdio transport
type StdioTransport struct {
	reader *json.Decoder
	writer *json.Encoder
}

// NewStdioTransport creates a new stdio transport
func NewStdioTransport(r io.Reader, w io.Writer) *StdioTransport {
	return &StdioTransport{
		reader: json.NewDecoder(r),
		writer: json.NewEncoder(w),
	}
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

// NewRequest creates a new MCP request
func NewRequest(id interface{}, method string, params interface{}) (*MCPRequest, error) {
	var paramsBytes json.RawMessage
	if params != nil {
		var err error
		paramsBytes, err = json.Marshal(params)
		if err != nil {
			return nil, err
		}
	}
	return &MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  paramsBytes,
	}, nil
}

// NewResponse creates a new MCP response
func NewResponse(id interface{}, result interface{}, err *MCPError) (*MCPResponse, error) {
	var resultBytes json.RawMessage
	if result != nil && err == nil {
		var marshalErr error
		resultBytes, marshalErr = json.Marshal(result)
		if marshalErr != nil {
			return nil, marshalErr
		}
	}
	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  resultBytes,
		Error:   err,
	}, nil
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

// NewError creates a new MCP error
func NewError(code int, message string, data interface{}) (*MCPError, error) {
	var dataBytes json.RawMessage
	if data != nil {
		var err error
		dataBytes, err = json.Marshal(data)
		if err != nil {
			return nil, err
		}
	}
	return &MCPError{
		Code:    code,
		Message: message,
		Data:    dataBytes,
	}, nil
}

// ValidateCapabilities checks if the server capabilities match the given capabilities
func ValidateCapabilities(serverCapabilities CapabilitiesOpts, requiredCapabilities []string) error {
	for _, cap := range requiredCapabilities {
		switch cap {
		case "resources":
			if serverCapabilities.Resources == nil {
				return fmt.Errorf("server does not support resources capability")
			}
		case "tools":
			if serverCapabilities.Tools == nil {
				return fmt.Errorf("server does not support tools capability")
			}
		case "prompts":
			if serverCapabilities.Prompts == nil {
				return fmt.Errorf("server does not support prompts capability")
			}
		case "sampling":
			if serverCapabilities.Sampling == nil {
				return fmt.Errorf("server does not support sampling capability")
			}
		case "logging":
			if serverCapabilities.Logging == nil {
				return fmt.Errorf("server does not support logging capability")
			}
		}
	}
	return nil
}

// CapabilityOptsFromConfig converts capability configuration to MCP capability options
func CapabilityOptsFromConfig(capOpt interface{}) CapabilitiesOpts {
	// This method would normally parse your config format into capability options
	// For a simple implementation, just return a default set of capabilities
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
	}
}
