package schema

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	serviceschema "github.com/listen808/citebase/internal/service/schema"
	"github.com/listen808/citebase/internal/text2sql"
	"github.com/listen808/citebase/internal/utils"
)

// SchemaAPI Schema Intelligence API
type SchemaAPI struct {
	service *serviceschema.SchemaService
}

func NewSchemaAPI() *SchemaAPI {
	return &SchemaAPI{
		service: serviceschema.NewSchemaService(nil),
	}
}

// NewSchemaAPIWithService 使用已创建的 SchemaService（用于共享 Text2SQL 依赖）
func NewSchemaAPIWithService(s *serviceschema.SchemaService) *SchemaAPI {
	return &SchemaAPI{
		service: s,
	}
}

// maxSchemaPageSize 列表接口单页上限：page_size 不设上限会让任意请求退化为全表拉取，
// 大库上配合 LIMIT 大偏移扫描还会拖垮数据库。
const maxSchemaPageSize = 100

// schemaListPagination 统一钳制 schema 系列列表接口的分页参数。
func schemaListPagination(c *gin.Context) (page, pageSize int) {
	page, pageSize, _ = utils.ParsePagination(c, 20)
	if pageSize > maxSchemaPageSize {
		pageSize = maxSchemaPageSize
	}
	return
}

// requireSchemaSpaceAccess 校验当前登录用户对 :id 指向的 SchemaSpace 的访问权限。
//
// SchemaSpace 与知识库空间一一绑定（space_id），但服务层所有方法只接收 schemaSpaceID、
// 不含调用者身份，物理上无法自行鉴权；AutoRBAC 又只覆盖了写/扫描等管理操作。
// 不在此收口的话，任意登录用户都能按 id 枚举他人绑定库的表/字段/关系元数据，
// 并通过 AskQuestion 在别人的数据源上生成执行查询。校验失败时已写出响应，调用方只需 return。
func (api *SchemaAPI) requireSchemaSpaceAccess(c *gin.Context, schemaSpaceID int) bool {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		response.Unauthorized(c, "unauthorized")
		return false
	}
	if middleware.IsSuperAdmin(c) {
		return true
	}
	space, err := api.service.GetSchemaSpace(schemaSpaceID)
	if err != nil || space == nil {
		response.NotFound(c, "schema space not found")
		return false
	}
	ids, err := workspacerepo.NewSpaceRepository().FindAccessibleSpaceIDs(uid, false)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return false
	}
	for _, id := range ids {
		if id == space.SpaceID {
			return true
		}
	}
	response.ForbiddenKey(c, response.MsgNoPermissionKBSSpace)
	return false
}

// requireSchemaTaskAccess 校验当前用户对扫描任务所属 SchemaSpace 的访问权限。
// 任务只有 taskID，Pause/Resume/Cancel/Delete 若不校验归属，任意登录用户都能
// 打断或清空他人（乃至全站）正在运行的元数据扫描。
func (api *SchemaAPI) requireSchemaTaskAccess(c *gin.Context, taskID int) bool {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		response.Unauthorized(c, "unauthorized")
		return false
	}
	task, err := api.service.GetScanTaskStatus(taskID)
	if err != nil || task == nil {
		response.NotFound(c, "scan task not found")
		return false
	}
	if middleware.IsSuperAdmin(c) {
		return true
	}
	return api.requireSchemaSpaceAccess(c, task.SchemaSpaceID)
}

// --- Schema Space ---

