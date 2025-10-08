package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

type TemplateAPIHandler struct {
	templateStorage  *TemplateStorage
	workflowStorage  *Storage
	logger           *logging.Logger
}

func NewTemplateAPIHandler(templateStorage *TemplateStorage, workflowStorage *Storage, logger *logging.Logger) *TemplateAPIHandler {
	return &TemplateAPIHandler{
		templateStorage:  templateStorage,
		workflowStorage:  workflowStorage,
		logger:           logger,
	}
}

func (h *TemplateAPIHandler) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	filter := TemplateFilter{
		Category:   r.URL.Query().Get("category"),
		Difficulty: r.URL.Query().Get("difficulty"),
		Search:     r.URL.Query().Get("search"),
		SortBy:     r.URL.Query().Get("sort"),
		Limit:      50,
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if tagsStr := r.URL.Query().Get("tags"); tagsStr != "" {
		filter.Tags = strings.Split(tagsStr, ",")
	}

	templates, err := h.templateStorage.ListTemplates(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list templates: %v", err)
		http.Error(w, "Failed to list templates", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleGetTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	templateID := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	templateID = strings.Split(templateID, "/")[0]

	if templateID == "" {
		http.Error(w, "Template ID required", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	template, err := h.templateStorage.GetTemplate(ctx, templateID)
	if err != nil {
		h.logger.Error("Failed to get template: %v", err)
		if err.Error() == "template not found" {
			http.Error(w, "Template not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to get template", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(template); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	var template Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.logger.Error("Failed to decode template: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.templateStorage.CreateTemplate(ctx, &template); err != nil {
		h.logger.Error("Failed to create template: %v", err)
		http.Error(w, "Failed to create template", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(template); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	templateID := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	templateID = strings.Split(templateID, "/")[0]

	if templateID == "" {
		http.Error(w, "Template ID required", http.StatusBadRequest)

		return
	}

	var template Template
	if err := json.NewDecoder(r.Body).Decode(&template); err != nil {
		h.logger.Error("Failed to decode template: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)

		return
	}

	template.ID = templateID

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.templateStorage.UpdateTemplate(ctx, &template); err != nil {
		h.logger.Error("Failed to update template: %v", err)
		if err.Error() == "template not found" {
			http.Error(w, "Template not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update template", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(template); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	templateID := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	templateID = strings.Split(templateID, "/")[0]

	if templateID == "" {
		http.Error(w, "Template ID required", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.templateStorage.DeleteTemplate(ctx, templateID); err != nil {
		h.logger.Error("Failed to delete template: %v", err)
		if err.Error() == "template not found" {
			http.Error(w, "Template not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete template", http.StatusInternalServerError)
		}

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Template deleted successfully",
		"id":      templateID,
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleGetCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	categoryCounts, err := h.templateStorage.GetCategoryCounts(ctx)
	if err != nil {
		h.logger.Error("Failed to get category counts: %v", err)
		http.Error(w, "Failed to get categories", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"categories": categoryCounts,
		"count":      len(categoryCounts),
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) HandleInstallTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	templateID := strings.TrimPrefix(r.URL.Path, "/api/templates/")
	templateID = strings.TrimSuffix(templateID, "/install")

	if templateID == "" {
		http.Error(w, "Template ID required", http.StatusBadRequest)

		return
	}

	var req InstallTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Parameters = make(map[string]interface{})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	workflowID, err := h.templateStorage.InstallTemplate(ctx, h.workflowStorage, templateID, req.Parameters)
	if err != nil {
		h.logger.Error("Failed to install template: %v", err)
		http.Error(w, fmt.Sprintf("Failed to install template: %v", err), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(InstallTemplateResponse{
		WorkflowID: workflowID,
		Message:    "Template installed successfully",
	}); err != nil {
		h.logger.Error("Failed to encode response: %v", err)
	}
}

func (h *TemplateAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/templates/categories", h.HandleGetCategories)

	mux.HandleFunc("/api/templates", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			h.HandleListTemplates(w, r)
		case http.MethodPost:
			h.HandleCreateTemplate(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/templates/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.HasSuffix(path, "/install") {
			h.HandleInstallTemplate(w, r)

			return
		}

		switch r.Method {
		case http.MethodGet:
			h.HandleGetTemplate(w, r)
		case http.MethodPut:
			h.HandleUpdateTemplate(w, r)
		case http.MethodDelete:
			h.HandleDeleteTemplate(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	fmt.Println("Registered template API routes:")
	fmt.Println("  GET    /api/templates              - List all templates (with filters)")
	fmt.Println("  POST   /api/templates              - Create template")
	fmt.Println("  GET    /api/templates/:id          - Get template by ID")
	fmt.Println("  PUT    /api/templates/:id          - Update template")
	fmt.Println("  DELETE /api/templates/:id          - Delete template")
	fmt.Println("  GET    /api/templates/categories   - List categories with counts")
	fmt.Println("  POST   /api/templates/:id/install  - Install template as workflow")
}
