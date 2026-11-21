package admin

import (
	"errors"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

var (
	ErrPluginKeyExists = errors.New("plugin key already exists")
)

// PluginService 插件业务逻辑层
type PluginService struct {
	pluginRepo *adminrepo.PluginRepository
}

func NewPluginService() *PluginService {
	return &PluginService{
		pluginRepo: adminrepo.NewPluginRepository(),
	}
}

// GetPlugins 分页获取插件列表
func (s *PluginService) GetPlugins(keyword string, page, pageSize int) ([]adminmodel.PluginResponse, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	plugins, total, err := s.pluginRepo.FindByKeyword(keyword, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	responses := make([]adminmodel.PluginResponse, len(plugins))
	for i, p := range plugins {
		responses[i] = p.ToResponse()
	}
	return responses, total, nil
}

// GetPlugin 根据 ID 获取插件
func (s *PluginService) GetPlugin(id uint) (adminmodel.PluginResponse, error) {
	plugin, err := s.pluginRepo.FindByID(id)
	if err != nil {
		return adminmodel.PluginResponse{}, err
	}
	return plugin.ToResponse(), nil
}

// CreatePlugin 创建插件
func (s *PluginService) CreatePlugin(req *adminmodel.CreatePluginRequest) (adminmodel.PluginResponse, error) {
	has, err := s.pluginRepo.HasKey(req.PluginKey)
	if err != nil {
		return adminmodel.PluginResponse{}, err
	}
	if has {
		return adminmodel.PluginResponse{}, ErrPluginKeyExists
	}

	plugin := &adminmodel.Plugin{
		PluginKey:   req.PluginKey,
		Title:       req.Title,
		Description: req.Description,
		CoverURL:    req.CoverURL,
		ConfValue:   req.ConfValue,
		IsUsed:      req.IsUsed,
	}
	if err := s.pluginRepo.Create(plugin); err != nil {
		return adminmodel.PluginResponse{}, err
	}
	return plugin.ToResponse(), nil
}

// UpdatePlugin 更新插件
func (s *PluginService) UpdatePlugin(id uint, req *adminmodel.UpdatePluginRequest) (adminmodel.PluginResponse, error) {
	plugin, err := s.pluginRepo.FindByID(id)
	if err != nil {
		return adminmodel.PluginResponse{}, err
	}

	plugin.Title = req.Title
	plugin.Description = req.Description
	plugin.CoverURL = req.CoverURL
	plugin.IsUsed = req.IsUsed

	if err := s.pluginRepo.Update(plugin); err != nil {
		return adminmodel.PluginResponse{}, err
	}
	return plugin.ToResponse(), nil
}

// UpdatePluginConfig 更新插件配置
func (s *PluginService) UpdatePluginConfig(id uint, confValue string) error {
	return s.pluginRepo.UpdateConfValue(id, confValue)
}

// TogglePlugin 切换插件启用状态
func (s *PluginService) TogglePlugin(id uint) (string, error) {
	plugin, err := s.pluginRepo.FindByID(id)
	if err != nil {
		return "", err
	}

	if plugin.IsUsed == 1 {
		if err := s.pluginRepo.SetUnused(id); err != nil {
			return "", err
		}
		return "插件已停用", nil
	}
	if err := s.pluginRepo.SetUsed(id); err != nil {
		return "", err
	}
	return "插件已启用", nil
}

// DeletePlugin 删除插件
func (s *PluginService) DeletePlugin(id uint) error {
	return s.pluginRepo.Delete(id)
}
