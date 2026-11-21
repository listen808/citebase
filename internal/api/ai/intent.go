package ai

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

type IntentAPI struct {
	svc *aiSvc.IntentService
}

func NewIntentAPI() *IntentAPI {
	return &IntentAPI{svc: aiSvc.NewIntentService()}
}

// DetectIntent 识别用户输入意图
// @Summary 识别用户输入意图
// @Tags AI
// @Accept json
// @Produce json
// @Param body body ai.DetectIntentRequest true "待识别的问题"
// @Success 200 {object} ai.DetectIntentResponse
// @Router /api/v1/ai/intent/detect [post]
func (api *IntentAPI) DetectIntent(c *gin.Context) {
	var req aiSvc.DetectIntentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	response.Success(c, api.svc.DetectIntent(c.Request.Context(), ai.GetModule(), &req))
}
