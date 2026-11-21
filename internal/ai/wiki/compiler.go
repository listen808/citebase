// Package wiki 实现 LLM Wiki 的"编译期"：把源文档在入库时编译成持久、互链、
// 可版本化的知识页面，而不是让每次查询重新检索原始片段。
//
// 并发边界按状态依赖划分，而不是按模型调用划分（AWS 实践结论）：
//   - Plan   串行：决定"知识应该落到哪一页"，必须看到此前已生效的规划结果
//   - Generate 并发：页面内容生成耗时长，彼此无状态依赖
//   - Publish  乐观锁：判断"草稿是否还有资格写入这一页"
package wiki

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

// Candidate 一个待编译的知识候选（概念/实体/源摘要等）。
type Candidate struct {
	Name        string // 知识名，用于生成 slug 与语义去重
	PageType    string // concept / entity / source / method / compare / analysis
	Title       string
	Summary     string
	SourceDocID int
	SourceText  string   // 源文档正文片段，Generate 阶段的依据
	Links       []string // 关联到的其它候选名，用于建立交叉引用
	Confidence  float64  // 提取置信度 0–1，从分析器传递
}

// CompileCost 一次编译的 token 消耗（P2 成本追踪）。
// generateOne 每页生成时累加，PlanResult 汇总后随编译日志落库，
// 让管理员能看清"一次全量编译烧了多少 token"，与全局 token 配额分开核算。
type CompileCost struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (c *CompileCost) Add(u provider.TokenUsage) {
	c.PromptTokens += u.PromptTokens
	c.CompletionTokens += u.CompletionTokens
	c.TotalTokens += u.TotalTokens
}

// PlanResult 规划结果。
type PlanResult struct {
	Created []string // 新建 planned 占位的 slug
	Updated []string // 命中已有页面、仅并入溯源的 slug
	Skipped int      // 内容哈希未变更直接跳过的候选数
	Cost    CompileCost
}

// Compiler Wiki 编译管线。
type Compiler struct {
	pageRepo       *knowledgerepo.WikiPageRepository
	linkRepo       *knowledgerepo.WikiLinkRepository
	pinRepo        *knowledgerepo.WikiPinRepository
	reviewRepo     *knowledgerepo.WikiReviewRepository
	lintRepo       *knowledgerepo.WikiLintRepository
	histRepo       *knowledgerepo.WikiPageHistoryRepository
	provider       provider.Provider
	multiagentOrch MultiagentRunner // W5：多智能体编排器（可选，配置开启时注入）

	planMu sync.Mutex // Plan 串行：知识划分是有状态决策，必须互斥
}

func NewCompiler(p provider.Provider) *Compiler {
	return &Compiler{
		pageRepo:   knowledgerepo.NewWikiPageRepository(),
		linkRepo:   knowledgerepo.NewWikiLinkRepository(),
		pinRepo:    knowledgerepo.NewWikiPinRepository(),
		reviewRepo: knowledgerepo.NewWikiReviewRepository(),
		lintRepo:   knowledgerepo.NewWikiLintRepository(),
		histRepo:   knowledgerepo.NewWikiPageHistoryRepository(),
		provider:   p,
	}
}

// Provider P1：暴露底层模型，供两步摄入的 Step1 分析复用同一 provider。
func (c *Compiler) Provider() provider.Provider {
	return c.provider
}

