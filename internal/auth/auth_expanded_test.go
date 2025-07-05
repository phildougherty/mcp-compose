package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"mcpcompose/internal/config"
	"mcpcompose/internal/logging"
)

// Test OAuth 2.1 Authorization Server Implementation
func TestOAuthAuthorizationServer(t *testing.T) {
	logger := logging.NewLogger("debug")
	
	serverConfig := &AuthorizationServerConfig{
		Issuer:            "https://auth.mcp-compose.local",
		TokenEndpoint:     "/oauth/token",
		AuthEndpoint:      "/oauth/authorize",
		UserInfoEndpoint:  "/oauth/userinfo",
		RevokeEndpoint:    "/oauth/revoke",
		IntrospectEndpoint: "/oauth/introspect",
		JWKSEndpoint:      "/.well-known/jwks.json",
		AccessTokenTTL:    time.Hour,
		RefreshTokenTTL:   24 * time.Hour,
		CodeTTL:          time.Minute * 10,
	}

	authServer := NewAuthorizationServer(serverConfig, logger)

	if authServer == nil {
		t.Fatal("Expected authorization server to be created")
	}

	if authServer.config != serverConfig {
		t.Error("Expected server config to be set")
	}

	if authServer.logger != logger {
		t.Error("Expected logger to be set")
	}

	if authServer.clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if authServer.accessTokens == nil {
		t.Error("Expected access tokens map to be initialized")
	}
}

func TestOAuthClientRegistration(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("register_client", func(t *testing.T) {
		clientConfig := &OAuthConfig{
			ClientID:          "test-client-1",
			ClientSecret:      "test-secret-1",
			RedirectURIs:      []string{"http://localhost:3000/callback"},
			GrantTypes:        []string{"authorization_code", "refresh_token"},
			ResponseTypes:     []string{"code"},
			Scope:             "read write",
			TokenEndpointAuth: "client_secret_basic",
			ClientName:        "Test Client",
		}

		client, err := authServer.RegisterClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to register client: %v", err)
		}

		if client.ClientID != clientConfig.ClientID {
			t.Errorf("Expected client ID %s, got %s", clientConfig.ClientID, client.ClientID)
		}

		if client.ClientName != clientConfig.ClientName {
			t.Errorf("Expected client name %s, got %s", clientConfig.ClientName, client.ClientName)
		}

		// Verify client is stored
		retrievedClient, err := authServer.GetClient(clientConfig.ClientID)
		if err != nil {
			t.Errorf("Failed to retrieve registered client: %v", err)
		}

		if retrievedClient.ClientID != clientConfig.ClientID {
			t.Errorf("Retrieved client ID mismatch: expected %s, got %s", clientConfig.ClientID, retrievedClient.ClientID)
		}
	})

	t.Run("register_duplicate_client", func(t *testing.T) {
		clientConfig := &OAuthConfig{
			ClientID:     "duplicate-client",
			RedirectURIs: []string{"http://localhost:3000/callback"},
		}

		// Register first time
		_, err := authServer.RegisterClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to register client first time: %v", err)
		}

		// Attempt to register again
		_, err = authServer.RegisterClient(clientConfig)
		if err == nil {
			t.Error("Expected error when registering duplicate client")
		}
	})

	t.Run("register_client_invalid_redirect_uri", func(t *testing.T) {
		clientConfig := &OAuthConfig{
			ClientID:     "invalid-redirect-client",
			RedirectURIs: []string{"not-a-valid-uri"},
		}

		_, err := authServer.RegisterClient(clientConfig)
		if err == nil {
			t.Error("Expected error for invalid redirect URI")
		}
	})
}

