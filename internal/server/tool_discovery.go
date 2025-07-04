package server

import (
	"fmt"
	"strings"
	"time"

	"mcpcompose/internal/openapi"
)

func (h *ProxyHandler) refreshToolCache() {
	h.toolCacheMu.Lock()
	defer h.toolCacheMu.Unlock()

	// Only refresh if cache is expired
	if time.Now().Before(h.cacheExpiry) {
		return
	}

	h.logger.Info("Refreshing tool cache...")
	newCache := make(map[string]string)

	for serverName := range h.Manager.config.Servers {
		tools, err := h.discoverServerTools(serverName)
		if err != nil {
			h.logger.Warning("Failed to discover tools for %s during cache refresh: %v", serverName, err)
			continue
		}

		for _, tool := range tools {
			newCache[tool.Name] = serverName
			h.logger.Debug("Cached tool %s -> %s", tool.Name, serverName)
		}
	}

	h.toolCache = newCache
	h.cacheExpiry = time.Now().Add(5 * time.Minute) // Cache for 5 minutes
	h.logger.Info("Tool cache refreshed with %d tools", len(newCache))
}

func (h *ProxyHandler) discoverServerTools(serverName string) ([]openapi.ToolSpec, error) {
	h.logger.Info("Discovering tools from server %s via internal proxy methods", serverName)

	// Create tools/list request
	toolsRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "tools/list",
	}

	// Check if server exists
	if _, exists := h.Manager.GetServerInstance(serverName); !exists {
		h.logger.Warning("Server instance %s not found, using generic fallback", serverName)
		return h.getGenericToolForServer(serverName), nil
	}

	serverConfig := h.Manager.config.Servers[serverName]

	// Determine the transport protocol
	protocol := serverConfig.Protocol
	if protocol == "" {
		protocol = "stdio" // default
	}

	// Route based on protocol
	h.logger.Info("Server %s using protocol: %s", serverName, protocol)

	// Retry logic with exponential backoff
	maxRetries := 3
	baseTimeout := 10 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		h.logger.Debug("Tool discovery attempt %d/%d for server %s (protocol: %s)", attempt, maxRetries, serverName, protocol)
		timeout := time.Duration(attempt) * baseTimeout // 10s, 20s, 30s

		var response map[string]interface{}
		var err error

		switch protocol {
		case "sse":
			// Use SSE discovery
			response, err = h.sendSSEToolsRequestWithRetry(serverName, toolsRequest, timeout, attempt)
		case "http":
			// Use HTTP discovery
			response, err = h.sendHTTPToolsRequestWithRetry(serverName, toolsRequest, timeout, attempt)
		case "stdio":
			if serverConfig.StdioHosterPort > 0 {
				// Use socat TCP connection
				containerName := fmt.Sprintf("mcp-compose-%s", serverName)
				socatHost := containerName
				socatPort := serverConfig.StdioHosterPort
				response, err = h.sendRawTCPRequestWithRetry(socatHost, socatPort, toolsRequest, timeout, attempt)
			} else {
				// STDIO server - skip for now and use generic
				h.logger.Warning("Direct STDIO server %s tool discovery not implemented, using generic fallback", serverName)
				return h.getGenericToolForServer(serverName), nil
			}
		default:
			h.logger.Warning("Unknown protocol %s for server %s, using generic fallback", protocol, serverName)
			return h.getGenericToolForServer(serverName), nil
		}

		if err == nil {
			// Success - parse and return tools
			specs, parseErr := h.parseToolsResponse(serverName, response)
			if parseErr == nil && len(specs) > 0 {
				toolNames := make([]string, len(specs))
				for i, spec := range specs {
					toolNames[i] = spec.Name
				}
				h.logger.Info("Successfully discovered %d tools from %s: %v", len(specs), serverName, toolNames)
				return specs, nil
			}
			if parseErr != nil {
				h.logger.Warning("Failed to parse tools response from %s on attempt %d: %v", serverName, attempt, parseErr)
				err = parseErr
			}
		}

		// Log the failure and decide whether to retry
		isTimeout := strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "i/o timeout")
		isConnectionError := strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "no such host")

		if attempt < maxRetries && (isTimeout || isConnectionError) {
			waitTime := time.Duration(attempt*2) * time.Second // 2s, 4s wait between retries
			h.logger.Warning("Tool discovery attempt %d/%d failed for %s (%v), retrying in %v", attempt, maxRetries, serverName, err, waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Final attempt failed or non-retryable error
		h.logger.Warning("Tool discovery failed for %s after %d attempts: %v, using generic fallback", serverName, attempt, err)
		break
	}

	// All retries failed, use generic fallback
	return h.getGenericToolForServer(serverName), fmt.Errorf("failed to discover tools after %d attempts", maxRetries)
}

