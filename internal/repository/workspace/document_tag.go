package workspace

import (
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"gorm.io/gorm"
)

// DocumentTagRepository 文档标签仓库
type DocumentTagRepository struct {
	db *gorm.DB
}

// NewDocumentTagRepository 创建文档标签仓库实例
func NewDocumentTagRepository(db *gorm.DB) *DocumentTagRepository {
	return &DocumentTagRepository{db: db}
}

// BatchSetTags 批量设置文档标签（先删后增，保证一致性）
func (r *DocumentTagRepository) BatchSetTags(documentID int, tagNames []string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 删除旧标签
		if err := tx.Where("document_id = ?", documentID).Delete(&workspacemodel.DocumentTag{}).Error; err != nil {
			return err
		}
		// 插入新标签
		now := int(time.Now().Unix())
		for _, name := range tagNames {
			if name == "" {
				continue
			}
			dt := workspacemodel.DocumentTag{
				DocumentID: documentID,
				TagName:    name,
				CreateTime: now,
			}
			if err := tx.Create(&dt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// AddTags 为文档追加标签（去重）
func (r *DocumentTagRepository) AddTags(documentID int, tagNames []string) error {
	now := int(time.Now().Unix())
	for _, name := range tagNames {
		if name == "" {
			continue
		}
		dt := workspacemodel.DocumentTag{
			DocumentID: documentID,
			TagName:    name,
			CreateTime: now,
		}
		// 忽略重复（唯一索引冲突）
		r.db.Where("document_id = ? AND tag_name = ?", documentID, name).FirstOrCreate(&dt)
	}
	return nil
}

// RemoveTags 移除文档的指定标签
func (r *DocumentTagRepository) RemoveTags(documentID int, tagNames []string) error {
	return r.db.Where("document_id = ? AND tag_name IN ?", documentID, tagNames).
		Delete(&workspacemodel.DocumentTag{}).Error
}

// GetTagsByDocument 获取文档的所有标签
func (r *DocumentTagRepository) GetTagsByDocument(documentID int) ([]string, error) {
	var tags []string
	err := r.db.Model(&workspacemodel.DocumentTag{}).
		Where("document_id = ?", documentID).
		Order("create_time ASC").
		Pluck("tag_name", &tags).Error
	return tags, err
}

// GetDocumentsByTag 根据标签获取文档 ID 列表
func (r *DocumentTagRepository) GetDocumentsByTag(tagName string, limit int) ([]int, error) {
	var docIDs []int
	q := r.db.Model(&workspacemodel.DocumentTag{}).
		Where("tag_name = ?", tagName).
		Order("create_time DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Pluck("document_id", &docIDs).Error
	return docIDs, err
}

// GetAllTags 获取所有去重标签
func (r *DocumentTagRepository) GetAllTags(keyword string, limit int) ([]string, error) {
	var tags []string
	q := r.db.Model(&workspacemodel.DocumentTag{}).
		Select("DISTINCT tag_name")
	if keyword != "" {
		q = q.Where("tag_name LIKE ?", "%"+keyword+"%")
	}
	q = q.Order("tag_name ASC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Pluck("tag_name", &tags).Error
	return tags, err
}

// GetTagStats 获取标签使用统计
func (r *DocumentTagRepository) GetTagStats(limit int) ([]workspacemodel.TagStats, error) {
	var stats []workspacemodel.TagStats
	q := r.db.Model(&workspacemodel.DocumentTag{}).
		Select("tag_name, COUNT(*) as count").
		Group("tag_name").
		Order("count DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	err := q.Find(&stats).Error
	return stats, err
}

// GetTagsByDocuments 批量获取多个文档的标签（map[documentID][]tagName）
func (r *DocumentTagRepository) GetTagsByDocuments(documentIDs []int) (map[int][]string, error) {
	if len(documentIDs) == 0 {
		return map[int][]string{}, nil
	}
	var rows []workspacemodel.DocumentTag
	err := r.db.Where("document_id IN ?", documentIDs).
		Order("create_time ASC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[int][]string)
	for _, row := range rows {
		result[row.DocumentID] = append(result[row.DocumentID], row.TagName)
	}
	return result, nil
}
