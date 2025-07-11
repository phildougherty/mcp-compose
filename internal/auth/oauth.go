// internal/auth/oauth.go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	// Random string generation lengths
	AuthCodeLength          = 32
	AccessTokenLength       = 64
	RefreshTokenLength      = 64
	ClientIDLength          = 40
	ClientSecretLength      = 8
	StateLength             = 32
	NonceLength             = 32
	PKCECodeVerifierLength  = 64
	PKCECodeChallengeLength = 128

	// Auth timing constants
	AuthCodeLifetimeMinutes = 10
	DefaultCleanupInterval  = 5 // minutes

	// String split parameter
	AuthHeaderSplitParts = 2
)

// OAuthConfig represents OAuth 2.1 configuration
type OAuthConfig struct {
	ClientID            string   `json:"client_id" yaml:"client_id"`
	ClientSecret        string   `json:"client_secret,omitempty" yaml:"client_secret,omitempty"`
	RedirectURIs        []string `json:"redirect_uris" yaml:"redirect_uris"`
	GrantTypes          []string `json:"grant_types" yaml:"grant_types"`
	ResponseTypes       []string `json:"response_types" yaml:"response_types"`
	Scope               string   `json:"scope,omitempty" yaml:"scope,omitempty"`
	TokenEndpointAuth   string   `json:"token_endpoint_auth_method" yaml:"token_endpoint_auth_method"`
	ClientName          string   `json:"client_name,omitempty" yaml:"client_name,omitempty"`
	ClientURI           string   `json:"client_uri,omitempty" yaml:"client_uri,omitempty"`
	LogoURI             string   `json:"logo_uri,omitempty" yaml:"logo_uri,omitempty"`
	TosURI              string   `json:"tos_uri,omitempty" yaml:"tos_uri,omitempty"`
	PolicyURI           string   `json:"policy_uri,omitempty" yaml:"policy_uri,omitempty"`
	SoftwareID          string   `json:"software_id,omitempty" yaml:"software_id,omitempty"`
	SoftwareVersion     string   `json:"software_version,omitempty" yaml:"software_version,omitempty"`
	CodeChallengeMethod string   `json:"code_challenge_method,omitempty" yaml:"code_challenge_method,omitempty"`
}

// AuthorizationServer implements OAuth 2.1 authorization server
type AuthorizationServer struct {
	config           *AuthorizationServerConfig
	clients          map[string]*OAuthClient
	authCodes        map[string]*AuthorizationCode
	accessTokens     map[string]*AccessToken
	refreshTokens    map[string]*RefreshToken
	deviceCodes      map[string]*DeviceCode
	mu               sync.RWMutex
	logger           *logging.Logger
	tokenGenerator   TokenGenerator
	codeVerifier     CodeVerifier
	dynamicClients   bool
	supportedScopes  []string
	authCodeLifetime time.Duration
	tokenLifetime    time.Duration
	refreshLifetime  time.Duration
}

// AuthorizationServerConfig contains server configuration
type AuthorizationServerConfig struct {
	Issuer                                 string   `json:"issuer" yaml:"issuer"`
	AuthorizationEndpoint                  string   `json:"authorization_endpoint" yaml:"authorization_endpoint"`
	TokenEndpoint                          string   `json:"token_endpoint" yaml:"token_endpoint"`
	UserinfoEndpoint                       string   `json:"userinfo_endpoint" yaml:"userinfo_endpoint"`     // Add this
	RevocationEndpoint                     string   `json:"revocation_endpoint" yaml:"revocation_endpoint"` // Add this
	JWKSUri                                string   `json:"jwks_uri,omitempty" yaml:"jwks_uri,omitempty"`
	RegistrationEndpoint                   string   `json:"registration_endpoint,omitempty" yaml:"registration_endpoint,omitempty"`
	ScopesSupported                        []string `json:"scopes_supported,omitempty" yaml:"scopes_supported,omitempty"`
	ResponseTypesSupported                 []string `json:"response_types_supported" yaml:"response_types_supported"`
	ResponseModesSupported                 []string `json:"response_modes_supported,omitempty" yaml:"response_modes_supported,omitempty"`
	GrantTypesSupported                    []string `json:"grant_types_supported" yaml:"grant_types_supported"`
	TokenEndpointAuthMethodsSupported      []string `json:"token_endpoint_auth_methods_supported" yaml:"token_endpoint_auth_methods_supported"`
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported,omitempty" yaml:"revocation_endpoint_auth_methods_supported,omitempty"` // Add this
	CodeChallengeMethodsSupported          []string `json:"code_challenge_methods_supported" yaml:"code_challenge_methods_supported"`
	ServiceDocumentation                   string   `json:"service_documentation,omitempty" yaml:"service_documentation,omitempty"`
	UILocalesSupported                     []string `json:"ui_locales_supported,omitempty" yaml:"ui_locales_supported,omitempty"`
	OpPolicyURI                            string   `json:"op_policy_uri,omitempty" yaml:"op_policy_uri,omitempty"`
	OpTosURI                               string   `json:"op_tos_uri,omitempty" yaml:"op_tos_uri,omitempty"`
	DeviceAuthorizationEndpoint            string   `json:"device_authorization_endpoint,omitempty" yaml:"device_authorization_endpoint,omitempty"`
}

