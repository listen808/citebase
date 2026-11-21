package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

// HierarchicalSummarizer 层级摘要生成器
// 支持对超长文档（100K+ tokens）进行层级化的摘要处理
type HierarchicalSummarizer struct {
	provider provider.Provider
	maxInput int // 单次最大输入 token 数
}

// NewHierarchicalSummarizer 创建层级摘要生成器
func NewHierarchicalSummarizer(p provider.Provider) *HierarchicalSummarizer {
	return &HierarchicalSummarizer{
		provider: p,
		maxInput: 8000, // 留空间给 prompt
	}
}

// Summarize 对分块后的内容进行层级摘要
// chunks: 已分块的内容
// question: 用户问题（用于聚焦摘要）
// 返回: 最终摘要文本
func (hs *HierarchicalSummarizer) Summarize(ctx context.Context, chunks []ChunkedContent, question string) (string, error) {
	if hs.provider == nil {
		return hs.fallbackSummarize(chunks), nil
	}

	logx.Infof("[HierarchicalSummarizer] Starting hierarchical summarize for %d chunks", len(chunks))

	// 第一层：对每个块生成摘要
	level1Summaries, err := hs.level1Summarize(ctx, chunks, question)
	if err != nil {
		return "", err
	}

	// 如果第一层摘要数量较少，直接返回拼接结果
	if len(level1Summaries) <= 5 {
		return hs.buildFinalSummary(question, level1Summaries), nil
	}

	// 第二层：对第一层摘要再进行分组合成
	level2Summaries, err := hs.level2Summarize(ctx, level1Summaries, question)
	if err != nil {
		return hs.buildFinalSummary(question, level1Summaries), nil
	}

	// 如果第二层摘要仍然较多，进行第三层
	if len(level2Summaries) > 3 {
		level3Summaries, err := hs.level3Summarize(ctx, level2Summaries, question)
		if err != nil {
			return hs.buildFinalSummary(question, level2Summaries), nil
		}
		return hs.buildFinalSummary(question, level3Summaries), nil
	}

	return hs.buildFinalSummary(question, level2Summaries), nil
}

// level1Summarize 第一层摘要：对每个内容块生成独立摘要
func (hs *HierarchicalSummarizer) level1Summarize(ctx context.Context, chunks []ChunkedContent, question string) ([]string, error) {
	summaries := make([]string, 0, len(chunks))

	for i, chunk := range chunks {
		summary, err := hs.summarizeSingle(ctx, question, chunk.Text, fmt.Sprintf("文档片段 %d", i+1))
		if err != nil {
			logx.Infof("[HierarchicalSummarizer] Failed to summarize chunk %d: %v", i, err)
			// 降级：使用原始文本截断
			summaries = append(summaries, hs.truncateToTokens(chunk.Text, 500))
		} else {
			summaries = append(summaries, summary)
		}
	}

	return summaries, nil
}

// level2Summarize 第二层摘要：对第一层摘要进行分组合成
func (hs *HierarchicalSummarizer) level2Summarize(ctx context.Context, summaries []string, question string) ([]string, error) {
	// 每 3-5 个摘要为一组
	groupSize := 4
	var groups [][]string

	for i := 0; i < len(summaries); i += groupSize {
		end := i + groupSize
		if end > len(summaries) {
			end = len(summaries)
		}
		groups = append(groups, summaries[i:end])
	}

	results := make([]string, 0, len(groups))

	for i, group := range groups {
		combined := strings.Join(group, "\n\n")
		summary, err := hs.summarizeGroup(ctx, question, combined, fmt.Sprintf("第 %d 组", i+1))
		if err != nil {
			logx.Infof("[HierarchicalSummarizer] Failed to summarize group %d: %v", i, err)
			results = append(results, combined)
		} else {
			results = append(results, summary)
		}
	}

	return results, nil
}

