package ai

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

// PromptVersionAPI Prompt 版本 API
type PromptVersionAPI struct {
	svc *aiSvc.Text2SQLService
}

// NewPromptVersionAPI 创建 Prompt 版本 API
func NewPromptVersionAPI() *PromptVersionAPI {
	return &PromptVersionAPI{
		svc: aiSvc.NewText2SQLService(
			nil,
			airepo.NewPromptVersionRepo(repos.DB),
			nil, nil,
		),
	}
}

// List GET /api/v1/ai/data-query/prompt-versions
func (api *PromptVersionAPI) List(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	list, err := api.svc.ListPromptVersions(spaceID)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, list)
}

// Create POST /api/v1/ai/data-query/prompt-versions
func (api *PromptVersionAPI) Create(c *gin.Context) {
	var p aimodel.PromptVersion
	if err := c.ShouldBindJSON(&p); err != nil {
		response.FailSafe(c, http.StatusBadRequest, "invalid params", err)
		return
	}
	if err := api.svc.CreatePromptVersion(&p); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "create failed", err)
		return
	}
	response.Success(c, p)
}

// Update PUT /api/v1/ai/data-query/prompt-versions/:id
func (api *PromptVersionAPI) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var p aimodel.PromptVersion
	if err := c.ShouldBindJSON(&p); err != nil {
		response.FailSafe(c, http.StatusBadRequest, "invalid params", err)
		return
	}
	if err := api.svc.UpdatePromptVersion(id, &p); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "update failed", err)
		return
	}
	response.Success(c, p)
}

// Delete DELETE /api/v1/ai/data-query/prompt-versions/:id
func (api *PromptVersionAPI) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := api.svc.DeletePromptVersion(id); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "delete failed", err)
		return
	}
	response.Success(c, nil)
}

// Activate POST /api/v1/ai/data-query/prompt-versions/:id/activate
func (api *PromptVersionAPI) Activate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := api.svc.ActivatePromptVersion(id); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "activate failed", err)
		return
	}
	response.Success(c, gin.H{"message": "activated"})
}
