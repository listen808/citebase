package ai

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// SQLConversationTurn Text2SQL对话轮次记录（入库）
type SQLConversationTurn struct {
	ID              uint64         `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ConversationID  uint64         `gorm:"column:conversation_id;not null;index:idx_sql_conv_id" json:"conversation_id"` // 关联aw_ai_conversation.id
	UserID          uint64         `gorm:"column:user_id;not null;index:idx_sql_user_id" json:"user_id"`
	SpaceID         uint64         `gorm:"column:space_id;default:0;index:idx_sql_space_id" json:"space_id"`
	TurnNumber      int            `gorm:"column:turn_number;not null" json:"turn_number"`                  // 轮次序号（1,2,3...）
	UserQuery       string         `gorm:"column:user_query;type:text;not null" json:"user_query"`          // 用户原始查询
	ResolvedQuery   string         `gorm:"column:resolved_query;type:text" json:"resolved_query"`           // 引用解析后的查询
	IntentType      string         `gorm:"column:intent_type;size:20;default:new_query" json:"intent_type"` // new_query/follow_up/modify/clarify
	GeneratedSQL    string         `gorm:"column:generated_sql;type:text" json:"generated_sql"`             // 生成的SQL
	SQLExplanation  string         `gorm:"column:sql_explanation;type:text" json:"sql_explanation"`         // SQL解释
	UsedTables      JSONString     `gorm:"column:used_tables;type:json" json:"used_tables"`                 // 使用的表列表
	UsedFields      JSONString     `gorm:"column:used_fields;type:json" json:"used_fields"`                 // 使用的字段列表
	ExecutionResult string         `gorm:"column:execution_result;type:text" json:"execution_result"`       // 执行结果JSON
	ExecutionError  string         `gorm:"column:execution_error;type:text" json:"execution_error"`         // 执行错误
	IsSuccess       bool           `gorm:"column:is_success;default:false" json:"is_success"`               // 是否执行成功
	FeedbackRating  int8           `gorm:"column:feedback_rating;default:0" json:"feedback_rating"`         // 用户反馈评分（-1/0/1）
	EmbeddingID     string         `gorm:"column:embedding_id;size:100" json:"embedding_id"`                // 向量存储ID（用于检索）
	TokenUsage      int            `gorm:"column:token_usage;default:0" json:"token_usage"`                 // Token消耗
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
}

func (SQLConversationTurn) TableName() string {
	return "aw_text2sql_conversation_turn"
}

// SQLConversationSummary Text2SQL对话摘要（用于快速加载上下文）
type SQLConversationSummary struct {
	ID               uint64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ConversationID   uint64     `gorm:"column:conversation_id;not null;uniqueIndex:uk_conversation_id" json:"conversation_id"`
	UserID           uint64     `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	SpaceID          uint64     `gorm:"column:space_id;default:0" json:"space_id"`
	Summary          string     `gorm:"column:summary;type:text" json:"summary"`                      // 对话摘要（LLM生成）
	KeyTables        JSONString `gorm:"column:key_tables;type:json" json:"key_tables"`                // 关键表
	KeyFields        JSONString `gorm:"column:key_fields;type:json" json:"key_fields"`                // 关键字段
	CurrentSQL       string     `gorm:"column:current_sql;type:text" json:"current_sql"`              // 当前最新SQL
	TurnCount        int        `gorm:"column:turn_count;default:0" json:"turn_count"`                // 总轮次
	SuccessCount     int        `gorm:"column:success_count;default:0" json:"success_count"`          // 成功轮次
	LastQueryPattern string     `gorm:"column:last_query_pattern;size:200" json:"last_query_pattern"` // 最后查询模式
	CreatedAt        time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SQLConversationSummary) TableName() string {
	return "aw_text2sql_conversation_summary"
}

// SQLTurnEmbedding 对话轮次向量记录（用于语义检索）
type SQLTurnEmbedding struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TurnID         uint64    `gorm:"column:turn_id;not null;uniqueIndex:uk_turn_id" json:"turn_id"` // 关联aw_text2sql_conversation_turn.id
	ConversationID uint64    `gorm:"column:conversation_id;not null;index:idx_conversation_id" json:"conversation_id"`
	UserID         uint64    `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	QueryEmbedding string    `gorm:"column:query_embedding;type:text" json:"query_embedding"` // 查询向量（JSON数组）
	SQLEmbedding   string    `gorm:"column:sql_embedding;type:text" json:"sql_embedding"`     // SQL向量（JSON数组）
	CombinedText   string    `gorm:"column:combined_text;type:text" json:"combined_text"`     // 组合文本（查询+SQL）用于检索
	VectorStoreID  string    `gorm:"column:vector_store_id;size:100" json:"vector_store_id"`  // 外部向量存储ID
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (SQLTurnEmbedding) TableName() string {
	return "aw_text2sql_turn_embedding"
}

// SQLConversationSession Text2SQL会话状态（内存+数据库双写）
type SQLConversationSession struct {
	ID             uint64         `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionKey     string         `gorm:"column:session_key;size:100;not null;uniqueIndex:uk_session_key" json:"session_key"` // 会话标识（user_id+conversation_id）
	UserID         uint64         `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	ConversationID uint64         `gorm:"column:conversation_id;not null;index:idx_conversation_id" json:"conversation_id"`
	SpaceID        uint64         `gorm:"column:space_id;default:0" json:"space_id"`
	StateJSON      string         `gorm:"column:state_json;type:longtext" json:"state_json"` // ConversationState序列化JSON
	IsActive       bool           `gorm:"column:is_active;default:true" json:"is_active"`
	LastAccessAt   time.Time      `gorm:"column:last_access_at" json:"last_access_at"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
}

func (SQLConversationSession) TableName() string {
	return "aw_text2sql_session"
}

// SQLConversationTurnDTO 数据传输对象（用于向量存储）
type SQLConversationTurnDTO struct {
	ID             uint64   `json:"id"`
	ConversationID uint64   `json:"conversation_id"`
	UserID         uint64   `json:"user_id"`
	TurnNumber     int      `json:"turn_number"`
	UserQuery      string   `json:"user_query"`
	GeneratedSQL   string   `json:"generated_sql"`
	IntentType     string   `json:"intent_type"`
	UsedTables     []string `json:"used_tables"`
	IsSuccess      bool     `json:"is_success"`
	CombinedText   string   `json:"combined_text"` // 组合文本用于embedding
}

// BuildCombinedText 构建组合文本（用于向量检索）
func (t *SQLConversationTurnDTO) BuildCombinedText() string {
	return t.UserQuery + " | " + t.GeneratedSQL + " | 表: " + strings.Join(t.UsedTables, ",")
}
