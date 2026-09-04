package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const sensitiveValuePrefix = "enc:v1:"

var ErrSensitiveValueInvalid = errors.New("encrypted sensitive value is invalid")

// EncryptSensitiveValue encrypts a value for database storage. The purpose is
// authenticated with the ciphertext so a secret cannot be moved between
// unrelated settings.
func EncryptSensitiveValue(value, purpose string) (string, error) {
	if strings.TrimSpace(purpose) == "" {
		return "", errors.New("sensitive value purpose is required")
	}
	block, err := aes.NewCipher(sensitiveValueKey())
	if err != nil {
		return "", fmt.Errorf("initialize sensitive value cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("initialize sensitive value GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate sensitive value nonce: %w", err)
	}
	payload := gcm.Seal(nonce, nonce, []byte(value), []byte(purpose))
	return sensitiveValuePrefix + base64.RawURLEncoding.EncodeToString(payload), nil
}

func DecryptSensitiveValue(value, purpose string) (string, error) {
	if strings.TrimSpace(purpose) == "" || !strings.HasPrefix(value, sensitiveValuePrefix) {
		return "", ErrSensitiveValueInvalid
	}
	payload, err := base64.RawURLEncoding.Strict().DecodeString(strings.TrimPrefix(value, sensitiveValuePrefix))
	if err != nil {
		return "", ErrSensitiveValueInvalid
	}
	block, err := aes.NewCipher(sensitiveValueKey())
	if err != nil {
		return "", fmt.Errorf("initialize sensitive value cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("initialize sensitive value GCM: %w", err)
	}
	if len(payload) < gcm.NonceSize()+gcm.Overhead() {
		return "", ErrSensitiveValueInvalid
	}
	nonce := payload[:gcm.NonceSize()]
	plaintext, err := gcm.Open(nil, nonce, payload[gcm.NonceSize():], []byte(purpose))
	if err != nil {
		return "", ErrSensitiveValueInvalid
	}
	return string(plaintext), nil
}

func GenerateSecureToken(byteLength int) (string, error) {
	if byteLength <= 0 {
		return "", errors.New("secure token length must be positive")
	}
	random := make([]byte, byteLength)
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(random), nil
}

func sensitiveValueKey() []byte {
	digest := sha256.Sum256([]byte("new-api-sensitive-value-v1:" + CryptoSecret))
	return digest[:]
}