// Plan 串行规划阶段：为每个候选决定"新建页面"还是"更新已有页面"，并立即写入占位。
//
// 为什么必须串行：并发任务若基于同一份目录快照各自规划，"税率政策" 可能被任务 A 建成
// tax、被任务 B 建成 tax-policy。两个 ID 在存储层无冲突，但语义等价 —— 冲突发生在
// 语义空间，传统锁识别不了。串行 + 立即落库占位，让后序任务看得见前序决定。
func (c *Compiler) Plan(ctx context.Context, spaceID int, candidates []Candidate) (*PlanResult, error) {
	c.planMu.Lock()
	defer c.planMu.Unlock()

	res := &PlanResult{}
	if len(candidates) == 0 {
		return res, nil
	}

	existing, err := c.pageRepo.ListAllForSpace(spaceID)
	if err != nil {
		return nil, fmt.Errorf("load wiki index: %w", err)
	}

	for _, cand := range candidates {
		if cand.Name == "" {
			continue
		}
		slug := Slugify(cand.Name)

		// 三级去重：精确 slug → 标题模糊匹配 → 可选 LLM 语义确认
		match := matchExisting(existing, slug, cand.Name)
		if match == nil && c.provider != nil {
			match = c.llmMatch(ctx, existing, cand.Name)
		}

		hash := ContentHash(cand.SourceText)
		if match != nil && hash != "" && match.ContentHash == hash {
			// 源内容未变更，直接跳过重编译（对应 nashsu 的 SHA256 增量缓存）
			res.Skipped++
			continue
		}

		pageType := cand.PageType
		if pageType == "" {
			pageType = string(knowledgemodel.WikiPageTypeConcept)
		}
		p, err := c.pageRepo.PlanPlaceholder(spaceID, slug, pageType, cand.Title, cand.Summary, cand.SourceDocID)
		if err != nil {
			logx.Infof("[WikiCompiler] Plan placeholder failed slug=%s: %v", slug, err)
			continue
		}
		if hash != "" {
			p.ContentHash = hash
			_ = c.pageRepo.SaveDraft(spaceID, slug, p.Content, p.Summary, hash, p.Sources, cand.Confidence)
		}
		if match != nil {
			res.Updated = append(res.Updated, slug)
		} else {
			res.Created = append(res.Created, slug)
			existing = append(existing, p)
			// P1 #4: 取代链 —— 新页面创建时，检查是否有同标题但不同 slug 的旧页面
			// 旧页面被新页面取代（仅限 LLM provenance）
			c.checkSupersession(spaceID, slug, cand.Title, existing)
		}
	}

	return res, nil
}

// matchExisting 精确 + 模糊两级匹配已有页面。
func matchExisting(pages []*knowledgemodel.WikiPage, slug, name string) *knowledgemodel.WikiPage {
	name = strings.TrimSpace(name)
	for _, p := range pages {
		if p.Slug == slug {
			return p
		}
	}
	// 模糊：一方包含另一方，或公共前缀 >= 4 个字符（中文场景常见的简称/全称关系）
	for _, p := range pages {
		if p.Title == "" {
			continue
		}
		a, b := strings.TrimSpace(p.Title), name
		if a == b {
			return p
		}
		if len(a) >= 2 && len(b) >= 2 && (strings.Contains(a, b) || strings.Contains(b, a)) {
			return p
		}
		if commonPrefixLen(a, b) >= 4 {
			return p
		}
	}
	return nil
}

// checkSupersession P1 #4: 新页面创建时，检查是否有同标题但不同 slug 的旧 LLM 页面。
// 如果有，标记旧页面为被取代（superseded_by = 新 slug）。
func (c *Compiler) checkSupersession(spaceID int, newSlug, title string, existing []*knowledgemodel.WikiPage) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	for _, p := range existing {
		if p.Slug == newSlug || p.SupersededBy != "" {
			continue
		}
		if p.Provenance == knowledgemodel.WikiProvenanceHuman {
			continue
		}
		// 同标题不同 slug = 新页面取代旧页面
		if strings.TrimSpace(p.Title) == title {
			if err := c.pageRepo.MarkSuperseded(spaceID, p.Slug, newSlug); err == nil {
				if c.linkRepo != nil {
					_ = c.linkRepo.Upsert(spaceID, newSlug, p.Slug, "supersedes", 1.0)
				}
				logx.Infof("[WikiCompiler] Superseded page %s → %s (same title '%s')", p.Slug, newSlug, title)
			}
		}
	}
}

