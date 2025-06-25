// internal/openapi/schema.go
package openapi

import (
	"fmt"
	"strings"
)

// OpenAPISchema represents an OpenAPI 3.0 schema
type OpenAPISchema struct {
	OpenAPI    string                `json:"openapi"`
	Info       Info                  `json:"info"`
	Servers    []Server              `json:"servers"`
	Paths      map[string]PathItem   `json:"paths"`
	Specs      []ToolSpec            `json:"specs,omitempty"` // Added specs field for OpenWebUI
	Components Components            `json:"components"`
	Security   []map[string][]string `json:"security,omitempty"`
}

// Enhanced ToolSpec with full MCP annotations
type ToolSpec struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
	// MCP Tool Annotations
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations represents MCP tool annotations
type ToolAnnotations struct {
	ReadOnlyHint    bool `json:"readOnlyHint,omitempty"`    // Tool is read-only
	DestructiveHint bool `json:"destructiveHint,omitempty"` // Tool may be destructive
	IdempotentHint  bool `json:"idempotentHint,omitempty"`  // Tool is idempotent
	OpenWorldHint   bool `json:"openWorldHint,omitempty"`   // Tool accepts arbitrary parameters
}

type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

type PathItem struct {
	Post Operation `json:"post,omitempty"`
}

