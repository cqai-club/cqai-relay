package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 上游地址
const (
	upstreamModelsURL  = "https://basellm.github.io/llm-metadata/api/newapi/models.json"
	upstreamVendorsURL = "https://basellm.github.io/llm-metadata/api/newapi/vendors.json"
	upstreamAllURL     = "https://basellm.github.io/llm-metadata/api/all.json"
)

func normalizeLocale(locale string) (string, bool) {
	l := strings.ToLower(strings.TrimSpace(locale))
	switch l {
	case "en", "ja":
		return l, true
	case "zh", "zh-cn", "zh-tw":
		return "zh", true
	default:
		return "", false
	}
}

func getUpstreamBase() string {
	return common.GetEnvOrDefaultString("SYNC_UPSTREAM_BASE", "https://basellm.github.io/llm-metadata")
}

func getUpstreamURLs(locale string) (modelsURL, vendorsURL string) {
	base := strings.TrimRight(getUpstreamBase(), "/")
	if l, ok := normalizeLocale(locale); ok && l != "" {
		return fmt.Sprintf("%s/api/i18n/%s/newapi/models.json", base, l),
			fmt.Sprintf("%s/api/i18n/%s/newapi/vendors.json", base, l)
	}
	return fmt.Sprintf("%s/api/newapi/models.json", base), fmt.Sprintf("%s/api/newapi/vendors.json", base)
}

func getUpstreamAllURL() string {
	return strings.TrimRight(getUpstreamBase(), "/") + "/api/all.json"
}

type upstreamEnvelope[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    []T    `json:"data"`
}

type upstreamModel struct {
	Description                string                `json:"description"`
	Endpoints                  json.RawMessage       `json:"endpoints"`
	Icon                       string                `json:"icon"`
	ModelName                  string                `json:"model_name"`
	NameRule                   int                   `json:"name_rule"`
	Status                     int                   `json:"status"`
	Tags                       string                `json:"tags"`
	Capabilities               []types.ModelCategory `json:"capabilities"`
	VendorName                 string                `json:"vendor_name"`
	InputModalities            []string              `json:"input_modalities,omitempty"`
	OutputModalities           []string              `json:"output_modalities,omitempty"`
	SupportedParameters        []string              `json:"supported_parameters,omitempty"`
	ContextLength              int64                 `json:"context_length,omitempty"`
	MaxOutputTokens            int64                 `json:"max_output_tokens,omitempty"`
	StructuredMetadata         bool                  `json:"-"`
	StructuredMetadataConflict string                `json:"-"`
	SpecializedModelKind       string                `json:"-"`
}

type upstreamVendor struct {
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
}

var (
	etagCache  = make(map[string]string)
	bodyCache  = make(map[string][]byte)
	cacheMutex sync.RWMutex
)

type overwriteField struct {
	ModelName string   `json:"model_name"`
	Fields    []string `json:"fields"`
}

type syncRequest struct {
	Overwrite []overwriteField `json:"overwrite"`
	Locale    string           `json:"locale"`
}

func newHTTPClient() *http.Client {
	timeoutSec := common.GetEnvOrDefault("SYNC_HTTP_TIMEOUT_SECONDS", 10)
	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   time.Duration(timeoutSec) * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: time.Duration(timeoutSec) * time.Second,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}
		if strings.HasSuffix(host, "github.io") {
			if conn, err := dialer.DialContext(ctx, "tcp4", addr); err == nil {
				return conn, nil
			}
			return dialer.DialContext(ctx, "tcp6", addr)
		}
		return dialer.DialContext(ctx, network, addr)
	}
	return &http.Client{Transport: transport}
}

var (
	httpClientOnce sync.Once
	httpClient     *http.Client
)

func getHTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		httpClient = newHTTPClient()
	})
	return httpClient
}

