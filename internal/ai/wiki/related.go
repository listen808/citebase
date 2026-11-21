package wiki

import (
	"math"
	"sort"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// 4-Signal 相关性权重（nashsu/llm_wiki 实证采用的配比）。
const (
	wDirectLink    = 3.0 // 直接 [[wikilink]] 相连
	wSourceOverlap = 4.0 // 共享同一个源文档
	wAdamicAdar    = 1.5 // 共享共同邻居，按邻居度数衰减
	wTypeAffinity  = 1.0 // 同类型页面（entity↔entity、concept↔concept）
)

// Related 一个相关页面及其相关性得分。
type Related struct {
	Slug   string  `json:"slug"`
	Title  string  `json:"title"`
	Type   string  `json:"type"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// RelatedPages 计算与指定页面最相关的页面，用于图谱扩展检索与"相关阅读"推荐。
//
// 只有链接结构而没有打分模型时，图扩展检索退化成"邻居即相关"，长尾页会被热门页压制；
// 四个信号叠加后能区分"直接引用"、"同源"和"偶然共现"。
func (c *Compiler) RelatedPages(spaceID int, slug string, limit int) ([]Related, error) {
	if limit <= 0 {
		limit = 10
	}
	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil, err
	}
	links, err := c.linkRepo.ListBySpace(spaceID)
	if err != nil {
		return nil, err
	}

	bySlug := make(map[string]*knowledgemodel.WikiPage, len(pages))
	for _, p := range pages {
		bySlug[p.Slug] = p
	}
	target, ok := bySlug[slug]
	if !ok {
		return nil, nil
	}

	neighbors := make(map[string]bool, len(links))
	degree := make(map[string]int, len(pages))
	sourceIndex := make(map[int][]string, len(pages))
	for _, p := range pages {
		for _, s := range p.Sources {
			sourceIndex[s] = append(sourceIndex[s], p.Slug)
		}
	}
	for _, l := range links {
		degree[l.FromSlug]++
		degree[l.ToSlug]++
		if l.FromSlug == slug {
			neighbors[l.ToSlug] = true
		}
		if l.ToSlug == slug {
			neighbors[l.FromSlug] = true
		}
	}

	// 二跳邻居（用于 Adamic-Adar：共同邻居）
	secondHop := make(map[string]map[string]bool)
	for _, l := range links {
		if secondHop[l.FromSlug] == nil {
			secondHop[l.FromSlug] = make(map[string]bool)
		}
		if secondHop[l.ToSlug] == nil {
			secondHop[l.ToSlug] = make(map[string]bool)
		}
		secondHop[l.FromSlug][l.ToSlug] = true
		secondHop[l.ToSlug][l.FromSlug] = true
	}

	targetSources := make(map[int]bool, len(target.Sources))
	for _, s := range target.Sources {
		targetSources[s] = true
	}

	out := make([]Related, 0, len(pages))
	for _, p := range pages {
		if p.Slug == slug {
			continue
		}
		var score float64
		var reasons []string

		if neighbors[p.Slug] {
			score += wDirectLink
			reasons = append(reasons, "直接引用")
		}
		shared := 0
		for _, s := range p.Sources {
			if targetSources[s] {
				shared++
			}
		}
		if shared > 0 {
			score += wSourceOverlap
			reasons = append(reasons, "同源文档")
		}
		aa := 0.0
		for common := range secondHop[slug] {
			if common == p.Slug {
				continue
			}
			if secondHop[p.Slug][common] {
				if d := degree[common]; d > 1 {
					aa += 1 / math.Log(float64(d))
				}
			}
		}
		if aa > 0 {
			score += wAdamicAdar * aa
			reasons = append(reasons, "共同邻居")
		}
		if p.Type == target.Type {
			score += wTypeAffinity
			reasons = append(reasons, "同类型")
		}
		if score <= 0 {
			continue
		}
		out = append(out, Related{Slug: p.Slug, Title: p.Title, Type: p.Type,
			Score: score, Reason: joinNonEmpty(reasons, "、")})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func joinNonEmpty(parts []string, sep string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += sep
		}
		out += p
	}
	return out
}
