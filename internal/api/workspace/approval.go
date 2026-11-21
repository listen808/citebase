package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
)

// ApprovalAPI 文档审批流接口
type ApprovalAPI struct {
	approvalService *workspacesvc.ApprovalService
}

func NewApprovalAPI() *ApprovalAPI {
	return &ApprovalAPI{approvalService: workspacesvc.NewApprovalService()}
}

// SubmitApproval 提交审批
// @Summary 提交文档审批
// @Tags 文档
// @Accept json
// @Produce json
// @Param request body workspace.ApprovalRequest true "审批请求"
// @Success 200 {object} object
// @Router /api/v1/documents/approvals [post]
func (api *ApprovalAPI) SubmitApproval(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	var req workspacemodel.ApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	a, err := api.approvalService.Submit(userID, &req)
	if err != nil {
		response.ErrorWithErr(c, err)
		return
	}
	response.Success(c, a.ToResponse())
}

// HandleApproval 处理审批（通过/驳回）
// @Summary 处理审批
// @Tags 文档
// @Accept json
// @Produce json
// @Param id path int true "审批记录ID"
// @Param request body workspace.ApprovalHandleRequest true "审批处理"
// @Success 200 {object} object
// @Router /api/v1/documents/approvals/{id}/handle [post]
func (api *ApprovalAPI) HandleApproval(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid approval id")
		return
	}
	var req workspacemodel.ApprovalHandleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	a, err := api.approvalService.Handle(id, userID, &req)
	if err != nil {
		response.ErrorWithErr(c, err)
		return
	}
	response.Success(c, a.ToResponse())
}

// GetDocumentApproval 获取文档审批记录
// @Summary 获取文档审批
// @Tags 文档
// @Produce json
// @Param id path int true "文档ID"
// @Success 200 {object} object
// @Router /api/v1/documents/{id}/approval [get]
func (api *ApprovalAPI) GetDocumentApproval(c *gin.Context) {
	docID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	a, err := api.approvalService.GetByDocument(docID)
	if err != nil {
		response.NotFound(c, "no approval for this document")
		return
	}
	response.Success(c, a.ToResponse())
}

// ListPendingApprovals 待我审批
func (api *ApprovalAPI) ListPendingApprovals(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	page, pageSize, _ := utils.ParsePagination(c, 20)
	list, total, err := api.approvalService.ListPending(userID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, list, total, page, pageSize)
}

// ListSubmittedApprovals 我提交的
func (api *ApprovalAPI) ListSubmittedApprovals(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	page, pageSize, _ := utils.ParsePagination(c, 20)
	list, total, err := api.approvalService.ListSubmitted(userID, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Paginated(c, list, total, page, pageSize)
}
