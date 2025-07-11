// internal/protocol/roots.go
package protocol

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RootManager manages MCP roots according to specification
type RootManager struct {
	roots    map[string]*RootEntry
	watchers map[string]*RootWatcher
	mu       sync.RWMutex
}

// RootEntry represents a managed root
type RootEntry struct {
	Root        Root             `json:"root"`
	Added       time.Time        `json:"added"`
	LastUsed    time.Time        `json:"lastUsed"`
	Permissions *RootPermissions `json:"permissions,omitempty"`
}

// RootPermissions defines what operations are allowed on a root
type RootPermissions struct {
	Read  bool `json:"read"`
	Write bool `json:"write"`
	List  bool `json:"list"`
	Watch bool `json:"watch"`
}

// RootWatcher monitors changes to a root
type RootWatcher struct {
	RootURI    string
	Callback   func(uri string, event string)
	LastChange time.Time
	Active     bool
}

// RootsListRequest represents a roots/list request
type RootsListRequest struct {
	// No parameters for basic list
}

// RootsListResponse represents a roots/list response
type RootsListResponse struct {
	Roots []Root `json:"roots"`
}

// NewRootManager creates a new root manager
func NewRootManager() *RootManager {

	return &RootManager{
		roots:    make(map[string]*RootEntry),
		watchers: make(map[string]*RootWatcher),
	}
}

// AddRoot adds a root to the manager
func (rm *RootManager) AddRoot(root Root, permissions *RootPermissions) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Validate root URI
	if err := rm.validateRootURI(root.URI); err != nil {

		return NewValidationError("uri", root.URI, err.Error())
	}

	// Set default permissions if not provided
	if permissions == nil {
		permissions = &RootPermissions{
			Read:  true,
			Write: false,
			List:  true,
			Watch: false,
		}
	}

	entry := &RootEntry{
		Root:        root,
		Added:       time.Now(),
		LastUsed:    time.Now(),
		Permissions: permissions,
	}

	rm.roots[root.URI] = entry

	return nil
}

// RemoveRoot removes a root from the manager
func (rm *RootManager) RemoveRoot(uri string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, exists := rm.roots[uri]; !exists {

		return NewResourceError(uri, "remove", "root not found")
	}

	// Stop watching if active
	if watcher, exists := rm.watchers[uri]; exists {
		watcher.Active = false
		delete(rm.watchers, uri)
	}

	delete(rm.roots, uri)

	return nil
}

// GetRoot retrieves a root by URI
func (rm *RootManager) GetRoot(uri string) (*RootEntry, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	entry, exists := rm.roots[uri]
	if !exists {

		return nil, NewResourceError(uri, "get", "root not found")
	}

	// Update last used time
	entry.LastUsed = time.Now()

	return entry, nil
}

// ListRoots returns all roots
func (rm *RootManager) ListRoots() []Root {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	roots := make([]Root, 0, len(rm.roots))
	for _, entry := range rm.roots {
		roots = append(roots, entry.Root)
	}

	return roots
}

// CheckRootAccess checks if a path is within any managed roots
func (rm *RootManager) CheckRootAccess(path string, operation string) (*RootEntry, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Normalize the path for comparison
	normalizedPath := rm.normalizePath(path)

	// Check against all roots
	for _, entry := range rm.roots {
		if rm.isPathInRoot(normalizedPath, entry.Root.URI) {
			// Check permissions
			if !rm.hasPermission(entry.Permissions, operation) {

				return nil, NewAuthorizationError(path, operation)
			}

			entry.LastUsed = time.Now()

			return entry, nil
		}
	}

	return nil, NewAuthorizationError(path, "not within any managed root")
}

// WatchRoot starts watching a root for changes
func (rm *RootManager) WatchRoot(uri string, callback func(uri string, event string)) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Check if root exists
	entry, exists := rm.roots[uri]
	if !exists {

		return NewResourceError(uri, "watch", "root not found")
	}

	// Check watch permission
	if !entry.Permissions.Watch {

		return NewAuthorizationError(uri, "watch")
	}

	// Create watcher
	watcher := &RootWatcher{
		RootURI:    uri,
		Callback:   callback,
		LastChange: time.Now(),
		Active:     true,
	}

	rm.watchers[uri] = watcher

	return nil
}

