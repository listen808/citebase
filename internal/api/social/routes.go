package social

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes 注册收藏、关注、评论、通知相关路由（需要认证 + RBAC）
func RegisterRoutes(v1 *gin.RouterGroup) {
	collectionAPI := NewCollectionAPI()
	followAPI := NewFollowAPI()
	commentAPI := NewCommentAPI()
	notifAPI := NewNotificationAPI()

	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware(), middleware.AutoRBAC())
	{
		// 收藏
		authGroup.POST("/collections/toggle", collectionAPI.ToggleCollection)
		authGroup.GET("/collections", collectionAPI.GetMyCollections)
		authGroup.GET("/collections/check", collectionAPI.IsCollected)

		// 关注
		authGroup.POST("/follow/user", followAPI.ToggleFollowUser)
		authGroup.POST("/follow/document", followAPI.ToggleFollowDocument)
		authGroup.GET("/follow/following", followAPI.GetFollowingList)
		authGroup.GET("/follow/followers", followAPI.GetFollowersList)
		authGroup.GET("/follow/stats", followAPI.GetFollowStats)
		authGroup.GET("/follow/check", followAPI.CheckFollowStatus)

		// 评论
		authGroup.POST("/comments", commentAPI.Create)
		authGroup.GET("/comments/:document_id", commentAPI.List)
		authGroup.DELETE("/comments/:id", commentAPI.Delete)
		authGroup.PUT("/comments/:id/resolve", commentAPI.Resolve)

		// 通知
		authGroup.GET("/notifications", notifAPI.List)
		authGroup.GET("/notifications/unread-count", notifAPI.UnreadCount)
		authGroup.PUT("/notifications/:id/read", notifAPI.MarkAsRead)
		authGroup.PUT("/notifications/read-all", notifAPI.MarkAllAsRead)
		authGroup.DELETE("/notifications/:id", notifAPI.Delete)
	}
}
