package wiki

import (
	"context"
	"fmt"
	"sort"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
)

const deepResearchTopPages = 5
const deepResearchMaxQueries = 3

// DeepResearchReport 深度研究回灌的结果。
type DeepResearchReport struct {
	Queries   []string `json:"queries"`        // LLM 生成的研究查询
	Reference []string `json:"reference"`      // 参考的已有页面 slug
	PageSlug  string   `json:"page_slug"`      // 生成的研究纪要页（draft，待审核）
	ReviewID  int      `json:"review_id"`      // 对应的审核项
	Created   bool     `json:"created"`        // 是否成功产出纪要页
	Note      string   `json:"note,omitempty"` // 未产出时的原因
}

// DeepResearch 针对 Graph Insight（知识缺口 / 意外连接 / 桥节点）的回灌闭环：
//
//  1. LLM 依据 insight 生成 2-3 条研究查询（失败时退化为 insight 的页面名）
//  2. 在已发布页面上做词项匹配，收集与查询相关的已知材料
//  3. LLM 生成一份研究纪要（analysis 型）：已知综合 + 缺口分析 + 下一步该找什么
//  4. 纪要以 draft 落页并自动创建审核项（action=deep_research），
//     人工审核通过后发布 —— 与治理流同一条管道，不另开旁路
//
// 源文档侧的补充仍走既有 Compile 入口（前端引导用户导入相关文档）。
func (c *Compiler) DeepResearch(ctx context.Context, spaceID int, ins Insight) (*DeepResearchReport, error) {
	report := &DeepResearchReport{}
	if len(ins.Slugs) == 0 {
		report.Note = "insight 缺少关联页面"
		return report, nil
	}

	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil, err
	}
	if len(pages) == 0 {
		report.Note = "空间内暂无已发布页面"
		return report, nil
	}

	// 1) 生成研究查询（W5：多智能体开启时走 orchestrator 路径）
	var queries []string
	if c.multiagentOrch != nil {
		queries = c.generateQueriesViaMultiagent(ctx, ins, c.multiagentOrch)
	} else {
		queries = c.generateResearchQueries(ctx, ins)
	}
	report.Queries = queries

	// 2) 收集已知材料（词项匹配打分，取 top N）
	reference, material := collectReferenceMaterial(pages, queries, deepResearchTopPages)
	report.Reference = reference

	// 3) 生成研究纪要
	title := "深度研究：" + strings.Join(ins.Slugs, " / ")
	if len([]rune(title)) > 200 {
		title = truncate(title, 200)
	}
	slug := Slugify("deep-research-" + strings.Join(ins.Slugs, "-"))
	if len(slug) > 180 {
		slug = truncate(slug, 180)
	}

	var content string
	if c.multiagentOrch != nil {
		content = c.generateContentViaMultiagent(ctx, queries, material, c.multiagentOrch)
	} else {
		content = c.generateResearchContent(ctx, queries, material)
	}
	if content == "" {
		// 无 LLM 或 LLM 失败：退化为确定性的材料汇编，仍然可审核、可追溯
		content = deterministicResearchContent(queries, reference)
	}

	// 4) 落 draft + 创建审核项
	if _, err := c.pageRepo.PlanPlaceholder(spaceID, slug,
		string(knowledgemodel.WikiPageTypeAnalysis), title,
		firstSentence(content, 120), 0); err != nil {
		return nil, fmt.Errorf("plan research page: %w", err)
	}
	if err := c.pageRepo.SaveDraft(spaceID, slug, content,
		firstSentence(content, 120), ContentHash(content), nil, 0.75); err != nil {
		return nil, fmt.Errorf("save research draft: %w", err)
	}
	// 溯源：纪要参考了哪些页面（页面间引用入图谱）
	for _, ref := range reference {
		_ = c.linkRepo.Upsert(spaceID, slug, ref, "research", 1)
		_ = c.linkRepo.Upsert(spaceID, ref, slug, "research", 1)
	}
	if c.reviewRepo != nil {
		review := &knowledgemodel.WikiReview{
			SpaceID: spaceID, PageSlug: slug,
			Kind:   string(knowledgemodel.WikiPageTypeAnalysis),
			Action: knowledgemodel.WikiReviewActionDeepResearch,
			Reason: "Graph Insight（" + ins.Kind + "）触发的深度研究纪要，待人工审核",
		}
		if err := c.reviewRepo.Create(review); err == nil {
			report.ReviewID = review.ID
		}
	}
	InvalidateIndex(spaceID)

	report.PageSlug = slug
	report.Created = true
	return report, nil
}

