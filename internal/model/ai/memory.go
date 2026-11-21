package ai

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// MemoryType 记忆类型
type MemoryType string

const (
	MemoryTypePreference MemoryType = "preference" // 用户偏好
	MemoryTypeKnowledge  MemoryType = "knowledge"  // 知识记忆
	MemoryTypeSession    MemoryType = "session"    // 会话摘要
	MemoryTypeEntity     MemoryType = "entity"     // 实体记忆（表名、字段名等）
	// MemoryTypeUserFeedback 用户反馈（点踩样本 = 失败样本库）。
	// P0-4：CollectFeedback 原先写的是 knowledge，而 GetFeedbackSummary 按
	// "user_feedback" 读 —— 读写类型不一致，汇总永远是空的。
	MemoryTypeUserFeedback MemoryType = "user_feedback"
)

// MemoryStatus 记忆确认状态（P2-8 待确认区）。
// LLM 从对话推断出的记忆先进 pending，用户点头（confirm）后才进 prompt，
// 避免错误记忆污染后续所有回答。存量记忆迁移时默认 confirmed，行为不变。
type MemoryStatus string

const (
	MemoryStatusPending   MemoryStatus = "pending"   // 待确认：LLM 刚推断，尚未进入召回
	MemoryStatusConfirmed MemoryStatus = "confirmed" // 已确认：可被召回注入 prompt
)

// MemoryTier 记忆层级（P1-7 §3.2.2）
type MemoryTier string

const (
	MemoryTierShort MemoryTier = "short" // 短期记忆（默认，TTL 24h，Importance < 0.7）
	MemoryTierLong  MemoryTier = "long"  // 长期记忆（Importance >= 0.7，90天未命中降级）
	MemoryTierStale MemoryTier = "stale" // 降级候选（90天未命中，180天未命中删除）
)

// AIMemory 用户记忆表
type AIMemory struct {
	ID             uint         `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         uint         `gorm:"column:user_id;index:idx_user_id;not null" json:"user_id"`
	MemoryType     MemoryType   `gorm:"column:memory_type;size:20;not null;index:idx_memory_type" json:"memory_type"`
	Content        string       `gorm:"column:content;type:text;not null" json:"content"`
	Summary        string       `gorm:"column:summary;size:500" json:"summary"`                                         // 简短摘要，用于快速展示
	SourceConvID   uint         `gorm:"column:source_conv_id;default:0" json:"source_conv_id"`                          // 来源会话ID
	Tags           JSONString   `gorm:"column:tags;type:json" json:"tags"`                                              // 标签 ["mysql","table:users"]
	Importance     float64      `gorm:"column:importance;default:0.5" json:"importance"`                                // 重要性 0-1
	AccessCount    uint         `gorm:"column:access_count;default:0" json:"access_count"`                              // 访问次数（= hit_count）
	ExpiresAt      *time.Time   `gorm:"column:expires_at" json:"expires_at,omitempty"`                                  // 过期时间
	MemoryTier     MemoryTier   `gorm:"column:memory_tier;size:10;default:short;index:idx_mem_tier" json:"memory_tier"` // P1-7: 记忆层级（单列索引，供 ExpireStaleMemories 使用）
	Status         MemoryStatus `gorm:"column:status;size:10;default:confirmed;index:idx_mem_status" json:"status"`     // P2-8: 确认状态（pending 不进召回）
	LastAccessedAt *time.Time   `gorm:"column:last_accessed_at" json:"last_accessed_at,omitempty"`                      // P1-7: 最后访问时间（区别于 UpdatedAt）
	CreatedAt      time.Time    `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time    `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (AIMemory) TableName() string {
	return "aw_ai_memory"
}

// AIMemorySummary 对话会话记忆摘要
type AIMemorySummary struct {
	ID           uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       uint       `gorm:"column:user_id;index:idx_user_id;not null" json:"user_id"`
	ConvID       uint       `gorm:"column:conv_id;uniqueIndex:uk_conv_id" json:"conv_id"` // 关联会话ID
	Summary      string     `gorm:"column:summary;type:text;not null" json:"summary"`
	KeyPoints    JSONString `gorm:"column:key_points;type:json" json:"key_points"` // 关键要点列表
	Entities     JSONString `gorm:"column:entities;type:json" json:"entities"`     // 提取的实体
	TokenCount   uint       `gorm:"column:token_count;default:0" json:"token_count"`
	MessageCount uint       `gorm:"column:message_count;default:0" json:"message_count"`
	CreatedAt    time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AIMemorySummary) TableName() string {
	return "aw_ai_memory_summary"
}

// JSONString JSON字符串类型（GORM序列化）
type JSONString []string

func (j JSONString) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONString) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// MemorySearchRequest 记忆搜索请求
type MemorySearchRequest struct {
	UserID     uint   `json:"user_id"`
	Query      string `json:"query"`       // 搜索关键词
	MemoryType string `json:"memory_type"` // 过滤类型
	Limit      int    `json:"limit"`       // 返回数量
}

// MemorySearchResult 记忆搜索结果
type MemorySearchResult struct {
	Memory   AIMemory `json:"memory"`
	Score    float64  `json:"score"`    // 相关性分数
	Relevant bool     `json:"relevant"` // 是否相关
}
