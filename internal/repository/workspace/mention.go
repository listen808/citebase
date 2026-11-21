package workspace

import (
	"errors"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// ErrMentionNotFound 提及记录不存在
var ErrMentionNotFound = errors.New("mention not found")

// DocumentMentionRepository @提及 数据访问层
type DocumentMentionRepository struct {
	db *gorm.DB
}

// NewDocumentMentionRepository 创建仓储实例
func NewDocumentMentionRepository() *DocumentMentionRepository {
	return &DocumentMentionRepository{db: repos.DB}
}

// ReplaceByDocument 替换某文档的全部提及记录（先删后插，保证与内容一致）。
func (r *DocumentMentionRepository) ReplaceByDocument(docID int, userIDs []int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("document_id = ?", docID).Delete(&workspacemodel.DocumentMention{}).Error; err != nil {
			return err
		}
		now := int(time.Now().Unix())
		for _, uid := range userIDs {
			if uid <= 0 {
				continue
			}
			m := &workspacemodel.DocumentMention{DocumentID: docID, UserID: uid, CreateTime: now}
			if err := tx.Create(m).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// FindByDocument 获取文档的提及用户 ID 列表
func (r *DocumentMentionRepository) FindByDocument(docID int) ([]int, error) {
	var mentions []workspacemodel.DocumentMention
	if err := r.db.Where("document_id = ?", docID).Find(&mentions).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(mentions))
	for _, m := range mentions {
		ids = append(ids, m.UserID)
	}
	return ids, nil
}

// FindByUser 获取 @ 了某用户的所有文档 ID（分页）
func (r *DocumentMentionRepository) FindByUser(userID, offset, limit int) ([]int, int64, error) {
	var total int64
	r.db.Model(&workspacemodel.DocumentMention{}).Where("user_id = ?", userID).Count(&total)
	var mentions []workspacemodel.DocumentMention
	err := r.db.Where("user_id = ?", userID).Order("create_time DESC").
		Offset(offset).Limit(limit).Find(&mentions).Error
	if err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(mentions))
	for _, m := range mentions {
		ids = append(ids, m.DocumentID)
	}
	return ids, total, nil
}
