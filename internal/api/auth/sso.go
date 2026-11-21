package auth

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware/jwt"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	authrepo "github.com/listen808/citebase/internal/repository/auth"
	"github.com/listen808/citebase/internal/response"
	authsvc "github.com/listen808/citebase/internal/service/auth"
	"github.com/listen808/citebase/internal/utils"
)

// maxSSOPageSize 列表接口单页上限，避免 page_size 过大退化为全表拉取
const maxSSOPageSize = 100

// AuthSSOAPI SSO 统一认证配置 API
type AuthSSOAPI struct {
	ssoRepo *authrepo.AuthSSORepository
}

func NewAuthSSOAPI() *AuthSSOAPI {
	return &AuthSSOAPI{
		ssoRepo: authrepo.NewAuthSSORepository(),
	}
}

// GetAuthSSOs 获取 SSO 配置列表
// @Summary SSO 配置列表
// @Tags 认证配置
// @Produce json
// @Param page query int false "页码"
// @Param keyword query string false "搜索关键词"
// @Success 200
// @Router /api/v1/auth-sso [get]
func (api *AuthSSOAPI) GetAuthSSOs(c *gin.Context) {
	// 分页参数必须钳制：page=0 会算出负 offset（GORM 生成非法 SQL → 500），
	// page_size 不设上限则一次拉全表并在内存构造等长响应
	page, pageSize, offset := utils.ParsePagination(c, 20)
	if pageSize > maxSSOPageSize {
		pageSize = maxSSOPageSize
		offset = (page - 1) * pageSize
	}
	keyword := c.DefaultQuery("keyword", "")

	ssos, total, err := api.ssoRepo.FindByKeyword(keyword, offset, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	responses := make([]adminmodel.AuthSSOResponse, len(ssos))
	for i, s := range ssos {
		responses[i] = s.ToResponse()
	}

	response.Paginated(c, responses, total, page, pageSize)
}

// GetAuthSSOByID 根据 ID 获取 SSO 配置
// @Summary 获取 SSO 配置
// @Tags 认证配置
// @Produce json
// @Param id path int true "SSO ID"
// @Success 200
// @Router /api/v1/auth-sso/{id} [get]
func (api *AuthSSOAPI) GetAuthSSOByID(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	sso, err := api.ssoRepo.FindByID(id)
	if err != nil {
		response.NotFoundKey(c, response.MsgSSONotFound)
		return
	}

	response.Success(c, sso.ToResponse())
}

// GetUsedAuthSSO 获取已启用的 SSO 列表（公开接口，无需认证）
func (api *AuthSSOAPI) GetUsedAuthSSO(c *gin.Context) {
	ssos, err := api.getUsedAuthSSOs()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, ssos)
}

// getUsedAuthSSOs 内部方法：获取已启用的 SSO（不暴露敏感字段）
func (api *AuthSSOAPI) getUsedAuthSSOs() ([]map[string]interface{}, error) {
	used, err := api.ssoRepo.GetUsed()
	if err != nil {
		return nil, err
	}
	if used == nil {
		return []map[string]interface{}{}, nil
	}
	return []map[string]interface{}{
		{
			"id":              used.ID,
			"name":            used.Name,
			"username_prefix": used.UsernamePrefix,
		},
	}, nil
}

// CreateAuthSSO 创建 SSO 配置
// @Summary 创建 SSO 配置
// @Tags 认证配置
// @Accept json
// @Produce json
// @Success 201
// @Router /api/v1/auth-sso [post]
func (api *AuthSSOAPI) CreateAuthSSO(c *gin.Context) {
	var req adminmodel.CreateAuthSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 检查名称重复
	exists, _ := api.ssoRepo.HasName(req.Name)
	if exists {
		response.ConflictKey(c, response.MsgSSONameTaken)
		return
	}

	// 检查前缀重复
	exists, _ = api.ssoRepo.HasPrefix(req.UsernamePrefix)
	if exists {
		response.ConflictKey(c, response.MsgSSOPrefixTaken)
		return
	}

	sso := &adminmodel.AuthSSO{
		Name:           req.Name,
		URL:            req.URL,
		UsernamePrefix: req.UsernamePrefix,
		ExtData:        req.ExtData,
	}

	if err := api.ssoRepo.Create(sso); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, sso.ToResponse())
}

