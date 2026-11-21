package ai

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// SQLTrace SQL 生成执行链路追踪
type SQLTrace struct {
	ID             uint64           `gorm:"primaryKey" json:"id"`
	TraceID        string           `gorm:"column:trace_id;size:64;not null;uniqueIndex:uk_trace_id" json:"trace_id"`
	SpaceID        uint64           `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	ConversationID int              `gorm:"column:conversation_id;default:0" json:"conversation_id"`
	UserQuery      string           `gorm:"column:user_query;type:text;not null" json:"user_query"`
	Intent         string           `gorm:"size:50" json:"intent"`
	Steps          SQLTraceStepList `gorm:"column:steps;type:longtext" json:"steps"`
	FinalSQL       string           `gorm:"column:final_sql;type:longtext" json:"final_sql"`
	ExecutionOK    bool             `gorm:"column:execution_ok;default:false" json:"execution_ok"`
	DurationMs     int              `gorm:"column:duration_ms;default:0" json:"duration_ms"`
	CreatedAt      time.Time        `gorm:"autoCreateTime" json:"created_at"`
}

func (SQLTrace) TableName() string {
	return "aw_ai_sql_trace"
}

// SQLTraceStep 单个追踪步骤
type SQLTraceStep struct {
	Name       string `json:"name"`
	Input      string `json:"input"`
	Output     string `json:"output"`
	DurationMs int    `json:"duration_ms"`
	Status     string `json:"status"`
	ErrorMsg   string `json:"error_msg,omitempty"`
}

type SQLTraceStepList []SQLTraceStep

func (s SQLTraceStepList) Value() (driver.Value, error) {
	return json.Marshal(s)
}

func (s *SQLTraceStepList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, s)
}
