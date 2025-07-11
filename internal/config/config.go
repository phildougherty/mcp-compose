// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/phildougherty/mcp-compose/internal/constants"

	yaml "gopkg.in/yaml.v3"
)

// ProxyAuthConfig defines authentication settings for the proxy itself
type ProxyAuthConfig struct {
	Enabled       bool   `yaml:"enabled,omitempty"`
	APIKey        string `yaml:"api_key,omitempty"`        // If you want to store the API key in the config file
	OAuthFallback bool   `yaml:"oauth_fallback,omitempty"` // Allow OAuth as fallback
}

// ComposeConfig represents the entire mcp-compose.yaml file
type ComposeConfig struct {
	Version       string                       `yaml:"version"`
	ProxyAuth     ProxyAuthConfig              `yaml:"proxy_auth,omitempty"`
	OAuth         *OAuthConfig                 `yaml:"oauth,omitempty"`
	Audit         *AuditConfig                 `yaml:"audit,omitempty"`
	RBAC          *RBACConfig                  `yaml:"rbac,omitempty"`
	Users         map[string]*User             `yaml:"users,omitempty"`
	OAuthClients  map[string]*OAuthClient      `yaml:"oauth_clients,omitempty"`
	Servers       map[string]ServerConfig      `yaml:"servers"`
	Connections   map[string]ConnectionConfig  `yaml:"connections,omitempty"`
	Logging       LoggingConfig                `yaml:"logging,omitempty"`
	Monitoring    MonitoringConfig             `yaml:"monitoring,omitempty"`
	Development   DevelopmentConfig            `yaml:"development,omitempty"`
	Environments  map[string]EnvironmentConfig `yaml:"environments,omitempty"`
	CurrentEnv    string                       `yaml:"-"`
	Dashboard     DashboardConfig              `yaml:"dashboard,omitempty"`
	Networks      map[string]NetworkConfig     `yaml:"networks,omitempty"`
	Volumes       map[string]VolumeConfig      `yaml:"volumes,omitempty"`
	TaskScheduler *TaskScheduler               `yaml:"task_scheduler,omitempty"`
	Memory        MemoryConfig                 `yaml:"memory"`
}

// OAuth 2.1 Configuration
type OAuthConfig struct {
	Enabled         bool                `yaml:"enabled"`
	Issuer          string              `yaml:"issuer"`
	Endpoints       OAuthEndpoints      `yaml:"endpoints"`
	Tokens          TokenConfig         `yaml:"tokens"`
	Security        OAuthSecurityConfig `yaml:"security"`
	GrantTypes      []string            `yaml:"grant_types"`
	ResponseTypes   []string            `yaml:"response_types"`
	ScopesSupported []string            `yaml:"scopes_supported"`
}

type OAuthEndpoints struct {
	Authorization string `yaml:"authorization"`
	Token         string `yaml:"token"`
	UserInfo      string `yaml:"userinfo"`
	Revoke        string `yaml:"revoke"`
	Discovery     string `yaml:"discovery"`
}

type TokenConfig struct {
	AccessTokenTTL  string `yaml:"access_token_ttl"`
	RefreshTokenTTL string `yaml:"refresh_token_ttl"`
	CodeTTL         string `yaml:"authorization_code_ttl"`
	Algorithm       string `yaml:"algorithm"`
}

type OAuthSecurityConfig struct {
	RequirePKCE bool `yaml:"require_pkce"`
}

// Audit Configuration
type AuditConfig struct {
	Enabled   bool            `yaml:"enabled"`
	LogLevel  string          `yaml:"log_level"`
	Storage   string          `yaml:"storage"`
	Retention RetentionConfig `yaml:"retention"`
	Events    []string        `yaml:"events"`
}

type RetentionConfig struct {
	MaxEntries int    `yaml:"max_entries"`
	MaxAge     string `yaml:"max_age"`
}

// RBAC Configuration
type RBACConfig struct {
	Enabled bool            `yaml:"enabled"`
	Scopes  []Scope         `yaml:"scopes"`
	Roles   map[string]Role `yaml:"roles"`
}

type Scope struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type Role struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Scopes      []string `yaml:"scopes"`
}

// User Management
type User struct {
	Username     string    `yaml:"username"`
	Email        string    `yaml:"email"`
	PasswordHash string    `yaml:"password_hash"`
	Role         string    `yaml:"role"`
	Enabled      bool      `yaml:"enabled"`
	CreatedAt    time.Time `yaml:"created_at"`
}

// OAuth Clients
type OAuthClient struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret *string  `yaml:"client_secret"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description"`
	RedirectURIs []string `yaml:"redirect_uris"`
	Scopes       []string `yaml:"scopes"`
	GrantTypes   []string `yaml:"grant_types"`
	PublicClient bool     `yaml:"public_client"`
	AutoApprove  bool     `yaml:"auto_approve"`
}

type OAuthClientConfig struct {
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret,omitempty"`
	Name         string   `yaml:"name"`
	Description  string   `yaml:"description,omitempty"`
	RedirectURIs []string `yaml:"redirect_uris"`
	Scopes       []string `yaml:"scopes"`
	GrantTypes   []string `yaml:"grant_types"`
	PublicClient bool     `yaml:"public_client,omitempty"`
	AutoApprove  bool     `yaml:"auto_approve,omitempty"`
}

