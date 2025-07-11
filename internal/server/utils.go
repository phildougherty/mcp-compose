package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"mcpcompose/internal/auth"
	"mcpcompose/internal/config"
	"mcpcompose/internal/logging"
)

func (h *ProxyHandler) startConnectionMaintenance() {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.maintainStdioConnections()
				h.maintainHttpConnections()
				h.maintainSSEConnections()
				h.maintainEnhancedSSEConnections()
			case <-h.ctx.Done():
				return
			}
		}
	}()
}

func (h *ProxyHandler) startOAuthTokenCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				h.authServer.CleanupExpiredTokens()
			case <-h.ctx.Done():
				return
			}
		}
	}()
	h.logger.Info("OAuth token cleanup scheduled every 5 minutes")
}

func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Try X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func getServerNameFromPath(path string) string {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	if len(parts) > 0 && parts[0] != "" && parts[0] != "api" {
		return parts[0]
	}
	return "proxy"
}

func initializeOAuth(oauthConfig *config.OAuthConfig, logger *logging.Logger) (*auth.AuthorizationServer, *auth.AuthenticationMiddleware, *auth.ResourceMetadataHandler) {
	// Use the issuer from config, with a sensible default for container environments
	defaultIssuer := "http://mcp-compose-http-proxy:9876"
	if oauthConfig.Issuer != "" {
		defaultIssuer = oauthConfig.Issuer
	}

	// Create authorization server config
	serverConfig := &auth.AuthorizationServerConfig{
		Issuer:                                 defaultIssuer, // Use config value or container-aware default
		AuthorizationEndpoint:                  "/oauth/authorize",
		TokenEndpoint:                          "/oauth/token",
		UserinfoEndpoint:                       "/oauth/userinfo",
		RevocationEndpoint:                     "/oauth/revoke",
		RegistrationEndpoint:                   "/oauth/register",
		ScopesSupported:                        []string{"mcp:*", "mcp:tools", "mcp:resources", "mcp:prompts"},
		ResponseTypesSupported:                 []string{"code"},
		GrantTypesSupported:                    []string{"authorization_code", "client_credentials", "refresh_token"},
		TokenEndpointAuthMethodsSupported:      []string{"client_secret_post", "client_secret_basic", "none"},
		RevocationEndpointAuthMethodsSupported: []string{"client_secret_post", "client_secret_basic", "none"},
		CodeChallengeMethodsSupported:          []string{"plain", "S256"},
	}

	// Apply any additional config overrides
	if len(oauthConfig.ScopesSupported) > 0 {
		serverConfig.ScopesSupported = oauthConfig.ScopesSupported
	}

	logger.Info("OAuth server initialized with issuer: %s", serverConfig.Issuer)

	authServer := auth.NewAuthorizationServer(serverConfig, logger)
	authMiddleware := auth.NewAuthenticationMiddleware(authServer)

	// Create resource metadata handler
	authServers := []string{serverConfig.Issuer}
	resourceMeta := auth.NewResourceMetadataHandler(authServers, serverConfig.ScopesSupported)

	return authServer, authMiddleware, resourceMeta
}

// authenticateRequest handles authentication for server requests
func (h *ProxyHandler) authenticateRequest(w http.ResponseWriter, r *http.Request, serverName string, instance *ServerInstance) bool {
	// Skip authentication for OPTIONS requests
	if r.Method == "OPTIONS" {
		return true
	}

	var authenticatedViaOAuth bool
	var authenticatedViaAPIKey bool
	var requiresAuth bool

	// Determine if authentication is required
	apiKeyToCheck := h.getAPIKeyToCheck()
	oauthRequired := h.oauthEnabled && instance.Config.Authentication != nil && instance.Config.Authentication.Enabled
	apiKeyRequired := apiKeyToCheck != ""

	// Check if any authentication is required
	requiresAuth = oauthRequired || apiKeyRequired

	// If no authentication is configured, allow access
	if !requiresAuth {
		return true
	}

	// Extract token from Authorization header
	token := h.extractBearerToken(r)
	if token == "" {
		if requiresAuth && (instance.Config.Authentication == nil || !instance.Config.Authentication.OptionalAuth) {
			h.sendAuthenticationError(w, "missing_token", "Access token required")
			return false
		}
		return true // Allow if no auth required or optional auth
	}

	// Try OAuth authentication first (if enabled and configured)
	if h.oauthEnabled && h.authServer != nil {
		accessToken, err := h.validateOAuthToken(token)
		if err == nil && accessToken != nil {
			// OAuth token is valid
			// Check server-specific OAuth scope requirements
			if instance.Config.Authentication != nil && instance.Config.Authentication.RequiredScope != "" {
				if !h.hasRequiredScope(accessToken.Scope, instance.Config.Authentication.RequiredScope) {
					h.sendOAuthError(w, "insufficient_scope", "Required scope not granted: "+instance.Config.Authentication.RequiredScope)
					return false
				}
			}
			// Add OAuth context to request
			client, _ := h.authServer.GetClient(accessToken.ClientID)
			ctx := context.WithValue(r.Context(), auth.ClientContextKey, client)
			ctx = context.WithValue(ctx, auth.TokenContextKey, accessToken)
			ctx = context.WithValue(ctx, auth.UserContextKey, accessToken.UserID)
			ctx = context.WithValue(ctx, auth.ScopeContextKey, accessToken.Scope)
			ctx = context.WithValue(ctx, auth.AuthTypeContextKey, "oauth")
			*r = *r.WithContext(ctx)
			h.logger.Debug("Request authenticated via OAuth for server %s (scope: %s)", serverName, accessToken.Scope)
			return true
		}
		// OAuth validation failed, but don't return error yet - try API key fallback
	}

	// Try API key authentication if not authenticated via OAuth
	if !authenticatedViaOAuth && apiKeyToCheck != "" {
		if token == apiKeyToCheck {
			// Add API key context
			ctx := context.WithValue(r.Context(), auth.AuthTypeContextKey, "api_key")
			*r = *r.WithContext(ctx)
			h.logger.Debug("Request authenticated via API key for server %s", serverName)
			return true
		}
	}

	// Check if API key fallback is allowed for OAuth-configured servers
	if oauthRequired && !authenticatedViaOAuth {
		// Check if server allows API key fallback
		allowAPIKey := instance.Config.Authentication == nil ||
			instance.Config.Authentication.AllowAPIKey == nil ||
			*instance.Config.Authentication.AllowAPIKey

		if !allowAPIKey {
			h.sendOAuthError(w, "invalid_token", "OAuth authentication required (API key not allowed)")
			return false
		}
	}

	// Authentication failed
	if requiresAuth && !authenticatedViaOAuth && !authenticatedViaAPIKey {
		if h.oauthEnabled {
			h.sendOAuthError(w, "invalid_token", "Invalid access token or API key")
		} else {
			h.sendAuthenticationError(w, "invalid_token", "Invalid API key")
		}
		return false
	}

	// Check if server requires authentication but none was provided
	if oauthRequired && !instance.Config.Authentication.OptionalAuth && !authenticatedViaOAuth && !authenticatedViaAPIKey {
		h.sendOAuthError(w, "access_denied", "Authentication required for this server")
		return false
	}

	return true
}

