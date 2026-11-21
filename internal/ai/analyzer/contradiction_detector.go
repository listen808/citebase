package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// ContradictionDetector 矛盾检测器
type ContradictionDetector struct {
	provider provider.Provider
}

// NewContradictionDetector 创建矛盾检测器
func NewContradictionDetector(p provider.Provider) *ContradictionDetector {
	return &ContradictionDetector{provider: p}
}

// DocumentContradiction 文档矛盾
type DocumentContradiction struct {
	DocAID      int    `json:"doc_a_id"`
	DocAName    string `json:"doc_a_name"`
	DocASection string `json:"doc_a_section"`
	DocBID      int    `json:"doc_b_id"`
	DocBName    string `json:"doc_b_name"`
	DocBSection string `json:"doc_b_section"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // high/medium/low
	Type        string `json:"type"`     // factual/logical/temporal
}

// DetectContradictions 检测文档间的矛盾
func (cd *ContradictionDetector) DetectContradictions(ctx context.Context, docs []DocSummary) ([]DocumentContradiction, error) {
	if cd.provider == nil {
		return nil, fmt.Errorf("contradiction detector not initialized")
	}

	if len(docs) < 2 {
		return nil, nil
	}

	var contradictions []DocumentContradiction

	batchSize := 5
	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]

		batchContradictions, err := cd.detectBatchContradictions(ctx, batch)
		if err != nil {
			log.Errorf("[ContradictionDetector] Batch %d error: %v", i/batchSize, err)
			continue
		}
		contradictions = append(contradictions, batchContradictions...)
	}

	return contradictions, nil
}

func (cd *ContradictionDetector) detectBatchContradictions(ctx context.Context, docs []DocSummary) ([]DocumentContradiction, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var docDescs []string
	for _, doc := range docs {
		snippet := doc.Summary
		runes := []rune(snippet)
		if len(runes) > 500 {
			snippet = string(runes[:500]) + "..."
		}
		docDescs = append(docDescs, fmt.Sprintf("文档ID:%d 名称:%s\n摘要:%s\n关键点:%s",
			doc.SourceDocID, doc.SourceName, snippet, strings.Join(doc.KeyPoints, ", ")))
	}

	prompt := `请检测以下文档之间是否存在矛盾或不一致。

文档列表：
` + strings.Join(docDescs, "\n\n") + `

输出JSON数组：
[{
  "doc_a_id": 1,
  "doc_a_name": "文档A",
  "doc_a_section": "相关章节",
  "doc_b_id": 2,
  "doc_b_name": "文档B",
  "doc_b_section": "相关章节",
  "description": "矛盾描述",
  "severity": "high/medium/low",
  "type": "factual/logical/temporal"
}]

如果没有矛盾，输出空数组 []`

	resp, err := cd.provider.Chat(ctx, "你是一个文档一致性检查专家。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var contradictions []DocumentContradiction
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &contradictions); err != nil {
		return nil, fmt.Errorf("parse contradictions: %w", err)
	}

	return contradictions, nil
}