// UpdateAuthSSO 更新 SSO 配置
// @Summary 更新 SSO 配置
// @Tags 认证配置
// @Accept json
// @Produce json
// @Success 200
// @Router /api/v1/auth-sso/{id} [put]
func (api *AuthSSOAPI) UpdateAuthSSO(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req adminmodel.UpdateAuthSSORequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	sso, err := api.ssoRepo.FindByID(id)
	if err != nil {
		response.NotFoundKey(c, response.MsgSSONotFound)
		return
	}

	conflict, _ := api.ssoRepo.HasNameExcludeID(id, req.Name)
	if conflict {
		response.ConflictKey(c, response.MsgSSONameTaken)
		return
	}

	conflict, _ = api.ssoRepo.HasPrefixExcludeID(id, req.UsernamePrefix)
	if conflict {
		response.ConflictKey(c, response.MsgSSOPrefixTaken)
		return
	}

	sso.Name = req.Name
	sso.URL = req.URL
	sso.UsernamePrefix = req.UsernamePrefix
	sso.ExtData = req.ExtData

	if err := api.ssoRepo.Update(sso); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, sso.ToResponse())
}

// DeleteAuthSSO 删除 SSO 配置
// @Summary 删除 SSO 配置
// @Tags 认证配置
// @Produce json
// @Success 200
// @Router /api/v1/auth-sso/{id} [delete]
func (api *AuthSSOAPI) DeleteAuthSSO(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := api.ssoRepo.Delete(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}

// SSOLogin 发起 SSO 登录（GET：重定向到 SSO 提供方；POST：同样）
// GET  /api/v1/auth/sso/:id/login
// POST /api/v1/auth/sso/:id/login
func (api *AuthSSOAPI) SSOLogin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid SSO id")
		return
	}

	sso, err := api.ssoRepo.FindByID(id)
	if err != nil {
		response.NotFound(c, "SSO configuration not found")
		return
	}

	// 生成一次性 state（服务端登记，绑定本 SSO 配置，10 分钟有效）
	state, err := authsvc.IssueSSOState(sso.ID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 从 ext_data 中提取 callback_url
	callbackURL := ""
	if sso.ExtData != "" {
		var ext map[string]string
		if json.Unmarshal([]byte(sso.ExtData), &ext) == nil {
			callbackURL = ext["callback_url"]
		}
	}

	// 构建 SSO 重定向 URL（简化模式：直接拼接）
	// 实际项目中，这里应该是 OAuth2/OIDC 的 /authorize 端点
	// 简化模式：SSO provider URL 作为入口，将 callback 放在 query 参数中
	ssoLoginURL := sso.URL
	if strings.Contains(ssoLoginURL, "?") {
		ssoLoginURL += "&"
	} else {
		ssoLoginURL += "?"
	}
	ssoLoginURL += "state=" + state
	if callbackURL != "" {
		ssoLoginURL += "&redirect_uri=" + callbackURL
	} else {
		// 默认回调到本地 callback
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		callbackBase := fmt.Sprintf("%s://%s", scheme, c.Request.Host)
		ssoLoginURL += "&redirect_uri=" + fmt.Sprintf("%s/api/v1/auth/sso/callback?sso_id=%d", callbackBase, sso.ID)
	}

	response.Success(c, gin.H{
		"redirect_url": ssoLoginURL,
		"state":        state,
	})
}

