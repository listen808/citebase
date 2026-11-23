package ai

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	wiki "github.com/listen808/citebase/internal/ai/wiki"
	"github.com/listen808/citebase/internal/middleware"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/response"
	aisvc "github.com/listen808/citebase/internal/service/ai"
)

// WikiAPI 知识编译（LLM Wiki）相关接口。
//
// 本层只做三件事：参数校验、空间权限收口、响应封套。编译/治理/查询的业务
// 逻辑在 internal/service/ai/wiki_service.go（api 不再直接持有 repository）。
//
// 权限口径与 internal/api/workspace/access.go 一致：读走 CheckSpaceAccess（公开空间放行），
// 写要求 space 成员（privilege>=1）；超级管理员在 GetUserSpacePrivilege 里直接是 2。
type WikiAPI struct {
	svc *aisvc.WikiService
}

func NewWikiAPI() *WikiAPI {
	return &WikiAPI{svc: aisvc.NewWikiService()}
}

// wikiFail 把业务层的哨兵错误映射到稳定话术键；未知错误按服务端错误兜底。
func wikiFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, aisvc.ErrWikiAIDisabled):
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
	case errors.Is(err, aisvc.ErrWikiModelUnconfigured):
		response.ServerErrorKey(c, response.MsgAIModelUnconfigured)
	case errors.Is(err, aisvc.ErrWikiNoContent):
		response.BadRequestKey(c, response.MsgWikiNoCompilableContent)
	case errors.Is(err, aisvc.ErrWikiPageNotFound):
		response.NotFoundKey(c, response.MsgPageNotFound)
	case errors.Is(err, aisvc.ErrWikiVersionNotFound):
		response.BadRequestKey(c, response.MsgPageVersionNotFound)
	case errors.Is(err, knowledgerepo.ErrStaleDraft):
		response.BadRequestKey(c, response.MsgPageUpdated)
	default:
		response.ServerErrorSafe(c, err)
	}
}

// Compile 把指定文档编译为知识页面（Plan 串行去重 → Generate 并发生成）。
// @Summary Wiki 知识编译
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/doc_ids/page_type/concurrency"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/compile [post]
func (api *WikiAPI) Compile(c *gin.Context) {
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
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	out, err := api.svc.Compile(c.Request.Context(), req.SpaceID, req.DocIDs, req.PageType, req.Concurrency)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{
		"space_id": out.SpaceID,
		"mode":     out.Mode,
		"created":  out.Created,
		"updated":  out.Updated,
		"skipped":  out.Skipped,
	})
}

// ListPages 列出空间下的知识页面。status 为空则返回全部。
// @Summary Wiki 页面列表
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param status query string false "页面状态"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages [get]
func (api *WikiAPI) ListPages(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	pages, err := api.svc.ListPages(spaceID, c.Query("status"))
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"pages": pages, "total": len(pages)})
}

// GetPage 读取单个页面（含正文）。
// @Summary Wiki 页面详情
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug} [get]
func (api *WikiAPI) GetPage(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	page, err := api.svc.GetPage(spaceID, slug)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// Publish 发布草稿。基线版本不匹配时返回冲突，调用方需重新生成草稿（rebase）。
// @Summary Wiki 页面发布
// @Tags AI
// @Accept json
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Param body body map[string]int true "base_version"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/publish [post]
func (api *WikiAPI) Publish(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	var req struct {
		BaseVersion int `json:"base_version"`
	}
	_ = c.ShouldBindJSON(&req)
	page, err := api.svc.Publish(spaceID, slug, req.BaseVersion)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"page": page})
}

// MarkHumanEdited 标记页面含人工确认的内容。
// provenance 切到 human 后，源文档下架触发的重编译不会自动删除该页知识。
// @Summary Wiki 页面标记人工编辑
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/human-edit [post]
func (api *WikiAPI) MarkHumanEdited(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	if err := api.svc.MarkHumanEdited(spaceID, slug); err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"slug": slug, "provenance": "human"})
}

// Search 目录 ∪ 全文 双路召回：先读 Wiki 编译产物，未命中时由调用方回退到 chunk 检索。
// @Summary Wiki 知识检索
// @Tags AI
// @Accept json
// @Produce json
// @Param space_id query int true "空间ID"
// @Param body body map[string]interface{} true "question/fulltext_top_k"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/search [post]
func (api *WikiAPI) Search(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
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
	hits, err := api.svc.Search(c.Request.Context(), spaceID, req.Question, req.FulltextTopK)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"hits": hits, "total": len(hits)})
}

