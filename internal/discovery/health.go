package discovery

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	DefaultHealthCheckInterval = 10 * time.Second
	DefaultHealthCheckTimeout  = 5 * time.Second
	MaxConsecutiveFailures     = 3
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

type HealthCheck struct {
	ServerName         string
	Endpoint           string
	Status             HealthStatus
	ConsecutiveFailures int
	LastCheck          time.Time
	LastHealthy        time.Time
	LastError          string
}

type HealthMonitor struct {
	checks         map[string]*HealthCheck
	mu             sync.RWMutex
	interval       time.Duration
	timeout        time.Duration
	logger         *logging.Logger
	stopCh         chan struct{}
	onStatusChange func(string, HealthStatus)
}

func NewHealthMonitor(logger *logging.Logger) *HealthMonitor {
	return &HealthMonitor{
		checks:   make(map[string]*HealthCheck),
		interval: DefaultHealthCheckInterval,
		timeout:  DefaultHealthCheckTimeout,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

func (m *HealthMonitor) OnStatusChange(handler func(string, HealthStatus)) {
	m.onStatusChange = handler
}

func (m *HealthMonitor) AddServer(name, endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.checks[name]; !exists {
		m.checks[name] = &HealthCheck{
			ServerName: name,
			Endpoint:   endpoint,
			Status:     HealthStatusUnknown,
			LastCheck:  time.Now(),
		}

		m.logger.Info("Added health check for server: %s (%s)", name, endpoint)
	}
}

func (m *HealthMonitor) RemoveServer(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.checks, name)
	m.logger.Info("Removed health check for server: %s", name)
}

func (m *HealthMonitor) Start(ctx context.Context) error {
	m.logger.Info("Starting health monitor")

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Health monitor stopped")

			return ctx.Err()

		case <-m.stopCh:
			m.logger.Info("Health monitor stopped")

			return nil

		case <-ticker.C:
			m.checkAll(ctx)
		}
	}
}

func (m *HealthMonitor) Stop() {
	close(m.stopCh)
}

func (m *HealthMonitor) checkAll(ctx context.Context) {
	m.mu.RLock()
	checks := make([]*HealthCheck, 0, len(m.checks))
	for _, check := range m.checks {
		checks = append(checks, check)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, check := range checks {
		wg.Add(1)

		go func(c *HealthCheck) {
			defer wg.Done()
			m.performHealthCheck(ctx, c)
		}(check)
	}

	wg.Wait()
}

func (m *HealthMonitor) performHealthCheck(ctx context.Context, check *HealthCheck) {
	checkCtx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(checkCtx, "GET", check.Endpoint, nil)
	if err != nil {
		m.handleFailure(check, fmt.Sprintf("failed to create request: %v", err))

		return
	}

	client := &http.Client{
		Timeout: m.timeout,
	}

	resp, err := client.Do(req)
	if err != nil {
		m.handleFailure(check, fmt.Sprintf("request failed: %v", err))

		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		m.handleSuccess(check)
	} else {
		m.handleFailure(check, fmt.Sprintf("unhealthy status code: %d", resp.StatusCode))
	}
}

func (m *HealthMonitor) handleSuccess(check *HealthCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldStatus := check.Status
	check.Status = HealthStatusHealthy
	check.ConsecutiveFailures = 0
	check.LastCheck = time.Now()
	check.LastHealthy = time.Now()
	check.LastError = ""

	if oldStatus != HealthStatusHealthy {
		m.logger.Info("Server %s is now healthy", check.ServerName)

		if m.onStatusChange != nil {
			go m.onStatusChange(check.ServerName, HealthStatusHealthy)
		}
	}
}

func (m *HealthMonitor) handleFailure(check *HealthCheck, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldStatus := check.Status
	check.ConsecutiveFailures++
	check.LastCheck = time.Now()
	check.LastError = reason

	if check.ConsecutiveFailures >= MaxConsecutiveFailures {
		check.Status = HealthStatusUnhealthy

		if oldStatus != HealthStatusUnhealthy {
			m.logger.Warning("Server %s is now unhealthy after %d failures: %s",
				check.ServerName, check.ConsecutiveFailures, reason)

			if m.onStatusChange != nil {
				go m.onStatusChange(check.ServerName, HealthStatusUnhealthy)
			}
		}
	} else {
		m.logger.Debug("Health check failed for %s (%d/%d): %s",
			check.ServerName, check.ConsecutiveFailures, MaxConsecutiveFailures, reason)
	}
}

func (m *HealthMonitor) GetStatus(name string) (HealthStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if check, exists := m.checks[name]; exists {
		return check.Status, true
	}

	return HealthStatusUnknown, false
}

func (m *HealthMonitor) GetAllStatuses() map[string]HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make(map[string]HealthStatus)
	for name, check := range m.checks {
		statuses[name] = check.Status
	}

	return statuses
}

func (m *HealthMonitor) GetHealthCheck(name string) (*HealthCheck, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	check, exists := m.checks[name]

	return check, exists
}