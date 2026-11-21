package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils"
)

// EmailAPI 邮件服务器管理 API
type EmailAPI struct {
	emailService *adminsvc.EmailService
}

func NewEmailAPI() *EmailAPI {
	return &EmailAPI{
		emailService: adminsvc.NewEmailService(),
	}
}

// GetEmails 获取邮件服务器列表
// @Summary 邮件服务器列表
// @Tags 邮件管理
// @Produce json
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param keyword query string false "搜索关键词"
// @Success 200 {object} object
// @Router /api/v1/emails [get]
func (api *EmailAPI) GetEmails(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.DefaultQuery("keyword", "")

	emails, total, err := api.emailService.GetEmailsByPage(page, pageSize, keyword)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, emails, total, page, pageSize)
}

// CreateEmail 创建邮件服务器
// @Summary 创建邮件服务器
// @Tags 邮件管理
// @Accept json
// @Produce json
// @Param body body object true "邮件服务器配置"
// @Success 201 {object} object
// @Router /api/v1/emails [post]
func (api *EmailAPI) CreateEmail(c *gin.Context) {
	var req adminmodel.CreateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	email, err := api.emailService.CreateEmail(&req)
	if err != nil {
		if err == adminsvc.ErrEmailNameExists {
			response.ConflictKey(c, response.MsgEmailServerNameTaken)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, email.ToResponse())
}

// GetEmail 获取单个邮件服务器
// @Summary 获取邮件服务器详情
// @Tags 邮件管理
// @Produce json
// @Param id path int true "邮件服务器 ID"
// @Success 200 {object} object
// @Router /api/v1/emails/{id} [get]
func (api *EmailAPI) GetEmail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	email, err := api.emailService.GetEmailByID(id)
	if err != nil {
		if err == adminsvc.ErrEmailNotFound {
			response.ErrorWithErr(c, response.ErrNotFound, http.StatusNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, email)
}

// UpdateEmail 更新邮件服务器
// @Summary 更新邮件服务器
// @Tags 邮件管理
// @Accept json
// @Produce json
// @Param id path int true "邮件服务器 ID"
// @Param body body object true "更新内容"
// @Success 200 {object} object
// @Router /api/v1/emails/{id} [put]
func (api *EmailAPI) UpdateEmail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	var req adminmodel.UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	email, err := api.emailService.UpdateEmail(id, &req)
	if err != nil {
		if err == adminsvc.ErrEmailNotFound {
			response.ErrorWithErr(c, response.ErrNotFound, http.StatusNotFound)
			return
		}
		if err == adminsvc.ErrEmailNameExists {
			response.ConflictKey(c, response.MsgEmailServerNameTaken)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, email)
}

// DeleteEmail 删除邮件服务器
// @Summary 删除邮件服务器
// @Tags 邮件管理
// @Produce json
// @Param id path int true "邮件服务器 ID"
// @Success 200
// @Router /api/v1/emails/{id} [delete]
func (api *EmailAPI) DeleteEmail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	if err := api.emailService.DeleteEmail(id); err != nil {
		if err == adminsvc.ErrEmailNotFound {
			response.ErrorWithErr(c, response.ErrNotFound, http.StatusNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}

// SetDefaultEmail 设为默认邮件服务器
// @Summary 设为默认
// @Tags 邮件管理
// @Produce json
// @Param id path int true "邮件服务器 ID"
// @Success 200
// @Router /api/v1/emails/{id}/default [put]
func (api *EmailAPI) SetDefaultEmail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	if err := api.emailService.SetDefaultEmail(id); err != nil {
		if err == adminsvc.ErrEmailNotFound {
			response.ErrorWithErr(c, response.ErrNotFound, http.StatusNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "设置成功")
}

// TestEmail 发送测试邮件
// @Summary 发送测试邮件
// @Tags 邮件管理
// @Accept json
// @Produce json
// @Param body body object true "测试邮件内容"
// @Success 200
// @Router /api/v1/emails/test [post]
func (api *EmailAPI) TestEmail(c *gin.Context) {
	var req adminmodel.TestEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithErr(c, response.ErrBadRequest)
		return
	}

	if err := api.emailService.TestEmail(&req); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "测试邮件发送成功")
}