// llmMatch 语义级确认：让模型判断候选名与已有页面中是否有同一知识。
// 仅在精确/模糊都未命中时调用，控制成本。
func (c *Compiler) llmMatch(ctx context.Context, pages []*knowledgemodel.WikiPage, name string) *knowledgemodel.WikiPage {
	if len(pages) == 0 {
		return nil
	}
	const maxCompare = 30
	names := make([]string, 0, maxCompare)
	index := make(map[string]*knowledgemodel.WikiPage, maxCompare)
	for i, p := range pages {
		if i >= maxCompare {
			break
		}
		label := p.Title
		if label == "" {
			label = p.Slug
		}
		names = append(names, label)
		index[label] = p
	}

	prompt := fmt.Sprintf("已有知识页面：\n%s\n\n新概念：%s\n\n"+
		"如果新概念与已有页面中的某一项指的是同一份知识，只回复该页面名称原文；否则只回复 NONE。不要解释。",
		strings.Join(names, "\n"), name)

	resp, err := c.provider.Chat(ctx,
		"你是知识库去重助手，负责判断概念是否与已有页面重复。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(64), provider.WithTemperature(0))
	if err != nil {
		return nil
	}
	answer := strings.TrimSpace(resp.Content)
	if answer == "" || answer == "NONE" {
		return nil
	}
	return index[answer]
}

// Generate 并发生成阶段：为每个已规划的页面生成正文。
// 返回本轮汇总的 token 成本（*CompileCost，供调用方写入编译日志）。
func (c *Compiler) Generate(ctx context.Context, spaceID int, candidates []Candidate, concurrency int) (*CompileCost, error) {
	if len(candidates) == 0 {
		return &CompileCost{}, nil
	}
	if concurrency <= 0 {
		concurrency = 4
	}
	if concurrency > len(candidates) {
		concurrency = len(candidates)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	var cost CompileCost

	for _, cand := range candidates {
		if cand.SourceText == "" {
			continue
		}
		wg.Add(1)
		go func(cd Candidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			u, err := c.generateOne(ctx, spaceID, cd)
			mu.Lock()
			if u != nil {
				cost.Add(*u)
			}
			if err != nil && firstErr == nil {
				firstErr = err
			}
			mu.Unlock()
		}(cand)
	}
	wg.Wait()
	return &cost, firstErr
}

// generateOne 生成单页正文并落草稿；返回本次模型调用的 token 用量（供成本追踪累加）。
func (c *Compiler) generateOne(ctx context.Context, spaceID int, cand Candidate) (*provider.TokenUsage, error) {
	slug := Slugify(cand.Name)
	system := `你是知识库编纂助手。把给定的资料编译为一个结构化的知识页面。
要求：
1. 只写资料中确实存在的事实，不确定就省略，绝不编造
2. 使用 Markdown，正文里用 [[页面名]] 指向相关概念
3. 先给一句话定义，再分节展开
4. 不要重复粘贴大段原文，要做提炼和重组` + purposePromptBlock(spaceID) + schemaPromptBlock(spaceID)
	prompt := fmt.Sprintf("页面名称：%s\n页面类型：%s\n\n资料：\n%s",
		cand.Title, cand.PageType, truncate(cand.SourceText, 8000))

	resp, err := c.provider.Chat(ctx, system,
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("generate page %s: %w", slug, err)
	}

	content := resp.Content
	summary := firstSentence(content, 120)
	sources := []int{}
	if cand.SourceDocID > 0 {
		sources = append(sources, cand.SourceDocID)
	}
	if err := c.pageRepo.SaveDraft(spaceID, slug, content, summary, ContentHash(cand.SourceText), sources, cand.Confidence); err != nil {
		return nil, fmt.Errorf("save draft %s: %w", slug, err)
	}

	// 交叉引用：双向写入，保证图谱连通
	for _, l := range cand.Links {
		if l == "" || l == cand.Name {
			continue
		}
		toSlug := Slugify(l)
		_ = c.linkRepo.Upsert(spaceID, slug, toSlug, "ref", 1)
		_ = c.linkRepo.Upsert(spaceID, toSlug, slug, "ref", 1)
	}
	return &resp.TokenUsage, nil
}

// Publish 发布阶段：版本号 +1。基线版本不匹配时返回 ErrStaleDraft，调用方应重新生成草稿。
func (c *Compiler) Publish(spaceID int, slug string, baseVersion int) (*knowledgemodel.WikiPage, error) {
	p, err := c.pageRepo.Publish(spaceID, slug, baseVersion)
	if err == nil {
		InvalidateIndex(spaceID)
	}
	return p, err
}

// GetPageForRead / ListPagesForRead / LinksForRead 供 MCP 服务端等只读外部入口使用。
// 只暴露已发布页面：未审核的草稿不得泄漏给外部 Agent。
func (c *Compiler) GetPageForRead(spaceID int, slug string) (*knowledgemodel.WikiPage, error) {
	p, err := c.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		return nil, err
	}
	if p.Status != string(knowledgemodel.WikiPageStatusPublished) {
		return nil, fmt.Errorf("页面未发布，不可读取")
	}
	return p, nil
}

func (c *Compiler) ListPagesForRead(spaceID int, status string) ([]*knowledgemodel.WikiPage, error) {
	if status == "" {
		status = string(knowledgemodel.WikiPageStatusPublished)
	}
	return c.pageRepo.ListBySpace(spaceID, status)
}

// ListPendingReviews P5：待人工裁决的审核项（只读），
// 供 MCP 服务端暴露给外部 Agent —— 帮人看清"哪些知识还没确认"。
func (c *Compiler) ListPendingReviews(spaceID int) ([]*knowledgemodel.WikiReview, error) {
	return c.reviewRepo.ListBySpace(spaceID, knowledgemodel.WikiReviewStatusPending)
}

func (c *Compiler) LinksForRead(spaceID int) ([]*knowledgemodel.WikiLink, error) {
	return c.linkRepo.ListBySpace(spaceID)
}

// ExportPage 把单个已发布页渲染成带 YAML frontmatter 的 markdown 文件。
// 用于 OKF/知识格式导出：slug 即文件名，frontmatter 带类型/状态/来源，
// 正文保留 [[wiki-link]] 原样（Obsidian 等工具直接可解析）。
func (c *Compiler) ExportPage(p *knowledgemodel.WikiPage) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("id: " + itoa(p.ID) + "\n")
	b.WriteString("slug: " + p.Slug + "\n")
	b.WriteString("type: " + p.Type + "\n")
	b.WriteString("status: " + p.Status + "\n")
	b.WriteString("provenance: " + p.Provenance + "\n")
	b.WriteString("version: " + itoa(p.Version) + "\n")
	b.WriteString("title: " + yamlQuote(p.Title) + "\n")
	b.WriteString("confidence: " + strconv.FormatFloat(p.Confidence, 'f', 2, 64) + "\n")
	if !p.UpdatedAt.IsZero() {
		b.WriteString("updated: " + p.UpdatedAt.Format("2006-01-02") + "\n")
	}
	if len(p.Sources) > 0 {
		b.WriteString("sources: [" + intSlice(p.Sources) + "]\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(p.Content)
	if !strings.HasSuffix(p.Content, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

// ExportSpace 导出空间内所有已发布页，返回「文件名 → markdown 内容」。
// 文件名用 slug（唯一、稳定、可直接当 Obsidian/OHF 的节点名）。
func (c *Compiler) ExportSpace(spaceID int) (map[string]string, error) {
	pages, err := c.ListPagesForRead(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(pages))
	for _, p := range pages {
		out[p.Slug+".md"] = c.ExportPage(p)
	}
	return out, nil
}

func itoa(n int) string { return strconv.Itoa(n) }

func intSlice(ids []int) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.Itoa(id))
	}
	return strings.Join(parts, ",")
}

// yamlQuote 给 YAML 字符串值加引号并转义，避免标题里的冒号/换行破坏 frontmatter。
func yamlQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// History 返回页面版本历史（降序），供前端版本时间线展示。
func (c *Compiler) History(spaceID int, slug string) ([]*knowledgemodel.WikiPageHistory, error) {
	return c.histRepo.ListByPage(spaceID, slug)
}

// Rollback 回滚到指定历史版本：历史内容进入草稿区后走 Publish。
// 返回发布后的页面。
func (c *Compiler) Rollback(spaceID int, slug string, toVersion int) (*knowledgemodel.WikiPage, error) {
	p, _, err := c.pageRepo.Rollback(spaceID, slug, toVersion)
	if err != nil {
		return nil, err
	}
	return c.Publish(spaceID, slug, p.BaseVersion)
}

// 写入时矛盾标注的正文标记（确定性标记块，便于重复摄入时先去重）。
const contradictionMarker = "<!-- contradiction: -->"

// MarkContradictions 写入时矛盾标注：新页面与既有页面冲突时，把待裁决说明
// 写进页面正文（可见、可随版本历史追溯），并在 Lint 登记一条 open 的 contradiction。
// 矛盾项由人裁决（resolve），Lint 定期体检只负责结构性问题，不替代这条链路。
func (c *Compiler) MarkContradictions(spaceID int, slug string, items []string) error {
	if len(items) == 0 {
		return nil
	}
	p, err := c.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		return err
	}
	// 去重：同一批矛盾（同样内容哈希）不重复追加
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		seen[ContentHash(it)] = true
	}
	block := func() string {
		var b strings.Builder
		b.WriteString(contradictionMarker + "\n")
		for _, it := range items {
			b.WriteString("- " + it + "\n")
		}
		return b.String()
	}
	if !strings.Contains(p.Content, contradictionMarker) {
		p.Content = strings.TrimSpace(p.Content) + "\n\n## 待裁决矛盾\n\n" + block()
	} else {
		// 已有标注块：合并未重复的条目后整体替换旧块
		parts := strings.SplitN(p.Content, contradictionMarker, 2)
		existingBlock := strings.TrimPrefix(parts[1], "\n")
		existingBlock = strings.TrimSuffix(existingBlock, "\n")
		merged := map[string]bool{}
		for _, line := range strings.Split(existingBlock, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") {
				merged[ContentHash(strings.TrimPrefix(line, "- "))] = true
			}
		}
		var extra []string
		for _, it := range items {
			if !merged[ContentHash(it)] {
				extra = append(extra, it)
			}
		}
		if len(extra) == 0 {
			return nil
		}
		var b strings.Builder
		b.WriteString(contradictionMarker + "\n")
		for _, line := range strings.Split(existingBlock, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "- ") {
				b.WriteString(line + "\n")
			}
		}
		for _, it := range extra {
			b.WriteString("- " + it + "\n")
		}
		p.Content = strings.TrimRight(parts[0], "\n") + "\n\n" + b.String()
	}
	p.Summary = firstSentence(p.Content, 120)
	if err := c.pageRepo.SaveDraft(spaceID, slug, p.Content, p.Summary, ContentHash(p.Content), []int(p.Sources), p.Confidence); err != nil {
		return err
	}
	for _, it := range items {
		_ = c.lintRepo.Create(&knowledgemodel.WikiLintIssue{
			SpaceID: spaceID, PageSlug: slug, Category: knowledgemodel.LintCategoryContradict,
			Severity: "warning", Detail: it, Status: "open",
		})
	}
	InvalidateIndex(spaceID)
	return nil
}

