package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/constants"
)

// ConnectionManager manages enhanced SSE connection lifecycle with limits and backpressure
type ConnectionManager struct {
	handler                *ProxyHandler
	mu                     sync.RWMutex
	metrics                map[string]*ConnectionMetrics
	healthChan             chan HealthCheckResult
	globalConnectionLimit  int
	perServerLimit         int
	activeConnections      map[string]int
	totalActiveConnections int
	requestQueue           chan *QueuedRequest
	queueTimeout           time.Duration
	ctx                    context.Context
	cancel                 context.CancelFunc
	wg                     sync.WaitGroup
}

// QueuedRequest represents a queued request waiting for connection availability
type QueuedRequest struct {
	ServerName string
	Request    interface{}
	ResponseCh chan QueuedResponse
	Timeout    time.Time
}

// ConnectionMetrics tracks detailed connection performance
type ConnectionMetrics struct {
	ServerName          string
	ConnectionType      string
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	AverageResponseTime time.Duration
	LastResponseTime    time.Duration
	ConnectionUptime    time.Duration
	CreatedAt           time.Time
	LastUsed            time.Time
	ErrorRate           float64
	mu                  sync.RWMutex
}

// HealthCheckResult represents the result of a connection health check
type HealthCheckResult struct {
	ServerName   string
	Healthy      bool
	ResponseTime time.Duration
	Error        error
	CheckedAt    time.Time
}

// QueuedResponse represents the response to a queued request
type QueuedResponse struct {
	Response interface{}
	Error    error
}

// NewConnectionManager creates a new connection manager with limits
func NewConnectionManager(handler *ProxyHandler) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())

	cm := &ConnectionManager{
		handler:                handler,
		metrics:                make(map[string]*ConnectionMetrics),
		healthChan:             make(chan HealthCheckResult, constants.HealthCheckBufferSize),
		globalConnectionLimit:  1000,
		perServerLimit:         100,
		activeConnections:      make(map[string]int),
		totalActiveConnections: 0,
		requestQueue:           make(chan *QueuedRequest, 500),
		queueTimeout:           30 * time.Second,
		ctx:                    ctx,
		cancel:                 cancel,
	}

	cm.wg.Add(1)
	go cm.processRequestQueue()

	return cm
}

// RecordRequest records a request attempt and its outcome
func (cm *ConnectionManager) RecordRequest(serverName string, success bool, responseTime time.Duration) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	connMetrics, exists := cm.metrics[serverName]
	if !exists {
		connMetrics = &ConnectionMetrics{
			ServerName:     serverName,
			ConnectionType: "enhanced_sse",
			CreatedAt:      time.Now(),
		}
		cm.metrics[serverName] = connMetrics
	}

	connMetrics.mu.Lock()
	defer connMetrics.mu.Unlock()

	connMetrics.TotalRequests++
	connMetrics.LastUsed = time.Now()
	connMetrics.LastResponseTime = responseTime

	if success {
		connMetrics.SuccessfulRequests++
	} else {
		connMetrics.FailedRequests++
	}

	// Update average response time (simple moving average)
	if connMetrics.TotalRequests > 1 {
		connMetrics.AverageResponseTime = time.Duration(
			(int64(connMetrics.AverageResponseTime)*(connMetrics.TotalRequests-1) + int64(responseTime)) /
				connMetrics.TotalRequests,
		)
	} else {
		connMetrics.AverageResponseTime = responseTime
	}

	// Calculate error rate
	if connMetrics.TotalRequests > 0 {
		connMetrics.ErrorRate = float64(connMetrics.FailedRequests) / float64(connMetrics.TotalRequests)
	}

	// Update connection uptime
	connMetrics.ConnectionUptime = time.Since(connMetrics.CreatedAt)
}

// GetConnectionStats returns detailed connection statistics
func (cm *ConnectionManager) GetConnectionStats(serverName string) *ConnectionMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if metrics, exists := cm.metrics[serverName]; exists {
		metrics.mu.RLock()
		defer metrics.mu.RUnlock()

		// Return a copy to avoid race conditions

		return &ConnectionMetrics{
			ServerName:          metrics.ServerName,
			ConnectionType:      metrics.ConnectionType,
			TotalRequests:       metrics.TotalRequests,
			SuccessfulRequests:  metrics.SuccessfulRequests,
			FailedRequests:      metrics.FailedRequests,
			AverageResponseTime: metrics.AverageResponseTime,
			LastResponseTime:    metrics.LastResponseTime,
			ConnectionUptime:    metrics.ConnectionUptime,
			CreatedAt:           metrics.CreatedAt,
			LastUsed:            metrics.LastUsed,
			ErrorRate:           metrics.ErrorRate,
		}
	}

	return nil
}

