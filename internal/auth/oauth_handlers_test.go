package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

func TestHandleAuthorize(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:            "test-client",
		ClientSecret:        "test-secret",
		RedirectURIs:        []string{"http://localhost:3000/callback"},
		GrantTypes:          []string{"authorization_code"},
		ResponseTypes:       []string{"code"},
		CodeChallengeMethod: "S256",
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("get_authorization_page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=test-client&redirect_uri=http://localhost:3000/callback&scope=read&state=random-state", nil)
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "Authorization Request") {
			t.Error("Response should contain authorization page")
		}

		if !strings.Contains(body, "test-client") {
			t.Error("Response should contain client ID")
		}
	})

	t.Run("post_authorization_approval", func(t *testing.T) {
		form := url.Values{}
		form.Set("client_id", "test-client")
		form.Set("redirect_uri", "http://localhost:3000/callback")
		form.Set("response_type", "code")
		form.Set("scope", "read")
		form.Set("state", "random-state")
		form.Set("action", "approve")

		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("Expected status 302, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		if location == "" {
			t.Fatal("Expected redirect location")
		}

		redirectURL, err := url.Parse(location)
		if err != nil {
			t.Fatalf("Failed to parse redirect URL: %v", err)
		}

		code := redirectURL.Query().Get("code")
		if code == "" {
			t.Error("Expected authorization code in redirect")
		}

		state := redirectURL.Query().Get("state")
		if state != "random-state" {
			t.Errorf("Expected state 'random-state', got %s", state)
		}
	})

	t.Run("post_authorization_denial", func(t *testing.T) {
		form := url.Values{}
		form.Set("client_id", "test-client")
		form.Set("redirect_uri", "http://localhost:3000/callback")
		form.Set("response_type", "code")
		form.Set("scope", "read")
		form.Set("state", "random-state")
		form.Set("action", "deny")

		req := httptest.NewRequest(http.MethodPost, "/oauth/authorize", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("Expected status 302, got %d", w.Code)
		}

		location := w.Header().Get("Location")
		redirectURL, err := url.Parse(location)
		if err != nil {
			t.Fatalf("Failed to parse redirect URL: %v", err)
		}

		errorParam := redirectURL.Query().Get("error")
		if errorParam != "access_denied" {
			t.Errorf("Expected error 'access_denied', got %s", errorParam)
		}
	})

	t.Run("invalid_client_id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=invalid-client&redirect_uri=http://localhost:3000/callback", nil)
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code != http.StatusFound {
			t.Errorf("Expected status 302 for invalid client, got %d", w.Code)
		}
	})

	t.Run("missing_required_parameters", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?client_id=test-client", nil)
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code == http.StatusOK {
			t.Error("Expected error for missing response_type")
		}
	})

	t.Run("options_method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/oauth/authorize", nil)
		w := httptest.NewRecorder()

		authServer.HandleAuthorize(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 for OPTIONS, got %d", w.Code)
		}
	})
}

