package model

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

const (
	ModelAliasStatusActive  = "active"
	ModelAliasStatusRetired = "retired"
)

type ModelAlias struct {
	Id                 int    `json:"id"`
	AliasName          string `json:"alias_name" gorm:"size:255;not null;uniqueIndex"`
	CanonicalModelName string `json:"canonical_model_name" gorm:"size:255;not null;index"`
	Status             string `json:"status" gorm:"type:varchar(16);not null;index"`
	RetireAfter        int64  `json:"retire_after" gorm:"bigint;index"`
	Source             string `json:"source" gorm:"type:varchar(32);not null"`
	LastUsedTime       int64  `json:"last_used_time,omitempty" gorm:"bigint"`
	RequestCount       int64  `json:"request_count" gorm:"bigint"`
	CreatedTime        int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime        int64  `json:"updated_time" gorm:"bigint"`
}

type aliasUsage struct {
	Count    int64
	LastUsed int64
}

var (
	modelAliasLock  sync.RWMutex
	modelAliasCache = make(map[string]ModelAlias)
	aliasUsageLock  sync.Mutex
	aliasUsageMap   = make(map[string]aliasUsage)
	aliasFlushOnce  sync.Once
)

func InitModelAliasCache() error {
	if DB == nil || !DB.Migrator().HasTable(&ModelAlias{}) {
		modelAliasLock.Lock()
		modelAliasCache = make(map[string]ModelAlias)
		modelAliasLock.Unlock()
		return nil
	}
	var aliases []ModelAlias
	if err := DB.Where("status = ?", ModelAliasStatusActive).Find(&aliases).Error; err != nil {
		return err
	}
	next := make(map[string]ModelAlias, len(aliases))
	for _, alias := range aliases {
		next[alias.AliasName] = alias
	}
	modelAliasLock.Lock()
	modelAliasCache = next
	modelAliasLock.Unlock()
	aliasFlushOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				FlushModelAliasUsage()
			}
		}()
	})
	return nil
}

func SyncModelAliasCache(frequency int) {
	if frequency <= 0 {
		frequency = 60
	}
	ticker := time.NewTicker(time.Duration(frequency) * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if err := InitModelAliasCache(); err != nil {
			common.SysError("failed to sync model aliases: " + err.Error())
		}
	}
}

func ResolveModelAlias(name string) (string, *ModelAlias) {
	modelAliasLock.RLock()
	alias, ok := modelAliasCache[name]
	modelAliasLock.RUnlock()
	if !ok {
		return name, nil
	}
	copy := alias
	return alias.CanonicalModelName, &copy
}

func GetActiveModelAliases() []ModelAlias {
	modelAliasLock.RLock()
	aliases := make([]ModelAlias, 0, len(modelAliasCache))
	for _, alias := range modelAliasCache {
		aliases = append(aliases, alias)
	}
	modelAliasLock.RUnlock()
	mergePendingAliasUsage(aliases)
	return aliases
}

func GetActiveAliasNamesForCanonical(canonicalModelName string) []string {
	modelAliasLock.RLock()
	aliases := make([]string, 0)
	for _, alias := range modelAliasCache {
		if alias.CanonicalModelName == canonicalModelName {
			aliases = append(aliases, alias.AliasName)
		}
	}
	modelAliasLock.RUnlock()
	return aliases
}

func ModelHasActiveAliases(canonicalModelName string) (bool, error) {
	if DB == nil || !DB.Migrator().HasTable(&ModelAlias{}) {
		return false, nil
	}
	var count int64
	err := DB.Model(&ModelAlias{}).
		Where("canonical_model_name = ? AND status = ?", canonicalModelName, ModelAliasStatusActive).
		Count(&count).Error
	return count > 0, err
}

