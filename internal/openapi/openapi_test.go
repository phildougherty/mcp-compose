package openapi

import (
	"strings"
	"testing"
)

func TestOpenAPISchema(t *testing.T) {
	schema := &OpenAPISchema{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       "Test API",
			Description: "Test Description",
			Version:     "1.0.0",
		},
		Servers: []Server{
			{
				URL:         "http://localhost:8080",
				Description: "Test Server",
			},
		},
		Paths:      make(map[string]PathItem),
		Components: Components{},
	}

	if schema.OpenAPI != "3.1.0" {
		t.Errorf("Expected OpenAPI version '3.1.0', got %s", schema.OpenAPI)
	}

	if schema.Info.Title != "Test API" {
		t.Errorf("Expected title 'Test API', got %s", schema.Info.Title)
	}

	if len(schema.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(schema.Servers))
	}

	if schema.Servers[0].URL != "http://localhost:8080" {
		t.Errorf("Expected server URL 'http://localhost:8080', got %s", schema.Servers[0].URL)
	}
}

func TestToolSpec(t *testing.T) {
	annotations := &ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: false,
		IdempotentHint:  true,
		OpenWorldHint:   false,
	}

	spec := ToolSpec{
		Type:        "function",
		Name:        "test-tool",
		Description: "A test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Test input",
				},
			},
		},
		Annotations: annotations,
	}

	if spec.Type != "function" {
		t.Errorf("Expected type 'function', got %s", spec.Type)
	}

	if spec.Name != "test-tool" {
		t.Errorf("Expected name 'test-tool', got %s", spec.Name)
	}

	if !spec.Annotations.ReadOnlyHint {
		t.Error("Expected ReadOnlyHint to be true")
	}

	if spec.Annotations.DestructiveHint {
		t.Error("Expected DestructiveHint to be false")
	}

	if !spec.Annotations.IdempotentHint {
		t.Error("Expected IdempotentHint to be true")
	}
}

func TestToolAnnotations(t *testing.T) {
	annotations := &ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: true,
		IdempotentHint:  false,
		OpenWorldHint:   true,
	}

	if !annotations.ReadOnlyHint {
		t.Error("Expected ReadOnlyHint to be true")
	}

	if !annotations.DestructiveHint {
		t.Error("Expected DestructiveHint to be true")
	}

	if annotations.IdempotentHint {
		t.Error("Expected IdempotentHint to be false")
	}

	if !annotations.OpenWorldHint {
		t.Error("Expected OpenWorldHint to be true")
	}
}

