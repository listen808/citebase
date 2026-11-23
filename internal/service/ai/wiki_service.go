package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag/multiagent"
	wiki "github.com/listen808/citebase/internal/ai/wiki"
	logx "github.com/listen808/citebase/internal/log"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// WikiService 是 LLM Wiki 的业务层：编译、治理与查询都收敛在这里，
// api 层只做参数校验、权限收口与响应。此前这些逻辑直接写在 api/ai/wiki.go 里
// （936 行 handler 持有 5 个 repository），调度器与 MCP 各自 NewCompiler，
// 规则无法复用也无法单测。
//
// 空间权限不在这里判：本仓库的既定口径是"越权只能在 handler 层收口"
// （见 internal/api/workspace/access.go），service 签名里没有用户维度。
type WikiService struct {
	pageRepo   *knowledgerepo.WikiPageRepository
	linkRepo   *knowledgerepo.WikiLinkRepository
	pinRepo    *knowledgerepo.WikiPinRepository
	reviewRepo *knowledgerepo.WikiReviewRepository
	lintRepo   *knowledgerepo.WikiLintRepository
}

func NewWikiService() *WikiService {
	return &WikiService{
		pageRepo:   knowledgerepo.NewWikiPageRepository(),
		linkRepo:   knowledgerepo.NewWikiLinkRepository(),
		pinRepo:    knowledgerepo.NewWikiPinRepository(),
		reviewRepo: knowledgerepo.NewWikiReviewRepository(),
		lintRepo:   knowledgerepo.NewWikiLintRepository(),
	}
}

// 领域错误：api 层按这些哨兵值映射到稳定话术键（internal/response/messages.go）。
var (
	ErrWikiAIDisabled        = errors.New("wiki: ai service disabled")
	ErrWikiModelUnconfigured = errors.New("wiki: ai model unconfigured")
	ErrWikiPageNotFound      = errors.New("wiki: page not found")
	ErrWikiVersionNotFound   = errors.New("wiki: page version not found")
	ErrWikiNoContent         = errors.New("wiki: no compilable content")
)

// CompileOutcome 一次编译的结果摘要（响应体直接由它拼）。
type CompileOutcome struct {
	SpaceID int
	Mode    string
	Created []string
	Updated []string
	Skipped int
}

// ArticleResult 结构化长文生成的结果。
type ArticleResult struct {
	Topic          string
	Sections       []wiki.Section
	Markdown       string
	SourceCount    int
	CitationIssues []wiki.CitationIssue
}

// ---- 读 ----

func (s *WikiService) ListPages(spaceID int, status string) ([]*knowledgemodel.WikiPage, error) {
	return s.pageRepo.ListBySpace(spaceID, status)
}

func (s *WikiService) GetPage(spaceID int, slug string) (*knowledgemodel.WikiPage, error) {
	page, err := s.pageRepo.FindBySlug(spaceID, slug)
	if err != nil || page == nil {
		return nil, ErrWikiPageNotFound
	}
	return page, nil
}

func (s *WikiService) GetLinks(spaceID int) ([]*knowledgemodel.WikiLink, error) {
	return s.linkRepo.ListBySpace(spaceID)
}

func (s *WikiService) GetRelated(spaceID int, slug string, limit int) ([]wiki.Related, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).RelatedPages(spaceID, slug, limit)
}

func (s *WikiService) History(spaceID int, slug string) ([]*knowledgemodel.WikiPageHistory, error) {
	return wiki.NewCompiler(nil).History(spaceID, slug)
}

func (s *WikiService) GetIndex(spaceID int) (string, []wiki.IndexEntry, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).BuildIndex(spaceID)
}

func (s *WikiService) GetPurpose(spaceID int) string { return wiki.GetPurpose(spaceID) }

func (s *WikiService) SetPurpose(spaceID int, purpose string) (string, error) {
	if err := wiki.SetPurpose(spaceID, purpose); err != nil {
		return "", err
	}
	return wiki.GetPurpose(spaceID), nil
}

func (s *WikiService) GetSchema(spaceID int) string { return wiki.GetSchema(spaceID) }

