package ai

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	config "github.com/listen808/citebase/internal/utils/config"
)

// 高级问答：调试 / 多跳 / Self-RAG / 查询分类 / 文档对话。

type askWithDebugResponse struct {
	Answer  string            `json:"answer"`
	Sources []rag.Source      `json:"sources"`
	Debug   *rag.RAGDebugInfo `json:"debug"`
}

func (api *ChatAPI) AskWithDebug(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	var answer *rag.AIAnswer
	var debug *rag.RAGDebugInfo
	var err error

	answer, debug, err = retriever.AskWithDebug(userCtx(c), req.Question, req.SpaceID, req.Limit)

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

	sourceCount := 0
	answerText := ""
	tokenUsage := 0
	if answer != nil {
		sourceCount = len(answer.Sources)
		answerText = answer.Answer
		tokenUsage = answer.TokenUsage.TotalTokens
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "ask_debug", targetID, req.Question, answerText, modelName, providerName, result, reason, tokenUsage, sourceCount, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] AskWithDebug error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, askWithDebugResponse{
		Answer:  answer.Answer,
		Sources: answer.Sources,
		Debug:   debug,
	})
}

func (api *ChatAPI) MultiHopAsk(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	answer, err := retriever.AskWithMultiHop(userCtx(c), req.Question, req.SpaceID, req.Limit)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	}

	sourceCount := 0
	tokenUsage := 0
	if answer != nil {
		sourceCount = len(answer.Sources)
		tokenUsage = answer.TokenUsage.TotalTokens
	}

	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		reason = err.Error()
	}

	answerText := ""
	if answer != nil {
		answerText = answer.Answer
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	RecordAIAudit(c, userID, "multi_hop_ask", targetID, req.Question, answerText, modelName, providerName, result, reason, tokenUsage, sourceCount, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] MultiHopAsk error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"answer":  answer.Answer,
		"sources": answer.Sources,
	})
}

func (api *ChatAPI) SelfRAGAsk(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	answer, err := retriever.AskWithSelfRAG(userCtx(c), req.Question, req.SpaceID, req.Limit)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	}

	sourceCount := 0
	tokenUsage := 0
	if answer != nil {
		sourceCount = len(answer.Sources)
		tokenUsage = answer.TokenUsage.TotalTokens
	}

	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		reason = err.Error()
	}

	answerText := ""
	if answer != nil {
		answerText = answer.Answer
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	RecordAIAudit(c, userID, "self_rag_ask", targetID, req.Question, answerText, modelName, providerName, result, reason, tokenUsage, sourceCount, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] SelfRAGAsk error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"answer":  answer.Answer,
		"sources": answer.Sources,
	})
}

func (api *ChatAPI) ClassifyQuery(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.AdaptiveRetriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIAdaptiveUnconfigured)
		return
	}

	queryType := retriever.ClassifyQuery(c.Request.Context(), req.Question)

	response.Success(c, gin.H{
		"query_type":  queryType,
		"description": rag.GetStrategyDescription(queryType),
	})
}

type chatWithDocRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages" binding:"required"`
	SpaceID      int    `json:"space_id"`
	DocID        int    `json:"doc_id"`
	SelectedText string `json:"selected_text"`
}

// ChatWithDoc 针对特定文档进行 AI 对话
func (api *ChatAPI) ChatWithDoc(c *gin.Context) {
	var req chatWithDocRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "messages is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	// 加载文档内容作为上下文
	docRepo := workspacerepo.NewDocumentRepository()
	docCtx := ""

	if req.DocID > 0 {
		doc, err := docRepo.FindByID(req.DocID)
		if err != nil || doc == nil {
			response.BadRequest(c, "document not found")
			return
		}
		// 归属校验：doc_id 完全由请求方控制，不校验空间可见性时本接口等价于
		// 「用 doc_id 枚举任意私有空间文档正文」（模型会把原文复述给调用者）
		visible, unrestricted, err := accessibleSpaceIDs(c)
		if err != nil {
			response.ServerErrorSafe(c, err)
			return
		}
		if !canAccessSpace(doc.SpaceID, visible, unrestricted) {
			response.ForbiddenKey(c, response.MsgNoPermissionDocSpace)
			return
		}
		// 加载文档Markdown内容
		docService := workspacesvc.NewDocumentService()
		content, err := docService.GetDocumentContent(doc.ID)
		if err == nil && content != "" {
			if req.SelectedText != "" {
				docCtx = fmt.Sprintf("用户选中文档《%s》的以下内容:\n\n%s", doc.Name, req.SelectedText)
			} else {
				truncated := content
				if len(content) > 8000 {
					truncated = content[:8000] + "\n...(内容已截断)"
				}
				docCtx = fmt.Sprintf("用户正在查看文档《%s》:\n\n%s", doc.Name, truncated)
			}
		}
	}

	// 构建消息列表，将文档上下文作为 system 消息
	systemMsg := ""
	if docCtx != "" {
		systemMsg = fmt.Sprintf("你是一个知识库AI助手。以下是相关文档内容，请基于此内容回答用户问题。\n\n%s", docCtx)
	}

	// 取最后一条用户消息作为 prompt
	userMsg := ""
	if len(req.Messages) > 0 {
		last := req.Messages[len(req.Messages)-1]
		if last.Role == "user" {
			userMsg = last.Content
		}
	}
	if userMsg == "" {
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				userMsg = msg.Content
				break
			}
		}
	}

	// 调用 AI
	prim := m.PrimaryProvider()
	if prim == nil {
		response.ServerErrorKey(c, response.MsgAIPrimaryModelUnconfigured)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	messages := []provider.Message{provider.NewTextMessage("user", userMsg)}
	result, err := prim.Chat(ctx, systemMsg, messages)
	if err != nil {
		logx.Infof("[ChatWithDoc] AI error: %v", err)
		response.ServerErrorKey(c, response.MsgAIChatFailed)
		return
	}

	response.Success(c, gin.H{"content": result.Content})
}
