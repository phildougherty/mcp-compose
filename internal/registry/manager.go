package registry

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

type Manager struct {
	db      *sql.DB
	logger  *logging.Logger
	mu      sync.RWMutex
	cache   map[int]*Server
	lastSync time.Time
}

type Server struct {
	ID               int              `json:"id"`
	Name             string           `json:"name"`
	DisplayName      string           `json:"displayName"`
	Description      string           `json:"description"`
	DockerImage      *string          `json:"dockerImage"`
	NpmPackage       *string          `json:"npmPackage"`
	Category         string           `json:"category"`
	Tags             []string         `json:"tags"`
	Protocol         string           `json:"protocol"`
	Capabilities     []string         `json:"capabilities"`
	ConfigTemplate   json.RawMessage  `json:"configTemplate"`
	Featured         bool             `json:"featured"`
	Downloads        int              `json:"downloads"`
	Rating           float64          `json:"rating"`
	Author           *string          `json:"author"`
	RepositoryURL    *string          `json:"repositoryUrl"`
	DocumentationURL *string          `json:"documentationUrl"`
	IconURL          *string          `json:"iconUrl"`
	Version          string           `json:"version"`
	CreatedAt        time.Time        `json:"createdAt"`
	UpdatedAt        time.Time        `json:"updatedAt"`
}

type Category struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	SortOrder   int    `json:"sortOrder"`
}

type InstallRequest struct {
	ServerID int                    `json:"serverId"`
	UserID   string                 `json:"userId"`
	Config   map[string]interface{} `json:"config"`
}

type InstalledServer struct {
	ID          int                    `json:"id"`
	ServerID    int                    `json:"serverId"`
	UserID      string                 `json:"userId"`
	InstalledAt time.Time              `json:"installedAt"`
	Config      map[string]interface{} `json:"config"`
	Status      string                 `json:"status"`
}

func NewManager(db *sql.DB, logger *logging.Logger) (*Manager, error) {
	m := &Manager{
		db:     db,
		logger: logger,
		cache:  make(map[int]*Server),
	}

	if err := m.ensureSchema(); err != nil {
		logger.Warning("Failed to ensure schema, registry may not function properly: %v", err)
	}

	if err := m.initializeCache(); err != nil {
		logger.Warning("Failed to initialize cache, continuing with empty cache: %v", err)
	}

	return m, nil
}

