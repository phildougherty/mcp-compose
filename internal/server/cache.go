package server

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/phildougherty/mcp-compose/internal/logging"
)

// ResponseCache manages caching of MCP responses
type ResponseCache struct {
	cache           *cache.Cache
	logger          *logging.Logger
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	maxSize         int
	hits            int64
	misses          int64
	evictions       int64
	mu              sync.RWMutex
	metricsCallback func(hits, misses, evictions int64, size int)
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	DefaultTTL      time.Duration
	CleanupInterval time.Duration
	MaxSize         int
}

// DefaultCacheConfig returns sensible defaults for response caching
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		DefaultTTL:      5 * time.Minute,
		CleanupInterval: 10 * time.Minute,
		MaxSize:         1000,
	}
}

// CacheEntry represents a cached response with metadata
type CacheEntry struct {
	Response   interface{}
	CachedAt   time.Time
	ServerName string
	Method     string
	Size       int
}

// NewResponseCache creates a new response cache
func NewResponseCache(cfg CacheConfig, logger *logging.Logger) *ResponseCache {
	rc := &ResponseCache{
		cache:           cache.New(cfg.DefaultTTL, cfg.CleanupInterval),
		logger:          logger,
		defaultTTL:      cfg.DefaultTTL,
		cleanupInterval: cfg.CleanupInterval,
		maxSize:         cfg.MaxSize,
	}

	rc.cache.OnEvicted(func(key string, value interface{}) {
		rc.mu.Lock()
		rc.evictions++
		rc.mu.Unlock()
		rc.logger.Debug("Cache entry evicted: %s", key)
		rc.updateMetrics()
	})

	return rc
}

// SetMetricsCallback sets a callback for cache metrics updates
func (rc *ResponseCache) SetMetricsCallback(callback func(hits, misses, evictions int64, size int)) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.metricsCallback = callback
}

// Get retrieves a response from cache
func (rc *ResponseCache) Get(serverName, method string, params interface{}) (interface{}, bool) {
	key := rc.generateKey(serverName, method, params)

	entry, found := rc.cache.Get(key)
	if !found {
		rc.mu.Lock()
		rc.misses++
		rc.mu.Unlock()
		rc.updateMetrics()

		return nil, false
	}

	rc.mu.Lock()
	rc.hits++
	rc.mu.Unlock()
	rc.updateMetrics()

	if cacheEntry, ok := entry.(*CacheEntry); ok {
		rc.logger.Debug("Cache hit for %s:%s (cached %v ago)", serverName, method, time.Since(cacheEntry.CachedAt))

		return cacheEntry.Response, true
	}

	return nil, false
}

// Set stores a response in cache
func (rc *ResponseCache) Set(serverName, method string, params interface{}, response interface{}, ttl time.Duration) error {
	if rc.shouldCache(method) {
		key := rc.generateKey(serverName, method, params)

		responseSize := 0
		if data, err := json.Marshal(response); err == nil {
			responseSize = len(data)
		}

		entry := &CacheEntry{
			Response:   response,
			CachedAt:   time.Now(),
			ServerName: serverName,
			Method:     method,
			Size:       responseSize,
		}

		if rc.cache.ItemCount() >= rc.maxSize {
			rc.evictOldestEntry()
		}

		if ttl == 0 {
			ttl = rc.defaultTTL
		}

		rc.cache.Set(key, entry, ttl)
		rc.logger.Debug("Cached response for %s:%s (size: %d bytes, ttl: %v)", serverName, method, responseSize, ttl)
		rc.updateMetrics()

		return nil
	}

	return nil
}

// Invalidate removes cached entries for a server
func (rc *ResponseCache) Invalidate(serverName string) int {
	count := 0
	for key, item := range rc.cache.Items() {
		if entry, ok := item.Object.(*CacheEntry); ok {
			if entry.ServerName == serverName {
				rc.cache.Delete(key)
				count++
			}
		}
	}

	if count > 0 {
		rc.logger.Info("Invalidated %d cache entries for server %s", count, serverName)
		rc.updateMetrics()
	}

	return count
}