// GenerateArticle 大纲 → 逐节写作 → 引用校验 → 润色（STORM 式结构化长文生成）。
// 引用校验发现问题时如实返回，不静默放过 —— 无来源的章节对用户没有价值。
// @Summary Wiki 结构化长文生成
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "topic/doc_ids/skip_polish"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/article/generate [post]
func (api *WikiAPI) GenerateArticle(c *gin.Context) {
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
	res, err := api.svc.GenerateArticle(c.Request.Context(), req.Topic, req.DocIDs, req.SkipPolish)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{
		"topic":           res.Topic,
		"sections":        res.Sections,
		"markdown":        res.Markdown,
		"source_count":    res.SourceCount,
		"citation_issues": res.CitationIssues,
	})
}

// GetPurpose 读取空间研究目标（P2 Purpose 层，对齐 LLM Wiki 的 purpose.md）。
// @Summary Wiki 研究目标读取
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/purpose [get]
func (api *WikiAPI) GetPurpose(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	response.Success(c, gin.H{"space_id": spaceID, "purpose": api.svc.GetPurpose(spaceID)})
}

// UpdatePurpose 写入空间研究目标。空串 = 清除。立即生效于后续编译与检索选页。
// @Summary Wiki 研究目标写入
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/purpose"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/purpose [put]
func (api *WikiAPI) UpdatePurpose(c *gin.Context) {
	var req struct {
		SpaceID int    `json:"space_id"`
		Purpose string `json:"purpose"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	purpose, err := api.svc.SetPurpose(req.SpaceID, req.Purpose)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"space_id": req.SpaceID, "purpose": purpose})
}

// GetSchema 读取空间行为约束（P2 SCHEMA 层，对齐 LLM Wiki 的 SCHEMA.md）。
// @Summary Wiki 行为约束读取
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/schema [get]
func (api *WikiAPI) GetSchema(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	response.Success(c, gin.H{"space_id": spaceID, "schema": api.svc.GetSchema(spaceID)})
}

// UpdateSchema 写入空间行为约束。空串 = 清除。立即生效于后续编译的 system prompt。
// @Summary Wiki 行为约束写入
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/schema"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/schema [put]
func (api *WikiAPI) UpdateSchema(c *gin.Context) {
	var req struct {
		SpaceID int    `json:"space_id"`
		Schema  string `json:"schema"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return
	}
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	schema, err := api.svc.SetSchema(req.SpaceID, req.Schema)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"space_id": req.SpaceID, "schema": schema})
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
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	hist, err := api.svc.History(spaceID, slug)
	if err != nil {
		wikiFail(c, err)
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
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	var req struct {
		Version int `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Version <= 0 {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	page, err := api.svc.Rollback(spaceID, slug, req.Version)
	if err != nil {
		wikiFail(c, err)
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
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	var req struct {
		Items []string `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Items) == 0 {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	if err := api.svc.MarkContradictions(spaceID, slug, req.Items); err != nil {
		wikiFail(c, err)
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
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	page, err := api.svc.ProposeAnalysis(c.Request.Context(), req.SpaceID, req.Title, req.Question, req.Answer)
	if err != nil {
		wikiFail(c, err)
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
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}

	if c.DefaultQuery("format", "zip") == "md" {
		slug := c.Query("slug")
		if slug == "" {
			response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
			return
		}
		markdown, err := api.svc.ExportPageMarkdown(spaceID, slug)
		if err != nil {
			wikiFail(c, err)
			return
		}
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(slug+".md")))
		c.String(200, markdown)
		return
	}

	pages, links, err := api.svc.ExportSpace(spaceID)
	if err != nil {
		wikiFail(c, err)
		return
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedPageNames(pages) {
		w, err := zw.Create(name)
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(pages[name]))
	}
	// links.md：交叉引用边导出为 markdown 表格，供其他 llm-wiki 工具重建图
	if len(links) > 0 {
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

// sortedPageNames 按稳定顺序写 zip（避免两次导出文件顺序不同）。
func sortedPageNames(pages map[string]string) []string {
	names := make([]string, 0, len(pages))
	for name := range pages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// CheckPins 重新编译后复核人工 pin：满足放行 / 冲突进人工裁决 / 锚点失效标记 orphan。
// @Summary Wiki 人工锚点复核
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/check-pins [post]
func (api *WikiAPI) CheckPins(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	checks, err := api.svc.CheckPins(c.Request.Context(), spaceID, slug)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"checks": checks, "total": len(checks)})
}

// CreatePin 记录一条人工确认的事实锚点。
// @Summary Wiki 人工锚点创建
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/page_slug/kind/claim/anchor"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pins [post]
func (api *WikiAPI) CreatePin(c *gin.Context) {
	var req struct {
		SpaceID  int    `json:"space_id"`
		PageSlug string `json:"page_slug"`
		Kind     string `json:"kind"`
		Claim    string `json:"claim"`
		Anchor   string `json:"anchor"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 ||
		req.PageSlug == "" || strings.TrimSpace(req.Claim) == "" {
		response.BadRequestKey(c, response.MsgSpacePageClaimRequired)
		return
	}
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	userID, _ := middleware.GetUserID(c)
	pin, err := api.svc.CreatePin(req.SpaceID, req.PageSlug, req.Kind, req.Claim, req.Anchor, userID)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"pin": pin})
}

// ListPins 列出空间的 pin。
// @Summary Wiki 人工锚点列表
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param status query string false "锚点状态"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pins [get]
func (api *WikiAPI) ListPins(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	pins, err := api.svc.ListPins(spaceID, c.Query("status"))
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"pins": pins, "total": len(pins)})
}

// ResolvePin 处理一条 pin（确认已解决 / 关闭）。
// @Summary Wiki 人工锚点处理
// @Tags AI
// @Produce json
// @Param id path int true "锚点ID"
// @Param status query string false "目标状态"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pins/{id}/resolve [post]
func (api *WikiAPI) ResolvePin(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	spaceID, err := api.svc.PinSpaceID(id)
	if err != nil || !requireWikiMember(c, spaceID) {
		return
	}
	status := c.Query("status")
	if err := api.svc.ResolvePin(id, status); err != nil {
		wikiFail(c, err)
		return
	}
	if status == "" {
		status = "resolved"
	}
	response.Success(c, gin.H{"id": id, "status": status})
}

// ListReviews 异步审核队列。
// @Summary Wiki 审核队列
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param status query string false "审核状态"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/reviews [get]
func (api *WikiAPI) ListReviews(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	items, err := api.svc.ListReviews(spaceID, c.Query("status"))
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"reviews": items, "total": len(items)})
}

// ResolveReview 处理审核项：批准时走乐观锁发布，基线不匹配则要求 rebase。
// @Summary Wiki 审核项处理
// @Tags AI
// @Accept json
// @Produce json
// @Param id path int true "审核项ID"
// @Param body body map[string]string true "action"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/reviews/{id}/resolve [post]
func (api *WikiAPI) ResolveReview(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	spaceID, err := api.svc.ReviewSpaceID(id)
	if err != nil || !requireWikiMember(c, spaceID) {
		return
	}
	var req struct {
		Action string `json:"action"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := api.svc.ResolveReview(id, req.Action, currentUserID(c)); err != nil {
		wikiFail(c, err)
		return
	}
	if req.Action == "" {
		req.Action = "accepted"
	}
	response.Success(c, gin.H{"id": id, "status": req.Action})
}

// RunLint 执行一次体检（孤立页 / 稀疏社区 / 死链 / 过时内容），结果全量替换未处理项。
// @Summary Wiki 体检执行
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/stale_days"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/lint [post]
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
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	issues, err := api.svc.RunLint(req.SpaceID, req.StaleDays)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"issues": issues, "total": len(issues)})
}

