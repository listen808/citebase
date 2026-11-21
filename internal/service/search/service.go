package search

import (
	modelsearch "github.com/listen808/citebase/internal/model/search"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	searchrepo "github.com/listen808/citebase/internal/repository/search"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"

	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	logx "github.com/listen808/citebase/internal/log"
)

// SearchService 搜索服务层
type SearchService struct {
	searchRepo *searchrepo.SearchRepository
	docStorage *utils.DocumentStorage
}

// NewSearchService 创建搜索服务实例
func NewSearchService() *SearchService {
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}
	return &SearchService{
		searchRepo: searchrepo.NewSearchRepository(),
		docStorage: utils.NewDocumentStorageWithDir(docDir),
	}
}

// SearchDocuments 搜索文档
func (s *SearchService) SearchDocuments(req *modelsearch.SearchDocumentRequest) ([]*modelsearch.SearchDocumentResult, error) {
	searchType := req.Type
	if searchType == "" {
		searchType = SearchTypeAll
	}

	// 预获取过滤条件的文档 ID 集合（author/tag），用于在搜索结果中过滤
	var authorIDSet, tagIDSet map[int]bool
	var filterErr error
	if req.AuthorID > 0 {
		authorIDSet, filterErr = s.searchRepo.FindDocumentIDsByAuthor(req.AuthorID)
		if filterErr != nil {
			logx.Infof("[SearchService] FindDocumentIDsByAuthor error: %v", filterErr)
		}
	}
	if req.TagName != "" {
		tagIDSet, filterErr = s.searchRepo.FindDocumentIDsByTag(req.TagName)
		if filterErr != nil {
			logx.Infof("[SearchService] FindDocumentIDsByTag error: %v", filterErr)
		}
	}

	// 有过滤条件时扩大搜索范围，避免过滤后结果过少
	searchLimit := req.Limit
	if req.AuthorID > 0 || req.TagName != "" || req.TimeFrom > 0 || req.TimeTo > 0 {
		if searchLimit > 0 && searchLimit < 100 {
			searchLimit = searchLimit * 3
		}
	}

	searchOpts := &SearchOptions{
		Query:    req.Query,
		Type:     searchType,
		SpaceID:  req.SpaceID,
		SpaceIDs: req.AllowedSpaceIDs,
		Limit:    searchLimit,
		Offset:   req.Offset,
	}

	result, err := SearchDocumentsWithOptions(searchOpts)

	if err != nil {
		logx.Infof("[SearchService] SearchDocumentsWithOptions error: %v", err)
		return nil, err
	}

	if result == nil || result.Hits == nil {
		return []*modelsearch.SearchDocumentResult{}, nil
	}

	// 预取所有 hits 涉及的文档（docID + path 父节点 ID），一次批量查询替代主循环中的 N+1
	docMap := make(map[int]*workspacemodel.Document)
	if len(result.Hits) > 0 {
		idSet := make(map[int]bool)
		for _, hit := range result.Hits {
			if hit == nil || hit.Fields == nil {
				continue
			}
			// 解析 hit 的 docID
			var docID int
			if id, ok := hit.Fields["id"].(float64); ok {
				docID = int(id)
			}
			if docID == 0 {
				if parsed, err := strconv.Atoi(hit.ID); err == nil {
					docID = parsed
				}
			}
			if docID > 0 {
				idSet[docID] = true
			}
			// 解析 path 中的父节点 ID
			if path, ok := hit.Fields["path"].(string); ok && path != "" && path != "0" {
				for _, part := range strings.Split(path, ",") {
					part = strings.TrimSpace(part)
					if part == "" || part == "0" {
						continue
					}
					pid := 0
					fmt.Sscanf(part, "%d", &pid)
					if pid > 0 {
						idSet[pid] = true
					}
				}
			}
		}
		if len(idSet) > 0 {
			allIDs := make([]int, 0, len(idSet))
			for id := range idSet {
				allIDs = append(allIDs, id)
			}
			docs, err := s.searchRepo.FindDocumentsByIDs(allIDs)
			if err != nil {
				// 旧实现吞掉此错误后 docMap 为空，草稿过滤整体失效——
				// DB 抖动时未发布文档会原样出现在搜索结果里。宁可报错，不可泄露。
				logx.Errorf("[SearchService] 预取文档失败，放弃本次搜索结果: %v", err)
				return nil, err
			}
			for _, d := range docs {
				docMap[d.ID] = d
			}
		}
	}

	seen := make(map[int]int)
	var uniqueResults []*modelsearch.SearchDocumentResult

	spaceNames := make(map[int]string)

	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}

		var docID int
		if hit.Fields != nil {
			if id, ok := hit.Fields["id"].(float64); ok {
				docID = int(id)
			}
		}
		if docID == 0 {
			if parsed, err := strconv.Atoi(hit.ID); err == nil {
				docID = parsed
			}
		}
		if docID == 0 {
			continue
		}

		docResult := &modelsearch.SearchDocumentResult{
			DocumentID: docID,
			Score:      hit.Score,
			Highlights: make(map[string]string),
		}

		if hit.Fields != nil {
			if spaceID, ok := hit.Fields["space_id"].(float64); ok {
				docResult.SpaceID = int(spaceID)
				if docResult.SpaceID > 0 {
					if name, ok := spaceNames[docResult.SpaceID]; ok {
						docResult.SpaceName = name
					} else {
						if space, err := s.searchRepo.FindSpaceByID(docResult.SpaceID); err == nil && space != nil {
							spaceNames[docResult.SpaceID] = space.Name
							docResult.SpaceName = space.Name
						}
					}
				}
			}
			if name, ok := hit.Fields["name"].(string); ok {
				docResult.Name = name
			}
			if docType, ok := hit.Fields["type"].(float64); ok {
				docResult.Type = int(docType)
			}
			if path, ok := hit.Fields["path"].(string); ok {
				docResult.Path = s.resolvePath(path, docResult.SpaceID, docMap)
				docResult.PathIDs = parsePathIDs(path)
			}
		}

		// 从预取的 docMap 填充 ParentID
		if doc, ok := docMap[docID]; ok && doc != nil {
			docResult.ParentID = doc.ParentID
		}

		if hit.Fragments != nil {
			if fragments, ok := hit.Fragments["name"]; ok && len(fragments) > 0 {
				docResult.Highlights["name"] = fragments[0]
			}
			if fragments, ok := hit.Fragments["content"]; ok && len(fragments) > 0 {
				docResult.Highlights["content"] = fragments[0]
			}
		}

		// 提取多片段高亮（从 hit.Locations）
		if hit.Locations != nil {
			docResult.MatchCount = CountMatches(hit.Locations)
			docResult.HighlightFragments = make(map[string][]modelsearch.HighlightFragment)

			// 从 content 字段提取片段
			if contentLocs, ok := hit.Locations["content"]; ok && len(contentLocs) > 0 {
				// 读取文档内容用于片段提取（从预取的 docMap 取，避免 N+1）
				content := ""
				if doc, ok := docMap[docID]; ok && doc != nil {
					if doc.Type == workspacemodel.DocumentTypePage {
						content = s.readDocumentContent(doc, docMap)
					}
				}
				if content != "" {
					fragments := ExtractFragments(contentLocs, content, 150, 3)
					if len(fragments) > 0 {
						docResult.HighlightFragments["content"] = fragments
					}
				}
			}

			// 从 name 字段提取片段
			if nameLocs, ok := hit.Locations["name"]; ok && len(nameLocs) > 0 {
				fragments := ExtractFragments(nameLocs, docResult.Name, 150, 3)
				if len(fragments) > 0 {
					docResult.HighlightFragments["name"] = fragments
				}
			}
		}

		if idx, exists := seen[docID]; exists {
			if hit.Score > uniqueResults[idx].Score {
				uniqueResults[idx] = docResult
			}
		} else {
			seen[docID] = len(uniqueResults)
			uniqueResults = append(uniqueResults, docResult)
		}
	}

	// 回填 CreateTime（docMap 已在主循环之前预取，这里直接复用）
	for _, r := range uniqueResults {
		if doc, ok := docMap[r.DocumentID]; ok {
			r.CreateTime = int64(doc.CreateTime)
		}
	}

	// #7 过滤草稿文档（公开搜索不返回草稿）
	filtered := filterDraftsByDocMap(uniqueResults, docMap)

	// 应用过滤条件：作者/标签/时间范围
	if authorIDSet != nil {
		filtered = filterByDocIDSet(filtered, authorIDSet)
	}
	if tagIDSet != nil {
		filtered = filterByDocIDSet(filtered, tagIDSet)
	}
	if req.TimeFrom > 0 || req.TimeTo > 0 {
		filtered = filterByTimeRange(filtered, req.TimeFrom, req.TimeTo)
	}

	// 应用排序
	if req.SortBy == "time_desc" {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreateTime > filtered[j].CreateTime
		})
	} else if req.SortBy == "time_asc" {
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].CreateTime < filtered[j].CreateTime
		})
	}
	// 默认 relevance 按 Bleve score 排序，uniqueResults 已按 score 降序

	// 截断到请求的 limit
	if req.Limit > 0 && len(filtered) > req.Limit {
		filtered = filtered[:req.Limit]
	}

	return filtered, nil
}

