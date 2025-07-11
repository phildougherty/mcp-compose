// internal/auth/resource_metadata.go
package auth

import (
	"encoding/json"
	"net/http"
)

// ProtectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC 9728)
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource,omitempty"`
	AuthorizationServers   []string `json:"authorization_servers"`
	JWKSUri                string   `json:"jwks_uri,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ResourceDocumentation  string   `json:"resource_documentation,omitempty"`
	ResourcePolicyURI      string   `json:"resource_policy_uri,omitempty"`
	ResourceTosURI         string   `json:"resource_tos_uri,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
}

// ResourceMetadataHandler handles protected resource metadata requests
type ResourceMetadataHandler struct {
	metadata *ProtectedResourceMetadata
}

// NewResourceMetadataHandler creates a new resource metadata handler
func NewResourceMetadataHandler(authServers []string, scopes []string) *ResourceMetadataHandler {
	metadata := &ProtectedResourceMetadata{
		Resource:               "mcp-compose",
		AuthorizationServers:   authServers,
		BearerMethodsSupported: []string{"header", "body"},
		ScopesSupported:        scopes,
	}

	return &ResourceMetadataHandler{
		metadata: metadata,
	}
}

// HandleProtectedResourceMetadata handles requests to /.well-known/oauth-protected-resource
func (h *ResourceMetadataHandler) HandleProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(h.metadata); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// SetResource sets the resource identifier
func (h *ResourceMetadataHandler) SetResource(resource string) {
	h.metadata.Resource = resource
}

// SetJWKSUri sets the JWKS URI
func (h *ResourceMetadataHandler) SetJWKSUri(uri string) {
	h.metadata.JWKSUri = uri
}

// SetDocumentation sets the resource documentation URI
func (h *ResourceMetadataHandler) SetDocumentation(uri string) {
	h.metadata.ResourceDocumentation = uri
}

// SetPolicyURI sets the resource policy URI
func (h *ResourceMetadataHandler) SetPolicyURI(uri string) {
	h.metadata.ResourcePolicyURI = uri
}

// SetTosURI sets the resource terms of service URI
func (h *ResourceMetadataHandler) SetTosURI(uri string) {
	h.metadata.ResourceTosURI = uri
}

// AddAuthorizationServer adds an authorization server
func (h *ResourceMetadataHandler) AddAuthorizationServer(server string) {
	for _, existing := range h.metadata.AuthorizationServers {
		if existing == server {

			return // Already exists
		}
	}
	h.metadata.AuthorizationServers = append(h.metadata.AuthorizationServers, server)
}

// RemoveAuthorizationServer removes an authorization server
func (h *ResourceMetadataHandler) RemoveAuthorizationServer(server string) {
	for i, existing := range h.metadata.AuthorizationServers {
		if existing == server {
			h.metadata.AuthorizationServers = append(
				h.metadata.AuthorizationServers[:i],
				h.metadata.AuthorizationServers[i+1:]...,
			)

			break
		}
	}
}

// GetMetadata returns the current metadata
func (h *ResourceMetadataHandler) GetMetadata() *ProtectedResourceMetadata {

	return h.metadata
}
