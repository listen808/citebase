package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/tokenizer"
)

// ContextStrategy 上下文处理策略
type ContextStrategy string

const (
	ContextStrategyDirect    ContextStrategy = "direct"     // 直接发送
	ContextStrategyTruncate  ContextStrategy = "truncate"   // 截断
	ContextStrategyMapReduce ContextStrategy = "map_reduce" // 分块摘要
)

// ContextManager 管理长上下文，根据 token 数自动选择策略
type ContextManager struct {
	tokenizer         tokenizer.Tokenizer
	provider          provider.Provider
	maxContextTokens  int // 最大上下文 token 数
	directThreshold   int // 直接发送阈值
	truncateThreshold int // 截断阈值（超过则 map_reduce）
}

// NewContextManager 创建上下文管理器
func NewContextManager(p provider.Provider) *ContextManager {
	return &ContextManager{
		tokenizer:         tokenizer.NewSimpleTokenizer(),
		provider:          p,
		maxContextTokens:  128000,
		directThreshold:   8000,
		truncateThreshold: 32000,
	}
}

// NewContextManagerWithLimits 创建带自定义限制的上下文管理器
func NewContextManagerWithLimits(p provider.Provider, maxContext, directThreshold, truncateThreshold int) *ContextManager {
	return &ContextManager{
		tokenizer:         tokenizer.NewSimpleTokenizer(),
		provider:          p,
		maxContextTokens:  maxContext,
		directThreshold:   directThreshold,
		truncateThreshold: truncateThreshold,
	}
}

// SelectStrategy 根据 token 数选择策略
func (cm *ContextManager) SelectStrategy(tokenCount int) ContextStrategy {
	if tokenCount < cm.directThreshold {
		return ContextStrategyDirect
	}
	if tokenCount < cm.truncateThreshold {
		return ContextStrategyTruncate
	}
	return ContextStrategyMapReduce
}

// BuildContext 构建上下文，自动选择策略
// question: 用户问题
// docs: 检索到的文档内容列表
// history: 对话历史
// 返回: 构建好的上下文文本和使用的策略
func (cm *ContextManager) BuildContext(ctx context.Context, question string, docs []string, history []ChatMessage) (string, ContextStrategy, error) {
	// 计算总 token 数
	var allParts []string
	allParts = append(allParts, question)
	allParts = append(allParts, docs...)
	for _, h := range history {
		allParts = append(allParts, h.Question, h.Answer)
	}

	totalText := strings.Join(allParts, "\n")
	tokenCount := cm.tokenizer.CountTokens(totalText)

	strategy := cm.SelectStrategy(tokenCount)
	logx.Infof("[ContextManager] token_count=%d, strategy=%s", tokenCount, strategy)

	switch strategy {
	case ContextStrategyDirect:
		return cm.buildDirectContext(question, docs, history), strategy, nil
	case ContextStrategyTruncate:
		return cm.buildTruncatedContext(question, docs, history), strategy, nil
	case ContextStrategyMapReduce:
		result, err := cm.buildMapReduceContext(ctx, question, docs, history)
		return result, strategy, err
	default:
		return cm.buildDirectContext(question, docs, history), strategy, nil
	}
}

// buildDirectContext 直接拼接所有内容
func (cm *ContextManager) buildDirectContext(question string, docs []string, history []ChatMessage) string {
	var b strings.Builder

	if len(history) > 0 {
		b.WriteString("对话历史：\n")
		for _, h := range history {
			b.WriteString(fmt.Sprintf("用户: %s\n助手: %s\n", h.Question, h.Answer))
		}
		b.WriteString("\n")
	}

	if len(docs) > 0 {
		b.WriteString("参考文档：\n")
		for i, doc := range docs {
			b.WriteString(fmt.Sprintf("--- 文档 %d ---\n%s\n\n", i+1, doc))
		}
	}

	return b.String()
}

