package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionsBulkAtomicallyRestoresPrepublishedPricingWhenPublicationFails(t *testing.T) {
	previousDB := DB
	previousRatios := ratio_setting.GetModelRatioCopy()
	t.Cleanup(func() {
		DB = previousDB
		encoded, err := common.Marshal(previousRatios)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
	})

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	DB = db
	require.NoError(t, db.AutoMigrate(&Option{}))

	baseRatios := map[string]float64{"legacy-model": 2}
	baseJSON, err := common.Marshal(baseRatios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(baseJSON)))
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		"ModelRatio": string(baseJSON),
		"PayMethods": operation_setting.PayMethods2JsonString(),
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	nextRatios := map[string]float64{"legacy-model": 2, "canonical-model": 2}
	nextJSON, err := common.Marshal(nextRatios)
	require.NoError(t, err)

	err = UpdateOptionsBulkAtomically(map[string]string{
		"ModelRatio": string(nextJSON),
		"PayMethods": "{",
	}, func(_ *gorm.DB) error { return nil })
	require.Error(t, err)

	_, canonicalConfigured, _ := ratio_setting.GetModelRatio("canonical-model")
	assert.False(t, canonicalConfigured)
	legacyRatio, legacyConfigured, _ := ratio_setting.GetModelRatio("legacy-model")
	assert.True(t, legacyConfigured)
	assert.Equal(t, 2.0, legacyRatio)

	var optionCount int64
	require.NoError(t, db.Model(&Option{}).Count(&optionCount).Error)
	assert.Zero(t, optionCount)
}