func (s *WikiService) SetSchema(spaceID int, schema string) (string, error) {
	if err := wiki.SetSchema(spaceID, schema); err != nil {
		return "", err
	}
	return wiki.GetSchema(spaceID), nil
}

func (s *WikiService) Search(ctx context.Context, spaceID int, question string, fulltextTopK int) ([]wiki.Retrieved, error) {
	m := aimod.GetModule()
	return wiki.NewRetriever(providerOrNil(m)).Retrieve(ctx, spaceID, question, fulltextTopK)
}

func (s *WikiService) ListPins(spaceID int, status string) ([]*knowledgemodel.WikiPin, error) {
	return s.pinRepo.ListBySpace(spaceID, status)
}

func (s *WikiService) ListReviews(spaceID int, status string) ([]*knowledgemodel.WikiReview, error) {
	return s.reviewRepo.ListBySpace(spaceID, status)
}

func (s *WikiService) ListLint(spaceID int, status string) ([]*knowledgemodel.WikiLintIssue, error) {
	return s.lintRepo.ListBySpace(spaceID, status)
}

func (s *WikiService) Insights(spaceID int) ([]wiki.Insight, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).Insights(spaceID)
}

func (s *WikiService) CheckPins(ctx context.Context, spaceID int, slug string) ([]wiki.PinCheck, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).CheckPins(ctx, spaceID, slug)
}

// ExportPageMarkdown 单页导出（format=md）。
func (s *WikiService) ExportPageMarkdown(spaceID int, slug string) (string, error) {
	c := wiki.NewCompiler(nil)
	p, err := c.GetPageForRead(spaceID, slug)
	if err != nil || p == nil {
		return "", ErrWikiPageNotFound
	}
	return c.ExportPage(p), nil
}

// ExportSpace 整空间导出：每页一份 markdown + 交叉引用边（handler 负责打包成 zip）。
func (s *WikiService) ExportSpace(spaceID int) (map[string]string, []*knowledgemodel.WikiLink, error) {
	c := wiki.NewCompiler(nil)
	pages, err := c.ExportSpace(spaceID)
	if err != nil {
		return nil, nil, err
	}
	if len(pages) == 0 {
		return nil, nil, ErrWikiNoContent
	}
	links, _ := c.LinksForRead(spaceID)
	return pages, links, nil
}

// ---- 写 ----

// Compile 把指定文档编译为知识页面：Plan（串行去重）→ Generate（并发生成）。
// 产出的是 draft，需经 Publish 或人工审核后才对查询可见。
func (s *WikiService) Compile(ctx context.Context, spaceID int, docIDs []int, pageType string, concurrency int) (*CompileOutcome, error) {
	m := aimod.GetModule()
	if m == nil || !m.Enabled() {
		return nil, ErrWikiAIDisabled
	}
	p := providerOrNil(m)
	if p == nil {
		return nil, ErrWikiModelUnconfigured
	}

	docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs(docIDs)
	if err != nil {
		return nil, err
	}

	var candidates []wiki.Candidate
	if pageType == "" {
		for _, d := range docs {
			if d == nil || d.Content == "" {
				continue
			}
			candidates = append(candidates,
				wiki.AnalyzeDocumentToCandidates(ctx, p, d.DocID, d.DocName, d.Content)...)
		}
	} else {
		for _, d := range docs {
			if d == nil || d.Content == "" {
				continue
			}
			candidates = append(candidates, wiki.Candidate{
				Name:        d.DocName,
				PageType:    pageType,
				Title:       d.DocName,
				Summary:     d.BreadCrumb,
				SourceDocID: d.DocID,
				SourceText:  d.Content,
			})
		}
	}
	if len(candidates) == 0 {
		return nil, ErrWikiNoContent
	}

	compiler := wiki.NewCompiler(p)
	plan, err := compiler.Plan(ctx, spaceID, candidates)
	if err != nil {
		return nil, err
	}
	cost, err := compiler.Generate(ctx, spaceID, candidates, concurrency)
	if err != nil {
		return nil, err
	}

	// 元页面维护（best-effort，失败不影响编译结果返回）
	var logLines []string
	for _, slug := range plan.Created {
		logLines = append(logLines, "新建页面：`"+slug+"`")
	}
	for _, slug := range plan.Updated {
		logLines = append(logLines, "更新页面：`"+slug+"`（并入溯源）")
	}
	if cost != nil && cost.TotalTokens > 0 {
		logLines = append(logLines, fmt.Sprintf("Token 消耗：prompt %d / completion %d / 共 %d",
			cost.PromptTokens, cost.CompletionTokens, cost.TotalTokens))
	}
	compiler.AppendCompileLog(spaceID, logLines)
	if err := compiler.RefreshOverview(ctx, spaceID); err != nil {
		logx.Infof("[WikiService] Refresh overview failed for space %d: %v", spaceID, err)
	}

	// 置信度治理：低置信度页面自动进入审核队列
	for _, slug := range append(append([]string{}, plan.Created...), plan.Updated...) {
		compiler.CheckConfidence(ctx, spaceID, slug)
	}

	// P1 #3: 标记源文档为 raw 层不可变（后续内容变更触发 wiki 重编译）
	docRepo := workspacerepo.NewDocumentRepository()
	for _, d := range docs {
		if d != nil {
			_ = docRepo.MarkImmutable(d.DocID, true)
		}
	}

	return &CompileOutcome{
		SpaceID: spaceID,
		Mode:    twoStepModeLabel(pageType),
		Created: plan.Created,
		Updated: plan.Updated,
		Skipped: plan.Skipped,
	}, nil
}

