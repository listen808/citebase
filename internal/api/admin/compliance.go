package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// ComplianceAPI 合规报告 API
type ComplianceAPI struct {
	reportSvc *adminsvc.ComplianceReportService
}

// NewComplianceAPI 创建合规报告 API 实例
func NewComplianceAPI() *ComplianceAPI {
	return &ComplianceAPI{reportSvc: adminsvc.NewComplianceReportService()}
}

// GetComplianceReport 获取合规报告（JSON）
// @Summary 合规报告
// @Tags 统计监控
// @Router /api/v1/stats/compliance-report [get]
func (api *ComplianceAPI) GetComplianceReport(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	rep, err := api.reportSvc.GenerateReport(c.Request.Context(), days)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, rep)
}

// ExportComplianceReport 导出合规报告（CSV/JSON）
// @Summary 导出合规报告
// @Tags 统计监控
// @Router /api/v1/stats/compliance-report/export [get]
func (api *ComplianceAPI) ExportComplianceReport(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 365 {
		days = 30
	}
	rep, err := api.reportSvc.GenerateReport(c.Request.Context(), days)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	format := c.DefaultQuery("format", "csv")
	if format == "json" {
		out, err := api.reportSvc.ExportJSON(rep)
		if err != nil {
			response.ServerErrorSafe(c, err)
			return
		}
		c.Header("Content-Disposition", "attachment; filename=compliance-report.json")
		c.Data(http.StatusOK, "application/json", []byte(out))
		return
	}
	out, err := api.reportSvc.ExportCSV(rep)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	c.Header("Content-Disposition", "attachment; filename=compliance-report.csv")
	c.Data(http.StatusOK, "text/csv", []byte(out))
}
