package server

import (
	"fmt"
	"runtime"
	"time"
)

// InitializePerformanceFeatures adds performance features to the proxy handler
func (h *ProxyHandler) InitializePerformanceFeatures() {
	h.logger.Info("Initializing performance features")

	if h.metrics == nil {
		h.metrics = NewMetricsCollector()
		h.logger.Info("Metrics collector initialized")
	}

	if h.responseCache == nil {
		h.responseCache = NewResponseCache(DefaultCacheConfig(), h.logger)
		h.responseCache.SetMetricsCallback(func(hits, misses, evictions int64, size int) {
			h.metrics.SetCacheSize(size)
		})
		h.logger.Info("Response cache initialized")
	}

	if h.profiling == nil {
		h.profiling = NewProfilingServer(false, h.APIKey, h.logger)
	}

	if h.connectionPools == nil {
		h.connectionPools = make(map[string]*ConnectionPool)
		h.logger.Info("Connection pools initialized")
	}

	go h.startMetricsUpdateWorker()
	h.logger.Info("Performance features initialized successfully")
}

// startMetricsUpdateWorker periodically updates system metrics
func (h *ProxyHandler) startMetricsUpdateWorker() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-h.ctx.Done():

				return
			case <-ticker.C:
				h.updateSystemMetrics()
			}
		}
	}()
}

// updateSystemMetrics collects and updates system metrics
func (h *ProxyHandler) updateSystemMetrics() {
	if h.metrics == nil {

		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	h.metrics.UpdateSystemMetrics(
		runtime.NumGoroutine(),
		m.Alloc,
		m.TotalAlloc,
	)

	if h.connectionPools != nil {
		h.poolMutex.RLock()
		for serverName, pool := range h.connectionPools {
			stats := pool.Stats()
			h.metrics.UpdateConnectionPool(
				serverName,
				stats.MaxConnections,
				stats.ActiveCount,
				stats.IdleCount,
			)
		}
		h.poolMutex.RUnlock()
	}

	if h.responseCache != nil {
		cacheStats := h.responseCache.Stats()
		h.metrics.SetCacheSize(cacheStats.Size)
	}
}

// GetConnectionPool retrieves or creates a connection pool for a server
func (h *ProxyHandler) GetConnectionPool(serverName string) (*ConnectionPool, error) {
	if h.connectionPools == nil {
		h.poolMutex.Lock()
		h.connectionPools = make(map[string]*ConnectionPool)
		h.poolMutex.Unlock()
	}

	h.poolMutex.RLock()
	pool, exists := h.connectionPools[serverName]
	h.poolMutex.RUnlock()

	if exists {

		return pool, nil
	}

	h.poolMutex.Lock()
	defer h.poolMutex.Unlock()

	pool, exists = h.connectionPools[serverName]
	if exists {

		return pool, nil
	}

	instance, ok := h.Manager.GetServerInstance(serverName)
	if !ok {

		return nil, fmt.Errorf("server %s not found", serverName)
	}

	pool = NewConnectionPool(serverName, instance.Config, h, DefaultPoolConfig())
	h.connectionPools[serverName] = pool
	h.logger.Info("Created connection pool for server %s", serverName)

	return pool, nil
}

// CloseConnectionPools closes all connection pools
func (h *ProxyHandler) CloseConnectionPools() {
	if h.connectionPools == nil {

		return
	}

	h.poolMutex.Lock()
	defer h.poolMutex.Unlock()

	for name, pool := range h.connectionPools {
		h.logger.Debug("Closing connection pool for %s", name)
		if err := pool.Close(); err != nil {
			h.logger.Warning("Error closing connection pool for %s: %v", name, err)
		}
	}
	h.connectionPools = make(map[string]*ConnectionPool)
}