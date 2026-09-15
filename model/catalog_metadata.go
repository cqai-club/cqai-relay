package model

import (
	"sync"

	"github.com/QuantumNous/new-api/relaykit/types"
)

// ModelCatalogMetadata contains non-sensitive metadata exposed with a user
// visible model list.
type ModelCatalogMetadata struct {
	Description         string
	Icon                string
	Vendor              string
	Categories          []types.ModelCategory
	Architecture        *types.ModelArchitecture
	SupportedParameters []string
	ContextLength       int64
	MaxOutputTokens     int64
}

var (
	modelCatalogMetadataMap  = make(map[string]ModelCatalogMetadata)
	modelCatalogMetadataLock sync.RWMutex
)

// GetModelCatalogMetadata returns the metadata for one model. The lookup uses
// the same exact and name-rule resolution as the pricing cache.
func GetModelCatalogMetadata(modelName string) ModelCatalogMetadata {
	GetPricing()

	modelCatalogMetadataLock.RLock()
	metadata, ok := modelCatalogMetadataMap[modelName]
	modelCatalogMetadataLock.RUnlock()
	if !ok {
		return ModelCatalogMetadata{Categories: CatalogModelCategories(nil)}
	}
	metadata.Categories = append([]types.ModelCategory(nil), metadata.Categories...)
	metadata.Architecture = types.CloneModelArchitecture(metadata.Architecture)
	metadata.SupportedParameters = append([]string(nil), metadata.SupportedParameters...)
	return metadata
}

func replaceModelCatalogMetadata(metadata map[string]ModelCatalogMetadata) {
	modelCatalogMetadataLock.Lock()
	modelCatalogMetadataMap = metadata
	modelCatalogMetadataLock.Unlock()
}

func clearModelCatalogMetadata() {
	replaceModelCatalogMetadata(make(map[string]ModelCatalogMetadata))
}
