// internal/auth/memory_store.go
package auth

import (
	"fmt"
	"sync"
	"time"
)

type MemoryTokenStore struct {
	accessTokens  map[string]*AccessToken
	refreshTokens map[string]*RefreshToken
	authCodes     map[string]*AuthorizationCode
	mu            sync.RWMutex
	cleanup       chan struct{}
}

func NewMemoryTokenStore() *MemoryTokenStore {
	ts := &MemoryTokenStore{
		accessTokens:  make(map[string]*AccessToken),
		refreshTokens: make(map[string]*RefreshToken),
		authCodes:     make(map[string]*AuthorizationCode),
		cleanup:       make(chan struct{}),
	}

	// Start cleanup routine
	go ts.cleanupExpiredTokens()

	return ts
}

func (ts *MemoryTokenStore) StoreAccessToken(token *AccessToken) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.accessTokens[token.Token] = token

	return nil
}

func (ts *MemoryTokenStore) GetAccessToken(token string) (*AccessToken, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	accessToken, exists := ts.accessTokens[token]
	if !exists {

		return nil, fmt.Errorf("token not found")
	}

	if accessToken.Revoked {

		return nil, fmt.Errorf("token revoked")
	}

	if time.Now().After(accessToken.ExpiresAt) {

		return nil, fmt.Errorf("token expired")
	}

	return accessToken, nil
}

func (ts *MemoryTokenStore) RevokeAccessToken(token string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	accessToken, exists := ts.accessTokens[token]
	if !exists {

		return fmt.Errorf("token not found")
	}

	accessToken.Revoked = true

	return nil
}

func (ts *MemoryTokenStore) CleanupExpiredTokens() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	now := time.Now()

	for token, accessToken := range ts.accessTokens {
		if accessToken.Revoked || now.After(accessToken.ExpiresAt) {
			delete(ts.accessTokens, token)
		}
	}

	for token, refreshToken := range ts.refreshTokens {
		if refreshToken.Revoked || now.After(refreshToken.ExpiresAt) {
			delete(ts.refreshTokens, token)
		}
	}

	for code, authCode := range ts.authCodes {
		if authCode.Used || now.After(authCode.ExpiresAt) {
			delete(ts.authCodes, code)
		}
	}
}

func (ts *MemoryTokenStore) cleanupExpiredTokens() {
	ticker := time.NewTicker(DefaultCleanupInterval * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ts.CleanupExpiredTokens()
		case <-ts.cleanup:

			return
		}
	}
}

func (ts *MemoryTokenStore) GetStats() (int, int, int) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	activeAccess := 0
	activeRefresh := 0
	activeCodes := 0

	now := time.Now()

	for _, token := range ts.accessTokens {
		if !token.Revoked && now.Before(token.ExpiresAt) {
			activeAccess++
		}
	}

	for _, token := range ts.refreshTokens {
		if !token.Revoked && now.Before(token.ExpiresAt) {
			activeRefresh++
		}
	}

	for _, code := range ts.authCodes {
		if !code.Used && now.Before(code.ExpiresAt) {
			activeCodes++
		}
	}

	return activeAccess, activeRefresh, activeCodes
}

func (ts *MemoryTokenStore) Close() {
	close(ts.cleanup)
}