// OAuthClient represents a registered OAuth client
type OAuthClient struct {
	ID                      string    `json:"client_id"`
	Secret                  string    `json:"client_secret,omitempty"`
	RedirectURIs            []string  `json:"redirect_uris"`
	GrantTypes              []string  `json:"grant_types"`
	ResponseTypes           []string  `json:"response_types"`
	Scope                   string    `json:"scope,omitempty"`
	ClientName              string    `json:"client_name,omitempty"`
	ClientURI               string    `json:"client_uri,omitempty"`
	LogoURI                 string    `json:"logo_uri,omitempty"`
	TosURI                  string    `json:"tos_uri,omitempty"`
	PolicyURI               string    `json:"policy_uri,omitempty"`
	TokenEndpointAuthMethod string    `json:"token_endpoint_auth_method"`
	CreatedAt               time.Time `json:"client_id_issued_at"`
	ExpiresAt               time.Time `json:"client_secret_expires_at,omitempty"`
	SoftwareID              string    `json:"software_id,omitempty"`
	SoftwareVersion         string    `json:"software_version,omitempty"`
	CodeChallengeMethod     string    `json:"code_challenge_method,omitempty"`
	Public                  bool      `json:"public"`
}

// AuthorizationCode represents an authorization code
type AuthorizationCode struct {
	Code            string                 `json:"code"`
	ClientID        string                 `json:"client_id"`
	UserID          string                 `json:"user_id"`
	RedirectURI     string                 `json:"redirect_uri"`
	Scope           string                 `json:"scope"`
	ExpiresAt       time.Time              `json:"expires_at"`
	CreatedAt       time.Time              `json:"created_at"`
	Challenge       string                 `json:"code_challenge,omitempty"`
	ChallengeMethod string                 `json:"code_challenge_method,omitempty"`
	State           string                 `json:"state,omitempty"`
	Nonce           string                 `json:"nonce,omitempty"`
	Claims          map[string]interface{} `json:"claims,omitempty"`
	Used            bool                   `json:"used"`
}

// IsExpired checks if the authorization code is expired
func (c *AuthorizationCode) IsExpired() bool {

	return time.Now().After(c.ExpiresAt)
}

// AccessToken represents an access token
type AccessToken struct {
	Token     string                 `json:"access_token"`
	Type      string                 `json:"token_type"`
	ClientID  string                 `json:"client_id"`
	UserID    string                 `json:"user_id"`
	Scope     string                 `json:"scope"`
	ExpiresAt time.Time              `json:"expires_at"`
	CreatedAt time.Time              `json:"created_at"`
	Claims    map[string]interface{} `json:"claims,omitempty"`
	Revoked   bool                   `json:"revoked"`
}

// IsExpired checks if the access token is expired
func (t *AccessToken) IsExpired() bool {

	return time.Now().After(t.ExpiresAt)
}

