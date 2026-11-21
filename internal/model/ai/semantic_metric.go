package ai

import (
	"time"

	"gorm.io/gorm"
)

// SemanticMetric 语义指标
type SemanticMetric struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	SpaceID         uint64         `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Name            string         `gorm:"size:200;not null" json:"name"`
	Code            string         `gorm:"size:100;not null" json:"code"`
	Description     string         `gorm:"type:text" json:"description"`
	Formula         string         `gorm:"type:text;not null" json:"formula"`
	DefaultTable    string         `gorm:"size:100;not null" json:"default_table"`
	Dimensions      string         `gorm:"type:text" json:"dimensions"`
	TimeField       string         `gorm:"size:100" json:"time_field"`
	TimeGranularity string         `gorm:"size:20" json:"time_granularity"`
}

func (SemanticMetric) TableName() string {
	return "aw_ai_semantic_metric"
}

// SemanticDimension 语义维度
type SemanticDimension struct {
	ID          uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	SpaceID     uint64         `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Name        string         `gorm:"size:200;not null" json:"name"`
	Code        string         `gorm:"size:100;not null" json:"code"`
	Field       string         `gorm:"size:100;not null" json:"field"`
	Table       string         `gorm:"size:100;not null" json:"table"`
	ValueType   string         `gorm:"size:20;default:string" json:"value_type"`
	Description string         `gorm:"type:text" json:"description"`
}

func (SemanticDimension) TableName() string {
	return "aw_ai_semantic_dimension"
}
