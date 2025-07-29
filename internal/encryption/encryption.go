package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// EncryptionConfig holds encryption settings
type EncryptionConfig struct {
	Enabled bool
	Key     []byte
}

// CreateEncryptionConfig creates encryption config from password
func CreateEncryptionConfig(password string, enabled bool) *EncryptionConfig {
	if !enabled {
		return &EncryptionConfig{Enabled: false}
	}

	// Use SHA-256 to derive key from password
	hash := sha256.Sum256([]byte(password))
	return &EncryptionConfig{
		Enabled: true,
		Key:     hash[:],
	}
}

// Encrypt encrypts data using AES-256-GCM
func (ec *EncryptionConfig) Encrypt(plaintext []byte) ([]byte, error) {
	if !ec.Enabled {
		return plaintext, nil
	}

	block, err := aes.NewCipher(ec.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate a random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt the data
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-256-GCM
func (ec *EncryptionConfig) Decrypt(ciphertext []byte) ([]byte, error) {
	if !ec.Enabled {
		return ciphertext, nil
	}

	block, err := aes.NewCipher(ec.Key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// Extract nonce and encrypted data
	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// GenerateRandomKey generates a random 256-bit key for encryption
func GenerateRandomKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	_, err := rand.Read(key)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random key: %w", err)
	}
	return key, nil
}
