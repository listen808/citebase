package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/provider"
)

// StreamHandler 处理 SSE 流式输出
type StreamHandler struct {
	agent *Agent
}

// NewStreamHandler 创建流式处理器
func NewStreamHandler(agent *Agent) *StreamHandler {
	return &StreamHandler{agent: agent}
}

// StreamQueryRequest 流式查询请求
type StreamQueryRequest struct {
	Query          string                `json:"query" binding:"required"`
	SpaceID        uint64                `json:"space_id"`
	ConversationID int                   `json:"conversation_id"`
	History        []ConversationMessage `json:"history,omitempty"`
	// UserID 只由服务端从认证上下文写入，禁止由请求体传入
	UserID int `json:"-"`
}

// userIDFromContext 读取认证中间件写入的登录用户 ID（存入的是 Go int）
func userIDFromContext(c *gin.Context) int {
	if v, exists := c.Get("user_id"); exists {
		if id, ok := v.(int); ok && id > 0 {
			return id
		}
	}
	return 0
}

// HandleStreamQuery 处理 SSE 流式查询（Gin Handler）
func (h *StreamHandler) HandleStreamQuery(c *gin.Context) {
	var req StreamQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userIDFromContext(c)

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ctx := c.Request.Context()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// P0 安全修复：执行链/生成链必须知道调用者是谁，否则会话与数据都无法按用户隔离
	if req.UserID <= 0 {
		h.sendSSE(c, flusher, "error", "缺少调用者身份，拒绝执行")
		return
	}

	// P1（清单 42）：生成前先用执行链同一套归属校验判定“该用户是否可见该空间”，
	// 不能只信请求体里的 space_id——否则任意登录用户可让服务端加载他人空间的
	// 表结构/字段样本进 LLM 提示词并回显。
	if exec := h.agent.generator.GetExecutor(); exec != nil {
		if err := exec.AuthorizeSpaceAccess(req.SpaceID, req.UserID); err != nil {
			h.sendSSE(c, flusher, "error", fmt.Sprintf("无权访问该空间: %v", err))
			return
		}
	}

	// 加载知识库（请求级），连同该用户该会话专属的对话状态一起绑定
	kb := NewKnowledgeBase(h.agent.generator.repo)
	if err := kb.LoadFromSpace(req.SpaceID); err != nil {
		h.sendSSE(c, flusher, "error", fmt.Sprintf("加载知识库失败: %v", err))
		return
	}
	scope := &RequestScope{
		UserID:  req.UserID,
		SpaceID: req.SpaceID,
		KB:      kb,
		State:   ConversationStateFor(req.ConversationID, req.UserID, int(req.SpaceID)),
	}
	// P1 首要任务 2：请求级上下文沿 ctx 传递（不再写回进程级共享字段）
	ctx = WithRequestScope(ctx, scope)

	// 如果提供了历史消息，只回放到本次请求专属的对话状态，
	// 不再写回进程级单例（旧实现会让并发用户的轮次互相混写）
	if len(req.History) > 0 {
		for _, msg := range req.History {
			if msg.Role == "user" {
				scope.State.RecordTurn(msg.Content, msg.SQL, nil)
			}
		}
	}

	cfg := &AgentReActConfig{
		MaxSteps:        8,
		EnableClarify:   true,
		EnableInterpret: true,
		StreamCallback: func(event StreamEvent) {
			h.sendSSE(c, flusher, event.Type, event.Content)
		},
	}

	result, err := h.agent.Generate(ctx, req.Query, req.SpaceID, cfg)
	if err != nil {
		h.sendSSE(c, flusher, "error", fmt.Sprintf("生成失败: %v", err))
		return
	}

	// 发送最终结果
	finalResult := map[string]interface{}{
		"sql":            result.SQL,
		"explanation":    result.Explanation,
		"confidence":     result.Confidence,
		"tables":         result.Tables,
		"steps":          result.Steps,
		"interpretation": result.Interpretation,
	}
	if result.Result != nil && result.Result.ExecutionResult != nil {
		finalResult["execution"] = result.Result.ExecutionResult
	}

	finalJSON, _ := json.Marshal(finalResult)
	h.sendSSE(c, flusher, "final", string(finalJSON))
	h.sendSSE(c, flusher, "done", "")
}

// StreamQuery 流式查询（非 Gin 版本，用于 API 直接调用）
//
// P0 安全修复：必须显式传入调用者身份（userID）与会话 ID，请求级状态只绑定到
// 该 (conversationID, userID) 专属实例，不再写回进程级共享对象。
func (h *StreamHandler) StreamQuery(ctx context.Context, query string, spaceID uint64, conversationID, userID int, callback func(StreamEvent)) (*AgentResult, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("缺少调用者身份，拒绝执行")
	}
	// P1（清单 42）：与 SSE 入口一致，生成前校验调用者对该空间的可见性
	if exec := h.agent.generator.GetExecutor(); exec != nil {
		if err := exec.AuthorizeSpaceAccess(spaceID, userID); err != nil {
			return nil, fmt.Errorf("无权访问该空间: %w", err)
		}
	}

	// 加载知识库
	kb := NewKnowledgeBase(h.agent.generator.repo)
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, fmt.Errorf("加载知识库失败: %w", err)
	}
	// P1 首要任务 2：请求级上下文沿 ctx 传递
	ctx = WithRequestScope(ctx, &RequestScope{
		UserID:  userID,
		SpaceID: spaceID,
		KB:      kb,
		State:   ConversationStateFor(conversationID, userID, int(spaceID)),
	})

	cfg := &AgentReActConfig{
		MaxSteps:        8,
		EnableClarify:   true,
		EnableInterpret: true,
		StreamCallback: func(event StreamEvent) {
			if callback != nil {
				callback(event)
			}
		},
	}

	return h.agent.Generate(ctx, query, spaceID, cfg)
}

