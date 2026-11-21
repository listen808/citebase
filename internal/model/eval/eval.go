package eval

import "time"

// EvalDataset 评估数据集
type EvalDataset struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;size:255;not null" json:"name"`
	SpaceID   int       `gorm:"column:space_id;not null;default:0" json:"space_id"`
	Questions string    `gorm:"column:questions;type:text" json:"-"` // JSON-encoded []EvalQuestion
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (EvalDataset) TableName() string {
	return "aw_eval_dataset"
}

// EvalResult 评估结果
type EvalResult struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DatasetID    int64     `gorm:"column:dataset_id;not null;index:idx_dataset_id" json:"dataset_id"`
	DatasetName  string    `gorm:"column:dataset_name;size:255" json:"dataset_name"`
	SpaceID      int       `gorm:"column:space_id;not null;default:0" json:"space_id"`
	OverallScore float64   `gorm:"column:overall_score;type:decimal(5,4)" json:"overall_score"`
	MetricsJSON  string    `gorm:"column:metrics_json;type:text" json:"-"`
	DetailsJSON  string    `gorm:"column:details_json;type:longtext" json:"-"`
	Duration     string    `gorm:"column:duration;size:32" json:"duration"`
	Status       string    `gorm:"column:status;size:32;default:'completed'" json:"status"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (EvalResult) TableName() string {
	return "aw_eval_result"
}
