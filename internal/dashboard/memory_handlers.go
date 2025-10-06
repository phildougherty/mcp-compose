package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (d *DashboardServer) handleMemoryProxy(w http.ResponseWriter, r *http.Request) {
	// Extract the path after /api/memory/
	path := strings.TrimPrefix(r.URL.Path, "/api/memory")

	d.logger.Info("Memory proxy request: %s %s", r.Method, path)

	// Map REST-like calls to MCP tool calls
	var toolName string
	var toolArgs map[string]interface{}

	switch {
	case path == "/stats" && r.Method == "GET":
		toolName = "read_graph"
		toolArgs = map[string]interface{}{}

	case path == "/entities" && r.Method == "GET":
		toolName = "read_graph"
		toolArgs = map[string]interface{}{}

	case path == "/entities" && r.Method == "POST":
		toolName = "create_entities"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)

			return
		}
		if err := json.Unmarshal(body, &toolArgs); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)

			return
		}

	case strings.HasPrefix(path, "/entities/") && r.Method == "GET":
		toolName = "open_nodes"
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "entities" {
			toolArgs = map[string]interface{}{
				"names": []string{pathParts[1]},
			}
		} else {
			http.Error(w, "Invalid entity name in path", http.StatusBadRequest)

			return
		}

	case strings.HasPrefix(path, "/entities/") && r.Method == "DELETE":
		toolName = "delete_entities"
		pathParts := strings.Split(strings.Trim(path, "/"), "/")
		if len(pathParts) >= 2 && pathParts[0] == "entities" {
			toolArgs = map[string]interface{}{
				"entityNames": []string{pathParts[1]},
			}
		} else {
			http.Error(w, "Invalid entity name in path", http.StatusBadRequest)

			return
		}

	case path == "/relationships" && r.Method == "POST":
		toolName = "create_relations"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)

			return
		}
		if err := json.Unmarshal(body, &toolArgs); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)

			return
		}

	case path == "/relationships" && r.Method == "DELETE":
		toolName = "delete_relations"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)

			return
		}
		if err := json.Unmarshal(body, &toolArgs); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)

			return
		}

	case path == "/search" && r.Method == "POST":
		toolName = "search_nodes"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)

			return
		}
		var searchBody map[string]interface{}
		if err := json.Unmarshal(body, &searchBody); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)

			return
		}
		toolArgs = searchBody

	case path == "/observations" && r.Method == "POST":
		toolName = "add_observations"
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)

			return
		}
		if err := json.Unmarshal(body, &toolArgs); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)

			return
		}

	default:
		http.Error(w, "Unknown endpoint", http.StatusNotFound)

		return
	}

	// Create session with memory service
	session, err := d.inspectorService.CreateSession("memory")
	if err != nil {
		d.logger.Error("Failed to create memory session: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Failed to create session: %v"}`, err), http.StatusInternalServerError)

		return
	}

	d.logger.Info("Created session %s for memory server", session.ID)

	// Create the MCP request
	inspectorReq := InspectorRequest{
		SessionID: session.ID,
		Method:    "tools/call",
		Params:    json.RawMessage(fmt.Sprintf(`{"name": "%s", "arguments": %s}`, toolName, mustJSON(toolArgs))),
	}

	// Execute the request
	response, err := d.inspectorService.ExecuteRequest(session.ID, inspectorReq)
	if err != nil {
		d.logger.Error("Memory tool call failed: %v", err)
		http.Error(w, fmt.Sprintf(`{"error": "Tool call failed: %v"}`, err), http.StatusInternalServerError)

		return
	}

	// Clean up session
	if err := d.inspectorService.DestroySession(session.ID); err != nil {
		d.logger.Error("Failed to destroy session: %v", err)
	}

	// Return the result
	w.Header().Set("Content-Type", "application/json")

	// Extract the tool call result
	if response.Result != nil {
		// The result should contain the tool output
		if resultMap, ok := response.Result.(map[string]interface{}); ok {
			if content, exists := resultMap["content"]; exists {
				// Tool results are usually in content array
				if contentArray, ok := content.([]interface{}); ok && len(contentArray) > 0 {
					if contentItem, ok := contentArray[0].(map[string]interface{}); ok {
						if text, exists := contentItem["text"]; exists {
							// Try to parse as JSON, fallback to raw text
							var jsonResult interface{}
							if err := json.Unmarshal([]byte(text.(string)), &jsonResult); err == nil {
								if err := json.NewEncoder(w).Encode(jsonResult); err != nil {
									d.logger.Error("Failed to encode JSON result: %v", err)
								}

								return
							}
						}
					}
				}
			}
			// Fallback to returning the whole result
			if err := json.NewEncoder(w).Encode(resultMap); err != nil {
				d.logger.Error("Failed to encode result map: %v", err)
			}

			return
		}
		if err := json.NewEncoder(w).Encode(response.Result); err != nil {
			d.logger.Error("Failed to encode response result: %v", err)
		}
	} else if response.Error != nil {
		w.WriteHeader(http.StatusInternalServerError)
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"error": response.Error,
		}); err != nil {
			d.logger.Error("Failed to encode error response: %v", err)
		}
	} else {
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "success",
		}); err != nil {
			d.logger.Error("Failed to encode success response: %v", err)
		}
	}
}

// Separate handlers are kept for backwards compatibility but delegate to proxy
func (d *DashboardServer) handleMemoryStats(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}

func (d *DashboardServer) handleMemoryEntities(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}

func (d *DashboardServer) handleMemoryEntity(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}

func (d *DashboardServer) handleMemoryRelationships(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}

func (d *DashboardServer) handleMemorySearch(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}

func (d *DashboardServer) handleMemoryObservations(w http.ResponseWriter, r *http.Request) {
	d.handleMemoryProxy(w, r)
}
