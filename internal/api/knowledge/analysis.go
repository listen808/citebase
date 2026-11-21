package knowledge

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/analyzer"
	"github.com/listen808/citebase/internal/middleware"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	commonmodel "github.com/listen808/citebase/internal/model/common"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/utils"
)

type AnalysisAPI struct{}

func NewAnalysisAPI() *AnalysisAPI {
	return &AnalysisAPI{}
}

// visibleSpaceSet 返回当前调用者可见的知识库空间集合；unrestricted 表示超管（不裁剪）。
//
// 分析类接口在 space_id 缺省（=0）时按「全库」读取文档清单与实体，
// 若不按调用者可见范围裁剪，任意登录用户都能拿到全站空间的文档名与实体清单。
func visibleSpaceSet(c *gin.Context) (map[int]bool, bool, error) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		return nil, false, err
	}
	isSuper := middleware.IsSuperAdmin(c)
	if isSuper {
		return nil, true, nil
	}
	ids, err := workspacerepo.NewSpaceRepository().FindAccessibleSpaceIDs(uid, false)
	if err != nil {
		return nil, false, err
	}
	set := make(map[int]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, false, nil
}

// analysisScope 把「space_id 缺省 = 全库」的读取范围收敛为调用者可见的空间集合。
// 返回 nil 表示不裁剪（超管）；ok=false 时已写出响应，调用方直接 return。
func analysisScope(c *gin.Context, reqSpaceID int) ([]int, bool, error) {
	visible, unrestricted, err := visibleSpaceSet(c)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return nil, false, err
	}
	if reqSpaceID > 0 {
		if !unrestricted && !visible[reqSpaceID] {
			response.ForbiddenKey(c, response.MsgNoPermissionAnalyzeSpace)
			return nil, false, nil
		}
		return []int{reqSpaceID}, true, nil
	}
	if unrestricted {
		return nil, true, nil
	}
	ids := make([]int, 0, len(visible))
	for id := range visible {
		ids = append(ids, id)
	}
	return ids, true, nil
}

type DetectDuplicatesRequest struct {
	SpaceID       int     `json:"space_id"`
	Threshold     float64 `json:"threshold"`
	ExcludeDocIDs []int   `json:"exclude_doc_ids"`
}

