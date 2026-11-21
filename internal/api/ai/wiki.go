package ai

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/ai/rag/multiagent"
	"github.com/listen808/citebase/internal/ai/wiki"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
)

// WikiAPI 知识编译（LLM Wiki）相关接口。
type WikiAPI struct {
	pageRepo   *knowledgerepo.WikiPageRepository
	linkRepo   *knowledgerepo.WikiLinkRepository
	pinRepo    *knowledgerepo.WikiPinRepository
	reviewRepo *knowledgerepo.WikiReviewRepository
	lintRepo   *knowledgerepo.WikiLintRepository
}

func NewWikiAPI() *WikiAPI {
	return &WikiAPI{
		pageRepo:   knowledgerepo.NewWikiPageRepository(),
		linkRepo:   knowledgerepo.NewWikiLinkRepository(),
		pinRepo:    knowledgerepo.NewWikiPinRepository(),
		reviewRepo: knowledgerepo.NewWikiReviewRepository(),
		lintRepo:   knowledgerepo.NewWikiLintRepository(),
	}
}

// Compile 把指定文档编译为知识页面：Plan（串行去重）→ Generate（并发生成）。
// 产出的是 draft，需经 Publish 或人工审核后才对查询可见。
func (api *WikiAPI) Compile(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	p := m.PrimaryProvider()
	if p == nil {
		response.ServerErrorKey(c, response.MsgAIModelUnconfigured)
		return
	}
	if _, ok := middleware.RequireUser(c); !ok {
		return
	}

	var req struct {
		SpaceID     int    `json:"space_id"`
		DocIDs      []int  `json:"doc_ids"`
		PageType    string `json:"page_type"`
		Concurrency int    `json:"concurrency"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	if req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if len(req.DocIDs) == 0 {
		response.BadRequestKey(c, response.MsgDocIDsRequired)
		return
	}

	docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs(req.DocIDs)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if len(docs) == 0 {
		response.BadRequestKey(c, response.MsgWikiNoCompilableContent)
		return
	}

	compiler := wiki.NewCompiler(p)
	ctx := c.Request.Context()

	// P1 两步摄入：page_type 为空 = Two-Step CoT 模式（默认）——
	// Step1 对每篇文档做实体/概念分析，映射成 source/concept/entity 候选；
	// 显式传 page_type 则保持旧的单型编译行为。
	var candidates []wiki.Candidate
	if req.PageType == "" {
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
				PageType:    req.PageType,
				Title:       d.DocName,
				Summary:     d.BreadCrumb,
				SourceDocID: d.DocID,
				SourceText:  d.Content,
			})
		}
	}
	if len(candidates) == 0 {
		response.BadRequestKey(c, response.MsgWikiNoCompilableContent)
		return
	}

	plan, err := compiler.Plan(ctx, req.SpaceID, candidates)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	cost, err := compiler.Generate(ctx, req.SpaceID, candidates, req.Concurrency)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
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
	compiler.AppendCompileLog(req.SpaceID, logLines)
	if err := compiler.RefreshOverview(ctx, req.SpaceID); err != nil {
		logx.Infof("[WikiAPI] Refresh overview failed for space %d: %v", req.SpaceID, err)
	}

	// 置信度治理：低置信度页面自动进入审核队列
	for _, slug := range append(append([]string{}, plan.Created...), plan.Updated...) {
		compiler.CheckConfidence(ctx, req.SpaceID, slug)
	}

	// P1 #3: 标记源文档为 raw 层不可变（后续内容变更触发 wiki 重编译）
	docRepo := workspacerepo.NewDocumentRepository()
	for _, d := range docs {
		if d != nil {
			_ = docRepo.MarkImmutable(d.DocID, true)
		}
	}

	response.Success(c, gin.H{
		"space_id": req.SpaceID,
		"mode":     twoStepModeLabel(req.PageType),
		"created":  plan.Created,
		"updated":  plan.Updated,
		"skipped":  plan.Skipped,
	})
}

// twoStepModeLabel P1：响应里标明本次编译走的模式，便于前端展示与排查。
func twoStepModeLabel(pageType string) string {
	if pageType == "" {
		return "two-step-cot"
	}
	return "single-type:" + pageType
}

// ListPages 列出空间下的知识页面。status 为空则返回全部。
func (api *WikiAPI) ListPages(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	pages, err := api.pageRepo.ListBySpace(spaceID, c.Query("status"))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"pages": pages, "total": len(pages)})
}

// GetPage 读取单个页面（含正文）。
func (api *WikiAPI) GetPage(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	page, err := api.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		response.NotFoundKey(c, response.MsgPageNotFound)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// Publish 发布草稿。基线版本不匹配时返回冲突，调用方需重新生成草稿（rebase）。
func (api *WikiAPI) Publish(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	var req struct {
		BaseVersion int `json:"base_version"`
	}
	_ = c.ShouldBindJSON(&req)

	m := aimod.GetModule()
	compiler := wiki.NewCompiler(m.PrimaryProvider())
	page, err := compiler.Publish(spaceID, slug, req.BaseVersion)
	if err != nil {
		if err == knowledgerepo.ErrStaleDraft {
			response.BadRequestKey(c, response.MsgPageUpdated)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// MarkHumanEdited 标记页面含人工确认的内容。
// provenance 切到 human 后，源文档下架触发的重编译不会自动删除该页知识。
func (api *WikiAPI) MarkHumanEdited(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	if err := api.pageRepo.MarkHumanEdited(spaceID, slug); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"slug": slug, "provenance": knowledgemodel.WikiProvenanceHuman})
}

// Search 目录 ∪ 全文 双路召回：先读 Wiki 编译产物，未命中时由调用方回退到 chunk 检索。
func (api *WikiAPI) Search(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	var req struct {
		Question     string `json:"question"`
		FulltextTopK int    `json:"fulltext_top_k"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Question) == "" {
		response.BadRequestKey(c, response.MsgQuestionRequired)
		return
	}

	m := aimod.GetModule()
	r := wiki.NewRetriever(m.PrimaryProvider())
	hits, err := r.Retrieve(c.Request.Context(), spaceID, req.Question, req.FulltextTopK)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"hits": hits, "total": len(hits)})
}

