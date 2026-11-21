package ai

import "time"

// SQLSecurityPolicy 空间级 SQL 执行安全策略
// SpaceID = 0 表示全局默认策略
type SQLSecurityPolicy struct {
	ID            uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	SpaceID       uint64    `json:"space_id" gorm:"column:space_id;not null;uniqueIndex:idx_space_policy"`
	MaxRows       int       `json:"max_rows" gorm:"default:200"`
	TimeoutSec    int       `json:"timeout_sec" gorm:"default:15"`
	MaxCostTokens int       `json:"max_cost_tokens" gorm:"default:0"` // 0 表示不限制
	AllowPatterns string    `json:"allow_patterns" gorm:"type:text"`  // 换行分隔的正则白名单
	DenyPatterns  string    `json:"deny_patterns" gorm:"type:text"`   // 换行分隔的正则黑名单
	AllowDML      bool      `json:"allow_dml" gorm:"default:false"`
	AllowDDL      bool      `json:"allow_ddl" gorm:"default:false"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (SQLSecurityPolicy) TableName() string {
	return "aw_ai_sql_security_policy"
}

// SQLExplanation 查询解释，作为 JSON 嵌入 SQLHistory
type SQLExplanation struct {
	NaturalLanguage string         `json:"natural_language"`
	FieldMapping    []FieldMapping `json:"field_mapping"`
	TableMapping    []TableMapping `json:"table_mapping"`
	Confidence      float64        `json:"confidence"`
	ReasoningSteps  []string       `json:"reasoning_steps"`
	Caveats         []string       `json:"caveats"`
}

// FieldMapping 用户术语到数据库字段的映射
type FieldMapping struct {
	UserTerm string `json:"user_term"`
	Table    string `json:"table"`
	Field    string `json:"field"`
}

// TableMapping 用户概念到数据库表的映射
type TableMapping struct {
	UserConcept string `json:"user_concept"`
	Table       string `json:"table"`
}
