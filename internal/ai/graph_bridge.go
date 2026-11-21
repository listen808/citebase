package ai

import (
	"context"

	"github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/ai/wiki"
)

// wikiGraphExpanderAdapter P4：把 wiki 包的 2-hop 图扩展适配成
// 主检索链路（rag.Retriever）期望的 WikiGraphExpander 接口。
// 放在模块层是因为 rag 与 wiki 互不引用（避免包间环）。
type wikiGraphExpanderAdapter struct {
	c *wiki.Compiler
}

// ExpandContextForDocs 实现 rag.WikiGraphExpander。
func (a *wikiGraphExpanderAdapter) ExpandContextForDocs(ctx context.Context, spaceID int, docIDs []int, maxPages int) []rag.GraphPage {
	pages := a.c.ExpandContextForDocs(spaceID, docIDs, maxPages)
	out := make([]rag.GraphPage, 0, len(pages))
	for _, p := range pages {
		out = append(out, rag.GraphPage{Slug: p.Slug, Title: p.Title, Content: p.Content})
	}
	return out
}