// GenerateArticle 大纲 → 逐节写作 → 引用校验 → 润色（STORM 式结构化长文生成）。
// 引用校验发现问题时如实返回，不静默放过 —— 无来源的章节对用户没有价值。
func (api *WikiAPI) GenerateArticle(c *gin.Context) {
	m := aimod.GetModule()
	p := m.PrimaryProvider()
	if p == nil {
		response.ServerErrorKey(c, response.MsgAIModelUnconfigured)
		return
	}
	if _, ok := middleware.RequireUser(c); !ok {
		return
	}

	var req struct {
		Topic      string `json:"topic"`
		DocIDs     []int  `json:"doc_ids"`
		SkipPolish bool   `json:"skip_polish"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Topic) == "" {
		response.BadRequestKey(c, response.MsgTopicRequired)
		return
	}

	materials := ""
	sourceCount := 0
	if len(req.DocIDs) > 0 {
		docs, err := analyzer.NewDocReader(0).ReadDocumentsByIDs(req.DocIDs)
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
	ctx := c.Request.Context()

	sections, err := compiler.PlanOutline(ctx, req.Topic, materials)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	sections, err = compiler.WriteSections(ctx, req.Topic, sections, materials)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	issues := wiki.VerifyCitations(sections, sourceCount)

	markdown := renderSections(sections)
	if !req.SkipPolish {
		if polished, err := compiler.Polish(ctx, req.Topic, sections); err == nil && polished != "" {
			markdown = polished
		}
	}

	response.Success(c, gin.H{
		"topic":           req.Topic,
		"sections":        sections,
		"markdown":        markdown,
		"source_count":    sourceCount,
		"citation_issues": issues,
	})
}

func renderSections(sections []wiki.Section) string {
	out := ""
	for _, s := range sections {
		out += "## " + s.Title + "\n\n" + s.Content + "\n\n"
	}
	return out
}

// GetPurpose 读取空间研究目标（P2 Purpose 层，对齐 LLM Wiki 的 purpose.md）。
func (api *WikiAPI) GetPurpose(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	response.Success(c, gin.H{
		"space_id": spaceID,
		"purpose":  wiki.GetPurpose(spaceID),
	})
}

// UpdatePurpose 写入空间研究目标。空串 = 清除。立即生效于后续编译与检索选页。
func (api *WikiAPI) UpdatePurpose(c *gin.Context) {
	var req struct {
		SpaceID int    `json:"space_id"`
		Purpose string `json:"purpose"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if err := wiki.SetPurpose(req.SpaceID, req.Purpose); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{
		"space_id": req.SpaceID,
		"purpose":  wiki.GetPurpose(req.SpaceID),
	})
}

