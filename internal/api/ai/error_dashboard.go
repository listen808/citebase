package ai

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

type ErrorDashboardAPI struct {
	svc *aiSvc.Text2SQLService
}

func NewErrorDashboardAPI() *ErrorDashboardAPI {
	return &ErrorDashboardAPI{
		svc: aiSvc.NewText2SQLService(
			nil, nil, nil,
			airepo.NewText2SQLFeedbackRepo(repos.DB),
		),
	}
}

// Summary GET /api/v1/ai/text2sql/error-summary
func (api *ErrorDashboardAPI) Summary(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	summary, err := api.svc.GetErrorSummary(spaceID)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, summary)
}

// RecentFailures GET /api/v1/ai/text2sql/recent-failures
func (api *ErrorDashboardAPI) RecentFailures(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := api.svc.GetRecentFailures(spaceID, limit)
	if err != nil {
		response.FailSafe(c, http.StatusInternalServerError, "query failed", err)
		return
	}
	response.Success(c, list)
}
