// pkg/protocol/protocol.go
package protocol

import (
	"encoding/json"
	"fmt"
)

// MCPCapabilities represents MCP capabilities
type MCPCapabilities struct {
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Sampling  *SamplingCapability  `json:"sampling,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
	Subscribe   bool `json:"subscribe,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability represents prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// SamplingCapability represents sampling capability
type SamplingCapability struct {
}

// LoggingCapability represents logging capability
type LoggingCapability struct {
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

// InitializeRequest represents an initialize request
type InitializeRequest struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResponse represents an initialize response
type InitializeResponse struct {
	ProtocolVersion string          `json:"protocolVersion"`
	Capabilities    MCPCapabilities `json:"capabilities"`
	ServerInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
	Instructions string `json:"instructions,omitempty"`
}

// ValidateCapabilities checks if the server capabilities match the given capabilities
func ValidateCapabilities(serverCapabilities MCPCapabilities, requiredCapabilities []string) error {
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
