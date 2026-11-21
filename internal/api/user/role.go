package user

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	usersvc "github.com/listen808/citebase/internal/service/user"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/safego"
)

type RoleAPI struct {
	roleService *usersvc.RoleService
	logService  *adminsvc.LogService
}

func NewRoleAPI() *RoleAPI {
	return &RoleAPI{
		roleService: usersvc.NewRoleService(),
		logService:  adminsvc.NewLogService(),
	}
}

// GetRoles 获取所有角色列表
// @Summary 获取角色列表
// @Tags 角色
// @Produce json
// @Success 200 {array} object
// @Router /api/v1/roles [get]
func (api *RoleAPI) GetRoles(c *gin.Context) {
	roles, err := api.roleService.GetRoles()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, roles)
}

// CreateRole creates a new role
func (api *RoleAPI) CreateRole(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name required")
		return
	}
	role, err := api.roleService.CreateRole(req.Name)
	if err != nil {
		response.ServerError(c, "failed to create role")
		return
	}
	response.Success(c, role)

	// 记录角色创建日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(role.ID, 0, operatorID, adminmodel.DocActionCreate, "创建角色: "+role.Name)
	})
}

// UpdateRole updates a role
func (api *RoleAPI) UpdateRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name required")
		return
	}
	role, err := api.roleService.UpdateRole(id, req.Name)
	if err != nil {
		if err == usersvc.ErrCannotModifySuperAdmin {
			response.Forbidden(c, "cannot modify super admin")
			return
		}
		response.ServerError(c, "failed to update")
		return
	}
	response.Success(c, role)

	// 记录角色更新日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "更新角色: "+req.Name)
	})
}

// DeleteRole deletes a role
func (api *RoleAPI) DeleteRole(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := api.roleService.DeleteRole(id); err != nil {
		if err == usersvc.ErrCannotModifySuperAdmin {
			response.Forbidden(c, "cannot delete super admin")
			return
		}
		response.ServerError(c, "failed to delete")
		return
	}
	// P0-9: 删除角色可能级联影响用户角色，主动失效全部 RBAC 缓存
	middleware.InvalidateAllRBACCache()
	response.SuccessWithMsg(c, nil, "deleted")

	// 记录角色删除日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionDelete, "删除角色")
	})
}

// GetPrivileges gets privilege tree
func (api *RoleAPI) GetPrivileges(c *gin.Context) {
	tree, err := api.roleService.GetPrivileges()
	if err != nil {
		response.ServerError(c, "failed to get privileges")
		return
	}
	response.Success(c, tree)
}

// GetRolePrivileges gets role's privilege list
func (api *RoleAPI) GetRolePrivileges(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	privileges, err := api.roleService.GetRolePrivileges(id)
	if err != nil {
		response.ServerError(c, "failed to get privileges")
		return
	}
	response.Success(c, privileges)
}

