package knowledge

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// AIExtractedEntities AI 从文档中提取的结构化实体信息
type AIExtractedEntities struct {
	ID          int        `gorm:"primaryKey" json:"id"`
	DocID       int        `gorm:"index:idx_doc_id;not null" json:"doc_id"` // 源文档 ID
	URLs        JSONArray  `gorm:"type:json" json:"urls"`                   // URL 列表
	ConfigItems JSONArray  `gorm:"type:json" json:"config_items"`           // 配置项列表
	Aliases     JSONString `gorm:"type:json" json:"aliases"`                // 别名列表
	ActionItems JSONString `gorm:"type:json" json:"action_items"`           // 待办事项列表
	URLGroups   JSONArray  `gorm:"type:json" json:"url_groups"`             // URL 分组列表
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (AIExtractedEntities) TableName() string {
	return "aw_ai_extracted_entities"
}

// JSONArray 用于存储 JSON 数组的自定义类型
type JSONArray []map[string]interface{}

// Value 实现 driver.Valuer 接口
func (j JSONArray) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
func (j *JSONArray) Scan(value interface{}) error {
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

// JSONString 用于存储 JSON 字符串数组的自定义类型
type JSONString []string

// Value 实现 driver.Valuer 接口
func (j JSONString) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan 实现 sql.Scanner 接口
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
