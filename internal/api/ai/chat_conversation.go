package ai

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/utils"
)

// 会话 CRUD。

func (api *ChatAPI) ListConversations(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	page, pageSize, _ := utils.ParsePagination(c, 20)
	convType := c.DefaultQuery("type", "")
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	convRepo := airepo.NewAIConversationRepository()

	var convs []*aimodel.AIConversation
	var total int64

	if convType != "" {
		convs, total, err = convRepo.FindByUserIDAndType(uid, convType, pageSize, (page-1)*pageSize)
	} else {
		convs, total, err = convRepo.FindByUserID(uid, pageSize, (page-1)*pageSize)
	}

	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"conversations": convs,
		"total":         total,
		"page":          page,
		"page_size":     pageSize,
	})
}

func (api *ChatAPI) CreateConversation(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var req struct {
		Title   string `json:"title"`
		SpaceID int    `json:"space_id"`
		Type    string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Title = "新对话"
	}

	convType := req.Type
	if convType == "" {
		convType = "ai"
	}

	conv := &aimodel.AIConversation{
		UserID:   uid,
		SpaceID:  req.SpaceID,
		Type:     convType,
		Title:    req.Title,
		Messages: aimodel.ConvMsgs{},
	}
	if conv.Title == "" {
		conv.Title = "新对话"
	}

	convRepo := airepo.NewAIConversationRepository()
	if err := convRepo.Create(conv); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"conversation": conv})
}

func (api *ChatAPI) GetConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid conversation id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	convRepo := airepo.NewAIConversationRepository()
	conv, err := convRepo.FindByID(id)
	if err != nil {
		response.NotFoundKey(c, response.MsgConversationNotFound)
		return
	}

	if uid > 0 && conv.UserID != uid {
		response.ForbiddenKey(c, response.MsgNoPermissionConversation)
		return
	}

	response.Success(c, gin.H{"conversation": conv})
}

func (api *ChatAPI) DeleteConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid conversation id")
		return
	}

	userID, exists := c.Get("user_id")
	if !exists {
		response.Unauthorized(c, "user not authenticated")
		return
	}
	uid, ok := userID.(int)
	if !ok {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	convRepo := airepo.NewAIConversationRepository()
	if uid > 0 {
		conv, err := convRepo.FindByID(id)
		if err != nil {
			response.NotFoundKey(c, response.MsgConversationNotFound)
			return
		}
		if conv.UserID != uid {
			response.ForbiddenKey(c, response.MsgNoPermissionConversationDel)
			return
		}
	}

	if err := convRepo.Delete(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}

func (api *ChatAPI) UpdateConversation(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid conversation id")
		return
	}

	var req struct {
		Title    string                   `json:"title"`
		Messages []map[string]interface{} `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	convRepo := airepo.NewAIConversationRepository()
	conv, err := convRepo.FindByID(id)
	if err != nil {
		response.NotFoundKey(c, response.MsgConversationNotFound)
		return
	}

	if uid > 0 && conv.UserID != uid {
		response.ForbiddenKey(c, response.MsgNoPermissionConversationUpd)
		return
	}

	if req.Title != "" {
		conv.Title = req.Title
	}
	if req.Messages != nil {
		conv.Messages = aimodel.ConvMsgs(req.Messages)
	}

	if err := convRepo.Update(conv); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"conversation": conv})
}
