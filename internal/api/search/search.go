package search

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware"
	modelsearch "github.com/listen808/citebase/internal/model/search"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	searchrepo "github.com/listen808/citebase/internal/repository/search"
	"github.com/listen808/citebase/internal/response"
	searchsvc "github.com/listen808/citebase/internal/service/search"
	"github.com/listen808/citebase/internal/utils/config"
)

type SearchAPI struct {
	searchService *searchsvc.SearchService
	searchRepo    *searchrepo.SearchRepository
	analytics     *searchsvc.SearchAnalytics
	reindexStatus *ReindexStatus
	statusMu      sync.RWMutex
}

// ReindexStatus 重建索引状态
type ReindexStatus struct {
	IsRunning bool   `json:"is_running"`
	Progress  int    `json:"progress"` // 0-100
	Total     int    `json:"total"`
	Current   int    `json:"current"`
	Message   string `json:"message"`
	StartTime int64  `json:"start_time"`
}

func NewSearchAPI() *SearchAPI {
	return &SearchAPI{
		searchService: searchsvc.NewSearchService(),
		searchRepo:    searchrepo.NewSearchRepository(),
		analytics:     searchsvc.NewSearchAnalytics(),
		reindexStatus: &ReindexStatus{},
	}
}

// SearchDocuments 搜索文档
// @Summary 全文搜索文档
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索关键字，支持多关键字：A|B（OR）或 A B（AND）"
// @Param type query string false "搜索类型：all/title/content" default(all)
// @Param space_id query int false "空间 ID"
// @Param author_id query int false "作者用户 ID"
// @Param tag_name query string false "标签名"
// @Param time_from query int false "时间范围起始（Unix 时间戳）"
// @Param time_to query int false "时间范围结束（Unix 时间戳）"
// @Param sort_by query string false "排序方式：relevance(默认)/time_desc/time_asc"
// @Param limit query int false "结果数量" default(20)
// @Success 200 {array} object
// @Router /api/v1/search/fulltext [get]
func (api *SearchAPI) SearchDocuments(c *gin.Context) {
	// 系统设置「开启全文搜索」总开关（search 组）：显式配为 "0" 才算关闭，缺键/其他值都视为开启。
	// 只拦全文档检索这一条路，标题快搜（type=title）同走本 handler 一起被拦是符合开关语义的。
	if v, err := adminrepo.NewConfigRepository().GetValueByKey("fulltext_search_open"); err == nil && v == "0" {
		response.ErrorKey(c, http.StatusServiceUnavailable, response.MsgFulltextSearchDisabled)
		return
	}

	query := c.Query("query")
	if query == "" {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	start := time.Now()

	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	searchType := c.DefaultQuery("type", "all")
	authorID, _ := strconv.Atoi(c.Query("author_id"))
	tagName := c.Query("tag_name")
	timeFrom, _ := strconv.ParseInt(c.Query("time_from"), 10, 64)
	timeTo, _ := strconv.ParseInt(c.Query("time_to"), 10, 64)
	sortBy := c.DefaultQuery("sort_by", "relevance")

	// 验证搜索类型
	if searchType != "all" && searchType != "title" && searchType != "content" {
		searchType = "all"
	}

	// 验证排序方式
	if sortBy != "relevance" && sortBy != "time_desc" && sortBy != "time_asc" {
		sortBy = "relevance"
	}

	// 限制最大返回数量
	if limit <= 0 || limit > 500 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// 同义词自动扩展：将查询扩展为多变体 OR 查询
	expandedQuery := api.expandQueryForSearch(query)

	// 权限过滤前置：从 context 提取身份，调用 service 层统一权限函数
	uid, _ := middleware.GetUserID(c)
	roleID, _ := c.Get("role_id")
	var role int
	if r, ok := roleID.(int); ok {
		role = r
	}
	accessibleSpaceIDs, isSuperAdmin, _ := searchsvc.GetUserAccessibleSpaceIDs(uid, role)

	req := &modelsearch.SearchDocumentRequest{
		Query:    expandedQuery,
		SpaceID:  spaceID,
		Limit:    limit,
		Offset:   offset,
		Type:     searchType,
		AuthorID: authorID,
		TagName:  tagName,
		TimeFrom: timeFrom,
		TimeTo:   timeTo,
		SortBy:   sortBy,
	}
	// 非超管时注入空间过滤（超管不传，避免大量空间 ID 构造 DisjunctionQuery）
	if !isSuperAdmin {
		req.AllowedSpaceIDs = accessibleSpaceIDs
	}

	results, err := api.searchService.SearchDocuments(req)
	if err != nil {
		log.Errorf("[SearchAPI] Search error for query=%q: %v", query, err)
		// 返回具体错误（索引状态/初始化失败原因），便于前端排查
		response.ServerErrorSafe(c, err)
		return
	}

	// 权限过滤兜底：非超管时仍按空间过滤（防止 Bleve 查询遗漏）
	if !isSuperAdmin {
		allowedSet := make(map[int]bool, len(accessibleSpaceIDs))
		for _, id := range accessibleSpaceIDs {
			allowedSet[id] = true
		}
		filtered := make([]*modelsearch.SearchDocumentResult, 0, len(results))
		for _, r := range results {
			if allowedSet[r.SpaceID] {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// 记录口径：在权限过滤之后取结果数。一次检索命中 5 篇但用户一篇都无权访问时，
	// 用户看到的就是"没有结果"——无结果率要回答的正是这个问题，而不是索引里有没有。
	searchsvc.RecordSearch(searchsvc.SearchLogEntry{
		Query:       query,
		Source:      modelsearch.SearchSourceFulltext,
		Type:        searchType,
		SpaceID:     spaceID,
		UserID:      uid,
		ResultCount: len(results),
		LatencyMS:   time.Since(start).Milliseconds(),
	})

	response.Success(c, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// expandQueryForSearch 使用同义词扩展查询，合并为 OR 语法
func (api *SearchAPI) expandQueryForSearch(query string) string {
	expanded := expandQueryWithSynonyms(query)
	if len(expanded) <= 1 {
		return query
	}
	// 合并多变体为 OR 查询：var1|var2|var3
	return strings.Join(expanded, "|")
}

// GetIndexStats 获取索引统计
// @Summary 获取索引统计
// @Tags 搜索
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/search/stats [get]
func (api *SearchAPI) GetIndexStats(c *gin.Context) {
	stats, err := api.searchService.GetIndexStats()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, stats)
}

// ReindexAll 重新索引所有文档（异步执行）
// @Summary 重新索引所有文档
// @Tags 搜索
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/search/reindex [post]
func (api *SearchAPI) ReindexAll(c *gin.Context) {
	// 检查是否已经在运行
	api.statusMu.RLock()
	isRunning := api.reindexStatus.IsRunning
	api.statusMu.RUnlock()

	if isRunning {
		response.BadRequest(c, "reindex is already running")
		return
	}

	// 启动异步重建索引
	go func() {
		api.statusMu.Lock()
		api.reindexStatus.IsRunning = true
		api.reindexStatus.StartTime = time.Now().Unix()
		api.reindexStatus.Progress = 0
		api.reindexStatus.Current = 0
		api.reindexStatus.Total = 0
		api.reindexStatus.Message = "正在重建索引..."
		api.statusMu.Unlock()

		count, err := api.searchService.ReindexAllDocuments(func(current, total int) {
			progress := 0
			if total > 0 {
				progress = int(float64(current) / float64(total) * 100)
			}
			api.statusMu.Lock()
			api.reindexStatus.Current = current
			api.reindexStatus.Total = total
			api.reindexStatus.Progress = progress
			api.reindexStatus.Message = fmt.Sprintf("正在重建索引... (%d/%d)", current, total)
			api.statusMu.Unlock()
		})

		api.statusMu.Lock()
		if err != nil {
			log.Errorf("[SearchAPI] Reindex failed: %v", err)
			api.reindexStatus.Message = fmt.Sprintf("重建失败: %v", err)
		} else {
			api.reindexStatus.Message = fmt.Sprintf("重建完成，共 %d 个文档", count)
		}
		api.reindexStatus.Progress = 100
		api.reindexStatus.IsRunning = false
		api.statusMu.Unlock()
	}()

	response.SuccessWithMsg(c, gin.H{"message": "重建索引任务已启动"}, "reindex task started")
}

// GetReindexStatus 获取重建索引状态
// @Summary 获取重建索引状态
// @Tags 搜索
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/search/reindex/status [get]
func (api *SearchAPI) GetReindexStatus(c *gin.Context) {
	api.statusMu.RLock()
	status := *api.reindexStatus
	api.statusMu.RUnlock()
	response.Success(c, status)
}

// IndexDocument 索引单个文档（用于实时更新）
// @Summary 索引单个文档
// @Tags 搜索
// @Accept json
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {message} string
// @Router /api/v1/search/index/:id [post]
func (api *SearchAPI) IndexDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	// 从数据库获取文档并索引
	err = api.searchService.IndexDocumentByID(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, gin.H{"id": id}, "document indexed")
}

// SearchSuggest 搜索建议/自动补全
// @Summary 搜索建议
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索关键字"
// @Param limit query int false "结果数量" default(10)
// @Success 200 {array} object
// @Router /api/v1/search/suggest [get]
func (api *SearchAPI) SearchSuggest(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		response.Success(c, []interface{}{})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit > 20 {
		limit = 20
	}

	req := &modelsearch.SearchDocumentRequest{
		Query: query,
		Type:  searchsvc.SearchTypeTitle, // 自动补全只搜索标题
		Limit: limit,
	}

	results, err := api.searchService.SearchDocuments(req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 权限过滤
	if allowedSet := getUserAccessibleSpaceIDs(c, api.searchRepo); allowedSet != nil {
		filtered := make([]*modelsearch.SearchDocumentResult, 0, len(results))
		for _, r := range results {
			if allowedSet[r.SpaceID] {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}

	// 只返回标题用于自动补全
	suggestions := make([]string, 0, len(results))
	for _, r := range results {
		suggestions = append(suggestions, r.Name)
	}

	response.Success(c, suggestions)
}

// DebugIndexDocument 调试接口：检查指定文档的索引状态
// @Summary 调试：检查文档索引状态
// @Tags 搜索
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/search/debug/:id [get]
func (api *SearchAPI) DebugIndexDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	// 获取索引统计
	stats, _ := searchsvc.GetIndexStats()

	// 获取文档信息
	docInfo, _ := api.searchService.GetDocumentInfoForDebug(id)

	response.Success(c, gin.H{
		"index_stats": stats,
		"document":    docInfo,
		"debug_tip":   "如果 index_stats.is_open=false，请重启服务器。如果 document.content为空但 document.file_exists=true，请重新索引。",
	})
}

// InitIndexDiagnostic 诊断接口：强制初始化索引并返回状态
func (api *SearchAPI) InitIndexDiagnostic(c *gin.Context) {
	indexPath := config.AppConfig.Storage.IndexDir

	dirExists := false
	if info, err := os.Stat(filepath.Dir(indexPath)); err == nil && info.IsDir() {
		dirExists = true
	}
	indexDirExists := false
	if info, err := os.Stat(indexPath); err == nil && info.IsDir() {
		indexDirExists = true
	}

	stats, _ := searchsvc.GetIndexStats()

	// 如果索引未初始化，尝试强制初始化
	initResult := "already_initialized"
	if stats["is_open"] == false {
		log.Errorf("[SearchAPI] Force-initializing index at: %s", indexPath)
		if err := searchsvc.ForceInitIndex(indexPath); err != nil {
			initResult = fmt.Sprintf("init_failed: %v", err)
		} else {
			initResult = "init_succeeded"
			// 刷新 stats
			stats, _ = searchsvc.GetIndexStats()
		}
	}

	response.Success(c, gin.H{
		"index_path":        indexPath,
		"parent_dir_exists": dirExists,
		"index_dir_exists":  indexDirExists,
		"init_result":       initResult,
		"index_stats":       stats,
	})
}

// GetSynonyms 获取搜索同义词配置
// @Summary 获取搜索同义词
// @Tags 搜索
// @Produce json
// @Success 200 {object} object
// @Router /api/v1/search/synonyms [get]
func (api *SearchAPI) GetSynonyms(c *gin.Context) {
	synonyms := config.AppConfig.Search.Synonyms
	if synonyms == nil {
		synonyms = []config.SynonymGroup{}
	}

	synonymMap := make(map[string][]string)
	for _, group := range synonyms {
		if len(group.Words) < 2 {
			continue
		}
		for _, word := range group.Words {
			others := make([]string, 0, len(group.Words)-1)
			for _, w := range group.Words {
				if w != word {
					others = append(others, w)
				}
			}
			synonymMap[word] = others
		}
	}

	response.Success(c, gin.H{
		"synonyms":    synonyms,
		"synonym_map": synonymMap,
	})
}

// ExpandQueryWithSynonyms 使用同义词扩展搜索查询
// @Summary 同义词扩展查询
// @Tags 搜索
// @Produce json
// @Param query query string true "搜索关键字"
// @Success 200 {object} object
// @Router /api/v1/search/expand [get]
func (api *SearchAPI) ExpandQueryWithSynonyms(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		response.Success(c, gin.H{"original": query, "expanded": []string{}})
		return
	}

	expanded := expandQueryWithSynonyms(query)
	response.Success(c, gin.H{
		"original": query,
		"expanded": expanded,
	})
}

func expandQueryWithSynonyms(query string) []string {
	result := []string{query}
	synonyms := config.AppConfig.Search.Synonyms

	for _, group := range synonyms {
		if len(group.Words) < 2 {
			continue
		}
		for _, word := range group.Words {
			if !containsWord(query, word) {
				continue
			}
			for _, syn := range group.Words {
				if syn == word {
					continue
				}
				expanded := replaceWord(query, word, syn)
				if !containsString(result, expanded) {
					result = append(result, expanded)
				}
			}
		}
	}

	return result
}

func containsWord(text, word string) bool {
	return strings.Contains(text, word)
}

func replaceWord(text, old, newWord string) string {
	return strings.ReplaceAll(text, old, newWord)
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// GetTags 获取所有标签列表（用于搜索筛选）
// @Summary 获取标签列表
// @Tags 搜索
// @Produce json
// @Param keyword query string false "标签关键字"
// @Param limit query int false "返回数量" default(50)
// @Success 200 {array} string
// @Router /api/v1/search/tags [get]
func (api *SearchAPI) GetTags(c *gin.Context) {
	keyword := c.Query("keyword")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	tags, err := api.searchRepo.GetAllTags(keyword, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"tags": tags})
}

// getUserAccessibleSpaceIDs 获取用户可访问的空间 ID 集合
// 安全语义（fail-closed）：
//   - 出错或用户无任何可访问空间时返回空 map（非 nil），调用方过滤后会得到空结果
//   - 永不返回 nil，避免 `if allowedSet != nil` 模式跳过过滤导致越权
func getUserAccessibleSpaceIDs(c *gin.Context, repo *searchrepo.SearchRepository) map[int]bool {
	uid, _ := middleware.GetUserID(c)
	roleID, _ := c.Get("role_id")
	isSuperAdmin := false
	if uid != 0 {
		if r, ok := roleID.(int); ok && r == 1 {
			isSuperAdmin = true
		}
	}
	accessibleIDs, err := repo.FindAccessibleSpaceIDs(uid, isSuperAdmin)
	if err != nil || len(accessibleIDs) == 0 {
		// fail-closed：返回空 map，调用方过滤后为空结果
		return make(map[int]bool)
	}
	allowedSet := make(map[int]bool, len(accessibleIDs))
	for _, id := range accessibleIDs {
		allowedSet[id] = true
	}
	return allowedSet
}
