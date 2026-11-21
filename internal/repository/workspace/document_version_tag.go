package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type DocumentVersionTagRepository struct {
	db *gorm.DB
}

func NewDocumentVersionTagRepository() *DocumentVersionTagRepository {
	return &DocumentVersionTagRepository{db: repository.DB}
}

func NewDocumentVersionTagRepositoryWithDB(db *gorm.DB) *DocumentVersionTagRepository {
	return &DocumentVersionTagRepository{db: db}
}

func (r *DocumentVersionTagRepository) Create(tag *workspacemodel.DocumentVersionTag) error {
	return r.db.Create(tag).Error
}

func (r *DocumentVersionTagRepository) FindByDocumentID(documentID int64) ([]workspacemodel.DocumentVersionTag, error) {
	var tags []workspacemodel.DocumentVersionTag
	err := r.db.Where("document_id = ?", documentID).
		Order("version DESC, create_time DESC").
		Find(&tags).Error
	return tags, err
}

func (r *DocumentVersionTagRepository) FindByDocumentAndVersion(documentID int64, version int) ([]workspacemodel.DocumentVersionTag, error) {
	var tags []workspacemodel.DocumentVersionTag
	err := r.db.Where("document_id = ? AND version = ?", documentID, version).
		Order("create_time DESC").
		Find(&tags).Error
	return tags, err
}

func (r *DocumentVersionTagRepository) FindByTagName(tagName string, spaceID int64) ([]workspacemodel.DocumentVersionTag, error) {
	var tags []workspacemodel.DocumentVersionTag
	query := r.db.Where("tag_name = ?", tagName)
	if spaceID > 0 {
		query = query.Joins("JOIN aw_document d ON d.document_id = aw_document_version_tag.document_id AND d.space_id = ?", spaceID)
	}
	err := query.Order("create_time DESC").Find(&tags).Error
	return tags, err
}

func (r *DocumentVersionTagRepository) Delete(id int64) error {
	return r.db.Delete(&workspacemodel.DocumentVersionTag{}, id).Error
}

// FindBySpaceID 一次取回空间内全部版本标签。
// 旧 service 层先 GetDistinctTagNames 再对每个名字 FindByTagName（M+N 次查询），
// 提供单查询版本以便一次性取回后在内存分组。
func (r *DocumentVersionTagRepository) FindBySpaceID(spaceID int64) ([]workspacemodel.DocumentVersionTag, error) {
	var tags []workspacemodel.DocumentVersionTag
	err := r.db.Joins("JOIN aw_document d ON d.document_id = aw_document_version_tag.document_id AND d.space_id = ?", spaceID).
		Order("create_time DESC").
		Find(&tags).Error
	return tags, err
}

func (r *DocumentVersionTagRepository) DeleteByDocumentAndVersion(documentID int64, version int) error {
	return r.db.Where("document_id = ? AND version = ?", documentID, version).
		Delete(&workspacemodel.DocumentVersionTag{}).Error
}

func (r *DocumentVersionTagRepository) GetDistinctTagNames(spaceID int64) ([]string, error) {
	var names []string
	err := r.db.Model(&workspacemodel.DocumentVersionTag{}).
		Joins("JOIN aw_document d ON d.document_id = aw_document_version_tag.document_id AND d.space_id = ?", spaceID).
		Distinct("tag_name").
		Pluck("tag_name", &names).Error
	return names, err
}

func (r *DocumentVersionTagRepository) CountByDocument(documentID int64) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.DocumentVersionTag{}).
		Where("document_id = ?", documentID).
		Count(&count).Error
	return count, err
}