func TestHandleToken(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
		GrantTypes:   []string{"authorization_code", "client_credentials", "refresh_token"},
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("authorization_code_grant", func(t *testing.T) {
		authServer.mu.Lock()
		authCode, _ := authServer.generateAuthorizationCode("test-client", "test-user", "http://localhost:3000/callback", "read write", "", "")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", authCode.Code)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")
		form.Set("redirect_uri", "http://localhost:3000/callback")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse token response: %v", err)
		}

		if response["access_token"] == nil {
			t.Error("Response should contain access_token")
		}

		if response["token_type"] != "Bearer" {
			t.Errorf("Expected token_type Bearer, got %v", response["token_type"])
		}

		if response["refresh_token"] == nil {
			t.Error("Response should contain refresh_token")
		}
	})

	t.Run("client_credentials_grant", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")
		form.Set("scope", "read")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse token response: %v", err)
		}

		if response["access_token"] == nil {
			t.Error("Response should contain access_token")
		}
	})

	t.Run("refresh_token_grant", func(t *testing.T) {
		authServer.mu.Lock()
		refreshToken, _ := authServer.generateRefreshToken("test-client", "test-user", "read write")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", refreshToken.Token)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse token response: %v", err)
		}

		if response["access_token"] == nil {
			t.Error("Response should contain new access_token")
		}

		if response["refresh_token"] == nil {
			t.Error("Response should contain new refresh_token")
		}
	})

	t.Run("invalid_grant_type", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "invalid_grant")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid_client_credentials", func(t *testing.T) {
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "wrong-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", w.Code)
		}
	})

	t.Run("expired_authorization_code", func(t *testing.T) {
		authServer.mu.Lock()
		code, _ := authServer.tokenGenerator.GenerateAuthorizationCode()
		expiredAuthCode := &AuthorizationCode{
			Code:        code,
			ClientID:    "test-client",
			UserID:      "test-user",
			RedirectURI: "http://localhost:3000/callback",
			Scope:       "read",
			ExpiresAt:   time.Now().Add(-1 * time.Hour),
			CreatedAt:   time.Now().Add(-2 * time.Hour),
		}
		authServer.authCodes[code] = expiredAuthCode
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", code)
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")
		form.Set("redirect_uri", "http://localhost:3000/callback")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for expired code, got %d", w.Code)
		}
	})
}

func TestHandleUserInfo(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
		ClientName:   "Test Application",
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("valid_access_token", func(t *testing.T) {
		authServer.mu.Lock()
		accessToken, _ := authServer.generateAccessToken("test-client", "test-user", "read write")
		authServer.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken.Token)
		w := httptest.NewRecorder()

		authServer.HandleUserInfo(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var response map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("Failed to parse userinfo response: %v", err)
		}

		if response["sub"] != "test-user" {
			t.Errorf("Expected sub 'test-user', got %v", response["sub"])
		}

		if response["client_id"] != "test-client" {
			t.Errorf("Expected client_id 'test-client', got %v", response["client_id"])
		}

		if response["scope"] != "read write" {
			t.Errorf("Expected scope 'read write', got %v", response["scope"])
		}

		if response["active"] != true {
			t.Error("Expected active to be true")
		}
	})

	t.Run("missing_authorization_header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
		w := httptest.NewRecorder()

		authServer.HandleUserInfo(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("invalid_access_token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		authServer.HandleUserInfo(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401, got %d", w.Code)
		}
	})

	t.Run("revoked_access_token", func(t *testing.T) {
		authServer.mu.Lock()
		accessToken, _ := authServer.generateAccessToken("test-client", "test-user", "read")
		accessToken.Revoked = true
		authServer.revokedTokens[accessToken.Token] = time.Now()
		authServer.mu.Unlock()

		req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken.Token)
		w := httptest.NewRecorder()

		authServer.HandleUserInfo(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for revoked token, got %d", w.Code)
		}
	})
}

func TestHandleRevoke(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("revoke_access_token", func(t *testing.T) {
		authServer.mu.Lock()
		accessToken, _ := authServer.generateAccessToken("test-client", "test-user", "read")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("token", accessToken.Token)
		form.Set("token_type_hint", "access_token")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		authServer.mu.RLock()
		if !accessToken.Revoked {
			t.Error("Access token should be marked as revoked")
		}

		if _, exists := authServer.revokedTokens[accessToken.Token]; !exists {
			t.Error("Token should be in revoked tokens list")
		}
		authServer.mu.RUnlock()
	})

	t.Run("revoke_refresh_token", func(t *testing.T) {
		authServer.mu.Lock()
		refreshToken, _ := authServer.generateRefreshToken("test-client", "test-user", "read")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("token", refreshToken.Token)
		form.Set("token_type_hint", "refresh_token")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		authServer.mu.RLock()
		if !refreshToken.Revoked {
			t.Error("Refresh token should be marked as revoked")
		}
		authServer.mu.RUnlock()
	})

	t.Run("revoke_with_basic_auth", func(t *testing.T) {
		authServer.mu.Lock()
		accessToken, _ := authServer.generateAccessToken("test-client", "test-user", "read")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("token", accessToken.Token)

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth("test-client", "test-secret")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}
	})

	t.Run("revoke_without_auth", func(t *testing.T) {
		authServer.mu.Lock()
		accessToken, _ := authServer.generateAccessToken("test-client", "test-user", "read")
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("token", accessToken.Token)

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 (per RFC 7009), got %d", w.Code)
		}
	})

	t.Run("revoke_invalid_token", func(t *testing.T) {
		form := url.Values{}
		form.Set("token", "invalid-token")
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200 even for invalid token, got %d", w.Code)
		}
	})

	t.Run("revoke_missing_token", func(t *testing.T) {
		form := url.Values{}
		form.Set("client_id", "test-client")
		form.Set("client_secret", "test-secret")

		req := httptest.NewRequest(http.MethodPost, "/oauth/revoke", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleRevoke(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing token, got %d", w.Code)
		}
	})
}

