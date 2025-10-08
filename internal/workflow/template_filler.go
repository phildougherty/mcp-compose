package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type TemplateFiller struct {
	logger *logging.Logger
}

func NewTemplateFiller(logger *logging.Logger) *TemplateFiller {
	return &TemplateFiller{
		logger: logger,
	}
}

func (f *TemplateFiller) FillTemplate(template *WorkflowTemplate, parameters map[string]interface{}) (*Workflow, error) {
	if err := f.validateParameters(template, parameters); err != nil {
		return nil, err
	}

	workflow := &Workflow{
		Name:        f.fillPlaceholders(template.Name, parameters),
		Description: f.fillPlaceholders(template.Description, parameters),
		Nodes:       make([]WorkflowNode, len(template.Nodes)),
		Edges:       make([]WorkflowEdge, len(template.Edges)),
		Version:     1,
		Metadata: Metadata{
			Tags:     template.Tags,
			Category: template.Category,
			CustomData: map[string]interface{}{
				"template_id": template.ID,
			},
		},
	}

	for i, node := range template.Nodes {
		filledNode := WorkflowNode{
			ID:       node.ID,
			Type:     node.Type,
			Position: node.Position,
		}

		nodeData := make(map[string]interface{})
		if err := json.Unmarshal(node.Data, &nodeData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal node data: %w", err)
		}

		filledData := f.fillNestedStructure(nodeData, parameters)

		filledDataJSON, err := json.Marshal(filledData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal filled node data: %w", err)
		}

		filledNode.Data = filledDataJSON
		workflow.Nodes[i] = filledNode
	}

	for i, edge := range template.Edges {
		workflow.Edges[i] = WorkflowEdge{
			ID:           edge.ID,
			Source:       edge.Source,
			Target:       edge.Target,
			SourceHandle: edge.SourceHandle,
			TargetHandle: edge.TargetHandle,
		}
	}

	return workflow, nil
}

func (f *TemplateFiller) validateParameters(template *WorkflowTemplate, parameters map[string]interface{}) error {
	for _, param := range template.RequiredParams {
		value, exists := parameters[param.Name]
		if !exists || value == nil {
			return &DeploymentValidationError{
				Field:   param.Name,
				Message: fmt.Sprintf("Required parameter missing: %s", param.Name),
			}
		}

		if err := f.validateParameterType(param.Name, value, param.Type); err != nil {
			return err
		}
	}

	for _, param := range template.OptionalParams {
		if value, exists := parameters[param.Name]; exists && value != nil {
			if err := f.validateParameterType(param.Name, value, param.Type); err != nil {
				return err
			}
		}
	}

	return nil
}

func (f *TemplateFiller) validateParameterType(name string, value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return &DeploymentValidationError{
				Field:   name,
				Message: fmt.Sprintf("Parameter %s must be a string", name),
			}
		}
	case "number":
		switch value.(type) {
		case int, int64, float64, float32:
		default:
			return &DeploymentValidationError{
				Field:   name,
				Message: fmt.Sprintf("Parameter %s must be a number", name),
			}
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return &DeploymentValidationError{
				Field:   name,
				Message: fmt.Sprintf("Parameter %s must be a boolean", name),
			}
		}
	case "array":
		switch value.(type) {
		case []interface{}, []string, []int, []float64:
		default:
			return &DeploymentValidationError{
				Field:   name,
				Message: fmt.Sprintf("Parameter %s must be an array", name),
			}
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return &DeploymentValidationError{
				Field:   name,
				Message: fmt.Sprintf("Parameter %s must be an object", name),
			}
		}
	}

	return nil
}

func (f *TemplateFiller) fillPlaceholders(text string, parameters map[string]interface{}) string {
	placeholderRegex := regexp.MustCompile(`\{\{([a-zA-Z0-9_.]+)\}\}`)

	result := placeholderRegex.ReplaceAllStringFunc(text, func(match string) string {
		key := strings.Trim(match, "{}")
		key = strings.TrimSpace(key)

		value := f.getNestedValue(parameters, key)
		if value == nil {
			f.logger.Warning("Placeholder %s not found in parameters", key)

			return match
		}

		return fmt.Sprintf("%v", value)
	})

	return result
}

func (f *TemplateFiller) fillNestedStructure(data interface{}, parameters map[string]interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			filledKey := f.fillPlaceholders(key, parameters)
			result[filledKey] = f.fillNestedStructure(value, parameters)
		}

		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, value := range v {
			result[i] = f.fillNestedStructure(value, parameters)
		}

		return result

	case string:
		return f.fillPlaceholders(v, parameters)

	default:
		return v
	}
}

