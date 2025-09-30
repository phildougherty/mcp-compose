package auth

import (
	"testing"
	"time"
)

func TestJWTManager(t *testing.T) {
	issuer := "https://test.mcp-compose.local"

	t.Run("create_jwt_manager", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		if manager == nil {
			t.Fatal("JWT manager should not be nil")
		}

		if manager.privateKey == nil {
			t.Error("Private key should not be nil")
		}

		if manager.publicKey == nil {
			t.Error("Public key should not be nil")
		}

		if manager.issuer != issuer {
			t.Errorf("Expected issuer %s, got %s", issuer, manager.issuer)
		}
	})

	t.Run("generate_and_validate_access_token", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		clientID := "test-client"
		userID := "test-user"
		scope := "read write"
		duration := 1 * time.Hour

		token, err := manager.GenerateAccessToken(clientID, userID, scope, duration)
		if err != nil {
			t.Fatalf("Failed to generate access token: %v", err)
		}

		if token == "" {
			t.Error("Generated token should not be empty")
		}

		claims, err := manager.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("Failed to validate access token: %v", err)
		}

		if claims.ClientID != clientID {
			t.Errorf("Expected client ID %s, got %s", clientID, claims.ClientID)
		}

		if claims.UserID != userID {
			t.Errorf("Expected user ID %s, got %s", userID, claims.UserID)
		}

		if claims.Scope != scope {
			t.Errorf("Expected scope %s, got %s", scope, claims.Scope)
		}

		if claims.Issuer != issuer {
			t.Errorf("Expected issuer %s, got %s", issuer, claims.Issuer)
		}

		if claims.Subject != userID {
			t.Errorf("Expected subject %s, got %s", userID, claims.Subject)
		}
	})

	t.Run("validate_expired_token", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		token, err := manager.GenerateAccessToken("test-client", "test-user", "read", -1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to generate expired token: %v", err)
		}

		_, err = manager.ValidateAccessToken(token)
		if err == nil {
			t.Error("Expected error for expired token")
		}
	})

	t.Run("validate_invalid_token", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		_, err = manager.ValidateAccessToken("invalid-token")
		if err == nil {
			t.Error("Expected error for invalid token")
		}
	})

	t.Run("validate_token_from_different_issuer", func(t *testing.T) {
		manager1, err := NewJWTManager("https://issuer1.example.com")
		if err != nil {
			t.Fatalf("Failed to create JWT manager 1: %v", err)
		}

		manager2, err := NewJWTManager("https://issuer2.example.com")
		if err != nil {
			t.Fatalf("Failed to create JWT manager 2: %v", err)
		}

		token, err := manager1.GenerateAccessToken("client", "user", "read", 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to generate token: %v", err)
		}

		_, err = manager2.ValidateAccessToken(token)
		if err == nil {
			t.Error("Expected error when validating token from different issuer")
		}
	})

	t.Run("generate_token_with_empty_fields", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		token, err := manager.GenerateAccessToken("client", "", "", 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to generate token with empty fields: %v", err)
		}

		claims, err := manager.ValidateAccessToken(token)
		if err != nil {
			t.Fatalf("Failed to validate token: %v", err)
		}

		if claims.UserID != "" {
			t.Errorf("Expected empty user ID, got %s", claims.UserID)
		}

		if claims.Scope != "" {
			t.Errorf("Expected empty scope, got %s", claims.Scope)
		}
	})

	t.Run("get_public_key", func(t *testing.T) {
		manager, err := NewJWTManager(issuer)
		if err != nil {
			t.Fatalf("Failed to create JWT manager: %v", err)
		}

		publicKey := manager.GetPublicKey()
		if publicKey == nil {
			t.Error("Public key should not be nil")
		}

		if publicKey != manager.publicKey {
			t.Error("GetPublicKey should return the same public key")
		}
	})
}