// AssignPrivileges assigns privileges to role
func (api *RoleAPI) AssignPrivileges(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var req struct {
		PrivilegeIDs []int `json:"privilege_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if err := api.roleService.AssignPrivileges(id, req.PrivilegeIDs); err != nil {
		if err == usersvc.ErrCannotModifySuperAdmin {
			response.Forbidden(c, "cannot modify super admin")
			return
		}
		response.ServerError(c, "failed to assign")
		return
	}
	// P0-9: 角色权限变更影响所有持有该角色的用户，主动失效全部 RBAC 缓存
	middleware.InvalidateAllRBACCache()
	response.SuccessWithMsg(c, nil, "assigned")

	// 记录权限分配日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "分配角色权限")
	})
}

// GetRoleUsers gets users belonging to a specific role
func (api *RoleAPI) GetRoleUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid role id")
		return
	}

	page, pageSize, _ := utils.ParsePagination(c, 20)

	users, total, roleName, err := api.roleService.GetRoleUsers(id, page, pageSize)
	if err != nil {
		response.ServerError(c, "failed to get users")
		return
	}

	resp := map[string]interface{}{
		"data":      users,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"role_name": roleName,
	}
	response.Success(c, resp)
}

// ResetUsers 将指定角色的所有用户重置为普通用户 (role_id=3)
func (api *RoleAPI) ResetUsers(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid role id")
		return
	}

	if err := api.roleService.ResetUsers(id); err != nil {
		if err == usersvc.ErrCannotModifySuperAdmin {
			response.Forbidden(c, "cannot reset super admin")
			return
		}
		response.ServerError(c, "failed to reset users")
		return
	}
	// P0-9: 用户角色被重置为普通用户，权限发生变更，主动失效全部 RBAC 缓存
	middleware.InvalidateAllRBACCache()

	response.SuccessWithMsg(c, nil, "users reset to normal user")

	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "重置角色用户")
	})
}

// AddUser 按用户名把已有用户划入指定角色
// @Summary 为角色添加用户
// @Tags 角色
// @Accept json
// @Produce json
// @Param id path int true "角色 ID"
// @Router /api/v1/roles/{id}/users [post]
func (api *RoleAPI) AddUser(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid role id")
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Username == "" {
		response.BadRequest(c, "username is required")
		return
	}

	user, err := api.roleService.AddUserToRole(id, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, usersvc.ErrCannotModifySuperAdmin):
			response.Forbidden(c, "cannot assign super admin role")
		case errors.Is(err, userrepo.ErrUserNotFound):
			response.Error(c, 404, 404, "user not found")
		default:
			response.ServerErrorSafe(c, err)
		}
		return
	}
	// 角色变更后该用户的权限集已变，失效其 RBAC 缓存
	middleware.InvalidateUserRBACCache(user.ID)

	response.Success(c, user)

	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "为角色添加用户 "+req.Username)
	})
}

func (api *RoleAPI) CreatePrivilege(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		Type       string `json:"type" binding:"required"`
		ParentID   int    `json:"parent_id"`
		Controller string `json:"controller"`
		Action     string `json:"action"`
		Icon       string `json:"icon"`
		Target     string `json:"target"`
		IsDisplay  int    `json:"is_display"`
		Sequence   int    `json:"sequence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name and type required")
		return
	}

	privilege := &usermodel.Privilege{
		Name:       req.Name,
		Type:       req.Type,
		ParentID:   req.ParentID,
		Controller: req.Controller,
		Action:     req.Action,
		Icon:       req.Icon,
		Target:     req.Target,
		IsDisplay:  req.IsDisplay,
		Sequence:   req.Sequence,
	}
	if err := api.roleService.CreatePrivilege(privilege); err != nil {
		response.ServerError(c, "failed to create privilege")
		return
	}
	response.Success(c, privilege)
}

func (api *RoleAPI) UpdatePrivilege(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid privilege id")
		return
	}
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		ParentID   *int   `json:"parent_id"`
		Controller string `json:"controller"`
		Action     string `json:"action"`
		Icon       string `json:"icon"`
		Target     string `json:"target"`
		IsDisplay  *int   `json:"is_display"`
		Sequence   *int   `json:"sequence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.ParentID != nil {
		updates["parent_id"] = *req.ParentID
	}
	if req.Controller != "" {
		updates["controller"] = req.Controller
	}
	if req.Action != "" {
		updates["action"] = req.Action
	}
	if req.Icon != "" {
		updates["icon"] = req.Icon
	}
	if req.Target != "" {
		updates["target"] = req.Target
	}
	if req.IsDisplay != nil {
		updates["is_display"] = *req.IsDisplay
	}
	if req.Sequence != nil {
		updates["sequence"] = *req.Sequence
	}
	if err := api.roleService.UpdatePrivilege(id, updates); err != nil {
		response.ServerError(c, "failed to update privilege")
		return
	}
	// P0-9: 权限元数据（controller/action）变更影响所有用户，主动失效全部 RBAC 缓存
	middleware.InvalidateAllRBACCache()
	privilege, _ := api.roleService.GetPrivilegeByID(id)
	response.Success(c, privilege)
}

func (api *RoleAPI) DeletePrivilege(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		response.BadRequest(c, "invalid privilege id")
		return
	}
	if err := api.roleService.DeletePrivilege(id); err != nil {
		response.ServerError(c, "failed to delete privilege")
		return
	}
	// P0-9: 权限删除影响所有持有该权限的用户，主动失效全部 RBAC 缓存
	middleware.InvalidateAllRBACCache()
	response.SuccessWithMsg(c, nil, "deleted")
}
