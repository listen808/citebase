package ai

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type BusinessRule struct {
	ID            uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	Name          string         `gorm:"size:200;not null" json:"name"`
	Type          string         `gorm:"size:50;not null" json:"type"`
	TableNam      string         `gorm:"column:table_name;size:100;not null;index:idx_table_name" json:"table_name"`
	FieldName     string         `gorm:"size:100" json:"field_name"`
	Content       string         `gorm:"type:text;not null" json:"content"`
	Description   string         `gorm:"type:text" json:"description"`
	Status        int8           `gorm:"default:1" json:"status"`
	SchemaSpaceID uint64         `gorm:"column:schema_space_id;not null;index:idx_schema_space_id" json:"schema_space_id"`
}

func (BusinessRule) TableName() string {
	return "aw_ai_business_rule"
}

type ColumnInfo struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Comment      string `json:"comment"`
	IsPK         bool   `json:"is_pk"`
	IsNullable   bool   `json:"is_nullable"`
	DefaultValue string `json:"default_value"`
	IsAutoInc    bool   `json:"is_auto_inc"`
	CharSet      string `json:"charset"`
	Collation    string `json:"collation"`
}

type ColumnInfoList []ColumnInfo

func (c ColumnInfoList) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *ColumnInfoList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

type TableConstraint struct {
	TableName      string   `json:"table_name"`
	ConstraintType string   `json:"constraint_type"`
	ConstraintName string   `json:"constraint_name"`
	Columns        []string `json:"columns"`
	RefTable       string   `json:"ref_table,omitempty"`
	RefColumns     []string `json:"ref_columns,omitempty"`
	IndexType      string   `json:"index_type,omitempty"`
	Comment        string   `json:"comment,omitempty"`
}

type TableConstraintList []TableConstraint

func (c TableConstraintList) Value() (driver.Value, error) {
	return json.Marshal(c)
}

func (c *TableConstraintList) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, c)
}

type TableCache struct {
	ID        uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	// 表缓存是多空间设计：唯一键必须是 (space_id, table_name)。旧的全局 table_name
	// 唯一索引会让不同空间的同名表在 upsert 时互相覆盖甚至写入失败。
	TableNam         string              `gorm:"column:table_name;size:100;uniqueIndex:idx_table_cache_space_name" json:"table_name"`
	TableComment     string              `gorm:"size:500" json:"table_comment"`
	DDLContent       string              `gorm:"type:text;not null" json:"ddl_content"`
	ColumnsJSON      ColumnInfoList      `gorm:"type:json" json:"columns_json"`
	ConstraintsJSON  TableConstraintList `gorm:"type:json" json:"constraints_json"`
	SpaceID          *uint64             `gorm:"column:space_id;uniqueIndex:idx_table_cache_space_name" json:"space_id"`
	SchemaSpaceID    *uint64             `gorm:"column:schema_space_id" json:"schema_space_id"`
	DocID            *uint64             `gorm:"column:doc_id" json:"doc_id"`
	DBType           string              `gorm:"size:50" json:"db_type"`
	RowCountEstimate int64               `gorm:"default:0" json:"row_count_estimate"`
	Version          int                 `gorm:"default:1" json:"version"`
}

func (TableCache) TableName() string {
	return "aw_ai_table_cache"
}

type TableRelation struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	SourceTable  string         `gorm:"size:100;not null" json:"source_table"`
	SourceField  string         `gorm:"size:100;not null" json:"source_field"`
	TargetTable  string         `gorm:"size:100;not null" json:"target_table"`
	TargetField  string         `gorm:"size:100;not null" json:"target_field"`
	RelationType string         `gorm:"size:50;default:one_to_many" json:"relation_type"`
	Confidence   float32        `gorm:"default:1" json:"confidence"`
	SpaceID      *uint64        `gorm:"column:space_id" json:"space_id"`
}

func (TableRelation) TableName() string {
	return "aw_ai_table_relation"
}

type RecognizedEntity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
	Field string `json:"field,omitempty"`
}

// AgentStepInfo Agent ReAct 循环中的一步（用于前端展示思考过程）
type AgentStepInfo struct {
	Step        int                    `json:"step"`
	Thought     string                 `json:"thought"`
	Action      string                 `json:"action"`
	ActionInput map[string]interface{} `json:"action_input,omitempty"`
	Observation string                 `json:"observation"`
	DurationMs  int64                  `json:"duration_ms"`
}

type SQLResult struct {
	SQL             string             `json:"sql"`
	Tables          []string           `json:"tables"`
	Entities        []RecognizedEntity `json:"entities"`
	Relations       []TableRelation    `json:"relations"`
	Explanations    []string           `json:"explanations"`
	Warnings        []string           `json:"warnings,omitempty"`
	ExecutionResult interface{}        `json:"execution_result,omitempty"`
	Explanation     *SQLExplanation    `json:"explanation,omitempty"`
	TraceID         string             `json:"trace_id,omitempty"`
	GeneratorTier   string             `json:"generator_tier,omitempty"`
	Steps           []AgentStepInfo    `json:"steps,omitempty"`
	// P0: LLM 原始 CoT 输出（含思考流程+字段映射+SQL），供前端完整展示生成过程
	RawCoT string `json:"raw_cot,omitempty"`
	// P1: selfReflect 修正前后 SQL 对比
	SelfReflectBefore string `json:"self_reflect_before,omitempty"`
	SelfReflectAfter  string `json:"self_reflect_after,omitempty"`
	// 字段映射（用户说的X → DDL中的Y）
	FieldMappings []string `json:"field_mappings,omitempty"`
}

type PromptTemplate struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	Type         string         `json:"type" gorm:"size:20;not null;default:'sql'"`
	QueryType    string         `gorm:"size:50;not null;default:'basic'" json:"query_type"`
	SystemPrompt string         `gorm:"type:text" json:"system_prompt"`
	UserTemplate string         `gorm:"type:text;not null" json:"user_template"`
	Description  string         `gorm:"size:500" json:"description"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	IsDefault    bool           `gorm:"default:false" json:"is_default"`
	Sort         int            `gorm:"default:0" json:"sort"`
	SpaceID      *uint64        `gorm:"column:space_id" json:"space_id"`
}

func (PromptTemplate) TableName() string {
	return "aw_ai_prompt_template"
}
