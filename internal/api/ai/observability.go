package ai

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/observability"
	"github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/response"
)

type ObservabilityAPI struct{}

func NewObservabilityAPI() *ObservabilityAPI {
	return &ObservabilityAPI{}
}

func (api *ObservabilityAPI) GetMetrics(c *gin.Context) {
	provider := observability.DefaultProvider()
	prefix := c.Query("prefix")
	metrics := provider.ListMetrics(prefix)
	response.Success(c, gin.H{
		"metrics": metrics,
	})
}

func (api *ObservabilityAPI) ListTraces(c *gin.Context) {
	provider := observability.DefaultProvider()
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 || limit > 200 {
		limit = 50
	}
	traces := provider.ListRecentTraces(limit)
	response.Success(c, gin.H{
		"traces": traces,
		"total":  len(traces),
	})
}

func (api *ObservabilityAPI) GetTrace(c *gin.Context) {
	provider := observability.DefaultProvider()
	traceID := c.Param("id")
	if traceID == "" {
		response.BadRequest(c, "trace id is required")
		return
	}
	trace, err := provider.GetTrace(traceID)
	if err != nil {
		response.NotFound(c, "trace not found")
		return
	}
	response.Success(c, gin.H{
		"trace": trace,
	})
}

func (api *ObservabilityAPI) GetDashboard(c *gin.Context) {
	provider := observability.DefaultProvider()

	metrics := provider.ListMetrics("ai_")

	var totalRequests float64
	var totalTokens float64
	var avgDuration float64
	var activeRequests float64
	var cacheHits float64
	var cacheMisses float64

	histData, hasHist := provider.GetHistogram(observability.MetricAIRequestDuration, map[string]string{})

	for _, m := range metrics {
		switch m.Name {
		case observability.MetricAIRequestsTotal:
			totalRequests += m.Value
		case observability.MetricAITokenUsageTotal:
			totalTokens += m.Value
		case observability.MetricAIActiveRequests:
			activeRequests += m.Value
		case observability.MetricAICacheHitRatio:
			if m.Labels["type"] == "hit" {
				cacheHits += m.Value
			} else if m.Labels["type"] == "miss" {
				cacheMisses += m.Value
			}
		}
	}

	if hasHist && histData.Count > 0 {
		avgDuration = histData.Sum / float64(histData.Count)
	}

	var cacheHitRatio float64
	if cacheHits+cacheMisses > 0 {
		cacheHitRatio = cacheHits / (cacheHits + cacheMisses) * 100
	}

	recentTraces := provider.ListRecentTraces(10)

	response.Success(c, gin.H{
		"summary": gin.H{
			"total_requests":  int64(totalRequests),
			"total_tokens":    int64(totalTokens),
			"avg_duration_ms": avgDuration,
			"active_requests": int64(activeRequests),
			"cache_hit_ratio": cacheHitRatio,
			"cache_hits":      int64(cacheHits),
			"cache_misses":    int64(cacheMisses),
		},
		"metrics":       metrics,
		"recent_traces": recentTraces,
		// P0-2：reranker 是否在跑真 cross-encoder 必须可见。默认配置下 endpoint 为空，
		// 会静默降级成打分模拟，排序调不好时无法区分"模型问题"还是"压根没 rerank"。
		"retrieval": gin.H{
			"reranker_mode":         rag.RerankerMode(),
			"rerank_fallback_total": rag.RerankFallbackTotal(),
		},
	})
}
