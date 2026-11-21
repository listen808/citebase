package admin

import (
	"database/sql"
	"errors"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"time"
)

var (
	ErrPluginNotFound = errors.New("plugin not found")
)

// PluginRepository 插件数据访问层
type PluginRepository struct {
	db *gorm.DB
}

func NewPluginRepository() *PluginRepository {
	return &PluginRepository{db: repos.DB}
}

// GetAll 获取所有插件
func (r *PluginRepository) GetAll() ([]*adminmodel.Plugin, error) {
	var plugins []*adminmodel.Plugin
	err := r.db.Order("plugin_id DESC").Find(&plugins).Error
	return plugins, err
}

// GetAllMap 获取所有插件的 key-map
func (r *PluginRepository) GetAllMap() (map[string]*adminmodel.Plugin, error) {
	plugins, err := r.GetAll()
	if err != nil {
		return nil, err
	}
	m := make(map[string]*adminmodel.Plugin)
	for _, p := range plugins {
		m[p.PluginKey] = p
	}
	return m, nil
}

// FindByKeyword 分页搜索
func (r *PluginRepository) FindByKeyword(keyword string, offset, limit int) ([]*adminmodel.Plugin, int64, error) {
	var plugins []*adminmodel.Plugin
	var total int64
	query := r.db.Model(&adminmodel.Plugin{})
	if keyword != "" {
		query = query.Where("title LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("plugin_id DESC").Offset(offset).Limit(limit).Find(&plugins).Error; err != nil {
		return nil, 0, err
	}
	return plugins, total, nil
}

// GetByID 根据 ID 获取
func (r *PluginRepository) FindByID(id uint) (*adminmodel.Plugin, error) {
	var plugin adminmodel.Plugin
	err := r.db.First(&plugin, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPluginNotFound
		}
		return nil, err
	}
	return &plugin, nil
}

// GetByKey 根据 plugin_key 获取
func (r *PluginRepository) GetByKey(key string) (*adminmodel.Plugin, error) {
	var plugin adminmodel.Plugin
	err := r.db.Where("plugin_key = ?", key).First(&plugin).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPluginNotFound
		}
		return nil, err
	}
	return &plugin, nil
}

// Create 创建插件
func (r *PluginRepository) Create(p *adminmodel.Plugin) error {
	return r.db.Create(p).Error
}

// Update 更新插件基本信息
func (r *PluginRepository) Update(p *adminmodel.Plugin) error {
	return r.db.Save(p).Error
}

// UpdateConfValue 更新插件配置
func (r *PluginRepository) UpdateConfValue(id uint, confValue string) error {
	return r.db.Model(&adminmodel.Plugin{}).Where("plugin_id = ?", id).
		Updates(map[string]interface{}{
			"conf_value":  confValue,
			"update_time": time.Now().Unix(),
		}).Error
}

// SetUsed 启用插件
func (r *PluginRepository) SetUsed(id uint) error {
	return r.db.Model(&adminmodel.Plugin{}).Where("plugin_id = ?", id).
		Updates(map[string]interface{}{
			"is_used":     1,
			"update_time": time.Now().Unix(),
		}).Error
}

// SetUnused 停用插件
func (r *PluginRepository) SetUnused(id uint) error {
	return r.db.Model(&adminmodel.Plugin{}).Where("plugin_id = ?", id).
		Updates(map[string]interface{}{
			"is_used":     0,
			"update_time": time.Now().Unix(),
		}).Error
}

// Delete 删除插件
func (r *PluginRepository) Delete(id uint) error {
	return r.db.Delete(&adminmodel.Plugin{}, id).Error
}

// HasKey 检查 plugin_key 是否存在
func (r *PluginRepository) HasKey(key string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.Plugin{}).Where("plugin_key = ?", key).Count(&count).Error
	return count > 0, err
}

// Count 统计插件总数
func (r *PluginRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&adminmodel.Plugin{}).Count(&count).Error
	return count, err
}