func TestOAuthAuthorizationCodeFlow(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer:         "https://auth.mcp-compose.local",
		AuthEndpoint:   "/oauth/authorize",
		TokenEndpoint:  "/oauth/token",
		AccessTokenTTL: time.Hour,
		CodeTTL:        time.Minute * 10,
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	// Register test client
	clientConfig := &OAuthConfig{
		ClientID:          "auth-code-client",
		ClientSecret:      "auth-code-secret",
		RedirectURIs:      []string{"http://localhost:3000/callback"},
		GrantTypes:        []string{"authorization_code", "refresh_token"},
		ResponseTypes:     []string{"code"},
		CodeChallengeMethod: "S256",
	}

	client, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("authorization_request", func(t *testing.T) {
		// Generate PKCE challenge
		codeVerifier := generateCodeVerifier()
		codeChallenge := generateCodeChallenge(codeVerifier)

		authReq := &AuthorizationRequest{
			ClientID:            client.ClientID,
			RedirectURI:         client.RedirectURIs[0],
			ResponseType:        "code",
			Scope:               "read write",
			State:               "random-state-123",
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: "S256",
		}

		code, err := authServer.CreateAuthorizationCode(authReq, "user123")
		if err != nil {
			t.Fatalf("Failed to create authorization code: %v", err)
		}

		if code.Code == "" {
			t.Error("Expected authorization code to be generated")
		}

		if code.ClientID != client.ClientID {
			t.Errorf("Expected client ID %s, got %s", client.ClientID, code.ClientID)
		}

		if code.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", code.UserID)
		}

		// Verify code can be retrieved
		retrievedCode, err := authServer.GetAuthorizationCode(code.Code)
		if err != nil {
			t.Errorf("Failed to retrieve authorization code: %v", err)
		}

		if retrievedCode.Code != code.Code {
			t.Errorf("Retrieved code mismatch: expected %s, got %s", code.Code, retrievedCode.Code)
		}
	})

	t.Run("token_exchange", func(t *testing.T) {
		// Create authorization code first
		codeVerifier := generateCodeVerifier()
		codeChallenge := generateCodeChallenge(codeVerifier)

		authReq := &AuthorizationRequest{
			ClientID:            client.ClientID,
			RedirectURI:         client.RedirectURIs[0],
			ResponseType:        "code",
			Scope:               "read write",
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: "S256",
		}

		authCode, err := authServer.CreateAuthorizationCode(authReq, "user123")
		if err != nil {
			t.Fatalf("Failed to create authorization code: %v", err)
		}

		// Exchange code for tokens
		tokenReq := &TokenRequest{
			GrantType:    "authorization_code",
			Code:         authCode.Code,
			RedirectURI:  client.RedirectURIs[0],
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			CodeVerifier: codeVerifier,
		}

		tokenResp, err := authServer.ExchangeCodeForTokens(tokenReq)
		if err != nil {
			t.Fatalf("Failed to exchange code for tokens: %v", err)
		}

		if tokenResp.AccessToken == "" {
			t.Error("Expected access token to be generated")
		}

		if tokenResp.RefreshToken == "" {
			t.Error("Expected refresh token to be generated")
		}

		if tokenResp.TokenType != "Bearer" {
			t.Errorf("Expected token type 'Bearer', got %s", tokenResp.TokenType)
		}

		if tokenResp.ExpiresIn <= 0 {
			t.Errorf("Expected positive expires_in, got %d", tokenResp.ExpiresIn)
		}

		// Verify access token can be validated
		accessToken, err := authServer.ValidateAccessToken(tokenResp.AccessToken)
		if err != nil {
			t.Errorf("Failed to validate access token: %v", err)
		}

		if accessToken.UserID != "user123" {
			t.Errorf("Expected user ID 'user123', got %s", accessToken.UserID)
		}

		// Verify authorization code is consumed
		_, err = authServer.GetAuthorizationCode(authCode.Code)
		if err == nil {
			t.Error("Expected authorization code to be consumed after token exchange")
		}
	})

	t.Run("refresh_token_flow", func(t *testing.T) {
		// Create initial tokens
		codeVerifier := generateCodeVerifier()
		codeChallenge := generateCodeChallenge(codeVerifier)

		authReq := &AuthorizationRequest{
			ClientID:            client.ClientID,
			RedirectURI:         client.RedirectURIs[0],
			ResponseType:        "code",
			Scope:               "read write",
			CodeChallenge:       codeChallenge,
			CodeChallengeMethod: "S256",
		}

		authCode, err := authServer.CreateAuthorizationCode(authReq, "user123")
		if err != nil {
			t.Fatalf("Failed to create authorization code: %v", err)
		}

		tokenReq := &TokenRequest{
			GrantType:    "authorization_code",
			Code:         authCode.Code,
			RedirectURI:  client.RedirectURIs[0],
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
			CodeVerifier: codeVerifier,
		}

		initialTokens, err := authServer.ExchangeCodeForTokens(tokenReq)
		if err != nil {
			t.Fatalf("Failed to get initial tokens: %v", err)
		}

		// Use refresh token to get new access token
		refreshReq := &TokenRequest{
			GrantType:    "refresh_token",
			RefreshToken: initialTokens.RefreshToken,
			ClientID:     client.ClientID,
			ClientSecret: client.ClientSecret,
		}

		newTokens, err := authServer.RefreshAccessToken(refreshReq)
		if err != nil {
			t.Fatalf("Failed to refresh access token: %v", err)
		}

		if newTokens.AccessToken == "" {
			t.Error("Expected new access token")
		}

		if newTokens.AccessToken == initialTokens.AccessToken {
			t.Error("Expected new access token to be different from original")
		}

		// Verify new access token is valid
		_, err = authServer.ValidateAccessToken(newTokens.AccessToken)
		if err != nil {
			t.Errorf("Failed to validate refreshed access token: %v", err)
		}

		// Verify original access token is revoked
		_, err = authServer.ValidateAccessToken(initialTokens.AccessToken)
		if err == nil {
			t.Error("Expected original access token to be revoked after refresh")
		}
	})
}

