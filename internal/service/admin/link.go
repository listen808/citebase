package admin

import (
	"errors"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

var (
	ErrLinkNameExists = errors.New("link name already exists")
)

// LinkService 快捷链接业务逻辑层
type LinkService struct {
	linkRepo *adminrepo.LinkRepository
}

func NewLinkService() *LinkService {
	return &LinkService{
		linkRepo: adminrepo.NewLinkRepository(),
	}
}

// GetLinks 分页获取快捷链接列表
func (s *LinkService) GetLinks(page, pageSize int, keyword string) ([]adminmodel.Link, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.linkRepo.List(page, pageSize, keyword)
}

// GetAllLinks 获取全部有效快捷链接
func (s *LinkService) GetAllLinks() ([]adminmodel.Link, error) {
	return s.linkRepo.ListAll()
}

// CreateLink 创建快捷链接
func (s *LinkService) CreateLink(name, url string, sort int) (*adminmodel.Link, error) {
	exists, err := s.linkRepo.ExistsName(0, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrLinkNameExists
	}

	link := &adminmodel.Link{
		Name: name,
		URL:  url,
		Sort: sort,
	}
	if err := s.linkRepo.Create(link); err != nil {
		return nil, err
	}
	return link, nil
}

// UpdateLink 更新快捷链接
func (s *LinkService) UpdateLink(id uint, name, url string, sort int) (*adminmodel.Link, error) {
	link, err := s.linkRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	exists, err := s.linkRepo.ExistsName(id, name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrLinkNameExists
	}

	link.Name = name
	link.URL = url
	link.Sort = sort
	if err := s.linkRepo.Update(link); err != nil {
		return nil, err
	}
	return link, nil
}

// RecordClick 记一次链接打开，返回累计次数。
func (s *LinkService) RecordClick(id uint) (int, error) {
	return s.linkRepo.IncrementClick(id)
}

// DeleteLink 删除快捷链接
func (s *LinkService) DeleteLink(id uint) error {
	if _, err := s.linkRepo.FindByID(id); err != nil {
		return err
	}
	return s.linkRepo.Delete(id)
}
