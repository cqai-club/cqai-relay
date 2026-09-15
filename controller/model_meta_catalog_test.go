package controller

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareManualModelMetadataDerivesCategories(t *testing.T) {
	item := model.Model{
		InputModalities:     []string{" Text ", "IMAGE"},
		OutputModalities:    []string{"text"},
		SupportedParameters: []string{"Tools", "tools", "Reasoning"},
		Capabilities:        []types.ModelCategory{types.ModelCategoryOther},
		ContextLength:       128000,
		MaxOutputTokens:     8192,
	}
	require.NoError(t, prepareManualModelMetadata(&item))
	assert.Equal(t, []string{"text", "image"}, item.InputModalities)
	assert.Equal(t, []string{"text"}, item.OutputModalities)
	assert.Equal(t, []string{"reasoning", "tools"}, item.SupportedParameters)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryTextMultimodal}, item.Capabilities)
	assert.Equal(t, model.ModelMetadataStatusConfirmed, item.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceManual, item.MetadataSource)
}

func TestRestoreModelMetadataAutoPreservesLastKnownMetadata(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	item := model.Model{
		ModelName: "manual-vision", Description: "keep", InputModalities: []string{"text", "image"},
		OutputModalities: []string{"text"}, SupportedParameters: []string{"tools"},
		Capabilities:  []types.ModelCategory{types.ModelCategoryTextMultimodal},
		ContextLength: 128000, MaxOutputTokens: 8192,
		MetadataStatus: model.ModelMetadataStatusConfirmed, MetadataSource: model.ModelMetadataSourceManual,
		Status: 1, SyncOfficial: 0,
	}
	require.NoError(t, db.Create(&item).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	id := strconv.Itoa(item.Id)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/models/"+id+"/restore_auto", nil)
	ctx.Params = gin.Params{{Key: "id", Value: id}}
	RestoreModelMetadataAuto(ctx)
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool        `json:"success"`
		Data    model.Model `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.True(t, response.Success)
	assert.Equal(t, model.ModelMetadataStatusPending, response.Data.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceChannel, response.Data.MetadataSource)
	assert.Equal(t, 1, response.Data.SyncOfficial)
	assert.Equal(t, "keep", response.Data.Description)
	assert.Equal(t, []string{"text", "image"}, response.Data.InputModalities)
	assert.Equal(t, []string{"text"}, response.Data.OutputModalities)
	assert.Equal(t, []string{"tools"}, response.Data.SupportedParameters)
	assert.EqualValues(t, 128000, response.Data.ContextLength)
	assert.EqualValues(t, 8192, response.Data.MaxOutputTokens)
}