// ContradictionBlock 返回页面当前未裁决的矛盾条目（从正文标记块解析）。
func ContradictionBlock(content string) []string {
	i := strings.Index(content, contradictionMarker)
	if i < 0 {
		return nil
	}
	rest := content[i+len(contradictionMarker):]
	var out []string
	for _, line := range strings.Split(rest, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- ") {
			out = append(out, strings.TrimPrefix(line, "- "))
		} else if line != "" {
			break
		}
	}
	return out
}

// ProposeAnalysis 好答案回填：一次被采纳的问答沉淀为 analysis 型草稿页，
// 人审核后才进入检索层（知识复利的闭环入口）。
func (c *Compiler) ProposeAnalysis(ctx context.Context, spaceID int, title, question, answer string) (*knowledgemodel.WikiPage, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("AI 模型未配置，无法生成知识页")
	}
	slug := Slugify(title)
	if slug == "" {
		return nil, fmt.Errorf("标题为空")
	}
	prompt := fmt.Sprintf("问题：%s\n\n回答：%s\n\n"+
		"把上面的问答编译为一个结构化的知识页面。要求：\n"+
		"1. 只保留回答中确实成立的内容，去掉口语与冗余\n"+
		"2. 开头用一句话概括该知识，再分节展开\n"+
		"3. 正文里用 [[页面名]] 指向相关概念\n"+
		"4. 不要编造问答中没有的内容",
		question, truncate(answer, 6000))
	resp, err := c.provider.Chat(ctx,
		"你是知识库编纂助手，负责把优质问答沉淀为可复用的知识页面。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("generate analysis page: %w", err)
	}
	content := resp.Content
	summary := firstSentence(content, 120)
	if _, err := c.pageRepo.PlanPlaceholder(spaceID, slug,
		string(knowledgemodel.WikiPageTypeAnalysis), title, summary, 0); err != nil {
		return nil, err
	}
	if err := c.pageRepo.SaveDraft(spaceID, slug, content, summary, ContentHash(answer), []int{}, 0.80); err != nil {
		return nil, err
	}
	return c.pageRepo.FindBySlug(spaceID, slug)
}

