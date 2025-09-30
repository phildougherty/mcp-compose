package config

import (
	"os"
	"strings"
	"testing"
)

func TestNewEncryptionManager(t *testing.T) {
	tests := []struct {
		name        string
		envVar      string
		wantEnabled bool
	}{
		{
			name:        "with password",
			envVar:      "test-password-123",
			wantEnabled: true,
		},
		{
			name:        "without password",
			envVar:      "",
			wantEnabled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				os.Setenv(masterPasswordEnv, tt.envVar)
				defer os.Unsetenv(masterPasswordEnv)
			}

			em, err := NewEncryptionManager()
			if err != nil {
				t.Fatalf("NewEncryptionManager() error = %v", err)
			}

			if em.IsEnabled() != tt.wantEnabled {
				t.Errorf("IsEnabled() = %v, want %v", em.IsEnabled(), tt.wantEnabled)
			}
		})
	}
}

func TestEncryptionManager_Encrypt(t *testing.T) {
	os.Setenv(masterPasswordEnv, "test-password-123")
	defer os.Unsetenv(masterPasswordEnv)

	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
		wantErr   bool
	}{
		{
			name:      "normal text",
			plaintext: "secret-value-123",
			wantErr:   false,
		},
		{
			name:      "empty text",
			plaintext: "",
			wantErr:   false,
		},
		{
			name:      "already encrypted",
			plaintext: "ENC[base64data]",
			wantErr:   false,
		},
		{
			name:      "long text",
			plaintext: strings.Repeat("a", 1000),
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := em.Encrypt(tt.plaintext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.plaintext == "" {
				if encrypted != "" {
					t.Error("Expected empty ciphertext for empty plaintext")
				}

				return
			}

			if tt.plaintext == "ENC[base64data]" {
				if encrypted != tt.plaintext {
					t.Error("Expected already encrypted text to remain unchanged")
				}

				return
			}

			if !strings.HasPrefix(encrypted, encryptedPrefix) {
				t.Error("Expected encrypted text to have prefix")
			}

			if !strings.HasSuffix(encrypted, encryptedSuffix) {
				t.Error("Expected encrypted text to have suffix")
			}

			if encrypted == tt.plaintext {
				t.Error("Expected ciphertext to be different from plaintext")
			}
		})
	}
}

func TestEncryptionManager_Decrypt(t *testing.T) {
	os.Setenv(masterPasswordEnv, "test-password-123")
	defer os.Unsetenv(masterPasswordEnv)

	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	plaintext := "secret-value-123"
	encrypted, err := em.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
		want       string
		wantErr    bool
	}{
		{
			name:       "valid encrypted text",
			ciphertext: encrypted,
			want:       plaintext,
			wantErr:    false,
		},
		{
			name:       "unencrypted text",
			ciphertext: "plain-text",
			want:       "plain-text",
			wantErr:    false,
		},
		{
			name:       "empty text",
			ciphertext: "",
			want:       "",
			wantErr:    false,
		},
		{
			name:       "invalid format",
			ciphertext: "ENC[invalid-base64!!!]",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decrypted, err := em.Decrypt(tt.ciphertext)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if !tt.wantErr && decrypted != tt.want {
				t.Errorf("Decrypt() = %v, want %v", decrypted, tt.want)
			}
		})
	}
}

func TestEncryptionManager_RoundTrip(t *testing.T) {
	os.Setenv(masterPasswordEnv, "test-password-123")
	defer os.Unsetenv(masterPasswordEnv)

	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	testCases := []string{
		"simple-secret",
		"complex-secret-with-special-chars!@#$%^&*()",
		"multi\nline\nsecret",
		strings.Repeat("long-secret-", 100),
	}

	for _, plaintext := range testCases {
		encrypted, err := em.Encrypt(plaintext)
		if err != nil {
			t.Errorf("Encrypt() error = %v", err)

			continue
		}

		decrypted, err := em.Decrypt(encrypted)
		if err != nil {
			t.Errorf("Decrypt() error = %v", err)

			continue
		}

		if decrypted != plaintext {
			t.Errorf("Round trip failed: got %v, want %v", decrypted, plaintext)
		}
	}
}

func TestEncryptionManager_DisabledEncryption(t *testing.T) {
	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	if em.IsEnabled() {
		t.Skip("Skipping disabled encryption test as encryption is enabled")
	}

	plaintext := "secret-value"

	encrypted, err := em.Encrypt(plaintext)
	if err != nil {
		t.Errorf("Encrypt() error = %v", err)
	}

	if encrypted != plaintext {
		t.Error("Expected plaintext to remain unchanged when encryption is disabled")
	}

	decrypted, err := em.Decrypt(plaintext)
	if err != nil {
		t.Errorf("Decrypt() error = %v", err)
	}

	if decrypted != plaintext {
		t.Error("Expected plaintext to remain unchanged when decryption is disabled")
	}
}

func TestEncryptionManager_WrongPassword(t *testing.T) {
	os.Setenv(masterPasswordEnv, "password1")
	em1, _ := NewEncryptionManager()
	os.Unsetenv(masterPasswordEnv)

	plaintext := "secret-value"
	encrypted, err := em1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	os.Setenv(masterPasswordEnv, "password2")
	em2, _ := NewEncryptionManager()
	os.Unsetenv(masterPasswordEnv)

	_, err = em2.Decrypt(encrypted)
	if err == nil {
		t.Error("Expected error when decrypting with wrong password")
	}
}