func fetchJSONDocument(ctx context.Context, url string) ([]byte, string, bool, error) {
	var lastErr error
	attempts := common.GetEnvOrDefault("SYNC_HTTP_RETRY", 3)
	if attempts < 1 {
		attempts = 1
	}
	baseDelay := 200 * time.Millisecond
	maxMB := common.GetEnvOrDefault("SYNC_HTTP_MAX_MB", 10)
	maxBytes := int64(maxMB) << 20
	for attempt := 0; attempt < attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, "", false, err
		}
		// ETag conditional request
		cacheMutex.RLock()
		if et := etagCache[url]; et != "" {
			req.Header.Set("If-None-Match", et)
		}
		cacheMutex.RUnlock()

		resp, err := getHTTPClient().Do(req)
		if err != nil {
			lastErr = err
			// backoff with jitter
			sleep := baseDelay * time.Duration(1<<attempt)
			jitter := time.Duration(rand.Intn(150)) * time.Millisecond
			time.Sleep(sleep + jitter)
			continue
		}
		var document []byte
		var responseETag string
		fromCache := false
		func() {
			defer resp.Body.Close()
			switch resp.StatusCode {
			case http.StatusOK:
				// read body into buffer for caching and flexible decode
				limited := io.LimitReader(resp.Body, maxBytes+1)
				buf, err := io.ReadAll(limited)
				if err != nil {
					lastErr = err
					return
				}
				if int64(len(buf)) > maxBytes {
					lastErr = fmt.Errorf("response exceeds %d bytes", maxBytes)
					return
				}
				document = append([]byte(nil), buf...)
				responseETag = resp.Header.Get("ETag")
				lastErr = nil
			case http.StatusNotModified:
				// use cache
				cacheMutex.RLock()
				buf := bodyCache[url]
				cacheMutex.RUnlock()
				if len(buf) == 0 {
					lastErr = errors.New("cache miss for 304 response")
					return
				}
				document = append([]byte(nil), buf...)
				fromCache = true
				lastErr = nil
			default:
				lastErr = errors.New(resp.Status)
			}
		}()
		if lastErr == nil {
			return document, responseETag, fromCache, nil
		}
		sleep := baseDelay * time.Duration(1<<attempt)
		jitter := time.Duration(rand.Intn(150)) * time.Millisecond
		time.Sleep(sleep + jitter)
	}
	cacheMutex.RLock()
	cached := append([]byte(nil), bodyCache[url]...)
	cacheMutex.RUnlock()
	if len(cached) > 0 {
		return cached, "", true, nil
	}
	return nil, "", false, lastErr
}

func decodeFetchedJSON(url string, document []byte, responseETag string, fromCache bool, decode func([]byte) error) error {
	if err := decode(document); err != nil {
		if !fromCache {
			cacheMutex.RLock()
			cached := append([]byte(nil), bodyCache[url]...)
			cacheMutex.RUnlock()
			if len(cached) > 0 && !bytes.Equal(cached, document) {
				if cachedErr := decode(cached); cachedErr == nil {
					return nil
				}
			}
		}
		return err
	}
	if fromCache {
		return nil
	}
	cacheMutex.Lock()
	bodyCache[url] = append([]byte(nil), document...)
	if responseETag == "" {
		delete(etagCache, url)
	} else {
		etagCache[url] = responseETag
	}
	cacheMutex.Unlock()
	return nil
}

func fetchJSON[T any](ctx context.Context, url string, out *upstreamEnvelope[T]) error {
	document, responseETag, fromCache, err := fetchJSONDocument(ctx, url)
	if err != nil {
		return err
	}
	return decodeFetchedJSON(url, document, responseETag, fromCache, func(body []byte) error {
		var envelope upstreamEnvelope[T]
		if err := common.Unmarshal(body, &envelope); err != nil {
			var values []T
			if arrayErr := common.Unmarshal(body, &values); arrayErr != nil {
				return err
			}
			envelope.Success = true
			envelope.Data = values
			envelope.Message = ""
		}
		if !envelope.Success {
			if envelope.Message != "" {
				return fmt.Errorf("catalog rejected the request: %s", envelope.Message)
			}
			return errors.New("catalog rejected the request")
		}
		if len(envelope.Data) == 0 {
			return errors.New("catalog returned no entries")
		}
		*out = envelope
		return nil
	})
}

func ensureVendorID(vendorName string, vendorByName map[string]upstreamVendor, vendorIDCache map[string]int, createdVendors *int) int {
	if vendorName == "" {
		return 0
	}
	if id, ok := vendorIDCache[vendorName]; ok {
		return id
	}
	var existing model.Vendor
	if err := model.DB.Where("name = ?", vendorName).First(&existing).Error; err == nil {
		vendorIDCache[vendorName] = existing.Id
		return existing.Id
	}
	uv := vendorByName[vendorName]
	v := &model.Vendor{
		Name:        vendorName,
		Description: uv.Description,
		Icon:        coalesce(uv.Icon, ""),
		Status:      chooseStatus(uv.Status, 1),
	}
	if err := v.Insert(); err == nil {
		*createdVendors++
		vendorIDCache[vendorName] = v.Id
		return v.Id
	}
	vendorIDCache[vendorName] = 0
	return 0
}

