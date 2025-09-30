package ai

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	healthCheckInterval    = 30 * time.Second
	circuitBreakerThreshold = 5
)

type ProviderStatus struct {
	Name            string
	Healthy         bool
	LastHealthCheck time.Time
	ConsecutiveFailures int
	Enabled         bool
}

type Manager struct {
	providers       []Provider
	providerStatus  map[string]*ProviderStatus
	fallbackOrder   []string
	mu              sync.RWMutex
	healthTicker    *time.Ticker
	stopCh          chan struct{}
	ctx             context.Context
	cancel          context.CancelFunc
}

type ManagerConfig struct {
	FallbackOrder []string
	Providers     []Provider
}

func NewManager(config *ManagerConfig) (*Manager, error) {
	if len(config.Providers) == 0 {
		return nil, fmt.Errorf("at least one provider is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		providers:      config.Providers,
		providerStatus: make(map[string]*ProviderStatus),
		fallbackOrder:  config.FallbackOrder,
		healthTicker:   time.NewTicker(healthCheckInterval),
		stopCh:         make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}

	for _, provider := range config.Providers {
		m.providerStatus[provider.Name()] = &ProviderStatus{
			Name:            provider.Name(),
			Healthy:         true,
			LastHealthCheck: time.Time{},
			ConsecutiveFailures: 0,
			Enabled:         true,
		}
	}

	if len(m.fallbackOrder) == 0 {
		m.fallbackOrder = []string{"claude", "openai", "ollama", "openrouter"}
	}

	go m.healthCheckLoop()

	return m, nil
}

func (m *Manager) Chat(ctx context.Context, messages []Message) (string, error) {
	var lastErr error

	for _, providerName := range m.fallbackOrder {
		provider := m.getProvider(providerName)
		if provider == nil {
			continue
		}

		if !m.isProviderHealthy(providerName) {
			continue
		}

		response, err := provider.Chat(ctx, messages)
		if err == nil {
			m.recordSuccess(providerName)

			return response, nil
		}

		m.recordFailure(providerName)
		lastErr = err
	}

	if lastErr != nil {
		return "", fmt.Errorf("all providers failed, last error: %w", lastErr)
	}

	return "", fmt.Errorf("no healthy providers available")
}

func (m *Manager) Stream(ctx context.Context, messages []Message) (<-chan string, error) {
	for _, providerName := range m.fallbackOrder {
		provider := m.getProvider(providerName)
		if provider == nil {
			continue
		}

		if !m.isProviderHealthy(providerName) {
			continue
		}

		ch, err := provider.Stream(ctx, messages)
		if err == nil {
			wrappedCh := make(chan string, 100)

			go func() {
				defer close(wrappedCh)

				for {
					select {
					case msg, ok := <-ch:
						if !ok {
							m.recordSuccess(providerName)

							return
						}

						select {
						case wrappedCh <- msg:
						case <-ctx.Done():
							return
						}
					case <-ctx.Done():
						return
					}
				}
			}()

			return wrappedCh, nil
		}

		m.recordFailure(providerName)
	}

	return nil, fmt.Errorf("no healthy providers available for streaming")
}

func (m *Manager) getProvider(name string) Provider {
	for _, provider := range m.providers {
		if provider.Name() == name {
			return provider
		}
	}

	return nil
}

func (m *Manager) isProviderHealthy(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status, exists := m.providerStatus[name]
	if !exists {
		return false
	}

	if !status.Enabled {
		return false
	}

	if status.ConsecutiveFailures >= circuitBreakerThreshold {
		return false
	}

	return status.Healthy
}

func (m *Manager) recordSuccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, exists := m.providerStatus[name]; exists {
		status.Healthy = true
		status.ConsecutiveFailures = 0
	}
}

func (m *Manager) recordFailure(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if status, exists := m.providerStatus[name]; exists {
		status.ConsecutiveFailures++
		if status.ConsecutiveFailures >= circuitBreakerThreshold {
			status.Healthy = false
		}
	}
}

func (m *Manager) healthCheckLoop() {
	m.performHealthChecks()

	for {
		select {
		case <-m.healthTicker.C:
			m.performHealthChecks()
		case <-m.stopCh:
			return
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *Manager) performHealthChecks() {
	ctx, cancel := context.WithTimeout(m.ctx, 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for _, provider := range m.providers {
		wg.Add(1)

		go func(p Provider) {
			defer wg.Done()

			err := p.Health(ctx)

			m.mu.Lock()
			defer m.mu.Unlock()

			status, exists := m.providerStatus[p.Name()]
			if !exists {
				return
			}

			status.LastHealthCheck = time.Now()

			if err != nil {
				status.ConsecutiveFailures++
				if status.ConsecutiveFailures >= circuitBreakerThreshold {
					status.Healthy = false
				}
			} else {
				status.Healthy = true
				status.ConsecutiveFailures = 0
			}
		}(provider)
	}

	wg.Wait()
}

func (m *Manager) GetProviderStatus() map[string]*ProviderStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statusCopy := make(map[string]*ProviderStatus)
	for name, status := range m.providerStatus {
		statusCopy[name] = &ProviderStatus{
			Name:            status.Name,
			Healthy:         status.Healthy,
			LastHealthCheck: status.LastHealthCheck,
			ConsecutiveFailures: status.ConsecutiveFailures,
			Enabled:         status.Enabled,
		}
	}

	return statusCopy
}

func (m *Manager) EnableProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, exists := m.providerStatus[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	status.Enabled = true
	status.ConsecutiveFailures = 0

	return nil
}

func (m *Manager) DisableProvider(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, exists := m.providerStatus[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	status.Enabled = false

	return nil
}

func (m *Manager) GetHealthyProvider() (Provider, error) {
	for _, providerName := range m.fallbackOrder {
		if m.isProviderHealthy(providerName) {
			provider := m.getProvider(providerName)
			if provider != nil {
				return provider, nil
			}
		}
	}

	return nil, fmt.Errorf("no healthy providers available")
}

func (m *Manager) Stop() {
	close(m.stopCh)
	m.healthTicker.Stop()
	m.cancel()
}

func (m *Manager) ResetCircuitBreaker(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	status, exists := m.providerStatus[name]
	if !exists {
		return fmt.Errorf("provider %s not found", name)
	}

	status.ConsecutiveFailures = 0
	status.Healthy = true

	return nil
}