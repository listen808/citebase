package knowledge

import (
	"strconv"

	"github.com/gin-gonic/gin"
	logx "github.com/listen808/citebase/internal/log"
	commonmodel "github.com/listen808/citebase/internal/model/common"
	"github.com/listen808/citebase/internal/response"
	knowledgesvc "github.com/listen808/citebase/internal/service/knowledge"
	"github.com/listen808/citebase/internal/utils"
)

type AIAnalysisResultAPI struct {
	analysisService *knowledgesvc.AIAnalysisService
}

func NewAIAnalysisResultAPI() *AIAnalysisResultAPI {
	return &AIAnalysisResultAPI{
		analysisService: knowledgesvc.NewAIAnalysisService(),
	}
}

func (api *AIAnalysisResultAPI) GetResults(c *gin.Context) {
	spaceIDStr := c.Param("spaceId")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil {
		response.BadRequest(c, "invalid space_id")
		return
	}

	var pq commonmodel.PageQuery
	pgNum, pgSize, _ := utils.ParsePagination(c, 20)
	pq.PageNum, pq.PageSize = pgNum, pgSize

	if spaceID <= 0 {
		response.Success(c, &commonmodel.PageResult[any]{
			List:     []any{},
			Total:    0,
			PageNum:  pq.PageNum,
			PageSize: pq.PageSize,
		})
		return
	}

	results, err := api.analysisService.GetResultsBySpaceID(spaceID, pq)
	if err != nil {
		logx.Errorf("[AnalysisAPI] GetResultsBySpaceID failed: spaceID=%d, page=%d, pageSize=%d, error=%v", spaceID, pq.PageNum, pq.PageSize, err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, results)
}

func (api *AIAnalysisResultAPI) GetResultByDocID(c *gin.Context) {
	docIDStr := c.Param("doc_id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil {
		response.BadRequest(c, "invalid doc_id")
		return
	}

	result, err := api.analysisService.GetResultByDocID(docID)
	if err != nil {
		response.NotFound(c, "analysis result not found")
		return
	}

	response.Success(c, result)
}

func (api *AIAnalysisResultAPI) GetCategories(c *gin.Context) {
	spaceIDStr := c.Param("spaceId")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil {
		response.BadRequest(c, "invalid space_id")
		return
	}
	if spaceID <= 0 {
		response.Success(c, []interface{}{})
		return
	}

	categories, err := api.analysisService.GetCategoryTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, categories)
}

func (api *AIAnalysisResultAPI) GetQualityStats(c *gin.Context) {
	spaceIDStr := c.Param("spaceId")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil {
		response.BadRequest(c, "invalid space_id")
		return
	}
	if spaceID <= 0 {
		response.Success(c, gin.H{"avg_score": 0, "total": 0, "low_quality": 0, "high_quality": 0})
		return
	}

	stats, err := api.analysisService.GetQualityStats(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, stats)
}

func (api *AIAnalysisResultAPI) GetKnowledgeGaps(c *gin.Context) {
	spaceIDStr := c.Param("spaceId")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil {
		response.BadRequest(c, "invalid space_id")
		return
	}
	if spaceID <= 0 {
		response.Success(c, []interface{}{})
		return
	}

	gaps, err := api.analysisService.GetKnowledgeGaps(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gaps)
}
