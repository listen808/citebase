package admin

import (
	"strconv"

	"github.com/gin-gonic/gin"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils"
)

// PluginAPI 插件管理 API
type PluginAPI struct {
	pluginService *adminsvc.PluginService
}

func NewPluginAPI() *PluginAPI {
	return &PluginAPI{
		pluginService: adminsvc.NewPluginService(),
	}
}

// GetPlugins 获取插件列表
func (api *PluginAPI) GetPlugins(c *gin.Context) {
	page, pageSize, _ := utils.ParsePagination(c, 20)
	keyword := c.DefaultQuery("keyword", "")

	responses, total, err := api.pluginService.GetPlugins(keyword, page, pageSize)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Paginated(c, responses, total, page, pageSize)
}

// GetPlugin 获取单个插件
func (api *PluginAPI) GetPlugin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid plugin id")
		return
	}

	resp, err := api.pluginService.GetPlugin(uint(id))
	if err != nil {
		response.NotFound(c, "plugin not found")
		return
	}

	response.Success(c, resp)
}

// CreatePlugin 创建插件
func (api *PluginAPI) CreatePlugin(c *gin.Context) {
	var req adminmodel.CreatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := api.pluginService.CreatePlugin(&req)
	if err != nil {
		if err == adminsvc.ErrPluginKeyExists {
			response.BadRequestKey(c, response.MsgPluginKeyTaken)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, resp)
}

// UpdatePlugin 更新插件
func (api *PluginAPI) UpdatePlugin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid plugin id")
		return
	}

	var req adminmodel.UpdatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := api.pluginService.UpdatePlugin(uint(id), &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, resp)
}

// UpdatePluginConfig 更新插件配置
func (api *PluginAPI) UpdatePluginConfig(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid plugin id")
		return
	}

	var req adminmodel.UpdatePluginConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := api.pluginService.UpdatePluginConfig(uint(id), req.ConfValue); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "配置更新成功")
}

// TogglePlugin 启用/停用插件
func (api *PluginAPI) TogglePlugin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid plugin id")
		return
	}

	msg, err := api.pluginService.TogglePlugin(uint(id))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, msg)
}

// DeletePlugin 删除插件
func (api *PluginAPI) DeletePlugin(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "invalid plugin id")
		return
	}

	if err := api.pluginService.DeletePlugin(uint(id)); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "删除成功")
}
