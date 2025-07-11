// internal/auth/token_store.go
package auth

import (
	"fmt"
	"sync"
	"time"
)

// TokenStore manages the lifecycle of OAuth tokens and codes in memory.
type TokenStore struct {
	accessTokens  map[string]*AccessToken
	refreshTokens map[string]*RefreshToken
	authCodes     map[string]*AuthorizationCode
	mu            sync.RWMutex
	stopChan      chan struct{}
}

// NewTokenStore creates and starts a new in-memory token store.
func NewTokenStore() *TokenStore {
	ts := &TokenStore{
		accessTokens:  make(map[string]*AccessToken),
		refreshTokens: make(map[string]*RefreshToken),
		authCodes:     make(map[string]*AuthorizationCode),
		stopChan:      make(chan struct{}),
	}
	go ts.startCleanupRoutine()

	return ts
}

// StoreAccessToken adds an access token to the store.
func (ts *TokenStore) StoreAccessToken(token *AccessToken) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.accessTokens[token.Token] = token
}

// GetAccessToken retrieves a valid access token.
func (ts *TokenStore) GetAccessToken(tokenStr string) (*AccessToken, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	token, ok := ts.accessTokens[tokenStr]
	if !ok || token.IsExpired() {

		return nil, fmt.Errorf("invalid or expired access token")
	}

	return token, nil
}

// StoreRefreshToken adds a refresh token to the store.
func (ts *TokenStore) StoreRefreshToken(token *RefreshToken) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.refreshTokens[token.Token] = token
}

// GetRefreshToken retrieves a valid refresh token.
func (ts *TokenStore) GetRefreshToken(tokenStr string) (*RefreshToken, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	token, ok := ts.refreshTokens[tokenStr]
	if !ok || token.IsExpired() {

		return nil, fmt.Errorf("invalid or expired refresh token")
	}

	return token, nil
}

// StoreAuthorizationCode adds an authorization code to the store.
func (ts *TokenStore) StoreAuthorizationCode(code *AuthorizationCode) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.authCodes[code.Code] = code
}

// GetAndUseAuthorizationCode retrieves a valid auth code and marks it as used.
func (ts *TokenStore) GetAndUseAuthorizationCode(codeStr string) (*AuthorizationCode, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	code, ok := ts.authCodes[codeStr]
	if !ok || code.IsExpired() || code.Used {

		return nil, fmt.Errorf("invalid, expired, or already used authorization code")
	}
	code.Used = true
	// We leave the used code for a short time to detect replay attacks,
	// the cleanup routine will remove it.

	return code, nil
}

// RevokeAccessToken marks an access token as revoked.
func (ts *TokenStore) RevokeAccessToken(tokenStr string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if token, ok := ts.accessTokens[tokenStr]; ok {
		token.Revoked = true
	}
}

// RevokeRefreshToken marks a refresh token as revoked.
func (ts *TokenStore) RevokeRefreshToken(tokenStr string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if token, ok := ts.refreshTokens[tokenStr]; ok {
		token.Revoked = true
	}
}

// startCleanupRoutine periodically cleans expired tokens.
func (ts *TokenStore) startCleanupRoutine() {
	ticker := time.NewTicker(DefaultCleanupInterval * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ts.performCleanup()
		case <-ts.stopChan:

			return
		}
	}
}

// performCleanup iterates and removes expired entries.
func (ts *TokenStore) performCleanup() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	// Remove the unused 'now' variable and use IsExpired() methods directly
	for k, v := range ts.accessTokens {
		if v.IsExpired() {
			delete(ts.accessTokens, k)
		}
	}
	for k, v := range ts.refreshTokens {
		if v.IsExpired() {
			delete(ts.refreshTokens, k)
		}
	}
	for k, v := range ts.authCodes {
		if v.IsExpired() {
			delete(ts.authCodes, k)
		}
	}
}

// Close stops the cleanup routine.
func (ts *TokenStore) Close() {
	close(ts.stopChan)
}
