package wiki

import (
	"context"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// 派生页面上限：每个派生页都是一次 LLM 调用。
// 不设上限的实体抽取会把一篇长文膨胀成几十个页面，首次摄入成本失控。
const (
	maxConceptsPerDoc   = 3
	maxEntitiesPerDoc   = 4 // people + organizations + technologies 合计
	candidateExcerptLen = 6000
)

// AnalyzeDocumentToCandidates 两步摄入（Two-Step CoT）的完整链路：
//
//	Step 1 分析：实体/概念/人物/组织抽取（LLM 失败时 analyzer 内部回退正则）
//	Step 2 编排：把分析结果映射成知识候选（source / concept / entity 三型），
//	            连同源文档一起交给 Plan / Generate 建页与交叉引用
//
// 返回的第一个候选永远是 source 型源摘要页 —— 即使 Step1 失败，
// 源摘要页仍会产出（降级为"只有源页"的最小编译结果）。
func AnalyzeDocumentToCandidates(ctx context.Context, p provider.Provider, docID int, docName, content string) []Candidate {
	cands := []Candidate{{
		Name:        docName,
		PageType:    string(knowledgemodel.WikiPageTypeSource),
		Title:       docName,
		SourceDocID: docID,
		SourceText:  content,
	}}
	if p == nil || strings.TrimSpace(content) == "" {
		return cands
	}

	extracted, err := analyzer.NewEntityExtractor(p).ExtractFromContent(ctx, content)
	if err != nil {
		logx.Infof("[WikiIngest] Step1 entity extraction failed for doc %d: %v", docID, err)
		return cands
	}

	// 概念页：定义 + 原文摘录作为生成依据，回链源页
	for _, c := range extracted.Concepts {
		if len(cands) > maxConceptsPerDoc {
			break
		}
		if strings.TrimSpace(c.Name) == "" || c.Name == docName {
			continue
		}
		conf := c.Confidence
		if conf <= 0 {
			conf = 0.70
		}
		cands = append(cands, Candidate{
			Name:        c.Name,
			PageType:    string(knowledgemodel.WikiPageTypeConcept),
			Title:       c.Name,
			Summary:     firstSentence(c.Definition, 120),
			SourceDocID: docID,
			SourceText: fmt.Sprintf("定义：%s\n\n原文摘录：\n%s",
				c.Definition, truncate(content, candidateExcerptLen)),
			Links:      []string{docName},
			Confidence: conf,
		})
	}

	// 实体页：人物 / 组织 / 技术合并计数
	entityCount := 0
	appendEntity := func(name, desc string, confidence float64) {
		if entityCount >= maxEntitiesPerDoc {
			return
		}
		name = strings.TrimSpace(name)
		if name == "" || name == docName {
			return
		}
		if confidence <= 0 {
			confidence = 0.70
		}
		entityCount++
		cands = append(cands, Candidate{
			Name:        name,
			PageType:    string(knowledgemodel.WikiPageTypeEntity),
			Title:       name,
			Summary:     firstSentence(desc, 120),
			SourceDocID: docID,
			SourceText: fmt.Sprintf("描述：%s\n\n原文摘录：\n%s",
				desc, truncate(content, candidateExcerptLen)),
			Links:      []string{docName},
			Confidence: confidence,
		})
	}
	for _, per := range extracted.People {
		appendEntity(per.Name, fmt.Sprintf("%s（角色：%s）。%s", per.Name, per.Role, per.Description), per.Confidence)
	}
	for _, org := range extracted.Organizations {
		appendEntity(org.Name, fmt.Sprintf("%s（类型：%s）。%s", org.Name, org.Type, org.Description), org.Confidence)
	}
	for _, tech := range extracted.Technologies {
		appendEntity(tech.Name, fmt.Sprintf("%s（类别：%s）。", tech.Name, tech.Category), tech.Confidence)
	}
	return cands
}
