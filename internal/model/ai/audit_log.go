package ai

import "time"

type AIAuditLog struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID      int       `gorm:"column:user_id;not null;default:0;index:idx_user_id" json:"user_id"`
	Action      string    `gorm:"column:action;size:64;not null;index:idx_action" json:"action"`
	TargetID    string    `gorm:"column:target_id;size:64;default:''" json:"target_id"`
	Question    string    `gorm:"column:question;type:text" json:"question"`
	Answer      string    `gorm:"column:answer;type:text" json:"answer"`
	Detail      string    `gorm:"column:detail;type:text" json:"detail"`
	TokenUsage  int       `gorm:"column:token_usage;default:0" json:"token_usage"`
	SourceCount int       `gorm:"column:source_count;default:0" json:"source_count"`
	Model       string    `gorm:"column:model;size:128;default:''" json:"model"`
	Provider    string    `gorm:"column:provider;size:64;default:''" json:"provider"`
	Result      string    `gorm:"column:result;size:32;default:'success'" json:"result"`
	Reason      string    `gorm:"column:reason;size:512;default:''" json:"reason"`
	IPAddress   string    `gorm:"column:ip_address;size:64;default:''" json:"ip_address"`
	UserAgent   string    `gorm:"column:user_agent;size:255;default:''" json:"user_agent"`
	DurationMs  int64     `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AIAuditLog) TableName() string {
	return "aw_ai_audit_log"
}
