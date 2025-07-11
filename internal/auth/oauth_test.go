package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"mcpcompose/internal/logging"
)

// Test OAuth 2.1 Authorization Server Implementation
func TestOAuthAuthorizationServer(t *testing.T) {
	logger := logging.NewLogger("debug")

	serverConfig := &AuthorizationServerConfig{
		Issuer:                "https://auth.mcp-compose.local",
		TokenEndpoint:         "/oauth/token",
		AuthorizationEndpoint: "/oauth/authorize",
		UserinfoEndpoint:      "/oauth/userinfo",
		RevocationEndpoint:    "/oauth/revoke",
		JWKSUri:               "/.well-known/jwks.json",
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

		if client.ID != clientConfig.ClientID {
			t.Errorf("Expected client ID %s, got %s", clientConfig.ClientID, client.ID)
		}

		if client.ClientName != clientConfig.ClientName {
			t.Errorf("Expected client name %s, got %s", clientConfig.ClientName, client.ClientName)
		}

		// Verify client is stored
		retrievedClient, exists := authServer.GetClient(clientConfig.ClientID)
		if !exists {
			t.Error("Failed to retrieve registered client")
		}

		if retrievedClient.ID != clientConfig.ClientID {
			t.Errorf("Expected retrieved client ID %s, got %s", clientConfig.ClientID, retrievedClient.ID)
		}
	})

	t.Run("duplicate_client_registration", func(t *testing.T) {
		clientConfig := &OAuthConfig{
			ClientID:     "duplicate-client",
			RedirectURIs: []string{"http://localhost:3000/callback"},
		}

		// Register first time
		_, err := authServer.RegisterClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to register client first time: %v", err)
		}

		// Try to register again
		_, err = authServer.RegisterClient(clientConfig)
		if err == nil {
			t.Error("Expected error when registering duplicate client")
		}
	})

	t.Run("invalid_redirect_uri", func(t *testing.T) {
		clientConfig := &OAuthConfig{
			ClientID:     "invalid-uri-client",
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
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	// Register a client
	clientConfig := &OAuthConfig{
		ClientID:            "auth-code-client",
		ClientSecret:        "secret",
		RedirectURIs:        []string{"http://localhost:3000/callback"},
		GrantTypes:          []string{"authorization_code"},
		ResponseTypes:       []string{"code"},
		CodeChallengeMethod: "S256",
	}

	client, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("authorization_code_validation", func(t *testing.T) {
		// Test access token validation
		testToken := "test-access-token"

		// Create a test access token
		accessToken := &AccessToken{
			Token:     testToken,
			Type:      "Bearer",
			ClientID:  client.ID,
			UserID:    "test-user",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		// Store the token for testing
		authServer.mu.Lock()
		authServer.accessTokens[testToken] = accessToken
		authServer.mu.Unlock()

		// Validate the token
		validatedToken, err := authServer.ValidateAccessToken(testToken)
		if err != nil {
			t.Errorf("Failed to validate access token: %v", err)
		}

		if validatedToken.Token != testToken {
			t.Errorf("Expected token %s, got %s", testToken, validatedToken.Token)
		}

		if validatedToken.ClientID != client.ID {
			t.Errorf("Expected client ID %s, got %s", client.ID, validatedToken.ClientID)
		}
	})

	t.Run("expired_token_validation", func(t *testing.T) {
		// Test expired token
		expiredToken := "expired-token"

		accessToken := &AccessToken{
			Token:     expiredToken,
			Type:      "Bearer",
			ClientID:  client.ID,
			UserID:    "test-user",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(-time.Hour), // Expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		authServer.mu.Lock()
		authServer.accessTokens[expiredToken] = accessToken
		authServer.mu.Unlock()

		// Should fail validation
		_, err := authServer.ValidateAccessToken(expiredToken)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})
}

func TestJWTTokenGeneration(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	_ = NewAuthorizationServer(serverConfig, logger)

	t.Run("token_generation", func(t *testing.T) {
		// Test token generation through the generator
		generator := &DefaultTokenGenerator{}

		accessToken, err := generator.GenerateAccessToken()
		if err != nil {
			t.Fatalf("Failed to generate access token: %v", err)
		}

		if accessToken == "" {
			t.Error("Generated access token should not be empty")
		}

		refreshToken, err := generator.GenerateRefreshToken()
		if err != nil {
			t.Fatalf("Failed to generate refresh token: %v", err)
		}

		if refreshToken == "" {
			t.Error("Generated refresh token should not be empty")
		}

		if accessToken == refreshToken {
			t.Error("Access token and refresh token should be different")
		}
	})

	t.Run("pkce_code_challenge", func(t *testing.T) {
		verifier := &DefaultCodeVerifier{}

		codeVerifier, err := verifier.GenerateCodeVerifier()
		if err != nil {
			t.Fatalf("Failed to generate code verifier: %v", err)
		}

		if len(codeVerifier) != 128 {
			t.Errorf("Expected code verifier length 128, got %d", len(codeVerifier))
		}

		// Test S256 challenge
		challenge, err := verifier.GenerateCodeChallenge(codeVerifier, "S256")
		if err != nil {
			t.Fatalf("Failed to generate code challenge: %v", err)
		}

		// Verify the challenge
		isValid := verifier.VerifyCodeChallenge(codeVerifier, challenge, "S256")
		if !isValid {
			t.Error("Code challenge verification failed")
		}

		// Test plain challenge
		plainChallenge, err := verifier.GenerateCodeChallenge(codeVerifier, "plain")
		if err != nil {
			t.Fatalf("Failed to generate plain code challenge: %v", err)
		}

		if plainChallenge != codeVerifier {
			t.Error("Plain code challenge should equal code verifier")
		}
	})
}

func TestRBACPermissions(t *testing.T) {
	rbac := NewRBACManager()

	t.Run("role_creation_and_assignment", func(t *testing.T) {
		// Create roles
		adminRole := rbac.CreateRole("admin", []string{"read", "write", "delete", "admin"})
		_ = rbac.CreateRole("user", []string{"read", "write"})
		_ = rbac.CreateRole("guest", []string{"read"})

		if adminRole.Name != "admin" {
			t.Errorf("Expected admin role name 'admin', got %s", adminRole.Name)
		}

		if len(adminRole.Permissions) != 4 {
			t.Errorf("Expected 4 admin permissions, got %d", len(adminRole.Permissions))
		}

		// Assign roles to users
		err := rbac.AssignRole("user1", "admin")
		if err != nil {
			t.Errorf("Failed to assign admin role: %v", err)
		}

		err = rbac.AssignRole("user2", "user")
		if err != nil {
			t.Errorf("Failed to assign user role: %v", err)
		}

		err = rbac.AssignRole("user3", "guest")
		if err != nil {
			t.Errorf("Failed to assign guest role: %v", err)
		}

		// Test permissions
		if !rbac.HasPermission("user1", "admin") {
			t.Error("user1 should have admin permission")
		}

		if !rbac.HasPermission("user2", "write") {
			t.Error("user2 should have write permission")
		}

		if rbac.HasPermission("user3", "write") {
			t.Error("user3 should not have write permission")
		}

		if !rbac.HasPermission("user3", "read") {
			t.Error("user3 should have read permission")
		}
	})

	t.Run("wildcard_permissions", func(t *testing.T) {
		rbac.CreateRole("superuser", []string{"*"})
		if err := rbac.AssignRole("superuser1", "superuser"); err != nil {
			t.Fatalf("Failed to assign superuser role: %v", err)
		}

		// Superuser should have any permission
		if !rbac.HasPermission("superuser1", "any_permission") {
			t.Error("superuser should have wildcard permissions")
		}

		if !rbac.HasPermission("superuser1", "read") {
			t.Error("superuser should have read permission")
		}

		if !rbac.HasPermission("superuser1", "admin") {
			t.Error("superuser should have admin permission")
		}
	})

	t.Run("multiple_roles", func(t *testing.T) {
		rbac.CreateRole("editor", []string{"read", "write", "edit"})
		rbac.CreateRole("moderator", []string{"moderate", "ban"})

		// Assign multiple roles
		if err := rbac.AssignRole("multi_user", "editor"); err != nil {
			t.Fatalf("Failed to assign editor role: %v", err)
		}
		if err := rbac.AssignRole("multi_user", "moderator"); err != nil {
			t.Fatalf("Failed to assign moderator role: %v", err)
		}

		roles := rbac.GetUserRoles("multi_user")
		if len(roles) != 2 {
			t.Errorf("Expected 2 roles, got %d", len(roles))
		}

		// Should have permissions from both roles
		if !rbac.HasPermission("multi_user", "edit") {
			t.Error("multi_user should have edit permission from editor role")
		}

		if !rbac.HasPermission("multi_user", "moderate") {
			t.Error("multi_user should have moderate permission from moderator role")
		}

		if rbac.HasPermission("multi_user", "admin") {
			t.Error("multi_user should not have admin permission")
		}
	})
}

func TestAuthenticationMiddleware(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("oauth_authentication", func(t *testing.T) {
		middleware := NewAuthenticationMiddleware(authServer)
		middleware.SetAPIKey("test-api-key")

		// Register a test client
		clientConfig := &OAuthConfig{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			RedirectURIs: []string{"http://localhost:3000/callback"},
		}
		_, err := authServer.RegisterClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to register client: %v", err)
		}

		// Create a test access token
		testToken := "oauth-test-token"
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

		// Create test handler
		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if user context is set
			userID, ok := GetUserFromContext(r.Context())
			if !ok {
				t.Error("Expected user ID in context")

				return
			}

			if userID != "test-user" {
				t.Errorf("Expected user ID 'test-user', got %s", userID)
			}

			authType, ok := GetAuthTypeFromContext(r.Context())
			if !ok {
				t.Error("Expected auth type in context")

				return
			}

			if authType != "oauth" {
				t.Errorf("Expected auth type 'oauth', got %s", authType)
			}

			w.WriteHeader(http.StatusOK)
		})

		// Test OAuth authentication
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)
		w := httptest.NewRecorder()

		handler := middleware.RequireAuthentication(testHandler)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("api_key_fallback", func(t *testing.T) {
		middleware := NewAuthenticationMiddleware(authServer)
		middleware.SetAPIKey("test-api-key")

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authType, ok := GetAuthTypeFromContext(r.Context())
			if !ok {
				t.Error("Expected auth type in context")

				return
			}

			if authType != "api_key" {
				t.Errorf("Expected auth type 'api_key', got %s", authType)
			}

			w.WriteHeader(http.StatusOK)
		})

		// Test API key authentication
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer test-api-key")
		w := httptest.NewRecorder()

		handler := middleware.RequireAuthentication(testHandler)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("scope_validation", func(t *testing.T) {
		middleware := NewAuthenticationMiddleware(authServer)

		// Register a test client
		clientConfig := &OAuthConfig{
			ClientID:     "test-client-scope",
			ClientSecret: "test-secret",
			RedirectURIs: []string{"http://localhost:3000/callback"},
		}
		_, err := authServer.RegisterClient(clientConfig)
		if err != nil {
			t.Fatalf("Failed to register client: %v", err)
		}

		// Create token with specific scope
		testToken := "scoped-token"
		accessToken := &AccessToken{
			Token:     testToken,
			Type:      "Bearer",
			ClientID:  "test-client-scope",
			UserID:    "test-user",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		authServer.mu.Lock()
		authServer.accessTokens[testToken] = accessToken
		authServer.mu.Unlock()

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+testToken)

		// Test with required scope that exists
		w := httptest.NewRecorder()
		handler := middleware.RequireAuthentication(middleware.RequireScope("read")(testHandler))
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for valid scope, got %d", w.Code)
		}

		// Test with required scope that doesn't exist
		w = httptest.NewRecorder()
		handler = middleware.RequireAuthentication(middleware.RequireScope("admin")(testHandler))
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for invalid scope, got %d", w.Code)
		}
	})

	t.Run("missing_authorization", func(t *testing.T) {
		middleware := NewAuthenticationMiddleware(authServer)

		testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := middleware.RequireAuthentication(testHandler)
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for missing auth, got %d", w.Code)
		}
	})
}