// GetSchema 读取空间行为约束（P2 SCHEMA 层，对齐 LLM Wiki 的 SCHEMA.md）。
func (api *WikiAPI) GetSchema(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	response.Success(c, gin.H{
		"space_id": spaceID,
		"schema":   wiki.GetSchema(spaceID),
	})
}

// UpdateSchema 写入空间行为约束。空串 = 清除。立即生效于后续编译的 system prompt。
func (api *WikiAPI) UpdateSchema(c *gin.Context) {
	var req struct {
		SpaceID int    `json:"space_id"`
		Schema  string `json:"schema"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if err := wiki.SetSchema(req.SpaceID, req.Schema); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{
		"space_id": req.SpaceID,
		"schema":   wiki.GetSchema(req.SpaceID),
	})
}

// ListHistory 页面版本历史（降序）。
// @Summary Wiki 页面版本历史
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/history [get]
func (api *WikiAPI) ListHistory(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	compiler := wiki.NewCompiler(nil)
	hist, err := compiler.History(spaceID, slug)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"slug": slug, "history": hist, "total": len(hist)})
}

// RollbackPage 回滚到指定历史版本。
// @Summary Wiki 页面回滚到历史版本
// @Tags AI
// @Accept json
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Param body body map[string]int true "version"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/rollback [post]
func (api *WikiAPI) RollbackPage(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Version <= 0 {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	compiler := wiki.NewCompiler(aimod.GetModule().PrimaryProvider())
	page, err := compiler.Rollback(spaceID, slug, req.Version)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.BadRequestKey(c, response.MsgPageVersionNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// MarkContradictions 写入时矛盾标注：把冲突项写进页面正文并登记 Lint 待裁决。
// @Summary Wiki 页面写入时矛盾标注
// @Tags AI
// @Accept json
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Param body body map[string]interface{} true "items: []string"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/contradictions [post]
func (api *WikiAPI) MarkContradictions(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	var req struct {
		Items []string `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	if err := wiki.NewCompiler(nil).MarkContradictions(spaceID, slug, req.Items); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"slug": slug, "marked": len(req.Items)})
}

