package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DocumentLinkRepository struct {
	db *gorm.DB
}

func NewDocumentLinkRepository() *DocumentLinkRepository {
	return &DocumentLinkRepository{db: repos.DB}
}

func (r *DocumentLinkRepository) ReplaceLinks(sourceDocID int, spaceID int, links []*workspacemodel.DocumentLink) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("source_doc_id = ?", sourceDocID).Delete(&workspacemodel.DocumentLink{}).Error; err != nil {
			return err
		}
		if len(links) == 0 {
			return nil
		}
		for i := range links {
			links[i].SourceDocID = sourceDocID
			links[i].SpaceID = spaceID
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(links).Error
	})
}

func (r *DocumentLinkRepository) DeleteBySource(sourceDocID int) error {
	return r.db.Where("source_doc_id = ?", sourceDocID).Delete(&workspacemodel.DocumentLink{}).Error
}

func (r *DocumentLinkRepository) DeleteByTarget(targetDocID int) error {
	return r.db.Where("target_doc_id = ?", targetDocID).Delete(&workspacemodel.DocumentLink{}).Error
}

type BacklinkResult struct {
	SourceDocID   int    `json:"source_doc_id"`
	SourceDocName string `json:"source_doc_name"`
	SpaceID       int    `json:"space_id"`
	SpaceName     string `json:"space_name"`
	LinkText      string `json:"link_text"`
}

func (r *DocumentLinkRepository) GetBacklinks(targetDocID int) ([]*BacklinkResult, error) {
	var results []*BacklinkResult
	err := r.db.Table("aw_document_link").
		Select("aw_document_link.source_doc_id, aw_document.name as source_doc_name, aw_document_link.space_id, aw_space.name as space_name, aw_document_link.link_text").
		Joins("LEFT JOIN aw_document ON aw_document_link.source_doc_id = aw_document.document_id").
		Joins("LEFT JOIN aw_space ON aw_document_link.space_id = aw_space.space_id").
		Where("aw_document_link.target_doc_id = ?", targetDocID).
		Scan(&results).Error
	return results, err
}

type ForwardLinkResult struct {
	TargetDocID   int    `json:"target_doc_id"`
	TargetDocName string `json:"target_doc_name"`
	SpaceID       int    `json:"space_id"`
	SpaceName     string `json:"space_name"`
	LinkText      string `json:"link_text"`
}

func (r *DocumentLinkRepository) GetForwardLinks(sourceDocID int) ([]*ForwardLinkResult, error) {
	var results []*ForwardLinkResult
	err := r.db.Table("aw_document_link").
		Select("aw_document_link.target_doc_id, aw_document.name as target_doc_name, aw_document_link.space_id, aw_space.name as space_name, aw_document_link.link_text").
		Joins("LEFT JOIN aw_document ON aw_document_link.target_doc_id = aw_document.document_id").
		Joins("LEFT JOIN aw_space ON aw_document_link.space_id = aw_space.space_id").
		Where("aw_document_link.source_doc_id = ?", sourceDocID).
		Scan(&results).Error
	return results, err
}
