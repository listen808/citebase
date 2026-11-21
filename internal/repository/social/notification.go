package social

import (
	"time"

	socialmodel "github.com/listen808/citebase/internal/model/social"
	"gorm.io/gorm"
)

// NotificationRepository 通知仓库
type NotificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository 创建通知仓库实例
func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create 创建通知
func (r *NotificationRepository) Create(n *socialmodel.Notification) error {
	now := int(time.Now().Unix())
	n.CreateTime = now
	n.UpdateTime = now
	return r.db.Create(n).Error
}

// GetByUserID 获取用户通知列表（分页）
func (r *NotificationRepository) GetByUserID(userID int, page, pageSize int) ([]socialmodel.Notification, int64, error) {
	var notifications []socialmodel.Notification
	var total int64
	r.db.Model(&socialmodel.Notification{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.Where("user_id = ?", userID).
		Order("create_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&notifications).Error
	return notifications, total, err
}

// GetUnreadByUserID 获取用户未读通知
func (r *NotificationRepository) GetUnreadByUserID(userID int, page, pageSize int) ([]socialmodel.Notification, int64, error) {
	var notifications []socialmodel.Notification
	var total int64
	r.db.Model(&socialmodel.Notification{}).Where("user_id = ? AND is_read = 0", userID).Count(&total)
	err := r.db.Where("user_id = ? AND is_read = 0", userID).
		Order("create_time DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&notifications).Error
	return notifications, total, err
}

// MarkAsRead 标记单条通知已读
func (r *NotificationRepository) MarkAsRead(id int, userID int) error {
	return r.db.Model(&socialmodel.Notification{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_read", 1).Error
}

// MarkAllAsRead 标记用户所有通知已读
func (r *NotificationRepository) MarkAllAsRead(userID int) error {
	return r.db.Model(&socialmodel.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Update("is_read", 1).Error
}

// GetUnreadCount 获取用户未读通知数
func (r *NotificationRepository) GetUnreadCount(userID int) (int64, error) {
	var count int64
	err := r.db.Model(&socialmodel.Notification{}).
		Where("user_id = ? AND is_read = 0", userID).
		Count(&count).Error
	return count, err
}

// Delete 删除通知
func (r *NotificationRepository) Delete(id int, userID int) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).
		Delete(&socialmodel.Notification{}).Error
}

// BatchDelete 批量删除通知
func (r *NotificationRepository) BatchDelete(ids []int, userID int) error {
	return r.db.Where("id IN ? AND user_id = ?", ids, userID).
		Delete(&socialmodel.Notification{}).Error
}
