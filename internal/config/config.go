// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings" // Import the strings package

	"gopkg.in/yaml.v3"
)

// ProxyAuthConfig defines authentication settings for the proxy itself
type ProxyAuthConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	APIKey  string `yaml:"api_key,omitempty"` // If you want to store the API key in the config file
}

// ComposeConfig represents the entire mcp-compose.yaml file
type ComposeConfig struct {
	Version      string                       `yaml:"version"`
	ProxyAuth    ProxyAuthConfig              `yaml:"proxy_auth,omitempty"` // <--- ADD THIS LINE
	Servers      map[string]ServerConfig      `yaml:"servers"`
	Connections  map[string]ConnectionConfig  `yaml:"connections,omitempty"`
	Logging      LoggingConfig                `yaml:"logging,omitempty"`
	Monitoring   MonitoringConfig             `yaml:"monitoring,omitempty"`
	Development  DevelopmentConfig            `yaml:"development,omitempty"`
	Environments map[string]EnvironmentConfig `yaml:"environments,omitempty"`
	CurrentEnv   string                       `yaml:"-"` // Runtime environment name
}

type ServerConfig struct {
	// Process-based setup
	Command string   `yaml:"command,omitempty"`
	Args    []string `yaml:"args,omitempty"`

	// Container-based setup
	Image   string      `yaml:"image,omitempty"`
	Build   BuildConfig `yaml:"build,omitempty"`
	Runtime string      `yaml:"runtime,omitempty"`
	Pull    bool        `yaml:"pull,omitempty"`

	// Common settings
	WorkDir         string            `yaml:"workdir,omitempty"`
	Env             map[string]string `yaml:"env,omitempty"`
	Ports           []string          `yaml:"ports,omitempty"`
	HttpPort        int               `yaml:"http_port,omitempty"`
	HttpPath        string            `yaml:"http_path,omitempty"`
	Protocol        string            `yaml:"protocol,omitempty"` // "http", "sse", or "stdio" (default)
	StdioHosterPort int               `yaml:"stdio_hoster_port,omitempty"`
	Capabilities    []string          `yaml:"capabilities,omitempty"`
	DependsOn       []string          `yaml:"depends_on,omitempty"`

	// Enhanced settings
	Volumes       []string            `yaml:"volumes,omitempty"`
	Resources     ResourcesConfig     `yaml:"resources,omitempty"`
	Tools         []ToolConfig        `yaml:"tools,omitempty"`
	Prompts       []PromptConfig      `yaml:"prompts,omitempty"`
	Sampling      SamplingConfig      `yaml:"sampling,omitempty"`
	Security      SecurityConfig      `yaml:"security,omitempty"`
	Lifecycle     LifecycleConfig     `yaml:"lifecycle,omitempty"`
	CapabilityOpt CapabilityOptConfig `yaml:"capability_options,omitempty"`
	NetworkMode   string              `yaml:"network_mode,omitempty"`
	Networks      []string            `yaml:"networks,omitempty"`

	// Transport-specific settings
	SSEPath      string `yaml:"sse_path,omitempty"`      // Path for SSE endpoint
	SSEPort      int    `yaml:"sse_port,omitempty"`      // Port for SSE (if different from http_port)
	SSEHeartbeat int    `yaml:"sse_heartbeat,omitempty"` // SSE heartbeat interval in seconds
}

type BuildConfig struct {
	Context    string            `yaml:"context,omitempty"`
	Dockerfile string            `yaml:"dockerfile,omitempty"`
	Args       map[string]string `yaml:"args,omitempty"` // For --build-arg
}

// ConnectionConfig represents connection settings for MCP communication
type ConnectionConfig struct {
	Transport      string `yaml:"transport"` // stdio, http+sse, tcp, websocket
	Port           int    `yaml:"port,omitempty"`
	Host           string `yaml:"host,omitempty"`
	Path           string `yaml:"path,omitempty"`
	Expose         bool   `yaml:"expose,omitempty"`
	TLS            bool   `yaml:"tls,omitempty"`
	CertFile       string `yaml:"cert_file,omitempty"`
	KeyFile        string `yaml:"key_file,omitempty"`
	Authentication string `yaml:"auth,omitempty"` // none, basic, token
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

// SecurityConfig defines security settings for a server
type SecurityConfig struct {
	Auth          AuthConfig          `yaml:"auth,omitempty"`
	AccessControl AccessControlConfig `yaml:"access_control,omitempty"`
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
	PreStart    string      `yaml:"pre_start,omitempty"`
	PostStart   string      `yaml:"post_start,omitempty"`
	PreStop     string      `yaml:"pre_stop,omitempty"`
	PostStop    string      `yaml:"post_stop,omitempty"`
	HealthCheck HealthCheck `yaml:"health_check,omitempty"`
}

// HealthCheck defines health check configuration
type HealthCheck struct {
	Endpoint string `yaml:"endpoint,omitempty"`
	Interval string `yaml:"interval,omitempty"`
	Timeout  string `yaml:"timeout,omitempty"`
	Retries  int    `yaml:"retries,omitempty"`
	Action   string `yaml:"action,omitempty"` // Action to take when health check fails (e.g., "restart")
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

// LoadConfig loads and parses the compose file with environment support
func LoadConfig(filePath string) (*ComposeConfig, error) {
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
	if err := validateConfig(&config); err != nil {
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

// validateConfig performs basic validation on the loaded configuration
func validateConfig(config *ComposeConfig) error {
	if config.Version != "1" {
		return fmt.Errorf("unsupported version: '%s', expected '1'", config.Version)
	}

	for name, server := range config.Servers {
		if server.Command == "" && server.Image == "" {
			return fmt.Errorf("server '%s' must specify either command or image", name)
		}

		// Validate protocol
		if server.Protocol != "" && server.Protocol != "stdio" && server.Protocol != "http" && server.Protocol != "sse" {
			return fmt.Errorf("server '%s' has invalid protocol: '%s'. Must be 'stdio', 'http', or 'sse'", name, server.Protocol)
		}

		// Validate HTTP/SSE configuration
		if (server.Protocol == "http" || server.Protocol == "sse") && server.HttpPort == 0 {
			// Check if port can be inferred from args or ports
			hasPortInArgs := false
			for _, arg := range server.Args {
				if strings.HasPrefix(arg, "--port") {
					hasPortInArgs = true
					break
				}
			}

			hasPortMapping := false
			if len(server.Ports) > 0 {
				for _, p := range server.Ports {
					parts := strings.Split(p, ":")
					if len(parts) > 0 && parts[len(parts)-1] != "" {
						hasPortMapping = true
						break
					}
				}
			}

			if !hasPortInArgs && !hasPortMapping {
				return fmt.Errorf("server '%s' uses '%s' protocol but 'http_port' is not defined and cannot be inferred", name, server.Protocol)
			}
		}

		// Validate capabilities
		validCaps := map[string]bool{"resources": true, "tools": true, "prompts": true, "sampling": true, "logging": true}
		for _, cap := range server.Capabilities {
			if !validCaps[cap] {
				return fmt.Errorf("server '%s' has invalid capability: '%s'", name, cap)
			}
		}

		// Validate dependencies
		for _, dep := range server.DependsOn {
			if _, exists := config.Servers[dep]; !exists {
				return fmt.Errorf("server '%s' depends on undefined server '%s'", name, dep)
			}
		}
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
	// Check specific capability options (this part might be more complex depending on your full config structure)
	// switch capability {
	// case "resources":
	// 	return server.CapabilityOpt.Resources.Enabled
	// // ... other capabilities
	// }
	return false // Default if not explicitly listed or in detailed options
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
