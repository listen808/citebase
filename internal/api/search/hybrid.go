package search

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware"
	modelsearch "github.com/listen808/citebase/internal/model/search"
	searchrepo "github.com/listen808/citebase/internal/repository/search"
	"github.com/listen808/citebase/internal/response"
	searchsvc "github.com/listen808/citebase/internal/service/search"
)

// HybridSearch handles POST /api/v1/search/hybrid
func (api *SearchAPI) HybridSearch(c *gin.Context) {
	start := time.Now()
	var req searchsvc.HybridSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}

	if req.Query == "" {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	// Build the hybrid search service
	bleveAdapter := searchsvc.NewBleveAdapter()

	var vectorAdapter *searchsvc.VectorAdapter
	aiModule := ai.GetModule()
	if aiModule != nil && aiModule.Embedder() != nil {
		vsi := embedding.GetVectorStoreInterface()
		if vsi != nil {
			vectorAdapter = searchsvc.NewVectorAdapter(aiModule.Embedder(), vsi)
		}
	}

	cfg := searchsvc.DefaultHybridSearchConfig()
	hybridSvc := searchsvc.NewHybridSearchService(bleveAdapter, vectorAdapter, cfg, searchrepo.NewSearchRepository())

	// P0-1/P1 前置权限过滤：从 context 提取身份，调用 service 层统一权限函数，
	// 与 search.go /search/fulltext 保持同一套语义。
	uid, _ := middleware.GetUserID(c)
	roleID, _ := c.Get("role_id")
	var role int
	if r, ok := roleID.(int); ok {
		role = r
	}
	accessibleSpaceIDs, isSuperAdmin, _ := searchsvc.GetUserAccessibleSpaceIDs(uid, role)
	var allowedSpaceIDs []uint
	if !isSuperAdmin {
		allowedSpaceIDs = searchsvc.IntersectSpaceIDs(req.SpaceIDs, accessibleSpaceIDs)
	}
	req.AllowedSpaceIDs = allowedSpaceIDs

	results, total, executedMode, err := hybridSvc.Search(c.Request.Context(), &req)
	if err != nil {
		log.Errorf("[SearchAPI] HybridSearch error for query=%q: %v", req.Query, err)
		response.ServerErrorSafe(c, err)
		return
	}

	// 混合检索的请求体是 space_ids 数组，只在恰好指定一个空间时记录 space_id，
	// 跨空间检索记 0（与全文检索"未限定空间"同一语义），不臆造一个代表空间。
	spaceID := 0
	if len(req.SpaceIDs) == 1 {
		spaceID = int(req.SpaceIDs[0])
	}
	uid, _ = middleware.GetUserID(c)
	// P3: Type 字段记录实际执行模式（intent 检测后确定），而非请求中的 search_mode。
	searchsvc.RecordSearch(searchsvc.SearchLogEntry{
		Query:       req.Query,
		Source:      modelsearch.SearchSourceHybrid,
		Type:        executedMode,
		SpaceID:     spaceID,
		UserID:      uid,
		ResultCount: total,
		LatencyMS:   time.Since(start).Milliseconds(),
	})

	response.Success(c, gin.H{
		"results": results,
		"total":   total,
	})
}