func (api *AnalysisAPI) StartAnalysis(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	sched := m.Scheduler()
	if sched == nil {
		response.ServerErrorKey(c, response.MsgAIAnalyzerUnconfigured)
		return
	}

	userID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	var req struct {
		Type           string `json:"type"`
		SpaceID        int    `json:"space_id"`
		TargetSpaceIDs []int  `json:"target_space_ids"`
		TargetDocID    int    `json:"target_doc_id"`
		DocIDs         []int  `json:"doc_ids"`
		DateFrom       string `json:"date_from"`
		DateTo         string `json:"date_to"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Type = ""
	}

	taskType := analyzer.TaskTypeFull
	if req.Type != "" {
		taskType = req.Type
	}

	if taskType == analyzer.TaskTypeSingleDoc && len(req.DocIDs) > 1 {
		taskType = analyzer.TaskTypeMultiDoc
	}
	if taskType == analyzer.TaskTypeMultiDoc && len(req.DocIDs) == 1 {
		req.TargetDocID = req.DocIDs[0]
		taskType = analyzer.TaskTypeSingleDoc
	}

	spaceIDs := req.TargetSpaceIDs
	if len(spaceIDs) == 0 && req.SpaceID > 0 {
		spaceIDs = []int{req.SpaceID}
	}

	var modifiedSince int64
	if req.DateFrom != "" {
		if t, err := time.ParseInLocation("2006-01-02", req.DateFrom, time.Local); err == nil {
			modifiedSince = t.Unix()
		}
	}
	_ = req.DateTo

	task := &analyzer.AnalysisTask{
		Type:           taskType,
		TargetSpaceIDs: spaceIDs,
		TargetDocID:    req.TargetDocID,
		TargetDocIDs:   req.DocIDs,
		ModifiedSince:  modifiedSince,
	}

	recordAuditLog(userID, "start_analysis", "", "started", fmt.Sprintf("type=%s, spaces=%v, doc=%d, docs=%v", taskType, spaceIDs, req.TargetDocID, req.DocIDs), c)

	startedTask, err := sched.RunAnalysis(context.Background(), task)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "分析任务已启动",
		"task_id": startedTask.ID,
	})
}

func (api *AnalysisAPI) GetAnalysisProgress(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	sched := m.Scheduler()
	if sched == nil {
		response.ServerErrorKey(c, response.MsgAIAnalyzerUnconfigured)
		return
	}

	taskIDStr := c.Param("id")
	if taskIDStr == "" {
		taskIDStr = c.DefaultQuery("task_id", "0-0")
	}

	progress, err := sched.GetTask(taskIDStr)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, progress)
}

func (api *AnalysisAPI) ListAnalysisTasks(c *gin.Context) {
	m := ai.GetModule()

	var pq commonmodel.PageQuery
	pgNum, pgSize, _ := utils.ParsePagination(c, 20)
	pq.PageNum, pq.PageSize = pgNum, pgSize

	if !m.Enabled() || m.Scheduler() == nil {
		response.Success(c, &commonmodel.PageResult[*analyzer.AnalysisTask]{
			List:     []*analyzer.AnalysisTask{},
			Total:    0,
			PageNum:  pq.PageNum,
			PageSize: pq.Limit(),
		})
		return
	}

	sched := m.Scheduler()
	tasks := sched.ListTasks()
	total := len(tasks)

	start := pq.Offset()
	pageSize := pq.Limit()
	if start >= total {
		start = 0
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	if start > end {
		start = 0
		end = 0
	}
	pagedTasks := tasks[start:end]

	response.Success(c, &commonmodel.PageResult[*analyzer.AnalysisTask]{
		List:     pagedTasks,
		Total:    int64(total),
		PageNum:  pq.PageNum,
		PageSize: pageSize,
	})
}

func (api *AnalysisAPI) PauseAnalysis(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	sched := m.Scheduler()
	if sched == nil {
		response.ServerErrorKey(c, response.MsgAIAnalyzerUnconfigured)
		return
	}

	taskIDStr := c.Param("id")
	if err := sched.PauseTask(taskIDStr); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "分析任务已暂停",
	})
}

func (api *AnalysisAPI) ResumeAnalysis(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	sched := m.Scheduler()
	if sched == nil {
		response.ServerErrorKey(c, response.MsgAIAnalyzerUnconfigured)
		return
	}

	taskIDStr := c.Param("id")
	if err := sched.ResumeTask(taskIDStr); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"message": "分析任务已恢复",
	})
}

func (api *AnalysisAPI) UpdateDocReviewStatus(c *gin.Context) {
	var req struct {
		DocID  int    `json:"doc_id" binding:"required"`
		Status int    `json:"status" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "doc_id and status are required")
		return
	}
	docID := req.DocID

	m := ai.GetModule()
	if !m.Enabled() || m.Scheduler() == nil {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	reason := req.Reason
	statusStr := "pending"
	switch req.Status {
	case 1:
		statusStr = "approved"
	case 2:
		statusStr = "rejected"
	case 3:
		statusStr = "manual_edit"
	}

	if err := m.Scheduler().UpdateReviewStatus(docID, statusStr); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"message": "审核状态已更新", "status": statusStr, "reason": reason})
}

