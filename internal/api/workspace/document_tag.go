package workspace

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// DocumentTagAPI 文档标签 API
type DocumentTagAPI struct {
	tagService *workspacesvc.DocumentTagService
}

// NewDocumentTagAPI 创建文档标签 API 实例
func NewDocumentTagAPI() *DocumentTagAPI {
	tagRepo := workspacerepo.NewDocumentTagRepository(repository.DB)
	return &DocumentTagAPI{
		tagService: workspacesvc.NewDocumentTagService(tagRepo),
	}
}

// GetDocumentTags 获取文档标签
// GET /documents/:id/tags
func (api *DocumentTagAPI) GetDocumentTags(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil || docID <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}

	tags, err := api.tagService.GetDocumentTags(docID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"tags": tags})
}

// SetDocumentTags 设置文档标签（全量替换）
// PUT /documents/:id/tags
func (api *DocumentTagAPI) SetDocumentTags(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil || docID <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	// 标签属于空间内数据：不校验成员身份时任意登录用户都能全量替换他人私有文档的标签
	if !requireDocMember(c, docID) {
		return
	}

	var req struct {
		Tags string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tagNames := parseTagNames(req.Tags)
	if err := api.tagService.SetDocumentTags(docID, tagNames); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	tags, _ := api.tagService.GetDocumentTags(docID)
	response.SuccessWithMsg(c, gin.H{"tags": tags}, "标签设置成功")
}

// AddDocumentTags 追加文档标签
// POST /documents/:id/tags
func (api *DocumentTagAPI) AddDocumentTags(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil || docID <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	// 同 SetDocumentTags：写他人空间的标签属水平越权
	if !requireDocMember(c, docID) {
		return
	}

	var req struct {
		Tags string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tagNames := parseTagNames(req.Tags)
	if len(tagNames) == 0 {
		response.BadRequest(c, "tags is required")
		return
	}

	if err := api.tagService.AddDocumentTags(docID, tagNames); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	tags, _ := api.tagService.GetDocumentTags(docID)
	response.SuccessWithMsg(c, gin.H{"tags": tags}, "标签添加成功")
}

// RemoveDocumentTags 移除文档标签
// DELETE /documents/:id/tags
func (api *DocumentTagAPI) RemoveDocumentTags(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil || docID <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	// 同 SetDocumentTags：写他人空间的标签属水平越权
	if !requireDocMember(c, docID) {
		return
	}

	var req struct {
		Tags string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tagNames := parseTagNames(req.Tags)
	if len(tagNames) == 0 {
		response.BadRequest(c, "tags is required")
		return
	}

	if err := api.tagService.RemoveDocumentTags(docID, tagNames); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	tags, _ := api.tagService.GetDocumentTags(docID)
	response.SuccessWithMsg(c, gin.H{"tags": tags}, "标签移除成功")
}

// GetAllTags 获取所有标签（全局标签池）
// GET /tags?keyword=&limit=
func (api *DocumentTagAPI) GetAllTags(c *gin.Context) {
	keyword := c.DefaultQuery("keyword", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	tags, err := api.tagService.GetAllTags(keyword, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, tags)
}

// GetTagStats 获取标签统计
// GET /tags/stats?limit=
func (api *DocumentTagAPI) GetTagStats(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	stats, err := api.tagService.GetTagStats(limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, stats)
}

// GetDocumentsByTag 根据标签获取文档列表
// GET /tags/:tag/documents?limit=
func (api *DocumentTagAPI) GetDocumentsByTag(c *gin.Context) {
	tagName := c.Param("tag")
	if tagName == "" {
		response.BadRequest(c, "tag name is required")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	docIDs, err := api.tagService.GetDocumentsByTag(tagName, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"document_ids": docIDs})
}

// parseTagNames 解析标签字符串（逗号分隔）
func parseTagNames(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