// twoStepModeLabel P1：响应里标明本次编译走的模式，便于前端展示与排查。
func twoStepModeLabel(pageType string) string {
	if pageType == "" {
		return "two-step-cot"
	}
	return "single-type:" + pageType
}

// GenerateArticle 大纲 → 逐节写作 → 引用校验 → 润色（STORM 式结构化长文生成）。
// 引用校验发现问题时如实返回，不静默放过 —— 无来源的章节对用户没有价值。
func (s *WikiService) GenerateArticle(ctx context.Context, topic string, docIDs []int, skipPolish bool) (*ArticleResult, error) {
	m := aimod.GetModule()
	p := providerOrNil(m)
	if p == nil {
		return nil, ErrWikiModelUnconfigured
	}

	materials := ""
	sourceCount := 0
	if len(docIDs) > 0 {
		docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs(docIDs)
		if err == nil {
			for _, d := range docs {
				if d == nil || d.Content == "" {
					continue
				}
				sourceCount++
				materials += fmt.Sprintf("[%d] %s\n%s\n\n", sourceCount, d.DocName, d.Content)
			}
		}
	}

	compiler := wiki.NewCompiler(p)
	sections, err := compiler.PlanOutline(ctx, topic, materials)
	if err != nil {
		return nil, err
	}
	sections, err = compiler.WriteSections(ctx, topic, sections, materials)
	if err != nil {
		return nil, err
	}

	issues := wiki.VerifyCitations(sections, sourceCount)
	markdown := renderSections(sections)
	if !skipPolish {
		if polished, err := compiler.Polish(ctx, topic, sections); err == nil && polished != "" {
			markdown = polished
		}
	}

	return &ArticleResult{
		Topic:          topic,
		Sections:       sections,
		Markdown:       markdown,
		SourceCount:    sourceCount,
		CitationIssues: issues,
	}, nil
}

func renderSections(sections []wiki.Section) string {
	out := ""
	for _, s := range sections {
		out += "## " + s.Title + "\n\n" + s.Content + "\n\n"
	}
	return out
}

// Publish 发布草稿。基线版本不匹配时返回 knowledgerepo.ErrStaleDraft，调用方需重新生成草稿。
func (s *WikiService) Publish(spaceID int, slug string, baseVersion int) (*knowledgemodel.WikiPage, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).Publish(spaceID, slug, baseVersion)
}

