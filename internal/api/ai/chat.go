package ai

import (
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/ctxkeys"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	aiChatSvc "github.com/listen808/citebase/internal/service/ai"
	config "github.com/listen808/citebase/internal/utils/config"
)

type ChatAPI struct{}

func NewChatAPI() *ChatAPI {
	return &ChatAPI{}
}

// sseData 写出一帧 SSE data 负载。
//
// 旧实现手写字符串替换做 JSON 转义，只替换了双引号和换行，漏掉反斜杠、制表符与
// 其余控制字符（\u0000-\u001f）。模型输出代码/路径/JSON 示例时很常见，拼出来的
// `data: {"content": "..."}` 不是合法 JSON，前端 JSON.parse 抛错导致答案错乱或中断。
// 统一交给 json.Marshal 生成，行为与已有的 meta 帧一致。
func sseData(w io.Writer, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		logx.Infof("[SSE] 编码 data 帧失败: %v", err)
		return
	}
	fmt.Fprintf(w, "data: %s\n\n", b)
}

type askRequest struct {
	Question string            `json:"question" binding:"required"`
	SpaceID  int               `json:"space_id"`
	Limit    int               `json:"limit"`
	History  []chatHistoryItem `json:"history"`
	Images   []string          `json:"images"`
}

// chatHistoryItem 兼容两种历史格式：
//   - 通用 role/content（前端约定）
//   - question/answer（后端 rag.ChatMessage 约定）
//
// 后端不再假设单一格式，避免前端传来的 {role, content} 因字段名不匹配而被
// 静默丢弃，导致多轮对话上下文（及压缩层预算评估）失效。
type chatHistoryItem struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// toRAGHistory 将请求历史转换为 rag.ChatMessage 问答对。
// role/content 形式按 用户->助手 顺序两两配对；question/answer 形式直接采用。
func toRAGHistory(items []chatHistoryItem) []rag.ChatMessage {
	out := make([]rag.ChatMessage, 0, len(items))
	var pendingQ string
	for _, it := range items {
		if it.Question != "" || it.Answer != "" {
			out = append(out, rag.ChatMessage{Question: it.Question, Answer: it.Answer})
			continue
		}
		switch it.Role {
		case "user":
			pendingQ = it.Content
		case "assistant":
			if pendingQ != "" {
				out = append(out, rag.ChatMessage{Question: pendingQ, Answer: it.Content})
				pendingQ = ""
			}
		}
	}
	return out
}

func (api *ChatAPI) Ask(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	if m.PrimaryProvider() == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}
	if m.Retriever() == nil {
		response.ServerErrorKey(c, response.MsgAIRetrieverUnconfigured)
		return
	}

	// 使用 ChatService 执行问答业务逻辑
	chatSvc := aiChatSvc.NewChatService(m)
	userID, _ := middleware.GetUserID(c)

	result, err := chatSvc.Ask(userCtx(c), &aiChatSvc.AskRequest{
		Question: req.Question,
		SpaceID:  req.SpaceID,
		Limit:    req.Limit,
		History:  toRAGHistory(req.History),
		Images:   req.Images,
	}, userID)

	// 记录审计日志
	providerName := ""
	modelName := ""
	if p := aimod.GetModule().PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	sourceCount := 0
	tokenUsage := 0
	answerText := ""
	if result != nil && result.Answer != nil {
		sourceCount = len(result.Answer.Sources)
		tokenUsage = result.Answer.TokenUsage.TotalTokens
		answerText = result.Answer.Answer
	}

	resultStr := "success"
	reason := ""
	if err != nil {
		resultStr = "failed"
		reason = err.Error()
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	RecordAIAudit(
		c,
		userID,
		"ask",
		targetID,
		req.Question,
		answerText,
		modelName,
		providerName,
		resultStr,
		reason,
		tokenUsage,
		sourceCount,
		result.DurationMs,
	)

	if err != nil {
		logx.Infof("[ChatAPI] Ask error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"answer":  result.Answer.Answer,
		"sources": result.Answer.Sources,
	})
}

func (api *ChatAPI) GenerateSummary(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	summarizer := m.Summarizer()
	if summarizer == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	summary, tokenUsage, err := summarizer.GenerateSummary(userCtx(c), docID)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

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

	targetID := fmt.Sprintf("%d", docID)
	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "summary", targetID, fmt.Sprintf("文档ID: %d", docID), summary, modelName, providerName, result, reason, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] Summary error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"summary": summary})
}

