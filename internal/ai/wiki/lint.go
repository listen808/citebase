package wiki

import (
	"sort"
	"strconv"
	"strings"
	"time"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

// Lint 定期体检：矛盾、孤立页、稀疏社区、死链、过时内容、知识空白。
// 能确定性地判定的问题直接给出；需要模型判断的（矛盾）交给上层接线调用。
//
// 结果每轮全量重算并替换未处理项，避免陈旧问题在库里累积。
func (c *Compiler) Lint(spaceID int, staleDays int) ([]*knowledgemodel.WikiLintIssue, error) {
	if staleDays <= 0 {
		staleDays = 90
	}
	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil, err
	}
	links, err := c.linkRepo.ListBySpace(spaceID)
	if err != nil {
		return nil, err
	}

	slugSet := make(map[string]bool, len(pages))
	for _, p := range pages {
		slugSet[p.Slug] = true
	}

	degree := make(map[string]int, len(pages))
	issues := make([]*knowledgemodel.WikiLintIssue, 0)

	// 死链：指向不存在页面的交叉引用
	for _, l := range links {
		degree[l.FromSlug]++
		degree[l.ToSlug]++
		if !slugSet[l.ToSlug] {
			issues = append(issues, &knowledgemodel.WikiLintIssue{
				SpaceID: spaceID, PageSlug: l.FromSlug, Category: knowledgemodel.LintCategoryDeadLink,
				Severity: "warning", Detail: "引用了不存在的页面: " + l.ToSlug,
			})
		}
	}

	// 孤立页：几乎不与其它页面连接（度 <= 1）
	for _, p := range pages {
		if degree[p.Slug] <= 1 {
			issues = append(issues, &knowledgemodel.WikiLintIssue{
				SpaceID: spaceID, PageSlug: p.Slug, Category: knowledgemodel.LintCategoryIsolated,
				Severity: "warning", Detail: "该页面几乎没有交叉引用，可能是知识孤岛",
			})
		}
	}

	// 稀疏社区：内聚度 = 实际边数 / 可能边数，低于 0.15 说明这块知识内部关联薄弱
	for _, members := range communities(pages, links) {
		if len(members) < 3 {
			continue
		}
		internal := 0
		memberSet := make(map[string]bool, len(members))
		for _, m := range members {
			memberSet[m] = true
		}
		for _, l := range links {
			if memberSet[l.FromSlug] && memberSet[l.ToSlug] {
				internal++
			}
		}
		possible := len(members) * (len(members) - 1) / 2
		cohesion := float64(internal) / float64(possible)
		if cohesion < 0.15 {
			issues = append(issues, &knowledgemodel.WikiLintIssue{
				SpaceID: spaceID, Category: knowledgemodel.LintCategorySparse,
				Severity: "info",
				Detail:   formatSparseDetail(members, cohesion),
			})
		}
	}

	// 过时内容：超过阈值未重新编译
	cutoff := time.Now().AddDate(0, 0, -staleDays)
	for _, p := range pages {
		last := p.UpdatedAt
		if p.CompiledAt != nil {
			last = *p.CompiledAt
		}
		if last.Before(cutoff) {
			issues = append(issues, &knowledgemodel.WikiLintIssue{
				SpaceID: spaceID, PageSlug: p.Slug, Category: knowledgemodel.LintCategoryStale,
				Severity: "warning", Detail: "超过 " + strconv.Itoa(staleDays) + " 天未重新编译，可能已过时",
			})
		}
	}

	if c.lintRepo != nil {
		if err := c.lintRepo.ReplaceOpen(spaceID, issues); err != nil {
			return issues, err
		}
	}
	return issues, nil
}

// Insight Graph Insights：值得人工关注的结构性发现（P2-2）。
type Insight struct {
	Kind   string   `json:"kind"` // surprising / gap / bridge
	Slugs  []string `json:"slugs"`
	Score  float64  `json:"score"`
	Detail string   `json:"detail"`
	Action string   `json:"action"`
}

// Insights 从链接结构里找出：意外连接、知识缺口、桥节点。
// 每类都带一个可执行动作（补写/深研），让"发现缺口"能真正收敛成"补全知识"。
func (c *Compiler) Insights(spaceID int) ([]Insight, error) {
	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil, err
	}
	links, err := c.linkRepo.ListBySpace(spaceID)
	if err != nil {
		return nil, err
	}

	comms := communities(pages, links)
	communityOf := make(map[string]int, len(pages))
	for ci, members := range comms {
		for _, m := range members {
			communityOf[m] = ci
		}
	}

	out := make([]Insight, 0)

	// 意外连接：跨社区或跨类型的边，往往是被忽略的关联
	for _, l := range links {
		ci, ok1 := communityOf[l.FromSlug]
		cj, ok2 := communityOf[l.ToSlug]
		if ok1 && ok2 && ci != cj {
			out = append(out, Insight{Kind: "surprising", Slugs: []string{l.FromSlug, l.ToSlug},
				Score: l.Weight, Detail: "跨越两个知识簇的关联，值得确认是否有隐含联系",
				Action: knowledgemodel.WikiReviewActionDeepResearch})
		}
	}

	// 桥节点：邻居分布在 3 个以上社区，是把多个知识域串起来的枢纽
	neighborComms := make(map[string]map[int]bool)
	for _, l := range links {
		if neighborComms[l.FromSlug] == nil {
			neighborComms[l.FromSlug] = make(map[int]bool)
		}
		if neighborComms[l.ToSlug] == nil {
			neighborComms[l.ToSlug] = make(map[int]bool)
		}
		if ci, ok := communityOf[l.ToSlug]; ok {
			neighborComms[l.FromSlug][ci] = true
		}
		if ci, ok := communityOf[l.FromSlug]; ok {
			neighborComms[l.ToSlug][ci] = true
		}
	}
	for slug, set := range neighborComms {
		if len(set) >= 3 {
			out = append(out, Insight{Kind: "bridge", Slugs: []string{slug},
				Score: float64(len(set)), Detail: "连接 3 个以上知识簇的枢纽页面",
				Action: knowledgemodel.WikiReviewActionDeepResearch})
		}
	}

	// 知识缺口：被多次引用但还没有独立页面的概念（有入边无出边且正文很短）
	degree := make(map[string]int)
	for _, l := range links {
		degree[l.FromSlug]++
		degree[l.ToSlug]++
	}
	for _, p := range pages {
		if degree[p.Slug] >= 3 && len([]rune(p.Content)) < 200 {
			out = append(out, Insight{Kind: "gap", Slugs: []string{p.Slug},
				Score: float64(degree[p.Slug]), Detail: "被频繁引用但内容过于单薄，建议补全",
				Action: knowledgemodel.WikiReviewActionDeepResearch})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}

func formatSparseDetail(members []string, cohesion float64) string {
	limit := 5
	if len(members) < limit {
		limit = len(members)
	}
	return "知识簇 [" + strings.Join(members[:limit], ", ") + "] 内聚度 " +
		strconv.FormatFloat(cohesion, 'f', 2, 64) + "，内部交叉引用薄弱"
}