// buildTruncatedContext 截断文档内容以适应上下文窗口
func (cm *ContextManager) buildTruncatedContext(question string, docs []string, history []ChatMessage) string {
	// 预留 token 给问题、历史和系统提示
	reservedTokens := cm.tokenizer.CountTokens(question) + 500
	for _, h := range history {
		reservedTokens += cm.tokenizer.CountTokens(h.Question) + cm.tokenizer.CountTokens(h.Answer) + 8
	}

	availableTokens := cm.directThreshold - reservedTokens
	if availableTokens < 1000 {
		availableTokens = 1000
	}

	var b strings.Builder

	if len(history) > 0 {
		b.WriteString("对话历史：\n")
		for _, h := range history {
			b.WriteString(fmt.Sprintf("用户: %s\n助手: %s\n", h.Question, h.Answer))
		}
		b.WriteString("\n")
	}

	if len(docs) > 0 {
		b.WriteString("参考文档（已截断）：\n")
		usedTokens := 0
		for i, doc := range docs {
			docTokens := cm.tokenizer.CountTokens(doc)
			if usedTokens+docTokens > availableTokens {
				// 截断这个文档
				remainingTokens := availableTokens - usedTokens
				if remainingTokens > 100 {
					truncated := truncateToTokens(doc, remainingTokens, cm.tokenizer)
					b.WriteString(fmt.Sprintf("--- 文档 %d ---\n%s\n\n", i+1, truncated))
				}
				break
			}
			b.WriteString(fmt.Sprintf("--- 文档 %d ---\n%s\n\n", i+1, doc))
			usedTokens += docTokens
		}
	}

	return b.String()
}

// buildMapReduceContext 使用 map-reduce 策略处理长上下文
func (cm *ContextManager) buildMapReduceContext(ctx context.Context, question string, docs []string, history []ChatMessage) (string, error) {
	if cm.provider == nil {
		// 无 provider 时降级为截断策略
		return cm.buildTruncatedContext(question, docs, history), nil
	}

	// Map 阶段：对每个文档生成摘要
	summaries := make([]string, 0, len(docs))
	for i, doc := range docs {
		summary, err := cm.summarizeDoc(ctx, question, doc, i+1)
		if err != nil {
			logx.Infof("[ContextManager] summarize doc %d failed: %v, using truncated version", i+1, err)
			// 降级：使用截断版本
			summary = truncateToTokens(doc, 500, cm.tokenizer)
		}
		summaries = append(summaries, summary)
	}

	// Reduce 阶段：拼接摘要
	var b strings.Builder

	if len(history) > 0 {
		b.WriteString("对话历史：\n")
		for _, h := range history {
			b.WriteString(fmt.Sprintf("用户: %s\n助手: %s\n", h.Question, h.Answer))
		}
		b.WriteString("\n")
	}

	b.WriteString("参考文档摘要：\n")
	for i, summary := range summaries {
		b.WriteString(fmt.Sprintf("--- 文档 %d 摘要 ---\n%s\n\n", i+1, summary))
	}

	return b.String(), nil
}

// summarizeDoc 对单个文档生成与问题相关的摘要
func (cm *ContextManager) summarizeDoc(ctx context.Context, question, doc string, docIndex int) (string, error) {
	// 截断过长的文档
	input := doc
	if len(input) > 12000 {
		input = input[:12000] + "..."
	}

	prompt := fmt.Sprintf(`请根据问题，从以下文档中提取与问题相关的关键信息，生成简洁摘要（不超过300字）。

问题：%s

文档内容：
%s

要求：
1. 只提取与问题相关的信息
2. 保留关键数据、定义、步骤
3. 如果文档与问题无关，返回"该文档与问题无关"`, question, input)

	resp, err := cm.provider.Chat(ctx, "你是一个文档摘要助手，擅长提取关键信息。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(512),
		provider.WithTemperature(0.2),
	)
	if err != nil {
		return "", fmt.Errorf("summarize error: %w", err)
	}

	return strings.TrimSpace(resp.Content), nil
}

// truncateToTokens 将文本截断到指定 token 数
func truncateToTokens(text string, maxTokens int, t tokenizer.Tokenizer) string {
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