type ServerConfig struct {
	// Process-based setup
	Command         string              `yaml:"command,omitempty"`
	Args            []string            `yaml:"args,omitempty"`
	Image           string              `yaml:"image,omitempty"`
	Build           BuildConfig         `yaml:"build,omitempty"`
	Runtime         string              `yaml:"runtime,omitempty"`
	Pull            bool                `yaml:"pull,omitempty"`
	WorkDir         string              `yaml:"workdir,omitempty"`
	Env             map[string]string   `yaml:"env,omitempty"`
	Ports           []string            `yaml:"ports,omitempty"`
	HttpPort        int                 `yaml:"http_port,omitempty"`
	HttpPath        string              `yaml:"http_path,omitempty"`
	Protocol        string              `yaml:"protocol,omitempty"` // "http", "sse", or "stdio" (default)
	StdioHosterPort int                 `yaml:"stdio_hoster_port,omitempty"`
	Capabilities    []string            `yaml:"capabilities,omitempty"`
	DependsOn       []string            `yaml:"depends_on,omitempty"`
	Volumes         []string            `yaml:"volumes,omitempty"`
	Resources       ResourcesConfig     `yaml:"resources,omitempty"`
	Tools           []ToolConfig        `yaml:"tools,omitempty"`
	Prompts         []PromptConfig      `yaml:"prompts,omitempty"`
	Sampling        SamplingConfig      `yaml:"sampling,omitempty"`
	Security        SecurityConfig      `yaml:"security,omitempty"`
	Lifecycle       LifecycleConfig     `yaml:"lifecycle,omitempty"`
	CapabilityOpt   CapabilityOptConfig `yaml:"capability_options,omitempty"`
	NetworkMode     string              `yaml:"network_mode,omitempty"`
	Networks        []string            `yaml:"networks,omitempty"`
	Authentication  *ServerAuthConfig   `yaml:"authentication,omitempty"`
	OAuth           *ServerOAuthConfig  `yaml:"oauth,omitempty"`
	SSEPath         string              `yaml:"sse_path,omitempty"`      // Path for SSE endpoint
	SSEPort         int                 `yaml:"sse_port,omitempty"`      // Port for SSE (if different from http_port)
	SSEHeartbeat    int                 `yaml:"sse_heartbeat,omitempty"` // SSE heartbeat interval in seconds

	// NEW: Docker-style container security and resource options
	Privileged    bool              `yaml:"privileged,omitempty"`
	User          string            `yaml:"user,omitempty"`
	Groups        []string          `yaml:"groups,omitempty"`
	ReadOnly      bool              `yaml:"read_only,omitempty"`
	Tmpfs         []string          `yaml:"tmpfs,omitempty"`
	CapAdd        []string          `yaml:"cap_add,omitempty"`
	CapDrop       []string          `yaml:"cap_drop,omitempty"`
	SecurityOpt   []string          `yaml:"security_opt,omitempty"`
	Deploy        DeployConfig      `yaml:"deploy,omitempty"`
	RestartPolicy string            `yaml:"restart,omitempty"`
	StopSignal    string            `yaml:"stop_signal,omitempty"`
	StopTimeout   *int              `yaml:"stop_grace_period,omitempty"`
	HealthCheck   *HealthCheck      `yaml:"healthcheck,omitempty"`
	Hostname      string            `yaml:"hostname,omitempty"`
	DomainName    string            `yaml:"domainname,omitempty"`
	DNS           []string          `yaml:"dns,omitempty"`
	DNSSearch     []string          `yaml:"dns_search,omitempty"`
	ExtraHosts    []string          `yaml:"extra_hosts,omitempty"`
	LogDriver     string            `yaml:"log_driver,omitempty"`
	LogOptions    map[string]string `yaml:"log_options,omitempty"`
	Labels        map[string]string `yaml:"labels,omitempty"`
	Annotations   map[string]string `yaml:"annotations,omitempty"`
	Platform      string            `yaml:"platform,omitempty"`
}

type ServerAuthConfig struct {
	Enabled       bool     `yaml:"enabled"`
	RequiredScope string   `yaml:"required_scope,omitempty"`
	OptionalAuth  bool     `yaml:"optional_auth,omitempty"`
	Scopes        []string `yaml:"scopes,omitempty"`
	AllowAPIKey   *bool    `yaml:"allow_api_key,omitempty"`
}

type ServerOAuthConfig struct {
	Enabled             bool     `yaml:"enabled"`
	RequiredScope       string   `yaml:"required_scope"`
	AllowAPIKeyFallback bool     `yaml:"allow_api_key_fallback"`
	OptionalAuth        bool     `yaml:"optional_auth"`
	AllowedClients      []string `yaml:"allowed_clients"`
}

type BuildConfig struct {
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"` // For --build-arg
	Target     string            `yaml:"target,omitempty"`
	NoCache    bool              `yaml:"no_cache,omitempty"`
	Pull       bool              `yaml:"pull,omitempty"`
	Platform   string            `yaml:"platform,omitempty"`
}

// NEW: Deploy configuration for resource management
type DeployConfig struct {
	Resources     ResourcesDeployConfig `yaml:"resources,omitempty"`
	RestartPolicy string                `yaml:"restart_policy,omitempty"`
	Replicas      int                   `yaml:"replicas,omitempty"`
	UpdateConfig  UpdateConfig          `yaml:"update_config,omitempty"`
}

type ResourcesDeployConfig struct {
	Limits       ResourceLimitsConfig `yaml:"limits,omitempty"`
	Reservations ResourceLimitsConfig `yaml:"reservations,omitempty"`
}

type ResourceLimitsConfig struct {
	CPUs        string `yaml:"cpus,omitempty"`
	Memory      string `yaml:"memory,omitempty"`
	MemorySwap  string `yaml:"memory_swap,omitempty"`
	PIDs        int    `yaml:"pids,omitempty"`
	BlkioWeight int    `yaml:"blkio_weight,omitempty"`
}

