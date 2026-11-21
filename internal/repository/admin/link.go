package admin

import (
	"database/sql"
	"errors"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrLinkNotFound = errors.New("link not found")
)

// LinkRepository 快捷链接数据访问层
type LinkRepository struct {
	db *gorm.DB
}

func NewLinkRepository() *LinkRepository {
	return &LinkRepository{db: repos.DB}
}

func (r *LinkRepository) Create(link *adminmodel.Link) error {
	return r.db.Create(link).Error
}

func (r *LinkRepository) Update(link *adminmodel.Link) error {
	return r.db.Save(link).Error
}

func (r *LinkRepository) Delete(linkID uint) error {
	return r.db.Model(&adminmodel.Link{}).Where("link_id = ?", linkID).Update("is_delete", 1).Error
}

func (r *LinkRepository) FindByID(linkID uint) (*adminmodel.Link, error) {
	var link adminmodel.Link
	err := r.db.Where("link_id = ?", linkID).Scopes(repos.NotDeleted).First(&link).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLinkNotFound
		}
		return nil, ErrLinkNotFound
	}
	return &link, nil
}

func (r *LinkRepository) GetByName(name string) (*adminmodel.Link, error) {
	var link adminmodel.Link
	err := r.db.Where("name = ?", name).Scopes(repos.NotDeleted).First(&link).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrLinkNotFound
		}
		return nil, ErrLinkNotFound
	}
	return &link, nil
}

func (r *LinkRepository) ExistsName(linkID uint, name string) (bool, error) {
	var count int64
	query := r.db.Model(&adminmodel.Link{}).Where("name = ?", name).Scopes(repos.NotDeleted)
	if linkID > 0 {
		query = query.Where("link_id != ?", linkID)
	}
	err := query.Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LinkRepository) List(page, pageSize int, keyword string) ([]adminmodel.Link, int64, error) {
	var links []adminmodel.Link
	var total int64

	query := r.db.Model(&adminmodel.Link{}).Scopes(repos.NotDeleted)
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}

	// 尝试 Count，如果表不存在返回空
	countErr := query.Count(&total).Error
	if countErr != nil {
		return []adminmodel.Link{}, 0, nil
	}

	offset := (page - 1) * pageSize
	err := query.Order("link_id DESC").Offset(offset).Limit(pageSize).Find(&links).Error
	if err != nil {
		return []adminmodel.Link{}, 0, nil
	}

	if len(links) == 0 {
		return []adminmodel.Link{}, 0, nil
	}

	return links, total, nil
}

// IncrementClick 原子自增点击次数并返回自增后的值。
// 不能"读-改-写"：两个人同时点同一条链接会互相吞掉一次；也不能不看 is_delete，
// 否则已删除的链接还能被计数。
func (r *LinkRepository) IncrementClick(linkID uint) (int, error) {
	res := r.db.Model(&adminmodel.Link{}).
		Where("link_id = ? AND is_delete = 0", linkID).
		UpdateColumn("click_count", gorm.Expr("click_count + 1"))
	if res.Error != nil {
		return 0, res.Error
	}
	if res.RowsAffected == 0 {
		return 0, ErrLinkNotFound
	}
	var link adminmodel.Link
	if err := r.db.Select("click_count").Where("link_id = ?", linkID).First(&link).Error; err != nil {
		return 0, ErrLinkNotFound
	}
	return link.ClickCount, nil
}

func (r *LinkRepository) ListAll() ([]adminmodel.Link, error) {
	var links []adminmodel.Link
	err := r.db.Scopes(repos.NotDeleted).Order("sort ASC").Find(&links).Error
	if err != nil {
		return []adminmodel.Link{}, nil
	}
	if len(links) == 0 {
		return []adminmodel.Link{}, nil
	}
	return links, nil
}
