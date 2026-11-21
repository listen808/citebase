package rag

import (
	"fmt"
	"regexp"
	"strings"
)

// citationRe 匹配答案中的 [n] 引用标记。
var citationRe = regexp.MustCompile(`\[(\d+)\]`)

// 引用可验证性状态。用于前端对每条来源标注"是否可点击溯源 / 是否被答案真正引用"。
const (
	// VerifyStatusCited 片段出现在原文且被答案引用（最可信）。
	VerifyStatusCited = "cited"
	// VerifyStatusUnverified 来源已检索，但答案未显式引用该编号。
	VerifyStatusUnverified = "unverified"
	// VerifyStatusNotFound 片段无法在原文中定位（不可信，需警示）。
	VerifyStatusNotFound = "not_found"
)

// ReasoningStep 推理链中的单个可可视化步骤（检索/重排/生成等）。
type ReasoningStep struct {
	Stage  string `json:"stage"`            // retrieve / rerank / generate / verify
	Title  string `json:"title"`            // 人类可读标题
	Detail string `json:"detail,omitempty"` // 步骤说明
	Count  int    `json:"count,omitempty"`  // 如召回数量
	Status string `json:"status,omitempty"` // ok / warn / skip
}

// VerifyCitations 对来源做轻量可验证性标注（纯文本，无 LLM 依赖）。
//
// 规则：
//   - 若答案文本包含形如 [n] 的引用标记且该 n 等于 source.CitationIndex，则标记 Cited=true，状态 cited。
//   - 否则根据 snippet 是否能在原文片段（source.Snippet）中保留足够连续子串，
//     判断 not_found 或 unverified。这里用简单的子串覆盖估算，作为前端可信度提示。
//
// originalTexts 为可选参数：key 为 DocumentID，value 为该文档原文全文（用于更严格校验）。
// 不传时仅基于答案中的引用标记与 snippet 自身判断。
func VerifyCitations(answer string, sources []Source, originalTexts map[int]string) []Source {
	citedIdx := extractCitationIndices(answer)
	out := make([]Source, len(sources))
	for i, s := range sources {
		ns := s
		_, cited := citedIdx[s.CitationIndex]
		ns.Cited = cited
		if cited {
			ns.VerifyStatus = VerifyStatusCited
		} else if _, ok := originalTexts[s.DocumentID]; ok {
			// 有原文时做片段存在性校验
			if snippetFoundIn(originalTexts[s.DocumentID], s.Snippet) {
				ns.VerifyStatus = VerifyStatusUnverified
			} else {
				ns.VerifyStatus = VerifyStatusNotFound
			}
		} else {
			ns.VerifyStatus = VerifyStatusUnverified
		}
		ns.DocURL = DocURLFor(s.SpaceID, s.DocumentID)
		out[i] = ns
	}
	return out
}

// DocURLFor 构造文档的可点击跳转 URL（前端路由约定）。
func DocURLFor(spaceID, docID int) string {
	if docID <= 0 {
		return ""
	}
	return fmt.Sprintf("/space/%d/doc/%d", spaceID, docID)
}

// extractCitationIndices 从答案文本提取所有 [n] 引用编号。
func extractCitationIndices(answer string) map[int]struct{} {
	idx := make(map[int]struct{})
	for _, m := range citationRe.FindAllStringSubmatch(answer, -1) {
		if len(m) >= 2 {
			var n int
			if _, err := fmt.Sscanf(m[1], "%d", &n); err == nil && n > 0 {
				idx[n] = struct{}{}
			}
		}
	}
	return idx
}

// snippetFoundIn 判断 snippet 是否能（近似）在原文中找到。
// 采用"最长连续 12 字子串"匹配，容忍少量标点差异。
func snippetFoundIn(original, snippet string) bool {
	original = strings.ReplaceAll(original, "\n", " ")
	snippet = strings.ReplaceAll(snippet, "\n", " ")
	runes := []rune(snippet)
	const win = 12
	if len(runes) <= win {
		return strings.Contains(original, snippet)
	}
	for start := 0; start+win <= len(runes); start += win / 2 {
		window := string(runes[start : start+win])
		if strings.Contains(original, window) {
			return true
		}
	}
	return false
}