func TestConcurrentOAuthOperations(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://auth.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("concurrent_client_registration", func(t *testing.T) {
		var wg sync.WaitGroup
		numClients := 10
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


// Mock RBAC implementation for testing
type RBACRole struct {
	Name        string
	Permissions []string
}

type RBACUser struct {
	UserID string
	Roles  []string
}

type RBACManager struct {
	roles map[string]*RBACRole
	users map[string]*RBACUser
	mu    sync.RWMutex
}

func NewRBACManager() *RBACManager {

	return &RBACManager{
		roles: make(map[string]*RBACRole),
		users: make(map[string]*RBACUser),
	}
}

func (r *RBACManager) CreateRole(name string, permissions []string) *RBACRole {
	r.mu.Lock()
	defer r.mu.Unlock()

	role := &RBACRole{
		Name:        name,
		Permissions: permissions,
	}
	r.roles[name] = role

	return role
}

func (r *RBACManager) AssignRole(userID, roleName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.roles[roleName]; !exists {

		return fmt.Errorf("role not found: %s", roleName)
	}

	user, exists := r.users[userID]
	if !exists {
		user = &RBACUser{
			UserID: userID,
			Roles:  []string{},
		}
		r.users[userID] = user
	}

	// Check if role is already assigned
	for _, role := range user.Roles {
		if role == roleName {

			return nil // Already assigned
		}
	}

	user.Roles = append(user.Roles, roleName)

	return nil
}

func (r *RBACManager) HasPermission(userID, permission string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {

		return false
	}

	for _, roleName := range user.Roles {
		role, exists := r.roles[roleName]
		if !exists {
			continue
		}

		for _, perm := range role.Permissions {
			if perm == permission || perm == "*" {

				return true
			}
		}
	}


	return false
}

func (r *RBACManager) GetUserRoles(userID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[userID]
	if !exists {

		return []string{}
	}


	return user.Roles
}

func (r *RBACManager) GetRolePermissions(roleName string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	role, exists := r.roles[roleName]
	if !exists {

		return []string{}
	}


	return role.Permissions
}
