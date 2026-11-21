package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils"
)

// LogAPI 日志管理 API
type LogAPI struct {
	logService *adminsvc.LogService
}

func NewLogAPI() *LogAPI {
	return &LogAPI{logService: adminsvc.NewLogService()}
}

// GetDocumentHistory 获取文档的操作历史
// @Summary 文档操作历史
// @Tags 日志
// @Produce json
// @Param id path int true "文档 ID"
// @Param limit query int false "返回数量" default(20)
// @Success 200 {array} object
// @Router /api/v1/documents/{id}/history [get]
func (api *LogAPI) GetDocumentHistory(c *gin.Context) {
	idStr := c.Param("id")
	docID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	history, err := api.logService.GetDocumentHistory(docID, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, history)
}

// GetUserActivity 获取用户活动记录
// @Summary 用户活动记录
// @Tags 日志
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param action query string false "操作类型筛选 create/edit/delete"
// @Success 200 {object} object
// @Router /api/v1/logs/activity [get]
func (api *LogAPI) GetUserActivity(c *gin.Context) {
	_, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	page, pageSize, _ := utils.ParsePagination(c, 20)
	actionFilter := c.Query("action")

	if pageSize > 100 {
		pageSize = 100
	}

	var activity []*adminmodel.DocumentLogResponse
	var total int64

	if actionFilter != "" {
		activity, total, err = api.logService.GetUserActivityFiltered(0, actionFilter, page, pageSize)
	} else {
		activity, total, err = api.logService.GetUserActivity(0, page, pageSize)
	}

	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 转换为前端期望的格式
	type ActivityItem struct {
		ID           int    `json:"id"`
		UserID       int    `json:"user_id"`
		Username     string `json:"username,omitempty"`
		GivenName    string `json:"given_name,omitempty"`
		DocumentID   int    `json:"document_id"`
		SpaceID      int    `json:"space_id"`
		DocumentName string `json:"document_name,omitempty"`
		SpaceName    string `json:"space_name,omitempty"`
		ChangeLog    string `json:"change_log,omitempty"`
		Action       string `json:"action"`      // "create", "edit", "delete"
		TargetType   string `json:"target_type"` // "document", "space", "user"
		TargetID     int    `json:"target_id"`
		Detail       string `json:"detail,omitempty"`
		CreateTime   int    `json:"create_time"`
	}

	activityItems := make([]*ActivityItem, len(activity))
	for i, log := range activity {
		// 转换 action 为字符串
		var actionStr string
		switch log.Action {
		case adminmodel.DocActionCreate:
			actionStr = "create"
		case adminmodel.DocActionModify:
			actionStr = "edit"
		case adminmodel.DocActionDelete:
			actionStr = "delete"
		case adminmodel.DocActionRestore:
			actionStr = "restore"
		default:
			actionStr = "unknown"
		}

		// 判断目标类型
		var targetType string
		var targetID int
		if log.DocumentID > 0 {
			targetType = "document"
			targetID = log.DocumentID
		} else if log.SpaceID > 0 {
			targetType = "space"
			targetID = log.SpaceID
		} else {
			targetType = "unknown"
			targetID = log.ID
		}

		activityItems[i] = &ActivityItem{
			ID:           log.ID,
			UserID:       log.UserID,
			Username:     log.Username,
			GivenName:    log.GivenName,
			DocumentID:   log.DocumentID,
			SpaceID:      log.SpaceID,
			DocumentName: log.DocumentName,
			SpaceName:    log.SpaceName,
			ChangeLog:    log.Comment,
			Action:       actionStr,
			TargetType:   targetType,
			TargetID:     targetID,
			Detail:       log.Comment,
			CreateTime:   log.CreateTime,
		}
	}

	response.Paginated(c, activityItems, total, page, pageSize)
}

// GetUserActivityByID 获取指定用户的活动记录（管理员或查看其他用户）
// @Summary 指定用户活动记录
// @Tags 日志
// @Produce json
// @Param id path int true "用户 ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} object
// @Router /api/v1/logs/users/{id}/activity [get]
func (api *LogAPI) GetUserActivityByID(c *gin.Context) {
	idStr := c.Param("id")
	targetUserID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	page, pageSize, _ := utils.ParsePagination(c, 20)

	if pageSize > 100 {
		pageSize = 100
	}

	activity, total, err := api.logService.GetUserActivity(targetUserID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, activity, total, page, pageSize)
}

// GetRecentErrors 获取最近错误日志（用于系统监控页）
// @Summary 最近错误日志
// @Tags 日志
// @Produce json
// @Param limit query int false "返回数量" default(5)
// @Success 200
// @Router /api/v1/logs/recent-errors [get]
func (api *LogAPI) GetRecentErrors(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit < 1 || limit > 20 {
		limit = 5
	}
	logs, err := api.logService.GetRecentErrorLogs(limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, logs)
}

// GetSystemLogs 获取系统日志（仅管理员）
// @Summary 系统日志
// @Tags 日志
// @Produce json
// @Param level query int false "日志级别 3=ERROR 4=WARN 6=INFO"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(50)
// @Success 200 {object} object
// @Router /api/v1/logs/system [get]
func (api *LogAPI) GetSystemLogs(c *gin.Context) {
	level, _ := strconv.Atoi(c.DefaultQuery("level", "0"))
	page, pageSize, _ := utils.ParsePagination(c, 50)

	if pageSize > 200 {
		pageSize = 200
	}

	logs, total, err := api.logService.GetSystemLogs(level, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, logs, total, page, pageSize)
}

// GetLogDetail 获取单条日志详情
func (api *LogAPI) GetLogDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid log id")
		return
	}

	log, err := api.logService.GetLogDetail(id)
	if err != nil {
		response.NotFound(c, "log not found")
		return
	}

	response.Success(c, log)
}

// GetActivityLogDetail 获取单条活动日志详情
func (api *LogAPI) GetActivityLogDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid log id")
		return
	}

	log, err := api.logService.GetLogDetail(id)
	if err != nil {
		response.NotFound(c, "activity log not found")
		return
	}

	response.Success(c, log)
}