// GetAllConnectionStats returns statistics for all connections
func (cm *ConnectionManager) GetAllConnectionStats() map[string]*ConnectionMetrics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	result := make(map[string]*ConnectionMetrics)
	for serverName := range cm.metrics {
		result[serverName] = cm.GetConnectionStats(serverName)
	}

	return result
}

// HealthCheck performs a health check on a specific connection
func (cm *ConnectionManager) HealthCheck(serverName string) HealthCheckResult {
	start := time.Now()

	// Create a simple ping request to test the connection
	pingRequest := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("health-check-%d", start.Unix()),
		"method":  "ping",
	}

	_, err := cm.handler.sendOptimalSSERequest(serverName, pingRequest)
	responseTime := time.Since(start)

	result := HealthCheckResult{
		ServerName:   serverName,
		Healthy:      err == nil,
		ResponseTime: responseTime,
		Error:        err,
		CheckedAt:    time.Now(),
	}

	// Send result to health check channel
	select {
	case cm.healthChan <- result:
	default:
		// Channel is full, skip this result
	}

	return result
}

// PerformHealthChecks runs health checks on all active connections
func (cm *ConnectionManager) PerformHealthChecks() {
	cm.handler.EnhancedSSEMutex.RLock()
	serverNames := make([]string, 0, len(cm.handler.EnhancedSSEConnections))
	for serverName := range cm.handler.EnhancedSSEConnections {
		serverNames = append(serverNames, serverName)
	}
	cm.handler.EnhancedSSEMutex.RUnlock()

	// Run health checks concurrently
	var wg sync.WaitGroup
	for _, serverName := range serverNames {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			cm.HealthCheck(name)
		}(serverName)
	}
	wg.Wait()
}

// CleanupStaleConnections removes connections that haven't been used recently
func (cm *ConnectionManager) CleanupStaleConnections(maxIdleTime time.Duration) int {
	cleanedCount := 0

	cm.handler.EnhancedSSEMutex.Lock()
	defer cm.handler.EnhancedSSEMutex.Unlock()

	for serverName, conn := range cm.handler.EnhancedSSEConnections {
		if conn == nil {
			continue
		}

		conn.mu.RLock()
		timeSinceLastUsed := time.Since(conn.LastUsed)
		conn.mu.RUnlock()

		if timeSinceLastUsed > maxIdleTime {
			cm.handler.logger.Info("Cleaning up stale enhanced SSE connection to %s (idle for %v)",
				serverName, timeSinceLastUsed)

			cm.handler.closeEnhancedSSEConnection(conn)
			delete(cm.handler.EnhancedSSEConnections, serverName)

			// Remove from metrics as well
			cm.mu.Lock()
			delete(cm.metrics, serverName)
			cm.mu.Unlock()

			cleanedCount++
		}
	}

	return cleanedCount
}

// GetHealthCheckResults returns recent health check results
func (cm *ConnectionManager) GetHealthCheckResults(maxResults int) []HealthCheckResult {
	results := make([]HealthCheckResult, 0, maxResults)

	// Drain the channel up to maxResults
	for i := 0; i < maxResults; i++ {
		select {
		case result := <-cm.healthChan:
			results = append(results, result)
		default:

			return results
		}
	}

	return results
}

// GetConnectionSummary returns a summary of all connection statuses
func (cm *ConnectionManager) GetConnectionSummary() map[string]interface{} {
	allStats := cm.GetAllConnectionStats()

	summary := map[string]interface{}{
		"total_connections": len(allStats),
		"connections":       make(map[string]interface{}),
		"overall_stats": map[string]interface{}{
			"total_requests":     int64(0),
			"total_errors":       int64(0),
			"average_uptime":     time.Duration(0),
			"average_error_rate": 0.0,
		},
	}

	var totalRequests, totalErrors int64
	var totalUptime time.Duration
	var totalErrorRate float64

	for serverName, stats := range allStats {
		summary["connections"].(map[string]interface{})[serverName] = map[string]interface{}{
			"total_requests":        stats.TotalRequests,
			"successful_requests":   stats.SuccessfulRequests,
			"failed_requests":       stats.FailedRequests,
			"error_rate":            stats.ErrorRate,
			"average_response_time": stats.AverageResponseTime.String(),
			"last_response_time":    stats.LastResponseTime.String(),
			"connection_uptime":     stats.ConnectionUptime.String(),
			"last_used":             stats.LastUsed.Format(time.RFC3339),
		}

		totalRequests += stats.TotalRequests
		totalErrors += stats.FailedRequests
		totalUptime += stats.ConnectionUptime
		totalErrorRate += stats.ErrorRate
	}

	if len(allStats) > 0 {
		summary["overall_stats"].(map[string]interface{})["total_requests"] = totalRequests
		summary["overall_stats"].(map[string]interface{})["total_errors"] = totalErrors
		summary["overall_stats"].(map[string]interface{})["average_uptime"] = (totalUptime / time.Duration(len(allStats))).String()
		summary["overall_stats"].(map[string]interface{})["average_error_rate"] = totalErrorRate / float64(len(allStats))
	}

	return summary
}

