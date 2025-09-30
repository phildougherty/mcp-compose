package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	encryptedPrefix    = "ENC["
	encryptedSuffix    = "]"
	saltLength         = 32
	keyLength          = 32
	pbkdf2Iterations   = 100000
	masterPasswordEnv  = "MCP_MASTER_PASSWORD"
	masterPasswordFile = ".mcp-master-password"
)

var sensitiveFields = map[string]bool{
	"api_key":                true,
	"password":               true,
	"password_hash":          true,
	"client_secret":          true,
	"oauth_client_secret":    true,
	"token":                  true,
	"access_token":           true,
	"refresh_token":          true,
	"secret":                 true,
	"database_url":           true,
	"postgres_password":      true,
	"openrouter_api_key":     true,
	"mcp_proxy_api_key":      true,
	"github_token":           true,
	"bearer_token":           true,
	"private_key":            true,
	"encryption_key":         true,
	"signing_key":            true,
}

type EncryptionManager struct {
	masterPassword string
	enabled        bool
}

func NewEncryptionManager() (*EncryptionManager, error) {
	masterPassword := os.Getenv(masterPasswordEnv)

	if masterPassword == "" {
		data, err := os.ReadFile(masterPasswordFile)
		if err == nil {
			masterPassword = strings.TrimSpace(string(data))
		}
	}

	if masterPassword == "" {

		return &EncryptionManager{
			enabled: false,
		}, nil
	}

	return &EncryptionManager{
		masterPassword: masterPassword,
		enabled:        true,
	}, nil
}

func (em *EncryptionManager) IsEnabled() bool {

	return em.enabled
}