// ProposeAnalysis 好答案回填：把被采纳的问答沉淀为 analysis 草稿页。
// @Summary Wiki 好答案回填（问答 → analysis 草稿页）
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/title/question/answer"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/propose-analysis [post]
func (api *WikiAPI) ProposeAnalysis(c *gin.Context) {
	m := aimod.GetModule()
	p := m.PrimaryProvider()
	if p == nil {
		response.ServerErrorKey(c, response.MsgAIModelUnconfigured)
		return
	}
	var req struct {
		SpaceID  int    `json:"space_id"`
		Title    string `json:"title"`
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 ||
		strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Answer) == "" {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	page, err := wiki.NewCompiler(p).ProposeAnalysis(c.Request.Context(), req.SpaceID, req.Title, req.Question, req.Answer)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// ExportWiki 把空间内所有已发布 wiki 页导出为标准 markdown（OKF 风格）。
// @Summary Wiki 知识页导出（OKF/Markdown，zip 或单页）
// @Tags AI
// @Produce application/zip
// @Param space_id query int true "空间ID"
// @Param format query string false "zip（默认，打包全部）或 md（单页）"
// @Param slug query string false "format=md 时的页面 slug"
// @Success 200 {file} binary
// @Router /api/v1/ai/wiki/export [get]
// format=zip（默认）：打包全部页为 .zip，每页一个 .md 文件 + 一份 links.md 交叉引用表。
// format=md&slug=X：导出单页 markdown 文件。
// 导入侧复用现有文档 markdown 通道（/workspace 的 ImportDocuments），不另造格式。
func (api *WikiAPI) ExportWiki(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if _, ok := middleware.RequireUser(c); !ok {
		return
	}
	compiler := wiki.NewCompiler(nil)

	format := c.DefaultQuery("format", "zip")
	if format == "md" {
		slug := c.Query("slug")
		if slug == "" {
			response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
			return
		}
		p, err := compiler.GetPageForRead(spaceID, slug)
		if err != nil {
			response.NotFoundKey(c, response.MsgPageNotFound)
			return
		}
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(slug+".md")))
		c.String(200, compiler.ExportPage(p))
		return
	}

	pages, err := compiler.ExportSpace(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if len(pages) == 0 {
		response.BadRequestKey(c, response.MsgWikiNoCompilableContent)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(pages[name]))
	}
	// links.md：交叉引用边导出为 markdown 表格，供其他 llm-wiki 工具重建图
	if links, err := compiler.LinksForRead(spaceID); err == nil && len(links) > 0 {
		var lb strings.Builder
		lb.WriteString("# 交叉引用\n\n| from | to | type | weight |\n|---|---|---|---|\n")
		for _, l := range links {
			fmt.Fprintf(&lb, "| %s | %s | %s | %g |\n", l.FromSlug, l.ToSlug, l.LinkType, l.Weight)
		}
		if w, err := zw.Create("links.md"); err == nil {
			_, _ = w.Write([]byte(lb.String()))
		}
	}
	if err := zw.Close(); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fmt.Sprintf("wiki-space-%d.zip", spaceID))))
	c.Data(200, "application/zip", buf.Bytes())
}

// CheckPins 重新编译后复核人工 pin：满足放行 / 冲突进人工裁决 / 锚点失效标记 orphan。
func (api *WikiAPI) CheckPins(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	m := aimod.GetModule()
	checks, err := wiki.NewCompiler(m.PrimaryProvider()).CheckPins(c.Request.Context(), spaceID, slug)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"checks": checks, "total": len(checks)})
}

