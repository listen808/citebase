package ai

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	dbrepo "github.com/listen808/citebase/internal/repository"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

type DataSensitivityAPI struct {
	svc *aiSvc.DataSensitivityService
}

func NewDataSensitivityAPI() *DataSensitivityAPI {
	return &DataSensitivityAPI{
		svc: aiSvc.NewDataSensitivityService(airepo.NewDataSensitivityRepo(dbrepo.DB)),
	}
}

func (api *DataSensitivityAPI) CreateRule(c *gin.Context) {
	var req aiSvc.CreateSensitivityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgInvalidParameter)
		return
	}
	rule, err := api.svc.CreateRule(&req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, rule)
}

func (api *DataSensitivityAPI) UpdateRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgIDInvalid)
		return
	}
	var req aiSvc.CreateSensitivityRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgInvalidParameter)
		return
	}
	rule, err := api.svc.UpdateRule(id, &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, rule)
}

func (api *DataSensitivityAPI) DeleteRule(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgIDInvalid)
		return
	}
	if err := api.svc.DeleteRule(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (api *DataSensitivityAPI) ListRules(c *gin.Context) {
	spaceIDStr := c.Query("space_id")
	spaceID, err := strconv.ParseUint(spaceIDStr, 10, 64)
	if err != nil {
		response.BadRequestKey(c, response.MsgSpaceIDInvalid)
		return
	}
	rules, err := api.svc.ListRules(spaceID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, 500, "查询失败")
		return
	}
	response.Success(c, rules)
}

func (api *DataSensitivityAPI) DetectPII(c *gin.Context) {
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgTextEmpty)
		return
	}
	result := api.svc.DetectPII(req.Text)
	response.Success(c, result)
}

// 保留 aimodel 导入以兼容其他文件中可能的使用
var _ aimodel.DataSensitivityLevel