func ModelNameConflictsWithAlias(tx *gorm.DB, modelName string) (bool, error) {
	if tx == nil {
		tx = DB
	}
	modelName = strings.TrimSpace(modelName)
	if tx == nil || modelName == "" || !tx.Migrator().HasTable(&ModelAlias{}) {
		return false, nil
	}
	var count int64
	if err := tx.Model(&ModelAlias{}).Where("alias_name = ?", modelName).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func MarkModelAliasUsed(aliasName string) {
	if aliasName == "" {
		return
	}
	now := common.GetTimestamp()
	aliasUsageLock.Lock()
	usage := aliasUsageMap[aliasName]
	usage.Count++
	usage.LastUsed = now
	aliasUsageMap[aliasName] = usage
	aliasUsageLock.Unlock()
}

func FlushModelAliasUsage() {
	if DB == nil {
		return
	}
	aliasUsageLock.Lock()
	pending := aliasUsageMap
	aliasUsageMap = make(map[string]aliasUsage)
	aliasUsageLock.Unlock()
	for aliasName, usage := range pending {
		result := DB.Model(&ModelAlias{}).Where("alias_name = ?", aliasName).Updates(map[string]interface{}{
			"last_used_time": usage.LastUsed,
			"request_count":  gorm.Expr("request_count + ?", usage.Count),
			"updated_time":   common.GetTimestamp(),
		})
		if result.Error != nil {
			aliasUsageLock.Lock()
			current := aliasUsageMap[aliasName]
			current.Count += usage.Count
			if usage.LastUsed > current.LastUsed {
				current.LastUsed = usage.LastUsed
			}
			aliasUsageMap[aliasName] = current
			aliasUsageLock.Unlock()
		}
	}
}

func mergePendingAliasUsage(aliases []ModelAlias) {
	aliasUsageLock.Lock()
	defer aliasUsageLock.Unlock()
	for i := range aliases {
		usage := aliasUsageMap[aliases[i].AliasName]
		aliases[i].RequestCount += usage.Count
		if usage.LastUsed > aliases[i].LastUsedTime {
			aliases[i].LastUsedTime = usage.LastUsed
		}
	}
}

func ValidateModelAlias(tx *gorm.DB, aliasName string, canonicalModelName string) error {
	if tx == nil {
		tx = DB
	}
	aliasName = strings.TrimSpace(aliasName)
	canonicalModelName = strings.TrimSpace(canonicalModelName)
	if aliasName == "" || canonicalModelName == "" || aliasName == canonicalModelName {
		return fmt.Errorf("invalid model alias")
	}
	var canonicalCount int64
	if err := tx.Model(&Model{}).Where("model_name = ?", canonicalModelName).Count(&canonicalCount).Error; err != nil {
		return err
	}
	if canonicalCount != 1 {
		return fmt.Errorf("canonical model %q does not exist", canonicalModelName)
	}
	var standardCount int64
	if err := tx.Model(&Model{}).Where("model_name = ?", aliasName).Count(&standardCount).Error; err != nil {
		return err
	}
	if standardCount > 0 {
		return fmt.Errorf("alias %q conflicts with an existing model", aliasName)
	}
	var targetAlias ModelAlias
	if err := tx.Where("alias_name = ?", canonicalModelName).First(&targetAlias).Error; err == nil {
		return fmt.Errorf("alias chains are not allowed: %q is already an alias", canonicalModelName)
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	var existing ModelAlias
	if err := tx.Where("alias_name = ?", aliasName).First(&existing).Error; err == nil {
		if existing.CanonicalModelName != canonicalModelName {
			return fmt.Errorf("alias %q is already assigned to %q", aliasName, existing.CanonicalModelName)
		}
		return nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	return nil
}

func CreateOrActivateModelAlias(tx *gorm.DB, aliasName string, canonicalModelName string, source string, retireAfter int64) error {
	if tx == nil {
		tx = DB
	}
	aliasName = strings.TrimSpace(aliasName)
	canonicalModelName = strings.TrimSpace(canonicalModelName)
	source = strings.TrimSpace(source)
	if source == "" || retireAfter <= 0 {
		return fmt.Errorf("model alias source and retirement date are required")
	}
	switch source {
	case ModelMetadataSourceManual,
		ModelMetadataSourceBaseLLMExact,
		ModelMetadataSourceBaseLLMNormalized,
		ModelMetadataSourceChannel,
		ModelMetadataSourceMigration:
	default:
		return fmt.Errorf("unsupported model alias source %q", source)
	}
	if err := ValidateModelAlias(tx, aliasName, canonicalModelName); err != nil {
		return err
	}
	now := common.GetTimestamp()
	var existing ModelAlias
	err := tx.Where("alias_name = ?", aliasName).First(&existing).Error
	if err == nil {
		return tx.Model(&ModelAlias{}).Where("id = ?", existing.Id).Updates(map[string]interface{}{
			"status":       ModelAliasStatusActive,
			"retire_after": retireAfter,
			"source":       source,
			"updated_time": now,
		}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return tx.Create(&ModelAlias{
		AliasName:          aliasName,
		CanonicalModelName: canonicalModelName,
		Status:             ModelAliasStatusActive,
		RetireAfter:        retireAfter,
		Source:             source,
		CreatedTime:        now,
		UpdatedTime:        now,
	}).Error
}

func ListModelAliases(status string) ([]ModelAlias, error) {
	FlushModelAliasUsage()
	query := DB.Model(&ModelAlias{})
	switch status = strings.ToLower(strings.TrimSpace(status)); status {
	case "expired", "due":
		query = query.Where("status = ? AND retire_after > ? AND retire_after <= ?", ModelAliasStatusActive, 0, common.GetTimestamp())
	case "", "all":
	default:
		query = query.Where("status = ?", status)
	}
	var aliases []ModelAlias
	err := query.Order("id DESC").Find(&aliases).Error
	return aliases, err
}

func RetireModelAliases(aliasNames []string) (int, int, error) {
	aliasNames = normalizeLookupValues(aliasNames)
	if len(aliasNames) == 0 {
		return 0, 0, nil
	}
	migratedTokens := 0
	retiredAliases := 0
	migratedTokenKeys := make([]string, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var aliases []ModelAlias
		if err := lockForUpdate(tx).Where("alias_name IN ? AND status = ?", aliasNames, ModelAliasStatusActive).Find(&aliases).Error; err != nil {
			return err
		}
		canonicalByAlias := make(map[string]string, len(aliases))
		for _, alias := range aliases {
			canonicalByAlias[alias.AliasName] = alias.CanonicalModelName
		}
		if len(canonicalByAlias) == 0 {
			return nil
		}
		var tokens []Token
		if err := lockForUpdate(tx).Where("model_limits_enabled = ?", true).Find(&tokens).Error; err != nil {
			return err
		}
		for i := range tokens {
			limits := normalizeLookupValues(strings.Split(tokens[i].ModelLimits, ","))
			changed := false
			for j, limit := range limits {
				if canonical, ok := canonicalByAlias[limit]; ok {
					limits[j] = canonical
					changed = true
				}
			}
			if !changed {
				continue
			}
			limits = normalizeLookupValues(limits)
			if err := invalidateTokenCacheForMutation(tokens[i].Key); err != nil {
				common.SysLog("failed to invalidate token cache before alias retirement: " + err.Error())
			}
			if err := tx.Model(&Token{}).Where("id = ?", tokens[i].Id).Update("model_limits", strings.Join(limits, ",")).Error; err != nil {
				return err
			}
			migratedTokenKeys = append(migratedTokenKeys, tokens[i].Key)
			migratedTokens++
		}
		result := tx.Model(&ModelAlias{}).Where("alias_name IN ? AND status = ?", aliasNames, ModelAliasStatusActive).Updates(map[string]interface{}{
			"status":       ModelAliasStatusRetired,
			"updated_time": common.GetTimestamp(),
		})
		if result.Error != nil {
			return result.Error
		}
		retiredAliases = int(result.RowsAffected)
		return nil
	})
	if err != nil {
		return 0, 0, err
	}
	// Raise a fresh fence after commit and remove any stale snapshot that a
	// concurrent reader may have republished while a long-running migration was
	// still seeing the pre-commit database state.
	for _, key := range migratedTokenKeys {
		if err := invalidateTokenCacheForMutation(key); err != nil {
			common.SysLog("failed to invalidate token cache after alias retirement: " + err.Error())
		}
	}
	modelAliasLock.Lock()
	for _, aliasName := range aliasNames {
		delete(modelAliasCache, aliasName)
	}
	modelAliasLock.Unlock()
	if err := InitModelAliasCache(); err != nil {
		common.SysError("failed to refresh model alias cache after retirement: " + err.Error())
	}
	return migratedTokens, retiredAliases, nil
}