func (em *EncryptionManager) Encrypt(plaintext string) (string, error) {
	if !em.enabled {

		return plaintext, nil
	}

	if plaintext == "" {

		return plaintext, nil
	}

	if strings.HasPrefix(plaintext, encryptedPrefix) {

		return plaintext, nil
	}

	salt := make([]byte, saltLength)

	if _, err := io.ReadFull(rand.Reader, salt); err != nil {

		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	key := pbkdf2.Key([]byte(em.masterPassword), salt, pbkdf2Iterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {

		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {

		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {

		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	combined := append(salt, ciphertext...)

	encoded := base64.StdEncoding.EncodeToString(combined)

	return encryptedPrefix + encoded + encryptedSuffix, nil
}

func (em *EncryptionManager) Decrypt(ciphertext string) (string, error) {
	if !em.enabled {

		return ciphertext, nil
	}

	if ciphertext == "" {

		return ciphertext, nil
	}

	if !strings.HasPrefix(ciphertext, encryptedPrefix) || !strings.HasSuffix(ciphertext, encryptedSuffix) {

		return ciphertext, nil
	}

	encoded := strings.TrimPrefix(ciphertext, encryptedPrefix)
	encoded = strings.TrimSuffix(encoded, encryptedSuffix)

	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {

		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	if len(combined) < saltLength {

		return "", fmt.Errorf("invalid ciphertext: too short")
	}

	salt := combined[:saltLength]
	encrypted := combined[saltLength:]

	key := pbkdf2.Key([]byte(em.masterPassword), salt, pbkdf2Iterations, keyLength, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {

		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {

		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	if len(encrypted) < gcm.NonceSize() {

		return "", fmt.Errorf("invalid ciphertext: nonce too short")
	}

	nonce := encrypted[:gcm.NonceSize()]
	ciphertextBytes := encrypted[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {

		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

func (em *EncryptionManager) EncryptConfig(cfg *ComposeConfig) error {
	if !em.enabled {

		return nil
	}

	if cfg.ProxyAuth.APIKey != "" && !isEncrypted(cfg.ProxyAuth.APIKey) {
		encrypted, err := em.Encrypt(cfg.ProxyAuth.APIKey)
		if err != nil {

			return fmt.Errorf("failed to encrypt proxy API key: %w", err)
		}
		cfg.ProxyAuth.APIKey = encrypted
	}

	if cfg.OAuth != nil {
		for clientID, client := range cfg.OAuthClients {
			if client.ClientSecret != nil && *client.ClientSecret != "" && !isEncrypted(*client.ClientSecret) {
				encrypted, err := em.Encrypt(*client.ClientSecret)
				if err != nil {

					return fmt.Errorf("failed to encrypt OAuth client secret for %s: %w", clientID, err)
				}
				cfg.OAuthClients[clientID].ClientSecret = &encrypted
			}
		}
	}

	if cfg.Memory.PostgresPassword != "" && !isEncrypted(cfg.Memory.PostgresPassword) {
		encrypted, err := em.Encrypt(cfg.Memory.PostgresPassword)
		if err != nil {

			return fmt.Errorf("failed to encrypt memory postgres password: %w", err)
		}
		cfg.Memory.PostgresPassword = encrypted
	}

	if cfg.Memory.DatabaseURL != "" && !isEncrypted(cfg.Memory.DatabaseURL) {
		encrypted, err := em.Encrypt(cfg.Memory.DatabaseURL)
		if err != nil {

			return fmt.Errorf("failed to encrypt memory database URL: %w", err)
		}
		cfg.Memory.DatabaseURL = encrypted
	}

	if cfg.TaskScheduler != nil {
		if cfg.TaskScheduler.OpenRouterAPIKey != "" && !isEncrypted(cfg.TaskScheduler.OpenRouterAPIKey) {
			encrypted, err := em.Encrypt(cfg.TaskScheduler.OpenRouterAPIKey)
			if err != nil {

				return fmt.Errorf("failed to encrypt task scheduler OpenRouter API key: %w", err)
			}
			cfg.TaskScheduler.OpenRouterAPIKey = encrypted
		}

		if cfg.TaskScheduler.MCPProxyAPIKey != "" && !isEncrypted(cfg.TaskScheduler.MCPProxyAPIKey) {
			encrypted, err := em.Encrypt(cfg.TaskScheduler.MCPProxyAPIKey)
			if err != nil {

				return fmt.Errorf("failed to encrypt task scheduler MCP proxy API key: %w", err)
			}
			cfg.TaskScheduler.MCPProxyAPIKey = encrypted
		}
	}

	for serverName, server := range cfg.Servers {
		for key, value := range server.Env {
			if isSensitiveField(key) && value != "" && !isEncrypted(value) {
				encrypted, err := em.Encrypt(value)
				if err != nil {

					return fmt.Errorf("failed to encrypt env var %s for server %s: %w", key, serverName, err)
				}
				server.Env[key] = encrypted
			}
		}
		cfg.Servers[serverName] = server
	}

	return nil
}

func (em *EncryptionManager) DecryptConfig(cfg *ComposeConfig) error {
	if !em.enabled {

		return nil
	}

	if cfg.ProxyAuth.APIKey != "" && isEncrypted(cfg.ProxyAuth.APIKey) {
		decrypted, err := em.Decrypt(cfg.ProxyAuth.APIKey)
		if err != nil {

			return fmt.Errorf("failed to decrypt proxy API key: %w", err)
		}
		cfg.ProxyAuth.APIKey = decrypted
	}

	if cfg.OAuth != nil {
		for clientID, client := range cfg.OAuthClients {
			if client.ClientSecret != nil && *client.ClientSecret != "" && isEncrypted(*client.ClientSecret) {
				decrypted, err := em.Decrypt(*client.ClientSecret)
				if err != nil {

					return fmt.Errorf("failed to decrypt OAuth client secret for %s: %w", clientID, err)
				}
				cfg.OAuthClients[clientID].ClientSecret = &decrypted
			}
		}
	}

	if cfg.Memory.PostgresPassword != "" && isEncrypted(cfg.Memory.PostgresPassword) {
		decrypted, err := em.Decrypt(cfg.Memory.PostgresPassword)
		if err != nil {

			return fmt.Errorf("failed to decrypt memory postgres password: %w", err)
		}
		cfg.Memory.PostgresPassword = decrypted
	}

	if cfg.Memory.DatabaseURL != "" && isEncrypted(cfg.Memory.DatabaseURL) {
		decrypted, err := em.Decrypt(cfg.Memory.DatabaseURL)
		if err != nil {

			return fmt.Errorf("failed to decrypt memory database URL: %w", err)
		}
		cfg.Memory.DatabaseURL = decrypted
	}

	if cfg.TaskScheduler != nil {
		if cfg.TaskScheduler.OpenRouterAPIKey != "" && isEncrypted(cfg.TaskScheduler.OpenRouterAPIKey) {
			decrypted, err := em.Decrypt(cfg.TaskScheduler.OpenRouterAPIKey)
			if err != nil {

				return fmt.Errorf("failed to decrypt task scheduler OpenRouter API key: %w", err)
			}
			cfg.TaskScheduler.OpenRouterAPIKey = decrypted
		}

		if cfg.TaskScheduler.MCPProxyAPIKey != "" && isEncrypted(cfg.TaskScheduler.MCPProxyAPIKey) {
			decrypted, err := em.Decrypt(cfg.TaskScheduler.MCPProxyAPIKey)
			if err != nil {

				return fmt.Errorf("failed to decrypt task scheduler MCP proxy API key: %w", err)
			}
			cfg.TaskScheduler.MCPProxyAPIKey = decrypted
		}
	}

	for serverName, server := range cfg.Servers {
		for key, value := range server.Env {
			if isSensitiveField(key) && value != "" && isEncrypted(value) {
				decrypted, err := em.Decrypt(value)
				if err != nil {

					return fmt.Errorf("failed to decrypt env var %s for server %s: %w", key, serverName, err)
				}
				server.Env[key] = decrypted
			}
		}
		cfg.Servers[serverName] = server
	}

	return nil
}

func isEncrypted(value string) bool {

	return strings.HasPrefix(value, encryptedPrefix) && strings.HasSuffix(value, encryptedSuffix)
}

func isSensitiveField(fieldName string) bool {
	lowerField := strings.ToLower(fieldName)

	if sensitiveFields[lowerField] {

		return true
	}

	for sensitive := range sensitiveFields {
		if strings.Contains(lowerField, sensitive) {

			return true
		}
	}

	return false
}

func GenerateMasterPassword(length int) (string, error) {
	if length < 16 {
		length = 32
	}

	bytes := make([]byte, length)

	if _, err := rand.Read(bytes); err != nil {

		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	password := base64.URLEncoding.EncodeToString(bytes)

	if len(password) > length {
		password = password[:length]
	}

	return password, nil
}