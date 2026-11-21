package social

import (
	"fmt"

	"github.com/listen808/citebase/internal/api/websocket"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	"gorm.io/gorm"
)

// NotificationService 通知服务
type NotificationService struct {
	notifRepo *socialrepo.NotificationRepository
}

// NewNotificationService 创建通知服务实例
func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		notifRepo: socialrepo.NewNotificationRepository(db),
	}
}

// CreateNotification 创建通知并实时推送
func (s *NotificationService) CreateNotification(userID int, notifType, title, content string, relatedID int) error {
	if userID <= 0 {
		return fmt.Errorf("invalid user id")
	}
	n := &socialmodel.Notification{
		UserID:    userID,
		Type:      notifType,
		Title:     title,
		Content:   content,
		RelatedID: relatedID,
	}
	if err := s.notifRepo.Create(n); err != nil {
		return err
	}

	// 实时 WebSocket 推送（best-effort，失败不影响主流程）
	hub := websocket.GetHub()
	hub.NotifyUser(userID, title, content, notifType, relatedID)
	if unreadCount, err := s.notifRepo.GetUnreadCount(userID); err == nil {
		hub.NotifyUnreadCount(userID, unreadCount)
	}
	return nil
}

// GetNotifications 获取通知列表
func (s *NotificationService) GetNotifications(userID int, page, pageSize int) ([]socialmodel.NotificationResponse, int64, error) {
	notifications, total, err := s.notifRepo.GetByUserID(userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]socialmodel.NotificationResponse, len(notifications))
	for i, n := range notifications {
		resp[i] = n.ToResponse()
	}
	return resp, total, nil
}

// GetUnreadNotifications 获取未读通知列表
func (s *NotificationService) GetUnreadNotifications(userID int, page, pageSize int) ([]socialmodel.NotificationResponse, int64, error) {
	notifications, total, err := s.notifRepo.GetUnreadByUserID(userID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]socialmodel.NotificationResponse, len(notifications))
	for i, n := range notifications {
		resp[i] = n.ToResponse()
	}
	return resp, total, nil
}

// MarkAsRead 标记通知已读
func (s *NotificationService) MarkAsRead(id int, userID int) error {
	return s.notifRepo.MarkAsRead(id, userID)
}

// MarkAllAsRead 标记所有通知已读
func (s *NotificationService) MarkAllAsRead(userID int) error {
	return s.notifRepo.MarkAllAsRead(userID)
}

// GetUnreadCount 获取未读通知数
func (s *NotificationService) GetUnreadCount(userID int) (int64, error) {
	return s.notifRepo.GetUnreadCount(userID)
}

// DeleteNotification 删除通知
func (s *NotificationService) DeleteNotification(id int, userID int) error {
	return s.notifRepo.Delete(id, userID)
}
