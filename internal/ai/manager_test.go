package ai

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockProvider struct {
	name          string
	chatResponse  string
	chatError     error
	healthError   error
	streamEnabled bool
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Chat(ctx context.Context, messages []Message) (string, error) {
	if m.chatError != nil {
		return "", m.chatError
	}

	return m.chatResponse, nil
}

func (m *mockProvider) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
	ch := make(chan string, 10)

	if m.chatError != nil {
		close(ch)

		return ch, m.chatError
	}

	go func() {
		defer close(ch)
		ch <- m.chatResponse
	}()

	return ch, nil
}

func (m *mockProvider) Health(ctx context.Context) error {
	return m.healthError
}

func (m *mockProvider) ChatWithTools(ctx context.Context, messages []Message, tools []Tool) (*ChatResponse, error) {
	if m.chatError != nil {
		return nil, m.chatError
	}

	return &ChatResponse{
		TextContent: m.chatResponse,
		Content: []ContentBlock{
			TextBlock{Type: "text", Text: m.chatResponse},
		},
	}, nil
}

func (m *mockProvider) StreamWithTools(ctx context.Context, messages []Message, tools []Tool) (<-chan *ChatResponse, error) {
	ch := make(chan *ChatResponse, 10)

	if m.chatError != nil {
		close(ch)

		return ch, m.chatError
	}

	go func() {
		defer close(ch)
		ch <- &ChatResponse{
			TextContent: m.chatResponse,
			Content: []ContentBlock{
				TextBlock{Type: "text", Text: m.chatResponse},
			},
		}
	}()

	return ch, nil
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name    string
		config  *ManagerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ManagerConfig{
				Providers: []Provider{
					&mockProvider{name: "test1"},
				},
			},
			wantErr: false,
		},
		{
			name: "no providers",
			config: &ManagerConfig{
				Providers: []Provider{},
			},
			wantErr: true,
		},
		{
			name: "custom fallback order",
			config: &ManagerConfig{
				Providers: []Provider{
					&mockProvider{name: "test1"},
					&mockProvider{name: "test2"},
				},
				FallbackOrder: []string{"test2", "test1"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager, err := NewManager(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewManager() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr {
				if manager == nil {
					t.Error("NewManager() returned nil manager")
				}
				defer manager.Stop()
			}
		})
	}
}

func TestManager_Chat(t *testing.T) {
	tests := []struct {
		name         string
		providers    []Provider
		fallbackOrder []string
		expectedText string
		wantErr      bool
	}{
		{
			name: "first provider succeeds",
			providers: []Provider{
				&mockProvider{
					name:         "test1",
					chatResponse: "response1",
				},
				&mockProvider{
					name:         "test2",
					chatResponse: "response2",
				},
			},
			fallbackOrder: []string{"test1", "test2"},
			expectedText:  "response1",
			wantErr:       false,
		},
		{
			name: "first provider fails, second succeeds",
			providers: []Provider{
				&mockProvider{
					name:      "test1",
					chatError: errors.New("test error"),
				},
				&mockProvider{
					name:         "test2",
					chatResponse: "response2",
				},
			},
			fallbackOrder: []string{"test1", "test2"},
			expectedText:  "response2",
			wantErr:       false,
		},
		{
			name: "all providers fail",
			providers: []Provider{
				&mockProvider{
					name:      "test1",
					chatError: errors.New("test error 1"),
				},
				&mockProvider{
					name:      "test2",
					chatError: errors.New("test error 2"),
				},
			},
			fallbackOrder: []string{"test1", "test2"},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ManagerConfig{
				Providers:     tt.providers,
				FallbackOrder: tt.fallbackOrder,
			}

			manager, err := NewManager(config)
			if err != nil {
				t.Fatalf("NewManager() error = %v", err)
			}
			defer manager.Stop()

			ctx := context.Background()
			messages := []Message{
				{Role: "user", Content: "test"},
			}

			response, err := manager.Chat(ctx, messages)

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

func TestManager_Stream(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		chatResponse: "response",
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider},
		FallbackOrder: []string{"test"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "test"},
	}

	ch, err := manager.Stream(ctx, messages)
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

	if chunks[0] != "response" {
		t.Errorf("Stream() chunk = %v, want response", chunks[0])
	}
}

func TestManager_CircuitBreaker(t *testing.T) {
	provider := &mockProvider{
		name:      "test",
		chatError: errors.New("test error"),
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider},
		FallbackOrder: []string{"test"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "test"},
	}

	for i := 0; i < circuitBreakerThreshold; i++ {
		_, _ = manager.Chat(ctx, messages)
	}

	if manager.isProviderHealthy("test") {
		t.Error("Provider should be unhealthy after circuit breaker threshold")
	}

	err = manager.ResetCircuitBreaker("test")
	if err != nil {
		t.Errorf("ResetCircuitBreaker() error = %v", err)
	}

	if !manager.isProviderHealthy("test") {
		t.Error("Provider should be healthy after reset")
	}
}

func TestManager_EnableDisableProvider(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		chatResponse: "response",
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider},
		FallbackOrder: []string{"test"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	if !manager.isProviderHealthy("test") {
		t.Error("Provider should be healthy initially")
	}

	err = manager.DisableProvider("test")
	if err != nil {
		t.Errorf("DisableProvider() error = %v", err)
	}

	if manager.isProviderHealthy("test") {
		t.Error("Provider should be unhealthy after disable")
	}

	err = manager.EnableProvider("test")
	if err != nil {
		t.Errorf("EnableProvider() error = %v", err)
	}

	if !manager.isProviderHealthy("test") {
		t.Error("Provider should be healthy after enable")
	}
}

func TestManager_GetProviderStatus(t *testing.T) {
	provider := &mockProvider{
		name:         "test",
		chatResponse: "response",
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider},
		FallbackOrder: []string{"test"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	status := manager.GetProviderStatus()

	if len(status) == 0 {
		t.Error("GetProviderStatus() returned empty map")
	}

	testStatus, exists := status["test"]
	if !exists {
		t.Error("GetProviderStatus() missing test provider")
	}

	if testStatus.Name != "test" {
		t.Errorf("Status.Name = %v, want test", testStatus.Name)
	}

	if !testStatus.Enabled {
		t.Error("Status.Enabled should be true")
	}
}

func TestManager_GetHealthyProvider(t *testing.T) {
	provider1 := &mockProvider{
		name:      "test1",
		chatError: errors.New("error"),
	}

	provider2 := &mockProvider{
		name:         "test2",
		chatResponse: "response",
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider1, provider2},
		FallbackOrder: []string{"test1", "test2"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	ctx := context.Background()
	messages := []Message{
		{Role: "user", Content: "test"},
	}

	for i := 0; i < circuitBreakerThreshold; i++ {
		_, _ = manager.Chat(ctx, messages)
	}

	healthyProvider, err := manager.GetHealthyProvider()
	if err != nil {
		t.Fatalf("GetHealthyProvider() error = %v", err)
	}

	if healthyProvider.Name() != "test2" {
		t.Errorf("GetHealthyProvider() name = %v, want test2", healthyProvider.Name())
	}
}

func TestManager_HealthCheckLoop(t *testing.T) {
	provider := &mockProvider{
		name:        "test",
		healthError: errors.New("health check failed"),
	}

	config := &ManagerConfig{
		Providers:     []Provider{provider},
		FallbackOrder: []string{"test"},
	}

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	defer manager.Stop()

	time.Sleep(100 * time.Millisecond)

	status := manager.GetProviderStatus()
	testStatus := status["test"]

	if testStatus.LastHealthCheck.IsZero() {
		t.Error("Health check should have run")
	}
}