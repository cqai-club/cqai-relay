package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayhelper "github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestTokenAllowsCanonicalAndActiveAliasNames(t *testing.T) {
	previousDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	t.Cleanup(func() {
		model.DB = previousDB
		require.NoError(t, model.InitModelAliasCache())
	})
	require.NoError(t, db.AutoMigrate(&model.Model{}, &model.ModelAlias{}))
	require.NoError(t, db.Create(&model.Model{
		ModelName: "gpt-5", MetadataStatus: model.ModelMetadataStatusConfirmed,
		MetadataSource: model.ModelMetadataSourceManual, Status: 1, SyncOfficial: 1,
	}).Error)
	require.NoError(t, model.CreateOrActivateModelAlias(db, "OpenAI/GPT-5", "gpt-5", model.ModelMetadataSourceBaseLLMNormalized, common.GetTimestamp()+3600))
	require.NoError(t, model.InitModelAliasCache())

	assert.True(t, tokenAllowsModel(map[string]bool{"gpt-5": true}, "gpt-5", "OpenAI/GPT-5"))
	assert.True(t, tokenAllowsModel(map[string]bool{"OpenAI/GPT-5": true}, "gpt-5", "OpenAI/GPT-5"))
	assert.True(t, tokenAllowsModel(map[string]bool{"OpenAI/GPT-5": true}, "gpt-5", "gpt-5"))
	assert.False(t, tokenAllowsModel(map[string]bool{"other": true}, "gpt-5", "OpenAI/GPT-5"))
}

func TestDistributeResolvesAliasBeforeRoutingAndMapsCanonicalToUpstream(t *testing.T) {
	setupOriginTaskDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Model{}, &model.ModelAlias{}))
	require.NoError(t, model.DB.Create(&model.Model{
		ModelName: "gpt-5", MetadataStatus: model.ModelMetadataStatusConfirmed,
		MetadataSource: model.ModelMetadataSourceBaseLLMExact, Status: 1, SyncOfficial: 1,
	}).Error)
	require.NoError(t, model.CreateOrActivateModelAlias(model.DB, "OpenAI/GPT-5", "gpt-5", model.ModelMetadataSourceBaseLLMNormalized, common.GetTimestamp()+3600))
	require.NoError(t, model.InitModelAliasCache())

	mapping := `{"gpt-5":"commandcode-gpt-5"}`
	channel := &model.Channel{
		Name: "canonical-channel", Key: "sk-test", Status: common.ChannelStatusEnabled,
		Type: constant.ChannelTypeOpenAI, Models: "gpt-5", Group: "default", ModelMapping: &mapping,
	}
	require.NoError(t, model.DB.Create(channel).Error)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"OpenAI/GPT-5"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("resolved_task_model", "OpenAI/GPT-5")
	service.GetChannelConstraints(c).AddPin(taskdto.ChannelPin{
		ChannelId: channel.Id, Source: taskdto.PinSourceOriginTask,
		Rank: taskdto.PinRankOriginTask, RetryMode: taskdto.PinRetrySameChannel,
	})

	Distribute()(c)
	require.False(t, c.IsAborted())
	assert.Equal(t, "gpt-5", common.GetContextKeyString(c, constant.ContextKeyOriginalModel))
	assert.Equal(t, "OpenAI/GPT-5", common.GetContextKeyString(c, constant.ContextKeyRequestedModel))
	aliases := model.GetActiveModelAliases()
	require.Len(t, aliases, 1)
	assert.EqualValues(t, 1, aliases[0].RequestCount)

	info := &relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5"},
	}
	require.NoError(t, relayhelper.ModelMappedHelper(c, info, nil))
	assert.Equal(t, "commandcode-gpt-5", info.UpstreamModelName)
}

