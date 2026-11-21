package texttosql

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
	"github.com/listen808/citebase/internal/utils"
)

// DataSourceAPI 数据源管理 API
type DataSourceAPI struct {
	svc *aiSvc.DataSourceService
}

// NewDataSourceAPI 创建数据源管理 API
func NewDataSourceAPI() *DataSourceAPI {
	return &DataSourceAPI{
		svc: aiSvc.NewDataSourceService(airepo.NewDataSourceRepo(repository.DB)),
	}
}

// Create 创建数据源
func (a *DataSourceAPI) Create(c *gin.Context) {
	var req aiSvc.CreateDataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ds, err := a.svc.Create(&req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Created(c, ds)
}

// Update 更新数据源
func (a *DataSourceAPI) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidDataSourceID)
		return
	}
	var req aiSvc.CreateDataSourceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	ds, err := a.svc.Update(id, &req)
	if err != nil {
		if errors.Is(err, aiSvc.ErrDataSourceNotFound) {
			response.NotFoundKey(c, response.MsgDataSourceNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, ds)
}

// Delete 删除数据源
func (a *DataSourceAPI) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidDataSourceID)
		return
	}
	if err := a.svc.Delete(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}

// List 列出数据源
func (a *DataSourceAPI) List(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	list, err := a.svc.List(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, list)
}

// Get 获取数据源详情
func (a *DataSourceAPI) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidDataSourceID)
		return
	}
	ds, err := a.svc.Get(id)
	if err != nil {
		if errors.Is(err, aiSvc.ErrDataSourceNotFound) {
			response.NotFoundKey(c, response.MsgDataSourceNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, ds)
}

// TestConnection 测试数据库连接
func (a *DataSourceAPI) TestConnection(c *gin.Context) {
	var req aiSvc.TestConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := a.svc.TestConnection(&req); err != nil {
		// 这一句的 detail 是给"刚填完连接参数"的管理员看的（驱动说的是 host/port/口令），
		// 句子本身仍然按键翻译 —— 与 error_leak 那条不冲突：这里的配置是他自己填的。
		response.BadRequestKeyDetail(c, response.MsgDataSourceConnectFailed, err.Error())
		return
	}
	response.Success(c, gin.H{"message": "连接成功"})
}

// Sync 同步数据源表结构
func (a *DataSourceAPI) Sync(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidDataSourceID)
		return
	}
	result, err := a.svc.Sync(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, result)
}

// SQLHistoryAPI SQL 历史记录 API
type SQLHistoryAPI struct {
	svc *aiSvc.SQLHistoryService
}

// NewSQLHistoryAPI 创建 SQL 历史记录 API
func NewSQLHistoryAPI() *SQLHistoryAPI {
	return &SQLHistoryAPI{
		svc: aiSvc.NewSQLHistoryService(airepo.NewSQLHistoryRepo(repository.DB)),
	}
}

// sessionUser 取登录态用户 ID；缺失时已写出 401，调用方只需 return。
// 「我的 SQL 历史 / 记忆 / 数据源」这类归属判定绝不能取自查询参数，否则等于任意越权。
func sessionUser(c *gin.Context) (uint64, bool) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		response.Unauthorized(c, "unauthorized")
		return 0, false
	}
	return uint64(uid), true
}

// List 列出 SQL 历史
func (a *SQLHistoryAPI) List(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	userID, ok := sessionUser(c)
	if !ok {
		return
	}
	page, pageSize, _ := utils.ParsePagination(c, 20)

	list, total, err := a.svc.List(spaceID, userID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, list, total, page, pageSize)
}

// ListFavorites 列出收藏的 SQL
func (a *SQLHistoryAPI) ListFavorites(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	userID, ok := sessionUser(c)
	if !ok {
		return
	}
	page, pageSize, _ := utils.ParsePagination(c, 20)

	list, total, err := a.svc.ListFavorites(spaceID, userID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, list, total, page, pageSize)
}

