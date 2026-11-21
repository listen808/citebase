package ai

import (
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AIUsageDailyRepository 用量日聚合仓库
type AIUsageDailyRepository struct {
	db *gorm.DB
}

func NewAIUsageDailyRepository() *AIUsageDailyRepository {
	return &AIUsageDailyRepository{db: repos.DB}
}

// Increment 原子累加一条用量记录（按天/用户/模型/行为一行，不存在则插入）。
// 用 ON DUPLICATE KEY UPDATE 保证并发安全，避免读-改-写竞态。
func (r *AIUsageDailyRepository) Increment(userID int, provider, model, action string, tokens int, durationMs int64, failed bool) error {
	if repos.DB == nil {
		return nil // 启动早期或测试环境没有 DB，静默跳过
	}
	today := time.Now().Truncate(24 * time.Hour)
	row := aimodel.AIUsageDaily{
		Date:       today,
		UserID:     userID,
		Provider:   provider,
		Model:      model,
		Action:     action,
		DurationMs: durationMs,
	}
	if failed {
		row.FailedCount = 1
	} else {
		row.RequestCount = 1
		row.TokenUsage = tokens
	}

	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "date"}, {Name: "user_id"}, {Name: "provider"}, {Name: "model"}, {Name: "action"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"request_count": gorm.Expr("request_count + ?", row.RequestCount),
			"token_usage":   gorm.Expr("token_usage + ?", row.TokenUsage),
			"duration_ms":   gorm.Expr("duration_ms + ?", row.DurationMs),
			"failed_count":  gorm.Expr("failed_count + ?", row.FailedCount),
			"updated_at":    time.Now(),
		}),
	}).Create(&row).Error
}

// SumByUser 某用户当日累计 token（供配额拦截使用）
func (r *AIUsageDailyRepository) SumByUser(userID int, day time.Time) (int64, error) {
	var total int64
	err := r.db.Model(&aimodel.AIUsageDaily{}).
		Where("user_id = ? AND date = ?", userID, day.Truncate(24*time.Hour)).
		Select("COALESCE(SUM(token_usage),0)").
		Scan(&total).Error
	return total, err
}