// MarkHumanEdited 标记页面含人工确认的内容。
// provenance 切到 human 后，源文档下架触发的重编译不会自动删除该页知识。
func (s *WikiService) MarkHumanEdited(spaceID int, slug string) error {
	return s.pageRepo.MarkHumanEdited(spaceID, slug)
}

// DeletePage 删除页面（含死链清理）。源文档下架请用 RemoveSource 走级联规则。
func (s *WikiService) DeletePage(spaceID int, slug string) error {
	page, err := s.pageRepo.FindBySlug(spaceID, slug)
	if err != nil || page == nil {
		return ErrWikiPageNotFound
	}
	if err := s.pageRepo.Delete(page.ID); err != nil {
		return err
	}
	// 页面已经删掉了，边清不干净都不该把整个操作翻成失败 —— 但必须留痕：
	// 静默丢掉会留下一条指向不存在页面的边，而 Wiki 图与治理报告都把它当真边读。
	if err := s.linkRepo.DeleteBySlug(spaceID, slug); err != nil {
		logx.Warnf("[wiki] 页面 %s（空间 %d）已删除，但其关联边清理失败（会留下死边，可重跑治理）: %v", slug, spaceID, err)
	}
	wiki.InvalidateIndex(spaceID)
	return nil
}

// SupersedePage 手动取代：标记旧页面被新页面取代（LLM Wiki 取代链）。
// 与自动取代共用 SupersedePage 治理逻辑：人工页面不取代、已取代页面不重复取代。
func (s *WikiService) SupersedePage(spaceID int, oldSlug, newSlug string) error {
	if _, err := s.pageRepo.FindBySlug(spaceID, oldSlug); err != nil {
		return ErrWikiPageNotFound
	}
	if _, err := s.pageRepo.FindBySlug(spaceID, newSlug); err != nil {
		return ErrWikiPageNotFound
	}
	if err := wiki.NewCompiler(nil).SupersedePage(spaceID, oldSlug, newSlug); err != nil {
		return err
	}
	wiki.InvalidateIndex(spaceID)
	return nil
}

// RemoveSource 源文档下架的级联清理：独占页删除、共享页摘除溯源、死链清理。
func (s *WikiService) RemoveSource(spaceID, sourceDocID int) (*wiki.CascadeResult, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).RemoveSource(spaceID, sourceDocID)
}

// Rollback 回滚到指定历史版本。
func (s *WikiService) Rollback(spaceID int, slug string, version int) (*knowledgemodel.WikiPage, error) {
	m := aimod.GetModule()
	page, err := wiki.NewCompiler(providerOrNil(m)).Rollback(spaceID, slug, version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWikiVersionNotFound
		}
		return nil, err
	}
	return page, nil
}

// MarkContradictions 写入时矛盾标注：把冲突项写进页面正文并登记 Lint 待裁决。
func (s *WikiService) MarkContradictions(spaceID int, slug string, items []string) error {
	return wiki.NewCompiler(nil).MarkContradictions(spaceID, slug, items)
}

// ProposeAnalysis 好答案回填：把被采纳的问答沉淀为 analysis 草稿页。
func (s *WikiService) ProposeAnalysis(ctx context.Context, spaceID int, title, question, answer string) (*knowledgemodel.WikiPage, error) {
	m := aimod.GetModule()
	p := providerOrNil(m)
	if p == nil {
		return nil, ErrWikiModelUnconfigured
	}
	return wiki.NewCompiler(p).ProposeAnalysis(ctx, spaceID, title, question, answer)
}

