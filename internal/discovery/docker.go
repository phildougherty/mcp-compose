package discovery

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/container"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	MCPLabelRole     = "mcp.compose/role"
	MCPLabelName     = "mcp.compose/name"
	MCPLabelProtocol = "mcp.compose/protocol"
	MCPLabelPort     = "mcp.compose/port"
	MCPLabelVersion  = "mcp.compose/version"
)

type Server struct {
	Name         string
	ContainerID  string
	Protocol     string
	Port         int
	Status       string
	HealthStatus string
	Metadata     map[string]string
	LastSeen     time.Time
	Registered   bool
}

type DockerWatcher struct {
	runtime      container.Runtime
	servers      map[string]*Server
	mu           sync.RWMutex
	onChange     func(*Server, ChangeType)
	logger       *logging.Logger
	stopCh       chan struct{}
	pollInterval time.Duration
}

type ChangeType int

const (
	ChangeTypeAdded ChangeType = iota
	ChangeTypeUpdated
	ChangeTypeRemoved
)

func NewDockerWatcher(runtime container.Runtime, logger *logging.Logger) *DockerWatcher {
	return &DockerWatcher{
		runtime:      runtime,
		servers:      make(map[string]*Server),
		logger:       logger,
		stopCh:       make(chan struct{}),
		pollInterval: 5 * time.Second,
	}
}

func (w *DockerWatcher) OnChange(handler func(*Server, ChangeType)) {
	w.onChange = handler
}

func (w *DockerWatcher) Start(ctx context.Context) error {
	w.logger.Info("Starting Docker container watcher")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	if err := w.scan(ctx); err != nil {
		w.logger.Warnf("Initial scan failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Docker watcher stopped")

			return ctx.Err()

		case <-w.stopCh:
			w.logger.Info("Docker watcher stopped")

			return nil

		case <-ticker.C:
			if err := w.scan(ctx); err != nil {
				w.logger.Warnf("Scan failed: %v", err)
			}
		}
	}
}

func (w *DockerWatcher) Stop() {
	close(w.stopCh)
}

func (w *DockerWatcher) scan(ctx context.Context) error {
	containers, err := w.listMCPContainers(ctx)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	seenServers := make(map[string]bool)

	for _, containerID := range containers {
		info, err := w.runtime.InspectContainer(containerID)
		if err != nil {
			w.logger.Warnf("Failed to inspect container %s: %v", containerID, err)

			continue
		}

		if info.Labels[MCPLabelRole] != "server" {
			continue
		}

		server := w.parseServerInfo(info)
		seenServers[server.Name] = true

		w.mu.Lock()
		existing, exists := w.servers[server.Name]

		if !exists {
			w.servers[server.Name] = server
			w.mu.Unlock()

			w.logger.Infof("Discovered new MCP server: %s (%s)", server.Name, server.Protocol)

			if w.onChange != nil {
				w.onChange(server, ChangeTypeAdded)
			}
		} else {
			if existing.Status != server.Status || existing.HealthStatus != server.HealthStatus {
				existing.Status = server.Status
				existing.HealthStatus = server.HealthStatus
				existing.LastSeen = server.LastSeen
				w.mu.Unlock()

				w.logger.Infof("Server status changed: %s (%s -> %s)",
					server.Name, existing.Status, server.Status)

				if w.onChange != nil {
					w.onChange(existing, ChangeTypeUpdated)
				}
			} else {
				existing.LastSeen = server.LastSeen
				w.mu.Unlock()
			}
		}
	}

	w.mu.Lock()
	for name, server := range w.servers {
		if !seenServers[name] && time.Since(server.LastSeen) > 30*time.Second {
			delete(w.servers, name)
			w.logger.Infof("Server removed: %s", name)

			if w.onChange != nil {
				w.onChange(server, ChangeTypeRemoved)
			}
		}
	}
	w.mu.Unlock()

	return nil
}

func (w *DockerWatcher) listMCPContainers(ctx context.Context) ([]string, error) {
	containerIDs := []string{}

	containers, err := w.runtime.ListContainers()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, container := range containers {
		if role, ok := container.Labels[MCPLabelRole]; ok && role == "server" {
			containerIDs = append(containerIDs, container.ID)
		}
	}

	return containerIDs, nil
}

func (w *DockerWatcher) parseServerInfo(info *container.ContainerInfo) *Server {
	name := info.Labels[MCPLabelName]
	if name == "" {
		name = strings.TrimPrefix(info.Name, "/")
	}

	protocol := info.Labels[MCPLabelProtocol]
	if protocol == "" {
		protocol = "stdio"
	}

	port := 0
	if portStr, ok := info.Labels[MCPLabelPort]; ok {
		fmt.Sscanf(portStr, "%d", &port)
	}

	healthStatus := "unknown"
	if info.State == "running" {
		healthStatus = "healthy"
	} else if info.State == "exited" || info.State == "dead" {
		healthStatus = "unhealthy"
	}

	return &Server{
		Name:         name,
		ContainerID:  info.ID,
		Protocol:     protocol,
		Port:         port,
		Status:       info.State,
		HealthStatus: healthStatus,
		Metadata:     info.Labels,
		LastSeen:     time.Now(),
		Registered:   false,
	}
}

func (w *DockerWatcher) GetServers() []*Server {
	w.mu.RLock()
	defer w.mu.RUnlock()

	servers := make([]*Server, 0, len(w.servers))
	for _, server := range w.servers {
		servers = append(servers, server)
	}

	return servers
}

func (w *DockerWatcher) GetServer(name string) (*Server, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	server, exists := w.servers[name]

	return server, exists
}

func (w *DockerWatcher) MarkRegistered(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if server, exists := w.servers[name]; exists {
		server.Registered = true
	}
}

func (w *DockerWatcher) MarkUnregistered(name string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if server, exists := w.servers[name]; exists {
		server.Registered = false
	}
}