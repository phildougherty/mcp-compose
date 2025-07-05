package server

import (
	"context"
	"fmt"
	"testing"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/container"
)

func TestServerLifecycle(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"test-server": {
				Protocol: "stdio",
				Command:  "echo hello",
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	// Test server lifecycle
	t.Run("start_server", func(t *testing.T) {
		err := manager.StartServer("test-server")
		if err != nil {
			t.Errorf("Failed to start server: %v", err)
		}

		status, err := manager.GetServerStatus("test-server")
		if err != nil {
			t.Errorf("Failed to get server status: %v", err)
		}

		if status == "" {
			t.Error("Server status should not be empty")
		}
	})

	t.Run("stop_server", func(t *testing.T) {
		err := manager.StopServer("test-server")
		if err != nil {
			t.Errorf("Failed to stop server: %v", err)
		}
	})

	t.Run("start_nonexistent_server", func(t *testing.T) {
		err := manager.StartServer("nonexistent")
		if err == nil {
			t.Error("Expected error when starting nonexistent server")
		}
	})
}

func TestServerHealthChecks(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"health-test": {
				Protocol: "http",
				HttpPort: 8080,
				HealthCheck: &config.HealthCheck{
					Test:     []string{"CMD", "curl", "-f", "http://localhost:8080/health"},
					Interval: "30s",
					Timeout:  "5s",
					Retries:  3,
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	// Test health check configuration
	instance, exists := manager.GetServerInstance("health-test")
	if !exists {
		t.Fatal("Expected health-test server to exist")
	}

	if instance.Config.HealthCheck == nil {
		t.Error("Expected health check to be configured")
	}

	if instance.Config.HealthCheck.Retries != 3 {
		t.Errorf("Expected 3 retries, got %d", instance.Config.HealthCheck.Retries)
	}
}

func TestServerAuthentication(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"auth-server": {
				Protocol: "http",
				HttpPort: 8080,
				Authentication: &config.ServerAuthConfig{
					Enabled:       true,
					RequiredScope: "api:read",
					AllowAPIKey:   &[]bool{true}[0],
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("auth-server")
	if !exists {
		t.Fatal("Expected auth-server to exist")
	}

	if instance.Config.Authentication == nil {
		t.Error("Expected authentication to be configured")
	}

	if !instance.Config.Authentication.Enabled {
		t.Error("Expected authentication to be enabled")
	}

	if instance.Config.Authentication.RequiredScope != "api:read" {
		t.Errorf("Expected scope 'api:read', got %q", instance.Config.Authentication.RequiredScope)
	}
}

func TestServerOAuth(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"oauth-server": {
				Protocol: "http",
				HttpPort: 8080,
				OAuth: &config.ServerOAuthConfig{
					Enabled:             true,
					RequiredScope:       "server:access",
					AllowAPIKeyFallback: true,
					AllowedClients:      []string{"client1", "client2"},
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("oauth-server")
	if !exists {
		t.Fatal("Expected oauth-server to exist")
	}

	if instance.Config.OAuth == nil {
		t.Error("Expected OAuth to be configured")
	}

	if !instance.Config.OAuth.Enabled {
		t.Error("Expected OAuth to be enabled")
	}

	if len(instance.Config.OAuth.AllowedClients) != 2 {
		t.Errorf("Expected 2 allowed clients, got %d", len(instance.Config.OAuth.AllowedClients))
	}
}

func TestServerCapabilities(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"capability-server": {
				Protocol:     "stdio",
				Command:      "test-server",
				Capabilities: []string{"tools", "resources", "prompts"},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("capability-server")
	if !exists {
		t.Fatal("Expected capability-server to exist")
	}

	if len(instance.Config.Capabilities) != 3 {
		t.Errorf("Expected 3 capabilities, got %d", len(instance.Config.Capabilities))
	}

	expectedCaps := map[string]bool{
		"tools":     true,
		"resources": true,
		"prompts":   true,
	}

	for _, cap := range instance.Config.Capabilities {
		if !expectedCaps[cap] {
			t.Errorf("Unexpected capability: %s", cap)
		}
	}
}

func TestServerEnvironmentVariables(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"env-server": {
				Protocol: "stdio",
				Command:  "test-server",
				Env: map[string]string{
					"API_KEY":    "secret123",
					"DEBUG":      "true",
					"LOG_LEVEL":  "info",
					"PORT":       "8080",
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("env-server")
	if !exists {
		t.Fatal("Expected env-server to exist")
	}

	if len(instance.Config.Env) != 4 {
		t.Errorf("Expected 4 environment variables, got %d", len(instance.Config.Env))
	}

	if instance.Config.Env["API_KEY"] != "secret123" {
		t.Errorf("Expected API_KEY=secret123, got %s", instance.Config.Env["API_KEY"])
	}

	if instance.Config.Env["DEBUG"] != "true" {
		t.Errorf("Expected DEBUG=true, got %s", instance.Config.Env["DEBUG"])
	}
}

func TestServerVolumes(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"volume-server": {
				Image:    "test-image:latest",
				Protocol: "http",
				HttpPort: 8080,
				Volumes: []string{
					"/host/data:/container/data:ro",
					"/host/logs:/container/logs",
					"named-volume:/app/data",
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("volume-server")
	if !exists {
		t.Fatal("Expected volume-server to exist")
	}

	if len(instance.Config.Volumes) != 3 {
		t.Errorf("Expected 3 volumes, got %d", len(instance.Config.Volumes))
	}

	// Check for read-only volume
	found := false
	for _, volume := range instance.Config.Volumes {
		if volume == "/host/data:/container/data:ro" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find read-only volume mount")
	}
}

func TestServerNetworking(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Networks: map[string]config.NetworkConfig{
			"frontend": {
				Driver: "bridge",
			},
			"backend": {
				Driver: "overlay",
			},
		},
		Servers: map[string]config.ServerConfig{
			"network-server": {
				Image:       "test-image:latest",
				Protocol:    "http",
				HttpPort:    8080,
				Networks:    []string{"frontend", "backend"},
				NetworkMode: "bridge",
				Ports:       []string{"8080:8080", "9090:9090"},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("network-server")
	if !exists {
		t.Fatal("Expected network-server to exist")
	}

	if len(instance.Config.Networks) != 2 {
		t.Errorf("Expected 2 networks, got %d", len(instance.Config.Networks))
	}

	if instance.Config.NetworkMode != "bridge" {
		t.Errorf("Expected network mode 'bridge', got %s", instance.Config.NetworkMode)
	}

	if len(instance.Config.Ports) != 2 {
		t.Errorf("Expected 2 port mappings, got %d", len(instance.Config.Ports))
	}
}

func TestServerResourceLimits(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"resource-server": {
				Image:    "test-image:latest",
				Protocol: "http",
				HttpPort: 8080,
				Deploy: config.DeployConfig{
					Resources: config.ResourcesDeployConfig{
						Limits: config.ResourceLimitsConfig{
							CPUs:   "0.5",
							Memory: "512M",
							PIDs:   100,
						},
						Reservations: config.ResourceLimitsConfig{
							CPUs:   "0.25",
							Memory: "256M",
						},
					},
					Replicas: 2,
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("resource-server")
	if !exists {
		t.Fatal("Expected resource-server to exist")
	}

	if instance.Config.Deploy.Resources.Limits.CPUs != "0.5" {
		t.Errorf("Expected CPU limit 0.5, got %s", instance.Config.Deploy.Resources.Limits.CPUs)
	}

	if instance.Config.Deploy.Resources.Limits.Memory != "512M" {
		t.Errorf("Expected memory limit 512M, got %s", instance.Config.Deploy.Resources.Limits.Memory)
	}

	if instance.Config.Deploy.Replicas != 2 {
		t.Errorf("Expected 2 replicas, got %d", instance.Config.Deploy.Replicas)
	}
}

func TestServerSecurity(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"secure-server": {
				Image:      "test-image:latest",
				Protocol:   "http",
				HttpPort:   8080,
				User:       "1000:1000",
				ReadOnly:   true,
				Privileged: false,
				CapDrop:    []string{"ALL"},
				CapAdd:     []string{"NET_BIND_SERVICE"},
				SecurityOpt: []string{
					"no-new-privileges:true",
					"apparmor:unconfined",
				},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	instance, exists := manager.GetServerInstance("secure-server")
	if !exists {
		t.Fatal("Expected secure-server to exist")
	}

	if instance.Config.User != "1000:1000" {
		t.Errorf("Expected user 1000:1000, got %s", instance.Config.User)
	}

	if !instance.Config.ReadOnly {
		t.Error("Expected server to be read-only")
	}

	if instance.Config.Privileged {
		t.Error("Expected server to not be privileged")
	}

	if len(instance.Config.CapDrop) != 1 || instance.Config.CapDrop[0] != "ALL" {
		t.Error("Expected all capabilities to be dropped")
	}

	if len(instance.Config.CapAdd) != 1 || instance.Config.CapAdd[0] != "NET_BIND_SERVICE" {
		t.Error("Expected NET_BIND_SERVICE capability to be added")
	}
}

func TestServerDependencies(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"database": {
				Image:    "postgres:13",
				Protocol: "tcp",
				Env: map[string]string{
					"POSTGRES_DB":       "testdb",
					"POSTGRES_USER":     "test",
					"POSTGRES_PASSWORD": "password",
				},
			},
			"app-server": {
				Image:     "app:latest",
				Protocol:  "http",
				HttpPort:  8080,
				DependsOn: []string{"database"},
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	appInstance, exists := manager.GetServerInstance("app-server")
	if !exists {
		t.Fatal("Expected app-server to exist")
	}

	if len(appInstance.Config.DependsOn) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(appInstance.Config.DependsOn))
	}

	if appInstance.Config.DependsOn[0] != "database" {
		t.Errorf("Expected dependency on 'database', got %s", appInstance.Config.DependsOn[0])
	}
}

func TestConcurrentServerOperations(t *testing.T) {
	cfg := &config.ComposeConfig{
		Version: "1",
		Servers: map[string]config.ServerConfig{
			"concurrent-1": {
				Protocol: "stdio",
				Command:  "echo hello1",
			},
			"concurrent-2": {
				Protocol: "stdio",
				Command:  "echo hello2",
			},
			"concurrent-3": {
				Protocol: "stdio",
				Command:  "echo hello3",
			},
		},
	}

	manager, err := NewManager(cfg, &container.NullRuntime{})
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer manager.Shutdown()

	// Test concurrent operations
	done := make(chan error, 3)

	// Start servers concurrently
	for i := 1; i <= 3; i++ {
		go func(id int) {
			serverName := fmt.Sprintf("concurrent-%d", id)
			err := manager.StartServer(serverName)
			done <- err
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 3; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Concurrent start failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Concurrent start timed out")
		}
	}

	// Test concurrent status checks
	statusDone := make(chan error, 3)
	for i := 1; i <= 3; i++ {
		go func(id int) {
			serverName := fmt.Sprintf("concurrent-%d", id)
			_, err := manager.GetServerStatus(serverName)
			statusDone <- err
		}(i)
	}

	// Wait for status checks
	for i := 0; i < 3; i++ {
		select {
		case err := <-statusDone:
			if err != nil {
				t.Errorf("Concurrent status check failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("Concurrent status check timed out")
		}
	}
}