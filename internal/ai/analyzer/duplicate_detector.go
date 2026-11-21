package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/utils"
)

// DuplicateDetector 重复文档检测器
type DuplicateDetector struct {
	provider provider.Provider
}

// NewDuplicateDetector 创建检测器
func NewDuplicateDetector(p provider.Provider) *DuplicateDetector {
	return &DuplicateDetector{
		provider: p,
	}
}

// DocPair 文档对
type DocPair struct {
	DocA       *workspacemodel.Document `json:"doc_a"`
	DocB       *workspacemodel.Document `json:"doc_b"`
	Similarity float64                  `json:"similarity"`
	Reason     string                   `json:"reason"`
}

// DetectSimilarDocs 检测相似文档
func (d *DuplicateDetector) DetectSimilarDocs(ctx context.Context, docs []*workspacemodel.Document, results *AnalysisResult) ([]DocPair, error) {
	if len(docs) < 2 {
		return nil, nil
	}

	// 使用摘要计算相似度（比全文更快）
	docSummaries := make(map[int]string)
	for _, summary := range results.Summaries {
		docSummaries[summary.SourceDocID] = summary.Summary
	}

	// 计算两两相似度
	var pairs []DocPair
	for i := 0; i < len(docs); i++ {
		for j := i + 1; j < len(docs); j++ {
			summaryA := docSummaries[docs[i].ID]
			summaryB := docSummaries[docs[j].ID]

			if summaryA == "" || summaryB == "" {
				continue
			}

			// 使用余弦相似度计算
			similarity, err := d.calculateSimilarity(ctx, summaryA, summaryB)
			if err != nil {
				log.Errorf("[DuplicateDetector] calculate similarity error: %v", err)
				continue
			}

			// 相似度 > 0.85 认为是重复
			if similarity >= 0.85 {
				pairs = append(pairs, DocPair{
					DocA:       docs[i],
					DocB:       docs[j],
					Similarity: similarity,
					Reason:     fmt.Sprintf("摘要相似度：%.2f", similarity),
				})
			} else if similarity >= 0.70 {
				// 0.70-0.85 认为是疑似重复
				pairs = append(pairs, DocPair{
					DocA:       docs[i],
					DocB:       docs[j],
					Similarity: similarity,
					Reason:     fmt.Sprintf("疑似重复，摘要相似度：%.2f", similarity),
				})
			}
		}
	}

	return pairs, nil
}

// calculateSimilarity 计算两个文本的相似度
func (d *DuplicateDetector) calculateSimilarity(ctx context.Context, textA, textB string) (float64, error) {
	// 使用简单的字符串相似度（降级方案）
	return d.simpleSimilarity(textA, textB), nil
}

// SimpleSimilarityPublic 公开的字符串相似度方法（供 API 层调用）
func (d *DuplicateDetector) SimpleSimilarityPublic(a, b string) float64 {
	return d.simpleSimilarity(a, b)
}

// simpleSimilarity 简单的字符串相似度（Jaccard 相似度）
func (d *DuplicateDetector) simpleSimilarity(a, b string) float64 {
	wordsA := tokenize(a)
	wordsB := tokenize(b)

	intersection := 0
	union := len(wordsA) + len(wordsB)

	for wordA := range wordsA {
		if wordsB[wordA] {
			intersection++
		} else {
			union++
		}
	}

	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

// tokenize 简单分词
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	// 按空格和标点分词
	for _, word := range splitBySpace(text) {
		if len(word) > 1 {
			words[word] = true
		}
	}
	return words
}

func splitBySpace(text string) []string {
	// 简单实现，实际应该使用更好的分词
	result := make([]string, 0)
	current := ""
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' || r == ',' || r == '.' || r == '，' || r == '。' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// MergePlan 合并计划
type MergePlan struct {
	TargetDocID    int      `json:"target_doc_id"`    // 目标文档 ID（保留的文档）
	SourceDocID    int      `json:"source_doc_id"`    // 源文档 ID（被合并的文档）
	SectionsToKeep []string `json:"sections_to_keep"` // 保留的章节
	GapsToFill     []string `json:"gaps_to_fill"`     // 需要补充的信息
	Reason         string   `json:"reason"`           // 合并理由
}

// GenerateMergePlan 生成合并计划
func (d *DuplicateDetector) GenerateMergePlan(ctx context.Context, pair DocPair) (*MergePlan, error) {
	if d.provider == nil {
		// 降级方案：保留 ID 较小的文档
		targetID := pair.DocA.ID
		sourceID := pair.DocB.ID
		if pair.DocA.ID > pair.DocB.ID {
			targetID = pair.DocB.ID
			sourceID = pair.DocA.ID
		}
		return &MergePlan{
			TargetDocID:    targetID,
			SourceDocID:    sourceID,
			SectionsToKeep: []string{"来自两个文档的所有内容"},
			GapsToFill:     []string{},
			Reason:         "自动合并：保留 ID 较小的文档",
		}, nil
	}

	prompt := fmt.Sprintf(`有两个相似文档，请给出合并建议。

文档 A: ID=%d, 名称=%s
文档 B: ID=%d, 名称=%s

相似度：%.2f
判定理由：%s

请输出 JSON：
{
  "target_doc_id": %d,
  "source_doc_id": %d,
  "sections_to_keep": ["章节 1", "章节 2"],
  "gaps_to_fill": ["需要补充的信息"],
  "reason": "合并理由"
}`,
		pair.DocA.ID, pair.DocA.Name,
		pair.DocB.ID, pair.DocB.Name,
		pair.Similarity, pair.Reason,
		pair.DocA.ID, pair.DocB.ID)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := d.provider.Chat(ctx, "你是一个文档管理专家，只输出 JSON，不要解释。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[DuplicateDetector] generate merge plan error: %v", err)
		// 降级方案
		targetID := pair.DocA.ID
		sourceID := pair.DocB.ID
		if pair.DocA.ID > pair.DocB.ID {
			targetID = pair.DocB.ID
			sourceID = pair.DocA.ID
		}
		return &MergePlan{
			TargetDocID:    targetID,
			SourceDocID:    sourceID,
			SectionsToKeep: []string{"来自两个文档的所有内容"},
			GapsToFill:     []string{},
			Reason:         "自动合并：保留 ID 较小的文档",
		}, nil
	}

	plan := &MergePlan{}
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), plan); err != nil {
		log.Errorf("[DuplicateDetector] merge plan JSON parse error: %v, raw: %s", err, jsonStr)
		targetID := pair.DocA.ID
		sourceID := pair.DocB.ID
		if pair.DocA.ID > pair.DocB.ID {
			targetID = pair.DocB.ID
			sourceID = pair.DocA.ID
		}
		plan = &MergePlan{
			TargetDocID:    targetID,
			SourceDocID:    sourceID,
			SectionsToKeep: []string{"来自两个文档的所有内容"},
			GapsToFill:     []string{},
			Reason:         "自动合并：保留 ID 较小的文档",
		}
	}

	return plan, nil
}
