package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// WebhookAPI handles webhook API requests.
type WebhookAPI struct {
	service *workspacesvc.WebhookService
}

func NewWebhookAPI() *WebhookAPI {
	return &WebhookAPI{
		service: workspacesvc.NewWebhookService(),
	}
}

// CreateWebhook creates a new webhook configuration.
func (api *WebhookAPI) CreateWebhook(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	var req workspace.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := api.service.CreateWebhook(int64(userID), &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// GetUserWebhooks returns all webhooks for the current user.
func (api *WebhookAPI) GetUserWebhooks(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	webhooks, err := api.service.GetUserWebhooks(int64(userID))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, webhooks)
}

// GetWebhook returns a webhook by ID.
func (api *WebhookAPI) GetWebhook(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	result, err := api.service.GetWebhook(int64(userID), id)
	if err != nil {
		response.NotFound(c, "webhook not found")
		return
	}

	response.Success(c, result)
}

// UpdateWebhook updates a webhook configuration.
func (api *WebhookAPI) UpdateWebhook(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req workspace.UpdateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := api.service.UpdateWebhook(int64(userID), id, &req)
	if err != nil {
		if err == workspacesvc.ErrWebhookNotFound {
			response.NotFound(c, "webhook not found")
			return
		}
		if err == workspacesvc.ErrWebhookNotOwner {
			response.Forbidden(c, "not your webhook")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// DeleteWebhook deletes a webhook configuration.
func (api *WebhookAPI) DeleteWebhook(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := api.service.DeleteWebhook(int64(userID), id); err != nil {
		if err == workspacesvc.ErrWebhookNotFound {
			response.NotFound(c, "webhook not found")
			return
		}
		if err == workspacesvc.ErrWebhookNotOwner {
			response.Forbidden(c, "not your webhook")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// GetWebhookDeliveries returns recent deliveries for a webhook.
func (api *WebhookAPI) GetWebhookDeliveries(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	limitStr := c.DefaultQuery("limit", "20")
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 100 {
		limit = 20
	}

	deliveries, err := api.service.GetWebhookDeliveries(int64(userID), id, limit)
	if err != nil {
		if err == workspacesvc.ErrWebhookNotFound {
			response.NotFound(c, "webhook not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, deliveries)
}

// GetSupportedEvents returns the list of supported webhook events.
func (api *WebhookAPI) GetSupportedEvents(c *gin.Context) {
	events := workspacesvc.SupportedWebhookEvents()
	response.Success(c, events)
}
