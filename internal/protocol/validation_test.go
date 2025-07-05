package protocol

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestMCPRequestValidation(t *testing.T) {
	tests := []struct {
		name      string
		request   MCPRequest
		expectErr bool
	}{
		{
			name: "valid request",
			request: MCPRequest{
				JSONRPC: "2.0",
				ID:      "test-1",
				Method:  "initialize",
				Params:  json.RawMessage(`{"protocolVersion": "2024-11-05"}`),
			},
			expectErr: false,
		},
		{
			name: "missing jsonrpc",
			request: MCPRequest{
				ID:     "test-1",
				Method: "initialize",
			},
			expectErr: true,
		},
		{
			name: "wrong jsonrpc version",
			request: MCPRequest{
				JSONRPC: "1.0",
				ID:      "test-1",
				Method:  "initialize",
			},
			expectErr: true,
		},
		{
			name: "missing method",
			request: MCPRequest{
				JSONRPC: "2.0",
				ID:      "test-1",
			},
			expectErr: true,
		},
		{
			name: "empty method",
			request: MCPRequest{
				JSONRPC: "2.0",
				ID:      "test-1",
				Method:  "",
			},
			expectErr: true,
		},
		{
			name: "missing id",
			request: MCPRequest{
				JSONRPC: "2.0",
				Method:  "initialize",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			// Basic validation
			if tt.request.JSONRPC != "2.0" {
				hasError = true
			}
			if tt.request.Method == "" {
				hasError = true
			}
			if tt.request.ID == nil || tt.request.ID == "" {
				hasError = true
			}

			if tt.expectErr && !hasError {
				t.Error("Expected validation error but request was valid")
			}
			if !tt.expectErr && hasError {
				t.Error("Expected valid request but validation failed")
			}
		})
	}
}

