package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateInternalProvisionTokenEncryptsStorageAndUpdatesRuntime(t *testing.T) {
	previousDB := DB
	previousCryptoSecret := common.CryptoSecret
	previousOptionMap := common.OptionMap
	previousToken, previousSource := common.GetInternalProvisionToken()
	t.Cleanup(func() {
		DB = previousDB
		common.CryptoSecret = previousCryptoSecret
		common.OptionMap = previousOptionMap
		common.SetInternalProvisionToken(previousToken, previousSource)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}))
	DB = db
	common.CryptoSecret = "stable-database-encryption-secret"
	common.OptionMap = make(map[string]string)

	require.NoError(t, UpdateInternalProvisionToken("server-only-token-value"))

	var option Option
	require.NoError(t, db.First(&option, "key = ?", common.InternalProvisionTokenOptionKey).Error)
	assert.NotEqual(t, "server-only-token-value", option.Value)
	assert.NotContains(t, option.Value, "server-only-token-value")
	token, source := common.GetInternalProvisionToken()
	assert.Equal(t, "server-only-token-value", token)
	assert.Equal(t, "database", source)
}

func TestApplyInternalProvisionTokenFailsClosedForInvalidCiphertext(t *testing.T) {
	previousToken, previousSource := common.GetInternalProvisionToken()
	t.Cleanup(func() { common.SetInternalProvisionToken(previousToken, previousSource) })
	common.SetInternalProvisionToken("environment-fallback", "environment")

	err := applyInternalProvisionTokenOption("not-encrypted")

	assert.Error(t, err)
	token, source := common.GetInternalProvisionToken()
	assert.Empty(t, token)
	assert.Equal(t, "database", source)
}
