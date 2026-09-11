package model

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func TestModelCategoryNormalizationPreservesSupportedOrderAndFallback(t *testing.T) {
	values := []types.ModelCategory{
		types.ModelCategoryImage,
		"unsupported",
		types.ModelCategoryTextMultimodal,
		types.ModelCategoryImage,
	}

	assert.Equal(t, []types.ModelCategory{
		types.ModelCategoryImage,
		types.ModelCategoryTextMultimodal,
	}, NormalizeModelCategories(values))
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, CatalogModelCategories(nil))
}

func TestValidateModelCategoriesRejectsUnsupportedValues(t *testing.T) {
	assert.NoError(t, ValidateModelCategories([]types.ModelCategory{types.ModelCategoryAudio}))
	assert.Error(t, ValidateModelCategories([]types.ModelCategory{"unsupported"}))
}
