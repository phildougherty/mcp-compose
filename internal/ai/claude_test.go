package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClaudeProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *ClaudeConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ClaudeConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name:    "missing API key",
			config:  &ClaudeConfig{},
			wantErr: true,
		},
		{
			name: "config with defaults",
			config: &ClaudeConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewClaudeProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClaudeProvider() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				if provider == nil {
					t.Error("NewClaudeProvider() returned nil provider")
				}
				if provider.Name() != "claude" {
					t.Errorf("provider.Name() = %v, want claude", provider.Name())
				}
				if provider.config.Model == "" {
					t.Error("provider.config.Model should have default value")
				}
			}
		})
	}
}

func TestClaudeProvider_Chat(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		messages       []Message
		wantErr        bool
		expectedText   string
	}{
		{
			name: "successful response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := claudeResponse{
					ID:   "test-id",
					Type: "message",
					Role: "assistant",
					Content: []claudeContentBlock{
						{Type: "text", Text: "Hello, world!"},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			wantErr:      false,
			expectedText: "Hello, world!",
		},
		{
			name: "API error response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"error": {"type": "invalid_request_error", "message": "Invalid request"}}`))
			},
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			wantErr: true,
		},
		{
			name: "rate limit response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
			},
			messages: []Message{
				{Role: "user", Content: "Hello"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			config := &ClaudeConfig{
				APIKey:         "test-key",
				RequestTimeout: 5 * time.Second,
			}

			provider, err := NewClaudeProvider(config)
			if err != nil {
				t.Fatalf("NewClaudeProvider() error = %v", err)
			}

			provider.httpClient = server.Client()
			provider.apiURL = server.URL

			ctx := context.Background()
			response, err := provider.makeRequest(ctx, tt.messages, false)

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

func TestClaudeProvider_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		events := []string{
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`,
			`data: [DONE]`,
		}

		for _, event := range events {
			w.Write([]byte(event + "\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	}))
	defer server.Close()

	config := &ClaudeConfig{
		APIKey:         "test-key",
		RequestTimeout: 5 * time.Second,
	}

	provider, err := NewClaudeProvider(config)
	if err != nil {
		t.Fatalf("NewClaudeProvider() error = %v", err)
	}

	provider.httpClient = server.Client()
	provider.apiURL = server.URL

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "Hello"},
	}

	ch, err := provider.Stream(ctx, messages)
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var chunks []string
	for chunk := range ch {
		chunks = append(chunks, chunk)
	}

	if len(chunks) == 0 {
		t.Error("Stream() returned no chunks")
	}
}

func TestClaudeProvider_Health(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name: "healthy",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := claudeResponse{
					ID:   "test-id",
					Type: "message",
					Role: "assistant",
					Content: []claudeContentBlock{
						{Type: "text", Text: "Hello"},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "unhealthy",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.serverResponse))
			defer server.Close()

			config := &ClaudeConfig{
				APIKey:         "test-key",
				RequestTimeout: 5 * time.Second,
			}

			provider, err := NewClaudeProvider(config)
			if err != nil {
				t.Fatalf("NewClaudeProvider() error = %v", err)
			}

			provider.httpClient = server.Client()
			provider.apiURL = server.URL

			ctx := context.Background()
			err = provider.Health(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}