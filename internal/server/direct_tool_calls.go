package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"mcpcompose/internal/constants"
	"mcpcompose/internal/dashboard"
)

// mcpResponseRecorder captures HTTP responses for MCP tool calls
type mcpResponseRecorder struct {
	statusCode int
	body       []byte
	headers    http.Header
}

func (r *mcpResponseRecorder) Header() http.Header {
	if r.headers == nil {
		r.headers = make(http.Header)
	}


	return r.headers
}

func (r *mcpResponseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

func (r *mcpResponseRecorder) Write(body []byte) (int, error) {
	r.body = append(r.body, body...)

	return len(body), nil
}

func (h *ProxyHandler) handleDirectToolCall(w http.ResponseWriter, r *http.Request, toolName string) {
	// Authenticate
	apiKeyToCheck := h.APIKey
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.ProxyAuth.Enabled {
		apiKeyToCheck = h.Manager.config.ProxyAuth.APIKey
	}

	if apiKeyToCheck != "" {
		authHeader := r.Header.Get("Authorization")
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != apiKeyToCheck {
			h.corsError(w, "Unauthorized", http.StatusUnauthorized)

			return
		}
	}

	h.logger.Info("Handling direct tool call: %s", toolName)

	// Parse request body as tool arguments
	var arguments map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&arguments); err != nil {
		h.logger.Error("Failed to decode request body for tool %s: %v", toolName, err)
		h.corsError(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	// Find which server has this tool
	serverName, found := h.findServerForTool(toolName)
	if !found {
		h.logger.Warning("Tool %s not found in any server", toolName)
		h.corsError(w, "Tool not found", http.StatusNotFound)

		return
	}

	h.logger.Info("Routing tool %s to server %s", toolName, serverName)

	dashboard.BroadcastActivity("INFO", "tool", serverName, getClientIP(r),
		fmt.Sprintf("Tool called: %s", toolName),
		map[string]interface{}{"tool": toolName, "arguments": arguments})

	// Create MCP tools/call request
	mcpRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      h.getNextRequestID(),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      toolName,
			"arguments": arguments,
		},
	}

	// Forward to the appropriate server and get response
	if instance, exists := h.Manager.GetServerInstance(serverName); exists {
		// Convert to request body
		requestBody, err := json.Marshal(mcpRequest)
		if err != nil {
			h.logger.Error("Failed to marshal MCP request for tool %s: %v", toolName, err)
			h.corsError(w, "Internal server error", http.StatusInternalServerError)

			return
		}

		// Create new request
		newRequest := r.Clone(r.Context())
		newRequest.Body = io.NopCloser(bytes.NewReader(requestBody))
		newRequest.ContentLength = int64(len(requestBody))

		// Create a simple response recorder
		recorder := &mcpResponseRecorder{
			statusCode: constants.HTTPStatusSuccess,
			headers:    make(http.Header),
		}

		h.handleServerForward(recorder, newRequest, serverName, instance)

		// Parse and format the MCP response
		if recorder.statusCode == 200 && len(recorder.body) > 0 {
			var mcpResponse map[string]interface{}
			if err := json.Unmarshal(recorder.body, &mcpResponse); err == nil {
				// Check for MCP error
				if mcpError, hasError := mcpResponse["error"].(map[string]interface{}); hasError {
					errorResponse := map[string]interface{}{
						"error": mcpError["message"],
					}
					if data, hasData := mcpError["data"]; hasData {
						errorResponse["details"] = data
					}
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(w).Encode(errorResponse)

					return
				}

				// Extract and format the successful result
				if result, exists := mcpResponse["result"]; exists {
					if resultMap, ok := result.(map[string]interface{}); ok {
						if content, exists := resultMap["content"]; exists {
							// Process the content like MCPO does
							cleanResult := h.processMCPContent(content)
							w.Header().Set("Content-Type", "application/json")
							_ = json.NewEncoder(w).Encode(cleanResult)

							return
						}
					}
				}
			}
		}

		// Fallback to original response if formatting fails
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(recorder.statusCode)
		_, _ = w.Write(recorder.body)
	} else {
		h.corsError(w, "Server not found", http.StatusNotFound)
	}
}