type UpdateConfig struct {
	Parallelism     int    `yaml:"parallelism,omitempty"`
	Delay           string `yaml:"delay,omitempty"`
	FailureAction   string `yaml:"failure_action,omitempty"`
	Monitor         string `yaml:"monitor,omitempty"`
	MaxFailureRatio string `yaml:"max_failure_ratio,omitempty"`
}

// NEW: Network configuration
type NetworkConfig struct {
	Driver      string            `yaml:"driver,omitempty"`
	DriverOpts  map[string]string `yaml:"driver_opts,omitempty"`
	Attachable  bool              `yaml:"attachable,omitempty"`
	Enable_ipv6 bool              `yaml:"enable_ipv6,omitempty"`
	IPAM        IPAMConfig        `yaml:"ipam,omitempty"`
	Internal    bool              `yaml:"internal,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	External    bool              `yaml:"external,omitempty"`
}

type IPAMConfig struct {
	Driver  string            `yaml:"driver,omitempty"`
	Config  []IPAMConfigEntry `yaml:"config,omitempty"`
	Options map[string]string `yaml:"options,omitempty"`
}

type IPAMConfigEntry struct {
	Subnet  string `yaml:"subnet,omitempty"`
	Gateway string `yaml:"gateway,omitempty"`
}

// NEW: Volume configuration
type VolumeConfig struct {
	Driver     string            `yaml:"driver,omitempty"`
	DriverOpts map[string]string `yaml:"driver_opts,omitempty"`
	External   bool              `yaml:"external,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty"`
}

// ConnectionConfig represents connection settings for MCP communication
type ConnectionConfig struct {
	Transport      string        `yaml:"transport"` // stdio, http+sse, tcp, websocket
	Port           int           `yaml:"port,omitempty"`
	Host           string        `yaml:"host,omitempty"`
	Path           string        `yaml:"path,omitempty"`
	Expose         bool          `yaml:"expose,omitempty"`
	TLS            bool          `yaml:"tls,omitempty"`
	CertFile       string        `yaml:"cert_file,omitempty"`
	KeyFile        string        `yaml:"key_file,omitempty"`
	Authentication string        `yaml:"auth,omitempty"` // none, basic, token
	Timeouts       TimeoutConfig `yaml:"timeouts,omitempty"`
}

// TimeoutConfig defines configurable timeout values
type TimeoutConfig struct {
	Connect       string `yaml:"connect,omitempty"`        // Default: "10s"
	Read          string `yaml:"read,omitempty"`           // Default: "30s"
	Write         string `yaml:"write,omitempty"`          // Default: "30s"
	Idle          string `yaml:"idle,omitempty"`           // Default: "60s"
	HealthCheck   string `yaml:"health_check,omitempty"`   // Default: "5s"
	Shutdown      string `yaml:"shutdown,omitempty"`       // Default: "30s"
	LifecycleHook string `yaml:"lifecycle_hook,omitempty"` // Default: "30s"
}

// ResourcesConfig defines resource-related configuration for a server
type ResourcesConfig struct {
	Paths        []ResourcePath `yaml:"paths,omitempty"`
	SyncInterval string         `yaml:"sync_interval,omitempty"`
	CacheTTL     int            `yaml:"cache_ttl,omitempty"`
	Watch        bool           `yaml:"watch,omitempty"`
}

// ResourcePath defines a resource path mapping
type ResourcePath struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	Watch    bool   `yaml:"watch,omitempty"`
	ReadOnly bool   `yaml:"read_only,omitempty"`
}

// ToolConfig defines a tool configuration
type ToolConfig struct {
	Name        string             `yaml:"name"`
	Description string             `yaml:"description,omitempty"`
	Parameters  []ToolParameter    `yaml:"parameters,omitempty"`
	Timeout     string             `yaml:"timeout,omitempty"`
	Mocks       []ToolMockResponse `yaml:"mocks,omitempty"`
}

// ToolParameter defines a parameter for a tool
type ToolParameter struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Default     interface{} `yaml:"default,omitempty"`
}

// ToolMockResponse defines a mock response for testing a tool
type ToolMockResponse struct {
	Input    map[string]interface{} `yaml:"input"`
	Response map[string]interface{} `yaml:"response"`
	Status   string                 `yaml:"status,omitempty"`
}

// PromptConfig defines a prompt template configuration
type PromptConfig struct {
	Name        string           `yaml:"name"`
	Template    string           `yaml:"template"`
	Description string           `yaml:"description,omitempty"`
	Variables   []PromptVariable `yaml:"variables,omitempty"`
}

// PromptVariable defines a variable used in a prompt template
type PromptVariable struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Required    bool        `yaml:"required,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Default     interface{} `yaml:"default,omitempty"`
}

// SamplingConfig defines sampling configuration for a server
type SamplingConfig struct {
	Models []ModelConfig `yaml:"models,omitempty"`
}

// ModelConfig defines a model configuration for sampling
type ModelConfig struct {
	Name        string  `yaml:"name"`
	Provider    string  `yaml:"provider,omitempty"`
	MaxTokens   int     `yaml:"max_tokens,omitempty"`
	Temperature float64 `yaml:"temperature,omitempty"`
	TopP        float64 `yaml:"top_p,omitempty"`
	TopK        int     `yaml:"top_k,omitempty"`
}