// UnwatchRoot stops watching a root
func (rm *RootManager) UnwatchRoot(uri string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	watcher, exists := rm.watchers[uri]
	if !exists {

		return NewResourceError(uri, "unwatch", "watcher not found")
	}

	watcher.Active = false
	delete(rm.watchers, uri)

	return nil
}

// validateRootURI validates a root URI according to MCP specification
func (rm *RootManager) validateRootURI(uri string) error {
	if uri == "" {

		return fmt.Errorf("URI cannot be empty")
	}

	// Parse as URL to validate format
	parsedURL, err := url.Parse(uri)
	if err != nil {

		return fmt.Errorf("invalid URI format: %v", err)
	}

	// Check for supported schemes
	switch parsedURL.Scheme {
	case "file":
		// Validate file path
		if parsedURL.Path == "" {

			return fmt.Errorf("file URI must have a path")
		}
	case "http", "https":
		// Validate HTTP URL
		if parsedURL.Host == "" {

			return fmt.Errorf("HTTP URI must have a host")
		}
	case "":
		// Relative path - treat as file
		if !filepath.IsAbs(uri) {

			return fmt.Errorf("relative paths not supported as roots")
		}
	default:
		// Custom schemes are allowed but should be documented
		// Consider logging a warning for unknown schemes
	}

	return nil
}

// normalizePath normalizes a path for consistent comparison
func (rm *RootManager) normalizePath(path string) string {
	// Handle file:// URIs
	if strings.HasPrefix(path, "file://") {
		if parsedURL, err := url.Parse(path); err == nil {
			path = parsedURL.Path
		}
	}

	// Clean the path

	return filepath.Clean(path)
}

// isPathInRoot checks if a path is within a root
func (rm *RootManager) isPathInRoot(path string, rootURI string) bool {
	normalizedRoot := rm.normalizePath(rootURI)

	// For file paths, check if path is under root directory
	if !strings.HasPrefix(path, normalizedRoot) {

		return false
	}

	// Ensure it's actually a subdirectory, not just a prefix match
	if len(path) > len(normalizedRoot) {
		// The next character should be a path separator

		return path[len(normalizedRoot)] == filepath.Separator
	}

	// Exact match is allowed

	return true
}

// hasPermission checks if a permission is granted
func (rm *RootManager) hasPermission(permissions *RootPermissions, operation string) bool {
	if permissions == nil {

		return false
	}

	switch operation {
	case "read":

		return permissions.Read
	case "write", "create", "update", "delete":

		return permissions.Write
	case "list":

		return permissions.List
	case "watch":

		return permissions.Watch
	default:

		return false
	}
}

// GetRootStats returns statistics about root usage
func (rm *RootManager) GetRootStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := map[string]interface{}{
		"totalRoots":     len(rm.roots),
		"activeWatchers": len(rm.watchers),
		"roots":          make([]map[string]interface{}, 0, len(rm.roots)),
	}

	for uri, entry := range rm.roots {
		rootStat := map[string]interface{}{
			"uri":         uri,
			"name":        entry.Root.Name,
			"added":       entry.Added,
			"lastUsed":    entry.LastUsed,
			"permissions": entry.Permissions,
		}

		if watcher, watched := rm.watchers[uri]; watched {
			rootStat["watching"] = watcher.Active
			rootStat["lastChange"] = watcher.LastChange
		}

		stats["roots"] = append(stats["roots"].([]map[string]interface{}), rootStat)
	}

	return stats
}

// NotifyRootChange notifies watchers of a root change
func (rm *RootManager) NotifyRootChange(uri string, event string) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	// Find all watchers that should be notified
	for watchedURI, watcher := range rm.watchers {
		if watcher.Active && rm.isPathInRoot(uri, watchedURI) {
			watcher.LastChange = time.Now()
			go watcher.Callback(uri, event) // Call async to avoid blocking
		}
	}
}

// CreateDefaultRoots creates default roots for common use cases
func (rm *RootManager) CreateDefaultRoots() error {
	// Add current working directory as default read-only root
	workingDir, err := filepath.Abs(".")
	if err != nil {

		return fmt.Errorf("failed to get working directory: %v", err)
	}

	workingRoot := Root{
		URI:  "file://" + workingDir,
		Name: "Working Directory",
	}

	workingPermissions := &RootPermissions{
		Read:  true,
		Write: false,
		List:  true,
		Watch: true,
	}

	if err := rm.AddRoot(workingRoot, workingPermissions); err != nil {

		return fmt.Errorf("failed to add working directory root: %v", err)
	}

	return nil
}
