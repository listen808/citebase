package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/provider"
)

// HyDERetriever HyDE检索器
type HyDERetriever struct {
	provider provider.Provider
	embedder embedding.Embedder
}

// NewHyDERetriever 创建HyDE检索器
func NewHyDERetriever(p provider.Provider, e embedding.Embedder) *HyDERetriever {
	return &HyDERetriever{
		provider: p,
		embedder: e,
	}
}

// RetrieveWithHyDE 使用HyDE策略检索
func (h *HyDERetriever) RetrieveWithHyDE(ctx context.Context, question string, spaceID int, limit int) ([]VectorResult, error) {
	if h.provider == nil || h.embedder == nil {
		return nil, fmt.Errorf("HyDE retriever not properly initialized")
	}

	hypotheticalDoc, err := h.generateHypotheticalDocument(ctx, question)
	if err != nil {
		logx.Infof("[HyDE] Generate hypothetical document error: %v, fallback to normal retrieval", err)
		return nil, err
	}

	vectorResults := embedding.VectorSearch(hypotheticalDoc, spaceID, limit, h.embedder)
	if len(vectorResults) == 0 {
		return nil, nil
	}

	var results []VectorResult
	for _, vr := range vectorResults {
		results = append(results, VectorResult{
			DocID:   vr.DocID,
			Score:   vr.Score,
			Name:    vr.Name,
			SpaceID: vr.SpaceID,
		})
	}

	return results, nil
}

// generateHypotheticalDocument 生成假设性文档
func (h *HyDERetriever) generateHypotheticalDocument(ctx context.Context, question string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	prompt := "请根据以下问题，生成一个可能包含答案的知识库文档片段。\n这个文档应该像是从知识库中找到的真实文档，包含具体的技术细节和说明。\n只输出文档内容，不要添加任何解释。\n\n问题：" + question

	resp, err := h.provider.Chat(ctx, "你是一个技术文档专家，擅长生成知识库文档。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.3))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}
