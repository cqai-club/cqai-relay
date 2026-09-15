package model

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureModelMetadataRecordsCreatesPendingOtherIdempotently(t *testing.T) {
	db := setupModelAliasTestDB(t)
	require.NoError(t, EnsureModelMetadataRecords(db, []string{"unknown-model", "unknown-model", ""}, ModelMetadataSourceChannel))
	require.NoError(t, EnsureModelMetadataRecords(db, []string{"unknown-model"}, ModelMetadataSourceChannel))

	var rows []Model
	require.NoError(t, db.Where("model_name = ?", "unknown-model").Find(&rows).Error)
	require.Len(t, rows, 1)
	assert.Equal(t, ModelMetadataStatusPending, rows[0].MetadataStatus)
	assert.Equal(t, ModelMetadataSourceChannel, rows[0].MetadataSource)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, rows[0].Capabilities)
}

func TestEnsureModelMetadataRecordsDoesNotRecreateAliasAsStandardModel(t *testing.T) {
	db := setupModelAliasTestDB(t)
	createAliasTestModel(t, db, "canonical-model")
	require.NoError(t, CreateOrActivateModelAlias(db, "legacy-model", "canonical-model", ModelMetadataSourceMigration, 100))

	require.NoError(t, EnsureModelMetadataRecords(db, []string{"legacy-model"}, ModelMetadataSourceChannel))

	var count int64
	require.NoError(t, db.Model(&Model{}).Where("model_name = ?", "legacy-model").Count(&count).Error)
	assert.Zero(t, count)
}

func TestInitializeModelMetadataClassifiesLegacyRows(t *testing.T) {
	db := setupModelAliasTestDB(t)
	require.NoError(t, db.Create(&[]Model{
		{ModelName: "legacy-known", Capabilities: []types.ModelCategory{types.ModelCategoryText}, Status: 1},
		{ModelName: "legacy-unknown", Status: 1},
	}).Error)
	require.NoError(t, db.Create(&Ability{Group: "default", Model: "disabled-channel-model", ChannelId: 9, Enabled: false}).Error)

	require.NoError(t, InitializeModelMetadata())

	var known Model
	require.NoError(t, db.Where("model_name = ?", "legacy-known").First(&known).Error)
	assert.Equal(t, ModelMetadataStatusConfirmed, known.MetadataStatus)
	assert.Equal(t, ModelMetadataSourceMigration, known.MetadataSource)
	var unknown Model
	require.NoError(t, db.Where("model_name = ?", "legacy-unknown").First(&unknown).Error)
	assert.Equal(t, ModelMetadataStatusPending, unknown.MetadataStatus)
	assert.Equal(t, ModelMetadataSourceMigration, unknown.MetadataSource)
	var disabled Model
	require.NoError(t, db.Where("model_name = ?", "disabled-channel-model").First(&disabled).Error)
	assert.Equal(t, ModelMetadataStatusPending, disabled.MetadataStatus)
	assert.Equal(t, ModelMetadataSourceChannel, disabled.MetadataSource)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, disabled.Capabilities)
}
