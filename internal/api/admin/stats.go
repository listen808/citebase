package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// StatsAPI 统计与监控 API
type StatsAPI struct {
	statsService *adminsvc.StatsService
}

func NewStatsAPI() *StatsAPI {
	return &StatsAPI{
		statsService: adminsvc.NewStatsService(),
	}
}

// GetDashboardStats 获取仪表盘统计数据
// @Summary 仪表盘统计
// @Tags 统计监控
// @Produce json
// @Param days query int false "天数(默认10)"
// @Success 200
// @Router /api/v1/stats/dashboard [get]
func (api *StatsAPI) GetDashboardStats(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "10")
	days := 10
	if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
		days = d
	}
	stats, err := api.statsService.GetDashboardStats(days)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, stats)
}

// GetServerStatus 获取服务器运行状态
// @Summary 服务器状态
// @Tags 统计监控
// @Produce json
// @Success 200
// @Router /api/v1/stats/server [get]
func (api *StatsAPI) GetServerStatus(c *gin.Context) {
	status, err := api.statsService.GetServerStatus()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, status)
}

// GetServerTime 获取服务器时间
// @Summary 服务器时间
// @Tags 统计监控
// @Produce json
// @Success 200
// @Router /api/v1/stats/time [get]
func (api *StatsAPI) GetServerTime(c *gin.Context) {
	timeInfo := api.statsService.GetServerTime()
	response.Success(c, timeInfo)
}

// GetCollectDocRank 获取文档收藏排行
// @Summary 文档收藏排行
// @Tags 统计监控
// @Produce json
// @Param limit query int false "数量(默认20)"
// @Success 200
// @Router /api/v1/stats/collect-doc-rank [get]
func (api *StatsAPI) GetCollectDocRank(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit := 20
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	items, err := api.statsService.GetCollectDocRank(limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, items)
}
