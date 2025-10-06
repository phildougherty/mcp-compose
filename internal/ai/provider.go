package ai

import (
	"context"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type ContentBlock interface {
	BlockType() string
}

type TextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (t TextBlock) BlockType() string { return "text" }

type ToolUseBlock struct {
	Type  string                 `json:"type"`
	ID    string                 `json:"id"`
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

func (t ToolUseBlock) BlockType() string { return "tool_use" }

type ToolResultBlock struct {
	Type      string `json:"type"`
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

func (t ToolResultBlock) BlockType() string { return "tool_result" }

type MessageWithContent struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

type ChatRequest struct {
	Messages []Message `json:"messages"`
	Tools    []Tool    `json:"tools,omitempty"`
}

type ChatResponse struct {
	Content      []ContentBlock `json:"content"`
	StopReason   string         `json:"stop_reason,omitempty"`
	ToolCalls    []ToolUseBlock `json:"tool_calls,omitempty"`
	TextContent  string         `json:"text_content,omitempty"`
}

type Provider interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error)
	Stream(ctx context.Context, messages []Message) (<-chan string, error)
	StreamWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan *ChatResponse, error)
	Health(ctx context.Context) error
	Name() string
	ListModels(ctx context.Context) ([]string, error)
}

type Config struct {
	Claude     *ClaudeConfig     `yaml:"claude,omitempty"`
	OpenAI     *OpenAIConfig     `yaml:"openai,omitempty"`
	Ollama     *OllamaConfig     `yaml:"ollama,omitempty"`
	OpenRouter *OpenRouterConfig `yaml:"openrouter,omitempty"`
}

type ClaudeConfig struct {
	APIKey         string        `yaml:"api_key"`
	Model          string        `yaml:"model"`
	MaxTokens      int           `yaml:"max_tokens"`
	Temperature    float64       `yaml:"temperature"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type OpenAIConfig struct {
	APIKey         string        `yaml:"api_key"`
	Model          string        `yaml:"model"`
	MaxTokens      int           `yaml:"max_tokens"`
	Temperature    float64       `yaml:"temperature"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type OllamaConfig struct {
	BaseURL        string        `yaml:"base_url"`
	Model          string        `yaml:"model"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
}

type OpenRouterConfig struct {
	APIKey         string        `yaml:"api_key"`
	Model          string        `yaml:"model"`
	MaxTokens      int           `yaml:"max_tokens"`
	Temperature    float64       `yaml:"temperature"`
	RequestTimeout time.Duration `yaml:"request_timeout"`
	SiteURL        string        `yaml:"site_url"`
	AppName        string        `yaml:"app_name"`
}

type ProviderError struct {
	Provider string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return e.Provider + ": " + e.Message + ": " + e.Err.Error()
	}

	return e.Provider + ": " + e.Message
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}