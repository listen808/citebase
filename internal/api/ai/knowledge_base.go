package ai

import (
	"net/http"
	"strconv"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
)

// KnowledgeBaseAPI 知识库管理 API — 提供知识库列表、分块统计、召回测试、统计信息
type KnowledgeBaseAPI struct{}

// NewKnowledgeBaseAPI 创建知识库管理 API 实例
func NewKnowledgeBaseAPI() *KnowledgeBaseAPI {
	return &KnowledgeBaseAPI{}
}

// knowledgeBaseItem 知识库列表项
type knowledgeBaseItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	DocCount    int64  `json:"doc_count"`
	TotalWords  int64  `json:"total_words"`
	LastUpdated string `json:"last_updated"`
	Status      int    `json:"status"`
}

// ListBases GET /api/v1/ai/knowledge-bases — 列出知识库（空间 + 文档统计）
func (api *KnowledgeBaseAPI) ListBases(c *gin.Context) {
	// 只列出调用者可见的空间：此前直接查全表，等于向任意登录用户暴露所有私有空间
	visibleIDs, unrestricted, err := accessibleSpaceIDs(c)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if !unrestricted && len(visibleIDs) == 0 {
		response.Success(c, gin.H{"items": []knowledgeBaseItem{}, "total": 0})
		return
	}

	q := repository.DB.Where("is_delete = 0 AND status = 1")
	if !unrestricted {
		q = q.Where("space_id IN ?", visibleIDs)
	}

	var spaces []workspacemodel.Space
	if err := q.Order("update_time DESC").Find(&spaces).Error; err != nil {
		logx.Infof("[KnowledgeBase] ListBases query error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询知识库列表失败")
		return
	}

	items := make([]knowledgeBaseItem, 0, len(spaces))
	spaceIDs := make([]int, 0, len(spaces))
	for _, s := range spaces {
		spaceIDs = append(spaceIDs, s.ID)
	}
	// 一条 GROUP BY 取回全部文档数：原先在 for 里逐空间 Count，
	// 既是不必要的 N 次查询，又是"查询失败就显示 0"的那一处 —— 数字看着正常，
	// 其实是数据库错误被吞掉了。
	docCounts, err := workspacerepo.NewDocumentRepository().CountBySpaceIDsGrouped(spaceIDs)
	if err != nil {
		logx.Infof("[KnowledgeBase] ListBases count error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计知识库文档数失败")
		return
	}

	for _, s := range spaces {
		lastUpdated := ""
		if s.UpdateTime > 0 {
			lastUpdated = time.Unix(int64(s.UpdateTime), 0).Format(time.RFC3339)
		}

		items = append(items, knowledgeBaseItem{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
			DocCount:    docCounts[s.ID],
			LastUpdated: lastUpdated,
			Status:      s.Status,
		})
	}

	response.Success(c, gin.H{"knowledge_bases": items, "total": len(items)})
}

// docChunkInfo 文档分块信息
type docChunkInfo struct {
	DocID        int    `json:"doc_id"`
	DocName      string `json:"doc_name"`
	Type         int    `json:"type"`
	UpdateTime   string `json:"update_time"`
	HasVector    bool   `json:"has_vector"`
	VectorStatus string `json:"vector_status"`
}

