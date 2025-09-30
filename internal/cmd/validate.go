package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/fatih/color"
	"github.com/phildougherty/mcp-compose/internal/compose"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type ValidationResult struct {
	Category string
	Item     string
	Status   string
	Message  string
	Fix      string
}

func NewValidateCommand() *cobra.Command {
	var verbose bool
	var checkImages bool
	var checkEnv bool

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate the compose configuration file",
		Long: `Validate the compose configuration file for syntax errors, missing dependencies,
Docker image availability, environment variables, port conflicts, and security issues.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")

			return runValidation(file, verbose, checkImages, checkEnv)
		},
	}

	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Show detailed validation results")
	cmd.Flags().BoolVar(&checkImages, "check-images", true, "Check Docker image availability")
	cmd.Flags().BoolVar(&checkEnv, "check-env", true, "Validate environment variables")

	return cmd
}

func runValidation(configFile string, verbose, checkImages, checkEnv bool) error {
	results := []ValidationResult{}

	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	fmt.Println("Validating MCP-Compose configuration...\n")

	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		results = append(results, ValidationResult{
			Category: "Syntax",
			Item:     "Config File",
			Status:   "FAIL",
			Message:  err.Error(),
			Fix:      "Check YAML syntax and file structure",
		})
		printResults(results, verbose)

		return fmt.Errorf("configuration validation failed")
	}

	results = append(results, ValidationResult{
		Category: "Syntax",
		Item:     "Config File",
		Status:   "PASS",
		Message:  "Valid YAML syntax",
	})

	if err := compose.Validate(configFile); err != nil {
		results = append(results, ValidationResult{
			Category: "Schema",
			Item:     "Configuration",
			Status:   "FAIL",
			Message:  err.Error(),
			Fix:      "Review configuration against schema",
		})
	} else {
		results = append(results, ValidationResult{
			Category: "Schema",
			Item:     "Configuration",
			Status:   "PASS",
			Message:  "Passes schema validation",
		})
	}

	for serverName, server := range cfg.Servers {
		if err := validateServer(serverName, server, &results, checkImages); err != nil {
			results = append(results, ValidationResult{
				Category: "Server",
				Item:     serverName,
				Status:   "FAIL",
				Message:  err.Error(),
			})
		}
	}

	if checkEnv {
		validateEnvironmentVariables(cfg, &results)
	}

	validateOAuthConfiguration(cfg, &results)

	validatePortConflicts(cfg, &results)

	validateSecurity(cfg, &results)

	if checkImages {
		validateDockerImages(cfg, &results)
	}

	printResults(results, verbose)

	failCount := 0
	warnCount := 0
	for _, r := range results {
		if r.Status == "FAIL" {
			failCount++
		} else if r.Status == "WARN" {
			warnCount++
		}
	}

	fmt.Println()
	if failCount > 0 {
		red.Printf("FAILED: Validation failed: %d errors, %d warnings\n", failCount, warnCount)

		return fmt.Errorf("configuration validation failed")
	} else if warnCount > 0 {
		yellow.Printf("WARNING: Validation passed with warnings: %d warnings\n", warnCount)
	} else {
		green.Println("Validation passed successfully")
	}

	return nil
}

func validateServer(name string, server config.ServerConfig, results *[]ValidationResult, checkImages bool) error {
	if server.Command == "" && server.Image == "" && server.Build.Context == "" {
		*results = append(*results, ValidationResult{
			Category: "Server",
			Item:     name,
			Status:   "FAIL",
			Message:  "Must specify command, image, or build context",
			Fix:      "Add one of: command, image, or build.context",
		})

		return fmt.Errorf("missing required field")
	}

	if (server.Protocol == "http" || server.Protocol == "sse") && server.HttpPort == 0 {
		if !hasPortMapping(server) {
			*results = append(*results, ValidationResult{
				Category: "Server",
				Item:     name,
				Status:   "FAIL",
				Message:  fmt.Sprintf("Protocol '%s' requires http_port", server.Protocol),
				Fix:      "Set http_port field or add port mapping",
			})
		}
	}

	for _, dep := range server.DependsOn {
		*results = append(*results, ValidationResult{
			Category: "Dependencies",
			Item:     fmt.Sprintf("%s -> %s", name, dep),
			Status:   "INFO",
			Message:  "Dependency declared",
		})
	}

	if server.Build.Context != "" {
		if _, err := os.Stat(server.Build.Context); os.IsNotExist(err) {
			*results = append(*results, ValidationResult{
				Category: "Build",
				Item:     name,
				Status:   "FAIL",
				Message:  fmt.Sprintf("Build context not found: %s", server.Build.Context),
				Fix:      "Create the directory or update the path",
			})
		} else {
			dockerfilePath := server.Build.Dockerfile
			if dockerfilePath == "" {
				dockerfilePath = "Dockerfile"
			}
			fullPath := fmt.Sprintf("%s/%s", server.Build.Context, dockerfilePath)
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				*results = append(*results, ValidationResult{
					Category: "Build",
					Item:     name,
					Status:   "WARN",
					Message:  fmt.Sprintf("Dockerfile not found: %s", fullPath),
					Fix:      "Create Dockerfile or specify correct path",
				})
			} else {
				*results = append(*results, ValidationResult{
					Category: "Build",
					Item:     name,
					Status:   "PASS",
					Message:  "Build context and Dockerfile found",
				})
			}
		}
	}

	*results = append(*results, ValidationResult{
		Category: "Server",
		Item:     name,
		Status:   "PASS",
		Message:  "Server configuration valid",
	})

	return nil
}

func hasPortMapping(server config.ServerConfig) bool {
	if len(server.Ports) > 0 {
		return true
	}
	for _, arg := range server.Args {
		if strings.Contains(arg, "--port") || strings.Contains(arg, "-p") {
			return true
		}
	}

	return false
}

func validateEnvironmentVariables(cfg *config.ComposeConfig, results *[]ValidationResult) {
	requiredVars := make(map[string]bool)

	if cfg.ProxyAuth.Enabled && strings.Contains(cfg.ProxyAuth.APIKey, "${") {
		varName := extractEnvVar(cfg.ProxyAuth.APIKey)
		requiredVars[varName] = true
	}

	for serverName, server := range cfg.Servers {
		for key, value := range server.Env {
			if strings.Contains(value, "${") {
				varName := extractEnvVar(value)
				requiredVars[varName] = true

				if os.Getenv(varName) == "" {
					*results = append(*results, ValidationResult{
						Category: "Environment",
						Item:     fmt.Sprintf("%s.%s", serverName, key),
						Status:   "WARN",
						Message:  fmt.Sprintf("Environment variable %s not set", varName),
						Fix:      fmt.Sprintf("Set %s in environment or .env file", varName),
					})
				} else {
					*results = append(*results, ValidationResult{
						Category: "Environment",
						Item:     fmt.Sprintf("%s.%s", serverName, key),
						Status:   "PASS",
						Message:  fmt.Sprintf("%s is set", varName),
					})
				}
			}
		}
	}

	if len(requiredVars) == 0 {
		*results = append(*results, ValidationResult{
			Category: "Environment",
			Item:     "Variables",
			Status:   "INFO",
			Message:  "No environment variables required",
		})
	}
}

func extractEnvVar(value string) string {
	start := strings.Index(value, "${")
	if start == -1 {
		return ""
	}
	end := strings.Index(value[start:], "}")
	if end == -1 {
		return ""
	}
	varWithDefault := value[start+2 : start+end]

	parts := strings.Split(varWithDefault, ":-")

	return parts[0]
}

func validateOAuthConfiguration(cfg *config.ComposeConfig, results *[]ValidationResult) {
	if cfg.OAuth == nil || !cfg.OAuth.Enabled {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "Configuration",
			Status:   "INFO",
			Message:  "OAuth not enabled",
		})

		return
	}

	if cfg.OAuth.Issuer == "" {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "Issuer",
			Status:   "FAIL",
			Message:  "OAuth issuer URL is required",
			Fix:      "Set oauth.issuer field",
		})
	} else {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "Issuer",
			Status:   "PASS",
			Message:  fmt.Sprintf("Issuer configured: %s", cfg.OAuth.Issuer),
		})
	}

	if !cfg.OAuth.Security.RequirePKCE {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "PKCE",
			Status:   "WARN",
			Message:  "PKCE is disabled (not recommended for production)",
			Fix:      "Enable oauth.security.require_pkce",
		})
	} else {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "PKCE",
			Status:   "PASS",
			Message:  "PKCE enabled",
		})
	}

	if len(cfg.OAuthClients) == 0 {
		*results = append(*results, ValidationResult{
			Category: "OAuth",
			Item:     "Clients",
			Status:   "WARN",
			Message:  "No OAuth clients configured",
			Fix:      "Add oauth_clients section",
		})
	}
}

func validatePortConflicts(cfg *config.ComposeConfig, results *[]ValidationResult) {
	usedPorts := make(map[int]string)

	for serverName, server := range cfg.Servers {
		if server.HttpPort > 0 {
			if existing, exists := usedPorts[server.HttpPort]; exists && existing != serverName {
				*results = append(*results, ValidationResult{
					Category: "Ports",
					Item:     fmt.Sprintf("%s (port %d)", serverName, server.HttpPort),
					Status:   "FAIL",
					Message:  fmt.Sprintf("Port conflict with %s", existing),
					Fix:      "Change http_port to a unique value",
				})
			} else {
				usedPorts[server.HttpPort] = serverName
			}
		}

		for _, portMapping := range server.Ports {
			parts := strings.Split(portMapping, ":")
			if len(parts) >= 1 {
				var hostPort int
				fmt.Sscanf(parts[0], "%d", &hostPort)
				if hostPort > 0 {
					if existing, exists := usedPorts[hostPort]; exists && existing != serverName {
						*results = append(*results, ValidationResult{
							Category: "Ports",
							Item:     fmt.Sprintf("%s (port %d)", serverName, hostPort),
							Status:   "FAIL",
							Message:  fmt.Sprintf("Port conflict with %s", existing),
							Fix:      "Change port mapping",
						})
					} else {
						usedPorts[hostPort] = serverName
					}
				}
			}
		}
	}

	if len(usedPorts) > 0 {
		*results = append(*results, ValidationResult{
			Category: "Ports",
			Item:     "Allocation",
			Status:   "PASS",
			Message:  fmt.Sprintf("No port conflicts detected (%d ports allocated)", len(usedPorts)),
		})
	}
}

func validateSecurity(cfg *config.ComposeConfig, results *[]ValidationResult) {
	productionMode := os.Getenv("MCP_ENV") == "production"

	for serverName, server := range cfg.Servers {
		if server.Privileged {
			*results = append(*results, ValidationResult{
				Category: "Security",
				Item:     serverName,
				Status:   "WARN",
				Message:  "Running in privileged mode (security risk)",
				Fix:      "Remove privileged: true unless absolutely necessary",
			})
		}

		if len(server.CapAdd) > 0 {
			*results = append(*results, ValidationResult{
				Category: "Security",
				Item:     serverName,
				Status:   "WARN",
				Message:  fmt.Sprintf("Adding capabilities: %v", server.CapAdd),
				Fix:      "Review if capabilities are necessary",
			})
		}

		if server.User == "" && productionMode {
			*results = append(*results, ValidationResult{
				Category: "Security",
				Item:     serverName,
				Status:   "WARN",
				Message:  "Running as root user",
				Fix:      "Set user field (e.g., user: \"1000:1000\")",
			})
		}

		hasNoNewPrivileges := false
		for _, opt := range server.SecurityOpt {
			if strings.Contains(opt, "no-new-privileges") {
				hasNoNewPrivileges = true

				break
			}
		}

		if !hasNoNewPrivileges && productionMode {
			*results = append(*results, ValidationResult{
				Category: "Security",
				Item:     serverName,
				Status:   "WARN",
				Message:  "no-new-privileges not set",
				Fix:      "Add security_opt: [\"no-new-privileges:true\"]",
			})
		}

		if server.Security.AllowDockerSocket {
			*results = append(*results, ValidationResult{
				Category: "Security",
				Item:     serverName,
				Status:   "WARN",
				Message:  "Docker socket access enabled (high security risk)",
				Fix:      "Disable unless required for container management",
			})
		}
	}

	if cfg.ProxyAuth.Enabled {
		*results = append(*results, ValidationResult{
			Category: "Security",
			Item:     "Proxy Authentication",
			Status:   "PASS",
			Message:  "Proxy authentication enabled",
		})
	} else if productionMode {
		*results = append(*results, ValidationResult{
			Category: "Security",
			Item:     "Proxy Authentication",
			Status:   "WARN",
			Message:  "Proxy authentication disabled in production",
			Fix:      "Enable proxy_auth for production deployments",
		})
	}
}

func validateDockerImages(cfg *config.ComposeConfig, results *[]ValidationResult) {
	runtime, err := container.DetectRuntime()
	if err != nil {
		*results = append(*results, ValidationResult{
			Category: "Images",
			Item:     "Runtime",
			Status:   "WARN",
			Message:  "Could not detect container runtime",
			Fix:      "Install Docker or Podman",
		})

		return
	}

	*results = append(*results, ValidationResult{
		Category: "Images",
		Item:     "Runtime",
		Status:   "PASS",
		Message:  fmt.Sprintf("Using %s", runtime.GetRuntimeName()),
	})

	for serverName, server := range cfg.Servers {
		if server.Image == "" {
			continue
		}

		cmd := exec.Command(runtime.GetRuntimeName(), "image", "inspect", server.Image)
		if err := cmd.Run(); err != nil {
			*results = append(*results, ValidationResult{
				Category: "Images",
				Item:     serverName,
				Status:   "WARN",
				Message:  fmt.Sprintf("Image %s not found locally", server.Image),
				Fix:      fmt.Sprintf("Run: %s pull %s", runtime.GetRuntimeName(), server.Image),
			})
		} else {
			*results = append(*results, ValidationResult{
				Category: "Images",
				Item:     serverName,
				Status:   "PASS",
				Message:  fmt.Sprintf("Image %s available", server.Image),
			})
		}
	}
}

func printResults(results []ValidationResult, verbose bool) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	cyan := color.New(color.FgCyan)

	categories := make(map[string][]ValidationResult)
	for _, r := range results {
		categories[r.Category] = append(categories[r.Category], r)
	}

	for category, items := range categories {
		fmt.Printf("\n%s:\n", cyan.Sprint(category))

		for _, item := range items {
			if !verbose && item.Status == "PASS" {
				continue
			}

			status := item.Status
			switch item.Status {
			case "PASS":
				status = green.Sprint("PASS")
			case "FAIL":
				status = red.Sprint("FAIL")
			case "WARN":
				status = yellow.Sprint("WARN")
			case "INFO":
				status = cyan.Sprint("INFO")
			}

			fmt.Printf("  %s %-30s %s\n", status, item.Item, item.Message)

			if verbose && item.Fix != "" {
				fmt.Printf("    → %s\n", color.New(color.Faint).Sprint(item.Fix))
			}
		}
	}
}

func init() {
	data, err := os.ReadFile("config/schema.json")
	if err != nil {
		return
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return
	}
}

func validateAgainstSchema(cfg *config.ComposeConfig, schemaPath string) error {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		return fmt.Errorf("failed to parse schema: %w", err)
	}

	configYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(configYAML, &configMap); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return nil
}