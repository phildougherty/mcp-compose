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

func (s *AuthorizationServer) HandleAuthorize(w http.ResponseWriter, r *http.Request) {
	// Enable CORS for oauth endpoints
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)

		return
	}

	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Parse authorization request
	authReq, err := s.parseAuthorizationRequest(r)
	if err != nil {
		s.logger.Error("Failed to parse authorization request: %v", err)
		s.redirectWithError(w, r, "", "invalid_request", err.Error(), "")

		return
	}

	s.logger.Info("Processing authorization request for client: %s", authReq.ClientID)

	// Validate client
	client, exists := s.GetClient(authReq.ClientID)
	if !exists {
		s.logger.Error("Unknown client: %s", authReq.ClientID)
		s.redirectWithError(w, r, authReq.RedirectURI, "invalid_client", "Unknown client", authReq.State)

		return
	}

	// Validate redirect URI
	if !s.validateRedirectURI(client, authReq.RedirectURI) {
		s.logger.Error("Invalid redirect URI: %s for client: %s", authReq.RedirectURI, authReq.ClientID)
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)

		return
	}

	// Validate response type
	if !contains(client.ResponseTypes, authReq.ResponseType) {
		s.redirectWithError(w, r, authReq.RedirectURI, "unsupported_response_type", "Response type not supported for this client", authReq.State)

		return
	}

	// Validate scope
	if authReq.Scope != "" && !s.validateScope(authReq.Scope) {
		s.redirectWithError(w, r, authReq.RedirectURI, "invalid_scope", "Invalid scope", authReq.State)

		return
	}

	// Handle GET request - show authorization page
	if r.Method == http.MethodGet {
		s.logger.Info("Showing authorization page for client: %s", authReq.ClientID)
		s.showAutoApprovalPage(w, r, authReq, client)

		return
	}

	// Handle POST request - process authorization
	if r.Method == http.MethodPost {
		s.logger.Info("Processing authorization POST for client: %s", authReq.ClientID)
		s.processAuthorization(w, r, authReq, client)

		return
	}
}

