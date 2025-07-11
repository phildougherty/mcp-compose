// internal/protocol/resources.go
package protocol

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ResourceManager provides complete MCP resource capabilities
type ResourceManager struct {
	resources    map[string]*Resource
	cache        map[string]*CachedResource
	transformers map[string]ResourceTransformer
	mu           sync.RWMutex
}

// Resource represents a complete MCP resource with full metadata and capabilities
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	MimeType    string                 `json:"mimeType,omitempty"`
	Size        int64                  `json:"size,omitempty"`
	Content     *ResourceContentData   `json:"content,omitempty"`
	Metadata    *ResourceMetadata      `json:"metadata,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	Template    *URITemplate           `json:"template,omitempty"`
	Embedding   *ResourceEmbedding     `json:"embedding,omitempty"`
	Cache       *CacheConfig           `json:"cache,omitempty"`
	Created     time.Time              `json:"created"`
	Modified    time.Time              `json:"modified"`
	Accessed    time.Time              `json:"accessed"`
}

// ResourceContentData represents the actual content of a resource
type ResourceContentData struct {
	Type         string    `json:"type"`                  // "text", "blob"
	Data         string    `json:"data"`                  // Base64 for blob, direct for text
	Encoding     string    `json:"encoding"`              // "utf-8", "base64", etc.
	Hash         string    `json:"hash,omitempty"`        // Content hash
	Compression  string    `json:"compression,omitempty"` // "gzip", "deflate", etc.
	LastModified time.Time `json:"lastModified"`
}

// ResourceMetadata contains extended metadata about a resource
type ResourceMetadata struct {
	Author      string                 `json:"author,omitempty"`
	Version     string                 `json:"version,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Language    string                 `json:"language,omitempty"`
	Encoding    string                 `json:"encoding,omitempty"`
	ContentType string                 `json:"contentType,omitempty"`
	Permissions *ResourcePermissions   `json:"permissions,omitempty"`
	Indexing    *IndexingConfig        `json:"indexing,omitempty"`
	Custom      map[string]interface{} `json:"custom,omitempty"`
}

// ResourcePermissions defines access permissions for a resource
type ResourcePermissions struct {
	Read    bool     `json:"read"`
	Write   bool     `json:"write"`
	Execute bool     `json:"execute"`
	Delete  bool     `json:"delete"`
	Share   bool     `json:"share"`
	Roles   []string `json:"roles,omitempty"`
	Users   []string `json:"users,omitempty"`
}

// IndexingConfig controls how a resource should be indexed
type IndexingConfig struct {
	Enabled    bool     `json:"enabled"`
	FullText   bool     `json:"fullText"`
	Keywords   []string `json:"keywords,omitempty"`
	Summary    string   `json:"summary,omitempty"`
	Searchable bool     `json:"searchable"`
}

// ResourceEmbedding contains information for embedding resources in prompts
type ResourceEmbedding struct {
	Strategy  string                 `json:"strategy"` // "inline", "reference", "summary"
	MaxSize   int64                  `json:"maxSize,omitempty"`
	Transform string                 `json:"transform,omitempty"` // "text", "markdown", "json"
	Context   map[string]interface{} `json:"context,omitempty"`
	Fallback  string                 `json:"fallback,omitempty"` // Fallback strategy if embedding fails
}

// CacheConfig controls caching behavior for a resource
type CacheConfig struct {
	Enabled     bool          `json:"enabled"`
	TTL         time.Duration `json:"ttl,omitempty"`
	Refreshable bool          `json:"refreshable"`
	Tags        []string      `json:"tags,omitempty"`
	Key         string        `json:"key,omitempty"`
}

