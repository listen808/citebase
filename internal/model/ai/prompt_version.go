package ai

import (
	"time"

	"gorm.io/gorm"
)

// PromptVersion 提示词版本
type PromptVersion struct {
	ID           uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	SpaceID      uint64         `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Name         string         `gorm:"size:200;not null" json:"name"`
	QueryType    string         `gorm:"column:query_type;size:50;not null" json:"query_type"`
	Version      int            `gorm:"not null" json:"version"`
	SystemPrompt string         `gorm:"column:system_prompt;type:text" json:"system_prompt"`
	UserTemplate string         `gorm:"column:user_template;type:text" json:"user_template"`
	Active       bool           `gorm:"default:false" json:"active"`
	ExperimentID string         `gorm:"column:experiment_id;size:64" json:"experiment_id"`
}

func (PromptVersion) TableName() string {
	return "aw_ai_prompt_version"
}

// ABExperiment A/B 实验
type ABExperiment struct {
	ID           uint64    `gorm:"primaryKey" json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SpaceID      uint64    `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Name         string    `gorm:"size:200;not null" json:"name"`
	ExperimentID string    `gorm:"column:experiment_id;size:64;not null;uniqueIndex:uk_experiment_id" json:"experiment_id"`
	Description  string    `gorm:"type:text" json:"description"`
	TrafficA     int       `gorm:"column:traffic_a;default:50" json:"traffic_a"`
	TrafficB     int       `gorm:"column:traffic_b;default:50" json:"traffic_b"`
	MetricA      int       `gorm:"column:metric_a;default:0" json:"metric_a"`
	MetricB      int       `gorm:"column:metric_b;default:0" json:"metric_b"`
	Status       int8      `gorm:"default:1" json:"status"`
}

func (ABExperiment) TableName() string {
	return "aw_ai_ab_experiment"
}
