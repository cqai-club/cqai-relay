package model

import (
	"errors"
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type legacyModelMetadataRow struct {
	Id           int    `gorm:"primaryKey"`
	ModelName    string `gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Capabilities string `gorm:"type:text"`
	Status       int
	SyncOfficial int
	DeletedAt    gorm.DeletedAt `gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`
}

func (legacyModelMetadataRow) TableName() string {
	return "models"
}

// TestModelAliasDatabaseCompatibility is intentionally opt-in because it needs
// an empty disposable MySQL or PostgreSQL database. CI and release checks can
// run it with MODEL_ALIAS_DATABASE_TEST_DRIVER and
// MODEL_ALIAS_DATABASE_TEST_DSN.
func TestModelAliasDatabaseCompatibility(t *testing.T) {
	driver := os.Getenv("MODEL_ALIAS_DATABASE_TEST_DRIVER")
	dsn := os.Getenv("MODEL_ALIAS_DATABASE_TEST_DSN")
	if driver == "" || dsn == "" {
		t.Skip("external database test is not configured")
	}

	var (
		db     *gorm.DB
		dbType common.DatabaseType
		err    error
	)
	switch driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		dbType = common.DatabaseTypeMySQL
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
		dbType = common.DatabaseTypePostgreSQL
	default:
		t.Fatalf("unsupported integration database driver %q", driver)
	}
	require.NoError(t, err)

	previousDB := DB
	previousLogDB := LOG_DB
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	DB = db
	LOG_DB = db
	common.SetMainDatabaseType(dbType)
	common.SetLogDatabaseType(dbType)
	common.RedisEnabled = false
	initCol()
	t.Cleanup(func() {
		modelAliasLock.Lock()
		modelAliasCache = make(map[string]ModelAlias)
		modelAliasLock.Unlock()
		aliasUsageLock.Lock()
		aliasUsageMap = make(map[string]aliasUsage)
		aliasUsageLock.Unlock()
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousMainDatabaseType)
		common.SetLogDatabaseType(previousLogDatabaseType)
		common.RedisEnabled = previousRedisEnabled
		initCol()
	})

	require.NoError(t, db.AutoMigrate(&legacyModelMetadataRow{}))
	require.NoError(t, db.Create(&legacyModelMetadataRow{
		ModelName: "known-with-capabilities", Capabilities: `["text"]`, Status: 1, SyncOfficial: 1,
	}).Error)
	require.NoError(t, db.Create(&legacyModelMetadataRow{
		ModelName: "unknown-without-capabilities", Capabilities: `[]`, Status: 1, SyncOfficial: 1,
	}).Error)

	require.NoError(t, migrateDB())
	assert.True(t, db.Migrator().HasTable(&ModelAlias{}))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "MetadataStatus"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "MetadataSource"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "InputModalities"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "OutputModalities"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "SupportedParameters"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "ContextLength"))
	assert.True(t, db.Migrator().HasColumn(&Model{}, "MaxOutputTokens"))

	var confirmed Model
	require.NoError(t, db.Where("model_name = ?", "known-with-capabilities").First(&confirmed).Error)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryText}, confirmed.Capabilities)
	assert.Equal(t, ModelMetadataStatusConfirmed, confirmed.MetadataStatus)
	assert.Equal(t, ModelMetadataSourceMigration, confirmed.MetadataSource)
	confirmed.InputModalities = []string{"text", "image", "file"}
	confirmed.OutputModalities = []string{"text"}
	confirmed.SupportedParameters = []string{"reasoning", "tools"}
	confirmed.ContextLength = 1_050_000
	confirmed.MaxOutputTokens = 128_000
	require.NoError(t, db.Model(&Model{}).Where("id = ?", confirmed.Id).
		Select("input_modalities", "output_modalities", "supported_parameters", "context_length", "max_output_tokens").
		Updates(&confirmed).Error)
	var structured Model
	require.NoError(t, db.Where("id = ?", confirmed.Id).First(&structured).Error)
	assert.Equal(t, []string{"text", "image", "file"}, structured.InputModalities)
	assert.Equal(t, []string{"text"}, structured.OutputModalities)
	assert.Equal(t, []string{"reasoning", "tools"}, structured.SupportedParameters)
	assert.EqualValues(t, 1_050_000, structured.ContextLength)
	assert.EqualValues(t, 128_000, structured.MaxOutputTokens)

	var pending Model
	require.NoError(t, db.Where("model_name = ?", "unknown-without-capabilities").First(&pending).Error)
	assert.Empty(t, pending.Capabilities)
	assert.Equal(t, ModelMetadataStatusPending, pending.MetadataStatus)
	assert.Equal(t, ModelMetadataSourceMigration, pending.MetadataSource)

	require.NoError(t, CreateOrActivateModelAlias(db, "legacy-known", confirmed.ModelName, ModelMetadataSourceMigration, 1234))
	require.NoError(t, CreateOrActivateModelAlias(db, "legacy-known", confirmed.ModelName, ModelMetadataSourceMigration, 1234))
	var count int64
	require.NoError(t, db.Model(&ModelAlias{}).Where("alias_name = ?", "legacy-known").Count(&count).Error)
	assert.EqualValues(t, 1, count)
	conflictingModel := &Model{
		ModelName: "legacy-known", MetadataStatus: ModelMetadataStatusPending,
		MetadataSource: ModelMetadataSourceChannel, Status: 1, SyncOfficial: 1,
	}
	assert.Error(t, conflictingModel.Insert())
	require.NoError(t, EnsureModelMetadataRecords(db, []string{"legacy-known"}, ModelMetadataSourceChannel))
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "legacy-known").Count(&count).Error)
	assert.Zero(t, count)

	duplicate := ModelAlias{
		AliasName: "legacy-known", CanonicalModelName: confirmed.ModelName,
		Status: ModelAliasStatusActive, RetireAfter: 1234, Source: ModelMetadataSourceMigration,
	}
	assert.Error(t, db.Create(&duplicate).Error)

	rollbackMarker := errors.New("rollback marker")
	err = db.Transaction(func(tx *gorm.DB) error {
		require.NoError(t, CreateOrActivateModelAlias(tx, "rolled-back-alias", confirmed.ModelName, ModelMetadataSourceMigration, 1234))
		return rollbackMarker
	})
	assert.ErrorIs(t, err, rollbackMarker)
	require.NoError(t, db.Model(&ModelAlias{}).Where("alias_name = ?", "rolled-back-alias").Count(&count).Error)
	assert.Zero(t, count)

	// Re-running the application migration must preserve the migrated metadata
	// and the single alias row.
	require.NoError(t, migrateDB())
	require.NoError(t, db.Model(&ModelAlias{}).Where("alias_name = ?", "legacy-known").Count(&count).Error)
	assert.EqualValues(t, 1, count)
}
