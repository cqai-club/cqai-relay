package dto

import "github.com/QuantumNous/new-api/relaykit/types"

// 这里不好动就不动了，本来想独立出来的（
type OpenAIModels struct {
	Id                     string                   `json:"id"`
	CanonicalSlug          string                   `json:"canonical_slug"`
	Name                   string                   `json:"name"`
	Object                 string                   `json:"object"`
	Created                int                      `json:"created"`
	OwnedBy                string                   `json:"owned_by"`
	Vendor                 string                   `json:"vendor,omitempty"`
	Description            string                   `json:"description,omitempty"`
	Icon                   string                   `json:"icon,omitempty"`
	Categories             []types.ModelCategory    `json:"categories,omitempty"`
	Architecture           *types.ModelArchitecture `json:"architecture"`
	SupportedParameters    []string                 `json:"supported_parameters"`
	ContextLength          *int64                   `json:"context_length"`
	MaxOutputTokens        *int64                   `json:"max_output_tokens"`
	SupportedEndpointTypes []types.EndpointType     `json:"supported_endpoint_types"`
	CanonicalId            string                   `json:"canonical_id,omitempty"`
	Deprecated             bool                     `json:"deprecated,omitempty"`
	RetireAfter            int64                    `json:"retire_after,omitempty"`
}

type AnthropicModel struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type GeminiModel struct {
	Name                       interface{}   `json:"name"`
	BaseModelId                interface{}   `json:"baseModelId"`
	Version                    interface{}   `json:"version"`
	DisplayName                interface{}   `json:"displayName"`
	Description                interface{}   `json:"description"`
	InputTokenLimit            interface{}   `json:"inputTokenLimit"`
	OutputTokenLimit           interface{}   `json:"outputTokenLimit"`
	SupportedGenerationMethods []interface{} `json:"supportedGenerationMethods"`
	Thinking                   interface{}   `json:"thinking"`
	Temperature                interface{}   `json:"temperature"`
	MaxTemperature             interface{}   `json:"maxTemperature"`
	TopP                       interface{}   `json:"topP"`
	TopK                       interface{}   `json:"topK"`
}
