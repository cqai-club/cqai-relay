package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	NameRuleExact = iota
	NameRulePrefix
	NameRuleContains
	NameRuleSuffix
)

const (
	ModelMetadataStatusPending   = "pending"
	ModelMetadataStatusConfirmed = "confirmed"

	ModelMetadataSourceManual            = "manual"
	ModelMetadataSourceBaseLLMExact      = "basellm_exact"
	ModelMetadataSourceBaseLLMNormalized = "basellm_normalized"
	ModelMetadataSourceChannel           = "channel"
	ModelMetadataSourceMigration         = "migration"
)

type BoundChannel struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type Model struct {
	Id                  int                   `json:"id"`
	ModelName           string                `json:"model_name" gorm:"size:128;not null;uniqueIndex:uk_model_name_delete_at,priority:1"`
	Description         string                `json:"description,omitempty" gorm:"type:text"`
	Icon                string                `json:"icon,omitempty" gorm:"type:varchar(128)"`
	Tags                string                `json:"tags,omitempty" gorm:"type:varchar(255)"`
	VendorID            int                   `json:"vendor_id,omitempty" gorm:"index"`
	Endpoints           string                `json:"endpoints,omitempty" gorm:"type:text"`
	Capabilities        []types.ModelCategory `json:"capabilities,omitempty" gorm:"serializer:json;type:text"`
	InputModalities     []string              `json:"input_modalities" gorm:"serializer:json;type:text"`
	OutputModalities    []string              `json:"output_modalities" gorm:"serializer:json;type:text"`
	SupportedParameters []string              `json:"supported_parameters" gorm:"serializer:json;type:text"`
	ContextLength       int64                 `json:"context_length" gorm:"type:bigint"`
	MaxOutputTokens     int64                 `json:"max_output_tokens" gorm:"type:bigint"`
	MetadataStatus      string                `json:"metadata_status" gorm:"type:varchar(16);index"`
	MetadataSource      string                `json:"metadata_source" gorm:"type:varchar(32);index"`
	Status              int                   `json:"status" gorm:"default:1"`
	SyncOfficial        int                   `json:"sync_official" gorm:"default:1"`
	CreatedTime         int64                 `json:"created_time" gorm:"bigint"`
	UpdatedTime         int64                 `json:"updated_time" gorm:"bigint"`
	DeletedAt           gorm.DeletedAt        `json:"-" gorm:"index;uniqueIndex:uk_model_name_delete_at,priority:2"`

	BoundChannels []BoundChannel `json:"bound_channels,omitempty" gorm:"-"`
	EnableGroups  []string       `json:"enable_groups,omitempty" gorm:"-"`
	QuotaTypes    []int          `json:"quota_types,omitempty" gorm:"-"`
	NameRule      int            `json:"name_rule" gorm:"default:0"`

	MatchedModels []string `json:"matched_models,omitempty" gorm:"-"`
	MatchedCount  int      `json:"matched_count,omitempty" gorm:"-"`
}

func (mi *Model) Insert() error {
	if conflict, err := ModelNameConflictsWithAlias(DB, mi.ModelName); err != nil {
		return err
	} else if conflict {
		return fmt.Errorf("model name %q is reserved by a model alias", mi.ModelName)
	}
	now := common.GetTimestamp()
	mi.CreatedTime = now
	mi.UpdatedTime = now
	mi.NormalizeStructuredMetadata()
	if mi.MetadataStatus == "" {
		if len(NormalizeModelCategories(mi.Capabilities)) > 0 {
			mi.MetadataStatus = ModelMetadataStatusConfirmed
		} else {
			mi.MetadataStatus = ModelMetadataStatusPending
		}
	}
	if mi.MetadataSource == "" {
		mi.MetadataSource = ModelMetadataSourceManual
	}

	// 保存原始值（因为 Create 后可能被 GORM 的 default 标签覆盖为 1）
	originalStatus := mi.Status
	originalSyncOfficial := mi.SyncOfficial

	// 先创建记录（GORM 会对零值字段应用默认值）
	if err := DB.Create(mi).Error; err != nil {
		return err
	}

	// 使用保存的原始值进行更新，确保零值能正确保存
	return DB.Model(&Model{}).Where("id = ?", mi.Id).Updates(map[string]interface{}{
		"status":          originalStatus,
		"sync_official":   originalSyncOfficial,
		"metadata_status": mi.MetadataStatus,
		"metadata_source": mi.MetadataSource,
	}).Error
}