// generateResearchQueries LLM 生成研究查询；失败退化为 insight 关联页面名。
func (c *Compiler) generateResearchQueries(ctx context.Context, ins Insight) []string {
	fallback := make([]string, 0, len(ins.Slugs))
	for _, s := range ins.Slugs {
		fallback = append(fallback, strings.ReplaceAll(s, "-", " "))
	}
	if c.provider == nil {
		return fallback
	}
	prompt := fmt.Sprintf("知识图谱体检发现：[%s] %s（相关页面：%s）\n"+
		"请生成 %d 条用于补充研究该缺口的检索查询，每行一条，不要编号和解释。",
		ins.Kind, ins.Detail, strings.Join(ins.Slugs, "、"), deepResearchMaxQueries)
	resp, err := c.provider.Chat(ctx,
		"你是知识库研究查询生成器，输出的每行将直接用于检索。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(200), provider.WithTemperature(0.3))
	if err != nil {
		return fallback
	}
	var out []string
	for _, line := range strings.Split(resp.Content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			out = append(out, line)
		}
		if len(out) >= deepResearchMaxQueries {
			break
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// generateResearchContent LLM 生成研究纪要正文；失败返回空串。
func (c *Compiler) generateResearchContent(ctx context.Context, queries []string, material string) string {
	if c.provider == nil {
		return ""
	}
	system := `你是知识库研究助手。基于给定的已有页面材料，围绕研究问题写一份研究纪要：
1. 已知综合：材料里对这个主题已经确认了什么（逐条列出，标注来自哪个页面）
2. 缺口分析：研究问题里哪些部分材料回答不了
3. 下一步：为了补全缺口，建议查找什么类型的资料/数据/文档
只使用材料中存在的信息，绝不编造。`
	prompt := fmt.Sprintf("研究问题：\n%s\n\n已有材料：\n%s",
		strings.Join(queries, "\n"), truncate(material, 10000))
	resp, err := c.provider.Chat(ctx, system,
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(1500), provider.WithTemperature(0.3))
	if err != nil {
		logx.Infof("[WikiDeepResearch] LLM generate failed: %v", err)
		return ""
	}
	return strings.TrimSpace(resp.Content)
}

// deterministicResearchContent 无 LLM 时的退化产出：研究问题 + 相关页面汇编。
func deterministicResearchContent(queries, reference []string) string {
	var b strings.Builder
	b.WriteString("# 研究问题\n\n")
	for _, q := range queries {
		b.WriteString("- " + q + "\n")
	}
	b.WriteString("\n# 相关已知页面\n\n")
	for _, r := range reference {
		b.WriteString("- [[" + r + "]]\n")
	}
	return b.String()
}

// collectReferenceMaterial 用查询词项在已发布页面上做匹配打分，返回 top N 的
// （slug 列表, 拼接材料文本）。刻意不用 BM25 全套 —— 查询是 LLM 生成的自然语言，
// 词项重叠度已够排序，BM25 精排留给检索主链路。
func collectReferenceMaterial(pages []*knowledgemodel.WikiPage, queries []string, topN int) ([]string, string) {
	qTokens := make(map[string]bool)
	for _, q := range queries {
		for _, t := range tokenize(q) {
			qTokens[t] = true
		}
	}

	type scored struct {
		page  *knowledgemodel.WikiPage
		score int
	}
	hits := make([]scored, 0, len(pages))
	for _, p := range pages {
		text := p.Title + "\n" + p.Summary + "\n" + p.Content
		tokens := make(map[string]bool)
		for _, t := range tokenize(text) {
			tokens[t] = true
		}
		score := 0
		for t := range qTokens {
			if tokens[t] {
				score++
			}
		}
		if score > 0 {
			hits = append(hits, scored{page: p, score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	if len(hits) > topN {
		hits = hits[:topN]
	}

	slugs := make([]string, 0, len(hits))
	var b strings.Builder
	for _, h := range hits {
		slugs = append(slugs, h.page.Slug)
		fmt.Fprintf(&b, "## %s\n%s\n\n", h.page.Title, truncate(h.page.Content, 3000))
	}
	return slugs, b.String()
}
