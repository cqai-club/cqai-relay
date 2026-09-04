package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSensitiveValueRoundTripAndBinding(t *testing.T) {
	previousSecret := CryptoSecret
	CryptoSecret = "stable-test-encryption-secret"
	t.Cleanup(func() { CryptoSecret = previousSecret })

	ciphertext, err := EncryptSensitiveValue("server-only-token", "internal-token")
	require.NoError(t, err)
	assert.NotContains(t, ciphertext, "server-only-token")
	assert.True(t, strings.HasPrefix(ciphertext, sensitiveValuePrefix))

	plaintext, err := DecryptSensitiveValue(ciphertext, "internal-token")
	require.NoError(t, err)
	assert.Equal(t, "server-only-token", plaintext)

	_, err = DecryptSensitiveValue(ciphertext, "different-setting")
	assert.ErrorIs(t, err, ErrSensitiveValueInvalid)
}

func TestSensitiveValueRejectsDifferentEncryptionSecret(t *testing.T) {
	previousSecret := CryptoSecret
	CryptoSecret = "first-secret"
	t.Cleanup(func() { CryptoSecret = previousSecret })

	ciphertext, err := EncryptSensitiveValue("server-only-token", "internal-token")
	require.NoError(t, err)

	CryptoSecret = "second-secret"
	_, err = DecryptSensitiveValue(ciphertext, "internal-token")
	assert.ErrorIs(t, err, ErrSensitiveValueInvalid)
}

func TestGenerateSecureTokenProducesURLSafeEntropy(t *testing.T) {
	token, err := GenerateSecureToken(32)
	require.NoError(t, err)
	assert.Len(t, token, 43)
	assert.NotContains(t, token, "=")
}