// CreatePin 记录一条人工确认的事实锚点。
func (s *WikiService) CreatePin(spaceID int, pageSlug, kind, claim, anchor string, createdBy int) (*knowledgemodel.WikiPin, error) {
	if kind == "" {
		kind = "correction"
	}
	pin := &knowledgemodel.WikiPin{
		SpaceID:    spaceID,
		PageSlug:   pageSlug,
		Kind:       kind,
		Claim:      strings.TrimSpace(claim),
		Anchor:     anchor,
		Provenance: knowledgemodel.WikiProvenanceHuman,
		Status:     string(knowledgemodel.WikiPinStatusActive),
		CreatedBy:  createdBy,
	}
	if err := s.pinRepo.Create(pin); err != nil {
		return nil, err
	}
	// 页面一旦含人工确认的知识，provenance 切到 human，源下架不会自动删它。
	// 这一步失败必须撤回 pin 并报错：留着 pin 而 provenance 仍是 auto，
	// 用户看到"钉住成功"，而源一下架这个页面照样会被连带删掉 —— 钉住这件事根本没生效。
	if err := s.pageRepo.MarkHumanEdited(spaceID, pageSlug); err != nil {
		if delErr := s.pinRepo.Delete(pin.ID); delErr != nil {
			logx.Errorf("[wiki] pin %d 已建但标记人工编辑失败，且回滚 pin 也失败（需人工清理）: mark=%v delete=%v", pin.ID, err, delErr)
		}
		return nil, err
	}
	return pin, nil
}

// PinSpaceID 取 pin 所属空间（handler 在 resolve 前做权限校验用）。
func (s *WikiService) PinSpaceID(id int) (int, error) {
	pin, err := s.pinRepo.FindByID(id)
	if err != nil || pin == nil {
		return 0, ErrWikiPageNotFound
	}
	return pin.SpaceID, nil
}

// ReviewSpaceID 取审核项所属空间（handler 在 resolve 前做权限校验用）。
func (s *WikiService) ReviewSpaceID(id int) (int, error) {
	item, err := s.reviewRepo.FindByID(id)
	if err != nil || item == nil {
		return 0, ErrWikiPageNotFound
	}
	return item.SpaceID, nil
}

// ResolvePin 处理一条 pin（确认已解决 / 关闭）。
func (s *WikiService) ResolvePin(id int, status string) error {
	if status == "" {
		status = string(knowledgemodel.WikiPinStatusResolved)
	}
	return s.pinRepo.UpdateStatus(id, status)
}

// ResolveReview 处理审核项：批准时走乐观锁发布，基线不匹配则要求 rebase。
func (s *WikiService) ResolveReview(id int, action string, resolvedBy int) error {
	if action == "" {
		action = knowledgemodel.WikiReviewStatusAccepted
	}
	return s.reviewRepo.Resolve(id, action, resolvedBy)
}

// RunLint 执行一次体检（孤立页 / 稀疏社区 / 死链 / 过时内容），结果全量替换未处理项。
func (s *WikiService) RunLint(spaceID int, staleDays int) ([]*knowledgemodel.WikiLintIssue, error) {
	m := aimod.GetModule()
	return wiki.NewCompiler(providerOrNil(m)).Lint(spaceID, staleDays)
}

// RunDeepResearch 针对 Graph Insight 触发深度研究回灌（P3）：
// 生成研究查询 → 汇编已知材料 → 产出研究纪要 draft + 审核项。
func (s *WikiService) RunDeepResearch(ctx context.Context, spaceID int, ins wiki.Insight) (*wiki.DeepResearchReport, error) {
	m := aimod.GetModule()
	compiler := wiki.NewCompiler(providerOrNil(m))
	// W5：多智能体深度研究开关开启时，注入编排器走 researcher→writer→critic 闭环
	if m != nil && wiki.MultiagentEnabled() {
		if orch := m.NewOrchestrator(); orch != nil {
			compiler.SetMultiagentOrchestrator(&wikiMultiagentAdapter{orch: orch})
		}
	}
	return compiler.DeepResearch(ctx, spaceID, ins)
}

// providerOrNil 取主模型：模块未装配时返回 nil，只读路径允许无模型运行。
func providerOrNil(m *aimod.Module) provider.Provider {
	if m == nil {
		return nil
	}
	return m.PrimaryProvider()
}

// wikiMultiagentAdapter 将 multiagent.Orchestrator 适配为 wiki.MultiagentRunner 接口，
// 避免 wiki 包直接导入 multiagent 包（导入循环）。
type wikiMultiagentAdapter struct {
	orch *multiagent.Orchestrator
}

func (a *wikiMultiagentAdapter) Run(ctx context.Context, question string) (string, error) {
	return a.orch.Run(ctx, question, nil)
}
