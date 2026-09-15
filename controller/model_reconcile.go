package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const modelAliasCompatibilityDays = 30

type modelReconcileMatch struct {
	ChannelID           int                   `json:"channel_id"`
	ChannelName         string                `json:"channel_name"`
	UpstreamModel       string                `json:"upstream_model"`
	CanonicalModel      string                `json:"canonical_model"`
	MatchType           string                `json:"match_type"`
	MetadataSource      string                `json:"metadata_source"`
	CreateAlias         bool                  `json:"create_alias"`
	Capabilities        []types.ModelCategory `json:"capabilities"`
	InputModalities     []string              `json:"input_modalities"`
	OutputModalities    []string              `json:"output_modalities"`
	SupportedParameters []string              `json:"supported_parameters"`
	ContextLength       int64                 `json:"context_length"`
	MaxOutputTokens     int64                 `json:"max_output_tokens"`
	MetadataStatus      string                `json:"metadata_status"`
	RetireAfter         int64                 `json:"retire_after,omitempty"`
}

type modelReconcilePending struct {
	ChannelID     int    `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	UpstreamModel string `json:"upstream_model"`
	Reason        string `json:"reason"`
}

type modelReconcileConflict struct {
	ChannelID      int    `json:"channel_id"`
	ChannelName    string `json:"channel_name"`
	UpstreamModel  string `json:"upstream_model"`
	CanonicalModel string `json:"canonical_model,omitempty"`
	Reason         string `json:"reason"`
	Detail         string `json:"detail,omitempty"`
}

type modelAliasPreview struct {
	AliasName          string `json:"alias_name"`
	CanonicalModelName string `json:"canonical_model_name"`
	RetireAfter        int64  `json:"retire_after"`
	Source             string `json:"source"`
}

type modelReconcilePreview struct {
	SafeMatches []modelReconcileMatch    `json:"safe_matches"`
	Pending     []modelReconcilePending  `json:"pending"`
	Conflicts   []modelReconcileConflict `json:"conflicts"`
	Aliases     []modelAliasPreview      `json:"aliases"`
	Source      map[string]string        `json:"source"`
}

type modelReconcileApplyItem struct {
	ChannelID      int    `json:"channel_id"`
	UpstreamModel  string `json:"upstream_model"`
	CanonicalModel string `json:"canonical_model"`
}

type modelReconcileApplyRequest struct {
	Items  []modelReconcileApplyItem `json:"items"`
	Locale string                    `json:"locale"`
}

type modelCatalogSnapshot struct {
	Models  []upstreamModel
	Vendors []upstreamVendor
	Source  map[string]string
}

type baseLLMStructuredProvider struct {
	ID     string                            `json:"id"`
	Name   string                            `json:"name"`
	Models map[string]baseLLMStructuredModel `json:"models"`
}

type baseLLMStructuredModel struct {
	ID               string                   `json:"id"`
	Family           string                   `json:"family"`
	Attachment       bool                     `json:"attachment"`
	PDF              bool                     `json:"pdf"`
	Modalities       baseLLMModalities        `json:"modalities"`
	Limit            baseLLMLimit             `json:"limit"`
	ToolCall         bool                     `json:"tool_call"`
	Reasoning        bool                     `json:"reasoning"`
	ReasoningOptions []baseLLMReasoningOption `json:"reasoning_options"`
	StructuredOutput bool                     `json:"structured_output"`
	Temperature      bool                     `json:"temperature"`
}

type baseLLMReasoningOption struct {
	Type string `json:"type"`
}

type baseLLMModalities struct {
	Input  []string `json:"input"`
	Output []string `json:"output"`
}

type baseLLMLimit struct {
	Context int64 `json:"context"`
	Output  int64 `json:"output"`
}

type baseLLMStructuredCandidate struct {
	ProviderKey  string
	ProviderName string
	ModelKey     string
	Model        baseLLMStructuredModel
}

type modelCatalogAdapter interface {
	Fetch(ctx context.Context, locale string) (modelCatalogSnapshot, error)
}

type baseLLMCatalogAdapter struct{}

var activeModelCatalogAdapter modelCatalogAdapter = baseLLMCatalogAdapter{}
var modelReconcileApplyMutex sync.Mutex

func normalizeCatalogMatchKey(name string) string {
	name = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, name)
	if slash := strings.Index(name, "/"); slash >= 0 && slash+1 < len(name) {
		name = name[slash+1:]
	}
	return name
}

func buildCatalogIndexes(catalog []upstreamModel) (map[string][]upstreamModel, map[string][]upstreamModel) {
	exact := make(map[string][]upstreamModel, len(catalog))
	normalized := make(map[string][]upstreamModel, len(catalog))
	for _, item := range catalog {
		item.ModelName = strings.TrimSpace(item.ModelName)
		if item.ModelName == "" {
			continue
		}
		exact[item.ModelName] = append(exact[item.ModelName], item)
		key := normalizeCatalogMatchKey(item.ModelName)
		normalized[key] = append(normalized[key], item)
	}
	return exact, normalized
}

func matchCatalogModel(name string, exact map[string][]upstreamModel, normalized map[string][]upstreamModel) (upstreamModel, string, bool, bool) {
	if matches := exact[name]; len(matches) == 1 {
		return matches[0], "exact", true, false
	} else if len(matches) > 1 {
		return upstreamModel{}, "", false, true
	}
	matches := normalized[normalizeCatalogMatchKey(name)]
	unique := make(map[string]upstreamModel, len(matches))
	for _, item := range matches {
		unique[item.ModelName] = item
	}
	if len(unique) == 1 {
		for _, item := range unique {
			return item, "normalized", true, false
		}
	}
	return upstreamModel{}, "", false, len(unique) > 1
}

func inferUpstreamModelCategories(item upstreamModel) []types.ModelCategory {
	var endpoints []types.EndpointType
	_ = common.Unmarshal(item.Endpoints, &endpoints)
	if item.StructuredMetadata {
		if item.SpecializedModelKind != "" {
			return []types.ModelCategory{types.ModelCategoryOther}
		}
		return model.DeriveModelCategories(item.InputModalities, item.OutputModalities, endpoints)
	}
	if categories := normalizeInferredCategories(item.Capabilities); len(categories) > 0 {
		return categories
	}
	endpointSet := make(map[types.EndpointType]struct{}, len(endpoints))
	for _, endpoint := range endpoints {
		endpointSet[endpoint] = struct{}{}
	}
	for _, endpoint := range []types.EndpointType{
		types.EndpointTypeEmbeddings,
		types.EndpointTypeJinaRerank,
		types.EndpointTypeOpenAIAlphaSearch,
	} {
		if _, ok := endpointSet[endpoint]; ok {
			return []types.ModelCategory{types.ModelCategoryOther}
		}
	}
	categories := make([]types.ModelCategory, 0, 4)
	if _, ok := endpointSet[types.EndpointTypeImageGeneration]; ok {
		categories = append(categories, types.ModelCategoryImage)
	}
	if _, ok := endpointSet[types.EndpointTypeOpenAIVideo]; ok {
		categories = append(categories, types.ModelCategoryVideo)
	}
	tags := strings.ToLower(item.Tags)
	_, audioEndpoint := endpointSet[types.EndpointType("audio")]
	if !audioEndpoint {
		for _, endpoint := range []types.EndpointType{"openai-audio", "audio-speech", "audio-transcription", "audio-translation"} {
			if _, ok := endpointSet[endpoint]; ok {
				audioEndpoint = true
				break
			}
		}
	}
	if audioEndpoint || strings.Contains(tags, "audio") || strings.Contains(tags, "音频") {
		categories = append(categories, types.ModelCategoryAudio)
	}
	textEndpoint := false
	for _, endpoint := range []types.EndpointType{
		types.EndpointTypeOpenAI,
		types.EndpointTypeOpenAIResponse,
		types.EndpointTypeOpenAIResponseCompact,
		types.EndpointTypeAnthropic,
		types.EndpointTypeGemini,
	} {
		if _, ok := endpointSet[endpoint]; ok {
			textEndpoint = true
			break
		}
	}
	if textEndpoint {
		if strings.Contains(tags, "vision") || strings.Contains(tags, "multimodal") || strings.Contains(tags, "多模态") {
			categories = append(categories, types.ModelCategoryTextMultimodal)
		} else {
			categories = append(categories, types.ModelCategoryText)
		}
	}
	categories = normalizeInferredCategories(categories)
	if len(categories) == 0 {
		return []types.ModelCategory{types.ModelCategoryOther}
	}
	return categories
}

func normalizeInferredCategories(categories []types.ModelCategory) []types.ModelCategory {
	categories = model.NormalizeDerivedModelCategories(categories)
	if len(categories) == 0 {
		return nil
	}
	hasMultimodal := false
	hasSpecific := false
	for _, category := range categories {
		if category == types.ModelCategoryTextMultimodal {
			hasMultimodal = true
		}
		if category != types.ModelCategoryOther {
			hasSpecific = true
		}
	}
	result := make([]types.ModelCategory, 0, len(categories))
	for _, category := range categories {
		if hasMultimodal && category == types.ModelCategoryText {
			continue
		}
		if hasSpecific && category == types.ModelCategoryOther {
			continue
		}
		result = append(result, category)
	}
	return result
}

func upstreamMetadataStatus(item upstreamModel) string {
	if item.StructuredMetadata {
		return model.ModelMetadataStatusConfirmed
	}
	return model.ModelMetadataStatusPending
}

func isProtectedModelMetadata(item model.Model) bool {
	if item.SyncOfficial == 0 || item.MetadataSource == model.ModelMetadataSourceManual {
		return true
	}
	if item.MetadataSource != model.ModelMetadataSourceMigration {
		return false
	}
	categories := model.CatalogModelCategories(item.Capabilities)
	return len(categories) != 1 || categories[0] != types.ModelCategoryOther
}

func applyUpstreamMetadata(target *model.Model, item upstreamModel, source string, vendorID int) {
	target.Description = item.Description
	target.Icon = item.Icon
	target.Tags = item.Tags
	target.VendorID = vendorID
	target.NameRule = item.NameRule
	if len(item.Endpoints) > 0 && string(item.Endpoints) != "null" {
		target.Endpoints = string(item.Endpoints)
	}
	if item.StructuredMetadata {
		target.InputModalities = append([]string(nil), item.InputModalities...)
		target.OutputModalities = append([]string(nil), item.OutputModalities...)
		target.SupportedParameters = append([]string(nil), item.SupportedParameters...)
		target.ContextLength = item.ContextLength
		target.MaxOutputTokens = item.MaxOutputTokens
		target.MetadataStatus = model.ModelMetadataStatusConfirmed
		target.MetadataSource = source
		target.NormalizeStructuredMetadata()
		target.Capabilities = inferUpstreamModelCategories(item)
	} else if !target.HasStructuredMetadata() {
		target.Capabilities = inferUpstreamModelCategories(item)
		target.MetadataStatus = model.ModelMetadataStatusPending
		target.MetadataSource = source
	}
	target.UpdatedTime = common.GetTimestamp()
}

func modelMetadataNeedsCatalogRefresh(local model.Model, item upstreamModel, source string, vendorID int) bool {
	if isProtectedModelMetadata(local) {
		return false
	}
	wanted := local
	applyUpstreamMetadata(&wanted, item, source, vendorID)
	return strings.TrimSpace(local.Description) != strings.TrimSpace(wanted.Description) ||
		strings.TrimSpace(local.Icon) != strings.TrimSpace(wanted.Icon) ||
		strings.TrimSpace(local.Tags) != strings.TrimSpace(wanted.Tags) ||
		local.VendorID != wanted.VendorID || local.NameRule != wanted.NameRule ||
		local.Endpoints != wanted.Endpoints ||
		!slices.Equal(model.NormalizeModelCatalogValues(local.InputModalities), wanted.InputModalities) ||
		!slices.Equal(model.NormalizeModelCatalogValues(local.OutputModalities), wanted.OutputModalities) ||
		!slices.Equal(model.NormalizeModelCatalogValues(local.SupportedParameters), wanted.SupportedParameters) ||
		local.ContextLength != wanted.ContextLength || local.MaxOutputTokens != wanted.MaxOutputTokens ||
		!slices.Equal(model.CatalogModelCategories(local.Capabilities), model.CatalogModelCategories(wanted.Capabilities)) ||
		local.MetadataStatus != wanted.MetadataStatus || local.MetadataSource != wanted.MetadataSource
}

func fetchModelCatalog(ctx context.Context, locale string) ([]upstreamModel, []upstreamVendor, map[string]string, error) {
	snapshot, err := activeModelCatalogAdapter.Fetch(ctx, locale)
	return snapshot.Models, snapshot.Vendors, snapshot.Source, err
}

func (baseLLMCatalogAdapter) Fetch(ctx context.Context, locale string) (modelCatalogSnapshot, error) {
	modelsURL, vendorsURL := getUpstreamURLs(locale)
	allURL := getUpstreamAllURL()
	var modelsEnv upstreamEnvelope[upstreamModel]
	var vendorsEnv upstreamEnvelope[upstreamVendor]
	var structuredProviders map[string]baseLLMStructuredProvider
	var modelErr error
	var vendorErr error
	var structuredErr error
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		modelErr = fetchJSON(ctx, modelsURL, &modelsEnv)
	}()
	go func() {
		defer wg.Done()
		vendorErr = fetchJSON(ctx, vendorsURL, &vendorsEnv)
	}()
	go func() {
		defer wg.Done()
		document, responseETag, fromCache, err := fetchJSONDocument(ctx, allURL)
		if err != nil {
			structuredErr = err
			return
		}
		structuredErr = decodeFetchedJSON(allURL, document, responseETag, fromCache, func(body []byte) error {
			var providers map[string]baseLLMStructuredProvider
			if err := common.Unmarshal(body, &providers); err != nil {
				return err
			}
			if len(providers) == 0 {
				return errors.New("structured model catalog returned no providers")
			}
			hasModels := false
			for _, provider := range providers {
				if len(provider.Models) > 0 {
					hasModels = true
					break
				}
			}
			if !hasModels {
				return errors.New("structured model catalog returned no models")
			}
			structuredProviders = providers
			return nil
		})
	}()
	wg.Wait()
	source := map[string]string{
		"locale":      locale,
		"models_url":  modelsURL,
		"vendors_url": vendorsURL,
		"all_url":     allURL,
	}
	if vendorErr != nil {
		source["vendors_warning"] = vendorErr.Error()
	}
	if structuredErr != nil {
		source["all_warning"] = structuredErr.Error()
	}
	if modelErr != nil {
		return modelCatalogSnapshot{Source: source}, modelErr
	}
	if !modelsEnv.Success {
		return modelCatalogSnapshot{Source: source}, fmt.Errorf("model catalog rejected the request: %s", modelsEnv.Message)
	}
	if len(modelsEnv.Data) == 0 {
		return modelCatalogSnapshot{Source: source}, fmt.Errorf("model catalog returned no models")
	}
	if structuredErr == nil {
		modelsEnv.Data = enrichBaseLLMStructuredMetadata(modelsEnv.Data, structuredProviders)
	}
	return modelCatalogSnapshot{Models: modelsEnv.Data, Vendors: vendorsEnv.Data, Source: source}, nil
}

func normalizeStructuredIdentity(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func appendStructuredCandidate(index map[string][]baseLLMStructuredCandidate, key string, candidate baseLLMStructuredCandidate) {
	key = normalizeStructuredIdentity(key)
	if key == "" {
		return
	}
	for _, existing := range index[key] {
		if existing.ProviderKey == candidate.ProviderKey && existing.ModelKey == candidate.ModelKey {
			return
		}
	}
	index[key] = append(index[key], candidate)
}

func enrichBaseLLMStructuredMetadata(catalog []upstreamModel, providers map[string]baseLLMStructuredProvider) []upstreamModel {
	providerIndex := make(map[string][]string)
	globalModelIndex := make(map[string][]baseLLMStructuredCandidate)
	providerModels := make(map[string]map[string][]baseLLMStructuredCandidate)

	for providerKey, provider := range providers {
		providerKey = normalizeStructuredIdentity(providerKey)
		if providerKey == "" {
			continue
		}
		for _, identity := range []string{providerKey, provider.ID, provider.Name} {
			normalized := normalizeStructuredIdentity(identity)
			if normalized == "" || slices.Contains(providerIndex[normalized], providerKey) {
				continue
			}
			providerIndex[normalized] = append(providerIndex[normalized], providerKey)
		}
		modelIndex := make(map[string][]baseLLMStructuredCandidate)
		for modelKey, structured := range provider.Models {
			candidate := baseLLMStructuredCandidate{
				ProviderKey: providerKey, ProviderName: provider.Name,
				ModelKey: modelKey, Model: structured,
			}
			appendStructuredCandidate(modelIndex, modelKey, candidate)
			appendStructuredCandidate(modelIndex, structured.ID, candidate)
			appendStructuredCandidate(globalModelIndex, modelKey, candidate)
			appendStructuredCandidate(globalModelIndex, structured.ID, candidate)
		}
		providerModels[providerKey] = modelIndex
	}

	result := append([]upstreamModel(nil), catalog...)
	for index := range result {
		item := &result[index]
		modelKey := normalizeStructuredIdentity(item.ModelName)
		if modelKey == "" {
			continue
		}

		candidates := make([]baseLLMStructuredCandidate, 0)
		providerKeys := providerIndex[normalizeStructuredIdentity(item.VendorName)]
		for _, providerKey := range providerKeys {
			for _, candidate := range providerModels[providerKey][modelKey] {
				duplicate := false
				for _, existing := range candidates {
					if existing.ProviderKey == candidate.ProviderKey && existing.ModelKey == candidate.ModelKey {
						duplicate = true
						break
					}
				}
				if !duplicate {
					candidates = append(candidates, candidate)
				}
			}
		}
		if len(candidates) == 0 {
			candidates = append(candidates, globalModelIndex[modelKey]...)
		}
		if len(candidates) > 1 {
			item.StructuredMetadataConflict = fmt.Sprintf("structured metadata for %q matched %d providers", item.ModelName, len(candidates))
			continue
		}
		if len(candidates) == 1 {
			applyBaseLLMStructuredMetadata(item, candidates[0].Model)
		}
	}
	return result
}

func applyBaseLLMStructuredMetadata(target *upstreamModel, structured baseLLMStructuredModel) {
	input := make([]string, 0, len(structured.Modalities.Input)+1)
	for _, modality := range structured.Modalities.Input {
		if strings.EqualFold(strings.TrimSpace(modality), "pdf") {
			input = append(input, "file")
			continue
		}
		input = append(input, modality)
	}
	if structured.Attachment || structured.PDF {
		input = append(input, "file")
	}
	parameters := make([]string, 0, 5)
	if structured.ToolCall {
		parameters = append(parameters, "tools")
	}
	if structured.Reasoning {
		parameters = append(parameters, "reasoning")
	}
	for _, option := range structured.ReasoningOptions {
		if strings.EqualFold(strings.TrimSpace(option.Type), "effort") {
			parameters = append(parameters, "reasoning_effort")
			break
		}
	}
	if structured.StructuredOutput {
		parameters = append(parameters, "structured_outputs")
	}
	if structured.Temperature {
		parameters = append(parameters, "temperature")
	}
	target.InputModalities = model.NormalizeModelCatalogValues(input)
	target.OutputModalities = model.NormalizeModelCatalogValues(structured.Modalities.Output)
	target.SpecializedModelKind = baseLLMSpecializedKind(structured)
	target.SupportedParameters = model.NormalizeModelCatalogValues(parameters)
	target.ContextLength = max(structured.Limit.Context, 0)
	target.MaxOutputTokens = max(structured.Limit.Output, 0)
	target.StructuredMetadata = len(target.InputModalities) > 0 || len(target.OutputModalities) > 0 ||
		len(target.SupportedParameters) > 0 || target.ContextLength > 0 || target.MaxOutputTokens > 0
}

func baseLLMSpecializedKind(structured baseLLMStructuredModel) string {
	identities := []string{structured.ID, structured.Family}
	for _, identity := range identities {
		tokens := strings.FieldsFunc(strings.ToLower(identity), func(r rune) bool {
			return r == '/' || r == ':' || r == '-' || r == '_' || r == '.' || unicode.IsSpace(r)
		})
		for _, token := range tokens {
			switch token {
			case "embed", "embedding", "embeddings":
				return "embedding"
			case "rerank", "reranker":
				return "rerank"
			}
		}
	}
	return ""
}

func parseModelMapping(value string) (map[string]string, error) {
	mapping := make(map[string]string)
	value = strings.TrimSpace(value)
	if value == "" || value == "{}" {
		return mapping, nil
	}
	if err := common.Unmarshal([]byte(value), &mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func loadChannelsForReconcile(channelIDs []int) ([]model.Channel, error) {
	query := model.DB.Model(&model.Channel{})
	if len(channelIDs) > 0 {
		query = query.Where("id IN ?", channelIDs)
	}
	var channels []model.Channel
	err := query.Order("id ASC").Find(&channels).Error
	return channels, err
}

func buildModelReconcilePreview(channels []model.Channel, catalog []upstreamModel, source map[string]string, now int64) (modelReconcilePreview, error) {
	preview := modelReconcilePreview{
		SafeMatches: make([]modelReconcileMatch, 0),
		Pending:     make([]modelReconcilePending, 0),
		Conflicts:   make([]modelReconcileConflict, 0),
		Aliases:     make([]modelAliasPreview, 0),
		Source:      source,
	}
	exact, normalized := buildCatalogIndexes(catalog)
	retireAfter := now + int64(modelAliasCompatibilityDays*24*time.Hour/time.Second)

	var metadataRows []model.Model
	if err := model.DB.Find(&metadataRows).Error; err != nil {
		return preview, err
	}
	metadataByName := make(map[string]model.Model, len(metadataRows))
	for _, item := range metadataRows {
		metadataByName[item.ModelName] = item
	}
	var aliases []model.ModelAlias
	if err := model.DB.Find(&aliases).Error; err != nil {
		return preview, err
	}
	aliasByName := make(map[string]model.ModelAlias, len(aliases))
	for _, alias := range aliases {
		aliasByName[alias.AliasName] = alias
	}

	for _, channel := range channels {
		mapping, err := parseModelMapping(channel.GetModelMapping())
		if err != nil {
			preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
				ChannelID: channel.Id, ChannelName: channel.Name, Reason: "invalid_model_mapping", Detail: err.Error(),
			})
			continue
		}
		canonicalCounts := make(map[string]int)
		for _, configuredModel := range uniqueModelNames(channel.Models) {
			upstreamName := configuredModel
			if mapped := strings.TrimSpace(mapping[configuredModel]); mapped != "" && mapped != configuredModel {
				upstreamName = mapped
			}
			if upstream, _, matched, _ := matchCatalogModel(upstreamName, exact, normalized); matched {
				canonicalCounts[upstream.ModelName]++
			}
		}
		channelMatches := make([]modelReconcileMatch, 0)
		for _, configuredModel := range uniqueModelNames(channel.Models) {
			upstreamName := configuredModel
			if mapped := strings.TrimSpace(mapping[configuredModel]); mapped != "" && mapped != configuredModel {
				upstreamName = mapped
			}
			upstream, matchType, matched, ambiguous := matchCatalogModel(upstreamName, exact, normalized)
			if !matched {
				reason := "not_found"
				if ambiguous {
					reason = "ambiguous_normalized_match"
					preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
						ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName, Reason: reason,
					})
				} else {
					preview.Pending = append(preview.Pending, modelReconcilePending{
						ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName, Reason: reason,
					})
				}
				continue
			}
			canonicalName := upstream.ModelName
			if upstream.StructuredMetadataConflict != "" {
				preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
					ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
					CanonicalModel: canonicalName, Reason: "structured_metadata_ambiguous",
					Detail: upstream.StructuredMetadataConflict,
				})
				continue
			}
			if alias, ok := aliasByName[canonicalName]; ok {
				preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
					ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
					CanonicalModel: canonicalName, Reason: "canonical_name_is_alias",
					Detail: fmt.Sprintf("status=%s target=%s", alias.Status, alias.CanonicalModelName),
				})
				continue
			}
			if canonicalCounts[canonicalName] > 1 {
				preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
					ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
					CanonicalModel: canonicalName, Reason: "duplicate_canonical_in_channel",
				})
				continue
			}
			if mapped := strings.TrimSpace(mapping[configuredModel]); mapped != "" && mapped != configuredModel && canonicalName != configuredModel {
				preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
					ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
					CanonicalModel: canonicalName, Reason: "existing_mapping_mismatch",
					Detail: fmt.Sprintf("configured model %q already maps to %q", configuredModel, mapped),
				})
				continue
			}
			if configuredModel != canonicalName {
				if metadata, ok := metadataByName[configuredModel]; ok &&
					isProtectedModelMetadata(metadata) {
					preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
						ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
						CanonicalModel: canonicalName, Reason: "confirmed_metadata_preserved",
						Detail: fmt.Sprintf("status=%s source=%s sync_official=%d", metadata.MetadataStatus, metadata.MetadataSource, metadata.SyncOfficial),
					})
					continue
				}
			}
			aliasActive := false
			needsAlias := upstreamName != canonicalName
			if needsAlias {
				if metadata, ok := metadataByName[upstreamName]; ok && upstreamName != configuredModel &&
					isProtectedModelMetadata(metadata) {
					preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
						ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
						CanonicalModel: canonicalName, Reason: "confirmed_metadata_preserved",
						Detail: fmt.Sprintf("status=%s source=%s sync_official=%d", metadata.MetadataStatus, metadata.MetadataSource, metadata.SyncOfficial),
					})
					continue
				}
				if alias, ok := aliasByName[upstreamName]; ok {
					if alias.CanonicalModelName != canonicalName {
						preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
							ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
							CanonicalModel: canonicalName, Reason: "alias_occupied",
						})
						continue
					}
					if alias.Status != model.ModelAliasStatusActive {
						preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
							ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
							CanonicalModel: canonicalName, Reason: "alias_retired",
						})
						continue
					}
					aliasActive = true
				}
				if existingUpstream := strings.TrimSpace(mapping[canonicalName]); existingUpstream != "" && existingUpstream != upstreamName {
					preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
						ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
						CanonicalModel: canonicalName, Reason: "canonical_mapping_occupied",
						Detail: fmt.Sprintf("canonical model already maps to %q", existingUpstream),
					})
					continue
				}
			}
			sourceName := model.ModelMetadataSourceBaseLLMExact
			if matchType == "normalized" {
				sourceName = model.ModelMetadataSourceBaseLLMNormalized
			}
			canonicalMetadata, hasCanonicalMetadata := metadataByName[canonicalName]
			needsMetadataUpdate := !hasCanonicalMetadata || modelMetadataNeedsCatalogRefresh(canonicalMetadata, upstream, sourceName, canonicalMetadata.VendorID)
			needsChannelRename := configuredModel != canonicalName
			if !needsMetadataUpdate && !needsChannelRename && (!needsAlias || aliasActive) {
				continue
			}
			categories := inferUpstreamModelCategories(upstream)
			if len(categories) == 0 {
				categories = []types.ModelCategory{types.ModelCategoryOther}
			}
			channelMatches = append(channelMatches, modelReconcileMatch{
				ChannelID: channel.Id, ChannelName: channel.Name, UpstreamModel: upstreamName,
				CanonicalModel: canonicalName, MatchType: matchType, MetadataSource: sourceName,
				CreateAlias: needsAlias && !aliasActive, Capabilities: categories, RetireAfter: retireAfter,
				InputModalities:     append([]string(nil), upstream.InputModalities...),
				OutputModalities:    append([]string(nil), upstream.OutputModalities...),
				SupportedParameters: append([]string(nil), upstream.SupportedParameters...),
				ContextLength:       upstream.ContextLength, MaxOutputTokens: upstream.MaxOutputTokens,
				MetadataStatus: upstreamMetadataStatus(upstream),
			})
		}

		for _, item := range channelMatches {
			if detail := pricingConflictForMatch(item); detail != "" {
				preview.Conflicts = append(preview.Conflicts, modelReconcileConflict{
					ChannelID: item.ChannelID, ChannelName: item.ChannelName, UpstreamModel: item.UpstreamModel,
					CanonicalModel: item.CanonicalModel, Reason: "pricing_conflict", Detail: detail,
				})
				continue
			}
			preview.SafeMatches = append(preview.SafeMatches, item)
			if item.CreateAlias {
				preview.Aliases = append(preview.Aliases, modelAliasPreview{
					AliasName: item.UpstreamModel, CanonicalModelName: item.CanonicalModel,
					RetireAfter: retireAfter, Source: item.MetadataSource,
				})
			}
		}
	}
	return preview, nil
}

func uniqueModelNames(models string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0)
	for _, name := range strings.Split(models, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func pricingConflictForMatch(item modelReconcileMatch) string {
	if item.UpstreamModel == item.CanonicalModel {
		return ""
	}
	_, conflicts := preparePricingCopies([]modelReconcileMatch{item})
	if len(conflicts) == 0 {
		return ""
	}
	return strings.Join(conflicts, "; ")
}

func preparePricingCopies(items []modelReconcileMatch) (map[string]string, []string) {
	modelRatios := ratio_setting.GetModelRatioCopy()
	modelPrices := ratio_setting.GetModelPriceCopy()
	billingModes := billing_setting.GetBillingModeCopy()
	billingExpressions := billing_setting.GetBillingExprCopy()
	floatSettings := []struct {
		key    string
		values map[string]float64
	}{
		{"ModelRatio", modelRatios},
		{"ModelPrice", modelPrices},
		{"CompletionRatio", ratio_setting.GetCompletionRatioCopy()},
		{"CacheRatio", ratio_setting.GetCacheRatioCopy()},
		{"CreateCacheRatio", ratio_setting.GetCreateCacheRatioCopy()},
		{"ImageRatio", ratio_setting.GetImageRatioCopy()},
		{"AudioRatio", ratio_setting.GetAudioRatioCopy()},
		{"AudioCompletionRatio", ratio_setting.GetAudioCompletionRatioCopy()},
	}
	stringSettings := []struct {
		key    string
		values map[string]string
	}{
		{"billing_setting.billing_mode", billingModes},
		{"billing_setting.billing_expr", billingExpressions},
	}
	changedFloat := make(map[string]bool)
	changedString := make(map[string]bool)
	preparedCanonical := make(map[string]bool)
	seenPairs := make(map[string]struct{})
	conflicts := make([]string, 0)
	for _, item := range items {
		if item.UpstreamModel == item.CanonicalModel {
			continue
		}
		pairKey := item.UpstreamModel + "\x00" + item.CanonicalModel
		if _, duplicate := seenPairs[pairKey]; duplicate {
			continue
		}
		seenPairs[pairKey] = struct{}{}
		sourceConfigured := helper.HasModelBillingConfig(item.UpstreamModel)
		canonicalConfigured := helper.HasModelBillingConfig(item.CanonicalModel) || preparedCanonical[item.CanonicalModel]
		sourceKind, sourceKindConflict := directPricingKind(item.UpstreamModel, modelRatios, modelPrices, billingModes, billingExpressions)
		canonicalKind, canonicalKindConflict := directPricingKind(item.CanonicalModel, modelRatios, modelPrices, billingModes, billingExpressions)
		if sourceKindConflict || canonicalKindConflict {
			conflicts = append(conflicts, fmt.Sprintf("multiple billing modes are configured for %s or %s", item.UpstreamModel, item.CanonicalModel))
			continue
		}
		if sourceKind != "" && canonicalKind != "" && sourceKind != canonicalKind {
			conflicts = append(conflicts, fmt.Sprintf("billing mode differs for %s and %s", item.UpstreamModel, item.CanonicalModel))
			continue
		}
		itemCopiedPrimary := false
		for i := range floatSettings {
			sourceValue, sourceOK := floatSettings[i].values[item.UpstreamModel]
			canonicalValue, canonicalOK := floatSettings[i].values[item.CanonicalModel]
			if sourceOK && canonicalOK && sourceValue != canonicalValue {
				conflicts = append(conflicts, fmt.Sprintf("%s differs for %s and %s", floatSettings[i].key, item.UpstreamModel, item.CanonicalModel))
				continue
			}
			if sourceOK && !canonicalOK {
				floatSettings[i].values[item.CanonicalModel] = sourceValue
				changedFloat[floatSettings[i].key] = true
				if floatSettings[i].key == "ModelRatio" || floatSettings[i].key == "ModelPrice" {
					itemCopiedPrimary = true
				}
			}
		}
		for i := range stringSettings {
			sourceValue, sourceOK := stringSettings[i].values[item.UpstreamModel]
			canonicalValue, canonicalOK := stringSettings[i].values[item.CanonicalModel]
			if sourceOK && canonicalOK && sourceValue != canonicalValue {
				conflicts = append(conflicts, fmt.Sprintf("%s differs for %s and %s", stringSettings[i].key, item.UpstreamModel, item.CanonicalModel))
				continue
			}
			if sourceOK && !canonicalOK {
				stringSettings[i].values[item.CanonicalModel] = sourceValue
				changedString[stringSettings[i].key] = true
				itemCopiedPrimary = true
			}
		}
		if sourceConfigured && !canonicalConfigured {
			if !itemCopiedPrimary {
				conflicts = append(conflicts, fmt.Sprintf("effective pricing for %s cannot be copied exactly", item.UpstreamModel))
			} else {
				preparedCanonical[item.CanonicalModel] = true
			}
		}
		if !sourceConfigured && !canonicalConfigured && !operation_setting.SelfUseModeEnabled {
			conflicts = append(conflicts, fmt.Sprintf("neither %s nor %s has pricing", item.UpstreamModel, item.CanonicalModel))
		}
	}
	if len(conflicts) > 0 {
		return nil, conflicts
	}
	options := make(map[string]string)
	for _, setting := range floatSettings {
		if !changedFloat[setting.key] {
			continue
		}
		encoded, err := common.Marshal(setting.values)
		if err != nil {
			return nil, []string{err.Error()}
		}
		options[setting.key] = string(encoded)
	}
	for _, setting := range stringSettings {
		if !changedString[setting.key] {
			continue
		}
		encoded, err := common.Marshal(setting.values)
		if err != nil {
			return nil, []string{err.Error()}
		}
		options[setting.key] = string(encoded)
	}
	return options, nil
}

func directPricingKind(name string, modelRatios map[string]float64, modelPrices map[string]float64, billingModes map[string]string, billingExpressions map[string]string) (string, bool) {
	kinds := make(map[string]struct{}, 3)
	if _, ok := modelRatios[name]; ok {
		kinds["ratio"] = struct{}{}
	}
	if _, ok := modelPrices[name]; ok {
		kinds["price"] = struct{}{}
	}
	if billingModes[name] == billing_setting.BillingModeTieredExpr || strings.TrimSpace(billingExpressions[name]) != "" {
		kinds["tiered_expr"] = struct{}{}
	}
	if len(kinds) > 1 {
		return "", true
	}
	for kind := range kinds {
		return kind, false
	}
	return "", false
}

func previewModelReconcile(ctx context.Context, locale string, channelIDs []int) (modelReconcilePreview, []upstreamModel, []upstreamVendor, error) {
	catalog, vendors, source, err := fetchModelCatalog(ctx, locale)
	if err != nil {
		return modelReconcilePreview{}, nil, nil, err
	}
	channels, err := loadChannelsForReconcile(channelIDs)
	if err != nil {
		return modelReconcilePreview{}, nil, nil, err
	}
	preview, err := buildModelReconcilePreview(channels, catalog, source, common.GetTimestamp())
	return preview, catalog, vendors, err
}

func PreviewModelReconcile(c *gin.Context) {
	var channelIDs []int
	if raw := strings.TrimSpace(c.Query("channel_id")); raw != "" {
		channelID, err := strconv.Atoi(raw)
		if err != nil || channelID <= 0 {
			common.ApiErrorMsg(c, "invalid channel_id")
			return
		}
		channelIDs = []int{channelID}
	}
	timeout := time.Duration(common.GetEnvOrDefault("SYNC_HTTP_TIMEOUT_SECONDS", 15)) * time.Second
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	preview, _, _, err := previewModelReconcile(ctx, c.Query("locale"), channelIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func ApplyModelReconcile(c *gin.Context) {
	var request modelReconcileApplyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(request.Items) == 0 {
		common.ApiErrorMsg(c, "no reconcile items selected")
		return
	}
	channelSet := make(map[int]struct{}, len(request.Items))
	channelIDs := make([]int, 0, len(request.Items))
	for _, item := range request.Items {
		if _, ok := channelSet[item.ChannelID]; ok {
			continue
		}
		channelSet[item.ChannelID] = struct{}{}
		channelIDs = append(channelIDs, item.ChannelID)
	}
	timeout := time.Duration(common.GetEnvOrDefault("SYNC_HTTP_TIMEOUT_SECONDS", 15)) * time.Second
	ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
	defer cancel()
	preview, catalog, vendors, err := previewModelReconcile(ctx, request.Locale, channelIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	safeByKey := make(map[string]modelReconcileMatch, len(preview.SafeMatches))
	for _, item := range preview.SafeMatches {
		safeByKey[reconcileItemKey(item.ChannelID, item.UpstreamModel, item.CanonicalModel)] = item
	}
	selected := make([]modelReconcileMatch, 0, len(request.Items))
	seen := make(map[string]struct{}, len(request.Items))
	for _, requested := range request.Items {
		key := reconcileItemKey(requested.ChannelID, requested.UpstreamModel, requested.CanonicalModel)
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		item, ok := safeByKey[key]
		if !ok {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "reconcile preview is stale", "data": preview})
			return
		}
		selected = append(selected, item)
	}
	result, err := applyModelReconcileMatches(selected, catalog, vendors)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
		return
	}
	common.ApiSuccess(c, result)
}

func reconcileItemKey(channelID int, upstream string, canonical string) string {
	return fmt.Sprintf("%d\x00%s\x00%s", channelID, upstream, canonical)
}

func applyModelReconcileMatches(selected []modelReconcileMatch, catalog []upstreamModel, vendors []upstreamVendor) (map[string]int, error) {
	modelReconcileApplyMutex.Lock()
	defer modelReconcileApplyMutex.Unlock()

	options, pricingConflicts := preparePricingCopies(selected)
	if len(pricingConflicts) > 0 {
		return nil, fmt.Errorf("pricing conflict: %s", strings.Join(pricingConflicts, "; "))
	}
	catalogByName := make(map[string]upstreamModel, len(catalog))
	for _, item := range catalog {
		catalogByName[item.ModelName] = item
	}
	vendorByName := make(map[string]upstreamVendor, len(vendors))
	for _, item := range vendors {
		vendorByName[item.Name] = item
	}
	vendorIDCache := make(map[string]int)
	createdVendors := 0
	createdModels := 0
	confirmedModels := 0
	createdAliases := 0
	retireAfter := common.GetTimestamp() + int64(modelAliasCompatibilityDays*24*time.Hour/time.Second)
	err := model.UpdateOptionsBulkAtomically(options, func(tx *gorm.DB) error {
		for _, item := range selected {
			upstream, ok := catalogByName[item.CanonicalModel]
			if !ok {
				return fmt.Errorf("catalog model %q disappeared", item.CanonicalModel)
			}
			if conflict, err := model.ModelNameConflictsWithAlias(tx, item.CanonicalModel); err != nil {
				return err
			} else if conflict {
				return fmt.Errorf("canonical model %q is reserved by a model alias", item.CanonicalModel)
			}
			var channel model.Channel
			if err := tx.Where("id = ?", item.ChannelID).First(&channel).Error; err != nil {
				return err
			}
			mapping, err := parseModelMapping(channel.GetModelMapping())
			if err != nil {
				return err
			}
			var canonical model.Model
			modelErr := tx.Where("model_name = ?", item.CanonicalModel).First(&canonical).Error
			vendorID := 0
			if upstream.VendorName != "" {
				if cachedID, ok := vendorIDCache[upstream.VendorName]; ok {
					vendorID = cachedID
				} else {
					var vendor model.Vendor
					vendorErr := tx.Where("name = ?", upstream.VendorName).First(&vendor).Error
					if vendorErr == gorm.ErrRecordNotFound {
						vendorMetadata := vendorByName[upstream.VendorName]
						now := common.GetTimestamp()
						vendor = model.Vendor{
							Name: upstream.VendorName, Description: vendorMetadata.Description,
							Icon: coalesce(vendorMetadata.Icon, ""), Status: chooseStatus(vendorMetadata.Status, 1),
							CreatedTime: now, UpdatedTime: now,
						}
						if err := tx.Create(&vendor).Error; err != nil {
							return err
						}
						createdVendors++
					} else if vendorErr != nil {
						return vendorErr
					}
					vendorID = vendor.Id
					vendorIDCache[upstream.VendorName] = vendorID
				}
			}
			if modelErr == gorm.ErrRecordNotFound {
				now := common.GetTimestamp()
				canonical = model.Model{
					ModelName:    item.CanonicalModel,
					Status:       chooseStatus(upstream.Status, 1),
					SyncOfficial: 1, NameRule: upstream.NameRule, CreatedTime: now, UpdatedTime: now,
				}
				applyUpstreamMetadata(&canonical, upstream, item.MetadataSource, vendorID)
				if err := tx.Create(&canonical).Error; err != nil {
					return err
				}
				createdModels++
				if canonical.MetadataStatus == model.ModelMetadataStatusConfirmed {
					confirmedModels++
				}
			} else if modelErr != nil {
				return modelErr
			} else if modelMetadataNeedsCatalogRefresh(canonical, upstream, item.MetadataSource, vendorID) {
				wasConfirmed := canonical.MetadataStatus == model.ModelMetadataStatusConfirmed
				applyUpstreamMetadata(&canonical, upstream, item.MetadataSource, vendorID)
				if err := tx.Model(&model.Model{}).Where("id = ?", canonical.Id).
					Select("description", "icon", "tags", "vendor_id", "endpoints", "capabilities", "input_modalities", "output_modalities", "supported_parameters", "context_length", "max_output_tokens", "metadata_status", "metadata_source", "updated_time").
					Updates(&canonical).Error; err != nil {
					return err
				}
				if !wasConfirmed && canonical.MetadataStatus == model.ModelMetadataStatusConfirmed {
					confirmedModels++
				}
			}
			if item.UpstreamModel == item.CanonicalModel {
				continue
			}
			if existing := strings.TrimSpace(mapping[item.CanonicalModel]); existing != "" && existing != item.UpstreamModel {
				return fmt.Errorf("channel %d canonical mapping changed", item.ChannelID)
			}
			var legacy model.Model
			if err := tx.Where("model_name = ?", item.UpstreamModel).First(&legacy).Error; err == nil {
				if isProtectedModelMetadata(legacy) {
					return fmt.Errorf("protected metadata for %q cannot become an alias", item.UpstreamModel)
				}
				if err := tx.Delete(&legacy).Error; err != nil {
					return err
				}
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
			var existingAlias model.ModelAlias
			aliasErr := tx.Where("alias_name = ?", item.UpstreamModel).First(&existingAlias).Error
			if aliasErr == gorm.ErrRecordNotFound {
				if err := model.CreateOrActivateModelAlias(tx, item.UpstreamModel, item.CanonicalModel, item.MetadataSource, retireAfter); err != nil {
					return err
				}
				createdAliases++
			} else if aliasErr != nil {
				return aliasErr
			} else if existingAlias.CanonicalModelName != item.CanonicalModel || existingAlias.Status != model.ModelAliasStatusActive {
				return fmt.Errorf("model alias %q changed while reconcile was applying", item.UpstreamModel)
			}
			models := uniqueModelNames(channel.Models)
			found := false
			for i, name := range models {
				if name == item.UpstreamModel {
					models[i] = item.CanonicalModel
					found = true
				}
				if name == item.CanonicalModel {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("channel %d no longer contains %q", item.ChannelID, item.UpstreamModel)
			}
			models = dedupeStrings(models)
			mapping[item.CanonicalModel] = item.UpstreamModel
			delete(mapping, item.UpstreamModel)
			mappingJSON, err := common.Marshal(mapping)
			if err != nil {
				return err
			}
			oldModels := channel.Models
			oldMapping := channel.ModelMapping
			query := tx.Model(&model.Channel{}).Where("id = ? AND models = ?", channel.Id, oldModels)
			if oldMapping == nil {
				query = query.Where("model_mapping IS NULL")
			} else {
				query = query.Where("model_mapping = ?", *oldMapping)
			}
			newModels := strings.Join(models, ",")
			newMapping := string(mappingJSON)
			if newModels != oldModels || oldMapping == nil || newMapping != *oldMapping {
				result := query.Updates(map[string]interface{}{"models": newModels, "model_mapping": newMapping})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return fmt.Errorf("channel %d changed while reconcile was applying", channel.Id)
				}
			}
			channel.Models = newModels
			channel.ModelMapping = &newMapping
			if err := channel.UpdateAbilities(tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := model.InitModelAliasCache(); err != nil {
		common.SysError("failed to refresh model alias cache after reconcile: " + err.Error())
	}
	model.InitChannelCache()
	model.RefreshPricing()
	return map[string]int{
		"applied": len(selected), "created_models": createdModels, "confirmed_models": confirmedModels,
		"created_aliases": createdAliases, "created_vendors": createdVendors,
	}, nil
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func GetModelAliases(c *gin.Context) {
	aliases, err := model.ListModelAliases(c.Query("status"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	sort.Slice(aliases, func(i, j int) bool { return aliases[i].Id > aliases[j].Id })
	common.ApiSuccess(c, aliases)
}

func RetireModelAliases(c *gin.Context) {
	var request struct {
		AliasNames []string `json:"alias_names"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	migratedTokens, retiredAliases, err := model.RetireModelAliases(request.AliasNames)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"migrated_tokens": migratedTokens, "retired_aliases": retiredAliases})
}
