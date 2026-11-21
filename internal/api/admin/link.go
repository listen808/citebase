package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils"
)

// LinkAPI 快捷链接 API
type LinkAPI struct {
	linkService *adminsvc.LinkService
}

func NewLinkAPI() *LinkAPI {
	return &LinkAPI{
		linkService: adminsvc.NewLinkService(),
	}
}

// GetLinks 获取链接列表
func (api *LinkAPI) GetLinks(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	if pageSize > 100 {
		pageSize = 100
	}
	keyword := c.DefaultQuery("keyword", "")

	links, total, err := api.linkService.GetLinks(page, pageSize, keyword)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, links, total, page, pageSize)
}

// GetAllLinks 获取全部链接（按 sequence 排序）
func (api *LinkAPI) GetAllLinks(c *gin.Context) {
	links, err := api.linkService.GetAllLinks()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, links)
}

// RecordClick POST /api/v1/links/:id/click —— 记一次打开并回累计次数。
// 权限走 PrivilegeMap 的 MinRole 3（任何登录用户）：链接将来摆到用户面前时，
// 计数不该只算管理员点的那几次。
func (api *LinkAPI) RecordClick(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		response.BadRequestKey(c, response.MsgLinkIDInvalid)
		return
	}
	count, err := api.linkService.RecordClick(uint(id))
	if err != nil {
		if err == adminrepo.ErrLinkNotFound {
			response.NotFoundKey(c, response.MsgLinkNotFound)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"id": id, "click_count": count})
}

// SaveLink 新增链接
func (api *LinkAPI) SaveLink(c *gin.Context) {
	var req struct {
		Name string `json:"name" form:"name" binding:"required"`
		URL  string `json:"url" form:"url" binding:"required"`
		Sort int    `json:"sort" form:"sort"`
	}

	if err := c.ShouldBind(&req); err != nil {
		response.BadRequestKey(c, response.MsgLinkFieldsEmpty)
		return
	}

	link, err := api.linkService.CreateLink(req.Name, req.URL, req.Sort)
	if err != nil {
		if err == adminsvc.ErrLinkNameExists {
			response.ConflictKey(c, response.MsgLinkNameTaken)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, link, "添加成功")
}

// UpdateLink 修改链接
func (api *LinkAPI) UpdateLink(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		response.BadRequestKey(c, response.MsgLinkNotFound)
		return
	}

	var req struct {
		Name string `json:"name" form:"name" binding:"required"`
		URL  string `json:"url" form:"url" binding:"required"`
		Sort int    `json:"sort" form:"sort"`
	}

	if err := c.ShouldBind(&req); err != nil {
		response.BadRequestKey(c, response.MsgLinkFieldsEmpty)
		return
	}

	link, err := api.linkService.UpdateLink(uint(id), req.Name, req.URL, req.Sort)
	if err != nil {
		if err == adminsvc.ErrLinkNameExists {
			response.ConflictKey(c, response.MsgLinkNameTaken)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, link, "修改成功")
}

// DeleteLink 删除链接
func (api *LinkAPI) DeleteLink(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		response.BadRequestKey(c, response.MsgNoLinkChosen)
		return
	}

	if err := api.linkService.DeleteLink(uint(id)); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}