// ListLint 列出体检问题。
// @Summary Wiki 体检问题列表
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param status query string false "默认 open"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/lint [get]
func (api *WikiAPI) ListLint(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	items, err := api.svc.ListLint(spaceID, c.DefaultQuery("status", "open"))
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"issues": items, "total": len(items)})
}

// GetInsights 结构性洞察：意外连接 / 知识缺口 / 桥节点，每类带一个可执行动作。
// @Summary Wiki 结构洞察
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/insights [get]
func (api *WikiAPI) GetInsights(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	out, err := api.svc.Insights(spaceID)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"insights": out, "total": len(out)})
}

// RunDeepResearch 针对 Graph Insight 触发深度研究回灌（P3）：
// 生成研究查询 → 汇编已知材料 → 产出研究纪要 draft + 审核项。
// @Summary Wiki 深度研究回灌
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/kind/slugs/detail"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/deep-research [post]
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
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	report, err := api.svc.RunDeepResearch(c.Request.Context(), req.SpaceID, wiki.Insight{
		Kind: req.Kind, Slugs: req.Slugs, Detail: req.Detail,
	})
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"report": report})
}

// GetRelated 4-Signal 相关性排序的相关页面。
// @Summary Wiki 相关页面
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Param limit query int false "默认 10"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/related/{slug} [get]
func (api *WikiAPI) GetRelated(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	out, err := api.svc.GetRelated(spaceID, slug, limit)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"related": out, "total": len(out)})
}