// ToggleFavorite 切换收藏状态
func (a *SQLHistoryAPI) ToggleFavorite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidNumericID)
		return
	}
	actor, ok := sessionUser(c)
	if !ok {
		return
	}
	if err := a.svc.ToggleFavorite(id, actor); err != nil {
		if err == aiSvc.ErrSQLHistoryForbidden {
			response.ForbiddenKey(c, response.MsgNoPermissionSQLHistory)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"message": "操作成功"})
}

// Rate 评分 SQL 历史
func (a *SQLHistoryAPI) Rate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidNumericID)
		return
	}
	var req struct {
		Rating int8 `json:"rating" binding:"required,min=1,max=5"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	actor, ok := sessionUser(c)
	if !ok {
		return
	}
	if err := a.svc.Rate(id, actor, req.Rating); err != nil {
		if err == aiSvc.ErrSQLHistoryForbidden {
			response.ForbiddenKey(c, response.MsgNoPermissionSQLHistory)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// P0-6：评分回流到模式成功率——低分的下次不再优先复用。
	if mm := aimod.GetModule().MemoryManager(); mm != nil {
		if h, gerr := a.svc.Get(id, actor); gerr == nil && h != nil {
			sql := h.ExecutedSQL
			if sql == "" {
				sql = h.GeneratedSQL
			}
			if sql != "" {
				mm.LearnFromFeedback(actor, h.QueryText, sql, req.Rating >= 4)
			}
		}
	}

	response.Success(c, gin.H{"message": "评分成功"})
}

// Save 保存 SQL 历史
func (a *SQLHistoryAPI) Save(c *gin.Context) {
	var req aiSvc.SaveSQLHistoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	userID, ok := sessionUser(c)
	if !ok {
		return
	}
	h, err := a.svc.Save(userID, &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// P0-6：把成功的 SQL 查询沉淀成查询模式。MemoryManager 的 LearnFromQuery /
	// GetRelevantPatterns 此前零调用点，跨会话学习实际从未发生过。
	if mm := aimod.GetModule().MemoryManager(); mm != nil && req.Status != "failed" {
		sql := req.ExecutedSQL
		if sql == "" {
			sql = req.GeneratedSQL
		}
		if sql != "" {
			mm.LearnFromQuery(userID, req.QueryText, sql, req.SpaceID)
		}
	}

	response.Created(c, h)
}

// Delete 删除 SQL 历史
func (a *SQLHistoryAPI) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidNumericID)
		return
	}
	actor, ok := sessionUser(c)
	if !ok {
		return
	}
	if err := a.svc.Delete(id, actor); err != nil {
		if err == aiSvc.ErrSQLHistoryForbidden {
			response.ForbiddenKey(c, response.MsgNoPermissionSQLHistory)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}

// Get 获取 SQL 历史详情
func (a *SQLHistoryAPI) Get(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidNumericID)
		return
	}
	actor, ok := sessionUser(c)
	if !ok {
		return
	}
	h, err := a.svc.Get(id, actor)
	if err != nil {
		if err == aiSvc.ErrSQLHistoryForbidden {
			response.ForbiddenKey(c, response.MsgNoPermissionSQLHistoryRead)
			return
		}
		if errors.Is(err, aiSvc.ErrSQLHistoryNotFound) {
			response.NotFoundKey(c, response.MsgRecordNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, h)
}

// Clear 清除 SQL 历史
func (a *SQLHistoryAPI) Clear(c *gin.Context) {
	userID, ok := sessionUser(c)
	if !ok {
		return
	}
	olderThanDays, _ := strconv.Atoi(c.DefaultQuery("older_than_days", "30"))
	clearAll := c.DefaultQuery("clear_all", "false") == "true"

	// clear_all 会跨用户删除全库历史：必须以「角色是否为超级管理员」判定，
	// 旧实现用 `uid == 1` 判断，等于「用户 ID 恰好是 1 的人」拿到全局删除权限。
	result, err := a.svc.Clear(userID, olderThanDays, clearAll, middleware.IsSuperAdmin(c))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, result)
}
