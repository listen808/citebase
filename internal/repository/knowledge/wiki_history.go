package knowledge

import (
	"gorm.io/gorm"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
)

// WikiPageHistoryRepository 版本历史只增不删：回滚依赖它，删一行就断一段历史。
type WikiPageHistoryRepository struct {
	db *gorm.DB
}

func NewWikiPageHistoryRepository() *WikiPageHistoryRepository {
	return &WikiPageHistoryRepository{db: repos.DB}
}

// ListByPage 按版本号降序返回某页的全部历史版本。
func (r *WikiPageHistoryRepository) ListByPage(spaceID int, slug string) ([]*knowledgemodel.WikiPageHistory, error) {
	var rows []*knowledgemodel.WikiPageHistory
	err := r.db.Where("space_id = ? AND slug = ?", spaceID, slug).
		Order("version DESC").Find(&rows).Error
	return rows, err
}

// Find 取某页的指定版本。
func (r *WikiPageHistoryRepository) Find(spaceID int, slug string, version int) (*knowledgemodel.WikiPageHistory, error) {
	var row knowledgemodel.WikiPageHistory
	err := r.db.Where("space_id = ? AND slug = ? AND version = ?", spaceID, slug, version).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}
