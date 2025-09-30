package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		apiKey         string
		authHeader     string
		method         string
		expectedStatus int
	}{
		{
			name:           "valid api key",
			apiKey:         "test-key-123",
			authHeader:     "Bearer test-key-123",
			method:         "GET",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid api key",
			apiKey:         "test-key-123",
			authHeader:     "Bearer wrong-key",
			method:         "GET",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "missing authorization header",
			apiKey:         "test-key-123",
			authHeader:     "",
			method:         "GET",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid header format",
			apiKey:         "test-key-123",
			authHeader:     "Basic test-key-123",
			method:         "GET",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "options method bypasses auth",
			apiKey:         "test-key-123",
			authHeader:     "",
			method:         "OPTIONS",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := NewAPIKeyMiddleware(tt.apiKey, handler)

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			middleware.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedStatus == http.StatusOK && tt.method != "OPTIONS" {
				authType, ok := GetAuthTypeFromContext(req.Context())
				if ok {
					assert.Equal(t, "api_key", authType)
				}
			}
		})
	}
}

func TestAuthenticationMiddleware_RequireAuthentication(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)
	middleware.SetAPIKey("test-api-key")

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
	_, err := authServer.RegisterClient(clientConfig)
	require.NoError(t, err)

	testToken := "test-oauth-token"
	accessToken := &AccessToken{
		Token:     testToken,
		Type:      "Bearer",
		ClientID:  "test-client",
		UserID:    "test-user",
		Scope:     "read write",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	authServer.mu.Lock()
	authServer.accessTokens[testToken] = accessToken
	authServer.mu.Unlock()

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectUser     bool
	}{
		{
			name:           "valid oauth token",
			authHeader:     "Bearer " + testToken,
			expectedStatus: http.StatusOK,
			expectUser:     true,
		},
		{
			name:           "valid api key",
			authHeader:     "Bearer test-api-key",
			expectedStatus: http.StatusOK,
			expectUser:     false,
		},
		{
			name:           "invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusUnauthorized,
			expectUser:     false,
		},
		{
			name:           "missing token",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
			expectUser:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.expectUser {
					userID, ok := GetUserFromContext(r.Context())
					assert.True(t, ok)
					assert.Equal(t, "test-user", userID)
				}

				authType, ok := GetAuthTypeFromContext(r.Context())
				assert.True(t, ok)
				if tt.authHeader == "Bearer test-api-key" {
					assert.Equal(t, "api_key", authType)
				} else if tt.expectUser {
					assert.Equal(t, "oauth", authType)
				}

				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			middleware.RequireAuthentication(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAuthenticationMiddleware_RequireScope(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
	_, err := authServer.RegisterClient(clientConfig)
	require.NoError(t, err)

	tests := []struct {
		name           string
		tokenScope     string
		requiredScope  string
		authType       string
		expectedStatus int
	}{
		{
			name:           "has required scope",
			tokenScope:     "read write admin",
			requiredScope:  "admin",
			authType:       "oauth",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "missing required scope",
			tokenScope:     "read write",
			requiredScope:  "admin",
			authType:       "oauth",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "wildcard scope",
			tokenScope:     "mcp:*",
			requiredScope:  "admin",
			authType:       "oauth",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "api key bypasses scope check",
			tokenScope:     "",
			requiredScope:  "admin",
			authType:       "api_key",
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			ctx := context.Background()
			ctx = context.WithValue(ctx, AuthTypeContextKey, tt.authType)
			if tt.tokenScope != "" {
				ctx = context.WithValue(ctx, ScopeContextKey, tt.tokenScope)
			}

			req := httptest.NewRequest("GET", "/test", nil)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()
			middleware.RequireScope(tt.requiredScope)(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAuthenticationMiddleware_OptionalAuthentication(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)
	middleware.SetAPIKey("test-api-key")

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
	_, err := authServer.RegisterClient(clientConfig)
	require.NoError(t, err)

	testToken := "test-oauth-token"
	accessToken := &AccessToken{
		Token:     testToken,
		Type:      "Bearer",
		ClientID:  "test-client",
		UserID:    "test-user",
		Scope:     "read write",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	authServer.mu.Lock()
	authServer.accessTokens[testToken] = accessToken
	authServer.mu.Unlock()

	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
		expectAuth     bool
	}{
		{
			name:           "with valid oauth token",
			authHeader:     "Bearer " + testToken,
			expectedStatus: http.StatusOK,
			expectAuth:     true,
		},
		{
			name:           "with valid api key",
			authHeader:     "Bearer test-api-key",
			expectedStatus: http.StatusOK,
			expectAuth:     true,
		},
		{
			name:           "without auth",
			authHeader:     "",
			expectedStatus: http.StatusOK,
			expectAuth:     false,
		},
		{
			name:           "with invalid token",
			authHeader:     "Bearer invalid-token",
			expectedStatus: http.StatusOK,
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, hasAuth := GetAuthTypeFromContext(r.Context())
				assert.Equal(t, tt.expectAuth, hasAuth)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			middleware.OptionalAuthentication(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestAuthenticationMiddleware_FlexibleAuthentication(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)
	middleware.SetAPIKey("test-api-key")

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
	_, err := authServer.RegisterClient(clientConfig)
	require.NoError(t, err)

	testToken := "test-oauth-token"
	accessToken := &AccessToken{
		Token:     testToken,
		Type:      "Bearer",
		ClientID:  "test-client",
		UserID:    "test-user",
		Scope:     "read write",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	authServer.mu.Lock()
	authServer.accessTokens[testToken] = accessToken
	authServer.mu.Unlock()

	tests := []struct {
		name           string
		preferOAuth    bool
		authHeader     string
		expectedStatus int
		expectedAuth   string
	}{
		{
			name:           "prefer oauth with valid token",
			preferOAuth:    true,
			authHeader:     "Bearer " + testToken,
			expectedStatus: http.StatusOK,
			expectedAuth:   "oauth",
		},
		{
			name:           "prefer api key with valid token",
			preferOAuth:    false,
			authHeader:     "Bearer test-api-key",
			expectedStatus: http.StatusOK,
			expectedAuth:   "api_key",
		},
		{
			name:           "fallback to api key when oauth unavailable",
			preferOAuth:    true,
			authHeader:     "Bearer test-api-key",
			expectedStatus: http.StatusOK,
			expectedAuth:   "api_key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authType, ok := GetAuthTypeFromContext(r.Context())
				assert.True(t, ok)
				assert.Equal(t, tt.expectedAuth, authType)
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			middleware.FlexibleAuthentication(tt.preferOAuth)(handler).ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestContextHelpers(t *testing.T) {
	t.Run("GetClientFromContext", func(t *testing.T) {
		client := &OAuthClient{ID: "test-client"}
		ctx := context.WithValue(context.Background(), ClientContextKey, client)

		result, ok := GetClientFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, client, result)

		_, ok = GetClientFromContext(context.Background())
		assert.False(t, ok)
	})

	t.Run("GetTokenFromContext", func(t *testing.T) {
		token := &AccessToken{Token: "test-token"}
		ctx := context.WithValue(context.Background(), TokenContextKey, token)

		result, ok := GetTokenFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, token, result)

		_, ok = GetTokenFromContext(context.Background())
		assert.False(t, ok)
	})

	t.Run("GetUserFromContext", func(t *testing.T) {
		userID := "test-user"
		ctx := context.WithValue(context.Background(), UserContextKey, userID)

		result, ok := GetUserFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, userID, result)

		_, ok = GetUserFromContext(context.Background())
		assert.False(t, ok)
	})

	t.Run("IsAPIKeyAuth", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), AuthTypeContextKey, "api_key")
		assert.True(t, IsAPIKeyAuth(ctx))

		ctx = context.WithValue(context.Background(), AuthTypeContextKey, "oauth")
		assert.False(t, IsAPIKeyAuth(ctx))

		assert.False(t, IsAPIKeyAuth(context.Background()))
	})

	t.Run("IsOAuthAuth", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), AuthTypeContextKey, "oauth")
		assert.True(t, IsOAuthAuth(ctx))

		ctx = context.WithValue(context.Background(), AuthTypeContextKey, "api_key")
		assert.False(t, IsOAuthAuth(ctx))

		assert.False(t, IsOAuthAuth(context.Background()))
	})
}

func TestExpiredTokenCleanup(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}
	_, err := authServer.RegisterClient(clientConfig)
	require.NoError(t, err)

	expiredToken := "expired-token"
	accessToken := &AccessToken{
		Token:     expiredToken,
		Type:      "Bearer",
		ClientID:  "test-client",
		UserID:    "test-user",
		Scope:     "read",
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	authServer.mu.Lock()
	authServer.accessTokens[expiredToken] = accessToken
	authServer.mu.Unlock()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)

	w := httptest.NewRecorder()
	middleware.RequireAuthentication(handler).ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	time.Sleep(100 * time.Millisecond)

	authServer.mu.RLock()
	_, exists := authServer.accessTokens[expiredToken]
	authServer.mu.RUnlock()

	assert.False(t, exists, "Expired token should be cleaned up")
}

func TestExtractToken(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)

	tests := []struct {
		name       string
		authHeader string
		expected   string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer test-token-123",
			expected:   "test-token-123",
		},
		{
			name:       "case insensitive bearer",
			authHeader: "bearer test-token-123",
			expected:   "test-token-123",
		},
		{
			name:       "empty header",
			authHeader: "",
			expected:   "",
		},
		{
			name:       "invalid format",
			authHeader: "Basic test-token-123",
			expected:   "",
		},
		{
			name:       "missing token",
			authHeader: "Bearer ",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := middleware.extractToken(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasScope(t *testing.T) {
	logger := logging.NewLogger("error")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.example.com",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)
	middleware := NewAuthenticationMiddleware(authServer)

	tests := []struct {
		name          string
		tokenScope    string
		requiredScope string
		expected      bool
	}{
		{
			name:          "exact match",
			tokenScope:    "read write admin",
			requiredScope: "admin",
			expected:      true,
		},
		{
			name:          "no match",
			tokenScope:    "read write",
			requiredScope: "admin",
			expected:      false,
		},
		{
			name:          "wildcard scope",
			tokenScope:    "mcp:* read",
			requiredScope: "admin",
			expected:      true,
		},
		{
			name:          "empty token scope",
			tokenScope:    "",
			requiredScope: "admin",
			expected:      false,
		},
		{
			name:          "multiple scopes",
			tokenScope:    "read write delete",
			requiredScope: "write",
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := middleware.hasScope(tt.tokenScope, tt.requiredScope)
			assert.Equal(t, tt.expected, result)
		})
	}
}