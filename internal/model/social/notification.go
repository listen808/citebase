package social

// Notification 通知模型 (对应 aw_notification 表)
type Notification struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int    `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	Type       string `gorm:"column:type;size:50" json:"type"` // system, document, comment, follow, like, workspace
	Title      string `gorm:"column:title;size:255" json:"title"`
	Content    string `gorm:"column:content;size:1000" json:"content"`
	IsRead     int    `gorm:"column:is_read;default:0" json:"is_read"` // 0 未读 1 已读
	RelatedID  int    `gorm:"column:related_id" json:"related_id"`
	CreateTime int    `gorm:"column:create_time" json:"create_time"`
	UpdateTime int    `gorm:"column:update_time" json:"update_time"`
}

func (Notification) TableName() string {
	return "aw_notification"
}

// NotificationType 通知类型常量
const (
	NotifTypeSystem    = "system"
	NotifTypeDocument  = "document"
	NotifTypeComment   = "comment"
	NotifTypeFollow    = "follow"
	NotifTypeLike      = "like"
	NotifTypeWorkspace = "workspace"
)

// NotificationResponse 通知响应
type NotificationResponse struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	IsRead     int    `json:"is_read"`
	RelatedID  int    `json:"related_id"`
	CreateTime int    `json:"create_time"`
}

func (n *Notification) ToResponse() NotificationResponse {
	return NotificationResponse{
		ID:         n.ID,
		UserID:     n.UserID,
		Type:       n.Type,
		Title:      n.Title,
		Content:    n.Content,
		IsRead:     n.IsRead,
		RelatedID:  n.RelatedID,
		CreateTime: n.CreateTime,
	}
}
