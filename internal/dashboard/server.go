package dashboard

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/constants"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/logging"

	"github.com/gorilla/websocket"
	_ "github.com/lib/pq"
)

//go:embed templates/*
var templates embed.FS

//go:embed templates/static/*
var static embed.FS

var reactFrontendAvailable = false

type DashboardServer struct {
	config           *config.ComposeConfig
	runtime          container.Runtime
	logger           *logging.Logger
	upgrader         websocket.Upgrader
	proxyURL         string
	apiKey           string
	templates        *template.Template
	httpClient       *http.Client
	inspectorService *InspectorService
	chatBroadcaster  *ChatBroadcaster
	chatService      *ChatService
	registryService  *RegistryService
	workflowHandler  *WorkflowHandler
	serverManager    ServerManager
	taskScheduler    TaskSchedulerManager
	memoryManager    MemoryManager
	aiManager        *ai.Manager
	mux              *http.ServeMux
}

type PageData struct {
	Title    string
	ProxyURL string
	APIKey   string
	Theme    string
	Port     int
}

func NewDashboardServer(cfg *config.ComposeConfig, runtime container.Runtime, proxyURL, apiKey string) *DashboardServer {
	// Override config with environment variables if running in container
	if envProxyURL := os.Getenv("MCP_PROXY_URL"); envProxyURL != "" {
		proxyURL = envProxyURL
		fmt.Printf("Using proxy URL from environment: %s\n", proxyURL)
	}

	if envAPIKey := os.Getenv("MCP_API_KEY"); envAPIKey != "" {
		apiKey = envAPIKey
	}

	// Override dashboard port from environment
	dashboardPort := cfg.Dashboard.Port
	if envPort := os.Getenv("MCP_DASHBOARD_PORT"); envPort != "" {
		if port, err := strconv.Atoi(envPort); err == nil {
			dashboardPort = port
			cfg.Dashboard.Port = port
		}
	}

	// Override dashboard host from environment
	if envHost := os.Getenv("MCP_DASHBOARD_HOST"); envHost != "" {
		cfg.Dashboard.Host = envHost
	}

	fmt.Printf("Dashboard will connect to proxy at: %s\n", proxyURL)
	fmt.Printf("Dashboard will listen on: %s:%d\n", cfg.Dashboard.Host, dashboardPort)

	// Parse templates with custom functions
	funcMap := template.FuncMap{
		"json": func(v interface{}) (string, error) {
			b, err := json.Marshal(v)

			return string(b), err
		},
	}

	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templates, "templates/*.html")
	if err != nil {
		// Return error instead of panicking for better error handling
		fmt.Printf("FATAL: Failed to parse dashboard templates: %v\n", err)
		os.Exit(1)
	}

	server := &DashboardServer{
		config:    cfg,
		runtime:   runtime,
		logger:    logging.NewLogger(cfg.Logging.Level),
		proxyURL:  proxyURL,
		apiKey:    apiKey,
		templates: tmpl,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  constants.WebSocketBufferSize,
			WriteBufferSize: constants.WebSocketBufferSize,
			CheckOrigin: func(r *http.Request) bool {

				return true // In production, implement proper origin checking
			},
		},
		httpClient: &http.Client{
			Timeout: func() time.Duration {
				// Get configurable timeout or use default
				if len(cfg.Connections) > 0 {
					for _, conn := range cfg.Connections {

						return conn.Timeouts.GetConnectTimeout()
					}
				}

				return constants.DefaultStatsTimeout // Default fallback
			}(),
		},
	}

	// Check if React frontend is available
	if _, err := GetFrontendFS(); err == nil {
		reactFrontendAvailable = true
		server.logger.Info("React frontend detected and available")
	} else {
		server.logger.Info("React frontend not available, using legacy Vue.js templates")
	}

	// Initialize inspector service
	server.inspectorService = NewInspectorService(server.logger, proxyURL, apiKey)

	// Initialize chat broadcaster
	server.chatBroadcaster = NewChatBroadcaster(server.logger)
	server.chatBroadcaster.start()

	// Initialize chat service if PostgreSQL is configured
	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		postgresURL = cfg.Dashboard.PostgresURL
	}

	if postgresURL != "" {
		db, err := sql.Open("postgres", postgresURL)
		if err != nil {
			server.logger.Error("Failed to connect to PostgreSQL: %v", err)
		} else {
			chatStorage, err := NewChatStorage(db)
			if err != nil {
				server.logger.Error("Failed to initialize chat storage: %v", err)
				db.Close()
			} else {
				aiManager, err := server.initializeAIManager(cfg)
				if err != nil {
					server.logger.Error("Failed to initialize AI manager: %v", err)
					db.Close()
				} else {
					server.aiManager = aiManager
					systemTools := server.createSystemToolsManager(cfg, runtime)
					server.chatService = NewChatService(aiManager, chatStorage, systemTools, server.logger, server.chatBroadcaster)
					server.logger.Info("Chat service initialized successfully")
				}
			}
		}
	} else {
		server.logger.Info("PostgreSQL not configured, chat service disabled")
	}

	if err := server.initializeRegistryService(); err != nil {
		server.logger.Error("Failed to initialize registry service: %v", err)
	}

	var aiManager *ai.Manager
	if postgresURL != "" {
		db, err := sql.Open("postgres", postgresURL)
		if err == nil {
			aiMgr, err := server.initializeAIManager(cfg)
			if err != nil {
				server.logger.Warning("Failed to initialize AI manager for workflows: %v", err)
			} else {
				aiManager = aiMgr
				if server.aiManager == nil {
					server.aiManager = aiMgr
				}
			}

			if err := server.initializeWorkflowHandler(db, aiManager); err != nil {
				server.logger.Error("Failed to initialize workflow handler: %v", err)
			}
		}
	}

	// Start cleanup goroutine
	go server.startInspectorCleanup()

	return server
}

