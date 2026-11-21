package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AIAuditLogRepository struct {
	db *gorm.DB
}

func NewAIAuditLogRepository() *AIAuditLogRepository {
	return &AIAuditLogRepository{db: repos.DB}
}

func (r *AIAuditLogRepository) Create(log *aimodel.AIAuditLog) error {
	return r.db.Create(log).Error
}

func (r *AIAuditLogRepository) FindByUserID(userID int, limit, offset int) ([]*aimodel.AIAuditLog, int64, error) {
	var logs []*aimodel.AIAuditLog
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// FindByUserIDAndAction 按用户+动作过滤的分页查询。
// 旧 service 层对「非管理员+指定 action」先取一页再内存过滤，total 仍是全部动作的计数，
// 导致每页条数远小于 limit、翻页出现空洞。action 必须下推为 SQL 条件。
func (r *AIAuditLogRepository) FindByUserIDAndAction(userID int, action string, limit, offset int) ([]*aimodel.AIAuditLog, int64, error) {
	var logs []*aimodel.AIAuditLog
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{}).Where("user_id = ? AND action = ?", userID, action)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

// FindByID 按主键取单条审计日志（旧调用方把主键误当 offset 传给 FindByAction）。
func (r *AIAuditLogRepository) FindByID(id int) (*aimodel.AIAuditLog, error) {
	var log aimodel.AIAuditLog
	if err := r.db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func (r *AIAuditLogRepository) FindByAction(action string, limit, offset int) ([]*aimodel.AIAuditLog, int64, error) {
	var logs []*aimodel.AIAuditLog
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{}).Where("action = ?", action)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func (r *AIAuditLogRepository) FindRecent(limit int) ([]*aimodel.AIAuditLog, error) {
	var logs []*aimodel.AIAuditLog
	err := r.db.Order("created_at DESC").Limit(limit).Find(&logs).Error
	return logs, err
}

func (r *AIAuditLogRepository) FindAll(limit, offset int) ([]*aimodel.AIAuditLog, int64, error) {
	var logs []*aimodel.AIAuditLog
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&logs).Error
	return logs, total, err
}

func (r *AIAuditLogRepository) CountByResult(result string) (int64, error) {
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{})
	// result 为空表示「不按结果过滤、统计全部事件」。旧实现把 "" 原样拼进
	// WHERE result = ''，而该列默认值是 'success'，导致统计恒为 0。
	if result != "" {
		query = query.Where("result = ?", result)
	}
	err := query.Count(&total).Error
	return total, err
}

// CountByResultWithinDays 统计最近 days 天内指定 result 的事件数（result 为空统计全部）。
// 合规报告的 PeriodDays 标注要求所有总量口径使用同一时间窗。
func (r *AIAuditLogRepository) CountByResultWithinDays(result string, days int) (int64, error) {
	var total int64
	query := r.db.Model(&aimodel.AIAuditLog{}).
		Where("created_at >= NOW() - INTERVAL ? DAY", days)
	if result != "" {
		query = query.Where("result = ?", result)
	}
	err := query.Count(&total).Error
	return total, err
}

// ActionCountStat 单个 action 的聚合统计（SQL GROUP BY，避免整表进内存）。
type ActionCountStat struct {
	Action       string `gorm:"column:action" json:"action"`
	Count        int64  `gorm:"column:cnt" json:"count"`
	SuccessCount int64  `gorm:"column:success_cnt" json:"success_count"`
	FailCount    int64  `gorm:"column:fail_cnt" json:"fail_count"`
}

// ActionStatsWithinDays 按 action 分组聚合最近 days 天的成功/失败次数。
func (r *AIAuditLogRepository) ActionStatsWithinDays(days int) ([]ActionCountStat, error) {
	var results []ActionCountStat
	err := r.db.Model(&aimodel.AIAuditLog{}).
		Select(`action,
			COUNT(*) as cnt,
			SUM(CASE WHEN result = 'success' THEN 1 ELSE 0 END) as success_cnt,
			SUM(CASE WHEN result = 'success' THEN 0 ELSE 1 END) as fail_cnt`).
		Where("created_at >= NOW() - INTERVAL ? DAY", days).
		Group("action").
		Order("cnt DESC").
		Scan(&results).Error
	return results, err
}