// CachedResource represents a cached resource
type CachedResource struct {
	Resource    *Resource `json:"resource"`
	CachedAt    time.Time `json:"cachedAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	AccessCount int64     `json:"accessCount"`
	LastAccess  time.Time `json:"lastAccess"`
	Tags        []string  `json:"tags"`
}

// ResourceTransformer defines the interface for transforming resources
type ResourceTransformer interface {
	// Transform transforms resource content
	Transform(resource *Resource, targetFormat string, options map[string]interface{}) (*ResourceContentData, error)
	// GetSupportedFormats returns supported transformation formats
	GetSupportedFormats() []string
	// GetTransformationOptions returns available options for transformation
	GetTransformationOptions(fromFormat, toFormat string) map[string]interface{}
}

// EmbeddedPromptResource represents a resource embedded in a prompt
type EmbeddedPromptResource struct {
	Type      string                 `json:"type"` // "resource"
	Resource  *Resource              `json:"resource"`
	Inline    bool                   `json:"inline"`
	Transform string                 `json:"transform,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// PromptMessage represents a prompt message with embedded resources
type PromptMessage struct {
	Role      string                    `json:"role"`
	Content   []PromptContentItem       `json:"content"`
	Resources []*EmbeddedPromptResource `json:"resources,omitempty"`
	Metadata  map[string]interface{}    `json:"metadata,omitempty"`
}

// PromptContentItem represents an item of content in a prompt message
type PromptContentItem struct {
	Type     string                  `json:"type"` // "text", "image", "resource"
	Text     string                  `json:"text,omitempty"`
	ImageURL string                  `json:"imageUrl,omitempty"`
	Resource *EmbeddedPromptResource `json:"resource,omitempty"`
}

// NewResourceManager creates a new resource manager
func NewResourceManager() *ResourceManager {

	return &ResourceManager{
		resources:    make(map[string]*Resource),
		cache:        make(map[string]*CachedResource),
		transformers: make(map[string]ResourceTransformer),
	}
}

// AddResource adds a resource to the manager
func (rm *ResourceManager) AddResource(resource *Resource) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if resource.URI == "" {

		return fmt.Errorf("resource URI cannot be empty")
	}

	// Generate content hash if not provided
	if resource.Content != nil && resource.Content.Hash == "" {
		resource.Content.Hash = rm.generateContentHash(resource.Content.Data)
	}

	// Set timestamps
	now := time.Now()
	if resource.Created.IsZero() {
		resource.Created = now
	}
	resource.Modified = now

	// Auto-detect MIME type if not provided
	if resource.MimeType == "" && resource.URI != "" {
		ext := filepath.Ext(resource.URI)
		if ext != "" {
			resource.MimeType = mime.TypeByExtension(ext)
		}
	}

	rm.resources[resource.URI] = resource

	// Add to cache if caching is enabled
	if resource.Cache != nil && resource.Cache.Enabled {
		rm.addToCache(resource)
	}

	return nil
}

// GetResource retrieves a resource by URI
func (rm *ResourceManager) GetResource(uri string) (*Resource, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Check cache first
	if cached := rm.getFromCache(uri); cached != nil {
		cached.AccessCount++
		cached.LastAccess = time.Now()

		return cached.Resource, nil
	}

	// Get from main storage
	resource, exists := rm.resources[uri]
	if !exists {

		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Update access time
	resource.Accessed = time.Now()

	return resource, nil
}

// EmbedResourceInPrompt embeds a resource in a prompt message
func (rm *ResourceManager) EmbedResourceInPrompt(uri string, strategy string, options map[string]interface{}) (*EmbeddedPromptResource, error) {
	resource, err := rm.GetResource(uri)
	if err != nil {

		return nil, fmt.Errorf("failed to get resource for embedding: %w", err)
	}

	embedded := &EmbeddedPromptResource{
		Type:     "resource",
		Resource: resource,
		Context:  options,
	}

	switch strategy {
	case "inline":
		embedded.Inline = true
		if resource.Embedding != nil && resource.Embedding.MaxSize > 0 {
			if resource.Size > resource.Embedding.MaxSize {
				// Use summary or reference strategy instead
				if resource.Embedding.Fallback != "" {

					return rm.EmbedResourceInPrompt(uri, resource.Embedding.Fallback, options)
				}
				embedded.Inline = false
			}
		}
	case "reference":
		embedded.Inline = false
	case "summary":
		embedded.Inline = false
		embedded.Transform = "summary"
	default:

		return nil, fmt.Errorf("unsupported embedding strategy: %s", strategy)
	}

	return embedded, nil
}

// TransformResource transforms a resource to a different format
func (rm *ResourceManager) TransformResource(uri string, targetFormat string, options map[string]interface{}) (*ResourceContentData, error) {
	rm.mu.RLock()
	resource, exists := rm.resources[uri]
	rm.mu.RUnlock()

	if !exists {

		return nil, fmt.Errorf("resource not found: %s", uri)
	}

	// Find appropriate transformer
	for _, transformer := range rm.transformers {
		formats := transformer.GetSupportedFormats()
		for _, format := range formats {
			if format == targetFormat {

				return transformer.Transform(resource, targetFormat, options)
			}
		}
	}

	return nil, fmt.Errorf("no transformer found for format: %s", targetFormat)
}

// RegisterTransformer registers a resource transformer
func (rm *ResourceManager) RegisterTransformer(name string, transformer ResourceTransformer) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.transformers[name] = transformer
}