func (s *AuthorizationServer) showAutoApprovalPage(w http.ResponseWriter, _ *http.Request, authReq *AuthorizationRequest, client *OAuthClient) {
	html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Authorization Request</title>
    <style>
        body { font-family: Arial, sans-serif; max-width: 600px; margin: 50px auto; padding: 20px; }
        .auth-box { border: 1px solid #ddd; padding: 20px; border-radius: 5px; background: #f9f9f9; }
        .client-info { background: #e7f3ff; padding: 10px; margin: 10px 0; border-radius: 3px; }
        .scope-list { background: #fff; padding: 10px; margin: 10px 0; border: 1px solid #ddd; border-radius: 3px; }
        .buttons { margin: 20px 0; }
        button { padding: 10px 20px; margin: 5px; border: none; border-radius: 3px; cursor: pointer; font-size: 16px; }
        .approve { background: #28a745; color: white; }
        .deny { background: #dc3545; color: white; }
    </style>
</head>
<body>
    <div class="auth-box">
        <h2>Authorization Request</h2>
        <div class="client-info">
            <strong>Application:</strong> %s<br>
            <strong>Client ID:</strong> %s
        </div>
        <div class="scope-list">
            <strong>Requested Permissions:</strong><br>
            %s
        </div>
        <p>Do you want to authorize this application?</p>
        <form method="POST" action="/oauth/authorize">
            <input type="hidden" name="client_id" value="%s">
            <input type="hidden" name="redirect_uri" value="%s">
            <input type="hidden" name="response_type" value="%s">
            <input type="hidden" name="scope" value="%s">
            <input type="hidden" name="state" value="%s">
            <input type="hidden" name="code_challenge" value="%s">
            <input type="hidden" name="code_challenge_method" value="%s">
            <div class="buttons">
                <button type="submit" name="action" value="approve" class="approve">Approve</button>
                <button type="submit" name="action" value="deny" class="deny">Deny</button>
            </div>
        </form>
    </div>
</body>
</html>`,
		getClientDisplayName(client),
		client.ID,
		formatScopes(authReq.Scope),
		authReq.ClientID,
		authReq.RedirectURI,
		authReq.ResponseType,
		authReq.Scope,
		authReq.State,
		authReq.CodeChallenge,
		authReq.CodeChallengeMethod,
	)

	w.Header().Set("Content-Type", "text/html")
	if _, err := w.Write([]byte(html)); err != nil {
		s.logger.Error("Failed to write authorization form: %v", err)
	}
}

func (s *AuthorizationServer) processAuthorization(w http.ResponseWriter, r *http.Request, authReq *AuthorizationRequest, client *OAuthClient) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		s.logger.Error("Failed to parse authorization form: %v", err)
		s.redirectWithError(w, r, authReq.RedirectURI, "server_error", "Failed to parse form", authReq.State)

		return
	}

	action := r.Form.Get("action")
	s.logger.Info("Authorization action: %s for client: %s", action, authReq.ClientID)

	if action != "approve" {
		s.logger.Info("User denied authorization for client: %s", authReq.ClientID)
		s.redirectWithError(w, r, authReq.RedirectURI, "access_denied", "User denied authorization", authReq.State)

		return
	}

	// Generate authorization code
	// For demo purposes, use a static user ID. In production, get from authenticated session
	userID := "demo-user"

	s.logger.Info("Generating authorization code for client: %s, user: %s", authReq.ClientID, userID)

	s.mu.Lock()
	authCode, err := s.generateAuthorizationCode(
		authReq.ClientID,
		userID,
		authReq.RedirectURI,
		authReq.Scope,
		authReq.CodeChallenge,
		authReq.CodeChallengeMethod,
	)
	s.mu.Unlock()

	if err != nil {
		s.logger.Error("Failed to generate authorization code: %v", err)
		s.redirectWithError(w, r, authReq.RedirectURI, "server_error", "Failed to generate authorization code", authReq.State)

		return
	}

	s.logger.Info("Generated authorization code: %s for client: %s", authCode.Code, authReq.ClientID)

	// Redirect back to client with authorization code
	redirectURL, err := url.Parse(authReq.RedirectURI)
	if err != nil {
		s.logger.Error("Invalid redirect URI: %v", err)
		http.Error(w, "Invalid redirect URI", http.StatusBadRequest)

		return
	}

	query := redirectURL.Query()
	query.Set("code", authCode.Code)
	if authReq.State != "" {
		query.Set("state", authReq.State)
	}
	redirectURL.RawQuery = query.Encode()

	s.logger.Info("Authorization approved for client %s, redirecting to: %s", client.ID, redirectURL.String())
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// Helper functions
func getClientDisplayName(client *OAuthClient) string {
	if client.ClientName != "" {

		return client.ClientName
	}

	return client.ID
}

func formatScopes(scope string) string {
	if scope == "" {

		return "No specific permissions requested"
	}

	scopes := strings.Fields(scope)
	formatted := make([]string, len(scopes))
	for i, s := range scopes {
		switch s {
		case "mcp:*":
			formatted[i] = "• Full access to all MCP resources"
		case "mcp:tools":
			formatted[i] = "• Access to MCP tools"
		case "mcp:resources":
			formatted[i] = "• Access to MCP resources"
		case "mcp:prompts":
			formatted[i] = "• Access to MCP prompts"
		default:
			formatted[i] = "• " + s
		}
	}

	return strings.Join(formatted, "<br>")
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
	var query url.Values

	// Use tagged switch instead of if-else chain
	switch r.Method {
	case http.MethodGet:
		query = r.URL.Query()
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {

			return nil, fmt.Errorf("failed to parse form: %w", err)
		}
		query = r.Form
	default:

		return nil, fmt.Errorf("unsupported method: %s", r.Method)
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

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode JSON response: %v", err)
	}
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

// HandleUserInfo handles userinfo requests
func (s *AuthorizationServer) HandleUserInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Extract access token from Authorization header
	token := s.extractBearerToken(r)
	if token == "" {
		s.sendUserInfoError(w, "invalid_token", "Access token required")

		return
	}

	// Validate access token
	accessToken, err := s.ValidateAccessToken(token)
	if err != nil {
		s.sendUserInfoError(w, "invalid_token", err.Error())

		return
	}

	// Get client information
	client, exists := s.GetClient(accessToken.ClientID)
	if !exists {
		s.sendUserInfoError(w, "invalid_token", "Invalid client")

		return
	}

	// Build userinfo response
	userInfo := map[string]interface{}{
		"sub":        accessToken.UserID,
		"client_id":  accessToken.ClientID,
		"scope":      accessToken.Scope,
		"iat":        accessToken.CreatedAt.Unix(),
		"exp":        accessToken.ExpiresAt.Unix(),
		"token_type": "Bearer",
		"active":     true,
	}

	// Add client information if available
	if client.ClientName != "" {
		userInfo["client_name"] = client.ClientName
	}

	// Add custom claims if they exist
	if accessToken.Claims != nil {
		for key, value := range accessToken.Claims {
			// Don't override standard claims
			if key != "sub" && key != "iat" && key != "exp" && key != "client_id" {
				userInfo[key] = value
			}
		}
	}

	// Add MCP-specific information
	userInfo["mcp_server"] = "mcp-compose"
	userInfo["mcp_version"] = "1.0.0"

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	if err := json.NewEncoder(w).Encode(userInfo); err != nil {
		s.logger.Error("Failed to encode userinfo response: %v", err)
	}
}

// HandleRevoke handles token revocation requests
func (s *AuthorizationServer) HandleRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	// Parse request
	if err := r.ParseForm(); err != nil {
		s.sendRevokeError(w, "invalid_request", "Failed to parse request")

		return
	}

	token := r.Form.Get("token")
	tokenTypeHint := r.Form.Get("token_type_hint") // "access_token" or "refresh_token"
	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")

	if token == "" {
		s.sendRevokeError(w, "invalid_request", "token parameter is required")

		return
	}

	// Authenticate client if credentials provided
	var authenticatedClient *OAuthClient
	if clientID != "" {
		// Try HTTP Basic authentication first
		if clientSecret == "" {
			if username, password, ok := r.BasicAuth(); ok {
				clientID = username
				clientSecret = password
			}
		}

		client, err := s.ValidateClient(clientID, clientSecret)
		if err != nil {
			s.sendRevokeError(w, "invalid_client", err.Error())

			return
		}
		authenticatedClient = client
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	revoked := false

	// Try to revoke as access token first (or if hint suggests it)
	if tokenTypeHint == "" || tokenTypeHint == "access_token" {
		if accessToken, exists := s.accessTokens[token]; exists {
			// Check if client owns this token (if client authenticated)
			if authenticatedClient == nil || accessToken.ClientID == authenticatedClient.ID {
				accessToken.Revoked = true
				revoked = true
				s.logger.Info("Revoked access token for client: %s", accessToken.ClientID)

				// Also revoke associated refresh tokens
				for _, refreshToken := range s.refreshTokens {
					if refreshToken.ClientID == accessToken.ClientID && refreshToken.UserID == accessToken.UserID {
						refreshToken.Revoked = true
						s.logger.Info("Revoked associated refresh token for client: %s", refreshToken.ClientID)
					}
				}
			}
		}
	}

	// Try to revoke as refresh token if not found as access token
	if !revoked && (tokenTypeHint == "" || tokenTypeHint == "refresh_token") {
		if refreshToken, exists := s.refreshTokens[token]; exists {
			// Check if client owns this token (if client authenticated)
			if authenticatedClient == nil || refreshToken.ClientID == authenticatedClient.ID {
				refreshToken.Revoked = true
				revoked = true
				s.logger.Info("Revoked refresh token for client: %s", refreshToken.ClientID)

				// Also revoke associated access tokens
				for _, accessToken := range s.accessTokens {
					if accessToken.ClientID == refreshToken.ClientID && accessToken.UserID == refreshToken.UserID {
						accessToken.Revoked = true
						s.logger.Info("Revoked associated access token for client: %s", accessToken.ClientID)
					}
				}
			}
		}
	}

	// RFC 7009: The authorization server responds with HTTP status code 200
	// regardless of whether the token was successfully revoked
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if revoked {
		s.logger.Info("Token revocation successful")
	} else {
		s.logger.Info("Token revocation requested for unknown or unauthorized token")
	}

	// Return empty JSON object as per RFC 7009
	if _, err := w.Write([]byte("{}")); err != nil {
		s.logger.Error("Failed to write revocation response: %v", err)
	}
}

// Helper method to extract bearer token for userinfo
func (s *AuthorizationServer) extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {

		return ""
	}

	parts := strings.SplitN(authHeader, " ", AuthHeaderSplitParts)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {

		return ""
	}

	return parts[1]
}

// Helper method to send userinfo errors
func (s *AuthorizationServer) sendUserInfoError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer error="%s", error_description="%s"`, errorCode, description))
	w.WriteHeader(http.StatusUnauthorized)

	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode JSON response: %v", err)
	}
}

// Helper method to send revoke errors
func (s *AuthorizationServer) sendRevokeError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")

	var statusCode int
	switch errorCode {
	case "invalid_client":
		statusCode = http.StatusUnauthorized
		w.Header().Set("WWW-Authenticate", "Basic")
	case "invalid_request":
		statusCode = http.StatusBadRequest
	default:
		statusCode = http.StatusBadRequest
	}

	w.WriteHeader(statusCode)

	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("Failed to encode JSON response: %v", err)
	}
}
