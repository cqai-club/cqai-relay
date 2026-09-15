package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupModelReconcileTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	previousDB := model.DB
	previousMemoryCache := common.MemoryCacheEnabled
	previousSelfUse := operation_setting.SelfUseModeEnabled
	common.MemoryCacheEnabled = false
	operation_setting.SelfUseModeEnabled = true
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.Model{}, &model.ModelAlias{}, &model.Vendor{}, &model.Token{}, &model.Option{}))
	require.NoError(t, model.InitModelAliasCache())
	t.Cleanup(func() {
		model.DB = previousDB
		common.MemoryCacheEnabled = previousMemoryCache
		operation_setting.SelfUseModeEnabled = previousSelfUse
		require.NoError(t, model.InitModelAliasCache())
	})
	return db
}

func TestCatalogMatchingUsesOnlyExactOrUniqueNormalization(t *testing.T) {
	catalog := []upstreamModel{
		{ModelName: "gpt-5"},
		{ModelName: "vendor/shared"},
		{ModelName: "other/shared"},
	}
	exact, normalized := buildCatalogIndexes(catalog)

	matched, matchType, ok, ambiguous := matchCatalogModel("gpt-5", exact, normalized)
	require.True(t, ok)
	assert.False(t, ambiguous)
	assert.Equal(t, "exact", matchType)
	assert.Equal(t, "gpt-5", matched.ModelName)

	matched, matchType, ok, ambiguous = matchCatalogModel("OpenAI/GPT-5", exact, normalized)
	require.True(t, ok)
	assert.False(t, ambiguous)
	assert.Equal(t, "normalized", matchType)
	assert.Equal(t, "gpt-5", matched.ModelName)

	matched, matchType, ok, ambiguous = matchCatalogModel(" Open AI / G P T - 5 ", exact, normalized)
	require.True(t, ok)
	assert.False(t, ambiguous)
	assert.Equal(t, "normalized", matchType)
	assert.Equal(t, "gpt-5", matched.ModelName)

	_, _, ok, ambiguous = matchCatalogModel("SHARED", exact, normalized)
	assert.False(t, ok)
	assert.True(t, ambiguous)

	_, _, ok, ambiguous = matchCatalogModel("gpt-5-typo", exact, normalized)
	assert.False(t, ok)
	assert.False(t, ambiguous)
}