func TestIsEncrypted(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "encrypted",
			value: "ENC[base64data]",
			want:  true,
		},
		{
			name:  "not encrypted",
			value: "plain-text",
			want:  false,
		},
		{
			name:  "partial prefix",
			value: "ENC[incomplete",
			want:  false,
		},
		{
			name:  "partial suffix",
			value: "incomplete]",
			want:  false,
		},
		{
			name:  "empty",
			value: "",
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEncrypted(tt.value)
			if got != tt.want {
				t.Errorf("isEncrypted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSensitiveField(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
		want      bool
	}{
		{
			name:      "api_key",
			fieldName: "api_key",
			want:      true,
		},
		{
			name:      "password",
			fieldName: "password",
			want:      true,
		},
		{
			name:      "case insensitive",
			fieldName: "API_KEY",
			want:      true,
		},
		{
			name:      "contains sensitive word",
			fieldName: "my_password_field",
			want:      true,
		},
		{
			name:      "not sensitive",
			fieldName: "username",
			want:      false,
		},
		{
			name:      "empty",
			fieldName: "",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSensitiveField(tt.fieldName)
			if got != tt.want {
				t.Errorf("isSensitiveField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncryptionManager_EncryptConfig(t *testing.T) {
	os.Setenv(masterPasswordEnv, "test-password-123")
	defer os.Unsetenv(masterPasswordEnv)

	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	secretValue := "my-secret-key"
	clientSecret := "oauth-client-secret"

	cfg := &ComposeConfig{
		ProxyAuth: ProxyAuthConfig{
			APIKey: secretValue,
		},
		OAuthClients: map[string]*OAuthClient{
			"test-client": {
				ClientSecret: &clientSecret,
			},
		},
		Memory: MemoryConfig{
			PostgresPassword: "db-password",
		},
		Servers: map[string]ServerConfig{
			"test-server": {
				Env: map[string]string{
					"API_KEY": "server-api-key",
					"USER":    "not-secret",
				},
			},
		},
	}

	err = em.EncryptConfig(cfg)
	if err != nil {
		t.Fatalf("EncryptConfig() error = %v", err)
	}

	if !isEncrypted(cfg.ProxyAuth.APIKey) {
		t.Error("Expected proxy API key to be encrypted")
	}

	if !isEncrypted(*cfg.OAuthClients["test-client"].ClientSecret) {
		t.Error("Expected OAuth client secret to be encrypted")
	}

	if !isEncrypted(cfg.Memory.PostgresPassword) {
		t.Error("Expected postgres password to be encrypted")
	}

	if !isEncrypted(cfg.Servers["test-server"].Env["API_KEY"]) {
		t.Error("Expected server API key to be encrypted")
	}

	if isEncrypted(cfg.Servers["test-server"].Env["USER"]) {
		t.Error("Expected non-sensitive env var to remain unencrypted")
	}
}

func TestEncryptionManager_DecryptConfig(t *testing.T) {
	os.Setenv(masterPasswordEnv, "test-password-123")
	defer os.Unsetenv(masterPasswordEnv)

	em, err := NewEncryptionManager()
	if err != nil {
		t.Fatalf("NewEncryptionManager() error = %v", err)
	}

	originalAPIKey := "my-secret-key"
	originalClientSecret := "oauth-client-secret"

	cfg := &ComposeConfig{
		ProxyAuth: ProxyAuthConfig{
			APIKey: originalAPIKey,
		},
		OAuthClients: map[string]*OAuthClient{
			"test-client": {
				ClientSecret: &originalClientSecret,
			},
		},
	}

	err = em.EncryptConfig(cfg)
	if err != nil {
		t.Fatalf("EncryptConfig() error = %v", err)
	}

	err = em.DecryptConfig(cfg)
	if err != nil {
		t.Fatalf("DecryptConfig() error = %v", err)
	}

	if cfg.ProxyAuth.APIKey != originalAPIKey {
		t.Errorf("Expected API key to be decrypted to original value, got %v", cfg.ProxyAuth.APIKey)
	}

	if *cfg.OAuthClients["test-client"].ClientSecret != originalClientSecret {
		t.Errorf("Expected client secret to be decrypted to original value, got %v", *cfg.OAuthClients["test-client"].ClientSecret)
	}
}

func TestGenerateMasterPassword(t *testing.T) {
	tests := []struct {
		name       string
		length     int
		wantLength int
	}{
		{
			name:       "default length",
			length:     32,
			wantLength: 32,
		},
		{
			name:       "custom length",
			length:     64,
			wantLength: 64,
		},
		{
			name:       "below minimum (should use default)",
			length:     8,
			wantLength: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := GenerateMasterPassword(tt.length)
			if err != nil {
				t.Errorf("GenerateMasterPassword() error = %v", err)

				return
			}

			if len(password) != tt.wantLength {
				t.Errorf("GenerateMasterPassword() length = %d, want %d", len(password), tt.wantLength)
			}
		})
	}

	pass1, _ := GenerateMasterPassword(32)
	pass2, _ := GenerateMasterPassword(32)

	if pass1 == pass2 {
		t.Error("Expected different passwords from multiple generations")
	}
}