// SSOCallback SSO 回调处理
// GET /api/v1/auth/sso/callback?code=xxx&state=xxx&sso_id=xxx
func (api *AuthSSOAPI) SSOCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	ssoIDStr := c.Query("sso_id")

	if code == "" || state == "" || ssoIDStr == "" {
		response.BadRequest(c, "missing code, state or sso_id")
		return
	}

	ssoID, _ := strconv.Atoi(ssoIDStr)
	sso, err := api.ssoRepo.FindByID(ssoID)
	if err != nil {
		response.NotFound(c, "SSO configuration not found")
		return
	}

	// state 必须是本服务签发、未过期且未被消费过的（防 CSRF 与重放）
	if !authsvc.ConsumeSSOState(state, ssoID) {
		response.Unauthorized(c, "invalid or expired state")
		return
	}

	// 用授权码向 IdP 换取真实身份：绝不允许把 code 直接当用户名
	identity, err := authsvc.ExchangeSSOCode(c.Request.Context(), sso.ExtData, code)
	if err != nil {
		logx.Warnf("[SSO] 授权码交换失败 (sso_id=%d): %v", ssoID, err)
		response.UnauthorizedKey(c, response.MsgSSOAuthCodeInvalid)
		return
	}

	// 组合完整用户名（加上前缀防止冲突）；identity.Username 已由服务层保证非空
	fullUsername := sso.UsernamePrefix + identity.Username

	// 查找或创建用户
	authService := authsvc.NewAuthService()
	user, err := authService.FindOrCreateSSOUser(fullUsername, sso.ID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 生成 JWT
	accessToken, err := jwt.GenerateToken(user.ID, user.Username, user.RoleID)
	if err != nil {
		response.ServerError(c, "token generation failed")
		return
	}

	refreshToken, err := jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		response.ServerError(c, "refresh token generation failed")
		return
	}

	response.Success(c, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_in":    7200,
		"user_info": authsvc.UserInfo{
			UserID:    user.ID,
			Username:  user.Username,
			GivenName: user.GivenName,
			RoleID:    user.RoleID,
		},
	})
}

// SetDefaultSSO 设为默认 SSO
// @Summary 设为默认认证
// @Tags 认证配置
// @Produce json
// @Success 200
// @Router /api/v1/auth-sso/{id}/default [put]
func (api *AuthSSOAPI) SetDefaultSSO(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	// 「取消全部默认」与「设置新默认」必须成对生效：
	// 旧实现丢弃 UnsetAllUsed 的错误，失败时会留下多条 is_used=1（登录页与实际使用的
	// 提供方不一致），或半途失败后一条都不剩（SSO 登录整体不可用）。
	// 仓储层未暴露事务接口，这里显式记录原默认并在失败时回滚。
	prev, err := api.ssoRepo.GetUsed()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if err := api.ssoRepo.UnsetAllUsed(); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if err := api.ssoRepo.SetUsed(id); err != nil {
		if prev != nil {
			if restoreErr := api.ssoRepo.SetUsed(prev.ID); restoreErr != nil {
				logx.Errorf("[SSO] 恢复原默认认证配置失败 id=%d: %v", prev.ID, restoreErr)
			}
		}
		response.ServerErrorSafe(c, err)
		return
	}
	// SetUsed 影响 0 行时不报错（id 不存在），需二次确认确实存在启用中的配置
	used, err := api.ssoRepo.GetUsed()
	if err != nil {
		if prev != nil {
			if restoreErr := api.ssoRepo.SetUsed(prev.ID); restoreErr != nil {
				logx.Errorf("[SSO] 恢复原默认认证配置失败 id=%d: %v", prev.ID, restoreErr)
			}
		}
		response.ServerErrorSafe(c, err)
		return
	}
	if used == nil {
		if prev != nil {
			if restoreErr := api.ssoRepo.SetUsed(prev.ID); restoreErr != nil {
				logx.Errorf("[SSO] 恢复原默认认证配置失败 id=%d: %v", prev.ID, restoreErr)
			}
		}
		response.NotFoundKey(c, response.MsgSSONotFound)
		return
	}

	response.SuccessWithMsg(c, nil, "设置成功")
}
