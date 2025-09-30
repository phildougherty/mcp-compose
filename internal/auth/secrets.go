package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

const (
	defaultAPIKeyLength = 32
	defaultGracePeriod  = 24 * time.Hour
	minAPIKeyLength     = 16
	maxActiveKeys       = 10
)

type APIKeyType string

const (
	APIKeyTypePrimary   APIKeyType = "primary"
	APIKeyTypeSecondary APIKeyType = "secondary"
	APIKeyTypeRotating  APIKeyType = "rotating"
)

type APIKey struct {
	Key        string
	Type       APIKeyType
	CreatedAt  time.Time
	ExpiresAt  time.Time
	IsActive   bool
	RotationID string
}

type SecretManager struct {
	mu        sync.RWMutex
	keys      map[string]*APIKey
	primary   string
	logger    *logging.Logger
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewSecretManager(logger *logging.Logger) *SecretManager {
	sm := &SecretManager{
		keys:   make(map[string]*APIKey),
		logger: logger,
		stopCh: make(chan struct{}),
	}

	sm.wg.Add(1)
	go sm.cleanupWorker()

	return sm
}

func (sm *SecretManager) AddKey(key string, keyType APIKeyType) error {
	if key == "" {

		return fmt.Errorf("key cannot be empty")
	}

	if len(key) < minAPIKeyLength {

		return fmt.Errorf("key must be at least %d characters", minAPIKeyLength)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if len(sm.keys) >= maxActiveKeys {

		return fmt.Errorf("maximum number of active keys reached")
	}

	if _, exists := sm.keys[key]; exists {

		return fmt.Errorf("key already exists")
	}

	apiKey := &APIKey{
		Key:       key,
		Type:      keyType,
		CreatedAt: time.Now(),
		IsActive:  true,
	}

	if keyType == APIKeyTypePrimary {
		sm.primary = key
	}

	sm.keys[key] = apiKey
	sm.logger.Info("Added API key of type: %s", keyType)

	return nil
}

func (sm *SecretManager) ValidateKey(key string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	apiKey, exists := sm.keys[key]
	if !exists {

		return false
	}

	if !apiKey.IsActive {

		return false
	}

	if !apiKey.ExpiresAt.IsZero() && time.Now().After(apiKey.ExpiresAt) {

		return false
	}

	return true
}

func (sm *SecretManager) GetPrimaryKey() string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	return sm.primary
}

func (sm *SecretManager) RotateKey(gracePeriod time.Duration) (string, error) {
	if gracePeriod <= 0 {
		gracePeriod = defaultGracePeriod
	}

	newKey, err := GenerateAPIKey(defaultAPIKeyLength)
	if err != nil {

		return "", fmt.Errorf("failed to generate new key: %w", err)
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	rotationID := fmt.Sprintf("rotation_%d", time.Now().Unix())

	if sm.primary != "" {
		if oldKey, exists := sm.keys[sm.primary]; exists {
			oldKey.Type = APIKeyTypeSecondary
			oldKey.ExpiresAt = time.Now().Add(gracePeriod)
			oldKey.RotationID = rotationID
			sm.logger.Info("Marked old primary key as secondary with grace period: %v", gracePeriod)
		}
	}

	apiKey := &APIKey{
		Key:        newKey,
		Type:       APIKeyTypePrimary,
		CreatedAt:  time.Now(),
		IsActive:   true,
		RotationID: rotationID,
	}

	sm.keys[newKey] = apiKey
	sm.primary = newKey

	sm.logger.Info("Rotated primary API key, grace period: %v", gracePeriod)

	return newKey, nil
}

func (sm *SecretManager) RevokeKey(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	apiKey, exists := sm.keys[key]
	if !exists {

		return fmt.Errorf("key not found")
	}

	if key == sm.primary {

		return fmt.Errorf("cannot revoke primary key, rotate first")
	}

	apiKey.IsActive = false
	apiKey.ExpiresAt = time.Now()

	sm.logger.Info("Revoked API key of type: %s", apiKey.Type)

	return nil
}

func (sm *SecretManager) ListKeys() []*APIKey {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	keys := make([]*APIKey, 0, len(sm.keys))

	for _, key := range sm.keys {
		keyCopy := *key
		keyCopy.Key = maskKey(key.Key)
		keys = append(keys, &keyCopy)
	}

	return keys
}

func (sm *SecretManager) GetKeyInfo(key string) (*APIKey, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	apiKey, exists := sm.keys[key]
	if !exists {

		return nil, fmt.Errorf("key not found")
	}

	keyCopy := *apiKey
	keyCopy.Key = maskKey(apiKey.Key)

	return &keyCopy, nil
}

func (sm *SecretManager) CleanupExpiredKeys() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, apiKey := range sm.keys {
		if !apiKey.ExpiresAt.IsZero() && now.After(apiKey.ExpiresAt) {
			delete(sm.keys, key)
			removed++
			sm.logger.Debug("Removed expired API key of type: %s", apiKey.Type)
		}
	}

	return removed
}

func (sm *SecretManager) cleanupWorker() {
	defer sm.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-sm.stopCh:

			return
		case <-ticker.C:
			removed := sm.CleanupExpiredKeys()
			if removed > 0 {
				sm.logger.Info("Cleaned up %d expired API keys", removed)
			}
		}
	}
}