func TestPKCEFlow(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:            "pkce-client",
		RedirectURIs:        []string{"http://localhost:3000/callback"},
		GrantTypes:          []string{"authorization_code"},
		ResponseTypes:       []string{"code"},
		TokenEndpointAuth:   "none",
		CodeChallengeMethod: "S256",
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("s256_code_challenge", func(t *testing.T) {
		verifier := &DefaultCodeVerifier{}
		codeVerifier, err := verifier.GenerateCodeVerifier()
		if err != nil {
			t.Fatalf("Failed to generate code verifier: %v", err)
		}

		codeChallenge, err := verifier.GenerateCodeChallenge(codeVerifier, "S256")
		if err != nil {
			t.Fatalf("Failed to generate code challenge: %v", err)
		}

		authServer.mu.Lock()
		authCode, _ := authServer.generateAuthorizationCode(
			"pkce-client",
			"test-user",
			"http://localhost:3000/callback",
			"read",
			codeChallenge,
			"S256",
		)
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", authCode.Code)
		form.Set("client_id", "pkce-client")
		form.Set("redirect_uri", "http://localhost:3000/callback")
		form.Set("code_verifier", codeVerifier)

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid_code_verifier", func(t *testing.T) {
		verifier := &DefaultCodeVerifier{}
		codeChallenge, _ := verifier.GenerateCodeChallenge("correct-verifier", "S256")

		authServer.mu.Lock()
		authCode, _ := authServer.generateAuthorizationCode(
			"pkce-client",
			"test-user",
			"http://localhost:3000/callback",
			"read",
			codeChallenge,
			"S256",
		)
		authServer.mu.Unlock()

		form := url.Values{}
		form.Set("grant_type", "authorization_code")
		form.Set("code", authCode.Code)
		form.Set("client_id", "pkce-client")
		form.Set("redirect_uri", "http://localhost:3000/callback")
		form.Set("code_verifier", "wrong-verifier")

		req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()

		authServer.HandleToken(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for invalid verifier, got %d", w.Code)
		}
	})
}

