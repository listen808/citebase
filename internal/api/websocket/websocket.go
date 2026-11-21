package websocket

import (
	"encoding/json"
	logx "github.com/listen808/citebase/internal/log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/listen808/citebase/internal/middleware/jwt"
	"github.com/listen808/citebase/internal/utils/safego"
)

// Hub WebSocket 连接管理中心
type Hub struct {
	mu         sync.RWMutex
	clients    map[int]map[*Client]bool // userID -> clients
	register   chan *Client
	unregister chan *Client
}

// Client 单个 WebSocket 客户端
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID int
	send   chan []byte
	done   chan struct{}
	// closeOnce 幂等保护 done 的关闭。send 永不关闭：任何 goroutine 向已关闭
	// channel 发送都会 panic（且发生在 gin Recovery 之外 → 整个进程死）。
	closeOnce sync.Once
}

// closeDone 幂等关闭 done（仅由 hub 单 goroutine 调用）。
// 关闭后 writePump 退出，trySend 立即失败，不再有并发关闭/发送竞争。
func (c *Client) closeDone() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// trySend 非阻塞发送：客户端已注销或缓冲区满时返回 false
func (c *Client) trySend(data []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- data:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

// Message WebSocket 消息格式
type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp,omitempty"`
}

var (
	instance *Hub
	once     sync.Once
)

// GetHub 获取全局 Hub 实例
func GetHub() *Hub {
	once.Do(func() {
		instance = &Hub{
			clients:    make(map[int]map[*Client]bool),
			register:   make(chan *Client),
			unregister: make(chan *Client),
		}
		safego.Go("run", func() { instance.run() })
	})
	return instance
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.userID] == nil {
				h.clients[client.userID] = make(map[*Client]bool)
			}
			h.clients[client.userID][client] = true
			h.mu.Unlock()
			logx.Infof("[WS] User %d connected (total: %d)", client.userID, h.UserCount(client.userID))

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			// P0：只关闭 done（幂等），不关闭 send，避免其它 goroutine 命中
			// send-on-closed-channel panic；writePump 看到 done 后自行退出。
			client.closeDone()
			h.mu.Unlock()
			logx.Infof("[WS] User %d disconnected (total: %d)", client.userID, h.UserCount(client.userID))
		}
	}
}

// UserCount 获取指定用户的连接数
func (h *Hub) UserCount(userID int) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID])
}

// SendToUser 向指定用户的所有连接发送消息
func (h *Hub) SendToUser(userID int, msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		logx.Infof("[WS] Failed to marshal message: %v", err)
		return
	}

	h.mu.RLock()
	clients, ok := h.clients[userID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// 复制一份避免持锁发送
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		// P0：走 trySend，注销中的连接不会再向 send 写入
		if !c.trySend(data) {
			logx.Infof("[WS] Dropping message for user %d (closed or buffer full)", userID)
		}
	}
}

// Broadcast 向所有连接广播消息
func (h *Hub) Broadcast(msg *Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	targets := make([]*Client, 0)
	for _, clients := range h.clients {
		for c := range clients {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.trySend(data)
	}
}

// NotifyUser 发送通知类型消息给指定用户
func (h *Hub) NotifyUser(userID int, title, content, notifType string, relatedID int) {
	h.SendToUser(userID, &Message{
		Type: "notification",
		Data: map[string]interface{}{
			"title":      title,
			"content":    content,
			"notif_type": notifType,
			"related_id": relatedID,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

// NotifyUnreadCount 发送未读数更新给指定用户
func (h *Hub) NotifyUnreadCount(userID int, count int64) {
	h.SendToUser(userID, &Message{
		Type: "unread_count",
		Data: map[string]interface{}{
			"count": count,
		},
		Timestamp: time.Now().Format(time.RFC3339),
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// P0 修复：防 CSWSH（Cross-Site WebSocket Hijacking）
	// Origin 为空时放行（非浏览器客户端），否则必须与 Host 同源
	CheckOrigin: checkSameOrigin,
}

// checkSameOrigin 校验 WebSocket 升级请求的 Origin 是否与 Host 同源
// 比较 hostname（不含端口）：dev 环境下 Vite proxy 会将 Host 改为后端端口，
// 但浏览器 Origin 仍是前端端口，若比较完整 host 会导致 dev 环境连接失败。
// 生产环境同域反代，hostname 比较与完整 host 比较等价，安全性不受影响。
func checkSameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Hostname()
	reqHost := r.Host
	// r.Host 可能含端口，用 SplitHostPort 提取 hostname
	if h, _, err := net.SplitHostPort(reqHost); err == nil {
		reqHost = h
	}
	return strings.EqualFold(originHost, reqHost)
}

// HandleWS 处理 WebSocket 连接
func HandleWS(w http.ResponseWriter, r *http.Request) {
	// 从 query param 获取 token
	token := r.URL.Query().Get("token")
	if token == "" {
		// 尝试从 Authorization header 获取（某些客户端不支持 query param）
		authHeader := r.Header.Get("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token = authHeader[7:]
		}
	}

	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	// 验证 JWT token
	claims, err := jwt.ParseToken(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	userID := claims.UserID

	// 升级 HTTP 连接为 WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logx.Infof("[WS] Upgrade failed: %v", err)
		return
	}

	hub := GetHub()
	client := &Client{
		hub:    hub,
		conn:   conn,
		userID: userID,
		send:   make(chan []byte, 256),
		done:   make(chan struct{}),
	}

	hub.register <- client

	// 启动读写协程
	safego.Go("writePump", func() { client.writePump() })
	safego.Go("readPump", func() { client.readPump() })
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logx.Infof("[WS] Read error from user %d: %v", c.userID, err)
			}
			break
		}

		// 处理客户端消息（目前只处理 ping）
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}
		if msg.Type == "ping" {
			c.hub.SendToUser(c.userID, &Message{Type: "pong"})
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			// send 永不关闭（只关闭 done），注销统一由下面 <-c.done 分支收尾
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-c.done:
			_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
