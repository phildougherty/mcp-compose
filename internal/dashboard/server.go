package dashboard

import (
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

	"mcpcompose/internal/config"
	"mcpcompose/internal/constants"
	"mcpcompose/internal/container"
	"mcpcompose/internal/logging"

	"github.com/gorilla/websocket"
)

//go:embed templates/*
var templates embed.FS

//go:embed templates/static/*
var static embed.FS

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

	// Initialize inspector service
	server.inspectorService = NewInspectorService(server.logger, proxyURL, apiKey)

	// Start cleanup goroutine
	go server.startInspectorCleanup()

	return server
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

	// Serve static files
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

	// Main dashboard
	mux.HandleFunc("/", d.handleIndex)
	d.logger.Info("Registered: /")

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
