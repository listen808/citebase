package admin

import (
	"database/sql"
	"errors"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrConfigNotFound = errors.New("config not found")
)

// ConfigRepository 系统配置数据访问层
type ConfigRepository struct {
	db *gorm.DB
}

func NewConfigRepository() *ConfigRepository {
	return &ConfigRepository{db: repos.DB}
}

func (r *ConfigRepository) FindByID(id int) (*adminmodel.Config, error) {
	var cfg adminmodel.Config
	err := r.db.First(&cfg, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *ConfigRepository) FindByKey(key string) (*adminmodel.Config, error) {
	var cfg adminmodel.Config
	err := r.db.Where("`key` = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrConfigNotFound
		}
		return nil, err
	}
	return &cfg, nil
}

func (r *ConfigRepository) GetValueByKey(key string) (string, error) {
	var cfg adminmodel.Config
	err := r.db.Select("value").Where("`key` = ?", key).First(&cfg).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return cfg.Value, nil
}

func (r *ConfigRepository) GetAllConfigs() ([]*adminmodel.Config, error) {
	var configs []*adminmodel.Config
	err := r.db.Order("`group_key`, sort ASC, config_id ASC").Find(&configs).Error
	return configs, err
}

func (r *ConfigRepository) GetConfigsByGroup(groupKey string) ([]*adminmodel.Config, error) {
	var configs []*adminmodel.Config
	err := r.db.Where("`group_key` = ?", groupKey).Order("sort ASC, config_id ASC").Find(&configs).Error
	return configs, err
}

func (r *ConfigRepository) UpdateValueByKey(key, value string) error {
	return r.db.Model(&adminmodel.Config{}).Where("`key` = ?", key).Update("value", value).Error
}

func (r *ConfigRepository) Update(id int, cfg *adminmodel.Config) error {
	return r.db.Save(cfg).Error
}

func (r *ConfigRepository) Create(cfg *adminmodel.Config) error {
	return r.db.Create(cfg).Error
}
