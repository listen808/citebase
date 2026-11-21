package admin

import (
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

type ConfigService struct {
	configRepo *adminrepo.ConfigRepository
}

func NewConfigService() *ConfigService {
	return &ConfigService{
		configRepo: adminrepo.NewConfigRepository(),
	}
}

func (s *ConfigService) GetAllConfigs() ([]*adminmodel.ConfigResponse, error) {
	configs, err := s.configRepo.GetAllConfigs()
	if err != nil {
		return nil, err
	}
	responses := make([]*adminmodel.ConfigResponse, len(configs))
	for i, c := range configs {
		resp := c.ToResponse()
		responses[i] = &resp
	}
	return responses, nil
}

func (s *ConfigService) GetConfigsByGroup(groupKey string) ([]*adminmodel.ConfigResponse, error) {
	configs, err := s.configRepo.GetConfigsByGroup(groupKey)
	if err != nil {
		return nil, err
	}
	responses := make([]*adminmodel.ConfigResponse, len(configs))
	for i, c := range configs {
		resp := c.ToResponse()
		responses[i] = &resp
	}
	return responses, nil
}

func (s *ConfigService) UpdateConfig(key string, value string) error {
	return s.configRepo.UpdateValueByKey(key, value)
}

func (s *ConfigService) GetConfigValue(key string) (string, error) {
	return s.configRepo.GetValueByKey(key)
}

type ConfigGroupResponse struct {
	GroupKey   string                       `json:"group_key"`
	GroupLabel string                       `json:"group_label"`
	Items      []*adminmodel.ConfigResponse `json:"items"`
}