// SyncUpstreamModels 同步上游模型与供应商：
// - 默认仅创建「未配置模型」
// - 可通过 overwrite 选择性覆盖更新本地已有模型的字段（前提：sync_official <> 0）
func SyncUpstreamModels(c *gin.Context) {
	var req syncRequest
	// 允许空体
	_ = c.ShouldBindJSON(&req)
	// 1) 获取未配置模型以及所有允许目录持续刷新的已启用模型。
	missing, err := model.GetMissingModels()
	if err != nil {
		common.SysError("failed to get missing models: " + err.Error())
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取模型列表失败，请稍后重试"})
		return
	}
	syncCandidates := append([]string(nil), missing...)
	enabledModels := model.GetEnabledModels()
	if len(enabledModels) > 0 {
		var metadataRows []model.Model
		if err := model.DB.Where("sync_official <> ? AND model_name IN ?", 0, enabledModels).Find(&metadataRows).Error; err != nil {
			common.SysError("failed to get pending model metadata: " + err.Error())
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取待识别模型失败，请稍后重试"})
			return
		}
		seen := make(map[string]struct{}, len(syncCandidates)+len(metadataRows))
		for _, name := range syncCandidates {
			seen[name] = struct{}{}
		}
		for _, metadata := range metadataRows {
			if isProtectedModelMetadata(metadata) {
				continue
			}
			name := metadata.ModelName
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			syncCandidates = append(syncCandidates, name)
		}
	}

	// 若既无可刷新模型，也未指定覆盖更新字段，则无需请求上游数据。
	if len(syncCandidates) == 0 && len(req.Overwrite) == 0 {
		modelsURL, vendorsURL := getUpstreamURLs(req.Locale)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"created_models":  0,
				"created_vendors": 0,
				"updated_models":  0,
				"skipped_models":  []string{},
				"created_list":    []string{},
				"updated_list":    []string{},
				"source": gin.H{
					"locale":      req.Locale,
					"models_url":  modelsURL,
					"vendors_url": vendorsURL,
					"all_url":     getUpstreamAllURL(),
				},
			},
		})
		return
	}

	// 2) 拉取上游 vendors 与 models
	timeoutSec := common.GetEnvOrDefault("SYNC_HTTP_TIMEOUT_SECONDS", 15)
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	upstreamModels, upstreamVendors, source, fetchErr := fetchModelCatalog(ctx, req.Locale)
	modelsURL := source["models_url"]
	vendorsURL := source["vendors_url"]
	allURL := source["all_url"]
	if fetchErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取上游模型失败: " + fetchErr.Error(), "locale": req.Locale, "source_urls": gin.H{"models_url": modelsURL, "vendors_url": vendorsURL}})
		return
	}

	// 建立映射
	vendorByName := make(map[string]upstreamVendor)
	for _, v := range upstreamVendors {
		if v.Name != "" {
			vendorByName[v.Name] = v
		}
	}
	modelByName := make(map[string]upstreamModel)
	for _, m := range upstreamModels {
		if m.ModelName != "" {
			modelByName[m.ModelName] = m
		}
	}

	// 3) 创建真正缺失的模型，并补全与目录精确同名的待识别占位记录。
	createdModels := 0
	createdVendors := 0
	updatedModels := 0
	skipped := make([]string, 0)
	createdList := make([]string, 0)
	updatedList := make([]string, 0)

	// 本地缓存：vendorName -> id
	vendorIDCache := make(map[string]int)

	for _, name := range syncCandidates {
		up, ok := modelByName[name]
		if !ok {
			skipped = append(skipped, name)
			continue
		}

		// 待识别占位记录只在允许官方同步时补全。
		var existing model.Model
		existingErr := model.DB.Where("model_name = ?", name).First(&existing).Error
		if existingErr == nil {
			if isProtectedModelMetadata(existing) {
				skipped = append(skipped, name)
				continue
			}
			vendorID := ensureVendorID(up.VendorName, vendorByName, vendorIDCache, &createdVendors)
			sourceName := model.ModelMetadataSourceBaseLLMExact
			if existing.MetadataSource == model.ModelMetadataSourceBaseLLMNormalized {
				sourceName = model.ModelMetadataSourceBaseLLMNormalized
			}
			if !modelMetadataNeedsCatalogRefresh(existing, up, sourceName, vendorID) {
				continue
			}
			applyUpstreamMetadata(&existing, up, sourceName, vendorID)
			if err := model.DB.Model(&model.Model{}).Where("id = ?", existing.Id).
				Select("description", "icon", "tags", "endpoints", "capabilities", "input_modalities", "output_modalities", "supported_parameters", "context_length", "max_output_tokens", "vendor_id", "name_rule", "metadata_status", "metadata_source", "updated_time").
				Updates(&existing).Error; err != nil {
				skipped = append(skipped, name)
				continue
			}
			updatedModels++
			updatedList = append(updatedList, name)
			continue
		}
		if existingErr != gorm.ErrRecordNotFound {
			skipped = append(skipped, name)
			continue
		}

		// 确保 vendor 存在
		vendorID := ensureVendorID(up.VendorName, vendorByName, vendorIDCache, &createdVendors)

		// 创建模型
		mi := &model.Model{ModelName: name, Status: chooseStatus(up.Status, 1), SyncOfficial: 1, NameRule: up.NameRule}
		applyUpstreamMetadata(mi, up, model.ModelMetadataSourceBaseLLMExact, vendorID)
		if err := mi.Insert(); err == nil {
			createdModels++
			createdList = append(createdList, name)
		} else {
			skipped = append(skipped, name)
		}
	}

	// 4) 处理可选覆盖（更新本地已有模型的差异字段）
	if len(req.Overwrite) > 0 {
		// vendorIDCache 已用于创建阶段，可复用
		for _, ow := range req.Overwrite {
			up, ok := modelByName[ow.ModelName]
			if !ok {
				continue
			}
			var local model.Model
			if err := model.DB.Where("model_name = ?", ow.ModelName).First(&local).Error; err != nil {
				continue
			}

			// 跳过被禁用官方同步的模型
			if isProtectedModelMetadata(local) {
				continue
			}

			// 映射 vendor
			newVendorID := ensureVendorID(up.VendorName, vendorByName, vendorIDCache, &createdVendors)

			// 应用字段覆盖（事务）
			_ = model.DB.Transaction(func(tx *gorm.DB) error {
				needUpdate := false
				if containsField(ow.Fields, "description") {
					local.Description = up.Description
					needUpdate = true
				}
				if containsField(ow.Fields, "icon") {
					local.Icon = up.Icon
					needUpdate = true
				}
				if containsField(ow.Fields, "tags") {
					local.Tags = up.Tags
					needUpdate = true
				}
				if containsField(ow.Fields, "capabilities") {
					if up.StructuredMetadata {
						local.InputModalities = append([]string(nil), up.InputModalities...)
						local.OutputModalities = append([]string(nil), up.OutputModalities...)
						local.SupportedParameters = append([]string(nil), up.SupportedParameters...)
						local.ContextLength = up.ContextLength
						local.MaxOutputTokens = up.MaxOutputTokens
						local.NormalizeStructuredMetadata()
						local.MetadataStatus = model.ModelMetadataStatusConfirmed
						local.Capabilities = inferUpstreamModelCategories(up)
					} else if !local.HasStructuredMetadata() {
						local.Capabilities = inferUpstreamModelCategories(up)
						local.MetadataStatus = model.ModelMetadataStatusPending
					}
					local.MetadataSource = model.ModelMetadataSourceBaseLLMExact
					needUpdate = true
				}
				if containsField(ow.Fields, "input_modalities") && up.StructuredMetadata {
					local.InputModalities = append([]string(nil), up.InputModalities...)
					needUpdate = true
				}
				if containsField(ow.Fields, "output_modalities") && up.StructuredMetadata {
					local.OutputModalities = append([]string(nil), up.OutputModalities...)
					needUpdate = true
				}
				if containsField(ow.Fields, "supported_parameters") && up.StructuredMetadata {
					local.SupportedParameters = append([]string(nil), up.SupportedParameters...)
					needUpdate = true
				}
				if containsField(ow.Fields, "context_length") && up.StructuredMetadata {
					local.ContextLength = up.ContextLength
					needUpdate = true
				}
				if containsField(ow.Fields, "max_output_tokens") && up.StructuredMetadata {
					local.MaxOutputTokens = up.MaxOutputTokens
					needUpdate = true
				}
				if containsField(ow.Fields, "vendor") {
					local.VendorID = newVendorID
					needUpdate = true
				}
				if containsField(ow.Fields, "name_rule") {
					local.NameRule = up.NameRule
					needUpdate = true
				}
				if containsField(ow.Fields, "status") {
					local.Status = chooseStatus(up.Status, local.Status)
					needUpdate = true
				}
				if !needUpdate {
					return nil
				}
				if up.StructuredMetadata && (containsField(ow.Fields, "input_modalities") || containsField(ow.Fields, "output_modalities") ||
					containsField(ow.Fields, "supported_parameters") || containsField(ow.Fields, "context_length") || containsField(ow.Fields, "max_output_tokens")) {
					local.MetadataStatus = model.ModelMetadataStatusConfirmed
					local.MetadataSource = model.ModelMetadataSourceBaseLLMExact
					local.NormalizeStructuredMetadata()
					local.Capabilities = inferUpstreamModelCategories(up)
				}
				if err := tx.Save(&local).Error; err != nil {
					return err
				}
				updatedModels++
				updatedList = append(updatedList, ow.ModelName)
				return nil
			})
		}
	}
	if createdModels > 0 || updatedModels > 0 {
		model.RefreshPricing()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"created_models":  createdModels,
			"created_vendors": createdVendors,
			"updated_models":  updatedModels,
			"skipped_models":  skipped,
			"created_list":    createdList,
			"updated_list":    updatedList,
			"source": gin.H{
				"locale":      req.Locale,
				"models_url":  modelsURL,
				"vendors_url": vendorsURL,
				"all_url":     allURL,
			},
		},
	})
}

