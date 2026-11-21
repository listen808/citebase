package collab

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
	logx "github.com/listen808/citebase/internal/log"
)

// Hub manages all collaborative editing connections.
// It tracks which clients are editing which documents and routes messages between them.
type Hub struct {
	mu         sync.RWMutex
	documents  map[uint]map[*Client]bool // docID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *DocMessage
}

// Client represents a single WebSocket connection editing a specific document.
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	userID   uint
	userName string
	color    string
	docID    uint
	send     chan []byte
	done     chan struct{}
	// closeOnce 保护 done 的关闭（P0：send 永不关闭，只关闭 done，
	// 避免任意 goroutine 向已关闭 channel 发送导致进程 panic）
	closeOnce sync.Once
}

// closeDone 幂等关闭 done，仅由 hub 的单 goroutine 调用。
// 关闭后 writePump 退出，所有 trySend 立即失败，不会再向 send 写入。
func (c *Client) closeDone() {
	c.closeOnce.Do(func() {
		close(c.done)
	})
}

// trySend 非阻塞向客户端发送。客户端已注销（done 关闭）或发送缓冲区满时返回 false。
// 取代原先各处对 c.send 的直接发送（含 readPump 中的裸阻塞发送）。
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

// DocMessage is a message targeted at a specific document.
type DocMessage struct {
	DocID  uint
	Sender *Client
	Data   []byte
}

// NewHub creates a new Hub and starts its event loop.
func NewHub() *Hub {
	h := &Hub{
		documents:  make(map[uint]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *DocMessage, 256),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.documents[client.docID] == nil {
				h.documents[client.docID] = make(map[*Client]bool)
			}
			h.documents[client.docID][client] = true
			h.mu.Unlock()
			logx.Infof("[Collab] User %d (%s) joined doc %d (clients: %d)",
				client.userID, client.userName, client.docID, h.DocClientCount(client.docID))

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.documents[client.docID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					if len(clients) == 0 {
						delete(h.documents, client.docID)
					}
				}
			}
			// P0：不再 close(client.send)。只关闭 done —— send 的关闭会让
			// 其它 goroutine（广播 / readPump / 注册路径）出现 send-on-closed-channel panic。
			// writePump 观察到 done 关闭后自行退出并关闭连接。
			client.closeDone()
			h.mu.Unlock()
			logx.Infof("[Collab] User %d (%s) left doc %d (clients: %d)",
				client.userID, client.userName, client.docID, h.DocClientCount(client.docID))

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.documents[msg.DocID]
			targets := make([]*Client, 0, len(clients))
			for c := range clients {
				if c != msg.Sender {
					targets = append(targets, c)
				}
			}
			h.mu.RUnlock()

			for _, c := range targets {
				if !c.trySend(msg.Data) {
					logx.Infof("[Collab] Dropping message for user %d doc %d (closed or buffer full)", c.userID, c.docID)
				}
			}
		}
	}
}

// DocClientCount returns the number of clients editing a specific document.
func (h *Hub) DocClientCount(docID uint) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.documents[docID])
}

// DocClients returns a snapshot of clients editing a document.
func (h *Hub) DocClients(docID uint) []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := h.documents[docID]
	result := make([]*Client, 0, len(clients))
	for c := range clients {
		result = append(result, c)
	}
	return result
}

// BroadcastToDoc sends a message to all clients editing a document except the sender.
func (h *Hub) BroadcastToDoc(docID uint, sender *Client, data []byte) {
	h.broadcast <- &DocMessage{
		DocID:  docID,
		Sender: sender,
		Data:   data,
	}
}

// BroadcastToDocAll sends a message to all clients editing a document including the sender.
func (h *Hub) BroadcastToDocAll(docID uint, data []byte) {
	h.mu.RLock()
	clients := h.documents[docID]
	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		// 在调用方 goroutine 中发送：必须走 trySend，避免向已注销客户端的 send 写入
		c.trySend(data)
	}
}

// ClientInfo represents a connected user's information for the API response.
type ClientInfo struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"user_name"`
	Color    string `json:"color"`
	JoinedAt string `json:"joined_at"`
}

// GetDocSessions returns information about all clients editing a document.
func (h *Hub) GetDocSessions(docID uint) []ClientInfo {
	h.mu.RLock()
	defer h.mu.RUnlock()
	clients := h.documents[docID]
	result := make([]ClientInfo, 0, len(clients))
	for c := range clients {
		result = append(result, ClientInfo{
			UserID:   c.userID,
			UserName: c.userName,
			Color:    c.color,
			JoinedAt: time.Now().Format(time.RFC3339), // simplified
		})
	}
	return result
}

// readPump reads messages from the WebSocket connection and routes them.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	// Yjs updates can be large; set a reasonable limit (5 MB)
	c.conn.SetReadLimit(5 * 1024 * 1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logx.Infof("[Collab] Read error user %d doc %d: %v", c.userID, c.docID, err)
			}
			break
		}

		// Parse the Yjs protocol message
		msgType, payload, err := DecodeMessage(message)
		if err != nil {
			logx.Infof("[Collab] Invalid message from user %d: %v", c.userID, err)
			continue
		}

		switch msgType {
		case MsgSync:
			subType, data, err := DecodeSyncMessage(payload)
			if err != nil {
				logx.Infof("[Collab] Invalid sync message from user %d: %v", c.userID, err)
				continue
			}

			switch subType {
			case SyncStep1:
				// Client requests our state. Send the full document state as SyncStep2.
				state := GetDocManager().GetDocState(c.docID)
				if len(state) > 0 {
					resp := EncodeSyncStep2(state)
					// P0：原来是无 select 的阻塞发送，缓冲区写满时会卡死读循环，
					// 且客户端注销后可能向已关闭 channel 发送；改为非阻塞 trySend。
					if !c.trySend(resp) {
						logx.Infof("[Collab] Drop SyncStep2 for user %d doc %d (closed or buffer full)", c.userID, c.docID)
					}
				}

			case SyncStep2:
				// Client sends us their full state (response to our SyncStep1).
				// Apply the update to our document state.
				if err := GetDocManager().ApplyUpdate(c.docID, data); err != nil {
					logx.Warnf("[Collab] Apply update failed for doc %d: %v", c.docID, err)
					continue
				}
				// Broadcast to other clients
				broadcastMsg := EncodeSyncUpdate(data)
				c.hub.BroadcastToDoc(c.docID, c, broadcastMsg)

			case SyncUpdate:
				// Incremental update from client.
				if err := GetDocManager().ApplyUpdate(c.docID, data); err != nil {
					logx.Warnf("[Collab] Apply update failed for doc %d: %v", c.docID, err)
					continue
				}
				broadcastMsg := EncodeSyncUpdate(data)
				c.hub.BroadcastToDoc(c.docID, c, broadcastMsg)
			}

		case MsgAwareness:
			// Awareness messages are broadcast to all other clients of the same doc.
			c.hub.BroadcastToDoc(c.docID, c, message)
		}
	}
}

// writePump writes messages from the send channel to the WebSocket connection.
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
			// send 通道不会被 close（只 close done），因此 !ok 永不发生，
			// 注销统一由下面的 <-c.done 分支收尾，避免 send-on-closed-channel。
			// Yjs uses binary WebSocket messages
			if err := c.conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
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
