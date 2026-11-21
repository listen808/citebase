package ai

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	commonmodel "github.com/listen808/citebase/internal/model/common"
	repos "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
	"github.com/listen808/citebase/internal/utils"
)

// SQLTraceAPI SQL Trace API
type SQLTraceAPI struct {
	svc *aiSvc.Text2SQLService
}

// NewSQLTraceAPI 创建 SQL Trace API
func NewSQLTraceAPI() *SQLTraceAPI {
	return &SQLTraceAPI{
		svc: aiSvc.NewText2SQLService(
			nil, nil,
			airepo.NewSQLTraceRepo(repos.DB),
			nil,
		),
	}
}

// GetTrace GET /api/v1/ai/text2sql/traces/:trace_id
func (api *SQLTraceAPI) GetTrace(c *gin.Context) {
	traceID := c.Param("trace_id")
	trace, err := api.svc.GetSQLTrace(traceID)
	if err != nil {
		response.ErrorWithErr(c, response.ErrNotFound, http.StatusNotFound)
		return
	}
	response.Success(c, trace)
}

// ListTraces GET /api/v1/ai/text2sql/traces
func (api *SQLTraceAPI) ListTraces(c *gin.Context) {
	spaceID, _ := strconv.ParseUint(c.Query("space_id"), 10, 64)

	var pq commonmodel.PageQuery
	pq.PageNum, pq.PageSize, _ = utils.ParsePagination(c, 20)

	since := time.Time{}
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			since = t
		}
	}

	list, total, err := api.svc.ListSQLTraces(spaceID, pq.PageNum, pq.PageSize, since)
	if err != nil {
		response.ErrorWithErr(c, response.ErrInternal, http.StatusInternalServerError)
		return
	}
	response.Success(c, &commonmodel.PageResult[aimodel.SQLTrace]{
		List:     list,
		Total:    total,
		PageNum:  pq.PageNum,
		PageSize: pq.PageSize,
	})
}