func (m *Manager) ensureSchema() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS marketplace_servers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			display_name VARCHAR(255) NOT NULL,
			description TEXT NOT NULL,
			docker_image VARCHAR(500),
			npm_package VARCHAR(255),
			category VARCHAR(100) NOT NULL,
			tags TEXT[] DEFAULT '{}',
			protocol VARCHAR(50) DEFAULT 'stdio',
			capabilities TEXT[] DEFAULT '{}',
			config_template JSONB NOT NULL,
			featured BOOLEAN DEFAULT false,
			downloads INTEGER DEFAULT 0,
			rating DECIMAL(3,2) DEFAULT 0.0,
			author VARCHAR(255),
			repository_url VARCHAR(500),
			documentation_url VARCHAR(500),
			icon_url VARCHAR(500),
			version VARCHAR(50) DEFAULT '1.0.0',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS marketplace_categories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL UNIQUE,
			display_name VARCHAR(255) NOT NULL,
			description TEXT,
			icon VARCHAR(100),
			sort_order INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS user_installed_servers (
			id SERIAL PRIMARY KEY,
			server_id INTEGER REFERENCES marketplace_servers(id) ON DELETE CASCADE,
			user_id VARCHAR(255) DEFAULT 'default',
			server_name VARCHAR(255) NOT NULL,
			installed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			config JSONB,
			status VARCHAR(50) DEFAULT 'active',
			UNIQUE(user_id, server_name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_marketplace_servers_category ON marketplace_servers(category)`,
		`CREATE INDEX IF NOT EXISTS idx_marketplace_servers_featured ON marketplace_servers(featured)`,
		`CREATE INDEX IF NOT EXISTS idx_user_installed_servers_user ON user_installed_servers(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_user_installed_servers_status ON user_installed_servers(status)`,
		`INSERT INTO marketplace_categories (name, display_name, description, icon, sort_order) VALUES
			('filesystem', 'File System', 'File and directory operations', 'folder', 1),
			('database', 'Databases', 'Database connectivity and tools', 'database', 2),
			('search', 'Search & Web', 'Web search and data fetching', 'search', 3),
			('productivity', 'Productivity', 'Note-taking and task management', 'clipboard', 4),
			('development', 'Development', 'Developer tools and utilities', 'code', 5),
			('ai', 'AI & ML', 'AI and machine learning tools', 'brain', 6),
			('communication', 'Communication', 'Messaging and collaboration', 'message-square', 7),
			('storage', 'Storage & Memory', 'Persistent storage and memory', 'hard-drive', 8)
		ON CONFLICT (name) DO NOTHING`,
	}

	for i, query := range queries {
		if _, err := m.db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to execute schema query %d: %w", i+1, err)
		}
	}

	m.logger.Info("Registry schema ensured successfully")

	return nil
}

func (m *Manager) initializeCache() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	servers, err := m.ListServers(ctx, nil)
	if err != nil {
		return err
	}

	m.mu.Lock()
	for i := range servers {
		m.cache[servers[i].ID] = &servers[i]
	}
	m.lastSync = time.Now()
	m.mu.Unlock()

	m.logger.Info("Initialized registry cache with %d servers", len(servers))

	return nil
}

func (m *Manager) ListServers(ctx context.Context, filter *ServerFilter) ([]Server, error) {
	query := `
		SELECT id, name, display_name, description, docker_image, npm_package,
		       category, tags, protocol, capabilities, config_template, featured,
		       downloads, rating, author, repository_url, documentation_url,
		       icon_url, version, created_at, updated_at
		FROM marketplace_servers
		WHERE 1=1
	`
	args := []interface{}{}
	argPos := 1

	if filter != nil {
		if filter.Category != "" {
			query += fmt.Sprintf(" AND category = $%d", argPos)
			args = append(args, filter.Category)
			argPos++
		}
		if filter.Featured != nil {
			query += fmt.Sprintf(" AND featured = $%d", argPos)
			args = append(args, *filter.Featured)
			argPos++
		}
		if filter.SearchQuery != "" {
			query += fmt.Sprintf(" AND (name ILIKE $%d OR display_name ILIKE $%d OR description ILIKE $%d)", argPos, argPos, argPos)
			args = append(args, "%"+filter.SearchQuery+"%")
			argPos++
		}
	}

	query += " ORDER BY featured DESC, downloads DESC, rating DESC"

	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		if isTableMissingError(err) {
			m.logger.Warning("marketplace_servers table does not exist, returning empty list")
			return []Server{}, nil
		}
		return nil, fmt.Errorf("failed to query servers: %w", err)
	}
	defer rows.Close()

	var servers []Server
	for rows.Next() {
		var s Server
		var tags, capabilities []byte

		err := rows.Scan(
			&s.ID, &s.Name, &s.DisplayName, &s.Description, &s.DockerImage, &s.NpmPackage,
			&s.Category, &tags, &s.Protocol, &capabilities, &s.ConfigTemplate, &s.Featured,
			&s.Downloads, &s.Rating, &s.Author, &s.RepositoryURL, &s.DocumentationURL,
			&s.IconURL, &s.Version, &s.CreatedAt, &s.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan server: %w", err)
		}

		if err := parsePostgresArray(tags, &s.Tags); err != nil {
			s.Tags = []string{}
		}
		if err := parsePostgresArray(capabilities, &s.Capabilities); err != nil {
			s.Capabilities = []string{}
		}

		servers = append(servers, s)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating servers: %w", err)
	}

	return servers, nil
}

func (m *Manager) GetServer(ctx context.Context, id int) (*Server, error) {
	m.mu.RLock()
	if cached, ok := m.cache[id]; ok {
		m.mu.RUnlock()
		return cached, nil
	}
	m.mu.RUnlock()

	query := `
		SELECT id, name, display_name, description, docker_image, npm_package,
		       category, tags, protocol, capabilities, config_template, featured,
		       downloads, rating, author, repository_url, documentation_url,
		       icon_url, version, created_at, updated_at
		FROM marketplace_servers
		WHERE id = $1
	`

	var s Server
	var tags, capabilities []byte

	err := m.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.Name, &s.DisplayName, &s.Description, &s.DockerImage, &s.NpmPackage,
		&s.Category, &tags, &s.Protocol, &capabilities, &s.ConfigTemplate, &s.Featured,
		&s.Downloads, &s.Rating, &s.Author, &s.RepositoryURL, &s.DocumentationURL,
		&s.IconURL, &s.Version, &s.CreatedAt, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("server not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query server: %w", err)
	}

	if err := parsePostgresArray(tags, &s.Tags); err != nil {
		s.Tags = []string{}
	}
	if err := parsePostgresArray(capabilities, &s.Capabilities); err != nil {
		s.Capabilities = []string{}
	}

	m.mu.Lock()
	m.cache[id] = &s
	m.mu.Unlock()

	return &s, nil
}

func (m *Manager) GetCategories(ctx context.Context) ([]Category, error) {
	query := `
		SELECT id, name, display_name, description, icon, sort_order
		FROM marketplace_categories
		ORDER BY sort_order ASC
	`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		if isTableMissingError(err) {
			m.logger.Warning("marketplace_categories table does not exist, returning empty list")
			return []Category{}, nil
		}
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category

		err := rows.Scan(&c.ID, &c.Name, &c.DisplayName, &c.Description, &c.Icon, &c.SortOrder)
		if err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating categories: %w", err)
	}

	return categories, nil
}

func (m *Manager) InstallServer(ctx context.Context, req *InstallRequest) error {
	if req.UserID == "" {
		req.UserID = "default"
	}

	server, err := m.GetServer(ctx, req.ServerID)
	if err != nil {
		return fmt.Errorf("failed to get server: %w", err)
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	query := `
		INSERT INTO user_installed_servers (server_id, user_id, server_name, config, status)
		VALUES ($1, $2, $3, $4, 'active')
		ON CONFLICT (user_id, server_name) DO UPDATE
		SET config = EXCLUDED.config, status = 'active', server_id = EXCLUDED.server_id
	`

	_, err = m.db.ExecContext(ctx, query, req.ServerID, req.UserID, server.Name, configJSON)
	if err != nil {
		return fmt.Errorf("failed to install server: %w", err)
	}

	updateQuery := `UPDATE marketplace_servers SET downloads = downloads + 1 WHERE id = $1`
	_, err = m.db.ExecContext(ctx, updateQuery, req.ServerID)
	if err != nil {
		m.logger.Warning("Failed to update download count: %v", err)
	}

	m.logger.Info("Server %d installed for user %s", req.ServerID, req.UserID)

	return nil
}

func (m *Manager) UninstallServer(ctx context.Context, serverID int, userID string) error {
	if userID == "" {
		userID = "default"
	}

	query := `DELETE FROM user_installed_servers WHERE server_id = $1 AND user_id = $2`
	result, err := m.db.ExecContext(ctx, query, serverID, userID)
	if err != nil {
		return fmt.Errorf("failed to uninstall server: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("server not installed")
	}

	m.logger.Info("Server %d uninstalled for user %s", serverID, userID)

	return nil
}

func (m *Manager) GetInstalledServers(ctx context.Context, userID string) ([]InstalledServer, error) {
	if userID == "" {
		userID = "default"
	}

	query := `
		SELECT id, server_id, user_id, installed_at, config, status
		FROM user_installed_servers
		WHERE user_id = $1 AND status = 'active'
		ORDER BY installed_at DESC
	`

	rows, err := m.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query installed servers: %w", err)
	}
	defer rows.Close()

	var installed []InstalledServer
	for rows.Next() {
		var is InstalledServer
		var configJSON []byte

		err := rows.Scan(&is.ID, &is.ServerID, &is.UserID, &is.InstalledAt, &configJSON, &is.Status)
		if err != nil {
			return nil, fmt.Errorf("failed to scan installed server: %w", err)
		}

		if err := json.Unmarshal(configJSON, &is.Config); err != nil {
			is.Config = make(map[string]interface{})
		}

		installed = append(installed, is)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating installed servers: %w", err)
	}

	return installed, nil
}

func (m *Manager) SyncConfigToDatabase(ctx context.Context, installedServerNames []string, userID string) error {
	if userID == "" {
		userID = "default"
	}

	filter := &ServerFilter{}
	allServers, err := m.ListServers(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to list servers: %w", err)
	}

	serverByName := make(map[string]*Server)
	for i := range allServers {
		serverByName[allServers[i].Name] = &allServers[i]
	}

	for _, serverName := range installedServerNames {
		server, exists := serverByName[serverName]
		if !exists {
			m.logger.Warning("Server '%s' in config but not in registry, skipping", serverName)

			continue
		}

		req := &InstallRequest{
			ServerID: server.ID,
			UserID:   userID,
			Config:   make(map[string]interface{}),
		}

		if err := m.InstallServer(ctx, req); err != nil {
			m.logger.Warning("Failed to sync server '%s' to database: %v", serverName, err)

			continue
		}

		m.logger.Info("Synced existing server '%s' to database", serverName)
	}

	return nil
}

type ServerFilter struct {
	Category    string
	Featured    *bool
	SearchQuery string
}

func parsePostgresArray(data []byte, dest *[]string) error {
	if len(data) == 0 || string(data) == "{}" {
		*dest = []string{}
		return nil
	}

	var arr []string
	if err := json.Unmarshal(data, &arr); err != nil {
		strData := string(data)
		if len(strData) > 2 && strData[0] == '{' && strData[len(strData)-1] == '}' {
			strData = strData[1 : len(strData)-1]
			if strData != "" {
				arr = []string{}
				for _, item := range splitPostgresArray(strData) {
					arr = append(arr, item)
				}
			}
		}
	}
	*dest = arr

	return nil
}

func splitPostgresArray(s string) []string {
	var result []string
	var current string
	inQuote := false

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, current)
				current = ""
			} else {
				current += string(ch)
			}
		default:
			current += string(ch)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

func isTableMissingError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "relation") && strings.Contains(errStr, "does not exist")
}

func (m *Manager) EnsureTablesExist() error {
	return m.ensureSchema()
}
