package workspace

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils/netguard"
	"github.com/listen808/citebase/internal/utils/safego"
)

var ErrWebhookNotFound = errors.New("webhook not found")
var ErrWebhookNotOwner = errors.New("not the owner of this webhook")
var ErrWebhookURLForbidden = errors.New("webhook URL 不被允许：仅支持 http/https，且目标不得为内网/环回/链路本地地址")

// WebhookService handles webhook business logic.
type WebhookService struct {
	repo         *workspacerepo.WebhookRepository
	httpClient   *http.Client
	spaceService *SpaceService
}

func NewWebhookService() *WebhookService {
	return &WebhookService{
		repo:         workspacerepo.NewWebhookRepository(),
		httpClient:   newWebhookHTTPClient(),
		spaceService: NewSpaceService(),
	}
}

// isBlockedWebhookIP 判定内网/特殊用途地址。
// 判据本体在 netguard.BlockedIP —— 抓取器与 webhook 必须共用同一份，
// 否则一边加了元数据地址、另一边没加，就是两条不同强度的内网防线。
// 这一侧多拦一条：云元数据端点（169.254.169.254 落在 link-local 段里，
// netguard 已按点名拦掉，这里不再重复列）。
func isBlockedWebhookIP(ip net.IP) bool {
	return netguard.BlockedIP(ip)
}

// validateWebhookURL 校验用户提交的回调地址：scheme 仅 http/https；
// host 为 IP 字面量时直接判定，域名在建立连接时由 DialContext 复核（防 DNS 指向内网）。
func validateWebhookURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return errors.New("webhook URL 格式无效")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrWebhookURLForbidden
	}
	if host := u.Hostname(); host != "" {
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedWebhookIP(ip) {
				return ErrWebhookURLForbidden
			}
		}
	}
	return nil
}

// newWebhookHTTPClient 出网客户端：
//  1. 连接建立前解析目标 IP 并拒绝环回/私网/链路本地/元数据地址；
//  2. **连的就是刚判定过的那个 IP**（netguard.DialContext 里钉住），
//     否则"校验时解析一次、连接时再解析一次"就是 DNS 重绑定（TTL 1 秒）的标准打法；
//  3. 每一跳重定向都复核目标地址，防止 3xx 引导到 127.0.0.1 或云元数据端点。
func newWebhookHTTPClient() *http.Client {
	return netguard.Client(10*time.Second, 10)
}

// CreateWebhook creates a new webhook configuration.
func (s *WebhookService) CreateWebhook(userID int64, req *workspacemodel.CreateWebhookRequest) (*workspacemodel.WebhookConfigResponse, error) {
	// URL 由用户任意指定且服务端会主动 POST 出去，必须先过 SSRF 校验
	if err := validateWebhookURL(req.URL); err != nil {
		return nil, err
	}
	// 创建者必须对该 spaceID 有编辑权：否则可在他人空间挂 webhook，
	// 空间事件（文档标题等内容）会被外传到攻击者地址
	if req.SpaceID > 0 {
		privilege, err := s.spaceService.GetUserSpacePrivilege(int(req.SpaceID), int(userID))
		if err != nil {
			return nil, err
		}
		if privilege < 1 {
			return nil, ErrNoEditPermission
		}
	}

	config := &workspacemodel.WebhookConfig{
		UserID:   userID,
		SpaceID:  req.SpaceID,
		Name:     req.Name,
		URL:      req.URL,
		Secret:   req.Secret,
		Events:   req.Events,
		IsActive: 1,
	}

	if err := s.repo.CreateConfig(config); err != nil {
		return nil, err
	}

	return s.toConfigResponse(config), nil
}

// GetUserWebhooks returns all webhooks for a user.
func (s *WebhookService) GetUserWebhooks(userID int64) ([]workspacemodel.WebhookConfigResponse, error) {
	configs, err := s.repo.FindConfigsByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.WebhookConfigResponse, 0, len(configs))
	for i := range configs {
		result = append(result, *s.toConfigResponse(&configs[i]))
	}
	return result, nil
}

// GetWebhook returns a webhook by ID.
// 必须校验归属：否则任何登录用户可枚举 ID 读取他人 webhook 的回调 URL/事件配置。
// 非本人一律按「不存在」处理，避免泄露他人 webhook 是否存在。
func (s *WebhookService) GetWebhook(userID, id int64) (*workspacemodel.WebhookConfigResponse, error) {
	config, err := s.repo.FindConfigByID(id)
	if err != nil {
		return nil, ErrWebhookNotFound
	}
	if config.UserID != userID {
		return nil, ErrWebhookNotFound
	}
	return s.toConfigResponse(config), nil
}

