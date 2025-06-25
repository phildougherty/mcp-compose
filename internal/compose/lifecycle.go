// internal/compose/lifecycle.go
package compose

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/logging"
)

// LifecycleManager handles pre/post hooks and health checks
type LifecycleManager struct {
	config     *config.ComposeConfig
	logger     *logging.Logger
	projectDir string
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(cfg *config.ComposeConfig, logger *logging.Logger, projectDir string) *LifecycleManager {
	return &LifecycleManager{
		config:     cfg,
		logger:     logger,
		projectDir: projectDir,
	}
}

// ExecutePreStartHook executes pre-start lifecycle hook
func (lm *LifecycleManager) ExecutePreStartHook(serverName string, hook string) error {
	return lm.executeHook("pre-start", serverName, hook)
}

// ExecutePostStartHook executes post-start lifecycle hook
func (lm *LifecycleManager) ExecutePostStartHook(serverName string, hook string) error {
	return lm.executeHook("post-start", serverName, hook)
}

// ExecutePreStopHook executes pre-stop lifecycle hook
func (lm *LifecycleManager) ExecutePreStopHook(serverName string, hook string) error {
	return lm.executeHook("pre-stop", serverName, hook)
}

// ExecutePostStopHook executes post-stop lifecycle hook
func (lm *LifecycleManager) ExecutePostStopHook(serverName string, hook string) error {
	return lm.executeHook("post-stop", serverName, hook)
}

func (lm *LifecycleManager) executeHook(phase, serverName, hook string) error {
	lm.logger.Info("Executing %s hook for %s: %s", phase, serverName, hook)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", hook)
	cmd.Dir = lm.projectDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		lm.logger.Error("Hook %s failed for %s: %v\nOutput: %s", phase, serverName, err, string(output))
		return fmt.Errorf("%s hook failed: %w", phase, err)
	}

	if len(output) > 0 {
		lm.logger.Info("Hook %s output for %s: %s", phase, serverName, strings.TrimSpace(string(output)))
	}

	return nil
}

// HealthChecker performs health checks on servers
type HealthChecker struct {
	config  config.HealthCheck
	logger  *logging.Logger
	baseURL string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(healthConfig config.HealthCheck, logger *logging.Logger, baseURL string) *HealthChecker {
	return &HealthChecker{
		config:  healthConfig,
		logger:  logger,
		baseURL: baseURL,
	}
}

// Check performs a health check
func (hc *HealthChecker) Check() error {
	if hc.config.Endpoint == "" {
		return fmt.Errorf("no health check endpoint configured")
	}

	timeout, err := time.ParseDuration(hc.config.Timeout)
	if err != nil {
		timeout = 5 * time.Second
	}

	// Implementation depends on check type
	// For now, assume HTTP endpoint check
	return hc.checkHTTPEndpoint(timeout)
}

func (hc *HealthChecker) checkHTTPEndpoint(timeout time.Duration) error {
	// Use the timeout parameter
	client := &http.Client{
		Timeout: timeout,
	}

	url := hc.baseURL
	if !strings.HasPrefix(hc.config.Endpoint, "http") {
		url = hc.baseURL + hc.config.Endpoint
	} else {
		url = hc.config.Endpoint
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	hc.logger.Debug("Health check passed for endpoint: %s", url)
	return nil
}