// RefreshToken represents a refresh token
type RefreshToken struct {
	Token     string    `json:"refresh_token"`
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

type TokenInfo struct {
	ClientID  string    `json:"client_id"`
	UserID    string    `json:"user_id"`
	Scope     string    `json:"scope"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	Revoked   bool      `json:"revoked"`
}

// IsExpired checks if the refresh token is expired
func (t *RefreshToken) IsExpired() bool {

	return time.Now().After(t.ExpiresAt)
}

// DeviceCode represents a device authorization code
type DeviceCode struct {
	DeviceCode      string    `json:"device_code"`
	UserCode        string    `json:"user_code"`
	VerificationURI string    `json:"verification_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
	Interval        int       `json:"interval"`
	ClientID        string    `json:"client_id"`
	Scope           string    `json:"scope"`
	UserID          string    `json:"user_id,omitempty"`
	Authorized      bool      `json:"authorized"`
}

// TokenGenerator interface for generating tokens
type TokenGenerator interface {
	GenerateAuthorizationCode() (string, error)
	GenerateAccessToken() (string, error)
	GenerateRefreshToken() (string, error)
	GenerateDeviceCode() (string, error)
	GenerateUserCode() (string, error)
	GenerateState() (string, error)
	GenerateClientID() (string, error)
	GenerateClientSecret() (string, error)
}

// CodeVerifier interface for PKCE
type CodeVerifier interface {
	VerifyCodeChallenge(verifier, challenge, method string) bool
	GenerateCodeVerifier() (string, error)
	GenerateCodeChallenge(verifier, method string) (string, error)
}

// DefaultTokenGenerator implements TokenGenerator
type DefaultTokenGenerator struct{}

// GenerateAuthorizationCode generates an authorization code
func (g *DefaultTokenGenerator) GenerateAuthorizationCode() (string, error) {

	return generateRandomString(AuthCodeLength)
}

// GenerateAccessToken generates an access token
func (g *DefaultTokenGenerator) GenerateAccessToken() (string, error) {

	return generateRandomString(AccessTokenLength)
}

// GenerateRefreshToken generates a refresh token
func (g *DefaultTokenGenerator) GenerateRefreshToken() (string, error) {

	return generateRandomString(AccessTokenLength)
}

// GenerateDeviceCode generates a device code
func (g *DefaultTokenGenerator) GenerateDeviceCode() (string, error) {

	return generateRandomString(ClientIDLength)
}

// GenerateUserCode generates a user code
func (g *DefaultTokenGenerator) GenerateUserCode() (string, error) {
	// Generate 8-character user code with only uppercase letters and numbers
	chars := "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // Exclude confusing chars

	return generateRandomStringFromSet(ClientSecretLength, chars)
}

// GenerateState generates a state parameter
func (g *DefaultTokenGenerator) GenerateState() (string, error) {

	return generateRandomString(AuthCodeLength)
}

// GenerateClientID generates a client ID
func (g *DefaultTokenGenerator) GenerateClientID() (string, error) {

	return generateRandomString(AuthCodeLength)
}

// GenerateClientSecret generates a client secret
func (g *DefaultTokenGenerator) GenerateClientSecret() (string, error) {

	return generateRandomString(AccessTokenLength)
}

// DefaultCodeVerifier implements CodeVerifier
type DefaultCodeVerifier struct{}

// VerifyCodeChallenge verifies a PKCE code challenge
func (v *DefaultCodeVerifier) VerifyCodeChallenge(verifier, challenge, method string) bool {
	switch method {
	case "plain":

		return verifier == challenge
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		expected := base64.RawURLEncoding.EncodeToString(hash[:])

		return expected == challenge
	default:

		return false
	}
}

// GenerateCodeVerifier generates a PKCE code verifier
func (v *DefaultCodeVerifier) GenerateCodeVerifier() (string, error) {
	// RFC 7636: 43-128 characters, A-Z, a-z, 0-9, -, ., _, ~
	chars := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"

	return generateRandomStringFromSet(PKCECodeChallengeLength, chars)
}

// GenerateCodeChallenge generates a PKCE code challenge
func (v *DefaultCodeVerifier) GenerateCodeChallenge(verifier, method string) (string, error) {
	switch method {
	case "plain":

		return verifier, nil
	case "S256":
		hash := sha256.Sum256([]byte(verifier))

		return base64.RawURLEncoding.EncodeToString(hash[:]), nil
	default:

		return "", fmt.Errorf("unsupported code challenge method: %s", method)
	}
}

// NewAuthorizationServer creates a new OAuth 2.1 authorization server
func NewAuthorizationServer(config *AuthorizationServerConfig, logger *logging.Logger) *AuthorizationServer {
	if config.AuthorizationEndpoint == "" {
		config.AuthorizationEndpoint = "/oauth/authorize"
	}
	if config.TokenEndpoint == "" {
		config.TokenEndpoint = "/oauth/token"
	}
	if config.RegistrationEndpoint == "" {
		config.RegistrationEndpoint = "/oauth/register"
	}
	if config.UserinfoEndpoint == "" {
		config.UserinfoEndpoint = "/oauth/userinfo"
	}
	if config.RevocationEndpoint == "" {
		config.RevocationEndpoint = "/oauth/revoke"
	}
	if len(config.ResponseTypesSupported) == 0 {
		config.ResponseTypesSupported = []string{"code"}
	}
	if len(config.GrantTypesSupported) == 0 {
		config.GrantTypesSupported = []string{"authorization_code", "client_credentials", "refresh_token"}
	}
	if len(config.TokenEndpointAuthMethodsSupported) == 0 {
		config.TokenEndpointAuthMethodsSupported = []string{"client_secret_post", "client_secret_basic", "none"}
	}
	if len(config.CodeChallengeMethodsSupported) == 0 {
		config.CodeChallengeMethodsSupported = []string{"plain", "S256"}
	}
	if len(config.ScopesSupported) == 0 {
		config.ScopesSupported = []string{"mcp:*", "mcp:tools", "mcp:resources", "mcp:prompts"}
	}

	return &AuthorizationServer{
		config:           config,
		clients:          make(map[string]*OAuthClient),
		authCodes:        make(map[string]*AuthorizationCode),
		accessTokens:     make(map[string]*AccessToken),
		refreshTokens:    make(map[string]*RefreshToken),
		deviceCodes:      make(map[string]*DeviceCode),
		logger:           logger,
		tokenGenerator:   &DefaultTokenGenerator{},
		codeVerifier:     &DefaultCodeVerifier{},
		dynamicClients:   true,
		supportedScopes:  config.ScopesSupported,
		authCodeLifetime: AuthCodeLifetimeMinutes * time.Minute,
		tokenLifetime:    1 * time.Hour,
		refreshLifetime:  24 * 7 * time.Hour, // 1 week
	}
}

// RegisterClient registers a new OAuth client
func (s *AuthorizationServer) RegisterClient(config *OAuthConfig) (*OAuthClient, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientID := config.ClientID
	if clientID == "" {
		var err error
		clientID, err = s.tokenGenerator.GenerateClientID()
		if err != nil {

			return nil, fmt.Errorf("failed to generate client ID: %w", err)
		}
	}

	// Check if client already exists
	if _, exists := s.clients[clientID]; exists {

		return nil, fmt.Errorf("client with ID %s already exists", clientID)
	}

	// Validate redirect URIs
	for _, uri := range config.RedirectURIs {
		if uri == "" {

			return nil, fmt.Errorf("redirect URI cannot be empty")
		}
		parsed, err := url.Parse(uri)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {

			return nil, fmt.Errorf("invalid redirect URI: %s", uri)
		}
	}

	// Determine if this is a public client (no secret)
	isPublic := config.ClientSecret == "" &&
		(config.TokenEndpointAuth == "none" || config.TokenEndpointAuth == "")

	var clientSecret string
	if !isPublic {
		if config.ClientSecret != "" {
			clientSecret = config.ClientSecret
		} else {
			var err error
			clientSecret, err = s.tokenGenerator.GenerateClientSecret()
			if err != nil {

				return nil, fmt.Errorf("failed to generate client secret: %w", err)
			}
		}
	}

	// Set default values
	grantTypes := config.GrantTypes
	if len(grantTypes) == 0 {
		if isPublic {
			grantTypes = []string{"authorization_code", "refresh_token"}
		} else {
			grantTypes = []string{"authorization_code", "client_credentials", "refresh_token"}
		}
	}

	responseTypes := config.ResponseTypes
	if len(responseTypes) == 0 {
		responseTypes = []string{"code"}
	}

	tokenEndpointAuth := config.TokenEndpointAuth
	if tokenEndpointAuth == "" {
		if isPublic {
			tokenEndpointAuth = "none"
		} else {
			tokenEndpointAuth = "client_secret_post"
		}
	}

	client := &OAuthClient{
		ID:                      clientID,
		Secret:                  clientSecret,
		RedirectURIs:            config.RedirectURIs,
		GrantTypes:              grantTypes,
		ResponseTypes:           responseTypes,
		Scope:                   config.Scope,
		ClientName:              config.ClientName,
		ClientURI:               config.ClientURI,
		LogoURI:                 config.LogoURI,
		TosURI:                  config.TosURI,
		PolicyURI:               config.PolicyURI,
		TokenEndpointAuthMethod: tokenEndpointAuth,
		CreatedAt:               time.Now(),
		SoftwareID:              config.SoftwareID,
		SoftwareVersion:         config.SoftwareVersion,
		CodeChallengeMethod:     config.CodeChallengeMethod,
		Public:                  isPublic,
	}

	// Set expiration for client secret if not public
	if !isPublic {
		client.ExpiresAt = time.Now().Add(365 * 24 * time.Hour) // 1 year
	}

	s.clients[clientID] = client
	s.logger.Info("Registered OAuth client: %s (public: %v)", clientID, isPublic)

	return client, nil
}

// GetClient retrieves a client by ID
func (s *AuthorizationServer) GetClient(clientID string) (*OAuthClient, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	client, exists := s.clients[clientID]

	return client, exists
}

// ValidateAccessToken validates an access token and returns it if valid
func (s *AuthorizationServer) ValidateAccessToken(token string) (*AccessToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	accessToken, exists := s.accessTokens[token]
	if !exists {

		return nil, fmt.Errorf("invalid token")
	}

	// Check expiration
	if accessToken.ExpiresAt.IsZero() || time.Now().After(accessToken.ExpiresAt) {
		// Remove expired token in a goroutine to avoid blocking
		go func() {
			s.mu.Lock()
			delete(s.accessTokens, token)
			s.mu.Unlock()
		}()

		return nil, fmt.Errorf("token expired")
	}

	return accessToken, nil
}

// HasScope checks if a token scope includes the required scope
func (s *AuthorizationServer) HasScope(tokenScope, requiredScope string) bool {
	if tokenScope == "" {

		return false
	}

	scopes := strings.Fields(tokenScope)
	for _, scope := range scopes {
		if scope == requiredScope || scope == "mcp:*" {

			return true
		}
	}

	return false
}

// CleanupExpiredTokens removes expired tokens (can be called periodically)
func (s *AuthorizationServer) CleanupExpiredTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Clean up expired access tokens
	for token, accessToken := range s.accessTokens {
		if now.After(accessToken.ExpiresAt) {
			delete(s.accessTokens, token)
		}
	}

	// Clean up expired refresh tokens
	for token, refreshToken := range s.refreshTokens {
		if now.After(refreshToken.ExpiresAt) {
			delete(s.refreshTokens, token)
		}
	}

	// Clean up expired authorization codes
	for code, authCode := range s.authCodes {
		if now.After(authCode.ExpiresAt) {
			delete(s.authCodes, code)
		}
	}
}

