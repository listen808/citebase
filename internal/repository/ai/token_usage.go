package ai

import (
	"fmt"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

// TokenUsageRepository Token使用量持久化（基于aw_config表）
type TokenUsageRepository struct {
	db *gorm.DB
}

func NewTokenUsageRepository() *TokenUsageRepository {
	return &TokenUsageRepository{db: repos.DB}
}

// configKey前缀
const (
	tokenUsageTotalPrefix = "ai_token_usage:"      // ai_token_usage:2026-05-19 → total值
	tokenUsageUserPrefix  = "ai_token_usage:user:" //nolint:gosec // G101: cache key prefix, not a credential
)

// LoadDayUsage 加载指定日期的总使用量和每用户使用量
func (r *TokenUsageRepository) LoadDayUsage(date string) (total int, byUser map[int]int, err error) {
	byUser = make(map[int]int)

	totalKey := tokenUsageTotalPrefix + date
	val, err := r.getValue(totalKey)
	if err != nil {
		return 0, byUser, err
	}
	if val != "" {
		total, _ = strconv.Atoi(val)
	}

	userPrefix := fmt.Sprintf("%s%s:%%", tokenUsageUserPrefix, date)
	var configs []adminmodel.Config
	if err := r.db.Where("`key` LIKE ?", userPrefix).Find(&configs).Error; err != nil {
		return total, byUser, err
	}
	for _, cfg := range configs {
		parts := strings.Split(cfg.Key, ":")
		if len(parts) >= 4 {
			uid, _ := strconv.Atoi(parts[3])
			usage, _ := strconv.Atoi(cfg.Value)
			if uid > 0 {
				byUser[uid] = usage
			}
		}
	}
	return total, byUser, nil
}

// SaveTotalUsage 保存指定日期的总使用量（upsert）
func (r *TokenUsageRepository) SaveTotalUsage(date string, total int) error {
	key := tokenUsageTotalPrefix + date
	value := strconv.Itoa(total)
	return r.upsertConfig(key, value, "AI Token日使用量", "ai_token_usage")
}

// SaveUserUsage 保存指定日期某用户的使用量（upsert）
func (r *TokenUsageRepository) SaveUserUsage(date string, userID int, usage int) error {
	key := fmt.Sprintf("%s%s:%d", tokenUsageUserPrefix, date, userID)
	value := strconv.Itoa(usage)
	label := fmt.Sprintf("AI Token用户使用量(用户%d)", userID)
	return r.upsertConfig(key, value, label, "ai_token_usage")
}

func (r *TokenUsageRepository) getValue(key string) (string, error) {
	var cfg adminmodel.Config
	err := r.db.Select("value").Where("`key` = ?", key).First(&cfg).Error
	if err != nil {
		if strings.Contains(err.Error(), "record not found") {
			return "", nil
		}
		return "", err
	}
	return cfg.Value, nil
}

func (r *TokenUsageRepository) upsertConfig(key, value, label, groupKey string) error {
	var count int64
	r.db.Model(&adminmodel.Config{}).Where("`key` = ?", key).Count(&count)
	now := int(time.Now().Unix())
	if count > 0 {
		return r.db.Model(&adminmodel.Config{}).Where("`key` = ?", key).Updates(map[string]interface{}{
			"value":       value,
			"update_time": now,
		}).Error
	}
	return r.db.Create(&adminmodel.Config{
		Key:        key,
		Value:      value,
		Label:      label,
		GroupKey:   groupKey,
		Type:       "text",
		IsPrivate:  1,
		CreateTime: now,
		UpdateTime: now,
	}).Error
}
