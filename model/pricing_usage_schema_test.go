package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pricingUsagePluginSource(version, usageSchema string) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1, key: "pricing-usage-probe", name: "Pricing Usage Probe", version: %q, author: {name: "Test"},
  models: ["pricing-usage-model"], fetchMode: "per_task", usageSchema: %s
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
`, version, usageSchema)
}

func TestPricingCarriesTaskUsageSchemaAndRefreshesWithPluginGeneration(t *testing.T) {
	resetPricingEndpointTestTables(t)
	const pluginKey = "pricing-usage-probe"
	initialSource := pricingUsagePluginSource("1.0.0", `{
  seconds: {type: "number", unit: "second", description: "Estimated duration."}
}`)
	_, err := jsplugin.DefaultRegistry.Register(initialSource, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry.Unregister(pluginKey) })

	insertPricingEndpointChannel(t, 901, constant.ChannelTypeTaskPlugin, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 901, "pricing-usage-model")
	insertPricingEndpointAbility(t, 901, "ordinary-model")

	initialPricing := pricingByModel(GetPricing())
	require.Contains(t, initialPricing, "pricing-usage-model")
	require.Contains(t, initialPricing, "ordinary-model")
	assert.Equal(t, "second", initialPricing["pricing-usage-model"].BillingUsageSchema["seconds"].Unit)
	assert.Equal(t, "Estimated duration.", initialPricing["pricing-usage-model"].BillingUsageSchema["seconds"].Description["en"])
	assert.Nil(t, initialPricing["ordinary-model"].BillingUsageSchema)

	updatedSource := pricingUsagePluginSource("1.1.0", `{
  seconds: {type: "number", unit: "second", description: "Measured duration."},
  clips: {type: "number", unit: "count", description: "Generated clip count."}
}`)
	_, err = jsplugin.DefaultRegistry.Register(updatedSource, jsplugin.Options{})
	require.NoError(t, err)
	lastGetPricingTime = time.Now().Add(-2 * time.Minute)

	refreshedPricing := pricingByModel(GetPricing())
	require.Len(t, refreshedPricing["pricing-usage-model"].BillingUsageSchema, 2)
	assert.Equal(t, "Measured duration.", refreshedPricing["pricing-usage-model"].BillingUsageSchema["seconds"].Description["en"])
	assert.Equal(t, "count", refreshedPricing["pricing-usage-model"].BillingUsageSchema["clips"].Unit)
}

func pricingVideoPluginSource() string {
	return `
export const meta = {
  apiVersion: 1,
  key: "pricing-video-probe",
  name: "Pricing Video Probe",
  icon: "Video.Color",
  description: {en: "Task plugin video generation.", zh: "任务插件视频生成。"},
  version: "1.0.0",
  author: {name: "Test"},
  models: ["pricing-video-model"],
  fetchMode: "per_task",
  protocols: [
    {name: "openai_responses", supports: ["stream", "sync", "background"]},
    "openai_video"
  ]
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {}; }
export function listArtifacts() { return []; }
export function buildContentRequest() { return {}; }
export const protocols = {
  openai_responses: {
    decodeRequest: function(ctx) { return ctx; },
    renderEvents: function() { return {events: [], state: null, done: true}; },
    renderFinal: function(ctx, task) { return task; }
  },
  openai_video: {
    decodeRequest: function(ctx) { return ctx; },
    render: function(ctx, task) { return task; }
  }
};
`
}

func TestPricingDerivesPendingTaskPluginCatalogFromProtocols(t *testing.T) {
	resetPricingEndpointTestTables(t)
	const pluginKey = "pricing-video-probe"
	_, err := jsplugin.DefaultRegistry.Register(pricingVideoPluginSource(), jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry.Unregister(pluginKey) })

	insertPricingEndpointChannel(t, 902, constant.ChannelTypeTaskPlugin, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, 902, "pricing-video-model")
	require.NoError(t, DB.Create(&Model{
		ModelName:      "pricing-video-model",
		Capabilities:   []relaytypes.ModelCategory{relaytypes.ModelCategoryOther},
		MetadataStatus: ModelMetadataStatusPending,
		MetadataSource: ModelMetadataSourceChannel,
		Status:         1,
		SyncOfficial:   1,
	}).Error)

	pricing := pricingByModel(GetPricing())["pricing-video-model"]
	assert.Equal(t, "Task plugin video generation.", pricing.Description)
	assert.Equal(t, "Video.Color", pricing.Icon)
	assert.Equal(t, []constant.EndpointType{
		constant.EndpointTypeOpenAIResponse,
		constant.EndpointTypeOpenAIVideo,
	}, pricing.SupportedEndpointTypes)
	assert.Equal(t, []relaytypes.ModelCategory{relaytypes.ModelCategoryVideo}, pricing.Categories)
	require.NotNil(t, pricing.Architecture)
	assert.Empty(t, pricing.Architecture.InputModalities)
	assert.Equal(t, []string{"video"}, pricing.Architecture.OutputModalities)

	require.NoError(t, DB.Model(&Model{}).Where("model_name = ?", "pricing-video-model").Updates(map[string]any{
		"metadata_status": ModelMetadataStatusConfirmed,
		"metadata_source": ModelMetadataSourceManual,
	}).Error)
	InvalidatePricingCache()

	manual := pricingByModel(GetPricing())["pricing-video-model"]
	assert.Empty(t, manual.Description)
	assert.Empty(t, manual.Icon)
	assert.Equal(t, []relaytypes.ModelCategory{relaytypes.ModelCategoryOther}, manual.Categories)
	assert.Nil(t, manual.Architecture)
}

func pricingByModel(pricings []Pricing) map[string]Pricing {
	result := make(map[string]Pricing, len(pricings))
	for _, pricing := range pricings {
		result[pricing.ModelName] = pricing
	}
	return result
}
