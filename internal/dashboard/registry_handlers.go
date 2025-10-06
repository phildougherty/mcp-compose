package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/registry"
)

type RegistryService struct {
	manager   *registry.Manager
	installer *registry.Installer
	registry  *registry.Registry
	db        *sql.DB
}

func (d *DashboardServer) initializeRegistryService() error {
	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		postgresURL = d.config.Dashboard.PostgresURL
	}

	if postgresURL == "" {
		d.logger.Info("PostgreSQL not configured, registry service disabled")
		return nil
	}

	db, err := sql.Open("postgres", postgresURL)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	manager, err := registry.NewManager(db, d.logger)
	if err != nil {
		db.Close()
		return fmt.Errorf("failed to create registry manager: %w", err)
	}

	configPath := os.Getenv("MCP_COMPOSE_CONFIG")
	if configPath == "" {
		configPath = "mcp-compose.yaml"
	}

	installer := registry.NewInstaller(configPath, d.logger)

	registryURL := os.Getenv("DOCKER_REGISTRY_URL")
	if registryURL == "" {
		registryURL = "http://localhost:5000"
	}

	dockerRegistry := registry.NewRegistry(registry.RegistryConfig{
		URL:     registryURL,
		Timeout: 30 * time.Second,
	}, d.logger)

	if err := dockerRegistry.HealthCheck(); err != nil {
		d.logger.Warning("Docker registry health check failed: %v", err)
	}

	d.registryService = &RegistryService{
		manager:   manager,
		installer: installer,
		registry:  dockerRegistry,
		db:        db,
	}

	d.logger.Info("Registry service initialized successfully")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		installedNames, err := installer.GetInstalledServerNames()
		if err != nil {
			d.logger.Warning("Failed to get installed server names from config: %v", err)
			return
		}

		if len(installedNames) > 0 {
			d.logger.Info("Syncing %d existing servers from config to database", len(installedNames))
			if err := manager.SyncConfigToDatabase(ctx, installedNames, "default"); err != nil {
				d.logger.Warning("Failed to sync config to database: %v", err)
			}
		}
	}()

	return nil
}

func (d *DashboardServer) registerRegistryRoutes() {
	if d.registryService == nil {
		d.logger.Info("Registry service not available, skipping registry routes")
		return
	}

	d.mux.HandleFunc("/api/registry/servers", d.handleRegistryServers)
	d.logger.Info("Registered: /api/registry/servers")

	d.mux.HandleFunc("/api/registry/servers/", d.handleRegistryServerDetail)
	d.logger.Info("Registered: /api/registry/servers/")

	d.mux.HandleFunc("/api/registry/categories", d.handleRegistryCategories)
	d.logger.Info("Registered: /api/registry/categories")

	d.mux.HandleFunc("/api/registry/featured", d.handleRegistryFeatured)
	d.logger.Info("Registered: /api/registry/featured")

	d.mux.HandleFunc("/api/registry/install", d.handleRegistryInstall)
	d.logger.Info("Registered: /api/registry/install")

	d.mux.HandleFunc("/api/registry/uninstall", d.handleRegistryUninstall)
	d.logger.Info("Registered: /api/registry/uninstall")

	d.mux.HandleFunc("/api/registry/install-and-start", d.handleRegistryInstallAndStart)
	d.logger.Info("Registered: /api/registry/install-and-start")

	d.mux.HandleFunc("/api/registry/installed", d.handleRegistryInstalled)
	d.logger.Info("Registered: /api/registry/installed")

	d.mux.HandleFunc("/api/registry/health", d.handleRegistryHealth)
	d.logger.Info("Registered: /api/registry/health")
}

