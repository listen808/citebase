package ai

import (
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	config "github.com/listen808/citebase/internal/utils/config"
)

// AgentAskStream Agent 流式问答（SSE）
// 3.1.2: RAG Agent 多轮工具调用流式输出，用户实时看到 thinking/tool_call/tool_result/content/done 事件
func (api *ChatAPI) AgentAskStream(c *gin.Context) {
	var req agentAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIRetrieverUnconfigured)
		return
	}

	// 复用 AgentAsk 的 AgentChat 构造逻辑
	agentChat := rag.NewAgentChatWithTools(m.PrimaryProvider(), retriever, req.EnabledTools, workspacesvc.NewSpaceService(), m.AdaptiveRetriever(), m.CorrectiveRAG())
	if m.HyDERetriever() != nil {
		agentChat.SetHyDERetriever(m.HyDERetriever())
	}

	agentUserID, _ := middleware.GetUserID(c)
	agentCtx := provider.ContextWithUserID(c.Request.Context(), agentUserID)

	// 调用流式方法
	var eventCh <-chan rag.AgentStreamEvent
	var streamErr error
	if req.SpaceID > 0 {
		if len(req.Images) > 0 {
			eventCh, streamErr = agentChat.ChatWithSpaceStream(agentCtx, req.Question, req.History, req.SpaceID, req.Limit, req.Images[0])
		} else {
			eventCh, streamErr = agentChat.ChatWithSpaceStream(agentCtx, req.Question, req.History, req.SpaceID, req.Limit)
		}
	} else if len(req.Images) > 0 {
		eventCh, streamErr = agentChat.ChatStream(agentCtx, req.Question, req.History, req.Images[0])
	} else {
		eventCh, streamErr = agentChat.ChatStream(agentCtx, req.Question, req.History)
	}

	if streamErr != nil {
		logx.Infof("[ChatAPI] AgentAskStream init error: %v", streamErr)
		response.ServerErrorSafe(c, streamErr)
		return
	}

	// 设置 SSE 头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	startTime := time.Now()
	var fullAnswer strings.Builder
	var allSources []rag.Source
	var tokenUsage int

	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	ctx := c.Request.Context()

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-eventCh:
			if !ok {
				// channel 关闭，发送末帧
				return emitAgentDone(c, w, fullAnswer.String(), allSources, tokenUsage, startTime, providerName, modelName, targetID, agentUserID, req.Question)
			}
			return handleAgentStreamEvent(c, w, event, &fullAnswer, &allSources, &tokenUsage, providerName, modelName, targetID, agentUserID, req.Question, startTime)
		case <-ctx.Done():
			// 客户端断开
			durationMs := time.Since(startTime).Milliseconds()
			RecordAIAudit(c, agentUserID, "agent_stream_ask", targetID, req.Question, fullAnswer.String(), modelName, providerName, "cancelled", "请求被取消", tokenUsage, len(allSources), durationMs)
			return false
		}
	})
}

// handleAgentStreamEvent 处理单个 Agent 流式事件，返回是否继续流
func handleAgentStreamEvent(c *gin.Context, w io.Writer, event rag.AgentStreamEvent, fullAnswer *strings.Builder, allSources *[]rag.Source, tokenUsage *int, providerName, modelName, targetID string, agentUserID int, question string, startTime time.Time) bool {
	switch event.Type {
	case "thinking":
		// 思考中事件
		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"thinking": true,
			"round":    event.Round,
			"content":  event.Content,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		return true

	case "tool_call":
		// 工具调用开始
		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"tool_call": event.ToolCall,
			"round":     event.Round,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		return true

	case "tool_result":
		// 工具执行结果
		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"tool_result": event.ToolResult,
			"round":       event.Round,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		return true

	case "content":
		// 最终答案内容：必须整体 JSON 编码，手写转义漏掉反斜杠/控制字符会产生非法 JSON
		fullAnswer.WriteString(event.Content)
		sseData(w, map[string]string{"content": event.Content})
		return true

	case "done":
		// 流结束，带 sources 和 meta
		if event.Sources != nil {
			*allSources = event.Sources
		}
		return emitAgentDone(c, w, fullAnswer.String(), *allSources, *tokenUsage, startTime, providerName, modelName, targetID, agentUserID, question)

	case "component":
		// 流式 UI 组件事件
		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"component": event.Component,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		return true

	case "error":
		// 错误事件
		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"error": event.Error,
		})
		fmt.Fprintf(w, "data: %s\n\n", string(jsonBytes))
		c.Writer.Flush()
		durationMs := time.Since(startTime).Milliseconds()
		RecordAIAudit(c, agentUserID, "agent_stream_ask", targetID, question, fullAnswer.String(), modelName, providerName, "failed", event.Error, *tokenUsage, len(*allSources), durationMs)
		return false

	default:
		return true
	}
}

// emitAgentDone 发送末帧 meta + [DONE]，记录审计，返回 false 停止流
func emitAgentDone(c *gin.Context, w io.Writer, answer string, sources []rag.Source, tokenUsage int, startTime time.Time, providerName, modelName, targetID string, agentUserID int, question string) bool {
	durationMs := time.Since(startTime).Milliseconds()

	// 验证引用
	enrichedSources := rag.VerifyCitations(answer, sources, nil)

	metaJSON, _ := json.Marshal(map[string]interface{}{
		"meta":        true,
		"token_usage": tokenUsage,
		"duration_ms": durationMs,
		"sources":     enrichedSources,
	})
	fmt.Fprintf(w, "data: %s\n\n", string(metaJSON))

	RecordAIAudit(c, agentUserID, "agent_stream_ask", targetID, question, answer, modelName, providerName, "success", "", tokenUsage, len(enrichedSources), durationMs)

	fmt.Fprint(w, "data: [DONE]\n\n")
	return false
}
