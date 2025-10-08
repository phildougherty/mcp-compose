package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type ParameterExtractor struct {
	aiManager *ai.Manager
	logger    *logging.Logger
}

func NewParameterExtractor(aiManager *ai.Manager, logger *logging.Logger) *ParameterExtractor {
	return &ParameterExtractor{
		aiManager: aiManager,
		logger:    logger,
	}
}

func (e *ParameterExtractor) ExtractFromDescription(ctx context.Context, description string, template *WorkflowTemplate) (map[string]interface{}, error) {
	if len(template.RequiredParams) == 0 && len(template.OptionalParams) == 0 {
		return map[string]interface{}{}, nil
	}

	params := e.extractWithRegex(description)

	if e.aiManager != nil {
		aiParams, err := e.extractWithAI(ctx, description, template)
		if err != nil {
			e.logger.Warning("AI parameter extraction failed, using regex only: %v", err)
		} else {
			for k, v := range aiParams {
				if _, exists := params[k]; !exists {
					params[k] = v
				}
			}
		}
	}

	for _, param := range template.RequiredParams {
		if params[param.Name] == nil && param.Default != nil {
			params[param.Name] = param.Default
		}
	}

	for _, param := range template.OptionalParams {
		if params[param.Name] == nil && param.Default != nil {
			params[param.Name] = param.Default
		}
	}

	for _, param := range template.RequiredParams {
		if params[param.Name] == nil {
			return nil, &DeploymentValidationError{
				Field:   param.Name,
				Message: fmt.Sprintf("Required parameter missing: %s", param.Name),
			}
		}
	}

	return params, nil
}

func (e *ParameterExtractor) extractWithRegex(description string) map[string]interface{} {
	params := make(map[string]interface{})

	repoRegex := regexp.MustCompile(`(?i)(?:repo(?:sitory)?|github)\s*[:\-]?\s*([a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+)`)
	if matches := repoRegex.FindStringSubmatch(description); len(matches) > 1 {
		params["repo"] = matches[1]
		params["repository"] = matches[1]
	}

	slackRegex := regexp.MustCompile(`(?i)(?:slack\s+channel|channel)\s*[:\-]?\s*([#@][a-zA-Z0-9_-]+)`)
	if matches := slackRegex.FindStringSubmatch(description); len(matches) > 1 {
		params["slack_channel"] = matches[1]
		params["channel"] = matches[1]
	}

	emailRegex := regexp.MustCompile(`(?i)(?:email|e-mail)\s*[:\-]?\s*([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})`)
	if matches := emailRegex.FindStringSubmatch(description); len(matches) > 1 {
		params["email"] = matches[1]
	}

	urlRegex := regexp.MustCompile(`(?i)(?:url|webhook)\s*[:\-]?\s*(https?://[^\s]+)`)
	if matches := urlRegex.FindStringSubmatch(description); len(matches) > 1 {
		params["url"] = matches[1]
		params["webhook_url"] = matches[1]
	}

	cronRegex := regexp.MustCompile(`(?i)(?:cron|schedule)\s*[:\-]?\s*([*0-9/,\-\s]+)`)
	if matches := cronRegex.FindStringSubmatch(description); len(matches) > 1 {
		params["schedule"] = matches[1]
		params["cron"] = matches[1]
	}

	intervalRegex := regexp.MustCompile(`(?i)every\s+(\d+)\s+(second|minute|hour|day)s?`)
	if matches := intervalRegex.FindStringSubmatch(description); len(matches) > 2 {
		params["interval"] = matches[1] + " " + matches[2]
	}

	return params
}

func (e *ParameterExtractor) extractWithAI(ctx context.Context, description string, template *WorkflowTemplate) (map[string]interface{}, error) {
	requiredParams := make([]string, len(template.RequiredParams))
	for i, p := range template.RequiredParams {
		requiredParams[i] = fmt.Sprintf("%s (%s): %s", p.Name, p.Type, p.Description)
	}

	optionalParams := make([]string, len(template.OptionalParams))
	for i, p := range template.OptionalParams {
		optionalParams[i] = fmt.Sprintf("%s (%s): %s", p.Name, p.Type, p.Description)
	}

	systemPrompt := fmt.Sprintf(`You are a parameter extraction expert.
Extract the following parameters from the user's workflow description.

Required parameters:
%s

Optional parameters:
%s

Return a JSON object with the extracted parameters.
Use null for parameters that cannot be extracted.
For string values, return the exact value found.
For numbers, return numeric values.
For booleans, return true or false.

Example response:
{
  "repo": "owner/repo-name",
  "slack_channel": "#notifications",
  "schedule": "0 9 * * *"
}`, strings.Join(requiredParams, "\n"), strings.Join(optionalParams, "\n"))

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Extract parameters from: %s", description)},
	}

	response, err := e.aiManager.Chat(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("AI extraction failed: %w", err)
	}

	response = strings.TrimSpace(response)
	if strings.HasPrefix(response, "```json") {
		response = strings.TrimPrefix(response, "```json")
		response = strings.TrimSuffix(response, "```")
		response = strings.TrimSpace(response)
	}

	var params map[string]interface{}
	if err := json.Unmarshal([]byte(response), &params); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	cleanedParams := make(map[string]interface{})
	for k, v := range params {
		if v != nil {
			cleanedParams[k] = v
		}
	}

	return cleanedParams, nil
}