func containsField(fields []string, key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, f := range fields {
		if strings.ToLower(strings.TrimSpace(f)) == key {
			return true
		}
	}
	return false
}

func coalesce(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func chooseStatus(primary, fallback int) int {
	if primary == 0 && fallback != 0 {
		return fallback
	}
	if primary != 0 {
		return primary
	}
	return 1
}

// SyncUpstreamPreview 预览上游与本地的差异（仅用于弹窗选择）
func SyncUpstreamPreview(c *gin.Context) {
	// 1) 拉取上游数据
	timeoutSec := common.GetEnvOrDefault("SYNC_HTTP_TIMEOUT_SECONDS", 15)
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	locale := c.Query("locale")
	upstreamModels, upstreamVendors, source, fetchErr := fetchModelCatalog(ctx, locale)
	modelsURL := source["models_url"]
	vendorsURL := source["vendors_url"]
	if fetchErr != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "获取上游模型失败: " + fetchErr.Error(), "locale": locale, "source_urls": gin.H{"models_url": modelsURL, "vendors_url": vendorsURL}})
		return
	}

	vendorByName := make(map[string]upstreamVendor)
	for _, v := range upstreamVendors {
		if v.Name != "" {
			vendorByName[v.Name] = v
		}
	}
	modelByName := make(map[string]upstreamModel)
	upstreamNames := make([]string, 0, len(upstreamModels))
	for _, m := range upstreamModels {
		if m.ModelName != "" {
			modelByName[m.ModelName] = m
			upstreamNames = append(upstreamNames, m.ModelName)
		}
	}

	// 2) 本地已有模型
	var locals []model.Model
	if len(upstreamNames) > 0 {
		_ = model.DB.Where("model_name IN ? AND sync_official <> 0", upstreamNames).Find(&locals).Error
	}

	// 本地 vendor 名称映射
	vendorIdSet := make(map[int]struct{})
	for _, m := range locals {
		if m.VendorID != 0 {
			vendorIdSet[m.VendorID] = struct{}{}
		}
	}
	vendorIDs := make([]int, 0, len(vendorIdSet))
	for id := range vendorIdSet {
		vendorIDs = append(vendorIDs, id)
	}
	idToVendorName := make(map[int]string)
	if len(vendorIDs) > 0 {
		var dbVendors []model.Vendor
		_ = model.DB.Where("id IN ?", vendorIDs).Find(&dbVendors).Error
		for _, v := range dbVendors {
			idToVendorName[v.Id] = v.Name
		}
	}

	// 3) 真正缺失或仅有待识别占位记录、且上游存在的精确同名模型。
	missingList, _ := model.GetMissingModels()
	enabledModels := model.GetEnabledModels()
	if len(enabledModels) > 0 {
		var pending []string
		if err := model.DB.Model(&model.Model{}).
			Where("metadata_status = ? AND metadata_source <> ? AND sync_official <> ? AND model_name IN ?",
				model.ModelMetadataStatusPending, model.ModelMetadataSourceManual, 0, enabledModels).
			Pluck("model_name", &pending).Error; err == nil {
			missingList = append(missingList, pending...)
		}
	}
	var missing []string
	missingSet := make(map[string]struct{}, len(missingList))
	for _, name := range missingList {
		if _, duplicate := missingSet[name]; duplicate {
			continue
		}
		missingSet[name] = struct{}{}
		if _, ok := modelByName[name]; ok {
			missing = append(missing, name)
		}
	}

	// 4) 计算冲突字段
	type conflictField struct {
		Field    string      `json:"field"`
		Local    interface{} `json:"local"`
		Upstream interface{} `json:"upstream"`
	}
	type conflictItem struct {
		ModelName string          `json:"model_name"`
		Fields    []conflictField `json:"fields"`
	}

	var conflicts []conflictItem
	for _, local := range locals {
		up, ok := modelByName[local.ModelName]
		if !ok {
			continue
		}
		fields := make([]conflictField, 0, 12)
		if strings.TrimSpace(local.Description) != strings.TrimSpace(up.Description) {
			fields = append(fields, conflictField{Field: "description", Local: local.Description, Upstream: up.Description})
		}
		if strings.TrimSpace(local.Icon) != strings.TrimSpace(up.Icon) {
			fields = append(fields, conflictField{Field: "icon", Local: local.Icon, Upstream: up.Icon})
		}
		if strings.TrimSpace(local.Tags) != strings.TrimSpace(up.Tags) {
			fields = append(fields, conflictField{Field: "tags", Local: local.Tags, Upstream: up.Tags})
		}
		upstreamCapabilities := inferUpstreamModelCategories(up)
		if (!local.HasStructuredMetadata() || up.StructuredMetadata) && !slices.Equal(model.NormalizeModelCategories(local.Capabilities), upstreamCapabilities) {
			fields = append(fields, conflictField{
				Field:    "capabilities",
				Local:    model.NormalizeModelCategories(local.Capabilities),
				Upstream: upstreamCapabilities,
			})
		}
		if up.StructuredMetadata && !slices.Equal(model.NormalizeModelCatalogValues(local.InputModalities), model.NormalizeModelCatalogValues(up.InputModalities)) {
			fields = append(fields, conflictField{Field: "input_modalities", Local: local.InputModalities, Upstream: up.InputModalities})
		}
		if up.StructuredMetadata && !slices.Equal(model.NormalizeModelCatalogValues(local.OutputModalities), model.NormalizeModelCatalogValues(up.OutputModalities)) {
			fields = append(fields, conflictField{Field: "output_modalities", Local: local.OutputModalities, Upstream: up.OutputModalities})
		}
		if up.StructuredMetadata && !slices.Equal(model.NormalizeModelCatalogValues(local.SupportedParameters), model.NormalizeModelCatalogValues(up.SupportedParameters)) {
			fields = append(fields, conflictField{Field: "supported_parameters", Local: local.SupportedParameters, Upstream: up.SupportedParameters})
		}
		if up.StructuredMetadata && local.ContextLength != up.ContextLength {
			fields = append(fields, conflictField{Field: "context_length", Local: local.ContextLength, Upstream: up.ContextLength})
		}
		if up.StructuredMetadata && local.MaxOutputTokens != up.MaxOutputTokens {
			fields = append(fields, conflictField{Field: "max_output_tokens", Local: local.MaxOutputTokens, Upstream: up.MaxOutputTokens})
		}
		// vendor 对比使用名称
		localVendor := idToVendorName[local.VendorID]
		if strings.TrimSpace(localVendor) != strings.TrimSpace(up.VendorName) {
			fields = append(fields, conflictField{Field: "vendor", Local: localVendor, Upstream: up.VendorName})
		}
		if local.NameRule != up.NameRule {
			fields = append(fields, conflictField{Field: "name_rule", Local: local.NameRule, Upstream: up.NameRule})
		}
		if local.Status != chooseStatus(up.Status, local.Status) {
			fields = append(fields, conflictField{Field: "status", Local: local.Status, Upstream: up.Status})
		}
		if len(fields) > 0 {
			conflicts = append(conflicts, conflictItem{ModelName: local.ModelName, Fields: fields})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"missing":   missing,
			"conflicts": conflicts,
			"source": gin.H{
				"locale":      locale,
				"models_url":  modelsURL,
				"vendors_url": vendorsURL,
				"all_url":     source["all_url"],
			},
		},
	})
}