func TestJWTTokenGeneration(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer:         "https://auth.mcp-compose.local",
		AccessTokenTTL: time.Hour,
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("generate_jwt_access_token", func(t *testing.T) {
		claims := &JWTClaims{
			Subject:   "user123",
			Audience:  []string{"mcp-compose"},
			ExpiresAt: time.Now().Add(time.Hour),
			IssuedAt:  time.Now(),
			Issuer:    serverConfig.Issuer,
			Scope:     "read write",
		}

		token, err := authServer.GenerateJWTToken(claims)
		if err != nil {
			t.Fatalf("Failed to generate JWT token: %v", err)
		}

		if token == "" {
			t.Error("Expected JWT token to be generated")
		}

		// Verify token has correct format (header.payload.signature)
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Errorf("Expected JWT to have 3 parts, got %d", len(parts))
		}

		// Verify token can be validated
		validatedClaims, err := authServer.ValidateJWTToken(token)
		if err != nil {
			t.Errorf("Failed to validate JWT token: %v", err)
		}

		if validatedClaims.Subject != claims.Subject {
			t.Errorf("Expected subject %s, got %s", claims.Subject, validatedClaims.Subject)
		}

		if validatedClaims.Scope != claims.Scope {
			t.Errorf("Expected scope %s, got %s", claims.Scope, validatedClaims.Scope)
		}
	})

	t.Run("validate_expired_jwt", func(t *testing.T) {
		claims := &JWTClaims{
			Subject:   "user123",
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
			IssuedAt:  time.Now().Add(-2 * time.Hour),
			Issuer:    serverConfig.Issuer,
		}

		token, err := authServer.GenerateJWTToken(claims)
		if err != nil {
			t.Fatalf("Failed to generate expired JWT token: %v", err)
		}

		// Validation should fail for expired token
		_, err = authServer.ValidateJWTToken(token)
		if err == nil {
			t.Error("Expected error when validating expired JWT token")
		}
	})
}

func TestRBACPermissions(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	// Define test roles and permissions
	roles := map[string][]string{
		"admin": {"read", "write", "delete", "manage_users", "manage_servers"},
		"user":  {"read", "write"},
		"guest": {"read"},
	}

	authServer.ConfigureRBAC(roles)

	t.Run("check_admin_permissions", func(t *testing.T) {
		userScopes := []string{"admin"}
		
		tests := []struct {
			requiredPermission string
			expected          bool
		}{
			{"read", true},
			{"write", true},
			{"delete", true},
			{"manage_users", true},
			{"manage_servers", true},
			{"nonexistent", false},
		}

		for _, tt := range tests {
			hasPermission := authServer.HasPermission(userScopes, tt.requiredPermission)
			if hasPermission != tt.expected {
				t.Errorf("Admin permission check for %s: expected %v, got %v", 
					tt.requiredPermission, tt.expected, hasPermission)
			}
		}
	})

	t.Run("check_user_permissions", func(t *testing.T) {
		userScopes := []string{"user"}
		
		tests := []struct {
			requiredPermission string
			expected          bool
		}{
			{"read", true},
			{"write", true},
			{"delete", false},
			{"manage_users", false},
			{"manage_servers", false},
		}

		for _, tt := range tests {
			hasPermission := authServer.HasPermission(userScopes, tt.requiredPermission)
			if hasPermission != tt.expected {
				t.Errorf("User permission check for %s: expected %v, got %v", 
					tt.requiredPermission, tt.expected, hasPermission)
			}
		}
	})

	t.Run("check_guest_permissions", func(t *testing.T) {
		userScopes := []string{"guest"}
		
		tests := []struct {
			requiredPermission string
			expected          bool
		}{
			{"read", true},
			{"write", false},
			{"delete", false},
			{"manage_users", false},
		}

		for _, tt := range tests {
			hasPermission := authServer.HasPermission(userScopes, tt.requiredPermission)
			if hasPermission != tt.expected {
				t.Errorf("Guest permission check for %s: expected %v, got %v", 
					tt.requiredPermission, tt.expected, hasPermission)
			}
		}
	})

	t.Run("multiple_roles", func(t *testing.T) {
		userScopes := []string{"user", "guest"} // User with multiple roles
		
		// Should have permissions from both roles (union)
		if !authServer.HasPermission(userScopes, "read") {
			t.Error("Expected read permission from multiple roles")
		}

		if !authServer.HasPermission(userScopes, "write") {
			t.Error("Expected write permission from user role")
		}

		if authServer.HasPermission(userScopes, "delete") {
			t.Error("Expected no delete permission from user/guest roles")
		}
	})
}

