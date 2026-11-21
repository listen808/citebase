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
	"github.com/listen808/citebase/internal/utils"
)

// SemanticLayerAPI 语义层 API
type SemanticLayerAPI struct {
	semanticSvc *aiSvc.SemanticService
}

// NewSemanticLayerAPI 创建语义层 API
func NewSemanticLayerAPI() *SemanticLayerAPI {
	metricRepo := airepo.NewSemanticMetricRepo(repos.DB)
	dimensionRepo := airepo.NewSemanticDimensionRepo(repos.DB)
	return &SemanticLayerAPI{
		semanticSvc: aiSvc.NewSemanticService(metricRepo, dimensionRepo),
	}
}

// ListMetrics GET /api/v1/ai/semantic-metrics
func (api *SemanticLayerAPI) ListMetrics(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	page, pageSize, _ := utils.ParsePagination(c, 20)

	list, total, err := api.semanticSvc.ListMetrics(spaceID, page, pageSize)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total})
}

// CreateMetric POST /api/v1/ai/semantic-metrics
func (api *SemanticLayerAPI) CreateMetric(c *gin.Context) {
	var m aimodel.SemanticMetric
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "invalid params: "+err.Error())
		return
	}
	if err := api.semanticSvc.CreateMetric(&m); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, m)
}

// UpdateMetric PUT /api/v1/ai/semantic-metrics/:id
func (api *SemanticLayerAPI) UpdateMetric(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m aimodel.SemanticMetric
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "invalid params: "+err.Error())
		return
	}
	if err := api.semanticSvc.UpdateMetric(id, &m); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, m)
}

// DeleteMetric DELETE /api/v1/ai/semantic-metrics/:id
func (api *SemanticLayerAPI) DeleteMetric(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := api.semanticSvc.DeleteMetric(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}

// ListDimensions GET /api/v1/ai/semantic-dimensions
func (api *SemanticLayerAPI) ListDimensions(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	page, pageSize, _ := utils.ParsePagination(c, 20)

	list, total, err := api.semanticSvc.ListDimensions(spaceID, page, pageSize)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, gin.H{"list": list, "total": total})
}

// CreateDimension POST /api/v1/ai/semantic-dimensions
func (api *SemanticLayerAPI) CreateDimension(c *gin.Context) {
	var d aimodel.SemanticDimension
	if err := c.ShouldBindJSON(&d); err != nil {
		response.BadRequest(c, "invalid params: "+err.Error())
		return
	}
	if err := api.semanticSvc.CreateDimension(&d); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, d)
}

// DeleteDimension DELETE /api/v1/ai/semantic-dimensions/:id
func (api *SemanticLayerAPI) DeleteDimension(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := api.semanticSvc.DeleteDimension(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}
