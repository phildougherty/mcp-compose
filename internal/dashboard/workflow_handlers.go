package dashboard

import (
	"database/sql"
	"net/http"

	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/logging"
	"github.com/phildougherty/mcp-compose/internal/workflow"
)

type WorkflowHandler struct {
	apiHandler        *workflow.APIHandler
	deploymentHandler *workflow.DeploymentHandler
	templateHandler   *workflow.TemplateAPIHandler
	logger            *logging.Logger
}

func NewWorkflowStorage(db *sql.DB) (*workflow.Storage, error) {
	return workflow.NewStorage(db)
}

func NewTemplateStorage(db *sql.DB) (*workflow.TemplateStorage, error) {
	return workflow.NewTemplateStorage(db)
}

func NewWorkflowHandler(storage *workflow.Storage, logger *logging.Logger) *WorkflowHandler {
	return &WorkflowHandler{
		apiHandler: workflow.NewAPIHandler(storage, logger),
		logger:     logger,
	}
}

func (h *WorkflowHandler) SetAIManager(aiManager *ai.Manager) {
	if aiManager != nil {
		h.apiHandler.SetAIManager(aiManager)
		h.deploymentHandler = workflow.NewDeploymentHandler(
			h.apiHandler.GetStorage(),
			aiManager,
			h.logger,
		)
		h.logger.Info("Deployment handler initialized with AI manager")
	}
}

func (h *WorkflowHandler) SetTemplateStorage(templateStorage *workflow.TemplateStorage) {
	if templateStorage != nil {
		h.templateHandler = workflow.NewTemplateAPIHandler(
			templateStorage,
			h.apiHandler.GetStorage(),
			h.logger,
		)
		h.logger.Info("Template handler initialized")
	}
}

func (h *WorkflowHandler) SetMCPProxyURL(url string) {
	if h.apiHandler != nil {
		h.apiHandler.SetMCPProxyURL(url)
	}
}

func (h *WorkflowHandler) SetMCPAPIKey(apiKey string) {
	if h.apiHandler != nil {
		h.apiHandler.SetMCPAPIKey(apiKey)
	}
}

func (h *WorkflowHandler) RegisterRoutes(mux *http.ServeMux) {
	h.apiHandler.RegisterRoutes(mux)

	if h.deploymentHandler != nil {
		mux.HandleFunc("/api/workflows/deploy", h.deploymentHandler.HandleDeploy)
		h.logger.Info("Registered: POST /api/workflows/deploy - Deploy workflow from natural language")
	}

	if h.templateHandler != nil {
		h.templateHandler.RegisterRoutes(mux)
	}
}

func (h *WorkflowHandler) Shutdown() {
	h.apiHandler.Shutdown()
}
