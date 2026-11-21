package collab

import (
	"github.com/gin-gonic/gin"
	collabws "github.com/listen808/citebase/internal/collab"
	"github.com/listen808/citebase/internal/middleware"
)

// RegisterRoutes registers collaborative editing routes.
func RegisterRoutes(r *gin.Engine, v1 *gin.RouterGroup) {
	hub := collabws.NewHub()
	docManager := collabws.GetDocManager()
	server := collabws.NewServer(hub, docManager)

	// WebSocket endpoint requires auth via token query param
	// Using gin.Engine directly because WebSocket doesn't work well with middleware groups
	// P1 修复：AccessLog 已提升为全局中间件（server.go setupGin），此处移除避免重复记录
	authGroup := v1.Group("")
	authGroup.Use(middleware.AuthMiddleware())
	{
		// REST endpoints for collab session management
		authGroup.GET("/collab/sessions/:documentId", server.GetSessions)
		authGroup.GET("/collab/state/:documentId", server.GetState)
	}

	// WebSocket endpoint (auth via query param, handled inside the handler)
	r.GET("/api/v1/collab/ws/:documentId", server.HandleWebSocket)
}