// ListChunks GET /api/v1/ai/knowledge-bases/:id/chunks — 查看知识库的分块情况
func (api *KnowledgeBaseAPI) ListChunks(c *gin.Context) {
	spaceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的知识库 ID")
		return
	}

	// 分块内容就是文档正文切片：必须确认该空间在调用者可见范围内
	visibleIDs, unrestricted, err := accessibleSpaceIDs(c)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if !canAccessSpace(spaceID, visibleIDs, unrestricted) {
		response.ForbiddenKey(c, response.MsgNoPermissionKBChunks)
		return
	}

	var docs []workspacemodel.Document
	if err := repository.DB.Where("space_id = ? AND is_delete = 0", spaceID).
		Order("update_time DESC").Find(&docs).Error; err != nil {
		logx.Infof("[KnowledgeBase] ListChunks query error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询文档列表失败")
		return
	}

	// 查询向量同步状态
	vectorStatuses, err := knowledgerepo.NewVectorSyncTaskRepository().ListDocStatusBySpace(spaceID)
	if err != nil {
		logx.Infof("[KnowledgeBase] ListChunks vector status query error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询向量同步状态失败")
		return
	}

	// 建立 doc_id -> latest status 映射
	vsMap := make(map[int]string)
	for _, vs := range vectorStatuses {
		if _, exists := vsMap[vs.DocID]; !exists {
			vsMap[vs.DocID] = vs.Status
		}
	}

	items := make([]docChunkInfo, 0, len(docs))
	for _, d := range docs {
		updateTime := ""
		if d.UpdateTime > 0 {
			updateTime = time.Unix(int64(d.UpdateTime), 0).Format(time.RFC3339)
		}
		vs, hasVS := vsMap[d.ID]
		if !hasVS {
			vs = "none"
		}
		items = append(items, docChunkInfo{
			DocID:        d.ID,
			DocName:      d.Name,
			Type:         d.Type,
			UpdateTime:   updateTime,
			HasVector:    hasVS && vs == "completed",
			VectorStatus: vs,
		})
	}

	// 统计汇总
	totalDocs, err := workspacerepo.NewDocumentRepository().CountBySpaceAndType(spaceID, workspacemodel.DocumentTypePage)
	if err != nil {
		logx.Infof("[KnowledgeBase] ListChunks count error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计文档数失败")
		return
	}

	var vectorizedCount int64
	for _, vs := range vsMap {
		if vs == "completed" {
			vectorizedCount++
		}
	}

	response.Success(c, gin.H{
		"documents":        items,
		"total_docs":       totalDocs,
		"vectorized_count": vectorizedCount,
	})
}

// testRetrievalRequest 召回测试请求
type testRetrievalRequest struct {
	Query string `json:"query" binding:"required"`
	TopK  int    `json:"top_k"`
}

// TestRetrieval POST /api/v1/ai/knowledge-bases/:id/test-retrieval — 召回测试
func (api *KnowledgeBaseAPI) TestRetrieval(c *gin.Context) {
	spaceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的知识库 ID")
		return
	}

	// 召回测试会原样返回命中片段正文：不校验空间可见性即等于按空间读取任意知识库内容
	visibleIDs, unrestricted, err := accessibleSpaceIDs(c)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if !canAccessSpace(spaceID, visibleIDs, unrestricted) {
		response.ForbiddenKey(c, response.MsgNoPermissionKBRecallTest)
		return
	}

	var req testRetrievalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效，需要 query 字段")
		return
	}
	if req.TopK <= 0 {
		req.TopK = 5
	}
	if req.TopK > 20 {
		req.TopK = 20
	}

	// 尝试使用真实 retriever
	m := aimod.GetModule()
	retriever := m.Retriever()
	if retriever != nil {
		sources, _ := retriever.RetrieveContextOnly(req.Query, spaceID, req.TopK)
		results := make([]gin.H, 0, len(sources))
		for i, s := range sources {
			results = append(results, gin.H{
				"index":         i + 1,
				"document_id":   s.DocumentID,
				"document_name": s.DocumentName,
				"score":         s.Score,
				"snippet":       s.Snippet,
				"heading_path":  s.HeadingPath,
				"source_type":   s.SourceType,
			})
		}
		response.Success(c, gin.H{
			"query":        req.Query,
			"space_id":     spaceID,
			"top_k":        req.TopK,
			"results":      results,
			"result_count": len(results),
			"method":       "vector",
		})
		return
	}

	// 降级：使用全文搜索模拟
	var docs []workspacemodel.Document
	query := "%" + req.Query + "%"
	if err := repository.DB.Where("space_id = ? AND is_delete = 0 AND name LIKE ?", spaceID, query).
		Limit(req.TopK).Find(&docs).Error; err != nil {
		logx.Infof("[KnowledgeBase] TestRetrieval fallback query error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "检索失败")
		return
	}

	results := make([]gin.H, 0, len(docs))
	for i, d := range docs {
		results = append(results, gin.H{
			"index":         i + 1,
			"document_id":   d.ID,
			"document_name": d.Name,
			"score":         0.0,
			"snippet":       "",
			"heading_path":  "",
			"source_type":   "fulltext_fallback",
		})
	}

	response.Success(c, gin.H{
		"query":        req.Query,
		"space_id":     spaceID,
		"top_k":        req.TopK,
		"results":      results,
		"result_count": len(results),
		"method":       "fulltext_fallback",
	})
}

