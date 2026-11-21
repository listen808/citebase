package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
)

// TemplateAPI 文档模板接口
type TemplateAPI struct {
	templateService *workspacesvc.TemplateService
}

func NewTemplateAPI() *TemplateAPI {
	return &TemplateAPI{templateService: workspacesvc.NewTemplateService()}
}

// CreateTemplate 创建模板
// @Summary 创建文档模板
// @Tags 模板
// @Accept json
// @Produce json
// @Param request body workspace.CreateTemplateRequest true "模板"
// @Success 200 {object} object
// @Router /api/v1/templates [post]
func (api *TemplateAPI) CreateTemplate(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	var req workspacemodel.CreateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	t, err := api.templateService.Create(userID, &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, t.ToResponse())
}

// requireTemplateWritable 校验调用者可以修改/删除该模板。
// 模板此前只按 :id 操作，service 层不接收用户维度，任何登录用户都能改删他人空间的模板。
// 放行：创建者本人、超级管理员、或模板所属空间的管理员（全局模板 SpaceID=0 仅创建者与超管）。
func (api *TemplateAPI) requireTemplateWritable(c *gin.Context, id int) bool {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return false
	}
	t, err := api.templateService.Get(id)
	if err != nil || t == nil {
		response.NotFound(c, "template not found")
		return false
	}
	if middleware.IsSuperAdmin(c) || t.CreateUserID == uid {
		return true
	}
	if t.SpaceID > 0 {
		privilege, perr := spaceSvc().GetUserSpacePrivilege(t.SpaceID, uid)
		if perr == nil && privilege >= 2 {
			return true
		}
	}
	response.ForbiddenKey(c, response.MsgTemplateNameEditDenied)
	return false
}

// UpdateTemplate 更新模板
func (api *TemplateAPI) UpdateTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid template id")
		return
	}
	if !api.requireTemplateWritable(c, id) {
		return
	}
	var req workspacemodel.UpdateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := api.templateService.Update(id, &req); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// DeleteTemplate 删除模板
func (api *TemplateAPI) DeleteTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid template id")
		return
	}
	if !api.requireTemplateWritable(c, id) {
		return
	}
	if err := api.templateService.Delete(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"id": id})
}

// GetTemplate 获取模板
func (api *TemplateAPI) GetTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid template id")
		return
	}
	t, err := api.templateService.Get(id)
	if err != nil {
		response.NotFound(c, "template not found")
		return
	}
	response.Success(c, t.ToResponse())
}

// ListTemplates 列出模板（含全局）
func (api *TemplateAPI) ListTemplates(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.DefaultQuery("space_id", "0"))
	page, pageSize, _ := utils.ParsePagination(c, 20)
	list, total, err := api.templateService.List(spaceID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, list, total, page, pageSize)
}