func TestGenerateOpenAPISchema(t *testing.T) {
	tools := []Tool{
		{
			Name:        "get-weather",
			Description: "Get current weather for a location",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"location": map[string]interface{}{
						"type":        "string",
						"description": "City name",
					},
					"units": map[string]interface{}{
						"type":        "string",
						"description": "Temperature units",
						"enum":        []interface{}{"celsius", "fahrenheit"},
					},
				},
				"required": []interface{}{"location"},
			},
			Annotations: &ToolAnnotations{
				ReadOnlyHint:   true,
				IdempotentHint: true,
			},
		},
		{
			Name:        "delete-file",
			Description: "Delete a file from the filesystem",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path to delete",
					},
				},
				"required": []interface{}{"path"},
			},
			Annotations: &ToolAnnotations{
				DestructiveHint: true,
			},
		},
	}

	schema, err := GenerateOpenAPISchema("test-server", tools)
	if err != nil {
		t.Fatalf("Failed to generate OpenAPI schema: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema to be generated")
	}

	// Test basic schema properties
	if schema.OpenAPI != "3.1.0" {
		t.Errorf("Expected OpenAPI version '3.1.0', got %s", schema.OpenAPI)
	}

	expectedTitle := "test-server MCP Server"
	if schema.Info.Title != expectedTitle {
		t.Errorf("Expected title %q, got %q", expectedTitle, schema.Info.Title)
	}

	if !strings.Contains(schema.Info.Description, "test-server") {
		t.Error("Expected description to contain server name")
	}

	// Test servers
	if len(schema.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(schema.Servers))
	}

	if schema.Servers[0].URL != "/" {
		t.Errorf("Expected server URL '/', got %s", schema.Servers[0].URL)
	}

	// Test paths
	if len(schema.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(schema.Paths))
	}

	// Check get-weather path
	weatherPath, exists := schema.Paths["/get-weather"]
	if !exists {
		t.Error("Expected /get-weather path to exist")
	} else {
		if weatherPath.Post.OperationID != "get_weather" {
			t.Errorf("Expected operation ID 'get_weather', got %s", weatherPath.Post.OperationID)
		}

		if weatherPath.Post.MCPMethod != "tools/call" {
			t.Errorf("Expected MCP method 'tools/call', got %s", weatherPath.Post.MCPMethod)
		}

		if weatherPath.Post.MCPHints == nil {
			t.Error("Expected MCP hints to be set")
		} else {
			if !weatherPath.Post.MCPHints.ReadOnlyHint {
				t.Error("Expected ReadOnlyHint to be true")
			}
			if !weatherPath.Post.MCPHints.IdempotentHint {
				t.Error("Expected IdempotentHint to be true")
			}
		}
	}

	// Check delete-file path
	deletePath, exists := schema.Paths["/delete-file"]
	if !exists {
		t.Error("Expected /delete-file path to exist")
	} else {
		if deletePath.Post.MCPHints == nil {
			t.Error("Expected MCP hints to be set")
		} else {
			if !deletePath.Post.MCPHints.DestructiveHint {
				t.Error("Expected DestructiveHint to be true")
			}
		}
	}

	// Test specs (for OpenWebUI compatibility)
	if len(schema.Specs) != 2 {
		t.Errorf("Expected 2 specs, got %d", len(schema.Specs))
	}

	// Find get-weather spec
	var weatherSpec *ToolSpec
	for i, spec := range schema.Specs {
		if spec.Name == "get-weather" {
			weatherSpec = &schema.Specs[i]
			break
		}
	}

	if weatherSpec == nil {
		t.Error("Expected to find get-weather spec")
	} else {
		if weatherSpec.Type != "function" {
			t.Errorf("Expected spec type 'function', got %s", weatherSpec.Type)
		}

		if !strings.Contains(weatherSpec.Description, "read-only") {
			t.Error("Expected description to contain 'read-only' hint")
		}

		if weatherSpec.Parameters == nil {
			t.Error("Expected parameters to be set")
		}

		if weatherSpec.Annotations == nil {
			t.Error("Expected annotations to be set")
		}
	}

	// Test components
	if schema.Components.Schemas == nil {
		t.Error("Expected schemas to be defined")
	}

	// Check request schemas
	if _, exists := schema.Components.Schemas["get-weatherRequest"]; !exists {
		t.Error("Expected get-weatherRequest schema to exist")
	}

	if _, exists := schema.Components.Schemas["delete-fileRequest"]; !exists {
		t.Error("Expected delete-fileRequest schema to exist")
	}

	// Check response schemas
	if _, exists := schema.Components.Schemas["get-weatherResponse"]; !exists {
		t.Error("Expected get-weatherResponse schema to exist")
	}

	// Check MCP standard schemas
	if _, exists := schema.Components.Schemas["MCPContent"]; !exists {
		t.Error("Expected MCPContent schema to exist")
	}

	if _, exists := schema.Components.Schemas["MCPError"]; !exists {
		t.Error("Expected MCPError schema to exist")
	}

	if _, exists := schema.Components.Schemas["MCPAnnotations"]; !exists {
		t.Error("Expected MCPAnnotations schema to exist")
	}

	// Test security schemes
	if _, exists := schema.Components.SecuritySchemes["MCPBearerAuth"]; !exists {
		t.Error("Expected MCPBearerAuth security scheme to exist")
	}
}

