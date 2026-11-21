package wiki

import (
	"sort"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// WikiContextPage 图扩展输出的单页上下文。
type WikiContextPage struct {
	Slug    string
	Title   string
	Content string
}

const graphExpandContentRunes = 1500

// ExpandContextForDocs 2-hop 图扩展（P4，供主问答链路注入背景上下文）：
//
//	1-hop 种子：与命中文档有来源重叠（Sources ∩ docIDs）的已发布 wiki 页；
//	2-hop 扩展：对种子页做 RelatedPages（4 信号：直连/同源/Adamic-Adar/型别亲和），
//	            得分跨种子累加后取 top maxPages。
//
// 返回按贡献分排序的页面内容（每页截断），空切片表示无可扩展内容。
func (c *Compiler) ExpandContextForDocs(spaceID int, docIDs []int, maxPages int) []WikiContextPage {
	if maxPages <= 0 {
		maxPages = 4
	}
	docSet := make(map[int]bool, len(docIDs))
	for _, id := range docIDs {
		docSet[id] = true
	}

	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil
	}

	// 1-hop：来源重叠页作为种子
	seeds := make([]*knowledgemodel.WikiPage, 0)
	for _, p := range pages {
		for _, s := range p.Sources {
			if docSet[s] {
				seeds = append(seeds, p)
				break
			}
		}
	}
	if len(seeds) == 0 {
		return nil
	}
	// 种子按溯源数量降序，最"核心"的页面优先扩展
	sort.Slice(seeds, func(i, j int) bool { return seeds[i].SourceCount > seeds[j].SourceCount })
	const maxSeeds = 3
	if len(seeds) > maxSeeds {
		seeds = seeds[:maxSeeds]
	}

	// 2-hop：相关页打分累加（排除种子自身）
	seedSet := make(map[string]bool, len(seeds))
	for _, s := range seeds {
		seedSet[s.Slug] = true
	}
	score := make(map[string]float64)
	for _, seed := range seeds {
		related, err := c.RelatedPages(spaceID, seed.Slug, 5)
		if err != nil {
			continue
		}
		for _, r := range related {
			if seedSet[r.Slug] {
				continue
			}
			score[r.Slug] += r.Score
		}
	}

	type scoredPair struct {
		slug  string
		score float64
	}
	ranked := make([]scoredPair, 0, len(score))
	for slug, s := range score {
		ranked = append(ranked, scoredPair{slug: slug, score: s})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > maxPages {
		ranked = ranked[:maxPages]
	}

	bySlug := make(map[string]*knowledgemodel.WikiPage, len(pages))
	for _, p := range pages {
		bySlug[p.Slug] = p
	}
	out := make([]WikiContextPage, 0, len(ranked))
	for _, r := range ranked {
		p := bySlug[r.slug]
		if p == nil {
			continue
		}
		out = append(out, WikiContextPage{
			Slug:    p.Slug,
			Title:   p.Title,
			Content: truncate(p.Content, graphExpandContentRunes),
		})
	}
	return out
}
