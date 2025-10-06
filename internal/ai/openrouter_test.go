package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenRouterProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *OpenRouterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &OpenRouterConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name:    "missing API key",
			config:  &OpenRouterConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenRouterProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenRouterProvider() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				if provider.Name() != "openrouter" {
					t.Errorf("provider.Name() = %v, want openrouter", provider.Name())
				}
				if provider.config.Model == "" {
					t.Error("provider.config.Model should have default value")
				}
			}
		})
	}
}

func TestOpenRouterProvider_Chat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openrouterResponse{
			ID:      "test-id",
			Model:   "test-model",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Choices: []struct {
				Index   int `json:"index"`
				Message struct {
					Role      string                `json:"role"`
					Content   string                `json:"content"`
					ToolCalls []openrouterToolCall  `json:"tool_calls,omitempty"`
				} `json:"message"`
				FinishReason string `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      struct {
						Role      string                `json:"role"`
						Content   string                `json:"content"`
						ToolCalls []openrouterToolCall  `json:"tool_calls,omitempty"`
					}{Role: "assistant", Content: "Hello, world!"},
					FinishReason: "stop",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &OpenRouterConfig{
		APIKey:         "test-key",
		RequestTimeout: 5 * time.Second,
	}

	provider, err := NewOpenRouterProvider(config)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	provider.httpClient = server.Client()
	provider.apiURL = server.URL

	ctx := context.Background()
	messages := []Message{{Role: "user", Content: "Hello"}}

	response, err := provider.Chat(ctx, messages)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if response != "Hello, world!" {
		t.Errorf("Chat() response = %v, want Hello, world!", response)
	}
}

func TestOpenRouterProvider_GetTotalCost(t *testing.T) {
	config := &OpenRouterConfig{
		APIKey: "test-key",
	}

	provider, err := NewOpenRouterProvider(config)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	cost := provider.GetTotalCost()
	if cost != 0 {
		t.Errorf("GetTotalCost() = %v, want 0", cost)
	}
}

func TestOpenRouterProvider_ResetCost(t *testing.T) {
	config := &OpenRouterConfig{
		APIKey: "test-key",
	}

	provider, err := NewOpenRouterProvider(config)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error = %v", err)
	}

	provider.totalCost = 10.5
	provider.ResetCost()

	cost := provider.GetTotalCost()
	if cost != 0 {
		t.Errorf("GetTotalCost() after reset = %v, want 0", cost)
	}
}