func (api *AnalysisAPI) GetDocMapping(c *gin.Context) {
	docIDStr := c.Param("id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	m := ai.GetModule()
	if !m.Enabled() || m.Scheduler() == nil {
		response.Success(c, gin.H{"mapping": nil})
		return
	}

	mapping, err := m.Scheduler().GetDocMapping(docID)
	if err != nil || mapping == nil {
		response.Success(c, gin.H{"mapping": nil})
		return
	}

	response.Success(c, gin.H{"mapping": mapping})
}

func (api *AnalysisAPI) DetectDuplicates(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	var req DetectDuplicatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.SpaceID = 0
	}

	detector := analyzer.NewDuplicateDetector(m.FastProvider())

	docReader := analyzer.NewDocReader(0)
	targetSpaceIDs, ok, _ := analysisScope(c, req.SpaceID)
	if !ok {
		return
	}
	docs, _, err := docReader.ReadAllDocuments(targetSpaceIDs, 0)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	if len(docs) < 2 {
		response.Success(c, gin.H{
			"total_duplicates": 0,
			"groups":           []interface{}{},
		})
		return
	}

	similarityThreshold := req.Threshold
	if similarityThreshold <= 0 {
		similarityThreshold = 0.7
	}

	type DocModel struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	type DuplicateGroup struct {
		DocIDs     []int    `json:"doc_ids"`
		DocNames   []string `json:"doc_names"`
		Similarity float64  `json:"similarity"`
		Reason     string   `json:"reason"`
	}

	excludeSet := make(map[int]bool)
	for _, id := range req.ExcludeDocIDs {
		excludeSet[id] = true
	}

	type simpleDoc struct {
		ID      int
		Name    string
		Content string
	}
	var simpleDocs []simpleDoc
	for _, d := range docs {
		if !excludeSet[d.DocID] {
			// 仅取前 4000 字符作为相似度比较的依据，避免对超大文档做全量字符串比对
			content := d.Content
			if len([]rune(content)) > 4000 {
				content = string([]rune(content)[:4000])
			}
			simpleDocs = append(simpleDocs, simpleDoc{ID: d.DocID, Name: d.DocName, Content: content})
		}
	}

	groups := []DuplicateGroup{}
	visited := make(map[[2]int]bool)

	for i := 0; i < len(simpleDocs); i++ {
		for j := i + 1; j < len(simpleDocs); j++ {
			key := [2]int{simpleDocs[i].ID, simpleDocs[j].ID}
			if visited[key] {
				continue
			}

			// 基于文档内容计算相似度，而非 doc_ID_name 字符串
			sim := detector.SimpleSimilarityPublic(
				simpleDocs[i].Content,
				simpleDocs[j].Content,
			)

			if sim >= similarityThreshold {
				groups = append(groups, DuplicateGroup{
					DocIDs:     []int{simpleDocs[i].ID, simpleDocs[j].ID},
					DocNames:   []string{simpleDocs[i].Name, simpleDocs[j].Name},
					Similarity: sim,
					Reason:     fmt.Sprintf("基于文档内容的 Jaccard 相似度 %.2f", sim),
				})
				visited[key] = true
			}
		}
	}

	totalDuplicates := 0
	for _, g := range groups {
		totalDuplicates += len(g.DocIDs)
	}

	response.Success(c, gin.H{
		"total_duplicates": totalDuplicates,
		"groups":           groups,
	})
}

func (api *AnalysisAPI) GetClusters(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, _ := strconv.Atoi(spaceIDStr)

	docReader := analyzer.NewDocReader(0)
	targetSpaceIDs, ok, _ := analysisScope(c, spaceID)
	if !ok {
		return
	}
	docs, _, err := docReader.ReadAllDocuments(targetSpaceIDs, 0)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	type Cluster struct {
		ID       string   `json:"id"`
		Label    string   `json:"label"`
		DocIDs   []int    `json:"doc_ids"`
		DocNames []string `json:"doc_names"`
	}

	spaceGroups := make(map[int]*Cluster)
	for _, doc := range docs {
		if c, ok := spaceGroups[doc.SpaceID]; ok {
			c.DocIDs = append(c.DocIDs, doc.DocID)
			c.DocNames = append(c.DocNames, doc.DocName)
		} else {
			spaceGroups[doc.SpaceID] = &Cluster{
				ID:       fmt.Sprintf("space_%d", doc.SpaceID),
				Label:    doc.SpaceName,
				DocIDs:   []int{doc.DocID},
				DocNames: []string{doc.DocName},
			}
		}
	}

	clusters := make([]Cluster, 0, len(spaceGroups))
	for _, c := range spaceGroups {
		clusters = append(clusters, *c)
	}

	response.Success(c, gin.H{"clusters": clusters})
}

