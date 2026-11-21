package workspace

import (
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type DocumentTrashRepository struct {
	db *gorm.DB
}

func NewDocumentTrashRepository() *DocumentTrashRepository {
	return &DocumentTrashRepository{db: repos.DB}
}

func NewDocumentTrashRepositoryWithDB(db *gorm.DB) *DocumentTrashRepository {
	return &DocumentTrashRepository{db: db}
}

func (r *DocumentTrashRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *DocumentTrashRepository) Create(item *workspacemodel.DocumentTrash) error {
	return r.db.Create(item).Error
}

func (r *DocumentTrashRepository) FindByDocumentID(docID int) (*workspacemodel.DocumentTrash, error) {
	var item workspacemodel.DocumentTrash
	if err := r.db.Where("document_id = ?", docID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *DocumentTrashRepository) FindBySpaceID(spaceID int) ([]*workspacemodel.DocumentTrash, error) {
	var items []*workspacemodel.DocumentTrash
	err := r.db.Where("space_id = ?", spaceID).Order("deleted_at DESC").Find(&items).Error
	return items, err
}

func (r *DocumentTrashRepository) DeleteByDocumentID(docID int) error {
	return r.db.Where("document_id = ?", docID).Delete(&workspacemodel.DocumentTrash{}).Error
}

func (r *DocumentTrashRepository) DeleteExpired(before time.Time) error {
	return r.db.Where("deleted_at < ?", before.Unix()).Delete(&workspacemodel.DocumentTrash{}).Error
}
