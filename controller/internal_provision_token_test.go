package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGenerateInternalProvisionTokenReturnsUnsavedSecureValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousToken, previousSource := common.GetInternalProvisionToken()
	common.SetInternalProvisionToken("current-token", "environment")
	t.Cleanup(func() { common.SetInternalProvisionToken(previousToken, previousSource) })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	GenerateInternalProvisionToken(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(response.Body.Bytes(), &payload))
	assert.True(t, payload.Success)
	assert.Len(t, payload.Data.Token, 43)
	token, source := common.GetInternalProvisionToken()
	assert.Equal(t, "current-token", token)
	assert.Equal(t, "environment", source)
}

func TestUpdateInternalProvisionTokenStoresCiphertext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB := model.DB
	previousCryptoSecret := common.CryptoSecret
	previousCryptoSecretPersistent := common.CryptoSecretPersistent
	previousOptionMap := common.OptionMap
	previousToken, previousSource := common.GetInternalProvisionToken()
	t.Cleanup(func() {
		model.DB = previousDB
		common.CryptoSecret = previousCryptoSecret
		common.CryptoSecretPersistent = previousCryptoSecretPersistent
		common.OptionMap = previousOptionMap
		common.SetInternalProvisionToken(previousToken, previousSource)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Option{}))
	model.DB = db
	common.OptionMap = make(map[string]string)
	common.CryptoSecret = "stable-controller-test-secret"
	common.CryptoSecretPersistent = true

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/internal-token", strings.NewReader(`{"token":"0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"}`))
	UpdateInternalProvisionToken(context)

	require.Equal(t, http.StatusOK, response.Code)
	var option model.Option
	require.NoError(t, db.First(&option, "key = ?", common.InternalProvisionTokenOptionKey).Error)
	assert.NotContains(t, option.Value, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG")
	token, source := common.GetInternalProvisionToken()
	assert.Equal(t, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFG", token)
	assert.Equal(t, "database", source)
}

func TestUpdateInternalProvisionTokenRequiresStableEncryptionSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previous := common.CryptoSecretPersistent
	common.CryptoSecretPersistent = false
	t.Cleanup(func() { common.CryptoSecretPersistent = previous })

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/internal-token", strings.NewReader(`{"token":"0123456789abcdefghijklmnopqrstuvwxyzABCDEFG"}`))
	UpdateInternalProvisionToken(context)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "SESSION_SECRET")
}

func TestGenericOptionUpdateCannotStorePlaintextInternalToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodPut, "/api/option/", strings.NewReader(`{"key":"InternalProvisionToken","value":"plaintext-secret"}`))

	UpdateOption(context)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), "dedicated internal token endpoint")
}
