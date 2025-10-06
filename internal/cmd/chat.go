package cmd

import (
	"database/sql"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"github.com/phildougherty/mcp-compose/internal/ai"
	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/dashboard"
	"github.com/phildougherty/mcp-compose/internal/logging"
	"github.com/phildougherty/mcp-compose/internal/task_scheduler"
	"github.com/phildougherty/mcp-compose/internal/tui"
)

func NewChatCommand() *cobra.Command {
	var sessionID string
	var provider string
	var model string

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive chat interface with MCP servers",
		Long: `Start an interactive chat session with AI that can access your MCP servers.
The chat interface uses Bubble Tea TUI and supports tool calling.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile, _ := cmd.Flags().GetString("file")

			return runChatTUI(configFile, sessionID, provider, model)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session-id", "", "Resume existing chat session")
	cmd.Flags().StringVar(&provider, "provider", "", "AI provider to use (openrouter, claude, openai, ollama)")
	cmd.Flags().StringVar(&model, "model", "", "Model to use")

	return cmd
}

func runChatTUI(configFile, sessionID, provider, model string) error {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	runtime, err := container.DetectRuntime()
	if err != nil {
		return fmt.Errorf("failed to detect container runtime: %w", err)
	}

	logger := logging.NewLogger(cfg.Logging.Level)
	logger.SetOutput(os.Stderr)

	var chatStorage *dashboard.ChatStorage
	postgresURL := os.Getenv("POSTGRES_URL")
	if postgresURL == "" {
		postgresURL = "host=localhost port=5433 user=postgres password=password dbname=mcp_compose sslmode=disable"
	}

	db, err := sql.Open("postgres", postgresURL)
	if err == nil {
		err = db.Ping()
	}

	if err == nil {
		chatStorage, err = dashboard.NewChatStorage(db)
		if err != nil {
			logger.Warning("Failed to initialize chat storage, using in-memory only: %v", err)
			chatStorage = nil
		}
	} else {
		logger.Warning("Database not available, using in-memory chat (messages won't persist): %v", err)
		chatStorage = nil
	}

	if chatStorage == nil && sessionID != "" {
		return fmt.Errorf("cannot resume session without database connection")
	}

	aiManager, err := initializeAIManager(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize AI manager: %w", err)
	}

	systemTools := createSystemToolsManager(cfg, runtime, logger)

	broadcaster := dashboard.NewChatBroadcaster(logger)
	chatService := dashboard.NewChatService(aiManager, chatStorage, systemTools, logger, broadcaster)

	var session *dashboard.ChatSession
	if sessionID != "" {
		session, err = chatService.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to load session: %w", err)
		}
	} else {
		if provider == "" {
			provider = "openrouter"
		}
		if model == "" {
			model = "z-ai/glm-4.6"
		}

		session, err = chatService.CreateSession("default", provider, model)
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
	}

	m := tui.NewChatModel(chatService, session)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run chat: %w", err)
	}

	return nil
}

func initializeAIManager(cfg *config.ComposeConfig, logger *logging.Logger) (*ai.Manager, error) {
	providers := []ai.Provider{}
	fallbackOrder := []string{}

	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		provider, err := ai.NewOpenRouterProvider(&ai.OpenRouterConfig{
			APIKey: apiKey,
			Model:  "anthropic/claude-3.5-sonnet",
		})
		if err != nil {
			logger.Error("Failed to create OpenRouter provider: %v", err)
		} else {
			providers = append(providers, provider)
			fallbackOrder = append(fallbackOrder, "openrouter")
			logger.Info("OpenRouter provider configured")
		}
	}

	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		provider, err := ai.NewClaudeProvider(&ai.ClaudeConfig{
			APIKey: apiKey,
			Model:  "claude-3-5-sonnet-20241022",
		})
		if err != nil {
			logger.Error("Failed to create Claude provider: %v", err)
		} else {
			providers = append(providers, provider)
			fallbackOrder = append(fallbackOrder, "claude")
			logger.Info("Claude provider configured")
		}
	}

	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		provider, err := ai.NewOpenAIProvider(&ai.OpenAIConfig{
			APIKey: apiKey,
			Model:  "gpt-4-turbo-preview",
		})
		if err != nil {
			logger.Error("Failed to create OpenAI provider: %v", err)
		} else {
			providers = append(providers, provider)
			fallbackOrder = append(fallbackOrder, "openai")
			logger.Info("OpenAI provider configured")
		}
	}

	if ollamaURL := os.Getenv("OLLAMA_URL"); ollamaURL != "" {
		provider, err := ai.NewOllamaProvider(&ai.OllamaConfig{
			BaseURL: ollamaURL,
			Model:   "llama2",
		})
		if err != nil {
			logger.Error("Failed to create Ollama provider: %v", err)
		} else {
			providers = append(providers, provider)
			fallbackOrder = append(fallbackOrder, "ollama")
			logger.Info("Ollama provider configured")
		}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no AI providers configured")
	}

	return ai.NewManager(&ai.ManagerConfig{
		Providers:     providers,
		FallbackOrder: fallbackOrder,
	})
}

func createSystemToolsManager(cfg *config.ComposeConfig, runtime container.Runtime, logger *logging.Logger) *dashboard.SystemToolsManager {
	var taskSched dashboard.TaskSchedulerManager
	if cfg.TaskScheduler.Enabled {
		taskSched = task_scheduler.NewManager(cfg, runtime)
	}

	var serverMgr dashboard.ServerManager

	return dashboard.NewSystemToolsManager(cfg, serverMgr, taskSched, nil)
}
