package collab

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware/jwt"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
)

// Server is the Yjs-compatible WebSocket server for collaborative editing.
type Server struct {
	hub  *Hub
	docs *DocManager
}

// NewServer creates a new collaborative editing server.
func NewServer(hub *Hub, docs *DocManager) *Server {
	return &Server{
		hub:  hub,
		docs: docs,
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	// CheckOrigin 防 CSWSH 劫持：Origin 头为空时放行（非浏览器客户端），
	// 否则必须与请求 Host 同源
	CheckOrigin: checkSameOrigin,
}

// checkSameOrigin 校验 WebSocket 升级请求的 Origin 是否与 Host 同源
// 防 CSWSH（Cross-Site WebSocket Hijacking）攻击
// 比较 hostname（不含端口）：dev 环境下 Vite proxy 会将 Host 改为后端端口，
// 但浏览器 Origin 仍是前端端口，若比较完整 host 会导致 dev 环境连接失败。
// 生产环境同域反代，hostname 比较与完整 host 比较等价，安全性不受影响。
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端（如 curl）无 Origin 头，放行；鉴权依赖 token 校验
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	reqHost := r.Host
	if h, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
	}
	return strings.EqualFold(originHost, reqHost)
}

// HandleWebSocket upgrades an HTTP connection to WebSocket and registers the client.
// Endpoint: GET /api/v1/collab/ws/:documentId?token=xxx
func (s *Server) HandleWebSocket(c *gin.Context) {
	// Parse document ID from URL
	docIDStr := c.Param("documentId")
	docID, err := strconv.ParseUint(docIDStr, 10, 64)
	if err != nil || docID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document ID"})
		return
	}

	// Authenticate via token query param or Authorization header
	token := c.Query("token")
	if token == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authentication token"})
		return
	}

	// 校验 token 是否已在黑名单中（用户已登出）
	if authsvc.IsTokenBlacklisted(token) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token has been revoked"})
		return
	}

	claims, err := jwt.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// P0 修复：检查用户是否被禁用或删除（与 AuthMiddleware 保持一致）
	user, err := userrepo.NewUserRepository().FindByID(claims.UserID)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	if user.IsForbidden == 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "user is forbidden"})
		return
	}
	if user.IsDelete == 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user account deleted"})
		return
	}

	// P0 修复：校验用户对该文档所属空间的访问权限，避免任意用户加入任意文档协作
	docIDInt := int(docID)
	doc, err := workspacerepo.NewDocumentRepository().FindByID(docIDInt)
	if err != nil || doc == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		return
	}
	spaceID := doc.SpaceID
	spaceRepo := workspacerepo.NewSpaceRepository()
	space, err := spaceRepo.FindByIDCached(spaceID)
	if err != nil || space == nil || space.IsDelete == 1 {
		c.JSON(http.StatusNotFound, gin.H{"error": "space not found"})
		return
	}
	// 公开空间放行
	if !space.IsPublic() {
		// 非公开空间：超级管理员放行，否则需为空间成员
		if claims.RoleID != 1 {
			spaceUser, err := spaceRepo.FindSpaceUser(spaceID, claims.UserID)
			if err != nil || spaceUser == nil {
				c.JSON(http.StatusForbidden, gin.H{"error": "no access permission to this space"})
				return
			}
		}
	}

	userID := uint(claims.UserID)
	userName := claims.Username

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logx.Infof("[Collab] WebSocket upgrade failed for user %d: %v", userID, err)
		return
	}

	// Initialize document state if not already loaded
	// P0：加载失败（数据库错误）时只记录告警 —— ApplyUpdate/persist 在加载成功前不会覆盖已有状态
	if err := s.docs.InitDoc(uint(docID)); err != nil {
		logx.Warnf("[Collab] Init doc %d state failed: %v", docID, err)
	}

	// Generate a color for the user's cursor
	color := generateColor()

	client := &Client{
		hub:      s.hub,
		conn:     conn,
		userID:   userID,
		userName: userName,
		color:    color,
		docID:    uint(docID),
		send:     make(chan []byte, 256),
		done:     make(chan struct{}),
	}

	s.hub.register <- client

	// Send the current document state to the new client as SyncStep1
	// This asks the client to compute the diff and send it back
	state := s.docs.GetDocState(uint(docID))
	if len(state) > 0 {
		syncMsg := EncodeSyncStep1(state)
		// P0：直接发送会在客户端注销后触发 send-on-closed-channel panic（且在 gin Recovery 之外）
		if !client.trySend(syncMsg) {
			logx.Infof("[Collab] Drop SyncStep1 for user %d doc %d (closed or buffer full)", userID, docID)
		}
	}

	// Start read/write goroutines
	go client.writePump()
	go client.readPump()

	// Monitor for disconnect to trigger doc cleanup
	go func() {
		<-client.done
		if s.hub.DocClientCount(uint(docID)) == 0 {
			s.docs.OnDocEmpty(uint(docID))
		}
	}()

	logx.Infof("[Collab] User %d (%s) connected to doc %d via WebSocket", userID, userName, docID)
}

// GetSessions returns the active collab sessions for a document.
// Endpoint: GET /api/v1/collab/sessions/:documentId
func (s *Server) GetSessions(c *gin.Context) {
	docIDStr := c.Param("documentId")
	docID, err := strconv.ParseUint(docIDStr, 10, 64)
	if err != nil || docID == 0 {
		response.BadRequest(c, "invalid document ID")
		return
	}

	sessions := s.hub.GetDocSessions(uint(docID))
	response.Success(c, gin.H{
		"document_id": docID,
		"sessions":    sessions,
		"count":       len(sessions),
	})
}

// GetState returns the collab state info for a document.
// Endpoint: GET /api/v1/collab/state/:documentId
func (s *Server) GetState(c *gin.Context) {
	docIDStr := c.Param("documentId")
	docID, err := strconv.ParseUint(docIDStr, 10, 64)
	if err != nil || docID == 0 {
		response.BadRequest(c, "invalid document ID")
		return
	}

	stateLen, version, active := s.docs.GetDocInfo(uint(docID))
	clientCount := s.hub.DocClientCount(uint(docID))

	response.Success(c, gin.H{
		"document_id":  docID,
		"active":       active,
		"state_length": stateLen,
		"version":      version,
		"clients":      clientCount,
	})
}

// generateColor creates a random pastel color for cursor display.
func generateColor() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	// Generate pastel colors (higher RGB values)
	r := 128 + int(b[0])/2
	g := 128 + int(b[1])/2
	bl := 128 + int(b[2])/2
	return fmt.Sprintf("#%02x%02x%02x", r, g, bl)
}

// Shutdown gracefully shuts down the collab server.
func (s *Server) Shutdown() {
	s.docs.Stop()
	logx.Info("[Collab] Server shutdown complete")
}

// ShutdownGlobal 停掉包级单例的落盘循环，供进程优雅关停调用。
//
// 为什么需要这条而不只是 Server.Shutdown()：collab.Server 是在
// internal/api/collab/routes.go 里就地 new 出来的局部变量，app/server 拿不到它，
// 于是原先 Server.Shutdown() 零调用方 —— 协作编辑的内存 Yjs 状态只有在
// 进程恰好不崩时才可能被保存。DocManager 是单例，所以退出路径按单例关就够了。
func ShutdownGlobal() {
	GetDocManager().Stop()
}
