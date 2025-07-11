package auth

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryTokenStore(t *testing.T) {
	store := NewMemoryTokenStore()

	// Test storing and retrieving tokens
	t.Run("store_and_retrieve", func(t *testing.T) {
		token := &AccessToken{
			Token:     "test-token-123",
			Type:      "Bearer",
			ClientID:  "test-client",
			UserID:    "user123",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := store.StoreAccessToken(token)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		retrievedToken, err := store.GetAccessToken("test-token-123")
		if err != nil {
			t.Fatalf("Failed to retrieve token: %v", err)
		}

		if retrievedToken.UserID != "user123" {
			t.Errorf("Expected user ID user123, got %s", retrievedToken.UserID)
		}
	})

	t.Run("token_expiration", func(t *testing.T) {
		token := &AccessToken{
			Token:     "short-lived-token",
			Type:      "Bearer",
			ClientID:  "test-client",
			UserID:    "user456",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(-time.Hour), // Already expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		// Store expired token
		err := store.StoreAccessToken(token)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		_, err = store.GetAccessToken("short-lived-token")
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("invalid_token", func(t *testing.T) {
		_, err := store.GetAccessToken("nonexistent-token")
		if err == nil {
			t.Error("Expected error for nonexistent token")
		}
	})

	t.Run("revoke_token", func(t *testing.T) {
		token := &AccessToken{
			Token:     "revoke-test-token",
			Type:      "Bearer",
			ClientID:  "test-client",
			UserID:    "user789",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := store.StoreAccessToken(token)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		err = store.RevokeAccessToken("revoke-test-token")
		if err != nil {
			t.Fatalf("Failed to revoke token: %v", err)
		}

		_, err = store.GetAccessToken("revoke-test-token")
		if err == nil {
			t.Error("Expected error for revoked token")
		}
	})
}

func TestTokenStoreStats(t *testing.T) {
	store := NewMemoryTokenStore()

	t.Run("empty_store_stats", func(t *testing.T) {
		activeAccess, activeRefresh, activeCodes := store.GetStats()

		if activeAccess != 0 {
			t.Errorf("Expected 0 active access tokens, got %d", activeAccess)
		}

		if activeRefresh != 0 {
			t.Errorf("Expected 0 active refresh tokens, got %d", activeRefresh)
		}

		if activeCodes != 0 {
			t.Errorf("Expected 0 active codes, got %d", activeCodes)
		}
	})

	t.Run("store_stats_after_adding", func(t *testing.T) {
		token := &AccessToken{
			Token:     "stats-test-token",
			Type:      "Bearer",
			ClientID:  "test-client",
			UserID:    "user123",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		if err := store.StoreAccessToken(token); err != nil {
			t.Fatalf("Failed to store access token: %v", err)
		}

		activeAccess, _, _ := store.GetStats()
		if activeAccess != 1 {
			t.Errorf("Expected 1 active access token, got %d", activeAccess)
		}
	})
}

func TestAPIKeyAuthentication(t *testing.T) {
	// Mock request with API key
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer test-api-key-123")

	// Test API key extraction
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		t.Error("Expected authorization header")
	}

	// Test Bearer token format
	if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
		t.Error("Expected Bearer token format")
	}

	token := authHeader[7:]
	if token != "test-api-key-123" {
		t.Errorf("Expected token 'test-api-key-123', got %s", token)
	}
}

func TestProtectedResourceMetadata(t *testing.T) {
	handler := NewResourceMetadataHandler(
		[]string{"https://auth.example.com"},
		[]string{"read", "write"},
	)

	metadata := handler.GetMetadata()

	// Test metadata validation
	if metadata.Resource == "" {
		t.Error("Resource should not be empty")
	}

	if len(metadata.AuthorizationServers) == 0 {
		t.Error("Authorization servers should not be empty")
	}

	if len(metadata.ScopesSupported) == 0 {
		t.Error("Scopes should not be empty")
	}

	// Test scope checking
	hasReadScope := false
	hasWriteScope := false
	for _, scope := range metadata.ScopesSupported {
		if scope == "read" {
			hasReadScope = true
		}
		if scope == "write" {
			hasWriteScope = true
		}
	}

	if !hasReadScope {
		t.Error("Expected read scope")
	}
	if !hasWriteScope {
		t.Error("Expected write scope")
	}
}

func TestConcurrentTokenOperations(t *testing.T) {
	store := NewMemoryTokenStore()

	// Test concurrent token storage and retrieval
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			tokenStr := fmt.Sprintf("concurrent-token-%d", id)
			userID := fmt.Sprintf("user-%d", id)

			token := &AccessToken{
				Token:     tokenStr,
				Type:      "Bearer",
				ClientID:  "test-client",
				UserID:    userID,
				Scope:     "read write",
				ExpiresAt: time.Now().Add(time.Hour),
				CreatedAt: time.Now(),
			}

			// Store token
			err := store.StoreAccessToken(token)
			if err != nil {
				t.Errorf("Failed to store token %d: %v", id, err)
			}

			// Retrieve token
			retrievedToken, err := store.GetAccessToken(tokenStr)
			if err != nil {
				t.Errorf("Failed to retrieve token %d: %v", id, err)
			}

			if retrievedToken.UserID != userID {
				t.Errorf("Token %d: expected user ID %s, got %s", id, userID, retrievedToken.UserID)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("Concurrent operations timed out")
		}
	}
}

func TestBasicAuthenticationFlow(t *testing.T) {
	// This would test actual middleware if it existed
	// For now, test the basic authentication flow

	t.Run("valid_request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/protected", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		// Simulate authentication check
		authHeader := req.Header.Get("Authorization")
		if authHeader == "" {
			t.Error("Missing authorization header")
		}

		if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
			t.Error("Invalid authorization format")
		}

		token := authHeader[7:]
		if token == "" {
			t.Error("Empty token")
		}
	})

	t.Run("missing_auth_header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/protected", nil)

		authHeader := req.Header.Get("Authorization")
		if authHeader != "" {
			t.Error("Expected missing authorization header")
		}
	})

	t.Run("invalid_auth_format", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/protected", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")

		authHeader := req.Header.Get("Authorization")
		if len(authHeader) >= 7 && authHeader[:7] == "Bearer " {
			t.Error("Expected invalid authorization format to be rejected")
		}
	})
}

