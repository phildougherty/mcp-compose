// internal/auth/middleware.go
package auth

import (
	"net/http"
	"strings"
)

// APIKeyMiddleware is middleware that checks for a valid API key
type APIKeyMiddleware struct {
	APIKey string
	Next   http.Handler
}

// NewAPIKeyMiddleware creates a new APIKeyMiddleware
func NewAPIKeyMiddleware(apiKey string, next http.Handler) *APIKeyMiddleware {
	return &APIKeyMiddleware{
		APIKey: apiKey,
		Next:   next,
	}
}

// ServeHTTP implements the http.Handler interface
func (m *APIKeyMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip authentication for OPTIONS requests
	if r.Method == "OPTIONS" {
		m.Next.ServeHTTP(w, r)
		return
	}

	// Get the Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
		return
	}

	// Check if it's a Bearer token
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Invalid Authorization header format", http.StatusUnauthorized)
		return
	}

	// Extract the token
	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token != m.APIKey {
		http.Error(w, "Invalid API key", http.StatusForbidden)
		return
	}

	// Call the next handler
	m.Next.ServeHTTP(w, r)
}
