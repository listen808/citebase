package knowledge

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AiRagasEvalRepository struct {
	db *gorm.DB
}

func NewAiRagasEvalRepository() *AiRagasEvalRepository {
	return &AiRagasEvalRepository{db: repos.DB}
}

func (r *AiRagasEvalRepository) Create(eval *knowledgemodel.AiRagasEval) error {
	return r.db.Create(eval).Error
}

func (r *AiRagasEvalRepository) FindByID(id int) (*knowledgemodel.AiRagasEval, error) {
	var eval knowledgemodel.AiRagasEval
	err := r.db.First(&eval, id).Error
	if err != nil {
		return nil, err
	}
	return &eval, nil
}

func (r *AiRagasEvalRepository) FindRecent(limit int) ([]*knowledgemodel.AiRagasEval, error) {
	var evals []*knowledgemodel.AiRagasEval
	err := r.db.Order("created_at DESC").Limit(limit).Find(&evals).Error
	return evals, err
}

func (r *AiRagasEvalRepository) FindWithOffset(limit, offset int) ([]*knowledgemodel.AiRagasEval, int64, error) {
	var evals []*knowledgemodel.AiRagasEval
	var total int64
	if err := r.db.Model(&knowledgemodel.AiRagasEval{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&evals).Error
	return evals, total, err
}

type RagasEvalStats struct {
	Total               int64   `json:"total"`
	AvgFaithfulness     float64 `json:"avg_faithfulness"`
	AvgAnswerRelevancy  float64 `json:"avg_answer_relevancy"`
	AvgContextPrecision float64 `json:"avg_context_precision"`
	AvgContextRecall    float64 `json:"avg_context_recall"`
}

func (r *AiRagasEvalRepository) Stats() (*RagasEvalStats, error) {
	var stats RagasEvalStats
	row := r.db.Model(&knowledgemodel.AiRagasEval{}).Select(`
		COUNT(*) as total,
		COALESCE(AVG(faithfulness), 0) as avg_faithfulness,
		COALESCE(AVG(answer_relevancy), 0) as avg_answer_relevancy,
		COALESCE(AVG(context_precision), 0) as avg_context_precision,
		COALESCE(AVG(CASE WHEN ground_truth != '' THEN context_recall ELSE NULL END), 0) as avg_context_recall
	`).Row()
	if err := row.Scan(&stats.Total, &stats.AvgFaithfulness, &stats.AvgAnswerRelevancy, &stats.AvgContextPrecision, &stats.AvgContextRecall); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (r *AiRagasEvalRepository) DeleteByID(id int) error {
	return r.db.Delete(&knowledgemodel.AiRagasEval{}, id).Error
}

func (r *AiRagasEvalRepository) ClearAll() error {
	return r.db.Where("1 = 1").Delete(&knowledgemodel.AiRagasEval{}).Error
}