func (d *DashboardServer) SetServerManager(mgr ServerManager) {
	d.serverManager = mgr
}

func (d *DashboardServer) SetTaskScheduler(ts TaskSchedulerManager) {
	d.taskScheduler = ts
}

func (d *DashboardServer) SetMemoryManager(mm MemoryManager) {
	d.memoryManager = mm
}

func (d *DashboardServer) Shutdown() {
	if d.chatBroadcaster != nil {
		d.chatBroadcaster.Stop()
	}

	if serverStatusBroadcaster.running {
		serverStatusBroadcaster.Stop()
	}

	if d.workflowHandler != nil {
		d.workflowHandler.Shutdown()
	}
}

func (d *DashboardServer) startInspectorCleanup() {
	ticker := time.NewTicker(constants.DefaultCleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		count := d.inspectorService.CleanupExpiredSessions(constants.DefaultSessionCleanupTime)
		if count > 0 {
			d.logger.Info("Cleaned up %d expired inspector sessions", count)
		}
	}
}

func (d *DashboardServer) Start(port int, host string) error {
	mux := http.NewServeMux()

	// Add debug logging
	d.logger.Info("=== REGISTERING ROUTES ===")

	// Serve static files based on frontend availability
	if reactFrontendAvailable {
		// Serve React frontend
		frontendFS, err := GetFrontendFS()
		if err != nil {
			d.logger.Error("Failed to get React frontend FS: %v", err)
		} else {
			// Serve assets directly (JS, CSS, images, etc.)
			mux.Handle("/assets/", http.FileServer(http.FS(frontendFS)))
			d.logger.Info("Registered: /assets/ (React frontend assets)")
		}
	} else {
		// Serve legacy static files
		staticFS, err := fs.Sub(static, "templates/static")
		if err != nil {
			d.logger.Warning("Failed to create embedded static file system: %v, using fallback", err)
			mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, ".css") {
					w.Header().Set("Content-Type", "text/css")
					if _, err := w.Write([]byte(`/* Basic fallback CSS */`)); err != nil {
						d.logger.Error("Failed to write CSS fallback: %v", err)
					}
				} else if strings.HasSuffix(r.URL.Path, ".js") {
					w.Header().Set("Content-Type", "application/javascript")
					if _, err := w.Write([]byte(`// Basic fallback JS`)); err != nil {
						d.logger.Error("Failed to write JS fallback: %v", err)
					}
				} else {
					http.NotFound(w, r)
				}
			})
		} else {
			mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
			d.logger.Info("Registered: /static/")
		}
	}

	// CRITICAL: CONTAINERS ROUTE MUST BE FIRST - Register with explicit logging
	d.logger.Info("Registering containers route: /api/containers/")
	mux.HandleFunc("/api/containers/", func(w http.ResponseWriter, r *http.Request) {
		d.logger.Info("=== CONTAINERS ROUTE HIT ===")
		d.logger.Info("Method: %s", r.Method)
		d.logger.Info("URL.Path: %s", r.URL.Path)
		d.logger.Info("URL.RawQuery: %s", r.URL.RawQuery)
		d.logger.Info("Host: %s", r.Host)
		d.handleContainers(w, r)
	})

	// Specific API endpoints - ALL MUST BE BEFORE CATCH-ALL
	mux.HandleFunc("/api/servers", d.handleAPIRequest(d.handleServers))
	d.logger.Info("Registered: /api/servers")

	mux.HandleFunc("/api/status", d.handleAPIRequest(d.handleStatus))
	d.logger.Info("Registered: /api/status")

	mux.HandleFunc("/api/connections", d.handleAPIRequest(d.handleConnections))
	d.logger.Info("Registered: /api/connections")

	mux.HandleFunc("/api/logs/", d.handleLogs)
	d.logger.Info("Registered: /api/logs/")

	mux.HandleFunc("/api/activity", d.handleActivityReceive)
	d.logger.Info("Registered: /api/activity")

	// Server control endpoints
	mux.HandleFunc("/api/servers/start", d.handleServerStart)
	d.logger.Info("Registered: /api/servers/start")

	mux.HandleFunc("/api/servers/stop", d.handleServerStop)
	d.logger.Info("Registered: /api/servers/stop")

	mux.HandleFunc("/api/servers/restart", d.handleServerRestart)
	d.logger.Info("Registered: /api/servers/restart")

	mux.HandleFunc("/api/proxy/reload", d.handleProxyReload)
	d.logger.Info("Registered: /api/proxy/reload")

	// Server documentation endpoints
	mux.HandleFunc("/api/server-docs/", d.handleServerDocs)
	d.logger.Info("Registered: /api/server-docs/")

	mux.HandleFunc("/api/server-openapi/", d.handleServerOpenAPI)
	d.logger.Info("Registered: /api/server-openapi/")

	mux.HandleFunc("/api/server-direct/", d.handleServerDirect)
	d.logger.Info("Registered: /api/server-direct/")

	mux.HandleFunc("/api/server-logs/", d.handleServerLogs)
	d.logger.Info("Registered: /api/server-logs/")

	// OAuth and security endpoints
	mux.HandleFunc("/api/oauth/status", d.handleOAuthStatus)
	d.logger.Info("Registered: /api/oauth/status")

	mux.HandleFunc("/api/oauth/clients/", d.handleOAuthClients)
	d.logger.Info("Registered: /api/oauth/clients/")

	mux.HandleFunc("/api/oauth/clients", d.handleOAuthClients)
	d.logger.Info("Registered: /api/oauth/clients")

	mux.HandleFunc("/api/oauth/scopes", d.handleOAuthScopes)
	d.logger.Info("Registered: /api/oauth/scopes")

	mux.HandleFunc("/oauth/register", d.handleOAuthRegister)
	d.logger.Info("Registered: /oauth/register")

	mux.HandleFunc("/oauth/token", d.handleOAuthToken)
	d.logger.Info("Registered: /oauth/token")

	mux.HandleFunc("/oauth/authorize", d.handleOAuthAuthorize)
	d.logger.Info("Registered: /oauth/authorize")

	mux.HandleFunc("/oauth/callback", d.handleOAuthCallback)
	d.logger.Info("Registered: /oauth/callback")

	// Audit endpoints
	mux.HandleFunc("/api/audit/entries", d.handleAuditEntries)
	d.logger.Info("Registered: /api/audit/entries")

	mux.HandleFunc("/api/audit/stats", d.handleAuditStats)
	d.logger.Info("Registered: /api/audit/stats")

	// Activity endpoints
	mux.HandleFunc("/ws/activity", d.handleActivityWebSocket)
	d.logger.Info("Registered: /ws/activity")

	mux.HandleFunc("/api/activity/history", d.handleActivityHistory)
	d.logger.Info("Registered: /api/activity/history")

	mux.HandleFunc("/api/activity/stats", d.handleActivityStats)
	d.logger.Info("Registered: /api/activity/stats")

	// WebSocket endpoints
	mux.HandleFunc("/ws/dashboard", d.handleDashboardWebSocket)
	d.logger.Info("Registered: /ws/dashboard")

	mux.HandleFunc("/ws/logs", d.handleLogWebSocket)
	d.logger.Info("Registered: /ws/logs")

	mux.HandleFunc("/ws/metrics", d.handleMetricsWebSocket)
	d.logger.Info("Registered: /ws/metrics")

	// Inspector endpoints
	mux.HandleFunc("/api/inspector/connect", d.handleInspectorConnect)
	d.logger.Info("Registered: /api/inspector/connect")

	mux.HandleFunc("/api/inspector/request", d.handleInspectorRequest)
	d.logger.Info("Registered: /api/inspector/request")

	mux.HandleFunc("/api/inspector/disconnect", d.handleInspectorDisconnect)
	d.logger.Info("Registered: /api/inspector/disconnect")

	// Memory endpoints
	mux.HandleFunc("/api/memory/stats", d.handleMemoryStats)
	d.logger.Info("Registered: /api/memory/stats")

	mux.HandleFunc("/api/memory/entities", d.handleMemoryEntities)
	d.logger.Info("Registered: /api/memory/entities")

	mux.HandleFunc("/api/memory/entities/", d.handleMemoryEntity)
	d.logger.Info("Registered: /api/memory/entities/")

	mux.HandleFunc("/api/memory/relationships", d.handleMemoryRelationships)
	d.logger.Info("Registered: /api/memory/relationships")

	mux.HandleFunc("/api/memory/search", d.handleMemorySearch)
	d.logger.Info("Registered: /api/memory/search")

	mux.HandleFunc("/api/memory/observations", d.handleMemoryObservations)
	d.logger.Info("Registered: /api/memory/observations")

	// Chat endpoints
	if d.chatService != nil {
		d.mux = mux
		d.registerChatRoutes()
		d.logger.Info("Chat service routes registered")
	} else {
		d.logger.Info("Chat service not available, skipping chat routes")
	}

	// AI models endpoint
	if d.aiManager != nil {
		mux.HandleFunc("/api/ai/models", d.handleListModels)
		d.logger.Info("Registered: /api/ai/models")
	}

	// Registry endpoints
	if d.registryService != nil {
		d.mux = mux
		d.registerRegistryRoutes()
		d.logger.Info("Registry service routes registered")
	} else {
		d.logger.Info("Registry service not available, skipping registry routes")
	}

	// Workflow endpoints
	if d.workflowHandler != nil {
		d.mux = mux
		d.registerWorkflowRoutes()
		d.logger.Info("Workflow service routes registered")
	} else {
		d.logger.Info("Workflow service not available, skipping workflow routes")
	}

	// Task scheduler endpoints (if available)
	if d.inspectorService != nil {
		mux.HandleFunc("/api/task-scheduler/health", d.handleTaskSchedulerHealth)
		d.logger.Info("Registered: /api/task-scheduler/health")

		mux.HandleFunc("/api/task-scheduler/", d.handleTaskSchedulerProxy)
		d.logger.Info("Registered: /api/task-scheduler/")
	} else {
		d.logger.Info("Inspector service not available, skipping task scheduler routes")
	}

	// Server-specific OAuth endpoints - MUST be before catch-all /api/servers/
	mux.HandleFunc("/api/servers/", func(w http.ResponseWriter, r *http.Request) {
		d.logger.Info("=== SERVER-SPECIFIC ROUTE HIT ===")
		d.logger.Info("Method: %s", r.Method)
		d.logger.Info("URL.Path: %s", r.URL.Path)

		if strings.Contains(r.URL.Path, "/oauth") ||
			strings.Contains(r.URL.Path, "/test-oauth") ||
			strings.Contains(r.URL.Path, "/tokens") {
			d.logger.Info("Routing to OAuth API proxy")
			d.handleOAuthAPIProxy(w, r)

			return
		}
		d.logger.Info("Routing to general API proxy")
		d.handleAPIProxy(w, r)
	})
	d.logger.Info("Registered: /api/servers/ (with OAuth routing)")

	// CATCH-ALL ROUTES - THESE MUST BE ABSOLUTELY LAST
	d.logger.Info("Registering catch-all: /api/")
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		d.logger.Info("=== CATCH-ALL API ROUTE HIT ===")
		d.logger.Info("Method: %s", r.Method)
		d.logger.Info("URL.Path: %s", r.URL.Path)
		d.logger.Info("WARNING: This should NOT happen for /api/containers/ requests!")
		d.handleAPIProxy(w, r)
	})

	d.logger.Info("=== ALL ROUTES REGISTERED ===")
	d.logger.Info("Route registration order:")
	d.logger.Info("1. /api/containers/ (SPECIFIC)")
	d.logger.Info("2. Other specific /api/ routes")
	d.logger.Info("3. /api/servers/ (SPECIFIC with OAuth routing)")
	d.logger.Info("4. /api/ (CATCH-ALL - LAST)")

	// Main dashboard route - SPA fallback (MUST BE LAST)
	if reactFrontendAvailable {
		mux.HandleFunc("/", d.handleReactIndex)
		d.logger.Info("Registered: / (React SPA with fallback)")
	} else {
		mux.HandleFunc("/", d.handleIndex)
		d.logger.Info("Registered: / (legacy Vue.js template)")
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", host, port)
	d.logger.Info("Starting MCP-Compose Dashboard at http://%s", addr)

	// Get configurable timeouts or use defaults
	readTimeout := constants.ShortTimeout
	writeTimeout := constants.ShortTimeout
	idleTimeout := constants.DefaultIdleTimeout

	if len(d.config.Connections) > 0 {
		for _, conn := range d.config.Connections {
			readTimeout = conn.Timeouts.GetReadTimeout()
			writeTimeout = conn.Timeouts.GetWriteTimeout()
			idleTimeout = conn.Timeouts.GetIdleTimeout()

			break
		}
	}

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	d.logger.Info("Dashboard server starting...")

	return server.ListenAndServe()
}

