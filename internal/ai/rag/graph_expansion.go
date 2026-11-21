package rag

import (
	"context"
	"fmt"
	"strings"
)

// GraphPage 图扩展补充的 wiki 页面上下文。
// 它不是文档来源：不进引用编号、不进 Source 列表，只作为背景注入生成 prompt。
type GraphPage struct {
	Slug    string
	Title   string
	Content string
}

// WikiGraphExpander 由外部（ai 模块）注入的 wiki 图谱扩展实现。
// 语义：给定主链路命中的文档 ID（1-hop 种子），沿 wiki 链接图做
// 来源重叠到相关页（4 信号含 Adamic-Adar 共同邻居）的 2-hop 扩展。
type WikiGraphExpander interface {
	ExpandContextForDocs(ctx context.Context, spaceID int, docIDs []int, maxPages int) []GraphPage
}

// graphExpansionMaxPages 图扩展上下文页上限：补充背景不是主证据，
// 页数过多会稀释检索命中文档的注意力预算。
const graphExpansionMaxPages = 4
const graphExpansionSeedDocs = 3
const graphPageContentRunes = 1200

// SetGraphExpander 注入 wiki 图扩展（ai 模块在启动时按配置调用）。
func (r *Retriever) SetGraphExpander(e WikiGraphExpander) {
	r.graphExpander = e
}

// expandGraphContext P4：对最终命中的来源做 2-hop 图扩展，返回可注入的背景文本。
// 失败/未启用一律返回空串——图扩展是增益项，绝不能因为它拖垮主链路。
func (r *Retriever) expandGraphContext(ctx context.Context, sources []Source) string {
	if r.graphExpander == nil || len(sources) == 0 {
		return ""
	}

	seedDocs := make([]int, 0, graphExpansionSeedDocs)
	seen := make(map[int]bool)
	spaceID := 0
	for _, s := range sources {
		if spaceID == 0 {
			spaceID = s.SpaceID
		}
		if seen[s.DocumentID] {
			continue
		}
		seen[s.DocumentID] = true
		seedDocs = append(seedDocs, s.DocumentID)
		if len(seedDocs) >= graphExpansionSeedDocs {
			break
		}
	}

	pages := r.graphExpander.ExpandContextForDocs(ctx, spaceID, seedDocs, graphExpansionMaxPages)
	if len(pages) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("以下内容来自知识库图谱扩展（与命中文档通过交叉引用关联），仅作背景参考：不属于引用来源，禁止为其标注引用编号。\n")
	for _, p := range pages {
		fmt.Fprintf(&b, "\n### %s\n%s\n", p.Title, truncateRunes(p.Content, graphPageContentRunes))
	}
	return b.String()
}