// Helper methods for authentication
func (h *ProxyHandler) extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return ""
	}
	return parts[1]
}

func (h *ProxyHandler) validateOAuthToken(token string) (*auth.AccessToken, error) {
	if h.authServer == nil {
		return nil, fmt.Errorf("OAuth not enabled")
	}
	return h.authServer.ValidateAccessToken(token)
}

func (h *ProxyHandler) hasRequiredScope(tokenScope, requiredScope string) bool {
	if h.authServer == nil {
		return false
	}
	return h.authServer.HasScope(tokenScope, requiredScope)
}

func (h *ProxyHandler) getAPIKeyToCheck() string {
	var apiKeyToCheck string
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.ProxyAuth.Enabled {
		apiKeyToCheck = h.Manager.config.ProxyAuth.APIKey
	}
	if h.APIKey != "" {
		apiKeyToCheck = h.APIKey
	}
	return apiKeyToCheck
}

func (h *ProxyHandler) sendOAuthError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="mcp-compose"`)
	w.WriteHeader(http.StatusUnauthorized)
	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (h *ProxyHandler) sendAuthenticationError(w http.ResponseWriter, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	if errorCode == "missing_token" {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.WriteHeader(http.StatusUnauthorized)
	response := map[string]string{
		"error":             errorCode,
		"error_description": description,
	}
	_ = json.NewEncoder(w).Encode(response)
}

func (h *ProxyHandler) registerDefaultOAuthClients() {
	if !h.oauthEnabled || h.authServer == nil {
		return
	}

	// Register default test client with both proxy and dashboard callbacks
	testClientConfig := &auth.OAuthConfig{
		ClientID:     "HFakeCpMUQnRX_m5HJKamRjU_vufUnNbG4xWpmUyvzo",
		ClientSecret: "test-secret-123",
		RedirectURIs: []string{
			"http://desk:3111/oauth/callback",           // Dashboard callback
			"http://192.168.86.201:3111/oauth/callback", // External dashboard callback
			"http://desk:9876/oauth/callback",           // Proxy callback (fallback)
			"http://192.168.86.201:9876/oauth/callback", // External proxy callback
		},
		GrantTypes:        []string{"authorization_code", "client_credentials", "refresh_token"},
		ResponseTypes:     []string{"code"},
		Scope:             "mcp:* mcp:tools mcp:resources mcp:prompts",
		ClientName:        "Testing",
		TokenEndpointAuth: "client_secret_post",
	}

	_, err := h.authServer.RegisterClient(testClientConfig)
	if err != nil {
		h.logger.Warning("Failed to register default test client: %v", err)
	} else {
		h.logger.Info("Registered default OAuth test client with dashboard callback support")
	}

	// Register any clients from config
	if h.Manager != nil && h.Manager.config != nil && h.Manager.config.OAuthClients != nil {
		for name, clientConfig := range h.Manager.config.OAuthClients {
			// Handle client secret pointer properly
			var clientSecret string
			if clientConfig.ClientSecret != nil {
				clientSecret = *clientConfig.ClientSecret
			}

			oauthConfig := &auth.OAuthConfig{
				ClientID:      clientConfig.ClientID,
				ClientSecret:  clientSecret, // Now using the dereferenced value
				RedirectURIs:  clientConfig.RedirectURIs,
				GrantTypes:    clientConfig.GrantTypes,
				ResponseTypes: []string{"code"},
				Scope:         strings.Join(clientConfig.Scopes, " "),
				ClientName:    clientConfig.Name,
			}

			if clientConfig.PublicClient {
				oauthConfig.TokenEndpointAuth = "none"
			} else {
				oauthConfig.TokenEndpointAuth = "client_secret_post"
			}

			_, err := h.authServer.RegisterClient(oauthConfig)
			if err != nil {
				h.logger.Warning("Failed to register OAuth client %s: %v", name, err)
			} else {
				h.logger.Info("Registered OAuth client: %s", name)
			}
		}
	}
}
