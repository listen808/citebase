package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/tokenizer"
)

// ChunkedContent 分块后的内容块
type ChunkedContent struct {
	ID         string  `json:"id"`
	Text       string  `json:"text"`
	StartPos   int     `json:"start_pos"`
	EndPos     int     `json:"end_pos"`
	Importance float64 `json:"importance"` // 重要性评分 0-1
	Tokens     int     `json:"tokens"`
}

// LongContextConfig 长上下文处理配置
type LongContextConfig struct {
	MaxContextTokens    int     // 最大上下文 token 数
	DirectThreshold     int     // 直接发送阈值
	TruncateThreshold   int     // 截断阈值
	MinChunkTokens      int     // 最小分块 token 数
	MaxChunkTokens      int     // 最大分块 token 数
	ImportanceThreshold float64 // 重要性阈值，低于此值的块会被丢弃
	SummaryThreshold    int     // 超过此 token 数启用摘要
}

// DefaultLongContextConfig 默认配置
func DefaultLongContextConfig() *LongContextConfig {
	return &LongContextConfig{
		MaxContextTokens:    128000,
		DirectThreshold:     8000,
		TruncateThreshold:   32000,
		MinChunkTokens:      500,
		MaxChunkTokens:      4000,
		ImportanceThreshold: 0.3,
		SummaryThreshold:    20000,
	}
}

// LongContextHandler 长上下文处理器，统一管理超长文档处理
type LongContextHandler struct {
	config     *LongContextConfig
	tokenizer  tokenizer.Tokenizer
	provider   provider.Provider
	ranker     *ImportanceRanker
	summarizer *HierarchicalSummarizer
	budgetMgr  *ContextBudgetManager
}

// NewLongContextHandler 创建长上下文处理器
func NewLongContextHandler(p provider.Provider) *LongContextHandler {
	return NewLongContextHandlerWithConfig(p, DefaultLongContextConfig())
}

// NewLongContextHandlerWithConfig 创建带配置的处理器
func NewLongContextHandlerWithConfig(p provider.Provider, cfg *LongContextConfig) *LongContextHandler {
	return &LongContextHandler{
		config:     cfg,
		tokenizer:  tokenizer.NewSimpleTokenizer(),
		provider:   p,
		ranker:     NewImportanceRanker(p),
		summarizer: NewHierarchicalSummarizer(p),
		budgetMgr:  NewContextBudgetManager(cfg.MaxContextTokens),
	}
}

// ProcessLongDocument 处理超长文档
// doc: 待处理的文档内容
// question: 用户问题（用于相关性评估）
// 返回: 处理后的上下文内容
func (h *LongContextHandler) ProcessLongDocument(ctx context.Context, doc, question string) (string, error) {
	totalTokens := h.tokenizer.CountTokens(doc)
	logx.Infof("[LongContextHandler] Processing document, tokens=%d", totalTokens)

	// 策略1: 直接发送（文档较短）
	if totalTokens < h.config.DirectThreshold {
		return doc, nil
	}

	// 策略2: 语义分块 + 重要性排序 + 截断（文档中等长度）
	if totalTokens < h.config.SummaryThreshold {
		return h.processMediumDocument(ctx, doc, question)
	}

	// 策略3: 层级摘要（超长文档）
	return h.processUltraLongDocument(ctx, doc, question)
}

// processMediumDocument 处理中等长度文档
func (h *LongContextHandler) processMediumDocument(ctx context.Context, doc, question string) (string, error) {
	// 语义分块
	chunks := h.semanticChunk(doc)
	logx.Infof("[LongContextHandler] Chunked into %d pieces", len(chunks))

	// 评估每个块的重要性
	for i := range chunks {
		chunks[i].Importance = h.ranker.EvaluateImportance(ctx, question, chunks[i].Text)
		chunks[i].Tokens = h.tokenizer.CountTokens(chunks[i].Text)
	}

	// 按重要性排序
	h.sortByImportance(chunks)

	// Token预算分配
	questionTokens := h.tokenizer.CountTokens(question)
	availableTokens := h.config.TruncateThreshold - questionTokens - 1000 // 预留空间

	// 选择重要内容
	selected := h.selectContentByBudget(chunks, availableTokens)

	return h.buildContext(question, selected), nil
}

// processUltraLongDocument 处理超长文档
func (h *LongContextHandler) processUltraLongDocument(ctx context.Context, doc, question string) (string, error) {
	// 语义分块
	chunks := h.semanticChunk(doc)
	logx.Infof("[LongContextHandler] Ultra-long doc chunked into %d pieces", len(chunks))

	// 层级摘要
	summary, err := h.summarizer.Summarize(ctx, chunks, question)
	if err != nil {
		logx.Infof("[LongContextHandler] Hierarchical summarize failed: %v, falling back to truncate", err)
		return h.truncateToThreshold(doc, h.config.TruncateThreshold), nil
	}

	return summary, nil
}

