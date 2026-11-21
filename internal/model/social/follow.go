package social

// ============================================================
// 用户关注模型 (对应 aw_follow 表)
// type=1: 关注文档  type=2: 关注用户
// ============================================================

// FollowType 关注类型常量
const (
	FollowTypeDocument = 1 // 关注文档
	FollowTypeUser     = 2 // 关注用户
)

// Follow 用户关注模型
type Follow struct {
	FollowID   int `gorm:"column:follow_id;primaryKey;autoIncrement" json:"follow_id"`
	UserID     int `gorm:"column:user_id;index:idx_user_id;uniqueIndex:idx_user_object_type" json:"user_id"`
	Type       int `gorm:"column:type;uniqueIndex:idx_user_object_type" json:"type"` // 1=文档 2=用户
	ObjectID   int `gorm:"column:object_id;index:idx_object_id;uniqueIndex:idx_user_object_type" json:"object_id"`
	CreateTime int `gorm:"column:create_time" json:"create_time"`
	UpdateTime int `gorm:"column:update_time" json:"update_time"`
}

// 在 AutoMigrate 后手动补建联合唯一索引（GORM 不支持通过 tag 直接建复合唯一索引）
// 该逻辑在 internal/repository/db.go 的 migrateIndexes 函数中统一执行

// TableName 指定表名
func (Follow) TableName() string {
	return "aw_follow"
}

// FollowUserInfo 关注的用户信息
type FollowUserInfo struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	GivenName string `json:"given_name"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar,omitempty"`
}

// FollowDocInfo 关注的文档信息
type FollowDocInfo struct {
	DocumentID int    `json:"document_id"`
	Name       string `json:"name"`
	SpaceID    int    `json:"space_id"`
	SpaceName  string `json:"space_name,omitempty"`
}

// FollowResponse 关注响应
type FollowResponse struct {
	FollowID   int `json:"id"`
	UserID     int `json:"user_id"`
	Type       int `json:"type"`
	ObjectID   int `json:"object_id"`
	CreateTime int `json:"create_time"`
	// 扩展字段（填充后返回）
	ObjectName string          `json:"object_name,omitempty"`
	TargetUser *FollowUserInfo `json:"target_user,omitempty"`
	TargetDoc  *FollowDocInfo  `json:"target_doc,omitempty"`
}

// ToResponse 转换为响应对象
func (f *Follow) ToResponse() FollowResponse {
	return FollowResponse{
		FollowID:   f.FollowID,
		UserID:     f.UserID,
		Type:       f.Type,
		ObjectID:   f.ObjectID,
		CreateTime: f.CreateTime,
	}
}
