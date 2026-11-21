package user

import (
	"encoding/csv"
	"fmt"
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
	"strconv"
	"time"
)

type UserAPI struct {
	userService *usersvc.UserService
	logService  *adminsvc.LogService
}

func NewUserAPI() *UserAPI {
	return &UserAPI{
		userService: usersvc.NewUserService(),
		logService:  adminsvc.NewLogService(),
	}
}

// CreateUser 创建用户
// @Summary 创建用户
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body object true "用户信息"
// @Success 201 {object} object
// @Router /api/v1/users [post]
func (api *UserAPI) CreateUser(c *gin.Context) {
	var req usermodel.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := utils.ValidatePasswordStrength(req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := api.userService.CreateUser(&req)
	if err != nil {
		if err == usersvc.ErrUserAlreadyExists {
			response.Conflict(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, user.ToResponse())

	// 记录用户创建日志（使用用户ID作为document_id占位）
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(user.ID, 0, operatorID, adminmodel.DocActionCreate, "创建用户: "+user.Username)
	})
}

// GetUserByID 根据 ID 获取用户
// @Summary 获取用户信息
// @Tags 用户
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {object} object
// @Router /api/v1/users/:id [get]
func (api *UserAPI) GetUserByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := api.userService.GetUserByID(id)
	if err != nil {
		if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, user)
}

// UpdateUser 更新用户
// @Summary 更新用户信息
// @Tags 用户
// @Accept json
// @Produce json
// @Param id path int true "用户 ID"
// @Param request body object true "用户信息"
// @Success 200 {object} object
// @Router /api/v1/users/:id [put]
func (api *UserAPI) UpdateUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req usermodel.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := api.userService.UpdateUser(id, &req)
	if err != nil {
		if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		if err == usersvc.ErrCannotOperateRoot || err == usersvc.ErrRootCannotChange {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// P0-9: 用户角色/状态变更后主动失效其 RBAC 与用户状态缓存
	if req.RoleID > 0 {
		middleware.InvalidateUserRBACCache(id)
		middleware.InvalidateUserCache(id)
	}

	response.Success(c, user.ToResponse())
}

// DeleteUser 删除用户
// @Summary 删除用户
// @Tags 用户
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {message} string
// @Router /api/v1/users/:id [delete]
func (api *UserAPI) DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	if err := api.userService.DeleteUser(id); err != nil {
		if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		if err == usersvc.ErrCannotOperateRoot {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// P0-5/P0-9: 主动失效用户状态与 RBAC 缓存，防止被删用户在 TTL 内继续访问
	middleware.InvalidateUserCache(id)
	middleware.InvalidateUserRBACCache(id)

	// 记录用户删除日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionDelete, "删除用户")
	})

	response.SuccessWithMsg(c, nil, "user deleted successfully")
}

// GetUserList 获取用户列表
// @Summary 获取用户列表
// @Tags 用户
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Param role_id query int false "角色ID筛选"
// @Param keyword query string false "关键词搜索(用户名/姓名)"
// @Success 200 {array} object
// @Router /api/v1/users [get]
func (api *UserAPI) GetUserList(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 10)
	roleID, _ := strconv.Atoi(c.DefaultQuery("role_id", "0"))
	keyword := c.DefaultQuery("keyword", "")

	// 限制最大页数
	if pageSize > 100 {
		pageSize = 100
	}

	users, total, err := api.userService.GetUserList(page, pageSize, roleID, keyword)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, users, total, page, pageSize)
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Tags 用户
// @Accept json
// @Produce json
// @Param id path int true "用户 ID"
// @Param request body object true "{old_password?: string, new_password: string}"
// @Success 200 {message} string
// @Router /api/v1/users/:id/password [put]
func (api *UserAPI) ChangePassword(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password" binding:"required,min=8,max=60"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := utils.ValidatePasswordStrength(req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 如果提供了旧密码，使用修改密码逻辑；否则使用重置密码逻辑（管理员）
	if req.OldPassword != "" {
		if err := api.userService.ChangePassword(id, req.OldPassword, req.NewPassword); err != nil {
			if err == usersvc.ErrInvalidPassword {
				response.Unauthorized(c, "old password is incorrect")
				return
			}
			if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
				response.NotFound(c, "user not found")
				return
			}
			response.ServerErrorSafe(c, err)
			return
		}
	} else {
		// 管理员重置密码 - 需要管理员权限
		operatorRoleID, exists := c.Get("role_id")
		if !exists || operatorRoleID.(int) != 1 {
			response.Forbidden(c, "only super admin can reset password")
			return
		}
		if err := api.userService.ResetPassword(id, req.NewPassword); err != nil {
			if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
				response.NotFound(c, "user not found")
				return
			}
			if err == usersvc.ErrCannotOperateRoot {
				// 服务层禁止对超管走管理员重置（超管改自己的密码走 PUT /auth/me/password），
				// 不映射的话会落进 ServerErrorSafe 变成 500 "internal server error"。
				response.Forbidden(c, "cannot reset super administrator password")
				return
			}
			response.ServerErrorSafe(c, err)
			return
		}
	}

	response.SuccessWithMsg(c, nil, "password changed successfully")

	// 记录密码修改日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "修改用户密码")
	})
}

// ToggleUserForbidden 切换用户禁用状态
// @Summary 切换用户禁用状态
// @Tags 用户
// @Produce json
// @Param id path int true "用户 ID"
// @Success 200 {message} string
// @Router /api/v1/users/:id/forbidden [put]
func (api *UserAPI) ToggleUserForbidden(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	if err := api.userService.ToggleUserForbidden(id); err != nil {
		if err == usersvc.ErrUserNotFound || err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		if err == usersvc.ErrCannotOperateRoot {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// P0-5: 主动失效用户状态缓存，确保禁用/解禁立即生效
	middleware.InvalidateUserCache(id)

	response.SuccessWithMsg(c, nil, "user status updated")

	// 记录用户状态变更日志
	operatorID, _ := middleware.GetUserID(c)
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, 0, operatorID, adminmodel.DocActionModify, "切换用户禁用状态")
	})
}

// ExportUsersCSV 导出用户列表为 CSV
// @Summary 导出用户列表
// @Tags 用户
// @Produce text/csv
// @Success 200 {file} binary "users.csv"
// @Router /api/v1/users/export [get]
func (api *UserAPI) ExportUsersCSV(c *gin.Context) {
	users, err := api.userService.GetAllUsers()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 设置响应头
	filename := fmt.Sprintf("users_%s.csv", time.Now().Format("20060102150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// 添加 UTF-8 BOM 以支持 Excel
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// 写入表头
	header := []string{"用户ID", "用户名", "姓名", "邮箱", "手机号", "部门", "职位", "角色", "状态", "创建时间"}
	_ = writer.Write(header)

	// 写入数据行
	for _, user := range users {
		status := "正常"
		if user.IsForbidden == 1 {
			status = "禁用"
		}
		roleName := "普通用户"
		if user.RoleID == 1 {
			roleName = "超级管理员"
		} else if user.RoleID == 2 {
			roleName = "管理员"
		}
		createTime := time.Unix(int64(user.CreateTime), 0).Format("2006-01-02 15:04:05")

		row := []string{
			strconv.Itoa(user.ID),
			user.Username,
			user.GivenName,
			user.Email,
			user.Mobile,
			user.Department,
			user.Position,
			roleName,
			status,
			createTime,
		}
		_ = writer.Write(row)
	}
}

// GetUsersForImport 获取全部用户（用于导入联系人）
func (api *UserAPI) GetUsersForImport(c *gin.Context) {
	users, err := api.userService.GetAllUsersBasic()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, users)
}
