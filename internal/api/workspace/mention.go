package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// MentionAPI @提及 接口
type MentionAPI struct {
	mentionService *workspacesvc.MentionService
}

func NewMentionAPI() *MentionAPI {
	return &MentionAPI{mentionService: workspacesvc.NewMentionService()}
}

// GetDocumentMentions 获取文档的 @提及 用户列表
// @Summary 获取文档提及用户
// @Tags 文档
// @Produce json
// @Param id path int true "文档ID"
// @Success 200 {object} object
// @Router /api/v1/documents/{id}/mentions [get]
func (api *MentionAPI) GetDocumentMentions(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	users, err := api.mentionService.GetMentions(docID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, users)
}

// PreviewMentions 预览文本中提取的 @提及 标识
// @Summary 预览提及标识
// @Tags 文档
// @Accept json
// @Produce json
// @Param request body object true "文本"
// @Success 200 {object} object
// @Router /api/v1/mentions/preview [post]
func (api *MentionAPI) PreviewMentions(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"mentions": workspacemodel.ExtractMentions(req.Content)})
}