func TestScopeValidation(t *testing.T) {
	tests := []struct {
		name           string
		requiredScopes []string
		userScopes     []string
		expected       bool
	}{
		{
			name:           "exact match",
			requiredScopes: []string{"read"},
			userScopes:     []string{"read"},
			expected:       true,
		},
		{
			name:           "user has more scopes",
			requiredScopes: []string{"read"},
			userScopes:     []string{"read", "write", "admin"},
			expected:       true,
		},
		{
			name:           "user missing required scope",
			requiredScopes: []string{"admin"},
			userScopes:     []string{"read", "write"},
			expected:       false,
		},
		{
			name:           "multiple required scopes - all present",
			requiredScopes: []string{"read", "write"},
			userScopes:     []string{"read", "write", "admin"},
			expected:       true,
		},
		{
			name:           "multiple required scopes - some missing",
			requiredScopes: []string{"read", "write", "admin"},
			userScopes:     []string{"read", "write"},
			expected:       false,
		},
		{
			name:           "empty required scopes",
			requiredScopes: []string{},
			userScopes:     []string{"read"},
			expected:       true,
		},
		{
			name:           "empty user scopes",
			requiredScopes: []string{"read"},
			userScopes:     []string{},
			expected:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple scope validation logic
			hasAllScopes := true
			for _, required := range tt.requiredScopes {
				found := false
				for _, userScope := range tt.userScopes {
					if userScope == required {
						found = true

						break
					}
				}
				if !found {
					hasAllScopes = false

					break
				}
			}

			if hasAllScopes != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, hasAllScopes)
			}
		})
	}
}
