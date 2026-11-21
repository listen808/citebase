package workspace

import (
	"errors"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var ErrTranslationNotFound = errors.New("translation not found")

// DocumentTranslationRepository 文档翻译仓储
type DocumentTranslationRepository struct {
	db *gorm.DB
}

func NewDocumentTranslationRepository() *DocumentTranslationRepository {
	return &DocumentTranslationRepository{db: repos.DB}
}

// Create 创建翻译关联
func (r *DocumentTranslationRepository) Create(t *workspacemodel.DocumentTranslation) error {
	return r.db.Create(t).Error
}

// GetByDocumentID 获取文档的所有翻译版本
func (r *DocumentTranslationRepository) GetByDocumentID(documentID int) ([]*workspacemodel.DocumentTranslation, error) {
	var translations []*workspacemodel.DocumentTranslation
	err := r.db.Where("document_id = ?", documentID).Find(&translations).Error
	return translations, err
}

// GetByDocumentAndLang 获取指定语言的翻译
func (r *DocumentTranslationRepository) GetByDocumentAndLang(documentID int, lang string) (*workspacemodel.DocumentTranslation, error) {
	var t workspacemodel.DocumentTranslation
	err := r.db.Where("document_id = ? AND lang = ?", documentID, lang).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTranslationNotFound
		}
		return nil, err
	}
	return &t, nil
}

// Update 更新翻译关联
func (r *DocumentTranslationRepository) Update(t *workspacemodel.DocumentTranslation) error {
	return r.db.Model(&workspacemodel.DocumentTranslation{}).
		Where("id = ?", t.ID).
		Updates(map[string]interface{}{
			"translated_document_id": t.TranslatedDocID,
			"translation_status":     t.TranslationStatus,
		}).Error
}

// Delete 删除翻译关联
func (r *DocumentTranslationRepository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&workspacemodel.DocumentTranslation{}).Error
}

// GetTranslations 获取翻译组（包括反向关联）
func (r *DocumentTranslationRepository) GetTranslationGroup(documentID int) ([]*workspacemodel.DocumentTranslation, error) {
	var translations []*workspacemodel.DocumentTranslation
	// 查询 document_id = ? 或 translated_document_id = ? 的所有记录
	err := r.db.Where("document_id = ? OR translated_document_id = ?", documentID, documentID).
		Find(&translations).Error
	return translations, err
}
