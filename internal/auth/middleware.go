// internal/auth/middleware.go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AuthContext keys
type contextKey string

const (
	ClientContextKey   contextKey = "oauth_client"
	TokenContextKey    contextKey = "oauth_token"
	UserContextKey     contextKey = "oauth_user"
	ScopeContextKey    contextKey = "oauth_scope"
	AuthTypeContextKey contextKey = "auth_type"
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

	// Add auth type to context
	ctx := context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
	r = r.WithContext(ctx)

	// Call the next handler
	m.Next.ServeHTTP(w, r)
}

// AuthenticationMiddleware validates OAuth tokens and API keys
type AuthenticationMiddleware struct {
	server *AuthorizationServer
	apiKey string
}

// NewAuthenticationMiddleware creates a new authentication middleware
func NewAuthenticationMiddleware(server *AuthorizationServer) *AuthenticationMiddleware {

	return &AuthenticationMiddleware{
		server: server,
	}
}

// SetAPIKey sets the API key for fallback authentication
func (m *AuthenticationMiddleware) SetAPIKey(apiKey string) {
	m.apiKey = apiKey
}

// RequireAuthentication middleware that requires valid OAuth token or API key
func (m *AuthenticationMiddleware) RequireAuthentication(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for OPTIONS requests
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)

			return
		}

		token := m.extractToken(r)
		if token == "" {
			m.sendUnauthorized(w, "missing_token", "Access token required")

			return
		}

		// Try OAuth authentication first
		accessToken, oauthErr := m.validateOAuthToken(token)
		if oauthErr == nil && accessToken != nil {
			// OAuth authentication successful
			client, exists := m.server.GetClient(accessToken.ClientID)
			if !exists {
				m.sendUnauthorized(w, "invalid_token", "Invalid client")

				return
			}

			// Add OAuth context
			ctx := context.WithValue(r.Context(), ClientContextKey, client)
			ctx = context.WithValue(ctx, TokenContextKey, accessToken)
			ctx = context.WithValue(ctx, UserContextKey, accessToken.UserID)
			ctx = context.WithValue(ctx, ScopeContextKey, accessToken.Scope)
			ctx = context.WithValue(ctx, AuthTypeContextKey, "oauth")

			next.ServeHTTP(w, r.WithContext(ctx))

			return
		}

		// Fallback to API key authentication if configured
		if m.apiKey != "" && token == m.apiKey {
			// API key authentication successful
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
			next.ServeHTTP(w, r.WithContext(ctx))

			return
		}

		// Both authentication methods failed
		if oauthErr != nil {
			m.sendUnauthorized(w, "invalid_token", oauthErr.Error())
		} else {
			m.sendUnauthorized(w, "invalid_token", "Invalid access token or API key")
		}
	})
}

// RequireScope middleware that requires specific OAuth scope
func (m *AuthenticationMiddleware) RequireScope(requiredScope string) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check auth type
			authType, ok := r.Context().Value(AuthTypeContextKey).(string)
			if !ok {
				m.sendForbidden(w, "insufficient_scope", "No authentication information")

				return
			}

			// API key authentication bypasses scope checks (for backward compatibility)
			if authType == "api_key" {
				next.ServeHTTP(w, r)

				return
			}

			// OAuth authentication requires scope validation
			tokenScope, ok := r.Context().Value(ScopeContextKey).(string)
			if !ok {
				m.sendForbidden(w, "insufficient_scope", "No scope information")

				return
			}

			if !m.hasScope(tokenScope, requiredScope) {
				m.sendForbidden(w, "insufficient_scope", "Required scope not granted")

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuthentication middleware that optionally validates OAuth token or API key
func (m *AuthenticationMiddleware) OptionalAuthentication(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := m.extractToken(r)
		if token == "" {
			next.ServeHTTP(w, r)

			return
		}

		// Try OAuth authentication first
		accessToken, err := m.validateOAuthToken(token)
		if err == nil && accessToken != nil {
			client, exists := m.server.GetClient(accessToken.ClientID)
			if exists {
				// Add OAuth context
				ctx := context.WithValue(r.Context(), ClientContextKey, client)
				ctx = context.WithValue(ctx, TokenContextKey, accessToken)
				ctx = context.WithValue(ctx, UserContextKey, accessToken.UserID)
				ctx = context.WithValue(ctx, ScopeContextKey, accessToken.Scope)
				ctx = context.WithValue(ctx, AuthTypeContextKey, "oauth")

				next.ServeHTTP(w, r.WithContext(ctx))

				return
			}
		}

		// Try API key authentication
		if m.apiKey != "" && token == m.apiKey {
			ctx := context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
			next.ServeHTTP(w, r.WithContext(ctx))

			return
		}

		// Continue without authentication context
		next.ServeHTTP(w, r)
	})
}

// RequireAPIKey middleware that only accepts API key authentication (legacy)
func (m *AuthenticationMiddleware) RequireAPIKey(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.apiKey == "" {
			next.ServeHTTP(w, r)

			return
		}

		token := m.extractToken(r)
		if token == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)

			return
		}

		if token != m.apiKey {
			http.Error(w, "Invalid API key", http.StatusForbidden)

			return
		}

		ctx := context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// FlexibleAuthentication middleware that accepts either OAuth or API key with preference
