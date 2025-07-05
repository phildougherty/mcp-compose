// internal/dashboard/inspector_handlers.go
package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (d *DashboardServer) handleInspectorConnect(w http.ResponseWriter, r *http.Request) {
	d.logger.Error("=== INSPECTOR CONNECT CALLED === THIS IS A TEST LOG")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body first for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		d.logger.Error("Failed to read request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Failed to read request body"}`, http.StatusBadRequest)
		return
	}

	d.logger.Info("Inspector connect request body: %s", string(bodyBytes))

	var request struct {
		Server string `json:"server"`
	}

	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		d.logger.Error("Failed to decode inspector connect request: %v. Body was: %s", err, string(bodyBytes))
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	d.logger.Info("Inspector connect request for server: '%s'", request.Server)

	if request.Server == "" {
		d.logger.Error("Empty server name in request. Full request: %+v", request)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error": "Server name required"}`, http.StatusBadRequest)
		return
	}

	// Health check endpoint
	if request.Server == "__healthcheck__" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "available",
		})
		return
	}

	if d.inspectorService == nil {
		d.logger.Error("Inspector service is nil during connect!")
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("Inspector service not available"), http.StatusInternalServerError)
		return
	}

	session, err := d.inspectorService.CreateSession(request.Server)
	if err != nil {
		d.logger.Error("Failed to create inspector session for %s: %v", request.Server, err)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError(err.Error()), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"sessionId": session.ID,
		"result": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]interface{}{
				"name":    session.ServerName,
				"version": "unknown",
			},
			"capabilities": session.Capabilities,
		},
	}

	d.logger.Info("Created inspector session %s for server %s", session.ID, request.Server)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (d *DashboardServer) handleInspectorRequest(w http.ResponseWriter, r *http.Request) {
	d.logger.Error("=== INSPECTOR REQUEST CALLED === THIS IS A TEST LOG")

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the raw body first for debugging
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		d.logger.Error("Failed to read inspector request body: %v", err)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("Failed to read request body"), http.StatusBadRequest)
		return
	}

	d.logger.Info("Inspector request body: %s", string(bodyBytes))

	var request struct {
		SessionID string          `json:"sessionId"`
		Method    string          `json:"method"`
		Params    json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(bodyBytes, &request); err != nil {
		d.logger.Error("Failed to decode inspector request: %v. Body was: %s", err, string(bodyBytes))
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("Invalid request body"), http.StatusBadRequest)
		return
	}

	d.logger.Info("Inspector request: sessionId='%s', method='%s', params=%s", request.SessionID, request.Method, string(request.Params))

	if request.SessionID == "" || request.Method == "" {
		d.logger.Error("Missing required fields: sessionId='%s', method='%s'", request.SessionID, request.Method)
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("SessionID and Method required"), http.StatusBadRequest)
		return
	}

	inspectorReq := InspectorRequest{
		SessionID: request.SessionID,
		Method:    request.Method,
		Params:    request.Params,
	}

	d.logger.Info("Executing inspector request: %s.%s", request.SessionID, request.Method)

	if d.inspectorService == nil {
		d.logger.Error("Inspector service is nil!")
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("Inspector service not available"), http.StatusInternalServerError)
		return
	}

	response, err := d.inspectorService.ExecuteRequest(request.SessionID, inspectorReq)
	if err != nil {
		d.logger.Error("Inspector request failed: %v", err)
		if strings.Contains(err.Error(), "not found") {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, jsonError(err.Error()), http.StatusNotFound)
		} else {
			w.Header().Set("Content-Type", "application/json")
			http.Error(w, jsonError(err.Error()), http.StatusInternalServerError)
		}
		return
	}

	d.logger.Info("Inspector request successful, sending response")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (d *DashboardServer) handleInspectorDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		SessionID string `json:"sessionId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("Invalid request body"), http.StatusBadRequest)
		return
	}

	if request.SessionID == "" {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError("SessionID required"), http.StatusBadRequest)
		return
	}

	err := d.inspectorService.DestroySession(request.SessionID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, jsonError(err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "disconnected",
	})
}

func jsonError(message string) string {
	return `{"error": "` + strings.ReplaceAll(message, `"`, `\"`) + `"}`
}
