package social

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	socialsvc "github.com/listen808/citebase/internal/service/social"
)

type FollowAPI struct {
	followService *socialsvc.FollowService
}

func NewFollowAPI() *FollowAPI {
	return &FollowAPI{
		followService: socialsvc.NewFollowService(),
	}
}

// ToggleFollowUser 关注或取消关注用户
// @Summary 关注或取消关注用户
// @Tags 关注
// @Accept json
// @Produce json
// @Param request body object{user_id=int} true "目标用户ID"
// @Success 200 {object} object{followed=bool, follow=object}
// @Router /api/v1/follow/user [post]
func (api *FollowAPI) ToggleFollowUser(c *gin.Context) {
	var req struct {
		UserID int `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 从 context 获取当前用户ID
	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	followed, followResp, err := api.followService.ToggleFollowUser(currentUserID, req.UserID)
	if err != nil {
		if err == socialsvc.ErrCannotFollowSelf {
			response.BadRequest(c, err.Error())
			return
		}
		if err == userrepo.ErrUserNotFound {
			response.NotFound(c, "user not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"followed": followed,
		"follow":   followResp,
	})
}

// GetFollowingList 获取我关注的人列表
// @Summary 获取关注列表
// @Tags 关注
// @Produce json
// @Param user_id query int false "用户ID，不传则查当前用户"
// @Success 200 {object} object{data=[]object, following=int, followers=int}
// @Router /api/v1/follow/following [get]
func (api *FollowAPI) GetFollowingList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	var userID int
	var err error

	if userIDStr != "" {
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			response.BadRequest(c, "invalid user_id")
			return
		}
	} else {
		uid, err := middleware.GetUserID(c)
		if err != nil {
			response.Unauthorized(c, "unauthorized")
			return
		}
		userID = uid
	}

	list, err := api.followService.GetFollowingList(userID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	following, followers, _ := api.followService.GetFollowStats(userID)

	response.Success(c, gin.H{
		"data":      list,
		"following": following,
		"followers": followers,
	})
}

// GetFollowersList 获取我的粉丝列表
// @Summary 获取粉丝列表
// @Tags 关注
// @Produce json
// @Param user_id query int false "用户ID，不传则查当前用户"
// @Success 200 {object} object{data=[]object, followers=int}
// @Router /api/v1/follow/followers [get]
func (api *FollowAPI) GetFollowersList(c *gin.Context) {
	userIDStr := c.Query("user_id")
	var userID int
	var err error

	if userIDStr != "" {
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			response.BadRequest(c, "invalid user_id")
			return
		}
	} else {
		uid, err := middleware.GetUserID(c)
		if err != nil {
			response.Unauthorized(c, "unauthorized")
			return
		}
		userID = uid
	}

	list, err := api.followService.GetFollowersList(userID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	_, followers, _ := api.followService.GetFollowStats(userID)

	response.Success(c, gin.H{
		"data":      list,
		"followers": followers,
	})
}

// ToggleFollowDocument 关注或取消关注文档
// @Summary 关注或取消关注文档
// @Tags 关注
// @Accept json
// @Produce json
// @Param request body object{document_id=int} true "文档ID"
// @Success 200 {object} object{followed=bool, follow=object}
// @Router /api/v1/follow/document [post]
func (api *FollowAPI) ToggleFollowDocument(c *gin.Context) {
	var req struct {
		DocumentID int `json:"document_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	followed, followResp, err := api.followService.ToggleFollowDocument(currentUserID, req.DocumentID)
	if err != nil {
		if err == workspacerepo.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"followed": followed,
		"follow":   followResp,
	})
}

// CheckFollowStatus 检查关注状态
// @Summary 检查是否关注用户/文档
// @Tags 关注
// @Produce json
// @Param type query int true "关注类型: 1=文档 2=用户"
// @Param object_id query int true "对象ID"
// @Success 200 {object} object{is_following=bool}
// @Router /api/v1/follow/check [get]
func (api *FollowAPI) CheckFollowStatus(c *gin.Context) {
	typeStr := c.Query("type")
	objectIDStr := c.Query("object_id")

	followType, err := strconv.Atoi(typeStr)
	if err != nil || (followType != 1 && followType != 2) {
		response.BadRequest(c, "invalid type, must be 1 (document or 2 (user)")
		return
	}

	objectID, err := strconv.Atoi(objectIDStr)
	if err != nil {
		response.BadRequest(c, "invalid object_id")
		return
	}

	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	var isFollowing bool
	if followType == 2 {
		isFollowing = api.followService.IsFollowingUser(currentUserID, objectID)
	} else {
		isFollowing = api.followService.IsFollowingDocument(currentUserID, objectID)
	}

	response.Success(c, gin.H{
		"is_following": isFollowing,
	})
}

// GetFollowStats 获取关注统计
// @Summary 获取关注统计
// @Tags 关注
// @Produce json
// @Param user_id query int false "用户ID，不传则查当前用户"
// @Success 200 {object} object{following=int, followers=int}
// @Router /api/v1/follow/stats [get]
func (api *FollowAPI) GetFollowStats(c *gin.Context) {
	userIDStr := c.Query("user_id")
	var userID int
	var err error

	if userIDStr != "" {
		userID, err = strconv.Atoi(userIDStr)
		if err != nil {
			response.BadRequest(c, "invalid user_id")
			return
		}
	} else {
		uid, err := middleware.GetUserID(c)
		if err != nil {
			response.Unauthorized(c, "unauthorized")
			return
		}
		userID = uid
	}

	following, followers, err := api.followService.GetFollowStats(userID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"following": following,
		"followers": followers,
	})
}
