package ai

import (
	"time"
)

// ColumnStats 列级统计
type ColumnStats struct {
	ID            uint64    `gorm:"primaryKey" json:"id"`
	SpaceID       uint64    `gorm:"column:space_id;not null" json:"space_id"`
	TableNam      string    `gorm:"column:table_name;size:100;not null" json:"table_name"`
	ColumnName    string    `gorm:"column:column_name;size:100;not null" json:"column_name"`
	DataType      string    `gorm:"size:50" json:"data_type"`
	RowCount      int64     `gorm:"default:0" json:"row_count"`
	NullCount     int64     `gorm:"default:0" json:"null_count"`
	DistinctCount int64     `gorm:"default:0" json:"distinct_count"`
	MinValue      string    `gorm:"size:500" json:"min_value"`
	MaxValue      string    `gorm:"size:500" json:"max_value"`
	AvgValue      string    `gorm:"size:500" json:"avg_value"`
	SampleValues  string    `gorm:"type:text" json:"sample_values"`
	Distribution  string    `gorm:"type:text" json:"distribution"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ColumnStats) TableName() string {
	return "aw_ai_column_stats"
}