// CreatePin 记录一条人工确认的事实锚点。
func (api *WikiAPI) CreatePin(c *gin.Context) {
	var req struct {
		SpaceID  int    `json:"space_id"`
		PageSlug string `json:"page_slug"`
		Kind     string `json:"kind"`
		Claim    string `json:"claim"`
		Anchor   string `json:"anchor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 || req.PageSlug == "" || strings.TrimSpace(req.Claim) == "" {
		response.BadRequestKey(c, response.MsgSpacePageClaimRequired)
		return
	}
	userID, _ := middleware.GetUserID(c)
	if req.Kind == "" {
		req.Kind = "correction"
	}
	pin := &knowledgemodel.WikiPin{
		SpaceID:    req.SpaceID,
		PageSlug:   req.PageSlug,
		Kind:       req.Kind,
		Claim:      strings.TrimSpace(req.Claim),
		Anchor:     req.Anchor,
		Provenance: knowledgemodel.WikiProvenanceHuman,
		Status:     string(knowledgemodel.WikiPinStatusActive),
		CreatedBy:  userID,
	}
	if err := api.pinRepo.Create(pin); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	// 页面一旦含人工确认的知识，provenance 切到 human，源下架不会自动删它。
	// 这一步失败必须撤回 pin 并报错：留着 pin 而 provenance 仍是 auto，
	// 用户看到"钉住成功"，而源一下架这个页面照样会被连带删掉 —— 钉住这件事根本没生效。
	if err := api.pageRepo.MarkHumanEdited(req.SpaceID, req.PageSlug); err != nil {
		if delErr := api.pinRepo.Delete(pin.ID); delErr != nil {
			logx.Errorf("[wiki] pin %d 已建但标记人工编辑失败，且回滚 pin 也失败（需人工清理）: mark=%v delete=%v", pin.ID, err, delErr)
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"pin": pin})
}

// ListPins 列出空间的 pin。
func (api *WikiAPI) ListPins(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	pins, err := api.pinRepo.ListBySpace(spaceID, c.Query("status"))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"pins": pins, "total": len(pins)})
}

// ResolvePin 处理一条 pin（确认已解决 / 关闭）。
func (api *WikiAPI) ResolvePin(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}
	status := c.Query("status")
	if status == "" {
		status = string(knowledgemodel.WikiPinStatusResolved)
	}
	if err := api.pinRepo.UpdateStatus(id, status); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": status})
}

// ListReviews 异步审核队列。
func (api *WikiAPI) ListReviews(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	items, err := api.reviewRepo.ListBySpace(spaceID, c.Query("status"))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"reviews": items, "total": len(items)})
}

// ResolveReview 处理审核项：批准时走乐观锁发布，基线不匹配则要求 rebase。
func (api *WikiAPI) ResolveReview(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.Action == "" {
		req.Action = knowledgemodel.WikiReviewStatusAccepted
	}
	userID, _ := middleware.GetUserID(c)
	if err := api.reviewRepo.Resolve(id, req.Action, userID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "status": req.Action})
}

// RunLint 执行一次体检（孤立页 / 稀疏社区 / 死链 / 过时内容），结果全量替换未处理项。
func (api *WikiAPI) RunLint(c *gin.Context) {
	var req struct {
		SpaceID   int `json:"space_id"`
		StaleDays int `json:"stale_days"`
	}
	_ = c.ShouldBindJSON(&req)
	if req.SpaceID <= 0 {
		req.SpaceID, _ = strconv.Atoi(c.Query("space_id"))
	}
	if req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	m := aimod.GetModule()
	issues, err := wiki.NewCompiler(m.PrimaryProvider()).Lint(req.SpaceID, req.StaleDays)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"issues": issues, "total": len(issues)})
}

// ListLint 列出体检问题。
func (api *WikiAPI) ListLint(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	items, err := api.lintRepo.ListBySpace(spaceID, c.DefaultQuery("status", "open"))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"issues": items, "total": len(items)})
}

// GetInsights 结构性洞察：意外连接 / 知识缺口 / 桥节点，每类带一个可执行动作。
func (api *WikiAPI) GetInsights(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	m := aimod.GetModule()
	out, err := wiki.NewCompiler(m.PrimaryProvider()).Insights(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"insights": out, "total": len(out)})
}

// RunDeepResearch 针对 Graph Insight 触发深度研究回灌（P3）：
// 生成研究查询 → 汇编已知材料 → 产出研究纪要 draft + 审核项。
func (api *WikiAPI) RunDeepResearch(c *gin.Context) {
	var req struct {
		SpaceID int      `json:"space_id"`
		Kind    string   `json:"kind"`
		Slugs   []string `json:"slugs"`
		Detail  string   `json:"detail"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 || len(req.Slugs) == 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	m := aimod.GetModule()
	if _, ok := middleware.RequireUser(c); !ok {
		return
	}
	compiler := wiki.NewCompiler(m.PrimaryProvider())
	// W5：多智能体深度研究开关开启时，注入编排器走 researcher→writer→critic 闭环
	if wiki.MultiagentEnabled() {
		if orch := m.NewOrchestrator(); orch != nil {
			compiler.SetMultiagentOrchestrator(&wikiMultiagentAdapter{orch: orch})
		}
	}
	report, err := compiler.DeepResearch(c.Request.Context(),
		req.SpaceID, wiki.Insight{Kind: req.Kind, Slugs: req.Slugs, Detail: req.Detail})
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"report": report})
}

