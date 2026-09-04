package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
)

func TestImageGenerationModelDetection(t *testing.T) {
	for _, model := range []string{"gpt-image-1", "gpt-image-1.5", "gpt-image-2", "GPT-IMAGE-2"} {
		assert.Truef(t, IsImageGenerationModel(model), "model %q should be detected as an image generation model", model)
	}

	assert.Equal(t,
		constant.EndpointTypeImageGeneration,
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "gpt-image-2")[0],
	)
}