// IndexEntry 目录条目。
type IndexEntry struct {
	Slug        string
	Type        string
	Title       string
	Summary     string
	SourceCount int
}

// 目录缓存：目录是每次查询都要发给模型的稳定前缀，重复构建既浪费一次全表查询，
// 也让 provider 侧的 prompt 前缀缓存无法命中（内容必须逐字节一致）。
// 缓存失效走 TTL + 页面变更显式失效双保险。
var indexCache = struct {
	mu sync.RWMutex
	m  map[int]indexCacheEntry
}{m: make(map[int]indexCacheEntry)}

type indexCacheEntry struct {
	builtAt time.Time
	text    string
	entries []IndexEntry
}

const indexCacheTTL = 5 * time.Minute

// InvalidateIndex 页面新增/发布/删除后显式失效目录缓存。
func InvalidateIndex(spaceID int) {
	indexCache.mu.Lock()
	delete(indexCache.m, spaceID)
	indexCache.mu.Unlock()
}

// BuildIndex 构建确定性目录（固定按 slug 排序）。
// 目录是查询阶段的主要导航入口，也是 Prompt Caching 的稳定前缀 —— 因此排序必须固定，
// 且不得包含时间戳等动态字段。
func (c *Compiler) BuildIndex(spaceID int) (string, []IndexEntry, error) {
	indexCache.mu.RLock()
	if e, ok := indexCache.m[spaceID]; ok && time.Since(e.builtAt) < indexCacheTTL {
		indexCache.mu.RUnlock()
		return e.text, e.entries, nil
	}
	indexCache.mu.RUnlock()

	pages, err := c.pageRepo.ListBySpace(spaceID, string(knowledgemodel.WikiPageStatusPublished))
	if err != nil {
		return "", nil, err
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].Slug < pages[j].Slug })

	entries := make([]IndexEntry, 0, len(pages))
	var b strings.Builder
	b.WriteString("# 知识目录\n\n")
	for _, p := range pages {
		entries = append(entries, IndexEntry{
			Slug:        p.Slug,
			Type:        p.Type,
			Title:       p.Title,
			Summary:     p.Summary,
			SourceCount: p.SourceCount,
		})
		fmt.Fprintf(&b, "- [%s](%s) `%s` · %d 源 · %s\n", p.Title, p.Slug, p.Type, p.SourceCount, p.Summary)
	}

	text := b.String()
	indexCache.mu.Lock()
	indexCache.m[spaceID] = indexCacheEntry{builtAt: time.Now(), text: text, entries: entries}
	indexCache.mu.Unlock()
	return text, entries, nil
}

// Slugify 生成 URL 安全的 slug，保留中日韩字符。
func Slugify(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

func firstSentence(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	runes := []rune(s)
	if len(runes) > n {
		runes = runes[:n]
	}
	return string(runes)
}

func commonPrefixLen(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	n := len(ra)
	if len(rb) < n {
		n = len(rb)
	}
	i := 0
	for i < n && ra[i] == rb[i] {
		i++
	}
	return i
}