// DeletePage 删除页面（含死链清理）。源文档下架请用 RemoveSource 走级联规则。
// @Summary Wiki 页面删除
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "页面 slug"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug} [delete]
func (api *WikiAPI) DeletePage(c *gin.Context) {
	spaceID, slug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	if err := api.svc.DeletePage(spaceID, slug); err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"slug": slug})
}

// SupersedePage 手动取代：标记旧页面被新页面取代（LLM Wiki 取代链）。
// 与自动取代共用 SupersedePage 治理逻辑：人工页面不取代、已取代页面不重复取代。
// @Summary Wiki 页面取代
// @Tags AI
// @Accept json
// @Produce json
// @Param space_id query int true "空间ID"
// @Param slug path string true "被取代页面 slug"
// @Param body body map[string]string true "superseded_by"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/pages/{slug}/supersede [post]
func (api *WikiAPI) SupersedePage(c *gin.Context) {
	spaceID, oldSlug, ok := querySpaceAndSlug(c)
	if !ok || !requireWikiMember(c, spaceID) {
		return
	}
	var req struct {
		SupersededBy string `json:"superseded_by"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SupersededBy) == "" {
		response.BadRequestKey(c, response.MsgRequestParameterError)
		return
	}
	if err := api.svc.SupersedePage(spaceID, oldSlug, req.SupersededBy); err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"slug": oldSlug, "superseded_by": req.SupersededBy})
}

// RemoveSource 源文档下架的级联清理：独占页删除、共享页摘除溯源、死链清理。
// @Summary Wiki 源文档下架级联
// @Tags AI
// @Accept json
// @Produce json
// @Param body body map[string]interface{} true "space_id/source_doc_id"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/sources/remove [post]
func (api *WikiAPI) RemoveSource(c *gin.Context) {
	var req struct {
		SpaceID     int `json:"space_id"`
		SourceDocID int `json:"source_doc_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SpaceID <= 0 || req.SourceDocID <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDAndSourceDocRequired)
		return
	}
	if !requireWikiMember(c, req.SpaceID) {
		return
	}
	res, err := api.svc.RemoveSource(req.SpaceID, req.SourceDocID)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"result": res})
}

// GetIndex 返回确定性构建的知识目录（查询阶段的导航入口 / Prompt Caching 前缀）。
// @Summary Wiki 知识目录
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/index [get]
func (api *WikiAPI) GetIndex(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	text, entries, err := api.svc.GetIndex(spaceID)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"markdown": text, "entries": entries, "total": len(entries)})
}

// GetLinks 返回空间的交叉引用边，用于图谱可视化。
// @Summary Wiki 交叉引用边
// @Tags AI
// @Produce json
// @Param space_id query int true "空间ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/wiki/links [get]
func (api *WikiAPI) GetLinks(c *gin.Context) {
	spaceID, ok := querySpaceID(c)
	if !ok || !requireWikiReadable(c, spaceID) {
		return
	}
	links, err := api.svc.GetLinks(spaceID)
	if err != nil {
		wikiFail(c, err)
		return
	}
	response.Success(c, gin.H{"links": links, "total": len(links)})
}

// ---- 参数小工具 ----

func querySpaceID(c *gin.Context) (int, bool) {
	id, _ := strconv.Atoi(c.Query("space_id"))
	if id <= 0 {
		response.BadRequestKey(c, response.MsgSpaceIDRequired)
		return 0, false
	}
	return id, true
}

func querySpaceAndSlug(c *gin.Context) (int, string, bool) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	slug := c.Param("slug")
	if spaceID <= 0 || slug == "" {
		response.BadRequestKey(c, response.MsgSpaceIDAndSlugRequired)
		return 0, "", false
	}
	return spaceID, slug, true
}

func pathID(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequestKey(c, response.MsgParameterError)
		return 0, false
	}
	return id, true
}

func currentUserID(c *gin.Context) int {
	id, _ := middleware.GetUserID(c)
	return id
}