func (api *AnalysisAPI) MergeCluster(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	clusterIDStr := c.Param("id")
	clusterID, err := strconv.Atoi(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "invalid cluster id")
		return
	}

	var req struct {
		DocIDs []int `json:"doc_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "doc_ids are required")
		return
	}

	if len(req.DocIDs) < 2 {
		response.BadRequest(c, "at least 2 documents required for merge")
		return
	}

	detector := analyzer.NewDuplicateDetector(m.FastProvider())

	pair := analyzer.DocPair{
		DocA:       &workspacemodel.Document{ID: req.DocIDs[0]},
		DocB:       &workspacemodel.Document{ID: req.DocIDs[1]},
		Similarity: 0.8,
		// :id 只是调用方给的第几组重复项，系统里并不存在持久化的数字簇 ID
		// （/ai/analysis/clusters 返回的是 space_N 这种按空间分组）。
		// 原先写成 "手动合并 (cluster %d)"，会让人以为有一个真实簇被合并了。
		Reason: fmt.Sprintf("手动生成合并方案（重复组 #%d）", clusterID),
	}

	plan, err := detector.GenerateMergePlan(context.Background(), pair)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"plan":    plan,
		"message": "合并计划已生成",
	})
}

func (api *AnalysisAPI) GetKnowledgeGraph(c *gin.Context) {
	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, _ := strconv.Atoi(spaceIDStr)

	// 知识图谱对普通用户开放，但 space_id=0 时旧实现直接 FindAllEntities，
	// 等于把全站空间的实体清单交给任意登录用户；这里按可见空间裁剪。
	visible, unrestricted, verr := visibleSpaceSet(c)
	if verr != nil {
		response.ServerErrorSafe(c, verr)
		return
	}
	if spaceID > 0 && !unrestricted && !visible[spaceID] {
		response.ForbiddenKey(c, response.MsgNoPermissionSpaceGraph)
		return
	}

	nodes := make([]map[string]interface{}, 0)
	edges := make([]map[string]interface{}, 0)

	// 优先从数据库读取持久化的知识图谱
	kgRepo := knowledgerepo.NewKnowledgeGraphRepository(repository.DB)
	var dbEntities []knowledgemodel.KnowledgeEntity
	var dbRelations []knowledgemodel.KnowledgeRelation
	var dbErr error

	if spaceID > 0 {
		dbEntities, dbErr = kgRepo.FindEntitiesBySpaceID(spaceID)
		if dbErr == nil {
			dbRelations, dbErr = kgRepo.FindRelationsBySpaceID(spaceID)
		}
	} else {
		dbEntities, dbErr = kgRepo.FindAllEntities()
		if dbErr == nil {
			dbRelations, dbErr = kgRepo.FindAllRelations()
		}
		if dbErr == nil && !unrestricted {
			allowed := make(map[int]bool, len(dbEntities))
			keptEntities := make([]knowledgemodel.KnowledgeEntity, 0, len(dbEntities))
			for _, e := range dbEntities {
				if visible[e.SpaceID] {
					keptEntities = append(keptEntities, e)
					allowed[e.ID] = true
				}
			}
			keptRels := make([]knowledgemodel.KnowledgeRelation, 0, len(dbRelations))
			for _, r := range dbRelations {
				if allowed[r.SourceEntityID] && allowed[r.TargetEntityID] {
					keptRels = append(keptRels, r)
				}
			}
			dbEntities = keptEntities
			dbRelations = keptRels
		}
	}

	if dbErr == nil && len(dbEntities) > 0 {
		entityIDMap := make(map[int]string)
		for _, e := range dbEntities {
			nodeID := fmt.Sprintf("e%d", e.ID)
			entityIDMap[e.ID] = nodeID
			nodes = append(nodes, map[string]interface{}{
				"id":    nodeID,
				"label": e.Name,
				"type":  e.Type,
				"group": e.Type,
			})
		}
		for _, r := range dbRelations {
			srcID := entityIDMap[r.SourceEntityID]
			tgtID := entityIDMap[r.TargetEntityID]
			if srcID != "" && tgtID != "" {
				edges = append(edges, map[string]interface{}{
					"source":     srcID,
					"target":     tgtID,
					"label":      r.RelationType,
					"confidence": r.Confidence,
				})
			}
		}
	}

	// 数据库无数据时 fallback 到内存中的最新任务结果
	// 该兜底取的是「全站最近一次完成任务」的结果，不区分空间与归属，
	// 因此只对不受空间裁剪的超级管理员开放，普通用户宁可看到空图。
	if len(nodes) == 0 && unrestricted {
		sched := m.Scheduler()
		if sched != nil {
			tasks := sched.ListTasks()
			var latestResult *analyzer.AnalysisResult
			for i := len(tasks) - 1; i >= 0; i-- {
				if tasks[i].Result != nil && tasks[i].Status == analyzer.TaskStatusCompleted {
					latestResult = tasks[i].Result
					break
				}
			}
			if latestResult != nil && latestResult.KnowledgeGraph != nil {
				for _, e := range latestResult.KnowledgeGraph.Entities {
					nodes = append(nodes, map[string]interface{}{
						"id":    e.ID,
						"label": e.Name,
						"type":  e.Type,
						"group": e.Type,
					})
				}
				for _, r := range latestResult.KnowledgeGraph.Relations {
					edges = append(edges, map[string]interface{}{
						"source":     r.SourceID,
						"target":     r.TargetID,
						"label":      r.Type,
						"confidence": r.Weight,
					})
				}
			}
		}
	}

	response.Success(c, gin.H{
		"nodes": nodes,
		"edges": edges,
		"metadata": map[string]interface{}{
			"entity_count":   len(nodes),
			"relation_count": len(edges),
		},
	})
}

func (api *AnalysisAPI) ExtractEntities(c *gin.Context) {
	var req struct {
		DocID int `json:"doc_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DocID <= 0 {
		response.BadRequest(c, "doc_id is required")
		return
	}
	docID := req.DocID

	m := ai.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	docReader := analyzer.NewDocReader(0)
	docs, _, err := docReader.ReadAllDocuments(nil, 0)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var targetDoc *analyzer.DocumentContent
	for _, d := range docs {
		if d.DocID == docID {
			targetDoc = d
			break
		}
	}

	if targetDoc == nil {
		response.NotFound(c, "document not found")
		return
	}

	extractor := analyzer.NewEntityExtractor(m.FastProvider())
	entities, err := extractor.ExtractFromContent(context.Background(), targetDoc.Content)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"doc_id":   docID,
		"entities": entities,
	})
}

