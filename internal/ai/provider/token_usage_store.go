package provider

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"

	repos "github.com/listen808/citebase/internal/repository"
)

type DBTokenUsageStore struct{}

func NewDBTokenUsageStore() *DBTokenUsageStore {
	return &DBTokenUsageStore{}
}

func (s *DBTokenUsageStore) LoadDayUsage(date string) (total int, byUser map[int]int, err error) {
	byUser = make(map[int]int)
	if repos.DB == nil {
		logx.Info("[TokenUsage] DB not available, token quota persistence disabled")
		return 0, byUser, nil
	}

	key := fmt.Sprintf("token_usage_%s", date)
	var val string
	row := repos.DB.Table("aw_config").Select("value").Where("`key` = ?", key).Row()
	if err := row.Scan(&val); err != nil {
		return 0, byUser, nil
	}
	total, _ = strconv.Atoi(val)

	var rows []struct {
		Key   string `gorm:"column:key"`
		Value string `gorm:"column:value"`
	}
	prefix := fmt.Sprintf("token_usage_%s_user_", date)
	repos.DB.Table("aw_config").Select("`key`, value").Where("`key` LIKE ?", prefix+"%").Find(&rows)
	for _, r := range rows {
		userKey := r.Key[len(prefix):]
		userID, _ := strconv.Atoi(userKey)
		usage, _ := strconv.Atoi(r.Value)
		if userID > 0 {
			byUser[userID] = usage
		}
	}
	return total, byUser, nil
}

func (s *DBTokenUsageStore) SaveTotalUsage(date string, total int) error {
	if repos.DB == nil {
		return nil
	}
	key := fmt.Sprintf("token_usage_%s", date)
	return upsertConfig(key, strconv.Itoa(total), fmt.Sprintf("Token用量 %s", date), "ai_token")
}

func (s *DBTokenUsageStore) SaveUserUsage(date string, userID int, usage int) error {
	if repos.DB == nil {
		return nil
	}
	key := fmt.Sprintf("token_usage_%s_user_%d", date, userID)
	return upsertConfig(key, strconv.Itoa(usage), fmt.Sprintf("Token用量 %s 用户%d", date, userID), "ai_token")
}

func upsertConfig(key, value, label, groupKey string) error {
	var count int64
	repos.DB.Table("aw_config").Where("`key` = ?", key).Count(&count)
	if count > 0 {
		return repos.DB.Table("aw_config").Where("`key` = ?", key).Updates(map[string]interface{}{
			"value": value,
		}).Error
	}
	return repos.DB.Table("aw_config").Create(map[string]interface{}{
		"`key`":       key,
		"value":       value,
		"label":       label,
		"group_key":   groupKey,
		"type":        "text",
		"create_time": 0,
		"update_time": 0,
	}).Error
}
