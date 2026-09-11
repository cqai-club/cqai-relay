package types

// ModelCategory identifies a user-visible capability category for a model.
type ModelCategory string

const (
	ModelCategoryImage          ModelCategory = "image"
	ModelCategoryVideo          ModelCategory = "video"
	ModelCategoryText           ModelCategory = "text"
	ModelCategoryTextMultimodal ModelCategory = "text-multimodal"
	ModelCategoryAudio          ModelCategory = "audio"
	ModelCategoryOther          ModelCategory = "other"
)

// IsModelCategory reports whether value is a supported model capability category.
func IsModelCategory(value ModelCategory) bool {
	switch value {
	case ModelCategoryImage,
		ModelCategoryVideo,
		ModelCategoryText,
		ModelCategoryTextMultimodal,
		ModelCategoryAudio,
		ModelCategoryOther:
		return true
	default:
		return false
	}
}
