package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// NormalizeModelCategories removes unsupported and duplicate capability values
// while preserving the public category order.
func NormalizeModelCategories(values []types.ModelCategory) []types.ModelCategory {
	seen := make(map[types.ModelCategory]struct{}, len(values))
	normalized := make([]types.ModelCategory, 0, len(values))
	for _, value := range values {
		if !types.IsModelCategory(value) {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

// ValidateModelCategories rejects unsupported capability values in model
// metadata submitted by an administrator.
func ValidateModelCategories(values []types.ModelCategory) error {
	for _, value := range values {
		if !types.IsModelCategory(value) {
			return fmt.Errorf("unsupported model capability category %q", value)
		}
	}
	return nil
}

// CatalogModelCategories supplies the public fallback for a model without
// explicit capability metadata.
func CatalogModelCategories(values []types.ModelCategory) []types.ModelCategory {
	normalized := NormalizeModelCategories(values)
	if len(normalized) == 0 {
		return []types.ModelCategory{types.ModelCategoryOther}
	}
	return normalized
}

// NormalizeModelCatalogValues normalizes case-insensitive OpenRouter catalog
// values while retaining unknown future values for forward compatibility.
func NormalizeModelCatalogValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func modelCatalogValueSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range NormalizeModelCatalogValues(values) {
		result[value] = struct{}{}
	}
	return result
}

// DeriveModelCategories derives the legacy coarse category list from the
// authoritative input/output modalities. Endpoint types are consulted only
// when no structured output modality is available.
func DeriveModelCategories(inputModalities []string, outputModalities []string, endpointTypes []types.EndpointType) []types.ModelCategory {
	inputSet := modelCatalogValueSet(inputModalities)
	outputSet := modelCatalogValueSet(outputModalities)
	endpointSet := make(map[types.EndpointType]struct{}, len(endpointTypes))
	for _, endpoint := range endpointTypes {
		endpointSet[endpoint] = struct{}{}
	}

	hasSpecializedEndpoint := false
	for _, endpoint := range []types.EndpointType{
		types.EndpointTypeEmbeddings,
		types.EndpointTypeJinaRerank,
		types.EndpointTypeOpenAIAlphaSearch,
	} {
		_, hasSpecializedEndpoint = endpointSet[endpoint]
		if hasSpecializedEndpoint {
			break
		}
	}
	hasGenerativeEndpoint := false
	for _, endpoint := range []types.EndpointType{
		types.EndpointTypeOpenAI,
		types.EndpointTypeOpenAIResponse,
		types.EndpointTypeOpenAIResponseCompact,
		types.EndpointTypeAnthropic,
		types.EndpointTypeGemini,
		types.EndpointTypeImageGeneration,
		types.EndpointTypeOpenAIVideo,
	} {
		_, hasGenerativeEndpoint = endpointSet[endpoint]
		if hasGenerativeEndpoint {
			break
		}
	}
	if hasSpecializedEndpoint && !hasGenerativeEndpoint {
		return []types.ModelCategory{types.ModelCategoryOther}
	}

	result := make([]types.ModelCategory, 0, 4)
	if _, exists := outputSet["image"]; exists {
		result = append(result, types.ModelCategoryImage)
	}
	if _, exists := outputSet["video"]; exists {
		result = append(result, types.ModelCategoryVideo)
	}
	for _, audioModality := range []string{"audio", "speech", "transcription"} {
		_, inputAudio := inputSet[audioModality]
		_, outputAudio := outputSet[audioModality]
		if inputAudio || outputAudio {
			result = append(result, types.ModelCategoryAudio)
			break
		}
	}
	if _, textOutput := outputSet["text"]; textOutput {
		multimodal := false
		for input := range inputSet {
			if input != "text" {
				multimodal = true
				break
			}
		}
		if multimodal {
			result = append(result, types.ModelCategoryTextMultimodal)
		} else {
			result = append(result, types.ModelCategoryText)
		}
	}

	if len(result) == 0 {
		return []types.ModelCategory{types.ModelCategoryOther}
	}
	return NormalizeDerivedModelCategories(result)
}

// NormalizeDerivedModelCategories enforces the public category invariants.
func NormalizeDerivedModelCategories(values []types.ModelCategory) []types.ModelCategory {
	values = NormalizeModelCategories(values)
	hasMultimodal := false
	hasSpecific := false
	for _, value := range values {
		hasMultimodal = hasMultimodal || value == types.ModelCategoryTextMultimodal
		hasSpecific = hasSpecific || value != types.ModelCategoryOther
	}
	result := make([]types.ModelCategory, 0, len(values))
	for _, value := range values {
		if hasMultimodal && value == types.ModelCategoryText {
			continue
		}
		if hasSpecific && value == types.ModelCategoryOther {
			continue
		}
		result = append(result, value)
	}
	return result
}

// ModelArchitecture builds the public OpenRouter-style representation.
func ModelArchitecture(inputModalities []string, outputModalities []string) *types.ModelArchitecture {
	input := NormalizeModelCatalogValues(inputModalities)
	output := NormalizeModelCatalogValues(outputModalities)
	if len(input) == 0 && len(output) == 0 {
		return nil
	}
	return &types.ModelArchitecture{
		Modality:         strings.Join(input, "+") + "->" + strings.Join(output, "+"),
		InputModalities:  input,
		OutputModalities: output,
	}
}

// NormalizeStructuredMetadata normalizes structured catalog fields and keeps
// the legacy capability cache derived from modalities whenever they exist.
func (mi *Model) NormalizeStructuredMetadata() {
	normalizedCategories := NormalizeModelCategories(mi.Capabilities)
	confirmedOther := mi.MetadataStatus == ModelMetadataStatusConfirmed &&
		len(normalizedCategories) == 1 &&
		normalizedCategories[0] == types.ModelCategoryOther
	mi.InputModalities = NormalizeModelCatalogValues(mi.InputModalities)
	mi.OutputModalities = NormalizeModelCatalogValues(mi.OutputModalities)
	mi.SupportedParameters = NormalizeModelCatalogValues(mi.SupportedParameters)
	sort.Strings(mi.SupportedParameters)
	if mi.ContextLength < 0 {
		mi.ContextLength = 0
	}
	if mi.MaxOutputTokens < 0 {
		mi.MaxOutputTokens = 0
	}
	if len(mi.InputModalities) > 0 || len(mi.OutputModalities) > 0 {
		if confirmedOther {
			// A confirmed `other` value is also the durable compatibility marker
			// for embeddings, rerank, and search models whose source modalities
			// may otherwise look like ordinary text input/output.
			mi.Capabilities = []types.ModelCategory{types.ModelCategoryOther}
		} else {
			mi.Capabilities = DeriveModelCategories(mi.InputModalities, mi.OutputModalities, mi.EndpointTypes())
		}
	} else {
		mi.Capabilities = NormalizeDerivedModelCategories(mi.Capabilities)
	}
}

func (mi Model) HasStructuredMetadata() bool {
	return len(mi.InputModalities) > 0 || len(mi.OutputModalities) > 0 ||
		len(mi.SupportedParameters) > 0 || mi.ContextLength > 0 || mi.MaxOutputTokens > 0
}
