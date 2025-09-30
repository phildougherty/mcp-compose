package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/phildougherty/mcp-compose/internal/logging"
)

func TestNewSecretManager(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	if sm == nil {
		t.Error("Expected non-nil SecretManager")
	}

	if sm.keys == nil {
		t.Error("Expected initialized keys map")
	}
}

func TestSecretManager_AddKey(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	tests := []struct {
		name    string
		key     string
		keyType APIKeyType
		wantErr bool
	}{
		{
			name:    "valid primary key",
			key:     "test-key-primary-1234567890",
			keyType: APIKeyTypePrimary,
			wantErr: false,
		},
		{
			name:    "valid secondary key",
			key:     "test-key-secondary-1234567890",
			keyType: APIKeyTypeSecondary,
			wantErr: false,
		},
		{
			name:    "empty key",
			key:     "",
			keyType: APIKeyTypePrimary,
			wantErr: true,
		},
		{
			name:    "short key",
			key:     "short",
			keyType: APIKeyTypePrimary,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sm.AddKey(tt.key, tt.keyType)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddKey() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if !sm.ValidateKey(tt.key) {
					t.Error("Expected key to be valid after adding")
				}

				if tt.keyType == APIKeyTypePrimary && sm.GetPrimaryKey() != tt.key {
					t.Error("Expected primary key to be set")
				}
			}
		})
	}
}

