// internal/auth/handlers.go
package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HandleAuthorize handles authorization requests
func (s *AuthorizationServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request parameters
	req, err := s.parseAuthorizationRequest(r)
	if err != nil {
		s.redirectWithError(w, r, req.RedirectURI, "invalid_request", err.Error(), req.State)
		return
	}

	// Validate client
	client, exists := s.GetClient(req.ClientID)
	if !exists {
		s.redirectWithError(w, r, req.RedirectURI, "invalid_client", "Unknown client", req.State)
		return
	}

	// Validate redirect URI
	if !s.validateRedirectURI(client, req.RedirectURI) {
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)
		return
	}

	// Validate response type
	if !contains(client.ResponseTypes, req.ResponseType) {
		s.redirectWithError(w, r, req.RedirectURI, "unsupported_response_type", "Response type not allowed for this client", req.State)
		return
	}

	// Validate scope
	if req.Scope != "" && !s.validateScope(req.Scope) {
		s.redirectWithError(w, r, req.RedirectURI, "invalid_scope", "Invalid scope", req.State)
		return
	}

	// For simplicity, auto-approve for now
	// In production, show consent screen here
	userID := "system_user" // This would come from authentication

	// Generate authorization code
	code, err := s.generateAuthorizationCode(client.ID, userID, req.RedirectURI, req.Scope, req.CodeChallenge, req.CodeChallengeMethod)
	if err != nil {
		s.redirectWithError(w, r, req.RedirectURI, "server_error", "Failed to generate authorization code", req.State)
		return
	}

	// Redirect with code
	redirectURL, err := url.Parse(req.RedirectURI)
	if err != nil {
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)
		return
	}

	query := redirectURL.Query()
	query.Set("code", code.Code)
	if req.State != "" {
		query.Set("state", req.State)
	}
	redirectURL.RawQuery = query.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// HandleToken handles token requests
func (s *AuthorizationServer) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	if err := r.ParseForm(); err != nil {
		s.sendTokenError(w, "invalid_request", "Failed to parse request")
		return
	}

	grantType := r.Form.Get("grant_type")
	switch grantType {
	case "authorization_code":
		s.handleAuthorizationCodeGrant(w, r)
	case "client_credentials":
		s.handleClientCredentialsGrant(w, r)
	case "refresh_token":
		s.handleRefreshTokenGrant(w, r)
	default:
		s.sendTokenError(w, "unsupported_grant_type", "Grant type not supported")
	}
}

// HandleRegister handles dynamic client registration
func (s *AuthorizationServer) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if !s.dynamicClients {
		http.Error(w, "Dynamic client registration not supported", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config OAuthConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate registration request
	if len(config.RedirectURIs) == 0 {
		http.Error(w, "redirect_uris is required", http.StatusBadRequest)
		return
	}

	// Register client
	client, err := s.RegisterClient(&config)
	if err != nil {
		s.logger.Error("Failed to register client: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return client information
	response := map[string]interface{}{
		"client_id":                  client.ID,
		"client_secret":              client.Secret,
		"client_id_issued_at":        client.CreatedAt.Unix(),
		"redirect_uris":              client.RedirectURIs,
		"grant_types":                client.GrantTypes,
		"response_types":             client.ResponseTypes,
		"token_endpoint_auth_method": client.TokenEndpointAuthMethod,
	}

	if !client.ExpiresAt.IsZero() {
		response["client_secret_expires_at"] = client.ExpiresAt.Unix()
	} else {
		response["client_secret_expires_at"] = 0
	}

	if client.ClientName != "" {
		response["client_name"] = client.ClientName
	}
	if client.ClientURI != "" {
		response["client_uri"] = client.ClientURI
	}
	if client.LogoURI != "" {
		response["logo_uri"] = client.LogoURI
	}
	if client.Scope != "" {
		response["scope"] = client.Scope
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode registration response: %v", err)
	}
}

// AuthorizationRequest represents an authorization request
type AuthorizationRequest struct {
	ResponseType        string
	ClientID            string
	RedirectURI         string
	Scope               string
	State               string
	CodeChallenge       string
	CodeChallengeMethod string
	Nonce               string
}

func (s *AuthorizationServer) parseAuthorizationRequest(r *http.Request) (*AuthorizationRequest, error) {
	query := r.URL.Query()
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			return nil, fmt.Errorf("failed to parse form")
		}
		// Use form values for POST
		for key, values := range r.Form {
			if len(values) > 0 {
				query.Set(key, values[0])
			}
		}
	}

	req := &AuthorizationRequest{
		ResponseType:        query.Get("response_type"),
		ClientID:            query.Get("client_id"),
		RedirectURI:         query.Get("redirect_uri"),
		Scope:               query.Get("scope"),
		State:               query.Get("state"),
		CodeChallenge:       query.Get("code_challenge"),
		CodeChallengeMethod: query.Get("code_challenge_method"),
		Nonce:               query.Get("nonce"),
	}

	// Validate required parameters
	if req.ResponseType == "" {
		return nil, fmt.Errorf("response_type is required")
	}
	if req.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}

	// Set default PKCE method
	if req.CodeChallenge != "" && req.CodeChallengeMethod == "" {
		req.CodeChallengeMethod = "plain"
	}

	return req, nil
}

