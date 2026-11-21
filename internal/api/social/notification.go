package social

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/repository"
	"github.com/listen808/citebase/internal/response"
	socialsvc "github.com/listen808/citebase/internal/service/social"
	"github.com/listen808/citebase/internal/utils"
)

// NotificationAPI 通知 API
type NotificationAPI struct {
	notifService *socialsvc.NotificationService
}

// NewNotificationAPI 创建通知 API 实例
func NewNotificationAPI() *NotificationAPI {
	return &NotificationAPI{
		notifService: socialsvc.NewNotificationService(repository.DB),
	}
}

// List 获取通知列表
// GET /notifications?type=all|unread&page=&page_size=
func (api *NotificationAPI) List(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	notifType := c.DefaultQuery("type", "all")
	page, pageSize, _ := utils.ParsePagination(c, 20)

	if notifType == "unread" {
		notifications, total, err := api.notifService.GetUnreadNotifications(uid, page, pageSize)
		if err != nil {
			response.ServerErrorSafe(c, err)
			return
		}
		response.Paginated(c, notifications, total, page, pageSize)
		return
	}

	notifications, total, err := api.notifService.GetNotifications(uid, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, notifications, total, page, pageSize)
}

// UnreadCount 获取未读通知数
// GET /notifications/unread-count
func (api *NotificationAPI) UnreadCount(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	count, err := api.notifService.GetUnreadCount(uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"count": count})
}

// MarkAsRead 标记通知已读
// PUT /notifications/:id/read
func (api *NotificationAPI) MarkAsRead(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid notification id")
		return
	}

	if err := api.notifService.MarkAsRead(id, uid); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "标记已读成功")
}

// MarkAllAsRead 标记所有通知已读
// PUT /notifications/read-all
func (api *NotificationAPI) MarkAllAsRead(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	if err := api.notifService.MarkAllAsRead(uid); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "全部标记已读成功")
}

// Delete 删除通知
// DELETE /notifications/:id
func (api *NotificationAPI) Delete(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid notification id")
		return
	}

	if err := api.notifService.DeleteNotification(id, uid); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "删除成功")
}