func TestBaseLLMCatalogAdapterReusesETagCache(t *testing.T) {
	var requests atomic.Int32
	var conditionalRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Header.Get("If-None-Match") == `"catalog-v1"` {
			conditionalRequests.Add(1)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", `"catalog-v1"`)
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/models.json") {
			_, _ = w.Write([]byte(`[{"model_name":"gpt-5"}]`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/all.json") {
			_, _ = w.Write([]byte(`{"openai":{"id":"openai","name":"OpenAI","models":{"gpt-5":{"id":"gpt-5","modalities":{"input":["text"],"output":["text"]}}}}}`))
			return
		}
		_, _ = w.Write([]byte(`[{"name":"OpenAI"}]`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	adapter := baseLLMCatalogAdapter{}
	first, err := adapter.Fetch(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, first.Models, 1)
	second, err := adapter.Fetch(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, second.Models, 1)
	assert.EqualValues(t, 6, requests.Load())
	assert.EqualValues(t, 3, conditionalRequests.Load())
}

func TestFetchJSONKeepsLastValidDocumentOnNetworkParseAndSemanticFailures(t *testing.T) {
	var failureMode atomic.Int32
	var seenIfNoneMatch atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenIfNoneMatch.Store(r.Header.Get("If-None-Match"))
		switch failureMode.Load() {
		case 1:
			w.Header().Set("ETag", `"empty-v2"`)
			_, _ = w.Write([]byte(`{}`))
			return
		case 2:
			w.Header().Set("ETag", `"rejected-v3"`)
			_, _ = w.Write([]byte(`{"success":false,"message":"unavailable"}`))
			return
		case 3:
			w.Header().Set("ETag", `"invalid-v2"`)
			_, _ = w.Write([]byte(`{"data":`))
			return
		}
		w.Header().Set("ETag", `"valid-v1"`)
		_, _ = w.Write([]byte(`[{"model_name":"cached-model"}]`))
	}))
	t.Setenv("SYNC_HTTP_RETRY", "1")

	var first upstreamEnvelope[upstreamModel]
	require.NoError(t, fetchJSON(context.Background(), server.URL, &first))
	require.Len(t, first.Data, 1)

	failureMode.Store(1)
	var afterEmpty upstreamEnvelope[upstreamModel]
	require.NoError(t, fetchJSON(context.Background(), server.URL, &afterEmpty))
	require.Len(t, afterEmpty.Data, 1)
	assert.Equal(t, "cached-model", afterEmpty.Data[0].ModelName)
	assert.Equal(t, `"valid-v1"`, seenIfNoneMatch.Load())

	failureMode.Store(2)
	var afterRejected upstreamEnvelope[upstreamModel]
	require.NoError(t, fetchJSON(context.Background(), server.URL, &afterRejected))
	require.Len(t, afterRejected.Data, 1)
	assert.Equal(t, "cached-model", afterRejected.Data[0].ModelName)
	assert.Equal(t, `"valid-v1"`, seenIfNoneMatch.Load())

	failureMode.Store(3)
	var afterInvalid upstreamEnvelope[upstreamModel]
	require.NoError(t, fetchJSON(context.Background(), server.URL, &afterInvalid))
	require.Len(t, afterInvalid.Data, 1)
	assert.Equal(t, "cached-model", afterInvalid.Data[0].ModelName)
	assert.Equal(t, `"valid-v1"`, seenIfNoneMatch.Load())

	server.Close()
	var afterNetworkFailure upstreamEnvelope[upstreamModel]
	require.NoError(t, fetchJSON(context.Background(), server.URL, &afterNetworkFailure))
	require.Len(t, afterNetworkFailure.Data, 1)
	assert.Equal(t, "cached-model", afterNetworkFailure.Data[0].ModelName)
}

func TestBaseLLMCatalogKeepsLastValidStructuredSnapshotOnSemanticFailure(t *testing.T) {
	var invalidStructured atomic.Bool
	var lastStructuredIfNoneMatch atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`[{"model_name":"cached-vision","vendor_name":"Example"}]`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`[{"name":"Example"}]`))
		case "/api/all.json":
			lastStructuredIfNoneMatch.Store(r.Header.Get("If-None-Match"))
			if invalidStructured.Load() {
				w.Header().Set("ETag", `"invalid-v2"`)
				_, _ = w.Write([]byte(`{}`))
				return
			}
			w.Header().Set("ETag", `"valid-v1"`)
			_, _ = w.Write([]byte(`{"example":{"id":"example","name":"Example","models":{"cached-vision":{"id":"cached-vision","modalities":{"input":["text","image"],"output":["text"]}}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	adapter := baseLLMCatalogAdapter{}
	first, err := adapter.Fetch(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, first.Models, 1)
	assert.Equal(t, []string{"text", "image"}, first.Models[0].InputModalities)

	invalidStructured.Store(true)
	for range 2 {
		afterInvalid, err := adapter.Fetch(context.Background(), "")
		require.NoError(t, err)
		require.Len(t, afterInvalid.Models, 1)
		assert.Equal(t, []string{"text", "image"}, afterInvalid.Models[0].InputModalities)
		assert.Equal(t, `"valid-v1"`, lastStructuredIfNoneMatch.Load())
	}
}

func TestBaseLLMStructuredMetadataUsesExactVendorAndModelAndConservativeParameters(t *testing.T) {
	catalog := enrichBaseLLMStructuredMetadata([]upstreamModel{
		{ModelName: "vision-model", VendorName: "Example"},
		{ModelName: "text-embedding-3-small", VendorName: "OpenAI"},
	}, map[string]baseLLMStructuredProvider{
		"example": {
			ID: "example", Name: "Example",
			Models: map[string]baseLLMStructuredModel{
				"vision-model": {
					ID: "vision-model", Attachment: true,
					Modalities: baseLLMModalities{Input: []string{"text", "image"}, Output: []string{"text"}},
					Limit:      baseLLMLimit{Context: 128000, Output: 8192}, ToolCall: true, Reasoning: true,
					ReasoningOptions: []baseLLMReasoningOption{{Type: "toggle"}, {Type: "budget_tokens"}},
					StructuredOutput: true, Temperature: true,
				},
			},
		},
		"openai": {
			ID: "openai", Name: "OpenAI",
			Models: map[string]baseLLMStructuredModel{
				"text-embedding-3-small": {
					ID: "text-embedding-3-small", Family: "text-embedding",
					Modalities: baseLLMModalities{Input: []string{"text"}, Output: []string{"text"}},
				},
			},
		},
	})
	require.Len(t, catalog, 2)
	assert.True(t, catalog[0].StructuredMetadata)
	assert.Equal(t, []string{"text", "image", "file"}, catalog[0].InputModalities)
	assert.Equal(t, []string{"text"}, catalog[0].OutputModalities)
	assert.Equal(t, []string{"tools", "reasoning", "structured_outputs", "temperature"}, catalog[0].SupportedParameters)
	assert.NotContains(t, catalog[0].SupportedParameters, "reasoning_effort")
	assert.EqualValues(t, 128000, catalog[0].ContextLength)
	assert.EqualValues(t, 8192, catalog[0].MaxOutputTokens)
	assert.Equal(t, []string{"text"}, catalog[1].OutputModalities)
	assert.Equal(t, "embedding", catalog[1].SpecializedModelKind)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, inferUpstreamModelCategories(catalog[1]))

	var effort upstreamModel
	applyBaseLLMStructuredMetadata(&effort, baseLLMStructuredModel{
		Modalities:       baseLLMModalities{Input: []string{"text"}, Output: []string{"text"}},
		Reasoning:        true,
		ReasoningOptions: []baseLLMReasoningOption{{Type: "effort"}},
	})
	assert.Equal(t, []string{"reasoning", "reasoning_effort"}, effort.SupportedParameters)
}

func TestPreparePricingCopiesDeduplicatesSameMatchAcrossChannels(t *testing.T) {
	setupModelReconcileTestDB(t)
	originalRatios := ratio_setting.GetModelRatioCopy()
	t.Cleanup(func() {
		encoded, err := common.Marshal(originalRatios)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
	})
	ratios := ratio_setting.GetModelRatioCopy()
	ratios["Vendor/shared-price"] = 2.5
	delete(ratios, "shared-price")
	encoded, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))

	items := []modelReconcileMatch{
		{ChannelID: 1, UpstreamModel: "Vendor/shared-price", CanonicalModel: "shared-price"},
		{ChannelID: 2, UpstreamModel: "Vendor/shared-price", CanonicalModel: "shared-price"},
	}
	options, conflicts := preparePricingCopies(items)
	assert.Empty(t, conflicts)
	var copied map[string]float64
	require.NoError(t, common.UnmarshalJsonStr(options["ModelRatio"], &copied))
	assert.Equal(t, 2.5, copied["shared-price"])
}

func TestPreparePricingCopiesRejectsDifferentCanonicalPricing(t *testing.T) {
	setupModelReconcileTestDB(t)
	originalRatios := ratio_setting.GetModelRatioCopy()
	t.Cleanup(func() {
		encoded, err := common.Marshal(originalRatios)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
	})
	ratios := ratio_setting.GetModelRatioCopy()
	ratios["Vendor/conflicting-price"] = 2.5
	ratios["conflicting-price"] = 3
	encoded, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))

	options, conflicts := preparePricingCopies([]modelReconcileMatch{{
		ChannelID: 1, UpstreamModel: "Vendor/conflicting-price", CanonicalModel: "conflicting-price",
	}})

	assert.Nil(t, options)
	require.NotEmpty(t, conflicts)
	assert.Contains(t, strings.Join(conflicts, "\n"), "ModelRatio differs")
}

func TestSyncUpstreamModelsDoesNotOverwriteManualPendingMetadata(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "manual-pending", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "manual-pending", Capabilities: []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceManual,
		Description: "keep me", Status: 1, SyncOfficial: 1,
	}).Error)
	t.Setenv("SYNC_UPSTREAM_BASE", "http://127.0.0.1:1")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var metadata model.Model
	require.NoError(t, db.Where("model_name = ?", "manual-pending").First(&metadata).Error)
	assert.Equal(t, model.ModelMetadataStatusPending, metadata.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceManual, metadata.MetadataSource)
	assert.Equal(t, "keep me", metadata.Description)
}

func TestInferUpstreamModelCategories(t *testing.T) {
	tests := []struct {
		name     string
		model    upstreamModel
		expected []types.ModelCategory
	}{
		{
			name: "structured capabilities win",
			model: upstreamModel{
				Capabilities: []types.ModelCategory{types.ModelCategoryText, types.ModelCategoryImage},
				Endpoints:    json.RawMessage(`["openai-video"]`),
			},
			expected: []types.ModelCategory{types.ModelCategoryText, types.ModelCategoryImage},
		},
		{name: "image", model: upstreamModel{Endpoints: json.RawMessage(`["image-generation"]`)}, expected: []types.ModelCategory{types.ModelCategoryImage}},
		{name: "video", model: upstreamModel{Endpoints: json.RawMessage(`["openai-video"]`)}, expected: []types.ModelCategory{types.ModelCategoryVideo}},
		{name: "text", model: upstreamModel{Endpoints: json.RawMessage(`["openai-response"]`)}, expected: []types.ModelCategory{types.ModelCategoryText}},
		{name: "multimodal text", model: upstreamModel{Endpoints: json.RawMessage(`["anthropic"]`), Tags: "Vision,Reasoning"}, expected: []types.ModelCategory{types.ModelCategoryTextMultimodal}},
		{name: "audio", model: upstreamModel{Tags: "Audio"}, expected: []types.ModelCategory{types.ModelCategoryAudio}},
		{name: "audio endpoint", model: upstreamModel{Endpoints: json.RawMessage(`["audio-speech"]`)}, expected: []types.ModelCategory{types.ModelCategoryAudio}},
		{name: "embedding stays other", model: upstreamModel{Endpoints: json.RawMessage(`["embeddings"]`)}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "specialized endpoint wins over text", model: upstreamModel{Endpoints: json.RawMessage(`["openai","embeddings"]`)}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "rerank stays other", model: upstreamModel{Endpoints: json.RawMessage(`["jina-rerank"]`)}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "search stays other", model: upstreamModel{Endpoints: json.RawMessage(`["openai-alpha-search"]`)}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "unknown stays other", model: upstreamModel{}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "explicit other", model: upstreamModel{Capabilities: []types.ModelCategory{types.ModelCategoryOther}}, expected: []types.ModelCategory{types.ModelCategoryOther}},
		{name: "multimodal excludes text", model: upstreamModel{Capabilities: []types.ModelCategory{types.ModelCategoryText, types.ModelCategoryTextMultimodal}}, expected: []types.ModelCategory{types.ModelCategoryTextMultimodal}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, inferUpstreamModelCategories(test.model))
		})
	}
}

func TestSyncUpstreamModelsKeepsEndpointInferencePendingWithoutStructuredCatalog(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "exact-pending", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "exact-pending", Capabilities: []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceChannel,
		Status: 1, SyncOfficial: 1,
	}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`[{"model_name":"exact-pending","vendor_name":"Test","description":"confirmed","endpoints":["openai-response"]}]`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`[{"name":"Test","status":1}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"success":true`)

	var metadata model.Model
	require.NoError(t, db.Where("model_name = ?", "exact-pending").First(&metadata).Error)
	assert.Equal(t, model.ModelMetadataStatusPending, metadata.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceBaseLLMExact, metadata.MetadataSource)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryText}, metadata.Capabilities)
	assert.JSONEq(t, `["openai-response"]`, metadata.Endpoints)
}

func TestSyncUpstreamModelsConfirmsExactStructuredMetadata(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "exact-structured", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "exact-structured", Capabilities: []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceChannel,
		Status: 1, SyncOfficial: 1,
	}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`[{"model_name":"exact-structured","vendor_name":"OpenAI","description":"confirmed"}]`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`[{"name":"OpenAI","status":1}]`))
		case "/api/all.json":
			_, _ = w.Write([]byte(`{"openai":{"id":"openai","name":"OpenAI","models":{"exact-structured":{"id":"exact-structured","modalities":{"input":["text","image"],"output":["text"]},"limit":{"context":128000,"output":8192},"tool_call":true}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var metadata model.Model
	require.NoError(t, db.Where("model_name = ?", "exact-structured").First(&metadata).Error)
	assert.Equal(t, model.ModelMetadataStatusConfirmed, metadata.MetadataStatus)
	assert.Equal(t, []string{"text", "image"}, metadata.InputModalities)
	assert.Equal(t, []string{"text"}, metadata.OutputModalities)
	assert.Equal(t, []string{"tools"}, metadata.SupportedParameters)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryTextMultimodal}, metadata.Capabilities)
	assert.EqualValues(t, 128000, metadata.ContextLength)
	assert.EqualValues(t, 8192, metadata.MaxOutputTokens)
}

func TestSyncUpstreamModelsPersistsSpecializedCategoryWithoutRewritingArchitecture(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "text-embedding-3-small", ChannelId: 1, Enabled: true}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`[{"model_name":"text-embedding-3-small","vendor_name":"OpenAI","endpoints":["openai","embeddings"]}]`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`[{"name":"OpenAI","status":1}]`))
		case "/api/all.json":
			_, _ = w.Write([]byte(`{"openai":{"id":"openai","name":"OpenAI","models":{"text-embedding-3-small":{"id":"text-embedding-3-small","family":"text-embedding","modalities":{"input":["text"],"output":["text"]}}}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var metadata model.Model
	require.NoError(t, db.Where("model_name = ?", "text-embedding-3-small").First(&metadata).Error)
	assert.Equal(t, model.ModelMetadataStatusConfirmed, metadata.MetadataStatus)
	assert.Equal(t, []string{"text"}, metadata.InputModalities)
	assert.Equal(t, []string{"text"}, metadata.OutputModalities)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryOther}, metadata.Capabilities)
}

func TestSyncUpstreamModelsPreservesLastStructuredMetadataWhenAllCatalogFails(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Ability{Group: "default", Model: "preserve-structured", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "preserve-structured", Description: "old description",
		InputModalities: []string{"text", "image"}, OutputModalities: []string{"text"},
		SupportedParameters: []string{"tools"}, ContextLength: 32000, MaxOutputTokens: 4096,
		Capabilities:   []types.ModelCategory{types.ModelCategoryTextMultimodal},
		MetadataStatus: model.ModelMetadataStatusConfirmed, MetadataSource: model.ModelMetadataSourceChannel,
		Status: 1, SyncOfficial: 1,
	}).Error)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/newapi/models.json":
			_, _ = w.Write([]byte(`[{"model_name":"preserve-structured","vendor_name":"OpenAI","description":"new description","endpoints":["openai"]}]`))
		case "/api/newapi/vendors.json":
			_, _ = w.Write([]byte(`[{"name":"OpenAI","status":1}]`))
		default:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(server.Close)
	t.Setenv("SYNC_UPSTREAM_BASE", server.URL)
	t.Setenv("SYNC_HTTP_RETRY", "1")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/models/sync_upstream", strings.NewReader(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncUpstreamModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)

	var metadata model.Model
	require.NoError(t, db.Where("model_name = ?", "preserve-structured").First(&metadata).Error)
	assert.Equal(t, "new description", metadata.Description)
	assert.Equal(t, []string{"text", "image"}, metadata.InputModalities)
	assert.Equal(t, []string{"text"}, metadata.OutputModalities)
	assert.Equal(t, []string{"tools"}, metadata.SupportedParameters)
	assert.EqualValues(t, 32000, metadata.ContextLength)
	assert.EqualValues(t, 4096, metadata.MaxOutputTokens)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryTextMultimodal}, metadata.Capabilities)
	assert.Equal(t, model.ModelMetadataStatusConfirmed, metadata.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceChannel, metadata.MetadataSource)
}

func TestModelReconcilePreviewSeparatesSafePendingAndConflicts(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "OpenAI/GPT-5", Capabilities: []types.ModelCategory{types.ModelCategoryOther}, MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceChannel, Status: 1, SyncOfficial: 1},
		{ModelName: "codex-auto-review", Capabilities: []types.ModelCategory{types.ModelCategoryOther}, MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceChannel, Status: 1, SyncOfficial: 1},
	}).Error)
	channels := []model.Channel{{Id: 7, Name: "CommandCode", Models: "OpenAI/GPT-5,codex-auto-review"}}
	catalog := []upstreamModel{{ModelName: "gpt-5", Endpoints: json.RawMessage(`["openai-response"]`)}}

	preview, err := buildModelReconcilePreview(channels, catalog, nil, 100)
	require.NoError(t, err)

	require.Len(t, preview.SafeMatches, 1)
	assert.Equal(t, "OpenAI/GPT-5", preview.SafeMatches[0].UpstreamModel)
	assert.Equal(t, "gpt-5", preview.SafeMatches[0].CanonicalModel)
	assert.Equal(t, model.ModelMetadataSourceBaseLLMNormalized, preview.SafeMatches[0].MetadataSource)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryText}, preview.SafeMatches[0].Capabilities)
	require.Len(t, preview.Pending, 1)
	assert.Equal(t, "codex-auto-review", preview.Pending[0].UpstreamModel)
	require.Len(t, preview.Aliases, 1)
}

func TestModelReconcilePreviewRejectsDuplicateCanonicalWithinChannel(t *testing.T) {
	setupModelReconcileTestDB(t)
	channels := []model.Channel{{Id: 9, Name: "duplicate", Models: "vendor-a/gpt-5,vendor-b/gpt-5"}}
	catalog := []upstreamModel{{ModelName: "gpt-5"}}

	preview, err := buildModelReconcilePreview(channels, catalog, nil, 100)
	require.NoError(t, err)

	assert.Empty(t, preview.SafeMatches)
	require.Len(t, preview.Conflicts, 2)
	for _, conflict := range preview.Conflicts {
		assert.Equal(t, "duplicate_canonical_in_channel", conflict.Reason)
	}
}

func TestModelReconcilePreviewPreservesManualMetadata(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "Vendor/GPT-5", Capabilities: []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceManual,
		Status: 1, SyncOfficial: 1,
	}).Error)

	preview, err := buildModelReconcilePreview(
		[]model.Channel{{Id: 10, Name: "manual", Models: "Vendor/GPT-5"}},
		[]upstreamModel{{ModelName: "gpt-5"}}, nil, 100,
	)
	require.NoError(t, err)
	assert.Empty(t, preview.SafeMatches)
	require.Len(t, preview.Conflicts, 1)
	assert.Equal(t, "confirmed_metadata_preserved", preview.Conflicts[0].Reason)
}

func TestModelReconcilePreviewRejectsOccupiedAlias(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	for _, name := range []string{"gpt-5", "other-canonical"} {
		require.NoError(t, db.Create(&model.Model{
			ModelName: name, Capabilities: []types.ModelCategory{types.ModelCategoryText},
			MetadataStatus: model.ModelMetadataStatusConfirmed, MetadataSource: model.ModelMetadataSourceBaseLLMExact,
			Status: 1, SyncOfficial: 1,
		}).Error)
	}
	require.NoError(t, model.CreateOrActivateModelAlias(db, "Vendor/GPT-5", "other-canonical", model.ModelMetadataSourceBaseLLMNormalized, 100))

	preview, err := buildModelReconcilePreview(
		[]model.Channel{{Id: 12, Name: "occupied", Models: "Vendor/GPT-5"}},
		[]upstreamModel{{ModelName: "gpt-5"}}, nil, 100,
	)
	require.NoError(t, err)
	assert.Empty(t, preview.SafeMatches)
	require.Len(t, preview.Conflicts, 1)
	assert.Equal(t, "alias_occupied", preview.Conflicts[0].Reason)
}

func TestModelReconcilePreviewRejectsCanonicalNameReservedByAlias(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "real-canonical", Capabilities: []types.ModelCategory{types.ModelCategoryText},
		MetadataStatus: model.ModelMetadataStatusConfirmed, MetadataSource: model.ModelMetadataSourceManual,
		Status: 1, SyncOfficial: 1,
	}).Error)
	require.NoError(t, model.CreateOrActivateModelAlias(db, "gpt-5", "real-canonical", model.ModelMetadataSourceManual, 100))

	preview, err := buildModelReconcilePreview(
		[]model.Channel{{Id: 13, Name: "reserved", Models: "Vendor/GPT-5"}},
		[]upstreamModel{{ModelName: "gpt-5"}}, nil, 100,
	)
	require.NoError(t, err)
	assert.Empty(t, preview.SafeMatches)
	require.Len(t, preview.Conflicts, 1)
	assert.Equal(t, "canonical_name_is_alias", preview.Conflicts[0].Reason)
}

func TestApplyModelReconcileRollsBackPricingWhenChannelChanges(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	originalRatios := ratio_setting.GetModelRatioCopy()
	t.Cleanup(func() {
		encoded, err := common.Marshal(originalRatios)
		require.NoError(t, err)
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))
	})
	testRatios := ratio_setting.GetModelRatioCopy()
	testRatios["Vendor/transaction-source"] = 2
	delete(testRatios, "transaction-target")
	encoded, err := common.Marshal(testRatios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(encoded)))

	_, err = applyModelReconcileMatches([]modelReconcileMatch{{
		ChannelID: 999, ChannelName: "missing", UpstreamModel: "Vendor/transaction-source",
		CanonicalModel: "transaction-target", MatchType: "normalized",
		MetadataSource: model.ModelMetadataSourceBaseLLMNormalized,
	}}, []upstreamModel{{ModelName: "transaction-target"}}, nil)
	require.Error(t, err)

	var optionCount int64
	require.NoError(t, db.Model(&model.Option{}).Where("key = ?", "ModelRatio").Count(&optionCount).Error)
	assert.Zero(t, optionCount)
	_, canonicalConfigured := ratio_setting.GetModelRatioCopy()["transaction-target"]
	assert.False(t, canonicalConfigured)
}

func TestApplyModelReconcileUpdatesChannelMetadataAliasAndAbilitiesIdempotently(t *testing.T) {
	db := setupModelReconcileTestDB(t)
	channel := model.Channel{Id: 11, Name: "CommandCode", Models: "OpenAI/GPT-5", Group: "default", Status: common.ChannelStatusEnabled}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Model{
		ModelName: "OpenAI/GPT-5", Capabilities: []types.ModelCategory{types.ModelCategoryOther},
		MetadataStatus: model.ModelMetadataStatusPending, MetadataSource: model.ModelMetadataSourceChannel,
		Status: 1, SyncOfficial: 1,
	}).Error)
	catalog := []upstreamModel{{
		ModelName: "gpt-5", Description: "Canonical GPT-5", VendorName: "OpenAI",
		Endpoints:          json.RawMessage(`["openai-response"]`),
		InputModalities:    []string{"text"},
		OutputModalities:   []string{"text"},
		StructuredMetadata: true,
	}}
	vendors := []upstreamVendor{{Name: "OpenAI", Status: 1}}
	selected := []modelReconcileMatch{{
		ChannelID: 11, ChannelName: "CommandCode", UpstreamModel: "OpenAI/GPT-5",
		CanonicalModel: "gpt-5", MatchType: "normalized",
		MetadataSource: model.ModelMetadataSourceBaseLLMNormalized,
		CreateAlias:    true, Capabilities: []types.ModelCategory{types.ModelCategoryText},
	}}

	result, err := applyModelReconcileMatches(selected, catalog, vendors)
	require.NoError(t, err)
	assert.Equal(t, 1, result["applied"])

	require.NoError(t, db.First(&channel, 11).Error)
	assert.Equal(t, "gpt-5", channel.Models)
	var mapping map[string]string
	require.NoError(t, common.UnmarshalJsonStr(channel.GetModelMapping(), &mapping))
	assert.Equal(t, "OpenAI/GPT-5", mapping["gpt-5"])

	var canonical model.Model
	require.NoError(t, db.Where("model_name = ?", "gpt-5").First(&canonical).Error)
	assert.Equal(t, model.ModelMetadataStatusConfirmed, canonical.MetadataStatus)
	assert.Equal(t, model.ModelMetadataSourceBaseLLMNormalized, canonical.MetadataSource)
	assert.Equal(t, []types.ModelCategory{types.ModelCategoryText}, canonical.Capabilities)
	var legacyCount int64
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "OpenAI/GPT-5").Count(&legacyCount).Error)
	assert.Zero(t, legacyCount)

	var aliasCount int64
	require.NoError(t, db.Model(&model.ModelAlias{}).Where("alias_name = ?", "OpenAI/GPT-5").Count(&aliasCount).Error)
	assert.EqualValues(t, 1, aliasCount)
	var ability model.Ability
	require.NoError(t, db.Where("channel_id = ? AND model = ?", 11, "gpt-5").First(&ability).Error)

	preview, err := buildModelReconcilePreview([]model.Channel{channel}, catalog, nil, 200)
	require.NoError(t, err)
	assert.Empty(t, preview.SafeMatches)
	assert.Empty(t, preview.Aliases)

	require.NoError(t, db.Model(&model.ModelAlias{}).Where("alias_name = ?", "OpenAI/GPT-5").Update("retire_after", 1234).Error)

	_, err = applyModelReconcileMatches(selected, catalog, vendors)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.ModelAlias{}).Where("alias_name = ?", "OpenAI/GPT-5").Count(&aliasCount).Error)
	assert.EqualValues(t, 1, aliasCount)
	var storedAlias model.ModelAlias
	require.NoError(t, db.Where("alias_name = ?", "OpenAI/GPT-5").First(&storedAlias).Error)
	assert.EqualValues(t, 1234, storedAlias.RetireAfter)
}
