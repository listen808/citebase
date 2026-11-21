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

// ABExperimentAPI A/B 实验 API
type ABExperimentAPI struct {
	svc *aiSvc.Text2SQLService
}

// NewABExperimentAPI 创建 A/B 实验 API
func NewABExperimentAPI() *ABExperimentAPI {
	return &ABExperimentAPI{
		svc: aiSvc.NewText2SQLService(
			airepo.NewABExperimentRepo(repos.DB),
			nil, nil, nil,
		),
	}
}

// List GET /api/v1/ai/data-query/ab-experiments
func (api *ABExperimentAPI) List(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	list, err := api.svc.ListABExperiments(spaceID)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, list)
}

// Create POST /api/v1/ai/data-query/ab-experiments
func (api *ABExperimentAPI) Create(c *gin.Context) {
	var e aimodel.ABExperiment
	if err := c.ShouldBindJSON(&e); err != nil {
		response.FailSafe(c, http.StatusBadRequest, "invalid params", err)
		return
	}
	if err := api.svc.CreateABExperiment(&e); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "create failed", err)
		return
	}
	response.Success(c, e)
}

// Update PUT /api/v1/ai/data-query/ab-experiments/:id
func (api *ABExperimentAPI) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var e aimodel.ABExperiment
	if err := c.ShouldBindJSON(&e); err != nil {
		response.FailSafe(c, http.StatusBadRequest, "invalid params", err)
		return
	}
	if err := api.svc.UpdateABExperiment(id, &e); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "update failed", err)
		return
	}
	response.Success(c, e)
}

// Delete DELETE /api/v1/ai/data-query/ab-experiments/:id
func (api *ABExperimentAPI) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := api.svc.DeleteABExperiment(id); err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "delete failed", err)
		return
	}
	response.Success(c, nil)
}

// Get GET /api/v1/ai/data-query/ab-experiments/:id
func (api *ABExperimentAPI) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	e, err := api.svc.GetABExperiment(id)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, e)
}