// knowledgeBaseStats 知识库统计信息
type knowledgeBaseStats struct {
	SpaceID          int     `json:"space_id"`
	SpaceName        string  `json:"space_name"`
	TotalDocs        int64   `json:"total_docs"`
	TotalPages       int64   `json:"total_pages"`
	TotalDirs        int64   `json:"total_dirs"`
	VectorizedCount  int64   `json:"vectorized_count"`
	PendingVectors   int64   `json:"pending_vectors"`
	FailedVectors    int64   `json:"failed_vectors"`
	VectorCompletion float64 `json:"vector_completion"`
	LastVectorSync   string  `json:"last_vector_sync"`
}

// GetStats GET /api/v1/ai/knowledge-bases/:id/stats — 知识库统计信息
func (api *KnowledgeBaseAPI) GetStats(c *gin.Context) {
	spaceID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的知识库 ID")
		return
	}

	// 统计的是私有空间的文档数与向量化进度：同族另外三个入口
	// （ListBases / ListChunks / TestRetrieval）都做了可见性校验，这里漏了。
	visibleIDs, unrestricted, err := accessibleSpaceIDs(c)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	if !canAccessSpace(spaceID, visibleIDs, unrestricted) {
		response.ForbiddenKey(c, response.MsgNoPermissionKBStats)
		return
	}

	// 查询空间基本信息
	var space workspacemodel.Space
	if err := repository.DB.Where("space_id = ? AND is_delete = 0", spaceID).First(&space).Error; err != nil {
		response.Error(c, http.StatusNotFound, 404, "知识库不存在")
		return
	}

	docRepo := workspacerepo.NewDocumentRepository()
	syncRepo := knowledgerepo.NewVectorSyncTaskRepository()

	// 文档统计：每个数字都会决定页面上的进度环，取不到就是取不到，
	// 不能像原先那样让 Count 的错误静默成 0 —— 那会把"数据库报错"渲染成"这个知识库是空的"。
	totalDocs, err := docRepo.CountBySpaceID(spaceID)
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats totalDocs error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计文档数失败")
		return
	}
	totalPages, err := docRepo.CountBySpaceAndType(spaceID, workspacemodel.DocumentTypePage)
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats totalPages error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计页面数失败")
		return
	}
	totalDirs, err := docRepo.CountBySpaceAndType(spaceID, workspacemodel.DocumentTypeDir)
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats totalDirs error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计目录数失败")
		return
	}

	// 向量同步统计
	vectorizedCount, err := syncRepo.CountDistinctDocs(spaceID, "completed")
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats vectorizedCount error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计向量化进度失败")
		return
	}
	failedCount, err := syncRepo.CountDistinctDocs(spaceID, "failed")
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats failedCount error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "统计向量化失败数失败")
		return
	}
	// 实际 pending = 总数 - 已完成
	pendingRemaining := totalDocs - vectorizedCount
	if pendingRemaining < 0 {
		pendingRemaining = 0
	}

	// 最后同步时间
	lastSync, err := syncRepo.LatestCompletedAt(spaceID)
	if err != nil {
		logx.Infof("[KnowledgeBase] GetStats lastSync error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询最近同步时间失败")
		return
	}

	lastSyncStr := ""
	if lastSync.Valid {
		lastSyncStr = lastSync.Time.Format(time.RFC3339)
	}

	completion := float64(0)
	if totalDocs > 0 {
		completion = float64(vectorizedCount) / float64(totalDocs) * 100
	}

	stats := knowledgeBaseStats{
		SpaceID:          spaceID,
		SpaceName:        space.Name,
		TotalDocs:        totalDocs,
		TotalPages:       totalPages,
		TotalDirs:        totalDirs,
		VectorizedCount:  vectorizedCount,
		PendingVectors:   pendingRemaining,
		FailedVectors:    failedCount,
		VectorCompletion: completion,
		LastVectorSync:   lastSyncStr,
	}

	response.Success(c, stats)
}
