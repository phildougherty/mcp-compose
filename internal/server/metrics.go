package server

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsCollector manages Prometheus metrics for MCP proxy
type MetricsCollector struct {
	requestTotal            *prometheus.CounterVec
	requestDuration         *prometheus.HistogramVec
	activeConnections       *prometheus.GaugeVec
	connectionPoolSize      *prometheus.GaugeVec
	connectionPoolActive    *prometheus.GaugeVec
	connectionPoolIdle      *prometheus.GaugeVec
	rateLimitExceeded       *prometheus.CounterVec
	cacheHits               prometheus.Counter
	cacheMisses             prometheus.Counter
	cacheEvictions          prometheus.Counter
	cacheSize               prometheus.Gauge
	uptime                  prometheus.Gauge
	goroutineCount          prometheus.Gauge
	memoryAllocated         prometheus.Gauge
	memoryTotal             prometheus.Gauge
	healthCheckTotal        *prometheus.CounterVec
	healthCheckDuration     *prometheus.HistogramVec
	connectionErrorsTotal   *prometheus.CounterVec
	responseSize            *prometheus.HistogramVec
	requestSize             *prometheus.HistogramVec
	registry                *prometheus.Registry
	startTime               time.Time
}

// NewMetricsCollector creates a new Prometheus metrics collector
func NewMetricsCollector() *MetricsCollector {
	mc := &MetricsCollector{
		registry:  prometheus.NewRegistry(),
		startTime: time.Now(),
	}

	mc.requestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_requests_total",
			Help: "Total number of MCP proxy requests by method and status",
		},
		[]string{"server", "method", "status"},
	)

	mc.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_request_duration_seconds",
			Help:    "Duration of MCP proxy requests in seconds",
			Buckets: []float64{0.001, 0.005, 0.010, 0.025, 0.050, 0.100, 0.250, 0.500, 1.000, 2.500, 5.000, 10.000},
		},
		[]string{"server", "method"},
	)

	mc.activeConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_active_connections",
			Help: "Number of active connections by server and protocol",
		},
		[]string{"server", "protocol"},
	)

	mc.connectionPoolSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_connection_pool_size",
			Help: "Maximum size of connection pool by server",
		},
		[]string{"server"},
	)

	mc.connectionPoolActive = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_connection_pool_active",
			Help: "Number of active connections in pool by server",
		},
		[]string{"server"},
	)

	mc.connectionPoolIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_connection_pool_idle",
			Help: "Number of idle connections in pool by server",
		},
		[]string{"server"},
	)

	mc.rateLimitExceeded = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_rate_limit_exceeded_total",
			Help: "Total number of rate limit exceeded errors by server",
		},
		[]string{"server"},
	)

	mc.cacheHits = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mcp_proxy_cache_hits_total",
			Help: "Total number of cache hits",
		},
	)

	mc.cacheMisses = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mcp_proxy_cache_misses_total",
			Help: "Total number of cache misses",
		},
	)

	mc.cacheEvictions = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "mcp_proxy_cache_evictions_total",
			Help: "Total number of cache evictions",
		},
	)

	mc.cacheSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_cache_size",
			Help: "Current number of items in cache",
		},
	)

	mc.uptime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_uptime_seconds",
			Help: "Time since proxy started in seconds",
		},
	)

	mc.goroutineCount = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_goroutines",
			Help: "Current number of goroutines",
		},
	)

	mc.memoryAllocated = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_memory_allocated_bytes",
			Help: "Current memory allocated in bytes",
		},
	)

	mc.memoryTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "mcp_proxy_memory_total_bytes",
			Help: "Total memory allocated (cumulative) in bytes",
		},
	)

	mc.healthCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_health_checks_total",
			Help: "Total number of health checks by server and result",
		},
		[]string{"server", "result"},
	)

	mc.healthCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_health_check_duration_seconds",
			Help:    "Duration of health checks in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0},
		},
		[]string{"server"},
	)

	mc.connectionErrorsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_proxy_connection_errors_total",
			Help: "Total number of connection errors by server and error type",
		},
		[]string{"server", "error_type"},
	)

	mc.responseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_response_size_bytes",
			Help:    "Size of response payloads in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000, 10000000},
		},
		[]string{"server", "method"},
	)

	mc.requestSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_proxy_request_size_bytes",
			Help:    "Size of request payloads in bytes",
			Buckets: []float64{100, 1000, 10000, 100000, 1000000},
		},
		[]string{"server", "method"},
	)

	mc.registry.MustRegister(
		mc.requestTotal,
		mc.requestDuration,
		mc.activeConnections,
		mc.connectionPoolSize,
		mc.connectionPoolActive,
		mc.connectionPoolIdle,
		mc.rateLimitExceeded,
		mc.cacheHits,
		mc.cacheMisses,
		mc.cacheEvictions,
		mc.cacheSize,
		mc.uptime,
		mc.goroutineCount,
		mc.memoryAllocated,
		mc.memoryTotal,
		mc.healthCheckTotal,
		mc.healthCheckDuration,
		mc.connectionErrorsTotal,
		mc.responseSize,
		mc.requestSize,
	)

	return mc
}

