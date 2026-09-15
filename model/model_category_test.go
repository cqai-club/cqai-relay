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

func TestDeriveModelCategoriesFromStructuredModalities(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		output    []string
		endpoints []types.EndpointType
		expected  []types.ModelCategory
	}{
		{name: "text", input: []string{"text"}, output: []string{"text"}, expected: []types.ModelCategory{types.ModelCategoryText}},
		{name: "vision text", input: []string{"text", "image"}, output: []string{"text"}, expected: []types.ModelCategory{types.ModelCategoryTextMultimodal}},
		{name: "image generation", input: []string{"text"}, output: []string{"image"}, expected: []types.ModelCategory{types.ModelCategoryImage}},
		{name: "video generation", input: []string{"text"}, output: []string{"video"}, expected: []types.ModelCategory{types.ModelCategoryVideo}},
		{name: "audio and text", input: []string{"text", "audio"}, output: []string{"text", "audio"}, expected: []types.ModelCategory{types.ModelCategoryAudio, types.ModelCategoryTextMultimodal}},
		{name: "embeddings modality", input: []string{"text"}, output: []string{"embeddings"}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "embedding endpoint overrides misleading text modalities", input: []string{"text"}, output: []string{"text"}, endpoints: []types.EndpointType{types.EndpointTypeEmbeddings}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "generative endpoint wins when both endpoint kinds are exposed", input: []string{"text"}, output: []string{"text"}, endpoints: []types.EndpointType{types.EndpointTypeOpenAI, types.EndpointTypeEmbeddings}, expected: []types.ModelCategory{types.ModelCategoryText}},
		{name: "unknown", expected: []types.ModelCategory{types.ModelCategoryOther}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, DeriveModelCategories(test.input, test.output, test.endpoints))
		})
	}
}

func TestModelArchitecturePreservesUnknownModalities(t *testing.T) {
	architecture := ModelArchitecture([]string{" Text ", "DEPTH", "depth"}, []string{"TEXT"})
	assert.Equal(t, &types.ModelArchitecture{
		Modality:         "text+depth->text",
		InputModalities:  []string{"text", "depth"},
		OutputModalities: []string{"text"},
	}, architecture)
}

func TestNormalizeStructuredMetadataPreservesConfirmedSpecializedCategory(t *testing.T) {
	item := Model{
		InputModalities:  []string{"text"},
		OutputModalities: []string{"text"},
		Capabilities:     []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus:   ModelMetadataStatusConfirmed,
	}

	item.NormalizeStructuredMetadata()

	assert.Equal(t, []string{"text"}, item.InputModalities)
	assert.Equal(t, []string{"text"}, item.OutputModalities)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, item.Capabilities)
}
