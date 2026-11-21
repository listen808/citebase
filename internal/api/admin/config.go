package admin

import (
	"sort"

	"github.com/gin-gonic/gin"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
)

// tokenUsageGroup 是 AI Token 计数器写入 aw_config 的内部组（每用户/每天一条），
// 不属于后台可编辑配置，列表里必须滤掉。
const tokenUsageGroup = "ai_token_usage"

// ConfigAPI 系统配置管理 API
type ConfigAPI struct {
	configService *adminsvc.ConfigService
}

func NewConfigAPI() *ConfigAPI {
	return &ConfigAPI{
		configService: adminsvc.NewConfigService(),
	}
}

// GetConfigs 获取所有系统配置（按分组）
// @Summary 系统配置列表
// @Tags 系统配置
// @Produce json
// @Success 200
// @Router /api/v1/configs [get]
func (api *ConfigAPI) GetConfigs(c *gin.Context) {
	groupKey := c.DefaultQuery("group_key", "")

	var configs []*adminmodel.ConfigResponse
	var err error

	if groupKey != "" {
		configs, err = api.configService.GetConfigsByGroup(groupKey)
	} else {
		configs, err = api.configService.GetAllConfigs()
	}

	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 按 group_key 分组
	groupMap := make(map[string][]*adminmodel.ConfigResponse)
	for _, cfg := range configs {
		gk := cfg.GroupKey
		if gk == "" {
			gk = "other"
		}
		// ai_token_usage 是计数器写入的内部组（每天一条 aw_config 记录），
		// 不是可编辑配置。之前它会作为裸 key 出现在「系统设置」左侧导航里。
		if gk == tokenUsageGroup {
			continue
		}
		groupMap[gk] = append(groupMap[gk], cfg)
	}

	// 转换为分组响应（组顺序固定：已知组按 groupOrder，未知组按 key 字母序排在后面；
	// 此前用 map 遍历导致每次刷新分组顺序都不同）
	type GroupItem struct {
		Key   string                       `json:"key"`
		Label string                       `json:"label"`
		Items []*adminmodel.ConfigResponse `json:"items"`
	}
	groupLabels := map[string]string{
		"basic":  "基础配置",
		"search": "搜索配置",
		"ai":     "AI 模型",
		"system": "系统信息",
		"email":  "邮件配置",
		"auth":   "认证配置",
		"other":  "其他",
	}
	groupOrder := map[string]int{"basic": 0, "search": 1, "ai": 2, "system": 3, "email": 4, "auth": 5}

	keys := make([]string, 0, len(groupMap))
	for k := range groupMap {
		keys = append(keys, k)
	}
	orderOf := func(k string) int {
		if o, ok := groupOrder[k]; ok {
			return o
		}
		return 100
	}
	sort.Slice(keys, func(i, j int) bool {
		oi, oj := orderOf(keys[i]), orderOf(keys[j])
		if oi != oj {
			return oi < oj
		}
		return keys[i] < keys[j]
	})

	groups := make([]GroupItem, 0, len(keys))
	for _, k := range keys {
		label := groupLabels[k]
		if label == "" {
			label = k
		}
		groups = append(groups, GroupItem{Key: k, Label: label, Items: groupMap[k]})
	}

	response.Success(c, groups)
}

// UpdateConfig 更新配置项
// @Summary 更新配置
// @Tags 系统配置
// @Accept json
// @Produce json
// @Param key query string true "配置键"
// @Param body body object true "配置值" example({"value": "new_value"})
// @Success 200
// @Router /api/v1/configs [put]
func (api *ConfigAPI) UpdateConfig(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		response.BadRequest(c, "key is required")
		return
	}

	var req adminmodel.UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := api.configService.UpdateConfig(key, req.Value); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.SuccessWithMsg(c, nil, "配置更新成功")
}

// GetConfigValue 获取单个配置值
// @Summary 获取配置值
// @Tags 系统配置
// @Produce json
// @Param key query string true "配置键"
// @Success 200
// @Router /api/v1/configs/value [get]
func (api *ConfigAPI) GetConfigValue(c *gin.Context) {
	key := c.Query("key")
	if key == "" {
		response.BadRequest(c, "key is required")
		return
	}

	value, err := api.configService.GetConfigValue(key)
	if err != nil {
		response.Success(c, gin.H{"key": key, "value": ""})
		return
	}

	response.Success(c, gin.H{"key": key, "value": value})
}
