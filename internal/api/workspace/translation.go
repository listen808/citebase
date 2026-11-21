package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// TranslationAPI 文档翻译管理 API
type TranslationAPI struct {
	docService *workspacesvc.DocumentService
}

func NewTranslationAPI() *TranslationAPI {
	return &TranslationAPI{
		docService: workspacesvc.NewDocumentService(),
	}
}

// GetSupportedLanguages 获取支持的语言列表
// @Summary 支持的语言列表
// @Tags 文档翻译
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/translations/languages [get]
func (api *TranslationAPI) GetSupportedLanguages(c *gin.Context) {
	response.Success(c, workspacemodel.SupportedLanguages)
}

// GetTranslations 获取文档的翻译版本列表
// @Summary 文档翻译版本列表
// @Tags 文档翻译
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/:id/translations [get]
func (api *TranslationAPI) GetTranslations(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	// 翻译列表会带出关联文档 ID 与语言状态，需按原文档所在空间校验可读性
	if !requireDocReadable(c, id) {
		return
	}

	translations, err := api.docService.GetTranslationGroup(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, translations)
}

// CreateTranslation 创建文档翻译关联
// @Summary 创建文档翻译
// @Tags 文档翻译
// @Accept json
// @Produce json
// @Param id path int true "原文档 ID"
// @Param request body object true "{lang, translated_document_id, translation_status}"
// @Success 201 {object} object
// @Router /api/v1/documents/:id/translations [post]
func (api *TranslationAPI) CreateTranslation(c *gin.Context) {
	idStr := c.Param("id")
	documentID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	var req struct {
		Lang                 string `json:"lang" binding:"required"`
		TranslatedDocumentID int    `json:"translated_document_id" binding:"required"`
		TranslationStatus    string `json:"translation_status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.TranslationStatus == "" {
		req.TranslationStatus = workspacemodel.TranslationStatusDraft
	}

	// 建立跨文档关联等于改写两个空间的数据关系：必须确认调用者是原文档所在空间成员，
	// 且对被关联的译文文档也有读取权限，否则可把他人私有文档挂到自己的翻译组上。
	if !requireDocMember(c, documentID) {
		return
	}
	if !requireDocReadable(c, req.TranslatedDocumentID) {
		return
	}

	translation, err := api.docService.CreateTranslation(documentID, req.Lang, req.TranslatedDocumentID, req.TranslationStatus)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, translation)
}

// DeleteTranslation 删除翻译关联
// @Summary 删除文档翻译
// @Tags 文档翻译
// @Produce json
// @Param id path int true "翻译关联 ID"
// @Success 200 {object} object
// @Router /api/v1/translations/:id [delete]
func (api *TranslationAPI) DeleteTranslation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !requireTranslationWritable(c, id) {
		return
	}
	if err := api.docService.DeleteTranslation(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}