// UpdateWebhook updates a webhook configuration.
func (s *WebhookService) UpdateWebhook(userID, id int64, req *workspacemodel.UpdateWebhookRequest) (*workspacemodel.WebhookConfigResponse, error) {
	config, err := s.repo.FindConfigByID(id)
	if err != nil {
		return nil, ErrWebhookNotFound
	}

	if config.UserID != userID {
		return nil, ErrWebhookNotOwner
	}

	if req.Name != "" {
		config.Name = req.Name
	}
	if req.URL != "" {
		// 更新同样必须过 SSRF 校验：否则可把已审批的 webhook 改指内网地址
		if err := validateWebhookURL(req.URL); err != nil {
			return nil, err
		}
		config.URL = req.URL
	}
	if req.Secret != "" {
		config.Secret = req.Secret
	}
	if req.Events != "" {
		config.Events = req.Events
	}
	if req.IsActive != nil {
		config.IsActive = *req.IsActive
	}

	if err := s.repo.UpdateConfig(config); err != nil {
		return nil, err
	}

	return s.toConfigResponse(config), nil
}

// DeleteWebhook deletes a webhook configuration.
func (s *WebhookService) DeleteWebhook(userID, id int64) error {
	config, err := s.repo.FindConfigByID(id)
	if err != nil {
		return ErrWebhookNotFound
	}

	if config.UserID != userID {
		return ErrWebhookNotOwner
	}

	return s.repo.DeleteConfig(id)
}

// TriggerWebhooks fires webhooks for a given event.
func (s *WebhookService) TriggerWebhooks(event string, spaceID int64, payload interface{}) {
	configs, err := s.repo.FindActiveConfigsByEvent(event, spaceID)
	if err != nil {
		logx.Warnf("[Webhook] Failed to find configs for event %s: %v", event, err)
		return
	}

	for _, config := range configs {
		safego.Go("deliverWebhook", func() { s.deliverWebhook(config, event, payload) })
	}
}

// GetWebhookDeliveries returns recent deliveries for a webhook.
// delivery.Response 存有远端返回内容（SSRF 场景下即内网响应回显），
// 因此同样先做归属校验，非本人按「不存在」处理。
func (s *WebhookService) GetWebhookDeliveries(userID, webhookID int64, limit int) ([]workspacemodel.WebhookDeliveryResponse, error) {
	config, err := s.repo.FindConfigByID(webhookID)
	if err != nil || config.UserID != userID {
		return nil, ErrWebhookNotFound
	}
	deliveries, err := s.repo.FindDeliveriesByWebhookID(webhookID, limit)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.WebhookDeliveryResponse, 0, len(deliveries))
	for i := range deliveries {
		result = append(result, workspacemodel.WebhookDeliveryResponse{
			ID:         deliveries[i].ID,
			WebhookID:  deliveries[i].WebhookID,
			Event:      deliveries[i].Event,
			StatusCode: deliveries[i].StatusCode,
			Response:   deliveries[i].Response,
			Success:    deliveries[i].Success,
			RetryCount: deliveries[i].RetryCount,
			CreateTime: deliveries[i].CreateTime,
		})
	}
	return result, nil
}

