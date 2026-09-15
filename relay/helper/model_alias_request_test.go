package helper

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAndValidateRequestUsesCanonicalModelAcrossBodyFormats(t *testing.T) {
	const alias = "OpenAI/Legacy-Model"
	const canonical = "canonical-model"
	tests := []struct {
		name       string
		path       string
		body       string
		format     types.RelayFormat
		modelName  func(dto.Request) string
		additional func(*testing.T, dto.Request)
	}{
		{
			name: "OpenAI chat", path: "/v1/chat/completions",
			body:      `{"model":"OpenAI/Legacy-Model","messages":[{"role":"user","content":"hi"}]}`,
			format:    types.RelayFormatOpenAI,
			modelName: func(request dto.Request) string { return request.(*dto.GeneralOpenAIRequest).Model },
		},
		{
			name: "Responses", path: "/v1/responses",
			body:      `{"model":"OpenAI/Legacy-Model","input":"hi"}`,
			format:    types.RelayFormatOpenAIResponses,
			modelName: func(request dto.Request) string { return request.(*dto.OpenAIResponsesRequest).Model },
		},
		{
			name: "Anthropic", path: "/v1/messages",
			body:      `{"model":"OpenAI/Legacy-Model","messages":[{"role":"user","content":"hi"}]}`,
			format:    types.RelayFormatClaude,
			modelName: func(request dto.Request) string { return request.(*dto.ClaudeRequest).Model },
		},
		{
			name: "image", path: "/v1/images/generations",
			body:      `{"model":"OpenAI/Legacy-Model","prompt":"hi"}`,
			format:    types.RelayFormatOpenAIImage,
			modelName: func(request dto.Request) string { return request.(*dto.ImageRequest).Model },
		},
		{
			name: "audio", path: "/v1/audio/speech",
			body:      `{"model":"OpenAI/Legacy-Model","input":"hi"}`,
			format:    types.RelayFormatOpenAIAudio,
			modelName: func(request dto.Request) string { return request.(*dto.AudioRequest).Model },
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			c.Request.Header.Set("Content-Type", "application/json")
			common.SetContextKey(c, constant.ContextKeyOriginalModel, canonical)
			common.SetContextKey(c, constant.ContextKeyRequestedModel, alias)

			request, err := GetAndValidateRequest(c, test.format)
			require.NoError(t, err)
			assert.Equal(t, canonical, test.modelName(request))
			if test.additional != nil {
				test.additional(t, request)
			}
		})
	}
}

func TestImageAliasUsesCanonicalModelDefaultsBeforeRelayMapping(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{"model":"OpenAI/GPT-Image-1","prompt":"hi"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	common.SetContextKey(c, constant.ContextKeyOriginalModel, "gpt-image-1")
	common.SetContextKey(c, constant.ContextKeyRequestedModel, "OpenAI/GPT-Image-1")

	request, err := GetAndValidateRequest(c, types.RelayFormatOpenAIImage)
	require.NoError(t, err)
	imageRequest := request.(*dto.ImageRequest)
	assert.Equal(t, "gpt-image-1", imageRequest.Model)
	assert.Equal(t, "auto", imageRequest.Quality)
}
