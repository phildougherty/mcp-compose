package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

func TestRateLimiter_PerIPLimit(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       10,
		PerIPBurst:      2,
		PerAPIKeyRate:   100,
		PerAPIKeyBurst:  20,
		CleanupInterval: 1 * time.Minute,
		MaxIdleTime:     1 * time.Hour,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	for i := 0; i < cfg.PerIPBurst; i++ {
		allowed, _ := rl.Allow(req)
		if !allowed {
			t.Errorf("Request %d should be allowed within burst", i)
		}
	}

	allowed, retryAfter := rl.Allow(req)
	if allowed {
		t.Error("Request should be rate limited after burst")
	}

	if retryAfter == 0 {
		t.Error("Retry-After should be set when rate limited")
	}
}

func TestRateLimiter_PerAPIKeyLimit(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       1000,
		PerIPBurst:      200,
		PerAPIKeyRate:   5,
		PerAPIKeyBurst:  2,
		CleanupInterval: 1 * time.Minute,
		MaxIdleTime:     1 * time.Hour,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("Authorization", "Bearer test-api-key-123")

	for i := 0; i < cfg.PerAPIKeyBurst; i++ {
		allowed, _ := rl.Allow(req)
		if !allowed {
			t.Errorf("Request %d should be allowed within burst", i)
		}
	}

	allowed, retryAfter := rl.Allow(req)
	if allowed {
		t.Error("Request should be rate limited after burst")
	}

	if retryAfter == 0 {
		t.Error("Retry-After should be set when rate limited")
	}
}

func TestRateLimiter_OAuthTokenLimit(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       1000,
		PerIPBurst:      200,
		PerOAuthRate:    5,
		PerOAuthBurst:   2,
		CleanupInterval: 1 * time.Minute,
		MaxIdleTime:     1 * time.Hour,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test.signature")

	for i := 0; i < cfg.PerOAuthBurst; i++ {
		allowed, _ := rl.Allow(req)
		if !allowed {
			t.Errorf("Request %d should be allowed within burst", i)
		}
	}

	allowed, retryAfter := rl.Allow(req)
	if allowed {
		t.Error("Request should be rate limited after burst")
	}

	if retryAfter == 0 {
		t.Error("Retry-After should be set when rate limited")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       10,
		PerIPBurst:      2,
		CleanupInterval: 1 * time.Minute,
		MaxIdleTime:     1 * time.Hour,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"

	for i := 0; i < cfg.PerIPBurst; i++ {
		allowed, _ := rl.Allow(req1)
		if !allowed {
			t.Errorf("IP1 request %d should be allowed", i)
		}
	}

	allowed, _ := rl.Allow(req1)
	if allowed {
		t.Error("IP1 should be rate limited")
	}

	allowed, _ = rl.Allow(req2)
	if !allowed {
		t.Error("IP2 should still be allowed")
	}
}

func TestRateLimiter_XForwardedFor(t *testing.T) {
	cfg := DefaultRateLimiterConfig()
	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")

	ip := getClientIPFromRequest(req)
	if ip != "10.0.0.1" {
		t.Errorf("Expected IP 10.0.0.1, got %s", ip)
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       10,
		PerIPBurst:      1,
		CleanupInterval: 1 * time.Minute,
		MaxIdleTime:     1 * time.Hour,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)
	if !handlerCalled {
		t.Error("Handler should be called for first request")
	}

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	handlerCalled = false
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w = httptest.NewRecorder()

	middleware.ServeHTTP(w, req)
	if handlerCalled {
		t.Error("Handler should not be called when rate limited")
	}

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("Retry-After header should be set")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := &RateLimiterConfig{
		PerIPRate:       100,
		PerIPBurst:      20,
		CleanupInterval: 100 * time.Millisecond,
		MaxIdleTime:     200 * time.Millisecond,
	}

	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	allowed, _ := rl.Allow(req)
	if !allowed {
		t.Error("First request should be allowed")
	}

	count := 0
	rl.ipLimiters.Range(func(key, value interface{}) bool {
		count++

		return true
	})

	if count != 1 {
		t.Errorf("Expected 1 limiter, got %d", count)
	}

	time.Sleep(400 * time.Millisecond)

	count = 0
	rl.ipLimiters.Range(func(key, value interface{}) bool {
		count++

		return true
	})

	if count != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", count)
	}
}

func BenchmarkRateLimiter_Allow(b *testing.B) {
	cfg := DefaultRateLimiterConfig()
	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(req)
	}
}

func BenchmarkRateLimiter_AllowWithAPIKey(b *testing.B) {
	cfg := DefaultRateLimiterConfig()
	logger := logging.NewLogger("error")
	rl := NewRateLimiter(cfg, logger)
	defer rl.Shutdown()

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("Authorization", "Bearer test-api-key-123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rl.Allow(req)
	}
}