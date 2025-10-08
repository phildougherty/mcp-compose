package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/robertkrimen/otto"
	"github.com/robfig/cron/v3"
)

type NodeValidator struct {
	cronParser cron.Parser
}

func NewNodeValidator() *NodeValidator {
	return &NodeValidator{
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor),
	}
}

func (nv *NodeValidator) ValidateNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	if node.Type == "" {
		return []WorkflowValidationErrorDetail{
			{
				Field:   "type",
				Message: "node type is required",
				NodeID:  node.ID,
			},
		}
	}

	validTypes := map[string]bool{
		NodeTypeTrigger:   true,
		NodeTypeAITask:    true,
		NodeTypeMCPServer: true,
		NodeTypeDecision:  true,
		NodeTypeTransform: true,
		NodeTypeCode:      true,
	}

	if !validTypes[node.Type] {
		return []WorkflowValidationErrorDetail{
			{
				Field:   "type",
				Message: fmt.Sprintf("invalid node type: %s", node.Type),
				NodeID:  node.ID,
			},
		}
	}

	switch node.Type {
	case NodeTypeTrigger:
		return nv.validateTriggerNode(node)
	case NodeTypeAITask:
		return nv.validateAITaskNode(node)
	case NodeTypeMCPServer:
		return nv.validateMCPServerNode(node)
	case NodeTypeDecision:
		return nv.validateDecisionNode(node)
	case NodeTypeTransform:
		return nv.validateTransformNode(node)
	case NodeTypeCode:
		return nv.validateCodeNode(node)
	}

	return nil
}

func (nv *NodeValidator) validateTriggerNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	schedule, hasSchedule := data["schedule"].(string)
	webhook, hasWebhook := data["webhook"].(string)
	event, hasEvent := data["event"].(string)

	if hasSchedule && schedule != "" {
		if _, err := nv.cronParser.Parse(schedule); err != nil {
			errors = append(errors, ValidationError{
				Field:   "data.schedule",
				Message: fmt.Sprintf("invalid cron schedule: %v", err),
				NodeID:  node.ID,
			})
		}
	}

	if hasWebhook && webhook != "" {
		if !nv.isValidWebhookPath(webhook) {
			errors = append(errors, ValidationError{
				Field:   "data.webhook",
				Message: "webhook path must start with / and contain only alphanumeric characters, hyphens, and slashes",
				NodeID:  node.ID,
			})
		}
	}

	if hasEvent && event == "" {
		errors = append(errors, ValidationError{
			Field:   "data.event",
			Message: "event type cannot be empty",
			NodeID:  node.ID,
		})
	}

	return errors
}

func (nv *NodeValidator) validateAITaskNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	var provider string
	var hasProvider bool

	if config, hasConfig := data["config"].(map[string]interface{}); hasConfig {
		provider, hasProvider = config["provider"].(string)
	} else {
		provider, hasProvider = data["provider"].(string)
	}

	if hasProvider && provider != "" {
		validProviders := map[string]bool{
			"openrouter": true,
			"openai":     true,
			"anthropic":  true,
			"local":      true,
		}

		if !validProviders[provider] {
			errors = append(errors, ValidationError{
				Field:   "data.provider",
				Message: fmt.Sprintf("invalid AI provider: %s (must be openrouter, openai, anthropic, or local)", provider),
				NodeID:  node.ID,
			})
		}
	}

	modelHint, hasModelHint := data["model_hint"].(string)
	if hasModelHint && modelHint != "" {
		if len(modelHint) > 256 {
			errors = append(errors, ValidationError{
				Field:   "data.model_hint",
				Message: "model hint must be 256 characters or less",
				NodeID:  node.ID,
			})
		}
	}

	return errors
}

func (nv *NodeValidator) validateMCPServerNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	params, hasParams := data["parameters"]
	if hasParams {
		if paramsStr, ok := params.(string); ok && paramsStr != "" {
			var paramsJSON interface{}
			if err := json.Unmarshal([]byte(paramsStr), &paramsJSON); err != nil {
				errors = append(errors, ValidationError{
					Field:   "data.parameters",
					Message: "parameters must be valid JSON",
					NodeID:  node.ID,
				})
			}
		}
	}

	return errors
}

func (nv *NodeValidator) validateDecisionNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	condition, hasCondition := data["condition"].(string)
	if hasCondition && condition != "" {
		if err := nv.validateJavaScript(condition); err != nil {
			errors = append(errors, ValidationError{
				Field:   "data.condition",
				Message: fmt.Sprintf("invalid JavaScript syntax: %v", err),
				NodeID:  node.ID,
			})
		}
	}

	return errors
}

func (nv *NodeValidator) validateTransformNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	transformCode, hasTransformCode := data["transform"].(string)
	if hasTransformCode && transformCode != "" {
		if err := nv.validateJavaScript(transformCode); err != nil {
			errors = append(errors, ValidationError{
				Field:   "data.transform",
				Message: fmt.Sprintf("invalid JavaScript syntax: %v", err),
				NodeID:  node.ID,
			})
		}
	}

	errorHandling, hasErrorHandling := data["error_handling"].(string)
	if hasErrorHandling && errorHandling != "" {
		validModes := map[string]bool{
			"fail":     true,
			"continue": true,
			"retry":    true,
		}

		if !validModes[errorHandling] {
			errors = append(errors, ValidationError{
				Field:   "data.error_handling",
				Message: "error_handling must be 'fail', 'continue', or 'retry'",
				NodeID:  node.ID,
			})
		}
	}

	return errors
}

func (nv *NodeValidator) validateCodeNode(node *WorkflowNode) []WorkflowValidationErrorDetail {
	var errors []WorkflowValidationErrorDetail

	var data map[string]interface{}
	if err := json.Unmarshal(node.Data, &data); err != nil {
		errors = append(errors, ValidationError{
			Field:   "data",
			Message: "invalid JSON data",
			NodeID:  node.ID,
		})

		return errors
	}

	language, hasLanguage := data["language"].(string)
	if hasLanguage && language != "" {
		validLanguages := map[string]bool{
			"javascript": true,
			"python":     true,
			"bash":       true,
		}

		if !validLanguages[language] {
			errors = append(errors, ValidationError{
				Field:   "data.language",
				Message: "language must be javascript, python, or bash",
				NodeID:  node.ID,
			})
		}
	}

	timeout, hasTimeout := data["timeout"]
	if hasTimeout {
		if timeoutFloat, ok := timeout.(float64); ok {
			if timeoutFloat <= 0 {
				errors = append(errors, ValidationError{
					Field:   "data.timeout",
					Message: "timeout must be positive",
					NodeID:  node.ID,
				})
			}
		}
	}

	return errors
}

func (nv *NodeValidator) validateJavaScript(code string) error {
	vm := otto.New()

	wrappedCode := "(function() { " + code + " })"

	_, err := vm.Compile("", wrappedCode)

	return err
}

func (nv *NodeValidator) isValidWebhookPath(path string) bool {
	if !strings.HasPrefix(path, "/") {
		return false
	}

	validPath := regexp.MustCompile(`^/[a-zA-Z0-9/_-]*$`)

	return validPath.MatchString(path)
}