func TestSecretManager_ValidateKey(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	validKey := "valid-test-key-1234567890"
	sm.AddKey(validKey, APIKeyTypePrimary)

	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "valid key",
			key:  validKey,
			want: true,
		},
		{
			name: "invalid key",
			key:  "invalid-key",
			want: false,
		},
		{
			name: "empty key",
			key:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sm.ValidateKey(tt.key)
			if got != tt.want {
				t.Errorf("ValidateKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecretManager_RotateKey(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	oldKey := "old-primary-key-1234567890"
	sm.AddKey(oldKey, APIKeyTypePrimary)

	gracePeriod := 1 * time.Hour
	newKey, err := sm.RotateKey(gracePeriod)
	if err != nil {
		t.Fatalf("RotateKey() error = %v", err)
	}

	if newKey == "" {
		t.Error("Expected non-empty new key")
	}

	if newKey == oldKey {
		t.Error("Expected new key to be different from old key")
	}

	if !sm.ValidateKey(newKey) {
		t.Error("Expected new key to be valid")
	}

	if !sm.ValidateKey(oldKey) {
		t.Error("Expected old key to still be valid during grace period")
	}

	if sm.GetPrimaryKey() != newKey {
		t.Error("Expected new key to be primary")
	}

	oldKeyInfo, err := sm.GetKeyInfo(oldKey)
	if err != nil {
		t.Fatalf("GetKeyInfo() error = %v", err)
	}

	if oldKeyInfo.Type != APIKeyTypeSecondary {
		t.Errorf("Expected old key type to be secondary, got %s", oldKeyInfo.Type)
	}

	if oldKeyInfo.ExpiresAt.IsZero() {
		t.Error("Expected old key to have expiration time")
	}
}

func TestSecretManager_RevokeKey(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	primaryKey := "primary-key-1234567890"
	secondaryKey := "secondary-key-1234567890"

	sm.AddKey(primaryKey, APIKeyTypePrimary)
	sm.AddKey(secondaryKey, APIKeyTypeSecondary)

	err := sm.RevokeKey(primaryKey)
	if err == nil {
		t.Error("Expected error when revoking primary key")
	}

	err = sm.RevokeKey(secondaryKey)
	if err != nil {
		t.Errorf("RevokeKey() error = %v", err)
	}

	if sm.ValidateKey(secondaryKey) {
		t.Error("Expected secondary key to be invalid after revocation")
	}

	err = sm.RevokeKey("non-existent-key")
	if err == nil {
		t.Error("Expected error when revoking non-existent key")
	}
}

func TestSecretManager_ListKeys(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	key1 := "test-key-1-1234567890"
	key2 := "test-key-2-1234567890"

	sm.AddKey(key1, APIKeyTypePrimary)
	sm.AddKey(key2, APIKeyTypeSecondary)

	keys := sm.ListKeys()

	if len(keys) != 2 {
		t.Errorf("ListKeys() returned %d keys, want 2", len(keys))
	}

	for _, key := range keys {
		if key.Key == key1 || key.Key == key2 {
			t.Error("Expected keys to be masked in list")
		}

		if !strings.Contains(key.Key, "****") {
			t.Error("Expected key to contain mask")
		}
	}
}

func TestSecretManager_GetKeyInfo(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	key := "test-key-info-1234567890"
	sm.AddKey(key, APIKeyTypePrimary)

	info, err := sm.GetKeyInfo(key)
	if err != nil {
		t.Fatalf("GetKeyInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("Expected non-nil key info")
	}

	if info.Type != APIKeyTypePrimary {
		t.Errorf("Expected type %s, got %s", APIKeyTypePrimary, info.Type)
	}

	if info.IsActive != true {
		t.Error("Expected key to be active")
	}

	if info.Key == key {
		t.Error("Expected key to be masked in info")
	}

	_, err = sm.GetKeyInfo("non-existent-key")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}
}

func TestSecretManager_CleanupExpiredKeys(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	key1 := "expired-key-1234567890"
	key2 := "active-key-1234567890"

	sm.AddKey(key1, APIKeyTypeSecondary)
	sm.AddKey(key2, APIKeyTypePrimary)

	sm.mu.Lock()
	sm.keys[key1].ExpiresAt = time.Now().Add(-1 * time.Hour)
	sm.mu.Unlock()

	removed := sm.CleanupExpiredKeys()

	if removed != 1 {
		t.Errorf("CleanupExpiredKeys() removed %d keys, want 1", removed)
	}

	if sm.ValidateKey(key1) {
		t.Error("Expected expired key to be removed")
	}

	if !sm.ValidateKey(key2) {
		t.Error("Expected active key to remain")
	}
}

func TestSecretManager_GetActiveKeyCount(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	if sm.GetActiveKeyCount() != 0 {
		t.Error("Expected 0 active keys initially")
	}

	sm.AddKey("key1-1234567890", APIKeyTypePrimary)
	sm.AddKey("key2-1234567890", APIKeyTypeSecondary)

	if sm.GetActiveKeyCount() != 2 {
		t.Errorf("GetActiveKeyCount() = %d, want 2", sm.GetActiveKeyCount())
	}

	sm.RevokeKey("key2-1234567890")

	if sm.GetActiveKeyCount() != 1 {
		t.Errorf("GetActiveKeyCount() = %d, want 1", sm.GetActiveKeyCount())
	}
}

func TestSecretManager_HasSecondaryKeys(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	if sm.HasSecondaryKeys() {
		t.Error("Expected no secondary keys initially")
	}

	sm.AddKey("primary-1234567890", APIKeyTypePrimary)

	if sm.HasSecondaryKeys() {
		t.Error("Expected no secondary keys after adding primary")
	}

	sm.AddKey("secondary-1234567890", APIKeyTypeSecondary)

	if !sm.HasSecondaryKeys() {
		t.Error("Expected secondary keys to exist")
	}
}

func TestSecretManager_SetPrimaryKey(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	key1 := "key1-1234567890"
	key2 := "key2-1234567890"

	sm.AddKey(key1, APIKeyTypePrimary)
	sm.AddKey(key2, APIKeyTypeSecondary)

	err := sm.SetPrimaryKey(key2)
	if err != nil {
		t.Errorf("SetPrimaryKey() error = %v", err)
	}

	if sm.GetPrimaryKey() != key2 {
		t.Error("Expected primary key to be updated")
	}

	key1Info, _ := sm.GetKeyInfo(key1)
	if key1Info.Type != APIKeyTypeSecondary {
		t.Error("Expected old primary to become secondary")
	}

	err = sm.SetPrimaryKey("non-existent")
	if err == nil {
		t.Error("Expected error when setting non-existent key as primary")
	}
}

func TestSecretManager_ExportKeys(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	sm.AddKey("export-key-1234567890", APIKeyTypePrimary)

	export := sm.ExportKeys()

	if export == nil {
		t.Fatal("Expected non-nil export")
	}

	if _, ok := export["keys"]; !ok {
		t.Error("Expected keys in export")
	}

	if _, ok := export["primary_key"]; !ok {
		t.Error("Expected primary_key in export")
	}

	if _, ok := export["total_keys"]; !ok {
		t.Error("Expected total_keys in export")
	}

	if _, ok := export["active_keys"]; !ok {
		t.Error("Expected active_keys in export")
	}
}

func TestGenerateAPIKey(t *testing.T) {
	tests := []struct {
		name   string
		length int
		want   int
	}{
		{
			name:   "default length",
			length: 32,
			want:   32,
		},
		{
			name:   "custom length",
			length: 64,
			want:   64,
		},
		{
			name:   "minimum length",
			length: 16,
			want:   16,
		},
		{
			name:   "below minimum (should use default)",
			length: 8,
			want:   32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := GenerateAPIKey(tt.length)
			if err != nil {
				t.Errorf("GenerateAPIKey() error = %v", err)

				return
			}

			if len(key) != tt.want {
				t.Errorf("GenerateAPIKey() length = %d, want %d", len(key), tt.want)
			}
		})
	}

	key1, _ := GenerateAPIKey(32)
	key2, _ := GenerateAPIKey(32)

	if key1 == key2 {
		t.Error("Expected different keys from multiple generations")
	}
}

func TestMaskKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "normal key",
			key:  "1234567890abcdef",
			want: "1234****cdef",
		},
		{
			name: "short key",
			key:  "short",
			want: "****",
		},
		{
			name: "empty key",
			key:  "",
			want: "****",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskKey(tt.key)
			if got != tt.want {
				t.Errorf("maskKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSecretManager_MaxActiveKeys(t *testing.T) {
	logger := logging.NewLogger("debug")
	sm := NewSecretManager(logger)
	defer sm.Close()

	for i := 0; i < maxActiveKeys; i++ {
		key := GenerateRandomKey(i)
		err := sm.AddKey(key, APIKeyTypeSecondary)
		if err != nil {
			t.Fatalf("Failed to add key %d: %v", i, err)
		}
	}

	extraKey := GenerateRandomKey(maxActiveKeys)
	err := sm.AddKey(extraKey, APIKeyTypeSecondary)
	if err == nil {
		t.Error("Expected error when exceeding max active keys")
	}
}

func GenerateRandomKey(seed int) string {

	return string([]byte("test-key-")[:9]) + string(rune('0'+seed)) + "-1234567890"
}