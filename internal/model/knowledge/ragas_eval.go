package knowledge

import "time"

type AiRagasEval struct {
	ID               int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Question         string    `gorm:"column:question;type:text;not null" json:"question"`
	Answer           string    `gorm:"column:answer;type:text;not null" json:"answer"`
	ExpectedAnswer   string    `gorm:"column:expected_answer;type:text" json:"expected_answer"`
	Contexts         string    `gorm:"column:contexts;type:text" json:"contexts"`
	GroundTruth      string    `gorm:"column:ground_truth;type:text" json:"ground_truth"`
	Faithfulness     float64   `gorm:"column:faithfulness;type:decimal(5,4)" json:"faithfulness"`
	AnswerRelevancy  float64   `gorm:"column:answer_relevancy;type:decimal(5,4)" json:"answer_relevancy"`
	ContextPrecision float64   `gorm:"column:context_precision;type:decimal(5,4)" json:"context_precision"`
	ContextRecall    float64   `gorm:"column:context_recall;type:decimal(5,4)" json:"context_recall"`
	AvgScore         float64   `gorm:"column:avg_score;type:decimal(5,4)" json:"avg_score"`
	Duration         string    `gorm:"column:duration;size:32;default:''" json:"duration"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AiRagasEval) TableName() string {
	return "aw_ai_ragas_eval"
}
