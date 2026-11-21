package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

type ABExperimentRepo struct {
	db *gorm.DB
}

func NewABExperimentRepo(db *gorm.DB) *ABExperimentRepo {
	return &ABExperimentRepo{db: db}
}

func (r *ABExperimentRepo) Create(e *aimodel.ABExperiment) error {
	return r.db.Create(e).Error
}

func (r *ABExperimentRepo) Update(e *aimodel.ABExperiment) error {
	return r.db.Save(e).Error
}

func (r *ABExperimentRepo) Delete(id uint64) error {
	return r.db.Delete(&aimodel.ABExperiment{}, id).Error
}

func (r *ABExperimentRepo) GetByExperimentID(experimentID string) (*aimodel.ABExperiment, error) {
	var e aimodel.ABExperiment
	err := r.db.Where("experiment_id = ?", experimentID).First(&e).Error
	return &e, err
}

func (r *ABExperimentRepo) ListBySpace(spaceID uint64) ([]aimodel.ABExperiment, error) {
	var list []aimodel.ABExperiment
	err := r.db.Where("space_id = ?", spaceID).Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *ABExperimentRepo) FindByID(id uint64) (*aimodel.ABExperiment, error) {
	var e aimodel.ABExperiment
	err := r.db.First(&e, id).Error
	return &e, err
}