// SecurityConfig defines security settings for a server (UPDATED)
type SecurityConfig struct {
	Auth          AuthConfig          `yaml:"auth,omitempty"`
	AccessControl AccessControlConfig `yaml:"access_control,omitempty"`

	// NEW: Docker-style security capabilities
	AllowDockerSocket  bool              `yaml:"allow_docker_socket,omitempty"`
	AllowHostMounts    []string          `yaml:"allow_host_mounts,omitempty"`
	AllowPrivilegedOps bool              `yaml:"allow_privileged_ops,omitempty"`
	TrustedImage       bool              `yaml:"trusted_image,omitempty"`
	NoNewPrivileges    bool              `yaml:"no_new_privileges,omitempty"`
	AppArmor           string            `yaml:"apparmor,omitempty"`
	Seccomp            string            `yaml:"seccomp,omitempty"`
	SELinux            map[string]string `yaml:"selinux,omitempty"`
}

// AuthConfig defines authentication configuration
type AuthConfig struct {
	Type        string `yaml:"type"` // api_key, oauth, none
	Header      string `yaml:"header,omitempty"`
	TokenSource string `yaml:"token_source,omitempty"`
}

// AccessControlConfig defines access control rules
type AccessControlConfig struct {
	Resources []AccessRule `yaml:"resources,omitempty"`
	Tools     []AccessRule `yaml:"tools,omitempty"`
}

// AccessRule defines an access rule for resources or tools
type AccessRule struct {
	Path   string `yaml:"path,omitempty"`
	Name   string `yaml:"name,omitempty"`
	Access string `yaml:"access"` // read-only, read-write, deny
}

// LifecycleConfig defines server lifecycle hooks
type LifecycleConfig struct {
	PreStart     string              `yaml:"pre_start,omitempty"`
	PostStart    string              `yaml:"post_start,omitempty"`
	PreStop      string              `yaml:"pre_stop,omitempty"`
	PostStop     string              `yaml:"post_stop,omitempty"`
	HealthCheck  HealthCheck         `yaml:"health_check,omitempty"`
	HumanControl *HumanControlConfig `yaml:"human_control,omitempty"`
}

type HumanControlConfig struct {
	RequireApproval     bool     `yaml:"require_approval,omitempty"`
	AutoApprovePatterns []string `yaml:"auto_approve_patterns,omitempty"`
	BlockPatterns       []string `yaml:"block_patterns,omitempty"`
	MaxTokens           int      `yaml:"max_tokens,omitempty"`
	AllowedModels       []string `yaml:"allowed_models,omitempty"`
	TimeoutSeconds      int      `yaml:"timeout_seconds,omitempty"`
}

// HealthCheck defines health check configuration (UPDATED)
type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
	Endpoint    string   `yaml:"endpoint,omitempty"` // Legacy support
	Action      string   `yaml:"action,omitempty"`   // Action when health check fails
}

type MemoryConfig struct {
	Enabled          bool              `yaml:"enabled"`
	Port             int               `yaml:"port"`
	Host             string            `yaml:"host"`
	DatabaseURL      string            `yaml:"database_url"`
	PostgresEnabled  bool              `yaml:"postgres_enabled"`
	PostgresPort     int               `yaml:"postgres_port"`
	PostgresDB       string            `yaml:"postgres_db"`
	PostgresUser     string            `yaml:"postgres_user"`
	PostgresPassword string            `yaml:"postgres_password"`
	CPUs             string            `yaml:"cpus"`
	Memory           string            `yaml:"memory"`
	PostgresCPUs     string            `yaml:"postgres_cpus"`
	PostgresMemory   string            `yaml:"postgres_memory"`
	Volumes          []string          `yaml:"volumes"`
	Authentication   *ServerAuthConfig `yaml:"authentication"`
}

type TaskScheduler struct {
	Enabled          bool              `yaml:"enabled"`
	Port             int               `yaml:"port"`
	Host             string            `yaml:"host"`
	DatabasePath     string            `yaml:"database_path"`
	LogLevel         string            `yaml:"log_level"`
	OpenRouterAPIKey string            `yaml:"openrouter_api_key"`
	OpenRouterModel  string            `yaml:"openrouter_model"`
	OllamaURL        string            `yaml:"ollama_url"`
	OllamaModel      string            `yaml:"ollama_model"`
	MCPProxyURL      string            `yaml:"mcp_proxy_url"`
	MCPProxyAPIKey   string            `yaml:"mcp_proxy_api_key"`
	OpenWebUIEnabled bool              `yaml:"openwebui_enabled"`
	Workspace        string            `yaml:"workspace"`
	CPUs             string            `yaml:"cpus"`
	Memory           string            `yaml:"memory"`
	Volumes          []string          `yaml:"volumes"`
	Env              map[string]string `yaml:"env"`
}

// CapabilityOptConfig defines capability-specific options
type CapabilityOptConfig struct {
	Resources ResourcesCapOpt `yaml:"resources,omitempty"`
	Tools     ToolsCapOpt     `yaml:"tools,omitempty"`
	Prompts   PromptsCapOpt   `yaml:"prompts,omitempty"`
	Sampling  SamplingCapOpt  `yaml:"sampling,omitempty"`
	Logging   LoggingCapOpt   `yaml:"logging,omitempty"`
}

// ResourcesCapOpt defines options for resources capability
type ResourcesCapOpt struct {
	Enabled     bool `yaml:"enabled"`
	ListChanged bool `yaml:"list_changed,omitempty"`
	Subscribe   bool `yaml:"subscribe,omitempty"`
}

// ToolsCapOpt defines options for tools capability
type ToolsCapOpt struct {
	Enabled     bool `yaml:"enabled"`
	ListChanged bool `yaml:"list_changed,omitempty"`
}

