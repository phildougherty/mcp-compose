package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type ExecutionContext struct {
	Input      map[string]interface{}
	Context    map[string]interface{}
	NodeOutputs map[string]map[string]interface{}
}

var templateVarRegex = regexp.MustCompile(`\{\{([^}]+)\}\}`)

func NewExecutionContext() *ExecutionContext {
	return &ExecutionContext{
		Input:      make(map[string]interface{}),
		Context:    make(map[string]interface{}),
		NodeOutputs: make(map[string]map[string]interface{}),
	}
}

func (ec *ExecutionContext) SetInput(input map[string]interface{}) {
	ec.Input = input
}

func (ec *ExecutionContext) SetNodeOutput(nodeID string, output map[string]interface{}) {
	ec.NodeOutputs[nodeID] = output
}

func (ec *ExecutionContext) SetContextVar(key string, value interface{}) {
	ec.Context[key] = value
}

func (ec *ExecutionContext) ResolveTemplateString(template string) (string, error) {
	result := templateVarRegex.ReplaceAllStringFunc(template, func(match string) string {
		varPath := strings.TrimSpace(match[2 : len(match)-2])

		value, err := ec.resolveVariablePath(varPath)
		if err != nil {
			return match
		}

		switch v := value.(type) {
		case string:
			return v
		case float64, int, int64, bool:
			return fmt.Sprintf("%v", v)
		default:
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				return match
			}

			return string(jsonBytes)
		}
	})

	return result, nil
}

func (ec *ExecutionContext) ResolveTemplateMap(input map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range input {
		resolvedValue, err := ec.resolveValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve key %s: %w", key, err)
		}

		result[key] = resolvedValue
	}

	return result, nil
}

func (ec *ExecutionContext) resolveValue(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case string:
		return ec.ResolveTemplateString(v)
	case map[string]interface{}:
		return ec.ResolveTemplateMap(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			resolvedItem, err := ec.resolveValue(item)
			if err != nil {
				return nil, err
			}

			result[i] = resolvedItem
		}

		return result, nil
	default:
		return value, nil
	}
}

func (ec *ExecutionContext) resolveVariablePath(path string) (interface{}, error) {
	parts := strings.Split(path, ".")

	if len(parts) == 0 {
		return nil, fmt.Errorf("empty variable path")
	}

	var current interface{}

	switch parts[0] {
	case "input":
		current = ec.Input
	case "context":
		current = ec.Context
	case "nodes":
		if len(parts) < 2 {
			return nil, fmt.Errorf("nodes path requires node ID")
		}

		nodeID := parts[1]
		nodeOutput, exists := ec.NodeOutputs[nodeID]
		if !exists {
			return nil, fmt.Errorf("node %s output not found", nodeID)
		}

		if len(parts) == 2 {
			return nodeOutput, nil
		}

		current = nodeOutput
		parts = parts[2:]

		return ec.traversePath(current, parts)
	default:
		return nil, fmt.Errorf("unknown root variable: %s", parts[0])
	}

	if len(parts) == 1 {
		return current, nil
	}

	return ec.traversePath(current, parts[1:])
}

func (ec *ExecutionContext) traversePath(obj interface{}, parts []string) (interface{}, error) {
	current := obj

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			value, exists := v[part]
			if !exists {
				return nil, fmt.Errorf("key %s not found", part)
			}

			current = value
		default:
			return nil, fmt.Errorf("cannot traverse non-map type at %s", part)
		}
	}

	return current, nil
}