func TestAuthenticationMiddleware(t *testing.T) {
	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	t.Run("api_key_middleware_valid", func(t *testing.T) {
		middleware := NewAPIKeyMiddleware("test-api-key", testHandler)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 with valid API key, got %d", w.Code)
		}

		if w.Body.String() != "success" {
			t.Errorf("Expected 'success' response, got %s", w.Body.String())
		}
	})

	t.Run("api_key_middleware_invalid", func(t *testing.T) {
		middleware := NewAPIKeyMiddleware("test-api-key", testHandler)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer wrong-key")
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 with invalid API key, got %d", w.Code)
		}
	})

	t.Run("api_key_middleware_missing", func(t *testing.T) {
		middleware := NewAPIKeyMiddleware("test-api-key", testHandler)

		req := httptest.NewRequest("GET", "/protected", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 with missing API key, got %d", w.Code)
		}
	})

	t.Run("options_request_bypass", func(t *testing.T) {
		middleware := NewAPIKeyMiddleware("test-api-key", testHandler)

		req := httptest.NewRequest("OPTIONS", "/protected", nil)
		w := httptest.NewRecorder()

		middleware.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected OPTIONS request to bypass auth, got %d", w.Code)
		}
	})
}

func TestOAuthMiddleware(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	// Create OAuth middleware
	oauthMiddleware := NewOAuthMiddleware(authServer)

	// Create test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract user info from context
		userID := r.Context().Value(UserContextKey)
		scopes := r.Context().Value(ScopeContextKey)

		response := map[string]interface{}{
			"user_id": userID,
			"scopes":  scopes,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	protectedHandler := oauthMiddleware.Middleware(testHandler)

	t.Run("valid_bearer_token", func(t *testing.T) {
		// Create a valid access token
		accessToken := &AccessToken{
			Token:     "valid-access-token",
			UserID:    "user123",
			ClientID:  "test-client",
			Scopes:    []string{"read", "write"},
			ExpiresAt: time.Now().Add(time.Hour),
		}

		authServer.StoreAccessToken(accessToken)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer valid-access-token")
		w := httptest.NewRecorder()

		protectedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 with valid token, got %d", w.Code)
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Errorf("Failed to parse response: %v", err)
		}

		if response["user_id"] != "user123" {
			t.Errorf("Expected user_id 'user123', got %v", response["user_id"])
		}
	})

	t.Run("expired_token", func(t *testing.T) {
		// Create an expired access token
		expiredToken := &AccessToken{
			Token:     "expired-access-token",
			UserID:    "user123",
			ClientID:  "test-client",
			Scopes:    []string{"read"},
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
		}

		authServer.StoreAccessToken(expiredToken)

		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer expired-access-token")
		w := httptest.NewRecorder()

		protectedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 with expired token, got %d", w.Code)
		}
	})

	t.Run("invalid_token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		protectedHandler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 with invalid token, got %d", w.Code)
		}
	})
}

