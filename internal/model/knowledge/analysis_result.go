package knowledge

import "time"

type AIAnalysisResult struct {
	ID            int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SpaceID       int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	DocID         int       `gorm:"column:doc_id;not null;index:idx_doc_id" json:"doc_id"`
	Summary       string    `gorm:"type:text" json:"summary"`
	KeyPoints     string    `gorm:"type:json" json:"key_points"`
	Category      string    `gorm:"type:varchar(100);index:idx_category" json:"category"`
	Tags          string    `gorm:"type:json" json:"tags"`
	QualityScore  float64   `gorm:"type:decimal(5,2)" json:"quality_score"`
	QualityIssues string    `gorm:"type:json" json:"quality_issues"`
	Entities      string    `gorm:"type:json" json:"entities"`
	ContentHash   string    `gorm:"type:varchar(64)" json:"content_hash"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AIAnalysisResult) TableName() string {
	return "aw_ai_analysis_result"
}