// InvalidateMethod removes cached entries for a specific method
func (rc *ResponseCache) InvalidateMethod(serverName, method string) int {
	count := 0
	for key, item := range rc.cache.Items() {
		if entry, ok := item.Object.(*CacheEntry); ok {
			if entry.ServerName == serverName && entry.Method == method {
				rc.cache.Delete(key)
				count++
			}
		}
	}

	if count > 0 {
		rc.logger.Debug("Invalidated %d cache entries for %s:%s", count, serverName, method)
		rc.updateMetrics()
	}

	return count
}

// Clear removes all cached entries
func (rc *ResponseCache) Clear() {
	rc.cache.Flush()
	rc.logger.Info("Cache cleared")
	rc.updateMetrics()
}

// shouldCache determines if a method should be cached
func (rc *ResponseCache) shouldCache(method string) bool {
	cacheableMethods := map[string]bool{
		"tools/list":      true,
		"resources/list":  true,
		"prompts/list":    true,
		"resources/read":  false,
		"tools/call":      false,
		"prompts/get":     false,
		"sampling/create": false,
		"ping":            false,
	}

	cacheable, exists := cacheableMethods[method]

	return exists && cacheable
}

// generateKey creates a cache key from request parameters
func (rc *ResponseCache) generateKey(serverName, method string, params interface{}) string {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	data := fmt.Sprintf("%s:%s:%s", serverName, method, string(paramsJSON))
	hash := sha256.Sum256([]byte(data))

	return fmt.Sprintf("%x", hash[:16])
}

// evictOldestEntry removes the oldest cache entry
func (rc *ResponseCache) evictOldestEntry() {
	var oldestKey string
	var oldestTime time.Time

	for key, item := range rc.cache.Items() {
		if entry, ok := item.Object.(*CacheEntry); ok {
			if oldestKey == "" || entry.CachedAt.Before(oldestTime) {
				oldestKey = key
				oldestTime = entry.CachedAt
			}
		}
	}

	if oldestKey != "" {
		rc.cache.Delete(oldestKey)
		rc.logger.Debug("Evicted oldest cache entry due to size limit")
	}
}

// Stats returns cache statistics
func (rc *ResponseCache) Stats() CacheStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	hitRate := 0.0
	total := rc.hits + rc.misses
	if total > 0 {
		hitRate = float64(rc.hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:       rc.hits,
		Misses:     rc.misses,
		Evictions:  rc.evictions,
		Size:       rc.cache.ItemCount(),
		MaxSize:    rc.maxSize,
		HitRate:    hitRate,
		TotalBytes: rc.calculateTotalSize(),
	}
}

// calculateTotalSize calculates total size of cached data
func (rc *ResponseCache) calculateTotalSize() int64 {
	var totalSize int64
	for _, item := range rc.cache.Items() {
		if entry, ok := item.Object.(*CacheEntry); ok {
			totalSize += int64(entry.Size)
		}
	}

	return totalSize
}

// updateMetrics notifies the metrics callback of cache changes
func (rc *ResponseCache) updateMetrics() {
	rc.mu.RLock()
	callback := rc.metricsCallback
	hits := rc.hits
	misses := rc.misses
	evictions := rc.evictions
	rc.mu.RUnlock()

	if callback != nil {
		callback(hits, misses, evictions, rc.cache.ItemCount())
	}
}

// CacheStats contains cache statistics
type CacheStats struct {
	Hits       int64
	Misses     int64
	Evictions  int64
	Size       int
	MaxSize    int
	HitRate    float64
	TotalBytes int64
}

// GetTTL returns the TTL for a specific method
func (rc *ResponseCache) GetTTL(method string) time.Duration {
	methodTTLs := map[string]time.Duration{
		"tools/list":     10 * time.Minute,
		"resources/list": 5 * time.Minute,
		"prompts/list":   10 * time.Minute,
	}

	if ttl, exists := methodTTLs[method]; exists {

		return ttl
	}

	return rc.defaultTTL
}