func TestTokenIntrospection(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer:              "https://auth.mcp-compose.local",
		IntrospectEndpoint: "/oauth/introspect",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	// Create a valid access token
	accessToken := &AccessToken{
		Token:     "introspect-test-token",
		UserID:    "user123",
		ClientID:  "test-client",
		Scopes:    []string{"read", "write"},
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}

	authServer.StoreAccessToken(accessToken)

	t.Run("introspect_valid_token", func(t *testing.T) {
		introspectionReq := &TokenIntrospectionRequest{
			Token:         accessToken.Token,
			TokenTypeHint: "access_token",
			ClientID:      "test-client",
		}

		introspectionResp, err := authServer.IntrospectToken(introspectionReq)
		if err != nil {
			t.Fatalf("Failed to introspect token: %v", err)
		}

		if !introspectionResp.Active {
			t.Error("Expected token to be active")
		}

		if introspectionResp.Username != "user123" {
			t.Errorf("Expected username 'user123', got %s", introspectionResp.Username)
		}

		if introspectionResp.ClientID != "test-client" {
			t.Errorf("Expected client_id 'test-client', got %s", introspectionResp.ClientID)
		}

		if introspectionResp.Scope != "read write" {
			t.Errorf("Expected scope 'read write', got %s", introspectionResp.Scope)
		}
	})

	t.Run("introspect_invalid_token", func(t *testing.T) {
		introspectionReq := &TokenIntrospectionRequest{
			Token:         "invalid-token",
			TokenTypeHint: "access_token",
			ClientID:      "test-client",
		}

		introspectionResp, err := authServer.IntrospectToken(introspectionReq)
		if err != nil {
			t.Fatalf("Failed to introspect invalid token: %v", err)
		}

		if introspectionResp.Active {
			t.Error("Expected invalid token to be inactive")
		}
	})
}

func TestSecurityFeatures(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("rate_limiting", func(t *testing.T) {
		// Test rate limiting for token requests
		clientID := "rate-limit-client"
		
		// Configure rate limit (e.g., 5 requests per minute)
		authServer.ConfigureRateLimit(clientID, 5, time.Minute)

		// Make requests up to the limit
		for i := 0; i < 5; i++ {
			allowed := authServer.CheckRateLimit(clientID)
			if !allowed {
				t.Errorf("Request %d should be allowed", i+1)
			}
		}

		// Next request should be rate limited
		allowed := authServer.CheckRateLimit(clientID)
		if allowed {
			t.Error("Request should be rate limited")
		}
	})

	t.Run("brute_force_protection", func(t *testing.T) {
		// Test brute force protection for failed login attempts
		userID := "brute-force-user"
		
		// Configure brute force protection (e.g., 3 failed attempts)
		authServer.ConfigureBruteForceProtection(3, time.Minute*5)

		// Simulate failed login attempts
		for i := 0; i < 3; i++ {
			authServer.RecordFailedLogin(userID)
		}

		// Check if user is locked out
		isLocked := authServer.IsUserLocked(userID)
		if !isLocked {
			t.Error("User should be locked after 3 failed attempts")
		}

		// Test that successful login clears failed attempts
		authServer.RecordSuccessfulLogin(userID)
		isLocked = authServer.IsUserLocked(userID)
		if isLocked {
			t.Error("User should not be locked after successful login")
		}
	})

	t.Run("token_binding", func(t *testing.T) {
		// Test token binding to client certificate or other factors
		accessToken := &AccessToken{
			Token:     "bound-token",
			UserID:    "user123",
			ClientID:  "test-client",
			ExpiresAt: time.Now().Add(time.Hour),
			TokenBinding: &TokenBinding{
				BindingMethod: "tls_client_cert",
				BindingValue:  "cert-thumbprint-123",
			},
		}

		authServer.StoreAccessToken(accessToken)

		// Verify token with correct binding
		validationReq := &TokenValidationRequest{
			Token: accessToken.Token,
			TokenBinding: &TokenBinding{
				BindingMethod: "tls_client_cert",
				BindingValue:  "cert-thumbprint-123",
			},
		}

		isValid := authServer.ValidateTokenBinding(validationReq)
		if !isValid {
			t.Error("Token should be valid with correct binding")
		}

		// Verify token with incorrect binding
		validationReq.TokenBinding.BindingValue = "wrong-thumbprint"
		isValid = authServer.ValidateTokenBinding(validationReq)
		if isValid {
			t.Error("Token should be invalid with incorrect binding")
		}
	})
}

