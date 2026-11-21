package wiki

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// CascadeResult 级联删除结果。
type CascadeResult struct {
	RemovedPages []string `json:"removed_pages"`
	PrunedPages  []string `json:"pruned_pages"`
	CleanedLinks int      `json:"cleaned_links"`
}

// RemoveSource 源文档下架时的级联清理。
//
// 规则（关键在第二条）：
//  1. 只被该源贡献的页面整页删除
//  2. 被多个源共享的实体/概念页只摘除该源的溯源条目，页面保留 ——
//     否则删一份文档会把别人贡献的知识一起抹掉
//  3. 删除后清理指向已删除页面的死链，避免图谱里留下悬空引用
//  4. provenance=human 的页面永不因源下架被删除
func (c *Compiler) RemoveSource(spaceID int, sourceDocID int) (*CascadeResult, error) {
	pages, err := c.pageRepo.ListAllForSpace(spaceID)
	if err != nil {
		return nil, err
	}

	res := &CascadeResult{}
	alive := make(map[string]bool, len(pages))
	for _, p := range pages {
		alive[p.Slug] = true
	}

	for _, p := range pages {
		has := false
		for _, s := range p.Sources {
			if s == sourceDocID {
				has = true
				break
			}
		}
		if !has {
			continue
		}
		if p.Provenance == knowledgemodel.WikiProvenanceHuman {
			continue
		}
		if len(p.Sources) == 1 {
			if err := c.pageRepo.Delete(p.ID); err != nil {
				continue
			}
			_ = c.linkRepo.DeleteBySlug(spaceID, p.Slug)
			delete(alive, p.Slug)
			res.RemovedPages = append(res.RemovedPages, p.Slug)
			continue
		}
		if err := c.pageRepo.RemoveSource(p.ID, sourceDocID); err != nil {
			continue
		}
		res.PrunedPages = append(res.PrunedPages, p.Slug)
	}

	// 死链清理
	links, err := c.linkRepo.ListBySpace(spaceID)
	if err == nil {
		for _, l := range links {
			if !alive[l.FromSlug] || !alive[l.ToSlug] {
				_ = c.linkRepo.DeleteBySlug(spaceID, l.ToSlug)
				res.CleanedLinks++
			}
		}
	}
	return res, nil
}
