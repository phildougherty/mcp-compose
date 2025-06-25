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
	config     *config.ComposeConfig
	runtime    container.Runtime
	logger     *logging.Logger
	upgrader   websocket.Upgrader
	proxyURL   string
	apiKey     string
	templates  *template.Template
	httpClient *http.Client
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

	return &DashboardServer{
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
}

func (d *DashboardServer) Start(port int, host string) error {
	mux := http.NewServeMux()

	// Serve static files
	staticFS, err := fs.Sub(static, "templates/static")
	if err != nil {
		return fmt.Errorf("failed to create static file system: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	mux.HandleFunc("/", d.handleIndex)
	mux.HandleFunc("/api/servers", d.handleServers)
	mux.HandleFunc("/api/status", d.handleStatus)
	mux.HandleFunc("/api/connections", d.handleConnections)
	mux.HandleFunc("/api/containers/", d.handleContainers)
	mux.HandleFunc("/api/logs/", d.handleLogs)
	mux.HandleFunc("/ws/logs", d.handleLogWebSocket)
	mux.HandleFunc("/ws/metrics", d.handleMetricsWebSocket)
	mux.HandleFunc("/ws/activity", d.handleActivityWebSocket)
	mux.HandleFunc("/api/activity", d.handleActivityReceive)
	mux.HandleFunc("/api/servers/start", d.handleServerStart)
	mux.HandleFunc("/api/servers/stop", d.handleServerStop)
	mux.HandleFunc("/api/servers/restart", d.handleServerRestart)
	mux.HandleFunc("/api/proxy/reload", d.handleProxyReload)
	mux.HandleFunc("/api/server-docs/", d.handleServerDocs)
	mux.HandleFunc("/api/server-openapi/", d.handleServerOpenAPI)
	mux.HandleFunc("/api/server-direct/", d.handleServerDirect)
	mux.HandleFunc("/api/server-logs/", d.handleServerLogs)
	mux.HandleFunc("/", d.handleCatchAll)

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
