package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	usermodel "github.com/listen808/citebase/internal/model/user"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
	usersvc "github.com/listen808/citebase/internal/service/user"
	"github.com/listen808/citebase/internal/utils"
)

type AuthAPI struct {
	authService *authsvc.AuthService
}

func NewAuthAPI() *AuthAPI {
	return &AuthAPI{
		authService: authsvc.NewAuthService(),
	}
}

// Login 用户登录
// @Summary 用户登录
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object true "登录凭证"
// @Success 200 {object} object
// @Router /api/v1/auth/login [post]
func (api *AuthAPI) Login(c *gin.Context) {
	var req authsvc.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	clientIP := c.ClientIP()
	resp, err := api.authService.Login(req.Username, req.Password, clientIP)
	if err != nil {
		if err == authsvc.ErrInvalidCredentials {
			response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, resp)
}

// RefreshToken 刷新访问令牌
// @Summary 刷新令牌
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object true "刷新令牌"
// @Success 200 {object} object
// @Router /api/v1/auth/refresh [post]
func (api *AuthAPI) RefreshToken(c *gin.Context) {
	var req authsvc.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	resp, err := api.authService.RefreshToken(req.RefreshToken)
	if err != nil {
		response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	response.Success(c, resp)
}

// Logout 用户登出
// @Summary 用户登出
// @Tags 认证
// @Produce json
// @Success 200 {message} string
// @Router /api/v1/auth/logout [post]
func (api *AuthAPI) Logout(c *gin.Context) {
	// 从 Authorization header 提取 token 并加入黑名单
	authHeader := c.GetHeader("Authorization")
	if parts := strings.SplitN(authHeader, " ", 2); len(parts) == 2 && parts[0] == "Bearer" {
		_ = api.authService.Logout(parts[1])
	}
	response.Success(c, nil)
}

// GetCurrentUser 获取当前登录用户信息
// @Summary 获取当前用户信息
// @Tags 认证
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/auth/me [get]
func (api *AuthAPI) GetCurrentUser(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	user, err := api.authService.GetFullUserInfo(uid)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, user)
}

// UpdateCurrentUser 当前登录用户更新自己的资料
// @Summary 更新当前用户资料
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object true "用户资料（不允许修改 role_id）"
// @Success 200 {object} object
// @Router /api/v1/auth/me [put]
func (api *AuthAPI) UpdateCurrentUser(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	var req usermodel.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}
	// 安全：禁止通过此接口修改角色
	req.RoleID = 0

	user, err := api.authService.UpdateOwnProfile(uid, &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, user)
}

// UpdateMyPreferences 当前登录用户保存个性化偏好（外观 × 材质 × 透明变体 × 行为三档）
// 行为三档 = 动效强度 / 对比度 / 透明度（2026-10-16 批2）。请求体的形状由 usermodel.Preferences 决定，
// 所以下面那条 @Param 描述刻意不逐一列字段名 —— 列了就得跟着加档改一次，而 docs/swagger 是生成物。
// @Summary 保存当前用户偏好
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object true "theme / material / glass_clear，取值清单见 model/user/preferences.go"
// @Success 200 {object} object
// @Router /api/v1/auth/me/preferences [put]
func (api *AuthAPI) UpdateMyPreferences(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	var req usermodel.Preferences
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	info, err := api.authService.SaveOwnPreferences(uid, &req)
	if err != nil {
		// 校验错一律折成同一句 400：错误信息直出去等于把服务端认得的取值清单
		// 变成对外接口面（而且这一清单的真相源在前端，不该由错误信息代它说话）。
		if errors.Is(err, usermodel.ErrPreferencesEmpty) ||
			errors.Is(err, usermodel.ErrPreferencesTheme) ||
			errors.Is(err, usermodel.ErrPreferencesMaterial) ||
			errors.Is(err, usermodel.ErrPreferencesMotion) ||
			errors.Is(err, usermodel.ErrPreferencesContrast) ||
			errors.Is(err, usermodel.ErrPreferencesTransparency) ||
			errors.Is(err, usermodel.ErrPreferencesAccent) ||
			errors.Is(err, usermodel.ErrPreferencesPersona) ||
			errors.Is(err, usermodel.ErrPreferencesTooLong) {
			response.ErrorWithErr(c, response.ErrBadRequest, http.StatusBadRequest)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, info)
}

// ChangeOwnPassword 当前登录用户修改自己的密码
// @Summary 修改当前用户密码
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body object true "旧密码与新密码"
// @Success 200 {message} string
// @Router /api/v1/auth/me/password [put]
func (api *AuthAPI) ChangeOwnPassword(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=8,max=60"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}
	if err := utils.ValidatePasswordStrength(req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := api.authService.ChangeOwnPassword(uid, req.OldPassword, req.NewPassword); err != nil {
		if err == usersvc.ErrInvalidPassword {
			response.ErrorWithErr(c, response.ErrUnauthorized, http.StatusUnauthorized)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}
