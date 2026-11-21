package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type SavedSearchRepository struct {
	db *gorm.DB
}

func NewSavedSearchRepository() *SavedSearchRepository {
	return &SavedSearchRepository{db: repository.DB}
}

func NewSavedSearchRepositoryWithDB(db *gorm.DB) *SavedSearchRepository {
	return &SavedSearchRepository{db: db}
}

func (r *SavedSearchRepository) Create(search *workspacemodel.SavedSearch) error {
	return r.db.Create(search).Error
}

func (r *SavedSearchRepository) FindByID(id int64) (*workspacemodel.SavedSearch, error) {
	var search workspacemodel.SavedSearch
	err := r.db.Where("id = ?", id).First(&search).Error
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func (r *SavedSearchRepository) FindByUserID(userID int64) ([]workspacemodel.SavedSearch, error) {
	var searches []workspacemodel.SavedSearch
	err := r.db.Where("user_id = ?", userID).
		Order("use_count DESC, update_time DESC").
		Find(&searches).Error
	return searches, err
}

func (r *SavedSearchRepository) FindPublic(limit int) ([]workspacemodel.SavedSearch, error) {
	var searches []workspacemodel.SavedSearch
	err := r.db.Where("is_public = 1").
		Order("use_count DESC").
		Limit(limit).
		Find(&searches).Error
	return searches, err
}

func (r *SavedSearchRepository) Update(search *workspacemodel.SavedSearch) error {
	return r.db.Save(search).Error
}

func (r *SavedSearchRepository) IncrementUseCount(id int64) error {
	return r.db.Model(&workspacemodel.SavedSearch{}).
		Where("id = ?", id).
		UpdateColumn("use_count", gorm.Expr("use_count + 1")).Error
}

func (r *SavedSearchRepository) Delete(id int64) error {
	return r.db.Delete(&workspacemodel.SavedSearch{}, id).Error
}

func (r *SavedSearchRepository) DeleteByUser(userID int64) error {
	return r.db.Where("user_id = ?", userID).Delete(&workspacemodel.SavedSearch{}).Error
}
