package admin

import "gorm.io/gorm"

// ============================================================
// 日志模型
// 对应表：aw_log_document / aw_log
// ============================================================

// DocumentLog 文档操作日志 (aw_log_document)
type DocumentLog struct {
	ID            int    `gorm:"-" json:"id"` // 仅用于响应，数据库主键是 log_document_id
	LogDocumentID int    `gorm:"column:log_document_id;primaryKey;autoIncrement" json:"log_document_id"`
	DocumentID    int    `gorm:"column:document_id;index:idx_document_id" json:"document_id"`
	SpaceID       int    `gorm:"column:space_id;index:idx_space_id" json:"space_id"`
	UserID        int    `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	Action        int    `gorm:"column:action" json:"action"` // 1 创建 2 修改 3 删除
	Comment       string `gorm:"column:comment;size:255" json:"comment"`
	Content       string `gorm:"column:content;size:1000" json:"content"`
	CreateTime    int    `gorm:"column:create_time" json:"create_time"`
}

// TableName 表名
func (DocumentLog) TableName() string {
	return "aw_log_document"
}

// 文档操作类型常量
const (
	DocActionCreate  = 1
	DocActionModify  = 2
	DocActionDelete  = 3
	DocActionRestore = 4
)

// DocumentLogResponse 日志响应
type DocumentLogResponse struct {
	ID           int    `json:"id"`
	DocumentID   int    `json:"document_id"`
	DocumentName string `json:"document_name,omitempty"`
	SpaceID      int    `json:"space_id"`
	SpaceName    string `json:"space_name,omitempty"`
	UserID       int    `json:"user_id"`
	Username     string `json:"username,omitempty"`
	GivenName    string `json:"given_name,omitempty"`
	Action       int    `json:"action"`
	ActionName   string `json:"action_name"`
	Comment      string `json:"comment"`
	CreateTime   int    `json:"create_time"`
}

// ToResponse 转换响应
func (d *DocumentLog) ToResponse() DocumentLogResponse {
	actionName := ""
	switch d.Action {
	case DocActionCreate:
		actionName = "创建"
	case DocActionModify:
		actionName = "修改"
	case DocActionDelete:
		actionName = "删除"
	case DocActionRestore:
		actionName = "恢复"
	}
	return DocumentLogResponse{
		ID:         d.LogDocumentID,
		DocumentID: d.DocumentID,
		SpaceID:    d.SpaceID,
		UserID:     d.UserID,
		Action:     d.Action,
		ActionName: actionName,
		Comment:    d.Comment,
		CreateTime: d.CreateTime,
	}
}

// SystemLog 系统操作日志 (aw_log)
type SystemLog struct {
	ID         int64  `gorm:"-" json:"id"` // 仅用于响应，数据库主键是 log_id
	LogID      int64  `gorm:"column:log_id;primaryKey;autoIncrement" json:"log_id"`
	Level      int    `gorm:"column:level" json:"-"` // 日志级别（int）：3=ERROR 4=WARN 6=INFO 7=DEBUG
	LevelName  string `gorm:"-" json:"level"`        // P0 修复：前端期望 level 为 string（如 "ERROR"），由 AfterFind 填充
	URL        string `gorm:"column:url;size:500" json:"url"`
	Method     string `gorm:"column:method;size:10" json:"method"`
	Param      string `gorm:"column:param;type:text" json:"param"`
	Path       string `gorm:"column:path;size:100" json:"path"`
	GetParams  string `gorm:"column:get;type:text" json:"get_params"`   // P0 修复：JSON 字段名对齐前端
	PostParams string `gorm:"column:post;type:text" json:"post_params"` // P0 修复：JSON 字段名对齐前端
	Message    string `gorm:"column:message;size:255" json:"message"`
	IP         string `gorm:"column:ip;size:100" json:"ip"`
	UserAgent  string `gorm:"column:user_agent;size:200" json:"user_agent"`
	Referer    string `gorm:"column:referer;size:100" json:"referer"`
	UserID     int    `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	Username   string `gorm:"column:username;size:100" json:"username"`
	CreateTime int    `gorm:"column:create_time;index:idx_create_time" json:"create_time"`
}

// AfterFind 查询后钩子：将 int Level 转换为 string LevelName
func (s *SystemLog) AfterFind(tx *gorm.DB) error {
	s.LevelName = SystemLogLevelName(s.Level)
	// 兼容前端 RecentError 接口（id 字段）
	s.ID = s.LogID
	return nil
}

// SystemLogLevelName 将数字日志级别映射为字符串名
func SystemLogLevelName(level int) string {
	switch level {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// TableName 表名
func (SystemLog) TableName() string {
	return "aw_log"
}

// LogLevel 日志级别
const (
	LogLevelDebug = 7
	LogLevelInfo  = 6
	LogLevelWarn  = 4
	LogLevelError = 3
)
