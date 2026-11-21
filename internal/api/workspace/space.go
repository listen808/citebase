package workspace

import (
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/safego"
)

type SpaceAPI struct {
	spaceService *workspacesvc.SpaceService
	logService   *adminsvc.LogService
}

func NewSpaceAPI() *SpaceAPI {
	return &SpaceAPI{
		spaceService: workspacesvc.NewSpaceService(),
		logService:   adminsvc.NewLogService(),
	}
}

// CreateSpace 创建空间
func (api *SpaceAPI) CreateSpace(c *gin.Context) {
	var req workspacemodel.CreateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	space, err := api.spaceService.CreateSpace(uid, &req)
	if err != nil {
		if err == workspacesvc.ErrSpaceAlreadyExists {
			response.ConflictKey(c, response.MsgSpaceNameTaken)
			return
		}
		if err == workspacesvc.ErrSpaceNameInvalid {
			response.BadRequestKey(c, response.MsgSpaceNameIllegalCharsHint)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, space.ToResponse())

	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(space.ID, space.ID, uid, adminmodel.DocActionCreate, "创建空间: "+space.Name)
	})
}

// GetSpaceByID 根据 ID 获取空间
func (api *SpaceAPI) GetSpaceByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	space, err := api.spaceService.GetSpaceByID(id)
	if err != nil {
		if err == workspacerepo.ErrSpaceNotFound {
			response.NotFound(c, "space not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 公开路由需校验空间访问权限
	userID, _ := middleware.GetUserID(c)
	ok, err := api.spaceService.CheckSpaceAccess(id, userID)
	if err != nil || !ok {
		response.Forbidden(c, "no permission to access this space")
		return
	}

	response.Success(c, space)
}

// GetMySpaces 获取当前用户的空间列表 (支持关键词和标签筛选)
func (api *SpaceAPI) GetMySpaces(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.DefaultQuery("keyword", "")
	tags := c.DefaultQuery("tags", "")

	if pageSize > 100 {
		pageSize = 100
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	spaces, total, err := api.spaceService.GetSpacesByUserID(uid, page, pageSize, keyword, tags)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, spaces, total, page, pageSize)
}

// GetAllSpaces 获取所有公开空间 (支持关键词、标签、访问级别筛选)
func (api *SpaceAPI) GetAllSpaces(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.DefaultQuery("keyword", "")
	tags := c.DefaultQuery("tags", "")
	visitLevel := c.DefaultQuery("visit_level", "")

	if pageSize > 100 {
		pageSize = 100
	}

	// 该路由在 publicGroup 上（可匿名访问），但挂了 OptionalAuth：
	// 有登录态时按该用户可见范围裁剪，无登录态时只剩公开空间。
	userID, _ := middleware.GetUserID(c)

	spaces, total, err := api.spaceService.GetAllSpaces(page, pageSize, keyword, tags, visitLevel, userID, middleware.IsSuperAdmin(c))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, spaces, total, page, pageSize)
}

// UpdateSpace 更新空间
func (api *SpaceAPI) UpdateSpace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	var req workspacemodel.UpdateSpaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	privilege, err := api.spaceService.GetUserSpacePrivilege(id, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if privilege < 2 {
		response.Forbidden(c, "only space admin can update space")
		return
	}

	space, err := api.spaceService.UpdateSpace(id, &req)
	if err != nil {
		if err == workspacerepo.ErrSpaceNotFound {
			response.NotFound(c, "space not found")
			return
		}
		if err == workspacesvc.ErrSpaceAlreadyExists {
			response.ConflictKey(c, response.MsgSpaceNameTaken)
			return
		}
		if err == workspacesvc.ErrSpaceNameInvalid {
			response.BadRequestKey(c, response.MsgSpaceNameIllegalChars)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, space.ToResponse())
}

// PreCheckDeleteSpace 删除空间前预检查
func (api *SpaceAPI) PreCheckDeleteSpace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	privilege, err := api.spaceService.GetUserSpacePrivilege(id, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if privilege < 2 {
		response.Forbidden(c, "only space admin can delete space")
		return
	}

	check, err := api.spaceService.PreCheckDeleteSpace(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, check)
}

// DeleteSpace 删除空间
func (api *SpaceAPI) DeleteSpace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	privilege, err := api.spaceService.GetUserSpacePrivilege(id, uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if privilege < 2 {
		response.Forbidden(c, "only space admin can delete space")
		return
	}

	if err := api.spaceService.DeleteSpace(id); err != nil {
		if err == workspacerepo.ErrSpaceNotFound {
			response.NotFound(c, "space not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	safego.Go("LogDocumentAction", func() { _ = api.logService.LogDocumentAction(id, id, uid, adminmodel.DocActionDelete, "删除空间") })

	response.SuccessWithMsg(c, nil, "space deleted successfully")
}

// DownloadSpace 下载空间文档打包
func (api *SpaceAPI) DownloadSpace(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	privilege, err := api.spaceService.GetUserSpacePrivilege(id, uid)
	if err != nil {
		if err == workspacesvc.ErrNotSpaceMember {
			response.Forbidden(c, "you are not a member of this space")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	if privilege < 1 {
		response.Forbidden(c, "you are not a member of this space")
		return
	}

	zipPath, fileName, err := api.spaceService.DownloadSpace(id)
	if err != nil {
		if err == workspacesvc.ErrSpaceExportDisabled {
			response.ForbiddenKey(c, response.MsgSpaceDisallowsExport)
			return
		}
		if err == workspacerepo.ErrSpaceNotFound {
			response.NotFound(c, "space not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	defer os.Remove(zipPath)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(fileName)))
	c.Header("Content-Type", "application/zip")
	c.File(zipPath)
}

// GetSpaceMembers 获取空间成员列表
func (api *SpaceAPI) GetSpaceMembers(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	members, err := api.spaceService.GetSpaceMembers(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, members)
}

// AddSpaceMember 添加空间成员
func (api *SpaceAPI) AddSpaceMember(c *gin.Context) {
	idStr := c.Param("id")
	spaceID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	var req workspacemodel.AddSpaceUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	operatorID, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.spaceService.AddSpaceMember(spaceID, operatorID, req.UserID, req.Privilege); err != nil {
		if err == workspacesvc.ErrNoPermission {
			response.Forbidden(c, "only space admin can add members")
			return
		}
		if err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(spaceID, spaceID, operatorID, adminmodel.DocActionCreate, "添加空间成员")
	})

	response.SuccessWithMsg(c, nil, "member added successfully")
}

// RemoveSpaceMember 移除空间成员
func (api *SpaceAPI) RemoveSpaceMember(c *gin.Context) {
	idStr := c.Param("id")
	spaceID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	var req struct {
		UserID int `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		targetUserID, _ := strconv.Atoi(c.Query("user_id"))
		req.UserID = targetUserID
	}
	targetUserID := req.UserID
	operatorID, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.spaceService.RemoveSpaceMember(spaceID, operatorID, targetUserID); err != nil {
		if err == workspacesvc.ErrNoPermission {
			response.Forbidden(c, "only space admin can remove members")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(spaceID, spaceID, operatorID, adminmodel.DocActionDelete, "移除空间成员")
	})

	response.SuccessWithMsg(c, nil, "member removed successfully")
}

// UpdateMemberPrivilege 更新成员权限
func (api *SpaceAPI) UpdateMemberPrivilege(c *gin.Context) {
	idStr := c.Param("id")
	spaceID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	var req workspacemodel.UpdateSpaceUserPrivilegeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	targetUserID := req.UserID
	if targetUserID == 0 {
		targetUserID, _ = strconv.Atoi(c.Query("user_id"))
	}
	operatorID, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.spaceService.UpdateMemberPrivilege(spaceID, operatorID, targetUserID, req.Privilege); err != nil {
		if err == workspacesvc.ErrNoPermission {
			response.Forbidden(c, "only space admin can update member privilege")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}

	response.SuccessWithMsg(c, nil, "privilege updated successfully")
}

// SearchTags 搜索标签建议
func (api *SpaceAPI) SearchTags(c *gin.Context) {
	keyword := c.DefaultQuery("keyword", "")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	tags, err := api.spaceService.SearchTags(keyword, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tags)
}

// SortSpaces 批量排序空间
func (api *SpaceAPI) SortSpaces(c *gin.Context) {
	var req workspacemodel.SortSpacesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.spaceService.SortSpaces(uid, &req); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "排序已保存")
}
