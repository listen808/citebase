package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

type PromptVersionRepo struct {
	db *gorm.DB
}

func NewPromptVersionRepo(db *gorm.DB) *PromptVersionRepo {
	return &PromptVersionRepo{db: db}
}

func (r *PromptVersionRepo) Create(p *aimodel.PromptVersion) error {
	return r.db.Create(p).Error
}

func (r *PromptVersionRepo) Update(p *aimodel.PromptVersion) error {
	return r.db.Save(p).Error
}

func (r *PromptVersionRepo) Delete(id uint64) error {
	return r.db.Delete(&aimodel.PromptVersion{}, id).Error
}

func (r *PromptVersionRepo) ListBySpace(spaceID uint64) ([]aimodel.PromptVersion, error) {
	var list []aimodel.PromptVersion
	err := r.db.Where("space_id = ?", spaceID).Order("query_type, version").Find(&list).Error
	return list, err
}

func (r *PromptVersionRepo) GetActiveByType(spaceID uint64, queryType string) (*aimodel.PromptVersion, error) {
	var p aimodel.PromptVersion
	err := r.db.Where("space_id = ? AND query_type = ? AND active = ?", spaceID, queryType, true).First(&p).Error
	return &p, err
}

func (r *PromptVersionRepo) GetByExperiment(spaceID uint64, queryType, experimentID string) ([]aimodel.PromptVersion, error) {
	var list []aimodel.PromptVersion
	err := r.db.Where("space_id = ? AND query_type = ? AND experiment_id = ?", spaceID, queryType, experimentID).Find(&list).Error
	return list, err
}

func (r *PromptVersionRepo) FindByID(id uint64) (*aimodel.PromptVersion, error) {
	var p aimodel.PromptVersion
	err := r.db.First(&p, id).Error
	return &p, err
}

func (r *PromptVersionRepo) DeactivateBySpaceAndType(spaceID uint64, queryType string) error {
	return r.db.Model(&aimodel.PromptVersion{}).
		Where("space_id = ? AND query_type = ? AND active = ?", spaceID, queryType, true).
		Update("active", false).Error
}