// semanticChunk 语义分块
func (h *LongContextHandler) semanticChunk(doc string) []ChunkedContent {
	var chunks []ChunkedContent

	// 按段落分割
	paragraphs := strings.Split(doc, "\n\n")
	currentChunk := ""
	currentPos := 0

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := h.tokenizer.CountTokens(para)

		// 如果当前块加上这个段落超过最大块大小，先保存当前块
		if h.tokenizer.CountTokens(currentChunk)+paraTokens > h.config.MaxChunkTokens && currentChunk != "" {
			chunks = append(chunks, ChunkedContent{
				ID:       fmt.Sprintf("chunk_%d", len(chunks)),
				Text:     strings.TrimSpace(currentChunk),
				StartPos: currentPos,
				EndPos:   currentPos + len(currentChunk),
				Tokens:   h.tokenizer.CountTokens(currentChunk),
			})
			currentPos += len(currentChunk) + 2
			currentChunk = ""
		}

		// 如果单个段落就超过最大块大小，需要拆分
		if paraTokens > h.config.MaxChunkTokens {
			if currentChunk != "" {
				chunks = append(chunks, ChunkedContent{
					ID:       fmt.Sprintf("chunk_%d", len(chunks)),
					Text:     strings.TrimSpace(currentChunk),
					StartPos: currentPos,
					EndPos:   currentPos + len(currentChunk),
					Tokens:   h.tokenizer.CountTokens(currentChunk),
				})
				currentPos += len(currentChunk) + 2
				currentChunk = ""
			}
			// 递归拆分长段落
			subChunks := h.splitLongParagraph(para, currentPos)
			chunks = append(chunks, subChunks...)
			currentPos = subChunks[len(subChunks)-1].EndPos
		} else {
			if currentChunk != "" {
				currentChunk += "\n\n" + para
			} else {
				currentChunk = para
			}
		}
	}

	// 保存最后一个块
	if currentChunk != "" {
		chunks = append(chunks, ChunkedContent{
			ID:       fmt.Sprintf("chunk_%d", len(chunks)),
			Text:     strings.TrimSpace(currentChunk),
			StartPos: currentPos,
			EndPos:   currentPos + len(currentChunk),
			Tokens:   h.tokenizer.CountTokens(currentChunk),
		})
	}

	return chunks
}

// splitLongParagraph 拆分过长的段落
func (h *LongContextHandler) splitLongParagraph(para string, startPos int) []ChunkedContent {
	var chunks []ChunkedContent
	maxLen := h.config.MaxChunkTokens * 4 // 估算字符数

	runes := []rune(para)
	current := ""

	for i := 0; i < len(runes); i++ {
		current += string(runes[i])

		if len(current) >= maxLen || i == len(runes)-1 {
			chunks = append(chunks, ChunkedContent{
				ID:       fmt.Sprintf("chunk_%d_sub", len(chunks)),
				Text:     strings.TrimSpace(current),
				StartPos: startPos,
				EndPos:   startPos + len(current),
				Tokens:   h.tokenizer.CountTokens(current),
			})
			startPos += len(current)
			current = ""
		}
	}

	return chunks
}

// sortByImportance 按重要性排序
func (h *LongContextHandler) sortByImportance(chunks []ChunkedContent) {
	// 冒泡排序（简化版）
	for i := 0; i < len(chunks)-1; i++ {
		for j := 0; j < len(chunks)-i-1; j++ {
			if chunks[j].Importance < chunks[j+1].Importance {
				chunks[j], chunks[j+1] = chunks[j+1], chunks[j]
			}
		}
	}
}

// selectContentByBudget 根据Token预算选择内容
func (h *LongContextHandler) selectContentByBudget(chunks []ChunkedContent, budget int) []ChunkedContent {
	var selected []ChunkedContent
	usedTokens := 0

	for _, chunk := range chunks {
		// 跳过重要性低于阈值的块
		if chunk.Importance < h.config.ImportanceThreshold {
			continue
		}

		if usedTokens+chunk.Tokens <= budget {
			selected = append(selected, chunk)
			usedTokens += chunk.Tokens
		} else {
			// 如果剩余空间足够容纳部分内容，截断添加
			remaining := budget - usedTokens
			if remaining >= h.config.MinChunkTokens {
				truncated := h.truncateToTokens(chunk.Text, remaining)
				selected = append(selected, ChunkedContent{
					ID:         chunk.ID,
					Text:       truncated,
					Importance: chunk.Importance,
					Tokens:     remaining,
				})
			}
			break
		}
	}

	return selected
}

// truncateToTokens 将文本截断到指定token数
func (h *LongContextHandler) truncateToTokens(text string, maxTokens int) string {
	runes := []rune(text)
	currentTokens := 0

	for i, r := range runes {
		var tokenCost int
		if r > 0x7F {
			tokenCost = 2
		} else {
			tokenCost = 1
		}

		if (currentTokens+tokenCost)/4 > maxTokens {
			return string(runes[:i]) + "..."
		}
		currentTokens += tokenCost
	}

	return text
}

// truncateToThreshold 截断到阈值
func (h *LongContextHandler) truncateToThreshold(text string, maxTokens int) string {
	return h.truncateToTokens(text, maxTokens)
}

// buildContext 构建上下文
func (h *LongContextHandler) buildContext(question string, chunks []ChunkedContent) string {
	if len(chunks) == 0 {
		return ""
	}

	var b strings.Builder

	b.WriteString("参考文档：\n\n")
	for i, chunk := range chunks {
		b.WriteString(fmt.Sprintf("--- 文档片段 %d ---\n%s\n\n", i+1, chunk.Text))
	}

	return b.String()
}

// ProcessWithHistory 处理带历史记录的上下文
func (h *LongContextHandler) ProcessWithHistory(ctx context.Context, question string, docs []string, history []ChatMessage) (string, ContextStrategy, error) {
	// 合并所有文档
	var allDocText strings.Builder
	for i, doc := range docs {
		allDocText.WriteString(fmt.Sprintf("\n--- 文档 %d ---\n%s", i+1, doc))
	}

	// 处理长文档
	processedDoc, err := h.ProcessLongDocument(ctx, allDocText.String(), question)
	if err != nil {
		return "", ContextStrategyDirect, err
	}

	// 使用已有的 ContextManager 构建最终上下文
	cm := NewContextManager(h.provider)
	cm.maxContextTokens = h.config.MaxContextTokens
	cm.directThreshold = h.config.DirectThreshold
	cm.truncateThreshold = h.config.TruncateThreshold

	return cm.BuildContext(ctx, question, []string{processedDoc}, history)
}
