package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/ctxkeys"
	logx "github.com/listen808/citebase/internal/log"
	"sync"
	"time"
)

type Message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"` // S16: 兼容string和[{type,image_url}]多模态
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
}

// NewTextMessage 创建纯文本消息（兼容旧代码）
func NewTextMessage(role, content string) Message {
	return Message{Role: role, Content: json.RawMessage(`"` + jsonEscape(content) + `"`)}
}

// NewImageMessage 创建带图片的消息（多模态）
func NewImageMessage(role, text, imageURL string) Message {
	parts := []map[string]interface{}{
		{"type": "text", "text": text},
		{"type": "image_url", "image_url": map[string]string{"url": imageURL}},
	}
	data, _ := json.Marshal(parts)
	return Message{Role: role, Content: data}
}

// TextContent 提取纯文本内容（如果Content是字符串则直接返回，否则返回空）
func (m Message) TextContent() string {
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s
	}
	return ""
}

// NewToolCallMessage 创建包含工具调用的 assistant 消息
func NewToolCallMessage(content string, toolCalls []ToolCall) Message {
	c := json.RawMessage(`"` + jsonEscape(content) + `"`)
	return Message{Role: "assistant", Content: c, ToolCalls: toolCalls}
}

// NewToolResultMessage 创建工具结果的 user/tool 消息
func NewToolResultMessage(toolCallID, content string) Message {
	return Message{Role: "user", Content: mustRawJSON(content), ToolCallID: toolCallID}
}

// jsonEscape 简易JSON字符串转义
func jsonEscape(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

type LLMResponse struct {
	Content    string
	TokenUsage TokenUsage
	Model      string
	ToolCalls  []ToolCall
}

type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ToolDefinition 定义一个可供 LLM 调用的工具
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema 格式
}

// ToolCall 表示 LLM 请求调用一个工具
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolResult 表示工具调用的结果
type ToolResult struct {
	ToolCallID string `json:"tool_call_id"`
	Content    string `json:"content"`
}

// ResponseFormat 指定 LLM 响应格式
type ResponseFormat struct {
	Type   string                 `json:"type"`             // "text" or "json_object"
	Schema map[string]interface{} `json:"schema,omitempty"` // JSON Schema 约束
}

// ProviderCapabilities 描述 Provider 的能力
type ProviderCapabilities struct {
	FunctionCalling bool `json:"function_calling"`
	Streaming       bool `json:"streaming"`
	Vision          bool `json:"vision"`
	JSONMode        bool `json:"json_mode"`
	ContextWindow   int  `json:"context_window"`
}

type StreamChunk struct {
	Content   string      `json:"content"`
	Done      bool        `json:"done"`
	Usage     *TokenUsage `json:"usage,omitempty"`
	ToolCall  *ToolCall   `json:"tool_call,omitempty"`
	LatencyMs int64       `json:"latency_ms,omitempty"`
	Error     string      `json:"error,omitempty"`
}

type CallOptions struct {
	MaxTokens      int
	Temperature    float64
	Model          string
	TopP           float64
	Tools          []ToolDefinition
	ToolChoice     interface{}
	ResponseFormat *ResponseFormat
	Stop           []string
}

type Option func(*CallOptions)

func WithMaxTokens(n int) Option {
	return func(c *CallOptions) { c.MaxTokens = n }
}

func WithTemperature(t float64) Option {
	return func(c *CallOptions) { c.Temperature = t }
}

func WithModel(m string) Option {
	return func(c *CallOptions) { c.Model = m }
}

func WithTopP(p float64) Option {
	return func(c *CallOptions) { c.TopP = p }
}

func WithTools(tools []ToolDefinition) Option {
	return func(c *CallOptions) { c.Tools = tools }
}

func WithToolChoice(choice interface{}) Option {
	return func(c *CallOptions) { c.ToolChoice = choice }
}

func WithResponseFormat(format *ResponseFormat) Option {
	return func(c *CallOptions) { c.ResponseFormat = format }
}

func WithJSONMode() Option {
	return func(c *CallOptions) { c.ResponseFormat = &ResponseFormat{Type: "json_object"} }
}

func WithStop(stop []string) Option {
	return func(c *CallOptions) { c.Stop = stop }
}

func applyOptions(opts ...Option) *CallOptions {
	cfg := &CallOptions{}
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}

type contextKey string

const userIDKey contextKey = "token_usage_user_id"

// ContextWithUserID 将 userID 注入 ctx（typed key + 共享 ctxkeys key）。
func ContextWithUserID(ctx context.Context, userID int) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return ctxkeys.WithUserID(ctx, userID)
}

func userIDFromContext(ctx context.Context) int {
	if v, ok := ctx.Value(userIDKey).(int); ok {
		return v
	}
	return 0
}

type Provider interface {
	Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error)
	StreamChat(ctx context.Context, system string, messages []Message, opts ...Option) (<-chan StreamChunk, error)
	ChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (*LLMResponse, error)
	StreamChatWithTools(ctx context.Context, system string, messages []Message, tools []ToolDefinition, opts ...Option) (<-chan StreamChunk, error)
	GetEmbedding(ctx context.Context, text string) ([]float64, error)
	GetEmbeddings(ctx context.Context, texts []string) ([][]float64, error)
	Name() string
	Capabilities() ProviderCapabilities
}

// ModelLister 由支持模型发现（GET /v1/models）的 Provider 可选实现
type ModelLister interface {
	ListModels(ctx context.Context) ([]string, error)
}

// S19: AI审计日志回调（由外部注册）
var AuditLogFunc func(providerName, action string, tokenUsage int, err error)