// CreateSchemaSpace 创建Schema空间
func (api *SchemaAPI) CreateSchemaSpace(c *gin.Context) {
	var req struct {
		SpaceID int    `json:"space_id" binding:"required"`
		Name    string `json:"name" binding:"required"`
		DBType  string `json:"db_type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.DBType == "" {
		req.DBType = "mysql"
	}

	space, err := api.service.CreateSchemaSpace(req.SpaceID, req.Name, req.DBType)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, space)
}

// GetSchemaSpace 获取Schema空间
func (api *SchemaAPI) GetSchemaSpace(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	// 读取他人绑定库的结构化元数据本身就是跨租户泄漏，必须在返回前收口
	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	space, err := api.service.GetSchemaSpace(id)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, space)
}

// ListSchemaSpaces 列出Schema空间
func (api *SchemaAPI) ListSchemaSpaces(c *gin.Context) {
	page, pageSize := schemaListPagination(c)

	spaces, total, err := api.service.ListSchemaSpaces(page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"data":      spaces,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// DeleteSchemaSpace 删除Schema空间
func (api *SchemaAPI) DeleteSchemaSpace(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	if err := api.service.DeleteSchemaSpace(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// GetSchemaStats 获取Schema统计
func (api *SchemaAPI) GetSchemaStats(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	stats, err := api.service.GetSchemaStats(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, stats)
}

// --- Scan Task ---

// StartScan 启动扫描
func (api *SchemaAPI) StartScan(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	task, err := api.service.StartScan(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, task)
}

// GetScanTaskStatus 获取扫描任务状态
func (api *SchemaAPI) GetScanTaskStatus(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "invalid task_id")
		return
	}

	task, err := api.service.GetScanTaskStatus(taskID)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}
	if !api.requireSchemaTaskAccess(c, taskID) {
		return
	}

	response.Success(c, task)
}

// ListScanTasks 列出扫描任务
func (api *SchemaAPI) ListScanTasks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	tasks, err := api.service.ListScanTasks(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tasks)
}

// DeleteScanTask 删除扫描任务
func (api *SchemaAPI) DeleteScanTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "invalid task_id")
		return
	}

	if !api.requireSchemaTaskAccess(c, taskID) {
		return
	}
	if err := api.service.DeleteScanTask(taskID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// DeleteAllScanTasks 删除所有扫描任务
func (api *SchemaAPI) DeleteAllScanTasks(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	if err := api.service.DeleteAllScanTasks(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// PauseScanTask 暂停扫描任务
func (api *SchemaAPI) PauseScanTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "invalid task_id")
		return
	}

	if !api.requireSchemaTaskAccess(c, taskID) {
		return
	}
	if err := api.service.PauseScanTask(taskID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// ResumeScanTask 恢复扫描任务
func (api *SchemaAPI) ResumeScanTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "invalid task_id")
		return
	}

	if !api.requireSchemaTaskAccess(c, taskID) {
		return
	}
	if err := api.service.ResumeScanTask(taskID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// CancelScanTask 取消扫描任务
func (api *SchemaAPI) CancelScanTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("task_id"))
	if err != nil {
		response.BadRequest(c, "invalid task_id")
		return
	}

	if !api.requireSchemaTaskAccess(c, taskID) {
		return
	}
	if err := api.service.CancelScanTask(taskID); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	response.Success(c, nil)
}

// DebugScan 调试扫描（同步执行，返回详细日志）
func (api *SchemaAPI) DebugScan(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	result, err := api.service.DebugScan(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// --- Table ---

// ListTables 列出表
func (api *SchemaAPI) ListTables(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	keyword := c.Query("keyword")
	page, pageSize := schemaListPagination(c)

	tables, total, err := api.service.ListTables(id, keyword, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"data":      tables,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetTableDetail 获取表详情
func (api *SchemaAPI) GetTableDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	tableName := c.Param("table_name")

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	table, columns, relations, err := api.service.GetTableDetail(id, tableName)
	if err != nil {
		response.NotFound(c, err.Error())
		return
	}

	response.Success(c, gin.H{
		"table":     table,
		"columns":   columns,
		"relations": relations,
	})
}

// --- Ask ---

// AskQuestion 问答生成SQL（支持多轮对话历史）
func (api *SchemaAPI) AskQuestion(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		Question string                         `json:"question" binding:"required"`
		History  []text2sql.ConversationMessage `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	// AskQuestion 会在该 SchemaSpace 绑定的数据源上生成并执行查询，
	// 不校验归属等于借别人的数据库连接读他人业务库
	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}

	var result *aimodel.SQLResult
	if len(req.History) > 0 {
		// 多轮对话模式：将历史上下文注入生成器
		result, err = api.service.AskQuestionWithHistory(c.Request.Context(), id, req.Question, req.History)
	} else {
		result, err = api.service.AskQuestion(c.Request.Context(), id, req.Question)
	}
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// ListTableCache 列出表缓存（按 SchemaSpace 作用域）
func (api *SchemaAPI) ListTableCache(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	keyword := c.Query("keyword")
	page, pageSize := schemaListPagination(c)

	caches, total, err := api.service.ListTableCache(id, keyword, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"data":      caches,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ListBusinessRules 列出业务规则（按 SchemaSpace 作用域）
func (api *SchemaAPI) ListBusinessRules(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	page, pageSize := schemaListPagination(c)

	rules, total, err := api.service.ListBusinessRules(id, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"data":      rules,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// CreateBusinessRule 创建业务规则（按 SchemaSpace 作用域）
func (api *SchemaAPI) CreateBusinessRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req struct {
		Name        string `json:"name" binding:"required"`
		Type        string `json:"type" binding:"required"`
		TableName   string `json:"table_name" binding:"required"`
		FieldName   string `json:"field_name"`
		Content     string `json:"content" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}

	rule := &aimodel.BusinessRule{
		SchemaSpaceID: uint64(id),
		Name:          req.Name,
		Type:          req.Type,
		TableNam:      req.TableName,
		FieldName:     req.FieldName,
		Content:       req.Content,
		Description:   req.Description,
		Status:        1,
	}

	if err := api.service.CreateBusinessRule(rule); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, rule)
}

// DeleteBusinessRule 删除业务规则（按 SchemaSpace 作用域）
func (api *SchemaAPI) DeleteBusinessRule(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	ruleID, err := strconv.ParseUint(c.Param("rule_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid rule_id")
		return
	}

	// 服务层 DeleteBusinessRule 只按 rule_id 删除，不带 schema_space_id 条件，
	// 因此在 handler 内确认该规则确实属于路径中的 :id 空间，否则是典型 IDOR 删除
	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	rule, err := api.service.TextToSQLGetRule(ruleID)
	if err != nil || rule == nil {
		response.NotFound(c, "business rule not found")
		return
	}
	if int(rule.SchemaSpaceID) != id {
		response.NotFound(c, "business rule not found in this schema space")
		return
	}
	if err := api.service.DeleteBusinessRule(ruleID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// RefreshKnowledge 刷新知识库（按 SchemaSpace 作用域）
func (api *SchemaAPI) RefreshKnowledge(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if !api.requireSchemaSpaceAccess(c, id) {
		return
	}
	if err := api.service.RefreshKnowledge(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// ExportCSV 导出 SQL 查询结果为 CSV 文件
func (api *SchemaAPI) ExportCSV(c *gin.Context) {
	var req struct {
		Columns []string         `json:"columns"`
		Rows    []map[string]any `json:"rows"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if len(req.Columns) == 0 {
		response.BadRequest(c, "columns is required")
		return
	}

	// P3：限制 Columns 和 Rows 数量，避免恶意大请求体耗尽内存
	if len(req.Columns) > 1000 {
		response.BadRequest(c, "too many columns (max 1000)")
		return
	}
	if len(req.Rows) > 100000 {
		response.BadRequest(c, "too many rows (max 100000)")
		return
	}

	// Build CSV with UTF-8 BOM for Excel compatibility
	var buf bytes.Buffer
	// UTF-8 BOM
	buf.WriteString("\xef\xbb\xbf")

	writer := csv.NewWriter(&buf)

	// Header row
	if err := writer.Write(req.Columns); err != nil {
		response.ServerErrorSafe(c, fmt.Errorf("failed to write CSV header: %w", err))
		return
	}

	// Data rows
	for _, row := range req.Rows {
		record := make([]string, len(req.Columns))
		for i, col := range req.Columns {
			if v, ok := row[col]; ok && v != nil {
				record[i] = fmt.Sprintf("%v", v)
			}
		}
		if err := writer.Write(record); err != nil {
			response.ServerErrorSafe(c, fmt.Errorf("failed to write CSV row: %w", err))
			return
		}
	}
	writer.Flush()

	if err := writer.Error(); err != nil {
		response.ServerErrorSafe(c, fmt.Errorf("CSV flush error: %w", err))
		return
	}

	filename := fmt.Sprintf("query_result_%s.csv", time.Now().Format("20060102_150405"))

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, "text/csv; charset=utf-8", buf.Bytes())
}
