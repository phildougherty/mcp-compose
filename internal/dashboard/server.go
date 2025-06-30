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
		panic(fmt.Errorf("failed to parse templates: %w", err))
	}

	server := &DashboardServer{
		config:    cfg,
		runtime:   runtime,
		logger:    logging.NewLogger(cfg.Logging.Level),
		proxyURL:  proxyURL,
		apiKey:    apiKey,
		templates: tmpl,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // In production, implement proper origin checking
			},
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	// Initialize inspector service
	server.inspectorService = NewInspectorService(server.logger, proxyURL, apiKey)

	// Start cleanup goroutine
	go server.startInspectorCleanup()

	return server
}

func (d *DashboardServer) startInspectorCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		count := d.inspectorService.CleanupExpiredSessions(30 * time.Minute)
		if count > 0 {
			d.logger.Info("Cleaned up %d expired inspector sessions", count)
		}
	}
}

func (d *DashboardServer) Start(port int, host string) error {
	mux := http.NewServeMux()

	// Serve static files - CHOOSE ONE APPROACH
	// Option 1: Use embedded static files (recommended)
	staticFS, err := fs.Sub(static, "templates/static")
	if err != nil {
		d.logger.Warning("Failed to create embedded static file system: %v, using fallback", err)
		// Fallback to basic static handler
		mux.HandleFunc("/static/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, ".css") {
				w.Header().Set("Content-Type", "text/css")
				w.Write([]byte(`/* Basic fallback CSS */`))
			} else if strings.HasSuffix(r.URL.Path, ".js") {
				w.Header().Set("Content-Type", "application/javascript")
				w.Write([]byte(`// Basic fallback JS`))
			} else {
				http.NotFound(w, r)
			}
		})
	} else {
		// Use embedded static files
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
		d.logger.Info("Serving embedded static files")
	}

	// Main dashboard
	mux.HandleFunc("/", d.handleIndex)

	// Existing API endpoints with proper method handling
	mux.HandleFunc("/api/servers", d.handleAPIRequest(d.handleServers))
	mux.HandleFunc("/api/status", d.handleAPIRequest(d.handleStatus))
	mux.HandleFunc("/api/connections", d.handleAPIRequest(d.handleConnections))
	mux.HandleFunc("/api/containers/", d.handleContainers)
	mux.HandleFunc("/api/logs/", d.handleLogs)

	// WebSocket endpoints
	mux.HandleFunc("/ws/logs", d.handleLogWebSocket)
	mux.HandleFunc("/ws/metrics", d.handleMetricsWebSocket)
	mux.HandleFunc("/ws/activity", d.handleActivityWebSocket)
	mux.HandleFunc("/api/activity", d.handleActivityReceive)

	// Server control endpoints
	mux.HandleFunc("/api/servers/start", d.handleServerStart)
	mux.HandleFunc("/api/servers/stop", d.handleServerStop)
	mux.HandleFunc("/api/servers/restart", d.handleServerRestart)
	mux.HandleFunc("/api/proxy/reload", d.handleProxyReload)

	// Server documentation endpoints
	mux.HandleFunc("/api/server-docs/", d.handleServerDocs)
	mux.HandleFunc("/api/server-openapi/", d.handleServerOpenAPI)
	mux.HandleFunc("/api/server-direct/", d.handleServerDirect)
	mux.HandleFunc("/api/server-logs/", d.handleServerLogs)

	// Inspector endpoints
	mux.HandleFunc("/api/inspector/connect", d.handleInspectorConnect)
	mux.HandleFunc("/api/inspector/request", d.handleInspectorRequest)
	mux.HandleFunc("/api/inspector/disconnect", d.handleInspectorDisconnect)

	// OAuth and Audit endpoints (proxy to main server)
	mux.HandleFunc("/api/oauth/status", d.handleOAuthStatus)
	mux.HandleFunc("/api/oauth/clients", d.handleOAuthClients)
	mux.HandleFunc("/api/oauth/clients/", d.handleOAuthClients) // For DELETE with client ID
	mux.HandleFunc("/api/oauth/scopes", d.handleOAuthScopes)
	mux.HandleFunc("/oauth/register", d.handleOAuthRegister)
	mux.HandleFunc("/api/audit/entries", d.handleAuditEntries)
	mux.HandleFunc("/api/audit/stats", d.handleAuditStats)
	mux.HandleFunc("/oauth/token", d.handleOAuthToken)
	mux.HandleFunc("/oauth/authorize", d.handleOAuthAuthorize)
	mux.HandleFunc("/oauth/callback", d.handleOAuthCallback)

	// OAuth API proxying routes - NEW FOR SERVER-SPECIFIC OAUTH
	mux.HandleFunc("/api/servers/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is an OAuth-related server API call
		if strings.Contains(r.URL.Path, "/oauth") ||
			strings.Contains(r.URL.Path, "/test-oauth") ||
			strings.Contains(r.URL.Path, "/tokens") {
			d.handleOAuthAPIProxy(w, r)
			return
		}
		// For other server API calls, use existing proxy logic
		d.handleAPIProxy(w, r)
	})

	// General API proxying for other endpoints
	mux.HandleFunc("/api/", d.handleAPIProxy)

	// REMOVED THE DUPLICATE STATIC FILE REGISTRATION HERE

	// Start server...
	addr := fmt.Sprintf("%s:%d", host, port)
	d.logger.Info("Starting MCP-Compose Dashboard at http://%s", addr)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
	defer resp.Body.Close()

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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"container": containerName,
		"logs":      logs,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