// GetRelated 4-Signal 相关性排序的相关页面。
func (api *WikiAPI) GetRelated(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	m := aimod.GetModule()
	out, err := wiki.NewCompiler(m.PrimaryProvider()).RelatedPages(spaceID, slug, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"related": out, "total": len(out)})
}

// DeletePage 删除页面（含死链清理）。源文档下架请用 RemoveSource 走级联规则。
func (api *WikiAPI) DeletePage(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	page, err := api.pageRepo.FindBySlug(spaceID, slug)
	if err != nil {
		response.NotFoundKey(c, response.MsgPageNotFound)
		return
	}
	if err := api.pageRepo.Delete(page.ID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	// 页面已经删掉了，边清不干净都不该把整个操作翻成失败 —— 但必须留痕：
	// 静默丢掉会留下一条指向不存在页面的边，而 Wiki 图与治理报告都把它当真边读。
	if err := api.linkRepo.DeleteBySlug(spaceID, slug); err != nil {
		logx.Warnf("[wiki] 页面 %s（空间 %d）已删除，但其关联边清理失败（会留下死边，可重跑治理）: %v", slug, spaceID, err)
	}
	wiki.InvalidateIndex(spaceID)
	response.Success(c, gin.H{"slug": slug})
}

// SupersedePage 手动取代：标记旧页面被新页面取代（LLM Wiki 取代链）。
// 与自动取代共用 SupersedePage 治理逻辑：人工页面不取代、已取代页面不重复取代。
func (api *WikiAPI) SupersedePage(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	oldSlug := c.Param("slug")
	if spaceID <= 0 || oldSlug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return
	}
	var req struct {
		SupersededBy string `json:"superseded_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SupersededBy) == "" {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	// 旧页面必须存在
	if _, err := api.pageRepo.FindBySlug(spaceID, oldSlug); err != nil {
		response.NotFoundKey(c, response.MsgPageNotFound)
		return
	}
	// 新页面也必须存在
	if _, err := api.pageRepo.FindBySlug(spaceID, req.SupersededBy); err != nil {
		response.NotFoundKey(c, response.MsgPageNotFound)
		return
	}

	compiler := wiki.NewCompiler(nil)
	if err := compiler.SupersedePage(spaceID, oldSlug, req.SupersededBy); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	wiki.InvalidateIndex(spaceID)
	response.Success(c, gin.H{"slug": oldSlug, "superseded_by": req.SupersededBy})
}

// RemoveSource 源文档下架的级联清理：独占页删除、共享页摘除溯源、死链清理。
func (api *WikiAPI) RemoveSource(c *gin.Context) {
	var req struct {
		SpaceID     int `json:"space_id"`
		SourceDocID int `json:"source_doc_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 || req.SourceDocID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDAndSourceDocRequired)
		return
	}
	m := aimod.GetModule()
	res, err := wiki.NewCompiler(m.PrimaryProvider()).RemoveSource(req.SpaceID, req.SourceDocID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"result": res})
}

// GetIndex 返回确定性构建的知识目录（查询阶段的导航入口 / Prompt Caching 前缀）。
func (api *WikiAPI) GetIndex(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	m := aimod.GetModule()
	compiler := wiki.NewCompiler(m.PrimaryProvider())
	text, entries, err := compiler.BuildIndex(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"markdown": text, "entries": entries, "total": len(entries)})
}

// GetLinks 返回空间的交叉引用边，用于图谱可视化。
func (api *WikiAPI) GetLinks(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	links, err := api.linkRepo.ListBySpace(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"links": links, "total": len(links)})
}

// wikiMultiagentAdapter 将 multiagent.Orchestrator 适配为 wiki.MultiagentRunner 接口，
// 避免 wiki 包直接导入 multiagent 包（导入循环）。
type wikiMultiagentAdapter struct {
	orch *multiagent.Orchestrator
}

func (a *wikiMultiagentAdapter) Run(ctx context.Context, question string) (string, error) {
	return a.orch.Run(ctx, question, nil)
}