func (s *AuthorizationServer) handleAuthorizationCodeGrant(w http.ResponseWriter, r *http.Request) {
	code := r.Form.Get("code")
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")
	redirectURI := r.Form.Get("redirect_uri")
	codeVerifier := r.Form.Get("code_verifier")

	// Authenticate client
	if clientID == "" || clientSecret == "" {
		// Try HTTP Basic authentication
		username, password, ok := r.BasicAuth()
		if ok {
			clientID = username
			clientSecret = password
		}
	}

	client, err := s.ValidateClient(clientID, clientSecret)
	if err != nil {
		s.sendTokenError(w, "invalid_client", err.Error())
		return
	}

	// Validate authorization code
	s.mu.Lock()
	authCode, exists := s.authCodes[code]
	if !exists {
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Invalid authorization code")
		return
	}

	// Check expiration
	if time.Now().After(authCode.ExpiresAt) {
		delete(s.authCodes, code)
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Authorization code expired")
		return
	}

	// Validate client and redirect URI
	if authCode.ClientID != clientID {
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Authorization code was not issued to this client")
		return
	}

	if authCode.RedirectURI != redirectURI {
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Redirect URI mismatch")
		return
	}

	// Verify PKCE if used
	if authCode.Challenge != "" {
		if codeVerifier == "" {
			s.mu.Unlock()
			s.sendTokenError(w, "invalid_grant", "Code verifier required")
			return
		}

		if !s.codeVerifier.VerifyCodeChallenge(codeVerifier, authCode.Challenge, authCode.ChallengeMethod) {
			s.mu.Unlock()
			s.sendTokenError(w, "invalid_grant", "Invalid code verifier")
			return
		}
	}

	// Generate access token
	accessToken, err := s.generateAccessToken(client.ID, authCode.UserID, authCode.Scope)
	if err != nil {
		s.mu.Unlock()
		s.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	// Generate refresh token if supported
	var refreshToken *RefreshToken
	if contains(client.GrantTypes, "refresh_token") {
		refreshToken, err = s.generateRefreshToken(client.ID, authCode.UserID, authCode.Scope)
		if err != nil {
			s.mu.Unlock()
			s.sendTokenError(w, "server_error", "Failed to generate refresh token")
			return
		}
	}

	// Remove authorization code (one-time use)
	delete(s.authCodes, code)
	s.mu.Unlock()

	// Create response
	response := map[string]interface{}{
		"access_token": accessToken.Token,
		"token_type":   "Bearer",
		"expires_in":   int(s.tokenLifetime.Seconds()),
	}

	if refreshToken != nil {
		response["refresh_token"] = refreshToken.Token
	}

	if authCode.Scope != "" {
		response["scope"] = authCode.Scope
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode token response: %v", err)
	}
}

func (s *AuthorizationServer) handleClientCredentialsGrant(w http.ResponseWriter, r *http.Request) {
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")
	scope := r.Form.Get("scope")

	// Authenticate client
	if clientID == "" || clientSecret == "" {
		username, password, ok := r.BasicAuth()
		if ok {
			clientID = username
			clientSecret = password
		}
	}

	client, err := s.ValidateClient(clientID, clientSecret)
	if err != nil {
		s.sendTokenError(w, "invalid_client", err.Error())
		return
	}

	// Check if client credentials grant is supported
	if !contains(client.GrantTypes, "client_credentials") {
		s.sendTokenError(w, "unauthorized_client", "Client credentials grant not allowed for this client")
		return
	}

	// Validate scope
	if scope != "" && !s.validateScope(scope) {
		s.sendTokenError(w, "invalid_scope", "Invalid scope")
		return
	}

	// Generate access token (no user context for client credentials)
	accessToken, err := s.generateAccessToken(client.ID, "", scope)
	if err != nil {
		s.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	response := map[string]interface{}{
		"access_token": accessToken.Token,
		"token_type":   "Bearer",
		"expires_in":   int(s.tokenLifetime.Seconds()),
	}

	if scope != "" {
		response["scope"] = scope
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode token response: %v", err)
	}
}

func (s *AuthorizationServer) handleRefreshTokenGrant(w http.ResponseWriter, r *http.Request) {
	refreshTokenValue := r.Form.Get("refresh_token")
	scope := r.Form.Get("scope")
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")

	if refreshTokenValue == "" {
		s.sendTokenError(w, "invalid_request", "refresh_token is required")
		return
	}

	// Authenticate client
	if clientID == "" || clientSecret == "" {
		username, password, ok := r.BasicAuth()
		if ok {
			clientID = username
			clientSecret = password
		}
	}

	client, err := s.ValidateClient(clientID, clientSecret)
	if err != nil {
		s.sendTokenError(w, "invalid_client", err.Error())
		return
	}

	// Find refresh token
	s.mu.Lock()
	var refreshToken *RefreshToken
	for _, rt := range s.refreshTokens {
		if rt.Token == refreshTokenValue && rt.ClientID == clientID {
			refreshToken = rt
			break
		}
	}

	if refreshToken == nil {
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Invalid refresh token")
		return
	}

	// Check expiration
	if time.Now().After(refreshToken.ExpiresAt) {
		delete(s.refreshTokens, refreshToken.Token)
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_grant", "Refresh token expired")
		return
	}

	// Validate scope (requested scope must be subset of original)
	originalScope := refreshToken.Scope
	if scope != "" && !s.isScopeSubset(scope, originalScope) {
		s.mu.Unlock()
		s.sendTokenError(w, "invalid_scope", "Requested scope exceeds original scope")
		return
	}

	// Use original scope if none requested
	if scope == "" {
		scope = originalScope
	}

	// Generate new access token
	accessToken, err := s.generateAccessToken(client.ID, refreshToken.UserID, scope)
	if err != nil {
		s.mu.Unlock()
		s.sendTokenError(w, "server_error", "Failed to generate access token")
		return
	}

	// Optionally generate new refresh token (refresh token rotation)
	newRefreshToken, err := s.generateRefreshToken(client.ID, refreshToken.UserID, scope)
	if err != nil {
		s.mu.Unlock()
		s.sendTokenError(w, "server_error", "Failed to generate refresh token")
		return
	}

	// Remove old refresh token
	delete(s.refreshTokens, refreshToken.Token)
	s.mu.Unlock()

	response := map[string]interface{}{
		"access_token":  accessToken.Token,
		"token_type":    "Bearer",
		"expires_in":    int(s.tokenLifetime.Seconds()),
		"refresh_token": newRefreshToken.Token,
	}

	if scope != "" {
		response["scope"] = scope
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode token response: %v", err)
	}
}

func (s *AuthorizationServer) generateAuthorizationCode(clientID, userID, redirectURI, scope, challenge, challengeMethod string) (*AuthorizationCode, error) {
	code, err := s.tokenGenerator.GenerateAuthorizationCode()
	if err != nil {
		return nil, err
	}

	authCode := &AuthorizationCode{
		Code:            code,
		ClientID:        clientID,
		UserID:          userID,
		RedirectURI:     redirectURI,
		Scope:           scope,
		ExpiresAt:       time.Now().Add(s.authCodeLifetime),
		CreatedAt:       time.Now(),
		Challenge:       challenge,
		ChallengeMethod: challengeMethod,
	}

	s.authCodes[code] = authCode
	return authCode, nil
}

func (s *AuthorizationServer) generateAccessToken(clientID, userID, scope string) (*AccessToken, error) {
	token, err := s.tokenGenerator.GenerateAccessToken()
	if err != nil {
		return nil, err
	}

	accessToken := &AccessToken{
		Token:     token,
		Type:      "Bearer",
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(s.tokenLifetime),
		CreatedAt: time.Now(),
	}

	s.accessTokens[token] = accessToken
	return accessToken, nil
}

func (s *AuthorizationServer) generateRefreshToken(clientID, userID, scope string) (*RefreshToken, error) {
	token, err := s.tokenGenerator.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}

	refreshToken := &RefreshToken{
		Token:     token,
		ClientID:  clientID,
		UserID:    userID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(s.refreshLifetime),
		CreatedAt: time.Now(),
	}

	s.refreshTokens[token] = refreshToken
	return refreshToken, nil
}

func (s *AuthorizationServer) validateRedirectURI(client *OAuthClient, uri string) bool {
	for _, registeredURI := range client.RedirectURIs {
		if registeredURI == uri {
			return true
		}
	}
	return false
}

func (s *AuthorizationServer) validateScope(scope string) bool {
	scopes := strings.Fields(scope)
	for _, requestedScope := range scopes {
		valid := false
		for _, supportedScope := range s.supportedScopes {
			if supportedScope == requestedScope || supportedScope == "mcp:*" {
				valid = true
				break
			}
		}
		if !valid {
			return false
		}
	}
	return true
}

func (s *AuthorizationServer) isScopeSubset(requested, original string) bool {
	requestedScopes := strings.Fields(requested)
	originalScopes := strings.Fields(original)

	for _, reqScope := range requestedScopes {
		if !contains(originalScopes, reqScope) {
			return false
		}
	}
	return true
}

func (s *AuthorizationServer) sendTokenError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusBadRequest)

	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *AuthorizationServer) redirectWithError(w http.ResponseWriter, r *http.Request, redirectURI, errorCode, description, state string) {
	if redirectURI == "" {
		http.Error(w, fmt.Sprintf("%s: %s", errorCode, description), http.StatusBadRequest)
		return
	}

	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)
		return
	}

	query := redirectURL.Query()
	query.Set("error", errorCode)
	query.Set("error_description", description)
	if state != "" {
		query.Set("state", state)
	}
	redirectURL.RawQuery = query.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
