package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/phildougherty/mcp-compose/internal/config"
)

type serverCatalogEntry struct {
	name        string
	image       string
	description string
	ports       []string
	env         map[string]string
	protocol    string
	httpPort    int
}

var serverCatalog = []serverCatalogEntry{
	{
		name:        "filesystem",
		image:       "mcp/filesystem:latest",
		description: "File system access with read/write capabilities",
		protocol:    "stdio",
	},
	{
		name:        "memory",
		image:       "mcp/memory:latest",
		description: "Persistent memory and knowledge graph storage",
		protocol:    "http",
		httpPort:    8001,
	},
	{
		name:        "git",
		image:       "mcp/git:latest",
		description: "Git repository operations and version control",
		protocol:    "stdio",
	},
	{
		name:        "github",
		image:       "mcp/github:latest",
		description: "GitHub API integration for issues, PRs, and repos",
		env:         map[string]string{"GITHUB_TOKEN": "${GITHUB_TOKEN}"},
		protocol:    "http",
		httpPort:    8002,
	},
	{
		name:        "postgres",
		image:       "postgres:15-alpine",
		description: "PostgreSQL database for data storage",
		ports:       []string{"5432:5432"},
		env: map[string]string{
			"POSTGRES_PASSWORD": "${POSTGRES_PASSWORD:-postgres}",
			"POSTGRES_USER":     "postgres",
			"POSTGRES_DB":       "mcp",
		},
	},
	{
		name:        "search",
		image:       "mcp/search:latest",
		description: "Web search capabilities via multiple providers",
		protocol:    "http",
		httpPort:    8003,
	},
}

type initModel struct {
	step            int
	profile         string
	selectedServers map[string]bool
	authType        string
	apiKey          string
	proxyPort       int
	outputPath      string
	cursor          int
	err             error
	quitting        bool
}

type stepMsg int

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)
)

func NewInitCommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive setup wizard for mcp-compose",
		Long: `Initialize a new mcp-compose configuration with an interactive wizard.
Choose your deployment profile, select MCP servers, configure authentication,
and generate a production-ready mcp-compose.yaml file.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInitWizard(outputPath)
		},
	}

	cmd.Flags().StringVarP(&outputPath, "output", "o", "mcp-compose.yaml", "Output file path")

	return cmd
}

func runInitWizard(outputPath string) error {
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("Warning: %s already exists. Continue? (y/N): ", outputPath)
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(response) != "y" {
			fmt.Println("Aborted.")

			return nil
		}
	}

	m := initModel{
		step:            0,
		selectedServers: make(map[string]bool),
		proxyPort:       9876,
		outputPath:      outputPath,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run wizard: %w", err)
	}

	if finalModel.(initModel).quitting {
		return nil
	}

	return nil
}

func (m initModel) Init() tea.Cmd {
	return nil
}

func (m initModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true

			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}

		case "down", "j":
			switch m.step {
			case 0:
				if m.cursor < 2 {
					m.cursor++
				}
			case 1:
				if m.cursor < len(serverCatalog)-1 {
					m.cursor++
				}
			case 2:
				if m.cursor < 2 {
					m.cursor++
				}
			}

		case "enter", " ":
			switch m.step {
			case 0:
				profiles := []string{"development", "staging", "production"}
				m.profile = profiles[m.cursor]
				m.step++
				m.cursor = 0

			case 1:
				serverName := serverCatalog[m.cursor].name
				m.selectedServers[serverName] = !m.selectedServers[serverName]

			case 2:
				authTypes := []string{"none", "api-key", "oauth"}
				m.authType = authTypes[m.cursor]
				if m.authType == "api-key" {
					m.step++
				} else {
					m.step = 4
				}
				m.cursor = 0

			case 3:
				fmt.Print("\nEnter API Key: ")
				fmt.Scanln(&m.apiKey)
				m.step++
				m.cursor = 0
			}

		case "n":
			if m.step == 1 {
				m.step++
				m.cursor = 0
			}

		case "g":
			if m.step == 4 {
				if err := m.generateConfig(); err != nil {
					m.err = err
					m.step++
				} else {
					m.step++
				}
			}
		}

	case stepMsg:
		m.step = int(msg)
	}

	return m, nil
}

func (m initModel) View() string {
	if m.quitting {
		return "Wizard cancelled.\n"
	}

	var s strings.Builder

	s.WriteString(titleStyle.Render("MCP-Compose Interactive Setup Wizard"))
	s.WriteString("\n\n")

	switch m.step {
	case 0:
		s.WriteString("Select deployment profile:\n\n")
		profiles := []struct {
			name string
			desc string
		}{
			{"development", "Local development with hot-reload and debug logging"},
			{"staging", "Pre-production testing with monitoring"},
			{"production", "Production deployment with security hardening"},
		}

		for i, p := range profiles {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s", cursor, p.name)))
			} else {
				s.WriteString(normalStyle.Render(fmt.Sprintf("%s %s", cursor, p.name)))
			}
			s.WriteString("\n")
			s.WriteString(dimStyle.Render(fmt.Sprintf("  %s", p.desc)))
			s.WriteString("\n")
		}

		s.WriteString("\n" + dimStyle.Render("↑/↓: navigate • enter: select • q: quit"))

	case 1:
		s.WriteString(fmt.Sprintf("Selected profile: %s\n\n", selectedStyle.Render(m.profile)))
		s.WriteString("Select MCP servers to enable:\n\n")

		for i, srv := range serverCatalog {
			cursor := " "
			checkbox := "[ ]"
			if m.selectedServers[srv.name] {
				checkbox = "[X]"
			}
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s %s", cursor, checkbox, srv.name)))
			} else {
				s.WriteString(normalStyle.Render(fmt.Sprintf("%s %s %s", cursor, checkbox, srv.name)))
			}
			s.WriteString("\n")
			s.WriteString(dimStyle.Render(fmt.Sprintf("     %s", srv.description)))
			s.WriteString("\n")
		}

		s.WriteString("\n" + dimStyle.Render("↑/↓: navigate • space: toggle • n: next • q: quit"))

	case 2:
		s.WriteString(fmt.Sprintf("Profile: %s\n", selectedStyle.Render(m.profile)))

		selected := []string{}
		for name := range m.selectedServers {
			if m.selectedServers[name] {
				selected = append(selected, name)
			}
		}
		s.WriteString(fmt.Sprintf("Servers: %s\n\n", selectedStyle.Render(strings.Join(selected, ", "))))

		s.WriteString("Configure authentication:\n\n")
		authTypes := []struct {
			name string
			desc string
		}{
			{"none", "No authentication (development only)"},
			{"api-key", "API key authentication (recommended)"},
			{"oauth", "OAuth 2.1 with PKCE (enterprise)"},
		}

		for i, auth := range authTypes {
			cursor := " "
			if m.cursor == i {
				cursor = ">"
				s.WriteString(selectedStyle.Render(fmt.Sprintf("%s %s", cursor, auth.name)))
			} else {
				s.WriteString(normalStyle.Render(fmt.Sprintf("%s %s", cursor, auth.name)))
			}
			s.WriteString("\n")
			s.WriteString(dimStyle.Render(fmt.Sprintf("  %s", auth.desc)))
			s.WriteString("\n")
		}

		s.WriteString("\n" + dimStyle.Render("↑/↓: navigate • enter: select • q: quit"))

	case 3:
		s.WriteString("Enter API Key (will be stored as ${MCP_API_KEY}):\n")
		s.WriteString(dimStyle.Render("Press Enter after entering the key\n"))

	case 4:
		s.WriteString(successStyle.Render("Configuration ready\n\n"))
		s.WriteString(fmt.Sprintf("Profile: %s\n", m.profile))

		selected := []string{}
		for name := range m.selectedServers {
			if m.selectedServers[name] {
				selected = append(selected, name)
			}
		}
		s.WriteString(fmt.Sprintf("Servers: %s\n", strings.Join(selected, ", ")))
		s.WriteString(fmt.Sprintf("Auth: %s\n", m.authType))
		s.WriteString(fmt.Sprintf("Output: %s\n\n", m.outputPath))

		s.WriteString(dimStyle.Render("Press 'g' to generate configuration or 'q' to quit"))

	case 5:
		if m.err != nil {
			s.WriteString(errorStyle.Render(fmt.Sprintf("✗ Error: %v\n", m.err)))
		} else {
			s.WriteString(successStyle.Render("Configuration generated successfully!\n\n"))
			s.WriteString(fmt.Sprintf("Created: %s\n\n", m.outputPath))
			s.WriteString("Next steps:\n")
			s.WriteString("  1. Review and edit the configuration if needed\n")
			s.WriteString("  2. Set required environment variables:\n")
			if m.authType == "api-key" {
				s.WriteString("     export MCP_API_KEY='your-secret-key'\n")
			}
			s.WriteString("  3. Validate: ./mcp-compose validate\n")
			s.WriteString("  4. Start servers: ./mcp-compose up\n")
			s.WriteString("  5. Start proxy: ./mcp-compose proxy --port 9876\n")
		}

		return s.String()
	}

	return s.String()
}

func (m *initModel) generateConfig() error {
	if err := m.validatePrerequisites(); err != nil {
		return fmt.Errorf("prerequisite check failed: %w", err)
	}

	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: make(map[string]config.ServerConfig),
	}

	if m.authType == "api-key" {
		cfg.ProxyAuth = config.ProxyAuthConfig{
			Enabled: true,
			APIKey:  "${MCP_API_KEY}",
		}
	} else if m.authType == "oauth" {
		cfg.OAuth = &config.OAuthConfig{
			Enabled: true,
			Issuer:  "http://localhost:9876",
			Endpoints: config.OAuthEndpoints{
				Authorization: "/oauth/authorize",
				Token:         "/oauth/token",
				UserInfo:      "/oauth/userinfo",
				Revoke:        "/oauth/revoke",
				Discovery:     "/.well-known/oauth-authorization-server",
			},
			Tokens: config.TokenConfig{
				AccessTokenTTL:  "1h",
				RefreshTokenTTL: "168h",
				CodeTTL:         "10m",
				Algorithm:       "RS256",
			},
			Security: config.OAuthSecurityConfig{
				RequirePKCE: true,
			},
			GrantTypes:      []string{"authorization_code", "refresh_token"},
			ResponseTypes:   []string{"code"},
			ScopesSupported: []string{"read", "write", "admin"},
		}
	}

	for _, srv := range serverCatalog {
		if !m.selectedServers[srv.name] {
			continue
		}

		serverConfig := config.ServerConfig{
			Image:    srv.image,
			Protocol: srv.protocol,
			Env:      srv.env,
			Ports:    srv.ports,
		}

		if srv.httpPort > 0 {
			serverConfig.HttpPort = srv.httpPort
		}

		if m.profile == "production" {
			serverConfig.Security = config.SecurityConfig{
				Auth: config.AuthConfig{
					Type: m.authType,
				},
			}
			serverConfig.User = "1000:1000"
			serverConfig.CapDrop = []string{"ALL"}
			serverConfig.SecurityOpt = []string{"no-new-privileges:true"}
		}

		cfg.Servers[srv.name] = serverConfig
	}

	cfg.Logging = config.LoggingConfig{
		Level:  "info",
		Format: "json",
	}

	if m.profile != "production" {
		cfg.Logging.Level = "debug"
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(m.outputPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	if m.authType == "api-key" {
		envPath := filepath.Join(filepath.Dir(m.outputPath), ".env.example")
		envContent := "# MCP-Compose Environment Variables\n"
		envContent += "MCP_API_KEY=your-secret-key-here\n"

		for _, srv := range serverCatalog {
			if m.selectedServers[srv.name] && srv.env != nil {
				for key := range srv.env {
					if strings.Contains(key, "TOKEN") || strings.Contains(key, "PASSWORD") {
						envContent += fmt.Sprintf("%s=\n", key)
					}
				}
			}
		}

		if err := os.WriteFile(envPath, []byte(envContent), 0644); err != nil {
			return fmt.Errorf("failed to write .env.example: %w", err)
		}
	}

	return nil
}

func (m *initModel) validatePrerequisites() error {
	runtimes := []string{"docker", "podman"}
	var availableRuntime string

	for _, runtime := range runtimes {
		if _, err := exec.LookPath(runtime); err == nil {
			availableRuntime = runtime

			break
		}
	}

	if availableRuntime == "" {
		return fmt.Errorf("neither Docker nor Podman is installed")
	}

	portInUse := false
	cmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -i:%d 2>/dev/null || ss -tuln 2>/dev/null | grep :%d", m.proxyPort, m.proxyPort))
	if output, _ := cmd.Output(); len(output) > 0 {
		portInUse = true
	}

	if portInUse {
		return fmt.Errorf("port %d is already in use", m.proxyPort)
	}

	return nil
}