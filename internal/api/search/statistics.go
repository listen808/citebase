package search

import (
	"strconv"

	"github.com/gin-gonic/gin"
	modelsearch "github.com/listen808/citebase/internal/model/search"
	"github.com/listen808/citebase/internal/response"
	searchsvc "github.com/listen808/citebase/internal/service/search"
)

// searchMonitorResponse 索引监控页的响应。索引事实与检索统计分两组：
// 前者是 Bleve 与文件系统的客观状态，后者来自 aw_search_log 的聚合。
type searchMonitorResponse struct {
	Index struct {
		IsOpen      bool   `json:"is_open"`
		DocCount    int64  `json:"doc_count"`
		IndexSize   int64  `json:"index_size"`
		IndexPath   string `json:"index_path"`
		LastUpdated int64  `json:"last_updated"`
		Indexing    bool   `json:"indexing"`
	} `json:"index"`
	Statistics *modelsearch.SearchStatistics `json:"statistics"`
}

// GetSearchMonitor 索引监控页数据：索引事实 + 检索行为统计
// @Summary 搜索监控统计
// @Tags 搜索
// @Produce json
// @Param days query int false "趋势窗口天数，默认 7，上限 365"
// @Param hot_limit query int false "热门搜索词条数，默认 10，上限 50"
// @Success 200 {object} object
// @Router /api/v1/search/monitor [get]
//
// 为什么不并入公开的 /search/stats：这个响应既含索引目录的绝对路径（routes.go 里
// 已经确立"含文件系统路径的诊断信息仅限管理员"这条边界），也对 aw_search_log 做
// 聚合——那张表存的是查询原文和 user_id，属于行为数据，不该由可选鉴权端点对外提供。
func (api *SearchAPI) GetSearchMonitor(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	hotLimit, _ := strconv.Atoi(c.DefaultQuery("hot_limit", "10"))

	stats, err := api.analytics.Statistics(days, hotLimit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	indexStats, err := api.searchService.GetIndexStats()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	size, lastUpdated := searchsvc.IndexDiskFacts()

	api.statusMu.RLock()
	indexing := api.reindexStatus.IsRunning
	api.statusMu.RUnlock()

	var resp searchMonitorResponse
	resp.Statistics = stats
	resp.Index.IsOpen = boolFromStats(indexStats, "is_open")
	resp.Index.DocCount = int64FromStats(indexStats, "doc_count")
	resp.Index.IndexSize = size
	resp.Index.IndexPath = searchsvc.IndexPath()
	resp.Index.LastUpdated = lastUpdated
	resp.Index.Indexing = indexing

	response.Success(c, resp)
}

func boolFromStats(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

// int64FromStats 兼容 Bleve DocCount 在不同分支返回 int/int64 两种宽度
func int64FromStats(m map[string]interface{}, key string) int64 {
	switch v := m[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}
