package ai

import "time"

// AIUsageDaily 按天/用户/模型聚合的用量与成本底账。
//
// P1-6：此前 Token 统计只能从 aw_ai_audit_log 一列聚合——审计表按每条请求一行增长，
// 拉一年报表要扫全表，且没有任何预算/配额载体。这张表把增量聚合收敛到
// 每天每用户每模型一行，直接支撑按部门/按模型计费与预算拦截。
type AIUsageDaily struct {
	ID       int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Date     time.Time `gorm:"column:date;type:date;not null;uniqueIndex:uk_usage_day" json:"date"`
	UserID   int       `gorm:"column:user_id;not null;uniqueIndex:uk_usage_day" json:"user_id"`
	Provider string    `gorm:"column:provider;type:varchar(64);uniqueIndex:uk_usage_day" json:"provider"`
	Model    string    `gorm:"column:model;type:varchar(128);uniqueIndex:uk_usage_day" json:"model"`
	Action   string    `gorm:"column:action;type:varchar(32);uniqueIndex:uk_usage_day" json:"action"`
	// RequestCount 请求次数
	RequestCount int `gorm:"column:request_count;not null;default:1" json:"request_count"`
	// TokenUsage 累计 token
	TokenUsage int `gorm:"column:token_usage;not null;default:0" json:"token_usage"`
	// DurationMs 累计耗时（毫秒），除以 RequestCount 得平均延迟
	DurationMs int64 `gorm:"column:duration_ms;not null;default:0" json:"duration_ms"`
	// FailedCount 失败请求次数（成本口径也要算失败调用——它们同样烧 token）
	FailedCount int       `gorm:"column:failed_count;not null;default:0" json:"failed_count"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AIUsageDaily) TableName() string {
	return "aw_ai_usage_daily"
}
