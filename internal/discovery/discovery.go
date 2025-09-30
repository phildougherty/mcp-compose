package discovery

import (
	"context"
	"fmt"
	"sync"

	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

type ProxyRegistry interface {
	RegisterServer(name, endpoint, protocol string) error
	UnregisterServer(name string) error
}

type DiscoveryService struct {
	watcher       *DockerWatcher
	health        *HealthMonitor
	registry      ProxyRegistry
	logger        *logging.Logger
	autoRegister  bool
	mu            sync.RWMutex
}

func NewDiscoveryService(runtime container.Runtime, registry ProxyRegistry, logger *logging.Logger) *DiscoveryService {
	watcher := NewDockerWatcher(runtime, logger)
	health := NewHealthMonitor(logger)

	service := &DiscoveryService{
		watcher:      watcher,
		health:       health,
		registry:     registry,
		logger:       logger,
		autoRegister: true,
	}

	watcher.OnChange(service.handleServerChange)
	health.OnStatusChange(service.handleHealthChange)

	return service
}

func (s *DiscoveryService) SetAutoRegister(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.autoRegister = enabled
}

func (s *DiscoveryService) Start(ctx context.Context) error {
	s.logger.Info("Starting discovery service")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.watcher.Start(ctx); err != nil && err != context.Canceled {
			s.logger.Errorf("Docker watcher error: %v", err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.health.Start(ctx); err != nil && err != context.Canceled {
			s.logger.Errorf("Health monitor error: %v", err)
		}
	}()

	wg.Wait()

	return nil
}

func (s *DiscoveryService) Stop() {
	s.logger.Info("Stopping discovery service")
	s.watcher.Stop()
	s.health.Stop()
}

func (s *DiscoveryService) handleServerChange(server *Server, changeType ChangeType) {
	s.mu.RLock()
	autoRegister := s.autoRegister
	s.mu.RUnlock()

	if !autoRegister {
		return
	}

	switch changeType {
	case ChangeTypeAdded:
		if server.Protocol == "http" || server.Protocol == "sse" {
			endpoint := fmt.Sprintf("http://localhost:%d", server.Port)

			if err := s.registry.RegisterServer(server.Name, endpoint, server.Protocol); err != nil {
				s.logger.Errorf("Failed to register server %s: %v", server.Name, err)
			} else {
				s.watcher.MarkRegistered(server.Name)
				s.health.AddServer(server.Name, endpoint+"/health")
				s.logger.Infof("Registered server %s with proxy", server.Name)
			}
		}

	case ChangeTypeRemoved:
		if err := s.registry.UnregisterServer(server.Name); err != nil {
			s.logger.Errorf("Failed to unregister server %s: %v", server.Name, err)
		} else {
			s.health.RemoveServer(server.Name)
			s.logger.Infof("Unregistered server %s from proxy", server.Name)
		}

	case ChangeTypeUpdated:
		if server.Status != "running" && server.Registered {
			if err := s.registry.UnregisterServer(server.Name); err != nil {
				s.logger.Errorf("Failed to unregister unhealthy server %s: %v", server.Name, err)
			} else {
				s.watcher.MarkUnregistered(server.Name)
				s.logger.Infof("Unregistered unhealthy server %s", server.Name)
			}
		}
	}
}

func (s *DiscoveryService) handleHealthChange(serverName string, status HealthStatus) {
	server, exists := s.watcher.GetServer(serverName)
	if !exists {
		return
	}

	s.mu.RLock()
	autoRegister := s.autoRegister
	s.mu.RUnlock()

	if !autoRegister {
		return
	}

	switch status {
	case HealthStatusHealthy:
		if !server.Registered && server.Status == "running" {
			endpoint := fmt.Sprintf("http://localhost:%d", server.Port)

			if err := s.registry.RegisterServer(server.Name, endpoint, server.Protocol); err != nil {
				s.logger.Errorf("Failed to re-register healthy server %s: %v", server.Name, err)
			} else {
				s.watcher.MarkRegistered(server.Name)
				s.logger.Infof("Re-registered healthy server %s", server.Name)
			}
		}

	case HealthStatusUnhealthy:
		if server.Registered {
			if err := s.registry.UnregisterServer(server.Name); err != nil {
				s.logger.Errorf("Failed to unregister unhealthy server %s: %v", server.Name, err)
			} else {
				s.watcher.MarkUnregistered(server.Name)
				s.logger.Infof("Unregistered unhealthy server %s", server.Name)
			}
		}
	}
}

func (s *DiscoveryService) GetServers() []*Server {
	return s.watcher.GetServers()
}

func (s *DiscoveryService) GetServer(name string) (*Server, bool) {
	return s.watcher.GetServer(name)
}

func (s *DiscoveryService) GetHealthStatus(name string) (HealthStatus, bool) {
	return s.health.GetStatus(name)
}

func (s *DiscoveryService) GetAllHealthStatuses() map[string]HealthStatus {
	return s.health.GetAllStatuses()
}