// level3Summarize 第三层摘要：对第二层摘要进行最终合成
func (hs *HierarchicalSummarizer) level3Summarize(ctx context.Context, summaries []string, question string) ([]string, error) {
	combined := strings.Join(summaries, "\n\n")

	prompt := fmt.Sprintf(`请将以下多个摘要片段整合为一个连贯的最终摘要，要求：
1. 保留所有重要信息
2. 按逻辑顺序组织
3. 删除重复内容
4. 长度控制在 1500 字以内

问题聚焦：%s

摘要内容：
%s

请输出一段完整、连贯的摘要。`, question, combined)

	resp, err := hs.provider.Chat(ctx, "你是一个专业的文档摘要专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2000), provider.WithTemperature(0.3))

	if err != nil {
		return nil, err
	}

	return []string{strings.TrimSpace(resp.Content)}, nil
}

// summarizeSingle 对单个内容块生成摘要
func (hs *HierarchicalSummarizer) summarizeSingle(ctx context.Context, question, content, label string) (string, error) {
	// 截断过长的内容
	truncated := hs.truncateToTokens(content, hs.maxInput-500)

	prompt := fmt.Sprintf(`请根据以下问题，从文档片段中提取相关信息，生成简洁摘要。

问题：%s

文档片段：
%s

要求：
1. 只提取与问题相关的信息
2. 保留关键数据、定义、步骤
3. 摘要不超过 300 字
4. 如果片段与问题无关，返回"[无关]"

请输出一段简洁的摘要。`, question, truncated)

	resp, err := hs.provider.Chat(ctx, "你是一个专业的文档摘要专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(400), provider.WithTemperature(0.2))

	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(resp.Content)
	// 如果返回"[无关]"，表示内容不相关
	if strings.HasPrefix(result, "[无关]") {
		return "", nil
	}

	return result, nil
}

// summarizeGroup 对一组摘要进行合成
func (hs *HierarchicalSummarizer) summarizeGroup(ctx context.Context, question, content, label string) (string, error) {
	prompt := fmt.Sprintf(`请将以下多个文档摘要整合为一个连贯的段落，要求：
1. 保留所有重要信息
2. 按逻辑顺序组织
3. 删除重复内容
4. 长度控制在 500 字以内

问题聚焦：%s

多个摘要：
%s

请输出一段完整、连贯的摘要。`, question, content)

	resp, err := hs.provider.Chat(ctx, "你是一个专业的文档摘要专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(800), provider.WithTemperature(0.3))

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(resp.Content), nil
}

// buildFinalSummary 构建最终摘要
func (hs *HierarchicalSummarizer) buildFinalSummary(question string, summaries []string) string {
	var b strings.Builder

	b.WriteString("参考文档摘要（基于问题：")
	b.WriteString(question)
	b.WriteString("）：\n\n")

	for i, summary := range summaries {
		b.WriteString(fmt.Sprintf("--- 摘要 %d ---\n%s\n\n", i+1, summary))
	}

	return b.String()
}

// fallbackSummarize 无 LLM 时的降级摘要策略
func (hs *HierarchicalSummarizer) fallbackSummarize(chunks []ChunkedContent) string {
	// 选择重要性最高的几个块
	const maxChunks = 5

	if len(chunks) <= maxChunks {
		var b strings.Builder
		for i, chunk := range chunks {
			b.WriteString(fmt.Sprintf("--- 文档片段 %d ---\n%s\n\n", i+1, chunk.Text))
		}
		return b.String()
	}

	// 选择前 N 个最重要的块
	var b strings.Builder
	b.WriteString("参考文档摘要：\n\n")

	for i := 0; i < maxChunks; i++ {
		b.WriteString(fmt.Sprintf("--- 文档片段 %d ---\n%s\n\n", i+1, chunks[i].Text))
	}

	b.WriteString(fmt.Sprintf("\n（共 %d 个片段，仅显示最重要的 %d 个）", len(chunks), maxChunks))

	return b.String()
}

// truncateToTokens 将文本截断到指定 token 数
func (hs *HierarchicalSummarizer) truncateToTokens(text string, maxTokens int) string {
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
