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

// RelationExtractor 关系抽取器 - 从文本中提取实体间的语义关系
type RelationExtractor struct {
	provider provider.Provider
}

// NewRelationExtractor 创建关系抽取器
func NewRelationExtractor(p provider.Provider) *RelationExtractor {
	return &RelationExtractor{provider: p}
}

// ExtractedRelation 抽取的关系
type ExtractedRelation struct {
	SourceEntity string  `json:"source_entity"` // 源实体
	TargetEntity string  `json:"target_entity"` // 目标实体
	RelationType string  `json:"relation_type"` // 关系类型
	Confidence   float64 `json:"confidence"`    // 置信度 0-1
	Evidence     string  `json:"evidence"`      // 证据文本
	Context      string  `json:"context"`       // 上下文
}

// RelationExtractionResult 关系抽取结果
type RelationExtractionResult struct {
	Relations []ExtractedRelation `json:"relations"`
	Entities  []string            `json:"entities"` // 识别到的实体列表
}

// ExtractRelations 从文本中抽取关系
func (r *RelationExtractor) ExtractRelations(ctx context.Context, content string) (*RelationExtractionResult, error) {
	if r.provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	if len(content) > 8000 {
		content = utils.TruncateRunes(content, 8000)
	}

	prompt := `请从以下文本中抽取实体间的语义关系。

关系类型包括：
- belongs_to: 属于关系（如：部门属于公司）
- depends_on: 依赖关系（如：模块依赖其他模块）
- contains: 包含关系（如：系统包含组件）
- uses: 使用关系（如：应用使用技术栈）
- implements: 实现关系（如：类实现接口）
- extends: 继承/扩展关系
- related_to: 相关关系
- part_of: 部分关系
- causes: 因果关系
- requires: 需要关系

文本内容：
%s

请以JSON格式输出，包含：
{
  "entities": ["实体1", "实体2", ...],
  "relations": [
    {
      "source_entity": "源实体",
      "target_entity": "目标实体",
      "relation_type": "关系类型",
      "confidence": 0.9,
      "evidence": "证据文本",
      "context": "上下文描述"
    }
  ]
}

只输出JSON，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := r.provider.Chat(ctx, "你是一个关系抽取专家，擅长从文本中识别实体间的语义关系。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, content)),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[RelationExtractor] LLM extract error: %v", err)
		return nil, err
	}

	result := &RelationExtractionResult{
		Relations: []ExtractedRelation{},
		Entities:  []string{},
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		log.Errorf("[RelationExtractor] JSON parse error: %v", err)
		return nil, err
	}

	return result, nil
}

// ExtractRelationsFromSummaries 从多个文档摘要中抽取跨文档关系
func (r *RelationExtractor) ExtractRelationsFromSummaries(ctx context.Context, summaries []DocSummary) ([]ExtractedRelation, error) {
	if r.provider == nil || len(summaries) < 2 {
		return nil, nil
	}

	var sb strings.Builder
	sb.WriteString("以下是多个文档的摘要，请抽取文档间的实体关系：\n\n")

	for i, s := range summaries {
		if i >= 10 { // 限制最多10个文档
			break
		}
		sb.WriteString(fmt.Sprintf("文档%d (%s):\n", s.SourceDocID, s.SourceName))
		sb.WriteString(fmt.Sprintf("摘要: %s\n", s.Summary))
		if len(s.KeyPoints) > 0 {
			sb.WriteString(fmt.Sprintf("要点: %s\n", strings.Join(s.KeyPoints, "; ")))
		}
		if len(s.Tags) > 0 {
			sb.WriteString(fmt.Sprintf("标签: %s\n", strings.Join(s.Tags, ", ")))
		}
		sb.WriteString("\n")
	}

	prompt := `请从以上文档信息中抽取实体间的关系，特别关注跨文档的关系。

输出JSON数组格式：
[
  {
    "source_entity": "源实体",
    "target_entity": "目标实体",
    "relation_type": "关系类型",
    "confidence": 0.9,
    "evidence": "证据",
    "context": "上下文"
  }
]

只输出JSON数组，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := r.provider.Chat(ctx, "你是一个跨文档关系抽取专家。只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", sb.String()+"\n"+prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[RelationExtractor] Cross-doc extract error: %v", err)
		return nil, err
	}

	var relations []ExtractedRelation
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &relations); err != nil {
		log.Errorf("[RelationExtractor] JSON parse error: %v", err)
		return nil, err
	}

	return relations, nil
}

// MergeWithExistingRelations 将抽取的关系与已有关系合并
func MergeWithExistingRelations(existing []KnowledgeRelation, extracted []ExtractedRelation) []KnowledgeRelation {
	existingSet := make(map[string]bool)
	for _, r := range existing {
		key := fmt.Sprintf("%d-%d-%s", r.SourceDocID, r.TargetDocID, r.RelationType)
		existingSet[key] = true
	}

	for _, r := range extracted {
		// 这里简化处理，实际需要根据实体找到对应的文档ID
		// 暂时跳过，由调用方处理
		_ = r
	}

	return existing
}