func (h *ProxyHandler) handleServerForward(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance) {
	// Authentication check - validate before processing the request
	if !h.authenticateRequest(w, r, serverName, instance) {

		return // Authentication failed, response already sent
	}

	w.Header().Set("Content-Type", "application/json")

	// Read request body ONCE and store it
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("Failed to read request body for %s: %v", serverName, err)
		h.sendMCPError(w, nil, -32700, "Error reading request body")

		return
	}

	// Parse JSON payload from the stored body
	var requestPayload map[string]interface{}
	if err := json.Unmarshal(body, &requestPayload); err != nil {
		h.logger.Error("Invalid JSON in request for %s: %v. Body: %s", serverName, err, string(body))
		h.sendMCPError(w, nil, -32700, "Invalid JSON in request")

		return
	}

	reqIDVal := requestPayload["id"]
	reqMethodVal, _ := requestPayload["method"].(string)

	dashboard.BroadcastActivity("INFO", "request", serverName, getClientIP(r),
		fmt.Sprintf("MCP Request: %s", reqMethodVal),
		map[string]interface{}{
			"method":   reqMethodVal,
			"id":       reqIDVal,
			"endpoint": r.URL.Path,
		})

	// ONLY handle proxy-specific standard methods, NOT server methods
	if isProxyStandardMethod(reqMethodVal) {
		h.handleProxyStandardMethod(w, r, requestPayload, reqIDVal, reqMethodVal)

		return
	}

	// FORWARD ALL OTHER METHODS TO THE ACTUAL MCP SERVERS
	// Get server config
	serverConfig, exists := h.Manager.config.Servers[serverName]
	if !exists {
		h.logger.Error("Server config not found for %s", serverName)
		h.sendMCPError(w, reqIDVal, -32602, "Server configuration not found")

		return
	}

	// Determine transport protocol
	protocolType := serverConfig.Protocol
	if protocolType == "" {
		protocolType = "stdio" // default
	}

	h.logger.Info("Forwarding request to server '%s' using '%s' transport: Method=%s, ID=%v",
		serverName, protocolType, reqMethodVal, reqIDVal)

	// Route based on transport protocol - pass the original body bytes
	switch protocolType {
	case "http":
		h.handleHTTPServerRequestWithBody(w, r, serverName, instance, body, reqIDVal, reqMethodVal)
	case "sse":
		h.handleSSEServerRequest(w, r, serverName, instance, requestPayload, reqIDVal, reqMethodVal)
	case "stdio":
		if serverConfig.StdioHosterPort > 0 {
			h.handleSocatSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		} else {
			h.handleSTDIOServerRequest(w, r, serverName, requestPayload, reqIDVal, reqMethodVal)
		}
	default:
		h.logger.Error("Unsupported transport protocol '%s' for server %s", protocolType, serverName)
		h.sendMCPError(w, reqIDVal, -32602, fmt.Sprintf("Unsupported transport protocol: %s", protocolType))
	}
}

// processMCPContent processes MCP content like the official MCPO tool does
func (h *ProxyHandler) processMCPContent(content interface{}) interface{} {
	if contentArray, ok := content.([]interface{}); ok {
		var processed []interface{}
		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, ok := itemMap["type"].(string); ok {
					switch itemType {
					case "text":
						if text, ok := itemMap["text"].(string); ok {
							// Try to parse as JSON first
							var jsonData interface{}
							if err := json.Unmarshal([]byte(text), &jsonData); err == nil {
								processed = append(processed, jsonData)
							} else {
								processed = append(processed, text)
							}
						} else {
							processed = append(processed, item)
						}
					case "image":
						if data, ok := itemMap["data"].(string); ok {
							if mimeType, ok := itemMap["mimeType"].(string); ok {
								imageURL := fmt.Sprintf("data:%s;base64,%s", mimeType, data)
								processed = append(processed, imageURL)
							} else {
								processed = append(processed, item)
							}
						} else {
							processed = append(processed, item)
						}
					default:
						processed = append(processed, item)
					}
				} else {
					processed = append(processed, item)
				}
			} else {
				processed = append(processed, item)
			}
		}

		// Return single item if array has only one element
		if len(processed) == 1 {

			return processed[0]
		}

		return processed
	}

	return content
}
