package workspace

import (
	"time"

	"gorm.io/gorm"
)

// WebhookConfig represents a configured webhook endpoint.
type WebhookConfig struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int64  `gorm:"column:user_id;not null;index:idx_webhook_user" json:"user_id"`
	SpaceID    int64  `gorm:"column:space_id;not null;default:0;index:idx_webhook_space" json:"space_id"`
	Name       string `gorm:"column:name;size:100;not null" json:"name"`
	URL        string `gorm:"column:url;size:500;not null" json:"url"`
	Secret     string `gorm:"column:secret;size:100;default:''" json:"secret"`
	Events     string `gorm:"column:events;size:500;not null" json:"events"` // comma-separated: doc.created,doc.updated,doc.deleted,comment.created
	IsActive   int    `gorm:"column:is_active;default:1" json:"is_active"`
	CreateTime int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
	UpdateTime int64  `gorm:"column:update_time;not null;default:0" json:"update_time"`
}

func (WebhookConfig) TableName() string {
	return "aw_webhook_config"
}

func (w *WebhookConfig) BeforeCreate(tx *gorm.DB) error {
	if w.CreateTime == 0 {
		w.CreateTime = time.Now().Unix()
	}
	if w.UpdateTime == 0 {
		w.UpdateTime = time.Now().Unix()
	}
	return nil
}

func (w *WebhookConfig) BeforeUpdate(tx *gorm.DB) error {
	w.UpdateTime = time.Now().Unix()
	return nil
}

// WebhookDelivery represents a delivery attempt for a webhook.
type WebhookDelivery struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	WebhookID  int64  `gorm:"column:webhook_id;not null;index:idx_delivery_webhook" json:"webhook_id"`
	Event      string `gorm:"column:event;size:50;not null" json:"event"`
	Payload    string `gorm:"column:payload;type:text" json:"payload"`
	StatusCode int    `gorm:"column:status_code;not null;default:0" json:"status_code"`
	Response   string `gorm:"column:response;size:1000;default:''" json:"response"`
	Success    int    `gorm:"column:success;default:0" json:"success"`
	RetryCount int    `gorm:"column:retry_count;default:0" json:"retry_count"`
	CreateTime int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
}

func (WebhookDelivery) TableName() string {
	return "aw_webhook_delivery"
}

func (d *WebhookDelivery) BeforeCreate(tx *gorm.DB) error {
	if d.CreateTime == 0 {
		d.CreateTime = time.Now().Unix()
	}
	return nil
}

// Webhook event constants
const (
	WebhookEventDocCreated        = "doc.created"
	WebhookEventDocUpdated        = "doc.updated"
	WebhookEventDocDeleted        = "doc.deleted"
	WebhookEventCommentCreated    = "comment.created"
	WebhookEventApprovalSubmitted = "approval.submitted"
	WebhookEventApprovalHandled   = "approval.handled"
)

// WebhookConfigResponse is the API response for webhook config.
type WebhookConfigResponse struct {
	ID         int64  `json:"id"`
	UserID     int64  `json:"user_id"`
	SpaceID    int64  `json:"space_id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Secret     string `json:"secret,omitempty"`
	Events     string `json:"events"`
	IsActive   int    `json:"is_active"`
	CreateTime int64  `json:"create_time"`
	UpdateTime int64  `json:"update_time"`
}

// WebhookDeliveryResponse is the API response for webhook delivery.
type WebhookDeliveryResponse struct {
	ID         int64  `json:"id"`
	WebhookID  int64  `json:"webhook_id"`
	Event      string `json:"event"`
	StatusCode int    `json:"status_code"`
	Response   string `json:"response"`
	Success    int    `json:"success"`
	RetryCount int    `json:"retry_count"`
	CreateTime int64  `json:"create_time"`
}

// CreateWebhookRequest is the request body for creating a webhook.
type CreateWebhookRequest struct {
	Name    string `json:"name" binding:"required,max=100"`
	URL     string `json:"url" binding:"required,url,max=500"`
	Secret  string `json:"secret" binding:"max=100"`
	Events  string `json:"events" binding:"required"`
	SpaceID int64  `json:"space_id"`
}

// UpdateWebhookRequest is the request body for updating a webhook.
type UpdateWebhookRequest struct {
	Name     string `json:"name" binding:"omitempty,max=100"`
	URL      string `json:"url" binding:"omitempty,url,max=500"`
	Secret   string `json:"secret" binding:"max=100"`
	Events   string `json:"events"`
	IsActive *int   `json:"is_active" binding:"omitempty,oneof=0 1"`
}