// addToCache adds a resource to cache
func (rm *ResourceManager) addToCache(resource *Resource) {
	if resource.Cache == nil || !resource.Cache.Enabled {

		return
	}

	cacheKey := resource.URI
	if resource.Cache.Key != "" {
		cacheKey = resource.Cache.Key
	}

	cached := &CachedResource{
		Resource:   resource,
		CachedAt:   time.Now(),
		Tags:       resource.Cache.Tags,
		LastAccess: time.Now(),
	}

	if resource.Cache.TTL > 0 {
		cached.ExpiresAt = cached.CachedAt.Add(resource.Cache.TTL)
	}

	rm.cache[cacheKey] = cached
}

// getFromCache retrieves a resource from cache
func (rm *ResourceManager) getFromCache(uri string) *CachedResource {
	cached, exists := rm.cache[uri]
	if !exists {

		return nil
	}

	// Check expiration
	if !cached.ExpiresAt.IsZero() && time.Now().After(cached.ExpiresAt) {
		delete(rm.cache, uri)

		return nil
	}

	return cached
}

// generateContentHash generates a hash for content
func (rm *ResourceManager) generateContentHash(content string) string {
	hash := md5.Sum([]byte(content))

	return fmt.Sprintf("%x", hash)
}

// CleanupCache removes expired cache entries
func (rm *ResourceManager) CleanupCache() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	for uri, cached := range rm.cache {
		if !cached.ExpiresAt.IsZero() && now.After(cached.ExpiresAt) {
			delete(rm.cache, uri)
		}
	}
}

// GetCacheStats returns cache statistics
func (rm *ResourceManager) GetCacheStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var totalAccess int64
	expiredCount := 0
	now := time.Now()

	for _, cached := range rm.cache {
		totalAccess += cached.AccessCount
		if !cached.ExpiresAt.IsZero() && now.After(cached.ExpiresAt) {
			expiredCount++
		}
	}

	return map[string]interface{}{
		"totalEntries":  len(rm.cache),
		"totalAccess":   totalAccess,
		"expiredCount":  expiredCount,
		"resourceCount": len(rm.resources),
	}
}

// Search searches for resources based on criteria
func (rm *ResourceManager) Search(query string, filters map[string]interface{}) []*Resource {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var results []*Resource
	queryLower := strings.ToLower(query)

	for _, resource := range rm.resources {
		if rm.matchesSearch(resource, queryLower, filters) {
			results = append(results, resource)
		}
	}

	return results
}

// matchesSearch checks if a resource matches search criteria
func (rm *ResourceManager) matchesSearch(resource *Resource, query string, filters map[string]interface{}) bool {
	// Text search
	if query != "" {
		searchText := strings.ToLower(resource.Name + " " + resource.Description)
		if resource.Metadata != nil {
			for _, tag := range resource.Metadata.Tags {
				searchText += " " + strings.ToLower(tag)
			}
		}
		if !strings.Contains(searchText, query) {

			return false
		}
	}

	// Apply filters
	for key, value := range filters {
		switch key {
		case "mimeType":
			if resource.MimeType != value {

				return false
			}
		case "tag":
			if resource.Metadata == nil {

				return false
			}
			found := false
			for _, tag := range resource.Metadata.Tags {
				if tag == value {
					found = true

					break
				}
			}
			if !found {

				return false
			}
		case "minSize":
			if size, ok := value.(int64); ok && resource.Size < size {

				return false
			}
		case "maxSize":
			if size, ok := value.(int64); ok && resource.Size > size {

				return false
			}
		}
	}

	return true
}

