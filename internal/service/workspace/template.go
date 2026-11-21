package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// TemplateService 文档模板服务
type TemplateService struct {
	tplRepo      *workspacerepo.DocumentTemplateRepository
	spaceService *SpaceService
}

// NewTemplateService 创建服务实例
func NewTemplateService() *TemplateService {
	return &TemplateService{
		tplRepo:      workspacerepo.NewDocumentTemplateRepository(),
		spaceService: NewSpaceService(),
	}
}

// Create 创建模板。必须校验创建者对目标空间有编辑权，
// 否则任何人可向他人 spaceID 植入模板（后续被他人插入文档造成内容污染）。
func (s *TemplateService) Create(userID int, req *workspacemodel.CreateTemplateRequest) (*workspacemodel.DocumentTemplate, error) {
	if req.SpaceID > 0 {
		privilege, err := s.spaceService.GetUserSpacePrivilege(req.SpaceID, userID)
		if err != nil {
			return nil, err
		}
		if privilege < 1 {
			return nil, ErrNoEditPermission
		}
	}
	t := &workspacemodel.DocumentTemplate{
		SpaceID:      req.SpaceID,
		Name:         req.Name,
		Description:  req.Description,
		Content:      req.Content,
		CreateUserID: userID,
	}
	if err := s.tplRepo.Create(t); err != nil {
		return nil, err
	}
	return t, nil
}

// Update 更新模板
func (s *TemplateService) Update(id int, req *workspacemodel.UpdateTemplateRequest) error {
	return s.tplRepo.Update(id, req)
}

// Delete 删除模板
func (s *TemplateService) Delete(id int) error {
	return s.tplRepo.Delete(id)
}

// Get 获取模板
func (s *TemplateService) Get(id int) (*workspacemodel.DocumentTemplate, error) {
	return s.tplRepo.FindByID(id)
}

// List 列出模板
func (s *TemplateService) List(spaceID, page, pageSize int) ([]workspacemodel.TemplateResponse, int64, error) {
	list, total, err := s.tplRepo.ListBySpace(spaceID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]workspacemodel.TemplateResponse, len(list))
	for i, t := range list {
		resp[i] = t.ToResponse()
	}
	return resp, total, nil
}
