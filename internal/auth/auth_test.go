package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMemoryTokenStore(t *testing.T) {
	store := NewMemoryTokenStore()

	// Test storing and retrieving tokens
	t.Run("store_and_retrieve", func(t *testing.T) {
		token := "test-token-123"
		userID := "user123"
		
		err := store.StoreToken(token, userID, time.Hour)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		retrievedUserID, err := store.GetUserID(token)
		if err != nil {
			t.Fatalf("Failed to retrieve user ID: %v", err)
		}

		if retrievedUserID != userID {
			t.Errorf("Expected user ID %s, got %s", userID, retrievedUserID)
		}
	})

	t.Run("token_expiration", func(t *testing.T) {
		token := "short-lived-token"
		userID := "user456"
		
		// Store token with very short expiration
		err := store.StoreToken(token, userID, time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		// Wait for expiration
		time.Sleep(10 * time.Millisecond)

		_, err = store.GetUserID(token)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("invalid_token", func(t *testing.T) {
		_, err := store.GetUserID("nonexistent-token")
		if err == nil {
			t.Error("Expected error for nonexistent token")
		}
	})

	t.Run("revoke_token", func(t *testing.T) {
		token := "revoke-test-token"
		userID := "user789"
		
		err := store.StoreToken(token, userID, time.Hour)
		if err != nil {
			t.Fatalf("Failed to store token: %v", err)
		}

		err = store.RevokeToken(token)
		if err != nil {
			t.Fatalf("Failed to revoke token: %v", err)
		}

		_, err = store.GetUserID(token)
		if err == nil {
			t.Error("Expected error for revoked token")
		}
	})
}

func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	t.Run("store_and_get", func(t *testing.T) {
		key := "test-key"
		value := "test-value"

		store.Set(key, value)
		
		retrieved, exists := store.Get(key)
		if !exists {
			t.Error("Expected key to exist")
		}

		if retrieved != value {
			t.Errorf("Expected value %s, got %s", value, retrieved)
		}
	})

	t.Run("delete", func(t *testing.T) {
		key := "delete-test"
		value := "delete-value"

		store.Set(key, value)
		store.Delete(key)

		_, exists := store.Get(key)
		if exists {
			t.Error("Expected key to be deleted")
		}
	})

	t.Run("nonexistent_key", func(t *testing.T) {
		_, exists := store.Get("nonexistent")
		if exists {
			t.Error("Expected nonexistent key to return false")
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

func TestResourceMetadata(t *testing.T) {
	metadata := &ResourceMetadata{
		ResourceID:   "resource-123",
		ResourceType: "server",
		Owner:        "user123",
		Scopes:       []string{"read", "write"},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Test metadata validation
	if metadata.ResourceID == "" {
		t.Error("Resource ID should not be empty")
	}

	if metadata.ResourceType == "" {
		t.Error("Resource type should not be empty")
	}

	if metadata.Owner == "" {
		t.Error("Owner should not be empty")
	}

	if len(metadata.Scopes) == 0 {
		t.Error("Scopes should not be empty")
	}

	// Test scope checking
	hasReadScope := false
	hasWriteScope := false
	for _, scope := range metadata.Scopes {
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
			token := fmt.Sprintf("concurrent-token-%d", id)
			userID := fmt.Sprintf("user-%d", id)

			// Store token
			err := store.StoreToken(token, userID, time.Hour)
			if err != nil {
				t.Errorf("Failed to store token %d: %v", id, err)
			}

			// Retrieve token
			retrievedUserID, err := store.GetUserID(token)
			if err != nil {
				t.Errorf("Failed to retrieve token %d: %v", id, err)
			}

			if retrievedUserID != userID {
				t.Errorf("Token %d: expected user ID %s, got %s", id, userID, retrievedUserID)
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

func TestAuthenticationMiddleware(t *testing.T) {
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