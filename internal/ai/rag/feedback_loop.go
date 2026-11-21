package rag

import (
	"context"
	"fmt"

	"github.com/listen808/citebase/internal/ai/provider"
)

// FeedbackLoop 检索反馈循环（P2-11）
type FeedbackLoop struct {
	maxIterations int
	minScore      float64
	retriever     *Retriever
}

// NewFeedbackLoop 创建反馈循环
func NewFeedbackLoop(r *Retriever) *FeedbackLoop {
	return &FeedbackLoop{
		maxIterations: 2,
		minScore:      0.6,
		retriever:     r,
	}
}

// FeedbackStats 反馈统计
type FeedbackStats struct {
	Iterations     int     `json:"iterations"`
	FinalScore     float64 `json:"final_score"`
	QueryRewritten bool    `json:"query_rewritten"`
	OriginalScore  float64 `json:"original_score"`
	Improved       bool    `json:"improved"`
}

// RetrieveWithFeedback 带反馈的检索
func (f *FeedbackLoop) RetrieveWithFeedback(
	ctx context.Context,
	question string,
	spaceID int,
	limit int,
) ([]Source, FeedbackStats, error) {
	stats := FeedbackStats{}

	// 初始检索
	sources, _ := f.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)

	// 评估初始结果质量
	stats.OriginalScore = avgScore(sources)
	stats.FinalScore = stats.OriginalScore

	// 如果初始质量足够，直接返回
	if stats.OriginalScore >= f.minScore || f.retriever.provider == nil {
		return sources, stats, nil
	}

	// 迭代改进
	currentQ := question
	for i := 0; i < f.maxIterations; i++ {
		stats.Iterations = i + 1

		// 重写查询
		rewritten, err := f.rewriteQuery(ctx, currentQ)
		if err != nil || rewritten == "" || rewritten == currentQ {
			break
		}
		currentQ = rewritten
		stats.QueryRewritten = true

		// 重新检索
		newSources, _ := f.retriever.RetrieveContextWithRAG(ctx, currentQ, spaceID, limit)

		newScore := avgScore(newSources)
		if newScore > stats.FinalScore {
			sources = newSources
			stats.FinalScore = newScore
			stats.Improved = true
		}

		if stats.FinalScore >= f.minScore {
			break
		}
	}

	return sources, stats, nil
}

// rewriteQuery 用 LLM 重写查询
func (f *FeedbackLoop) rewriteQuery(ctx context.Context, originalQ string) (string, error) {
	if f.retriever.provider == nil {
		return "", fmt.Errorf("provider not available")
	}
	systemPrompt := "你是一个查询重写助手，只输出重写后的问题。"
	userPrompt := fmt.Sprintf(`基于检索结果质量不佳的情况，请重写用户的问题以提高检索效果。

原始问题：%s

要求：
1. 保持原意不变
2. 使用更明确的关键词
3. 添加可能的同义词
4. 只返回重写后的问题，不要其他内容

重写后的问题：`, originalQ)

	messages := []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}
	resp, err := f.retriever.provider.Chat(ctx, systemPrompt, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// avgScore 计算来源的平均分数
func avgScore(sources []Source) float64 {
	if len(sources) == 0 {
		return 0
	}
	total := 0.0
	for _, s := range sources {
		total += s.Score
	}
	return total / float64(len(sources))
}
