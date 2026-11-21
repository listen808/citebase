package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware/jwt"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
)

// AuthorAPI 超级管理员专属登录 API（Author 入口）
type AuthorAPI struct {
	authService         *authsvc.AuthService
	authSSOAPI          *AuthSSOAPI
	unifiedLoginService *authsvc.UnifiedLoginService
}

func NewAuthorAPI() *AuthorAPI {
	return &AuthorAPI{
		authService:         authsvc.NewAuthService(),
		authSSOAPI:          NewAuthSSOAPI(),
		unifiedLoginService: authsvc.NewUnifiedLoginService(),
	}
}

// Login 超级管理员登录（Author 入口）
// 仅允许 role_id == 1 的超级管理员登录
func (api *AuthorAPI) Login(c *gin.Context) {
	var req authsvc.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := api.authService.Login(req.Username, req.Password, c.ClientIP())
	if err != nil {
		if err == authsvc.ErrInvalidCredentials {
			response.UnauthorizedKey(c, response.MsgInvalidCredentials)
			return
		}
		if err == authsvc.ErrAccountLocked {
			// 429 的语义保持不变（前端按状态码倒计时），句子改由键决定
			response.ErrorKey(c, http.StatusTooManyRequests, response.MsgAccountLocked)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 超级管理员验证：必须与「口令错误」返回完全一致的状态码与文案，
	// 否则该入口就成了「这个账号是不是超管 / 口令对不对」的判定预言机。
	if resp.UserInfo.RoleID != 1 {
		logx.Warnf("[Author] 非超级管理员尝试通过 Author 入口登录: user=%s id=%d role=%d", req.Username, resp.UserInfo.UserID, resp.UserInfo.RoleID)
		response.UnauthorizedKey(c, response.MsgInvalidCredentials)
		return
	}

	response.Success(c, resp)
}

// GetAuthorConfig 获取 Author 登录页配置（SSO 是否开启）
func (api *AuthorAPI) GetAuthorConfig(c *gin.Context) {
	// 系统设置「统一登录(SSO)」总开关（basic 组）：显式配为 "1" 才算开启，缺键/其他值都视为关闭。
	// 关闭时登录页不出现 SSO 入口（ssos 一并清空），但已配置的 SSO 提供方数据不受影响。
	ssoOpen := false
	if v, err := adminrepo.NewConfigRepository().GetValueByKey("sso_open"); err == nil {
		ssoOpen = v == "1"
	}

	// 获取已启用的 SSO 认证
	ssos, err := api.authSSOAPI.getUsedAuthSSOs()
	if err != nil {
		response.Success(c, gin.H{"sso_open": false, "sso_enabled": false})
		return
	}

	enabled := ssoOpen && len(ssos) > 0
	if !enabled {
		ssos = []map[string]interface{}{}
	}
	response.Success(c, gin.H{
		"sso_open":    enabled,
		"sso_enabled": enabled,
		"ssos":        ssos,
	})
}

// AuthorLoginRequest Author SSO 登录请求
type AuthorLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthorSSOLogin 统一登录入口 (LDAP/HTTP)
func (api *AuthorAPI) AuthorSSOLogin(c *gin.Context) {
	// binding:"required"：空用户名/空密码不能进入 AuthLogin，
	// 否则该公开端点会变成对后端 LDAP/HTTP 认证服务的匿名探测跳板
	var req AuthorLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgCredentialsEmpty)
		return
	}
	if strings.TrimSpace(req.Username) == "" || req.Password == "" {
		response.BadRequestKey(c, response.MsgCredentialsEmpty)
		return
	}

	userResp, err := api.unifiedLoginService.AuthLogin(req.Username, req.Password)
	if err != nil {
		// 认证失败原因（内部主机名/端口、bind DN、"未配置统一登录"等）只写日志，
		// 不能回显给未认证调用方，否则可用于枚举后端与配置状态
		logx.Warnf("[Author] 统一登录认证失败 user=%s ip=%s: %v", req.Username, c.ClientIP(), err)
		response.UnauthorizedKey(c, response.MsgAuthFailed)
		return
	}

	// 与「认证失败」返回完全一致的 401 文案：先用 401 再用 403 区分会让本入口
	// 变成「这个账号是不是超管」的判定预言机（与上方 Login 的处理保持一致）
	if userResp.RoleID != 1 {
		logx.Warnf("[Author] 非超级管理员尝试通过统一登录入口登录: user=%s id=%d role=%d", userResp.Username, userResp.ID, userResp.RoleID)
		response.UnauthorizedKey(c, response.MsgAuthFailed)
		return
	}

	accessToken, err := jwt.GenerateToken(userResp.ID, userResp.Username, userResp.RoleID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	refreshToken, _ := jwt.GenerateRefreshToken(userResp.ID)

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    7200,
		"user_info": authsvc.UserInfo{
			UserID:    userResp.ID,
			Username:  userResp.Username,
			GivenName: userResp.GivenName,
			RoleID:    userResp.RoleID,
		},
	})
}

func (api *AuthorAPI) AuthorAuthLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	userResp, err := api.unifiedLoginService.AuthLogin(req.Username, req.Password)
	if err != nil {
		// 统一登录失败时降级为本地口令登录，但必须留下失败原因，
		// 否则 LDAP/HTTP 后端故障与口令错误无法区分，事后也无法审计
		logx.Warnf("[Author] 统一登录失败，降级本地登录: user=%s ip=%s err=%v", req.Username, c.ClientIP(), err)
		resp, loginErr := api.authService.Login(req.Username, req.Password, c.ClientIP())
		if loginErr != nil {
			if loginErr == authsvc.ErrAccountLocked {
				response.ErrorKey(c, http.StatusTooManyRequests, response.MsgAccountLocked)
				return
			}
			response.UnauthorizedKey(c, response.MsgLoginFailed)
			return
		}
		response.Success(c, resp)
		return
	}

	accessToken, err := jwt.GenerateToken(userResp.ID, userResp.Username, userResp.RoleID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	refreshToken, _ := jwt.GenerateRefreshToken(userResp.ID)

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    7200,
		"user_info": authsvc.UserInfo{
			UserID:    userResp.ID,
			Username:  userResp.Username,
			GivenName: userResp.GivenName,
			RoleID:    userResp.RoleID,
		},
	})
}

func (api *AuthorAPI) getUsedSSOs() ([]map[string]interface{}, error) {
	return api.authSSOAPI.getUsedAuthSSOs()
}