func (api *ChatAPI) StreamAsk(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	p := m.PrimaryProvider()
	if p == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIRetrieverUnconfigured)
		return
	}

	safetyGuard := m.SafetyGuard()
	if safetyGuard != nil {
		if safe, reason := safetyGuard.CheckQuerySafety(c.Request.Context(), req.Question); !safe {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("X-Accel-Buffering", "no")
			sseData(c.Writer, map[string]interface{}{
				"message_key":    response.MsgAISafetyBlocked,
				"message_detail": reason,
			})
			c.Writer.Flush()
			return
		}
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	startTime := time.Now()

	// 读取当前用户身份，注入 ragCtx 供检索器做「个性化记忆召回」与「权限过滤」。
	userID, _ := middleware.GetUserID(c)
	ragCtx := c.Request.Context()
	if userID > 0 {
		ragCtx = ctxkeys.WithUserID(ragCtx, userID)
	}

	// 将对话历史注入 ctx，供检索器的上下文压缩层评估 token 预算。
	ragHistory := toRAGHistory(req.History)
	if len(ragHistory) > 0 {
		ragCtx = ctxkeys.WithChatHistory(ragCtx, ragHistory)
	}

	sources, contextText := retriever.RetrieveContextWithRAG(ragCtx, req.Question, req.SpaceID, req.Limit)
	sourceCount := len(sources)

	// P2-6: 个性化用户记忆——召回与问题相关的用户画像/偏好/历史背景，注入 prompt。
	memoryContext := retriever.RecallMemoryContext(ragCtx, req.Question)

	providerName := ""
	modelName := ""
	if p != nil {
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

	systemPrompt := fmt.Sprintf(`你是一个知识库AI助手。请基于以下提供的文档内容回答问题。
如果文档内容不足以回答问题，请如实说明你无法从已有知识中找到答案。
请用中文回答，保持简洁清晰。

文档内容：
%s`, contextText)

	// P2-6: 注入个性化用户记忆，使回答贴合用户偏好与历史背景。
	if memoryContext != "" {
		systemPrompt += "\n\n" + memoryContext
	}

	messages := make([]provider.Message, 0, len(ragHistory)*2+1)
	for _, h := range ragHistory {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	if len(req.Images) > 0 {
		messages = append(messages, provider.NewImageMessage("user", req.Question, req.Images[0]))
	} else {
		messages = append(messages, provider.NewTextMessage("user", req.Question))
	}

	ctx := provider.ContextWithUserID(c.Request.Context(), userID)

	ch, err := p.StreamChat(ctx, systemPrompt, messages)
	if err != nil {
		logx.Infof("[ChatAPI] Stream error: %v", err)
		durationMs := time.Since(startTime).Milliseconds()
		RecordAIAudit(c, userID, "ask", targetID, req.Question, "", modelName, providerName, "failed", err.Error(), 0, sourceCount, durationMs)
		fmt.Fprintf(c.Writer, "data: {\"error\": \"AI服务暂不可用\"}\n\n")
		c.Writer.Flush()
		return
	}

	var fullAnswer strings.Builder
	var tokenUsage int
	firstTokenRecorded := false

	c.Stream(func(w io.Writer) bool {
		select {
		case chunk, ok := <-ch:
			if !ok {
				durationMs := time.Since(startTime).Milliseconds()
				// P2-2: 在流式末帧附带可验证引用来源与推理链，供前端可视化。
				enrichedSources := rag.VerifyCitations(fullAnswer.String(), sources, nil)
				chain := rag.BuildReasoningChain(len(sources), len(sources), "基于答案引用标记标注来源可信度", false, true)
				comps := rag.ExtractComponents(fullAnswer.String())
				metaJSON, _ := json.Marshal(map[string]interface{}{
					"meta":            true,
					"token_usage":     tokenUsage,
					"duration_ms":     durationMs,
					"sources":         enrichedSources,
					"reasoning_chain": chain,
					"components":      comps,
				})
				fmt.Fprintf(w, "data: %s\n\n", string(metaJSON))
				RecordAIAudit(c, userID, "ask", targetID, req.Question, fullAnswer.String(), modelName, providerName, "success", "", tokenUsage, sourceCount, durationMs)
				fmt.Fprint(w, "data: [DONE]\n\n")
				return false
			}
			if chunk.Done {
				if chunk.Usage != nil && chunk.Usage.TotalTokens > 0 {
					tokenUsage = chunk.Usage.TotalTokens
					provider.RecordTokenUsage(userID, tokenUsage)
				}
				durationMs := time.Since(startTime).Milliseconds()
				// P2-2/P2-3: 流式末帧附带可验证引用来源与推理链、生成式组件，供前端可视化。
				// 注意：provider 在发送 Done chunk 后会关闭 channel，因此完整 meta 必须在此处下发，
				// 否则 !ok 分支（同样的逻辑）永远不会执行，导致引用来源/组件丢失。
				enrichedSources := rag.VerifyCitations(fullAnswer.String(), sources, nil)
				chain := rag.BuildReasoningChain(len(sources), len(sources), "基于答案引用标记标注来源可信度", false, true)
				comps := rag.ExtractComponents(fullAnswer.String())
				metaJSON, _ := json.Marshal(map[string]interface{}{
					"meta":            true,
					"token_usage":     tokenUsage,
					"duration_ms":     durationMs,
					"sources":         enrichedSources,
					"reasoning_chain": chain,
					"components":      comps,
				})
				fmt.Fprintf(w, "data: %s\n\n", string(metaJSON))
				RecordAIAudit(c, userID, "ask", targetID, req.Question, fullAnswer.String(), modelName, providerName, "success", "", tokenUsage, sourceCount, durationMs)
				fmt.Fprint(w, "data: [DONE]\n\n")
				return false
			}
			if !firstTokenRecorded && chunk.Content != "" {
				firstTokenRecorded = true
				firstTokenLatency := time.Since(startTime).Milliseconds()
				fmt.Fprintf(w, "data: {\"first_token\": true, \"latency_ms\": %d}\n\n", firstTokenLatency)
			}
			fullAnswer.WriteString(chunk.Content)
			sseData(w, map[string]string{"content": chunk.Content})
			return true
		case <-c.Request.Context().Done():
			durationMs := time.Since(startTime).Milliseconds()
			RecordAIAudit(c, userID, "ask", targetID, req.Question, fullAnswer.String(), modelName, providerName, "cancelled", "请求被取消", tokenUsage, sourceCount, durationMs)
			return false
		}
	})
}
