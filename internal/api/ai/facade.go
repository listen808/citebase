package ai

import (
	"net/http"
	"strconv"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/facade"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/middleware"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils"
)

type AIFacadeAPI struct {
	facade  *facade.AIFacade
	histSvc *aiSvc.SQLHistoryService
}

func NewAIFacadeAPI(f *facade.AIFacade, histSvc *aiSvc.SQLHistoryService) *AIFacadeAPI {
	return &AIFacadeAPI{facade: f, histSvc: histSvc}
}

func (api *AIFacadeAPI) GenerateSQL(c *gin.Context) {
	var req struct {
		Question string                         `json:"question" binding:"required"`
		SpaceID  uint64                         `json:"space_id"`
		History  []text2sql.ConversationMessage `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] GenerateSQL bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}

	if len(req.History) > 0 {
		logx.Infof("[GenerateSQL] Multi-turn: question=%q, history_len=%d", req.Question, len(req.History))
		for i, h := range req.History {
			logx.Infof("[GenerateSQL] History[%d]: role=%s, sql=%q", i, h.Role, h.SQL)
		}
	}

	// 安全收口：把调用者身份注入 ctx，provider 层据此做 Token 配额预检
	userID, _ := middleware.GetUserID(c)
	ctx := provider.ContextWithUserID(c.Request.Context(), userID)
	result, clarify, err := api.facade.GenerateSQLWithHistory(ctx, req.Question, req.SpaceID, req.History)
	if err != nil {
		logx.Infof("[facade] GenerateSQL error: question=%q, spaceID=%d, err=%v", req.Question, req.SpaceID, err)
		// P3：不回显内部错误细节，避免泄露 schema/SQL 结构信息
		response.Error(c, http.StatusInternalServerError, 500, "SQL生成失败")
		return
	}

	historyID := uint64(0)
	if api.histSvc != nil {
		userID, _ := middleware.GetUserID(c)
		historyID = api.histSvc.RecordSQLHistoryFromResult(userID, req.SpaceID, req.Question, result)
	}

	traceID := ""
	if result != nil {
		traceID = result.TraceID
	}

	resp := gin.H{
		"trace_id":   traceID,
		"history_id": historyID,
	}
	if result != nil {
		resp["sql"] = result.SQL
		resp["tables"] = result.Tables
		resp["entities"] = result.Entities
		resp["relations"] = result.Relations
		resp["explanations"] = result.Explanations
		resp["warnings"] = result.Warnings
		resp["explanation"] = result.Explanation
		resp["generator_tier"] = result.GeneratorTier
		// P0: 透传 CoT 思考步骤供前端展示
		if len(result.Steps) > 0 {
			resp["steps"] = result.Steps
		}
		// P0: 透传 LLM 原始 CoT 输出
		if result.RawCoT != "" {
			resp["raw_cot"] = result.RawCoT
		}
		// P0: 透传 selfReflect 修正前后 SQL
		if result.SelfReflectBefore != "" {
			resp["self_reflect_before"] = result.SelfReflectBefore
		}
		if result.SelfReflectAfter != "" {
			resp["self_reflect_after"] = result.SelfReflectAfter
		}
		// P0: 透传字段映射
		if len(result.FieldMappings) > 0 {
			resp["field_mappings"] = result.FieldMappings
		}
	}

	if clarify != nil && clarify.Needed {
		resp["clarify"] = clarify
	}
	response.Success(c, resp)
}

func (api *AIFacadeAPI) GenerateSQLWithRAG(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
		SpaceID  uint64 `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] GenerateSQLWithRAG bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	// 安全收口：把调用者身份注入 ctx，provider 层据此做 Token 配额预检
	userID, _ := middleware.GetUserID(c)
	ctx := provider.ContextWithUserID(c.Request.Context(), userID)
	result, err := api.facade.GenerateSQLWithRAG(ctx, req.Question, req.SpaceID)
	if err != nil {
		logx.Infof("[facade] GenerateSQLWithRAG error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "SQL生成失败")
		return
	}
	response.Success(c, result)
}

// GenerateSQLStream Agent 流式 SQL 生成（SSE）
// P0-11: 通过 Agent ReAct 模式流式输出思考过程和 SQL 结果
func (api *AIFacadeAPI) GenerateSQLStream(c *gin.Context) {
	gen := api.facade.GetTextToSQLGenerator()
	if gen == nil {
		response.Error(c, http.StatusServiceUnavailable, 503, "Text2SQL 生成器未初始化")
		return
	}
	handler := gen.GetStreamHandler()
	if handler == nil {
		response.Error(c, http.StatusServiceUnavailable, 503, "Agent 流式输出未启用（需要配置 LLM provider）")
		return
	}
	handler.HandleStreamQuery(c)
}

func (api *AIFacadeAPI) ExecuteSQL(c *gin.Context) {
	var req struct {
		SQL     string `json:"sql" binding:"required"`
		SpaceID uint64 `json:"space_id" binding:"required"`
		TraceID string `json:"trace_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] ExecuteSQL bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	// P0-10: 从 gin.Context 提取 userID，启用 RLS 行级过滤
	userID := 0
	if v, exists := c.Get("user_id"); exists {
		if id, ok := v.(int); ok {
			userID = id
		}
	}
	result, err := api.facade.ExecuteSQL(c.Request.Context(), req.SQL, req.SpaceID, req.TraceID, userID)
	if err != nil {
		logx.Infof("[facade] ExecuteSQL error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "SQL执行失败")
		return
	}

	api.histSvc.UpdateSQLHistoryExecution(req.TraceID, req.SQL, result)

	response.Success(c, result)
}

func (api *AIFacadeAPI) ListPromptTemplates(c *gin.Context) {
	templateType := c.Query("type")
	queryType := c.Query("query_type")
	schemaService := api.facade.GetSchemaService()
	if schemaService == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	templates, err := schemaService.ListPromptTemplates(templateType, queryType)
	if err != nil {
		logx.Infof("[facade] ListPromptTemplates error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询失败")
		return
	}
	response.Success(c, gin.H{"templates": templates})
}

func (api *AIFacadeAPI) ListBuiltInPromptTemplates(c *gin.Context) {
	builtins := text2sql.GetBuiltInPromptTemplates()
	response.Success(c, gin.H{"templates": builtins})
}

// ============================================================
// Business Rules
// ============================================================

func (api *AIFacadeAPI) ListRules(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	page, pageSize, _ := utils.ParsePagination(c, 20)
	ruleType := c.Query("type")
	tableName := c.Query("table_name")
	rules, total, err := svc.TextToSQLListRules(page, pageSize, ruleType, tableName)
	if err != nil {
		logx.Infof("[facade] ListRules error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询规则失败")
		return
	}
	response.Paginated(c, rules, total, page, pageSize)
}

func (api *AIFacadeAPI) GetRule(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的规则ID")
		return
	}
	rule, err := svc.TextToSQLGetRule(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "规则不存在")
		return
	}
	response.Success(c, rule)
}

func (api *AIFacadeAPI) CreateRule(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required,oneof=enum relation formula condition"`
		TableName   string `json:"table_name" binding:"required"`
		FieldName   string `json:"field_name"`
		Content     string `json:"content" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] CreateRule bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	rule := &modelai.BusinessRule{
		Name:        req.Name,
		Type:        req.Type,
		TableNam:    req.TableName,
		FieldName:   req.FieldName,
		Content:     req.Content,
		Description: req.Description,
		Status:      1,
	}
	if err := svc.TextToSQLCreateRule(rule); err != nil {
		logx.Infof("[facade] CreateRule error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "创建规则失败")
		return
	}
	response.Success(c, rule)
}

func (api *AIFacadeAPI) UpdateRule(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的规则ID")
		return
	}
	var req struct {
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required,oneof=enum relation formula condition"`
		TableName   string `json:"table_name" binding:"required"`
		FieldName   string `json:"field_name"`
		Content     string `json:"content" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] UpdateRule bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	rule, err := svc.TextToSQLGetRule(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "规则不存在")
		return
	}
	rule.Name = req.Name
	rule.Type = req.Type
	rule.TableNam = req.TableName
	rule.FieldName = req.FieldName
	rule.Content = req.Content
	rule.Description = req.Description
	if err := svc.TextToSQLUpdateRule(rule); err != nil {
		logx.Infof("[facade] UpdateRule error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "更新规则失败")
		return
	}
	response.Success(c, rule)
}

func (api *AIFacadeAPI) DeleteRule(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的规则ID")
		return
	}
	if err := svc.TextToSQLDeleteRule(id); err != nil {
		logx.Infof("[facade] DeleteRule error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "删除规则失败")
		return
	}
	response.Success(c, nil)
}

// ============================================================
// Tables
// ============================================================

func (api *AIFacadeAPI) ListTables(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.Query("keyword")
	tables, total, err := svc.TextToSQLListTables(page, pageSize, keyword)
	if err != nil {
		logx.Infof("[facade] ListTables error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询表失败")
		return
	}
	response.Paginated(c, tables, total, page, pageSize)
}

func (api *AIFacadeAPI) GetTableDetail(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	tableName := c.Param("name")
	table, err := svc.TextToSQLGetTableDetail(tableName)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "表不存在")
		return
	}
	response.Success(c, table)
}

func (api *AIFacadeAPI) GetTableRelations(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	tableName := c.Param("name")
	relations, err := svc.TextToSQLGetTableRelations(tableName)
	if err != nil {
		logx.Infof("[facade] GetTableRelations error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询关系失败")
		return
	}
	response.Success(c, relations)
}

// ============================================================
// Relations
// ============================================================

func (api *AIFacadeAPI) CreateRelation(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	var req struct {
		SourceTable string  `json:"source_table" binding:"required"`
		SourceField string  `json:"source_field" binding:"required"`
		TargetTable string  `json:"target_table" binding:"required"`
		TargetField string  `json:"target_field" binding:"required"`
		Confidence  float32 `json:"confidence"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] CreateRelation bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	rel := &modelai.TableRelation{
		SourceTable:  req.SourceTable,
		SourceField:  req.SourceField,
		TargetTable:  req.TargetTable,
		TargetField:  req.TargetField,
		RelationType: "one_to_many",
		Confidence:   req.Confidence,
	}
	if rel.Confidence == 0 {
		rel.Confidence = 1
	}
	if err := svc.TextToSQLCreateRelation(rel); err != nil {
		logx.Infof("[facade] CreateRelation error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "创建关系失败")
		return
	}
	response.Success(c, rel)
}

func (api *AIFacadeAPI) DeleteRelation(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的关系ID")
		return
	}
	if err := svc.TextToSQLDeleteRelation(id); err != nil {
		logx.Infof("[facade] DeleteRelation error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "删除关系失败")
		return
	}
	response.Success(c, nil)
}

// ============================================================
// Knowledge & Scan
// ============================================================

func (api *AIFacadeAPI) RefreshKnowledge(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	scannedCount, err := svc.TextToSQLScanAndImportAllDocs()
	if err != nil {
		logx.Infof("[facade] RefreshKnowledge scan error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "扫描文档失败")
		return
	}
	if err := svc.TextToSQLRefreshKnowledge(); err != nil {
		logx.Infof("[facade] RefreshKnowledge refresh error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "刷新知识库失败")
		return
	}
	response.Success(c, gin.H{"message": "知识库已刷新", "scanned_count": scannedCount})
}

func (api *AIFacadeAPI) ScanSpace(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	spaceID, err := strconv.ParseUint(c.Param("space_id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的空间ID")
		return
	}
	result, err := svc.TextToSQLScanSpace(spaceID)
	if err != nil {
		logx.Infof("[facade] ScanSpace error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "扫描失败")
		return
	}
	response.Success(c, result)
}

func (api *AIFacadeAPI) GetSpaceTables(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	spaceID, err := strconv.ParseUint(c.Param("space_id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的空间ID")
		return
	}
	tables, err := svc.TextToSQLGetSpaceTables(spaceID)
	if err != nil {
		logx.Infof("[facade] GetSpaceTables error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询失败")
		return
	}
	type TableDebug struct {
		TableName    string               `json:"table_name"`
		TableComment string               `json:"table_comment"`
		ColumnCount  int                  `json:"column_count"`
		Columns      []modelai.ColumnInfo `json:"columns"`
	}
	var debug []TableDebug
	for _, t := range tables {
		debug = append(debug, TableDebug{
			TableName:    t.TableNam,
			TableComment: t.TableComment,
			ColumnCount:  len(t.ColumnsJSON),
			Columns:      t.ColumnsJSON,
		})
	}
	response.Success(c, debug)
}

func (api *AIFacadeAPI) ScanSpaceWithRAG(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	spaceID, err := strconv.ParseUint(c.Param("space_id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的空间ID")
		return
	}
	result, err := svc.TextToSQLScanSpace(spaceID)
	if err != nil {
		logx.Infof("[facade] ScanSpaceWithRAG error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "扫描失败")
		return
	}
	response.Success(c, result)
}

// ============================================================
// Prompt Templates (CRUD)
// ============================================================

func (api *AIFacadeAPI) CreatePromptTemplate(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	var req struct {
		Name         string  `json:"name" binding:"required"`
		QueryType    string  `json:"query_type" binding:"required"`
		SystemPrompt string  `json:"system_prompt"`
		UserTemplate string  `json:"user_template" binding:"required"`
		Description  string  `json:"description"`
		IsActive     bool    `json:"is_active"`
		IsDefault    bool    `json:"is_default"`
		Sort         int     `json:"sort"`
		SpaceID      *uint64 `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] CreatePromptTemplate bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	tpl := &modelai.PromptTemplate{
		Name:         req.Name,
		QueryType:    req.QueryType,
		SystemPrompt: req.SystemPrompt,
		UserTemplate: req.UserTemplate,
		Description:  req.Description,
		IsActive:     req.IsActive,
		IsDefault:    req.IsDefault,
		Sort:         req.Sort,
		SpaceID:      req.SpaceID,
	}
	if err := svc.TextToSQLCreatePromptTemplate(tpl); err != nil {
		logx.Infof("[facade] CreatePromptTemplate error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "创建模板失败")
		return
	}
	response.Success(c, tpl)
}

func (api *AIFacadeAPI) GetPromptTemplateByID(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的模板ID")
		return
	}
	tpl, err := svc.TextToSQLGetPromptTemplateByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "模板不存在")
		return
	}
	response.Success(c, tpl)
}

func (api *AIFacadeAPI) UpdatePromptTemplate(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的模板ID")
		return
	}
	var req struct {
		Name         string  `json:"name" binding:"required"`
		QueryType    string  `json:"query_type" binding:"required"`
		SystemPrompt string  `json:"system_prompt"`
		UserTemplate string  `json:"user_template" binding:"required"`
		Description  string  `json:"description"`
		IsActive     bool    `json:"is_active"`
		IsDefault    bool    `json:"is_default"`
		Sort         int     `json:"sort"`
		SpaceID      *uint64 `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] UpdatePromptTemplate bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	tpl, err := svc.TextToSQLGetPromptTemplateByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "模板不存在")
		return
	}
	tpl.Name = req.Name
	tpl.QueryType = req.QueryType
	tpl.SystemPrompt = req.SystemPrompt
	tpl.UserTemplate = req.UserTemplate
	tpl.Description = req.Description
	tpl.IsActive = req.IsActive
	tpl.IsDefault = req.IsDefault
	tpl.Sort = req.Sort
	tpl.SpaceID = req.SpaceID
	if err := svc.TextToSQLUpdatePromptTemplate(tpl); err != nil {
		logx.Infof("[facade] UpdatePromptTemplate error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "更新模板失败")
		return
	}
	response.Success(c, tpl)
}

func (api *AIFacadeAPI) DeletePromptTemplate(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的模板ID")
		return
	}
	if err := svc.TextToSQLDeletePromptTemplate(id); err != nil {
		logx.Infof("[facade] DeletePromptTemplate error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "删除模板失败")
		return
	}
	response.Success(c, nil)
}

func (api *AIFacadeAPI) SetDefaultPromptTemplate(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的模板ID")
		return
	}
	var req struct {
		SpaceID uint64 `json:"space_id"`
	}
	_ = c.ShouldBindJSON(&req)
	if err := svc.TextToSQLSetDefaultPromptTemplate(id, req.SpaceID); err != nil {
		logx.Infof("[facade] SetDefaultPromptTemplate error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "设置默认模板失败")
		return
	}
	response.Success(c, gin.H{"message": "设置成功"})
}

func (api *AIFacadeAPI) TogglePromptTemplate(c *gin.Context) {
	svc := api.facade.GetSchemaService()
	if svc == nil {
		response.Error(c, http.StatusInternalServerError, 500, "服务不可用")
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的模板ID")
		return
	}
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] TogglePromptTemplate bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	if err := svc.TextToSQLTogglePromptTemplate(id, req.IsActive); err != nil {
		logx.Infof("[facade] TogglePromptTemplate error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "操作失败")
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// ============================================================
// Multi-Candidate SQL Generation
// ============================================================

func (api *AIFacadeAPI) GenerateMultiCandidateSQL(c *gin.Context) {
	var req struct {
		Question      string `json:"question" binding:"required"`
		SpaceID       uint64 `json:"space_id"`
		NumCandidates int    `json:"num_candidates"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] GenerateMultiCandidateSQL bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	if req.NumCandidates <= 0 {
		req.NumCandidates = 3
	}

	gen := api.facade.GetTextToSQLGenerator()
	if gen == nil {
		response.Error(c, http.StatusInternalServerError, 500, "SQL生成器未初始化")
		return
	}

	// 安全收口：把调用者身份注入 ctx，provider 层据此做 Token 配额预检
	userID, _ := middleware.GetUserID(c)
	ctx := provider.ContextWithUserID(c.Request.Context(), userID)
	result, candidates, err := gen.GenerateMultipleCandidatesContext(ctx, req.Question, req.SpaceID, req.NumCandidates)
	if err != nil {
		logx.Infof("[facade] GenerateMultiCandidateSQL error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "SQL生成失败")
		return
	}

	response.Success(c, gin.H{
		"best":       result,
		"candidates": candidates,
		"total":      len(candidates),
	})
}

// ============================================================
// SQL Evaluation
// ============================================================

func (api *AIFacadeAPI) EvaluateSQL(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
		SQL      string `json:"sql" binding:"required"`
		SpaceID  uint64 `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] EvaluateSQL bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}

	gen := api.facade.GetTextToSQLGenerator()
	if gen == nil {
		response.Error(c, http.StatusInternalServerError, 500, "SQL生成器未初始化")
		return
	}

	// 安全收口：把调用者身份注入 ctx，provider 层据此做 Token 配额预检
	userID, _ := middleware.GetUserID(c)
	ctx := provider.ContextWithUserID(c.Request.Context(), userID)
	evalResult := gen.EvaluateSQLContext(ctx, req.Question, req.SQL, req.SpaceID)
	response.Success(c, evalResult)
}

// ============================================================
// SQL Examples (Few-shot examples management)
// ============================================================

func (api *AIFacadeAPI) ListSQLExamples(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	verifiedOnly := c.Query("verified") == "1"

	repo := airepo.NewSQLExampleRepo()
	examples, total, err := repo.ListBySpace(spaceID, verifiedOnly, page, pageSize)
	if err != nil {
		logx.Infof("[facade] ListSQLExamples error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "查询示例失败")
		return
	}
	response.Paginated(c, examples, total, page, pageSize)
}

func (api *AIFacadeAPI) GetSQLExample(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的示例ID")
		return
	}
	repo := airepo.NewSQLExampleRepo()
	example, err := repo.FindByID(int(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "示例不存在")
		return
	}
	response.Success(c, example)
}

func (api *AIFacadeAPI) CreateSQLExample(c *gin.Context) {
	var req struct {
		Question string `json:"question" binding:"required"`
		SQL      string `json:"sql" binding:"required"`
		SpaceID  int    `json:"space_id"`
		Verified int    `json:"verified"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] CreateSQLExample bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	if req.Verified != 0 && req.Verified != 1 {
		req.Verified = 0
	}
	repo := airepo.NewSQLExampleRepo()
	err := repo.Create(&modelai.SQLExample{
		SpaceID:   req.SpaceID,
		Question:  req.Question,
		SQL:       req.SQL,
		Verified:  req.Verified,
		CreatedBy: 0,
	})
	if err != nil {
		logx.Infof("[facade] CreateSQLExample error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "创建示例失败")
		return
	}
	response.Success(c, gin.H{"message": "创建成功"})
}

func (api *AIFacadeAPI) UpdateSQLExample(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的示例ID")
		return
	}
	var req struct {
		Question string `json:"question"`
		SQL      string `json:"sql"`
		Verified int    `json:"verified"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logx.Infof("[facade] UpdateSQLExample bind error: %v", err)
		response.Error(c, http.StatusBadRequest, 400, "请求参数无效")
		return
	}
	repo := airepo.NewSQLExampleRepo()
	example, err := repo.FindByID(int(id))
	if err != nil {
		response.Error(c, http.StatusNotFound, 404, "示例不存在")
		return
	}
	if req.Question != "" {
		example.Question = req.Question
	}
	if req.SQL != "" {
		example.SQL = req.SQL
	}
	if req.Verified == 0 || req.Verified == 1 {
		example.Verified = req.Verified
	}
	err = repo.Update(example)
	if err != nil {
		logx.Infof("[facade] UpdateSQLExample error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "更新示例失败")
		return
	}
	response.Success(c, example)
}

func (api *AIFacadeAPI) VerifySQLExample(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的示例ID")
		return
	}
	repo := airepo.NewSQLExampleRepo()
	err = repo.Verify(int(id))
	if err != nil {
		logx.Infof("[facade] VerifySQLExample error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "验证失败")
		return
	}
	response.Success(c, gin.H{"message": "验证成功"})
}

func (api *AIFacadeAPI) DeleteSQLExample(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Error(c, http.StatusBadRequest, 400, "无效的示例ID")
		return
	}
	repo := airepo.NewSQLExampleRepo()
	err = repo.Delete(int(id))
	if err != nil {
		logx.Infof("[facade] DeleteSQLExample error: %v", err)
		response.Error(c, http.StatusInternalServerError, 500, "删除失败")
		return
	}
	response.Success(c, gin.H{"message": "删除成功"})
}