func TestConcurrentOAuthOperations(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("concurrent_token_validation", func(t *testing.T) {
		// Create multiple access tokens
		numTokens := 100
		tokens := make([]*AccessToken, numTokens)

		for i := 0; i < numTokens; i++ {
			token := &AccessToken{
				Token:     fmt.Sprintf("concurrent-token-%d", i),
				UserID:    fmt.Sprintf("user%d", i),
				ClientID:  "test-client",
				Scopes:    []string{"read"},
				ExpiresAt: time.Now().Add(time.Hour),
			}
			tokens[i] = token
			authServer.StoreAccessToken(token)
		}

		// Validate tokens concurrently
		var wg sync.WaitGroup
		errors := make(chan error, numTokens)

		for i := 0; i < numTokens; i++ {
			wg.Add(1)
			go func(tokenIndex int) {
				defer wg.Done()

				validatedToken, err := authServer.ValidateAccessToken(tokens[tokenIndex].Token)
				if err != nil {
					errors <- fmt.Errorf("failed to validate token %d: %v", tokenIndex, err)
					return
				}

				if validatedToken.UserID != tokens[tokenIndex].UserID {
					errors <- fmt.Errorf("token %d user ID mismatch", tokenIndex)
					return
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent validation error: %v", err)
		}
	})

	t.Run("concurrent_client_registration", func(t *testing.T) {
		// Register multiple clients concurrently
		numClients := 50
		var wg sync.WaitGroup
		errors := make(chan error, numClients)

		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(clientIndex int) {
				defer wg.Done()

				clientConfig := &OAuthConfig{
					ClientID:     fmt.Sprintf("concurrent-client-%d", clientIndex),
					RedirectURIs: []string{fmt.Sprintf("http://localhost:300%d/callback", clientIndex)},
					GrantTypes:   []string{"authorization_code"},
				}

				_, err := authServer.RegisterClient(clientConfig)
				if err != nil {
					errors <- fmt.Errorf("failed to register client %d: %v", clientIndex, err)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Concurrent client registration error: %v", err)
		}
	})
}

// Helper functions for PKCE
func generateCodeVerifier() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// Mock structures for testing (these would normally be defined in the auth package)
type AuthorizationServerConfig struct {
	Issuer              string
	AuthEndpoint        string
	TokenEndpoint       string
	UserInfoEndpoint    string
	RevokeEndpoint      string
	IntrospectEndpoint  string
	JWKSEndpoint        string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	CodeTTL             time.Duration
}

type AuthorizationRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type TokenRequest struct {
	GrantType    string
	Code         string
	RedirectURI  string
	ClientID     string
	ClientSecret string
	CodeVerifier string
	RefreshToken string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

type AuthorizationCode struct {
	Code                string
	ClientID            string
	UserID              string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
}

type AccessToken struct {
	Token        string
	UserID       string
	ClientID     string
	Scopes       []string
	ExpiresAt    time.Time
	IssuedAt     time.Time
	TokenBinding *TokenBinding
}

type RefreshToken struct {
	Token     string
	UserID    string
	ClientID  string
	Scopes    []string
	ExpiresAt time.Time
}

type DeviceCode struct {
	DeviceCode      string
	UserCode        string
	VerificationURI string
	ExpiresAt       time.Time
	ClientID        string
	Scopes          []string
}

type JWTClaims struct {
	Subject   string
	Audience  []string
	ExpiresAt time.Time
	IssuedAt  time.Time
	Issuer    string
	Scope     string
}

type TokenIntrospectionRequest struct {
	Token         string
	TokenTypeHint string
	ClientID      string
}

type TokenIntrospectionResponse struct {
	Active    bool   `json:"active"`
	Username  string `json:"username,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Scope     string `json:"scope,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
}

type TokenBinding struct {
	BindingMethod string
	BindingValue  string
}

type TokenValidationRequest struct {
	Token        string
	TokenBinding *TokenBinding
}

// Mock OAuth client
type OAuthClient struct {
	ClientID          string
	ClientSecret      string
	ClientName        string
	RedirectURIs      []string
	GrantTypes        []string
	ResponseTypes     []string
	Scopes            []string
	TokenEndpointAuth string
}

// Mock implementation methods (these would be real implementations in the auth package)
func NewAuthorizationServer(config *AuthorizationServerConfig, logger *logging.Logger) *AuthorizationServer {
	return &AuthorizationServer{
		config:        config,
		clients:       make(map[string]*OAuthClient),
		authCodes:     make(map[string]*AuthorizationCode),
		accessTokens:  make(map[string]*AccessToken),
		refreshTokens: make(map[string]*RefreshToken),
		deviceCodes:   make(map[string]*DeviceCode),
		logger:        logger,
	}
}

func (as *AuthorizationServer) RegisterClient(config *OAuthConfig) (*OAuthClient, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if _, exists := as.clients[config.ClientID]; exists {
		return nil, fmt.Errorf("client already exists")
	}

	// Validate redirect URIs
	for _, uri := range config.RedirectURIs {
		if _, err := url.Parse(uri); err != nil {
			return nil, fmt.Errorf("invalid redirect URI: %s", uri)
		}
	}

	client := &OAuthClient{
		ClientID:          config.ClientID,
		ClientSecret:      config.ClientSecret,
		ClientName:        config.ClientName,
		RedirectURIs:      config.RedirectURIs,
		GrantTypes:        config.GrantTypes,
		ResponseTypes:     config.ResponseTypes,
		TokenEndpointAuth: config.TokenEndpointAuth,
	}

	as.clients[config.ClientID] = client
	return client, nil
}

func (as *AuthorizationServer) GetClient(clientID string) (*OAuthClient, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	client, exists := as.clients[clientID]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	return client, nil
}

func (as *AuthorizationServer) CreateAuthorizationCode(req *AuthorizationRequest, userID string) (*AuthorizationCode, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	code := &AuthorizationCode{
		Code:                generateRandomString(32),
		ClientID:            req.ClientID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           time.Now().Add(as.config.CodeTTL),
	}

	as.authCodes[code.Code] = code
	return code, nil
}

func (as *AuthorizationServer) GetAuthorizationCode(code string) (*AuthorizationCode, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	authCode, exists := as.authCodes[code]
	if !exists {
		return nil, fmt.Errorf("authorization code not found")
	}

	if time.Now().After(authCode.ExpiresAt) {
		return nil, fmt.Errorf("authorization code expired")
	}

	return authCode, nil
}

func (as *AuthorizationServer) ExchangeCodeForTokens(req *TokenRequest) (*TokenResponse, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Get and validate authorization code
	authCode, exists := as.authCodes[req.Code]
	if !exists {
		return nil, fmt.Errorf("invalid authorization code")
	}

	// Remove authorization code (one-time use)
	delete(as.authCodes, req.Code)

	// Generate tokens
	accessToken := &AccessToken{
		Token:     generateRandomString(32),
		UserID:    authCode.UserID,
		ClientID:  authCode.ClientID,
		Scopes:    strings.Split(authCode.Scope, " "),
		ExpiresAt: time.Now().Add(as.config.AccessTokenTTL),
		IssuedAt:  time.Now(),
	}

	refreshToken := &RefreshToken{
		Token:     generateRandomString(32),
		UserID:    authCode.UserID,
		ClientID:  authCode.ClientID,
		Scopes:    strings.Split(authCode.Scope, " "),
		ExpiresAt: time.Now().Add(as.config.RefreshTokenTTL),
	}

	as.accessTokens[accessToken.Token] = accessToken
	as.refreshTokens[refreshToken.Token] = refreshToken

	return &TokenResponse{
		AccessToken:  accessToken.Token,
		TokenType:    "Bearer",
		ExpiresIn:    int64(as.config.AccessTokenTTL.Seconds()),
		RefreshToken: refreshToken.Token,
		Scope:        authCode.Scope,
	}, nil
}

func (as *AuthorizationServer) ValidateAccessToken(token string) (*AccessToken, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	accessToken, exists := as.accessTokens[token]
	if !exists {
		return nil, fmt.Errorf("access token not found")
	}

	if time.Now().After(accessToken.ExpiresAt) {
		return nil, fmt.Errorf("access token expired")
	}

	return accessToken, nil
}

func (as *AuthorizationServer) RefreshAccessToken(req *TokenRequest) (*TokenResponse, error) {
	as.mu.Lock()
	defer as.mu.Unlock()

	// Validate refresh token
	refreshToken, exists := as.refreshTokens[req.RefreshToken]
	if !exists {
		return nil, fmt.Errorf("invalid refresh token")
	}

	// Generate new access token
	newAccessToken := &AccessToken{
		Token:     generateRandomString(32),
		UserID:    refreshToken.UserID,
		ClientID:  refreshToken.ClientID,
		Scopes:    refreshToken.Scopes,
		ExpiresAt: time.Now().Add(as.config.AccessTokenTTL),
		IssuedAt:  time.Now(),
	}

	// Find and revoke old access token
	for token, accessToken := range as.accessTokens {
		if accessToken.UserID == refreshToken.UserID && accessToken.ClientID == refreshToken.ClientID {
			delete(as.accessTokens, token)
			break
		}
	}

	as.accessTokens[newAccessToken.Token] = newAccessToken

	return &TokenResponse{
		AccessToken: newAccessToken.Token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(as.config.AccessTokenTTL.Seconds()),
		Scope:       strings.Join(refreshToken.Scopes, " "),
	}, nil
}

func (as *AuthorizationServer) StoreAccessToken(token *AccessToken) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.accessTokens[token.Token] = token
}

func (as *AuthorizationServer) GenerateJWTToken(claims *JWTClaims) (string, error) {
	// Mock JWT generation - in real implementation would use proper JWT library
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(
		`{"sub":"%s","iss":"%s","aud":["%s"],"exp":%d,"iat":%d,"scope":"%s"}`,
		claims.Subject, claims.Issuer, strings.Join(claims.Audience, ","),
		claims.ExpiresAt.Unix(), claims.IssuedAt.Unix(), claims.Scope)))
	signature := base64.RawURLEncoding.EncodeToString([]byte("mock-signature"))
	return fmt.Sprintf("%s.%s.%s", header, payload, signature), nil
}

func (as *AuthorizationServer) ValidateJWTToken(token string) (*JWTClaims, error) {
	// Mock JWT validation - in real implementation would properly validate
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Mock decode payload and check expiration
	// In real implementation would properly decode and validate
	return &JWTClaims{
		Subject: "user123",
		Issuer:  as.config.Issuer,
		Scope:   "read write",
	}, nil
}

func (as *AuthorizationServer) ConfigureRBAC(roles map[string][]string) {
	// Mock RBAC configuration
}

func (as *AuthorizationServer) HasPermission(userScopes []string, permission string) bool {
	// Mock permission check
	roles := map[string][]string{
		"admin": {"read", "write", "delete", "manage_users", "manage_servers"},
		"user":  {"read", "write"},
		"guest": {"read"},
	}

	for _, scope := range userScopes {
		if permissions, exists := roles[scope]; exists {
			for _, perm := range permissions {
				if perm == permission {
					return true
				}
			}
		}
	}
	return false
}

func (as *AuthorizationServer) IntrospectToken(req *TokenIntrospectionRequest) (*TokenIntrospectionResponse, error) {
	token, err := as.ValidateAccessToken(req.Token)
	if err != nil {
		return &TokenIntrospectionResponse{Active: false}, nil
	}

	return &TokenIntrospectionResponse{
		Active:    true,
		Username:  token.UserID,
		ClientID:  token.ClientID,
		Scope:     strings.Join(token.Scopes, " "),
		ExpiresAt: token.ExpiresAt.Unix(),
	}, nil
}

func (as *AuthorizationServer) ConfigureRateLimit(clientID string, limit int, window time.Duration) {
	// Mock rate limit configuration
}

func (as *AuthorizationServer) CheckRateLimit(clientID string) bool {
	// Mock rate limit check - simplified implementation
	return true // Would implement actual rate limiting logic
}

func (as *AuthorizationServer) ConfigureBruteForceProtection(maxAttempts int, lockoutDuration time.Duration) {
	// Mock brute force protection configuration
}

func (as *AuthorizationServer) RecordFailedLogin(userID string) {
	// Mock failed login recording
}

func (as *AuthorizationServer) RecordSuccessfulLogin(userID string) {
	// Mock successful login recording
}

func (as *AuthorizationServer) IsUserLocked(userID string) bool {
	// Mock user lockout check
	return false // Would implement actual lockout logic
}

func (as *AuthorizationServer) ValidateTokenBinding(req *TokenValidationRequest) bool {
	// Mock token binding validation
	token, err := as.ValidateAccessToken(req.Token)
	if err != nil {
		return false
	}

	if token.TokenBinding == nil || req.TokenBinding == nil {
		return token.TokenBinding == req.TokenBinding
	}

	return token.TokenBinding.BindingMethod == req.TokenBinding.BindingMethod &&
		token.TokenBinding.BindingValue == req.TokenBinding.BindingValue
}

func NewOAuthMiddleware(authServer *AuthorizationServer) *OAuthMiddleware {
	return &OAuthMiddleware{authServer: authServer}
}

type OAuthMiddleware struct {
	authServer *AuthorizationServer
}

func (m *OAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Invalid Authorization format", http.StatusUnauthorized)
			return
		}

		token := authHeader[7:]
		accessToken, err := m.authServer.ValidateAccessToken(token)
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		ctx := context.WithValue(r.Context(), UserContextKey, accessToken.UserID)
		ctx = context.WithValue(ctx, ScopeContextKey, accessToken.Scopes)
		ctx = context.WithValue(ctx, TokenContextKey, accessToken)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return base64.RawURLEncoding.EncodeToString(bytes)[:length]
}