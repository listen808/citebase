package workspace

import (
	"fmt"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// DocumentTagService 文档标签服务
type DocumentTagService struct {
	tagRepo *workspacerepo.DocumentTagRepository
}

// NewDocumentTagService 创建文档标签服务实例
func NewDocumentTagService(tagRepo *workspacerepo.DocumentTagRepository) *DocumentTagService {
	return &DocumentTagService{tagRepo: tagRepo}
}

// SetDocumentTags 设置文档标签（全量替换）
func (s *DocumentTagService) SetDocumentTags(documentID int, tagNames []string) error {
	if documentID <= 0 {
		return fmt.Errorf("invalid document id")
	}
	return s.tagRepo.BatchSetTags(documentID, tagNames)
}

// AddDocumentTags 为文档追加标签
func (s *DocumentTagService) AddDocumentTags(documentID int, tagNames []string) error {
	if documentID <= 0 {
		return fmt.Errorf("invalid document id")
	}
	return s.tagRepo.AddTags(documentID, tagNames)
}

// RemoveDocumentTags 移除文档标签
func (s *DocumentTagService) RemoveDocumentTags(documentID int, tagNames []string) error {
	if documentID <= 0 {
		return fmt.Errorf("invalid document id")
	}
	return s.tagRepo.RemoveTags(documentID, tagNames)
}

// GetDocumentTags 获取文档的所有标签
func (s *DocumentTagService) GetDocumentTags(documentID int) ([]string, error) {
	if documentID <= 0 {
		return []string{}, fmt.Errorf("invalid document id")
	}
	return s.tagRepo.GetTagsByDocument(documentID)
}

// GetAllTags 获取所有标签（可选关键字过滤）
func (s *DocumentTagService) GetAllTags(keyword string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.tagRepo.GetAllTags(keyword, limit)
}

// GetTagStats 获取标签统计
func (s *DocumentTagService) GetTagStats(limit int) ([]workspacemodel.TagStats, error) {
	if limit <= 0 {
		limit = 20
	}
	return s.tagRepo.GetTagStats(limit)
}

// GetDocumentsByTag 根据标签获取文档 ID 列表
func (s *DocumentTagService) GetDocumentsByTag(tagName string, limit int) ([]int, error) {
	if tagName == "" {
		return []int{}, fmt.Errorf("tag name is required")
	}
	return s.tagRepo.GetDocumentsByTag(tagName, limit)
}

// GetTagsByDocuments 批量获取多个文档的标签
func (s *DocumentTagService) GetTagsByDocuments(documentIDs []int) (map[int][]string, error) {
	return s.tagRepo.GetTagsByDocuments(documentIDs)
}