func (d *DashboardServer) handleRegistryServers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	filter := &registry.ServerFilter{}

	if category := r.URL.Query().Get("category"); category != "" {
		filter.Category = category
	}

	if featuredStr := r.URL.Query().Get("featured"); featuredStr != "" {
		featured := featuredStr == "true"
		filter.Featured = &featured
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filter.SearchQuery = search
	}

	servers, err := d.registryService.manager.ListServers(ctx, filter)
	if err != nil {
		d.logger.Error("Failed to list servers: %v", err)
		http.Error(w, "Failed to list servers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryServerDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/registry/servers/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Server ID required", http.StatusBadRequest)
		return
	}

	serverID, err := strconv.Atoi(parts[0])
	if err != nil {
		http.Error(w, "Invalid server ID", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := d.registryService.manager.GetServer(ctx, serverID)
	if err != nil {
		d.logger.Error("Failed to get server: %v", err)
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	installed, err := d.registryService.installer.IsServerInstalled(server.Name)
	if err != nil {
		d.logger.Warning("Failed to check if server is installed: %v", err)
		installed = false
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"server":    server,
		"installed": installed,
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	categories, err := d.registryService.manager.GetCategories(ctx)
	if err != nil {
		d.logger.Error("Failed to get categories: %v", err)
		http.Error(w, "Failed to get categories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"categories": categories,
		"count":      len(categories),
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	featured := true
	filter := &registry.ServerFilter{
		Featured: &featured,
	}

	servers, err := d.registryService.manager.ListServers(ctx, filter)
	if err != nil {
		d.logger.Error("Failed to list featured servers: %v", err)
		http.Error(w, "Failed to list featured servers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ServerID int                    `json:"serverId"`
		Config   map[string]interface{} `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := d.registryService.manager.GetServer(ctx, req.ServerID)
	if err != nil {
		d.logger.Error("Failed to get server: %v", err)
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	installed, err := d.registryService.installer.IsServerInstalled(server.Name)
	if err != nil {
		d.logger.Error("Failed to check if server is installed: %v", err)
		http.Error(w, "Failed to check installation status", http.StatusInternalServerError)
		return
	}

	if installed {
		http.Error(w, "Server already installed", http.StatusConflict)
		return
	}

	if err := d.registryService.installer.InstallServerToConfig(server, req.Config); err != nil {
		d.logger.Error("Failed to install server to config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to install server: %v", err), http.StatusInternalServerError)
		return
	}

	d.logger.Info("Installed server '%s' with protocol '%s'", server.Name, server.Protocol)

	installReq := &registry.InstallRequest{
		ServerID: req.ServerID,
		UserID:   "default",
		Config:   req.Config,
	}

	if err := d.registryService.manager.InstallServer(ctx, installReq); err != nil {
		d.logger.Error("Failed to record installation: %v", err)
	}

	if err := d.reloadProxyConfig(); err != nil {
		d.logger.Warning("Server installed but failed to reload proxy config: %v", err)
	}

	startErr := d.startServerViaAPI(server.Name)
	if startErr != nil {
		d.logger.Warning("Server installed but failed to start: %v", startErr)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Server '%s' installed but failed to start: %v. Please run 'mcp-compose up %s' manually.", server.Name, startErr, server.Name),
			"server":  server,
		})

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Server '%s' installed and started successfully", server.Name),
		"server":  server,
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		ServerID int `json:"serverId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	server, err := d.registryService.manager.GetServer(ctx, req.ServerID)
	if err != nil {
		d.logger.Error("Failed to get server: %v", err)
		http.Error(w, "Server not found", http.StatusNotFound)
		return
	}

	if err := d.registryService.installer.UninstallServerFromConfig(server.Name); err != nil {
		d.logger.Error("Failed to uninstall server from config: %v", err)
		http.Error(w, fmt.Sprintf("Failed to uninstall server: %v", err), http.StatusInternalServerError)
		return
	}

	if err := d.registryService.manager.UninstallServer(ctx, req.ServerID, "default"); err != nil {
		d.logger.Error("Failed to record uninstallation: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Server '%s' uninstalled successfully", server.Name),
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryInstalled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.registryService == nil {
		http.Error(w, "Registry service not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	installed, err := d.registryService.manager.GetInstalledServers(ctx, "default")
	if err != nil {
		d.logger.Error("Failed to get installed servers: %v", err)
		http.Error(w, "Failed to get installed servers", http.StatusInternalServerError)
		return
	}

	serverDetails := make([]map[string]interface{}, 0, len(installed))
	for _, inst := range installed {
		server, err := d.registryService.manager.GetServer(ctx, inst.ServerID)
		if err != nil {
			d.logger.Warning("Failed to get server details for ID %d: %v", inst.ServerID, err)

			continue
		}

		serverDetails = append(serverDetails, map[string]interface{}{
			"installation": inst,
			"server":       server,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"installed": serverDetails,
		"count":     len(serverDetails),
	}); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}

func (d *DashboardServer) handleRegistryInstallAndStart(w http.ResponseWriter, r *http.Request) {
	d.handleRegistryInstall(w, r)
}

func (d *DashboardServer) reloadProxyConfig() error {
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/api/reload", proxyURL)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create reload request: %w", err)
	}

	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("config reload failed with status %d", resp.StatusCode)
	}

	d.logger.Info("Proxy config reloaded successfully")

	return nil
}

func (d *DashboardServer) startServerViaAPI(serverName string) error {
	proxyURL := os.Getenv("MCP_PROXY_URL")
	if proxyURL == "" {
		proxyURL = "http://localhost:9876"
	}

	url := fmt.Sprintf("%s/api/servers/%s/start", proxyURL, serverName)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	apiKey := os.Getenv("MCP_API_KEY")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server start failed with status %d", resp.StatusCode)
	}

	return nil
}

func (d *DashboardServer) handleRegistryHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := map[string]interface{}{
		"registry": "unknown",
		"database": "unknown",
	}

	if d.registryService == nil {
		health["status"] = "unavailable"
		health["registry"] = "not configured"
		health["database"] = "not configured"
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
		return
	}

	if err := d.registryService.registry.HealthCheck(); err != nil {
		health["registry"] = "unhealthy"
		health["registryError"] = err.Error()
	} else {
		health["registry"] = "healthy"
	}

	if err := d.registryService.db.Ping(); err != nil {
		health["database"] = "unhealthy"
		health["databaseError"] = err.Error()
	} else {
		health["database"] = "healthy"
	}

	health["status"] = "healthy"
	if health["registry"] == "unhealthy" || health["database"] == "unhealthy" {
		health["status"] = "degraded"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(health); err != nil {
		d.logger.Error("Failed to encode response: %v", err)
	}
}
