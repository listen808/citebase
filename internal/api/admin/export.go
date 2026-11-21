package admin

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// ExportAPI 数据导出 API（G037）
type ExportAPI struct {
	svc *adminsvc.ExportService
}

// NewExportAPI 创建导出 API
func NewExportAPI() *ExportAPI {
	return &ExportAPI{svc: adminsvc.NewExportService()}
}

// ExportData 批量导出数据（管理端，session 鉴权）
// @Summary 批量导出数据
// @Tags 数据导出
// @Param module path string true "导出模块 (documents|spaces|users)"
// @Param format query string false "导出格式 (csv|json)" default(csv)
// @Router /api/v1/stats/export/:module [get]
func (api *ExportAPI) ExportData(c *gin.Context) {
	module := c.Param("module")
	format := c.DefaultQuery("format", "csv")
	api.doExport(c, module, format)
}

// OpenExportData 批量导出数据（开放平台，API Key 鉴权）
// @Summary 批量导出数据（开放平台）
// @Tags 开放平台
// @Param module path string true "导出模块"
// @Param format query string false "导出格式" default(csv)
// @Router /api/v1/open/export/:module [get]
func (api *ExportAPI) OpenExportData(c *gin.Context) {
	module := c.Param("module")
	format := c.DefaultQuery("format", "csv")
	// 开放平台只凭 API Key 鉴权（密钥无权限位/scope，且可能发给第三方应用），
	// users 模块包含全量用户 PII（邮箱/角色等），不允许经该通道导出；
	// 用户目录导出必须走管理端 session + RBAC 的 ExportData。
	if module == "users" {
		response.ForbiddenKey(c, response.MsgOpenAPIUsersModuleDenied)
		return
	}
	api.doExport(c, module, format)
}

func (api *ExportAPI) doExport(c *gin.Context, module, format string) {
	switch format {
	case "csv":
		data, result, err := api.svc.ExportCSV(module)
		if err != nil {
			if strings.Contains(err.Error(), "unsupported module") {
				response.BadRequest(c, err.Error())
				return
			}
			response.ServerErrorSafe(c, err)
			return
		}
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+result.Filename)
		c.String(http.StatusOK, "\uFEFF"+string(data))
	case "json":
		data, result, err := api.svc.ExportJSON(module)
		if err != nil {
			if strings.Contains(err.Error(), "unsupported module") {
				response.BadRequest(c, err.Error())
				return
			}
			response.ServerErrorSafe(c, err)
			return
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Header("Content-Disposition", "attachment; filename="+result.Filename)
		c.Data(http.StatusOK, "application/json; charset=utf-8", data)
	default:
		response.BadRequest(c, "unsupported format: "+format+" (use csv or json)")
	}
}
