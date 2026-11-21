package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"sort"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// KeywordExtractor 关键词提取器 - 自动提取关键词/标签
type KeywordExtractor struct {
	provider provider.Provider
}

// NewKeywordExtractor 创建关键词提取器
func NewKeywordExtractor(p provider.Provider) *KeywordExtractor {
	return &KeywordExtractor{provider: p}
}

// ExtractedKeyword 抽取的关键词
type ExtractedKeyword struct {
	Keyword  string  `json:"keyword"`  // 关键词
	Score    float64 `json:"score"`    // 权重得分
	Category string  `json:"category"` // 类别（技术/业务/概念等）
}

// KeywordExtractionResult 关键词提取结果
type KeywordExtractionResult struct {
	Keywords       []ExtractedKeyword `json:"keywords"`
	TechnicalTerms []string           `json:"technical_terms"` // 技术术语
	BusinessTerms  []string           `json:"business_terms"`  // 业务术语
	Concepts       []string           `json:"concepts"`        // 概念
}

// ExtractKeywords 从文本中提取关键词
func (k *KeywordExtractor) ExtractKeywords(ctx context.Context, content string, topN int) (*KeywordExtractionResult, error) {
	if k.provider == nil {
		// 降级到规则提取
		return k.ruleBasedExtract(content, topN), nil
	}

	if len(content) > 6000 {
		content = utils.TruncateRunes(content, 6000)
	}

	prompt := `请从以下文本中提取关键词和重要术语。

要求：
1. 提取最重要的 %d 个关键词
2. 对每个关键词给出权重得分（0-1）
3. 将关键词分类为：技术术语、业务术语、概念
4. 分别列出技术术语、业务术语、概念

文本内容：
%s

请以JSON格式输出：
{
  "keywords": [
    {
      "keyword": "关键词",
      "score": 0.9,
      "category": "technical/business/concept"
    }
  ],
  "technical_terms": ["技术术语1", "技术术语2"],
  "business_terms": ["业务术语1", "业务术语2"],
  "concepts": ["概念1", "概念2"]
}

只输出JSON，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := k.provider.Chat(ctx, "你是一个关键词提取专家，擅长识别文本中的重要术语和概念。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, topN, content)),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[KeywordExtractor] LLM extract error: %v", err)
		return k.ruleBasedExtract(content, topN), nil
	}

	result := &KeywordExtractionResult{
		Keywords:       []ExtractedKeyword{},
		TechnicalTerms: []string{},
		BusinessTerms:  []string{},
		Concepts:       []string{},
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		log.Errorf("[KeywordExtractor] JSON parse error: %v", err)
		return k.ruleBasedExtract(content, topN), nil
	}

	return result, nil
}

// ruleBasedExtract 基于规则的关键词提取（降级方案）
func (k *KeywordExtractor) ruleBasedExtract(content string, topN int) *KeywordExtractionResult {
	// 简单的词频统计
	wordFreq := make(map[string]int)
	words := strings.FieldsFunc(content, func(r rune) bool {
		return r < ' ' || (r > '~' && r < 0x4E00) || r > 0x9FFF
	})

	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) >= 2 && len(word) <= 20 {
			wordFreq[word]++
		}
	}

	// 按频率排序
	type wordCount struct {
		word  string
		count int
	}
	var sorted []wordCount
	for word, count := range wordFreq {
		sorted = append(sorted, wordCount{word, count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})

	// 取前N个
	if len(sorted) > topN {
		sorted = sorted[:topN]
	}

	var keywords []ExtractedKeyword
	for _, wc := range sorted {
		keywords = append(keywords, ExtractedKeyword{
			Keyword:  wc.word,
			Score:    float64(wc.count) / float64(sorted[0].count),
			Category: "unknown",
		})
	}

	return &KeywordExtractionResult{
		Keywords:       keywords,
		TechnicalTerms: []string{},
		BusinessTerms:  []string{},
		Concepts:       []string{},
	}
}