func (s *WebhookService) deliverWebhook(config workspacemodel.WebhookConfig, event string, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		logx.Warnf("[Webhook] Failed to marshal payload: %v", err)
		return
	}

	delivery := &workspacemodel.WebhookDelivery{
		WebhookID: config.ID,
		Event:     event,
		Payload:   string(body),
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", config.URL, bytes.NewReader(body))
	if err != nil {
		logx.Warnf("[Webhook] Failed to create request: %v", err)
		delivery.StatusCode = 0
		delivery.Response = err.Error()
		_ = s.repo.CreateDelivery(delivery)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", event)
	req.Header.Set("X-Webhook-ID", fmt.Sprintf("%d", config.ID))

	if config.Secret != "" {
		sig := computeHMAC(body, config.Secret)
		req.Header.Set("X-Webhook-Signature", sig)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		logx.Warnf("[Webhook] Delivery failed: %v", err)
		delivery.StatusCode = 0
		delivery.Response = err.Error()
		_ = s.repo.CreateDelivery(delivery)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1000))

	delivery.StatusCode = resp.StatusCode
	delivery.Response = string(respBody)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		delivery.Success = 1
	}

	_ = s.repo.CreateDelivery(delivery)
}

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *WebhookService) toConfigResponse(config *workspacemodel.WebhookConfig) *workspacemodel.WebhookConfigResponse {
	// Mask secret for response
	secret := config.Secret
	if len(secret) > 4 {
		secret = secret[:4] + "****"
	}

	return &workspacemodel.WebhookConfigResponse{
		ID:         config.ID,
		UserID:     config.UserID,
		SpaceID:    config.SpaceID,
		Name:       config.Name,
		URL:        config.URL,
		Secret:     secret,
		Events:     config.Events,
		IsActive:   config.IsActive,
		CreateTime: config.CreateTime,
		UpdateTime: config.UpdateTime,
	}
}

// SupportedWebhookEvents returns the list of supported webhook events.
func SupportedWebhookEvents() []string {
	return []string{
		workspacemodel.WebhookEventDocCreated,
		workspacemodel.WebhookEventDocUpdated,
		workspacemodel.WebhookEventDocDeleted,
		workspacemodel.WebhookEventCommentCreated,
		workspacemodel.WebhookEventApprovalSubmitted,
		workspacemodel.WebhookEventApprovalHandled,
	}
}

// WebhookEventPayload 是发给用户端点的事件体。字段是稳定契约：
// 用户那边的自动化就按这些 key 取值，改名字等于把别人的集成打碎。
type WebhookEventPayload struct {
	Event        string `json:"event"`
	SpaceID      int64  `json:"space_id"`
	DocumentID   int64  `json:"document_id,omitempty"`
	DocumentName string `json:"document_name,omitempty"`
	CommentID    int64  `json:"comment_id,omitempty"`
	ApprovalID   int64  `json:"approval_id,omitempty"`
	ActorID      int64  `json:"actor_id,omitempty"`
	Timestamp    int64  `json:"timestamp"`
}

// webhookNotifier 是事件点用到的那一份服务。
// 用 sync.Once 而不是每个事件点 New 一份：NewWebhookService() 会连带建 SpaceService
// 与 http.Client，而一次文档写入可能同时触发六个事件点。
var (
	webhookFireOnce sync.Once
	webhookFiresvc  *WebhookService
)

func webhookService() *WebhookService {
	webhookFireOnce.Do(func() { webhookFiresvc = NewWebhookService() })
	return webhookFiresvc
}

// FireWebhook 由事件点调用，把事件异步派发出去。
//
// 这条函数存在的唯一理由是：TriggerWebhooks 以前**零调用方** —— 用户能在
// 「工具 → Webhook 嵌入」里建配置、能查投递记录、事件列表也照着六个事件写，
// 但没有任何一处代码真的发过。所以它看起来是个功能，实际是个空壳。
//
// 三个必须待在一起的边界：
//   - 数据库还没就绪时直接返回（启动早期与单测里 repo.DB 为 nil，
//     建服务会 panic 在一个"通知没发出去"的场合，不值得）；
//   - 派发是异步的，所以协程必须自带 recover —— 这里 panic 发生在 gin Recovery
//     之外，未捕获就是整进程退出（与 approval.go 的 notifySubmitterSafe 同一条理由）；
//   - 事件名先过一遍白名单：payload 里的 space_id 决定谁的配置被命中，
//     传错事件名不会报错、只会静默不投递。
func FireWebhook(event string, spaceID int64, fields func(*WebhookEventPayload)) {
	if repos.DB == nil {
		return
	}
	known := false
	for _, e := range SupportedWebhookEvents() {
		if e == event {
			known = true
			break
		}
	}
	if !known {
		logx.Warnf("[Webhook] 未知事件名 %q，未派发（SupportedWebhookEvents 里没有这一条）", event)
		return
	}

	payload := WebhookEventPayload{Event: event, SpaceID: spaceID, Timestamp: time.Now().Unix()}
	if fields != nil {
		fields(&payload)
	}

	svc := webhookService()
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("[Webhook] 派发 %s 时 panic: %v", event, r)
			}
		}()
		svc.TriggerWebhooks(event, spaceID, payload)
	}()
}