func (f *TemplateFiller) getNestedValue(parameters map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")

	var current interface{} = parameters
	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			current = m[part]
		} else {
			return nil
		}
	}

	return current
}

type WorkflowTemplate struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Category       string                 `json:"category"`
	Tags           []string               `json:"tags"`
	Nodes          []WorkflowNode         `json:"nodes"`
	Edges          []WorkflowEdge         `json:"edges"`
	RequiredParams []TemplateParameter    `json:"required_params"`
	OptionalParams []TemplateParameter    `json:"optional_params"`
	Examples       []map[string]interface{} `json:"examples,omitempty"`
}

type TemplateParameter struct {
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
	Validation  string      `json:"validation,omitempty"`
}

type TemplateRegistry struct {
	templates map[string]*WorkflowTemplate
}

func NewTemplateRegistry() *TemplateRegistry {
	registry := &TemplateRegistry{
		templates: make(map[string]*WorkflowTemplate),
	}

	registry.registerDefaultTemplates()

	return registry
}

func (r *TemplateRegistry) registerDefaultTemplates() {
	r.templates["github-pr-monitor"] = &WorkflowTemplate{
		ID:          "github-pr-monitor",
		Name:        "GitHub PR Monitor - {{repo}}",
		Description: "Monitor {{repo}} for new pull requests and send notifications to {{slack_channel}}",
		Category:    "monitoring",
		Tags:        []string{"github", "slack", "notifications"},
		Nodes: []WorkflowNode{
			{
				ID:   "trigger-1",
				Type: NodeTypeTrigger,
				Position: NodePosition{X: 100, Y: 100},
				Data: json.RawMessage(`{
					"label": "GitHub Webhook",
					"config": {
						"type": "webhook",
						"repo": "{{repo}}"
					}
				}`),
			},
			{
				ID:   "server-1",
				Type: NodeTypeMCPServer,
				Position: NodePosition{X: 300, Y: 100},
				Data: json.RawMessage(`{
					"label": "Send Slack Notification",
					"config": {
						"server": "slack",
						"tool": "send_message",
						"channel": "{{slack_channel}}",
						"message": "New PR: {{trigger.pr_title}}"
					}
				}`),
			},
		},
		Edges: []WorkflowEdge{
			{ID: "edge-1", Source: "trigger-1", Target: "server-1"},
		},
		RequiredParams: []TemplateParameter{
			{Name: "repo", Type: "string", Description: "GitHub repository (owner/repo)"},
			{Name: "slack_channel", Type: "string", Description: "Slack channel for notifications"},
		},
	}

	r.templates["scheduled-report"] = &WorkflowTemplate{
		ID:          "scheduled-report",
		Name:        "Scheduled Report - {{report_name}}",
		Description: "Generate and send {{report_name}} report {{schedule}}",
		Category:    "automation",
		Tags:        []string{"reports", "scheduling"},
		Nodes: []WorkflowNode{
			{
				ID:   "trigger-1",
				Type: NodeTypeTrigger,
				Position: NodePosition{X: 100, Y: 100},
				Data: json.RawMessage(`{
					"label": "Schedule Trigger",
					"config": {
						"type": "schedule",
						"cron": "{{schedule}}"
					}
				}`),
			},
			{
				ID:   "ai-1",
				Type: NodeTypeAITask,
				Position: NodePosition{X: 300, Y: 100},
				Data: json.RawMessage(`{
					"label": "Generate Report",
					"config": {
						"model": "{{model}}",
						"prompt": "Generate {{report_name}} report"
					}
				}`),
			},
		},
		Edges: []WorkflowEdge{
			{ID: "edge-1", Source: "trigger-1", Target: "ai-1"},
		},
		RequiredParams: []TemplateParameter{
			{Name: "report_name", Type: "string", Description: "Name of the report"},
			{Name: "schedule", Type: "string", Description: "Cron schedule expression"},
		},
		OptionalParams: []TemplateParameter{
			{Name: "model", Type: "string", Description: "AI model to use", Default: "gpt-4"},
		},
	}
}

func (r *TemplateRegistry) GetTemplate(id string) (*WorkflowTemplate, error) {
	template, exists := r.templates[id]
	if !exists {
		return nil, &TemplateNotFoundError{TemplateID: id}
	}

	return template, nil
}

func (r *TemplateRegistry) ListTemplates() []*WorkflowTemplate {
	templates := make([]*WorkflowTemplate, 0, len(r.templates))
	for _, template := range r.templates {
		templates = append(templates, template)
	}

	return templates
}

func (r *TemplateRegistry) RegisterTemplate(template *WorkflowTemplate) error {
	if template.ID == "" {
		template.ID = uuid.New().String()
	}

	r.templates[template.ID] = template

	return nil
}
