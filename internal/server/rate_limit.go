package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/config"
	"github.com/phildougherty/mcp-compose/internal/logging"

	"golang.org/x/time/rate"
)

type RateLimiterConfig struct {
	PerIPRate        int
	PerIPBurst       int
	PerAPIKeyRate    int
	PerAPIKeyBurst   int
	PerOAuthRate     int
	PerOAuthBurst    int
	CleanupInterval  time.Duration
	MaxIdleTime      time.Duration
}

func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		PerIPRate:        100,
		PerIPBurst:       20,
		PerAPIKeyRate:    1000,
		PerAPIKeyBurst:   200,
		PerOAuthRate:     500,
		PerOAuthBurst:    100,
		CleanupInterval:  5 * time.Minute,
		MaxIdleTime:      1 * time.Hour,
	}
}

type rateLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	config       *RateLimiterConfig
	ipLimiters   sync.Map
	keyLimiters  sync.Map
	logger       *logging.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewRateLimiter(cfg *RateLimiterConfig, logger *logging.Logger) *RateLimiter {
	if cfg == nil {
		cfg = DefaultRateLimiterConfig()
	}

	if logger == nil {
		logger = logging.NewLogger("info")
	}

	ctx, cancel := context.WithCancel(context.Background())

	rl := &RateLimiter{
		config: cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	rl.wg.Add(1)
	go rl.cleanupRoutine()

	return rl
}

func (rl *RateLimiter) getIPLimiter(ip string) *rate.Limiter {
	limiterVal, exists := rl.ipLimiters.Load(ip)
	if exists {
		entry := limiterVal.(*rateLimiterEntry)
		entry.lastSeen = time.Now()

		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Limit(rl.config.PerIPRate)/60, rl.config.PerIPBurst)
	entry := &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	rl.ipLimiters.Store(ip, entry)

	return limiter
}

func (rl *RateLimiter) getKeyLimiter(key string, isOAuth bool) *rate.Limiter {
	limiterVal, exists := rl.keyLimiters.Load(key)
	if exists {
		entry := limiterVal.(*rateLimiterEntry)
		entry.lastSeen = time.Now()

		return entry.limiter
	}

	var limiter *rate.Limiter
	if isOAuth {
		limiter = rate.NewLimiter(rate.Limit(rl.config.PerOAuthRate)/60, rl.config.PerOAuthBurst)
	} else {
		limiter = rate.NewLimiter(rate.Limit(rl.config.PerAPIKeyRate)/60, rl.config.PerAPIKeyBurst)
	}

	entry := &rateLimiterEntry{
		limiter:  limiter,
		lastSeen: time.Now(),
	}
	rl.keyLimiters.Store(key, entry)

	return limiter
}

func (rl *RateLimiter) Allow(r *http.Request) (bool, time.Duration) {
	ip := getClientIPFromRequest(r)
	ipLimiter := rl.getIPLimiter(ip)

	if !ipLimiter.Allow() {
		reservation := ipLimiter.Reserve()
		if reservation.OK() {
			delay := reservation.Delay()
			reservation.Cancel()

			return false, delay
		}

		return false, time.Second
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != authHeader {
			isOAuth := strings.Contains(token, ".")
			keyLimiter := rl.getKeyLimiter(token, isOAuth)

			if !keyLimiter.Allow() {
				reservation := keyLimiter.Reserve()
				if reservation.OK() {
					delay := reservation.Delay()
					reservation.Cancel()

					return false, delay
				}

				return false, time.Second
			}
		}
	}

	return true, 0
}

func getClientIPFromRequest(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func (rl *RateLimiter) cleanupRoutine() {
	defer rl.wg.Done()

	ticker := time.NewTicker(rl.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.ctx.Done():
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

func (rl *RateLimiter) cleanup() {
	now := time.Now()
	cutoff := now.Add(-rl.config.MaxIdleTime)

	var ipCount, keyCount int

	rl.ipLimiters.Range(func(key, value interface{}) bool {
		entry := value.(*rateLimiterEntry)
		if entry.lastSeen.Before(cutoff) {
			rl.ipLimiters.Delete(key)
			ipCount++
		}

		return true
	})

	rl.keyLimiters.Range(func(key, value interface{}) bool {
		entry := value.(*rateLimiterEntry)
		if entry.lastSeen.Before(cutoff) {
			rl.keyLimiters.Delete(key)
			keyCount++
		}

		return true
	})

	if ipCount > 0 || keyCount > 0 {
		rl.logger.Debug("Rate limiter cleanup: removed %d IP limiters, %d key limiters", ipCount, keyCount)
	}
}

func (rl *RateLimiter) Shutdown() {
	if rl.cancel != nil {
		rl.cancel()
	}

	rl.wg.Wait()
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed, retryAfter := rl.Allow(r)

		if !allowed {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.PerIPRate))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(retryAfter).Unix(), 10))

			w.WriteHeader(http.StatusTooManyRequests)

			response := map[string]interface{}{
				"error": map[string]interface{}{
					"code":    429,
					"message": "Rate limit exceeded",
					"retry_after": map[string]interface{}{
						"seconds": int(retryAfter.Seconds()),
						"time":    time.Now().Add(retryAfter).Format(time.RFC3339),
					},
				},
			}

			_ = encodeJSON(w, response)

			ip := getClientIPFromRequest(r)
			rl.logger.Warning("Rate limit exceeded for IP %s on %s %s (retry after %v)",
				ip, r.Method, r.URL.Path, retryAfter)

			return
		}

		next.ServeHTTP(w, r)
	})
}

func LoadRateLimiterConfigFromCompose(cfg *config.ComposeConfig) *RateLimiterConfig {
	rlConfig := DefaultRateLimiterConfig()

	if cfg == nil || !cfg.RateLimit.Enabled {
		return rlConfig
	}

	if cfg.RateLimit.PerIPRate > 0 {
		rlConfig.PerIPRate = cfg.RateLimit.PerIPRate
	}

	if cfg.RateLimit.PerIPBurst > 0 {
		rlConfig.PerIPBurst = cfg.RateLimit.PerIPBurst
	}

	if cfg.RateLimit.PerAPIKeyRate > 0 {
		rlConfig.PerAPIKeyRate = cfg.RateLimit.PerAPIKeyRate
	}

	if cfg.RateLimit.PerAPIKeyBurst > 0 {
		rlConfig.PerAPIKeyBurst = cfg.RateLimit.PerAPIKeyBurst
	}

	if cfg.RateLimit.PerOAuthRate > 0 {
		rlConfig.PerOAuthRate = cfg.RateLimit.PerOAuthRate
	}

	if cfg.RateLimit.PerOAuthBurst > 0 {
		rlConfig.PerOAuthBurst = cfg.RateLimit.PerOAuthBurst
	}

	if cfg.RateLimit.CleanupInterval != "" {
		if d, err := time.ParseDuration(cfg.RateLimit.CleanupInterval); err == nil {
			rlConfig.CleanupInterval = d
		}
	}

	if cfg.RateLimit.MaxIdleTime != "" {
		if d, err := time.ParseDuration(cfg.RateLimit.MaxIdleTime); err == nil {
			rlConfig.MaxIdleTime = d
		}
	}

	return rlConfig
}