// DefaultTextTransformer provides basic text transformations
type DefaultTextTransformer struct{}

func (dt *DefaultTextTransformer) Transform(resource *Resource, targetFormat string, options map[string]interface{}) (*ResourceContentData, error) {
	if resource.Content == nil {

		return nil, fmt.Errorf("resource has no content")
	}

	switch targetFormat {
	case "summary":

		return dt.createSummary(resource, options)
	case "markdown":

		return dt.convertToMarkdown(resource, options)
	case "json":

		return dt.convertToJSON(resource, options)
	default:

		return nil, fmt.Errorf("unsupported format: %s", targetFormat)
	}
}

func (dt *DefaultTextTransformer) GetSupportedFormats() []string {

	return []string{"summary", "markdown", "json"}
}

func (dt *DefaultTextTransformer) GetTransformationOptions(fromFormat, toFormat string) map[string]interface{} {
	options := make(map[string]interface{})

	switch toFormat {
	case "summary":
		options["maxLength"] = 200
		options["includeMetadata"] = true
	case "markdown":
		options["includeHeaders"] = true
		options["includeMetadata"] = false
	case "json":
		options["includeContent"] = true
		options["includeMetadata"] = true
	}

	return options
}

func (dt *DefaultTextTransformer) createSummary(resource *Resource, options map[string]interface{}) (*ResourceContentData, error) {
	maxLength := 200
	if ml, ok := options["maxLength"].(int); ok {
		maxLength = ml
	}

	summary := resource.Description
	if summary == "" {
		summary = resource.Name
	}

	if len(summary) > maxLength {
		summary = summary[:maxLength] + "..."
	}

	return &ResourceContentData{
		Type:         "text",
		Data:         summary,
		Encoding:     "utf-8",
		Hash:         fmt.Sprintf("%x", md5.Sum([]byte(summary))),
		LastModified: time.Now(),
	}, nil
}

func (dt *DefaultTextTransformer) convertToMarkdown(resource *Resource, options map[string]interface{}) (*ResourceContentData, error) {
	var md strings.Builder

	includeHeaders := true
	if ih, ok := options["includeHeaders"].(bool); ok {
		includeHeaders = ih
	}

	if includeHeaders {
		md.WriteString(fmt.Sprintf("# %s\n\n", resource.Name))
		if resource.Description != "" {
			md.WriteString(fmt.Sprintf("%s\n\n", resource.Description))
		}
	}

	if resource.Content != nil {
		md.WriteString(resource.Content.Data)
	}

	result := md.String()

	return &ResourceContentData{
		Type:         "text",
		Data:         result,
		Encoding:     "utf-8",
		Hash:         fmt.Sprintf("%x", md5.Sum([]byte(result))),
		LastModified: time.Now(),
	}, nil
}

func (dt *DefaultTextTransformer) convertToJSON(resource *Resource, options map[string]interface{}) (*ResourceContentData, error) {
	data := map[string]interface{}{
		"uri":         resource.URI,
		"name":        resource.Name,
		"description": resource.Description,
		"mimeType":    resource.MimeType,
		"size":        resource.Size,
		"created":     resource.Created,
		"modified":    resource.Modified,
	}

	includeContent := true
	if ic, ok := options["includeContent"].(bool); ok {
		includeContent = ic
	}

	includeMetadata := true
	if im, ok := options["includeMetadata"].(bool); ok {
		includeMetadata = im
	}

	if includeContent && resource.Content != nil {
		data["content"] = resource.Content
	}

	if includeMetadata && resource.Metadata != nil {
		data["metadata"] = resource.Metadata
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {

		return nil, fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return &ResourceContentData{
		Type:         "text",
		Data:         string(jsonData),
		Encoding:     "utf-8",
		Hash:         fmt.Sprintf("%x", md5.Sum(jsonData)),
		LastModified: time.Now(),
	}, nil
}