// sendSSE 发送 SSE 事件
func (h *StreamHandler) sendSSE(c *gin.Context, flusher http.Flusher, eventType, data string) {
	event := map[string]interface{}{
		"type": eventType,
		"data": data,
	}
	jsonBytes, err := json.Marshal(event)
	if err != nil {
		logx.Infof("[StreamHandler] JSON marshal error: %v", err)
		return
	}

	select {
	case <-c.Request.Context().Done():
		return
	default:
		fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonBytes))
		flusher.Flush()
	}
}

// ============================================================
// Streaming Agent - 使用 provider.StreamChatWithTools 实现真正的流式输出
// ============================================================

// StreamingAgent 支持流式输出的 Agent
type StreamingAgent struct {
	agent *Agent
}

// NewStreamingAgent 创建流式 Agent
func NewStreamingAgent(agent *Agent) *StreamingAgent {
	return &StreamingAgent{agent: agent}
}

// GenerateStreaming 流式生成 SQL（使用 SSE）
// conversationID/userID 用于定位该请求专属的会话状态（不再共用进程级实例）
func (sa *StreamingAgent) GenerateStreaming(ctx context.Context, text string, spaceID uint64, conversationID, userID int, eventCh chan<- StreamEvent) (*AgentResult, error) {
	defer close(eventCh)

	if userID <= 0 {
		return nil, fmt.Errorf("缺少调用者身份，拒绝执行")
	}
	// P1 首要任务 2：请求级上下文沿 ctx 传递（不再写回进程级共享字段）
	ctx = WithRequestScope(ctx, &RequestScope{
		UserID:  userID,
		SpaceID: spaceID,
		State:   ConversationStateFor(conversationID, userID, int(spaceID)),
	})

	cfg := &AgentReActConfig{
		MaxSteps:        8,
		EnableClarify:   true,
		EnableInterpret: true,
		StreamCallback: func(event StreamEvent) {
			select {
			case eventCh <- event:
			case <-ctx.Done():
				return
			}
		},
	}

	return sa.agent.Generate(ctx, text, spaceID, cfg)
}

// StreamChatWithTools 使用 provider 的流式聊天接口
func (sa *StreamingAgent) StreamChat(ctx context.Context, systemPrompt string, messages []provider.Message, tools []provider.ToolDefinition, eventCh chan<- StreamEvent) error {
	ch, err := sa.agent.provider.StreamChatWithTools(ctx, systemPrompt, messages, tools)
	if err != nil {
		return fmt.Errorf("stream chat failed: %w", err)
	}

	var toolCallBuffer strings.Builder
	var currentContent strings.Builder

	for chunk := range ch {
		if chunk.Error != "" {
			eventCh <- StreamEvent{Type: "error", Content: chunk.Error}
			return fmt.Errorf("stream error: %s", chunk.Error)
		}

		if chunk.Done {
			break
		}

		// 处理工具调用（流式输出）
		if chunk.ToolCall != nil {
			if chunk.ToolCall.Name != "" {
				toolCallBuffer.WriteString(fmt.Sprintf("Calling: %s\n", chunk.ToolCall.Name))
			}
			if len(chunk.ToolCall.Arguments) > 0 {
				toolCallBuffer.Write(chunk.ToolCall.Arguments)
			}
		}

		// 处理文本内容
		if chunk.Content != "" {
			currentContent.WriteString(chunk.Content)
			eventCh <- StreamEvent{
				Type:    "thought",
				Content: chunk.Content,
			}
		}
	}

	// 发送工具调用信息
	if toolCallBuffer.Len() > 0 {
		eventCh <- StreamEvent{
			Type:    "action",
			Content: toolCallBuffer.String(),
		}
	}

	return nil
}

// GinStreamHandler 处理 Gin 的 SSE 流式查询（使用真正的流式 LLM）
func (sa *StreamingAgent) GinStreamHandler(c *gin.Context) {
	var req StreamQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := userIDFromContext(c)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	eventCh := make(chan StreamEvent, 32)
	errCh := make(chan error, 1)

	go func() {
		_, err := sa.GenerateStreaming(ctx, req.Query, req.SpaceID, req.ConversationID, userID, eventCh)
		if err != nil {
			errCh <- err
		}
	}()

	for {
		select {
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			jsonBytes, _ := json.Marshal(event)
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonBytes))
			flusher.Flush()

			if event.Done {
				return
			}
		case err := <-errCh:
			errEvent := StreamEvent{Type: "error", Content: err.Error(), Done: true}
			jsonBytes, _ := json.Marshal(errEvent)
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonBytes))
			flusher.Flush()
			return
		case <-ctx.Done():
			return
		}
	}
}
