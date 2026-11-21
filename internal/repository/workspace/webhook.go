package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type WebhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository() *WebhookRepository {
	return &WebhookRepository{db: repository.DB}
}

func NewWebhookRepositoryWithDB(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

func (r *WebhookRepository) CreateConfig(config *workspacemodel.WebhookConfig) error {
	return r.db.Create(config).Error
}

func (r *WebhookRepository) FindConfigByID(id int64) (*workspacemodel.WebhookConfig, error) {
	var config workspacemodel.WebhookConfig
	err := r.db.Where("id = ?", id).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

func (r *WebhookRepository) FindConfigsByUserID(userID int64) ([]workspacemodel.WebhookConfig, error) {
	var configs []workspacemodel.WebhookConfig
	err := r.db.Where("user_id = ?", userID).Order("create_time DESC").Find(&configs).Error
	return configs, err
}

func (r *WebhookRepository) FindConfigsBySpaceID(spaceID int64) ([]workspacemodel.WebhookConfig, error) {
	var configs []workspacemodel.WebhookConfig
	err := r.db.Where("space_id = ? OR space_id = 0", spaceID).Where("is_active = 1").Find(&configs).Error
	return configs, err
}

func (r *WebhookRepository) FindActiveConfigsByEvent(event string, spaceID int64) ([]workspacemodel.WebhookConfig, error) {
	var configs []workspacemodel.WebhookConfig
	err := r.db.Where("is_active = 1 AND (space_id = ? OR space_id = 0) AND events LIKE ?", spaceID, "%"+event+"%").
		Find(&configs).Error
	return configs, err
}

func (r *WebhookRepository) UpdateConfig(config *workspacemodel.WebhookConfig) error {
	return r.db.Save(config).Error
}

func (r *WebhookRepository) DeleteConfig(id int64) error {
	return r.db.Delete(&workspacemodel.WebhookConfig{}, id).Error
}

func (r *WebhookRepository) CreateDelivery(delivery *workspacemodel.WebhookDelivery) error {
	return r.db.Create(delivery).Error
}

func (r *WebhookRepository) FindDeliveriesByWebhookID(webhookID int64, limit int) ([]workspacemodel.WebhookDelivery, error) {
	var deliveries []workspacemodel.WebhookDelivery
	err := r.db.Where("webhook_id = ?", webhookID).
		Order("create_time DESC").
		Limit(limit).
		Find(&deliveries).Error
	return deliveries, err
}

func (r *WebhookRepository) UpdateDelivery(delivery *workspacemodel.WebhookDelivery) error {
	return r.db.Save(delivery).Error
}

func (r *WebhookRepository) DeleteExpiredDeliveries(before int64) error {
	return r.db.Where("create_time < ?", before).Delete(&workspacemodel.WebhookDelivery{}).Error
}
