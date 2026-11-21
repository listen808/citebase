package auth

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware/jwt"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
)

// OIDCAPI OIDC 认证 API
type OIDCAPI struct {
	oidcService *authsvc.OIDCService
}

func NewOIDCAPI() *OIDCAPI {
	svc := authsvc.NewOIDCService()
	// Initialize all enabled providers on startup
	svc.InitAllProviders()
	return &OIDCAPI{
		oidcService: svc,
	}
}

// GetProviders 获取已启用的 OIDC 提供方列表（公开接口，无需认证）
// GET /api/v1/auth/oidc/providers
func (api *OIDCAPI) GetProviders(c *gin.Context) {
	providers, err := api.oidcService.GetEnabledProviders()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if providers == nil {
		providers = []adminmodel.OIDCProviderPublicInfo{}
	}
	response.Success(c, providers)
}

// Login 发起 OIDC 登录（重定向到 IdP）
// GET /api/v1/auth/oidc/:id/login
func (api *OIDCAPI) Login(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider id")
		return
	}

	// Build the callback URL — this points to the FRONTEND callback page,
	// not the backend API. The IdP will redirect the browser here, and the
	// frontend SSOCallbackView will then call the backend API to exchange the code.
	frontendCallbackURL := getFrontendBaseURL(c) + "/auth/sso/callback"

	// 服务端签发一次性 state + nonce：回调时必须原样带回并被消费，
	// 否则无法区分「IdP 发给本会话的授权码」与「攻击者投递的授权码」。
	state, nonce, err := authsvc.IssueOIDCState(uint(id), frontendCallbackURL)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	authURL, err := api.oidcService.GetAuthURL(uint(id), frontendCallbackURL, state, nonce)
	if err != nil {
		if err == authsvc.ErrProviderNotFound {
			response.NotFound(c, "OIDC provider not found")
			return
		}
		if err == authsvc.ErrProviderNotEnabled {
			response.BadRequest(c, "OIDC provider is not enabled")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"redirect_url": authURL,
		"state":        state,
	})
}

// Callback 处理 OIDC 回调
// GET /api/v1/auth/oidc/callback?code=xxx&state=xxx&provider_id=xxx
func (api *OIDCAPI) Callback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	providerIDStr := c.Query("provider_id")

	if code == "" || state == "" || providerIDStr == "" {
		response.BadRequest(c, "missing code, state or provider_id")
		return
	}

	providerID, err := strconv.ParseUint(providerIDStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid provider_id")
		return
	}

	// 校验并消费 state：必须是本服务签发、未过期、未被用过，且属于同一个 provider
	entry, ok := authsvc.ConsumeOIDCState(state, uint(providerID))
	if !ok {
		response.Unauthorized(c, "invalid or expired state")
		return
	}

	// 使用发起授权时登记的 redirect_uri，避免受 Host 头影响而产生不一致
	result, err := api.oidcService.HandleCallback(uint(providerID), code, entry.RedirectURI(), entry.Nonce())
	if err != nil {
		if err == authsvc.ErrAutoCreateDisabled {
			response.Error(c, http.StatusForbidden, 403, "用户不存在且未启用自动创建，请联系管理员")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// Generate JWT tokens
	accessToken, err := jwt.GenerateToken(result.User.ID, result.User.Username, result.User.RoleID)
	if err != nil {
		response.ServerError(c, "token generation failed")
		return
	}

	refreshToken, err := jwt.GenerateRefreshToken(result.User.ID)
	if err != nil {
		response.ServerError(c, "refresh token generation failed")
		return
	}

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    7200,
		"user_info":     result.UserInfo,
	})
}

// --- Admin CRUD handlers ---

// AdminGetProviders 获取所有 OIDC 提供方（管理员）
// GET /api/v1/auth/oidc/admin/providers
func (api *OIDCAPI) AdminGetProviders(c *gin.Context) {
	providers, err := api.oidcService.GetAllProviders()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	responses := make([]adminmodel.OIDCProviderResponse, len(providers))
	for i, p := range providers {
		responses[i] = p.ToResponse()
		responses[i].InitError = api.oidcService.InitError(p.ID)
	}
	response.Success(c, responses)
}

// attachInitError 把运行态的初始化失败原因附到单个响应上。
// Create/Update 在初始化失败时依旧返回成功（后端只 log warning），不附这一步，
// 管理员看到的就只有"保存成功"，而现实是登录页根本不出现该入口。
func (api *OIDCAPI) attachInitError(resp adminmodel.OIDCProviderResponse, id uint) adminmodel.OIDCProviderResponse {
	resp.InitError = api.oidcService.InitError(id)
	return resp
}

// AdminGetProvider 获取单个 OIDC 提供方（管理员）
// GET /api/v1/auth/oidc/admin/providers/:id
func (api *OIDCAPI) AdminGetProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	provider, err := api.oidcService.GetProvider(uint(id))
	if err != nil {
		response.NotFound(c, "OIDC provider not found")
		return
	}

	response.Success(c, api.attachInitError(provider.ToResponse(), provider.ID))
}

// AdminCreateProvider 创建 OIDC 提供方（管理员）
// POST /api/v1/auth/oidc/admin/providers
func (api *OIDCAPI) AdminCreateProvider(c *gin.Context) {
	var req adminmodel.CreateOIDCProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	provider, err := api.oidcService.CreateProvider(&req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, api.attachInitError(provider.ToResponse(), provider.ID))
}

// AdminUpdateProvider 更新 OIDC 提供方（管理员）
// PUT /api/v1/auth/oidc/admin/providers/:id
func (api *OIDCAPI) AdminUpdateProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req adminmodel.UpdateOIDCProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	provider, err := api.oidcService.UpdateProvider(uint(id), &req)
	if err != nil {
		if err == authsvc.ErrProviderNotFound {
			response.NotFound(c, "OIDC provider not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, api.attachInitError(provider.ToResponse(), provider.ID))
}

// AdminDeleteProvider 删除 OIDC 提供方（管理员）
// DELETE /api/v1/auth/oidc/admin/providers/:id
func (api *OIDCAPI) AdminDeleteProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := api.oidcService.DeleteProvider(uint(id)); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}

// getFrontendBaseURL extracts the frontend base URL (scheme + host) from the request.
// It prefers the Referer header (since the API is called from the frontend login page)
// and falls back to the request Host with X-Forwarded-Proto.
func getFrontendBaseURL(c *gin.Context) string {
	// Try Referer first (most reliable for proxied setups)
	if referer := c.GetHeader("Referer"); referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host
		}
	}

	// Fallback: derive from request
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s", scheme, c.Request.Host)
}