// EnrichAnswer 回填可验证引用字段到 AIAnswer（不修改原 answer 文本）。
// originalTexts 可选；传 nil 时跳过原文校验。
func EnrichAnswer(ans *AIAnswer, originalTexts map[int]string) *AIAnswer {
	if ans == nil {
		return nil
	}
	ans.Sources = VerifyCitations(ans.Answer, ans.Sources, originalTexts)
	return ans
}

// BuildReasoningChain 将检索/重排/校验/生成结果组装为可视化推理链。
// 该函数为纯组装，便于在 Ask 流程各阶段收集后统一调用。
func BuildReasoningChain(retrieved, reranked int, verifyNote string, hasHallucination bool, generationOK bool) []ReasoningStep {
	steps := []ReasoningStep{
		{Stage: "retrieve", Title: "检索相关文档", Detail: "向量 + 关键词混合召回", Count: retrieved, Status: statusOf(retrieved > 0)},
		{Stage: "rerank", Title: "重排序", Detail: "按问题与片段相关性重新排序", Count: reranked, Status: statusOf(reranked > 0)},
		{Stage: "generate", Title: "生成回答", Detail: "基于 Top-K 文档片段生成自然语言回答", Status: statusOf(generationOK)},
		{Stage: "verify", Title: "引用可验证性校验", Detail: verifyNote, Status: statusOf(!hasHallucination)},
	}
	return steps
}

func statusOf(ok bool) string {
	if ok {
		return "ok"
	}
	return "warn"
}

// GraphPathCitation is a citation derived from a knowledge graph path traversal.
type GraphPathCitation struct {
	Path         GraphPath `json:"path"`
	Cited        bool      `json:"cited"`
	VerifyStatus string    `json:"verify_status"`
	DocURL       string    `json:"doc_url,omitempty"`
}

// VerifyGraphPathCitations checks whether graph traversal paths are referenced
// in the answer text. Paths are matched by relation type keywords and doc IDs
// in citation markers like [G:n] where n is the graph path index.
func VerifyGraphPathCitations(answer string, paths []GraphPath) []GraphPathCitation {
	if len(paths) == 0 {
		return nil
	}

	citedPaths := extractGraphCitationIndices(answer)
	out := make([]GraphPathCitation, 0, len(paths))
	for i, p := range paths {
		gc := GraphPathCitation{
			Path:  p,
			Cited: false,
		}
		// Check if cited via [G:n] marker
		if _, ok := citedPaths[i]; ok {
			gc.Cited = true
			gc.VerifyStatus = VerifyStatusCited
		} else {
			// Check if relation keywords appear in the answer
			lowerAnswer := strings.ToLower(answer)
			relKeywords := strings.Split(strings.ToLower(p.Relation), "_")
			for _, kw := range relKeywords {
				if len(kw) >= 3 && strings.Contains(lowerAnswer, kw) {
					gc.Cited = true
					gc.VerifyStatus = VerifyStatusUnverified
					break
				}
			}
			if !gc.Cited {
				gc.VerifyStatus = VerifyStatusUnverified
			}
		}
		// Generate doc URLs for path endpoints
		if len(p.DocIDs) > 0 {
			gc.DocURL = fmt.Sprintf("/space/0/doc/%d", p.DocIDs[0])
		}
		out = append(out, gc)
	}
	return out
}

// extractGraphCitationIndices parses [G:n] markers from the answer text.
func extractGraphCitationIndices(answer string) map[int]struct{} {
	idx := make(map[int]struct{})
	re := regexp.MustCompile(`\[G:(\d+)\]`)
	for _, m := range re.FindAllStringSubmatch(answer, -1) {
		if len(m) >= 2 {
			var n int
			if _, err := fmt.Sscanf(m[1], "%d", &n); err == nil && n >= 0 {
				idx[n] = struct{}{}
			}
		}
	}
	return idx
}

// EnrichAnswerWithGraphPaths adds graph path citation info to the AIAnswer.
func EnrichAnswerWithGraphPaths(ans *AIAnswer, paths []GraphPath) *AIAnswer {
	if ans == nil || len(paths) == 0 {
		return ans
	}
	// P3：把图谱路径引用附加到 AIAnswer，供前端渲染
	ans.GraphPathCitations = VerifyGraphPathCitations(ans.Answer, paths)
	return ans
}