func (h *ProxyHandler) sendSSEToolsRequestWithRetry(serverName string, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	h.logger.Debug("Attempting enhanced SSE request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)
	return h.sendOptimalSSERequest(serverName, requestPayload)
}

func (h *ProxyHandler) sendHTTPToolsRequestWithRetry(serverName string, requestPayload map[string]interface{}, timeout time.Duration, attempt int) (map[string]interface{}, error) {
	h.logger.Debug("Attempting HTTP request to %s (attempt %d, timeout %v)", serverName, attempt, timeout)
	conn, connErr := h.getServerConnection(serverName)
	if connErr != nil {
		return nil, connErr
	}
	return h.sendHTTPJsonRequest(conn, requestPayload, timeout)
}

func (h *ProxyHandler) parseToolsResponse(serverName string, response map[string]interface{}) ([]openapi.ToolSpec, error) {
	h.logger.Debug("Parsing tools response for %s: %v", serverName, response)

	// Check for JSON-RPC error
	if errResp, ok := response["error"].(map[string]interface{}); ok {
		return nil, fmt.Errorf("server returned error: %v", errResp)
	}

	// Parse the tools from the response
	var specs []openapi.ToolSpec
	if result, ok := response["result"].(map[string]interface{}); ok {
		h.logger.Debug("Found result object for %s: %v", serverName, result)
		if tools, ok := result["tools"].([]interface{}); ok {
			h.logger.Debug("Found tools array for %s with %d tools", serverName, len(tools))
			for i, tool := range tools {
				if toolMap, ok := tool.(map[string]interface{}); ok {
					spec := openapi.ToolSpec{Type: "function"}
					if name, ok := toolMap["name"].(string); ok {
						spec.Name = name
					} else {
						h.logger.Warning("Tool %d in %s missing name field: %v", i, serverName, toolMap)
						continue
					}

					if desc, ok := toolMap["description"].(string); ok {
						spec.Description = desc
					} else {
						spec.Description = fmt.Sprintf("Tool from %s server", serverName)
					}

					if inputSchema, ok := toolMap["inputSchema"].(map[string]interface{}); ok {
						spec.Parameters = inputSchema
					} else {
						spec.Parameters = map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
							"required":   []string{},
						}
					}

					specs = append(specs, spec)
				} else {
					h.logger.Warning("Tool %d in %s is not a map: %v", i, serverName, tool)
				}
			}
		} else {
			h.logger.Warning("No 'tools' array found in result for %s. Result keys: %v", serverName, getKeys(result))
		}
	} else {
		h.logger.Warning("No 'result' object found in response for %s. Response keys: %v", serverName, getKeys(response))
	}

	h.logger.Debug("Parsed %d tools for %s: %v", len(specs), serverName, getToolNames(specs))

	if len(specs) == 0 {
		return nil, fmt.Errorf("no tools found in response")
	}

	return specs, nil
}

// Generic fallback that works with any MCP server
func (h *ProxyHandler) getGenericToolForServer(serverName string) []openapi.ToolSpec {
	return []openapi.ToolSpec{
		{
			Type:        "function",
			Name:        fmt.Sprintf("%s_execute", serverName),
			Description: fmt.Sprintf("Execute any command on %s MCP server", serverName),
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"method": map[string]interface{}{
						"type":        "string",
						"description": "MCP method to call (e.g., tools/call, prompts/list, resources/list)",
					},
					"params": map[string]interface{}{
						"type":                 "object",
						"description":          "Parameters for the MCP method",
						"additionalProperties": true,
					},
				},
				"required": []string{"method"},
			},
		},
	}
}

func (h *ProxyHandler) isKnownTool(toolName string) bool {
	// Refresh cache if needed
	h.refreshToolCache()

	h.toolCacheMu.RLock()
	serverName, exists := h.toolCache[toolName]
	h.toolCacheMu.RUnlock()

	if exists {
		h.logger.Debug("Tool cache lookup for '%s': found in server %s", toolName, serverName)
		return true
	}

	h.logger.Debug("Tool cache lookup for '%s': not found in cache of %d tools", toolName, len(h.toolCache))

	// Force refresh cache once if not found
	h.toolCacheMu.Lock()
	if time.Now().After(h.cacheExpiry) {
		h.toolCacheMu.Unlock()
		h.refreshToolCache()
		h.toolCacheMu.RLock()
		_, exists = h.toolCache[toolName]
		h.toolCacheMu.RUnlock()
		if exists {
			h.logger.Debug("Tool '%s' found after cache refresh", toolName)
			return true
		}
	} else {
		h.toolCacheMu.Unlock()
	}

	h.logger.Debug("Tool '%s' not found even after cache check", toolName)
	return false
}

func (h *ProxyHandler) findServerForTool(toolName string) (string, bool) {
	// Refresh cache if needed
	h.refreshToolCache()

	h.toolCacheMu.RLock()
	serverName, exists := h.toolCache[toolName]
	h.toolCacheMu.RUnlock()

	if exists {
		h.logger.Debug("Found tool %s in server %s via cache", toolName, serverName)
		return serverName, true
	}

	h.logger.Warning("Tool %s not found in cache", toolName)
	return "", false
}

// Helper functions for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func getToolNames(specs []openapi.ToolSpec) []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}
