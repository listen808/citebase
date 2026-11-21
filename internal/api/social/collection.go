package social

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	socialsvc "github.com/listen808/citebase/internal/service/social"
	"github.com/listen808/citebase/internal/utils"
)

// CollectionAPI 收藏 API
type CollectionAPI struct {
	collectionService *socialsvc.CollectionService
}

func NewCollectionAPI() *CollectionAPI {
	return &CollectionAPI{
		collectionService: socialsvc.NewCollectionService(),
	}
}

// ToggleCollection 收藏/取消收藏（文档或空间）
// @Summary 收藏/取消收藏
// @Tags 收藏
// @Accept json
// @Produce json
// @Param request body object{type=string,resource_id=string} true "type: 1=文档 2=空间"
// @Success 200 {object} object
// @Router /api/v1/collections/toggle [post]
func (api *CollectionAPI) ToggleCollection(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	var req struct {
		Type       int `json:"type" binding:"required,oneof=1 2"`
		ResourceID int `json:"resource_id" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	collected, collectionID, err := api.collectionService.ToggleCollection(uid, req.Type, req.ResourceID)
	if err != nil {
		if err == socialsvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		if err == socialsvc.ErrSpaceNotFound {
			response.NotFound(c, "space not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	if collected {
		response.SuccessWithMsg(c, gin.H{
			"collected":     true,
			"collection_id": collectionID,
		}, "收藏成功")
		return
	}
	response.SuccessWithMsg(c, gin.H{
		"collected":     false,
		"collection_id": collectionID,
	}, "取消收藏成功")
}

// GetMyCollections 获取我的收藏列表
// @Summary 我的收藏
// @Tags 收藏
// @Produce json
// @Param type query int false "收藏类型 1=文档 2=空间"
// @Param keyword query string false "搜索关键词"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} object
// @Router /api/v1/collections [get]
func (api *CollectionAPI) GetMyCollections(c *gin.Context) {
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	colType, _ := strconv.Atoi(c.DefaultQuery("type", "0"))
	keyword := c.Query("keyword")
	page, pageSize, _ := utils.ParsePagination(c, 20)

	items, total, err := api.collectionService.GetMyCollections(uid, colType, keyword, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, items, int64(total), page, pageSize)
}

// IsCollected 检查是否已收藏
// @Summary 检查收藏状态
// @Tags 收藏
// @Produce json
// @Param type query int true "收藏类型 1=文档 2=空间"
// @Param resource_id query int true "资源 ID"
// @Success 200 {object} object
// @Router /api/v1/collections/check [get]
func (api *CollectionAPI) IsCollected(c *gin.Context) {
	uid, _ := middleware.GetUserID(c)
	colType, _ := strconv.Atoi(c.Query("type"))
	resourceID, _ := strconv.Atoi(c.Query("resource_id"))

	if uid == 0 || colType == 0 || resourceID == 0 {
		response.BadRequest(c, "invalid parameters")
		return
	}

	collected, collectionID, err := api.collectionService.IsCollected(uid, colType, resourceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"collected":     collected,
		"collection_id": collectionID,
	})
}
