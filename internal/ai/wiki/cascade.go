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

// CascadePlan 级联的"决策"部分，抽成纯函数以便单测（RemoveSource 本身依赖 DB）。
// remove = 整页删除（只被该源贡献），prune = 只摘除该源（多源共享）。
// provenance=human 的页面永不参与任何级联。
func CascadePlan(pages []*knowledgemodel.WikiPage, sourceDocID int) (remove, prune []string) {
	if sourceDocID <= 0 {
		return nil, nil
	}
	for _, p := range pages {
		if p == nil || !wikiPageHasSource(p, sourceDocID) {
			continue
		}
		if p.Provenance == knowledgemodel.WikiProvenanceHuman {
			continue
		}
		if len(p.Sources) <= 1 {
			remove = append(remove, p.Slug)
			continue
		}
		prune = append(prune, p.Slug)
	}
	return remove, prune
}

func wikiPageHasSource(p *knowledgemodel.WikiPage, sourceDocID int) bool {
	for _, s := range p.Sources {
		if s == sourceDocID {
			return true
		}
	}
	return false
}

// RemoveSource 源文档下架时的级联清理。
//
// 规则（关键在第二条）：
//  1. 只被该源贡献的页面整页删除
//  2. 被多个源共享的实体/概念页只摘除该源的溯源条目，页面保留 ——
//     否则删一份文档会把别人贡献的知识一起抹掉
//  3. 删除后清理指向已删除页面（或从已删除页面出发）的死链
//  4. provenance=human 的页面永不因源下架被删除
func (c *Compiler) RemoveSource(spaceID int, sourceDocID int) (*CascadeResult, error) {
	pages, err := c.pageRepo.ListAllForSpace(spaceID)
	if err != nil {
		return nil, err
	}

	res := &CascadeResult{}
	alive := make(map[string]bool, len(pages))
	bySlug := make(map[string]*knowledgemodel.WikiPage, len(pages))
	for _, p := range pages {
		alive[p.Slug] = true
		bySlug[p.Slug] = p
	}

	toRemove, toPrune := CascadePlan(pages, sourceDocID)
	for _, slug := range toRemove {
		p := bySlug[slug]
		if p == nil {
			continue
		}
		if err := c.pageRepo.Delete(p.ID); err != nil {
			continue
		}
		_ = c.linkRepo.DeleteBySlug(spaceID, slug)
		delete(alive, slug)
		res.RemovedPages = append(res.RemovedPages, slug)
	}
	for _, slug := range toPrune {
		p := bySlug[slug]
		if p == nil {
			continue
		}
		if err := c.pageRepo.RemoveSource(p.ID, sourceDocID); err != nil {
			continue
		}
		res.PrunedPages = append(res.PrunedPages, slug)
	}

	// 死链清理：按"已死 slug"一次性删除。原实现按每条边的 to_slug 删，
	// 当 from 页已死而 to 页存活时会误删存活页的入边。
	links, err := c.linkRepo.ListBySpace(spaceID)
	if err == nil {
		dead := make(map[string]bool)
		for _, l := range links {
			if !alive[l.FromSlug] {
				dead[l.FromSlug] = true
			}
			if !alive[l.ToSlug] {
				dead[l.ToSlug] = true
			}
		}
		for slug := range dead {
			if err := c.linkRepo.DeleteBySlug(spaceID, slug); err == nil {
				res.CleanedLinks++
			}
		}
	}
	return res, nil
}
