package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelAliasTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	common.RedisEnabled = false
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Model{}, &ModelAlias{}, &Token{}, &Ability{}))
	require.NoError(t, InitModelAliasCache())
	t.Cleanup(func() {
		modelAliasLock.Lock()
		modelAliasCache = make(map[string]ModelAlias)
		modelAliasLock.Unlock()
		aliasUsageLock.Lock()
		aliasUsageMap = make(map[string]aliasUsage)
		aliasUsageLock.Unlock()
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})
	return db
}

func createAliasTestModel(t *testing.T, db *gorm.DB, name string) {
	t.Helper()
	require.NoError(t, db.Create(&Model{
		ModelName: name, Capabilities: []types.ModelCategory{types.ModelCategoryText},
		MetadataStatus: ModelMetadataStatusConfirmed, MetadataSource: ModelMetadataSourceManual,
		Status: 1, SyncOfficial: 1,
	}).Error)
}

func TestModelAliasLifecycleAndIdempotency(t *testing.T) {
	db := setupModelAliasTestDB(t)
	createAliasTestModel(t, db, "gpt-5")

	require.NoError(t, CreateOrActivateModelAlias(db, "OpenAI/GPT-5", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 1234))
	require.NoError(t, CreateOrActivateModelAlias(db, "OpenAI/GPT-5", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 5678))
	require.NoError(t, InitModelAliasCache())

	canonical, alias := ResolveModelAlias("OpenAI/GPT-5")
	assert.Equal(t, "gpt-5", canonical)
	require.NotNil(t, alias)
	assert.EqualValues(t, 5678, alias.RetireAfter)
	var count int64
	require.NoError(t, db.Model(&ModelAlias{}).Where("alias_name = ?", "OpenAI/GPT-5").Count(&count).Error)
	assert.EqualValues(t, 1, count)
	hasAliases, err := ModelHasActiveAliases("gpt-5")
	require.NoError(t, err)
	assert.True(t, hasAliases)
	expired, err := ListModelAliases("expired")
	require.NoError(t, err)
	require.Len(t, expired, 1)
	assert.Equal(t, "OpenAI/GPT-5", expired[0].AliasName)
}

func TestModelAliasRejectsStandardNameChainsAndOccupiedAliases(t *testing.T) {
	db := setupModelAliasTestDB(t)
	createAliasTestModel(t, db, "gpt-5")
	createAliasTestModel(t, db, "claude-4")
	createAliasTestModel(t, db, "existing-standard")

	require.NoError(t, CreateOrActivateModelAlias(db, "legacy-gpt", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 100))
	assert.Error(t, CreateOrActivateModelAlias(db, "other-legacy", "legacy-gpt", ModelMetadataSourceBaseLLMNormalized, 100))
	assert.Error(t, CreateOrActivateModelAlias(db, "existing-standard", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 100))
	assert.Error(t, CreateOrActivateModelAlias(db, "legacy-gpt", "claude-4", ModelMetadataSourceBaseLLMNormalized, 100))
	assert.Error(t, CreateOrActivateModelAlias(db, "invalid-source", "gpt-5", "unknown", 100))
}

func TestModelCreationAndRenameRejectNamesReservedByAliases(t *testing.T) {
	db := setupModelAliasTestDB(t)
	createAliasTestModel(t, db, "gpt-5")
	require.NoError(t, CreateOrActivateModelAlias(db, "legacy-gpt", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 100))

	conflicting := &Model{
		ModelName: "legacy-gpt", MetadataStatus: ModelMetadataStatusConfirmed,
		MetadataSource: ModelMetadataSourceManual, Status: 1, SyncOfficial: 1,
	}
	assert.Error(t, conflicting.Insert())

	rename := &Model{
		ModelName: "rename-source", MetadataStatus: ModelMetadataStatusConfirmed,
		MetadataSource: ModelMetadataSourceManual, Status: 1, SyncOfficial: 1,
	}
	require.NoError(t, rename.Insert())
	rename.ModelName = "legacy-gpt"
	assert.Error(t, rename.Update())
}

func TestRetireModelAliasesMigratesTokenLimitsBeforeDisabling(t *testing.T) {
	db := setupModelAliasTestDB(t)
	createAliasTestModel(t, db, "gpt-5")
	require.NoError(t, CreateOrActivateModelAlias(db, "OpenAI/GPT-5", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 100))
	require.NoError(t, InitModelAliasCache())
	require.NoError(t, db.Create(&Token{
		UserId: 1, Key: "test-token-key", Name: "legacy-token", Status: common.TokenStatusEnabled,
		ModelLimitsEnabled: true, ModelLimits: "OpenAI/GPT-5,gpt-5,other-model",
	}).Error)

	migratedTokens, retiredAliases, err := RetireModelAliases([]string{"OpenAI/GPT-5"})
	require.NoError(t, err)
	assert.Equal(t, 1, migratedTokens)
	assert.Equal(t, 1, retiredAliases)

	var token Token
	require.NoError(t, db.Where("key = ?", "test-token-key").First(&token).Error)
	assert.Equal(t, "gpt-5,other-model", token.ModelLimits)
	canonical, alias := ResolveModelAlias("OpenAI/GPT-5")
	assert.Equal(t, "OpenAI/GPT-5", canonical)
	assert.Nil(t, alias)
	var stored ModelAlias
	require.NoError(t, db.Where("alias_name = ?", "OpenAI/GPT-5").First(&stored).Error)
	assert.Equal(t, ModelAliasStatusRetired, stored.Status)
	hasAliases, err := ModelHasActiveAliases("gpt-5")
	require.NoError(t, err)
	assert.False(t, hasAliases)
}

func TestRetireModelAliasesInvalidatesSnapshotRepublishedBeforeCommit(t *testing.T) {
	db := setupModelAliasTestDB(t)
	server := miniredis.RunT(t)
	previousRDB := common.RDB
	common.RedisEnabled = true
	common.RDB = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RDB = previousRDB
	})

	createAliasTestModel(t, db, "gpt-5")
	require.NoError(t, CreateOrActivateModelAlias(db, "OpenAI/GPT-5", "gpt-5", ModelMetadataSourceBaseLLMNormalized, 100))
	require.NoError(t, InitModelAliasCache())
	staleToken := Token{
		UserId: 1, Key: "stale-alias-token", Name: "legacy-token", Status: common.TokenStatusEnabled,
		ModelLimitsEnabled: true, ModelLimits: "OpenAI/GPT-5",
	}
	require.NoError(t, db.Create(&staleToken).Error)

	republished := false
	require.NoError(t, db.Callback().Update().After("gorm:update").Register("test:republish_stale_token", func(tx *gorm.DB) {
		if tx.Statement.Table != "tokens" || republished {
			return
		}
		republished = true
		server.FastForward(time.Duration(tokenCacheFenceSeconds+1) * time.Second)
		_, err := cacheInitToken(staleToken)
		require.NoError(t, err)
	}))

	migratedTokens, retiredAliases, err := RetireModelAliases([]string{"OpenAI/GPT-5"})
	require.NoError(t, err)
	assert.Equal(t, 1, migratedTokens)
	assert.Equal(t, 1, retiredAliases)
	assert.True(t, republished)
	assert.False(t, server.Exists(getTokenCacheKey(staleToken.Key)))
	assert.True(t, server.Exists(getTokenCacheFenceKey(staleToken.Key)))
}
