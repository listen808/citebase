package ai

import "time"

// Text2SQLFeedback Text2SQL 执行反馈记录（成功/失败/修正）
type Text2SQLFeedback struct {
	ID            uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID       uint64    `gorm:"column:space_id;not null;index:idx_t2s_feedback_space_created" json:"space_id"`
	UserQuery     string    `gorm:"column:user_query;type:text;not null" json:"user_query"`
	GeneratedSQL  string    `gorm:"column:generated_sql;type:longtext" json:"generated_sql"`
	ExecutionOK   bool      `gorm:"column:execution_ok;not null;default:false" json:"execution_ok"`
	ErrorType     string    `gorm:"column:error_type;size:64;default:''" json:"error_type"`
	ErrorMessage  string    `gorm:"column:error_message;type:text" json:"error_message"`
	CorrectedSQL  string    `gorm:"column:corrected_sql;type:longtext" json:"corrected_sql"`
	CorrectionSrc string    `gorm:"column:correction_src;size:16;default:''" json:"correction_src"`
	DurationMs    int       `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Text2SQLFeedback) TableName() string {
	return "aw_ai_text2sql_feedback"
}
