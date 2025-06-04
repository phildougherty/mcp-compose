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

// ToolSpec represents a tool specification for OpenWebUI
type ToolSpec struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
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

// Tool represents an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// GenerateOpenAPISchema generates an OpenAPI schema for MCP tools
func GenerateOpenAPISchema(serverName string, tools []Tool) (*OpenAPISchema, error) {
	schema := &OpenAPISchema{
		OpenAPI: "3.0.0",
		Info: Info{
			Title:       fmt.Sprintf("%s API", serverName),
			Description: fmt.Sprintf("API for %s MCP server", serverName),
			Version:     "1.0.0",
		},
		Servers: []Server{
			{
				URL:         "/",
				Description: "MCP Server",
			},
		},
		Paths: make(map[string]PathItem),
		Components: Components{
			Schemas: make(map[string]Schema),
			SecuritySchemes: map[string]SecurityScheme{
				"ApiKeyAuth": {
					Type:   "http",
					Scheme: "bearer",
				},
			},
		},
	}

	// Create specs entries for OpenWebUI compatibility
	specs := make([]ToolSpec, 0, len(tools))

	// Add paths for each tool
	for _, tool := range tools {
		// Clean the operationId to ensure it's a valid identifier
		operationId := strings.ReplaceAll(tool.Name, "-", "_")
		operationId = strings.ReplaceAll(operationId, " ", "_")

		pathItem := PathItem{
			Post: Operation{
				Summary:     tool.Name,
				Description: tool.Description,
				OperationID: operationId,
				Tags:        []string{serverName},
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
						Description: "Successful response",
						Content: map[string]MediaType{
							"application/json": {
								Schema: Schema{
									Ref: fmt.Sprintf("#/components/schemas/%sResponse", tool.Name),
								},
							},
						},
					},
					"400": {
						Description: "Bad request",
					},
					"401": {
						Description: "Unauthorized",
					},
					"500": {
						Description: "Server error",
					},
				},
				Security: []map[string][]string{
					{
						"ApiKeyAuth": {},
					},
				},
			},
		}

		// Set the path to match OpenWebUI's expectations
		schema.Paths["/"+tool.Name] = pathItem

		// Add request schema
		requestSchema := convertJSONSchemaToOpenAPI(tool.InputSchema)
		schema.Components.Schemas[tool.Name+"Request"] = requestSchema

		// Add response schema
		responseSchema := Schema{
			Type: "object",
			Properties: map[string]Schema{
				"result": {
					Type:        "object",
					Description: "Tool execution result",
				},
			},
		}
		schema.Components.Schemas[tool.Name+"Response"] = responseSchema

		// Create a tool spec entry for OpenWebUI
		spec := ToolSpec{
			Type:        "function",
			Name:        tool.Name,
			Description: tool.Description,
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

	// Add the specs to the schema
	schema.Specs = specs

	return schema, nil
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