func TestTokenCleanup(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("cleanup_expired_tokens", func(t *testing.T) {
		authServer.mu.Lock()

		expiredToken := &AccessToken{
			Token:     "expired",
			ClientID:  "client",
			UserID:    "user",
			Scope:     "read",
			ExpiresAt: time.Now().Add(-1 * time.Hour),
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}
		authServer.accessTokens["expired"] = expiredToken

		validToken := &AccessToken{
			Token:     "valid",
			ClientID:  "client",
			UserID:    "user",
			Scope:     "read",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
		}
		authServer.accessTokens["valid"] = validToken

		authServer.mu.Unlock()

		authServer.CleanupExpiredTokens()

		authServer.mu.RLock()
		if _, exists := authServer.accessTokens["expired"]; exists {
			t.Error("Expired token should be removed")
		}

		if _, exists := authServer.accessTokens["valid"]; !exists {
			t.Error("Valid token should not be removed")
		}
		authServer.mu.RUnlock()
	})

	t.Run("cleanup_revoked_tokens", func(t *testing.T) {
		authServer.mu.Lock()

		revokedToken := &AccessToken{
			Token:     "revoked",
			ClientID:  "client",
			UserID:    "user",
			Scope:     "read",
			ExpiresAt: time.Now().Add(1 * time.Hour),
			CreatedAt: time.Now(),
			Revoked:   true,
		}
		authServer.accessTokens["revoked"] = revokedToken

		authServer.mu.Unlock()

		authServer.CleanupExpiredTokens()

		authServer.mu.RLock()
		if _, exists := authServer.accessTokens["revoked"]; exists {
			t.Error("Revoked token should be removed")
		}
		authServer.mu.RUnlock()
	})

	t.Run("cleanup_old_revocation_list", func(t *testing.T) {
		authServer.mu.Lock()

		authServer.revokedTokens["old-token"] = time.Now().Add(-25 * time.Hour)
		authServer.revokedTokens["recent-token"] = time.Now()

		authServer.mu.Unlock()

		authServer.CleanupExpiredTokens()

		authServer.mu.RLock()
		if _, exists := authServer.revokedTokens["old-token"]; exists {
			t.Error("Old revoked token should be removed from list")
		}

		if _, exists := authServer.revokedTokens["recent-token"]; !exists {
			t.Error("Recent revoked token should remain in list")
		}
		authServer.mu.RUnlock()
	})
}

func TestHandleDiscovery(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer:                "https://test.mcp-compose.local",
		AuthorizationEndpoint: "/oauth/authorize",
		TokenEndpoint:         "/oauth/token",
		UserinfoEndpoint:      "/oauth/userinfo",
		RevocationEndpoint:    "/oauth/revoke",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	t.Run("get_discovery_metadata", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()

		authServer.HandleDiscovery(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var metadata AuthorizationServerConfig
		if err := json.Unmarshal(w.Body.Bytes(), &metadata); err != nil {
			t.Fatalf("Failed to parse discovery metadata: %v", err)
		}

		if metadata.Issuer != serverConfig.Issuer {
			t.Errorf("Expected issuer %s, got %s", serverConfig.Issuer, metadata.Issuer)
		}

		if metadata.TokenEndpoint != serverConfig.TokenEndpoint {
			t.Errorf("Expected token endpoint %s, got %s", serverConfig.TokenEndpoint, metadata.TokenEndpoint)
		}

		if len(metadata.GrantTypesSupported) == 0 {
			t.Error("Expected grant types to be populated")
		}
	})

	t.Run("discovery_method_not_allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-authorization-server", nil)
		w := httptest.NewRecorder()

		authServer.HandleDiscovery(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

func TestConcurrentOAuthTokenOperations(t *testing.T) {
	logger := logging.NewLogger("debug")
	serverConfig := &AuthorizationServerConfig{
		Issuer: "https://test.mcp-compose.local",
	}
	authServer := NewAuthorizationServer(serverConfig, logger)

	clientConfig := &OAuthConfig{
		ClientID:     "concurrent-client",
		ClientSecret: "secret",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}

	_, err := authServer.RegisterClient(clientConfig)
	if err != nil {
		t.Fatalf("Failed to register client: %v", err)
	}

	t.Run("concurrent_token_generation", func(t *testing.T) {
		const numTokens = 50
		tokens := make(chan string, numTokens)
		errors := make(chan error, numTokens)

		for i := 0; i < numTokens; i++ {
			go func(index int) {
				authServer.mu.Lock()
				token, err := authServer.generateAccessToken("concurrent-client", fmt.Sprintf("user-%d", index), "read")
				authServer.mu.Unlock()

				if err != nil {
					errors <- err

					return
				}
				tokens <- token.Token
			}(i)
		}

		generatedTokens := make(map[string]bool)
		for i := 0; i < numTokens; i++ {
			select {
			case token := <-tokens:
				if generatedTokens[token] {
					t.Errorf("Duplicate token generated: %s", token)
				}
				generatedTokens[token] = true
			case err := <-errors:
				t.Errorf("Error generating token: %v", err)
			}
		}

		if len(generatedTokens) != numTokens {
			t.Errorf("Expected %d unique tokens, got %d", numTokens, len(generatedTokens))
		}
	})
}