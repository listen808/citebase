package ai

// BusinessGlossary 业务术语→DDL字段映射表
// 提供确定性的字段名映射，消除 LLM 对字段名的幻觉
type BusinessGlossary struct {
	ID          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SpaceID     int    `json:"space_id" gorm:"column:space_id;index:idx_space_term"`
	Term        string `json:"term" gorm:"type:varchar(200);not null;comment:'业务术语/用户可能使用的名称'"`
	TblName     string `json:"table_name" gorm:"type:varchar(100);not null;column:table_name;comment:'DDL表名'"`
	ColName     string `json:"column_name" gorm:"type:varchar(100);not null;column:column_name;comment:'DDL字段名'"`
	Description string `json:"description" gorm:"type:varchar(500);column:description;comment:'映射说明'"`
	CreatedBy   int    `json:"created_by" gorm:"default:0;column:created_by"`
	CreatedAt   string `json:"created_at" gorm:"autoCreateTime"`
}

func (BusinessGlossary) TableName() string { return "aw_ai_business_glossary" }