func (sm *SecretManager) GetActiveKeyCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	count := 0

	for _, key := range sm.keys {
		if key.IsActive {
			count++
		}
	}

	return count
}

func (sm *SecretManager) HasSecondaryKeys() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, key := range sm.keys {
		if key.Type == APIKeyTypeSecondary && key.IsActive {

			return true
		}
	}

	return false
}

func (sm *SecretManager) Close() error {
	close(sm.stopCh)

	done := make(chan struct{})
	go func() {
		sm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		sm.logger.Debug("Secret manager stopped")
	case <-time.After(5 * time.Second):
		sm.logger.Warning("Secret manager shutdown timeout")
	}

	return nil
}

func GenerateAPIKey(length int) (string, error) {
	if length < minAPIKeyLength {
		length = defaultAPIKeyLength
	}

	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {

		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	key := base64.URLEncoding.EncodeToString(bytes)

	if len(key) > length {
		key = key[:length]
	}

	return key, nil
}

func maskKey(key string) string {
	if len(key) <= 8 {

		return "****"
	}

	return key[:4] + "****" + key[len(key)-4:]
}

func (sm *SecretManager) ExportKeys() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	export := make(map[string]interface{})
	keys := make([]map[string]interface{}, 0, len(sm.keys))

	for _, apiKey := range sm.keys {
		keyInfo := map[string]interface{}{
			"key_masked":  maskKey(apiKey.Key),
			"type":        string(apiKey.Type),
			"created_at":  apiKey.CreatedAt,
			"expires_at":  apiKey.ExpiresAt,
			"is_active":   apiKey.IsActive,
			"rotation_id": apiKey.RotationID,
		}
		keys = append(keys, keyInfo)
	}

	export["keys"] = keys
	export["primary_key"] = maskKey(sm.primary)
	export["total_keys"] = len(sm.keys)
	export["active_keys"] = sm.GetActiveKeyCount()

	return export
}

func (sm *SecretManager) SetPrimaryKey(key string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	apiKey, exists := sm.keys[key]
	if !exists {

		return fmt.Errorf("key not found")
	}

	if !apiKey.IsActive {

		return fmt.Errorf("cannot set inactive key as primary")
	}

	if oldPrimary, exists := sm.keys[sm.primary]; exists && oldPrimary.Key != key {
		oldPrimary.Type = APIKeyTypeSecondary
	}

	apiKey.Type = APIKeyTypePrimary
	sm.primary = key

	sm.logger.Info("Set primary key to: %s", maskKey(key))

	return nil
}