func logAudit(providerName, action string, tokenUsage int, err error) {
	if AuditLogFunc != nil {
		AuditLogFunc(providerName, action, tokenUsage, err)
	}
}

// S20: Token计量+配额（持久化版）

// TokenUsageStore Token使用量持久化接口，由外部（repository层）注入实现
type TokenUsageStore interface {
	// LoadDayUsage 加载指定日期的总使用量和每用户使用量
	LoadDayUsage(date string) (total int, byUser map[int]int, err error)
	// SaveTotalUsage 保存指定日期的总使用量（upsert）
	SaveTotalUsage(date string, total int) error
	// SaveUserUsage 保存指定日期某用户的使用量（upsert）
	SaveUserUsage(date string, userID int, usage int) error
}

type TokenQuotaTracker struct {
	mu           sync.RWMutex
	dailyLimit   int
	perUserLimit int
	currentDate  string
	usageToday   int
	usageByUser  map[int]int // userID → daily usage
	store        TokenUsageStore
	stopReset    chan struct{}
}

var quotaTracker *TokenQuotaTracker

// InitTokenQuota 初始化配额追踪器，store为nil时退化为纯内存模式（无持久化）
func InitTokenQuota(dailyLimit, perUserLimit int, store TokenUsageStore) {
	now := time.Now()
	today := now.Format("2006-01-02")

	t := &TokenQuotaTracker{
		dailyLimit:   dailyLimit,
		perUserLimit: perUserLimit,
		currentDate:  today,
		usageByUser:  make(map[int]int),
		store:        store,
		stopReset:    make(chan struct{}),
	}

	if store != nil {
		total, byUser, err := store.LoadDayUsage(today)
		if err != nil {
			logx.Infof("[token-quota] load usage for %s failed: %v, starting from zero", today, err)
		} else {
			t.usageToday = total
			t.usageByUser = byUser
			logx.Infof("[token-quota] loaded usage for %s: total=%d, users=%d", today, total, len(byUser))
		}
	}

	quotaTracker = t
	go t.resetLoop()
}

// resetLoop 每日0点重置内存中的当日使用量（DB保留历史）
func (t *TokenQuotaTracker) resetLoop() {
	for {
		now := time.Now()
		nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 5, 0, now.Location())
		select {
		case <-time.After(time.Until(nextMidnight)):
			newDate := time.Now().Format("2006-01-02")
			t.mu.Lock()
			t.currentDate = newDate
			t.usageToday = 0
			t.usageByUser = make(map[int]int)
			if t.store != nil {
				if total, byUser, err := t.store.LoadDayUsage(newDate); err == nil {
					t.usageToday = total
					t.usageByUser = byUser
				}
			}
			t.mu.Unlock()
			logx.Infof("[token-quota] daily reset, new date: %s", newDate)
		case <-t.stopReset:
			return
		}
	}
}

// StopTokenQuota 停止定时重置协程（优雅关闭时调用）
func StopTokenQuota() {
	if quotaTracker != nil {
		close(quotaTracker.stopReset)
	}
}

// estimateTokenUsage 基于输出字符数估算token用量（中文约1.5字符/token，英文约4字符/token，取2.5均值）
func estimateTokenUsage(contentLen int) *TokenUsage {
	if contentLen <= 0 {
		return nil
	}
	estimated := contentLen * 10 / 25
	if estimated < 1 {
		estimated = 1
	}
	return &TokenUsage{CompletionTokens: estimated, TotalTokens: estimated}
}

func RecordTokenUsage(userID, tokens int) {
	if quotaTracker == nil {
		return
	}
	// P0-4: 在同一锁内捕获快照，避免 Unlock 后再 loadTotal/loadUser 的非原子读
	quotaTracker.mu.Lock()
	quotaTracker.usageToday += tokens
	quotaTracker.usageByUser[userID] += tokens
	date := quotaTracker.currentDate
	store := quotaTracker.store
	total := quotaTracker.usageToday
	userUsage := quotaTracker.usageByUser[userID]
	quotaTracker.mu.Unlock()

	if store != nil {
		if err := store.SaveTotalUsage(date, total); err != nil {
			logx.Infof("[token-quota] save total usage failed: %v", err)
		}
		if err := store.SaveUserUsage(date, userID, userUsage); err != nil {
			logx.Infof("[token-quota] save user usage failed: %v", err)
		}
	}
}

func CheckQuota(userID int) error {
	if quotaTracker == nil {
		return nil
	}
	quotaTracker.mu.RLock()
	total := quotaTracker.usageToday
	userUsage := quotaTracker.usageByUser[userID]
	dailyLimit := quotaTracker.dailyLimit
	perUserLimit := quotaTracker.perUserLimit
	quotaTracker.mu.RUnlock()

	if dailyLimit > 0 && total >= dailyLimit {
		return fmt.Errorf("AI daily token quota exceeded (%d/%d)", total, dailyLimit)
	}
	if perUserLimit > 0 && userUsage >= perUserLimit {
		return fmt.Errorf("user daily AI quota exceeded (%d/%d)", userUsage, perUserLimit)
	}
	return nil
}

func GetTokenUsage() (total int, byUser map[int]int) {
	if quotaTracker == nil {
		return 0, nil
	}
	quotaTracker.mu.RLock()
	defer quotaTracker.mu.RUnlock()
	cp := make(map[int]int, len(quotaTracker.usageByUser))
	for k, v := range quotaTracker.usageByUser {
		cp[k] = v
	}
	return quotaTracker.usageToday, cp
}