// GetTokenCount returns the number of active tokens (for monitoring)
func (s *AuthorizationServer) GetTokenCount() (int, int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.accessTokens), len(s.refreshTokens), len(s.authCodes)
}

// ValidateClient validates client credentials
func (s *AuthorizationServer) ValidateClient(clientID, clientSecret string) (*OAuthClient, error) {
	client, exists := s.GetClient(clientID)
	if !exists {

		return nil, fmt.Errorf("invalid client")
	}

	// For public clients, no secret validation needed
	if client.Public {

		return client, nil
	}

	// For confidential clients, validate secret
	if client.Secret != clientSecret {

		return nil, fmt.Errorf("invalid client credentials")
	}

	// Check if client secret is expired
	if !client.ExpiresAt.IsZero() && time.Now().After(client.ExpiresAt) {

		return nil, fmt.Errorf("client secret expired")
	}

	return client, nil
}

// generateRandomString generates a random string of specified length
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {

		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// generateRandomStringFromSet generates a random string using specific character set
func generateRandomStringFromSet(length int, charset string) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {

		return "", err
	}

	result := make([]byte, length)
	for i, b := range bytes {
		result[i] = charset[int(b)%len(charset)]
	}

	return string(result), nil
}

// GetMetadata returns OAuth 2.0 Authorization Server Metadata
func (s *AuthorizationServer) GetMetadata() *AuthorizationServerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.config
}

// HandleDiscovery handles requests to /.well-known/oauth-authorization-server
func (s *AuthorizationServer) HandleDiscovery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(s.config); err != nil {
		s.logger.Error("Failed to encode authorization server metadata: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// GetAllClients returns all registered clients
func (s *AuthorizationServer) GetAllClients() []*OAuthClient {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clients := make([]*OAuthClient, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}

	return clients
}

func (s *AuthorizationServer) GetAllAccessTokens() []TokenInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tokens []TokenInfo
	for _, token := range s.accessTokens {
		tokens = append(tokens, TokenInfo{
			ClientID:  token.ClientID,
			UserID:    token.UserID,
			Scope:     token.Scope,
			ExpiresAt: token.ExpiresAt,
			CreatedAt: token.CreatedAt,
			Revoked:   token.Revoked,
		})
	}

	return tokens
}
