package service

import (
	"fmt"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAccountProvisionTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:account-provision-%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.Token{},
		&model.ExternalAccountIdentity{},
		&model.AppCredential{},
		&model.Log{},
	))

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousQuota, previousRedis := common.QuotaForNewUser, common.RedisEnabled
	model.DB, model.LOG_DB = db, db
	common.QuotaForNewUser = 1_000
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.QuotaForNewUser, common.RedisEnabled = previousQuota, previousRedis
		_ = sqlDB.Close()
	})
	return db
}

func TestProvisionAccountIsIdempotentPerIdentityAndPlatform(t *testing.T) {
	db := setupAccountProvisionTestDB(t)
	request := AccountProvisionRequest{
		Issuer:   "https://auth.example.test/oidc",
		Subject:  "logto-user-1",
		Platform: "lingweave",
		Email:    "User@Example.test",
		Name:     "测试用户",
	}

	first, err := ProvisionAccount(request)
	require.NoError(t, err)
	second, err := ProvisionAccount(request)
	require.NoError(t, err)

	assert.True(t, first.UserCreated)
	assert.True(t, first.KeyCreated)
	assert.False(t, second.UserCreated)
	assert.False(t, second.KeyCreated)
	assert.Equal(t, first.UserId, second.UserId)
	assert.Equal(t, first.TokenId, second.TokenId)
	assert.Equal(t, first.ApiKey, second.ApiKey)
	assert.Equal(t, 1_000, first.Quota)
	assert.Equal(t, "lingweave", first.Platform)
	assert.Regexp(t, `^sk-[0-9A-Za-z]{48}$`, first.ApiKey)

	other, err := ProvisionAccount(AccountProvisionRequest{
		Issuer:   request.Issuer,
		Subject:  request.Subject,
		Platform: "another_app",
	})
	require.NoError(t, err)
	assert.Equal(t, first.UserId, other.UserId)
	assert.NotEqual(t, first.TokenId, other.TokenId)
	assert.NotEqual(t, first.ApiKey, other.ApiKey)
	assert.False(t, other.UserCreated)
	assert.True(t, other.KeyCreated)

	var user model.User
	require.NoError(t, db.First(&user, first.UserId).Error)
	assert.Equal(t, common.RoleCommonUser, user.Role)
	assert.Equal(t, common.UserStatusEnabled, user.Status)
	assert.Equal(t, "default", user.Group)
	assert.Equal(t, "user@example.test", user.Email)
	var tokens []model.Token
	require.NoError(t, db.Where("user_id = ?", first.UserId).Find(&tokens).Error)
	require.Len(t, tokens, 2)
	for _, token := range tokens {
		assert.False(t, token.UnlimitedQuota)
		assert.Equal(t, common.QuotaForNewUser, token.RemainQuota)
	}
}

func TestProvisionAccountHandlesConcurrentFirstRequests(t *testing.T) {
	db := setupAccountProvisionTestDB(t)
	request := AccountProvisionRequest{
		Issuer:   "https://auth.example.test/oidc",
		Subject:  "concurrent-user",
		Platform: "lingweave",
	}

	const workers = 4
	results := make([]*AccountProvisionResponse, workers)
	errorsByWorker := make([]error, workers)
	var waitGroup sync.WaitGroup
	for index := 0; index < workers; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			results[index], errorsByWorker[index] = ProvisionAccount(request)
		}(index)
	}
	waitGroup.Wait()

	for index := range results {
		require.NoError(t, errorsByWorker[index])
		require.NotNil(t, results[index])
		assert.Equal(t, results[0].UserId, results[index].UserId)
		assert.Equal(t, results[0].TokenId, results[index].TokenId)
		assert.Equal(t, results[0].ApiKey, results[index].ApiKey)
	}
	var userCount, tokenCount, identityCount, credentialCount int64
	require.NoError(t, db.Model(&model.User{}).Count(&userCount).Error)
	require.NoError(t, db.Model(&model.Token{}).Count(&tokenCount).Error)
	require.NoError(t, db.Model(&model.ExternalAccountIdentity{}).Count(&identityCount).Error)
	require.NoError(t, db.Model(&model.AppCredential{}).Count(&credentialCount).Error)
	assert.EqualValues(t, 1, userCount)
	assert.EqualValues(t, 1, tokenCount)
	assert.EqualValues(t, 1, identityCount)
	assert.EqualValues(t, 1, credentialCount)
}

func TestProvisionAccountAppliesRoleOnlyOnFirstCreation(t *testing.T) {
	db := setupAccountProvisionTestDB(t)
	request := AccountProvisionRequest{
		Issuer:   "https://auth.example.test/oidc",
		Subject:  "role-user",
		Platform: "lingweave",
		Role:     100,
	}

	first, err := ProvisionAccount(request)
	require.NoError(t, err)
	require.True(t, first.UserCreated)
	var user model.User
	require.NoError(t, db.First(&user, first.UserId).Error)
	assert.Equal(t, common.RoleRootUser, user.Role)
	var persisted model.User
	require.NoError(t, db.First(&persisted, first.UserId).Error)
	assert.Equal(t, common.RoleRootUser, persisted.Role)

	// Second provisioning with a different role must not re-role the user.
	request.Role = common.RoleCommonUser
	second, err := ProvisionAccount(request)
	require.NoError(t, err)
	require.False(t, second.UserCreated)
	assert.Equal(t, first.UserId, second.UserId)
	assert.Equal(t, common.RoleRootUser, user.Role)
	var persistedAfter model.User
	require.NoError(t, db.First(&persistedAfter, first.UserId).Error)
	assert.Equal(t, common.RoleRootUser, persistedAfter.Role)

	// Unknown/invalid role falls back to common user.
	request.Subject = "role-user-2"
	request.Role = 999
	res, err := ProvisionAccount(request)
	require.NoError(t, err)
	require.True(t, res.UserCreated)
	var user2 model.User
	require.NoError(t, db.First(&user2, res.UserId).Error)
	assert.Equal(t, common.RoleCommonUser, user2.Role)
}

func TestProvisionAccountRejectsInvalidIdentity(t *testing.T) {
	_, err := ProvisionAccount(AccountProvisionRequest{
		Issuer:   "not-a-url",
		Subject:  "user",
		Platform: "LingWeave!",
	})
	require.ErrorIs(t, err, ErrInvalidAccountProvisionRequest)
}
