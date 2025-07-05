package protocol

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestMCPProtocolBasics(t *testing.T) {
	// Test the basic MCP protocol structures
	t.Run("initialize_message", func(t *testing.T) {
		// Test client initialization request
		initRequest := MCPRequest{
			JSONRPC: "2.0",
			ID:      "init-1",
			Method:  "initialize",
			Params:  json.RawMessage(`{"protocolVersion": "2024-11-05"}`),
		}

		if initRequest.Method != "initialize" {
			t.Errorf("Expected method 'initialize', got %q", initRequest.Method)
		}

		if initRequest.JSONRPC != "2.0" {
			t.Errorf("Expected JSONRPC '2.0', got %q", initRequest.JSONRPC)
		}

		// Test server response
		initResponse := MCPResponse{
			JSONRPC: "2.0",
			ID:      initRequest.ID,
			Result:  json.RawMessage(`{"protocolVersion": "2024-11-05", "capabilities": {}}`),
		}

		if initResponse.ID != initRequest.ID {
			t.Errorf("Expected response ID to match request ID")
		}
	})

	t.Run("tools_list_message", func(t *testing.T) {
		// Test tools/list request
		listRequest := MCPRequest{
			JSONRPC: "2.0",
			ID:      "tools-1",
			Method:  "tools/list",
			Params:  json.RawMessage(`{}`),
		}

		if listRequest.Method != "tools/list" {
			t.Errorf("Expected method 'tools/list', got %q", listRequest.Method)
		}

		// Mock tools response
		toolsResponse := MCPResponse{
			JSONRPC: "2.0",
			ID:      listRequest.ID,
			Result:  json.RawMessage(`{"tools": []}`),
		}

		if toolsResponse.Error != nil {
			t.Errorf("Expected no error in tools list response")
		}
	})

	t.Run("resources_list_message", func(t *testing.T) {
		// Test resources/list request
		listRequest := MCPRequest{
			JSONRPC: "2.0",
			ID:      "resources-1",
			Method:  "resources/list",
			Params:  json.RawMessage(`{}`),
		}

		if listRequest.Method != "resources/list" {
			t.Errorf("Expected method 'resources/list', got %q", listRequest.Method)
		}

		// Mock resources response
		resourcesResponse := MCPResponse{
			JSONRPC: "2.0",
			ID:      listRequest.ID,
			Result:  json.RawMessage(`{"resources": []}`),
		}

		if resourcesResponse.Error != nil {
			t.Errorf("Expected no error in resources list response")
		}
	})

	t.Run("error_handling", func(t *testing.T) {
		// Test invalid method
		invalidRequest := MCPRequest{
			JSONRPC: "2.0",
			ID:      "error-1",
			Method:  "invalid/method",
			Params:  json.RawMessage(`{}`),
		}

		errorResponse := MCPResponse{
			JSONRPC: "2.0",
			ID:      invalidRequest.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}

		if errorResponse.Error == nil {
			t.Error("Expected error response")
		}
		if errorResponse.Error.Code != -32601 {
			t.Errorf("Expected error code -32601, got %d", errorResponse.Error.Code)
		}
	})
}

func TestMCPNotifications(t *testing.T) {
	// Test notification messages
	t.Run("progress_notification", func(t *testing.T) {
		progressNotification := MCPNotification{
			JSONRPC: "2.0",
			Method:  "notifications/progress",
			Params:  json.RawMessage(`{"progressToken": "token-1", "progress": 50, "total": 100}`),
		}

		if progressNotification.Method != "notifications/progress" {
			t.Errorf("Expected method 'notifications/progress', got %q", progressNotification.Method)
		}
	})

	t.Run("resource_updated_notification", func(t *testing.T) {
		updateNotification := MCPNotification{
			JSONRPC: "2.0",
			Method:  "notifications/resources/updated",
			Params:  json.RawMessage(`{"uri": "file:///test.txt"}`),
		}

		if updateNotification.Method != "notifications/resources/updated" {
			t.Errorf("Expected method 'notifications/resources/updated', got %q", updateNotification.Method)
		}
	})
}

func TestMCPMessageSerialization(t *testing.T) {
	// Test JSON serialization/deserialization
	t.Run("request_serialization", func(t *testing.T) {
		req := MCPRequest{
			JSONRPC: "2.0",
			ID:      "test-1",
			Method:  "test/method",
			Params:  json.RawMessage(`{"param": "value"}`),
		}

		data, err := json.Marshal(req)
		if err != nil {
			t.Fatalf("Failed to marshal request: %v", err)
		}

		var parsed MCPRequest
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal request: %v", err)
		}

		if parsed.Method != req.Method {
			t.Errorf("Expected method %q, got %q", req.Method, parsed.Method)
		}
	})

	t.Run("response_serialization", func(t *testing.T) {
		resp := MCPResponse{
			JSONRPC: "2.0",
			ID:      "test-1",
			Result:  json.RawMessage(`{"result": "success"}`),
		}

		data, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("Failed to marshal response: %v", err)
		}

		var parsed MCPResponse
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("Failed to unmarshal response: %v", err)
		}

		if parsed.ID != resp.ID {
			t.Errorf("Expected ID %v, got %v", resp.ID, parsed.ID)
		}
	})
}

func TestContextTimeout(t *testing.T) {
	// Test context timeout handling
	t.Run("context_timeout", func(t *testing.T) {
		// Create a context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Simulate a long-running operation
		done := make(chan bool, 1)
		go func() {
			time.Sleep(200 * time.Millisecond)
			done <- true
		}()

		select {
		case <-ctx.Done():
			// Expected timeout
			if ctx.Err() != context.DeadlineExceeded {
				t.Errorf("Expected deadline exceeded error, got %v", ctx.Err())
			}
		case <-done:
			t.Error("Expected timeout but operation completed")
		}
	})
}