// PromptsCapOpt defines options for prompts capability
type PromptsCapOpt struct {
	Enabled     bool `yaml:"enabled"`
	ListChanged bool `yaml:"list_changed,omitempty"`
}

// SamplingCapOpt defines options for sampling capability
type SamplingCapOpt struct {
	Enabled bool `yaml:"enabled"`
}

// LoggingCapOpt defines options for logging capability
type LoggingCapOpt struct {
	Enabled bool `yaml:"enabled"`
}

// LoggingConfig defines global logging configuration
type LoggingConfig struct {
	Level        string           `yaml:"level,omitempty"`
	Format       string           `yaml:"format,omitempty"`
	Destinations []LogDestination `yaml:"destinations,omitempty"`
}

// LogDestination defines a log destination
type LogDestination struct {
	Type string `yaml:"type"` // file, stdout
	Path string `yaml:"path,omitempty"`
}

// MonitoringConfig defines monitoring configuration
type MonitoringConfig struct {
	Metrics MetricsConfig `yaml:"metrics,omitempty"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	Port    int  `yaml:"port,omitempty"`
}

// DevelopmentConfig defines development and testing tools configuration
type DevelopmentConfig struct {
	Inspector InspectorConfig `yaml:"inspector,omitempty"`
	Testing   TestingConfig   `yaml:"testing,omitempty"`
}

// InspectorConfig defines MCP Inspector configuration
type InspectorConfig struct {
	Enabled bool `yaml:"enabled,omitempty"`
	Port    int  `yaml:"port,omitempty"`
}

// TestingConfig defines testing framework configuration
type TestingConfig struct {
	Scenarios []TestScenario `yaml:"scenarios,omitempty"`
}

// TestScenario defines a test scenario
type TestScenario struct {
	Name      string         `yaml:"name"`
	Tools     []ToolTest     `yaml:"tools,omitempty"`
	Resources []ResourceTest `yaml:"resources,omitempty"`
}

// ToolTest defines a tool test
type ToolTest struct {
	Name           string                 `yaml:"name"`
	Input          map[string]interface{} `yaml:"input"`
	ExpectedStatus string                 `yaml:"expected_status"`
}

// ResourceTest defines a resource test
type ResourceTest struct {
	Path           string `yaml:"path"`
	ExpectedStatus string `yaml:"expected_status"`
}

// EnvironmentConfig defines environment-specific configuration overrides
type EnvironmentConfig struct {
	Servers map[string]ServerOverrideConfig `yaml:"servers,omitempty"`
}

// ServerOverrideConfig defines environment-specific server overrides
type ServerOverrideConfig struct {
	Env       map[string]string `yaml:"env,omitempty"`
	Resources ResourcesConfig   `yaml:"resources,omitempty"`
}

// DashboardConfig defines configuration for the MCP-Compose Dashboard
type DashboardConfig struct {
	Enabled      bool                 `yaml:"enabled,omitempty"`
	Port         int                  `yaml:"port,omitempty"`
	Host         string               `yaml:"host,omitempty"`
	ProxyURL     string               `yaml:"proxy_url,omitempty"`
	PostgresURL  string               `yaml:"postgres_url,omitempty"`
	Theme        string               `yaml:"theme,omitempty"`
	LogStreaming bool                 `yaml:"log_streaming,omitempty"`
	ConfigEditor bool                 `yaml:"config_editor,omitempty"`
	Metrics      bool                 `yaml:"metrics,omitempty"`
	Security     *DashboardSecurity   `yaml:"security,omitempty"`
	AdminLogin   *DashboardAdminLogin `yaml:"admin_login,omitempty"`
}

type DashboardSecurity struct {
	Enabled          bool `yaml:"enabled"`
	OAuthConfig      bool `yaml:"oauth_config"`
	ClientManagement bool `yaml:"client_management"`
	UserManagement   bool `yaml:"user_management"`
	AuditLogs        bool `yaml:"audit_logs"`
}

type DashboardAdminLogin struct {
	Enabled        bool   `yaml:"enabled"`
	SessionTimeout string `yaml:"session_timeout"`
}

// loadDotEnv loads environment variables from .env file in the same directory as the config file
func loadDotEnv(configFilePath string) {
	// Get the directory of the config file
	configDir := filepath.Dir(configFilePath)
	envFilePath := filepath.Join(configDir, ".env")

	// Check if .env file exists
	if _, err := os.Stat(envFilePath); os.IsNotExist(err) {

		return // No .env file, that's okay
	}

	// Read .env file
	data, err := os.ReadFile(envFilePath)
	if err != nil {

		return // Could not read .env file, continue without it
	}

	// Parse .env file and set environment variables
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {

			continue
		}

		// Split on first = sign
		parts := strings.SplitN(line, "=", constants.EnvVarSplitParts)
		if len(parts) != constants.EnvVarSplitParts {

			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Only set if not already set in environment
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

// LoadConfig loads and parses the compose file with environment support
func LoadConfig(filePath string) (*ComposeConfig, error) {
	// Load .env file if it exists
	loadDotEnv(filePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {

		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}
	// Expand environment variables
	expandedData := os.ExpandEnv(string(data)) // Use os.ExpandEnv for ${VAR} and $VAR
	// Parse YAML
	var config ComposeConfig
	err = yaml.Unmarshal([]byte(expandedData), &config)
	if err != nil {

		return nil, fmt.Errorf("failed to parse config file '%s': %w", filePath, err)
	}
	// Get current environment from MCP_ENV environment variable
	envName := os.Getenv("MCP_ENV")
	if envName == "" {
		envName = "development" // Default environment
	}
	config.CurrentEnv = envName
	// Apply environment-specific overrides if they exist
	if envConfig, exists := config.Environments[envName]; exists {
		applyEnvironmentOverrides(&config, envConfig)
	}
	// Validate config
	if err := ValidateConfig(&config); err != nil {

		return nil, fmt.Errorf("invalid configuration in '%s': %w", filePath, err)
	}

	return &config, nil
}

// applyEnvironmentOverrides applies environment-specific overrides to the config
func applyEnvironmentOverrides(config *ComposeConfig, envConfig EnvironmentConfig) {
	// Apply server overrides
	for serverName, overrides := range envConfig.Servers {
		if server, exists := config.Servers[serverName]; exists {
			// Apply environment variables
			if len(overrides.Env) > 0 {
				if server.Env == nil {
					server.Env = make(map[string]string)
				}
				for k, v := range overrides.Env {
					server.Env[k] = v
				}
			}
			// Apply resource settings
			if len(overrides.Resources.Paths) > 0 {
				server.Resources.Paths = overrides.Resources.Paths
			}
			if overrides.Resources.SyncInterval != "" {
				server.Resources.SyncInterval = overrides.Resources.SyncInterval
			}
			if overrides.Resources.CacheTTL > 0 { // Should be CacheTTL not CacheTLL
				server.Resources.CacheTTL = overrides.Resources.CacheTTL
			}
			// Update the server in the config
			config.Servers[serverName] = server
		}
	}
}

// In internal/config/config.go, change the function signature to make it public:
func ValidateConfig(config *ComposeConfig) error {
	if config.Version != "1" {

		return fmt.Errorf("unsupported version: '%s', expected '1'", config.Version)
	}
	for name, server := range config.Servers {
		if err := validateServerConfig(name, server); err != nil {

			return err
		}
		// Validate dependencies
		for _, dep := range server.DependsOn {
			if _, exists := config.Servers[dep]; !exists {

				return fmt.Errorf("server '%s' depends on undefined server '%s'", name, dep)
			}
		}
		// Validate human control configuration
		if server.Lifecycle.HumanControl != nil {
			if err := validateHumanControlConfig(name, server.Lifecycle.HumanControl); err != nil {

				return err
			}
		}
		// Validate resource paths
		if err := validateResourcePaths(name, server.Resources); err != nil {

			return err
		}
		// Validate tools configuration
		if err := validateToolsConfig(name, server.Tools); err != nil {

			return err
		}
		// NEW: Validate security configuration
		if err := validateSecurityConfig(name, server.Security); err != nil {

			return err
		}
		// NEW: Validate resource limits
		if err := validateResourceLimits(name, server.Deploy.Resources); err != nil {

			return err
		}
	}
	// Validate global configuration
	if err := validateGlobalConfig(config); err != nil {

		return err
	}

	return nil
}

// GetTimeoutDuration returns a timeout duration with fallback to default
func (tc TimeoutConfig) GetConnectTimeout() time.Duration {
	if tc.Connect != "" {
		if d, err := time.ParseDuration(tc.Connect); err == nil {

			return d
		}
	}

	return constants.DefaultConnectTimeout
}

func (tc TimeoutConfig) GetReadTimeout() time.Duration {
	if tc.Read != "" {
		if d, err := time.ParseDuration(tc.Read); err == nil {

			return d
		}
	}

	return constants.DefaultReadTimeout
}

func (tc TimeoutConfig) GetWriteTimeout() time.Duration {
	if tc.Write != "" {
		if d, err := time.ParseDuration(tc.Write); err == nil {

			return d
		}
	}

	return constants.DefaultReadTimeout
}

func (tc TimeoutConfig) GetIdleTimeout() time.Duration {
	if tc.Idle != "" {
		if d, err := time.ParseDuration(tc.Idle); err == nil {

			return d
		}
	}

	return constants.DefaultProtoTimeout
}

func (tc TimeoutConfig) GetHealthCheckTimeout() time.Duration {
	if tc.HealthCheck != "" {
		if d, err := time.ParseDuration(tc.HealthCheck); err == nil {

			return d
		}
	}

	return constants.DefaultHealthTimeout
}

func (tc TimeoutConfig) GetShutdownTimeout() time.Duration {
	if tc.Shutdown != "" {
		if d, err := time.ParseDuration(tc.Shutdown); err == nil {

			return d
		}
	}

	return constants.DefaultReadTimeout
}

func (tc TimeoutConfig) GetLifecycleHookTimeout() time.Duration {
	if tc.LifecycleHook != "" {
		if d, err := time.ParseDuration(tc.LifecycleHook); err == nil {

			return d
		}
	}

	return constants.DefaultReadTimeout
}

func validateServerConfig(name string, server ServerConfig) error {
	// A server must specify either command, image, OR build context
	if server.Command == "" && server.Image == "" && server.Build.Context == "" {

		return fmt.Errorf("server '%s' must specify either command, image, or build context", name)
	}

	// If build context is provided, we don't need image or command (command can be in Dockerfile)
	if server.Build.Context != "" {
		// Build context is sufficient - command and image are optional
		// Command will be used to override Dockerfile CMD if provided
		// Image will be used as the tag name if provided
	} else if server.Image == "" && server.Command == "" {

		return fmt.Errorf("server '%s' must specify either command or image when not using build", name)
	}

	// Validate protocol
	if server.Protocol != "" {
		validProtocols := []string{"stdio", "http", "sse", "tcp"}
		valid := false
		for _, p := range validProtocols {
			if server.Protocol == p {
				valid = true

				break
			}
		}
		if !valid {

			return fmt.Errorf("server '%s' has invalid protocol: '%s'. Must be one of: %v", name, server.Protocol, validProtocols)
		}
	}

	// Validate HTTP/SSE configuration
	if (server.Protocol == "http" || server.Protocol == "sse") && server.HttpPort == 0 {
		if !hasPortInArgsOrMapping(server) {

			return fmt.Errorf("server '%s' uses '%s' protocol but 'http_port' is not defined and cannot be inferred", name, server.Protocol)
		}
	}

	// Validate capabilities
	validCaps := map[string]bool{
		"resources": true, "tools": true, "prompts": true,
		"sampling": true, "logging": true, "roots": true,
	}
	for _, cap := range server.Capabilities {
		if !validCaps[cap] {

			return fmt.Errorf("server '%s' has invalid capability: '%s'", name, cap)
		}
	}

	// Validate ports format
	for i, port := range server.Ports {
		if err := validatePortMapping(port); err != nil {

			return fmt.Errorf("server '%s' has invalid port mapping at index %d: %w", name, i, err)
		}
	}

	return nil
}

// Helper function to check if port can be inferred
func hasPortInArgsOrMapping(server ServerConfig) bool {
	// Check if port can be inferred from args
	for _, arg := range server.Args {
		if strings.HasPrefix(arg, "--port") || strings.HasPrefix(arg, "-p") {

			return true
		}
	}
	// Check if port mapping exists
	if len(server.Ports) > 0 {
		for _, p := range server.Ports {
			parts := strings.Split(p, ":")
			if len(parts) > 0 && parts[len(parts)-1] != "" {

				return true
			}
		}
	}

	return false
}

// Validate human control configuration
func validateHumanControlConfig(serverName string, hc *HumanControlConfig) error {
	if hc.TimeoutSeconds < 0 {

		return fmt.Errorf("server '%s' has invalid human control timeout: %d (must be >= 0)", serverName, hc.TimeoutSeconds)
	}
	if hc.MaxTokens < 0 {

		return fmt.Errorf("server '%s' has invalid human control max_tokens: %d (must be >= 0)", serverName, hc.MaxTokens)
	}
	if hc.TimeoutSeconds > 0 && hc.TimeoutSeconds < 5 {

		return fmt.Errorf("server '%s' human control timeout too low: %d seconds (minimum 5 seconds)", serverName, hc.TimeoutSeconds)
	}

	return nil
}

// Validate resource paths
func validateResourcePaths(serverName string, resources ResourcesConfig) error {
	for i, path := range resources.Paths {
		if path.Source == "" {

			return fmt.Errorf("server '%s' resource path %d missing source", serverName, i)
		}
		if path.Target == "" {

			return fmt.Errorf("server '%s' resource path %d missing target", serverName, i)
		}
		// Check if source path exists (warning, not error)
		if _, err := os.Stat(path.Source); os.IsNotExist(err) {
			// This could be a warning instead of an error
			continue
		}
	}
	// Validate sync interval if specified
	if resources.SyncInterval != "" {
		if _, err := time.ParseDuration(resources.SyncInterval); err != nil {

			return fmt.Errorf("server '%s' has invalid resource sync_interval '%s': %w", serverName, resources.SyncInterval, err)
		}
	}

	return nil
}

// Validate tools configuration
func validateToolsConfig(serverName string, tools []ToolConfig) error {
	toolNames := make(map[string]bool)
	for i, tool := range tools {
		if tool.Name == "" {

			return fmt.Errorf("server '%s' tool %d missing name", serverName, i)
		}
		if toolNames[tool.Name] {

			return fmt.Errorf("server '%s' has duplicate tool name: '%s'", serverName, tool.Name)
		}
		toolNames[tool.Name] = true
		// Validate timeout if specified
		if tool.Timeout != "" {
			if _, err := time.ParseDuration(tool.Timeout); err != nil {

				return fmt.Errorf("server '%s' tool '%s' has invalid timeout '%s': %w", serverName, tool.Name, tool.Timeout, err)
			}
		}
	}

	return nil
}

// NEW: Validate security configuration
func validateSecurityConfig(serverName string, security SecurityConfig) error {
	// Validate AppArmor profile
	if security.AppArmor != "" {
		validProfiles := []string{"unconfined", "default"}
		valid := false
		for _, profile := range validProfiles {
			if security.AppArmor == profile {
				valid = true

				break
			}
		}
		if !valid && !strings.HasPrefix(security.AppArmor, "/") {

			return fmt.Errorf("server '%s' has invalid apparmor profile: '%s'", serverName, security.AppArmor)
		}
	}

	// Validate seccomp profile
	if security.Seccomp != "" {
		validProfiles := []string{"unconfined", "default"}
		valid := false
		for _, profile := range validProfiles {
			if security.Seccomp == profile {
				valid = true

				break
			}
		}
		if !valid && !strings.HasPrefix(security.Seccomp, "/") {

			return fmt.Errorf("server '%s' has invalid seccomp profile: '%s'", serverName, security.Seccomp)
		}
	}

	return nil
}

// NEW: Validate resource limits
func validateResourceLimits(serverName string, resources ResourcesDeployConfig) error {
	// Validate CPU limits
	if resources.Limits.CPUs != "" {
		if _, err := strconv.ParseFloat(resources.Limits.CPUs, 64); err != nil {

			return fmt.Errorf("server '%s' has invalid CPU limit: '%s'", serverName, resources.Limits.CPUs)
		}
	}

	// Validate memory limits
	if resources.Limits.Memory != "" {
		if !isValidMemoryFormat(resources.Limits.Memory) {

			return fmt.Errorf("server '%s' has invalid memory limit format: '%s'", serverName, resources.Limits.Memory)
		}
	}

	if resources.Limits.MemorySwap != "" {
		if !isValidMemoryFormat(resources.Limits.MemorySwap) {

			return fmt.Errorf("server '%s' has invalid memory swap format: '%s'", serverName, resources.Limits.MemorySwap)
		}
	}

	// Validate PIDs limit
	if resources.Limits.PIDs < 0 {

		return fmt.Errorf("server '%s' has invalid PIDs limit: %d (must be >= 0)", serverName, resources.Limits.PIDs)
	}

	return nil
}

// Helper function to validate memory format (e.g., "512m", "1g", "2048k")
func isValidMemoryFormat(memory string) bool {
	if memory == "" {

		return true
	}

	validSuffixes := []string{"b", "k", "m", "g"}
	memory = strings.ToLower(memory)

	for _, suffix := range validSuffixes {
		if strings.HasSuffix(memory, suffix) {
			numPart := memory[:len(memory)-1]
			if _, err := strconv.ParseInt(numPart, 10, 64); err != nil {

				return false
			}

			return true
		}
	}

	// Check if it's just a number (bytes)
	if _, err := strconv.ParseInt(memory, 10, 64); err != nil {

		return false
	}

	return true
}

// Validate port mapping format
func validatePortMapping(portMapping string) error {
	parts := strings.Split(portMapping, ":")
	for _, part := range parts {
		if part == "" {

			return fmt.Errorf("empty port in mapping '%s'", portMapping)
		}
		// Check if it's a valid number
		if _, err := strconv.Atoi(part); err != nil {
			// Could be a port range like "8000-8010", validate differently
			if !strings.Contains(part, "-") {

				return fmt.Errorf("invalid port number '%s' in mapping '%s'", part, portMapping)
			}
		}
	}

	return nil
}

// Validate global configuration
func validateGlobalConfig(config *ComposeConfig) error {
	// Validate proxy auth
	if config.ProxyAuth.Enabled && config.ProxyAuth.APIKey == "" {

		return fmt.Errorf("proxy_auth is enabled but api_key is not specified")
	}
	// Validate dashboard config
	if config.Dashboard.Enabled {
		if config.Dashboard.Port <= 0 || config.Dashboard.Port > 65535 {

			return fmt.Errorf("dashboard port must be between 1 and 65535")
		}
		if config.Dashboard.ProxyURL == "" {

			return fmt.Errorf("dashboard is enabled but proxy_url is not specified")
		}
	}
	// Validate connections
	for name, conn := range config.Connections {
		if err := validateConnection(name, conn); err != nil {

			return err
		}
	}
	// Validate OAuth config if present
	if config.OAuth != nil && config.OAuth.Enabled {
		if err := validateOAuthConfig(config.OAuth); err != nil {

			return err
		}
	}

	return nil
}

// Validate OAuth configuration
func validateOAuthConfig(oauth *OAuthConfig) error {
	if oauth.Issuer == "" {

		return fmt.Errorf("oauth.issuer is required when OAuth is enabled")
	}
	// Validate token TTLs
	if oauth.Tokens.AccessTokenTTL != "" {
		if _, err := time.ParseDuration(oauth.Tokens.AccessTokenTTL); err != nil {

			return fmt.Errorf("invalid oauth.tokens.access_token_ttl: %w", err)
		}
	}
	if oauth.Tokens.RefreshTokenTTL != "" {
		if _, err := time.ParseDuration(oauth.Tokens.RefreshTokenTTL); err != nil {

			return fmt.Errorf("invalid oauth.tokens.refresh_token_ttl: %w", err)
		}
	}

	return nil
}

// Validate connection configuration
func validateConnection(name string, conn ConnectionConfig) error {
	validTransports := []string{"stdio", "http", "https", "tcp", "websocket", "http+sse"}
	valid := false
	for _, t := range validTransports {
		if conn.Transport == t {
			valid = true

			break
		}
	}
	if !valid {

		return fmt.Errorf("connection '%s' has invalid transport: '%s'", name, conn.Transport)
	}
	if conn.Port < 0 || conn.Port > 65535 {

		return fmt.Errorf("connection '%s' has invalid port: %d", name, conn.Port)
	}

	return nil
}

// GetProjectName returns the project name based on the directory containing the config file
func GetProjectName(filePath string) string {
	dir := filepath.Dir(filePath)
	if dir == "." {
		if cwd, err := os.Getwd(); err == nil {
			dir = cwd
		}
	}

	return filepath.Base(dir)
}

// IsCapabilityEnabled checks if a capability is enabled for a server
func IsCapabilityEnabled(server ServerConfig, capability string) bool {
	for _, cap := range server.Capabilities {
		if cap == capability {

			return true
		}
	}

	return false
}

// MergeEnv merges the provided env vars with the server's env vars
func MergeEnv(serverEnv, extraEnv map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range serverEnv { // Copy base
		result[k] = v
	}
	for k, v := range extraEnv { // Override or add
		result[k] = v
	}

	return result
}

// ConvertToEnvList converts an environment map to a list of KEY=VALUE strings
func ConvertToEnvList(env map[string]string) []string {
	var result []string
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}

	return result
}

// SaveConfig saves the configuration to a file
func SaveConfig(filePath string, config *ComposeConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {

		return fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(filePath, data, constants.DefaultFileMode); err != nil {

		return fmt.Errorf("failed to write config file '%s': %w", filePath, err)
	}

	return nil
}