func TestBuildToolDescription(t *testing.T) {
	tests := []struct {
		name     string
		tool     Tool
		expected []string // Substrings that should be present
	}{
		{
			name: "tool without annotations",
			tool: Tool{
				Name:        "simple-tool",
				Description: "A simple tool",
			},
			expected: []string{"A simple tool"},
		},
		{
			name: "tool with read-only annotation",
			tool: Tool{
				Name:        "readonly-tool",
				Description: "A read-only tool",
				Annotations: &ToolAnnotations{
					ReadOnlyHint: true,
				},
			},
			expected: []string{"A read-only tool", "read-only"},
		},
		{
			name: "tool with destructive annotation",
			tool: Tool{
				Name:        "destructive-tool",
				Description: "A destructive tool",
				Annotations: &ToolAnnotations{
					DestructiveHint: true,
				},
			},
			expected: []string{"A destructive tool", "⚠️ potentially destructive"},
		},
		{
			name: "tool with multiple annotations",
			tool: Tool{
				Name:        "complex-tool",
				Description: "A complex tool",
				Annotations: &ToolAnnotations{
					ReadOnlyHint:   true,
					IdempotentHint: true,
					OpenWorldHint:  true,
				},
			},
			expected: []string{"A complex tool", "read-only", "idempotent", "accepts additional parameters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildToolDescription(tt.tool)
			
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected description to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestBuildAnnotationDescription(t *testing.T) {
	tests := []struct {
		name        string
		annotations *ToolAnnotations
		expected    []string
	}{
		{
			name: "read-only annotation",
			annotations: &ToolAnnotations{
				ReadOnlyHint: true,
			},
			expected: []string{"read-only", "will not modify"},
		},
		{
			name: "destructive annotation",
			annotations: &ToolAnnotations{
				DestructiveHint: true,
			},
			expected: []string{"⚠️ WARNING", "destructive operations"},
		},
		{
			name: "idempotent annotation",
			annotations: &ToolAnnotations{
				IdempotentHint: true,
			},
			expected: []string{"idempotent", "same effect"},
		},
		{
			name: "open world annotation",
			annotations: &ToolAnnotations{
				OpenWorldHint: true,
			},
			expected: []string{"additional parameters", "beyond those specified"},
		},
		{
			name: "multiple annotations",
			annotations: &ToolAnnotations{
				ReadOnlyHint:    true,
				DestructiveHint: true,
				IdempotentHint:  true,
				OpenWorldHint:   true,
			},
			expected: []string{"read-only", "⚠️ WARNING", "idempotent", "additional parameters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAnnotationDescription(tt.annotations)
			
			for _, expected := range tt.expected {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected annotation description to contain %q, got: %s", expected, result)
				}
			}
		})
	}
}

func TestConvertJSONSchemaToOpenAPI(t *testing.T) {
	jsonSchema := map[string]interface{}{
		"type": "object",
		"description": "Test schema",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type": "string",
				"description": "Name field",
			},
			"age": map[string]interface{}{
				"type": "integer",
				"description": "Age field",
			},
			"tags": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"required": []interface{}{"name"},
	}

	schema := convertJSONSchemaToOpenAPI(jsonSchema)

	if schema.Type != "object" {
		t.Errorf("Expected type 'object', got %s", schema.Type)
	}

	if schema.Description != "Test schema" {
		t.Errorf("Expected description 'Test schema', got %s", schema.Description)
	}

	if len(schema.Properties) != 3 {
		t.Errorf("Expected 3 properties, got %d", len(schema.Properties))
	}

	// Test name property
	nameProperty, exists := schema.Properties["name"]
	if !exists {
		t.Error("Expected 'name' property to exist")
	} else {
		if nameProperty.Type != "string" {
			t.Errorf("Expected name type 'string', got %s", nameProperty.Type)
		}
		if nameProperty.Description != "Name field" {
			t.Errorf("Expected name description 'Name field', got %s", nameProperty.Description)
		}
	}

	// Test tags property (array)
	tagsProperty, exists := schema.Properties["tags"]
	if !exists {
		t.Error("Expected 'tags' property to exist")
	} else {
		if tagsProperty.Type != "array" {
			t.Errorf("Expected tags type 'array', got %s", tagsProperty.Type)
		}
		if tagsProperty.Items == nil {
			t.Error("Expected tags items to be defined")
		} else if tagsProperty.Items.Type != "string" {
			t.Errorf("Expected tags items type 'string', got %s", tagsProperty.Items.Type)
		}
	}

	// Test required fields
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field, got %d", len(schema.Required))
	}

	if schema.Required[0] != "name" {
		t.Errorf("Expected required field 'name', got %s", schema.Required[0])
	}
}

func TestGenerateOpenAPISchemaEmptyTools(t *testing.T) {
	tools := []Tool{}
	
	schema, err := GenerateOpenAPISchema("empty-server", tools)
	if err != nil {
		t.Fatalf("Failed to generate OpenAPI schema for empty tools: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected schema to be generated")
	}

	if len(schema.Paths) != 0 {
		t.Errorf("Expected 0 paths for empty tools, got %d", len(schema.Paths))
	}

	if len(schema.Specs) != 0 {
		t.Errorf("Expected 0 specs for empty tools, got %d", len(schema.Specs))
	}

	// Should still have standard MCP schemas
	if _, exists := schema.Components.Schemas["MCPContent"]; !exists {
		t.Error("Expected MCPContent schema to exist even with empty tools")
	}
}

func TestOperationSecurity(t *testing.T) {
	tools := []Tool{
		{
			Name:        "secure-tool",
			Description: "A tool requiring authentication",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{},
			},
		},
	}

	schema, err := GenerateOpenAPISchema("secure-server", tools)
	if err != nil {
		t.Fatalf("Failed to generate OpenAPI schema: %v", err)
	}

	securePath, exists := schema.Paths["/secure-tool"]
	if !exists {
		t.Fatal("Expected /secure-tool path to exist")
	}

	if len(securePath.Post.Security) == 0 {
		t.Error("Expected security requirements to be set")
	}

	if _, exists := securePath.Post.Security[0]["MCPBearerAuth"]; !exists {
		t.Error("Expected MCPBearerAuth security requirement")
	}
}

