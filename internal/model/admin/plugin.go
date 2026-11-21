package admin

import "time"

// Plugin 插件
type Plugin struct {
	ID          uint   `gorm:"primaryKey;column:plugin_id" json:"id"`
	PluginKey   string `gorm:"column:plugin_key;size:50" json:"plugin_key"`
	Title       string `gorm:"column:title" json:"title"`
	Description string `gorm:"column:description" json:"description"`
	CoverURL    string `gorm:"column:cover_url" json:"cover_url"`
	ConfValue   string `gorm:"column:conf_value" json:"conf_value"`
	IsUsed      int    `gorm:"column:is_used;default:0" json:"is_used"`
	CreateTime  int64  `gorm:"column:create_time" json:"create_time"`
	UpdateTime  int64  `gorm:"column:update_time" json:"update_time"`
}

// TableName 插件表名
func (Plugin) TableName() string {
	return "aw_plugin"
}

// PluginResponse 插件响应
type PluginResponse struct {
	ID          uint   `json:"id"`
	PluginKey   string `json:"plugin_key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	ConfValue   string `json:"conf_value"`
	IsUsed      int    `json:"is_used"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// ToResponse 转换为响应格式
func (p *Plugin) ToResponse() PluginResponse {
	created := "-"
	if p.CreateTime > 0 {
		created = time.Unix(p.CreateTime, 0).Format("2006-01-02 15:04")
	}
	updated := "-"
	if p.UpdateTime > 0 {
		updated = time.Unix(p.UpdateTime, 0).Format("2006-01-02 15:04")
	}
	return PluginResponse{
		ID:          p.ID,
		PluginKey:   p.PluginKey,
		Title:       p.Title,
		Description: p.Description,
		CoverURL:    p.CoverURL,
		ConfValue:   p.ConfValue,
		IsUsed:      p.IsUsed,
		CreatedAt:   created,
		UpdatedAt:   updated,
	}
}

// CreatePluginRequest 创建插件请求
type CreatePluginRequest struct {
	PluginKey   string `json:"plugin_key" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	ConfValue   string `json:"conf_value"`
	IsUsed      int    `json:"is_used"`
}

// UpdatePluginRequest 更新插件请求
type UpdatePluginRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	CoverURL    string `json:"cover_url"`
	IsUsed      int    `json:"is_used"`
}

// UpdatePluginConfigRequest 更新插件配置请求
type UpdatePluginConfigRequest struct {
	ConfValue string `json:"conf_value" binding:"required"`
}
