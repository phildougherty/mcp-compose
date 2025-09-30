package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTokenStore(t *testing.T) {
	store := NewTokenStore()
	defer store.Close()

	t.Run("Store and Retrieve Access Token", func(t *testing.T) {
		token := &AccessToken{
			Token:     "test-token-123",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		store.StoreAccessToken(token)

		retrieved, err := store.GetAccessToken("test-token-123")
		require.NoError(t, err)
		assert.Equal(t, token.Token, retrieved.Token)
		assert.Equal(t, token.ClientID, retrieved.ClientID)
		assert.Equal(t, token.UserID, retrieved.UserID)
	})

	t.Run("Retrieve Non-Existent Token", func(t *testing.T) {
		_, err := store.GetAccessToken("non-existent")
		assert.Error(t, err)
	})

	t.Run("Retrieve Expired Token", func(t *testing.T) {
		expiredToken := &AccessToken{
			Token:     "expired-token",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(-time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		store.StoreAccessToken(expiredToken)

		_, err := store.GetAccessToken("expired-token")
		assert.Error(t, err)
	})

	t.Run("Revoke Token", func(t *testing.T) {
		token := &AccessToken{
			Token:     "revoke-token",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		store.StoreAccessToken(token)
		store.RevokeAccessToken("revoke-token")

		_, err := store.GetAccessToken("revoke-token")
		assert.Error(t, err, "Revoked token should not be retrievable")
	})
}

func TestTokenStoreRefreshToken(t *testing.T) {
	store := NewTokenStore()
	defer store.Close()

	t.Run("Store and Retrieve Refresh Token", func(t *testing.T) {
		refreshToken := &RefreshToken{
			Token:     "refresh-token-123",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read write",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
		}

		store.StoreRefreshToken(refreshToken)

		retrieved, err := store.GetRefreshToken("refresh-token-123")
		require.NoError(t, err)
		assert.Equal(t, refreshToken.Token, retrieved.Token)
		assert.Equal(t, refreshToken.ClientID, retrieved.ClientID)
		assert.Equal(t, refreshToken.UserID, retrieved.UserID)
	})

	t.Run("Revoke Refresh Token", func(t *testing.T) {
		refreshToken := &RefreshToken{
			Token:     "revoke-refresh",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(24 * time.Hour),
			CreatedAt: time.Now(),
		}

		store.StoreRefreshToken(refreshToken)
		store.RevokeRefreshToken("revoke-refresh")

		_, err := store.GetRefreshToken("revoke-refresh")
		assert.Error(t, err, "Revoked refresh token should not be retrievable")
	})
}

func TestTokenStoreAuthorizationCode(t *testing.T) {
	store := NewTokenStore()
	defer store.Close()

	t.Run("Store and Use Auth Code", func(t *testing.T) {
		authCode := &AuthorizationCode{
			Code:        "auth-code-123",
			ClientID:    "client-1",
			UserID:      "user-1",
			RedirectURI: "http://localhost:3000/callback",
			Scope:       "read write",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			CreatedAt:   time.Now(),
		}

		store.StoreAuthorizationCode(authCode)

		retrieved, err := store.GetAndUseAuthorizationCode("auth-code-123")
		require.NoError(t, err)
		assert.Equal(t, authCode.Code, retrieved.Code)
		assert.Equal(t, authCode.ClientID, retrieved.ClientID)
		assert.True(t, retrieved.Used)
	})

	t.Run("Cannot Use Code Twice", func(t *testing.T) {
		authCode := &AuthorizationCode{
			Code:        "use-once-code",
			ClientID:    "client-1",
			UserID:      "user-1",
			RedirectURI: "http://localhost:3000/callback",
			Scope:       "read",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
			CreatedAt:   time.Now(),
		}

		store.StoreAuthorizationCode(authCode)

		_, err := store.GetAndUseAuthorizationCode("use-once-code")
		require.NoError(t, err)

		_, err = store.GetAndUseAuthorizationCode("use-once-code")
		assert.Error(t, err, "Used code should not be reusable")
	})
}

func TestTokenStoreIsExpiredMethods(t *testing.T) {
	t.Run("AuthorizationCode IsExpired", func(t *testing.T) {
		expired := &AuthorizationCode{
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		assert.True(t, expired.IsExpired())

		valid := &AuthorizationCode{
			ExpiresAt: time.Now().Add(time.Hour),
		}
		assert.False(t, valid.IsExpired())
	})

	t.Run("AccessToken IsExpired", func(t *testing.T) {
		expired := &AccessToken{
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		assert.True(t, expired.IsExpired())

		valid := &AccessToken{
			ExpiresAt: time.Now().Add(time.Hour),
		}
		assert.False(t, valid.IsExpired())
	})

	t.Run("RefreshToken IsExpired", func(t *testing.T) {
		expired := &RefreshToken{
			ExpiresAt: time.Now().Add(-time.Hour),
		}
		assert.True(t, expired.IsExpired())

		valid := &RefreshToken{
			ExpiresAt: time.Now().Add(time.Hour),
		}
		assert.False(t, valid.IsExpired())
	})
}

func TestMemoryTokenStoreCleanup(t *testing.T) {
	store := NewMemoryTokenStore()
	defer store.Close()

	t.Run("Cleanup Expired Tokens", func(t *testing.T) {
		expiredToken := &AccessToken{
			Token:     "expired-cleanup-token",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(-time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		validToken := &AccessToken{
			Token:     "valid-cleanup-token",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := store.StoreAccessToken(expiredToken)
		require.NoError(t, err)

		err = store.StoreAccessToken(validToken)
		require.NoError(t, err)

		store.CleanupExpiredTokens()

		_, err = store.GetAccessToken("expired-cleanup-token")
		assert.Error(t, err, "Expired token should be cleaned up")

		_, err = store.GetAccessToken("valid-cleanup-token")
		assert.NoError(t, err, "Valid token should remain")
	})

	t.Run("Get Stats", func(t *testing.T) {
		store := NewMemoryTokenStore()
		defer store.Close()

		token := &AccessToken{
			Token:     "stats-token",
			Type:      "Bearer",
			ClientID:  "client-1",
			UserID:    "user-1",
			Scope:     "read",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := store.StoreAccessToken(token)
		require.NoError(t, err)

		activeAccess, activeRefresh, activeCodes := store.GetStats()
		assert.Equal(t, 1, activeAccess)
		assert.Equal(t, 0, activeRefresh)
		assert.Equal(t, 0, activeCodes)
	})
}