func (r *AIAuditLogRepository) DeleteOlderThan(days int) error {
	return r.db.Where("created_at < NOW() - INTERVAL ? DAY", days).Delete(&aimodel.AIAuditLog{}).Error
}

// UsageByModel 按模型分组的使用量统计
type ModelUsageStat struct {
	Model        string  `gorm:"column:model" json:"model"`
	Provider     string  `gorm:"column:provider" json:"provider"`
	TotalUsage   int64   `gorm:"column:total_usage" json:"total_usage"`
	RequestCount int64   `gorm:"column:request_count" json:"request_count"`
	AvgUsage     float64 `gorm:"column:avg_usage" json:"avg_usage"`
}

func (r *AIAuditLogRepository) UsageByModel(days int, limit int) ([]ModelUsageStat, error) {
	var results []ModelUsageStat
	err := r.db.Model(&aimodel.AIAuditLog{}).
		Select(`COALESCE(NULLIF(model, ''), 'unknown') as model,
			COALESCE(NULLIF(provider, ''), 'unknown') as provider,
			SUM(token_usage) as total_usage,
			COUNT(*) as request_count,
			CASE WHEN COUNT(*) > 0 THEN SUM(token_usage) / COUNT(*) ELSE 0 END as avg_usage`).
		Where("created_at >= NOW() - INTERVAL ? DAY AND token_usage > 0", days).
		Group("model, provider").
		Order("total_usage DESC").
		Limit(limit).
		Find(&results).Error
	return results, err
}

// UsageByUser 按用户分组的使用量统计
type UserUsageStat struct {
	UserID       int     `gorm:"column:user_id" json:"user_id"`
	Username     string  `gorm:"column:username" json:"username"`
	GivenName    string  `gorm:"column:given_name" json:"given_name"`
	TotalUsage   int64   `gorm:"column:total_usage" json:"total_usage"`
	RequestCount int64   `gorm:"column:request_count" json:"request_count"`
	AvgUsage     float64 `gorm:"column:avg_usage" json:"avg_usage"`
	LastUsed     string  `gorm:"column:last_used" json:"last_used"`
}

func (r *AIAuditLogRepository) UsageByUser(days int, limit int) ([]UserUsageStat, error) {
	var results []UserUsageStat
	err := r.db.Raw(`
		SELECT a.user_id,
			COALESCE(u.username, CAST(a.user_id AS CHAR)) as username,
			COALESCE(u.given_name, '') as given_name,
			SUM(a.token_usage) as total_usage,
			COUNT(*) as request_count,
			CASE WHEN COUNT(*) > 0 THEN SUM(a.token_usage) / COUNT(*) ELSE 0 END as avg_usage,
			MAX(a.created_at) as last_used
		FROM aw_ai_audit_log a
		LEFT JOIN aw_user u ON a.user_id = u.user_id
		WHERE a.created_at >= NOW() - INTERVAL ? DAY AND a.token_usage > 0
		GROUP BY a.user_id, u.username, u.given_name
		ORDER BY total_usage DESC
		LIMIT ?
	`, days, limit).Scan(&results).Error
	return results, err
}

// UsageDailyTrend 每日使用量趋势
type DailyUsageStat struct {
	Date         string `gorm:"column:date" json:"date"`
	TotalUsage   int64  `gorm:"column:total_usage" json:"total_usage"`
	RequestCount int64  `gorm:"column:request_count" json:"request_count"`
	UniqueUsers  int64  `gorm:"column:unique_users" json:"unique_users"`
}

func (r *AIAuditLogRepository) UsageDailyTrend(days int) ([]DailyUsageStat, error) {
	var results []DailyUsageStat
	err := r.db.Raw(`
		SELECT DATE(created_at) as date,
			SUM(token_usage) as total_usage,
			COUNT(*) as request_count,
			COUNT(DISTINCT user_id) as unique_users
		FROM aw_ai_audit_log
		WHERE created_at >= NOW() - INTERVAL ? DAY AND token_usage > 0
		GROUP BY DATE(created_at)
		ORDER BY date ASC
	`, days).Scan(&results).Error
	return results, err
}
