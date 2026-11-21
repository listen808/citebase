package admin

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/response"
)

// ApiKeyAPI 开放平台密钥管理 API（G029）
type ApiKeyAPI struct {
	repo *adminrepo.ApiKeyRepository
}

// NewApiKeyAPI 创建密钥管理 API 实例
func NewApiKeyAPI() *ApiKeyAPI {
	return &ApiKeyAPI{repo: adminrepo.NewApiKeyRepository()}
}

// CreateApiKey 创建密钥，明文仅此一次返回
// @Summary 创建开放平台密钥
// @Tags 开放平台
// @Router /api/v1/open/api-keys [post]
func (api *ApiKeyAPI) CreateApiKey(c *gin.Context) {
	var req struct {
		AppName   string `json:"app_name" binding:"required"`
		RateLimit int    `json:"rate_limit"`
		ExpiresIn int    `json:"expires_in_days"` // 0 表示不过期
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKeyDetail(c, response.MsgParameterError, err.Error())
		return
	}
	ownerID, _ := middleware.GetUserID(c)
	rateLimit := req.RateLimit
	if rateLimit <= 0 {
		rateLimit = 60
	}
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		expiresAt = &t
	}
	rec, raw, err := api.repo.Create(req.AppName, ownerID, rateLimit, expiresAt)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{
		"id":         rec.ID,
		"app_name":   rec.AppName,
		"status":     rec.Status,
		"rate_limit": rec.RateLimit,
		"expires_at": rec.ExpiresAt,
		"api_key":    raw, // 仅此一次
		"created_at": rec.CreatedAt,
	})
}

// ListApiKeys 列出当前用户持有的密钥（管理员可见全部）
// @Summary 密钥列表
// @Tags 开放平台
// @Router /api/v1/open/api-keys [get]
func (api *ApiKeyAPI) ListApiKeys(c *gin.Context) {
	list, err := api.repo.FindAll()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	// 不泄露密钥密文
	type out struct {
		ID         int       `json:"id"`
		AppName    string    `json:"app_name"`
		Status     string    `json:"status"`
		RateLimit  int       `json:"rate_limit"`
		ExpiresAt  time.Time `json:"expires_at"`
		LastUsedAt time.Time `json:"last_used_at"`
		CreatedAt  time.Time `json:"created_at"`
	}
	items := make([]out, 0, len(list))
	for _, k := range list {
		items = append(items, out{
			ID: k.ID, AppName: k.AppName, Status: k.Status,
			RateLimit: k.RateLimit, ExpiresAt: k.ExpiresAt,
			LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
		})
	}
	response.Success(c, gin.H{"api_keys": items, "total": len(items)})
}

// RevokeApiKey 吊销密钥
// @Summary 吊销密钥
// @Tags 开放平台
// @Router /api/v1/open/api-keys/:id/revoke [post]
func (api *ApiKeyAPI) RevokeApiKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	if err := api.repo.Revoke(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "密钥已吊销")
}

// RegisterOpenAPIRoutes 注册开放平台对外路由（API Key 鉴权）
func RegisterOpenAPIRoutes(open *gin.RouterGroup) {
	api := NewApiKeyAPI()
	exportAPI := NewExportAPI()
	open.GET("/me", api.OpenMe)
	// 数据导出（G037）
	open.GET("/export/:module", exportAPI.OpenExportData)
}

// OpenMe 返回当前 API Key 对应的应用信息（验证鉴权链路）
// @Summary 开放平台应用信息
// @Tags 开放平台
// @Router /api/v1/open/me [get]
func (api *ApiKeyAPI) OpenMe(c *gin.Context) {
	info := middleware.GetAPIKeyInfo(c)
	if info == nil {
		response.Unauthorized(c, "unauthorized")
		return
	}
	response.Success(c, gin.H{
		"app_name": info.AppName,
		"owner_id": info.OwnerID,
		"key_id":   info.KeyID,
	})
}
