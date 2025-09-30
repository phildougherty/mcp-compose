package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOpenAIProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *OpenAIConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &OpenAIConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name:    "missing API key",
			config:  &OpenAIConfig{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOpenAIProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOpenAIProvider() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && provider.Name() != "openai" {
				t.Errorf("provider.Name() = %v, want openai", provider.Name())
			}
		})
	}
}

func TestOpenAIProvider_Chat(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		expectedText   string
	}{
		{
			name: "successful response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := openaiResponse{
					ID:      "test-id",
					Object:  "chat.completion",
					Created: time.Now().Unix(),
					Model:   "gpt-4",
					Choices: []struct {
						Index        int     `json:"index"`
						Message      Message `json:"message"`
						FinishReason string  `json:"finish_reason"`
					}{
						{
							Index:        0,
							Message:      Message{Role: "assistant", Content: "Hello, world!"},
							FinishReason: "stop",
						},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:      false,
			expectedText: "Hello, world!",
		},
		{
			name: "API error",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			config := &OpenAIConfig{
				APIKey:         "test-key",
				RequestTimeout: 5 * time.Second,
			}

			provider, err := NewOpenAIProvider(config)
			if err != nil {
				t.Fatalf("NewOpenAIProvider() error = %v", err)
			}

			provider.httpClient = server.Client()
			provider.apiURL = server.URL

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "Hello"}}

			response, err := provider.makeRequest(ctx, messages, false)

			if (err != nil) != tt.wantErr {
				t.Errorf("Chat() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && response != tt.expectedText {
				t.Errorf("Chat() response = %v, want %v", response, tt.expectedText)
			}
		})
	}
}