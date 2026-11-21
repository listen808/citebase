package ai

import (
	"errors"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

// SQLTrustAPI SQL 可信闭环 API
type SQLTrustAPI struct {
	trustSvc *aiSvc.SQLTrustService
}

func NewSQLTrustAPI(policyRepo *airepo.SQLSecurityPolicyRepo, historyRepo *airepo.SQLHistoryRepo, auditRepo *airepo.AIAuditLogRepository) *SQLTrustAPI {
	return &SQLTrustAPI{
		trustSvc: aiSvc.NewSQLTrustService(policyRepo, historyRepo, auditRepo),
	}
}

// GetSQLSecurityPolicy 获取空间生效安全策略
func (api *SQLTrustAPI) GetSQLSecurityPolicy(c *gin.Context) {
	spaceID, err := strconv.ParseUint(c.Query("space_id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgSpaceIDInvalid)
		return
	}

	policy, err := api.trustSvc.GetSQLSecurityPolicy(spaceID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "获取策略失败")
		return
	}
	response.Success(c, policy)
}

// UpdateSQLSecurityPolicy 更新空间安全策略
func (api *SQLTrustAPI) UpdateSQLSecurityPolicy(c *gin.Context) {
	var req modelai.SQLSecurityPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgInvalidParameter)
		return
	}

	if err := api.trustSvc.UpdateSQLSecurityPolicy(&req); err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "保存策略失败")
		return
	}
	response.Success(c, req)
}

// SubmitSQLFeedback 提交 SQL 反馈
func (api *SQLTrustAPI) SubmitSQLFeedback(c *gin.Context) {
	var req aiSvc.SubmitSQLFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgInvalidParameter)
		return
	}

	if err := api.trustSvc.SubmitSQLFeedback(&req); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"history_id": req.HistoryID})
}

// GetSQLExplanation 获取 SQL 解释
func (api *SQLTrustAPI) GetSQLExplanation(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgHistoryIDInvalid)
		return
	}

	explanation, err := api.trustSvc.GetSQLExplanation(id)
	if err != nil {
		// 按键分派而不是回显 err.Error()：原样回出的话，英文界面必读到中文，
		// 而 GORM 原文本来也只该进日志。
		switch {
		case errors.Is(err, aiSvc.ErrSQLHistoryNotFound):
			response.NotFoundKey(c, response.MsgHistoryNotFound)
		case errors.Is(err, aiSvc.ErrSQLHistoryQueryFailed):
			response.ServerErrorKey(c, response.MsgHistoryQueryFailed)
		case errors.Is(err, aiSvc.ErrSQLExplanationInvalid):
			response.ServerErrorKey(c, response.MsgExplanationParseFailed)
		default:
			response.ServerErrorSafe(c, err)
			return
		}
		logx.Warnf("[SQLTrust] 取 SQL 解释失败 id=%d: %v", id, err)
		return
	}
	response.Success(c, gin.H{"explanation": explanation})
}

// ListSQLAuditTraces 查询 Text-to-SQL 审计链路
func (api *SQLTrustAPI) ListSQLAuditTraces(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	logs, total, err := api.trustSvc.ListSQLAuditTraces(limit, offset)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "查询审计日志失败")
		return
	}

	response.Success(c, gin.H{
		"list":  logs,
		"total": total,
	})
}

// GetSQLAuditTrace 查询单条审计详情
func (api *SQLTrustAPI) GetSQLAuditTrace(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequestKey(c, response.MsgIDParamInvalid)
		return
	}

	log, err := api.trustSvc.GetSQLAuditTrace(id)
	if err != nil {
		if errors.Is(err, aiSvc.ErrSQLAuditTraceNotFound) {
			response.NotFoundKey(c, response.MsgAuditRecordNotFound)
		} else {
			response.ServerErrorSafe(c, err)
		}
		return
	}

	response.Success(c, log)
}

func (api *SQLTrustAPI) notImplemented(c *gin.Context) {
	response.Error(c, http.StatusNotImplemented, 501, fmt.Sprintf("%s 未实现", c.Request.URL.Path))
}