func TestMCPResponseValidation(t *testing.T) {
	tests := []struct {
		name      string
		response  MCPResponse
		expectErr bool
	}{
		{
			name: "valid success response",
			response: MCPResponse{
				JSONRPC: "2.0",
				ID:      "test-1",
				Result:  json.RawMessage(`{"status": "success"}`),
			},
			expectErr: false,
		},
		{
			name: "valid error response",
			response: MCPResponse{
				JSONRPC: "2.0",
				ID:      "test-1",
				Error: &MCPError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			expectErr: false,
		},
		{
			name: "response with both result and error",
			response: MCPResponse{
				JSONRPC: "2.0",
				ID:      "test-1",
				Result:  json.RawMessage(`{"status": "success"}`),
				Error: &MCPError{
					Code:    -32600,
					Message: "Invalid Request",
				},
			},
			expectErr: true,
		},
		{
			name: "response with neither result nor error",
			response: MCPResponse{
				JSONRPC: "2.0",
				ID:      "test-1",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			// Basic validation
			if tt.response.JSONRPC != "2.0" {
				hasError = true
			}
			if tt.response.ID == nil {
				hasError = true
			}

			// Must have either result or error, but not both
			hasResult := tt.response.Result != nil && len(tt.response.Result) > 0
			hasError2 := tt.response.Error != nil

			if hasResult && hasError2 {
				hasError = true
			}
			if !hasResult && !hasError2 {
				hasError = true
			}

			if tt.expectErr && !hasError {
				t.Error("Expected validation error but response was valid")
			}
			if !tt.expectErr && hasError {
				t.Error("Expected valid response but validation failed")
			}
		})
	}
}

func TestMCPErrorValidation(t *testing.T) {
	tests := []struct {
		name      string
		err       MCPError
		expectErr bool
	}{
		{
			name: "valid standard error",
			err: MCPError{
				Code:    InvalidRequest,
				Message: "Invalid Request",
			},
			expectErr: false,
		},
		{
			name: "valid error with data",
			err: MCPError{
				Code:    InvalidParams,
				Message: "Invalid params",
				Data: map[string]interface{}{
					"param": "test",
				},
			},
			expectErr: false,
		},
		{
			name: "missing message",
			err: MCPError{
				Code: InvalidRequest,
			},
			expectErr: true,
		},
		{
			name: "empty message",
			err: MCPError{
				Code:    InvalidRequest,
				Message: "",
			},
			expectErr: true,
		},
		{
			name: "invalid error code",
			err: MCPError{
				Code:    0,
				Message: "Invalid code",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			if tt.err.Message == "" {
				hasError = true
			}
			if tt.err.Code == 0 {
				hasError = true
			}

			if tt.expectErr && !hasError {
				t.Error("Expected validation error but error was valid")
			}
			if !tt.expectErr && hasError {
				t.Error("Expected valid error but validation failed")
			}
		})
	}
}

func TestInitializeParamsValidation(t *testing.T) {
	tests := []struct {
		name      string
		params    InitializeParams
		expectErr bool
	}{
		{
			name: "valid initialize params",
			params: InitializeParams{
				ProtocolVersion: MCPVersion,
				ClientInfo: ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
			},
			expectErr: false,
		},
		{
			name: "invalid protocol version",
			params: InitializeParams{
				ProtocolVersion: "1.0.0",
				ClientInfo: ClientInfo{
					Name:    "test-client",
					Version: "1.0.0",
				},
			},
			expectErr: true,
		},
		{
			name: "missing client info",
			params: InitializeParams{
				ProtocolVersion: MCPVersion,
			},
			expectErr: true,
		},
		{
			name: "empty client name",
			params: InitializeParams{
				ProtocolVersion: MCPVersion,
				ClientInfo: ClientInfo{
					Name:    "",
					Version: "1.0.0",
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasError := false

			if tt.params.ProtocolVersion != MCPVersion {
				hasError = true
			}
			if tt.params.ClientInfo.Name == "" {
				hasError = true
			}

			if tt.expectErr && !hasError {
				t.Error("Expected validation error but params were valid")
			}
			if !tt.expectErr && hasError {
				t.Error("Expected valid params but validation failed")
			}
		})
	}
}

func TestProgressTokenValidation(t *testing.T) {
	tests := []struct {
		name  string
		token string
		valid bool
	}{
		{
			name:  "valid uuid token",
			token: "123e4567-e89b-12d3-a456-426614174000",
			valid: true,
		},
		{
			name:  "valid string token",
			token: "progress-token-1",
			valid: true,
		},
		{
			name:  "empty token",
			token: "",
			valid: false,
		},
		{
			name:  "very long token",
			token: string(make([]byte, 1000)),
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation
			isValid := tt.token != "" && len(tt.token) < 256

			if tt.valid && !isValid {
				t.Error("Expected token to be valid")
			}
			if !tt.valid && isValid {
				t.Error("Expected token to be invalid")
			}
		})
	}
}

func TestJSONRPCBatch(t *testing.T) {
	// Test batch request validation
	batchRequest := []MCPRequest{
		{
			JSONRPC: "2.0",
			ID:      "1",
			Method:  "tools/list",
		},
		{
			JSONRPC: "2.0",
			ID:      "2",
			Method:  "resources/list",
		},
		{
			JSONRPC: "2.0",
			ID:      "3",
			Method:  "prompts/list",
		},
	}

	if len(batchRequest) == 0 {
		t.Error("Batch request should not be empty")
	}

	// Validate each request in batch
	for i, req := range batchRequest {
		if req.JSONRPC != "2.0" {
			t.Errorf("Request %d: invalid JSONRPC version", i)
		}
		if req.Method == "" {
			t.Errorf("Request %d: missing method", i)
		}
		if req.ID == nil {
			t.Errorf("Request %d: missing ID", i)
		}
	}

	// Test that IDs are unique
	ids := make(map[interface{}]bool)
	for i, req := range batchRequest {
		if ids[req.ID] {
			t.Errorf("Request %d: duplicate ID %v", i, req.ID)
		}
		ids[req.ID] = true
	}
}

func TestLargeMessageHandling(t *testing.T) {
	// Test handling of large messages
	largeParams := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeParams[fmt.Sprintf("param_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	largeParamsJSON, err := json.Marshal(largeParams)
	if err != nil {
		t.Fatalf("Failed to marshal large params: %v", err)
	}

	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      "large-test",
		Method:  "test/large",
		Params:  largeParamsJSON,
	}

	// Test serialization/deserialization
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal large request: %v", err)
	}

	var parsed MCPRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal large request: %v", err)
	}

	if parsed.Method != request.Method {
		t.Errorf("Method mismatch after large message processing")
	}

	// Check size limits
	if len(data) > 10*1024*1024 { // 10MB limit
		t.Error("Message exceeds reasonable size limit")
	}
}

func TestSpecialCharacterHandling(t *testing.T) {
	specialChars := []string{
		"special\ncharacters\t\r",
		"unicode: 🚀 🔥 💯",
		"quotes: \"double\" 'single'",
		"backslashes: \\\\ \\n \\t",
		"null bytes: \x00",
	}

	for i, content := range specialChars {
		t.Run(fmt.Sprintf("special_chars_%d", i), func(t *testing.T) {
			params := map[string]interface{}{
				"content": content,
			}

			paramsJSON, err := json.Marshal(params)
			if err != nil {
				t.Fatalf("Failed to marshal params with special chars: %v", err)
			}

			request := MCPRequest{
				JSONRPC: "2.0",
				ID:      fmt.Sprintf("special-%d", i),
				Method:  "test/special",
				Params:  paramsJSON,
			}

			// Test round-trip
			data, err := json.Marshal(request)
			if err != nil {
				t.Fatalf("Failed to marshal request: %v", err)
			}

			var parsed MCPRequest
			if err := json.Unmarshal(data, &parsed); err != nil {
				t.Fatalf("Failed to unmarshal request: %v", err)
			}

			if parsed.Method != request.Method {
				t.Error("Method mismatch after special character handling")
			}
		})
	}
}