func (api *AnalysisAPI) ClusterKnowledgeGraph(c *gin.Context) {
	response.Success(c, gin.H{
		"clusters": []interface{}{},
		"message":  "知识图谱聚类功能未实现",
	})
}

func (api *AnalysisAPI) SuggestRelations(c *gin.Context) {
	response.Success(c, gin.H{
		"suggestions": []interface{}{},
		"message":     "关系推荐功能未实现",
	})
}

func recordAuditLog(userID int, action, targetID, result, reason string, c *gin.Context) {
	recordAuditLogFull(userID, action, targetID, "", "", 0, 0, "", "", result, reason, 0, c)
}

func recordAuditLogFull(
	userID int,
	action, targetID, question, answer string,
	tokenUsage int,
	sourceCount int,
	modelName, providerName, result, reason string,
	durationMs int64,
	c *gin.Context,
) {
	ipAddress := ""
	if c != nil {
		ipAddress = c.ClientIP()
	}
	auditLogRepo := airepo.NewAIAuditLogRepository()
	auditLog := &aimodel.AIAuditLog{
		UserID:      userID,
		Action:      action,
		TargetID:    targetID,
		Question:    question,
		Answer:      answer,
		TokenUsage:  tokenUsage,
		SourceCount: sourceCount,
		Model:       modelName,
		Provider:    providerName,
		Result:      result,
		Reason:      reason,
		IPAddress:   ipAddress,
		DurationMs:  durationMs,
	}
	if err := auditLogRepo.Create(auditLog); err != nil {
		logx.Infof("[AuditLog] Failed to record audit log: %v", err)
	}
}
