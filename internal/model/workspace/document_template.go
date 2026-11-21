package workspace

// DocumentTemplate 文档模板模型 (对应 aw_document_template 表)
// 用于快速创建格式统一的文档（如会议纪要、需求文档等）。
type DocumentTemplate struct {
	ID           int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID      int    `gorm:"column:space_id;index:idx_tpl_space" json:"space_id"` // 0 表示全局模板
	Name         string `gorm:"column:name;size:100;uniqueIndex:uk_tpl_space_name" json:"name"`
	Description  string `gorm:"column:description;size:255" json:"description"`
	Content      string `gorm:"column:content;type:longtext" json:"content"`
	CreateUserID int    `gorm:"column:create_user_id;index:idx_create_user_id" json:"create_user_id"`
	CreateTime   int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime   int    `gorm:"column:update_time" json:"update_time"`
}

func (DocumentTemplate) TableName() string {
	return "aw_document_template"
}

// CreateTemplateRequest 创建模板请求
type CreateTemplateRequest struct {
	SpaceID     int    `json:"space_id"`
	Name        string `json:"name" binding:"required,max=100"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// UpdateTemplateRequest 更新模板请求
type UpdateTemplateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// TemplateResponse 模板响应
type TemplateResponse struct {
	ID           int    `json:"id"`
	SpaceID      int    `json:"space_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Content      string `json:"content"`
	CreateUserID int    `json:"create_user_id"`
	CreateTime   int    `json:"create_time"`
	UpdateTime   int    `json:"update_time"`
}

// ToResponse 转换为响应
func (t *DocumentTemplate) ToResponse() TemplateResponse {
	return TemplateResponse{
		ID:           t.ID,
		SpaceID:      t.SpaceID,
		Name:         t.Name,
		Description:  t.Description,
		Content:      t.Content,
		CreateUserID: t.CreateUserID,
		CreateTime:   t.CreateTime,
		UpdateTime:   t.UpdateTime,
	}
}