// EndpointTypes decodes either the legacy endpoint array or the custom
// endpoint object into the route types used for category derivation.
func (mi Model) EndpointTypes() []types.EndpointType {
	if strings.TrimSpace(mi.Endpoints) == "" {
		return nil
	}
	var endpointTypes []types.EndpointType
	if err := common.UnmarshalJsonStr(mi.Endpoints, &endpointTypes); err == nil {
		return endpointTypes
	}
	var endpoints map[string]interface{}
	if err := common.UnmarshalJsonStr(mi.Endpoints, &endpoints); err != nil {
		return nil
	}
	endpointTypes = make([]types.EndpointType, 0, len(endpoints))
	for endpoint := range endpoints {
		endpointTypes = append(endpointTypes, types.EndpointType(endpoint))
	}
	return endpointTypes
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	var cnt int64
	err := DB.Model(&Model{}).Where("model_name = ? AND id <> ?", name, id).Count(&cnt).Error
	return cnt > 0, err
}

func (mi *Model) Update() error {
	mi.NormalizeStructuredMetadata()
	var existing Model
	if err := DB.Select("id", "model_name").First(&existing, mi.Id).Error; err != nil {
		return err
	}
	if existing.ModelName != mi.ModelName {
		if hasAliases, err := ModelHasActiveAliases(existing.ModelName); err != nil {
			return err
		} else if hasAliases {
			return fmt.Errorf("model %q has active aliases", existing.ModelName)
		}
		if conflict, err := ModelNameConflictsWithAlias(DB, mi.ModelName); err != nil {
			return err
		} else if conflict {
			return fmt.Errorf("model name %q is reserved by a model alias", mi.ModelName)
		}
	}
	mi.UpdatedTime = common.GetTimestamp()
	// 使用 Select 强制更新所有字段，包括零值
	return DB.Model(&Model{}).Where("id = ?", mi.Id).
		Select("model_name", "description", "icon", "tags", "vendor_id", "endpoints", "capabilities", "input_modalities", "output_modalities", "supported_parameters", "context_length", "max_output_tokens", "metadata_status", "metadata_source", "status", "sync_official", "name_rule", "updated_time").
		Updates(mi).Error
}

func (mi *Model) Delete() error {
	return DB.Delete(mi).Error
}

func GetVendorModelCounts() (map[int64]int64, error) {
	var stats []struct {
		VendorID int64
		Count    int64
	}
	if err := DB.Model(&Model{}).
		Select("vendor_id as vendor_id, count(*) as count").
		Group("vendor_id").
		Scan(&stats).Error; err != nil {
		return nil, err
	}
	m := make(map[int64]int64, len(stats))
	for _, s := range stats {
		m[s.VendorID] = s.Count
	}
	return m, nil
}

func GetAllModels(offset int, limit int) ([]*Model, error) {
	models, _, err := SearchModels("", "", "", "", "", "", offset, limit)
	return models, err
}

func GetBoundChannelsByModelsMap(modelNames []string) (map[string][]BoundChannel, error) {
	result := make(map[string][]BoundChannel)
	if len(modelNames) == 0 {
		return result, nil
	}
	type row struct {
		Model string
		Name  string
		Type  int
	}
	var rows []row
	err := DB.Table("channels").
		Select("abilities.model as model, channels.name as name, channels.type as type").
		Joins("JOIN abilities ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ?", modelNames, true).
		Distinct().
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		result[r.Model] = append(result[r.Model], BoundChannel{Name: r.Name, Type: r.Type})
	}
	return result, nil
}

func normalizeLookupValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func GetPreferredModelOwnerChannelTypes(modelNames []string, groups []string) (map[string]int, error) {
	result := make(map[string]int)
	modelNames = normalizeLookupValues(modelNames)
	if len(modelNames) == 0 {
		return result, nil
	}

	type row struct {
		Model       string
		ChannelType int
	}
	var rows []row

	query := DB.Table("abilities").
		Select("abilities.model as model, channels.type as channel_type").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.model IN ? AND abilities.enabled = ? AND channels.status = ?", modelNames, true, common.ChannelStatusEnabled).
		Order("COALESCE(abilities.priority, 0) DESC").
		Order("abilities.weight DESC").
		Order("abilities.channel_id ASC")

	groups = normalizeLookupValues(groups)
	if len(groups) > 0 {
		query = query.Where("abilities."+commonGroupCol+" IN ?", groups)
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, r := range rows {
		if _, ok := result[r.Model]; ok {
			continue
		}
		result[r.Model] = r.ChannelType
	}
	return result, nil
}

func SearchModels(keyword string, vendor string, status string, syncOfficial string, metadataStatus string, metadataSource string, offset int, limit int) ([]*Model, int64, error) {
	var models []*Model
	db := DB.Model(&Model{})
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("model_name LIKE ? OR description LIKE ? OR tags LIKE ?", like, like, like)
	}
	if vendor != "" {
		if vid, err := strconv.Atoi(vendor); err == nil {
			db = db.Where("models.vendor_id = ?", vid)
		} else {
			db = db.Joins("JOIN vendors ON vendors.id = models.vendor_id").Where("vendors.name LIKE ?", "%"+vendor+"%")
		}
	}
	if statusValue, ok := parseModelStatusFilter(status); ok {
		db = db.Where("models.status = ?", statusValue)
	}
	if syncValue, ok := parseModelSyncFilter(syncOfficial); ok {
		db = db.Where("models.sync_official = ?", syncValue)
	}
	if metadataStatus = strings.TrimSpace(metadataStatus); metadataStatus != "" && metadataStatus != "all" {
		db = db.Where("models.metadata_status = ?", metadataStatus)
	}
	if metadataSource = strings.TrimSpace(metadataSource); metadataSource != "" && metadataSource != "all" {
		db = db.Where("models.metadata_source = ?", metadataSource)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := db.Order("models.id DESC").Offset(offset).Limit(limit).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	return models, total, nil
}

// InitializeModelMetadata classifies legacy rows after the new metadata columns
// are added. Values are written explicitly so all supported databases behave the
// same way without relying on database defaults.
func InitializeModelMetadata() error {
	var models []*Model
	if err := DB.Where("metadata_status = ? OR metadata_status IS NULL OR metadata_source = ? OR metadata_source IS NULL", "", "").Find(&models).Error; err != nil {
		return err
	}
	for _, item := range models {
		status := item.MetadataStatus
		if status == "" {
			status = ModelMetadataStatusPending
			if len(NormalizeModelCategories(item.Capabilities)) > 0 {
				status = ModelMetadataStatusConfirmed
			}
		}
		source := item.MetadataSource
		if source == "" {
			source = ModelMetadataSourceMigration
		}
		if err := DB.Model(&Model{}).Where("id = ?", item.Id).Updates(map[string]interface{}{
			"metadata_status": status,
			"metadata_source": source,
		}).Error; err != nil {
			return err
		}
	}
	var abilityModels []string
	if err := DB.Model(&Ability{}).Distinct("model").Pluck("model", &abilityModels).Error; err != nil {
		return err
	}
	return EnsureModelMetadataRecords(DB, abilityModels, ModelMetadataSourceChannel)
}

// EnsureModelMetadataRecords creates pending metadata placeholders for channel
// models. It deliberately performs no network access and is safe to call in the
// same transaction that updates channel abilities.
func EnsureModelMetadataRecords(tx *gorm.DB, modelNames []string, source string) error {
	if tx == nil {
		tx = DB
	}
	// Some narrowly scoped tests and maintenance tools create only the channel
	// tables. A missing metadata table cannot be populated, but it must not make
	// otherwise independent channel mutations fail.
	if !tx.Migrator().HasTable(&Model{}) {
		return nil
	}
	modelNames = normalizeLookupValues(modelNames)
	if len(modelNames) == 0 {
		return nil
	}
	var existing []string
	if err := tx.Model(&Model{}).Where("model_name IN ?", modelNames).Pluck("model_name", &existing).Error; err != nil {
		return err
	}
	exists := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		exists[name] = struct{}{}
	}
	if tx.Migrator().HasTable(&ModelAlias{}) {
		var reservedAliases []string
		if err := tx.Model(&ModelAlias{}).Where("alias_name IN ?", modelNames).Pluck("alias_name", &reservedAliases).Error; err != nil {
			return err
		}
		for _, name := range reservedAliases {
			exists[name] = struct{}{}
		}
	}
	now := common.GetTimestamp()
	for _, name := range modelNames {
		if _, ok := exists[name]; ok {
			continue
		}
		item := &Model{
			ModelName:      name,
			Capabilities:   []types.ModelCategory{types.ModelCategoryOther},
			MetadataStatus: ModelMetadataStatusPending,
			MetadataSource: source,
			Status:         1,
			SyncOfficial:   1,
			NameRule:       NameRuleExact,
			CreatedTime:    now,
			UpdatedTime:    now,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(item).Error; err != nil {
			return err
		}
	}
	return nil
}

// parseModelStatusFilter maps UI/API status values to the models.status column.
// Returns ok=false when no status filter should be applied.
func parseModelStatusFilter(status string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return 0, false
	case "enabled", "1":
		return 1, true
	case "disabled", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(status)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}

// parseModelSyncFilter maps UI/API sync values to the models.sync_official column.
// Returns ok=false when no sync filter should be applied.
func parseModelSyncFilter(syncOfficial string) (value int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(syncOfficial)) {
	case "", "all":
		return 0, false
	case "yes", "1":
		return 1, true
	case "no", "0":
		return 0, true
	default:
		n, err := strconv.Atoi(syncOfficial)
		if err != nil {
			return 0, false
		}
		return n, true
	}
}
