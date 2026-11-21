package ai

import (
	"context"
	"fmt"
	"time"

	aimod "github.com/listen808/citebase/internal/ai"
	rag "github.com/listen808/citebase/internal/ai/rag"
)

// ChatService 聊天/问答业务逻辑层
// 从 internal/api/ai/chat.go 中提取的业务逻辑
type ChatService struct {
	module *aimod.Module
}

// NewChatService 创建 ChatService 实例
func NewChatService(module *aimod.Module) *ChatService {
	return &ChatService{module: module}
}

// AskRequest 问答请求参数
type AskRequest struct {
	Question string            `json:"question" binding:"required"`
	SpaceID  int               `json:"space_id"`
	Limit    int               `json:"limit"`
	History  []rag.ChatMessage `json:"history"`
	Images   []string          `json:"images"`
	// Mode 选择问答引擎："rag"（默认检索增强）或 "agent"（多智能体编排+反思闭环，P0-2/P0-3）。
	Mode string `json:"mode"`
}

// AskResult 问答结果
type AskResult struct {
	Answer     *rag.AIAnswer
	DurationMs int64
	UserID     int
}

// Ask 执行 RAG 问答
func (s *ChatService) Ask(ctx context.Context, req *AskRequest, userID int) (*AskResult, error) {
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
	}

	if !s.module.Enabled() {
		return nil, fmt.Errorf("AI服务未启用")
	}

	retriever := s.module.Retriever()
	if retriever == nil {
		return nil, fmt.Errorf("AI服务未配置")
	}

	startTime := time.Now()
	var answer *rag.AIAnswer
	var err error

	if req.Mode == "agent" {
		// P0-2/P0-3：多智能体编排 + 规划反思闭环
		orc := s.module.NewOrchestrator()
		if orc == nil {
			return nil, fmt.Errorf("多智能体编排不可用（AI未启用）")
		}
		final, aerr := orc.Run(ctx, req.Question, req.History, req.Images...)
		if aerr != nil {
			err = aerr
		} else {
			answer = &rag.AIAnswer{Answer: final, Sources: nil}
		}
	} else if len(req.History) > 0 {
		answer, err = retriever.AskWithHistory(ctx, req.Question, req.SpaceID, req.Limit, req.History)
	} else {
		answer, err = retriever.Ask(ctx, req.Question, req.SpaceID, req.Limit)
	}

	durationMs := time.Since(startTime).Milliseconds()
	return &AskResult{Answer: answer, DurationMs: durationMs, UserID: userID}, err
}
