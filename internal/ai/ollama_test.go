package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewOllamaProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  *OllamaConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &OllamaConfig{
				BaseURL: "http://localhost:11434",
				Model:   "llama2",
			},
			wantErr: false,
		},
		{
			name:    "config with defaults",
			config:  &OllamaConfig{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOllamaProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOllamaProvider() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if provider.Name() != "ollama" {
				t.Errorf("provider.Name() = %v, want ollama", provider.Name())
			}
		})
	}
}

func TestOllamaProvider_Chat(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
		expectedText   string
	}{
		{
			name: "successful response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := ollamaResponse{
					Model:     "llama2",
					CreatedAt: time.Now().Format(time.RFC3339),
					Message: ollamaMessage{
						Role:    "assistant",
						Content: "Hello, world!",
					},
					Done: true,
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr:      false,
			expectedText: "Hello, world!",
		},
		{
			name: "error response",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := ollamaResponse{
					Error: "model not found",
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/chat" {
					tt.serverResponse(w, r)
				}
			}))
			defer server.Close()

			config := &OllamaConfig{
				BaseURL:        server.URL,
				Model:          "llama2",
				RequestTimeout: 5 * time.Second,
			}

			provider, err := NewOllamaProvider(config)
			if err != nil {
				t.Fatalf("NewOllamaProvider() error = %v", err)
			}

			ctx := context.Background()
			messages := []Message{{Role: "user", Content: "Hello"}}

			response, err := provider.Chat(ctx, messages)

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

func TestOllamaProvider_Health(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse func(w http.ResponseWriter, r *http.Request)
		wantErr        bool
	}{
		{
			name: "healthy",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := ollamaTagsResponse{
					Models: []struct {
						Name       string `json:"name"`
						ModifiedAt string `json:"modified_at"`
						Size       int64  `json:"size"`
					}{
						{Name: "llama2", ModifiedAt: time.Now().Format(time.RFC3339), Size: 1000000},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: false,
		},
		{
			name: "model not found",
			serverResponse: func(w http.ResponseWriter, r *http.Request) {
				resp := ollamaTagsResponse{
					Models: []struct {
						Name       string `json:"name"`
						ModifiedAt string `json:"modified_at"`
						Size       int64  `json:"size"`
					}{
						{Name: "other-model", ModifiedAt: time.Now().Format(time.RFC3339), Size: 1000000},
					},
				}

				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					tt.serverResponse(w, r)
				}
			}))
			defer server.Close()

			config := &OllamaConfig{
				BaseURL:        server.URL,
				Model:          "llama2",
				RequestTimeout: 5 * time.Second,
			}

			provider, err := NewOllamaProvider(config)
			if err != nil {
				t.Fatalf("NewOllamaProvider() error = %v", err)
			}

			ctx := context.Background()
			err = provider.Health(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Health() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOllamaProvider_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			resp := ollamaTagsResponse{
				Models: []struct {
					Name       string `json:"name"`
					ModifiedAt string `json:"modified_at"`
					Size       int64  `json:"size"`
				}{
					{Name: "llama2", ModifiedAt: time.Now().Format(time.RFC3339), Size: 1000000},
					{Name: "mistral", ModifiedAt: time.Now().Format(time.RFC3339), Size: 2000000},
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}
	}))
	defer server.Close()

	config := &OllamaConfig{
		BaseURL:        server.URL,
		Model:          "llama2",
		RequestTimeout: 5 * time.Second,
	}

	provider, err := NewOllamaProvider(config)
	if err != nil {
		t.Fatalf("NewOllamaProvider() error = %v", err)
	}

	ctx := context.Background()
	models, err := provider.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}

	if len(models) != 2 {
		t.Errorf("ListModels() returned %d models, want 2", len(models))
	}

	expectedModels := map[string]bool{"llama2": false, "mistral": false}
	for _, model := range models {
		if _, ok := expectedModels[model]; ok {
			expectedModels[model] = true
		}
	}

	for model, found := range expectedModels {
		if !found {
			t.Errorf("ListModels() missing model %s", model)
		}
	}
}