// Helper to handle API methods properly
func (d *DashboardServer) handleAPIRequest(handler func(http.ResponseWriter, *http.Request)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// Support HEAD for all API endpoints
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			return
		}
		handler(w, r)
	}
}

func (d *DashboardServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := PageData{
		Title:    "MCP-Compose Dashboard",
		ProxyURL: d.proxyURL,
		APIKey:   d.apiKey,
		Theme:    d.config.Dashboard.Theme,
		Port:     d.config.Dashboard.Port,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := d.templates.ExecuteTemplate(w, "index.html", data); err != nil {
		d.logger.Error("Failed to execute template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// proxyRequest forwards requests to the MCP proxy
func (d *DashboardServer) proxyRequest(endpoint string) ([]byte, error) {
	url := d.proxyURL + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {

		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if d.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+d.apiKey)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {

		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			d.logger.Error("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)

		return nil, fmt.Errorf("proxy returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (d *DashboardServer) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Extract server name from path
	path := r.URL.Path[len("/api/logs/"):]
	if path == "" {
		http.Error(w, "Server name required", http.StatusBadRequest)

		return
	}

	containerName := "mcp-compose-" + path
	tail := r.URL.Query().Get("tail")
	if tail == "" {
		tail = "100"
	}

	logs, err := d.getContainerLogs(containerName, tail, false)
	if err != nil {
		d.logger.Error("Failed to get logs for %s: %v", containerName, err)
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"container": containerName,
		"logs":      logs,
		"timestamp": time.Now().Format(time.RFC3339),
	}); err != nil {
		d.logger.Error("Failed to encode JSON response: %v", err)
	}
}

func (d *DashboardServer) handleActivityHistory(w http.ResponseWriter, r *http.Request) {
	if activityBroadcaster.storage == nil {
		http.Error(w, "Activity storage not available", http.StatusServiceUnavailable)

		return
	}

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	sinceStr := r.URL.Query().Get("since")

	limit := 100 // default
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	var since *time.Time
	if sinceStr != "" {
		if parsedSince, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &parsedSince
		}
	}

	activities, err := activityBroadcaster.storage.GetRecentActivities(limit, since)
	if err != nil {
		http.Error(w, "Failed to retrieve activities", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"activities": activities,
		"count":      len(activities),
	}); err != nil {
		d.logger.Error("Failed to encode JSON response: %v", err)
	}
}

func (d *DashboardServer) handleActivityStats(w http.ResponseWriter, r *http.Request) {
	if activityBroadcaster.storage == nil {
		http.Error(w, "Activity storage not available", http.StatusServiceUnavailable)

		return
	}

	stats, err := activityBroadcaster.storage.GetActivityStats()
	if err != nil {
		http.Error(w, "Failed to retrieve activity stats", http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (d *DashboardServer) initializeAIManager(cfg *config.ComposeConfig) (*ai.Manager, error) {
	managerConfig := &ai.ManagerConfig{
		Providers: []ai.Provider{},
	}

	// OpenAI
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		provider, err := ai.NewOpenAIProvider(&ai.OpenAIConfig{APIKey: apiKey})
		if err == nil {
			managerConfig.Providers = append(managerConfig.Providers, provider)
		}
	}

	// Anthropic
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		provider, err := ai.NewClaudeProvider(&ai.ClaudeConfig{APIKey: apiKey})
		if err == nil {
			managerConfig.Providers = append(managerConfig.Providers, provider)
		}
	}

	// OpenRouter
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		provider, err := ai.NewOpenRouterProvider(&ai.OpenRouterConfig{APIKey: apiKey})
		if err == nil {
			managerConfig.Providers = append(managerConfig.Providers, provider)
		}
	}

	// Ollama - always try to add with default or custom URL
	ollamaBaseURL := os.Getenv("OLLAMA_BASE_URL")
	if ollamaBaseURL == "" {
		ollamaBaseURL = "http://localhost:11434"
	}

	d.logger.Info("Initializing Ollama provider with URL: %s", ollamaBaseURL)
	ollamaProvider, err := ai.NewOllamaProvider(&ai.OllamaConfig{BaseURL: ollamaBaseURL})
	if err == nil {
		managerConfig.Providers = append(managerConfig.Providers, ollamaProvider)
		d.logger.Info("Successfully added Ollama provider to AI manager")
	} else {
		d.logger.Error("Failed to create Ollama provider: %v", err)
	}

	d.logger.Info("Total providers configured: %d", len(managerConfig.Providers))
	for i, p := range managerConfig.Providers {
		d.logger.Info("  Provider %d: %s", i+1, p.Name())
	}

	if len(managerConfig.Providers) == 0 {
		return nil, fmt.Errorf("no AI providers configured")
	}

	return ai.NewManager(managerConfig)
}

func (d *DashboardServer) createSystemToolsManager(cfg *config.ComposeConfig, runtime container.Runtime) *SystemToolsManager {
	return NewSystemToolsManager(cfg, d.serverManager, d.taskScheduler, d.memoryManager)
}

func (d *DashboardServer) initializeWorkflowHandler(db *sql.DB, aiManager *ai.Manager) error {
	workflowStorage, err := NewWorkflowStorage(db)
	if err != nil {
		return fmt.Errorf("failed to create workflow storage: %w", err)
	}

	d.workflowHandler = NewWorkflowHandler(workflowStorage, d.logger)

	if aiManager != nil {
		d.workflowHandler.SetAIManager(aiManager)
		d.logger.Info("Workflow handler initialized with AI manager")
	} else {
		d.logger.Warning("Workflow handler initialized without AI manager - deployment API will not be available")
	}

	d.workflowHandler.SetMCPProxyURL(d.proxyURL)
	d.workflowHandler.SetMCPAPIKey(d.apiKey)
	d.logger.Info("Workflow handler configured with proxy URL: %s", d.proxyURL)

	templateStorage, err := NewTemplateStorage(db)
	if err != nil {
		d.logger.Warning("Failed to create template storage: %v", err)
	} else {
		d.workflowHandler.SetTemplateStorage(templateStorage)
		d.logger.Info("Template storage initialized successfully")
	}

	d.logger.Info("Workflow handler initialized successfully")

	return nil
}

func (d *DashboardServer) registerWorkflowRoutes() {
	if d.workflowHandler != nil {
		d.workflowHandler.RegisterRoutes(d.mux)
		d.logger.Info("Workflow routes registered")
	}
}

func (d *DashboardServer) handleReactIndex(w http.ResponseWriter, r *http.Request) {
	// For SPA routing, serve index.html for all non-API/non-WebSocket routes
	// API and WebSocket routes should have already been handled by more specific handlers
	// If we're here for those paths, something is wrong with route registration
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/ws/") || strings.HasPrefix(r.URL.Path, "/oauth/") {
		d.logger.Warning("React index handler caught API/WS route: %s (route registration issue)", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	// Get the frontend filesystem
	frontendFS, err := GetFrontendFS()
	if err != nil {
		d.logger.Error("Failed to get React frontend FS: %v", err)
		http.Error(w, "Frontend not available", http.StatusInternalServerError)
		return
	}

	// Try to open index.html
	indexFile, err := frontendFS.Open("index.html")
	if err != nil {
		d.logger.Error("Failed to open index.html: %v", err)
		http.Error(w, "Frontend index not found", http.StatusInternalServerError)
		return
	}
	defer indexFile.Close()

	// Read and serve index.html
	content, err := io.ReadAll(indexFile)
	if err != nil {
		d.logger.Error("Failed to read index.html: %v", err)
		http.Error(w, "Failed to read frontend", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

func (d *DashboardServer) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	provider := r.URL.Query().Get("provider")
	if provider == "" {
		http.Error(w, "Provider parameter is required", http.StatusBadRequest)

		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var models []string
	var err error

	var providerName string
	switch provider {
	case "openrouter":
		providerName = "openrouter"
	case "openai":
		providerName = "openai"
	case "anthropic":
		providerName = "claude"
	case "local":
		providerName = "ollama"
	default:
		http.Error(w, "Unknown provider: "+provider, http.StatusBadRequest)

		return
	}

	p, err := d.aiManager.GetProvider(providerName)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s provider not configured: %v", provider, err), http.StatusBadRequest)

		return
	}

	models, err = p.ListModels(ctx)

	if err != nil {
		d.logger.Error("Failed to list models for provider %s: %v", provider, err)
		http.Error(w, "Failed to fetch models: "+err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"provider": provider,
		"models":   models,
	})
}