// filterByDocIDSet 按文档 ID 集合过滤结果
func filterByDocIDSet(results []*modelsearch.SearchDocumentResult, idSet map[int]bool) []*modelsearch.SearchDocumentResult {
	if idSet == nil {
		return results
	}
	filtered := make([]*modelsearch.SearchDocumentResult, 0, len(results))
	for _, r := range results {
		if idSet[r.DocumentID] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// filterDraftsByDocMap 过滤草稿文档（公开搜索不返回草稿）。
// 状态无法判定的命中（文档已删除/索引残留，docMap 查不到）同样保守剔除，
// 避免删除过的私有正文仍能通过检索结果读到。
func filterDraftsByDocMap(results []*modelsearch.SearchDocumentResult, docMap map[int]*workspacemodel.Document) []*modelsearch.SearchDocumentResult {
	filtered := make([]*modelsearch.SearchDocumentResult, 0, len(results))
	for _, r := range results {
		doc, ok := docMap[r.DocumentID]
		if !ok || doc == nil {
			continue
		}
		if doc.Status == workspacemodel.DocumentStatusDraft {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// filterByTimeRange 按时间范围过滤结果
func filterByTimeRange(results []*modelsearch.SearchDocumentResult, timeFrom, timeTo int64) []*modelsearch.SearchDocumentResult {
	filtered := make([]*modelsearch.SearchDocumentResult, 0, len(results))
	for _, r := range results {
		if timeFrom > 0 && r.CreateTime < timeFrom {
			continue
		}
		if timeTo > 0 && r.CreateTime > timeTo {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// ============================================================
// 自动索引：文档变更时同步 Bleve 索引
// ============================================================

// readDocumentContent 读取文档正文（file/db 由 storage.content_source 决定）
func (s *SearchService) readDocumentContent(doc *workspacemodel.Document, docMap map[int]*workspacemodel.Document) string {
	pageFile := getPageFilePath(doc, s.searchRepo, s.docStorage, docMap)
	content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
	if err != nil || content == "" {
		return ""
	}
	return content
}

// IndexDocument 索引单个文档（自动读取文件系统内容）
func (s *SearchService) IndexDocument(doc *workspacemodel.Document) error {
	// 草稿不索引：避免索引膨胀和搜索后过滤的开销
	if doc.Status == workspacemodel.DocumentStatusDraft {
		return s.DeleteDocumentIndex(doc.ID)
	}

	if doc.Type != workspacemodel.DocumentTypePage {
		if err := IndexDocumentByID(doc.ID, doc.SpaceID, doc.Name, "", doc.Type, doc.Path); err != nil {
			logx.Infof("[WARN] failed to index directory %d: %v", doc.ID, err)
		}
		return nil
	}

	content := s.readDocumentContent(doc, nil)
	if content == "" {
		latestVersion, err := s.searchRepo.GetLatestVersion(doc.ID)
		if err == nil && latestVersion > 0 {
			versionContent, err := s.searchRepo.GetVersionContent(doc.ID, latestVersion)
			if err == nil {
				content = versionContent
			}
		}
	}

	err := IndexDocumentByID(doc.ID, doc.SpaceID, doc.Name, content, doc.Type, doc.Path)
	if err != nil {
		logx.Infof("[SearchService] Failed to index document id=%d: %v", doc.ID, err)
		return err
	}

	return nil
}

// IndexDocumentByID 根据文档 ID 索引
func (s *SearchService) IndexDocumentByID(docID int) error {
	doc, err := s.searchRepo.FindDocumentByID(docID)
	if err != nil {
		return err
	}
	return s.IndexDocument(doc)
}

// DeleteDocumentIndex 删除文档索引
func (s *SearchService) DeleteDocumentIndex(docID int) error {
	docIDStr := strconv.Itoa(docID)
	return DeleteDocument(docIDStr)
}

// ============================================================
// 重建索引（全量重建：从数据库+文件系统重建）
// ============================================================

// reindexMu 串行化全量重建：启动自动重建与管理台手动重建可能并发触发，
// 并发 ResetIndex 会 close 对方正写入的索引实例，导致索引数据损坏（写入丢失）。
var reindexMu sync.Mutex

// ReindexAllDocuments 重建所有文档索引
func (s *SearchService) ReindexAllDocuments(progressCallback func(current, total int)) (int, error) {
	if !reindexMu.TryLock() {
		return 0, fmt.Errorf("reindex is already running")
	}
	defer reindexMu.Unlock()

	indexPath := config.AppConfig.Storage.IndexDir
	if err := ResetIndex(indexPath); err != nil {
		logx.Infof("[SearchService] ResetIndex failed: %v, trying ForceInitIndex...", err)
		if err2 := ForceInitIndex(indexPath); err2 != nil {
			return 0, fmt.Errorf("failed to initialize index: %w (reset: %v)", err2, err)
		}
	}

	var docs []*workspacemodel.Document
	var err error

	docs, err = s.searchRepo.FindAllDocumentsIncludeDirs()
	if err != nil {
		return s.reindexBySpaces(nil)
	}

	return s.indexDocuments(docs, progressCallback)
}

// reindexBySpaces 按空间重建索引（fallback）
func (s *SearchService) reindexBySpaces(progressCallback func(current, total int)) (int, error) {
	spaces, err := s.searchRepo.FindAllSpaces()
	if err != nil {
		return 0, err
	}

	var allDocs []*workspacemodel.Document
	for _, space := range spaces {
		docs, err := s.searchRepo.FindDocumentsBySpaceID(space.ID)
		if err != nil {
			continue
		}
		allDocs = append(allDocs, docs...)
	}

	logx.Infof("[SearchService] Reindex by spaces, found %d documents", len(allDocs))
	return s.indexDocuments(allDocs, progressCallback)
}

// indexDocuments 索引一组文档（公共方法，避免重复代码）
func (s *SearchService) indexDocuments(docs []*workspacemodel.Document, progressCallback func(current, total int)) (int, error) {
	successCount := 0
	total := len(docs)
	current := 0

	for _, doc := range docs {
		if doc.IsDelete == 1 {
			continue
		}
		// 草稿不索引
		if doc.Status == workspacemodel.DocumentStatusDraft {
			continue
		}

		var content string
		if doc.Type == workspacemodel.DocumentTypePage {
			content = s.readDocumentContent(doc, nil)

			if content == "" {
				latestVersion, err := s.searchRepo.GetLatestVersion(doc.ID)
				if err == nil && latestVersion > 0 {
					versionContent, err := s.searchRepo.GetVersionContent(doc.ID, latestVersion)
					if err == nil {
						content = versionContent
					}
				}
			}
		}

		err := IndexDocumentByID(doc.ID, doc.SpaceID, doc.Name, content, doc.Type, doc.Path)
		if err == nil {
			successCount++
		} else {
			// 写入失败必须留痕：静默计数会导致"重建完成"但索引实际缺失数据
			logx.Infof("[SearchService] IndexDocumentByID(%d, %s) failed: %v", doc.ID, doc.Name, err)
		}

		current++
		if progressCallback != nil {
			progressCallback(current, total)
		}
	}

	logx.Infof("[SearchService] Indexed %d documents", successCount)
	return successCount, nil
}

// GetIndexStats 获取索引统计
func (s *SearchService) GetIndexStats() (map[string]interface{}, error) {
	stats, err := GetIndexStats()
	if err != nil {
		return stats, err
	}
	// last_update 此前恒为 time.Now()，于是监控页每刷新一次都报"刚刚更新过"，
	// 索引早已停更也看不出来。改成索引目录中最新的段文件修改时间——可实测的值。
	_, lastUpdated := IndexDiskFacts()
	stats["last_update"] = lastUpdated
	return stats, nil
}

// GetDocumentInfoForDebug 获取文档信息用于调试
func (s *SearchService) GetDocumentInfoForDebug(docID int) (map[string]interface{}, error) {
	doc, err := s.searchRepo.FindDocumentByID(docID)
	if err != nil {
		return nil, fmt.Errorf("document not found: %v", err)
	}

	pageFile := getPageFilePath(doc, s.searchRepo, s.docStorage, nil)
	absFile := s.docStorage.GetAbsPageFile(pageFile)

	fileExists := false
	if _, err := os.Stat(absFile); err == nil {
		fileExists = true
	}

	content := s.readDocumentContent(doc, nil)

	versionContent := ""
	latestVersion, _ := s.searchRepo.GetLatestVersion(docID)
	if latestVersion > 0 {
		versionContent, _ = s.searchRepo.GetVersionContent(docID, latestVersion)
	}

	return map[string]interface{}{
		"id":             doc.ID,
		"name":           doc.Name,
		"space_id":       doc.SpaceID,
		"type":           doc.Type,
		"path":           doc.Path,
		"page_file":      pageFile,
		"abs_file":       absFile,
		"file_exists":    fileExists,
		"content_length": len(content),
		"content_preview": func() string {
			if len(content) > 200 {
				return content[:200] + "..."
			}
			return content
		}(),
		"version":             latestVersion,
		"version_content_len": len(versionContent),
	}, nil
}

// resolvePath 将 "0,1,2" 格式的路径转换为可读的 "父文档/子文档" 格式
// docMap 为主循环之前预取的文档 map，避免循环内逐个查询 DB
func (s *SearchService) resolvePath(rawPath string, spaceID int, docMap map[int]*workspacemodel.Document) string {
	if rawPath == "" || rawPath == "0" {
		return ""
	}
	parts := strings.Split(rawPath, ",")
	var pathNames []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		id := 0
		fmt.Sscanf(part, "%d", &id)
		if id > 0 {
			if doc, ok := docMap[id]; ok && doc != nil {
				pathNames = append(pathNames, doc.Name)
			}
		}
	}
	return strings.Join(pathNames, "/")
}

// parsePathIDs 将逗号分隔的路径字符串解析为 ID 列表（排除 0 和空值）
func parsePathIDs(rawPath string) []int {
	if rawPath == "" || rawPath == "0" {
		return nil
	}
	parts := strings.Split(rawPath, ",")
	var ids []int
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		id := 0
		fmt.Sscanf(part, "%d", &id)
		if id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func getPageFilePath(doc *workspacemodel.Document, searchRepo *searchrepo.SearchRepository, docStorage *utils.DocumentStorage, docMap map[int]*workspacemodel.Document) string {
	space, _ := searchRepo.FindSpaceByID(doc.SpaceID)
	spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
	if space != nil {
		spaceName = space.Name
	}

	if doc.Path == "0" || doc.Path == "" {
		if doc.Name == spaceName && doc.Type == workspacemodel.DocumentTypeDir {
			return filepath.Join(spaceName, utils.DocumentDefaultName+utils.DocumentPageSuffix)
		}
		result := docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
		return result
	}

	pathParts := strings.Split(doc.Path, ",")

	var parentPath string
	skipFirst := true
	for _, part := range pathParts {
		if part == "" || part == "0" {
			continue
		}
		parentID := 0
		fmt.Sscanf(part, "%d", &parentID)
		if parentID > 0 {
			if skipFirst {
				skipFirst = false
				continue
			}
			// 优先从预取的 docMap 取，避免 N+1；未命中再查 DB
			var parentDoc *workspacemodel.Document
			if docMap != nil {
				parentDoc = docMap[parentID]
			}
			if parentDoc == nil {
				parentDoc, _ = searchRepo.FindDocumentByID(parentID)
			}
			if parentDoc != nil {
				parentPath = parentPath + "/" + parentDoc.Name
			}
		}
	}

	var pageFile string
	if parentPath == "" {
		pageFile = docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	} else {
		fullPath := spaceName + parentPath

		if doc.Type == workspacemodel.DocumentTypePage {
			pageFile = fullPath + "/" + doc.Name + utils.DocumentPageSuffix
		} else {
			pageFile = fullPath + "/" + doc.Name + "/" + utils.DocumentDefaultName + utils.DocumentPageSuffix
		}
	}

	return filepath.Clean(pageFile)
}
