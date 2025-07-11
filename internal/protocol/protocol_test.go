package protocol

import (
	"encoding/json"
	"testing"
)

func TestMCPMessage(t *testing.T) {
	tests := []struct {
		name     string
		message  MCPMessage
		expected string
	}{
		{
			name: "request message",
			message: MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-id",
				Method:  "test/method",
				Params:  json.RawMessage(`{"param": "value"}`),
			},
			expected: `{"jsonrpc":"2.0","id":"test-id","method":"test/method","params":{"param": "value"}}`,
		},
		{
			name: "response message",
			message: MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-id",
				Result:  json.RawMessage(`{"result": "success"}`),
			},
			expected: `{"jsonrpc":"2.0","id":"test-id","result":{"result": "success"}}`,
		},
		{
			name: "error message",
			message: MCPMessage{
				JSONRPC: "2.0",
				ID:      "test-id",
				Error: &MCPError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			expected: `{"jsonrpc":"2.0","id":"test-id","error":{"code":-32600,"message":"Invalid Request"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			// Parse back to verify structure
			var parsed MCPMessage
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if parsed.JSONRPC != tt.message.JSONRPC {
				t.Errorf("Expected JSONRPC %q, got %q", tt.message.JSONRPC, parsed.JSONRPC)
			}
			if parsed.ID != tt.message.ID {
				t.Errorf("Expected ID %v, got %v", tt.message.ID, parsed.ID)
			}
			if parsed.Method != tt.message.Method {
				t.Errorf("Expected Method %q, got %q", tt.message.Method, parsed.Method)
			}
		})
	}
}

func TestMCPError(t *testing.T) {
	tests := []struct {
		name     string
		err      MCPError
		expected string
	}{
		{
			name: "basic error",
			err: MCPError{
				Code:    -32600,
				Message: "Invalid Request",
			},
			expected: "MCP Error -32600: Invalid Request",
		},
		{
			name: "error with data",
			err: MCPError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    map[string]interface{}{"param": "invalid"},
			},
			expected: "MCP Error -32602: Invalid params (data: {\"param\":\"invalid\"})",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.expected {
				t.Errorf("Expected error message %q, got %q", tt.expected, tt.err.Error())
			}
		})
	}
}

func TestMCPRequest(t *testing.T) {
	id := "test-id"
	method := "test/method"
	params := json.RawMessage(`{"param": "value"}`)

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %q", req.JSONRPC)
	}
	if req.ID != id {
		t.Errorf("Expected ID %q, got %v", id, req.ID)
	}
	if req.Method != method {
		t.Errorf("Expected Method %q, got %q", method, req.Method)
	}
	if string(req.Params) != string(params) {
		t.Errorf("Expected Params %q, got %q", string(params), string(req.Params))
	}
}

func TestMCPResponse(t *testing.T) {
	id := "test-id"
	result := json.RawMessage(`{"result": "success"}`)

	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %q", resp.JSONRPC)
	}
	if resp.ID != id {
		t.Errorf("Expected ID %q, got %v", id, resp.ID)
	}
	if string(resp.Result) != string(result) {
		t.Errorf("Expected Result %q, got %q", string(result), string(resp.Result))
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
}

func TestMCPErrorResponse(t *testing.T) {
	id := "test-id"
	err := &MCPError{
		Code:    -32600,
		Message: "Invalid Request",
	}

	resp := MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC 2.0, got %q", resp.JSONRPC)
	}
	if resp.ID != id {
		t.Errorf("Expected ID %q, got %v", id, resp.ID)
	}
	if resp.Result != nil {
		t.Errorf("Expected no result, got %v", resp.Result)
	}
	if resp.Error != err {
		t.Errorf("Expected error %v, got %v", err, resp.Error)
	}
}

func TestCapabilities(t *testing.T) {
	tests := []struct {
		name       string
		capability Capability
		expected   string
	}{
		{
			name:       "tools capability",
			capability: ToolsCapability,
			expected:   "tools",
		},
		{
			name:       "resources capability",
			capability: ResourcesCapability,
			expected:   "resources",
		},
		{
			name:       "prompts capability",
			capability: PromptsCapability,
			expected:   "prompts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.capability) != tt.expected {
				t.Errorf("Expected capability %q, got %q", tt.expected, string(tt.capability))
			}
		})
	}
}

func TestMCPVersion(t *testing.T) {
	expectedVersion := "2024-11-05"
	if MCPVersion != expectedVersion {
		t.Errorf("Expected MCP version %q, got %q", expectedVersion, MCPVersion)
	}
}
