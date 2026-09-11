package model

import (
	"fmt"

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