func (m *AuthenticationMiddleware) FlexibleAuthentication(preferOAuth bool) func(http.Handler) http.Handler {

	return func(next http.Handler) http.Handler {

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := m.extractToken(r)
			if token == "" {
				m.sendUnauthorized(w, "missing_token", "Access token required")

				return
			}

			var authSuccessful bool
			var ctx context.Context

			if preferOAuth {
				// Try OAuth first
				if accessToken, err := m.validateOAuthToken(token); err == nil && accessToken != nil {
					if client, exists := m.server.GetClient(accessToken.ClientID); exists {
						ctx = context.WithValue(r.Context(), ClientContextKey, client)
						ctx = context.WithValue(ctx, TokenContextKey, accessToken)
						ctx = context.WithValue(ctx, UserContextKey, accessToken.UserID)
						ctx = context.WithValue(ctx, ScopeContextKey, accessToken.Scope)
						ctx = context.WithValue(ctx, AuthTypeContextKey, "oauth")
						authSuccessful = true
					}
				}

				// Fallback to API key
				if !authSuccessful && m.apiKey != "" && token == m.apiKey {
					ctx = context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
					authSuccessful = true
				}
			} else {
				// Try API key first
				if m.apiKey != "" && token == m.apiKey {
					ctx = context.WithValue(r.Context(), AuthTypeContextKey, "api_key")
					authSuccessful = true
				}

				// Fallback to OAuth
				if !authSuccessful {
					if accessToken, err := m.validateOAuthToken(token); err == nil && accessToken != nil {
						if client, exists := m.server.GetClient(accessToken.ClientID); exists {
							ctx = context.WithValue(r.Context(), ClientContextKey, client)
							ctx = context.WithValue(ctx, TokenContextKey, accessToken)
							ctx = context.WithValue(ctx, UserContextKey, accessToken.UserID)
							ctx = context.WithValue(ctx, ScopeContextKey, accessToken.Scope)
							ctx = context.WithValue(ctx, AuthTypeContextKey, "oauth")
							authSuccessful = true
						}
					}
				}
			}

			if !authSuccessful {
				m.sendUnauthorized(w, "invalid_token", "Invalid access token or API key")

				return
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// extractToken extracts bearer token from Authorization header
func (m *AuthenticationMiddleware) extractToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {

		return ""
	}

	parts := strings.SplitN(authHeader, " ", AuthHeaderSplitParts)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {

		return ""
	}

	return parts[1]
}

// validateOAuthToken validates an OAuth access token
func (m *AuthenticationMiddleware) validateOAuthToken(token string) (*AccessToken, error) {
	if m.server == nil {

		return nil, fmt.Errorf("OAuth server not configured")
	}

	m.server.mu.RLock()
	defer m.server.mu.RUnlock()

	accessToken, exists := m.server.accessTokens[token]
	if !exists {

		return nil, fmt.Errorf("invalid token")
	}

	// Check expiration
	if accessToken.ExpiresAt.IsZero() || time.Now().After(accessToken.ExpiresAt) {
		// Remove expired token
		go func() {
			m.server.mu.Lock()
			delete(m.server.accessTokens, token)
			m.server.mu.Unlock()
		}()

		return nil, fmt.Errorf("token expired")
	}

	return accessToken, nil
}

// hasScope checks if token scope includes required scope
func (m *AuthenticationMiddleware) hasScope(tokenScope, requiredScope string) bool {
	if tokenScope == "" {

		return false
	}

	scopes := strings.Fields(tokenScope)
	for _, scope := range scopes {
		if scope == requiredScope || scope == "mcp:*" {

			return true
		}
	}

	return false
}

// sendUnauthorized sends unauthorized response
func (m *AuthenticationMiddleware) sendUnauthorized(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-compose"`)
	w.WriteHeader(http.StatusUnauthorized)

	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		m.server.logger.Error("Failed to encode unauthorized response: %v", err)
	}
}

// sendForbidden sends forbidden response
func (m *AuthenticationMiddleware) sendForbidden(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		m.server.logger.Error("Failed to encode forbidden response: %v", err)
	}
}

// Context helper functions

// GetClientFromContext extracts OAuth client from request context
func GetClientFromContext(ctx context.Context) (*OAuthClient, bool) {
	client, ok := ctx.Value(ClientContextKey).(*OAuthClient)

	return client, ok
}

// GetTokenFromContext extracts access token from request context
func GetTokenFromContext(ctx context.Context) (*AccessToken, bool) {
	token, ok := ctx.Value(TokenContextKey).(*AccessToken)

	return token, ok
}

// GetUserFromContext extracts user ID from request context
func GetUserFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserContextKey).(string)

	return userID, ok
}

// GetScopeFromContext extracts scope from request context
func GetScopeFromContext(ctx context.Context) (string, bool) {
	scope, ok := ctx.Value(ScopeContextKey).(string)

	return scope, ok
}

// GetAuthTypeFromContext extracts authentication type from request context
func GetAuthTypeFromContext(ctx context.Context) (string, bool) {
	authType, ok := ctx.Value(AuthTypeContextKey).(string)

	return authType, ok
}

// IsAPIKeyAuth checks if request was authenticated with API key
func IsAPIKeyAuth(ctx context.Context) bool {
	authType, ok := GetAuthTypeFromContext(ctx)

	return ok && authType == "api_key"
}

// IsOAuthAuth checks if request was authenticated with OAuth
func IsOAuthAuth(ctx context.Context) bool {
	authType, ok := GetAuthTypeFromContext(ctx)

	return ok && authType == "oauth"
}