func TestSchemaComponents(t *testing.T) {
	schema := &OpenAPISchema{
		Components: Components{
			Schemas: make(map[string]Schema),
		},
	}

	addMCPSchemas(schema)

	expectedSchemas := []string{
		"MCPContent",
		"MCPError", 
		"MCPMetadata",
		"MCPAnnotations",
	}

	for _, schemaName := range expectedSchemas {
		if _, exists := schema.Components.Schemas[schemaName]; !exists {
			t.Errorf("Expected schema %s to exist", schemaName)
		}
	}

	// Test MCPContent schema structure
	mcpContent := schema.Components.Schemas["MCPContent"]
	if mcpContent.Type != "object" {
		t.Errorf("Expected MCPContent type 'object', got %s", mcpContent.Type)
	}

	if len(mcpContent.Required) == 0 {
		t.Error("Expected MCPContent to have required fields")
	}

	if !contains(mcpContent.Required, "type") {
		t.Error("Expected 'type' to be required in MCPContent")
	}

	// Test MCPError schema structure
	mcpError := schema.Components.Schemas["MCPError"]
	if len(mcpError.Required) == 0 {
		t.Error("Expected MCPError to have required fields")
	}

	if !contains(mcpError.Required, "code") || !contains(mcpError.Required, "message") {
		t.Error("Expected 'code' and 'message' to be required in MCPError")
	}
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}