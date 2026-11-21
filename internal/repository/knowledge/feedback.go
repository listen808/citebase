package knowledge

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AIFeedbackRepository struct {
	db *gorm.DB
}

func NewAIFeedbackRepository(db *gorm.DB) *AIFeedbackRepository {
	return &AIFeedbackRepository{db: repos.DB}
}

func DefaultAIFeedbackRepository() *AIFeedbackRepository {
	return &AIFeedbackRepository{db: repos.DB}
}

func (r *AIFeedbackRepository) Create(feedback *knowledgemodel.AIFeedback) error {
	return r.db.Create(feedback).Error
}

func (r *AIFeedbackRepository) FindByUserID(userID int, page, pageSize int) ([]knowledgemodel.AIFeedback, int, error) {
	var feedbacks []knowledgemodel.AIFeedback
	var total int64

	query := r.db.Model(&knowledgemodel.AIFeedback{}).Where("user_id = ?", userID)
	query.Count(&total)

	err := query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&feedbacks).Error

	return feedbacks, int(total), err
}

func (r *AIFeedbackRepository) GetStats() (positive, negative int, err error) {
	var posCount, negCount int64
	r.db.Model(&knowledgemodel.AIFeedback{}).Where("rating = 1").Count(&posCount)
	r.db.Model(&knowledgemodel.AIFeedback{}).Where("rating = -1").Count(&negCount)
	return int(posCount), int(negCount), nil
}

func (r *AIFeedbackRepository) GetRecentReasons(limit int) ([]string, error) {
	var reasons []string
	err := r.db.Model(&knowledgemodel.AIFeedback{}).
		Where("rating = -1 AND reason != ''").
		Order("created_at DESC").
		Limit(limit).
		Pluck("reason", &reasons).Error
	return reasons, err
}
