package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type DeploymentProcessor struct {
	aiManager        *ai.Manager
	logger           *logging.Logger
	templateRegistry *TemplateRegistry
}

func NewDeploymentProcessor(aiManager *ai.Manager, logger *logging.Logger) *DeploymentProcessor {
	return &DeploymentProcessor{
		aiManager:        aiManager,
		logger:           logger,
		templateRegistry: NewTemplateRegistry(),
	}
}

func (p *DeploymentProcessor) MatchTemplate(ctx context.Context, description string) (*WorkflowTemplate, map[string]interface{}, error) {
	systemPrompt := `You are a workflow template matching expert.
Analyze the user's workflow description and determine which template best matches their needs.

Available templates:
1. github-pr-monitor: Monitor GitHub repository for new pull requests and send notifications
2. scheduled-report: Generate and send reports on a schedule
3. webhook-processor: Process incoming webhook events and trigger actions
4. data-sync: Synchronize data between two systems periodically
5. ai-content-generator: Generate content using AI on a schedule or trigger

Return a JSON object with:
{
  "template": "template-name",
  "confidence": 0.0-1.0,
  "reasoning": "why this template matches"
}

If confidence is below 0.6, return null for template.`

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Workflow description: %s", description)},
	}

	response, err := p.aiManager.Chat(ctx, messages)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to match template: %w", err)
	}

	var result struct {
		Template   string  `json:"template"`
		Confidence float64 `json:"confidence"`
		Reasoning  string  `json:"reasoning"`
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	if result.Template == "" || result.Confidence < 0.6 {
		return nil, nil, fmt.Errorf("no suitable template found (confidence: %.2f)", result.Confidence)
	}

	p.logger.Info("Matched template %s with confidence %.2f: %s", result.Template, result.Confidence, result.Reasoning)

	template, err := p.templateRegistry.GetTemplate(result.Template)
	if err != nil {
		return nil, nil, err
	}

	paramExtractor := NewParameterExtractor(p.aiManager, p.logger)
	params, err := paramExtractor.ExtractFromDescription(ctx, description, template)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to extract parameters: %w", err)
	}

	return template, params, nil
}

func (p *DeploymentProcessor) GenerateWorkflow(ctx context.Context, description string) (*Workflow, error) {
	systemPrompt := `You are a workflow generation expert.
Analyze the user's description and generate a complete workflow specification.

Available node types:
- trigger: Starts the workflow (schedule, webhook, manual)
- ai-task: Performs AI operations (text generation, analysis, etc.)
- mcp-server: Calls MCP server tools (GitHub, Slack, file operations, etc.)
- decision: Conditional branching based on data
- transform: Data transformation and manipulation
- code: Custom code execution

Return a JSON object with this exact structure:
{
  "name": "Workflow Name",
  "description": "Brief description",
  "nodes": [
    {
      "id": "node-1",
      "type": "trigger",
      "position": {"x": 100, "y": 100},
      "data": {
        "label": "Node Label",
        "config": {}
      }
    }
  ],
  "edges": [
    {
      "id": "edge-1",
      "source": "node-1",
      "target": "node-2"
    }
  ]
}

Important:
- Generate unique IDs for all nodes and edges
- Position nodes in a logical flow (left to right, top to bottom)
- Include all necessary configuration in node data
- Create valid edges connecting the nodes`

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Generate a workflow for: %s", description)},
	}

	response, err := p.aiManager.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("failed to generate workflow: %w", err)
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	var workflowSpec struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Nodes       []WorkflowNode `json:"nodes"`
		Edges       []WorkflowEdge `json:"edges"`
	}

	if err := json.Unmarshal([]byte(response), &workflowSpec); err != nil {
		return nil, fmt.Errorf("failed to parse generated workflow: %w", err)
	}

	for i := range workflowSpec.Nodes {
		if workflowSpec.Nodes[i].ID == "" {
			workflowSpec.Nodes[i].ID = fmt.Sprintf("node-%s", uuid.New().String()[:8])
		}
	}

	for i := range workflowSpec.Edges {
		if workflowSpec.Edges[i].ID == "" {
			workflowSpec.Edges[i].ID = fmt.Sprintf("edge-%s", uuid.New().String()[:8])
		}
	}

	workflow := &Workflow{
		Name:        workflowSpec.Name,
		Description: workflowSpec.Description,
		Nodes:       workflowSpec.Nodes,
		Edges:       workflowSpec.Edges,
		Version:     1,
	}

	return workflow, nil
}
