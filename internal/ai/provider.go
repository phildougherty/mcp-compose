package ai

import (
	"context"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	Chat(ctx context.Context, messages []Message) (string, error)
	Stream(ctx context.Context, messages []Message) (<-chan string, error)
	Health(ctx context.Context) error
	Name() string
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