type Operation struct {
	Summary     string                `json:"summary"`
	Description string                `json:"description,omitempty"`
	OperationID string                `json:"operationId"`
	RequestBody RequestBody           `json:"requestBody,omitempty"`
	Responses   map[string]Response   `json:"responses"`
	Security    []map[string][]string `json:"security,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	// MCP Operation Extensions
	MCPMethod string           `json:"x-mcp-method,omitempty"`
	MCPHints  *ToolAnnotations `json:"x-mcp-hints,omitempty"`
}

type RequestBody struct {
	Required bool                 `json:"required"`
	Content  map[string]MediaType `json:"content"`
}

type MediaType struct {
	Schema Schema `json:"schema"`
}

type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

type Components struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

type Schema struct {
	Type                 string            `json:"type,omitempty"`
	Properties           map[string]Schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Items                *Schema           `json:"items,omitempty"`
	Description          string            `json:"description,omitempty"`
	Ref                  string            `json:"$ref,omitempty"`
	AdditionalProperties *Schema           `json:"additionalProperties,omitempty"`
}

type SecurityScheme struct {
	Type         string `json:"type"`
	Scheme       string `json:"scheme,omitempty"`
	BearerFormat string `json:"bearerFormat,omitempty"`
	Name         string `json:"name,omitempty"`
	In           string `json:"in,omitempty"`
}

// Enhanced Tool with MCP annotations
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
	// MCP Tool Annotations
	Annotations *ToolAnnotations `json:"annotations,omitempty"`
}

// GenerateOpenAPISchema generates an OpenAPI schema for MCP tools with full annotations
func GenerateOpenAPISchema(serverName string, tools []Tool) (*OpenAPISchema, error) {
	schema := &OpenAPISchema{
		OpenAPI: "3.1.0",
		Info: Info{
			Title:       fmt.Sprintf("%s MCP Server", serverName),
			Description: fmt.Sprintf("MCP Server with Model Context Protocol compliance\n\n%s provides tools with full MCP annotation support including read-only hints, destructive operation warnings, and idempotency guarantees.", serverName),
			Version:     "1.0.0",
		},
		Servers: []Server{
			{
				URL:         "/",
				Description: fmt.Sprintf("%s MCP Server with annotation metadata", serverName),
			},
		},
		Paths: make(map[string]PathItem),
		Components: Components{
			Schemas: make(map[string]Schema),
			SecuritySchemes: map[string]SecurityScheme{
				"MCPBearerAuth": {
					Type:   "http",
					Scheme: "bearer",
				},
			},
		},
	}

	// Create specs entries for OpenWebUI compatibility with annotations
	specs := make([]ToolSpec, 0, len(tools))

	// Add paths for each tool with MCP annotations
	for _, tool := range tools {
		// Clean the operationId to ensure it's a valid identifier
		operationId := strings.ReplaceAll(tool.Name, "-", "_")
		operationId = strings.ReplaceAll(operationId, " ", "_")

		// Build operation with MCP hints
		operation := Operation{
			Summary:     tool.Name,
			Description: buildToolDescription(tool),
			OperationID: operationId,
			Tags:        []string{serverName, "mcp-tools"},
			MCPMethod:   "tools/call",
			MCPHints:    tool.Annotations,
			RequestBody: RequestBody{
				Required: true,
				Content: map[string]MediaType{
					"application/json": {
						Schema: Schema{
							Ref: fmt.Sprintf("#/components/schemas/%sRequest", tool.Name),
						},
					},
				},
			},
			Responses: map[string]Response{
				"200": {
					Description: "Successful MCP tool execution",
					Content: map[string]MediaType{
						"application/json": {
							Schema: Schema{
								Ref: fmt.Sprintf("#/components/schemas/%sResponse", tool.Name),
							},
						},
					},
				},
				"400": {
					Description: "Invalid parameters or MCP protocol error",
					Content: map[string]MediaType{
						"application/json": {
							Schema: Schema{
								Ref: "#/components/schemas/MCPError",
							},
						},
					},
				},
				"401": {
					Description: "Unauthorized - missing or invalid MCP session",
				},
				"500": {
					Description: "MCP server error or tool execution failure",
				},
			},
			Security: []map[string][]string{
				{
					"MCPBearerAuth": {},
				},
			},
		}

		pathItem := PathItem{
			Post: operation,
		}

		// Set the path to match OpenWebUI's expectations
		schema.Paths["/"+tool.Name] = pathItem

		// Add enhanced request schema with MCP metadata
		requestSchema := convertJSONSchemaToOpenAPI(tool.InputSchema)
		if tool.Annotations != nil {
			// Add MCP annotation information to schema description
			if requestSchema.Description == "" {
				requestSchema.Description = "MCP tool parameters"
			}
			requestSchema.Description += buildAnnotationDescription(tool.Annotations)
		}
		schema.Components.Schemas[tool.Name+"Request"] = requestSchema

		// Add enhanced response schema with MCP content types
		responseSchema := Schema{
			Type: "object",
			Properties: map[string]Schema{
				"content": {
					Type: "array",
					Items: &Schema{
						Ref: "#/components/schemas/MCPContent",
					},
					Description: "MCP tool execution results",
				},
				"isError": {
					Type:        "boolean",
					Description: "Whether the tool execution resulted in an error",
				},
				"_meta": {
					Ref:         "#/components/schemas/MCPMetadata",
					Description: "MCP execution metadata",
				},
			},
		}
		schema.Components.Schemas[tool.Name+"Response"] = responseSchema

		// Create enhanced tool spec entry for OpenWebUI with annotations
		spec := ToolSpec{
			Type:        "function",
			Name:        tool.Name,
			Description: buildToolDescription(tool),
			Annotations: tool.Annotations,
		}

		// Use the input schema as parameters
		if tool.InputSchema != nil {
			spec.Parameters = tool.InputSchema
		} else {
			spec.Parameters = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
				"required":   []string{},
			}
		}

		specs = append(specs, spec)
	}

	// Add standard MCP schemas
	addMCPSchemas(schema)

	// Add the specs to the schema
	schema.Specs = specs

	return schema, nil
}

// buildToolDescription creates enhanced description with MCP annotations
func buildToolDescription(tool Tool) string {
	desc := tool.Description
	if tool.Annotations != nil {
		var hints []string
		if tool.Annotations.ReadOnlyHint {
			hints = append(hints, "read-only")
		}
		if tool.Annotations.DestructiveHint {
			hints = append(hints, "⚠️ potentially destructive")
		}
		if tool.Annotations.IdempotentHint {
			hints = append(hints, "idempotent")
		}
		if tool.Annotations.OpenWorldHint {
			hints = append(hints, "accepts additional parameters")
		}

		if len(hints) > 0 {
			desc += fmt.Sprintf("\n\nMCP Hints: %s", strings.Join(hints, ", "))
		}
	}
	return desc
}

// buildAnnotationDescription adds MCP annotation info to schema descriptions
func buildAnnotationDescription(annotations *ToolAnnotations) string {
	var parts []string
	if annotations.ReadOnlyHint {
		parts = append(parts, "This tool is read-only and will not modify system state.")
	}
	if annotations.DestructiveHint {
		parts = append(parts, "⚠️ WARNING: This tool may perform destructive operations.")
	}
	if annotations.IdempotentHint {
		parts = append(parts, "This tool is idempotent - repeated calls with the same parameters will have the same effect.")
	}
	if annotations.OpenWorldHint {
		parts = append(parts, "This tool accepts additional parameters beyond those specified in the schema.")
	}

	if len(parts) > 0 {
		return "\n\nMCP Annotations:\n" + strings.Join(parts, "\n")
	}
	return ""
}

// addMCPSchemas adds standard MCP protocol schemas
func addMCPSchemas(schema *OpenAPISchema) {
	// MCP Content schema
	schema.Components.Schemas["MCPContent"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"type": {
				Type:        "string",
				Description: "Content type (text, image, resource)",
			},
			"text": {
				Type:        "string",
				Description: "Text content",
			},
			"data": {
				Type:        "string",
				Description: "Base64 encoded binary data for images",
			},
			"mimeType": {
				Type:        "string",
				Description: "MIME type for binary content",
			},
			"uri": {
				Type:        "string",
				Description: "Resource URI",
			},
		},
		Required: []string{"type"},
	}

	// MCP Error schema
	schema.Components.Schemas["MCPError"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"code": {
				Type:        "integer",
				Description: "MCP error code",
			},
			"message": {
				Type:        "string",
				Description: "Human-readable error message",
			},
			"data": {
				Type:                 "object",
				Description:          "Additional error data",
				AdditionalProperties: &Schema{Type: "string"},
			},
		},
		Required: []string{"code", "message"},
	}

	// MCP Metadata schema
	schema.Components.Schemas["MCPMetadata"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"progressToken": {
				Type:        "string",
				Description: "Progress tracking token",
			},
			"annotations": {
				Ref:         "#/components/schemas/MCPAnnotations",
				Description: "Tool execution annotations",
			},
		},
	}

	// MCP Annotations schema
	schema.Components.Schemas["MCPAnnotations"] = Schema{
		Type: "object",
		Properties: map[string]Schema{
			"readOnlyHint": {
				Type:        "boolean",
				Description: "Tool is read-only",
			},
			"destructiveHint": {
				Type:        "boolean",
				Description: "Tool may be destructive",
			},
			"idempotentHint": {
				Type:        "boolean",
				Description: "Tool is idempotent",
			},
			"openWorldHint": {
				Type:        "boolean",
				Description: "Tool accepts arbitrary parameters",
			},
		},
	}
}

// convertJSONSchemaToOpenAPI converts a JSON Schema to an OpenAPI Schema
func convertJSONSchemaToOpenAPI(jsonSchema map[string]interface{}) Schema {
	schema := Schema{}

	if typ, ok := jsonSchema["type"].(string); ok {
		schema.Type = typ
	}

	if desc, ok := jsonSchema["description"].(string); ok {
		schema.Description = desc
	}

	if props, ok := jsonSchema["properties"].(map[string]interface{}); ok {
		schema.Properties = make(map[string]Schema)
		for propName, propSchema := range props {
			if propSchemaMap, ok := propSchema.(map[string]interface{}); ok {
				schema.Properties[propName] = convertJSONSchemaToOpenAPI(propSchemaMap)
			}
		}
	}

	if req, ok := jsonSchema["required"].([]interface{}); ok {
		schema.Required = make([]string, len(req))
		for i, r := range req {
			if str, ok := r.(string); ok {
				schema.Required[i] = str
			}
		}
	}

	if items, ok := jsonSchema["items"].(map[string]interface{}); ok && schema.Type == "array" {
		itemsSchema := convertJSONSchemaToOpenAPI(items)
		schema.Items = &itemsSchema
	}

	return schema
}