func TestGetModelRequestPreservesAliasAcrossRelayEntrypoints(t *testing.T) {
	const alias = "OpenAI/GPT-5"
	tests := []struct {
		name string
		path string
		body string
	}{
		{name: "openai chat", path: "/v1/chat/completions", body: `{"model":"OpenAI/GPT-5"}`},
		{name: "responses", path: "/v1/responses", body: `{"model":"OpenAI/GPT-5"}`},
		{name: "anthropic", path: "/v1/messages", body: `{"model":"OpenAI/GPT-5"}`},
		{name: "gemini", path: "/v1beta/models/OpenAI/GPT-5:generateContent", body: `{}`},
		{name: "image", path: "/v1/images/generations", body: `{"model":"OpenAI/GPT-5"}`},
		{name: "audio", path: "/v1/audio/speech", body: `{"model":"OpenAI/GPT-5"}`},
		{name: "video", path: "/v1/videos", body: `{"model":"OpenAI/GPT-5"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body))
			c.Request.Header.Set("Content-Type", "application/json")

			request, shouldSelectChannel, err := getModelRequest(c)
			require.NoError(t, err)
			assert.True(t, shouldSelectChannel)
			assert.Equal(t, alias, request.Model)
		})
	}
}

func TestChannelMatchesExpectedTaskPluginUsesGenericChannelSetting(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeTaskPlugin}
	channel.SetSetting(dto.ChannelSettings{TaskPluginKey: "generic-alpha"})

	assert.True(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, "generic-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, channel, ""))
}

func TestChannelMatchesExpectedTaskPluginUsesPinnedLegacyIndex(t *testing.T) {
	registry := jsplugin.NewRegistry()
	alpha, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)
	pinnedGeneration := registry.Generation()

	require.NoError(t, registry.Unregister("legacy-alpha"))
	_, err = registry.Register(distributorTaskPluginSource("legacy-beta", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: pinnedGeneration,
		Plugin:     alpha,
	})
	channel := &model.Channel{Type: constant.ChannelTypeKling}

	assert.True(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, channel, "legacy-beta"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
}

func TestChannelMatchesExpectedTaskPluginRejectsUnindexedLegacyChannel(t *testing.T) {
	registry := jsplugin.NewRegistry()
	plugin, err := registry.Register(distributorTaskPluginSource("legacy-alpha", constant.ChannelTypeKling), jsplugin.Options{})
	require.NoError(t, err)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{
		Generation: registry.Generation(),
		Plugin:     plugin,
	})

	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "legacy-alpha"))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: 0}, "legacy-alpha"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeJimeng}, ""))
	assert.False(t, channelMatchesExpectedTaskPlugin(nil, &model.Channel{Type: constant.ChannelTypeKling}, "legacy-alpha"))

	c.Set("expected_task_plugin_key", "legacy-alpha")
	setupErr := SetupContextForSelectedChannel(c, &model.Channel{Type: constant.ChannelTypeJimeng}, "task-model")
	require.NotNil(t, setupErr)
	assert.Contains(t, setupErr.Error(), "does not match")
}

func TestSharedEndpointRebindsToSelectedLegacyProvider(t *testing.T) {
	registry := jsplugin.NewRegistry()
	_, err := registry.Register(distributorEndpointPluginSource("gemini-shared", constant.ChannelTypeGemini), jsplugin.Options{})
	require.NoError(t, err)
	_, err = registry.Register(distributorEndpointPluginSource("vertex-shared", constant.ChannelTypeVertexAi), jsplugin.Options{})
	require.NoError(t, err)
	candidates := registry.Generation().LookupEndpointCandidates("POST", "/v1/responses", "task-model")
	require.Len(t, candidates, 2)

	c, _ := gin.CreateTestContext(nil)
	c.Set(jsplugin.ContextKeyPinnedPlugin, jsplugin.PinnedPlugin{Generation: registry.Generation(), Plugin: candidates[0].Plugin})
	c.Set(jsplugin.ContextKeyPinnedEndpoint, jsplugin.PinnedEndpoint{
		Generation: registry.Generation(),
		Plugin:     candidates[0].Plugin,
		Protocol:   candidates[0].Protocol,
		Operation:  candidates[0].Operation,
		Model:      "task-model",
		Candidates: candidates,
	})
	c.Set("expected_task_plugin_key", candidates[0].Plugin.Meta.Key)

	geminiChannel := &model.Channel{Id: 1, Type: constant.ChannelTypeGemini}
	vertexChannel := &model.Channel{Id: 2, Type: constant.ChannelTypeVertexAi}
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, candidates[0].Plugin.Meta.Key))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, vertexChannel, candidates[0].Plugin.Meta.Key))
	assert.False(t, channelMatchesExpectedTaskPlugin(c, &model.Channel{Type: constant.ChannelTypeKling}, candidates[0].Plugin.Meta.Key))

	require.Nil(t, SetupContextForSelectedChannel(c, vertexChannel, "task-model"))
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	require.True(t, exists)
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	require.True(t, ok)
	assert.Equal(t, "vertex-shared", pinned.Plugin.Meta.Key)
	assert.Equal(t, "vertex-shared", c.GetString("expected_task_plugin_key"))
	assert.Equal(t, "vertex-shared", c.GetString("task_plugin_key"))
	assert.True(t, channelMatchesExpectedTaskPlugin(c, geminiChannel, "vertex-shared"), "a retry may select another declared provider")
}

func distributorTaskPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
`, key, key, channelType)
}

func distributorEndpointPluginSource(key string, channelType int) string {
	return fmt.Sprintf(`
export const meta = {
  apiVersion: 1,
  key: %q,
  name: %q,
  version: "1.0.0",
  author: {name: "Test"},
  channelTypes: [%d],
  models: ["task-model"],
  fetchMode: "per_task",
  protocols: [{name: "openai_responses", supports: ["stream", "sync", "background"]}],
};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId: "task"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status: "SUCCESS"}; }
export const protocols = {openai_responses: {
  decodeRequest: function(ctx) { return {kind: "submit", model: "task-model", requestBody: ctx.body.value}; },
  renderEvents: function() { return {events: [], state: null, done: false}; },
  renderFinal: function() { return {output: []}; },
}};
`, key, key, channelType)
}