// AcquireConnection attempts to acquire a connection slot for a server
func (cm *ConnectionManager) AcquireConnection(serverName string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.totalActiveConnections >= cm.globalConnectionLimit {

		return fmt.Errorf("global connection limit reached (%d)", cm.globalConnectionLimit)
	}

	serverConns := cm.activeConnections[serverName]
	if serverConns >= cm.perServerLimit {

		return fmt.Errorf("per-server connection limit reached for %s (%d)", serverName, cm.perServerLimit)
	}

	cm.activeConnections[serverName]++
	cm.totalActiveConnections++

	return nil
}

// ReleaseConnection releases a connection slot for a server
func (cm *ConnectionManager) ReleaseConnection(serverName string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.activeConnections[serverName] > 0 {
		cm.activeConnections[serverName]--
		cm.totalActiveConnections--
	}
}

// QueueRequest queues a request if connection limits are reached
func (cm *ConnectionManager) QueueRequest(serverName string, request interface{}, timeout time.Duration) (interface{}, error) {
	responseCh := make(chan QueuedResponse, 1)

	queuedReq := &QueuedRequest{
		ServerName: serverName,
		Request:    request,
		ResponseCh: responseCh,
		Timeout:    time.Now().Add(timeout),
	}

	select {
	case cm.requestQueue <- queuedReq:
	case <-time.After(5 * time.Second):

		return nil, fmt.Errorf("request queue full for server %s", serverName)
	}

	select {
	case resp := <-responseCh:

		return resp.Response, resp.Error
	case <-time.After(timeout):

		return nil, fmt.Errorf("queued request timed out for server %s", serverName)
	}
}

// processRequestQueue processes queued requests when connections become available
func (cm *ConnectionManager) processRequestQueue() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.ctx.Done():
			cm.handler.logger.Info("Request queue processor stopping")

			return
		case req := <-cm.requestQueue:
			if time.Now().After(req.Timeout) {
				req.ResponseCh <- QueuedResponse{
					Error: fmt.Errorf("request expired in queue"),
				}

				continue
			}

			if err := cm.AcquireConnection(req.ServerName); err != nil {
				time.Sleep(100 * time.Millisecond)

				select {
				case cm.requestQueue <- req:
				default:
					req.ResponseCh <- QueuedResponse{
						Error: fmt.Errorf("failed to requeue request: %w", err),
					}
				}

				continue
			}

			go func(qr *QueuedRequest) {
				defer cm.ReleaseConnection(qr.ServerName)

				response, err := cm.handler.sendOptimalSSERequest(qr.ServerName, qr.Request.(map[string]interface{}))
				qr.ResponseCh <- QueuedResponse{
					Response: response,
					Error:    err,
				}
			}(req)
		}
	}
}

// GetConnectionLimits returns current connection limits and usage
func (cm *ConnectionManager) GetConnectionLimits() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	perServerStats := make(map[string]int)
	for server, count := range cm.activeConnections {
		perServerStats[server] = count
	}

	return map[string]interface{}{
		"global_limit":          cm.globalConnectionLimit,
		"per_server_limit":      cm.perServerLimit,
		"total_active":          cm.totalActiveConnections,
		"per_server_active":     perServerStats,
		"queue_size":            len(cm.requestQueue),
		"queue_capacity":        cap(cm.requestQueue),
		"global_utilization_pct": float64(cm.totalActiveConnections) / float64(cm.globalConnectionLimit) * 100,
	}
}

// SetConnectionLimits updates connection limits
func (cm *ConnectionManager) SetConnectionLimits(globalLimit, perServerLimit int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.globalConnectionLimit = globalLimit
	cm.perServerLimit = perServerLimit
	cm.handler.logger.Info("Connection limits updated: global=%d, per-server=%d", globalLimit, perServerLimit)
}

// StartMonitoring starts background monitoring of connections
func (cm *ConnectionManager) StartMonitoring(interval time.Duration) {
	cm.wg.Add(1)
	go func() {
		defer cm.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cm.PerformHealthChecks()

				cleanedCount := cm.CleanupStaleConnections(constants.StaleConnectionThreshold)
				if cleanedCount > 0 {
					cm.handler.logger.Info("Cleaned up %d stale enhanced SSE connections", cleanedCount)
				}

			case <-cm.ctx.Done():

				return
			}
		}
	}()
}

// Shutdown gracefully shuts down the connection manager
func (cm *ConnectionManager) Shutdown() error {
	cm.handler.logger.Info("Shutting down connection manager")

	if cm.cancel != nil {
		cm.cancel()
	}

	close(cm.requestQueue)

	cm.wg.Wait()

	cm.handler.logger.Info("Connection manager shutdown complete")

	return nil
}