// RecordRequest records metrics for a request
func (mc *MetricsCollector) RecordRequest(server, method, status string, duration time.Duration, reqSize, respSize int) {
	mc.requestTotal.WithLabelValues(server, method, status).Inc()
	mc.requestDuration.WithLabelValues(server, method).Observe(duration.Seconds())

	if reqSize > 0 {
		mc.requestSize.WithLabelValues(server, method).Observe(float64(reqSize))
	}
	if respSize > 0 {
		mc.responseSize.WithLabelValues(server, method).Observe(float64(respSize))
	}
}

// RecordConnectionError records a connection error
func (mc *MetricsCollector) RecordConnectionError(server, errorType string) {
	mc.connectionErrorsTotal.WithLabelValues(server, errorType).Inc()
}

// RecordHealthCheck records a health check result
func (mc *MetricsCollector) RecordHealthCheck(server, result string, duration time.Duration) {
	mc.healthCheckTotal.WithLabelValues(server, result).Inc()
	mc.healthCheckDuration.WithLabelValues(server).Observe(duration.Seconds())
}

// RecordRateLimitExceeded records a rate limit exceeded event
func (mc *MetricsCollector) RecordRateLimitExceeded(server string) {
	mc.rateLimitExceeded.WithLabelValues(server).Inc()
}

// SetActiveConnections sets the number of active connections
func (mc *MetricsCollector) SetActiveConnections(server, protocol string, count int) {
	mc.activeConnections.WithLabelValues(server, protocol).Set(float64(count))
}

// UpdateConnectionPool updates connection pool metrics
func (mc *MetricsCollector) UpdateConnectionPool(server string, maxSize, active, idle int) {
	mc.connectionPoolSize.WithLabelValues(server).Set(float64(maxSize))
	mc.connectionPoolActive.WithLabelValues(server).Set(float64(active))
	mc.connectionPoolIdle.WithLabelValues(server).Set(float64(idle))
}

// RecordCacheHit records a cache hit
func (mc *MetricsCollector) RecordCacheHit() {
	mc.cacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (mc *MetricsCollector) RecordCacheMiss() {
	mc.cacheMisses.Inc()
}

// RecordCacheEviction records a cache eviction
func (mc *MetricsCollector) RecordCacheEviction() {
	mc.cacheEvictions.Inc()
}

// SetCacheSize sets the current cache size
func (mc *MetricsCollector) SetCacheSize(size int) {
	mc.cacheSize.Set(float64(size))
}

// UpdateSystemMetrics updates system-level metrics
func (mc *MetricsCollector) UpdateSystemMetrics(goroutines int, memAlloc, memTotal uint64) {
	mc.uptime.Set(time.Since(mc.startTime).Seconds())
	mc.goroutineCount.Set(float64(goroutines))
	mc.memoryAllocated.Set(float64(memAlloc))
	mc.memoryTotal.Set(float64(memTotal))
}

// Handler returns an HTTP handler for the metrics endpoint
func (mc *MetricsCollector) Handler() http.Handler {
	return promhttp.HandlerFor(mc.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// MetricsMiddleware wraps an HTTP handler to record metrics
func (mc *MetricsCollector) MetricsMiddleware(serverName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(recorder, r)

			duration := time.Since(start)
			status := "success"
			if recorder.statusCode >= 400 {
				status = "error"
			}

			mc.RecordRequest(serverName, r.Method, status, duration, int(r.ContentLength), recorder.bytesWritten)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode    int
	bytesWritten  int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytesWritten += n

	return n, err
}